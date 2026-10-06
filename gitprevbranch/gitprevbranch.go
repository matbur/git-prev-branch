// Package gitprevbranch resolves branches from Git's branch-switch history.
//
// The package is the library half of the git-prev-branch command. It shells
// out to git rather than talking to the repository format directly, so it
// behaves exactly like the porcelain the user already trusts -- in particular
// like the "@{-n}" revision syntax, which is what the CLI prints.
package gitprevbranch

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Debugf is a diagnostic sink. When set, it receives a line for every git
// command the package runs and its outcome. It exists for --debug style flags
// in command-line tools built on this package; a nil Debugf disables the
// output. Like [fmt.Printf], it is called with a format string and arguments.
//
//nolint:gochecknoglobals // a package-level sink the CLI wires to its --debug hook; that is the point.
var Debugf func(format string, args ...any)

var (
	// ErrNotGitRepo indicates the given directory is not inside a Git work tree.
	ErrNotGitRepo = errors.New("not a git repository")
	// ErrNoPreviousBranch indicates the history is shorter than the requested index.
	ErrNoPreviousBranch = errors.New("no previous branch found")
	// ErrInvalidIndex indicates the requested index is negative.
	ErrInvalidIndex = errors.New("invalid index")
	// ErrGitCommand indicates git itself failed for a reason unrelated to the query.
	ErrGitCommand = errors.New("git command failed")
)

// Previous reports the branch n steps back in the branch-switch history of the
// current working directory. n = 0 is the current branch, n = 1 the branch that
// was checked out before it.
func Previous(n int) (string, error) {
	return PreviousIn(n, ".")
}

// PreviousIn reports the branch n steps back in the branch-switch history of
// the repository containing dir ("" means the current working directory).
//
// n = 0 resolves the branch HEAD points at. Larger n asks git for the
// "@{-n}" revision, so the result is exactly the sequence `git checkout -`
// would walk back through, detached-HEAD hops included: entries git cannot
// resolve to a branch are reported as ErrNoPreviousBranch rather than as a
// raw object name.
func PreviousIn(n int, dir string) (string, error) {
	if n < 0 {
		return "", fmt.Errorf("%w: %d is negative", ErrInvalidIndex, n)
	}

	revision := "HEAD"
	if n > 0 {
		revision = fmt.Sprintf("@{-%d}", n)
	}

	stdout, stderr, err := runGit(dir, "rev-parse", "--abbrev-ref", revision)
	if err != nil {
		return "", classifyError(err, stderr, n)
	}

	branch := strings.TrimSpace(stdout)
	switch branch {
	case "", "HEAD":
		// Empty output means git had nothing to name (a detached hop); "HEAD"
		// is what --abbrev-ref prints for a detached HEAD itself.
		if n == 0 {
			return "", fmt.Errorf("%w: HEAD is detached", ErrNoPreviousBranch)
		}
		return "", fmt.Errorf("%w: index %d", ErrNoPreviousBranch, n)
	default:
		return branch, nil
	}
}

// classifyError maps a failed git invocation onto the package's error values.
func classifyError(runErr error, stderr string, n int) error {
	msg := strings.TrimSpace(stderr)
	switch {
	case strings.Contains(msg, "not a git repository"),
		strings.Contains(msg, "this operation must be run in a work tree"):
		return ErrNotGitRepo
	case strings.Contains(msg, "unknown revision"),
		strings.Contains(msg, "ambiguous argument"):
		return fmt.Errorf("%w: index %d", ErrNoPreviousBranch, n)
	case msg == "":
		return fmt.Errorf("%w: %w", ErrGitCommand, runErr)
	default:
		return fmt.Errorf("%w: %s", ErrGitCommand, firstLine(msg))
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// runGit runs git in dir and returns its separated stdout and stderr.
func runGit(dir string, args ...string) (string, string, error) {
	dir = pathOrCurrent(dir)
	display := fmt.Sprintf("git -C %q %s", dir, strings.Join(args, " "))
	debugf("running: %s", display)

	//nolint:gosec,noctx // dir travels as a single -C argv entry, so no shell is involved, and git runs synchronously to completion before the caller proceeds.
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if err != nil {
		debugf("%s failed: %v", display, err)
	} else {
		debugf("%s ok", display)
	}
	return out.String(), errOut.String(), err
}

func debugf(format string, args ...any) {
	if Debugf != nil {
		Debugf(format, args...)
	}
}

func pathOrCurrent(path string) string {
	if strings.TrimSpace(path) == "" {
		return "."
	}
	return path
}
