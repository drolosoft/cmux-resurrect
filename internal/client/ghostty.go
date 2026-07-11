package client

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// GhosttyClient implements Backend for Ghostty (macOS only, requires 1.3+).
//
// Limitations vs cmux backend:
//   - PinWorkspace is a no-op (Ghostty has no pin concept)
//   - SidebarState git info obtained via shell (not exposed by Ghostty API)
//   - Tree enumeration is slower (no single JSON snapshot, must loop via AppleScript)
//   - Split sizing cannot be controlled (always equal splits)
//   - AppleScript API is preview — breaking changes expected in Ghostty 1.4
//   - macOS only until Ghostty ships D-Bus support on Linux
type GhosttyClient struct {
	Timeout time.Duration
	AppName string // macOS application name for AppleScript (default: "Ghostty")

	// anchorWin memoizes the id of THE window this client operates on. All
	// tab-scoped operations address it explicitly ("window id ..."), never
	// "front window": front-window resolution shifts with focus/Spaces, and
	// a multi-workspace restore could end up creating one WINDOW per
	// workspace instead of tabs in one window (field report, 2026-07-11).
	anchorWin string
}

// NewGhosttyClient creates a GhosttyClient with sensible defaults.
func NewGhosttyClient() *GhosttyClient {
	return &GhosttyClient{
		Timeout: 10 * time.Second,
		AppName: "Ghostty",
	}
}

// NewGhosttyClientForApp creates a GhosttyClient targeting a specific app name.
// Used for cmux which speaks the same AppleScript protocol as Ghostty.
func NewGhosttyClientForApp(appName string) *GhosttyClient {
	return &GhosttyClient{
		Timeout: 10 * time.Second,
		AppName: appName,
	}
}

// appName returns the target app name for AppleScript, defaulting to "Ghostty".
func (g *GhosttyClient) appName() string {
	if g.AppName != "" {
		return g.AppName
	}
	return "Ghostty"
}

// tell returns the AppleScript fragment: tell application "AppName"
func (g *GhosttyClient) tell() string {
	return fmt.Sprintf(`tell application "%s"`, g.appName())
}

// windowClause returns the AppleScript specifier for the client's anchor
// window ("window id \"...\""), resolving it once: the front window at first
// use, or a freshly created window when none exists (the app auto-launches).
// The second return reports whether this call CREATED the window (its default
// tab may need closing after the first workspace tab is added).
func (g *GhosttyClient) windowClause() (string, bool, error) {
	if g.anchorWin != "" {
		return fmt.Sprintf(`window id "%s"`, escapeAppleScript(g.anchorWin)), false, nil
	}
	id, err := g.runScript(fmt.Sprintf(`tell application "%s" to id of front window`, g.appName()))
	created := false
	if err != nil || id == "" {
		// No window — create one and adopt it. Result is discarded: the
		// returned reference isn't coercible, but the window is created.
		_, _ = g.runScriptLines(g.tell(), `  make new window`, `end tell`)
		created = true
		deadline := time.Now().Add(NewWorkspaceDeadline)
		for time.Now().Before(deadline) {
			id, err = g.runScript(fmt.Sprintf(`tell application "%s" to id of front window`, g.appName()))
			if err == nil && id != "" {
				break
			}
			time.Sleep(PollInterval)
		}
		if id == "" {
			return "", false, fmt.Errorf("%s: no window available", g.appName())
		}
	}
	g.anchorWin = id
	return fmt.Sprintf(`window id "%s"`, escapeAppleScript(id)), created, nil
}

// mustWindowClause is windowClause for read paths that previously assumed a
// front window; on failure it falls back to "front window" (old behavior).
func (g *GhosttyClient) mustWindowClause() string {
	w, _, err := g.windowClause()
	if err != nil {
		return "front window"
	}
	return w
}

// runScript executes a single-line AppleScript via osascript.
func (g *GhosttyClient) runScript(script string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("osascript: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// runScriptLines executes a multi-line AppleScript (each line as a separate -e arg).
func (g *GhosttyClient) runScriptLines(lines ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.Timeout)
	defer cancel()
	args := make([]string, 0, len(lines)*2)
	for _, line := range lines {
		args = append(args, "-e", line)
	}
	cmd := exec.CommandContext(ctx, "osascript", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("osascript: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *GhosttyClient) Ping() error {
	out, err := g.runScript(fmt.Sprintf(`tell application "System Events" to (name of processes) contains "%s"`, g.appName()))
	if err != nil {
		return fmt.Errorf("ghostty ping: %w", err)
	}
	if out != "true" {
		return fmt.Errorf("ghostty is not running")
	}
	return nil
}

func (g *GhosttyClient) PinWorkspace(ref string) error {
	return nil // Ghostty does not support pinning tabs
}

func (g *GhosttyClient) UnpinWorkspace(ref string) error {
	return nil // Ghostty does not support pinning tabs
}

// GhosttyClient does not implement PaneGeometryProvider or PaneResizer.
// Type assertions in save/restore will fail, falling back to default behavior.

// parseTabIndex extracts the 1-based tab index from a ref like "tab:3".
func parseTabIndex(ref string) (int, error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return 0, fmt.Errorf("invalid tab ref: %s", ref)
	}
	return strconv.Atoi(parts[1])
}

// parseTerminalIndex extracts the 1-based terminal index from refs.
// "terminal:N" refs are already 1-based (pass through).
// "pane:N" refs are 0-based (cmux convention) — adds 1 for AppleScript.
// ghosttyTerminalSpecifier builds the AppleScript specifier for a surface
// ref. Refs of the form "tid:<uuid>" address the terminal by its unique id —
// immune to Ghostty re-indexing terminals when later splits are inserted
// (index-based sends could land in the WRONG pane). Index refs
// ("terminal:N" / "pane:N") remain supported for layout-derived targeting.
func ghosttyTerminalSpecifier(surfaceRef string, tabIdx int, win string) (string, error) {
	if id, ok := strings.CutPrefix(surfaceRef, "tid:"); ok && id != "" {
		return fmt.Sprintf(`terminal id "%s" of tab %d of %s`, escapeAppleScript(id), tabIdx, win), nil
	}
	termIdx, err := parseTerminalIndex(surfaceRef)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`terminal %d of tab %d of %s`, termIdx, tabIdx, win), nil
}

// cwdFromTitle extracts a working directory from a terminal title, for shells
// that don't emit OSC 7 (Ghostty's `working directory` AppleScript property
// stays empty then, but shell titles conventionally carry the cwd: bare paths
// or "user@host: ~/path"). The candidate is accepted only when it names an
// existing directory, so arbitrary titles can't fake a cwd.
func cwdFromTitle(title string) string {
	cand := strings.TrimSpace(title)
	if i := strings.LastIndex(cand, ": "); i >= 0 {
		cand = cand[i+2:]
	}
	cand = strings.TrimSpace(cand)
	if cand == "~" || strings.HasPrefix(cand, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		cand = home + cand[1:]
	}
	if !strings.HasPrefix(cand, "/") {
		return ""
	}
	if st, err := os.Stat(cand); err == nil && st.IsDir() {
		return cand
	}
	return ""
}

func parseTerminalIndex(ref string) (int, error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return 0, fmt.Errorf("invalid terminal ref: %s", ref)
	}
	idx, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	if parts[0] == "pane" {
		return idx + 1, nil
	}
	return idx, nil
}

// escapeAppleScript escapes characters for safe AppleScript string interpolation.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

func (g *GhosttyClient) Tree() (*TreeResponse, error) {
	out, err := g.runScriptLines(
		g.tell(),
		`  set output to ""`,
		`  set winCount to count of windows`,
		`  repeat with w from 1 to winCount`,
		`    set winID to id of window w`,
		`    set tabCount to count of tabs of window w`,
		`    set output to output & "WIN|" & winID & "|" & tabCount & linefeed`,
		`    repeat with t from 1 to tabCount`,
		`      set tabName to name of tab t of window w`,
		`      set isSel to selected of tab t of window w`,
		`      set termCount to count of terminals of tab t of window w`,
		`      set output to output & "TAB|" & t & "|" & tabName & "|" & isSel & "|" & termCount & linefeed`,
		`      repeat with term from 1 to termCount`,
		`        set termCWD to working directory of terminal term of tab t of window w`,
		`        set termName to name of terminal term of tab t of window w`,
		`        set output to output & "TERM|" & term & "|" & termCWD & "|" & termName & linefeed`,
		`      end repeat`,
		`    end repeat`,
		`  end repeat`,
		`  return output`,
		`end tell`,
	)
	if err != nil {
		return nil, fmt.Errorf("tree: %w", err)
	}

	resp := &TreeResponse{}
	var currentWindow *TreeWindow
	var currentWorkspace *TreeWorkspace

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "WIN":
			if currentWorkspace != nil && currentWindow != nil {
				currentWindow.Workspaces = append(currentWindow.Workspaces, *currentWorkspace)
				currentWorkspace = nil
			}
			if currentWindow != nil {
				resp.Windows = append(resp.Windows, *currentWindow)
			}
			tabCount, _ := strconv.Atoi(parts[2])
			currentWindow = &TreeWindow{
				Ref:            parts[1],
				Index:          len(resp.Windows),
				Active:         true,
				Visible:        true,
				Current:        len(resp.Windows) == 0,
				WorkspaceCount: tabCount,
			}
			currentWorkspace = nil

		case "TAB":
			if currentWorkspace != nil && currentWindow != nil {
				currentWindow.Workspaces = append(currentWindow.Workspaces, *currentWorkspace)
			}
			tabIdx, _ := strconv.Atoi(parts[1])
			tabName := parts[2]
			isSel := parts[3] == "true"
			ref := fmt.Sprintf("tab:%d", tabIdx)
			currentWorkspace = &TreeWorkspace{
				Ref:      ref,
				Title:    tabName,
				Index:    tabIdx - 1,
				Pinned:   false,
				Active:   isSel,
				Selected: isSel,
			}
			if isSel && currentWindow != nil {
				currentWindow.SelectedWorkspaceRef = ref
			}

		case "TERM":
			if currentWorkspace == nil {
				continue
			}
			termIdx, _ := strconv.Atoi(parts[1])
			termCWD := ""
			if len(parts) > 2 {
				termCWD = parts[2]
			}
			termName := ""
			if len(parts) > 3 {
				// The name is the last field and may itself contain pipes.
				termName = strings.Join(parts[3:], "|")
			}
			if termCWD == "" {
				// No OSC 7 from this shell — Ghostty's `working directory`
				// property stays empty. Shell titles conventionally carry
				// the cwd; use it when it names a real directory.
				termCWD = cwdFromTitle(termName)
			}
			paneRef := fmt.Sprintf("pane:%d", termIdx-1)
			surfaceRef := fmt.Sprintf("terminal:%d", termIdx)
			pane := TreePane{
				Ref:                paneRef,
				Index:              termIdx - 1,
				Active:             termIdx == 1,
				Focused:            termIdx == 1,
				SurfaceCount:       1,
				SelectedSurfaceRef: surfaceRef,
				SurfaceRefs:        []string{surfaceRef},
				Surfaces: []TreeSurface{
					{
						Ref:            surfaceRef,
						PaneRef:        paneRef,
						Type:           "terminal",
						Title:          termName,
						CWD:            termCWD,
						Index:          termIdx - 1,
						IndexInPane:    0,
						Active:         termIdx == 1,
						Focused:        termIdx == 1,
						Selected:       termIdx == 1,
						SelectedInPane: true,
					},
				},
			}
			currentWorkspace.Panes = append(currentWorkspace.Panes, pane)
		}
	}

	// Flush remaining workspace and window.
	if currentWorkspace != nil && currentWindow != nil {
		currentWindow.Workspaces = append(currentWindow.Workspaces, *currentWorkspace)
	}
	if currentWindow != nil {
		resp.Windows = append(resp.Windows, *currentWindow)
	}

	// Set Caller to the selected tab's first terminal in the first window.
	if len(resp.Windows) > 0 {
		for _, ws := range resp.Windows[0].Workspaces {
			if ws.Selected && len(ws.Panes) > 0 {
				resp.Caller = &CallerInfo{
					WorkspaceRef: ws.Ref,
					PaneRef:      ws.Panes[0].Ref,
					WindowRef:    resp.Windows[0].Ref,
					SurfaceRef:   ws.Panes[0].SurfaceRefs[0],
					SurfaceType:  "terminal",
				}
				resp.Active = resp.Caller
				break
			}
		}
	}

	return resp, nil
}

func (g *GhosttyClient) SidebarState(workspaceRef string) (*SidebarState, error) {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return nil, err
	}

	out, err := g.runScriptLines(
		g.tell(),
		fmt.Sprintf(`  set focTerm to focused terminal of tab %d of %s`, tabIdx, g.mustWindowClause()),
		`  return (working directory of focTerm) & "|" & (name of focTerm)`,
		`end tell`,
	)
	if err != nil {
		return nil, fmt.Errorf("sidebar state: %w", err)
	}
	cwd, name, _ := strings.Cut(out, "|")
	if cwd == "" {
		// The `working directory` property fills only from OSC 7; when the
		// shell doesn't emit it, fall back to the title (validated on disk).
		cwd = cwdFromTitle(name)
	}

	state := &SidebarState{
		CWD:        cwd,
		FocusedCWD: cwd,
	}

	if cwd != "" {
		if branch, err := g.gitBranch(cwd); err == nil {
			state.GitBranch = branch
		}
		state.GitDirty = g.gitDirty(cwd)
	}

	return state, nil
}

func (g *GhosttyClient) gitBranch(cwd string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *GhosttyClient) gitDirty(cwd string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), g.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", cwd, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func (g *GhosttyClient) ListWorkspaces() ([]WorkspaceInfo, error) {
	out, err := g.runScriptLines(
		g.tell(),
		fmt.Sprintf(`  set tabCount to count of tabs of %s`, g.mustWindowClause()),
		`  set output to ""`,
		`  repeat with t from 1 to tabCount`,
		fmt.Sprintf(`    set tabName to name of tab t of %s`, g.mustWindowClause()),
		fmt.Sprintf(`    set isSel to selected of tab t of %s`, g.mustWindowClause()),
		`    set output to output & "tab:" & t & "|" & tabName & "|" & isSel & linefeed`,
		`  end repeat`,
		`  return output`,
		`end tell`,
	)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}

	var workspaces []WorkspaceInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		workspaces = append(workspaces, WorkspaceInfo{
			Ref:      parts[0],
			Title:    parts[1],
			Selected: parts[2] == "true",
		})
	}
	return workspaces, nil
}

func (g *GhosttyClient) NewWorkspace(opts NewWorkspaceOpts) (string, error) {
	// Resolve THE window every workspace of this run goes into. Never "front
	// window": its resolution shifts with focus/Spaces, and a multi-workspace
	// restore could open one WINDOW per workspace instead of tabs in one
	// window. When no window exists (cold start) this creates and adopts one.
	win, createdWindow, err := g.windowClause()
	if err != nil {
		return "", err
	}

	beforeCount := 0
	if !createdWindow {
		beforeOut, err := g.runScript(fmt.Sprintf(`tell application "%s" to count of tabs of %s`, g.appName(), win))
		if err != nil {
			return "", fmt.Errorf("count tabs: %w", err)
		}
		beforeCount, _ = strconv.Atoi(beforeOut)
	}

	if opts.CWD != "" && g.appName() == "Ghostty" {
		// Ghostty supports surface configuration for setting initial CWD.
		_, err = g.runScriptLines(
			g.tell(),
			fmt.Sprintf(`  set cfg to new surface configuration from {initial working directory:"%s"}`, escapeAppleScript(opts.CWD)),
			fmt.Sprintf(`  new tab in %s with configuration cfg`, win),
			`end tell`,
		)
	} else {
		// cmux and fallback: create plain tab, cd to CWD after shell is ready.
		_, err = g.runScript(fmt.Sprintf(`tell application "%s" to new tab in %s`, g.appName(), win))
	}
	if err != nil {
		return "", fmt.Errorf("new tab: %w", err)
	}

	var ref string
	deadline := time.Now().Add(NewWorkspaceDeadline)
	for time.Now().Before(deadline) {
		afterOut, err := g.runScript(fmt.Sprintf(`tell application "%s" to count of tabs of %s`, g.appName(), win))
		if err != nil {
			time.Sleep(PollInterval)
			continue
		}
		afterCount, _ := strconv.Atoi(afterOut)
		if afterCount > beforeCount {
			ref = fmt.Sprintf("tab:%d", afterCount)
			break
		}
		time.Sleep(PollInterval)
	}
	if ref == "" {
		return "", fmt.Errorf("new tab created but could not determine ref")
	}

	// A freshly created anchor window spawns a default tab (tab 1) alongside
	// our configured one — close it so the workspace tab is tab 1.
	if createdWindow {
		out, _ := g.runScript(fmt.Sprintf(`tell application "%s" to count of tabs of %s`, g.appName(), win))
		if n, _ := strconv.Atoi(out); n > 1 {
			_, _ = g.runScript(fmt.Sprintf(`tell application "%s" to close tab (a reference to tab 1 of %s)`, g.appName(), win))
			time.Sleep(PollInterval)
			ref = "tab:1"
		}
	}

	// For non-Ghostty apps (cmux), cd to CWD since surface configuration isn't supported.
	if opts.CWD != "" && g.appName() != "Ghostty" {
		g.waitForShellReady(ref)
		_ = g.Send(ref, "", fmt.Sprintf(" cd %q", opts.CWD)+"\\n")
		time.Sleep(PollInterval)
	}

	if opts.Command != "" {
		g.waitForShellReady(ref)
		_ = g.Send(ref, "", opts.Command+"\\n")
	}

	return ref, nil
}

func (g *GhosttyClient) waitForShellReady(workspaceRef string) {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return
	}
	deadline := time.Now().Add(NewWorkspaceDeadline)
	for time.Now().Before(deadline) {
		cwd, err := g.runScript(fmt.Sprintf(
			`tell application "%s" to working directory of terminal 1 of tab %d of %s`, g.appName(),
			tabIdx, g.mustWindowClause(),
		))
		if err == nil && cwd != "" {
			return
		}
		time.Sleep(PollInterval)
	}
}

func (g *GhosttyClient) RenameWorkspace(ref, title string) error {
	tabIdx, err := parseTabIndex(ref)
	if err != nil {
		return err
	}
	_, err = g.runScript(fmt.Sprintf(
		`tell application "%s" to perform action "set_tab_title:%s" on terminal 1 of tab %d of %s`, g.appName(),
		escapeAppleScript(title), tabIdx, g.mustWindowClause(),
	))
	return err
}

// GhosttyClient deliberately does NOT implement client.SurfaceRenamer (GitHub #7).
// In Ghostty the only nameable unit is the tab, which maps to a crex workspace and
// is already titled via RenameWorkspace. Individual splits within a tab have no
// title property, and Ghostty has no sub-tabs (NewSurface returns ErrNotSupported),
// so per-pane Blueprint names have nowhere to render. Not implementing the
// interface makes applyName a clean no-op for Ghostty (vs. a set_tab_title that
// would fight the workspace title). Per-pane names still work on cmux, and the
// name is preserved in the Blueprint either way.

func (g *GhosttyClient) SelectWorkspace(ref string) error {
	tabIdx, err := parseTabIndex(ref)
	if err != nil {
		return err
	}
	_, err = g.runScript(fmt.Sprintf(
		`tell application "%s" to select tab (a reference to tab %d of %s)`, g.appName(),
		tabIdx, g.mustWindowClause(),
	))
	return err
}

func (g *GhosttyClient) NewSplit(direction, workspaceRef, surfaceRef string) (string, error) {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return "", fmt.Errorf("parse workspace ref: %w", err)
	}

	direction, err = validateSplitDirection(direction)
	if err != nil {
		return "", err
	}

	listIDs := func() (map[string]bool, error) {
		out, err := g.runScriptLines(
			g.tell(),
			`  set ids to ""`,
			fmt.Sprintf(`  repeat with t in terminals of tab %d of %s`, tabIdx, g.mustWindowClause()),
			`    set ids to ids & (id of t) & linefeed`,
			`  end repeat`,
			`  return ids`,
			`end tell`,
		)
		if err != nil {
			return nil, err
		}
		set := map[string]bool{}
		for _, line := range strings.Split(out, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				set[line] = true
			}
		}
		return set, nil
	}

	before, err := listIDs()
	if err != nil {
		return "", fmt.Errorf("list terminals: %w", err)
	}

	// Split an EXPLICIT terminal when the caller names one — splitting "the
	// focused terminal" depends on mutable focus state, and terminal indexes
	// drift as splits are inserted, so placement could land on the wrong
	// pane. Fall back to the focused terminal only without a target.
	splitTarget := fmt.Sprintf(`focused terminal of tab %d of %s`, tabIdx, g.mustWindowClause())
	if surfaceRef != "" {
		target, terr := ghosttyTerminalSpecifier(surfaceRef, tabIdx, g.mustWindowClause())
		if terr != nil {
			return "", fmt.Errorf("resolve split target: %w", terr)
		}
		splitTarget = target
	}

	_, err = g.runScriptLines(
		g.tell(),
		`  set focTerm to `+splitTarget,
		fmt.Sprintf(`  split focTerm direction %s`, direction),
		`end tell`,
	)
	if err != nil {
		return "", fmt.Errorf("split: %w", err)
	}

	// Identify the NEW terminal by id, never by index: Ghostty re-indexes
	// terminals when a split is inserted, so "the last index" can be an
	// EXISTING pane — sends addressed that way landed in the wrong pane.
	deadline := time.Now().Add(NewSplitDeadline)
	for time.Now().Before(deadline) {
		time.Sleep(PollInterval)
		after, err := listIDs()
		if err != nil {
			continue
		}
		for id := range after {
			if !before[id] {
				return "tid:" + id, nil
			}
		}
	}
	return "", fmt.Errorf("split created but could not determine new terminal ref")
}

func (g *GhosttyClient) NewPane(opts NewPaneOpts) (string, error) {
	// Ghostty has no browser pane concept — create a terminal split,
	// then open the URL in the system browser if requested.
	direction := opts.Direction
	if direction == "" {
		direction = "right"
	}
	ref, err := g.NewSplit(direction, opts.WorkspaceRef, "")
	if err != nil {
		return "", err
	}

	if opts.Type == "browser" && opts.URL != "" {
		// Wait for the NEW terminal's shell to initialize, not terminal 1.
		g.waitForTerminalReady(opts.WorkspaceRef, ref)
		_ = g.Send(opts.WorkspaceRef, ref, " "+fmt.Sprintf("open %q", opts.URL)+"\\n")
	}

	return ref, nil
}

// waitForTerminalReady waits until a specific terminal in a tab has a CWD.
// Accepts both index refs ("terminal:N"/"pane:N") and id refs ("tid:UUID").
func (g *GhosttyClient) waitForTerminalReady(workspaceRef, terminalRef string) {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return
	}
	target, err := ghosttyTerminalSpecifier(terminalRef, tabIdx, g.mustWindowClause())
	if err != nil {
		return
	}
	deadline := time.Now().Add(NewWorkspaceDeadline)
	for time.Now().Before(deadline) {
		cwd, err := g.runScript(fmt.Sprintf(
			`tell application "%s" to working directory of %s`, g.appName(), target,
		))
		if err == nil && cwd != "" {
			return
		}
		time.Sleep(PollInterval)
	}
}

// FocusPane focuses a terminal. Accepts both index refs ("pane:N") and id
// refs ("tid:UUID") — ids are immune to the re-indexing Ghostty performs
// when splits are inserted.
func (g *GhosttyClient) FocusPane(paneRef, workspaceRef string) error {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return err
	}
	target, err := ghosttyTerminalSpecifier(paneRef, tabIdx, g.mustWindowClause())
	if err != nil {
		return err
	}
	_, err = g.runScript(fmt.Sprintf(
		`tell application "%s" to focus %s`, g.appName(), target,
	))
	return err
}

// FirstSurfaceRef resolves the id of the tab's first (and, right after tab
// creation, only) terminal as a stable "tid:" ref. Implements
// FirstSurfaceResolver so restore can address splits at explicit targets.
func (g *GhosttyClient) FirstSurfaceRef(workspaceRef string) string {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return ""
	}
	id, err := g.runScript(fmt.Sprintf(
		`tell application "%s" to id of terminal 1 of tab %d of %s`, g.appName(), tabIdx, g.mustWindowClause(),
	))
	if err != nil || id == "" {
		return ""
	}
	return "tid:" + id
}

func (g *GhosttyClient) Send(workspaceRef, surfaceRef, text string) error {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return err
	}

	if surfaceRef == "" {
		surfaceRef = "terminal:1"
	}
	target, err := ghosttyTerminalSpecifier(surfaceRef, tabIdx, g.mustWindowClause())
	if err != nil {
		return err
	}

	needsEnter := false
	if strings.HasSuffix(text, "\\n") {
		text = strings.TrimSuffix(text, "\\n")
		needsEnter = true
	}

	// Combine input text + enter into a single osascript call.
	// Two separate calls leave a gap where Ghostty can shift focus
	// or process events, causing the enter key to be lost or misrouted.
	var lines []string
	lines = append(lines, g.tell())
	lines = append(lines, fmt.Sprintf(`  set t to %s`, target))
	if text != "" {
		lines = append(lines, fmt.Sprintf(`  input text "%s" to t`, escapeAppleScript(text)))
	}
	if needsEnter {
		lines = append(lines, `  send key "enter" to t`)
	}
	lines = append(lines, `end tell`)

	_, err = g.runScriptLines(lines...)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}

	return nil
}

func (g *GhosttyClient) NewSurface(paneRef, workspaceRef string) (string, error) {
	return "", ErrNotSupported
}

func (g *GhosttyClient) DryRunFormatter() DryRunFormatter { return GhosttyDryRun{} }

func (g *GhosttyClient) CloseWorkspace(ref string) error {
	tabIdx, err := parseTabIndex(ref)
	if err != nil {
		return err
	}
	_, err = g.runScript(fmt.Sprintf(
		`tell application "%s" to close tab (a reference to tab %d of %s)`, g.appName(),
		tabIdx, g.mustWindowClause(),
	))
	return err
}

// surfaceStateFromProbe builds a SurfaceState from the raw AppleScript probe:
// the `working directory` property (OSC 7-fed; empty when the shell doesn't
// emit it) and the terminal's title. The title fallback is stat-validated by
// cwdFromTitle, so junk titles never fake readiness. A shell is considered
// ready once we can see its cwd by either channel — both only report after
// the prompt is up.
func surfaceStateFromProbe(ref, prop, name string) *SurfaceState {
	cwd := prop
	if cwd == "" {
		cwd = cwdFromTitle(name)
	}
	return &SurfaceState{Ref: ref, CWD: cwd, Ready: cwd != ""}
}

// SurfaceState reports a terminal's live state. Implements SurfaceStater,
// enabling real per-surface readiness gating and cd verification on Ghostty
// (previously cds were typed blind after a timeout and could be flushed by
// slow shell startups). workspaceRef is required — Ghostty terminal refs are
// tab-local.
func (g *GhosttyClient) SurfaceState(workspaceRef, surfaceRef string) (*SurfaceState, error) {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return nil, err
	}
	spec, err := ghosttyTerminalSpecifier(surfaceRef, tabIdx, g.mustWindowClause())
	if err != nil {
		return nil, err
	}
	out, err := g.runScriptLines(
		g.tell(),
		fmt.Sprintf(`  set t to %s`, spec),
		`  return (working directory of t) & "|" & (name of t)`,
		`end tell`,
	)
	if err != nil {
		return nil, err
	}
	prop, name, _ := strings.Cut(out, "|")
	return surfaceStateFromProbe(surfaceRef, prop, name), nil
}
