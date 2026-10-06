package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Unit tests for the helpers
// ---------------------------------------------------------------------------

func TestParseStep(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{in: "0", want: 0},
		{in: "1", want: 1},
		{in: "42", want: 42},
		{in: "0007", want: 7},
		{in: "", wantErr: true},
		{in: "abc", wantErr: true},
		{in: "-1", wantErr: true},
		{in: "+1", wantErr: true},
		{in: "1.5", wantErr: true},
		{in: " 1", wantErr: true},
		{in: "99999999999999999999999999", wantErr: true},
	}

	for _, tc := range cases {
		got, err := parseStep(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseStep(%q) = %d, want an error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseStep(%q) = %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseStep(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseArgs(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantStep    int
		wantErrText string
	}{
		{name: "no argument defaults to one step back", args: nil, wantStep: 1},
		{name: "explicit zero", args: []string{"0"}, wantStep: 0},
		{name: "explicit step", args: []string{"7"}, wantStep: 7},
		{name: "not a number", args: []string{"abc"}, wantErrText: "invalid argument: abc"},
		{name: "negative step", args: []string{"-1"}, wantErrText: "invalid argument: -1"},
		{name: "two positionals", args: []string{"1", "2"}, wantErrText: "unexpected argument: 2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var errOut bytes.Buffer
			step, code := parseArgs(tc.args, &errOut)

			if tc.wantErrText == "" {
				if code != exitSuccess {
					t.Fatalf("code = %d, want %d (stderr: %s)", code, exitSuccess, errOut.String())
				}
				if step != tc.wantStep {
					t.Errorf("step = %d, want %d", step, tc.wantStep)
				}
				return
			}

			if code != exitError {
				t.Errorf("code = %d, want %d", code, exitError)
			}
			if !strings.Contains(errOut.String(), tc.wantErrText) {
				t.Errorf("stderr = %q, want it to contain %q", errOut.String(), tc.wantErrText)
			}
		})
	}
}

func TestAnswerAccepts(t *testing.T) {
	cases := []struct {
		answer          string
		acceptByDefault bool
		want            bool
	}{
		{answer: "y", want: true},
		{answer: "Y", want: true},
		{answer: "yes", want: true},
		{answer: "  YES  ", want: true},
		{answer: "n", acceptByDefault: true, want: false},
		{answer: "no", acceptByDefault: true, want: false},
		{answer: "", want: false},
		{answer: "", acceptByDefault: true, want: true},
		{answer: "anything else", acceptByDefault: true, want: true},
		{answer: "anything else", want: false},
	}

	for _, tc := range cases {
		if got := answerAccepts(tc.answer, tc.acceptByDefault); got != tc.want {
			t.Errorf("answerAccepts(%q, %v) = %v, want %v", tc.answer, tc.acceptByDefault, got, tc.want)
		}
	}
}

func TestConfirm(t *testing.T) {
	cases := []struct {
		name            string
		answer          string
		acceptByDefault bool
		want            bool
		wantHint        string
		wantAborted     bool
	}{
		{name: "yes", answer: "y\n", want: true, wantHint: "[y/N]"},
		{name: "no beats an accept default", answer: "n\n", acceptByDefault: true, want: false, wantHint: "[Y/n]", wantAborted: true},
		{name: "empty answer takes the reject default", answer: "\n", want: false, wantHint: "[y/N]", wantAborted: true},
		{name: "empty answer takes the accept default", answer: "\n", acceptByDefault: true, want: true, wantHint: "[Y/n]"},
		{name: "garbage takes the configured default", answer: "huh\n", want: false, wantHint: "[y/N]", wantAborted: true},
		{name: "eof without an answer", answer: "", want: false, wantHint: "[y/N]", wantAborted: true},
		{name: "eof after an answer", answer: "y", want: true, wantHint: "[y/N]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var errOut bytes.Buffer
			got := confirm(strings.NewReader(tc.answer), &errOut, "main", tc.acceptByDefault)

			if got != tc.want {
				t.Errorf("confirm() = %v, want %v", got, tc.want)
			}
			stderr := errOut.String()
			if !strings.Contains(stderr, "Use previous branch 'main'?") {
				t.Errorf("stderr = %q, want it to contain the prompt", stderr)
			}
			if !strings.Contains(stderr, tc.wantHint) {
				t.Errorf("stderr = %q, want it to contain the hint %q", stderr, tc.wantHint)
			}
			if aborted := strings.Contains(stderr, "aborted"); aborted != tc.wantAborted {
				t.Errorf("stderr = %q, aborted reported = %v, want %v", stderr, aborted, tc.wantAborted)
			}
		})
	}
}

func TestUndefinedFlag(t *testing.T) {
	if name, ok := undefinedFlag(fmt.Errorf("flag provided but not defined: -bogus")); !ok || name != "bogus" {
		t.Errorf("undefinedFlag() = %q, %v, want %q, true", name, ok, "bogus")
	}
	if _, ok := undefinedFlag(fmt.Errorf("flag needs an argument: -p")); ok {
		t.Error("undefinedFlag() reported success for a missing-argument error")
	}
}

// ---------------------------------------------------------------------------
// End-to-end tests against a real binary and a real repository
// ---------------------------------------------------------------------------

var binaryPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "git-prev-branch-e2e")
	if err != nil {
		fmt.Fprintf(os.Stderr, "temp dir: %v\n", err)
		os.Exit(1)
	}

	name := "git-prev-branch"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binaryPath = filepath.Join(tmp, name)

	build := exec.Command("go", "build", "-o", binaryPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build: %v\n%s", err, out)
		_ = os.RemoveAll(tmp)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

// e2eRepo builds a repository whose branch-switch history reads
// main -> feat -> main -> other -> feat, so from the current branch feat:
// 1 step back is other, 2 steps back is main.
func e2eRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
	} {
		gitIn(t, dir, args...)
	}
	writeAndCommit(t, dir, "first")
	gitIn(t, dir, "checkout", "-q", "-b", "feat")
	writeAndCommit(t, dir, "second")
	gitIn(t, dir, "checkout", "-q", "main")
	gitIn(t, dir, "checkout", "-q", "-b", "other")
	gitIn(t, dir, "checkout", "-q", "feat")
	return dir
}

func writeAndCommit(t *testing.T, dir, message string) {
	t.Helper()

	name := filepath.Join(dir, "file.txt")
	content, err := os.ReadFile(name)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read %s: %v", name, err)
	}
	if err := os.WriteFile(name, append(content, []byte(message+"\n")...), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	gitIn(t, dir, "add", "file.txt")
	gitIn(t, dir, "commit", "-q", "-m", message)
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

type invocation struct {
	dir      string
	stdin    string
	hasStdin bool
	args     []string
}

func (inv invocation) run(t *testing.T) (stdout, stderr string, code int) {
	t.Helper()

	cmd := exec.Command(binaryPath, inv.args...)
	cmd.Dir = inv.dir

	// An isolated HOME keeps a real ~/.config/git-prev-branch/config.yaml out
	// of the picture; git only needs the local config of the test repository.
	home := t.TempDir()
	cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home)

	if inv.hasStdin {
		cmd.Stdin = strings.NewReader(inv.stdin)
	}
	// With cmd.Stdin left nil the child gets /dev/null, which is the
	// "< /dev/null means no prompt" case from the README.

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()
	switch {
	case err == nil:
		return out.String(), errOut.String(), 0
	case isExitError(err):
		return out.String(), errOut.String(), exitCode(err)
	default:
		t.Fatalf("run %v: %v\nstdout: %s\nstderr: %s", inv.args, err, out.String(), errOut.String())
		return "", "", 0
	}
}

func isExitError(err error) bool {
	_, ok := err.(*exec.ExitError)
	return ok
}

func exitCode(err error) int {
	return err.(*exec.ExitError).ExitCode()
}

func TestEndToEndSuccess(t *testing.T) {
	repo := e2eRepo(t)

	cases := []struct {
		name string
		inv  invocation
		want string
	}{
		{
			name: "default step, no -y, stdin is not a terminal",
			inv:  invocation{dir: repo, args: []string{}},
			want: "other",
		},
		{
			name: "explicit -y",
			inv:  invocation{dir: repo, args: []string{"-y"}},
			want: "other",
		},
		{
			name: "long --yes flag",
			inv:  invocation{dir: repo, args: []string{"--yes"}},
			want: "other",
		},
		{
			name: "step zero is the current branch",
			inv:  invocation{dir: repo, args: []string{"-y", "0"}},
			want: "feat",
		},
		{
			name: "step two",
			inv:  invocation{dir: repo, args: []string{"-y", "2"}},
			want: "main",
		},
		{
			name: "flag after the positional argument",
			inv:  invocation{dir: repo, args: []string{"2", "-y"}},
			want: "main",
		},
		{
			name: "stdin piped from a command still prints without prompting",
			inv:  invocation{dir: repo, stdin: "y\n", hasStdin: true, args: []string{}},
			want: "other",
		},
		{
			name: "explicit repository path",
			inv:  invocation{dir: t.TempDir(), args: []string{"-y", "-p", repo}},
			want: "other",
		},
		{
			name: "long --path flag",
			inv:  invocation{dir: t.TempDir(), args: []string{"-y", "--path", repo}},
			want: "other",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := tc.inv.run(t)

			if code != exitSuccess {
				t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitSuccess, stderr)
			}
			if want := tc.want + "\n"; stdout != want {
				t.Errorf("stdout = %q, want %q", stdout, want)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want it empty on success", stderr)
			}
		})
	}
}

func TestEndToEndErrorsLeaveStdoutEmpty(t *testing.T) {
	repo := e2eRepo(t)
	malformed := filepath.Join(t.TempDir(), "config.yaml")
	writeTestFile(t, malformed, "interactive: [broken\n")
	badValue := filepath.Join(t.TempDir(), "config.yaml")
	writeTestFile(t, badValue, "interactive:\n  default_action: maybe\n")
	missing := filepath.Join(t.TempDir(), "missing.yaml")

	cases := []struct {
		name       string
		inv        invocation
		wantStderr string
	}{
		{
			name:       "step beyond the history",
			inv:        invocation{dir: repo, args: []string{"-y", "9"}},
			wantStderr: "no previous branch",
		},
		{
			name:       "not a repository",
			inv:        invocation{dir: t.TempDir(), args: []string{"-y"}},
			wantStderr: "not a git repository",
		},
		{
			name:       "repository path does not exist",
			inv:        invocation{dir: repo, args: []string{"-y", "-p", filepath.Join(repo, "nope")}},
			wantStderr: "error:",
		},
		{
			name:       "step is not a number",
			inv:        invocation{dir: repo, args: []string{"-y", "abc"}},
			wantStderr: "invalid argument: abc",
		},
		{
			name:       "step written as a flag",
			inv:        invocation{dir: repo, args: []string{"-1"}},
			wantStderr: "invalid argument: -1",
		},
		{
			name:       "unknown flag",
			inv:        invocation{dir: repo, args: []string{"--bogus"}},
			wantStderr: "flag provided but not defined: -bogus",
		},
		{
			name:       "flag without a value",
			inv:        invocation{dir: repo, args: []string{"-y", "-p"}},
			wantStderr: "flag needs an argument: -p",
		},
		{
			name:       "two positional arguments",
			inv:        invocation{dir: repo, args: []string{"-y", "1", "2"}},
			wantStderr: "unexpected argument: 2",
		},
		{
			name:       "config file that does not exist",
			inv:        invocation{dir: repo, args: []string{"-y", "-c", missing}},
			wantStderr: "config file not found",
		},
		{
			name:       "malformed config file",
			inv:        invocation{dir: repo, args: []string{"-y", "-c", malformed}},
			wantStderr: "malformed config file",
		},
		{
			name:       "config file with an unknown default_action",
			inv:        invocation{dir: repo, args: []string{"-y", "-c", badValue}},
			wantStderr: "default_action",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := tc.inv.run(t)

			if code != exitError {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, exitError, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want it empty on failure", stdout)
			}
			if !strings.Contains(stderr, tc.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tc.wantStderr)
			}
		})
	}
}

func TestEndToEndHelp(t *testing.T) {
	stdout, stderr, code := invocation{dir: t.TempDir(), args: []string{"-h"}}.run(t)

	if code != exitSuccess {
		t.Errorf("exit code = %d, want %d", code, exitSuccess)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("stdout = %q, want it to contain the usage text", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want it empty for -h", stderr)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
