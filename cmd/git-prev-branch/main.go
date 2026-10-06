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
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"golang.org/x/term"

	"github.com/matbur/git-prev-branch/gitprevbranch"
	"github.com/matbur/git-prev-branch/internal/config"
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
  -d, --debug         print decisions and git commands to stderr
  -h, --help          show this help

stdout receives only the branch name; prompts and diagnostics go to stderr.
Exit codes: 0 success, 1 error, 2 aborted at the confirmation prompt.
`

// cli is the whole command line grammar; kong turns the tags into flags and
// the positional argument, so there is no hand-written argument parsing left.
type cli struct {
	Config string `short:"c" help:"path to a custom configuration file"`
	Path   string `short:"p" help:"path to the Git repository to analyze (default: the current working directory)"`
	Yes    bool   `short:"y" help:"accept the detected branch without prompting"`
	Debug  bool   `short:"d" help:"print decisions and git commands to stderr"`
	Step   int    `          help:"how many steps back through the branch-switch history (0 is the current branch)" arg:"" optional:"" default:"1"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes one invocation and reports its exit code. The streams and the
// argument list are parameters so the behaviour can be tested without
// spawning a process.
func run(args []string, in *os.File, out, errOut io.Writer) int {
	var opts cli

	// kong exits through parser.Exit on -h/--help, and prints its own usage
	// and errors through parser.Stdout/parser.Stderr. Hooking Exit keeps run()
	// in charge of the exit code, and the custom help printer keeps our
	// usageText (and only it) on stdout.
	exitCode := -1
	parser, err := kong.New(&opts,
		kong.Exit(func(code int) { exitCode = code }),
		kong.Help(func(kong.HelpOptions, *kong.Context) error {
			fmt.Fprint(out, usageText)
			return nil
		}),
	)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return exitError
	}
	parser.Stdout = out
	parser.Stderr = errOut

	if _, parseErr := parser.Parse(args); exitCode >= 0 {
		return exitCode
	} else if parseErr != nil {
		fmt.Fprintf(errOut, "error: %v\n", parseErr)
		fmt.Fprint(errOut, usageText)
		return exitError
	}

	// Only reachable as `git-prev-branch -- -1`: kong rejects bare "-1" as an
	// unknown flag, but after "--" it parses as a negative step.
	if opts.Step < 0 {
		fmt.Fprintf(errOut, "error: invalid argument: -%d\n", -opts.Step)
		return exitError
	}

	// With --debug the packages log the decisions whose inputs live behind
	// their API: which config file won, and which git command was run. The
	// hook writes to errOut, so stdout keeps carrying only the branch name.
	dbg := func(format string, args ...any) {
		if opts.Debug {
			fmt.Fprintf(errOut, "debug: "+format+"\n", args...)
		}
	}
	gitprevbranch.Debugf = dbg
	config.Debugf = dbg
	defer func() {
		gitprevbranch.Debugf = nil
		config.Debugf = nil
	}()

	cfg, err := config.Load(opts.Config, opts.Path)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return exitError
	}

	dbg("resolving branch %d step(s) back", opts.Step)
	branch, err := gitprevbranch.PreviousIn(opts.Step, opts.Path)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return exitError
	}
	dbg("previous branch: %s", branch)

	// The prompt is shown exactly when stdin is an interactive terminal and
	// --yes was not passed; every other case prints the branch immediately.
	if !opts.Yes && term.IsTerminal(int(in.Fd())) {
		dbg("showing confirmation prompt (default action: %s)", cfg.Interactive.DefaultAction)
		if !confirm(in, errOut, branch, cfg.Interactive.AcceptsByDefault()) {
			return exitAbort
		}
	} else {
		dbg("skipping confirmation prompt (default action: %s)", cfg.Interactive.DefaultAction)
	}

	fmt.Fprintln(out, branch)
	return exitSuccess
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
