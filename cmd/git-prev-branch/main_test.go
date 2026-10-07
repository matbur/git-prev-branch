package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
			name:       "long path flag without a value",
			args:       []string{"--path"},
			wantCode:   exitError,
			wantStderr: `expected string value but got "EOL"`,
		},
		{
			name:       "config flag without a value",
			args:       []string{"-c"},
			wantCode:   exitError,
			wantStderr: `expected string value but got "EOL"`,
		},
		{
			name:       "long config flag without a value",
			args:       []string{"--config"},
			wantCode:   exitError,
			wantStderr: `expected string value but got "EOL"`,
		},
		{
			name:       "help does not mask a following unknown flag",
			args:       []string{"-h", "--bogus"},
			wantCode:   exitError,
			wantStderr: "unknown flag --bogus",
		},
		{
			name:       "negative step after --",
			args:       []string{"--", "-1"},
			wantCode:   exitError,
			wantStderr: "invalid argument: -1",
		},
		{name: "help", args: []string{"-h"}, wantCode: exitSuccess, wantStdoutHas: "Usage:"},
		{name: "long help", args: []string{"--help"}, wantCode: exitSuccess, wantStdoutHas: "Usage:"},
		{name: "version", args: []string{"-v"}, wantCode: exitSuccess, wantStdoutHas: "git-prev-branch "},
		{name: "long version", args: []string{"--version"}, wantCode: exitSuccess, wantStdoutHas: "git-prev-branch "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := run(tc.args, os.Stdin, &out, &errOut)

			require.Equal(t, tc.wantCode, code, "exit code (stdout: %q, stderr: %q)", out.String(), errOut.String())
			if tc.wantStdoutHas != "" {
				require.Contains(t, out.String(), tc.wantStdoutHas, "stdout")
			}
			if tc.wantStderr != "" {
				require.Contains(t, errOut.String(), tc.wantStderr, "stderr")
			}
			if tc.wantCode == exitSuccess {
				require.Empty(t, errOut.String(), "stderr on success")
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
	require.Equal(t, exitError, code, "exit code (stderr: %q)", errOut.String())

	for _, want := range []string{
		"debug: running: git -C ",
		"debug: no config file found, using built-in defaults",
	} {
		require.Contains(t, errOut.String(), want, "stderr")
	}
	require.Empty(t, out.String(), "stdout")
}

func TestResolveVersion(t *testing.T) {
	cases := []struct {
		stamped       string
		moduleVersion string
		want          string
	}{
		{stamped: "v1.2.3", moduleVersion: "v9.9.9", want: "v1.2.3"}, // the stamp wins
		{stamped: "v1.2.3", moduleVersion: "(devel)", want: "v1.2.3"},
		{stamped: "dev", moduleVersion: "v1.0.1", want: "v1.0.1"}, // go install ...@vX.Y.Z
		{
			stamped:       "dev",
			moduleVersion: "v0.0.0-20261007101010-abcdef123456",
			want:          "v0.0.0-20261007101010-abcdef123456", // go install ...@main
		},
		{stamped: "dev", moduleVersion: "(devel)", want: "dev"}, // plain go build/run
		{stamped: "dev", moduleVersion: "", want: "dev"},        // no build info at all
	}

	for _, tc := range cases {
		got := resolveVersion(tc.stamped, tc.moduleVersion)
		require.Equal(t, tc.want, got, "resolveVersion(%q, %q)", tc.stamped, tc.moduleVersion)
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
		got := answerAccepts(tc.answer, tc.acceptByDefault)
		require.Equal(t, tc.want, got, "answerAccepts(%q, %v)", tc.answer, tc.acceptByDefault)
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

			require.Equal(t, tc.want, got, "confirm()")
			stderr := errOut.String()
			require.Contains(t, stderr, "Use previous branch 'main'?", "stderr")
			require.Contains(t, stderr, tc.wantHint, "stderr")
			require.Equal(
				t,
				tc.wantAborted,
				strings.Contains(stderr, "aborted"),
				"aborted reported (stderr: %q)",
				stderr,
			)
		})
	}
}
