package gitprevbranch_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matbur/git-prev-branch/gitprevbranch"
)

// newRepo creates a throwaway repository with one commit on main and returns
// its path. Every test gets its own, so reflog history is fully controlled.
func newRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "Test")
	git(t, dir, "config", "commit.gpgsign", "false")
	commit(t, dir, "first")
	return dir
}

func commit(t *testing.T, dir, message string) {
	t.Helper()

	name := filepath.Join(dir, "file.txt")
	content := message + "\n"
	if data, err := os.ReadFile(name); err == nil {
		content = string(data) + message + "\n"
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	git(t, dir, "add", "file.txt")
	git(t, dir, "commit", "-q", "-m", message)
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestPreviousInZeroIsTheCurrentBranch(t *testing.T) {
	repo := newRepo(t)

	got, err := gitprevbranch.PreviousIn(0, repo)
	if err != nil {
		t.Fatalf("PreviousIn(0) = %v", err)
	}
	if got != "main" {
		t.Errorf("PreviousIn(0) = %q, want %q", got, "main")
	}
}

func TestPreviousInWalksBackThroughTheSwitchHistory(t *testing.T) {
	repo := newRepo(t)

	// main -> feat -> main -> other -> feat, so walking back from feat must
	// revisit main: the history is a sequence of switches, not a set of
	// distinct branches.
	git(t, repo, "checkout", "-q", "-b", "feat")
	commit(t, repo, "second")
	git(t, repo, "checkout", "-q", "main")
	git(t, repo, "checkout", "-q", "-b", "other")
	git(t, repo, "checkout", "-q", "feat")

	want := map[int]string{0: "feat", 1: "other", 2: "main", 3: "feat", 4: "main"}
	for step, wantBranch := range want {
		got, err := gitprevbranch.PreviousIn(step, repo)
		if err != nil {
			t.Fatalf("PreviousIn(%d) = %v", step, err)
		}
		if got != wantBranch {
			t.Errorf("PreviousIn(%d) = %q, want %q", step, got, wantBranch)
		}
	}
}

func TestPreviousInBeyondTheHistoryReportsNoPreviousBranch(t *testing.T) {
	repo := newRepo(t)
	git(t, repo, "checkout", "-q", "-b", "feat")
	commit(t, repo, "second")
	git(t, repo, "checkout", "-q", "main")
	git(t, repo, "checkout", "-q", "feat")

	// The reflog holds three switches (main, feat, main); there is no fourth.
	if _, err := gitprevbranch.PreviousIn(4, repo); !errors.Is(err, gitprevbranch.ErrNoPreviousBranch) {
		t.Errorf("PreviousIn(4) error = %v, want ErrNoPreviousBranch", err)
	}
	if _, err := gitprevbranch.PreviousIn(100, repo); !errors.Is(err, gitprevbranch.ErrNoPreviousBranch) {
		t.Errorf("PreviousIn(100) error = %v, want ErrNoPreviousBranch", err)
	}
}

func TestPreviousInOnDetachedHEAD(t *testing.T) {
	repo := newRepo(t)
	git(t, repo, "checkout", "-q", "-b", "feat")
	commit(t, repo, "second")
	git(t, repo, "checkout", "-q", "--detach", "HEAD~1")

	// There is no current branch to report...
	if _, err := gitprevbranch.PreviousIn(0, repo); !errors.Is(err, gitprevbranch.ErrNoPreviousBranch) {
		t.Errorf("PreviousIn(0) error = %v, want ErrNoPreviousBranch", err)
	}
	// ...but the branch switches before the detach are still walkable, with
	// the detached hop skipped exactly as git itself skips it: we left feat
	// for the detached HEAD, and feat for main before that.
	for step, want := range map[int]string{1: "feat", 2: "main"} {
		got, err := gitprevbranch.PreviousIn(step, repo)
		if err != nil {
			t.Fatalf("PreviousIn(%d) = %v", step, err)
		}
		if got != want {
			t.Errorf("PreviousIn(%d) = %q, want %q", step, got, want)
		}
	}
}

func TestPreviousInRepositoryWithoutCommits(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")

	for _, step := range []int{0, 1} {
		if _, err := gitprevbranch.PreviousIn(step, dir); !errors.Is(err, gitprevbranch.ErrNoPreviousBranch) {
			t.Errorf("PreviousIn(%d) error = %v, want ErrNoPreviousBranch", step, err)
		}
	}
}

func TestPreviousInRejectsNegativeIndex(t *testing.T) {
	repo := newRepo(t)

	if _, err := gitprevbranch.PreviousIn(-1, repo); !errors.Is(err, gitprevbranch.ErrInvalidIndex) {
		t.Errorf("PreviousIn(-1) error = %v, want ErrInvalidIndex", err)
	}
}

func TestPreviousInOutsideARepository(t *testing.T) {
	notARepo := t.TempDir()

	if _, err := gitprevbranch.PreviousIn(1, notARepo); !errors.Is(err, gitprevbranch.ErrNotGitRepo) {
		t.Errorf("PreviousIn(1) error = %v, want ErrNotGitRepo", err)
	}
	if _, err := gitprevbranch.PreviousIn(0, notARepo); !errors.Is(err, gitprevbranch.ErrNotGitRepo) {
		t.Errorf("PreviousIn(0) error = %v, want ErrNotGitRepo", err)
	}
}

func TestPreviousInMissingDirectoryIsAGitFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	if _, err := gitprevbranch.PreviousIn(1, missing); !errors.Is(err, gitprevbranch.ErrGitCommand) {
		t.Errorf("PreviousIn(1) error = %v, want ErrGitCommand", err)
	}
}

func TestPreviousInFromASubdirectoryOfTheWorkTree(t *testing.T) {
	repo := newRepo(t)
	git(t, repo, "checkout", "-q", "-b", "feat")
	commit(t, repo, "second")
	git(t, repo, "checkout", "-q", "main")
	sub := filepath.Join(repo, "nested", "deeper")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got, err := gitprevbranch.PreviousIn(1, sub)
	if err != nil {
		t.Fatalf("PreviousIn(1) from a subdirectory = %v", err)
	}
	if got != "feat" {
		t.Errorf("PreviousIn(1) from a subdirectory = %q, want %q", got, "feat")
	}
}

func TestPreviousUsesTheCurrentWorkingDirectory(t *testing.T) {
	repo := newRepo(t)
	git(t, repo, "checkout", "-q", "-b", "feat")
	commit(t, repo, "second")
	git(t, repo, "checkout", "-q", "main")
	t.Chdir(repo)

	got, err := gitprevbranch.Previous(1)
	if err != nil {
		t.Fatalf("Previous(1) = %v", err)
	}
	if got != "feat" {
		t.Errorf("Previous(1) = %q, want %q", got, "feat")
	}
}
