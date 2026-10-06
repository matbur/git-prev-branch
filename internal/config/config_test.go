package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/matbur/git-prev-branch/internal/config"
)

// isolatedHome points HOME (and USERPROFILE, for Windows) at an empty
// temporary directory, so a developer's real user config can never leak into
// a test run.
func isolatedHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func userConfig(home string) string {
	return filepath.Join(home, ".config", "git-prev-branch", "config.yaml")
}

// newRepo makes dir look like a repository root to findRepoRoot, which only
// ever stats .git -- no git invocation is involved.
func newRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	return dir
}

func repoConfig(root string) string {
	return filepath.Join(root, ".config", "git-prev-branch", "config.yaml")
}

func TestLoadUsesBuiltInDefaultsWhenNothingExists(t *testing.T) {
	isolatedHome(t)

	cfg, err := config.Load("", t.TempDir())
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.Interactive.DefaultAction != "reject" {
		t.Errorf("default_action = %q, want %q", cfg.Interactive.DefaultAction, "reject")
	}
	if cfg.Interactive.AcceptsByDefault() {
		t.Error("AcceptsByDefault() = true, want false for the shipped default")
	}
}

func TestLoadExplicitPathThatDoesNotExistIsAnError(t *testing.T) {
	isolatedHome(t)

	missing := filepath.Join(t.TempDir(), "nope.yaml")
	if _, err := config.Load(missing, ""); !errors.Is(err, config.ErrConfigNotFound) {
		t.Errorf("Load(%q) error = %v, want ErrConfigNotFound", missing, err)
	}
}

func TestLoadMalformedFileIsAnError(t *testing.T) {
	isolatedHome(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "interactive: [unclosed\n")

	if _, err := config.Load(path, ""); !errors.Is(err, config.ErrConfigMalformed) {
		t.Errorf("Load() error = %v, want ErrConfigMalformed", err)
	}
}

func TestLoadUnknownDefaultActionIsAnError(t *testing.T) {
	isolatedHome(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "interactive:\n  default_action: maybe\n")

	_, err := config.Load(path, "")
	if !errors.Is(err, config.ErrConfigMalformed) {
		t.Fatalf("Load() error = %v, want ErrConfigMalformed", err)
	}
}

func TestLoadNormalizesDefaultAction(t *testing.T) {
	isolatedHome(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "interactive:\n  default_action: \"  AcCePt \"\n")

	cfg, err := config.Load(path, "")
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.Interactive.DefaultAction != "accept" {
		t.Errorf("default_action = %q, want %q", cfg.Interactive.DefaultAction, "accept")
	}
	if !cfg.Interactive.AcceptsByDefault() {
		t.Error("AcceptsByDefault() = false, want true")
	}
}

func TestLoadEmptyDefaultActionKeepsTheRejectDefault(t *testing.T) {
	isolatedHome(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "interactive:\n  default_action: \"\"\n")

	cfg, err := config.Load(path, "")
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.Interactive.DefaultAction != "reject" {
		t.Errorf("default_action = %q, want %q", cfg.Interactive.DefaultAction, "reject")
	}
}

func TestLoadToleratesUnknownKeys(t *testing.T) {
	isolatedHome(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "future_section:\n  whatever: true\ninteractive:\n  default_action: accept\n")

	cfg, err := config.Load(path, "")
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.Interactive.DefaultAction != "accept" {
		t.Errorf("default_action = %q, want %q", cfg.Interactive.DefaultAction, "accept")
	}
}

func TestLoadPrecedence(t *testing.T) {
	home := isolatedHome(t)
	repo := newRepo(t)

	// All three locations exist; the repository-level file must win.
	writeFile(t, repoConfig(repo), "interactive:\n  default_action: accept\n")
	writeFile(t, userConfig(home), "interactive:\n  default_action: reject\n")
	explicit := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, explicit, "interactive:\n  default_action: reject\n")

	for _, tc := range []struct {
		name         string
		explicitPath string
		want         string
	}{
		{"explicit path wins", explicit, "reject"},
		{"repository config wins over user config", "", "accept"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := config.Load(tc.explicitPath, repo)
			if err != nil {
				t.Fatalf("Load() = %v", err)
			}
			if cfg.Interactive.DefaultAction != tc.want {
				t.Errorf("default_action = %q, want %q", cfg.Interactive.DefaultAction, tc.want)
			}
		})
	}

	// Without the repository file the user-level one is used...
	if err := os.Remove(repoConfig(repo)); err != nil {
		t.Fatalf("remove repo config: %v", err)
	}
	cfg, err := config.Load("", repo)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.Interactive.DefaultAction != "reject" {
		t.Errorf("default_action = %q, want %q", cfg.Interactive.DefaultAction, "reject")
	}

	// ...and outside any repository the user-level one is still used.
	writeFile(t, userConfig(home), "interactive:\n  default_action: accept\n")
	cfg, err = config.Load("", t.TempDir())
	if err != nil {
		t.Fatalf("Load() outside a repository = %v", err)
	}
	if cfg.Interactive.DefaultAction != "accept" {
		t.Errorf("default_action = %q, want %q", cfg.Interactive.DefaultAction, "accept")
	}
}

func TestLoadRepositoryConfigReachedFromASubdirectory(t *testing.T) {
	isolatedHome(t)
	repo := newRepo(t)
	writeFile(t, repoConfig(repo), "interactive:\n  default_action: accept\n")
	sub := filepath.Join(repo, "nested", "deeper")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cfg, err := config.Load("", sub)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.Interactive.DefaultAction != "accept" {
		t.Errorf("default_action = %q, want %q", cfg.Interactive.DefaultAction, "accept")
	}
}

func TestLoadWorktreeStyleGitFileCountsAsARoot(t *testing.T) {
	isolatedHome(t)
	root := t.TempDir()
	// In a linked worktree .git is a file, not a directory.
	writeFile(t, filepath.Join(root, ".git"), "gitdir: /somewhere/else/.git\n")
	writeFile(t, repoConfig(root), "interactive:\n  default_action: accept\n")

	cfg, err := config.Load("", root)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.Interactive.DefaultAction != "accept" {
		t.Errorf("default_action = %q, want %q", cfg.Interactive.DefaultAction, "accept")
	}
}

func TestLoadMalformedRepositoryConfigIsNotSkipped(t *testing.T) {
	isolatedHome(t)
	repo := newRepo(t)
	writeFile(t, repoConfig(repo), "interactive: [broken\n")

	if _, err := config.Load("", repo); !errors.Is(err, config.ErrConfigMalformed) {
		t.Errorf("Load() error = %v, want ErrConfigMalformed", err)
	}
}
