package detect

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// detectDroid finds the session of a Factory Droid process. Droid keeps one
// transcript per session in <home>/.factory/sessions/<encoded-cwd>/<id>.jsonl,
// where home is the process's FACTORY_HOME_OVERRIDE or the user's home, and
// the folder is the cwd with every non-alphanumeric character as "-". The
// newest transcript whose header records this cwd is resumed. Without one,
// Droid starts fresh, as it did before crex could resume it.
func detectDroid(cwd, pid string) *Session {
	fresh := &Session{Tool: "factory", CWD: cwd, Command: "droid"}

	home, custom := droidHome(pid)
	if home == "" {
		return fresh
	}

	sessionDir := filepath.Join(home, ".factory", "sessions", nonAlnum.ReplaceAllString(cwd, "-"))
	sessionID := latestDroidSession(sessionDir, cwd)
	if sessionID == "" {
		return fresh
	}

	return &Session{
		Tool:    "factory",
		CWD:     cwd,
		Command: droidResumeCommand(home, custom, sessionID),
	}
}

// droidHome returns the directory Droid puts .factory in for a process:
// $FACTORY_HOME_OVERRIDE from the process environment when set, the user's
// home otherwise. The variable replaces the home itself, not the .factory
// folder. custom reports whether it differs from the real home, in which case
// the resume command must set it again.
func droidHome(pid string) (dir string, custom bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}

	// A relative value would name a different folder from each directory, so
	// Droid's sessions cannot be found through it; keep the real home.
	override := processEnv(pid)["FACTORY_HOME_OVERRIDE"]
	if override == "" || !filepath.IsAbs(override) {
		return home, false
	}

	override = filepath.Clean(override)
	return override, override != home
}

// latestDroidSession returns the ID of the most recently modified Droid
// session in sessionDir that was started in cwd, or "" if none qualifies.
// The folder name alone is not enough: /w/acme/foo-bar and /w/acme-foo/bar
// both become -w-acme-foo-bar, so each header has to name cwd itself.
func latestDroidSession(sessionDir, cwd string) string {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return ""
	}

	type transcript struct {
		path    string
		modTime int64
	}
	var transcripts []transcript
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		transcripts = append(transcripts, transcript{
			path:    filepath.Join(sessionDir, entry.Name()),
			modTime: info.ModTime().UnixNano(),
		})
	}

	sort.Slice(transcripts, func(i, j int) bool {
		return transcripts[i].modTime > transcripts[j].modTime
	})

	for _, candidate := range transcripts {
		header, ok := readDroidSessionStart(candidate.path)
		if !ok || header.CWD != cwd || !validSessionID.MatchString(header.ID) {
			continue
		}

		// A Worker started by the Task tool writes its own transcript in the
		// same folder and names the session that called it. Resuming it would
		// drop the user into the subtask, not their conversation.
		if header.CallingSessionID != "" {
			continue
		}
		return header.ID
	}
	return ""
}

// droidSessionStart is the first line of a Droid transcript.
type droidSessionStart struct {
	Type             string `json:"type"`
	ID               string `json:"id"`
	CWD              string `json:"cwd"`
	CallingSessionID string `json:"callingSessionId"`
}

// readDroidSessionStart reads the session_start line that opens a Droid
// transcript. It reports false when the file cannot be read or does not start
// with one.
func readDroidSessionStart(path string) (droidSessionStart, bool) {
	f, err := os.Open(path)
	if err != nil {
		return droidSessionStart{}, false
	}
	defer func() { _ = f.Close() }()

	// A Worker's header carries its whole task prompt in the title, which can
	// be longer than a bufio.Scanner line, so the line is read whole.
	line, err := bufio.NewReader(f).ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return droidSessionStart{}, false
	}

	var header droidSessionStart
	if json.Unmarshal(line, &header) != nil || header.Type != "session_start" {
		return droidSessionStart{}, false
	}
	return header, true
}

// droidResumeCommand builds the resume command, setting FACTORY_HOME_OVERRIDE
// again when the session lives outside the user's home so Droid looks for it
// in the right place.
func droidResumeCommand(home string, custom bool, sessionID string) string {
	cmd := "droid --resume " + sessionID
	if custom {
		cmd = "FACTORY_HOME_OVERRIDE=" + shellQuote(home) + " " + cmd
	}
	return cmd
}
