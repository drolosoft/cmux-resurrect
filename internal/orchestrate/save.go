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
	tree, err := s.tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	if len(tree.Windows) == 0 {
		return nil, fmt.Errorf("no windows found")
	}

	// Capture the window the user is looking at. A layout describes one window's
	// worth of workspaces, so with several cmux windows open the current one is
	// the only defensible choice — taking the first would store a different
	// session than the one on screen.
	win := currentWindow(tree)

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

	// Per-surface data the tree can't report, gathered once per save.
	extras := s.gatherExtras()

	// Titles of workspaces whose split geometry was resolved from live pixel
	// frames. For these, live geometry is authoritative and a previously saved
	// split direction must not override it on re-save (GitHub #8 follow-up).
	geoTitles := make(map[string]bool)
	for _, tw := range workspaces {
		ws, geometryApplied, err := s.buildWorkspace(tw, extras)
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

// tree fetches the workspace tree for a save. It asks for EVERY window when
// the backend can, because a backend's default tree may be scoped to the
// focused window — which is not necessarily the one crex was run in. Falling
// back to the scoped tree keeps older backends working.
func (s *Saver) tree() (*client.TreeResponse, error) {
	if mw, ok := s.Client.(client.MultiWindowTreeProvider); ok {
		if t, err := mw.TreeAllWindows(); err == nil && t != nil && len(t.Windows) > 0 {
			return t, nil
		}
	}
	return s.Client.Tree()
}

// currentWindow picks the window a save should describe. A layout covers one
// window's worth of workspaces, so with several windows open the only
// defensible choice is the one the user is working in — taking the first would
// store a different session than the one on screen.
//
// In practice cmux already scopes `tree --json` to the caller's window, so this
// is a safety net rather than a hot path; it matters for any caller that hands
// us a multi-window tree. Order: the window holding the caller (ground truth
// for "where crex was run"), then the backend's own current/active flags, then
// the first. Callers must pass a non-empty slice.
func currentWindow(tree *client.TreeResponse) client.TreeWindow {
	windows := tree.Windows
	for _, ref := range []string{callerWindowRef(tree.Caller), callerWindowRef(tree.Active)} {
		if ref == "" {
			continue
		}
		for _, w := range windows {
			if w.Ref == ref {
				return w
			}
		}
	}
	for _, w := range windows {
		if w.Current {
			return w
		}
	}
	for _, w := range windows {
		if w.Active {
			return w
		}
	}
	return windows[0]
}

func callerWindowRef(c *client.CallerInfo) string {
	if c == nil {
		return ""
	}
	return c.WindowRef
}

// liveExtras is per-surface data gathered once per save from sources outside
// the tree — the backend's persisted session state. Both maps are keyed by
// surface ref and may be nil (backend can't report, or reporting failed);
// a nil map simply yields no information.
type liveExtras struct {
	profiles map[string]string // browser profile slug (GitHub #9)
	dirs     map[string]string // persisted working directory (GitHub #8)
}

// gatherExtras collects the optional per-surface data a backend can report.
// Every failure is soft: this is enrichment, never a reason to fail a save.
func (s *Saver) gatherExtras() liveExtras {
	var e liveExtras
	if pp, ok := s.Client.(client.BrowserProfileProvider); ok {
		if m, err := pp.SurfaceProfiles(); err == nil {
			e.profiles = m
		}
	}
	if dp, ok := s.Client.(client.SurfaceDirectoryProvider); ok {
		if m, err := dp.SurfaceDirectories(); err == nil {
			e.dirs = m
		}
	}
	return e
}

// surfaceCWD returns the working directory of a surface, most trustworthy
// source first:
//
//  1. the TTY foreground process — most precise when a foreground command
//     cd'd deeper than the shell;
//  2. a CWD the backend reported in the tree itself (Ghostty);
//  3. the live surface state (`cmux rpc debug.terminals`) when its shell is
//     READY — a running shell is authoritative, the user may have cd'd;
//  4. the directory the backend persisted for that surface. cmux spawns a
//     tab's shell lazily on first render, and until then it reports the
//     WORKSPACE directory for that tab — which collapsed every unopened tab
//     onto the first tab's path (GitHub #8). The persisted value survives
//     the lazy spawn;
//  5. the live state even when not ready — last resort, better than nothing.
func (s *Saver) surfaceCWD(wsRef string, surf client.TreeSurface, extras liveExtras) string {
	if surf.TTY != "" {
		if cwd := detect.ForegroundCWD(surf.TTY); cwd != "" {
			return cwd
		}
	}
	if surf.CWD != "" {
		return surf.CWD
	}

	var live *client.SurfaceState
	if ss, ok := s.Client.(client.SurfaceStater); ok && surf.Ref != "" {
		if st, err := ss.SurfaceState(wsRef, surf.Ref); err == nil {
			live = st
		}
	}
	if live != nil && live.Ready && live.CWD != "" {
		return live.CWD
	}
	if dir := extras.dirs[surf.Ref]; dir != "" {
		return dir
	}
	if live != nil {
		return live.CWD
	}
	return ""
}

// buildWorkspace captures one workspace. The returned bool reports whether
// pane geometry was resolved from live pixel frames (true) or fell back to the
// default right-chain (false); the merge step uses it to decide whether a
// previously saved split direction may override the live one.
func (s *Saver) buildWorkspace(tw client.TreeWorkspace, extras liveExtras) (*model.Workspace, bool, error) {
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
			if surf.Type == "browser" {
				pane.Profile = extras.profiles[surf.Ref]
			}
			if surf.Type == "terminal" {
				if surf.TTY != "" {
					if cmd := detect.ForegroundCommand(surf.TTY); cmd != "" {
						pane.Command = cmd
					}
				}
				// Always capture the per-pane CWD (GitHub #8). Eliding it when
				// it equals the workspace CWD loses the path on restore: only
				// the creation-first pane inherits the workspace CWD — a split
				// with no cwd gets no cd and lands wherever the backend spawns
				// it (2026-07-11 audit: Ghostty save-back lost the focused
				// split's folder because the sidebar CWD matched it).
				if cwd := s.surfaceCWD(tw.Ref, surf, extras); cwd != "" {
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
				if extra.Type == "browser" {
					es.Profile = extras.profiles[extra.Ref]
				}
				if extra.Type == "terminal" {
					if extra.TTY != "" {
						if cmd := detect.ForegroundCommand(extra.TTY); cmd != "" {
							es.Command = cmd
						}
					}
					if cwd := s.surfaceCWD(tw.Ref, extra, extras); cwd != "" {
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
	clear := func(cmd *string) {
		for _, pattern := range aiResumePatterns {
			if strings.HasPrefix(*cmd, pattern) {
				*cmd = ""
				return
			}
		}
	}
	for i := range layout.Workspaces {
		for j := range layout.Workspaces[i].Panes {
			p := &layout.Workspaces[i].Panes[j]
			clear(&p.Command)
			// Extra tabs carry resume commands too (GitHub #8); a stale
			// session id there would survive every re-save.
			for k := range p.Surfaces {
				clear(&p.Surfaces[k].Command)
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

	// Live surface titles by workspace title + pane index + surface index.
	// EVERY surface is recorded, not just the first of each pane: a pane's
	// extra tabs each run their own AI session (GitHub #8).
	surfaceTitles := make(map[surfaceTitleKey]string)
	for _, tw := range treeWorkspaces {
		for _, tp := range tw.Panes {
			for k, s := range tp.Surfaces {
				surfaceTitles[surfaceTitleKey{tw.Title, tp.Index, k}] = s.Title
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
	// when both the surface title confirms the tool AND a CWD matches a
	// detected session's CWD. This prevents CWD-mismatched workspaces from
	// stealing sessions that belong to other workspaces. Each slot is matched
	// on its OWN cwd first (a tab per git worktree resolves to its own
	// session), then on the workspace cwd.
	for i := range layout.Workspaces {
		ws := &layout.Workspaces[i]
		for _, sl := range commandSlots(ws, surfaceTitles) {
			for tool, patterns := range aiTitlePatterns {
				if !titleMatchesAI(sl.title, patterns) {
					continue
				}
				for _, cwd := range sl.searchCWDs() {
					if s := findSession(detected.ByCWD[cwd], tool); s != nil {
						*sl.cmd = s.Command
						consumed[s.Command] = true
						break
					}
				}
				break
			}
		}
	}

	// Pass 1b: Title match only (fallback). For slots with a matching title
	// but no CWD-matched session, assign any unconsumed session for that tool.
	for i := range layout.Workspaces {
		ws := &layout.Workspaces[i]
		for _, sl := range commandSlots(ws, surfaceTitles) {
			// Skip if already assigned in Pass 1a.
			if sl.assigned() {
				continue
			}
			for tool, patterns := range aiTitlePatterns {
				if !titleMatchesAI(sl.title, patterns) {
					continue
				}
				if s := findSession(detected.ByTool[tool], tool); s != nil {
					*sl.cmd = s.Command
					consumed[s.Command] = true
				}
				break
			}
		}
	}

	// Pass 2: CWD-only fallback for tools that don't set a recognizable
	// title (e.g. Codex). Restricted to single-pane workspaces, where a slot's
	// cwd identifies it unambiguously.
	for i := range layout.Workspaces {
		ws := &layout.Workspaces[i]
		if len(ws.Panes) != 1 || ws.Panes[0].Type != "terminal" {
			continue
		}
		for _, sl := range commandSlots(ws, surfaceTitles) {
			// Allow upgrade if the command is empty or a bare AI tool name
			// (set by foreground detection, e.g. "claude" without --resume).
			if sl.assigned() {
				continue
			}
			// Any tool: this pass exists for tools with no title signal.
			claimed := false
			for _, cwd := range sl.searchCWDs() {
				for _, s := range detected.ByCWD[cwd] {
					if !consumed[s.Command] {
						*sl.cmd = s.Command
						consumed[s.Command] = true
						claimed = true
						break
					}
				}
				if claimed {
					break
				}
			}
		}
	}
}

// surfaceTitleKey addresses one live surface: workspace title + the pane's
// stable cmux Index (array position drifts — applySplitGeometry reorders panes
// into creation order) + the surface's position within the pane.
type surfaceTitleKey struct {
	wsTitle string
	paneIdx int
	surfIdx int
}

// commandSlot is one command-bearing terminal target — a pane (its first
// surface) or one of the pane's extra tabs. AI detection writes through these
// so tabs are first-class targets, not just panes (GitHub #8).
type commandSlot struct {
	cmd   *string // the Command field to fill
	title string  // live surface title, used to confirm which tool runs here
	cwd   string  // the slot's own working directory ("" when not captured)
	wsCWD string  // the enclosing workspace's directory
}

// searchCWDs lists the directories to match a session against, most specific
// first. The workspace cwd is kept as a fallback so layouts saved before
// per-pane cwd capture still resolve their sessions.
func (s commandSlot) searchCWDs() []string {
	if s.cwd == "" || s.cwd == s.wsCWD {
		return []string{s.wsCWD}
	}
	return []string{s.cwd, s.wsCWD}
}

// assigned reports whether this slot already holds a real command. A bare AI
// tool name (e.g. "claude", from foreground detection) counts as unassigned so
// a later pass can upgrade it to a full resume command.
func (s commandSlot) assigned() bool {
	return *s.cmd != "" && !aiProcessNames[*s.cmd]
}

// commandSlots enumerates a workspace's terminal panes and their extra tabs,
// pairing each with its live surface title.
func commandSlots(ws *model.Workspace, titles map[surfaceTitleKey]string) []commandSlot {
	var out []commandSlot
	for j := range ws.Panes {
		p := &ws.Panes[j]
		if p.Type != "terminal" {
			continue
		}
		out = append(out, commandSlot{
			cmd:   &p.Command,
			title: titles[surfaceTitleKey{ws.Title, p.Index, 0}],
			cwd:   p.CWD,
			wsCWD: ws.CWD,
		})
		for k := range p.Surfaces {
			s := &p.Surfaces[k]
			// Surface.Type is omitted for terminals in older layouts.
			if s.Type != "" && s.Type != "terminal" {
				continue
			}
			out = append(out, commandSlot{
				cmd:   &s.Command,
				title: titles[surfaceTitleKey{ws.Title, p.Index, k + 1}],
				cwd:   s.CWD,
				wsCWD: ws.CWD,
			})
		}
	}
	return out
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
			// Preserve a browser pane's saved profile when live capture
			// yielded none: profile capture reads cmux's session file, and
			// its absence must not strip profiles on re-save (idempotency).
			// A live-reported profile always wins.
			if lp.Type == "browser" && lp.Profile == "" && ep.Profile != "" {
				lp.Profile = ep.Profile
			}
			// Same for extra browser tabs, matched positionally.
			for k := range lp.Surfaces {
				ls := &lp.Surfaces[k]
				if k < len(ep.Surfaces) && ls.Type == "browser" && ls.Profile == "" && ep.Surfaces[k].Profile != "" {
					ls.Profile = ep.Surfaces[k].Profile
				}
			}
		}
	}
}
