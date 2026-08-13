package detect

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

// knownShells is the set of shells we skip when looking for the foreground command.
var knownShells = map[string]bool{
	"zsh":  true,
	"bash": true,
	"fish": true,
	"sh":   true,
	"dash": true,
	"tcsh": true,
	"csh":  true,
}

// knownWrappers maps binary names that wrap subcommands.
// For example: "node /path/to/npm run dev" → "npm run dev"
var knownWrappers = map[string]bool{
	"node": true,
}

// ForegroundCommand returns the command running in the foreground of the
// given tty, or "" if only a shell is running. Best-effort: returns ""
// on any error.
func ForegroundCommand(tty string) string {
	if tty == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-t", tty, "-o", "pid,ppid,stat,args").Output()
	if err != nil {
		return ""
	}
	return parseForegroundCommand(string(out))
}

// ForegroundCWD returns the working directory of the foreground process on the
// given tty — the running command if any, otherwise the pane's shell. Used to
// capture a per-pane CWD so restore can recreate each pane in its own directory
// instead of collapsing them all to the workspace path (GitHub #8). Best-effort:
// returns "" on any error.
func ForegroundCWD(tty string) string {
	if tty == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-t", tty, "-o", "pid,ppid,stat,args").Output()
	if err != nil {
		return ""
	}
	pid := foregroundLeaderPID(string(out))
	if pid == "" {
		return ""
	}
	return batchCWDs([]struct{ pid, tool string }{{pid, ""}})[pid]
}

// foregroundLeaderPID returns the pid whose CWD best represents the pane: the
// command running under the shell if there is one, otherwise the pane's
// foreground shell. "" if nothing suitable is found.
func foregroundLeaderPID(psOut string) string {
	type proc struct {
		pid, ppid string
		isShell   bool
	}
	var fg []proc
	shellPIDs := make(map[string]bool)

	for _, line := range strings.Split(psOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, ppid, stat := fields[0], fields[1], fields[2]
		args := strings.Join(fields[3:], " ")
		if pid == "PID" || strings.Contains(args, "/usr/bin/login") {
			continue
		}
		isShell := knownShells[extractBinName(args)]
		if isShell {
			shellPIDs[pid] = true
		}
		if strings.Contains(stat, "+") {
			fg = append(fg, proc{pid: pid, ppid: ppid, isShell: isShell})
		}
	}

	// Prefer a foreground command that is a child of a shell (the running tool).
	for _, p := range fg {
		if shellPIDs[p.ppid] && !p.isShell {
			return p.pid
		}
	}
	// Fallback: the pane's top foreground shell (plain prompt) — its cwd is the
	// directory the user is working in.
	for _, p := range fg {
		if p.isShell && !shellPIDs[p.ppid] {
			return p.pid
		}
	}
	for _, p := range fg {
		if p.isShell {
			return p.pid
		}
	}
	return ""
}

// parseForegroundCommand extracts the foreground command from ps output.
// It looks for the process leader: a foreground (S+) process whose parent
// is a shell. If the leader is a shell itself, returns "".
func parseForegroundCommand(psOut string) string {
	type proc struct {
		pid, ppid, args string
	}

	var fgProcs []proc
	shellPIDs := make(map[string]bool)

	for _, line := range strings.Split(psOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, ppid, stat := fields[0], fields[1], fields[2]
		args := strings.Join(fields[3:], " ")

		// Skip header.
		if pid == "PID" {
			continue
		}

		// Skip login processes.
		if strings.Contains(args, "/usr/bin/login") {
			continue
		}

		// Identify shells (including login shells like -/bin/zsh).
		binName := extractBinName(args)
		if knownShells[binName] {
			shellPIDs[pid] = true
		}

		// Only keep foreground processes (stat contains "+").
		if strings.Contains(stat, "+") {
			fgProcs = append(fgProcs, proc{pid: pid, ppid: ppid, args: args})
		}
	}

	// Find the foreground process leader: a foreground process whose parent is a shell.
	for _, p := range fgProcs {
		if !shellPIDs[p.ppid] {
			continue
		}
		// This is a direct child of a shell in the foreground group.
		binName := extractBinName(p.args)

		// If this is a shell itself, it's just a shell prompt — return "".
		if knownShells[binName] {
			return ""
		}

		return cleanCommand(p.args)
	}

	return ""
}

// extractBinName gets the base binary name from an args string,
// handling login-shell prefix (e.g. "-/bin/zsh" → "zsh").
func extractBinName(args string) string {
	firstWord := strings.SplitN(args, " ", 2)[0]
	return filepath.Base(strings.TrimPrefix(firstWord, "-"))
}

// cleanCommand normalizes a process args string for display and restore.
// Strips absolute paths from the command binary and handles wrapper binaries
// like node running npm.
func cleanCommand(args string) string {
	parts := strings.Fields(args)
	if len(parts) == 0 {
		return ""
	}

	bin := filepath.Base(parts[0])

	// Handle wrapper binaries: "node /path/to/npm run dev" → "npm run dev"
	if knownWrappers[bin] && len(parts) >= 2 && strings.Contains(parts[1], "/") {
		wrappedBin := filepath.Base(parts[1])
		remaining := parts[2:]
		result := wrappedBin
		if len(remaining) > 0 {
			result += " " + strings.Join(remaining, " ")
		}
		return capLength(result)
	}

	// Regular command: strip path from binary, keep args.
	result := bin
	if len(parts) > 1 {
		result += " " + strings.Join(parts[1:], " ")
	}
	return capLength(result)
}

// maxCommandLen bounds how much of a command line is worth keeping in a saved
// layout. Beyond it only the binary name is kept — never a fragment.
const maxCommandLen = 200

// shellHazards are characters whose meaning changes when a command is typed
// back into a shell. crex replays saved commands as literal keystrokes, so a
// command containing any of them cannot be reproduced faithfully: the shell
// re-parses the quoting and the program receives something different from what
// was captured.
const shellHazards = "\"'`$\\{}();&|<>\n\r\t*?[]!#~"

// capLength reduces a captured command line to something that is SAFE TO RE-TYPE.
//
// A command is kept verbatim only when it is short enough and free of shell
// metacharacters; otherwise just the binary name survives. Two failures drove
// this (GitHub #8 field report):
//
//   - cmux launches Claude through a wrapper that injects a JSON `--settings`
//     blob. Typed back, the shell strips its quotes and the pane dies with a
//     JSON parse error. Reduced to "claude", AI detection then upgrades it to a
//     proper `--resume` command;
//   - the previous behavior appended "..." to anything over 80 characters,
//     which is worse than useless: a truncated command is unrunnable, and a
//     cut-off path can act on the WRONG target when replayed.
//
// The binary name alone is always safe to type: at worst it starts the tool
// fresh, which is what a user would do by hand.
func capLength(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= maxCommandLen && !strings.ContainsAny(s, shellHazards) {
		return s
	}
	if bin, _, found := strings.Cut(s, " "); found {
		return bin
	}
	return s
}
