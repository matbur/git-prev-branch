// Package config provides configuration handling for git-prev-branch.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// ErrConfigNotFound indicates the specified config file was not found.
	ErrConfigNotFound = errors.New("config file not found")
	// ErrConfigMalformed indicates the config file is malformed.
	ErrConfigMalformed = errors.New("malformed config file")
)

// Config represents the application configuration.
type Config struct {
	Interactive Interactive `yaml:"interactive"`
}

// Interactive contains interactive prompt configuration.
type Interactive struct {
	// DefaultAction controls default response to confirmation prompt.
	// Values: "accept" or "reject" (default is "reject")
	DefaultAction string `yaml:"default_action"`
}

// AcceptsByDefault reports whether an empty answer at the confirmation prompt
// accepts the detected branch. It is false unless default_action says so.
func (i Interactive) AcceptsByDefault() bool {
	return i.DefaultAction == "accept"
}

// Debugf is a diagnostic sink. When set, it receives a line for every config
// file location considered and for the one finally used. It exists for
// --debug style flags in command-line tools built on this package; a nil
// Debugf disables the output. Like [fmt.Printf], it is called with a format
// string and arguments.
//
//nolint:gochecknoglobals // a package-level sink the CLI wires to its --debug hook; that is the point.
var Debugf func(format string, args ...any)

// defaultReject is the interactive.default_action used when no config file
// says otherwise. The only accepted companion value is "accept".
const defaultReject = "reject"

func debugf(format string, args ...any) {
	if Debugf != nil {
		Debugf(format, args...)
	}
}

// Load loads configuration from the appropriate location.
// Precedence: --config <path> (if specified), <repoRoot>/.config/git-prev-branch/config.yaml,
// ~/.config/git-prev-branch/config.yaml. The built-in defaults are used if no config file exists.
// If explicitPath is provided and doesn't exist, returns ErrConfigNotFound.
// If a config file exists but is malformed, returns ErrConfigMalformed.
func Load(explicitPath string, repoPath string) (*Config, error) {
	cfg := defaultConfig()

	// 1. --config <path> wins over everything else, and a missing file there is
	// an error rather than a silent fall-through: the caller asked for it.
	if path := strings.TrimSpace(explicitPath); path != "" {
		if err := loadFromFile(path, cfg); err != nil {
			return nil, err
		}
		debugf("using config file %s", path)
		return cfg, nil
	}

	// 2./3. Otherwise the first default location that exists wins.
	for _, path := range defaultPaths(repoPath) {
		debugf("checking config file %s", path)
		err := loadFromFile(path, cfg)
		switch {
		case err == nil:
			debugf("using config file %s", path)
			return cfg, nil
		case errors.Is(err, ErrConfigNotFound):
			continue // this location simply does not exist
		default:
			return nil, err // malformed or unreadable: never silently ignored
		}
	}

	// No config file found, use the built-in defaults.
	debugf("no config file found, using built-in defaults")
	return cfg, nil
}

// defaultPaths returns the default config file locations in precedence order:
// the repository-level file first, then the user-level one. Locations that
// cannot be resolved (no git repository, no home directory) are left out.
func defaultPaths(repoPath string) []string {
	var paths []string

	if root, err := findRepoRoot(repoPath); err == nil {
		paths = append(paths, filepath.Join(root, ".config", "git-prev-branch", "config.yaml"))
	}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "git-prev-branch", "config.yaml"))
	}

	return paths
}

func defaultConfig() *Config {
	return &Config{
		Interactive: Interactive{
			DefaultAction: defaultReject,
		},
	}
}

func loadFromFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrConfigNotFound, path)
		}
		return fmt.Errorf("cannot read config file %s: %w", path, err)
	}
	if decodeErr := yaml.Unmarshal(data, cfg); decodeErr != nil {
		return fmt.Errorf("%w: %s: %w", ErrConfigMalformed, path, decodeErr)
	}
	return normalize(path, cfg)
}

// normalize validates and canonicalizes the values read from a config file.
// An absent or empty default_action keeps the built-in "reject"; anything else
// outside the two accepted values is a malformed file, so a typo cannot
// silently change what pressing Enter does at the confirmation prompt.
func normalize(path string, cfg *Config) error {
	action := strings.ToLower(strings.TrimSpace(cfg.Interactive.DefaultAction))
	switch action {
	case "":
		action = defaultReject
	case "accept", defaultReject:
	default:
		return fmt.Errorf(
			"%w: %s: interactive.default_action must be \"accept\" or \"reject\", got %q",
			ErrConfigMalformed, path, cfg.Interactive.DefaultAction,
		)
	}
	cfg.Interactive.DefaultAction = action
	return nil
}

// findRepoRoot finds the git repository root by walking up from startPath.
func findRepoRoot(startPath string) (string, error) {
	if strings.TrimSpace(startPath) == "" {
		startPath = "."
	}
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	current := absPath
	for {
		gitDir := filepath.Join(current, ".git")
		if info, statErr := os.Stat(gitDir); statErr == nil {
			if info.IsDir() || info.Mode().IsRegular() { // .git can be a file in worktrees, but usually dir
				return current, nil
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break // reached root
		}
		current = parent
	}
	return "", errors.New("not a git repo")
}
