package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755), "mkdir %s", filepath.Dir(path))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644), "write %s", path)
}

func userConfig(home string) string {
	return filepath.Join(home, ".config", "git-prev-branch", "config.yaml")
}

// newRepo makes dir look like a repository root to findRepoRoot, which only
// ever stats .git -- no git invocation is involved.
func newRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755), "mkdir .git")
	return dir
}

func repoConfig(root string) string {
	return filepath.Join(root, ".config", "git-prev-branch", "config.yaml")
}

func TestLoadUsesBuiltInDefaultsWhenNothingExists(t *testing.T) {
	isolatedHome(t)

	cfg, err := config.Load("", t.TempDir())
	require.NoError(t, err, "Load()")
	require.Equal(t, "reject", cfg.Interactive.DefaultAction)
	require.False(t, cfg.Interactive.AcceptsByDefault(), "AcceptsByDefault() for the shipped default")
}

func TestLoadExplicitPathThatDoesNotExistIsAnError(t *testing.T) {
	isolatedHome(t)

	missing := filepath.Join(t.TempDir(), "nope.yaml")
	_, err := config.Load(missing, "")
	require.ErrorIs(t, err, config.ErrConfigNotFound, "Load(%q)", missing)
}

func TestLoadMalformedFileIsAnError(t *testing.T) {
	isolatedHome(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "interactive: [unclosed\n")

	_, err := config.Load(path, "")
	require.ErrorIs(t, err, config.ErrConfigMalformed, "Load()")
}

func TestLoadUnknownDefaultActionIsAnError(t *testing.T) {
	isolatedHome(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "interactive:\n  default_action: maybe\n")

	_, err := config.Load(path, "")
	require.ErrorIs(t, err, config.ErrConfigMalformed, "Load()")
}

func TestLoadNormalizesDefaultAction(t *testing.T) {
	isolatedHome(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "interactive:\n  default_action: \"  AcCePt \"\n")

	cfg, err := config.Load(path, "")
	require.NoError(t, err, "Load()")
	require.Equal(t, "accept", cfg.Interactive.DefaultAction)
	require.True(t, cfg.Interactive.AcceptsByDefault(), "AcceptsByDefault()")
}

func TestLoadEmptyDefaultActionKeepsTheRejectDefault(t *testing.T) {
	isolatedHome(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "interactive:\n  default_action: \"\"\n")

	cfg, err := config.Load(path, "")
	require.NoError(t, err, "Load()")
	require.Equal(t, "reject", cfg.Interactive.DefaultAction)
}

func TestLoadToleratesUnknownKeys(t *testing.T) {
	isolatedHome(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, "future_section:\n  whatever: true\ninteractive:\n  default_action: accept\n")

	cfg, err := config.Load(path, "")
	require.NoError(t, err, "Load()")
	require.Equal(t, "accept", cfg.Interactive.DefaultAction)
}

func TestLoadDebugfReportsTheChosenLocation(t *testing.T) {
	home := isolatedHome(t)
	repo := newRepo(t)
	writeFile(t, repoConfig(repo), "interactive:\n  default_action: accept\n")
	writeFile(t, userConfig(home), "interactive:\n  default_action: reject\n")

	var got []string
	config.Debugf = func(format string, args ...any) {
		got = append(got, fmt.Sprintf(format, args...))
	}
	defer func() { config.Debugf = nil }()

	_, err := config.Load("", repo)
	require.NoError(t, err, "Load()")

	joined := strings.Join(got, "\n")
	require.Contains(t, joined, "checking config file "+repoConfig(repo), "debug lines")
	require.Contains(t, joined, "using config file "+repoConfig(repo), "debug lines")
}

func TestLoadDebugfReportsDefaultsWhenNothingExists(t *testing.T) {
	isolatedHome(t)

	var got []string
	config.Debugf = func(format string, args ...any) {
		got = append(got, fmt.Sprintf(format, args...))
	}
	defer func() { config.Debugf = nil }()

	_, err := config.Load("", t.TempDir())
	require.NoError(t, err, "Load()")

	joined := strings.Join(got, "\n")
	require.Contains(t, joined, "no config file found, using built-in defaults", "debug lines")
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
			require.NoError(t, err, "Load()")
			require.Equal(t, tc.want, cfg.Interactive.DefaultAction)
		})
	}

	// Without the repository file the user-level one is used...
	require.NoError(t, os.Remove(repoConfig(repo)), "remove repo config")
	cfg, err := config.Load("", repo)
	require.NoError(t, err, "Load()")
	require.Equal(t, "reject", cfg.Interactive.DefaultAction)

	// ...and outside any repository the user-level one is still used.
	writeFile(t, userConfig(home), "interactive:\n  default_action: accept\n")
	cfg, err = config.Load("", t.TempDir())
	require.NoError(t, err, "Load() outside a repository")
	require.Equal(t, "accept", cfg.Interactive.DefaultAction)
}

func TestLoadRepositoryConfigReachedFromASubdirectory(t *testing.T) {
	isolatedHome(t)
	repo := newRepo(t)
	writeFile(t, repoConfig(repo), "interactive:\n  default_action: accept\n")
	sub := filepath.Join(repo, "nested", "deeper")
	require.NoError(t, os.MkdirAll(sub, 0o755), "mkdir")

	cfg, err := config.Load("", sub)
	require.NoError(t, err, "Load()")
	require.Equal(t, "accept", cfg.Interactive.DefaultAction)
}

func TestLoadWorktreeStyleGitFileCountsAsARoot(t *testing.T) {
	isolatedHome(t)
	root := t.TempDir()
	// In a linked worktree .git is a file, not a directory.
	writeFile(t, filepath.Join(root, ".git"), "gitdir: /somewhere/else/.git\n")
	writeFile(t, repoConfig(root), "interactive:\n  default_action: accept\n")

	cfg, err := config.Load("", root)
	require.NoError(t, err, "Load()")
	require.Equal(t, "accept", cfg.Interactive.DefaultAction)
}

func TestLoadMalformedRepositoryConfigIsNotSkipped(t *testing.T) {
	isolatedHome(t)
	repo := newRepo(t)
	writeFile(t, repoConfig(repo), "interactive: [broken\n")

	_, err := config.Load("", repo)
	require.ErrorIs(t, err, config.ErrConfigMalformed, "Load()")
}
