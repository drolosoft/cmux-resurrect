package detect

import (
	"path/filepath"
	"strings"
)

// scriptInterpreters are binaries that run an AI CLI as a script argument.
// When the foreground process is one of these, the tool identity lives in a
// later path argument (e.g. `node …/claude/cli.js`) rather than in argv[0].
var scriptInterpreters = map[string]bool{
	"node": true, "bun": true, "deno": true, "npx": true, "bunx": true,
	"python": true, "python3": true, "ruby": true, "perl": true,
	"tsx": true, "ts-node": true,
}

// matchAIToolArgs identifies which AI tool a process is running from its full
// command line (`ps -o args`), for cases the `comm` (executable) column misses —
// shell aliases/functions, wrapper scripts, and interpreter-launched CLIs such as
// `node …/claude/cli.js` where `comm` is "node".
//
// isKnown reports whether a name is a registered tool ProcessName. Matching is
// deliberately conservative to avoid false positives:
//  1. argv[0] basename is a known tool (alias expansion / wrapper named after the tool)
//  2. argv[0] is a known interpreter AND a later non-flag path argument names a
//     tool as its basename (minus extension) or as a path segment
//
// It never matches a tool name that appears only as a file *argument* of a
// non-interpreter command (e.g. `cat claude`, `vim ~/claude/notes.md`).
func matchAIToolArgs(args string, isKnown func(string) bool) (string, bool) {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return "", false
	}

	argv0 := filepath.Base(fields[0])
	if isKnown(argv0) {
		return argv0, true
	}

	if !scriptInterpreters[argv0] {
		return "", false
	}

	// Interpreter case: scan path-like, non-flag arguments for a tool name.
	for _, f := range fields[1:] {
		if f == "" || strings.HasPrefix(f, "-") {
			continue
		}
		base := filepath.Base(f)
		base = strings.TrimSuffix(base, filepath.Ext(base)) // claude.js → claude
		if isKnown(base) {
			return base, true
		}
		for _, seg := range strings.Split(f, "/") {
			if seg != "" && isKnown(seg) {
				return seg, true
			}
		}
	}
	return "", false
}
