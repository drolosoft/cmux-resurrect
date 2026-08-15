package orchestrate

import (
	"encoding/json"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// buildCmuxLayout converts a workspace's flat pane list into cmux's
// `workspace create --layout` JSON: a split tree with per-surface cwd, name,
// url, and focus. Creating the whole workspace atomically means no typed
// `cd` per pane at all — no scrollback junk, no readiness races, exact split
// ratios (GitHub #8 family).
//
// Commands are deliberately NOT embedded: a layout command replaces the
// shell, so a finished command would close the pane. crex types commands
// after creation instead (readiness-gated), keeping AI-resume semantics.
//
// The second return is false when the workspace isn't representable (single
// plain pane — the plain --cwd path is simpler — or malformed focus targets);
// callers fall back to the sequential restore path.
// It also returns visualIdx: for each pane (creation order), the visual pane
// index cmux will assign in the created workspace (left-to-right, then
// top-to-bottom) — used to target post-creation command sends.
func buildCmuxLayout(ws model.Workspace) (string, []int, bool) {
	if len(ws.Panes) == 0 {
		return "", nil, false
	}
	if len(ws.Panes) == 1 && len(ws.Panes[0].Surfaces) == 0 {
		return "", nil, false
	}
	// Browser profiles cannot ride the atomic path: cmux (verified on 0.64.22)
	// silently ignores a surface's "profile" key in `workspace create --layout`
	// and opens the pane on the last-used profile, whereas `new-pane --profile`
	// honors it. Fall back to the sequential path so the profile is applied —
	// the layout gains nothing atomic that outweighs landing on the wrong login.
	if hasBrowserProfile(ws) {
		return "", nil, false
	}

	// Mutable tree: a node is either a leaf (surfaces set) or a split.
	type lnode struct {
		dir      string // "horizontal" | "vertical" for splits
		split    float64
		children [2]*lnode
		paneIdx  int // creation-array index for leaves, -1 for splits
	}

	// The rect-replay (which pane does each split target, and where does every
	// pane end up visually) lives in resolveSplitTargets, shared with the
	// sequential restore path.
	targets, finalVisual, ok := resolveSplitTargets(ws)
	if !ok {
		return "", nil, false
	}

	root := &lnode{paneIdx: 0}
	leafOf := map[int]*lnode{0: root}

	for i := 1; i < len(ws.Panes); i++ {
		p := ws.Panes[i]
		target := targets[i]

		dir := p.Split
		if dir == "" {
			dir = "right"
		}
		newFrac := p.SplitRatio
		if newFrac <= 0 || newFrac >= 1 {
			newFrac = 0.5
		}

		var cmuxDir string
		var first, second int // pane order within the split
		var firstFrac float64
		switch dir {
		case "right":
			cmuxDir, first, second, firstFrac = "horizontal", target, i, 1-newFrac
		case "left":
			cmuxDir, first, second, firstFrac = "horizontal", i, target, newFrac
		case "down":
			cmuxDir, first, second, firstFrac = "vertical", target, i, 1-newFrac
		case "up":
			cmuxDir, first, second, firstFrac = "vertical", i, target, newFrac
		default:
			return "", nil, false
		}

		// Mutate the target's leaf into a split holding both panes.
		leaf := leafOf[target]
		a := &lnode{paneIdx: first}
		b := &lnode{paneIdx: second}
		leaf.paneIdx = -1
		leaf.dir = cmuxDir
		leaf.split = firstFrac
		leaf.children = [2]*lnode{a, b}
		leafOf[first] = a
		leafOf[second] = b
	}

	// Serialize the tree.
	type surfaceJSON struct {
		Type string `json:"type"`
		Name string `json:"name,omitempty"`
		CWD  string `json:"cwd,omitempty"`
		URL  string `json:"url,omitempty"`
		// Profile is the browser profile slug. cmux ≤0.64 ignores unknown
		// layout keys, so embedding it is forward-compatible: it starts
		// working the moment cmux honors profiles in layout specs.
		Profile string `json:"profile,omitempty"`
		Focus   bool   `json:"focus,omitempty"`
	}
	type paneJSON struct {
		Surfaces []surfaceJSON `json:"surfaces"`
	}
	type nodeJSON struct {
		Direction string     `json:"direction,omitempty"`
		Split     *float64   `json:"split,omitempty"`
		Children  []nodeJSON `json:"children,omitempty"`
		Pane      *paneJSON  `json:"pane,omitempty"`
	}

	surfacesFor := func(p model.Pane) []surfaceJSON {
		typ := p.Type
		if typ == "" {
			typ = "terminal"
		}
		// Terminal leaves always get an explicit cwd: layouts saved by older
		// versions elide a pane's cwd when it equals the workspace cwd, and
		// an empty leaf cwd would leave the pane wherever cmux spawns it.
		cwd := p.CWD
		if cwd == "" && typ == "terminal" {
			cwd = ws.CWD
		}
		out := []surfaceJSON{{
			Type:    typ,
			Name:    p.Name,
			CWD:     expandHomeNonEmpty(cwd),
			URL:     p.URL,
			Profile: browserProfile(typ, p.Profile),
			Focus:   p.Focus,
		}}
		for _, s := range p.Surfaces {
			st := s.Type
			if st == "" {
				st = "terminal"
			}
			scwd := s.CWD
			if scwd == "" && st == "terminal" {
				scwd = ws.CWD
			}
			out = append(out, surfaceJSON{
				Type:    st,
				Name:    s.Name,
				CWD:     expandHomeNonEmpty(scwd),
				URL:     s.URL,
				Profile: browserProfile(st, s.Profile),
			})
		}
		return out
	}

	var toJSON func(n *lnode) nodeJSON
	toJSON = func(n *lnode) nodeJSON {
		if n.paneIdx >= 0 {
			return nodeJSON{Pane: &paneJSON{Surfaces: surfacesFor(ws.Panes[n.paneIdx])}}
		}
		out := nodeJSON{
			Direction: n.dir,
			Children:  []nodeJSON{toJSON(n.children[0]), toJSON(n.children[1])},
		}
		if n.split > 0.505 || n.split < 0.495 {
			s := n.split
			out.Split = &s
		}
		return out
	}

	data, err := json.Marshal(toJSON(root))
	if err != nil {
		return "", nil, false
	}
	return string(data), finalVisual, true
}

// hasBrowserProfile reports whether any browser pane or tab carries a
// non-default profile.
func hasBrowserProfile(ws model.Workspace) bool {
	for _, p := range ws.Panes {
		if p.Type == "browser" && p.Profile != "" {
			return true
		}
		for _, s := range p.Surfaces {
			if s.Type == "browser" && s.Profile != "" {
				return true
			}
		}
	}
	return false
}

// browserProfile guards the profile slug to browser surfaces only, so a
// stale profile on a hand-edited terminal pane can't leak into the layout.
func browserProfile(surfaceType, profile string) string {
	if surfaceType != "browser" {
		return ""
	}
	return profile
}

// expandHomeNonEmpty expands ~ but keeps "" as "" (omitted in JSON → the
// surface inherits the workspace cwd).
func expandHomeNonEmpty(p string) string {
	if p == "" {
		return ""
	}
	return expandHome(p)
}
