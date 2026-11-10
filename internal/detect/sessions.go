// Package detect discovers running AI CLI sessions (Claude Code, OpenCode,
// Codex, Amp, Gemini CLI, Copilot, Grok, and others) and returns resume
// commands for each. All functions are best-effort: if any detection step
// fails, it is silently skipped. The caller never sees an error.
package detect

import (
	"context"
	"os/exec"
	"regexp"
	"slices"
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

// SessionKey identifies one detected session: its resume command in its
// directory. The command alone is not enough, because Copilot and Grok resume
// with the same words everywhere and a tier-2 tool relaunches with its bare
// name, so two panes in two directories would look like one session.
type SessionKey struct {
	Command string
	CWD     string
}

// Key returns the key that identifies s.
func (s Session) Key() SessionKey {
	return SessionKey{Command: s.Command, CWD: s.CWD}
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
	// FlagSubcommand is the subcommand the flag belongs to, for a tool whose
	// top level rejects it: kiro-cli takes --trust-all-tools only after chat.
	// The flag goes after the subcommand, which a command without one gets.
	FlagSubcommand string
	// FlagOnFreshOnly is set for tools that refuse the flag on a resumed
	// session: the choice was stored when the session was created.
	FlagOnFreshOnly bool
	Detect          func(cwd, pid string) *Session
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
	// The tools below resume a session when they left one for the directory.
	// With none, each is started again with its bare launch command.
	{
		Name: "cursor",
		// The installer links both names to the same launcher under
		// ~/.local/share/cursor-agent/. "agent" alone could be anything,
		// so the install path has to confirm it.
		ProcessNames:   []string{"agent", "cursor-agent"},
		PathMarker:     "cursor-agent",
		Command:        "agent",
		TitlePatterns:  []string{"Cursor", "cursor"},
		AutoAcceptFlag: "--force",
		Detect:         detectCursor,
	},
	{
		Name:          "aider",
		ProcessNames:  []string{"aider"},
		TitlePatterns: []string{"Aider", "aider"},
		// --yes only worked as an argparse prefix of --yes-always.
		AutoAcceptFlag: "--yes-always",
		Detect:         detectAider,
	},
	{
		Name:         "pi",
		ProcessNames: []string{"pi"},
		// Pi titles the terminal "π - <session>". A plain "Pi" matched
		// "api", "pipeline" and "copilot". Pi has no flag: it does not ask
		// before running tools, and --approve is about trusting a project.
		TitlePatterns: []string{"π - "},
		Detect:        detectPi,
	},
	{
		Name: "rovo",
		// `acli rovodev run` starts Rovo Dev; the agent itself runs as
		// atlassian_cli_rovodev.
		ProcessNames:   []string{"atlassian_cli_rovodev"},
		LaunchPrefixes: []string{"acli rovodev"},
		Command:        "acli rovodev run",
		TitlePatterns:  []string{"Rovo", "rovo"},
		AutoAcceptFlag: "--yolo",
		Detect:         detectRovo,
	},
	{
		Name:           "hermes",
		ProcessNames:   []string{"hermes"},
		TitlePatterns:  []string{"Hermes", "hermes"},
		AutoAcceptFlag: "--yolo",
		// Hermes freezes --yolo into a session when it is created. Passing
		// it to --resume on a session started without it exits with an error.
		FlagOnFreshOnly: true,
		Detect:          detectHermes,
	},
	{
		Name: "codebuddy",
		// cbc is the short name of the same npm script. COIN-OR's solver
		// is also called cbc, so the path has to name CodeBuddy.
		ProcessNames: []string{"codebuddy", "cbc"},
		PathMarker:   "codebuddy",
		// CodeBuddy titles the terminal "✳ <topic>", the same mark Claude
		// Code uses. The pane goes to whichever tool has a session there.
		TitlePatterns:  []string{"CodeBuddy", "codebuddy", "✳ "},
		AutoAcceptFlag: "--dangerously-skip-permissions",
		Detect:         detectCodeBuddy,
	},
	{
		Name:           "factory",
		ProcessNames:   []string{"droid"},
		Command:        "droid",
		TitlePatterns:  []string{"Factory", "factory"},
		AutoAcceptFlag: "--skip-permissions-unsafe",
		Detect:         detectDroid,
	},
	{
		Name:         "qoder",
		ProcessNames: []string{"qoder", "qodercli"},
		// Once a session has a name the title is "◇ <name> | Ready",
		// "✦ <name> | Working" or "▲ <name> | Action Required", without
		// the word Qoder. Gemini CLI shares the first two marks.
		TitlePatterns: []string{"Qoder", "qoder", "◇ ", "✦ ", "▲ "},
		// The auto permission mode still lets a classifier stop commands;
		// --yolo is the one that skips every prompt.
		AutoAcceptFlag: "--yolo",
		Detect:         detectQoder,
	},
	{
		Name:         "qwen",
		ProcessNames: []string{"qwen"},
		// An untitled session shows "Qwen - <folder>". Once it has a name
		// the title is only that name, and the process has to identify it.
		TitlePatterns:  []string{"Qwen - "},
		AutoAcceptFlag: "--yolo",
		Detect:         detectQwen,
	},
	{
		Name:         "goose",
		ProcessNames: []string{"goose"},
		// Goose titles the terminal "🪿 <dir>". It has no flag: its default
		// mode already approves tool calls.
		TitlePatterns: []string{"🪿 "},
		Detect:        detectGoose,
	},
	{
		Name:           "crush",
		ProcessNames:   []string{"crush"},
		TitlePatterns:  []string{"crush "},
		AutoAcceptFlag: "--yolo",
		Detect:         detectCrush,
	},
	{
		Name: "kiro",
		// Kiro CLI 3 is one kiro-cli binary; the 2.x releases ran the chat
		// as kiro-cli-chat, which 3.1 no longer ships.
		ProcessNames:  []string{"kiro-cli", "kiro-cli-chat"},
		Command:       "kiro-cli",
		TitlePatterns: []string{"Kiro"},
		// `kiro-cli --trust-all-tools chat` exits with "unexpected
		// argument": the flag is chat's, not the top level's.
		AutoAcceptFlag: "--trust-all-tools",
		FlagSubcommand: "chat",
		Detect:         detectKiro,
	},
	{
		Name: "antigravity",
		// agy leaves the window title alone unless the user configures a
		// title command, so in practice the process identifies it. The
		// pattern only helps a title that names the tool.
		ProcessNames:   []string{"agy"},
		Command:        "agy",
		TitlePatterns:  []string{"Antigravity"},
		AutoAcceptFlag: "--dangerously-skip-permissions",
		Detect:         detectAntigravity,
	},
	{
		Name: "cline",
		// npm installs a Node wrapper that runs the compiled binary
		// bin/.cline, so ps shows both. Cline approves tool calls by
		// default; its --yolo is a separate mode for unattended runs, so
		// crex adds no flag.
		ProcessNames:  []string{"cline", ".cline"},
		TitlePatterns: []string{"Cline"},
		Detect:        detectCline,
	},
	{
		Name: "vibe",
		// Vibe 3 replaces the Python launcher with the bundled vibe-rs
		// program; Vibe 2 stays a Python script called vibe. The title is
		// "Vibe" only until the session gets a name of its own.
		ProcessNames:   []string{"vibe-rs", "vibe"},
		TitlePatterns:  []string{"Vibe"},
		AutoAcceptFlag: "--auto-approve",
		Detect:         detectVibe,
	},
	{
		Name:         "omp",
		ProcessNames: []string{"omp"},
		// oh-my-pi titles the terminal "π > label" when idle, "π ! label"
		// when it needs the user and "π : label" or "π: label" otherwise.
		// Pi itself uses "π - ".
		TitlePatterns:  []string{"π > ", "π ! ", "π : ", "π: "},
		AutoAcceptFlag: "--yolo",
		Detect:         detectOmp,
	},
	{
		Name: "kimi",
		// Kimi Code renames its process to kimi-code, which is what ps
		// shows as arguments for the npm build.
		ProcessNames:  []string{"kimi", "kimi-code"},
		TitlePatterns: []string{"Kimi Code"},
		// --yolo still asks before risky actions; --auto never asks.
		AutoAcceptFlag: "--auto",
		Detect:         detectKimi,
	},
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

// FlagOnFreshOnlyTools returns the tools whose auto-accept flag must not be
// added to a command that resumes a session.
func FlagOnFreshOnlyTools() map[string]bool {
	result := make(map[string]bool)
	for _, d := range registry {
		if d.FlagOnFreshOnly {
			result[d.Name] = true
		}
	}
	return result
}

// FlagSubcommands returns, for each tool that takes its auto-accept flag
// only after a subcommand, the name of that subcommand.
func FlagSubcommands() map[string]string {
	result := make(map[string]string)
	for _, d := range registry {
		if d.FlagSubcommand != "" {
			result[d.Name] = d.FlagSubcommand
		}
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
	// TTYs lists the terminals each session's processes run on, named the
	// way ps names them ("ttys004", "pts/3"). Save gives a session only to a
	// pane on one of them, so a session from another window never lands in
	// the one being saved. A session with no entry has no known terminal.
	TTYs map[SessionKey][]string
}

// AISessions scans for running AI CLI processes and resolves their session IDs.
//
// This function never returns an error. If detection fails at any stage,
// it returns whatever it found — possibly empty maps.
func AISessions() DetectedSessions {
	// Per-invocation caches (e.g. the `amp threads list` result) live until
	// the next call so each save sees fresh data.
	resetAmpCache()

	// Build tool name → detector lookup. The process scan reports tool
	// names, whatever binary each tool runs as.
	detectorByName := make(map[string]detector, len(registry))
	for _, d := range registry {
		detectorByName[d.Name] = d
	}

	procs := listAIProcesses(newProcessMatcher(registry))
	return collectSessions(procs, detectorByName)
}

// collectSessions asks each process's detector for its session and indexes
// what they return, with the terminals of all the processes each session was
// found for.
func collectSessions(procs []aiProcess, detectorByName map[string]detector) DetectedSessions {
	result := DetectedSessions{
		ByCWD:  make(map[string][]Session),
		ByTool: make(map[string][]Session),
		TTYs:   make(map[SessionKey][]string),
	}

	// Deduplicate detector invocations:
	//   - CWD-based tools (claude/opencode/codex): one call per (tool, cwd),
	//     since multiple instances in the same CWD resolve to the same session.
	//   - PID-based tools (amp, claude): one call per (tool, pid), so two
	//     instances in the same CWD each get their own session.
	// The processes skipped this way still run the session, so their
	// terminals are gathered under the key of the process that was asked.
	var keys []string
	first := make(map[string]aiProcess)
	ttys := make(map[string][]string)
	for _, p := range procs {
		key := p.tool + ":" + p.cwd
		if p.tool == "amp" || p.tool == "claude" {
			// amp resolves a thread per process; claude reads each process's
			// own session file and config dir, so two instances in one CWD
			// can hold different conversations.
			key = p.tool + ":" + p.pid
		}
		if _, ok := first[key]; !ok {
			keys = append(keys, key)
			first[key] = p
		}
		ttys[key] = appendTTY(ttys[key], p.tty)
	}

	seenSession := make(map[SessionKey]bool)
	for _, key := range keys {
		p := first[key]
		d := detectorByName[p.tool]

		s := d.Detect(p.cwd, p.pid)
		if s == nil {
			continue
		}
		addSession(result, seenSession, s)
		addTTYs(result, s.Key(), ttys[key])
	}
	return result
}

// appendTTY adds tty to ttys unless it is empty or already there.
func appendTTY(ttys []string, tty string) []string {
	if tty == "" || slices.Contains(ttys, tty) {
		return ttys
	}
	return append(ttys, tty)
}

// TTYName returns a terminal's name the way ps prints it: "ttys004" for both
// "/dev/ttys004" and "ttys004". It is "" for ps's marks of a process with no
// terminal ("??" on macOS, "?" on Linux).
func TTYName(device string) string {
	if device == "?" || device == "??" {
		return ""
	}
	return strings.TrimPrefix(device, "/dev/")
}

// addSession indexes s in result unless seen already holds the same session.
// PID-keyed detectors can still resolve two processes to the same session
// (e.g. the most-recent-transcript fallback); keep one. A nil s is ignored.
func addSession(result DetectedSessions, seen map[SessionKey]bool, s *Session) {
	if s == nil {
		return
	}

	key := s.Key()
	if seen[key] {
		return
	}
	seen[key] = true

	result.ByCWD[s.CWD] = append(result.ByCWD[s.CWD], *s)
	result.ByTool[s.Tool] = append(result.ByTool[s.Tool], *s)
}

// addTTYs records that the session key names runs on ttys. Two processes
// that resolve to one session leave it on both their terminals.
func addTTYs(result DetectedSessions, key SessionKey, ttys []string) {
	for _, tty := range ttys {
		result.TTYs[key] = appendTTY(result.TTYs[key], tty)
	}
}

// aiProcess holds a running AI CLI process.
type aiProcess struct {
	tool string // registry name: "claude", "opencode", "factory"
	pid  string
	cwd  string
	tty  string // the process's terminal as ps names it, "" when it has none
}

// parseProcessLine splits one line of `ps -o pid,tty,…` into the pid, the
// terminal and the fields of the column that follows them. ok is false for a
// line too short to hold all three.
func parseProcessLine(line string) (pid, tty string, rest []string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return "", "", nil, false
	}
	return fields[0], TTYName(fields[1]), fields[2:], true
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
	ttys := make(map[string]string)  // pid → terminal, to tell windows apart

	// Pass 1: match by `comm` (the executable name). Fast and unambiguous for
	// tools whose binary is named after the tool (e.g. an nvm/brew `claude` shim).
	if out, err := exec.CommandContext(ctx, "ps", "axo", "pid,tty,comm").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			pid, tty, comm, ok := parseProcessLine(line)
			if !ok {
				continue
			}
			if tool, ok := matcher.matchComm(comm[0]); ok {
				pids = append(pids, struct {
					pid  string
					tool string
				}{pid, tool})
				matched[pid] = true
				ttys[pid] = tty
			}
		}
	}

	// Pass 2: match by full command line (`args`), catching what `comm` misses —
	// shell aliases/functions, wrapper scripts, and interpreter-launched CLIs like
	// `node …/claude/cli.js` whose `comm` is "node" (GitHub #6).
	if out, err := exec.CommandContext(ctx, "ps", "axo", "pid=,tty=,args=").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			pid, tty, args, ok := parseProcessLine(line)
			if !ok {
				continue
			}
			if matched[pid] {
				continue
			}
			if tool, ok := matcher.matchArgs(strings.Join(args, " ")); ok {
				pids = append(pids, struct {
					pid  string
					tool string
				}{pid, tool})
				matched[pid] = true
				ttys[pid] = tty
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
		result = append(result, aiProcess{tool: p.tool, pid: p.pid, cwd: cwd, tty: ttys[p.pid]})
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
