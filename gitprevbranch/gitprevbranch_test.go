package gitprevbranch_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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
	require.NoError(t, os.WriteFile(name, []byte(content), 0o644), "write file")
	git(t, dir, "add", "file.txt")
	git(t, dir, "commit", "-q", "-m", message)
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()

	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	require.NoError(t, err, "git %s:\n%s", strings.Join(args, " "), out)
}

func TestPreviousInZeroIsTheCurrentBranch(t *testing.T) {
	repo := newRepo(t)

	got, err := gitprevbranch.PreviousIn(0, repo)
	require.NoError(t, err, "PreviousIn(0)")
	require.Equal(t, "main", got)
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
		require.NoError(t, err, "PreviousIn(%d)", step)
		require.Equal(t, wantBranch, got, "PreviousIn(%d)", step)
	}
}

func TestPreviousInBeyondTheHistoryReportsNoPreviousBranch(t *testing.T) {
	repo := newRepo(t)
	git(t, repo, "checkout", "-q", "-b", "feat")
	commit(t, repo, "second")
	git(t, repo, "checkout", "-q", "main")
	git(t, repo, "checkout", "-q", "feat")

	// The reflog holds three switches (main, feat, main); there is no fourth.
	_, err := gitprevbranch.PreviousIn(4, repo)
	require.ErrorIs(t, err, gitprevbranch.ErrNoPreviousBranch, "PreviousIn(4)")
	_, err = gitprevbranch.PreviousIn(100, repo)
	require.ErrorIs(t, err, gitprevbranch.ErrNoPreviousBranch, "PreviousIn(100)")
}

func TestPreviousInOnDetachedHEAD(t *testing.T) {
	repo := newRepo(t)
	git(t, repo, "checkout", "-q", "-b", "feat")
	commit(t, repo, "second")
	git(t, repo, "checkout", "-q", "--detach", "HEAD~1")

	// There is no current branch to report...
	_, err := gitprevbranch.PreviousIn(0, repo)
	require.ErrorIs(t, err, gitprevbranch.ErrNoPreviousBranch, "PreviousIn(0)")

	// ...but the branch switches before the detach are still walkable, with
	// the detached hop skipped exactly as git itself skips it: we left feat
	// for the detached HEAD, and feat for main before that.
	for step, want := range map[int]string{1: "feat", 2: "main"} {
		var got string
		got, err = gitprevbranch.PreviousIn(step, repo)
		require.NoError(t, err, "PreviousIn(%d)", step)
		require.Equal(t, want, got, "PreviousIn(%d)", step)
	}
}

func TestPreviousInRepositoryWithoutCommits(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")

	for _, step := range []int{0, 1} {
		_, err := gitprevbranch.PreviousIn(step, dir)
		require.ErrorIs(t, err, gitprevbranch.ErrNoPreviousBranch, "PreviousIn(%d)", step)
	}
}

func TestPreviousInRejectsNegativeIndex(t *testing.T) {
	repo := newRepo(t)

	_, err := gitprevbranch.PreviousIn(-1, repo)
	require.ErrorIs(t, err, gitprevbranch.ErrInvalidIndex, "PreviousIn(-1)")
}

func TestPreviousInOutsideARepository(t *testing.T) {
	notARepo := t.TempDir()

	_, err := gitprevbranch.PreviousIn(1, notARepo)
	require.ErrorIs(t, err, gitprevbranch.ErrNotGitRepo, "PreviousIn(1)")
	_, err = gitprevbranch.PreviousIn(0, notARepo)
	require.ErrorIs(t, err, gitprevbranch.ErrNotGitRepo, "PreviousIn(0)")
}

func TestPreviousInMissingDirectoryIsAGitFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	_, err := gitprevbranch.PreviousIn(1, missing)
	require.ErrorIs(t, err, gitprevbranch.ErrGitCommand, "PreviousIn(1)")
}

func TestPreviousInFromASubdirectoryOfTheWorkTree(t *testing.T) {
	repo := newRepo(t)
	git(t, repo, "checkout", "-q", "-b", "feat")
	commit(t, repo, "second")
	git(t, repo, "checkout", "-q", "main")
	sub := filepath.Join(repo, "nested", "deeper")
	require.NoError(t, os.MkdirAll(sub, 0o755), "mkdir")

	got, err := gitprevbranch.PreviousIn(1, sub)
	require.NoError(t, err, "PreviousIn(1) from a subdirectory")
	require.Equal(t, "feat", got)
}

func TestDebugfReportsTheGitCommands(t *testing.T) {
	repo := newRepo(t)

	var got []string
	gitprevbranch.Debugf = func(format string, args ...any) {
		got = append(got, fmt.Sprintf(format, args...))
	}
	defer func() { gitprevbranch.Debugf = nil }()

	_, err := gitprevbranch.PreviousIn(0, repo)
	require.NoError(t, err, "PreviousIn(0)")

	joined := strings.Join(got, "\n")
	require.Contains(t, joined, "running: git -C ", "debug lines")
	require.Contains(t, joined, "rev-parse", "debug lines")
	require.True(t, strings.HasSuffix(joined, " ok"), "debug lines = %q, want a success line", joined)
}

func TestPreviousUsesTheCurrentWorkingDirectory(t *testing.T) {
	repo := newRepo(t)
	git(t, repo, "checkout", "-q", "-b", "feat")
	commit(t, repo, "second")
	git(t, repo, "checkout", "-q", "main")
	t.Chdir(repo)

	got, err := gitprevbranch.Previous(1)
	require.NoError(t, err, "Previous(1)")
	require.Equal(t, "feat", got)
}
