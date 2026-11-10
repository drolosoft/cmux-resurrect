package detect

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// codexStateFile is the SQLite database where Codex 0.128 keeps one row per
// conversation in the threads table. The number is Codex's own schema
// generation; a future state_6 is not guessed at, and the detector falls
// back to the rollout files until it is known.
const codexStateFile = "state_5.sqlite"

// codexThreadCandidates is how many threads of a directory are fetched from
// the state database. More than one is needed to step over threads whose
// rollout file was deleted, which Codex can no longer resume.
const codexThreadCandidates = 20

// detectCodex finds the session of a Codex instance by CWD, never one from
// another directory: no resume is better than someone else's conversation.
// The Codex home is $CODEX_HOME (default ~/.codex). The sources, in order:
//
//   - state_5.sqlite, table threads (Codex 0.128+): the newest thread of the
//     directory that is not archived. When it can be read, it is the answer,
//     because only the database knows which threads are archived.
//   - sessions/YYYY/MM/DD/rollout-*.jsonl: the most recently written file
//     whose session_meta names the directory, however old.
//   - sessions/rollout-*.json (before 0.128): only a file that carries the
//     directory and matches it. Real ones carry none, so they are not used.
func detectCodex(cwd, pid string) *Session {
	codexHome, custom := codexHomeDir(pid)
	if codexHome == "" {
		return nil
	}

	sessionID, consulted := codexStateThread(codexHome, cwd)
	if !consulted {
		sessDir := filepath.Join(codexHome, "sessions")
		sessionID = latestCodexRollout(sessDir, cwd)
		if sessionID == "" {
			sessionID = latestCodexLegacyRollout(sessDir, cwd)
		}
	}
	if sessionID == "" || !validSessionID.MatchString(sessionID) {
		return nil
	}

	return &Session{
		Tool:    "codex",
		CWD:     cwd,
		Command: codexResumeCommand(codexHome, custom, sessionID),
	}
}

// codexHomeDir returns the Codex home a process uses: $CODEX_HOME from the
// process environment, or from crex's own when that cannot be read, and
// ~/.codex otherwise. custom reports whether it differs from the default,
// in which case the resume command must set it again.
func codexHomeDir(pid string) (dir string, custom bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	def := filepath.Join(home, ".codex")

	// A readable environment without CODEX_HOME means the default. Only
	// when it cannot be read at all is crex's own environment a better
	// guess than nothing.
	var value string
	if env := processEnv(pid); env != nil {
		value = env["CODEX_HOME"]
	} else {
		value = os.Getenv("CODEX_HOME")
	}
	if value == "" || !filepath.IsAbs(value) {
		return def, false
	}

	value = filepath.Clean(value)
	return value, value != def
}

// codexResumeCommand builds the command that resumes a Codex session,
// setting CODEX_HOME first when the session lives outside ~/.codex.
func codexResumeCommand(codexHome string, custom bool, sessionID string) string {
	cmd := "codex resume " + sessionID
	if custom {
		cmd = "CODEX_HOME=" + shellQuote(codexHome) + " " + cmd
	}
	return cmd
}

// codexStateThread asks the Codex state database for the newest thread of
// cwd that is not archived and whose rollout file still exists. consulted
// is false when there is no database or sqlite3 cannot read it (missing
// CLI, one older than 3.33 without -json, another schema, a WAL it cannot
// open read-only); the caller then
// looks at the rollout files instead. A database that was read but has no
// such thread returns ("", true).
func codexStateThread(codexHome, cwd string) (sessionID string, consulted bool) {
	dbPath := filepath.Join(codexHome, codexStateFile)
	if _, err := os.Stat(dbPath); err != nil {
		return "", false
	}

	// -readonly so a running Codex is never disturbed, and -json because a
	// rollout path may hold the "|" that the default list mode separates
	// columns with.
	query := `SELECT id, rollout_path FROM threads WHERE cwd = '` + escapeSQLite(cwd) +
		`' AND archived = 0 ORDER BY updated_at DESC, id DESC LIMIT ` + strconv.Itoa(codexThreadCandidates) + `;`
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sqlite3", "-readonly", "-json", dbPath, query).Output()
	if err != nil {
		return "", false
	}

	// sqlite3 prints nothing at all, not "[]", when no row matches.
	if strings.TrimSpace(string(out)) == "" {
		return "", true
	}
	var threads []struct {
		ID          string `json:"id"`
		RolloutPath string `json:"rollout_path"`
	}
	if err := json.Unmarshal(out, &threads); err != nil {
		return "", false
	}

	for _, thread := range threads {
		rollout := thread.RolloutPath
		if !filepath.IsAbs(rollout) {
			rollout = filepath.Join(codexHome, rollout)
		}
		if _, err := os.Stat(rollout); err != nil {
			continue
		}
		return thread.ID, true
	}
	return "", true
}

// latestCodexRollout returns the ID of the most recently written Codex
// 0.128 rollout file (sessions/YYYY/MM/DD/rollout-*.jsonl) whose
// session_meta names cwd, or "" if none does. The whole tree is listed with
// no cut by age, and files are ordered by modification time rather than by
// the day folder, because a conversation started months ago and continued
// today lives in the folder of the day it started.
func latestCodexRollout(sessDir, cwd string) string {
	type rolloutFile struct {
		path    string
		modTime int64
	}
	var files []rolloutFile

	// Errors on one entry only skip that entry; the walk is best-effort
	// like the rest of the package.
	_ = filepath.WalkDir(sessDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, ".jsonl") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		files = append(files, rolloutFile{path: path, modTime: info.ModTime().UnixNano()})
		return nil
	})

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	for _, file := range files {
		id, dir := readCodexJSONLMeta(file.path)
		if id != "" && dir == cwd && validSessionID.MatchString(id) {
			return id
		}
	}
	return ""
}

// readCodexJSONLMeta reads the first record of a Codex JSONL session file
// and extracts the session ID and CWD from its session_meta payload. The
// record is decoded as a JSON value instead of scanned as a line, so its
// size has no cap: it carries the whole base instructions (21.8 KB with
// Codex 0.128) and a line scanner would drop it without a word once it
// grew past its buffer.
func readCodexJSONLMeta(path string) (id, cwd string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer func() { _ = f.Close() }()

	var meta struct {
		Type    string `json:"type"`
		Payload struct {
			ID  string `json:"id"`
			CWD string `json:"cwd"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(f).Decode(&meta); err != nil {
		return "", ""
	}
	if meta.Type != "session_meta" {
		return "", ""
	}
	return meta.Payload.ID, meta.Payload.CWD
}

// latestCodexLegacyRollout handles pre-0.128 Codex sessions, stored as
// sessions/rollout-*.json. It returns the ID of the most recently written
// file that names cwd, or "" if none does. The files Codex wrote carry only
// session.{timestamp,id,instructions}: with no directory to compare, they
// are skipped rather than handed to whatever pane asks first.
func latestCodexLegacyRollout(sessDir, cwd string) string {
	matches, err := filepath.Glob(filepath.Join(sessDir, "rollout-*.json"))
	if err != nil || len(matches) == 0 {
		return ""
	}

	type rolloutFile struct {
		path    string
		modTime int64
	}
	var files []rolloutFile
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		files = append(files, rolloutFile{path: m, modTime: info.ModTime().UnixNano()})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	for _, file := range files {
		data, err := os.ReadFile(file.path)
		if err != nil {
			continue
		}
		var rollout struct {
			Session struct {
				ID  string `json:"id"`
				CWD string `json:"cwd"`
			} `json:"session"`
		}
		if err := json.Unmarshal(data, &rollout); err != nil {
			continue
		}
		if rollout.Session.CWD != cwd || !validSessionID.MatchString(rollout.Session.ID) {
			continue
		}
		return rollout.Session.ID
	}
	return ""
}
