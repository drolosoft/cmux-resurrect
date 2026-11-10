package detect

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// copilotSession is one session folder under <copilot home>/session-state.
type copilotSession struct {
	id       string    // folder name, which is the session ID
	cwd      string    // the `cwd` key of workspace.yaml
	lastUsed time.Time // when its event log was last written
	held     bool      // the process being saved holds its in-use lock
}

// detectCopilot finds the session of a GitHub Copilot CLI process and
// resumes it by ID. Sessions live in <copilot home>/session-state/<id>/,
// where the home is the process's COPILOT_HOME (default ~/.copilot). The
// session the process holds an in-use lock on wins; otherwise the most
// recently used session of the directory is taken.
//
// It never falls back to `copilot --continue`: that opens the most recent
// session of the directory or, when there is none, the most recent of all,
// which can belong to another project.
func detectCopilot(cwd, pid string) *Session {
	copilotHome, custom := copilotHomeDir(pid)
	if copilotHome == "" {
		return nil
	}

	sessions := copilotSessions(filepath.Join(copilotHome, "session-state"), pid)

	sessionID := latestCopilotSession(sessions, func(s copilotSession) bool { return s.held })
	if sessionID == "" {
		sessionID = latestCopilotSession(sessions, func(s copilotSession) bool { return s.cwd == cwd })
	}
	if sessionID == "" {
		return nil
	}
	return &Session{
		Tool:    "copilot",
		CWD:     cwd,
		Command: copilotResumeCommand(copilotHome, custom, sessionID),
	}
}

// copilotHomeDir returns Copilot's home directory for a process: its
// COPILOT_HOME, else the one crex runs with, else ~/.copilot. custom
// reports whether it differs from ~/.copilot, so the resume command has to
// set it again.
func copilotHomeDir(pid string) (dir string, custom bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	def := filepath.Join(home, ".copilot")

	v := processEnv(pid)["COPILOT_HOME"]
	if v == "" {
		v = os.Getenv("COPILOT_HOME")
	}
	if v == "" || !filepath.IsAbs(v) {
		return def, false
	}
	v = filepath.Clean(v)
	return v, v != def
}

// copilotSessions reads every session folder in stateDir. A folder whose
// name can't be typed safely as a session ID, or without a workspace.yaml,
// is skipped.
func copilotSessions(stateDir, pid string) []copilotSession {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return nil
	}

	var sessions []copilotSession
	for _, e := range entries {
		if !e.IsDir() || !validSessionID.MatchString(e.Name()) {
			continue
		}
		sessionDir := filepath.Join(stateDir, e.Name())
		workspace := filepath.Join(sessionDir, "workspace.yaml")
		data, err := os.ReadFile(workspace)
		if err != nil {
			continue
		}

		// The event log gets a line on every turn, so its time says when the
		// session was last used. A session without one falls back to its
		// metadata file.
		info, err := os.Stat(filepath.Join(sessionDir, "events.jsonl"))
		if err != nil {
			info, err = os.Stat(workspace)
		}
		if err != nil {
			continue
		}

		sessions = append(sessions, copilotSession{
			id:       e.Name(),
			cwd:      copilotWorkspaceCWD(data),
			lastUsed: info.ModTime(),
			held:     copilotHoldsSession(sessionDir, pid),
		})
	}
	return sessions
}

// copilotWorkspaceCWD returns the top-level `cwd` value of a workspace.yaml,
// or "" when there is none. The file is plain YAML written by the CLI, so a
// value is a bare path unless it needed quoting.
func copilotWorkspaceCWD(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		value, found := strings.CutPrefix(strings.TrimRight(line, "\r"), "cwd:")
		if found {
			return unquoteYAMLScalar(strings.TrimSpace(value))
		}
	}
	return ""
}

// unquoteYAMLScalar undoes the quoting YAML puts around a scalar that would
// not read back as plain text, such as a path containing ": ". A
// double-quoted value uses backslash escapes; a single-quoted one only
// doubles its quotes.
func unquoteYAMLScalar(value string) string {
	if len(value) < 2 {
		return value
	}
	switch {
	case value[0] == '"' && value[len(value)-1] == '"':
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
		return value[1 : len(value)-1]
	case value[0] == '\'' && value[len(value)-1] == '\'':
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'")
	}
	return value
}

// copilotHoldsSession reports whether process pid holds the session in
// sessionDir. Copilot CLI claims a session it creates or resumes by writing
// inuse.<pid>.lock into the session's folder, and removes it on exit.
func copilotHoldsSession(sessionDir, pid string) bool {
	if _, err := strconv.Atoi(pid); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(sessionDir, "inuse."+pid+".lock"))
	return err == nil
}

// latestCopilotSession returns the ID of the most recently used session
// that matches, or "" if none does. One process can hold more than one
// session (/new keeps the previous one running), so the lock alone is not
// enough either.
func latestCopilotSession(sessions []copilotSession, matches func(copilotSession) bool) string {
	var latest copilotSession
	for _, s := range sessions {
		if !matches(s) {
			continue
		}
		if latest.id == "" || s.lastUsed.After(latest.lastUsed) {
			latest = s
		}
	}
	return latest.id
}

// copilotResumeCommand builds the resume command, re-setting COPILOT_HOME
// when the session lives outside ~/.copilot so Copilot looks for it in the
// right place.
func copilotResumeCommand(copilotHome string, custom bool, sessionID string) string {
	cmd := "copilot --resume " + sessionID
	if custom {
		cmd = "COPILOT_HOME=" + shellQuote(copilotHome) + " " + cmd
	}
	return cmd
}
