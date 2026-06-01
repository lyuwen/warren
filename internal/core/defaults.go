package core

// defaults.go centralises Warren's filesystem-default helpers so each binary
// resolves the canonical config directory and database path the same way.
// Prior to this consolidation, three call sites used three subtly different
// forms (`"warren.db"`, `os.ExpandEnv("$HOME/.warren/...")`, the web
// default) and drifted apart. Phase 2 audit finding "warren.db default
// path" is the root cause of this file.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrLegacyDBOrphan = errors.New("legacy database would be orphaned")

// DefaultConfigDir returns the canonical Warren config directory.
//
// It is "$HOME/.warren" when the home directory is resolvable. When
// os.UserHomeDir() fails (unprivileged container, broken env), it falls
// back to ".warren" relative to the current working directory; the
// fallback is documented and exercised by tests.
//
// Use this helper instead of os.ExpandEnv("$HOME/.warren") — ExpandEnv
// silently substitutes the empty string for missing $HOME and yields
// "/.warren", which is wrong on every reasonable platform.
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".warren"
	}
	return filepath.Join(home, ".warren")
}

// DefaultDBPath returns the canonical Warren database path, joined under
// DefaultConfigDir(). When the home directory is unresolvable it falls
// back to "warren.db" in the current working directory.
func DefaultDBPath() string {
	dir := DefaultConfigDir()
	if dir == ".warren" {
		// Home-unresolvable fallback: keep the legacy cwd-relative shape so
		// a process without $HOME still has a writable spot.
		return "warren.db"
	}
	return filepath.Join(dir, "warren.db")
}

// EnsureConfigDir creates the given directory (mode 0755) if it does not
// already exist. Idempotent. Pass core.DefaultConfigDir() for the canonical
// location, or any custom path. Called by core.NewWarren so the TUI and
// web binaries inherit directory creation without duplicating MkdirAll
// across cmds.
func EnsureConfigDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("EnsureConfigDir: dir must not be empty")
	}
	return os.MkdirAll(dir, 0755)
}

type legacyDBError struct {
	cwd         string
	defaultPath string
}

func (e *legacyDBError) Error() string {
	defaultDir := filepath.Dir(e.defaultPath)
	binaryHint := "warren"
	return fmt.Sprintf(`Found legacy database at ./warren.db (cwd-relative).
Warren now uses %s by default.

To preserve your data:
    mkdir -p %s && mv ./warren.db %s

Or keep the legacy location with:
    %s -db ./warren.db

Refusing to start (would create empty database and orphan your data).`,
		e.defaultPath,
		defaultDir,
		e.defaultPath,
		binaryHint,
	)
}

func (e *legacyDBError) Unwrap() error { return ErrLegacyDBOrphan }

// CheckLegacyDB reports an actionable error when:
//   - the caller did NOT explicitly set the -db flag (userOverrode == false), AND
//   - filepath.Join(cwd, "warren.db") exists, AND
//   - defaultPath does NOT exist.
//
// In every other case it returns nil. Callers (warren-web, warren-tui)
// should print the error to stderr and exit non-zero rather than silently
// create a fresh empty database at the new default path while leaving the
// user's real data orphaned in cwd.
//
// userOverrode MUST be derived from `flag.Visit` (which iterates only flags
// the user actually set), not from comparing resolved values.
func CheckLegacyDB(userOverrode bool, cwd string, defaultPath string) error {
	if userOverrode {
		return nil
	}
	if cwd == "" || defaultPath == "" {
		return nil
	}
	legacyPath := filepath.Join(cwd, "warren.db")
	if _, err := os.Stat(legacyPath); err != nil {
		return nil // no legacy file in cwd; nothing to migrate
	}
	if _, err := os.Stat(defaultPath); err == nil {
		// New DB already exists — user has presumably migrated, or this is
		// a separate working directory that happens to have a stray
		// warren.db. Don't false-alarm.
		return nil
	}
	return &legacyDBError{cwd: cwd, defaultPath: defaultPath}
}
