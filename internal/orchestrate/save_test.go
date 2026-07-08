package orchestrate

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// mockClient implements client.Backend for testing.
type mockClient struct {
	treeResp     *client.TreeResponse
	sidebarCWDs  map[string]string
	pingErr      error
	workspaceSeq int
}

func (m *mockClient) Ping() error { return m.pingErr }

func (m *mockClient) Tree() (*client.TreeResponse, error) {
	return m.treeResp, nil
}

func (m *mockClient) SidebarState(ref string) (*client.SidebarState, error) {
	cwd, ok := m.sidebarCWDs[ref]
	if !ok {
		cwd = "/tmp/unknown"
	}
	return &client.SidebarState{CWD: cwd, FocusedCWD: cwd}, nil
}

func (m *mockClient) ListWorkspaces() ([]client.WorkspaceInfo, error) {
	return nil, nil
}

func (m *mockClient) NewWorkspace(opts client.NewWorkspaceOpts) (string, error) {
	m.workspaceSeq++
	return "workspace:new", nil
}

func (m *mockClient) RenameWorkspace(ref, title string) error           { return nil }
func (m *mockClient) SelectWorkspace(ref string) error                  { return nil }
func (m *mockClient) NewSplit(dir, ref, surfRef string) (string, error) { return "surface:mock", nil }
func (m *mockClient) NewPane(opts client.NewPaneOpts) (string, error) {
	return "surface:new", nil
}
func (m *mockClient) NewSurface(paneRef, workspaceRef string) (string, error) {
	return "surface:mock", nil
}
func (m *mockClient) FocusPane(pane, ws string) error         { return nil }
func (m *mockClient) Send(ws, surf, text string) error        { return nil }
func (m *mockClient) PinWorkspace(ref string) error           { return nil }
func (m *mockClient) UnpinWorkspace(ref string) error         { return nil }
func (m *mockClient) CloseWorkspace(ref string) error         { return nil }
func (m *mockClient) DryRunFormatter() client.DryRunFormatter { return client.CmuxDryRun{} }

func TestSave_FromFixture(t *testing.T) {
	// Load tree fixture.
	data, err := os.ReadFile("../../testdata/responses/tree-6-workspaces.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var treeResp client.TreeResponse
	if err := json.Unmarshal(data, &treeResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	mc := &mockClient{
		treeResp: &treeResp,
		sidebarCWDs: map[string]string{
			"workspace:1": "/home/user/projects/api-server",
			"workspace:2": "/home/user/Documents/notes",
			"workspace:3": "/home/user/projects/webapp",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	layout, err := saver.Save("test-session", "unit test")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	if layout.Name != "test-session" {
		t.Errorf("Name = %q", layout.Name)
	}
	if len(layout.Workspaces) != 3 {
		t.Fatalf("Workspaces = %d, want 3", len(layout.Workspaces))
	}

	// First workspace should have 2 panes (it has 2 in the fixture).
	ws0 := layout.Workspaces[0]
	if ws0.Title != "0 api-server" {
		t.Errorf("ws0.Title = %q", ws0.Title)
	}
	if ws0.CWD != "/home/user/projects/api-server" {
		t.Errorf("ws0.CWD = %q", ws0.CWD)
	}
	if len(ws0.Panes) != 2 {
		t.Errorf("ws0.Panes = %d, want 2", len(ws0.Panes))
	}
	// Second pane should default to split "right".
	if ws0.Panes[1].Split != "right" {
		t.Errorf("ws0.Panes[1].Split = %q, want right", ws0.Panes[1].Split)
	}

	// Verify file was written.
	if !store.Exists("test-session") {
		t.Error("layout file not written")
	}
}

func TestSave_MergePreservesUserEdits(t *testing.T) {
	data, _ := os.ReadFile("../../testdata/responses/tree-6-workspaces.json")
	var treeResp client.TreeResponse
	_ = json.Unmarshal(data, &treeResp)

	mc := &mockClient{
		treeResp: &treeResp,
		sidebarCWDs: map[string]string{
			"workspace:1": "/home/user/projects/api-server",
			"workspace:2": "/home/user/Documents/notes",
			"workspace:3": "/home/user/projects/webapp",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	// First save.
	_, _ = saver.Save("merge-test", "")

	// Manually edit the saved file to add user customizations.
	layout, _ := store.Load("merge-test")
	if len(layout.Workspaces[0].Panes) > 1 {
		layout.Workspaces[0].Panes[1].Split = "down"
		layout.Workspaces[0].Panes[1].Command = "make watch"
	}
	layout.Description = "my custom description"
	_ = store.Save("merge-test", layout)

	// Second save should preserve user edits.
	layout2, err := saver.Save("merge-test", "")
	if err != nil {
		t.Fatalf("second save: %v", err)
	}

	if layout2.Description != "my custom description" {
		t.Errorf("Description = %q, want 'my custom description'", layout2.Description)
	}
	if len(layout2.Workspaces[0].Panes) > 1 {
		if layout2.Workspaces[0].Panes[1].Split != "down" {
			t.Errorf("Split = %q, want down (user edit)", layout2.Workspaces[0].Panes[1].Split)
		}
		if layout2.Workspaces[0].Panes[1].Command != "make watch" {
			t.Errorf("Command = %q, want 'make watch' (user edit)", layout2.Workspaces[0].Panes[1].Command)
		}
	}
}

// TestSave_PreservesWorkspaceDescription verifies that a user-edited
// per-workspace description survives a re-save. cmux itself doesn't
// expose descriptions through Tree/SidebarState, so crex keeps them as
// a user-annotated field (aligned with cmux v0.63.2's "editable
// workspace descriptions" feature).
func TestSave_PreservesWorkspaceDescription(t *testing.T) {
	data, _ := os.ReadFile("../../testdata/responses/tree-6-workspaces.json")
	var treeResp client.TreeResponse
	_ = json.Unmarshal(data, &treeResp)

	mc := &mockClient{
		treeResp: &treeResp,
		sidebarCWDs: map[string]string{
			"workspace:1": "/home/user/projects/api-server",
			"workspace:2": "/home/user/Documents/notes",
			"workspace:3": "/home/user/projects/webapp",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	// First save.
	if _, err := saver.Save("desc-test", ""); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Annotate workspace[0] with a description.
	layout, _ := store.Load("desc-test")
	layout.Workspaces[0].Description = "backend API — reads postgres"
	if err := store.Save("desc-test", layout); err != nil {
		t.Fatalf("annotated save: %v", err)
	}

	// Re-save — the live tree doesn't expose descriptions, so the
	// merge must preserve the annotation.
	layout2, err := saver.Save("desc-test", "")
	if err != nil {
		t.Fatalf("second save: %v", err)
	}

	if got := layout2.Workspaces[0].Description; got != "backend API — reads postgres" {
		t.Errorf("Workspaces[0].Description = %q, want preserved annotation", got)
	}
	// Other workspaces should remain empty (no bleed).
	for i := 1; i < len(layout2.Workspaces); i++ {
		if got := layout2.Workspaces[i].Description; got != "" {
			t.Errorf("Workspaces[%d].Description = %q, want empty", i, got)
		}
	}
}

// --- Revision tracking tests ---

func TestSave_RevisionIncrements(t *testing.T) {
	data, err := os.ReadFile("../../testdata/responses/tree-6-workspaces.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var treeResp client.TreeResponse
	if err := json.Unmarshal(data, &treeResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	mc := &mockClient{
		treeResp: &treeResp,
		sidebarCWDs: map[string]string{
			"workspace:1": "/home/user/projects/api-server",
			"workspace:2": "/home/user/Documents/notes",
			"workspace:3": "/home/user/projects/webapp",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	// First save → Revision should be 1.
	layout1, err := saver.Save("rev-test", "")
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	if layout1.Revision != 1 {
		t.Errorf("first save: Revision = %d, want 1", layout1.Revision)
	}

	// Second save with same state → Revision should stay 1 (no content change).
	layout2, err := saver.Save("rev-test", "")
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if layout2.Revision != 1 {
		t.Errorf("second save (same state): Revision = %d, want 1", layout2.Revision)
	}
}

func TestSave_RevisionIncrementsOnChange(t *testing.T) {
	data, err := os.ReadFile("../../testdata/responses/tree-6-workspaces.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var treeResp client.TreeResponse
	if err := json.Unmarshal(data, &treeResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	mc := &mockClient{
		treeResp: &treeResp,
		sidebarCWDs: map[string]string{
			"workspace:1": "/home/user/projects/api-server",
			"workspace:2": "/home/user/Documents/notes",
			"workspace:3": "/home/user/projects/webapp",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	// First save → Revision = 1.
	layout1, err := saver.Save("rev-change-test", "")
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	if layout1.Revision != 1 {
		t.Errorf("first save: Revision = %d, want 1", layout1.Revision)
	}

	// Change CWD for workspace:1 — this will produce different content.
	mc.sidebarCWDs["workspace:1"] = "/home/user/projects/different-path"

	// Second save → Revision should be 2.
	layout2, err := saver.Save("rev-change-test", "")
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if layout2.Revision != 2 {
		t.Errorf("second save (changed state): Revision = %d, want 2", layout2.Revision)
	}
}

// --- layoutContentChanged tests ---

func baseLayout() *model.Layout {
	return &model.Layout{
		Name:        "test",
		Description: "desc",
		Version:     1,
		Revision:    0,
		SavedAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Workspaces: []model.Workspace{
			{
				Title:  "ws1",
				CWD:    "/home/user",
				Pinned: false,
				Active: true,
				Panes: []model.Pane{
					{Type: "terminal", Split: "", Command: "vim", URL: "", Focus: true},
					{Type: "terminal", Split: "right", Command: "make watch", URL: "", Focus: false},
				},
			},
		},
	}
}

func TestLayoutContentChanged_Identical(t *testing.T) {
	a := baseLayout()
	b := baseLayout()
	if layoutContentChanged(a, b) {
		t.Error("identical layouts should not be changed")
	}
}

func TestLayoutContentChanged_DifferentWorkspaceCount(t *testing.T) {
	a := baseLayout()
	b := baseLayout()
	b.Workspaces = append(b.Workspaces, model.Workspace{
		Title: "ws2",
		CWD:   "/home/user/extra",
		Panes: []model.Pane{{Type: "terminal"}},
	})
	if !layoutContentChanged(a, b) {
		t.Error("different workspace count should report changed")
	}
}

func TestLayoutContentChanged_DifferentCommand(t *testing.T) {
	a := baseLayout()
	b := baseLayout()
	b.Workspaces[0].Panes[0].Command = "nvim"
	if !layoutContentChanged(a, b) {
		t.Error("different pane command should report changed")
	}
}

func TestLayoutContentChanged_DifferentPaneCount(t *testing.T) {
	a := baseLayout()
	b := baseLayout()
	b.Workspaces[0].Panes = b.Workspaces[0].Panes[:1]
	if !layoutContentChanged(a, b) {
		t.Error("different pane count should report changed")
	}
}

func TestLayoutContentChanged_IgnoresDescription(t *testing.T) {
	a := baseLayout()
	b := baseLayout()
	b.Description = "completely different description"
	b.Workspaces[0].Description = "pane-level description"
	if layoutContentChanged(a, b) {
		t.Error("description-only difference should not report changed")
	}
}

func TestLayoutContentChanged_IgnoresSavedAt(t *testing.T) {
	a := baseLayout()
	b := baseLayout()
	b.SavedAt = time.Now().Add(24 * time.Hour)
	b.Revision = 42
	if layoutContentChanged(a, b) {
		t.Error("SavedAt/Revision-only difference should not report changed")
	}
}

func TestLayoutContentChanged_NilHandling(t *testing.T) {
	a := baseLayout()
	if !layoutContentChanged(nil, a) {
		t.Error("nil vs non-nil should report changed")
	}
	if !layoutContentChanged(a, nil) {
		t.Error("non-nil vs nil should report changed")
	}
	if layoutContentChanged(nil, nil) {
		t.Error("nil vs nil should not report changed")
	}
}

// geometryMockClient extends mockClient with PaneGeometryProvider.
type geometryMockClient struct {
	mockClient
	paneListByRef map[string]*client.PaneListResponse
}

func (m *geometryMockClient) PaneList(workspaceRef string) (*client.PaneListResponse, error) {
	resp, ok := m.paneListByRef[workspaceRef]
	if !ok {
		return nil, fmt.Errorf("no geometry for %s", workspaceRef)
	}
	return resp, nil
}

func TestSave_GeometryInfersAsideLayout(t *testing.T) {
	// Tree with one workspace, 3 panes (aside layout).
	treeResp := &client.TreeResponse{
		Windows: []client.TreeWindow{{
			Ref: "window:1",
			Workspaces: []client.TreeWorkspace{{
				Ref:   "workspace:1",
				Title: "dev",
				Index: 0,
				Panes: []client.TreePane{
					{Index: 0, Focused: true, Surfaces: []client.TreeSurface{{Type: "terminal", TTY: ""}}},
					{Index: 1, Surfaces: []client.TreeSurface{{Type: "browser", URL: strPtr("http://localhost:3000")}}},
					{Index: 2, Surfaces: []client.TreeSurface{{Type: "terminal", TTY: ""}}},
				},
			}},
		}},
	}

	gmc := &geometryMockClient{
		mockClient: mockClient{
			treeResp:    treeResp,
			sidebarCWDs: map[string]string{"workspace:1": "/home/user/project"},
		},
		paneListByRef: map[string]*client.PaneListResponse{
			"workspace:1": {
				WorkspaceRef:   "workspace:1",
				ContainerFrame: client.ContainerFrame{Width: 1000, Height: 800},
				Panes: []client.PaneListPane{
					{Index: 0, PixelFrame: client.PixelFrame{X: 0, Y: 0, Width: 500, Height: 800}},
					{Index: 1, PixelFrame: client.PixelFrame{X: 500, Y: 0, Width: 500, Height: 400}},
					{Index: 2, PixelFrame: client.PixelFrame{X: 500, Y: 400, Width: 500, Height: 400}},
				},
			},
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: gmc, Store: store}

	layout, err := saver.Save("geo-test", "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	ws := layout.Workspaces[0]
	if len(ws.Panes) != 3 {
		t.Fatalf("panes = %d, want 3", len(ws.Panes))
	}

	// Pane 0: no split.
	if ws.Panes[0].Split != "" {
		t.Errorf("pane 0: split = %q, want empty", ws.Panes[0].Split)
	}
	// Pane 1: split right (geometry-detected).
	if ws.Panes[1].Split != "right" {
		t.Errorf("pane 1: split = %q, want right", ws.Panes[1].Split)
	}
	// Pane 2: split down (geometry-detected, NOT the default "right").
	if ws.Panes[2].Split != "down" {
		t.Errorf("pane 2: split = %q, want down", ws.Panes[2].Split)
	}
}

func strPtr(s string) *string { return &s }

// geometrySurfaceMock combines pane geometry with live surface state, like the
// real cmux backend.
type geometrySurfaceMock struct {
	geometryMockClient
	surfaceCWDs map[string]string
}

func (m *geometrySurfaceMock) SurfaceState(ref string) (*client.SurfaceState, error) {
	cwd, ok := m.surfaceCWDs[ref]
	if !ok {
		return nil, nil
	}
	return &client.SurfaceState{Ref: ref, CWD: cwd, Ready: true}, nil
}

func TestSave_GeometryReordersPanesToCreationOrder(t *testing.T) {
	// Mirrored aside: P0 top-left, P1 bottom-left, P2 full-height right.
	// cmux's visual indexing puts the right pane last, but it must be saved
	// (and thus restored) second — right after P0 — or the left column's
	// down-split divides the full width and every pane lands in the wrong
	// place. CWDs must travel with their panes through the reorder.
	treeResp := &client.TreeResponse{
		Windows: []client.TreeWindow{{
			Ref: "window:1",
			Workspaces: []client.TreeWorkspace{{
				Ref:   "workspace:1",
				Title: "multi-dir",
				Index: 0,
				Panes: []client.TreePane{
					{Index: 0, Focused: true, Surfaces: []client.TreeSurface{{Ref: "surface:1", Type: "terminal"}}},
					{Index: 1, Surfaces: []client.TreeSurface{{Ref: "surface:2", Type: "terminal"}}},
					{Index: 2, Surfaces: []client.TreeSurface{{Ref: "surface:3", Type: "terminal"}}},
				},
			}},
		}},
	}

	mc := &geometrySurfaceMock{
		geometryMockClient: geometryMockClient{
			mockClient: mockClient{
				treeResp:    treeResp,
				sidebarCWDs: map[string]string{"workspace:1": "/home/user"},
			},
			paneListByRef: map[string]*client.PaneListResponse{
				"workspace:1": {
					WorkspaceRef:   "workspace:1",
					ContainerFrame: client.ContainerFrame{Width: 1000, Height: 800},
					Panes: []client.PaneListPane{
						{Index: 0, PixelFrame: client.PixelFrame{X: 0, Y: 0, Width: 500, Height: 400}},
						{Index: 1, PixelFrame: client.PixelFrame{X: 0, Y: 400, Width: 500, Height: 400}},
						{Index: 2, PixelFrame: client.PixelFrame{X: 500, Y: 0, Width: 500, Height: 800}},
					},
				},
			},
		},
		surfaceCWDs: map[string]string{
			"surface:1": "/home/user",           // P0 top-left
			"surface:2": "/home/user/git",       // P1 bottom-left
			"surface:3": "/home/user/downloads", // P2 full-height right
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	layout, err := saver.Save("geo-reorder", "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	ws := layout.Workspaces[0]
	if len(ws.Panes) != 3 {
		t.Fatalf("panes = %d, want 3", len(ws.Panes))
	}

	// Creation order: P0, P2 (right pane FIRST), P1. Every split carries an
	// explicit focus target (cmux keeps focus on the split pane, not the new
	// one), so both later panes split P0 at its live index 0.
	p0, p1, p2 := ws.Panes[0], ws.Panes[1], ws.Panes[2]
	if p0.Index != 0 || p0.Split != "" || p0.CWD != "" {
		t.Errorf("pane[0] = index %d split %q cwd %q, want index 0, no split, no cwd", p0.Index, p0.Split, p0.CWD)
	}
	if p1.Index != 2 || p1.Split != "right" || p1.FocusTarget != 0 {
		t.Errorf("pane[1] = index %d split %q focus %d, want index 2, right, 0", p1.Index, p1.Split, p1.FocusTarget)
	}
	if p1.CWD != "/home/user/downloads" {
		t.Errorf("pane[1] cwd = %q, want /home/user/downloads (must travel with the pane)", p1.CWD)
	}
	if p2.Index != 1 || p2.Split != "down" || p2.FocusTarget != 0 {
		t.Errorf("pane[2] = index %d split %q focus %d, want index 1, down, 0", p2.Index, p2.Split, p2.FocusTarget)
	}
	if p2.CWD != "/home/user/git" {
		t.Errorf("pane[2] cwd = %q, want /home/user/git (must travel with the pane)", p2.CWD)
	}
}

func TestMergeUserEdits_NoBrowserCommandLeak(t *testing.T) {
	live := &model.Layout{
		Name: "test",
		Workspaces: []model.Workspace{{
			Title: "ws1",
			Panes: []model.Pane{
				{Type: "terminal"},
				{Type: "terminal", Split: "right", Command: "lnav /tmp/app.log"},
				{Type: "browser", Split: "right", URL: "http://localhost:3000"},
			},
		}},
	}
	existing := &model.Layout{
		Name: "test",
		Workspaces: []model.Workspace{{
			Title: "ws1",
			Panes: []model.Pane{
				{Type: "terminal"},
				{Type: "terminal", Split: "right", Command: "lnav /tmp/app.log"},
				{Type: "browser", Split: "right", Command: "lnav /tmp/app.log", URL: "http://localhost:3000"},
			},
		}},
	}

	mergeUserEdits(live, existing)

	// The browser pane must NOT inherit the terminal command.
	if got := live.Workspaces[0].Panes[2].Command; got != "" {
		t.Errorf("browser pane leaked command = %q, want empty", got)
	}
}

func TestSave_NoGeometry_FallsBackToRight(t *testing.T) {
	// mockClient does NOT implement PaneGeometryProvider.
	treeResp := &client.TreeResponse{
		Windows: []client.TreeWindow{{
			Ref: "window:1",
			Workspaces: []client.TreeWorkspace{{
				Ref:   "workspace:1",
				Title: "compat",
				Index: 0,
				Panes: []client.TreePane{
					{Index: 0, Focused: true, Surfaces: []client.TreeSurface{{Type: "terminal", TTY: ""}}},
					{Index: 1, Surfaces: []client.TreeSurface{{Type: "terminal", TTY: ""}}},
					{Index: 2, Surfaces: []client.TreeSurface{{Type: "terminal", TTY: ""}}},
				},
			}},
		}},
	}

	mc := &mockClient{
		treeResp:    treeResp,
		sidebarCWDs: map[string]string{"workspace:1": "/tmp"},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	layout, err := saver.Save("compat-test", "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	ws := layout.Workspaces[0]
	for i := 1; i < len(ws.Panes); i++ {
		if ws.Panes[i].Split != "right" {
			t.Errorf("pane %d: split = %q, want right (default)", i, ws.Panes[i].Split)
		}
		if ws.Panes[i].SplitRatio != 0 {
			t.Errorf("pane %d: split_ratio = %f, want 0 (not set)", i, ws.Panes[i].SplitRatio)
		}
	}
}

func TestSplitRatio_TOMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "ratio-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{{
			Title: "ws1",
			CWD:   "/tmp",
			Panes: []model.Pane{
				{Type: "terminal", Focus: true, FocusTarget: -1},
				{Type: "terminal", Split: "right", SplitRatio: 0.30, Index: 1, FocusTarget: -1},
				{Type: "terminal", Split: "down", Index: 2, FocusTarget: -1},
			},
		}},
	}

	if err := store.Save("ratio-test", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Load("ratio-test")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	pane1 := loaded.Workspaces[0].Panes[1]
	if pane1.SplitRatio != 0.30 {
		t.Errorf("pane 1: split_ratio = %f, want 0.30", pane1.SplitRatio)
	}

	pane2 := loaded.Workspaces[0].Panes[2]
	if pane2.SplitRatio != 0 {
		t.Errorf("pane 2: split_ratio = %f, want 0", pane2.SplitRatio)
	}
}

// surfaceStateMock is a mockClient that also implements client.SurfaceStater,
// like the real cmux backend.
type surfaceStateMock struct {
	mockClient
	surfaceCWDs map[string]string
}

func (m *surfaceStateMock) SurfaceState(ref string) (*client.SurfaceState, error) {
	cwd, ok := m.surfaceCWDs[ref]
	if !ok {
		return nil, nil
	}
	return &client.SurfaceState{Ref: ref, CWD: cwd, Ready: true}, nil
}

func TestSave_PerPaneCWD_FallsBackToSurfaceState(t *testing.T) {
	// Current cmux builds report no tty for surfaces in `tree --json`, so
	// TTY-based foreground CWD capture yields nothing. Save must fall back
	// to the backend's live surface state or the per-pane CWD round-trip
	// breaks (GitHub #8).
	tree := &client.TreeResponse{
		Windows: []client.TreeWindow{{
			Workspaces: []client.TreeWorkspace{{
				Ref:   "workspace:1",
				Title: "multi-dir",
				Panes: []client.TreePane{
					{
						Index:   0,
						Focused: true,
						Surfaces: []client.TreeSurface{
							{Ref: "surface:1", Type: "terminal"}, // no TTY
							{Ref: "surface:2", Type: "terminal"}, // extra tab, no TTY
						},
					},
					{
						Index: 1,
						Surfaces: []client.TreeSurface{
							{Ref: "surface:3", Type: "terminal"}, // no TTY
						},
					},
				},
			}},
		}},
	}

	mc := &surfaceStateMock{
		mockClient: mockClient{
			treeResp:    tree,
			sidebarCWDs: map[string]string{"workspace:1": "/home/user"},
		},
		surfaceCWDs: map[string]string{
			"surface:1": "/home/user/docs",
			"surface:2": "/home/user/downloads",
			"surface:3": "/home/user/pictures",
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	layout, err := saver.Save("cwd-fallback", "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if len(layout.Workspaces) != 1 {
		t.Fatalf("Workspaces = %d, want 1", len(layout.Workspaces))
	}

	ws := layout.Workspaces[0]
	if len(ws.Panes) != 2 {
		t.Fatalf("Panes = %d, want 2", len(ws.Panes))
	}
	if got := ws.Panes[0].CWD; got != "/home/user/docs" {
		t.Errorf("pane 0 CWD = %q, want /home/user/docs", got)
	}
	if len(ws.Panes[0].Surfaces) != 1 {
		t.Fatalf("pane 0 extra surfaces = %d, want 1", len(ws.Panes[0].Surfaces))
	}
	if got := ws.Panes[0].Surfaces[0].CWD; got != "/home/user/downloads" {
		t.Errorf("pane 0 surface 1 CWD = %q, want /home/user/downloads", got)
	}
	if got := ws.Panes[1].CWD; got != "/home/user/pictures" {
		t.Errorf("pane 1 CWD = %q, want /home/user/pictures", got)
	}
}

func TestSave_PerPaneCWD_SurfaceStateEqualToWorkspaceCWDOmitted(t *testing.T) {
	// A surface whose live CWD matches the workspace CWD must not clutter
	// the layout with a redundant per-pane cwd.
	tree := &client.TreeResponse{
		Windows: []client.TreeWindow{{
			Workspaces: []client.TreeWorkspace{{
				Ref:   "workspace:1",
				Title: "same-dir",
				Panes: []client.TreePane{{
					Index:   0,
					Focused: true,
					Surfaces: []client.TreeSurface{
						{Ref: "surface:1", Type: "terminal"},
					},
				}},
			}},
		}},
	}

	mc := &surfaceStateMock{
		mockClient: mockClient{
			treeResp:    tree,
			sidebarCWDs: map[string]string{"workspace:1": "/home/user"},
		},
		surfaceCWDs: map[string]string{"surface:1": "/home/user"},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	layout, err := saver.Save("cwd-same", "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := layout.Workspaces[0].Panes[0].CWD; got != "" {
		t.Errorf("pane 0 CWD = %q, want empty (matches workspace cwd)", got)
	}
}
