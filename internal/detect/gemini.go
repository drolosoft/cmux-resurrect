package detect

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// geminiResumeLatest is the argument Gemini CLI accepts instead of a UUID to
// resume the newest conversation of the directory it starts in. It is the
// fallback when a session file exists but its UUID cannot be read.
const geminiResumeLatest = "latest"

// geminiSessionPrefix starts the name of every top-level session file
// (SESSION_FILE_PREFIX in Gemini CLI's chatRecordingTypes.ts).
const geminiSessionPrefix = "session-"

// geminiSlug matches the project identifiers Gemini CLI accepts in
// projects.json (registryDataSchema in projectRegistry.ts). Checking it
// keeps a damaged registry from sending the lookup outside ~/.gemini/tmp.
var geminiSlug = regexp.MustCompile(`^[a-z0-9-]+$`)

// geminiIgnoredPrefixes start the user messages that are not conversation:
// commands, help and context Gemini CLI injects itself. A session holding
// only these cannot be resumed (isIgnoredUserContent in Gemini CLI's
// packages/core/src/utils/sessionUtils.ts).
var geminiIgnoredPrefixes = []string{"/", "?", "<session_context>", "<hook_context>"}

// geminiRecord is the part of a session record the detector reads. The
// first line of a .jsonl session is the metadata (sessionId, kind); the
// following ones are messages (type, content, toolCalls, thoughts) or
// updates that do not matter here.
type geminiRecord struct {
	SessionID string            `json:"sessionId"`
	Kind      string            `json:"kind"`
	Type      string            `json:"type"`
	Content   json.RawMessage   `json:"content"`
	ToolCalls []json.RawMessage `json:"toolCalls"`
	Thoughts  []json.RawMessage `json:"thoughts"`
}

// geminiProjectHash returns the SHA-256 hex digest of a directory path,
// which named the project folder before the project registry.
func geminiProjectHash(dir string) string {
	h := sha256.Sum256([]byte(dir))
	return fmt.Sprintf("%x", h)
}

// detectGemini finds the session of a Gemini CLI process by its working
// directory. Sessions live in <gemini>/tmp/<slug>/chats/, where the slug is
// the one ~/.gemini/projects.json gives the directory; the SHA-256 folder of
// older releases is read too. The full session UUID comes from inside the
// newest file with conversation in it, since the file name carries only its
// first 8 characters and --resume refuses those.
func detectGemini(cwd, pid string) *Session {
	geminiDir := geminiGlobalDir(pid)
	if geminiDir == "" {
		return nil
	}

	for _, chatsDir := range geminiChatDirs(geminiDir, cwd) {
		sessionID, unreadable := latestGeminiSession(chatsDir)

		// A session exists but its UUID could not be read: "latest" lets
		// Gemini CLI pick the newest conversation of that same directory.
		if sessionID == "" && unreadable {
			sessionID = geminiResumeLatest
		}
		if sessionID == "" {
			continue
		}

		return &Session{
			Tool:    "gemini",
			CWD:     cwd,
			Command: "gemini --resume " + sessionID,
		}
	}
	return nil
}

// geminiGlobalDir returns the folder where Gemini CLI keeps its state:
// <home>/.gemini, where home is the process's GEMINI_CLI_HOME when it sets
// one (homedir in Gemini CLI's paths.ts), or the user's home otherwise.
func geminiGlobalDir(pid string) string {
	home := processEnv(pid)["GEMINI_CLI_HOME"]
	if home == "" || !filepath.IsAbs(home) {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		home = userHome
	}

	return filepath.Join(home, ".gemini")
}

// geminiChatDirs lists the chats folders that may hold the sessions of cwd:
// the one named by its slug in the project registry first, then the
// SHA-256 one that Gemini CLI migrates away from on its next start.
func geminiChatDirs(geminiDir, cwd string) []string {
	tempDir := filepath.Join(geminiDir, "tmp")

	var dirs []string
	if slug := geminiProjectSlug(geminiDir, cwd); slug != "" {
		dirs = append(dirs, filepath.Join(tempDir, slug, "chats"))
	}
	dirs = append(dirs, filepath.Join(tempDir, geminiProjectHash(cwd), "chats"))
	return dirs
}

// geminiProjectSlug returns the short identifier projects.json gives cwd,
// or "" when the registry is missing, damaged or does not know cwd. The
// registry is keyed by the resolved absolute path of the project.
func geminiProjectSlug(geminiDir, cwd string) string {
	data, err := os.ReadFile(filepath.Join(geminiDir, "projects.json"))
	if err != nil {
		return ""
	}

	var registry struct {
		Projects map[string]string `json:"projects"`
	}
	if json.Unmarshal(data, &registry) != nil {
		return ""
	}

	slug := registry.Projects[filepath.Clean(cwd)]
	if !geminiSlug.MatchString(slug) {
		return ""
	}
	return slug
}

// latestGeminiSession returns the UUID of the most recently modified
// session in chatsDir that Gemini CLI can resume. unreadable reports that
// some session file had no usable UUID, so the caller can still resume the
// directory's latest conversation when no UUID is found.
func latestGeminiSession(chatsDir string) (sessionID string, unreadable bool) {
	entries, err := os.ReadDir(chatsDir)
	if err != nil {
		return "", false
	}

	// Subagent sessions live in subfolders and are skipped with them.
	type sessionFile struct {
		path    string
		modTime int64
	}
	var files []sessionFile
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, geminiSessionPrefix) {
			continue
		}
		if !strings.HasSuffix(name, ".jsonl") && !strings.HasSuffix(name, ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, sessionFile{path: filepath.Join(chatsDir, name), modTime: info.ModTime().UnixNano()})
	}

	// A resumed conversation keeps writing to its old file, so the
	// modification time, not the name, tells which one is in use.
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	for _, file := range files {
		id, resumable := readGeminiSession(file.path)
		if id == "" || !validSessionID.MatchString(id) {
			unreadable = true
			continue
		}
		if resumable {
			return id, unreadable
		}
	}
	return "", unreadable
}

// readGeminiSession returns the session UUID stored in a session file and
// whether it holds conversation Gemini CLI offers to resume. Subagent
// sessions are never resumable. Files ending in .json use the older
// single-document format.
func readGeminiSession(path string) (sessionID string, resumable bool) {
	if strings.HasSuffix(path, ".json") {
		return readGeminiLegacySession(path)
	}

	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer func() { _ = file.Close() }()

	// The metadata is the first record and a prompt usually follows within
	// a few lines, so reading stops as soon as both are known.
	subagent := false
	reader := bufio.NewReader(file)
	for {
		line, readErr := reader.ReadBytes('\n')

		var record geminiRecord
		if json.Unmarshal(line, &record) == nil {
			if sessionID == "" && record.SessionID != "" {
				sessionID = record.SessionID
				subagent = record.Kind == "subagent"
			}
			if record.resumable() {
				resumable = true
			}
		}

		if readErr != nil || (sessionID != "" && resumable) {
			break
		}
	}

	return sessionID, resumable && !subagent
}

// readGeminiLegacySession reads a session saved by older Gemini CLI
// releases: one JSON document with the metadata and a messages array.
func readGeminiLegacySession(path string) (sessionID string, resumable bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var conversation struct {
		SessionID string         `json:"sessionId"`
		Kind      string         `json:"kind"`
		Messages  []geminiRecord `json:"messages"`
	}
	if json.Unmarshal(data, &conversation) != nil {
		return "", false
	}

	for _, message := range conversation.Messages {
		if message.resumable() {
			resumable = true
			break
		}
	}
	return conversation.SessionID, resumable && conversation.Kind != "subagent"
}

// resumable reports whether a message record is conversation, following
// isResumableMessageRecord in Gemini CLI's chatRecordingService.ts: a user
// prompt that is not a command or injected context, or a model turn with
// text, tool calls or thoughts.
func (record geminiRecord) resumable() bool {
	text := strings.TrimSpace(geminiContentText(record.Content))

	switch record.Type {
	case "user":
		if text == "" {
			return false
		}
		for _, prefix := range geminiIgnoredPrefixes {
			if strings.HasPrefix(text, prefix) {
				return false
			}
		}
		return true
	case "gemini":
		return text != "" || len(record.ToolCalls) > 0 || len(record.Thoughts) > 0
	}
	return false
}

// geminiContentText joins the text of a message's content. Gemini CLI
// stores it as a PartListUnion: a string, a part, or a list of strings and
// parts. Parts without text, such as function calls, add nothing.
func geminiContentText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}

	var parts []json.RawMessage
	if json.Unmarshal(raw, &parts) != nil {
		parts = []json.RawMessage{raw}
	}

	var builder strings.Builder
	for _, part := range parts {
		var bare string
		if json.Unmarshal(part, &bare) == nil {
			builder.WriteString(bare)
			continue
		}
		var textPart struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(part, &textPart) == nil {
			builder.WriteString(textPart.Text)
		}
	}
	return builder.String()
}
