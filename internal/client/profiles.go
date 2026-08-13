package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BrowserProfile is one cmux browser profile from `rpc browser.profiles.list`.
type BrowserProfile struct {
	ID      string `json:"id"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Default bool   `json:"built_in_default"`
}

// BrowserProfileProvider is optionally implemented by backends that can report
// which browser profile each browser surface uses. Save captures the profile
// slug per surface (GitHub #9); backends without profiles (Ghostty) are
// detected via type assertion and the field is simply not populated.
type BrowserProfileProvider interface {
	// SurfaceProfiles maps surface refs to their NON-default browser profile
	// slug. Surfaces on the default profile (or on a profile that no longer
	// exists) are omitted, so saved TOMLs stay clean.
	SurfaceProfiles() (map[string]string, error)
}

// BrowserProfileEnsurer is optionally implemented by backends that can create
// browser profiles on demand. Restore uses it so a layout that references a
// profile missing on this machine gets an empty profile bucket instead of a
// failed pane creation (newer cmux rejects unknown --profile selectors).
type BrowserProfileEnsurer interface {
	EnsureBrowserProfile(slug string) error
}

// parseBrowserProfiles parses `cmux rpc browser.profiles.list` JSON.
func parseBrowserProfiles(jsonOut string) ([]BrowserProfile, error) {
	var resp struct {
		Profiles []BrowserProfile `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &resp); err != nil {
		return nil, fmt.Errorf("parse browser.profiles.list: %w", err)
	}
	return resp.Profiles, nil
}

// parseSessionSurfaceProfiles extracts panel-UUID → profile-UUID from cmux's
// session persistence JSON. cmux exposes no API for a surface's assigned
// profile, so this reads the app's own session file; the walk is generic
// (any object with an "id" string and a "browser" object carrying a
// "profileID") so nesting changes in the session schema don't break it.
func parseSessionSurfaceProfiles(data []byte) (map[string]string, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse session file: %w", err)
	}
	out := make(map[string]string)
	var walk func(v any)
	walk = func(v any) {
		switch node := v.(type) {
		case map[string]any:
			id, _ := node["id"].(string)
			if browser, ok := node["browser"].(map[string]any); ok && id != "" {
				if pid, ok := browser["profileID"].(string); ok && pid != "" {
					out[id] = pid
				}
			}
			for _, child := range node {
				walk(child)
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(root)
	return out, nil
}

// composeSurfaceProfiles joins the session's panel→profileID map with the
// profile list and the live tree's surfaceUUID→ref map, producing
// ref → profile slug for surfaces on a NON-default profile. Panels whose
// profile was deleted resolve to nothing (they render on default anyway).
func composeSurfaceProfiles(panelProfiles map[string]string, profiles []BrowserProfile, idToRef map[string]string) map[string]string {
	slugByID := make(map[string]string, len(profiles))
	for _, p := range profiles {
		if !p.Default {
			slugByID[p.ID] = p.Slug
		}
	}
	out := make(map[string]string)
	for panelID, profileID := range panelProfiles {
		ref := idToRef[panelID]
		slug := slugByID[profileID]
		if ref != "" && slug != "" {
			out[ref] = slug
		}
	}
	return out
}

// browserProfileExists reports whether a selector (slug, display name, or
// UUID — all case-insensitive, matching cmux's own selector resolution)
// matches an existing profile.
func browserProfileExists(profiles []BrowserProfile, selector string) bool {
	if selector == "" {
		return false
	}
	for _, p := range profiles {
		if strings.EqualFold(p.Slug, selector) ||
			strings.EqualFold(p.Name, selector) ||
			strings.EqualFold(p.ID, selector) {
			return true
		}
	}
	return false
}

// parseTreeSurfaceIDs maps surface UUID → surface ref from
// `cmux tree --json --id-format both` output.
func parseTreeSurfaceIDs(jsonOut string) (map[string]string, error) {
	var raw struct {
		Windows []struct {
			Workspaces []struct {
				Panes []struct {
					Surfaces []struct {
						Ref string `json:"ref"`
						ID  string `json:"id"`
					} `json:"surfaces"`
				} `json:"panes"`
			} `json:"workspaces"`
		} `json:"windows"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &raw); err != nil {
		return nil, fmt.Errorf("parse tree: %w", err)
	}
	out := make(map[string]string)
	for _, w := range raw.Windows {
		for _, ws := range w.Workspaces {
			for _, p := range ws.Panes {
				for _, s := range p.Surfaces {
					if s.ID != "" && s.Ref != "" {
						out[s.ID] = s.Ref
					}
				}
			}
		}
	}
	return out, nil
}

// newestSessionFile returns the most recently modified session-*.json in dir,
// excluding the *-previous.json backup cmux keeps alongside it.
func newestSessionFile(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "session-*.json"))
	if err != nil {
		return "", err
	}
	best := ""
	var bestMod int64
	for _, m := range matches {
		if strings.HasSuffix(m, "-previous.json") {
			continue
		}
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		if best == "" || info.ModTime().UnixNano() > bestMod {
			best = m
			bestMod = info.ModTime().UnixNano()
		}
	}
	if best == "" {
		return "", fmt.Errorf("no cmux session file in %s", dir)
	}
	return best, nil
}

// browserProfiles fetches and parses the profile list from cmux.
func (c *CLIClient) browserProfiles() ([]BrowserProfile, error) {
	out, err := c.run("rpc", "browser.profiles.list")
	if err != nil {
		return nil, err
	}
	return parseBrowserProfiles(out)
}

// sessionDir returns the directory holding cmux's session persistence files.
func (c *CLIClient) sessionDir() string {
	if c.SessionDir != "" {
		return c.SessionDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "cmux")
}

// SurfaceProfiles implements BrowserProfileProvider. cmux has no API that
// reports which profile a browser surface uses, so this correlates three
// sources: the profile list (UUID→slug), cmux's own session persistence file
// (panel UUID→profile UUID), and the live tree (surface UUID→ref). Callers
// must treat errors as "no profile info" — the session file is cmux's private
// state and absence or drift must never fail a save.
func (c *CLIClient) SurfaceProfiles() (map[string]string, error) {
	profiles, err := c.browserProfiles()
	if err != nil {
		return nil, err
	}
	// Fast path: with only the built-in default profile there is nothing to
	// capture — skip the session-file read entirely.
	nonDefault := false
	for _, p := range profiles {
		if !p.Default {
			nonDefault = true
			break
		}
	}
	if !nonDefault {
		return map[string]string{}, nil
	}

	path, err := newestSessionFile(c.sessionDir())
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	panelProfiles, err := parseSessionSurfaceProfiles(data)
	if err != nil {
		return nil, err
	}

	// --all: same reason as SurfaceDirectories — the caller's window may not
	// be focused, and the scoped tree would drop its surfaces from the join.
	out, err := c.run("tree", "--all", "--json", "--id-format", "both")
	if err != nil {
		return nil, err
	}
	idToRef, err := parseTreeSurfaceIDs(out)
	if err != nil {
		return nil, err
	}

	return composeSurfaceProfiles(panelProfiles, profiles, idToRef), nil
}

// EnsureBrowserProfile implements BrowserProfileEnsurer: creates the profile
// when no existing profile matches the selector. Only the empty profile
// bucket is created — its contents (cookies, logins) are never touched.
func (c *CLIClient) EnsureBrowserProfile(slug string) error {
	if slug == "" {
		return nil
	}
	profiles, err := c.browserProfiles()
	if err != nil {
		return err
	}
	if browserProfileExists(profiles, slug) {
		return nil
	}
	_, err = c.run("browser", "profiles", "add", slug)
	return err
}
