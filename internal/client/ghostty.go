package client

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// GhosttyClient implements Backend for Ghostty (macOS only, requires 1.3+).
//
// Limitations vs cmux backend:
//   - PinWorkspace is a no-op (Ghostty has no pin concept)
//   - SidebarState returns no git info (not exposed by Ghostty API)
//   - Tree enumeration is slower (no single JSON snapshot, must loop via AppleScript)
//   - Split sizing cannot be controlled (always equal splits)
//   - AppleScript API is preview — breaking changes expected in Ghostty 1.4
//   - macOS only until Ghostty ships D-Bus support on Linux
type GhosttyClient struct {
	Timeout time.Duration
}

// NewGhosttyClient creates a GhosttyClient with sensible defaults.
func NewGhosttyClient() *GhosttyClient {
	return &GhosttyClient{
		Timeout: 10 * time.Second,
	}
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
	out, err := g.runScript(`tell application "System Events" to (name of processes) contains "Ghostty"`)
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

// --- Stub methods (replaced in subsequent tasks) ---

func (g *GhosttyClient) Tree() (*TreeResponse, error) {
	return nil, fmt.Errorf("ghostty: Tree not yet implemented")
}

func (g *GhosttyClient) SidebarState(workspaceRef string) (*SidebarState, error) {
	return nil, fmt.Errorf("ghostty: SidebarState not yet implemented")
}

func (g *GhosttyClient) ListWorkspaces() ([]WorkspaceInfo, error) {
	return nil, fmt.Errorf("ghostty: ListWorkspaces not yet implemented")
}

func (g *GhosttyClient) NewWorkspace(opts NewWorkspaceOpts) (string, error) {
	return "", fmt.Errorf("ghostty: NewWorkspace not yet implemented")
}

func (g *GhosttyClient) RenameWorkspace(ref, title string) error {
	return fmt.Errorf("ghostty: RenameWorkspace not yet implemented")
}

func (g *GhosttyClient) SelectWorkspace(ref string) error {
	return fmt.Errorf("ghostty: SelectWorkspace not yet implemented")
}

func (g *GhosttyClient) NewSplit(direction, workspaceRef string) (string, error) {
	return "", fmt.Errorf("ghostty: NewSplit not yet implemented")
}

func (g *GhosttyClient) FocusPane(paneRef, workspaceRef string) error {
	return fmt.Errorf("ghostty: FocusPane not yet implemented")
}

func (g *GhosttyClient) Send(workspaceRef, surfaceRef, text string) error {
	return fmt.Errorf("ghostty: Send not yet implemented")
}

func (g *GhosttyClient) CloseWorkspace(ref string) error {
	return fmt.Errorf("ghostty: CloseWorkspace not yet implemented")
}
