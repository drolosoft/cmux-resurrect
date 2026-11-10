package detect

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// qwenRuntimeDirVariable moves Qwen Code's runtime output, sessions
// included, and wins over every other setting (getRuntimeBaseDir in its
// packages/core/src/config/storage.ts).
const qwenRuntimeDirVariable = "QWEN_RUNTIME_DIR"

// qwenHomeVariable replaces ~/.qwen as a whole (getGlobalQwenDir in the same
// file). Without QWEN_RUNTIME_DIR the sessions live under it too.
const qwenHomeVariable = "QWEN_HOME"

// qwenHeadLines is how many records are read before giving up on finding the
// session's source. Qwen Code writes it near the start of the transcript
// (recordSessionSource) and reads 10 lines to list a session
// (MAX_PROMPT_SCAN_LINES in sessionService.ts).
const qwenHeadLines = 10

// qwenHeadBytes caps what is read from one transcript. Records carry whole
// messages, so a pasted file can make one line large; a line cut by the cap
// does not decode and is skipped.
const qwenHeadBytes = 1 << 20

// qwenSessionFile matches the transcripts Qwen Code lists and resumes
// (SESSION_FILE_PATTERN in sessionService.ts). Sidecars such as
// <id>.ledger.jsonl share the folder and do not match, and the id it leaves
// has no character a shell would read.
var qwenSessionFile = regexp.MustCompile(`^[0-9a-fA-F-]{32,36}\.jsonl$`)

// qwenNonConversationSubtypes are the system records that are not part of a
// conversation (ARTIFACT_RECORD_SUBTYPES and NON_CONVERSATION_RECORD_SUBTYPES
// in packages/core/src/utils/transcript-records.ts). A transcript holding only
// these loads as empty, and `qwen --resume <id>` exits with "No saved session
// found".
var qwenNonConversationSubtypes = map[string]bool{
	"session_artifact_event":           true,
	"session_artifact_snapshot":        true,
	"session_sources_snapshot":         true,
	"managed_session_header_v1":        true,
	"managed_session_event_v1":         true,
	"managed_session_commit_v1":        true,
	"managed_session_snapshot_base_v1": true,
}

// qwenHiddenSources are the session sources of sessions a machine owns: an
// agent host and each agent's tasks (HIDDEN_CATALOG_SOURCE_TYPES in
// packages/cli/src/runtime/agent-session-source.ts). `qwen --continue`
// passes them over, and a terminal pane never runs one.
var qwenHiddenSources = map[string]bool{
	"agent":      true,
	"agent-host": true,
}

// qwenRecordTypes are the record types Qwen Code accepts when it rebuilds a
// conversation (RECORD_TYPES in transcript-records.ts).
var qwenRecordTypes = map[string]bool{
	"user":        true,
	"assistant":   true,
	"tool_result": true,
	"system":      true,
}

// detectQwen finds the session of a Qwen Code process. Qwen Code keeps one
// transcript per session in <base>/projects/<encoded cwd>/chats/<id>.jsonl,
// where base is ~/.qwen unless the process moves it, and the folder is the
// cwd with every non-alphanumeric character as "-". The newest transcript
// that starts in this cwd and holds conversation is resumed. Without one,
// Qwen Code starts fresh, as it did before crex could resume it.
func detectQwen(cwd, pid string) *Session {
	fresh := &Session{Tool: "qwen", CWD: cwd, Command: "qwen"}

	baseDir, prefix := qwenRuntimeDir(pid)
	if baseDir == "" {
		return fresh
	}

	chatsDir := filepath.Join(baseDir, "projects", nonAlnum.ReplaceAllString(cwd, "-"), "chats")
	sessionID := latestQwenSession(chatsDir, cwd)
	if sessionID == "" {
		return fresh
	}

	return &Session{Tool: "qwen", CWD: cwd, Command: prefix + "qwen --resume " + sessionID}
}

// qwenRuntimeDir returns the folder a Qwen Code process keeps its sessions
// under, and the variable assignment the resume command must carry when that
// folder is not ~/.qwen: `qwen --resume <id>` only looks under its own base.
// QWEN_RUNTIME_DIR comes first, then QWEN_HOME, as in Qwen Code.
func qwenRuntimeDir(pid string) (dir, prefix string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	defaultDir := filepath.Join(home, ".qwen")
	environment := processEnv(pid)

	// Qwen Code reads a relative value against the directory it was started
	// from, which crex cannot know for sure; such a value is passed over.
	for _, name := range []string{qwenRuntimeDirVariable, qwenHomeVariable} {
		customDir := expandUserPath(environment[name], home)
		if customDir == "" {
			continue
		}
		if customDir == defaultDir {
			return defaultDir, ""
		}
		return customDir, name + "=" + shellQuote(customDir) + " "
	}

	return defaultDir, ""
}

// latestQwenSession returns the ID of the most recently modified Qwen Code
// session in chatsDir that `qwen --resume` would open for cwd, or "" if none
// qualifies. Archived sessions sit in chatsDir/archive/ and are skipped with
// the folder. The folder name alone is not enough: /w/acme/foo-bar and
// /w/acme-foo/bar both become -w-acme-foo-bar, so the transcript has to name
// cwd itself.
func latestQwenSession(chatsDir, cwd string) string {
	for _, path := range sessionFilesNewestFirst(chatsDir) {
		name := filepath.Base(path)
		if !qwenSessionFile.MatchString(name) {
			continue
		}

		head := readQwenTranscriptHead(path)
		if head.cwd != cwd || !head.conversation || head.hidden {
			continue
		}
		return strings.TrimSuffix(name, ".jsonl")
	}
	return ""
}

// qwenTranscriptHead is what the first records of a transcript tell about the
// session.
type qwenTranscriptHead struct {
	cwd          string // the cwd of the first record, which Qwen Code checks
	conversation bool   // some record is conversation, so the session loads
	hidden       bool   // a machine owns the session
}

// qwenRecord is the part of a transcript line the detector reads. The payload
// stays raw because its shape depends on the subtype, and a payload that is
// not an object must not make the whole line unreadable.
type qwenRecord struct {
	UUID          string          `json:"uuid"`
	SessionID     string          `json:"sessionId"`
	Type          string          `json:"type"`
	Subtype       string          `json:"subtype"`
	CWD           string          `json:"cwd"`
	SystemPayload json.RawMessage `json:"systemPayload"`
}

// readQwenTranscriptHead reads the start of a Qwen Code transcript. It stops
// once qwenHeadLines records have been read and one of them is conversation,
// so a long transcript is barely touched. An empty or unreadable file gives
// an empty head, which no cwd matches.
func readQwenTranscriptHead(path string) qwenTranscriptHead {
	var head qwenTranscriptHead

	file, err := os.Open(path)
	if err != nil {
		return head
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReader(io.LimitReader(file, qwenHeadBytes))
	records := 0
	for records < qwenHeadLines || !head.conversation {
		line, readErr := reader.ReadBytes('\n')

		var record qwenRecord
		if len(bytes.TrimSpace(line)) > 0 && json.Unmarshal(line, &record) == nil {
			records++

			// loadSession checks the cwd of the first record only, and a
			// first record from another directory means the session is not
			// this one's.
			if records == 1 {
				head.cwd = record.CWD
				if head.cwd == "" {
					return head
				}
			}

			if record.conversation() {
				head.conversation = true
			}
			if qwenHiddenSources[record.sourceType()] {
				head.hidden = true
			}
		}

		if readErr != nil {
			break
		}
	}
	return head
}

// conversation reports whether a record counts when Qwen Code rebuilds the
// conversation (validateTranscriptRecord and isTranscriptConversationRecord in
// transcript-records.ts): it has an identity, a known type, and is not one of
// the system records kept beside the conversation.
func (record qwenRecord) conversation() bool {
	if record.UUID == "" || record.SessionID == "" || !qwenRecordTypes[record.Type] {
		return false
	}
	return record.Type != "system" || !qwenNonConversationSubtypes[record.Subtype]
}

// sourceType returns who created the session, from a session_source record,
// or "" for any other record.
func (record qwenRecord) sourceType() string {
	if record.Type != "system" || record.Subtype != "session_source" {
		return ""
	}

	var payload struct {
		SourceType string `json:"sourceType"`
	}
	if json.Unmarshal(record.SystemPayload, &payload) != nil {
		return ""
	}
	return payload.SourceType
}
