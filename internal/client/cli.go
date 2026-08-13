package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CLIClient implements Backend by exec'ing the cmux binary.
type CLIClient struct {
	Binary  string
	Timeout time.Duration
	// SessionDir overrides where cmux's session persistence files are looked
	// up for browser-profile capture (tests). Empty = the real app support dir.
	SessionDir string
}

// NewCLIClient creates a CLIClient with sensible defaults.
func NewCLIClient() *CLIClient {
	return &CLIClient{
		Binary:  "cmux",
		Timeout: 10 * time.Second,
	}
}

func (c *CLIClient) run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Binary, args...)
	// cmux ≥0.64 prints alias/deprecation notices to stderr on legacy command
	// forms. Keep stdout clean for the JSON/text parsers: capture stderr
	// separately (surfaced only in errors) and silence the notices.
	cmd.Env = append(cmd.Environ(), "CMUX_QUIET=1")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("cmux %s: timed out after %s", strings.Join(args, " "), c.Timeout)
		}
		return "", fmt.Errorf("cmux %s: %w\n%s%s", strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (c *CLIClient) Ping() error {
	_, err := c.run("ping")
	return err
}

func (c *CLIClient) Tree() (*TreeResponse, error) {
	return c.tree("tree", "--json")
}

// TreeAllWindows returns every window, not just the focused one. Implements
// MultiWindowTreeProvider: the caller's window is missing from the scoped tree
// whenever another window has focus, and `caller.window_ref` is only useful if
// that window is actually in the response.
func (c *CLIClient) TreeAllWindows() (*TreeResponse, error) {
	return c.tree("tree", "--all", "--json")
}

func (c *CLIClient) tree(args ...string) (*TreeResponse, error) {
	out, err := c.run(args...)
	if err != nil {
		return nil, err
	}
	var resp TreeResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse tree JSON: %w", err)
	}
	return &resp, nil
}

func (c *CLIClient) SidebarState(workspaceRef string) (*SidebarState, error) {
	out, err := c.run("sidebar-state", "--workspace", workspaceRef)
	if err != nil {
		return nil, err
	}
	return ParseSidebarState(out)
}

func (c *CLIClient) ListWorkspaces() ([]WorkspaceInfo, error) {
	out, err := c.run("list-workspaces")
	if err != nil {
		return nil, err
	}
	return ParseListWorkspaces(out)
}

func (c *CLIClient) NewWorkspace(opts NewWorkspaceOpts) (string, error) {
	return c.createWorkspace(opts, "")
}

// NewWorkspaceLayout creates a workspace atomically from a split-tree layout
// JSON (cmux `new-workspace --layout`). Implements LayoutWorkspaceCreator.
func (c *CLIClient) NewWorkspaceLayout(opts NewWorkspaceOpts, layoutJSON string) (string, error) {
	return c.createWorkspace(opts, layoutJSON)
}

func (c *CLIClient) createWorkspace(opts NewWorkspaceOpts, layoutJSON string) (string, error) {
	// Snapshot existing workspace refs before creation. A failed snapshot
	// must abort: diffing against an empty set would make a pre-existing
	// workspace look like the new one, and the caller would then apply the
	// whole layout (rename, splits, commands) to the user's live workspace.
	before := make(map[string]bool)
	wsList, err := c.ListWorkspaces()
	if err != nil {
		return "", fmt.Errorf("pre-create workspace snapshot: %w", err)
	}
	for _, w := range wsList {
		before[w.Ref] = true
	}

	args := []string{"new-workspace"}
	if opts.CWD != "" {
		args = append(args, "--cwd", opts.CWD)
	}
	if opts.Command != "" {
		args = append(args, "--command", opts.Command)
	}
	if layoutJSON != "" {
		args = append(args, "--layout", layoutJSON)
	}
	if _, err := c.run(args...); err != nil {
		return "", err
	}

	// Poll list-workspaces and find the NEW ref (not in the before set).
	var ref string
	deadline := time.Now().Add(NewWorkspaceDeadline)
	for time.Now().Before(deadline) {
		ws, err := c.ListWorkspaces()
		if err != nil {
			time.Sleep(PollInterval)
			continue
		}
		for _, w := range ws {
			if !before[w.Ref] {
				ref = w.Ref
				break
			}
		}
		if ref != "" {
			break
		}
		time.Sleep(PollInterval)
	}
	if ref == "" {
		return "", fmt.Errorf("new workspace created but could not determine ref")
	}
	return ref, nil
}

// RenameSurface titles an individual surface/tab via `cmux rename-tab`. When
// surfaceRef is empty (the workspace's first/default surface, e.g. pane 0) it
// resolves the first surface from the tree rather than letting cmux fall back to
// an arbitrary default. If it can't be resolved, the rename is skipped.
func (c *CLIClient) RenameSurface(workspaceRef, surfaceRef, title string) error {
	if surfaceRef == "" {
		surfaceRef = c.firstSurfaceRef(workspaceRef)
		if surfaceRef == "" {
			return nil
		}
	}
	_, err := c.run("rename-tab", "--workspace", workspaceRef, "--surface", surfaceRef, title)
	return err
}

// SurfaceState returns live state for a specific surface via `cmux rpc
// debug.terminals`, or nil if the surface isn't found. Implements SurfaceStater.
func (c *CLIClient) SurfaceState(_ /* workspaceRef */, surfaceRef string) (*SurfaceState, error) {
	out, err := c.run("rpc", "debug.terminals")
	if err != nil {
		return nil, err
	}
	return parseSurfaceState(out, surfaceRef)
}

// parseSurfaceState extracts a surface's live state from `cmux rpc
// debug.terminals` JSON. Returns nil if the surface isn't present.
//
// Readiness is version-adaptive: cmux ≥0.64 flips runtime_surface_ready at
// first RENDER (before the shell accepts input — typing then loses the text)
// but reports each live shell's tty, so when any terminal in the response
// carries a tty, a surface without one is still spawning and not ready.
// cmux ≤0.63 never reports ttys and its ready flag already meant
// shell-input-ready, so the flag alone stays sufficient there.
func parseSurfaceState(jsonOut, surfaceRef string) (*SurfaceState, error) {
	var resp struct {
		Terminals []struct {
			SurfaceRef string  `json:"surface_ref"`
			CWD        string  `json:"current_directory"`
			Ready      bool    `json:"runtime_surface_ready"`
			TTY        *string `json:"tty"`
		} `json:"terminals"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &resp); err != nil {
		return nil, fmt.Errorf("parse debug.terminals: %w", err)
	}
	ttysReported := false
	for _, t := range resp.Terminals {
		if t.TTY != nil && *t.TTY != "" {
			ttysReported = true
			break
		}
	}
	for _, t := range resp.Terminals {
		if t.SurfaceRef == surfaceRef {
			ready := t.Ready
			if ttysReported && (t.TTY == nil || *t.TTY == "") {
				ready = false
			}
			return &SurfaceState{Ref: t.SurfaceRef, CWD: t.CWD, Ready: ready}, nil
		}
	}
	return nil, nil
}

// FirstSurfaceRef resolves the workspace's first surface ref ("" when it
// can't). Implements FirstSurfaceResolver: restore uses it to address splits
// at explicit targets.
func (c *CLIClient) FirstSurfaceRef(workspaceRef string) string {
	return c.firstSurfaceRef(workspaceRef)
}

// firstSurfaceRef returns the ref of the first surface in a workspace, or "".
func (c *CLIClient) firstSurfaceRef(workspaceRef string) string {
	tree, err := c.Tree()
	if err != nil || tree == nil {
		return ""
	}
	for _, w := range tree.Windows {
		for _, ws := range w.Workspaces {
			if ws.Ref != workspaceRef {
				continue
			}
			for _, p := range ws.Panes {
				for _, s := range p.Surfaces {
					return s.Ref
				}
			}
		}
	}
	return ""
}

func (c *CLIClient) RenameWorkspace(ref, title string) error {
	_, err := c.run("rename-workspace", "--workspace", ref, title)
	return err
}

func (c *CLIClient) SelectWorkspace(ref string) error {
	_, err := c.run("select-workspace", "--workspace", ref)
	return err
}

func (c *CLIClient) PinWorkspace(ref string) error {
	_, err := c.run("workspace-action", "--action", "pin", "--workspace", ref)
	return err
}

func (c *CLIClient) UnpinWorkspace(ref string) error {
	_, err := c.run("workspace-action", "--action", "unpin", "--workspace", ref)
	return err
}

func (c *CLIClient) CloseWorkspace(ref string) error {
	_, err := c.run("close-workspace", "--workspace", ref)
	return err
}

// surfaceSnapshot returns the set of surface refs currently in a workspace.
// New-ref discovery diffs against it; a failed snapshot must abort the caller
// — an empty set would make any pre-existing surface look "new" and route
// cd/commands into the user's live pane.
func (c *CLIClient) surfaceSnapshot(workspaceRef string) (map[string]bool, error) {
	before := make(map[string]bool)
	tree, err := c.Tree()
	if err != nil {
		return nil, fmt.Errorf("pre-op surface snapshot: %w", err)
	}
	for _, w := range tree.Windows {
		for _, ws := range w.Workspaces {
			if ws.Ref != workspaceRef {
				continue
			}
			for _, p := range ws.Panes {
				for _, s := range p.Surfaces {
					before[s.Ref] = true
				}
			}
		}
	}
	return before, nil
}

func (c *CLIClient) NewSplit(direction, workspaceRef, surfaceRef string) (string, error) {
	direction, err := validateSplitDirection(direction)
	if err != nil {
		return "", err
	}

	// Snapshot surface refs before split so we can detect the new one.
	var before map[string]bool
	if workspaceRef != "" {
		var err error
		before, err = c.surfaceSnapshot(workspaceRef)
		if err != nil {
			return "", err
		}
	}

	args := []string{"new-split", direction}
	if workspaceRef != "" {
		args = append(args, "--workspace", workspaceRef)
	}
	if surfaceRef != "" {
		args = append(args, "--surface", surfaceRef)
	}
	if _, err := c.run(args...); err != nil {
		return "", err
	}

	// Find the new surface by diffing against the snapshot.
	if workspaceRef != "" {
		deadline := time.Now().Add(NewSplitDeadline)
		for time.Now().Before(deadline) {
			time.Sleep(PollInterval)
			tree, err := c.Tree()
			if err != nil {
				continue
			}
			for _, w := range tree.Windows {
				for _, ws := range w.Workspaces {
					if ws.Ref != workspaceRef {
						continue
					}
					for _, p := range ws.Panes {
						for _, s := range p.Surfaces {
							if !before[s.Ref] {
								return s.Ref, nil
							}
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("split created but could not determine new surface ref")
}

// newPaneArgs builds the `cmux new-pane` argument list. --profile is emitted
// for browser panes only: cmux >0.64.20 honors it there and rejects it on
// terminal panes; older cmux ignores it either way.
func newPaneArgs(opts NewPaneOpts) []string {
	args := []string{"new-pane"}
	if opts.Type != "" {
		args = append(args, "--type", opts.Type)
	}
	if opts.Direction != "" {
		args = append(args, "--direction", opts.Direction)
	}
	if opts.WorkspaceRef != "" {
		args = append(args, "--workspace", opts.WorkspaceRef)
	}
	if opts.URL != "" {
		args = append(args, "--url", opts.URL)
	}
	if opts.Profile != "" && opts.Type == "browser" {
		args = append(args, "--profile", opts.Profile)
	}
	return args
}

func (c *CLIClient) NewPane(opts NewPaneOpts) (string, error) {
	// Snapshot surface refs before creation so we can detect the new one.
	var before map[string]bool
	if opts.WorkspaceRef != "" {
		var err error
		before, err = c.surfaceSnapshot(opts.WorkspaceRef)
		if err != nil {
			return "", err
		}
	}

	if _, err := c.run(newPaneArgs(opts)...); err != nil {
		return "", err
	}

	// Find the new surface by diffing against the snapshot.
	if opts.WorkspaceRef != "" {
		deadline := time.Now().Add(NewSplitDeadline)
		for time.Now().Before(deadline) {
			time.Sleep(PollInterval)
			tree, err := c.Tree()
			if err != nil {
				continue
			}
			for _, w := range tree.Windows {
				for _, ws := range w.Workspaces {
					if ws.Ref != opts.WorkspaceRef {
						continue
					}
					for _, p := range ws.Panes {
						for _, s := range p.Surfaces {
							if !before[s.Ref] {
								return s.Ref, nil
							}
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("new pane created but could not determine new surface ref")
}

func (c *CLIClient) NewSurface(paneRef, workspaceRef string) (string, error) {
	// Snapshot surface refs before creation so we can detect the new one.
	var before map[string]bool
	if workspaceRef != "" {
		var err error
		before, err = c.surfaceSnapshot(workspaceRef)
		if err != nil {
			return "", err
		}
	}

	args := []string{"new-surface", "--pane", paneRef}
	if workspaceRef != "" {
		args = append(args, "--workspace", workspaceRef)
	}
	if _, err := c.run(args...); err != nil {
		return "", err
	}

	// Find the new surface by diffing against the snapshot.
	if workspaceRef != "" {
		deadline := time.Now().Add(NewSplitDeadline)
		for time.Now().Before(deadline) {
			time.Sleep(PollInterval)
			tree, err := c.Tree()
			if err != nil {
				continue
			}
			for _, w := range tree.Windows {
				for _, ws := range w.Workspaces {
					if ws.Ref != workspaceRef {
						continue
					}
					for _, p := range ws.Panes {
						for _, s := range p.Surfaces {
							if !before[s.Ref] {
								return s.Ref, nil
							}
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("new-surface created but could not determine ref")
}

// FocusSurface selects a single surface (tab) within its pane. Implements
// SurfaceFocuser: this is what makes cmux render the tab and spawn its shell,
// without which any text sent to it is silently dropped (GitHub #8).
func (c *CLIClient) FocusSurface(workspaceRef, surfaceRef string) error {
	if surfaceRef == "" {
		return nil
	}
	params := fmt.Sprintf(`{"workspace_id":%q,"surface_id":%q}`, workspaceRef, surfaceRef)
	_, err := c.run("rpc", "surface.focus", params)
	return err
}

func (c *CLIClient) FocusPane(paneRef, workspaceRef string) error {
	args := []string{"focus-pane", "--pane", paneCLIRef(paneRef, workspaceRef)}
	if workspaceRef != "" {
		args = append(args, "--workspace", workspaceRef)
	}
	_, err := c.run(args...)
	return err
}

// paneCLIRef adapts a pane ref for a cmux CLI flag. cmux reads a "pane:N" string
// as a GLOBAL ref, but crex passes workspace-local indexes; when a workspace is
// specified, strip the "pane:" prefix so cmux reads N as the workspace-local
// index (otherwise it rejects it with "Missing or invalid pane_id"). Real refs
// and the no-workspace case pass through unchanged. Ghostty's own FocusPane
// consumes "pane:N" natively, so this cmux-only adaptation lives in CLIClient.
func paneCLIRef(paneRef, workspaceRef string) string {
	if workspaceRef != "" && strings.HasPrefix(paneRef, "pane:") {
		return strings.TrimPrefix(paneRef, "pane:")
	}
	return paneRef
}

func (c *CLIClient) DryRunFormatter() DryRunFormatter { return CmuxDryRun{} }

func (c *CLIClient) Send(workspaceRef, surfaceRef, text string) error {
	args := []string{"send"}
	if workspaceRef != "" {
		args = append(args, "--workspace", workspaceRef)
	}
	if surfaceRef != "" {
		args = append(args, "--surface", surfaceRef)
	}
	args = append(args, text)
	_, err := c.run(args...)
	return err
}

// PaneList returns pane geometry for a workspace. Implements PaneGeometryProvider.
func (c *CLIClient) PaneList(workspaceRef string) (*PaneListResponse, error) {
	// Resolve workspace ref to UUID (pane.list RPC requires workspace_id).
	uuid, err := c.resolveWorkspaceUUID(workspaceRef)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace UUID: %w", err)
	}

	params := fmt.Sprintf(`{"workspace_id": %q}`, uuid)
	out, err := c.run("rpc", "pane.list", params)
	if err != nil {
		return nil, fmt.Errorf("pane.list: %w", err)
	}

	var resp PaneListResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse pane.list: %w", err)
	}
	return &resp, nil
}

// resolveWorkspaceUUID maps a workspace ref (e.g. "workspace:16") to its UUID
// by querying the tree with --id-format both.
func (c *CLIClient) resolveWorkspaceUUID(workspaceRef string) (string, error) {
	// --all: a ref may belong to a window that does not currently have focus.
	out, err := c.run("tree", "--all", "--json", "--id-format", "both")
	if err != nil {
		return "", err
	}

	// Parse tree with id fields included.
	var raw struct {
		Windows []struct {
			Workspaces []struct {
				ID  string `json:"id"`
				Ref string `json:"ref"`
			} `json:"workspaces"`
		} `json:"windows"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return "", fmt.Errorf("parse tree: %w", err)
	}

	for _, w := range raw.Windows {
		for _, ws := range w.Workspaces {
			if ws.Ref == workspaceRef {
				return ws.ID, nil
			}
		}
	}
	return "", fmt.Errorf("workspace %s not found in tree", workspaceRef)
}

// ResizePane resizes a pane in the given direction. Implements PaneResizer.
func (c *CLIClient) ResizePane(opts ResizePaneOpts) error {
	if opts.Amount <= 0 {
		return nil // no-op for zero or negative
	}
	ref := paneCLIRef(opts.PaneRef, opts.WorkspaceRef)
	args := []string{"resize-pane", "--pane", ref, "-" + opts.Direction, "--amount", fmt.Sprintf("%d", opts.Amount)}
	if opts.WorkspaceRef != "" {
		args = append(args, "--workspace", opts.WorkspaceRef)
	}
	_, err := c.run(args...)
	return err
}
