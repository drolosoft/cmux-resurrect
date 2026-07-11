package orchestrate

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// RestoreMode determines how restore interacts with existing workspaces.
type RestoreMode int

const (
	// RestoreModeReplace closes all existing workspaces before restoring.
	RestoreModeReplace RestoreMode = iota
	// RestoreModeAdd adds restored workspaces on top of existing ones.
	RestoreModeAdd
)

// Restorer recreates a saved layout in cmux.
type Restorer struct {
	Client     client.Backend
	Store      persist.Store
	AutoAccept []string                                 // tool names or ["all"] for auto-accept injection
	SkipPing   bool                                     // skip backend ping check (for external apps like Alfred)
	OnProgress func(title string, panes int, err error) // called after each workspace
}

// restoreSurfaces creates additional surfaces (tabs) in a pane and sends their commands.
func (r *Restorer) restoreSurfaces(pane model.Pane, paneRef, workspaceRef string, result *RestoreResult, paneIdx int) {
	for j, surf := range pane.Surfaces {
		surfRef, err := r.Client.NewSurface(paneRef, workspaceRef)
		if err != nil {
			if err == client.ErrNotSupported {
				if r.OnProgress != nil {
					r.OnProgress("⚠ pane tabs not supported on this backend", 0, nil)
				}
				return // skip all extra surfaces on unsupported backends
			}
			result.Errors = append(result.Errors, fmt.Sprintf("  pane %d surface %d: new-surface: %v", paneIdx, j+1, err))
			continue
		}
		r.applyName(workspaceRef, surfRef, surf.Name)
		if surf.Command != "" || surf.CWD != "" {
			if err := waitForShellReady(r.Client, workspaceRef, surfRef); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d surface %d: shell not ready: %v", paneIdx, j+1, err))
			} else if err := r.Client.Send(workspaceRef, surfRef, noHistoryCmd(cwdCommand(surf.CWD, r.applyAutoAccept(surf.Command)))); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d surface %d: send command: %v", paneIdx, j+1, err))
			} else {
				r.verifyCWD(workspaceRef, surfRef, surf.CWD)
			}
		}
	}
}

// typeCommands sends each pane's (and extra surface's) command into an
// atomically created workspace. Structure and cwds are already native, so
// only the commands are typed — readiness-gated, without any cd prefix.
// Panes are addressed by their live visual index (from the layout builder);
// each pane's first surface receives the pane command.
func (r *Restorer) typeCommands(ws model.Workspace, ref string, visualIdx []int, result *RestoreResult) {
	// Resolve surface refs per visual pane index from the live tree.
	surfacesByPane := map[int][]string{}
	if tree, err := r.Client.Tree(); err == nil && tree != nil {
		for _, w := range tree.Windows {
			for _, tw := range w.Workspaces {
				if tw.Ref != ref {
					continue
				}
				for _, tp := range tw.Panes {
					for _, s := range tp.Surfaces {
						surfacesByPane[tp.Index] = append(surfacesByPane[tp.Index], s.Ref)
					}
				}
			}
		}
	}

	sendTo := func(surfaceRef, command string, label string) {
		if command == "" {
			return
		}
		// Never fall back to the focused surface: with an empty surfaceRef the
		// backend sends to whatever pane is active, so a failed resolution
		// would type every command into the SAME shell — e.g. pane 1's
		// `codex resume` typed into pane 0's just-started claude session.
		if surfaceRef == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("  %s: surface not resolved, command skipped", label))
			return
		}
		if err := waitForShellReady(r.Client, ref, surfaceRef); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("  %s shell not ready: %v", label, err))
			return
		}
		if err := r.Client.Send(ref, surfaceRef, noHistoryCmd(r.applyAutoAccept(command))); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("  %s send command: %v", label, err))
		}
	}

	for i, pane := range ws.Panes {
		if pane.Type == "browser" {
			continue // browser surfaces carry their url natively
		}
		vi := i
		if i < len(visualIdx) && visualIdx[i] >= 0 {
			vi = visualIdx[i]
		}
		surfs := surfacesByPane[vi]
		if pane.Command != "" {
			target := ""
			if len(surfs) > 0 {
				target = surfs[0]
			}
			sendTo(target, pane.Command, fmt.Sprintf("pane %d", i))
		}
		for j, extra := range pane.Surfaces {
			if extra.Command == "" || extra.Type == "browser" {
				continue
			}
			target := ""
			if j+1 < len(surfs) {
				target = surfs[j+1]
			}
			sendTo(target, extra.Command, fmt.Sprintf("pane %d tab %d", i, j+1))
		}
	}
}

// applyName sets a surface's title from an optional Blueprint name (GitHub #7).
// No-op when the name is empty or the backend can't rename individual surfaces.
func (r *Restorer) applyName(workspaceRef, surfaceRef, name string) {
	if name == "" {
		return
	}
	if rn, ok := r.Client.(client.SurfaceRenamer); ok {
		_ = rn.RenameSurface(workspaceRef, surfaceRef, name)
	}
}

// verifyCWD makes per-pane CWD restore reliable (GitHub #8). A freshly-created
// split's shell can drop the `cd` if it wasn't input-ready, leaving the pane in
// its inherited directory. After the cd-bearing Send, poll the surface's live
// cwd and re-send the BARE cd (never the command — idempotent, never re-runs a
// program) until it sticks or CWDVerifyTimeout passes. No-op when there's no
// target cwd/surface or the backend can't report surface state (e.g. Ghostty).
func (r *Restorer) verifyCWD(workspaceRef, surfaceRef, wantCWD string) {
	if wantCWD == "" || surfaceRef == "" {
		return
	}
	wantCWD = expandHome(wantCWD) // live cwd reports absolute paths
	ss, ok := r.Client.(client.SurfaceStater)
	if !ok {
		return
	}
	// Hard cap on the whole loop; the verify window proper only starts once
	// the shell is READY — typing into a pre-prompt shell is lost (the input
	// gets flushed at init) and leaves visible junk in the pane.
	hardDeadline := time.Now().Add(SurfaceReadyTimeout)
	var verifyDeadline, resendAllowedAt time.Time
	for time.Now().Before(hardDeadline) {
		st, err := ss.SurfaceState(workspaceRef, surfaceRef)
		if err == nil && st != nil && st.CWD == wantCWD {
			return // cd landed
		}
		if err == nil && st != nil && st.Ready {
			now := time.Now()
			if verifyDeadline.IsZero() {
				verifyDeadline = now.Add(CWDVerifyTimeout)
				// The original cd was just sent — give it (and the backend's
				// cwd report, which lags the prompt on Ghostty) time to
				// reflect before concluding it was lost. Re-sending on the
				// first stale reading typed a visible duplicate cd.
				resendAllowedAt = now.Add(CWDResendGrace)
			} else if now.After(verifyDeadline) {
				return // the ready shell had its fair chance; stop retrying
			}
			if now.After(resendAllowedAt) {
				_ = r.Client.Send(workspaceRef, surfaceRef, noHistoryCmd(cwdCommand(wantCWD, "")))
				resendAllowedAt = now.Add(CWDResendGrace)
			}
		}
		time.Sleep(CWDVerifyPoll)
	}
}

// applyAutoAccept injects the auto-accept flag into a command if configured.
func (r *Restorer) applyAutoAccept(command string) string {
	cmd, tool := InjectAutoAccept(command, r.AutoAccept)
	if tool != "" && r.OnProgress != nil {
		flag := autoAcceptCache[tool]
		r.OnProgress(fmt.Sprintf("⚡ auto-accept: %s %s", tool, flag), 0, nil)
	}
	return cmd
}

// shellQuote single-quotes a string for safe interpolation into a shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// expandHome expands a leading "~" or "~/" to the user's home directory, so
// portable layouts (like the shipped demo) can use machine-independent paths.
// "~user" forms and non-tilde paths pass through unchanged.
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return home + path[1:]
}

// cwdCommand builds the shell input for a pane/surface, prepending a `cd` into
// the saved per-pane directory when one was captured (GitHub #8). With no saved
// CWD it returns the command unchanged, so panes inherit the workspace path as
// before. A CWD with no command becomes a bare `cd`. Tilde paths are expanded
// here — a quoted `cd '~/x'` would NOT expand in the shell.
func cwdCommand(cwd, command string) string {
	if cwd == "" {
		return command
	}
	cd := "cd " + shellQuote(expandHome(cwd))
	if command == "" {
		return cd
	}
	return cd + " && " + command
}

// RestoreResult reports what happened during restore.
type RestoreResult struct {
	LayoutName       string
	WorkspacesTotal  int
	WorkspacesOK     int
	WorkspacesClosed int
	Errors           []string
	DryRun           bool
	Commands         []string // populated in dry-run mode
}

// Restore loads a layout and recreates it in cmux.
// When workspaceFilter is non-empty, only the workspace matching that title is restored.
// When skipMatching is true, workspaces whose title already exists are left untouched.
// When skipMatching is false (and mode is Replace), matching workspaces are also closed
// and recreated from the layout — useful when the saved layout has commands (e.g. AI
// resume) that the live tab no longer runs.
func (r *Restorer) Restore(name string, dryRun bool, mode RestoreMode, workspaceFilter string, skipMatching bool) (*RestoreResult, error) {
	layout, err := r.Store.Load(name)
	if err != nil {
		return nil, fmt.Errorf("load layout: %w", err)
	}

	if workspaceFilter != "" {
		// Try exact match first (case-insensitive).
		var exactMatch *model.Workspace
		var substringMatches []model.Workspace
		filterLower := strings.ToLower(workspaceFilter)

		for i, ws := range layout.Workspaces {
			titleLower := strings.ToLower(ws.Title)
			if titleLower == filterLower {
				exactMatch = &layout.Workspaces[i]
				break
			}
			if strings.Contains(titleLower, filterLower) {
				substringMatches = append(substringMatches, ws)
			}
		}

		switch {
		case exactMatch != nil:
			layout.Workspaces = []model.Workspace{*exactMatch}
		case len(substringMatches) == 1:
			layout.Workspaces = substringMatches
		case len(substringMatches) == 0:
			return nil, fmt.Errorf("workspace %q not found in layout %q", workspaceFilter, name)
		default:
			titles := make([]string, len(substringMatches))
			for i, ws := range substringMatches {
				titles[i] = fmt.Sprintf("%q", ws.Title)
			}
			return nil, fmt.Errorf("%q matches multiple workspaces in layout %q: %s",
				workspaceFilter, name, strings.Join(titles, ", "))
		}
	}

	if !dryRun && !r.SkipPing {
		if err := r.Client.Ping(); err != nil {
			return nil, fmt.Errorf("backend not reachable: %w", err)
		}
	}

	result := &RestoreResult{
		LayoutName:      layout.Name,
		WorkspacesTotal: len(layout.Workspaces),
		DryRun:          dryRun,
	}

	// Build set of layout titles for sync comparison.
	layoutTitles := make(map[string]bool, len(layout.Workspaces))
	for _, ws := range layout.Workspaces {
		layoutTitles[ws.Title] = true
	}

	// Snapshot existing workspace state.
	var callerRef string
	existingTitles := make(map[string]bool)
	if !dryRun {
		if tree, err := r.Client.Tree(); err == nil && tree.Caller != nil {
			callerRef = tree.Caller.WorkspaceRef
		}
		if existing, err := r.Client.ListWorkspaces(); err == nil {
			for _, ws := range existing {
				existingTitles[ws.Title] = true

				if mode == RestoreModeReplace && ws.Ref != callerRef {
					// Decide whether to close this workspace:
					// - Not in layout → always close (stale)
					// - In layout + skipMatching → keep (sync)
					// - In layout + !skipMatching → close (fresh recreate)
					shouldClose := !layoutTitles[ws.Title] || !skipMatching
					if shouldClose {
						_ = r.Client.UnpinWorkspace(ws.Ref)
						if err := r.Client.CloseWorkspace(ws.Ref); err != nil {
							result.Errors = append(result.Errors, fmt.Sprintf("close %s (%s): %v", ws.Ref, ws.Title, err))
						} else {
							result.WorkspacesClosed++
						}
						time.Sleep(DelayAfterClose)
					}
				}
			}
			if result.WorkspacesClosed > 0 {
				time.Sleep(DelayAfterCloseAll)
			}
		}
	} else if mode == RestoreModeReplace {
		result.Commands = append(result.Commands, "# Close workspaces not in layout (sync)")
	}

	// Sort workspaces by index.
	workspaces := make([]model.Workspace, len(layout.Workspaces))
	copy(workspaces, layout.Workspaces)
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].Index < workspaces[j].Index
	})

	// Create workspaces. Skip existing ones only when skipMatching is true.
	for _, ws := range workspaces {
		if !dryRun && skipMatching && existingTitles[ws.Title] {
			if r.OnProgress != nil {
				r.OnProgress(ws.Title, len(ws.Panes), fmt.Errorf("already open, skipped"))
			}
			continue
		}

		_, err := r.restoreWorkspace(ws, dryRun, result)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("workspace %q: %v", ws.Title, err))
			if r.OnProgress != nil && !dryRun {
				r.OnProgress(ws.Title, len(ws.Panes), err)
			}
			continue
		}
		result.WorkspacesOK++
		if r.OnProgress != nil && !dryRun {
			r.OnProgress(ws.Title, len(ws.Panes), nil)
		}
	}

	// Return focus to the caller's workspace.
	if callerRef != "" && !dryRun {
		if err := r.Client.SelectWorkspace(callerRef); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("select caller workspace: %v", err))
		}
	} else if dryRun {
		result.Commands = append(result.Commands, r.Client.DryRunFormatter().FmtSelectWorkspace("<caller>"))
	}

	return result, nil
}

func (r *Restorer) restoreWorkspace(ws model.Workspace, dryRun bool, result *RestoreResult) (string, error) {
	if dryRun {
		return r.dryRunWorkspace(ws, result)
	}

	// 1. Create workspace — atomically with the full split tree when the
	// backend supports it (cmux `--layout`): per-surface cwds, names, urls,
	// and focus land natively, with exact ratios and no typed `cd` per pane.
	if lc, ok := r.Client.(client.LayoutWorkspaceCreator); ok {
		if layoutJSON, visualIdx, buildable := buildCmuxLayout(ws); buildable {
			lref, lerr := lc.NewWorkspaceLayout(client.NewWorkspaceOpts{CWD: expandHome(ws.CWD)}, layoutJSON)
			if lerr == nil {
				time.Sleep(DelayAfterCreate)
				if err := r.Client.SelectWorkspace(lref); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("select workspace: %v", err))
				}
				time.Sleep(DelayAfterSelect)
				r.typeCommands(ws, lref, visualIdx, result)
				// Same tail as the sequential path: deferred rename (the
				// shell prompt overwrites early titles) and pinning. Focus
				// is already native via the layout's focus flag.
				time.Sleep(DelayBeforeRename)
				if err := r.Client.RenameWorkspace(lref, ws.Title); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("rename %q: %v", ws.Title, err))
				}
				if ws.Pinned {
					if err := r.Client.PinWorkspace(lref); err != nil {
						result.Errors = append(result.Errors, fmt.Sprintf("pin %q: %v", ws.Title, err))
					}
				}
				return lref, nil
			}
			// Layout creation failed (older cmux without --layout, or an
			// unrepresentable edge) — fall through to the sequential path.
		}
	}

	ref, err := r.Client.NewWorkspace(client.NewWorkspaceOpts{CWD: expandHome(ws.CWD)})
	if err != nil {
		return "", fmt.Errorf("new-workspace: %w", err)
	}

	// Small delay after creation.
	time.Sleep(DelayAfterCreate)

	// 2. Select workspace to ensure splits target the correct one.
	// Rename is deferred to after all workspaces are created (shell prompt overwrites title).
	if err := r.Client.SelectWorkspace(ref); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("select workspace: %v", err))
	}
	time.Sleep(DelayAfterSelect)

	// 3. Create additional panes (splits) and send commands.
	// Resolve which earlier pane each split targets so splits can be
	// addressed at an EXPLICIT surface ref: splitting "the focused pane"
	// found via live indexes drifts on Ghostty, which re-indexes terminals
	// when splits are inserted (panes then land in the wrong corner).
	splitTargets, _, targetsResolved := resolveSplitTargets(ws)
	paneRefs := make([]string, len(ws.Panes))
	if fr, ok := r.Client.(client.FirstSurfaceResolver); ok {
		paneRefs[0] = fr.FirstSurfaceRef(ref)
	}
	lastPane := len(ws.Panes) - 1
	for i, pane := range ws.Panes {
		if i == 0 {
			// First pane is the default one created with the workspace.
			if pane.Type == "browser" && pane.URL != "" {
				// The workspace always starts with a terminal pane;
				// open the URL via the shell as a fallback.
				if err := waitForShellReady(r.Client, ref, ""); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("  pane %d shell not ready: %v", i, err))
				} else if err := r.Client.Send(ref, "", noHistoryCmd(fmt.Sprintf("open %q", pane.URL))); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("  pane %d open url: %v", i, err))
				}
			} else {
				// The first pane already starts at the workspace CWD, so it
				// only needs a cd when its own cwd differs (GitHub #8).
				cd := pane.CWD
				if cd != "" && expandHome(cd) == expandHome(ws.CWD) {
					cd = ""
				}
				if pane.Command != "" || cd != "" {
					if err := waitForShellReady(r.Client, ref, ""); err != nil {
						result.Errors = append(result.Errors, fmt.Sprintf("  pane %d shell not ready: %v", i, err))
					} else if err := r.Client.Send(ref, "", noHistoryCmd(cwdCommand(cd, r.applyAutoAccept(pane.Command)))); err != nil {
						result.Errors = append(result.Errors, fmt.Sprintf("  pane %d send command: %v", i, err))
					}
				}
			}
			// Name the first surface if the Blueprint labeled it (#7).
			r.applyName(ref, "", pane.Name)
			// Create extra surfaces (tabs) in this pane.
			if len(pane.Surfaces) > 0 {
				r.restoreSurfaces(pane, "pane:0", ref, result, 0)
			}
			// If more panes follow, let the command settle before creating splits.
			if i < lastPane {
				time.Sleep(DelayAfterSplit)
			}
			continue
		}

		// Explicit split target by creation-order ref when resolvable.
		splitRef := ""
		if targetsResolved && splitTargets[i] >= 0 && pane.Type != "browser" {
			splitRef = paneRefs[splitTargets[i]]
		}

		// Focus a specific pane before splitting — fallback for backends
		// that can't resolve refs, and for browser panes (NewPane splits
		// whatever is focused).
		if splitRef == "" && pane.FocusTarget >= 0 {
			targetRef := fmt.Sprintf("pane:%d", pane.FocusTarget)
			if err := r.Client.FocusPane(targetRef, ref); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d focus target: %v", i, err))
			}
			time.Sleep(DelayAfterSelect)
		}

		direction := pane.Split
		if direction == "" {
			direction = "right"
		}

		if pane.Type == "browser" {
			// Browser panes use NewPane instead of NewSplit.
			bref, err := r.Client.NewPane(client.NewPaneOpts{
				Type:         "browser",
				Direction:    direction,
				WorkspaceRef: ref,
				URL:          pane.URL,
			})
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d new-pane browser: %v", i, err))
				continue
			}
			paneRefs[i] = bref
			// NewPane (browser) may not transfer focus like NewSplit.
			// Focus the new pane so subsequent splits target it, not pane 0.
			// Uses pane:N format (workspace-local index) — CLIClient converts
			// to plain index for cmux, Ghostty handles it natively.
			if i < lastPane {
				paneRef := fmt.Sprintf("pane:%d", i)
				if err := r.Client.FocusPane(paneRef, ref); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("  pane %d focus after browser: %v", i, err))
				}
				time.Sleep(DelayAfterSelect)
			}
			// Apply saved split ratio if available.
			if needsResize(pane.SplitRatio) {
				resizeAfterSplit(r, "", ref, direction, pane.SplitRatio)
			}
			// Browser panes don't have a shell — skip command sending.
		} else {
			surfaceRef, err := r.Client.NewSplit(direction, ref, splitRef)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d split: %v", i, err))
				continue
			}
			paneRefs[i] = surfaceRef

			// Name the split's surface if the Blueprint labeled it (#7).
			r.applyName(ref, surfaceRef, pane.Name)

			// Apply saved split ratio if available.
			if needsResize(pane.SplitRatio) {
				resizeAfterSplit(r, surfaceRef, ref, direction, pane.SplitRatio)
			}

			// Splits do NOT inherit the workspace cwd — they spawn wherever
			// the backend decides. A split without its own cwd (layouts saved
			// before per-pane cwds were always recorded) gets the workspace
			// cwd instead of no cd at all (GitHub #8, 2026-07-11 audit).
			cd := pane.CWD
			if cd == "" {
				cd = ws.CWD
			}
			if pane.Command != "" || cd != "" {
				// Wait for the shell in the new pane to become interactive before sending.
				if err := waitForShellReady(r.Client, ref, surfaceRef); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("  pane %d shell not ready: %v", i, err))
				} else if err := r.Client.Send(ref, surfaceRef, noHistoryCmd(cwdCommand(cd, r.applyAutoAccept(pane.Command)))); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("  pane %d send command: %v", i, err))
				} else {
					r.verifyCWD(ref, surfaceRef, cd)
				}
			}
			// Create extra surfaces (tabs) in this pane.
			if len(pane.Surfaces) > 0 {
				paneRef := paneRefForSurface(r.Client, surfaceRef, ref)
				if paneRef != "" {
					r.restoreSurfaces(pane, paneRef, ref, result, i)
				}
			}
		}
		// Let the command settle before creating the next split.
		// Without this, NewSplit can shift focus before the enter key
		// from the previous Send is fully processed.
		if i < lastPane {
			time.Sleep(DelayAfterSplit)
		}
	}

	// 4. Focus the right pane.
	for _, pane := range ws.Panes {
		if pane.Focus && pane.Index > 0 {
			paneRef := fmt.Sprintf("pane:%d", pane.Index)
			_ = r.Client.FocusPane(paneRef, ref)
			break
		}
	}

	// 5. Wait for shell to settle, then rename.
	// Shell prompt sets terminal title on startup; renaming too early gets overwritten.
	time.Sleep(DelayBeforeRename)
	if err := r.Client.RenameWorkspace(ref, ws.Title); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("rename %q: %v", ws.Title, err))
	}

	// 6. Pin if requested.
	if ws.Pinned {
		if err := r.Client.PinWorkspace(ref); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("pin %q: %v", ws.Title, err))
		}
	}

	return ref, nil
}

// needsResize returns true if a split ratio requires a resize after creation.
// Splits default to 50/50; ratios within ±2% of 0.5 are treated as equal.
func needsResize(ratio float64) bool {
	return ratio > 0 && (ratio < 0.48 || ratio > 0.52)
}

// resizeAfterSplit adjusts a newly created pane to match the saved split ratio.
// cmux resize-pane -D/-U/-L/-R grows the TARGET pane in that direction.
// To shrink the new pane (ratio < 0.5), we grow its SIBLING in the split direction.
// paneRef targets the sibling (pane 0 for the first split); pass "0" for the
// first split or the actual ref of the pane that was split.
func resizeAfterSplit(r *Restorer, paneRef, workspaceRef, direction string, ratio float64) {
	resizer, ok := r.Client.(client.PaneResizer)
	if !ok {
		return
	}

	// Delta: positive means new pane should be larger than 50%.
	delta := ratio - 0.5

	// Estimate cells from approximate workspace dimensions.
	const cellW, cellH = 9.0, 20.0

	var resizeDir string
	var amount int
	var targetPane string

	switch direction {
	case "right", "left":
		amount = int(1000 * absf(delta) / cellW)
		if delta < 0 {
			// New pane should be smaller → grow the sibling (pane 0) in split direction.
			targetPane = "pane:0"
			if direction == "right" {
				resizeDir = "R"
			} else {
				resizeDir = "L"
			}
		} else {
			// New pane should be bigger → grow it.
			targetPane = paneRef
			if direction == "right" {
				resizeDir = "R"
			} else {
				resizeDir = "L"
			}
		}
	case "down", "up":
		amount = int(800 * absf(delta) / cellH)
		if delta < 0 {
			// New pane should be smaller → grow the sibling (pane 0).
			targetPane = "pane:0"
			if direction == "down" {
				resizeDir = "D"
			} else {
				resizeDir = "U"
			}
		} else {
			// New pane should be bigger → grow it.
			targetPane = paneRef
			if direction == "down" {
				resizeDir = "D"
			} else {
				resizeDir = "U"
			}
		}
	}

	if amount == 0 {
		return
	}

	_ = resizer.ResizePane(client.ResizePaneOpts{
		PaneRef:      targetPane,
		WorkspaceRef: workspaceRef,
		Direction:    resizeDir,
		Amount:       amount,
	})
}

func absf(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func (r *Restorer) dryRunWorkspace(ws model.Workspace, result *RestoreResult) (string, error) {
	ref := fmt.Sprintf("workspace:new_%d", ws.Index)
	f := r.Client.DryRunFormatter()

	result.Commands = append(result.Commands, "")
	result.Commands = append(result.Commands, fmt.Sprintf("# %s", ws.Title))
	result.Commands = append(result.Commands, f.FmtNewWorkspace(ws.CWD))
	result.Commands = append(result.Commands, f.FmtRenameWorkspace(ref, ws.Title))

	for i, pane := range ws.Panes {
		if i == 0 {
			if pane.Type == "browser" && pane.URL != "" {
				result.Commands = append(result.Commands, f.FmtSend(ref, fmt.Sprintf("open %q", pane.URL)))
			} else if pane.Command != "" || pane.CWD != "" {
				result.Commands = append(result.Commands, f.FmtSend(ref, cwdCommand(pane.CWD, r.applyAutoAccept(pane.Command))))
			}
			for _, surf := range pane.Surfaces {
				result.Commands = append(result.Commands, f.FmtNewSurface(fmt.Sprintf("pane:%d", i), ref))
				if surf.Command != "" || surf.CWD != "" {
					result.Commands = append(result.Commands, f.FmtSend(ref, cwdCommand(surf.CWD, r.applyAutoAccept(surf.Command))))
				}
			}
			continue
		}
		if pane.FocusTarget >= 0 {
			result.Commands = append(result.Commands,
				f.FmtFocusPane(fmt.Sprintf("pane:%d", pane.FocusTarget), ref))
		}
		direction := pane.Split
		if direction == "" {
			direction = "right"
		}
		if pane.Type == "browser" {
			result.Commands = append(result.Commands, f.FmtNewPane(pane.Type, direction, ref, pane.URL))
		} else {
			result.Commands = append(result.Commands, f.FmtNewSplit(direction, ref))
			if pane.Command != "" || pane.CWD != "" {
				result.Commands = append(result.Commands, f.FmtSend(ref, cwdCommand(pane.CWD, r.applyAutoAccept(pane.Command))))
			}
			for _, surf := range pane.Surfaces {
				result.Commands = append(result.Commands, f.FmtNewSurface(fmt.Sprintf("pane:%d", i), ref))
				if surf.Command != "" || surf.CWD != "" {
					result.Commands = append(result.Commands, f.FmtSend(ref, cwdCommand(surf.CWD, r.applyAutoAccept(surf.Command))))
				}
			}
		}
	}

	return ref, nil
}
