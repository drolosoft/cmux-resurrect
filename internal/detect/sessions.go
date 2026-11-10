// Package detect discovers running AI CLI sessions (Claude Code, OpenCode,
// Codex, Amp, Gemini CLI, Copilot, Grok, and others) and returns resume
// commands for each. All functions are best-effort: if any detection step
// fails, it is silently skipped. The caller never sees an error.
package detect

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Session represents a detected AI CLI session.
type Session struct {
	Tool    string // e.g. "claude", "opencode", "codex", "amp", "gemini", "copilot", "grok"
	CWD     string // working directory of the process
	Command string // full resume command (e.g. "claude --resume <id>")
	Title   string // conversation title shown in the terminal title, if known (claude)
}

// detector defines how to detect and resolve sessions for one AI tool.
// Each tool registers its process names, title patterns, and detection logic.
// The Detect function receives both the working directory and the PID of
// the running process; tools that don't need the PID may ignore it.
//
// Name is what saved layouts and the auto_accept config store, so it never
// changes once a tool ships. What the tool runs as, and what starts it, can
// differ from it: Factory's binary is `droid`, Rovo Dev starts with
// `acli rovodev run`. ProcessNames and Command say so.
type detector struct {
	Name           string   // tool name (used as Session.Tool)
	ProcessNames   []string // binary names as shown by ps (e.g. "claude"), main one first
	PathMarker     string   // when set, a process counts only if its path contains it
	LaunchPrefixes []string // launcher command lines that also mean the tool (e.g. "acli rovodev")
	Command        string   // what a shell types to start the tool, when it is not Name
	TitlePatterns  []string // substrings in terminal title confirming the tool
	AutoAcceptFlag string   // CLI flag to skip permission prompts (empty if none)
	Detect         func(cwd, pid string) *Session
}

// launchCommand returns the words that start the tool from a shell: Command
// when the tool sets one, its Name otherwise.
func (d detector) launchCommand() string {
	if d.Command != "" {
		return d.Command
	}
	return d.Name
}

// registry holds all known AI tool detectors. To add a new tool,
// append a detector here — no other code changes needed.
var registry = []detector{
	{
		Name:         "claude",
		ProcessNames: []string{"claude"},
		// Claude Code sets various titles depending on the active screen:
		// "✳ Claude Code", "⠂ Claude Code", "✳ Available Commands", etc.
		// The ✳/⠂/⠐ prefixes are Claude-specific status indicators.
		TitlePatterns:  []string{"Claude Code", "claude", "✳ ", "⠂ ", "⠐ "},
		AutoAcceptFlag: "--dangerously-skip-permissions",
		Detect:         detectClaude,
	},
	{
		Name:          "opencode",
		ProcessNames:  []string{"opencode"},
		TitlePatterns: []string{"OpenCode", "opencode", "OC |"},
		// --auto is the documented flag; --yolo survives only as a hidden
		// alias. Older releases such as 1.14.48 have neither and refuse to
		// start with an unknown flag.
		AutoAcceptFlag: "--auto",
		Detect:         detectOpenCode,
	},
	{
		Name:          "codex",
		ProcessNames:  []string{"codex"},
		TitlePatterns: []string{"Codex", "codex"},
		// --full-auto never skipped approvals, and Codex 0.128 removed it
		// from the interactive CLI, which then refuses to start.
		AutoAcceptFlag: "--dangerously-bypass-approvals-and-sandbox",
		Detect:         detectCodex,
	},
	{
		Name:           "amp",
		ProcessNames:   []string{"amp"},
		TitlePatterns:  []string{"Amp", "amp"},
		AutoAcceptFlag: "--dangerously-allow-all",
		Detect:         detectAmp,
	},
	{
		Name:          "gemini",
		ProcessNames:  []string{"gemini"},
		TitlePatterns: []string{"◇ ", "✦ ", "✋ ", "Gemini", "gemini"},
		// --sandbox runs tools in a sandbox, it approves nothing. -y/--yolo
		// still works but is deprecated in favour of this one.
		AutoAcceptFlag: "--approval-mode=yolo",
		Detect:         detectGemini,
	},
	{
		Name:           "copilot",
		ProcessNames:   []string{"copilot"},
		TitlePatterns:  []string{"Copilot", "copilot"},
		AutoAcceptFlag: "--allow-all",
		Detect:         detectCopilot,
	},
	{
		Name:           "grok",
		ProcessNames:   []string{"grok"},
		TitlePatterns:  []string{"Grok", "grok"},
		AutoAcceptFlag: "--always-approve",
		Detect:         detectGrok,
	},
	// Tier 2: Process-aware detectors — recognized but no session file parsing.
	processAware(detector{
		Name: "cursor",
		// The installer links both names to the same launcher under
		// ~/.local/share/cursor-agent/. "agent" alone could be anything,
		// so the install path has to confirm it.
		ProcessNames:   []string{"agent", "cursor-agent"},
		PathMarker:     "cursor-agent",
		Command:        "agent",
		TitlePatterns:  []string{"Cursor", "cursor"},
		AutoAcceptFlag: "--force",
	}),
	processAware(detector{
		Name:          "aider",
		ProcessNames:  []string{"aider"},
		TitlePatterns: []string{"Aider", "aider"},
		// --yes only worked as an argparse prefix of --yes-always.
		AutoAcceptFlag: "--yes-always",
	}),
	processAware(detector{
		Name:         "pi",
		ProcessNames: []string{"pi"},
		// Pi titles the terminal "π - <session>". A plain "Pi" matched
		// "api", "pipeline" and "copilot". Pi has no flag: it does not ask
		// before running tools, and --approve is about trusting a project.
		TitlePatterns: []string{"π - "},
	}),
	processAware(detector{
		Name: "rovo",
		// `acli rovodev run` starts Rovo Dev; the agent itself runs as
		// atlassian_cli_rovodev.
		ProcessNames:   []string{"atlassian_cli_rovodev"},
		LaunchPrefixes: []string{"acli rovodev"},
		Command:        "acli rovodev run",
		TitlePatterns:  []string{"Rovo", "rovo"},
		AutoAcceptFlag: "--yolo",
	}),
	processAware(detector{
		Name:           "hermes",
		ProcessNames:   []string{"hermes"},
		TitlePatterns:  []string{"Hermes", "hermes"},
		AutoAcceptFlag: "--yolo",
	}),
	processAware(detector{
		Name: "codebuddy",
		// cbc is the short name of the same npm script. COIN-OR's solver
		// is also called cbc, so the path has to name CodeBuddy.
		ProcessNames:   []string{"codebuddy", "cbc"},
		PathMarker:     "codebuddy",
		TitlePatterns:  []string{"CodeBuddy", "codebuddy"},
		AutoAcceptFlag: "--dangerously-skip-permissions",
	}),
	processAware(detector{
		Name:           "factory",
		ProcessNames:   []string{"droid"},
		Command:        "droid",
		TitlePatterns:  []string{"Factory", "factory"},
		AutoAcceptFlag: "--skip-permissions-unsafe",
	}),
	processAware(detector{
		Name:          "qoder",
		ProcessNames:  []string{"qoder", "qodercli"},
		TitlePatterns: []string{"Qoder", "qoder"},
		// The auto permission mode still lets a classifier stop commands;
		// --yolo is the one that skips every prompt.
		AutoAcceptFlag: "--yolo",
	}),
}

// BareCommands returns every command a pane can hold that only says which AI
// tool runs there, with no session to resume: each process name, each launch
// command, and each tool name (older versions saved "factory" or "cursor" as
// the command of a tier-2 pane). The save flow treats these as unassigned so
// detection can replace them with a real resume or launch command.
func BareCommands() map[string]bool {
	result := make(map[string]bool)
	for _, d := range registry {
		for _, process := range d.ProcessNames {
			result[process] = true
		}
		result[d.launchCommand()] = true
		result[d.Name] = true
	}
	return result
}

// ToolLaunch finds the registered tool a command line starts. It returns the
// tool name and the words that launch it ("acli rovodev run" for rovo), so a
// caller can add flags right after those words. ok is false for any command
// that does not begin with a tool's launch command.
func ToolLaunch(cmdline string) (tool, launch string, ok bool) {
	for _, d := range registry {
		candidate := d.launchCommand()
		if cmdline != candidate && !strings.HasPrefix(cmdline, candidate+" ") {
			continue
		}

		// No launch command is a prefix of another today; preferring the
		// longest keeps it that way if one ever is.
		if len(candidate) > len(launch) {
			tool = d.Name
			launch = candidate
			ok = true
		}
	}
	return tool, launch, ok
}

// TitlePatterns returns the title patterns for all registered tools,
// keyed by tool name. Used by the save flow for pane matching.
func TitlePatterns() map[string][]string {
	result := make(map[string][]string, len(registry))
	for _, d := range registry {
		result[d.Name] = d.TitlePatterns
	}
	return result
}

// ToolTitlePatterns pairs a tool name with the title patterns that confirm it.
type ToolTitlePatterns struct {
	Tool     string
	Patterns []string
}

// OrderedTitlePatterns returns the title patterns of every registered tool in
// registry order. A title can match several tools, and ranging over the map
// from TitlePatterns tries them in a different order on every run.
func OrderedTitlePatterns() []ToolTitlePatterns {
	result := make([]ToolTitlePatterns, 0, len(registry))
	for _, d := range registry {
		result = append(result, ToolTitlePatterns{Tool: d.Name, Patterns: d.TitlePatterns})
	}
	return result
}

// AutoAcceptFlags returns a map of tool name → auto-accept CLI flag
// for all registered tools that have one. Tools without a flag are omitted.
func AutoAcceptFlags() map[string]string {
	result := make(map[string]string)
	for _, d := range registry {
		if d.AutoAcceptFlag != "" {
			result[d.Name] = d.AutoAcceptFlag
		}
	}
	return result
}

// DetectedSessions holds detected AI CLI sessions indexed for lookup.
type DetectedSessions struct {
	ByCWD  map[string][]Session // CWD → sessions (multiple tools can share a CWD)
	ByTool map[string][]Session // tool name → sessions (fallback when CWDs don't match)
}

// AISessions scans for running AI CLI processes and resolves their session IDs.
//
// This function never returns an error. If detection fails at any stage,
// it returns whatever it found — possibly empty maps.
func AISessions() DetectedSessions {
	// Per-invocation caches (e.g. the `amp threads list` result) live until
	// the next call so each save sees fresh data.
	resetAmpCache()

	result := DetectedSessions{
		ByCWD:  make(map[string][]Session),
		ByTool: make(map[string][]Session),
	}

	// Deduplicate detector invocations:
	//   - CWD-based tools (claude/opencode/codex): one call per (tool, cwd),
	//     since multiple instances in the same CWD resolve to the same session.
	//   - PID-based tools (amp, claude): one call per (tool, pid), so two
	//     instances in the same CWD each get their own session.
	seen := make(map[string]bool)
	seenCommand := make(map[string]bool)

	// Build tool name → detector lookup. The process scan reports tool
	// names, whatever binary each tool runs as.
	detectorByName := make(map[string]detector, len(registry))
	for _, d := range registry {
		detectorByName[d.Name] = d
	}

	procs := listAIProcesses(newProcessMatcher(registry))
	for _, p := range procs {
		key := p.tool + ":" + p.cwd
		if p.tool == "amp" || p.tool == "claude" {
			// amp resolves a thread per process; claude reads each process's
			// own session file and config dir, so two instances in one CWD
			// can hold different conversations.
			key = p.tool + ":" + p.pid
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		d := detectorByName[p.tool]
		s := d.Detect(p.cwd, p.pid)
		// PID-keyed detectors can still resolve two processes to the same
		// session (e.g. the most-recent-transcript fallback); keep one.
		if s != nil && !seenCommand[s.Command] {
			seenCommand[s.Command] = true
			result.ByCWD[s.CWD] = append(result.ByCWD[s.CWD], *s)
			result.ByTool[s.Tool] = append(result.ByTool[s.Tool], *s)
		}
	}
	return result
}

// aiProcess holds a running AI CLI process.
type aiProcess struct {
	tool string // registry name: "claude", "opencode", "factory"
	pid  string
	cwd  string
}

// listAIProcesses finds running claude, opencode, and codex processes
// and resolves their working directories via lsof.
// cmdTimeout is the maximum time for any subprocess (ps, lsof, sqlite3).
const cmdTimeout = 5 * time.Second

func listAIProcesses(matcher processMatcher) []aiProcess {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	var pids []struct {
		pid  string
		tool string
	}
	matched := make(map[string]bool) // pids already claimed, to avoid duplicates

	// Pass 1: match by `comm` (the executable name). Fast and unambiguous for
	// tools whose binary is named after the tool (e.g. an nvm/brew `claude` shim).
	if out, err := exec.CommandContext(ctx, "ps", "axo", "pid,comm").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			pid := fields[0]
			if tool, ok := matcher.matchComm(fields[1]); ok {
				pids = append(pids, struct {
					pid  string
					tool string
				}{pid, tool})
				matched[pid] = true
			}
		}
	}

	// Pass 2: match by full command line (`args`), catching what `comm` misses —
	// shell aliases/functions, wrapper scripts, and interpreter-launched CLIs like
	// `node …/claude/cli.js` whose `comm` is "node" (GitHub #6).
	if out, err := exec.CommandContext(ctx, "ps", "axo", "pid=,args=").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			pid := fields[0]
			if matched[pid] {
				continue
			}
			if tool, ok := matcher.matchArgs(strings.Join(fields[1:], " ")); ok {
				pids = append(pids, struct {
					pid  string
					tool string
				}{pid, tool})
				matched[pid] = true
			}
		}
	}

	if len(pids) == 0 {
		return nil
	}

	cwds := batchCWDs(pids)

	var result []aiProcess
	for _, p := range pids {
		cwd := cwds[p.pid]
		if cwd == "" {
			continue
		}
		result = append(result, aiProcess{tool: p.tool, pid: p.pid, cwd: cwd})
	}
	return result
}

// batchCWDs returns each PID's working directory in a single lsof call.
//
// While walking the output it also opportunistically seeds the amp
// thread cache: every amp process keeps an open write handle on its
// per-thread log file (~/.cache/amp/logs/threads/T-<id>.log), and that
// path is already in this lsof output. Picking it up here means amp
// detection adds no extra subprocess work.
func batchCWDs(pids []struct{ pid, tool string }) map[string]string {
	cwds := make(map[string]string)
	if len(pids) == 0 {
		return cwds
	}

	pidStrs := make([]string, len(pids))
	for i, p := range pids {
		pidStrs[i] = p.pid
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "lsof", "-p", strings.Join(pidStrs, ","), "-Fn").Output()
	if err != nil {
		return cwds
	}

	// `lsof -Fn` emits one process group at a time:
	//   p<pid>
	//   f<fd>       (e.g. fcwd, f21, f0u)
	//   n<path>
	//   f<fd>
	//   n<path>
	//   ...
	//
	// Each `f` line names a file descriptor; the following `n` line gives
	// that fd's path. We capture `fcwd` as the cwd; we also recognize the
	// amp per-thread log path and seed the amp cache as a side effect.
	var currentPID, lastFD string
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			currentPID = line[1:]
			lastFD = ""
		case 'f':
			lastFD = line[1:]
		case 'n':
			path := line[1:]
			if lastFD == "cwd" {
				cwds[currentPID] = path
			} else if id := threadIDFromLogPath(path); id != "" {
				seedAmpThread(currentPID, id)
			}
		}
	}
	return cwds
}

// escapeSQLite escapes single quotes for SQLite string literals.
// The input comes from lsof output (machine-sourced), not user input.
func escapeSQLite(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// validSessionID checks that a session ID contains only safe characters.
// This prevents corrupted or crafted IDs from injecting shell commands
// when the resume command is sent to a terminal via Send.
var validSessionID = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)
