package detect

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// CodeBuddy cuts a project folder name longer than codeBuddyMaxNameBytes to
// its first codeBuddyKeptNameBytes and adds a hash of the whole name, which
// keeps the folder under the 255-byte file name limit of most file systems.
const (
	codeBuddyMaxNameBytes  = 255
	codeBuddyKeptNameBytes = 180
)

// codeBuddyHeadLines is how many transcript lines are read looking for a
// cwd. Every history line has one except "topic" and "summary", so the
// first few always settle it.
const codeBuddyHeadLines = 8

// codeBuddyHeadBytes caps what is read from one transcript. CodeBuddy adds
// cwd at the end of a line, after the message, and keeps images out of the
// line as blob references, so a first prompt fits well within it.
const codeBuddyHeadBytes = 1 << 20

// detectCodeBuddy finds the session of a Tencent CodeBuddy Code process. The
// config dir comes from the process's CODEBUDDY_CONFIG_DIR (default
// ~/.codebuddy), and the transcripts live in
// <config>/projects/<folder named after the cwd>/<sessionId>.jsonl. The most
// recent transcript whose lines record this cwd is resumed. Without one,
// CodeBuddy is started fresh, as it was before crex could resume it.
func detectCodeBuddy(cwd, pid string) *Session {
	fresh := &Session{Tool: "codebuddy", CWD: cwd, Command: "codebuddy"}

	configDir, custom := codeBuddyConfigDir(pid)
	if configDir == "" {
		return fresh
	}

	cwds := codeBuddyCwdForms(cwd)
	for _, dir := range codeBuddyProjectDirs(configDir, cwds) {
		sessionID := latestCodeBuddySession(dir, cwds)
		if sessionID != "" {
			return &Session{
				Tool:    "codebuddy",
				CWD:     cwd,
				Command: codeBuddyResumeCommand(configDir, custom, sessionID),
			}
		}
	}
	return fresh
}

// codeBuddyConfigDir returns the CodeBuddy Code config directory a process
// uses: $CODEBUDDY_CONFIG_DIR from the process environment when set,
// ~/.codebuddy otherwise. custom reports whether it differs from the
// default, in which case the resume command must set it again.
func codeBuddyConfigDir(pid string) (dir string, custom bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	def := filepath.Join(home, ".codebuddy")

	env := processEnv(pid)
	v := env["CODEBUDDY_CONFIG_DIR"]
	if strings.TrimSpace(v) == "" || !filepath.IsAbs(v) {
		return def, false
	}
	v = filepath.Clean(v)
	return v, v != def
}

// codeBuddyCwdForms returns the paths CodeBuddy may have recorded for cwd.
// It names the folder after the real path, with symlinks resolved, and
// falls back to the path as given; crex may hold either one.
func codeBuddyCwdForms(cwd string) []string {
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil || resolved == cwd {
		return []string{cwd}
	}
	return []string{resolved, cwd}
}

// codeBuddyProjectDirs lists the project folders that may hold the
// transcripts of the given cwds, without repeating one.
func codeBuddyProjectDirs(configDir string, cwds []string) []string {
	var dirs []string
	for _, cwd := range cwds {
		dir := filepath.Join(configDir, "projects", codeBuddyProjectName(cwd))
		if !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// codeBuddySeparators matches the characters CodeBuddy turns into "-" when
// naming a project folder after its working directory.
var codeBuddySeparators = regexp.MustCompile(`[/\\:]`)

// codeBuddyDashRuns matches the runs of "-" that CodeBuddy folds into one.
var codeBuddyDashRuns = regexp.MustCompile(`-+`)

// codeBuddyProjectName names the project folder of a working directory the
// way CodeBuddy's compressWorkspacePathName does: "/", "\" and ":" become
// "-", leading and trailing ones are dropped and runs fold into one
// ("/src/app.io" → "src-app.io"). Unlike Claude Code, dots and underscores
// stay. A name over 255 bytes keeps its first 180 and gets a hash of the
// whole name, so two long paths do not end up in the same folder.
func codeBuddyProjectName(cwd string) string {
	name := codeBuddySeparators.ReplaceAllString(cwd, "-")
	name = strings.Trim(name, "-")
	name = codeBuddyDashRuns.ReplaceAllString(name, "-")

	if len(name) <= codeBuddyMaxNameBytes {
		return name
	}
	return truncateOnRune(name, codeBuddyKeptNameBytes) + "-" + codeBuddyNameHash(name)
}

// truncateOnRune cuts s to at most limit bytes without splitting a
// character, as the byte limit CodeBuddy applies to folder names.
func truncateOnRune(s string, limit int) string {
	if len(s) <= limit {
		return s
	}

	// Ranging over a string visits the byte offset where each character
	// starts; the last one within the limit is where the cut goes.
	end := 0
	for i := range s {
		if i > limit {
			break
		}
		end = i
	}
	return s[:end]
}

// codeBuddyNameHash is CodeBuddy's djb2Base36: the djb2 hash of the name's
// bytes, with the XOR variant, in base 36. JavaScript wraps the value to 32
// bits on every XOR, which uint32 arithmetic does for free.
func codeBuddyNameHash(name string) string {
	var hash uint32 = 5381
	for i := 0; i < len(name); i++ {
		hash = hash*33 ^ uint32(name[i])
	}
	return strconv.FormatUint(uint64(hash), 36)
}

// latestCodeBuddySession returns the ID of the most recently modified
// transcript in a CodeBuddy project folder whose lines record one of the
// given cwds, or "" if none qualifies. The cwd check matters because
// different paths can share a folder ("/src/my-app" and "/src/my/app" are
// both "src-my-app").
func latestCodeBuddySession(projectDir string, cwds []string) string {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return ""
	}

	type fileInfo struct {
		name    string
		modTime int64
	}
	var sessions []fileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}

		// Sub-agent sessions are named agent-<id>. CodeBuddy keeps them in
		// <parent>/subagents/, but writes one here when it has no parent;
		// neither is a conversation to resume.
		if strings.HasPrefix(e.Name(), "agent-") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, fileInfo{
			name:    e.Name(),
			modTime: info.ModTime().UnixNano(),
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].modTime > sessions[j].modTime
	})

	// Newest first, so usually only one transcript is opened.
	for _, session := range sessions {
		sessionID := strings.TrimSuffix(session.name, ".jsonl")
		if !validSessionID.MatchString(sessionID) {
			continue
		}

		recorded := codeBuddyTranscriptCwd(filepath.Join(projectDir, session.name))
		for _, cwd := range cwds {
			if recorded == cwd {
				return sessionID
			}
		}
	}
	return ""
}

// codeBuddyTranscriptCwd returns the working directory recorded in the
// first lines of a CodeBuddy transcript, or "" if none of them has one.
// Only the head of the file is read: transcripts grow with the
// conversation, and the cwd never changes within one.
func codeBuddyTranscriptCwd(transcript string) string {
	f, err := os.Open(transcript)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	r := bufio.NewReader(io.LimitReader(f, codeBuddyHeadBytes))
	for i := 0; i < codeBuddyHeadLines; i++ {
		line, err := r.ReadBytes('\n')

		// A line cut short by the limit does not decode and is skipped.
		var entry struct {
			CWD string `json:"cwd"`
		}
		if json.Unmarshal(line, &entry) == nil && entry.CWD != "" {
			return entry.CWD
		}

		if err != nil {
			break
		}
	}
	return ""
}

// codeBuddyResumeCommand builds the resume command, re-setting
// CODEBUDDY_CONFIG_DIR when the session lives outside ~/.codebuddy so
// CodeBuddy looks for it in the right place.
func codeBuddyResumeCommand(configDir string, custom bool, sessionID string) string {
	cmd := "codebuddy --resume " + sessionID
	if custom {
		cmd = "CODEBUDDY_CONFIG_DIR=" + shellQuote(configDir) + " " + cmd
	}
	return cmd
}
