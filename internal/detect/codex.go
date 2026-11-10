package detect

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

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
	defer func() { _ = f.Close() }()

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
