package detect

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// geminiProjectHash returns the SHA-256 hex digest of a directory path.
func geminiProjectHash(dir string) string {
	h := sha256.Sum256([]byte(dir))
	return fmt.Sprintf("%x", h)
}

// detectGemini finds the active session for a Gemini CLI instance by CWD.
// Sessions are stored as JSON files in ~/.gemini/tmp/<sha256-of-cwd>/chats/.
func detectGemini(cwd, _ string) *Session {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	hash := geminiProjectHash(cwd)
	chatsDir := filepath.Join(home, ".gemini", "tmp", hash, "chats")

	entries, err := os.ReadDir(chatsDir)
	if err != nil {
		return nil
	}

	type fileInfo struct {
		name    string
		modTime int64
	}
	var sessions []fileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, fileInfo{name: e.Name(), modTime: info.ModTime().UnixNano()})
	}
	if len(sessions) == 0 {
		return nil
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].modTime > sessions[j].modTime
	})

	// Extract session ID from filename: session-YYYY-MM-DDTHH-mm-<id>.json
	// Splitting "session-2026-05-27T14-30-<id>" on "-" yields 6 fields;
	// the session ID is always the last one.
	name := strings.TrimSuffix(sessions[0].name, ".json")
	parts := strings.Split(name, "-")
	if len(parts) < 6 {
		return nil
	}
	sessionID := parts[len(parts)-1]
	if sessionID == "" || !validSessionID.MatchString(sessionID) {
		return nil
	}

	return &Session{
		Tool:    "gemini",
		CWD:     cwd,
		Command: "gemini --resume " + sessionID,
	}
}
