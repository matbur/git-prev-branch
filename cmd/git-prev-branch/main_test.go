package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Unit tests for run() and the prompt helpers
// ---------------------------------------------------------------------------

// TestRunArgumentParsing covers everything run() rejects (or accepts) before
// any Git or config work happens, which is exactly where the kong grammar is
// exercised. It needs no repository, so it runs against the in-process run().
func TestRunArgumentParsing(t *testing.T) {
	cases := []struct {
		name          string
		args          []string
		wantCode      int
		wantStderr    string
		wantStdoutHas string
	}{
		{
			name:       "not a number",
			args:       []string{"abc"},
			wantCode:   exitError,
			wantStderr: `expected a valid 64 bit int but got "abc"`,
		},
		{name: "step written as a flag", args: []string{"-1"}, wantCode: exitError, wantStderr: "unknown flag -1"},
		{name: "unknown flag", args: []string{"--bogus"}, wantCode: exitError, wantStderr: "unknown flag --bogus"},
		{name: "two positionals", args: []string{"1", "2"}, wantCode: exitError, wantStderr: "unexpected argument 2"},
		{
			name:       "flag without a value",
			args:       []string{"-p"},
			wantCode:   exitError,
			wantStderr: `expected string value but got "EOL"`,
		},
		{
			name:       "negative step after --",
			args:       []string{"--", "-1"},
			wantCode:   exitError,
			wantStderr: "invalid argument: -1",
		},
		{name: "help", args: []string{"-h"}, wantCode: exitSuccess, wantStdoutHas: "Usage:"},
		{name: "long help", args: []string{"--help"}, wantCode: exitSuccess, wantStdoutHas: "Usage:"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := run(tc.args, os.Stdin, &out, &errOut)

			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tc.wantCode, errOut.String())
			}
			if tc.wantStdoutHas != "" && !strings.Contains(out.String(), tc.wantStdoutHas) {
				t.Errorf("stdout = %q, want it to contain %q", out.String(), tc.wantStdoutHas)
			}
			if tc.wantStderr != "" && !strings.Contains(errOut.String(), tc.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", errOut.String(), tc.wantStderr)
			}
			if tc.wantCode == exitSuccess && errOut.String() != "" {
				t.Errorf("stderr = %q, want it empty on success", errOut.String())
			}
		})
	}
}

// TestRunDebugPrintsDiagnosticsToStderr drives run() against a directory that
// is not a repository: the run fails, but --debug must still have printed the
// config decision and the git invocation it attempted, so the user can see
// what the tool did -- before the error, not instead of it.
func TestRunDebugPrintsDiagnosticsToStderr(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	var out, errOut bytes.Buffer
	code := run([]string{"-d", "-p", t.TempDir()}, os.Stdin, &out, &errOut)
	if code != exitError {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitError, errOut.String())
	}

	stderr := errOut.String()
	for _, want := range []string{
		"debug: running: git -C ",
		"debug: no config file found, using built-in defaults",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want it to contain %q", stderr, want)
		}
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want it empty", out.String())
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
		{
			name:            "no beats an accept default",
			answer:          "n\n",
			acceptByDefault: true,
			want:            false,
			wantHint:        "[Y/n]",
			wantAborted:     true,
		},
		{
			name:        "empty answer takes the reject default",
			answer:      "\n",
			want:        false,
			wantHint:    "[y/N]",
			wantAborted: true,
		},
		{
			name:            "empty answer takes the accept default",
			answer:          "\n",
			acceptByDefault: true,
			want:            true,
			wantHint:        "[Y/n]",
		},
		{
			name:        "garbage takes the configured default",
			answer:      "huh\n",
			want:        false,
			wantHint:    "[y/N]",
			wantAborted: true,
		},
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
