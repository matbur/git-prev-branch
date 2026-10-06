// Command git-prev-branch prints the branch n steps back in the Git
// branch-switch history.
//
// Stream discipline is the whole point of the tool: stdout carries the branch
// name and nothing else, so $(git-prev-branch) and pipes always capture a
// clean value, while the confirmation prompt, diagnostics and errors go to
// stderr. A non-zero exit therefore always means stdout is empty.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/matbur/git-prev-branch/config"
	"github.com/matbur/git-prev-branch/gitprevbranch"
	"golang.org/x/term"
)

const (
	exitSuccess = 0 // success: the branch name, and only it, on stdout
	exitError   = 1 // error: nothing on stdout
	exitAbort   = 2 // aborted at the confirmation prompt: nothing on stdout
)

const usageText = `git-prev-branch prints the branch n steps back in the Git branch-switch history.

Usage:
  git-prev-branch [flags] [n]

Arguments:
  n             how many steps back through the branch-switch history
                (default 1; 0 is the current branch)

Flags:
  -c, --config PATH   path to a custom configuration file
  -p, --path PATH     path to the Git repository to analyze
                      (default: the current working directory)
  -y, --yes           accept the detected branch without prompting
  -h, --help          show this help

stdout receives only the branch name; prompts and diagnostics go to stderr.
Exit codes: 0 success, 1 error, 2 aborted at the confirmation prompt.
`

// stepPattern is the accepted shape of the positional argument: decimal digits
// only, so "-1" and "1.5" are rejected by the parser rather than by git.
var stepPattern = regexp.MustCompile(`^\d+$`)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes one invocation and reports its exit code. The streams and the
// argument list are parameters so the behaviour can be tested without
// spawning a process.
func run(args []string, in *os.File, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("git-prev-branch", flag.ContinueOnError)
	flags.SetOutput(io.Discard) // usage and errors are rendered by us, see below
	flags.Usage = func() {}

	var (
		configPath string
		repoPath   string
		yes        bool
	)
	flags.StringVar(&configPath, "config", "", "path to a custom configuration file")
	flags.StringVar(&configPath, "c", "", "shorthand for -config")
	flags.StringVar(&repoPath, "path", "", "path to the Git repository to analyze")
	flags.StringVar(&repoPath, "p", "", "shorthand for -path")
	flags.BoolVar(&yes, "yes", false, "accept the detected branch without prompting")
	flags.BoolVar(&yes, "y", false, "shorthand for -yes")

	positional, err := collectArgs(flags, args)
	if err != nil {
		return flagFailure(err, out, errOut)
	}

	step, code := parseArgs(positional, errOut)
	if code != exitSuccess {
		return code
	}

	cfg, err := config.Load(configPath, repoPath)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return exitError
	}

	branch, err := gitprevbranch.PreviousIn(step, repoPath)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return exitError
	}

	// The prompt is shown exactly when stdin is an interactive terminal and
	// --yes was not passed; every other case prints the branch immediately.
	if !yes && term.IsTerminal(int(in.Fd())) {
		if !confirm(in, errOut, branch, cfg.Interactive.AcceptsByDefault()) {
			return exitAbort
		}
	}

	fmt.Fprintln(out, branch)
	return exitSuccess
}

// collectArgs parses args for flags and positionals, allowing the two to be
// interleaved: Go's flag package stops at the first non-flag token, so the
// remainder of the command line is parsed in further passes. Each pass
// consumes exactly one positional token, which also guarantees termination.
func collectArgs(flags *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	rest := args

	for len(rest) > 0 {
		if err := flags.Parse(rest); err != nil {
			return nil, err
		}
		if flags.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, flags.Arg(0))
		rest = flags.Args()[1:]
	}

	return positional, nil
}

// flagFailure renders a flag parsing error, or the help text for -h, and
// reports the exit code to use.
func flagFailure(err error, out, errOut io.Writer) int {
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(out, usageText)
		return exitSuccess
	}

	// `git-prev-branch -1` is a step spelled the wrong way round, not an
	// undefined flag: say what was meant instead of blaming the flag parser.
	if name, ok := undefinedFlag(err); ok && stepPattern.MatchString(name) {
		fmt.Fprintf(errOut, "error: invalid argument: -%s (the step is positional and must not be negative)\n", name)
		return exitError
	}

	// Unknown or malformed flags are ordinary errors: the flag package would
	// exit 2 on its own, and code 2 is reserved for the prompt abort.
	fmt.Fprintf(errOut, "error: %v\n", err)
	fmt.Fprint(errOut, usageText)
	return exitError
}

// undefinedFlag extracts the flag name from Go's "flag provided but not
// defined" error message.
func undefinedFlag(err error) (string, bool) {
	const prefix = "flag provided but not defined: -"
	msg := err.Error()
	if !strings.HasPrefix(msg, prefix) {
		return "", false
	}
	return strings.TrimPrefix(msg, prefix), true
}

// parseArgs turns the positional arguments into a step index, rejecting
// anything that is not a single non-negative decimal number. It returns the
// exit code to use when the arguments are unusable.
func parseArgs(args []string, errOut io.Writer) (step, code int) {
	switch len(args) {
	case 0:
		return 1, exitSuccess
	case 1:
		n, err := parseStep(args[0])
		if err != nil {
			fmt.Fprintf(errOut, "error: %v\n", err)
			return 0, exitError
		}
		return n, exitSuccess
	default:
		fmt.Fprintf(errOut, "error: unexpected argument: %s\n", args[1])
		return 0, exitError
	}
}

// parseStep parses the positional step argument.
func parseStep(s string) (int, error) {
	if !stepPattern.MatchString(s) {
		return 0, fmt.Errorf("invalid argument: %s", s)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid argument: %s", s)
	}
	return n, nil
}

// confirm shows the confirmation prompt on errOut and reports whether the
// detected branch was accepted. The prompt advertises what an empty answer
// does, so the shipped default (reject) is visible where it matters.
func confirm(in io.Reader, errOut io.Writer, branch string, acceptByDefault bool) bool {
	hint := "[y/N]"
	if acceptByDefault {
		hint = "[Y/n]"
	}
	fmt.Fprintf(errOut, "Use previous branch '%s'? %s ", branch, hint)

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		// No answer at all (Ctrl-D): nothing was echoed for the user to see.
		fmt.Fprintln(errOut, "aborted")
		return false
	}
	// A partial answer read together with EOF still counts as an answer.

	if !answerAccepts(line, acceptByDefault) {
		fmt.Fprintln(errOut, "aborted")
		return false
	}
	return true
}

// answerAccepts interprets a single answer line: y/yes accept, n/no reject,
// and anything else (including an empty line) means the configured default.
func answerAccepts(answer string, acceptByDefault bool) bool {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return acceptByDefault
	}
}
