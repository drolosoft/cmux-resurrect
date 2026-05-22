package orchestrate

import (
	"sync"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

// readinessMockClient returns configurable CWD values on successive calls.
type readinessMockClient struct {
	mu       sync.Mutex
	calls    int
	cwdSeq   []string // CWD to return on each SidebarState call
	fallback string
}

func (m *readinessMockClient) SidebarState(ref string) (*client.SidebarState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	idx := m.calls - 1
	if idx < len(m.cwdSeq) {
		cwd := m.cwdSeq[idx]
		if cwd == "" {
			return nil, nil
		}
		return &client.SidebarState{CWD: cwd}, nil
	}
	return &client.SidebarState{CWD: m.fallback}, nil
}

// Implement remaining Backend interface methods as no-ops:
func (m *readinessMockClient) Ping() error                                     { return nil }
func (m *readinessMockClient) Tree() (*client.TreeResponse, error)             { return nil, nil }
func (m *readinessMockClient) ListWorkspaces() ([]client.WorkspaceInfo, error) { return nil, nil }
func (m *readinessMockClient) NewWorkspace(opts client.NewWorkspaceOpts) (string, error) {
	return "", nil
}
func (m *readinessMockClient) RenameWorkspace(ref, title string) error         { return nil }
func (m *readinessMockClient) SelectWorkspace(ref string) error                { return nil }
func (m *readinessMockClient) NewSplit(dir, ref string) (string, error)        { return "", nil }
func (m *readinessMockClient) NewPane(opts client.NewPaneOpts) (string, error) { return "", nil }
func (m *readinessMockClient) FocusPane(pane, ws string) error                 { return nil }
func (m *readinessMockClient) Send(ws, surf, text string) error                { return nil }
func (m *readinessMockClient) PinWorkspace(ref string) error                   { return nil }
func (m *readinessMockClient) UnpinWorkspace(ref string) error                 { return nil }
func (m *readinessMockClient) CloseWorkspace(ref string) error                 { return nil }
func (m *readinessMockClient) DryRunFormatter() client.DryRunFormatter         { return client.CmuxDryRun{} }

func TestWaitForShellReady_StabilizesQuickly(t *testing.T) {
	mc := &readinessMockClient{cwdSeq: []string{"/tmp"}, fallback: "/tmp"}
	start := time.Now()
	err := waitForShellReady(mc, "workspace:1", "")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("took %v, expected under 2s for immediately stable shell", elapsed)
	}
}

func TestWaitForShellReady_WaitsForCWD(t *testing.T) {
	mc := &readinessMockClient{
		cwdSeq:   []string{"", "", "", "/tmp"},
		fallback: "/tmp",
	}
	err := waitForShellReady(mc, "workspace:1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForShellReady_ChangingCWDStillCompletes(t *testing.T) {
	mc := &readinessMockClient{
		cwdSeq:   []string{"/tmp/a", "/tmp/b", "/tmp/c", "/tmp/c"},
		fallback: "/tmp/c",
	}
	err := waitForShellReady(mc, "workspace:1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForShellReady_TimeoutOnNoCWD(t *testing.T) {
	mc := &readinessMockClient{cwdSeq: []string{}, fallback: ""}
	origTimeout := ShellReadyTimeout
	ShellReadyTimeout = 500 * time.Millisecond
	defer func() { ShellReadyTimeout = origTimeout }()

	err := waitForShellReady(mc, "workspace:1", "")
	if err == nil {
		t.Fatal("expected timeout error when CWD never appears")
	}
}
