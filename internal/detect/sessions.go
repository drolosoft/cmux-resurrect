// Package detect discovers running AI CLI sessions (Claude Code, OpenCode,
// Codex, Amp) and returns resume commands for each. All functions are
// best-effort: if any detection step fails, it is silently skipped.
// The caller never sees an error.
package detect

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Session represents a detected AI CLI session.
type Session struct {
	Tool    string // "claude", "opencode", "codex", "amp"
	CWD     string // working directory of the process
	Command string // full resume command (e.g. "claude --resume <id>")
}

// detector defines how to detect and resolve sessions for one AI tool.
// Each tool registers its process name, title patterns, and detection logic.
// The Detect function receives both the working directory and the PID of
// the running process; tools that don't need the PID may ignore it.
type detector struct {
	Name          string   // tool name (used as Session.Tool)
	ProcessName   string   // binary name as shown by ps (e.g. "claude")
	TitlePatterns []string // substrings in terminal title confirming the tool
	Detect        func(cwd, pid string) *Session
}

// registry holds all known AI tool detectors. To add a new tool,
// append a detector here — no other code changes needed.
var registry = []detector{
	{
		Name:        "claude",
		ProcessName: "claude",
		// Claude Code sets various titles depending on the active screen:
		// "✳ Claude Code", "⠂ Claude Code", "✳ Available Commands", etc.
		// The ✳/⠂/⠐ prefixes are Claude-specific status indicators.
		TitlePatterns: []string{"Claude Code", "claude", "✳ ", "⠂ ", "⠐ "},
		Detect:        detectClaude,
	},
	{
		Name:          "opencode",
		ProcessName:   "opencode",
		TitlePatterns: []string{"OpenCode", "opencode", "OC |"},
		Detect:        detectOpenCode,
	},
	{
		Name:          "codex",
		ProcessName:   "codex",
		TitlePatterns: []string{"Codex", "codex"},
		Detect:        detectCodex,
	},
	{
		Name:          "amp",
		ProcessName:   "amp",
		TitlePatterns: []string{"Amp", "amp"},
		Detect:        detectAmp,
	},
}

// ProcessNames returns the set of binary names for all registered AI tools.
// Used by the save flow to distinguish AI tool commands from generic
// foreground commands (e.g. "claude" vs "htop").
func ProcessNames() map[string]bool {
	result := make(map[string]bool, len(registry))
	for _, d := range registry {
		result[d.ProcessName] = true
	}
	return result
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
	//   - PID-based tools (amp): one call per (tool, pid), so two amp
	//     instances in the same CWD each get their own thread.
	seen := make(map[string]bool)

	// Build process name → detector lookup.
	detectorByName := make(map[string]detector, len(registry))
	for _, d := range registry {
		detectorByName[d.ProcessName] = d
	}

	procs := listAIProcesses(detectorByName)
	for _, p := range procs {
		key := p.tool + ":" + p.cwd
		if p.tool == "amp" {
			key = p.tool + ":" + p.pid
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		d := detectorByName[p.tool]
		s := d.Detect(p.cwd, p.pid)
		if s != nil {
			result.ByCWD[s.CWD] = append(result.ByCWD[s.CWD], *s)
			result.ByTool[s.Tool] = append(result.ByTool[s.Tool], *s)
		}
	}
	return result
}

// aiProcess holds a running AI CLI process.
type aiProcess struct {
	tool string // "claude", "opencode", "codex"
	pid  string
	cwd  string
}

// listAIProcesses finds running claude, opencode, and codex processes
// and resolves their working directories via lsof.
// cmdTimeout is the maximum time for any subprocess (ps, lsof, sqlite3).
const cmdTimeout = 5 * time.Second

func listAIProcesses(detectors map[string]detector) []aiProcess {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "axo", "pid,comm").Output()
	if err != nil {
		return nil
	}

	var pids []struct {
		pid  string
		tool string
	}

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid := fields[0]
		comm := filepath.Base(fields[1])

		if _, ok := detectors[comm]; ok {
			pids = append(pids, struct {
				pid  string
				tool string
			}{pid, comm})
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

// detectClaude finds the active session ID for a Claude Code instance by CWD.
// Sessions are stored as .jsonl files in ~/.claude/projects/<path-with-dashes>/.
func detectClaude(cwd, _ string) *Session {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	// Claude converts "/" to "-" for the project directory name.
	projectPath := strings.ReplaceAll(cwd, "/", "-")
	projectDir := filepath.Join(home, ".claude", "projects", projectPath)

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil
	}

	// Find the most recently modified .jsonl file. Skip files under 500
	// bytes — Claude creates tiny placeholder sessions (236 bytes) when
	// a --resume with a wrong ID fails. These placeholders can be more
	// recent than the actual running session and must be ignored.
	const minSessionSize = 500
	type fileInfo struct {
		name    string
		modTime int64
	}
	var sessions []fileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Size() < minSessionSize {
			continue
		}
		sessions = append(sessions, fileInfo{
			name:    e.Name(),
			modTime: info.ModTime().UnixNano(),
		})
	}
	if len(sessions) == 0 {
		return nil
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].modTime > sessions[j].modTime
	})

	sessionID := strings.TrimSuffix(sessions[0].name, ".jsonl")
	if !validSessionID.MatchString(sessionID) {
		return nil
	}
	return &Session{
		Tool:    "claude",
		CWD:     cwd,
		Command: "claude --resume " + sessionID,
	}
}

// detectOpenCode finds the active session for an OpenCode instance by CWD.
// Sessions are stored in ~/.local/share/opencode/opencode.db (SQLite).
// We shell out to the sqlite3 CLI (ships with macOS) to avoid CGO dependencies.
func detectOpenCode(cwd, _ string) *Session {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	dbPath := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}

	// Use the sqlite3 CLI to query the database in read-only mode.
	query := `SELECT id FROM session WHERE directory = '` + escapeSQLite(cwd) + `' ORDER BY time_updated DESC LIMIT 1;`
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sqlite3", "-readonly", dbPath, query).Output()
	if err != nil {
		return nil
	}

	sessionID := strings.TrimSpace(string(out))
	if sessionID == "" || !validSessionID.MatchString(sessionID) {
		return nil
	}

	return &Session{
		Tool:    "opencode",
		CWD:     cwd,
		Command: "opencode --session " + sessionID,
	}
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

// detectCodex finds the active session for a Codex instance by CWD.
// Codex 0.128+ stores sessions as JSONL files under ~/.codex/sessions/YYYY/MM/DD/.
// The first line of each file contains session metadata with "id" and "cwd".
// Falls back to the legacy rollout-*.json format in ~/.codex/sessions/ root.
func detectCodex(cwd, _ string) *Session {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	sessDir := filepath.Join(home, ".codex", "sessions")

	// Try new format first: dated subdirectories with .jsonl files.
	if s := detectCodexNew(sessDir, cwd); s != nil {
		return s
	}
	// Fall back to legacy format: rollout-*.json in root.
	return detectCodexLegacy(sessDir, cwd)
}

// detectCodexNew handles Codex 0.128+ sessions stored as .jsonl in dated subdirs.
// Each file's first line has type "session_meta" with payload.id and payload.cwd.
// Walks reverse-chronologically from today, bounded to 30 days, to avoid scanning
// the entire session history.
func detectCodexNew(sessDir, cwd string) *Session {
	now := time.Now()
	for daysBack := 0; daysBack < 30; daysBack++ {
		d := now.AddDate(0, 0, -daysBack)
		dayDir := filepath.Join(sessDir, d.Format("2006"), d.Format("01"), d.Format("02"))
		entries, err := os.ReadDir(dayDir)
		if err != nil {
			continue
		}

		// Collect rollout files with their mod times.
		type candidate struct {
			path    string
			modTime int64
		}
		var files []candidate
		for _, e := range entries {
			if e.IsDir() || !strings.HasPrefix(e.Name(), "rollout-") || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			files = append(files, candidate{
				path:    filepath.Join(dayDir, e.Name()),
				modTime: info.ModTime().UnixNano(),
			})
		}

		// Sort most recent first within the day.
		sort.Slice(files, func(i, j int) bool {
			return files[i].modTime > files[j].modTime
		})

		for _, f := range files {
			id, dir := readCodexJSONLMeta(f.path)
			if id != "" && dir == cwd && validSessionID.MatchString(id) {
				return &Session{
					Tool:    "codex",
					CWD:     cwd,
					Command: "codex resume " + id,
				}
			}
		}
	}
	return nil
}

// readCodexJSONLMeta reads the first line of a Codex JSONL session file
// and extracts the session ID and CWD from the session_meta payload.
func readCodexJSONLMeta(path string) (id, cwd string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return "", ""
	}

	var meta struct {
		Type    string `json:"type"`
		Payload struct {
			ID  string `json:"id"`
			CWD string `json:"cwd"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &meta); err != nil {
		return "", ""
	}
	if meta.Type != "session_meta" {
		return "", ""
	}
	return meta.Payload.ID, meta.Payload.CWD
}

// detectCodexLegacy handles pre-0.128 Codex sessions (rollout-*.json in root).
func detectCodexLegacy(sessDir, cwd string) *Session {
	matches, err := filepath.Glob(filepath.Join(sessDir, "rollout-*.json"))
	if err != nil || len(matches) == 0 {
		return nil
	}

	// Find the most recently modified rollout file.
	type fileInfo struct {
		path    string
		modTime int64
	}
	var files []fileInfo
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		files = append(files, fileInfo{path: m, modTime: info.ModTime().UnixNano()})
	}
	if len(files) == 0 {
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	data, err := os.ReadFile(files[0].path)
	if err != nil {
		return nil
	}

	var rollout struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
	}
	if err := json.Unmarshal(data, &rollout); err != nil || rollout.Session.ID == "" {
		return nil
	}
	if !validSessionID.MatchString(rollout.Session.ID) {
		return nil
	}

	return &Session{
		Tool:    "codex",
		CWD:     cwd,
		Command: "codex resume " + rollout.Session.ID,
	}
}

// detectAmp finds the active thread for a running Amp CLI instance.
//
// Each amp process holds an open write handle on its per-thread log file
// (~/.cache/amp/logs/threads/T-<id>.log) for the entire session lifetime.
// During AISessions() we batch-list every AI process's open files with a
// single lsof call (see batchProcessInfo) and seed the pid→thread map as
// a side effect, so by the time detectAmp runs it's a pure map lookup.
//
// The mapping is per-pid precise, so two amp instances sharing a CWD each
// resolve to their own thread.
func detectAmp(cwd, pid string) *Session {
	if pid == "" {
		return nil
	}
	id := ampCache.threadFor(pid)
	if id == "" || !validSessionID.MatchString(id) {
		return nil
	}
	return &Session{
		Tool:    "amp",
		CWD:     cwd,
		Command: "amp threads continue " + id,
	}
}

// ampCache holds the pid→threadID mapping for amp processes, populated as
// a side effect of the AISessions() lsof pass. Reset between invocations
// via resetAmpCache.
var ampCache = &ampThreadCache{}

type ampThreadCache struct {
	mu    sync.Mutex
	byPID map[string]string // pid → threadID
}

// resetAmpCache clears the cached pid→thread map. Called at the top of
// AISessions().
func resetAmpCache() {
	ampCache.mu.Lock()
	defer ampCache.mu.Unlock()
	ampCache.byPID = nil
}

// seedAmpThread records a (pid → threadID) mapping discovered during the
// lsof pass.
func seedAmpThread(pid, threadID string) {
	ampCache.mu.Lock()
	defer ampCache.mu.Unlock()
	if ampCache.byPID == nil {
		ampCache.byPID = make(map[string]string)
	}
	ampCache.byPID[pid] = threadID
}

// threadFor returns the thread ID for pid, or "" if none is known.
func (c *ampThreadCache) threadFor(pid string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.byPID[pid]
}

// ampLogPathPrefix and ampLogPathSuffix bracket the per-thread log path
// `.../logs/threads/T-<id>.log` as it appears in lsof output. We extract
// the thread ID by stripping these — no regex needed.
const (
	ampLogPathPrefix = "/logs/threads/"
	ampLogPathSuffix = ".log"
)

// threadIDFromLogPath returns the T-<id> suffix of a per-thread log path
// or "" if the path isn't one.
func threadIDFromLogPath(path string) string {
	i := strings.LastIndex(path, ampLogPathPrefix)
	if i < 0 {
		return ""
	}
	name := path[i+len(ampLogPathPrefix):]
	if !strings.HasSuffix(name, ampLogPathSuffix) {
		return ""
	}
	id := strings.TrimSuffix(name, ampLogPathSuffix)
	if !strings.HasPrefix(id, "T-") {
		return ""
	}
	return id
}
