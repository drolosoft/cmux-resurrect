package detect

import (
	"path/filepath"
	"slices"
	"strings"
)

// scriptInterpreters are binaries that run an AI CLI as a script argument.
// When the foreground process is one of these, the tool identity lives in a
// later path argument (e.g. `node …/claude/cli.js`) rather than in argv[0].
// They are listed by family: interpreterFamily turns python3.12 or Python
// into python before the lookup.
var scriptInterpreters = map[string]bool{
	"node": true, "bun": true, "deno": true, "npx": true, "bunx": true,
	"python": true, "ruby": true, "perl": true,
	"tsx": true, "ts-node": true,
}

// interpreterFamily returns the interpreter name without what installs add
// to it: the version (python3.12, ruby3.3) and the case. Homebrew's and
// python.org's framework builds re-exec themselves as
// Python.app/Contents/MacOS/Python, and uv links python to python3.12.
func interpreterFamily(name string) string {
	return strings.TrimRight(strings.ToLower(name), "0123456789.")
}

// processMatcher tells which registered tool a process runs, from what ps
// shows about it. It answers with the tool's registry name, never with the
// binary name, so `droid` comes back as "factory".
type processMatcher struct {
	byProcessName map[string]detector // every process name of every tool
	launchers     []detector          // tools that also run through a launcher binary
}

// newProcessMatcher indexes detectors by the names their processes take.
func newProcessMatcher(detectors []detector) processMatcher {
	matcher := processMatcher{byProcessName: make(map[string]detector)}
	for _, entry := range detectors {
		for _, process := range entry.ProcessNames {
			matcher.byProcessName[process] = entry
		}
		if len(entry.LaunchPrefixes) > 0 {
			matcher.launchers = append(matcher.launchers, entry)
		}
	}
	return matcher
}

// matchComm identifies a process from the `comm` column of ps: the
// executable path on macOS, the bare executable name on Linux.
func (matcher processMatcher) matchComm(comm string) (string, bool) {
	return matcher.tool(filepath.Base(comm), comm)
}

// matchArgs identifies which AI tool a process is running from its full
// command line (`ps -o args`), for cases the `comm` (executable) column misses —
// shell aliases/functions, wrapper scripts, and interpreter-launched CLIs such as
// `node …/claude/cli.js` where `comm` is "node".
//
// Matching is deliberately conservative to avoid false positives:
//  1. argv[0] and the words after it are a tool's launcher command line
//     (e.g. `acli rovodev run`)
//  2. argv[0] basename is a known process name (alias expansion / wrapper
//     named after the tool)
//  3. argv[0] is a known interpreter AND a later non-flag path argument names a
//     tool as its basename (minus extension) or as a path segment
//
// It never matches a tool name that appears only as a file *argument* of a
// non-interpreter command (e.g. `cat claude`, `vim ~/claude/notes.md`).
func (matcher processMatcher) matchArgs(args string) (string, bool) {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return "", false
	}

	if tool, ok := matcher.launchedTool(fields); ok {
		return tool, true
	}

	argv0 := filepath.Base(fields[0])
	if tool, ok := matcher.tool(argv0, fields[0]); ok {
		return tool, true
	}

	if !scriptInterpreters[interpreterFamily(argv0)] {
		return "", false
	}

	// Interpreter case: scan path-like, non-flag arguments for a tool name.
	for _, f := range fields[1:] {
		if f == "" || strings.HasPrefix(f, "-") {
			continue
		}
		base := filepath.Base(f)
		base = strings.TrimSuffix(base, filepath.Ext(base)) // claude.js → claude
		if tool, ok := matcher.tool(base, f); ok {
			return tool, true
		}
		for _, seg := range strings.Split(f, "/") {
			if seg == "" {
				continue
			}
			if tool, ok := matcher.tool(seg, f); ok {
				return tool, true
			}
		}
	}
	return "", false
}

// launchedTool matches a command line that starts a tool through a launcher
// binary, such as `acli rovodev run`: argv[0] is compared by basename, the
// words after it exactly. A launcher running anything else (`acli jira …`)
// is not a match.
func (matcher processMatcher) launchedTool(fields []string) (string, bool) {
	launcher := filepath.Base(fields[0])
	for _, entry := range matcher.launchers {
		for _, prefix := range entry.LaunchPrefixes {
			words := strings.Fields(prefix)
			if len(fields) < len(words) || launcher != words[0] {
				continue
			}
			if slices.Equal(fields[1:len(words)], words[1:]) {
				return entry.Name, true
			}
		}
	}
	return "", false
}

// tool returns the registry name of the tool a process name belongs to.
// path is where the name was seen: the executable path or a script argument.
// A tool with a PathMarker claims the process only when that path, or the
// file it links to, contains the marker; that keeps a generic name such as
// "agent" from matching every program called agent.
func (matcher processMatcher) tool(name, path string) (string, bool) {
	entry, known := matcher.byProcessName[name]
	if !known {
		return "", false
	}
	if entry.PathMarker != "" && !pathMentions(path, entry.PathMarker) {
		return "", false
	}
	return entry.Name, true
}

// pathMentions reports whether path, or the file an absolute path links to,
// contains marker. Installers often put a link in ~/.local/bin whose own name
// says nothing about the tool, while the real file sits in a directory named
// after it. A relative path is not resolved: it would be read against crex's
// working directory, not the process's.
func pathMentions(path, marker string) bool {
	if strings.Contains(path, marker) {
		return true
	}
	if !filepath.IsAbs(path) {
		return false
	}

	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	return strings.Contains(target, marker)
}
