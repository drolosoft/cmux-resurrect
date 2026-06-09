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
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cmux %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *CLIClient) Ping() error {
	_, err := c.run("ping")
	return err
}

func (c *CLIClient) Tree() (*TreeResponse, error) {
	out, err := c.run("tree", "--json")
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
	// Snapshot existing workspace refs before creation.
	before := make(map[string]bool)
	if wsList, err := c.ListWorkspaces(); err == nil {
		for _, w := range wsList {
			before[w.Ref] = true
		}
	}

	args := []string{"new-workspace"}
	if opts.CWD != "" {
		args = append(args, "--cwd", opts.CWD)
	}
	if opts.Command != "" {
		args = append(args, "--command", opts.Command)
	}
	_, err := c.run(args...)
	if err != nil {
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

func (c *CLIClient) NewSplit(direction, workspaceRef string) (string, error) {
	// Snapshot surface refs before split so we can detect the new one.
	before := make(map[string]bool)
	if workspaceRef != "" {
		if tree, err := c.Tree(); err == nil {
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
		}
	}

	args := []string{"new-split", direction}
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

	return "", fmt.Errorf("split created but could not determine new surface ref")
}

func (c *CLIClient) NewPane(opts NewPaneOpts) (string, error) {
	// Snapshot surface refs before creation so we can detect the new one.
	before := make(map[string]bool)
	if opts.WorkspaceRef != "" {
		if tree, err := c.Tree(); err == nil {
			for _, w := range tree.Windows {
				for _, ws := range w.Workspaces {
					if ws.Ref != opts.WorkspaceRef {
						continue
					}
					for _, p := range ws.Panes {
						for _, s := range p.Surfaces {
							before[s.Ref] = true
						}
					}
				}
			}
		}
	}

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
	if _, err := c.run(args...); err != nil {
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
	before := make(map[string]bool)
	if workspaceRef != "" {
		if tree, err := c.Tree(); err == nil {
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

func (c *CLIClient) FocusPane(paneRef, workspaceRef string) error {
	args := []string{"focus-pane", "--pane", paneRef}
	if workspaceRef != "" {
		args = append(args, "--workspace", workspaceRef)
	}
	_, err := c.run(args...)
	return err
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
	out, err := c.run("tree", "--json", "--id-format", "both")
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
	// Strip "pane:" prefix when workspace is specified — cmux interprets
	// "pane:N" as a global ref, but we pass workspace-local indexes.
	ref := opts.PaneRef
	if opts.WorkspaceRef != "" && strings.HasPrefix(ref, "pane:") {
		ref = strings.TrimPrefix(ref, "pane:")
	}
	args := []string{"resize-pane", "--pane", ref, "-" + opts.Direction, "--amount", fmt.Sprintf("%d", opts.Amount)}
	if opts.WorkspaceRef != "" {
		args = append(args, "--workspace", opts.WorkspaceRef)
	}
	_, err := c.run(args...)
	return err
}
