package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// rovoCommand starts Rovo Dev from a shell. It is the same command the
// registry entry relaunches with, so a folder without a session for the
// directory gives back exactly what crex typed before it could resume.
const rovoCommand = "acli rovodev run"

// rovoSessionsKey is the config.yml key, inside the sessions block, that moves
// the session folders away from ~/.rovodev/sessions.
const rovoSessionsKey = "persistenceDir:"

// detectRovo finds the latest Rovo Dev session opened in cwd.
//
// Rovo Dev keeps one folder per session under ~/.rovodev/sessions/, named by
// the session UUID, with a metadata.json that records the workspace it was
// opened in (Atlassian, "Manage sessions in Rovo Dev CLI"). The folder can be
// moved with sessions.persistenceDir in ~/.rovodev/config.yml. The newest
// session for cwd is resumed with `acli rovodev run --restore <uuid>`; with
// none, Rovo Dev starts fresh as it did before crex could resume it.
func detectRovo(cwd, _ string) *Session {
	session := &Session{Tool: "rovo", CWD: cwd, Command: rovoCommand}

	home, err := os.UserHomeDir()
	if err != nil {
		return session
	}

	// A missing config.yml is the usual case, and it leaves the default.
	config, _ := os.ReadFile(filepath.Join(home, ".rovodev", "config.yml"))
	sessionsDir := rovoSessionsDir(home, string(config))

	id := newestRovoSession(sessionsDir, cwd)
	if id == "" {
		return session
	}

	session.Command = rovoCommand + " --restore " + id
	return session
}

// newestRovoSession returns the UUID of the session in sessionsDir whose
// workspace is cwd and whose metadata.json changed last, or "" when there is
// none. Rovo Dev rewrites metadata.json as the session goes on, and the file
// has no documented timestamp field, so its modification time stands in for
// "most recent".
func newestRovoSession(sessionsDir, cwd string) string {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return ""
	}

	var newestID string
	var newestTime int64
	for _, entry := range entries {
		// The folder name is typed into the terminal as the restore argument,
		// so anything that is not a plain id is left alone.
		if !entry.IsDir() || !validSessionID.MatchString(entry.Name()) {
			continue
		}

		metadataPath := filepath.Join(sessionsDir, entry.Name(), "metadata.json")
		if rovoSessionWorkspace(metadataPath) != cwd {
			continue
		}

		info, err := os.Stat(metadataPath)
		if err != nil {
			continue
		}
		modified := info.ModTime().UnixNano()
		if newestID == "" || modified > newestTime {
			newestID = entry.Name()
			newestTime = modified
		}
	}
	return newestID
}

// rovoSessionWorkspace returns the workspace_path recorded in a session's
// metadata.json, or "" when the file is missing or is not valid JSON. A
// session that Rovo Dev was still writing when crex looked is skipped rather
// than guessed at.
func rovoSessionWorkspace(metadataPath string) string {
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return ""
	}

	var metadata struct {
		WorkspacePath string `json:"workspace_path"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return ""
	}
	return metadata.WorkspacePath
}

// rovoSessionsDir returns the folder that holds the Rovo Dev sessions:
// sessions.persistenceDir from config.yml when it is set, ~/.rovodev/sessions
// otherwise.
//
// The package has no YAML parser, and the setting is one key in a block, so a
// line scan reads it: a top-level `sessions:` line opens the block, and the
// next unindented line closes it. A value that is not absolute after
// expanding ~ falls back to the default, because nothing documents what a
// relative path would be relative to.
func rovoSessionsDir(home, config string) string {
	defaultDir := filepath.Join(home, ".rovodev", "sessions")

	inSessions := false
	for _, line := range strings.Split(config, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
		if !indented {
			// Only a bare header, maybe with a comment, opens the block.
			inSessions = rovoConfigValue(trimmed) == "sessions:"
			continue
		}
		if !inSessions || !strings.HasPrefix(trimmed, rovoSessionsKey) {
			continue
		}

		dir := rovoConfigValue(strings.TrimPrefix(trimmed, rovoSessionsKey))
		if dir == "~" || strings.HasPrefix(dir, "~/") {
			dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
		}
		if !filepath.IsAbs(dir) {
			return defaultDir
		}
		return dir
	}
	return defaultDir
}

// rovoConfigValue returns the scalar after a YAML key: the text between the
// quotes when it is quoted, the text before a trailing # comment otherwise.
// Escapes inside double quotes are not decoded; a session path has no reason
// to need them.
func rovoConfigValue(raw string) string {
	raw = strings.TrimSpace(raw)

	if strings.HasPrefix(raw, `"`) || strings.HasPrefix(raw, "'") {
		quote := raw[:1]
		end := strings.Index(raw[1:], quote)
		if end < 0 {
			return ""
		}
		return raw[1 : 1+end]
	}

	if comment := strings.Index(raw, " #"); comment >= 0 {
		raw = raw[:comment]
	}
	return strings.TrimSpace(raw)
}
