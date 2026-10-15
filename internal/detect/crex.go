package detect

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// crexBinaryName is the name crex is installed under, by Homebrew, by
// `go install` and by the Makefile's dev build alike.
const crexBinaryName = "crex"

// SelfExecutable returns the path of the running crex binary with symlinks
// resolved, or "" when the OS cannot tell. It is looked up once: the answer
// does not change while the process runs.
var SelfExecutable = sync.OnceValue(func() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	return resolvedPath(path)
})

// IsCrexCommand reports whether cmdline runs crex itself rather than merely
// mentioning it: `crex save now` and `/opt/homebrew/bin/crex restore x` do,
// `vim crex.toml` and `cd crex` do not. Leading VAR=value assignments are
// skipped.
//
// selfPath is the running executable (see SelfExecutable); "" disables the
// checks that need it. With it, a renamed build counts as crex when the
// command runs that same file, through a symlink or not, or names it without a
// directory, which is how saved commands store their binary.
func IsCrexCommand(cmdline, selfPath string) bool {
	_, bare := SplitEnvPrefix(strings.TrimSpace(cmdline))
	fields := strings.Fields(bare)
	if len(fields) == 0 {
		return false
	}
	program := fields[0]

	if filepath.Base(program) == crexBinaryName {
		return true
	}
	if selfPath == "" {
		return false
	}

	// A bare name is resolved through PATH by the shell, so the best that can
	// be said is that it carries the running executable's name.
	if !strings.Contains(program, "/") {
		return program == filepath.Base(selfPath)
	}
	return sameFile(program, selfPath)
}

// sameFile reports whether two paths name the same file once symlinks are
// followed. Paths that cannot be resolved are compared as written, so a
// missing file still matches its own spelling.
func sameFile(first, second string) bool {
	return resolvedPath(first) == resolvedPath(second)
}

// resolvedPath follows symlinks in path, falling back to the cleaned path
// when it does not exist or cannot be read.
func resolvedPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return resolved
}
