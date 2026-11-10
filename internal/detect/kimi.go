package detect

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// kimiMaxSlug is the longest basename slug Kimi Code puts in a session folder
// name (MAX_WORKDIR_SLUG_LENGTH in workdir-slug.ts). The hash that follows it
// keeps apart two directories whose long names start the same.
const kimiMaxSlug = 40

// kimiHashLength is how many hex digits of the path's sha256 Kimi Code keeps
// in the folder name (HASH_LENGTH in workdir-slug.ts).
const kimiHashLength = 12

// kimiSlugDisallowed matches the runs of characters that Kimi Code turns into
// a single "-" when it names a folder after a directory.
var kimiSlugDisallowed = regexp.MustCompile(`[^a-z0-9._-]+`)

// detectKimi finds the session of a Kimi Code process (MoonshotAI/kimi-code).
//
// Kimi Code keeps each session in
// <home>/sessions/<workDirKey>/<sessionId>/, with its metadata in state.json
// and the main agent's history in agents/main/wire.jsonl. <home> is
// $KIMI_CODE_HOME, or ~/.kimi-code. The newest session recorded in cwd is
// resumed with `kimi --session <id>`, which refuses a session from another
// directory anyway.
//
// The archived Kimi CLI (MoonshotAI/kimi-cli) ran as the same `kimi` binary
// but kept its sessions elsewhere and resumed them differently. Its store is
// not read: without a Kimi Code session the pane gets the bare command, as it
// did before crex could resume it.
func detectKimi(cwd, pid string) *Session {
	fresh := &Session{Tool: "kimi", CWD: cwd, Command: "kimi"}

	kimiDir, custom := kimiHomeDir(pid, cwd)
	if kimiDir == "" {
		return fresh
	}

	bucket := filepath.Join(kimiDir, "sessions", kimiWorkDirKey(cwd))
	sessionID := newestKimiSession(bucket, cwd)
	if sessionID == "" {
		return fresh
	}

	return &Session{
		Tool:    "kimi",
		CWD:     cwd,
		Command: kimiResumeCommand(kimiDir, custom, sessionID),
	}
}

// kimiHomeDir returns the Kimi Code data root a process uses, resolved as
// resolveKimiHome in agent-core-v2/src/app/bootstrap/bootstrap.ts does it:
// $KIMI_CODE_HOME when set, ~/.kimi-code otherwise. custom reports whether it
// differs from the default, in which case the resume command must set it
// again.
//
// Kimi takes the variable as it is, so a "~" in it is not expanded and a
// relative value is read from the process's working directory; that is why
// it goes through resolveFrom rather than expandUserPath. An empty value is
// treated as unset, as the CLI's own getDataDir does.
func kimiHomeDir(pid, cwd string) (dir string, custom bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	defaultDir := filepath.Join(home, ".kimi-code")

	dir = defaultDir
	if value := processEnv(pid)["KIMI_CODE_HOME"]; value != "" {
		dir = resolveFrom(cwd, value)
	}
	return dir, dir != defaultDir
}

// kimiWorkDirKey names the session folder of a directory the way Kimi Code's
// encodeWorkDirKey does (agent-core-v2/src/_base/utils/workdir-slug.ts):
// "wd_", the slug of the basename, "_" and the first 12 hex digits of the
// sha256 of the path. Backslashes count as "/" and trailing slashes are
// dropped before the path is hashed, so "/src/app/" shares a folder with
// "/src/app".
func kimiWorkDirKey(cwd string) string {
	normalized := strings.ReplaceAll(cwd, `\`, "/")
	normalized = strings.TrimRight(normalized, "/")
	base := normalized[strings.LastIndex(normalized, "/")+1:]

	sum := sha256.Sum256([]byte(normalized))
	hash := hex.EncodeToString(sum[:])[:kimiHashLength]

	return "wd_" + kimiSlug(base) + "_" + hash
}

// kimiSlug ports slugifyWorkDirName: the name is lowercased, every run of
// characters outside [a-z0-9._-] becomes "-", dashes are trimmed from both
// ends, and the result is cut to 40 characters and trimmed again. A slug left
// empty, or one that is "." or "..", is called "workspace".
func kimiSlug(name string) string {
	// JavaScript lowercases "İ" (U+0130) to "i" plus a combining dot
	// (U+0307), Go to a bare "i". Spelling the pair out keeps the dot, which
	// then becomes "-" just as it does in Kimi.
	name = strings.ReplaceAll(name, "\u0130", "i\u0307")

	slug := strings.ToLower(name)
	slug = kimiSlugDisallowed.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	// Only ASCII is left after the replacement, so a byte cut is a
	// character cut.
	if len(slug) > kimiMaxSlug {
		slug = slug[:kimiMaxSlug]
	}
	slug = strings.Trim(slug, "-")

	if slug == "" || slug == "." || slug == ".." {
		return "workspace"
	}
	return slug
}

// newestKimiSession returns the id of the most recently active session in a
// Kimi Code session folder that was recorded in cwd and is not archived, or ""
// when there is none. That is the session `kimi --continue` would pick: Kimi
// lists sessions by updatedAt, newest first, and leaves archived ones out.
func newestKimiSession(bucket, cwd string) string {
	entries, err := os.ReadDir(bucket)
	if err != nil {
		return ""
	}

	want := filepath.Clean(cwd)
	var newestID string
	var newestActivity int64
	for _, entry := range entries {
		// The folder name is typed into the terminal as the resume
		// argument, so anything that is not a plain id is left alone.
		if !entry.IsDir() || !validSessionID.MatchString(entry.Name()) {
			continue
		}

		sessionDir := filepath.Join(bucket, entry.Name())
		state, ok := readKimiState(sessionDir)
		if !ok || state.archived || filepath.Clean(state.cwd) != want {
			continue
		}

		activity := kimiSessionActivity(sessionDir, state)
		if newestID == "" || activity > newestActivity {
			newestID = entry.Name()
			newestActivity = activity
		}
	}
	return newestID
}

// kimiState is the part of a session's state.json that crex needs.
type kimiState struct {
	path      string // the state.json that was read
	cwd       string
	updatedAt int64 // epoch milliseconds, 0 when the file has none
	archived  bool
}

// readKimiState reads a session's state.json. Kimi Code looks for it in the
// session folder first and, for sessions written by older builds, in its
// session-meta folder (legacySessionMetaScopeOf in sessionLifecycle). Those
// older files may call the cwd workDir and store dates as strings, which
// normalizeSessionMeta still accepts. ok is false when there is no state.json
// or the first one found is not valid JSON, as when Kimi was still writing
// it; like Kimi, crex only falls back to the old place when the new file is
// missing.
func readKimiState(sessionDir string) (state kimiState, ok bool) {
	candidates := []string{
		filepath.Join(sessionDir, "state.json"),
		filepath.Join(sessionDir, "session-meta", "state.json"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// The values are kept raw because older files do not always have
		// the types the current one has, and a wrong type must not make
		// the whole file unreadable.
		var raw struct {
			CWD       string          `json:"cwd"`
			WorkDir   json.RawMessage `json:"workDir"`
			UpdatedAt json.RawMessage `json:"updatedAt"`
			Archived  json.RawMessage `json:"archived"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return kimiState{}, false
		}

		state = kimiState{
			path:      path,
			cwd:       raw.CWD,
			updatedAt: kimiEpochMillis(raw.UpdatedAt),
			archived:  string(raw.Archived) == "true",
		}
		if state.cwd == "" {
			// A legacy workDir that is not a string leaves the cwd empty,
			// and the session is then skipped as Kimi would skip it.
			_ = json.Unmarshal(raw.WorkDir, &state.cwd)
		}
		return state, true
	}
	return kimiState{}, false
}

// kimiEpochMillis reads a state.json date the way toEpochMs in
// sessionMetadataService.ts does: a number is already epoch milliseconds, a
// string is parsed as a date. Anything else is 0. Kimi writes ISO 8601 with
// toISOString, which RFC 3339 covers.
func kimiEpochMillis(raw json.RawMessage) int64 {
	var millis float64
	if json.Unmarshal(raw, &millis) == nil {
		return int64(millis)
	}

	var text string
	if json.Unmarshal(raw, &text) != nil {
		return 0
	}
	parsed, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return 0
	}
	return parsed.UnixMilli()
}

// kimiSessionActivity returns when a session was last active, in epoch
// milliseconds. updatedAt in state.json is what Kimi itself sorts by; a file
// without it falls back to when the main agent's wire.jsonl was last written,
// and then to the state file's own modification time.
func kimiSessionActivity(sessionDir string, state kimiState) int64 {
	if state.updatedAt > 0 {
		return state.updatedAt
	}

	wirePath := filepath.Join(sessionDir, "agents", "main", "wire.jsonl")
	if info, err := os.Stat(wirePath); err == nil {
		return info.ModTime().UnixMilli()
	}
	if info, err := os.Stat(state.path); err == nil {
		return info.ModTime().UnixMilli()
	}
	return 0
}

// kimiResumeCommand builds the resume command, setting KIMI_CODE_HOME again
// when the session lives outside ~/.kimi-code so Kimi looks for it there.
func kimiResumeCommand(kimiDir string, custom bool, sessionID string) string {
	command := "kimi --session " + sessionID
	if custom {
		command = "KIMI_CODE_HOME=" + shellQuote(kimiDir) + " " + command
	}
	return command
}
