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
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return nil, err
	}

	cwd, err := g.runScriptLines(
		`tell application "Ghostty"`,
		fmt.Sprintf(`  set focTerm to focused terminal of tab %d of front window`, tabIdx),
		`  return working directory of focTerm`,
		`end tell`,
	)
	if err != nil {
		return nil, fmt.Errorf("sidebar state: %w", err)
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
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *GhosttyClient) gitDirty(cwd string) bool {
	cmd := exec.Command("git", "-C", cwd, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func (g *GhosttyClient) ListWorkspaces() ([]WorkspaceInfo, error) {
	out, err := g.runScriptLines(
		`tell application "Ghostty"`,
		`  set tabCount to count of tabs of front window`,
		`  set output to ""`,
		`  repeat with t from 1 to tabCount`,
		`    set tabName to name of tab t of front window`,
		`    set isSel to selected of tab t of front window`,
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
	beforeOut, err := g.runScript(`tell application "Ghostty" to count of tabs of front window`)
	if err != nil {
		return "", fmt.Errorf("count tabs: %w", err)
	}
	beforeCount, _ := strconv.Atoi(beforeOut)

	if opts.CWD != "" {
		_, err = g.runScriptLines(
			`tell application "Ghostty"`,
			fmt.Sprintf(`  set cfg to new surface configuration from {initial working directory:"%s"}`, opts.CWD),
			`  new tab in front window with configuration cfg`,
			`end tell`,
		)
	} else {
		_, err = g.runScript(`tell application "Ghostty" to new tab in front window`)
	}
	if err != nil {
		return "", fmt.Errorf("new tab: %w", err)
	}

	var ref string
	deadline := time.Now().Add(NewWorkspaceDeadline)
	for time.Now().Before(deadline) {
		afterOut, err := g.runScript(`tell application "Ghostty" to count of tabs of front window`)
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
			`tell application "Ghostty" to working directory of terminal 1 of tab %d of front window`,
			tabIdx,
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
		`tell application "Ghostty" to perform action "set_tab_title:%s" on terminal 1 of tab %d of front window`,
		title, tabIdx,
	))
	return err
}

func (g *GhosttyClient) SelectWorkspace(ref string) error {
	tabIdx, err := parseTabIndex(ref)
	if err != nil {
		return err
	}
	_, err = g.runScript(fmt.Sprintf(
		`tell application "Ghostty" to select tab (a reference to tab %d of front window)`,
		tabIdx,
	))
	return err
}

func (g *GhosttyClient) NewSplit(direction, workspaceRef string) (string, error) {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return "", fmt.Errorf("parse workspace ref: %w", err)
	}

	if direction == "" {
		direction = "right"
	}

	beforeOut, err := g.runScript(fmt.Sprintf(
		`tell application "Ghostty" to count of terminals of tab %d of front window`,
		tabIdx,
	))
	if err != nil {
		return "", fmt.Errorf("count terminals: %w", err)
	}
	beforeCount, _ := strconv.Atoi(beforeOut)

	_, err = g.runScriptLines(
		`tell application "Ghostty"`,
		fmt.Sprintf(`  set focTerm to focused terminal of tab %d of front window`, tabIdx),
		fmt.Sprintf(`  split focTerm direction %s`, direction),
		`end tell`,
	)
	if err != nil {
		return "", fmt.Errorf("split: %w", err)
	}

	deadline := time.Now().Add(NewSplitDeadline)
	for time.Now().Before(deadline) {
		time.Sleep(PollInterval)
		afterOut, err := g.runScript(fmt.Sprintf(
			`tell application "Ghostty" to count of terminals of tab %d of front window`,
			tabIdx,
		))
		if err != nil {
			continue
		}
		afterCount, _ := strconv.Atoi(afterOut)
		if afterCount > beforeCount {
			return fmt.Sprintf("terminal:%d", afterCount), nil
		}
	}
	return "", fmt.Errorf("split created but could not determine new terminal ref")
}

func (g *GhosttyClient) FocusPane(paneRef, workspaceRef string) error {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return err
	}
	termIdx, err := parseTerminalIndex(paneRef)
	if err != nil {
		return err
	}
	_, err = g.runScript(fmt.Sprintf(
		`tell application "Ghostty" to focus terminal %d of tab %d of front window`,
		termIdx, tabIdx,
	))
	return err
}

func (g *GhosttyClient) Send(workspaceRef, surfaceRef, text string) error {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return err
	}

	termIdx := 1
	if surfaceRef != "" {
		termIdx, err = parseTerminalIndex(surfaceRef)
		if err != nil {
			return err
		}
	}

	target := fmt.Sprintf("terminal %d of tab %d of front window", termIdx, tabIdx)

	needsEnter := false
	if strings.HasSuffix(text, "\\n") {
		text = strings.TrimSuffix(text, "\\n")
		needsEnter = true
	}

	if text != "" {
		_, err = g.runScript(fmt.Sprintf(
			`tell application "Ghostty" to input text %q to %s`,
			text, target,
		))
		if err != nil {
			return fmt.Errorf("input text: %w", err)
		}
	}

	if needsEnter {
		_, err = g.runScript(fmt.Sprintf(
			`tell application "Ghostty" to send key "enter" to %s`,
			target,
		))
		if err != nil {
			return fmt.Errorf("send enter: %w", err)
		}
	}

	return nil
}

func (g *GhosttyClient) CloseWorkspace(ref string) error {
	tabIdx, err := parseTabIndex(ref)
	if err != nil {
		return err
	}
	_, err = g.runScript(fmt.Sprintf(
		`tell application "Ghostty" to close tab (a reference to tab %d of front window)`,
		tabIdx,
	))
	return err
}
