package client

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CLIClient implements CmuxClient by exec'ing the cmux binary.
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
	cmd := exec.Command(c.Binary, args...)
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

	// Poll list-workspaces to find the newly created workspace.
	// The new workspace is typically the last one or the selected one.
	var ref string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ws, err := c.ListWorkspaces()
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		// The new workspace becomes selected.
		for _, w := range ws {
			if w.Selected {
				ref = w.Ref
				break
			}
		}
		if ref != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
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

func (c *CLIClient) NewSplit(direction, workspaceRef string) error {
	args := []string{"new-split", direction}
	if workspaceRef != "" {
		args = append(args, "--workspace", workspaceRef)
	}
	_, err := c.run(args...)
	return err
}

func (c *CLIClient) FocusPane(paneRef, workspaceRef string) error {
	args := []string{"focus-pane", "--pane", paneRef}
	if workspaceRef != "" {
		args = append(args, "--workspace", workspaceRef)
	}
	_, err := c.run(args...)
	return err
}

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
