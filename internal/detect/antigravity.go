package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// antigravityCommand starts Antigravity CLI from a shell. A pane without a
// conversation for its directory gets exactly this back, which starts a
// fresh one as `agy -c` would do there anyway.
const antigravityCommand = "agy"

// detectAntigravity finds the last conversation Antigravity CLI had in cwd.
//
// agy keeps ~/.gemini/antigravity-cli/cache/last_conversations.json, a JSON
// object from each absolute workspace path to the id of its most recent
// conversation, and `agy -c` reads it to resume "the single most recent
// conversation associated with your active workspace" (Google, "Resume" in
// the Antigravity CLI docs). The id from that same file is resumed with
// `agy --conversation <id>`, which keeps working after the pane is restored
// even if another directory's entry changes meanwhile. The docs name no
// variable that moves the folder.
func detectAntigravity(cwd, _ string) *Session {
	session := &Session{Tool: "antigravity", CWD: cwd, Command: antigravityCommand}

	// The keys are absolute paths, so a relative cwd could only match one by
	// accident.
	if !filepath.IsAbs(cwd) {
		return session
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return session
	}

	cachePath := filepath.Join(home, ".gemini", "antigravity-cli", "cache", "last_conversations.json")
	id := antigravityConversation(cachePath, cwd)
	if id == "" {
		return session
	}

	session.Command = antigravityCommand + " --conversation " + id
	return session
}

// antigravityConversation returns the conversation id the cache at
// cachePath holds for cwd, or "" when the file is missing, damaged or has no
// usable entry for it.
//
// The cache is keyed by the directory agy started in, not by the root of its
// repository ("Antigravity CLI scopes conversation histories directly to
// your current working directory"), so only cwd itself is looked up: first
// as crex read it, then with its links resolved, because the docs do not say
// which spelling agy records (/tmp is /private/tmp on macOS).
func antigravityConversation(cachePath, cwd string) string {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return ""
	}

	// A file agy was still writing when crex looked is skipped rather than
	// guessed at.
	var conversations map[string]string
	if err := json.Unmarshal(data, &conversations); err != nil {
		return ""
	}

	candidates := []string{filepath.Clean(cwd)}
	resolved, err := filepath.EvalSymlinks(cwd)
	if err == nil && resolved != candidates[0] {
		candidates = append(candidates, resolved)
	}

	for _, workspace := range candidates {
		id := conversations[workspace]

		// The id is typed into the terminal as the argument of
		// --conversation, so anything that is not a plain id is left alone.
		if id != "" && validSessionID.MatchString(id) {
			return id
		}
	}
	return ""
}
