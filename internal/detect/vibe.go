package detect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// vibeHomeVariable moves Vibe's whole home, sessions included (get_vibe_home
// in Vibe's vibe/utils/vibe_home.py). The default is ~/.vibe.
const vibeHomeVariable = "VIBE_HOME"

// vibeSaveDirVariable moves only the session folder. Vibe reads its settings
// from VIBE_* variables with "__" between nested keys
// (vibe/core/config/layers/environment.py), and this one is save_dir of the
// session_logging group. Config files can move it too; crex only honours the
// environment, as for the other tools.
const vibeSaveDirVariable = "VIBE_SESSION_LOGGING__SAVE_DIR"

// vibeLegacyPrefix starts the name of every session folder Vibe 2.x wrote:
// session_<YYYYMMDD_HHMMSS>_<first 8 characters of the id>. The prefix is
// configurable, but a folder under another one is invisible to a `vibe
// --resume` started without that setting, so it is not offered.
const vibeLegacyPrefix = "session_"

// vibeUnifiedFolder is where Vibe 3.x keeps one store per session, inside the
// session folder (UnifiedSessionStore in Vibe's harness _storage.py).
const vibeUnifiedFolder = "unified"

// vibeSession is one session found in the session folder, before its
// directory has been checked.
type vibeSession struct {
	// folder holds the session: a Vibe 2.x folder with meta.json, or a Vibe
	// 3.x store with a CURRENT pointer.
	folder string
	// generation names the store's current generation; it is empty for a
	// Vibe 2.x session.
	generation string
	// modTime is when the session was last written to, in nanoseconds.
	modTime int64
}

// detectVibe finds the session a Mistral Vibe process left in its directory.
//
// Vibe keeps its sessions in <vibe home>/logs/session. Vibe 3.x writes one
// store per session in unified/<id>/; the sessions of Vibe 2.x stay beside it
// as session_<time>_<short id>/ folders with a meta.json, and Vibe 3.x still
// resumes them. The newest session of either kind whose recorded directory
// is cwd is resumed with `vibe --resume <id>`, which works from any directory.
// `vibe -c` is not used: it prefers the last session of the terminal it runs
// in, and a restored pane is a new terminal. Without a session, Vibe is
// started fresh, as it was before crex could resume it.
func detectVibe(cwd, pid string) *Session {
	fresh := &Session{Tool: "vibe", CWD: cwd, Command: "vibe"}

	sessionDir, prefix := vibeSessionLocation(pid)
	if sessionDir == "" {
		return fresh
	}

	sessionID := newestVibeSession(sessionDir, vibeCwdForms(cwd))
	if sessionID == "" {
		return fresh
	}

	return &Session{Tool: "vibe", CWD: cwd, Command: prefix + "vibe --resume " + sessionID}
}

// vibeSessionLocation returns the folder where the Vibe process keeps its
// sessions, and the variable assignments the resume command must carry when
// that folder is not the default one: `vibe --resume <id>` only looks in the
// folder its own environment points at. It returns "" when a variable holds
// a relative path, which Vibe resolves against the directory it was started
// from; guessing that could resume a session of another project.
func vibeSessionLocation(pid string) (sessionDir, prefix string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	environment := processEnv(pid)

	vibeHome := filepath.Join(home, ".vibe")
	if value := vibeVariable(environment, vibeHomeVariable); value != "" {
		customHome := expandUserPath(value, home)
		if customHome == "" {
			return "", ""
		}
		if customHome != vibeHome {
			vibeHome = customHome
			prefix = vibeHomeVariable + "=" + shellQuote(customHome) + " "
		}
	}

	sessionDir = filepath.Join(vibeHome, "logs", "session")
	if value := vibeVariable(environment, vibeSaveDirVariable); value != "" {
		customDir := expandUserPath(value, home)
		if customDir == "" {
			return "", ""
		}
		if customDir != sessionDir {
			sessionDir = customDir
			prefix += vibeSaveDirVariable + "=" + shellQuote(customDir) + " "
		}
	}

	return sessionDir, prefix
}

// vibeVariable reads one variable from the environment the Vibe process was
// started with. When crex cannot read that environment (no pid, or a process
// it may not inspect) it uses its own, which is what a Vibe started from the
// same shell sees too. Surrounding spaces are dropped, and an empty value
// means unset, as Vibe treats it.
func vibeVariable(environment map[string]string, name string) string {
	if environment == nil {
		return strings.TrimSpace(os.Getenv(name))
	}
	return strings.TrimSpace(environment[name])
}

// vibeCwdForms returns the paths Vibe may have recorded for cwd. Vibe stores
// the directory it was started in and compares resolved paths when it looks
// for a session, so the real path, with symlinks resolved, counts as well.
func vibeCwdForms(cwd string) []string {
	cleaned := filepath.Clean(cwd)

	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil || resolved == cleaned {
		return []string{cleaned}
	}
	return []string{cleaned, resolved}
}

// newestVibeSession returns the id of the most recently used session in
// sessionDir that was opened in, or now sits in, one of cwds, or "" when
// there is none. Sessions are sorted first and opened newest first, so the
// usual case reads one or two small files.
func newestVibeSession(sessionDir string, cwds []string) string {
	sessions := append(vibeLegacySessions(sessionDir), vibeUnifiedSessions(sessionDir)...)

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].modTime > sessions[j].modTime
	})

	for _, session := range sessions {
		var sessionID string
		var recorded []string
		if session.generation == "" {
			sessionID, recorded = readVibeLegacySession(session.folder)
		} else {
			sessionID, recorded = readVibeUnifiedSession(session.folder, session.generation)
		}

		// The id is typed into a terminal on restore, so a session that
		// names something else than a plain id is passed over.
		if !validSessionID.MatchString(sessionID) {
			continue
		}
		if vibeRecordedIn(recorded, cwds) {
			return sessionID
		}
	}

	return ""
}

// vibeRecordedIn reports whether one of the directories a session recorded
// is one of cwds.
func vibeRecordedIn(recorded, cwds []string) bool {
	for _, directory := range recorded {
		if directory == "" {
			continue
		}
		for _, cwd := range cwds {
			if filepath.Clean(directory) == cwd {
				return true
			}
		}
	}
	return false
}

// vibeLegacySessions lists the Vibe 2.x session folders in sessionDir. Like
// latest_session in Vibe's session_loader.py, a session is as recent as its
// messages.jsonl, and a folder without one, or with an empty one, is not a
// session Vibe would resume.
func vibeLegacySessions(sessionDir string) []vibeSession {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil
	}

	var sessions []vibeSession
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), vibeLegacyPrefix) {
			continue
		}

		folder := filepath.Join(sessionDir, entry.Name())
		info, err := os.Stat(filepath.Join(folder, "messages.jsonl"))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			continue
		}
		sessions = append(sessions, vibeSession{folder: folder, modTime: info.ModTime().UnixNano()})
	}
	return sessions
}

// readVibeLegacySession returns the id and the directories recorded in a
// Vibe 2.x meta.json: environment.working_directory, where the session is
// now, and origin_directory, where it began. Vibe offers a session from both.
// A file that is missing or is not valid JSON gives nothing.
func readVibeLegacySession(folder string) (sessionID string, directories []string) {
	data, err := os.ReadFile(filepath.Join(folder, "meta.json"))
	if err != nil {
		return "", nil
	}

	var metadata struct {
		SessionID   string `json:"session_id"`
		Environment struct {
			WorkingDirectory string `json:"working_directory"`
		} `json:"environment"`
		OriginDirectory string `json:"origin_directory"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return "", nil
	}

	return metadata.SessionID, []string{metadata.Environment.WorkingDirectory, metadata.OriginDirectory}
}

// vibeUnifiedSessions lists the Vibe 3.x session stores in sessionDir. A
// store is as recent as the later of its CURRENT pointer, replaced at each new
// generation, and its open recovery journal, which grows between them; the
// journal is named after the sequence that follows the pointer's.
func vibeUnifiedSessions(sessionDir string) []vibeSession {
	unifiedDir := filepath.Join(sessionDir, vibeUnifiedFolder)
	entries, err := os.ReadDir(unifiedDir)
	if err != nil {
		return nil
	}

	var sessions []vibeSession
	for _, entry := range entries {
		// ReadDir does not follow links, and Vibe refuses a store reached
		// through one, so a link is not a directory here either.
		if !entry.IsDir() {
			continue
		}

		folder := filepath.Join(unifiedDir, entry.Name())
		currentPath := filepath.Join(folder, "CURRENT")
		currentInfo, err := os.Stat(currentPath)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(currentPath)
		if err != nil {
			continue
		}

		var pointer struct {
			Generation       string `json:"generation"`
			SnapshotSequence int64  `json:"snapshot_sequence"`
		}
		// The generation becomes part of a path, so it has to be a plain
		// name.
		if json.Unmarshal(data, &pointer) != nil || !validSessionID.MatchString(pointer.Generation) {
			continue
		}

		modified := currentInfo.ModTime().UnixNano()
		journalPath := filepath.Join(folder, "journal", fmt.Sprintf("%016d.jsonl", pointer.SnapshotSequence+1))
		if journalInfo, err := os.Stat(journalPath); err == nil && journalInfo.ModTime().UnixNano() > modified {
			modified = journalInfo.ModTime().UnixNano()
		}

		sessions = append(sessions, vibeSession{folder: folder, generation: pointer.Generation, modTime: modified})
	}
	return sessions
}

// readVibeUnifiedSession returns the id and the directories recorded in the
// runtime state of a Vibe 3.x store's current generation: session_metadata.cwd,
// where the session is now, and origin_cwd, where it began when it has moved.
//
// A subagent has a store of its own in the same directory, but it is the
// parent's helper, not a conversation to come back to, and an ephemeral store
// is gone once its run ends; neither gives an id. The id must also name the
// store's own folder, since that is what `vibe --resume` looks up.
func readVibeUnifiedSession(folder, generation string) (sessionID string, directories []string) {
	data, err := os.ReadFile(filepath.Join(folder, "generations", generation, "runtime-state.json"))
	if err != nil {
		return "", nil
	}

	var state struct {
		SessionID       string `json:"session_id"`
		StorageLifetime string `json:"storage_lifetime"`
		Identity        struct {
			Kind string `json:"kind"`
		} `json:"identity"`
		SessionMetadata struct {
			CWD       string `json:"cwd"`
			OriginCWD string `json:"origin_cwd"`
		} `json:"session_metadata"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return "", nil
	}

	if state.Identity.Kind == "subagent" || state.StorageLifetime == "ephemeral" {
		return "", nil
	}
	if state.SessionID != filepath.Base(folder) {
		return "", nil
	}

	return state.SessionID, []string{state.SessionMetadata.CWD, state.SessionMetadata.OriginCWD}
}
