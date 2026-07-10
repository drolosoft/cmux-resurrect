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
func (m *readinessMockClient) RenameWorkspace(ref, title string) error           { return nil }
func (m *readinessMockClient) SelectWorkspace(ref string) error                  { return nil }
func (m *readinessMockClient) NewSplit(dir, ref, surfRef string) (string, error) { return "", nil }
func (m *readinessMockClient) NewPane(opts client.NewPaneOpts) (string, error)   { return "", nil }
func (m *readinessMockClient) NewSurface(paneRef, workspaceRef string) (string, error) {
	return "surface:mock", nil
}
func (m *readinessMockClient) FocusPane(pane, ws string) error         { return nil }
func (m *readinessMockClient) Send(ws, surf, text string) error        { return nil }
func (m *readinessMockClient) PinWorkspace(ref string) error           { return nil }
func (m *readinessMockClient) UnpinWorkspace(ref string) error         { return nil }
func (m *readinessMockClient) CloseWorkspace(ref string) error         { return nil }
func (m *readinessMockClient) DryRunFormatter() client.DryRunFormatter { return client.CmuxDryRun{} }

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

func TestWaitForShellReady_TimeoutOnNoCWD_BestEffort(t *testing.T) {
	// Backends whose CWD reporting never fills (Ghostty without OSC 7 shell
	// integration) would otherwise block every per-pane cd forever: the old
	// contract returned an error on timeout and the caller SKIPPED the cd.
	// New contract: after the full timeout the shell is almost certainly
	// interactive, so return nil (best effort) and let the cd be sent.
	mc := &readinessMockClient{cwdSeq: []string{}, fallback: ""}
	origTimeout := ShellReadyTimeout
	ShellReadyTimeout = 500 * time.Millisecond
	defer func() { ShellReadyTimeout = origTimeout }()

	start := time.Now()
	err := waitForShellReady(mc, "workspace:1", "")
	if err != nil {
		t.Fatalf("timeout must be best-effort (nil), got error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 400*time.Millisecond {
		t.Errorf("returned in %v — must still wait out the timeout before giving up", elapsed)
	}
}

// readyStateMock implements Backend + SurfaceStater with a scripted
// readiness/cwd timeline and records every Send.
type readyStateMock struct {
	readinessMockClient
	mu      sync.Mutex
	readyAt time.Time
	cwd     string // reported once a send has landed
	wantCWD string
	sends   []time.Time
	start   time.Time
}

func (m *readyStateMock) SurfaceState(_, ref string) (*client.SurfaceState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ready := time.Now().After(m.readyAt)
	return &client.SurfaceState{Ref: ref, CWD: m.cwd, Ready: ready}, nil
}

func (m *readyStateMock) Send(ws, surf, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sends = append(m.sends, time.Now())
	if time.Now().After(m.readyAt) {
		m.cwd = m.wantCWD // a send into a READY shell lands
	}
	return nil
}

func TestWaitForShellReady_WaitsPastHeuristicTimeoutForReadiness(t *testing.T) {
	// A slow shell (mail check, plugins) can take longer than the heuristic
	// timeout to become interactive. On backends with reliable per-surface
	// readiness (cmux), crex must keep waiting for Ready instead of giving
	// up at the heuristic timeout and typing into a dead shell.
	origHeur, origSurf := ShellReadyTimeout, SurfaceReadyTimeout
	ShellReadyTimeout = 100 * time.Millisecond
	SurfaceReadyTimeout = 2 * time.Second
	defer func() { ShellReadyTimeout, SurfaceReadyTimeout = origHeur, origSurf }()

	m := &readyStateMock{readyAt: time.Now().Add(400 * time.Millisecond)}
	start := time.Now()
	if err := waitForShellReady(m, "workspace:1", "surface:1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 350*time.Millisecond {
		t.Errorf("returned after %v — gave up before the shell became ready (heuristic timeout leak)", elapsed)
	}
}

func TestVerifyCWD_NeverSendsIntoNotReadyShell(t *testing.T) {
	// Typing into a shell that isn't at its prompt is lost AND leaves
	// visible junk. verifyCWD must poll silently until Ready, then send.
	origVerify, origSurf := CWDVerifyTimeout, SurfaceReadyTimeout
	CWDVerifyTimeout = 500 * time.Millisecond
	SurfaceReadyTimeout = 2 * time.Second
	defer func() { CWDVerifyTimeout, SurfaceReadyTimeout = origVerify, origSurf }()

	m := &readyStateMock{
		readyAt: time.Now().Add(300 * time.Millisecond),
		wantCWD: "/tmp/target",
		start:   time.Now(),
	}
	r := &Restorer{Client: m}
	r.verifyCWD("workspace:1", "surface:1", "/tmp/target")

	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sends) == 0 {
		t.Fatal("verifyCWD never sent the cd")
	}
	for i, ts := range m.sends {
		if ts.Before(m.readyAt) {
			t.Errorf("send %d fired %v BEFORE the shell was ready", i, m.readyAt.Sub(ts))
		}
	}
	if m.cwd != "/tmp/target" {
		t.Errorf("cd never landed: cwd = %q", m.cwd)
	}
}
