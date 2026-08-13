package client

import (
	"encoding/json"
	"fmt"
	"os"
)

// SurfaceDirectoryProvider is optionally implemented by backends that can
// report each surface's working directory from persisted state rather than a
// live shell. It exists because cmux spawns a tab's shell lazily on first
// render: a tab the user has not looked at yet has no shell, so the live tree
// and debug.terminals report the workspace's directory instead of the tab's
// own — which silently collapsed every unopened tab onto the first tab's path
// on save (GitHub #8). cmux persists the real per-tab directory in its session
// file, and that survives the lazy spawn.
type SurfaceDirectoryProvider interface {
	// SurfaceDirectories maps surface refs to their persisted working
	// directory. Callers must treat an error as "no information" — this reads
	// the backend's private state and must never fail a save.
	SurfaceDirectories() (map[string]string, error)
}

// parseSessionSurfaceDirectories extracts panel UUID → working directory for
// TERMINAL panels from cmux's session persistence JSON. The walk is generic
// (any object with a string "id", type "terminal", and a non-empty
// "directory") so nesting changes in the session schema don't break it.
func parseSessionSurfaceDirectories(data []byte) (map[string]string, error) {
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
			typ, _ := node["type"].(string)
			dir, _ := node["directory"].(string)
			if id != "" && typ == "terminal" && dir != "" {
				out[id] = dir
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

// composeSurfaceDirs re-keys panel UUID → directory by live surface ref,
// dropping panels that are no longer in the tree.
func composeSurfaceDirs(panelDirs, idToRef map[string]string) map[string]string {
	out := make(map[string]string, len(panelDirs))
	for id, dir := range panelDirs {
		if ref := idToRef[id]; ref != "" {
			out[ref] = dir
		}
	}
	return out
}

// SurfaceDirectories implements SurfaceDirectoryProvider by correlating cmux's
// session file (panel UUID → directory) with the live tree (surface UUID → ref).
func (c *CLIClient) SurfaceDirectories() (map[string]string, error) {
	path, err := newestSessionFile(c.sessionDir())
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	panelDirs, err := parseSessionSurfaceDirectories(data)
	if err != nil {
		return nil, err
	}
	if len(panelDirs) == 0 {
		return map[string]string{}, nil
	}

	// --all: save captures the CALLER's window, which need not be the focused
	// one, and the scoped tree omits every other window — without this the
	// UUID→ref join comes back empty and each tab's folder is silently lost.
	out, err := c.run("tree", "--all", "--json", "--id-format", "both")
	if err != nil {
		return nil, err
	}
	idToRef, err := parseTreeSurfaceIDs(out)
	if err != nil {
		return nil, err
	}
	return composeSurfaceDirs(panelDirs, idToRef), nil
}
