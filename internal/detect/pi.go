package detect

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// piAgentDirVariable moves Pi's whole agent directory, sessions included
// (ENV_AGENT_DIR in Pi's src/config.ts). The default is ~/.pi/agent.
const piAgentDirVariable = "PI_CODING_AGENT_DIR"

// piSessionDirVariable points Pi at one folder that holds the session files
// of every project side by side, with no per-directory folder inside
// (ENV_SESSION_DIR, read in Pi's src/main.ts). It wins over the agent dir.
const piSessionDirVariable = "PI_CODING_AGENT_SESSION_DIR"

// detectPi finds the session a Pi process is running in its directory.
// Pi keeps one JSONL file per session in
// <agent dir>/sessions/--<encoded cwd>--/<timestamp>_<id>.jsonl, and its
// first line names the session and the directory it belongs to. The newest
// file whose header matches cwd is resumed with `pi --session <id>`. When
// there is none, Pi is relaunched empty, as before it could be resumed.
func detectPi(cwd, pid string) *Session {
	fresh := &Session{Tool: "pi", CWD: cwd, Command: "pi"}

	folder, prefix := piSessionLocation(cwd, pid)
	if folder == "" {
		return fresh
	}

	sessionID := newestSessionByHeader(folder, cwd)
	if sessionID == "" {
		return fresh
	}

	return &Session{Tool: "pi", CWD: cwd, Command: prefix + "pi --session " + sessionID}
}

// piSessionLocation returns the folder where the Pi process keeps the
// sessions of cwd, and the variable assignment the resume command must
// carry when that folder is not the default one: `pi --session <id>` only
// finds a session in the folders it is told to look in.
func piSessionLocation(cwd, pid string) (folder, prefix string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	environment := processEnv(pid)

	// A session dir holds the files directly, so cwd adds no folder.
	if sessionDir := expandUserPath(piVariable(environment, piSessionDirVariable), home); sessionDir != "" {
		return sessionDir, piSessionDirVariable + "=" + shellQuote(sessionDir) + " "
	}

	agentDir := filepath.Join(home, ".pi", "agent")
	if customDir := expandUserPath(piVariable(environment, piAgentDirVariable), home); customDir != "" && customDir != agentDir {
		agentDir = customDir
		prefix = piAgentDirVariable + "=" + shellQuote(customDir) + " "
	}

	return filepath.Join(agentDir, "sessions", piSessionFolder(cwd)), prefix
}

// piVariable reads one variable from the environment the Pi process was
// started with. When crex cannot read that environment (no pid, or a
// process it may not inspect) it uses its own, which is what a Pi started
// from the same shell sees too.
func piVariable(environment map[string]string, name string) string {
	if environment == nil {
		return os.Getenv(name)
	}
	return environment[name]
}

// piSessionFolder names the folder Pi keeps the sessions of cwd in: the
// path without its leading separator, with "/", "\" and ":" turned into
// "-", between "--" marks ("/Users/x/proj" → "--Users-x-proj--"). This is
// getDefaultSessionDirPath in Pi's src/core/session-manager.ts. Different
// directories can share a folder ("/a/b" and "/a-b"), so the header still
// has to be checked.
func piSessionFolder(cwd string) string {
	path := cwd
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) {
		path = path[1:]
	}

	path = strings.NewReplacer("/", "-", `\`, "-", ":", "-").Replace(path)

	return "--" + path + "--"
}

// expandUserPath turns a directory taken from an environment variable into
// a clean absolute path, expanding a leading "~" the way Pi does. It
// returns "" for an empty or relative value, which crex cannot place
// without knowing the directory Pi was started from.
func expandUserPath(path, home string) string {
	if path == "~" {
		return home
	}
	if rest, found := strings.CutPrefix(path, "~/"); found {
		path = filepath.Join(home, rest)
	}

	if !filepath.IsAbs(path) {
		return ""
	}
	return filepath.Clean(path)
}

// newestSessionByHeader returns the id of the most recently written session
// file in folder whose header belongs to cwd, or "" when there is none.
// It reads a folder of <timestamp>_<id>.jsonl files that open with a
// {"type":"session","id":...,"cwd":...} line, which is how Pi and the tools
// built on it store sessions. Like `pi -c`, it goes by modification time,
// so the session used last wins over the one created last.
func newestSessionByHeader(folder, cwd string) string {
	want := filepath.Clean(cwd)

	for _, path := range sessionFilesNewestFirst(folder) {
		sessionID, sessionCWD := readSessionHeader(path)
		if sessionCWD == "" || filepath.Clean(sessionCWD) != want {
			continue
		}
		// The id is typed into a terminal on restore, so a file that
		// names something else than a plain id is passed over.
		if !validSessionID.MatchString(sessionID) {
			continue
		}
		return sessionID
	}

	return ""
}

// sessionFilesNewestFirst lists the .jsonl files in folder, the most
// recently modified first. A missing folder lists nothing.
func sessionFilesNewestFirst(folder string) []string {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil
	}

	type sessionFile struct {
		path    string
		modTime int64
	}
	var files []sessionFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, sessionFile{
			path:    filepath.Join(folder, entry.Name()),
			modTime: info.ModTime().UnixNano(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.path)
	}
	return paths
}

// readSessionHeader returns the session id and directory from the header of
// a session file: its first non-empty line, which must have type "session"
// and a string id. Pi skips blank lines before the header too
// (readSessionHeader in its session-manager.ts). Anything else gives "".
func readSessionHeader(path string) (sessionID, cwd string) {
	file, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var header struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			CWD  string `json:"cwd"`
		}
		if err := json.Unmarshal(line, &header); err != nil || header.Type != "session" {
			return "", ""
		}
		return header.ID, header.CWD
	}

	return "", ""
}
