package orchestrate

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/detect"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// Saver captures the current cmux state and persists it.
type Saver struct {
	Client client.Backend
	Store  persist.Store
}

// Save captures the live cmux state and writes it to the store.
func (s *Saver) Save(name, description string) (*model.Layout, error) {
	tree, err := s.Client.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	if len(tree.Windows) == 0 {
		return nil, fmt.Errorf("no windows found")
	}

	// Use the first (typically only) window.
	win := tree.Windows[0]

	layout := &model.Layout{
		Name:        name,
		Description: description,
		Version:     1,
		SavedAt:     time.Now().UTC(),
	}

	// Deduplicate workspaces with the same title. cmux can report ghost
	// workspaces (stale refs with no tty). When duplicates exist, keep the
	// one with the most panes that have ttys.
	workspaces := deduplicateWorkspaces(win.Workspaces)

	// Titles of workspaces whose split geometry was resolved from live pixel
	// frames. For these, live geometry is authoritative and a previously saved
	// split direction must not override it on re-save (GitHub #8 follow-up).
	geoTitles := make(map[string]bool)
	for _, tw := range workspaces {
		ws, geometryApplied, err := s.buildWorkspace(tw)
		if err != nil {
			// Log but don't fail — isolate errors per workspace.
			fmt.Fprintf(os.Stderr, "  warning: workspace %q: %v\n", tw.Title, err)
			continue
		}
		if geometryApplied {
			geoTitles[ws.Title] = true
		}
		layout.Workspaces = append(layout.Workspaces, *ws)
	}

	if len(layout.Workspaces) == 0 {
		return nil, fmt.Errorf("no workspaces could be captured")
	}

	// Load existing layout for merge and revision tracking.
	existing, loadErr := s.Store.Load(name)

	// If a TOML already exists, merge user-edited fields (description, commands).
	if loadErr == nil {
		mergeUserEdits(layout, existing, geoTitles)
	}

	// Clear all auto-detected commands before re-detection. Each save is
	// a fresh snapshot — detection re-assigns from current state.
	// (The 500-byte session file filter ensures re-detection picks the
	// correct active session, not placeholder files from failed resumes.)
	clearAutoDetectedCommands(layout)
	detected := detect.AISessions()

	// Auto-detect running AI CLI sessions and populate resume commands.
	// Surface titles from the tree confirm which panes actually run an AI CLI,
	// preventing false matches when multiple workspaces share a CWD.
	if os.Getenv("CREX_DEBUG") != "" {
		debugDetection(layout, win.Workspaces)
	}
	applyDetectedSessions(layout, win.Workspaces, detected)

	// Track revision: increment if content changed, preserve if identical.
	if loadErr == nil && existing != nil {
		if layoutContentChanged(layout, existing) {
			layout.Revision = existing.Revision + 1
		} else {
			layout.Revision = existing.Revision
		}
	} else {
		layout.Revision = 1
	}

	if err := s.Store.Save(name, layout); err != nil {
		return nil, fmt.Errorf("save layout: %w", err)
	}
	return layout, nil
}

// surfaceCWD returns the live working directory of a surface's shell. It
// prefers the TTY foreground process (most precise when a foreground command
// cd'd deeper); then a CWD the backend reported in the tree itself (Ghostty);
// then the backend's live surface state (`cmux rpc debug.terminals`, since
// current cmux builds report no tty in `tree --json`).
func (s *Saver) surfaceCWD(surf client.TreeSurface) string {
	if surf.TTY != "" {
		if cwd := detect.ForegroundCWD(surf.TTY); cwd != "" {
			return cwd
		}
	}
	if surf.CWD != "" {
		return surf.CWD
	}
	ss, ok := s.Client.(client.SurfaceStater)
	if !ok || surf.Ref == "" {
		return ""
	}
	st, err := ss.SurfaceState(surf.Ref)
	if err != nil || st == nil {
		return ""
	}
	return st.CWD
}

// buildWorkspace captures one workspace. The returned bool reports whether
// pane geometry was resolved from live pixel frames (true) or fell back to the
// default right-chain (false); the merge step uses it to decide whether a
// previously saved split direction may override the live one.
func (s *Saver) buildWorkspace(tw client.TreeWorkspace) (*model.Workspace, bool, error) {
	// Get CWD from sidebar-state.
	sidebar, err := s.Client.SidebarState(tw.Ref)
	if err != nil {
		return nil, false, fmt.Errorf("sidebar-state: %w", err)
	}

	ws := &model.Workspace{
		Title:  tw.Title,
		CWD:    sidebar.CWD,
		Pinned: tw.Pinned,
		Index:  tw.Index,
		Active: tw.Active || tw.Selected,
	}

	// Sort panes by index.
	panes := make([]client.TreePane, len(tw.Panes))
	copy(panes, tw.Panes)
	sort.Slice(panes, func(i, j int) bool {
		return panes[i].Index < panes[j].Index
	})

	for i, tp := range panes {
		pane := model.Pane{
			Type:        "terminal",
			Focus:       tp.Focused,
			Index:       tp.Index,
			FocusTarget: -1, // no refocus needed for saved layouts
		}

		// First pane has no split direction; subsequent default to "right".
		if i > 0 {
			pane.Split = "right"
		}

		// Use surface info for type, URL, and foreground command.
		if len(tp.Surfaces) > 0 {
			// Surface 0 → flat Pane fields (backward-compatible).
			surf := tp.Surfaces[0]
			pane.Type = surf.Type
			if surf.URL != nil {
				pane.URL = *surf.URL
			}
			if surf.Type == "terminal" {
				if surf.TTY != "" {
					if cmd := detect.ForegroundCommand(surf.TTY); cmd != "" {
						pane.Command = cmd
					}
				}
				// Capture a per-pane CWD only when it differs from the workspace
				// CWD, so restore can recreate this pane in its own directory
				// (GitHub #8) without cluttering the layout with redundant paths.
				if cwd := s.surfaceCWD(surf); cwd != "" && cwd != ws.CWD {
					pane.CWD = cwd
				}
			}

			// Surfaces 1..N → Pane.Surfaces (extra tabs in this pane).
			for _, extra := range tp.Surfaces[1:] {
				es := model.Surface{
					Type: extra.Type,
				}
				if extra.URL != nil {
					es.URL = *extra.URL
				}
				if extra.Type == "terminal" {
					if extra.TTY != "" {
						if cmd := detect.ForegroundCommand(extra.TTY); cmd != "" {
							es.Command = cmd
						}
					}
					if cwd := s.surfaceCWD(extra); cwd != "" && cwd != ws.CWD {
						es.CWD = cwd
					}
				}
				pane.Surfaces = append(pane.Surfaces, es)
			}
		}

		ws.Panes = append(ws.Panes, pane)
	}

	// Infer split directions from pane pixel geometry when available.
	geometryApplied := false
	if gp, ok := s.Client.(client.PaneGeometryProvider); ok {
		if paneList, err := gp.PaneList(tw.Ref); err == nil {
			geometryApplied = applySplitGeometry(ws, paneList)
		}
		// Silently fall back to default "right" if PaneList fails.
	}

	// Ensure at least one pane.
	if len(ws.Panes) == 0 {
		ws.Panes = []model.Pane{{Type: "terminal", Focus: true}}
	}

	return ws, geometryApplied, nil
}

// applySplitGeometry uses pane pixel geometry to set correct split directions,
// ratios, and focus targets on the workspace panes, and reorders the panes
// into a valid creation order. cmux indexes panes visually, and that order
// is not always buildable by sequential splits (a full-height right pane must
// exist before the left column is split under it) — so the saved pane array
// follows the inferred creation sequence, not the index order.
// It returns true when the geometry was resolved and applied, false when it
// bailed to the default right-chain (single pane, degenerate frames, or a
// tree/pane.list mismatch).
func applySplitGeometry(ws *model.Workspace, paneList *client.PaneListResponse) bool {
	if len(paneList.Panes) <= 1 || len(ws.Panes) <= 1 {
		return false
	}

	order := InferCreationOrder(paneList.Panes)
	if order == nil || len(order) != len(ws.Panes) {
		return false // BSP reconstruction failed or tree/pane.list mismatch, keep defaults
	}

	// Build lookup by cmux pane index.
	byIndex := make(map[int]model.Pane, len(ws.Panes))
	for _, p := range ws.Panes {
		byIndex[p.Index] = p
	}

	reordered := make([]model.Pane, 0, len(order))
	for i, step := range order {
		pane, ok := byIndex[step.PaneIndex]
		if !ok {
			return false // pane.list index not in tree, keep defaults
		}
		if i == 0 {
			pane.Split = ""
			pane.SplitRatio = 0
			pane.FocusTarget = -1
		} else {
			pane.Split = step.Direction
			pane.FocusTarget = step.FocusTarget
			pane.SplitRatio = 0
			// Only store ratio if it's meaningfully different from 0.5 (equal split).
			if step.Ratio > 0 && (step.Ratio < 0.48 || step.Ratio > 0.52) {
				pane.SplitRatio = step.Ratio
			}
		}
		reordered = append(reordered, pane)
	}
	ws.Panes = reordered
	return true
}

// deduplicateWorkspaces removes ghost workspaces that share a title with
// a real workspace. When duplicates exist, the workspace with the most
// panes that have ttys wins. Ghost workspaces in cmux are stale refs that
// appear in the tree but have no active terminals.
func deduplicateWorkspaces(workspaces []client.TreeWorkspace) []client.TreeWorkspace {
	type candidate struct {
		index    int
		ttyCount int
	}

	best := make(map[string]candidate) // title → best candidate
	for i, ws := range workspaces {
		ttys := 0
		for _, p := range ws.Panes {
			for _, s := range p.Surfaces {
				if s.TTY != "" {
					ttys++
				}
			}
		}
		prev, exists := best[ws.Title]
		if !exists || ttys > prev.ttyCount {
			best[ws.Title] = candidate{index: i, ttyCount: ttys}
		}
	}

	// If no duplicates found, return as-is (fast path).
	if len(best) == len(workspaces) {
		return workspaces
	}

	// Build deduplicated list preserving original order.
	kept := make(map[int]bool, len(best))
	for _, c := range best {
		kept[c.index] = true
	}
	var result []client.TreeWorkspace
	for i, ws := range workspaces {
		if kept[i] {
			result = append(result, ws)
		}
	}
	return result
}

// debugDetection prints detection diagnostics when CREX_DEBUG is set.
func debugDetection(layout *model.Layout, treeWorkspaces []client.TreeWorkspace) {
	detected := detect.AISessions()
	fmt.Fprintf(os.Stderr, "\n  [debug] Detected sessions:\n")
	for cwd, sessions := range detected.ByCWD {
		for _, s := range sessions {
			fmt.Fprintf(os.Stderr, "    tool=%s cwd=%s cmd=%s\n", s.Tool, cwd, s.Command)
		}
	}
	fmt.Fprintf(os.Stderr, "  [debug] Surface titles:\n")
	for _, tw := range treeWorkspaces {
		for _, tp := range tw.Panes {
			for _, s := range tp.Surfaces {
				fmt.Fprintf(os.Stderr, "    ws=%q pane=%d title=%q\n", tw.Title, tp.Index, s.Title)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "  [debug] Layout workspaces:\n")
	for _, ws := range layout.Workspaces {
		fmt.Fprintf(os.Stderr, "    ws=%q cwd=%s panes=%d\n", ws.Title, ws.CWD, len(ws.Panes))
	}
	fmt.Fprintln(os.Stderr)
}

// aiResumePatterns matches commands that were auto-detected in a previous save.
// These are cleared before re-detection to prevent stale commands from persisting.
var aiResumePatterns = []string{
	"claude --resume ",
	"claude --continue",
	"opencode --session ",
	"codex resume ",
	"amp threads continue ",
	"gemini --resume ",
	"copilot --continue",
	"grok --continue",
}

// clearAutoDetectedCommands removes all AI resume commands from the layout.
// Called before re-detection so each save starts fresh. User-set commands
// (like "npm run dev") are kept because they don't match AI patterns.
func clearAutoDetectedCommands(layout *model.Layout) {
	for i := range layout.Workspaces {
		for j := range layout.Workspaces[i].Panes {
			cmd := layout.Workspaces[i].Panes[j].Command
			for _, pattern := range aiResumePatterns {
				if strings.HasPrefix(cmd, pattern) {
					layout.Workspaces[i].Panes[j].Command = ""
					break
				}
			}
		}
	}
}

// aiProcessNames contains bare AI tool binary names (e.g. "claude", "opencode").
// These are cleared before AI detection so the specialized detector can assign
// full resume commands instead.
var aiProcessNames = detect.ProcessNames()

// aiTitlePatterns is populated from the detector registry in the detect package.
// Each tool's title patterns and detection logic are co-located there.
var aiTitlePatterns = detect.TitlePatterns()

// applyDetectedSessions scans for running AI CLI sessions (Claude Code,
// OpenCode, Codex) and sets the resume command on matching panes.
// Detection is best-effort: if anything fails, panes are left unchanged.
//
// Matching strategy (two passes):
//  1. Title-confirmed: both CWD and surface title agree → highest confidence.
//  2. CWD-only fallback: for tools that don't set a recognizable title,
//     match by CWD alone — but only if no other workspace already claimed
//     that CWD in pass 1.
//
// Each CWD is consumed after the first match to prevent duplicates.
func applyDetectedSessions(layout *model.Layout, treeWorkspaces []client.TreeWorkspace, detected detect.DetectedSessions) {
	if len(detected.ByCWD) == 0 {
		return
	}

	// Build a lookup of surface titles by workspace title + pane index.
	type paneKey struct {
		wsTitle string
		paneIdx int
	}
	surfaceTitles := make(map[paneKey]string)
	for _, tw := range treeWorkspaces {
		for _, tp := range tw.Panes {
			for _, s := range tp.Surfaces {
				surfaceTitles[paneKey{tw.Title, tp.Index}] = s.Title
				break // first surface per pane is enough
			}
		}
	}

	consumed := make(map[string]bool) // consumed session commands (unique per session)

	// findSession returns an unconsumed session for the given tool from a CWD list.
	findSession := func(sessions []detect.Session, tool string) *detect.Session {
		for i := range sessions {
			if sessions[i].Tool == tool && !consumed[sessions[i].Command] {
				return &sessions[i]
			}
		}
		return nil
	}

	// Pass 1a: Title + CWD match (highest confidence). Assign sessions only
	// when both the pane title confirms the tool AND the workspace CWD matches
	// a detected session's CWD. This prevents CWD-mismatched workspaces from
	// stealing sessions that belong to other workspaces.
	for i := range layout.Workspaces {
		ws := &layout.Workspaces[i]
		for j := range ws.Panes {
			if ws.Panes[j].Type != "terminal" {
				continue
			}
			title := surfaceTitles[paneKey{ws.Title, j}]
			for tool, patterns := range aiTitlePatterns {
				if !titleMatchesAI(title, patterns) {
					continue
				}
				if s := findSession(detected.ByCWD[ws.CWD], tool); s != nil {
					ws.Panes[j].Command = s.Command
					consumed[s.Command] = true
				}
				break
			}
		}
	}

	// Pass 1b: Title match only (fallback). For panes with a matching title
	// but no CWD-matched session, assign any unconsumed session for that tool.
	for i := range layout.Workspaces {
		ws := &layout.Workspaces[i]
		for j := range ws.Panes {
			if ws.Panes[j].Type != "terminal" {
				continue
			}
			// Skip if already assigned in Pass 1a.
			if ws.Panes[j].Command != "" && !aiProcessNames[ws.Panes[j].Command] {
				continue
			}
			title := surfaceTitles[paneKey{ws.Title, j}]
			for tool, patterns := range aiTitlePatterns {
				if !titleMatchesAI(title, patterns) {
					continue
				}
				if s := findSession(detected.ByTool[tool], tool); s != nil {
					ws.Panes[j].Command = s.Command
					consumed[s.Command] = true
				}
				break
			}
		}
	}

	// Pass 2: CWD-only fallback for tools that don't set a recognizable
	// title (e.g. Codex). Restricted to single-pane workspaces.
	for i := range layout.Workspaces {
		ws := &layout.Workspaces[i]
		if len(ws.Panes) != 1 || ws.Panes[0].Type != "terminal" {
			continue
		}
		// Allow upgrade if the command is empty or a bare AI tool name
		// (set by foreground detection, e.g. "claude" without --resume).
		if ws.Panes[0].Command != "" && !aiProcessNames[ws.Panes[0].Command] {
			continue
		}
		sessions := detected.ByCWD[ws.CWD]
		for _, s := range sessions {
			if !consumed[s.Command] {
				ws.Panes[0].Command = s.Command
				consumed[s.Command] = true
				break
			}
		}
	}
}

// titleMatchesAI checks whether a surface title contains any of the
// given AI tool name patterns (case-insensitive).
func titleMatchesAI(title string, patterns []string) bool {
	lower := strings.ToLower(title)
	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// layoutContentChanged compares the structural content of two layouts,
// ignoring metadata fields (SavedAt, Description, Revision).
func layoutContentChanged(a, b *model.Layout) bool {
	if a == nil || b == nil {
		return a != b
	}
	if len(a.Workspaces) != len(b.Workspaces) {
		return true
	}
	for i := range a.Workspaces {
		wa, wb := &a.Workspaces[i], &b.Workspaces[i]
		if wa.Title != wb.Title || wa.CWD != wb.CWD || wa.Pinned != wb.Pinned || wa.Active != wb.Active {
			return true
		}
		if len(wa.Panes) != len(wb.Panes) {
			return true
		}
		for j := range wa.Panes {
			pa, pb := &wa.Panes[j], &wb.Panes[j]
			if pa.Type != pb.Type || pa.Split != pb.Split || pa.Command != pb.Command || pa.URL != pb.URL || pa.Focus != pb.Focus || pa.SplitRatio != pb.SplitRatio {
				return true
			}
		}
	}
	return false
}

// mergeUserEdits preserves fields from an existing TOML that the live tree
// can't report: the workspace description, and user-typed commands.
//
// Panes are matched by their cmux pane Index (visual position), NOT by array
// position, because save reorders panes into a valid creation sequence — a
// positional match would apply a previous save's fields to the wrong pane and
// scramble re-saved layouts (GitHub #8 follow-up: re-saving over a name must
// reproduce the same layout).
//
// geoTitles lists workspaces whose split direction was resolved from live
// geometry; for those, live geometry is authoritative and a previously saved
// split is never reapplied. Split preservation survives only for workspaces
// with no geometry (e.g. never-rendered), where the default right-chain would
// otherwise clobber a hand-edited direction.
func mergeUserEdits(live, existing *model.Layout, geoTitles map[string]bool) {
	if live.Description == "" && existing.Description != "" {
		live.Description = existing.Description
	}

	// Build index of existing workspaces by title for matching.
	existByTitle := make(map[string]*model.Workspace)
	for i := range existing.Workspaces {
		existByTitle[existing.Workspaces[i].Title] = &existing.Workspaces[i]
	}

	for i := range live.Workspaces {
		lw := &live.Workspaces[i]
		ew, ok := existByTitle[lw.Title]
		if !ok {
			continue
		}
		// Preserve user-set workspace description (live tree doesn't expose it).
		if lw.Description == "" && ew.Description != "" {
			lw.Description = ew.Description
		}

		// Match existing panes by cmux pane Index (stable across reorder).
		exByIndex := make(map[int]*model.Pane, len(ew.Panes))
		for k := range ew.Panes {
			exByIndex[ew.Panes[k].Index] = &ew.Panes[k]
		}
		geoResolved := geoTitles[lw.Title]

		for j := range lw.Panes {
			lp := &lw.Panes[j]
			ep, ok := exByIndex[lp.Index]
			if !ok {
				continue
			}
			// Preserve a hand-edited split direction only when live geometry
			// did NOT resolve it. When geometry is authoritative, trust it —
			// reapplying a stale split mirrored aside layouts on re-save.
			if !geoResolved && lp.Split == "right" && ep.Split != "" && ep.Split != "right" {
				lp.Split = ep.Split
			}
			// Preserve user-set command, but never for browser panes
			// (browser panes don't run shell commands; a stale command
			// would leak through).
			if ep.Command != "" && lp.Type != "browser" {
				lp.Command = ep.Command
			}
		}
	}
}
