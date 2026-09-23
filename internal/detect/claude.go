package detect

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// claudeConfigDir returns the Claude Code config directory a process uses:
// $CLAUDE_CONFIG_DIR from the process environment when set (e.g. a
// `claude-pro` alias for `CLAUDE_CONFIG_DIR=~/.claude-pro claude`),
// ~/.claude otherwise. custom reports whether it differs from the default,
// in which case the resume command must set it again.
func claudeConfigDir(pid string) (dir string, custom bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	def := filepath.Join(home, ".claude")
	env := processEnv(pid)
	v := env["CLAUDE_CONFIG_DIR"]
	if v == "" || !filepath.IsAbs(v) {
		return def, false
	}
	v = filepath.Clean(v)
	return v, v != def
}

// nonAlnum matches the characters Claude Code replaces with "-" when naming a
// project directory after its working directory.
var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]`)

// claudeProjectDirs lists candidate project directories for cwd under a
// config dir. Claude replaces every non-alphanumeric character with "-"
// ("/src/app.io" → "-src-app-io"); the "/"-only form is kept as a fallback
// for older Claude Code versions.
func claudeProjectDirs(configDir, cwd string) []string {
	dirs := []string{filepath.Join(configDir, "projects", nonAlnum.ReplaceAllString(cwd, "-"))}
	if legacy := filepath.Join(configDir, "projects", strings.ReplaceAll(cwd, "/", "-")); legacy != dirs[0] {
		dirs = append(dirs, legacy)
	}
	return dirs
}

// claudeSessionForPID reads the session ID a live Claude Code process
// records in <config>/sessions/<pid>.json. This identifies the exact
// conversation of that process, even when several run in the same
// directory. Returns "" when the file is missing, belongs to another
// directory, or its transcript doesn't exist yet (nothing to resume).
func claudeSessionForPID(configDir, pid, cwd string) (sessionID, transcript string) {
	if pid == "" {
		return "", ""
	}
	data, err := os.ReadFile(filepath.Join(configDir, "sessions", pid+".json"))
	if err != nil {
		return "", ""
	}
	var meta struct {
		SessionID string `json:"sessionId"`
		CWD       string `json:"cwd"`
	}
	if json.Unmarshal(data, &meta) != nil || meta.CWD != cwd || !validSessionID.MatchString(meta.SessionID) {
		return "", ""
	}
	for _, dir := range claudeProjectDirs(configDir, cwd) {
		path := filepath.Join(dir, meta.SessionID+".jsonl")
		if _, err := os.Stat(path); err == nil {
			return meta.SessionID, path
		}
	}
	return "", ""
}

// claudeSessionTitle returns the conversation title recorded in a Claude
// transcript: the last title set with /rename, else the last generated one.
// Claude Code shows it in the terminal title. Returns "" if none.
func claudeSessionTitle(transcript string) string {
	f, err := os.Open(transcript)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	var aiTitle, customTitle string
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		// Title entries are small; skip decoding the (large) message lines.
		if bytes.Contains(line, []byte(`"ai-title"`)) || bytes.Contains(line, []byte(`"custom-title"`)) {
			var entry struct {
				Type        string `json:"type"`
				AITitle     string `json:"aiTitle"`
				CustomTitle string `json:"customTitle"`
			}
			if json.Unmarshal(line, &entry) == nil {
				switch {
				case entry.Type == "custom-title" && entry.CustomTitle != "":
					customTitle = entry.CustomTitle
				case entry.Type == "ai-title" && entry.AITitle != "":
					aiTitle = entry.AITitle
				}
			}
		}
		if err != nil {
			break
		}
	}
	if customTitle != "" {
		return customTitle
	}
	return aiTitle
}

// claudeResumeCommand builds the resume command, re-setting
// CLAUDE_CONFIG_DIR when the session lives outside ~/.claude so Claude
// looks for it in the right place.
func claudeResumeCommand(configDir string, custom bool, sessionID string) string {
	cmd := "claude --resume " + sessionID
	if custom {
		cmd = "CLAUDE_CONFIG_DIR=" + shellQuote(configDir) + " " + cmd
	}
	return cmd
}

// shellSafe matches strings that need no quoting in a POSIX shell.
var shellSafe = regexp.MustCompile(`^[a-zA-Z0-9_@%+=:,./-]+$`)

// shellQuote single-quotes s for a POSIX shell when needed.
func shellQuote(s string) string {
	if shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// envAssignment matches the name part of a leading shell assignment.
var envAssignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// SplitEnvPrefix splits leading VAR=value assignments off a command:
// "CLAUDE_CONFIG_DIR=/x claude --resume id" → ("CLAUDE_CONFIG_DIR=/x ",
// "claude --resume id"). Values may be single-quoted or backslash-escaped.
// Commands without assignments return ("", command).
func SplitEnvPrefix(command string) (prefix, rest string) {
	rest = command
	for envAssignment.MatchString(rest) {
		end := shellWordEnd(rest)
		if end >= len(rest) {
			// A lone assignment is not a command prefix.
			break
		}
		// Consume the separating spaces too.
		next := end
		for next < len(rest) && rest[next] == ' ' {
			next++
		}
		prefix += rest[:next]
		rest = rest[next:]
	}
	return prefix, rest
}

// shellWordEnd returns the index just past the first shell word of s,
// honoring single quotes and backslash escapes.
func shellWordEnd(s string) int {
	inQuote := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case inQuote:
			if c == '\'' {
				inQuote = false
			}
		case c == '\'':
			inQuote = true
		case c == '\\':
			i++
		case c == ' ' || c == '\t':
			return i
		}
	}
	return len(s)
}
