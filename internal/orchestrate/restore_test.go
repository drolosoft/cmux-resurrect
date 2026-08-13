package orchestrate

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

func TestRestore_DryRun(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "dry-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{
				Title:  "0 dev",
				CWD:    "/tmp/project",
				Pinned: true,
				Index:  0,
				Active: true,
				Panes: []model.Pane{
					{Type: "terminal", Focus: true},
					{Type: "terminal", Split: "right", Command: "go test ./..."},
				},
			},
			{
				Title:  "1 docs",
				CWD:    "/tmp/docs",
				Pinned: false,
				Index:  1,
				Panes: []model.Pane{
					{Type: "terminal", Command: "claude"},
				},
			},
		},
	}
	_ = store.Save("dry-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	result, err := restorer.Restore("dry-test", true, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore dry-run: %v", err)
	}

	if !result.DryRun {
		t.Error("DryRun should be true")
	}
	if result.WorkspacesTotal != 2 {
		t.Errorf("WorkspacesTotal = %d, want 2", result.WorkspacesTotal)
	}
	if result.WorkspacesOK != 2 {
		t.Errorf("WorkspacesOK = %d, want 2", result.WorkspacesOK)
	}
	if len(result.Commands) == 0 {
		t.Error("expected dry-run commands")
	}

	// Verify expected commands.
	hasNewWorkspace := false
	hasRename := false
	hasSplit := false
	hasSend := false
	hasSelect := false
	for _, cmd := range result.Commands {
		if containsStr(cmd, "new-workspace") {
			hasNewWorkspace = true
		}
		if containsStr(cmd, "rename-workspace") {
			hasRename = true
		}
		if containsStr(cmd, "new-split") {
			hasSplit = true
		}
		if containsStr(cmd, "send") {
			hasSend = true
		}
		if containsStr(cmd, "select-workspace") {
			hasSelect = true
		}
	}
	if !hasNewWorkspace {
		t.Error("missing new-workspace command")
	}
	if !hasRename {
		t.Error("missing rename-workspace command")
	}
	if !hasSplit {
		t.Error("missing new-split command")
	}
	if !hasSend {
		t.Error("missing send command")
	}
	if !hasSelect {
		t.Error("missing select-workspace command")
	}
}

func TestRestore_LayoutNotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	mc := &mockClient{}

	restorer := &Restorer{Client: mc, Store: store}
	_, err := restorer.Restore("nonexistent", false, RestoreModeAdd, "", true)
	if err == nil {
		t.Error("expected error for nonexistent layout")
	}
}

func TestRestore_WorkspaceFilter(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "filter-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 dev", CWD: "/tmp/dev", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "1 docs", CWD: "/tmp/docs", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "2 tests", CWD: "/tmp/tests", Index: 2, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("filter-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	result, err := restorer.Restore("filter-test", true, RestoreModeAdd, "1 docs", true)
	if err != nil {
		t.Fatalf("restore with filter: %v", err)
	}
	if result.WorkspacesTotal != 1 {
		t.Errorf("WorkspacesTotal = %d, want 1", result.WorkspacesTotal)
	}
	if result.WorkspacesOK != 1 {
		t.Errorf("WorkspacesOK = %d, want 1", result.WorkspacesOK)
	}
	hasTarget := false
	for _, cmd := range result.Commands {
		if containsStr(cmd, "1 docs") {
			hasTarget = true
		}
		if containsStr(cmd, "0 dev") || containsStr(cmd, "2 tests") {
			t.Errorf("filtered workspace should not appear: %s", cmd)
		}
	}
	if !hasTarget {
		t.Error("expected commands for '1 docs' workspace")
	}
}

func TestRestore_EmptyFilter_RestoresAll(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "all-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 dev", CWD: "/tmp", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "1 docs", CWD: "/tmp", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("all-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	result, err := restorer.Restore("all-test", true, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore all: %v", err)
	}
	if result.WorkspacesTotal != 2 {
		t.Errorf("WorkspacesTotal = %d, want 2", result.WorkspacesTotal)
	}
}

func TestRestore_WorkspaceFilter_SubstringMatch(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "sub-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 🗑️ Trash", CWD: "/tmp/trash", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "⠐ Claude Code", CWD: "/tmp/claude", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "2 tests", CWD: "/tmp/tests", Index: 2, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("sub-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	// "trash" should match "0 🗑️ Trash" (case-insensitive substring).
	result, err := restorer.Restore("sub-test", true, RestoreModeAdd, "trash", true)
	if err != nil {
		t.Fatalf("restore with substring filter: %v", err)
	}
	if result.WorkspacesTotal != 1 {
		t.Errorf("WorkspacesTotal = %d, want 1", result.WorkspacesTotal)
	}
	// Verify the correct workspace was selected.
	hasTarget := false
	for _, cmd := range result.Commands {
		if strings.Contains(cmd, "0 🗑️ Trash") {
			hasTarget = true
		}
		if strings.Contains(cmd, "⠐ Claude Code") || strings.Contains(cmd, "2 tests") {
			t.Errorf("filtered workspace should not appear: %s", cmd)
		}
	}
	if !hasTarget {
		t.Error("expected commands for '0 🗑️ Trash' workspace")
	}

	// "claude" should match "⠐ Claude Code".
	result, err = restorer.Restore("sub-test", true, RestoreModeAdd, "claude", true)
	if err != nil {
		t.Fatalf("restore with substring filter: %v", err)
	}
	if result.WorkspacesTotal != 1 {
		t.Errorf("WorkspacesTotal = %d, want 1", result.WorkspacesTotal)
	}
	// Verify the correct workspace was selected.
	hasTarget = false
	for _, cmd := range result.Commands {
		if strings.Contains(cmd, "⠐ Claude Code") || strings.Contains(cmd, "Claude Code") {
			hasTarget = true
		}
		if strings.Contains(cmd, "0 🗑️ Trash") || strings.Contains(cmd, "2 tests") {
			t.Errorf("filtered workspace should not appear: %s", cmd)
		}
	}
	if !hasTarget {
		t.Error("expected commands for '⠐ Claude Code' workspace")
	}
}

func TestRestore_WorkspaceFilter_NoMatch(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "nomatch-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 dev", CWD: "/tmp", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("nomatch-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	_, err := restorer.Restore("nomatch-test", true, RestoreModeAdd, "zzz", true)
	if err == nil {
		t.Fatal("expected error for non-matching filter")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}

func TestRestore_WorkspaceFilter_AmbiguousMatch(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "ambig-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "0 dev-api", CWD: "/tmp/api", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "1 dev-web", CWD: "/tmp/web", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "2 docs", CWD: "/tmp/docs", Index: 2, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("ambig-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	_, err := restorer.Restore("ambig-test", true, RestoreModeAdd, "dev", true)
	if err == nil {
		t.Fatal("expected error for ambiguous filter")
	}
	if !strings.Contains(err.Error(), "matches multiple") {
		t.Errorf("error = %q, want 'matches multiple'", err.Error())
	}
	if !strings.Contains(err.Error(), "0 dev-api") || !strings.Contains(err.Error(), "1 dev-web") {
		t.Errorf("error should list matching titles: %q", err.Error())
	}
}

func TestRestore_WorkspaceFilter_ExactMatchPriority(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "exact-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "dev", CWD: "/tmp/dev", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "dev-tools", CWD: "/tmp/devtools", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("exact-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	// "dev" exactly matches "dev", should NOT be ambiguous even though "dev-tools" also contains "dev".
	result, err := restorer.Restore("exact-test", true, RestoreModeAdd, "dev", true)
	if err != nil {
		t.Fatalf("exact match should not be ambiguous: %v", err)
	}
	if result.WorkspacesTotal != 1 {
		t.Errorf("WorkspacesTotal = %d, want 1", result.WorkspacesTotal)
	}
}

func TestRestore_WorkspaceFilter_ExactMatchPriority_ReversedOrder(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "exact-rev-test", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "dev-tools", CWD: "/tmp/devtools", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "dev", CWD: "/tmp/dev", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("exact-rev-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	// "dev" should exact-match "dev" at index 1, not pick "dev-tools" at index 0.
	result, err := restorer.Restore("exact-rev-test", true, RestoreModeAdd, "dev", true)
	if err != nil {
		t.Fatalf("exact match should not be ambiguous: %v", err)
	}
	if result.WorkspacesTotal != 1 {
		t.Errorf("WorkspacesTotal = %d, want 1", result.WorkspacesTotal)
	}
}

func TestRestore_BrowserPane_DryRun(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "browser-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{
				Title:  "0 dev",
				CWD:    "/tmp/project",
				Index:  0,
				Active: true,
				Panes: []model.Pane{
					{Type: "terminal", Focus: true},
					{Type: "browser", Split: "right", URL: "https://localhost:3000"},
				},
			},
		},
	}
	_ = store.Save("browser-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	result, err := restorer.Restore("browser-test", true, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore dry-run: %v", err)
	}

	hasBrowserCmd := false
	for _, cmd := range result.Commands {
		if strings.Contains(cmd, "browser") && strings.Contains(cmd, "https://localhost:3000") {
			hasBrowserCmd = true
		}
	}
	if !hasBrowserCmd {
		t.Errorf("expected browser pane command with URL, got commands: %v", result.Commands)
	}
}

func TestRestore_MixedTerminalBrowserPanes_DryRun(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "mixed-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{
				Title:  "0 fullstack",
				CWD:    "/tmp/project",
				Index:  0,
				Active: true,
				Panes: []model.Pane{
					{Type: "terminal", Focus: true, Command: "npm run dev"},
					{Type: "browser", Split: "right", URL: "https://localhost:3000"},
					{Type: "terminal", Split: "down", Command: "npm run test"},
				},
			},
		},
	}
	_ = store.Save("mixed-test", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	result, err := restorer.Restore("mixed-test", true, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore dry-run: %v", err)
	}

	hasSend := false
	hasBrowser := false
	hasSplit := false
	for _, cmd := range result.Commands {
		if strings.Contains(cmd, "npm run dev") {
			hasSend = true
		}
		if strings.Contains(cmd, "browser") && strings.Contains(cmd, "https://localhost:3000") {
			hasBrowser = true
		}
		if strings.Contains(cmd, "new-split") && strings.Contains(cmd, "down") {
			hasSplit = true
		}
	}
	if !hasSend {
		t.Error("missing terminal command 'npm run dev'")
	}
	if !hasBrowser {
		t.Error("missing browser pane command with URL")
	}
	if !hasSplit {
		t.Error("missing terminal split 'down' for third pane")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// syncMockClient implements client.Backend for sync-algorithm tests.
// It tracks which refs were closed, unpinned, and how many workspaces were created.
type syncMockClient struct {
	existingWorkspaces []client.WorkspaceInfo
	callerRef          string
	callerTitle        string
	closedRefs         map[string]bool
	unpinnedRefs       map[string]bool
	createdCount       int
}

func newSyncMockClient(existing []client.WorkspaceInfo, callerRef, callerTitle string) *syncMockClient {
	return &syncMockClient{
		existingWorkspaces: existing,
		callerRef:          callerRef,
		callerTitle:        callerTitle,
		closedRefs:         make(map[string]bool),
		unpinnedRefs:       make(map[string]bool),
	}
}

func (m *syncMockClient) Ping() error { return nil }

func (m *syncMockClient) Tree() (*client.TreeResponse, error) {
	// Build a minimal tree with the caller set and workspaces listed.
	var workspaces []client.TreeWorkspace
	for _, ws := range m.existingWorkspaces {
		workspaces = append(workspaces, client.TreeWorkspace{
			Ref:   ws.Ref,
			Title: ws.Title,
		})
	}
	resp := &client.TreeResponse{
		Windows: []client.TreeWindow{
			{Ref: "window:1", Workspaces: workspaces},
		},
	}
	if m.callerRef != "" {
		resp.Caller = &client.CallerInfo{WorkspaceRef: m.callerRef}
	}
	return resp, nil
}

func (m *syncMockClient) SidebarState(ref string) (*client.SidebarState, error) {
	return &client.SidebarState{CWD: "/tmp"}, nil
}

func (m *syncMockClient) ListWorkspaces() ([]client.WorkspaceInfo, error) {
	return m.existingWorkspaces, nil
}

func (m *syncMockClient) NewWorkspace(opts client.NewWorkspaceOpts) (string, error) {
	m.createdCount++
	return "workspace:new", nil
}

func (m *syncMockClient) RenameWorkspace(ref, title string) error { return nil }
func (m *syncMockClient) SelectWorkspace(ref string) error        { return nil }
func (m *syncMockClient) NewSplit(dir, ref, surfRef string) (string, error) {
	return "surface:mock", nil
}
func (m *syncMockClient) NewPane(opts client.NewPaneOpts) (string, error) {
	return "surface:new", nil
}
func (m *syncMockClient) NewSurface(paneRef, workspaceRef string) (string, error) {
	return "surface:mock", nil
}
func (m *syncMockClient) FocusPane(pane, ws string) error  { return nil }
func (m *syncMockClient) Send(ws, surf, text string) error { return nil }
func (m *syncMockClient) PinWorkspace(ref string) error    { return nil }

func (m *syncMockClient) UnpinWorkspace(ref string) error {
	m.unpinnedRefs[ref] = true
	return nil
}

func (m *syncMockClient) CloseWorkspace(ref string) error {
	m.closedRefs[ref] = true
	return nil
}

func (m *syncMockClient) DryRunFormatter() client.DryRunFormatter { return client.CmuxDryRun{} }

func TestRestore_Replace_SkipsMatchingTitles(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "sync-replace",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "dev", CWD: "/tmp/dev", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "docs", CWD: "/tmp/docs", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "new-tab", CWD: "/tmp/new", Index: 2, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("sync-replace", layout)

	mc := newSyncMockClient(
		[]client.WorkspaceInfo{
			{Ref: "workspace:1", Title: "dev"},
			{Ref: "workspace:2", Title: "stale"},
		},
		"workspace:1", // caller is "dev"
		"dev",
	)

	restorer := &Restorer{Client: mc, Store: store}
	result, err := restorer.Restore("sync-replace", false, RestoreModeReplace, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	// "dev" should NOT be closed (it matches layout).
	if mc.closedRefs["workspace:1"] {
		t.Error("dev (workspace:1) should not be closed — it matches the layout")
	}

	// "stale" should be closed (not in layout).
	if !mc.closedRefs["workspace:2"] {
		t.Error("stale (workspace:2) should be closed — it's not in the layout")
	}

	// "docs" and "new-tab" should be created (not in existing).
	if result.WorkspacesOK != 2 {
		t.Errorf("WorkspacesOK = %d, want 2 (docs + new-tab)", result.WorkspacesOK)
	}

	if result.WorkspacesClosed != 1 {
		t.Errorf("WorkspacesClosed = %d, want 1 (stale)", result.WorkspacesClosed)
	}

	// "dev" should be skipped (already exists), so only 2 created.
	if mc.createdCount != 2 {
		t.Errorf("createdCount = %d, want 2", mc.createdCount)
	}
}

func TestRestore_Add_SkipsMatchingTitles(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "sync-add",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "dev", CWD: "/tmp/dev", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
			{Title: "missing", CWD: "/tmp/missing", Index: 1, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("sync-add", layout)

	mc := newSyncMockClient(
		[]client.WorkspaceInfo{
			{Ref: "workspace:1", Title: "dev"},
			{Ref: "workspace:2", Title: "extra"},
		},
		"workspace:1",
		"dev",
	)

	restorer := &Restorer{Client: mc, Store: store}
	result, err := restorer.Restore("sync-add", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	// "dev" should be skipped (already exists).
	// "missing" should be created.
	if result.WorkspacesOK != 1 {
		t.Errorf("WorkspacesOK = %d, want 1 (missing)", result.WorkspacesOK)
	}

	// "extra" should NOT be closed in add mode.
	if mc.closedRefs["workspace:2"] {
		t.Error("extra (workspace:2) should not be closed in add mode")
	}

	// No workspaces should be closed.
	if len(mc.closedRefs) != 0 {
		t.Errorf("closedRefs = %v, want empty", mc.closedRefs)
	}

	if mc.createdCount != 1 {
		t.Errorf("createdCount = %d, want 1", mc.createdCount)
	}
}

func TestRestore_Replace_UnpinsBeforeClose(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "sync-unpin",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "kept", CWD: "/tmp/kept", Index: 0, Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}},
		},
	}
	_ = store.Save("sync-unpin", layout)

	mc := newSyncMockClient(
		[]client.WorkspaceInfo{
			{Ref: "workspace:1", Title: "kept"},
			{Ref: "workspace:2", Title: "pinned-stale"},
		},
		"workspace:1", // caller is "kept"
		"kept",
	)

	restorer := &Restorer{Client: mc, Store: store}
	result, err := restorer.Restore("sync-unpin", false, RestoreModeReplace, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	// "pinned-stale" should be unpinned THEN closed.
	if !mc.unpinnedRefs["workspace:2"] {
		t.Error("pinned-stale (workspace:2) should be unpinned before close")
	}
	if !mc.closedRefs["workspace:2"] {
		t.Error("pinned-stale (workspace:2) should be closed")
	}

	// "kept" should NOT be unpinned or closed (it's the caller and matches layout).
	if mc.unpinnedRefs["workspace:1"] {
		t.Error("kept (workspace:1) should not be unpinned")
	}
	if mc.closedRefs["workspace:1"] {
		t.Error("kept (workspace:1) should not be closed")
	}

	if result.WorkspacesClosed != 1 {
		t.Errorf("WorkspacesClosed = %d, want 1", result.WorkspacesClosed)
	}
}

// sendRecorderMock records every Send so tests can assert which surface got
// which text on the sequential (non-atomic) restore path.
type sendRecorderMock struct {
	mockClient
	sends []string // "surfaceRef|text"
}

func (m *sendRecorderMock) Send(ws, surf, text string) error {
	m.sends = append(m.sends, surf+"|"+text)
	return nil
}

func TestRestore_SequentialSplitWithoutCWDFallsBackToWorkspaceCWD(t *testing.T) {
	// Old layouts elide a split's cwd when it equals the workspace cwd. On
	// the sequential path (Ghostty, older cmux) a split gets no cd at all in
	// that case and lands wherever the backend spawns it — the workspace cwd
	// must be typed instead (audit 2026-07-11).
	origHeur, origSurf := ShellReadyTimeout, SurfaceReadyTimeout
	ShellReadyTimeout, SurfaceReadyTimeout = 100*time.Millisecond, 100*time.Millisecond
	defer func() { ShellReadyTimeout, SurfaceReadyTimeout = origHeur, origSurf }()

	mc := &sendRecorderMock{}
	r := &Restorer{Client: mc}
	ws := model.Workspace{
		Title: "resave",
		CWD:   "/home/u/downloads",
		Panes: []model.Pane{
			{Type: "terminal", Focus: true, FocusTarget: -1},
			{Type: "terminal", Split: "right", FocusTarget: 0}, // cwd elided by an older save
		},
	}
	result := &RestoreResult{}
	if _, err := r.restoreWorkspace(ws, false, result); err != nil {
		t.Fatalf("restoreWorkspace: %v", err)
	}
	var splitCd bool
	for _, s := range mc.sends {
		if strings.HasPrefix(s, "surface:mock|") && strings.Contains(s, "cd '/home/u/downloads'") {
			splitCd = true
		}
		if strings.HasPrefix(s, "|") {
			t.Errorf("pane 0 must not receive a cd (it inherits the workspace cwd), got send: %q", s)
		}
	}
	if !splitCd {
		t.Errorf("split pane never received cd to the workspace cwd; sends: %v", mc.sends)
	}
}

// splitTargetRecorderMock records every NewSplit target ref and FocusPane
// call, returning unique refs per split, and resolves the first surface —
// like a backend with stable per-surface addressing (cmux refs, Ghostty tids).
type splitTargetRecorderMock struct {
	mockClient
	splitTargets []string
	focusCalls   []string
	seq          int
}

func (m *splitTargetRecorderMock) FirstSurfaceRef(workspaceRef string) string {
	return "surface:first"
}

func (m *splitTargetRecorderMock) NewSplit(dir, ref, surfRef string) (string, error) {
	m.splitTargets = append(m.splitTargets, surfRef)
	m.seq++
	return fmt.Sprintf("surface:split%d", m.seq), nil
}

func (m *splitTargetRecorderMock) FocusPane(pane, ws string) error {
	m.focusCalls = append(m.focusCalls, pane)
	return nil
}

func TestRestore_SplitsAddressExplicitTargetRefs(t *testing.T) {
	// Splitting "the focused pane" located via live indexes drifts on
	// Ghostty (terminals re-index on insertion) — the user's quad came back
	// with panes in the wrong corner. Each split must target the resolved
	// pane's own ref; no focus dance needed.
	origHeur, origSurf := ShellReadyTimeout, SurfaceReadyTimeout
	ShellReadyTimeout, SurfaceReadyTimeout = 100*time.Millisecond, 100*time.Millisecond
	defer func() { ShellReadyTimeout, SurfaceReadyTimeout = origHeur, origSurf }()

	mc := &splitTargetRecorderMock{}
	r := &Restorer{Client: mc}
	// The user's quad: p1 right of visual 0, p2 down from visual 0, p3 down
	// from visual 2 (= p1 at that point).
	ws := model.Workspace{
		Title: "quad",
		CWD:   "/home/u",
		Panes: []model.Pane{
			{Type: "terminal", Focus: true, FocusTarget: -1},
			{Type: "terminal", Split: "right", FocusTarget: 0},
			{Type: "terminal", Split: "down", FocusTarget: 0},
			{Type: "terminal", Split: "down", FocusTarget: 2},
		},
	}
	result := &RestoreResult{}
	if _, err := r.restoreWorkspace(ws, false, result); err != nil {
		t.Fatalf("restoreWorkspace: %v", err)
	}
	want := []string{"surface:first", "surface:first", "surface:split1"}
	if len(mc.splitTargets) != len(want) {
		t.Fatalf("split targets = %v, want %v", mc.splitTargets, want)
	}
	for i := range want {
		if mc.splitTargets[i] != want[i] {
			t.Fatalf("split targets = %v, want %v", mc.splitTargets, want)
		}
	}
	if len(mc.focusCalls) != 0 {
		t.Errorf("focus dance still used despite explicit targets: %v", mc.focusCalls)
	}
}

// profileEnsureMock records NewPane opts and ensured browser profiles.
type profileEnsureMock struct {
	mockClient
	newPaneOpts []client.NewPaneOpts
	ensured     []string
}

func (m *profileEnsureMock) NewPane(opts client.NewPaneOpts) (string, error) {
	m.newPaneOpts = append(m.newPaneOpts, opts)
	return "surface:new", nil
}

func (m *profileEnsureMock) EnsureBrowserProfile(slug string) error {
	m.ensured = append(m.ensured, slug)
	return nil
}

func TestRestore_BrowserProfilePassedAndEnsured(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "profile-restore",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{
				Title: "dev",
				CWD:   "/tmp/project",
				Index: 0,
				Panes: []model.Pane{
					{Type: "terminal", Focus: true},
					{
						Type: "browser", Split: "right", URL: "http://localhost:3000",
						Profile: "work-admin",
						Surfaces: []model.Surface{
							{Type: "browser", URL: "http://localhost:3000/u", Profile: "work-user"},
						},
					},
					// Second pane on the same profile: ensure must dedupe.
					{Type: "browser", Split: "down", URL: "http://localhost:3000/b", Profile: "work-admin"},
				},
			},
		},
	}
	_ = store.Save("profile-restore", layout)

	mc := &profileEnsureMock{mockClient: mockClient{
		treeResp:    &client.TreeResponse{},
		sidebarCWDs: map[string]string{},
	}}
	restorer := &Restorer{Client: mc, Store: store}

	if _, err := restorer.Restore("profile-restore", false, RestoreModeAdd, "", true); err != nil {
		t.Fatalf("restore: %v", err)
	}

	// Distinct profiles ensured, sorted, before pane creation.
	if len(mc.ensured) != 2 {
		t.Fatalf("ensured = %v, want exactly [work-admin work-user]", mc.ensured)
	}
	got := map[string]bool{mc.ensured[0]: true, mc.ensured[1]: true}
	if !got["work-admin"] || !got["work-user"] {
		t.Errorf("ensured = %v, want work-admin + work-user", mc.ensured)
	}

	// Browser panes created with their profile.
	if len(mc.newPaneOpts) != 2 {
		t.Fatalf("NewPane calls = %d, want 2", len(mc.newPaneOpts))
	}
	for _, opts := range mc.newPaneOpts {
		if opts.Profile != "work-admin" {
			t.Errorf("NewPane profile = %q, want work-admin (opts: %+v)", opts.Profile, opts)
		}
	}
}

func TestTemplateUse_BrowserProfilePassed(t *testing.T) {
	mc := &profileEnsureMock{mockClient: mockClient{
		treeResp:    &client.TreeResponse{},
		sidebarCWDs: map[string]string{},
	}}
	tu := &TemplateUser{Client: mc}

	panes := []model.Pane{
		{Type: "terminal", Focus: true},
		{Type: "browser", Split: "right", Command: "http://localhost:3000", Profile: "work-admin"},
	}
	if _, err := tu.Use(panes, TemplateUseOpts{Title: "t", CWD: "/tmp"}, false); err != nil {
		t.Fatalf("template use: %v", err)
	}

	if len(mc.newPaneOpts) != 1 {
		t.Fatalf("NewPane calls = %d, want 1", len(mc.newPaneOpts))
	}
	if got := mc.newPaneOpts[0].Profile; got != "work-admin" {
		t.Errorf("NewPane profile = %q, want work-admin (dry-run advertises it, execute must match)", got)
	}
	if len(mc.ensured) != 1 || mc.ensured[0] != "work-admin" {
		t.Errorf("ensured = %v, want [work-admin]", mc.ensured)
	}
}

func TestRestore_DryRunShowsProfile(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	layout := &model.Layout{
		Name: "dry-profile", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{{
			Title: "dev", CWD: "/tmp", Index: 0,
			Panes: []model.Pane{
				{Type: "terminal", Focus: true},
				{Type: "browser", Split: "right", URL: "http://localhost:3000", Profile: "work-admin"},
			},
		}},
	}
	_ = store.Save("dry-profile", layout)

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}
	result, err := restorer.Restore("dry-profile", true, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	found := false
	for _, cmd := range result.Commands {
		if strings.Contains(cmd, "--profile work-admin") {
			found = true
		}
	}
	if !found {
		t.Errorf("dry-run commands missing --profile work-admin:\n%s", strings.Join(result.Commands, "\n"))
	}
}

// ensureFailMock fails every profile ensure; restore must degrade, not fail.
type ensureFailMock struct {
	profileEnsureMock
}

func (m *ensureFailMock) EnsureBrowserProfile(slug string) error {
	return fmt.Errorf("browser.profiles.list unsupported on this cmux")
}

func TestRestore_EnsureProfileFailureIsNonFatal(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	layout := &model.Layout{
		Name: "ensure-fail", Version: 1, SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{{
			Title: "dev", CWD: "/tmp", Index: 0,
			Panes: []model.Pane{
				{Type: "terminal", Focus: true},
				{Type: "browser", Split: "right", URL: "http://localhost:3000", Profile: "work-admin"},
			},
		}},
	}
	_ = store.Save("ensure-fail", layout)

	mc := &ensureFailMock{profileEnsureMock{mockClient: mockClient{
		treeResp:    &client.TreeResponse{},
		sidebarCWDs: map[string]string{},
	}}}
	restorer := &Restorer{Client: mc, Store: store}
	result, err := restorer.Restore("ensure-fail", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore must not fail when profile ensure fails: %v", err)
	}
	if len(mc.newPaneOpts) != 1 {
		t.Fatalf("browser pane not created after ensure failure (NewPane calls = %d)", len(mc.newPaneOpts))
	}
	warned := false
	for _, e := range result.Errors {
		if strings.Contains(e, "work-admin") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("ensure failure not surfaced as warning; errors = %v", result.Errors)
	}
}

// focusRecorderMock records the ORDER of surface focuses and sends. cmux only
// spawns a tab's shell when the tab is rendered, and `send` to an unspawned
// tab is silently discarded — so a tab's command must be preceded by a focus
// of that tab (GitHub #8).
type focusRecorderMock struct {
	mockClient
	calls      []string // "focus:<ref>" / "send:<ref>:<text>"
	surfaceSeq int
}

func (m *focusRecorderMock) FocusSurface(_, surfaceRef string) error {
	m.calls = append(m.calls, "focus:"+surfaceRef)
	return nil
}

func (m *focusRecorderMock) Send(_, surfaceRef, text string) error {
	m.calls = append(m.calls, "send:"+surfaceRef+":"+strings.TrimSpace(text))
	return nil
}

func (m *focusRecorderMock) NewSurface(paneRef, workspaceRef string) (string, error) {
	m.surfaceSeq++
	return fmt.Sprintf("surface:tab%d", m.surfaceSeq), nil
}

// focusBeforeSend reports whether every send to surfaceRef was preceded by a
// focus of that same surface.
func focusBeforeSend(calls []string, surfaceRef string) bool {
	focused := false
	for _, c := range calls {
		switch {
		case c == "focus:"+surfaceRef:
			focused = true
		case strings.HasPrefix(c, "send:"+surfaceRef+":"):
			if !focused {
				return false
			}
		}
	}
	return true
}

func TestRestoreSurfaces_FocusesTabBeforeSendingItsCommand(t *testing.T) {
	mc := &focusRecorderMock{mockClient: mockClient{
		treeResp:    &client.TreeResponse{},
		sidebarCWDs: map[string]string{},
	}}
	r := &Restorer{Client: mc}
	result := &RestoreResult{}

	pane := model.Pane{
		Type: "terminal",
		Surfaces: []model.Surface{
			{Type: "terminal", CWD: "/w/a", Command: "claude --resume aaa"},
			{Type: "terminal", CWD: "/w/b", Command: "claude --resume bbb"},
		},
	}
	r.restoreSurfaces(pane, "pane:0", "workspace:1", result, 0, "surface:0")

	for _, ref := range []string{"surface:tab1", "surface:tab2"} {
		if !focusBeforeSend(mc.calls, ref) {
			t.Errorf("command sent to %s without focusing it first — cmux discards input to an unspawned tab.\ncalls: %v", ref, mc.calls)
		}
	}
	sends := 0
	for _, c := range mc.calls {
		if strings.HasPrefix(c, "send:") {
			sends++
		}
	}
	if sends != 2 {
		t.Errorf("got %d sends, want 2 (one per tab): %v", sends, mc.calls)
	}
}

// layoutFocusMock drives the ATOMIC restore path (cmux `--layout`) while
// recording focuses and sends.
type layoutFocusMock struct {
	focusRecorderMock
	tree *client.TreeResponse
}

func (m *layoutFocusMock) NewWorkspaceLayout(_ client.NewWorkspaceOpts, _ string) (string, error) {
	return "workspace:new", nil
}

func (m *layoutFocusMock) Tree() (*client.TreeResponse, error) { return m.tree, nil }

func TestTypeCommands_FocusesEachTabBeforeItsCommand(t *testing.T) {
	// Live tree of the freshly created workspace: one pane, three tabs.
	tree := &client.TreeResponse{
		Windows: []client.TreeWindow{{
			Ref: "window:1",
			Workspaces: []client.TreeWorkspace{{
				Ref: "workspace:new", Title: "proj",
				Panes: []client.TreePane{{
					Index: 0,
					Surfaces: []client.TreeSurface{
						{Ref: "surface:10", Type: "terminal"},
						{Ref: "surface:11", Type: "terminal"},
						{Ref: "surface:12", Type: "terminal"},
					},
				}},
			}},
		}},
	}
	mc := &layoutFocusMock{
		focusRecorderMock: focusRecorderMock{mockClient: mockClient{sidebarCWDs: map[string]string{}}},
		tree:              tree,
	}
	r := &Restorer{Client: mc}
	result := &RestoreResult{}

	ws := model.Workspace{
		Title: "proj", CWD: "/w",
		Panes: []model.Pane{{
			Type: "terminal", Index: 0, CWD: "/w", Command: "claude --resume main",
			Surfaces: []model.Surface{
				{Type: "terminal", CWD: "/w/a", Command: "claude --resume aaa"},
				{Type: "terminal", CWD: "/w/b", Command: "claude --resume bbb"},
			},
		}},
	}
	r.typeCommands(ws, "workspace:new", []int{0}, result)

	for _, ref := range []string{"surface:11", "surface:12"} {
		if !focusBeforeSend(mc.calls, ref) {
			t.Errorf("tab %s got its command without being focused first.\ncalls: %v", ref, mc.calls)
		}
	}
	// All three commands must be delivered, each to its own surface.
	want := map[string]string{
		"surface:10": "claude --resume main",
		"surface:11": "claude --resume aaa",
		"surface:12": "claude --resume bbb",
	}
	for ref, cmd := range want {
		found := false
		for _, c := range mc.calls {
			if strings.HasPrefix(c, "send:"+ref+":") && strings.Contains(c, cmd) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s never received %q.\ncalls: %v", ref, cmd, mc.calls)
		}
	}
	// The pane must be left showing its FIRST tab, not the last one typed into.
	lastFocus := ""
	for _, c := range mc.calls {
		if strings.HasPrefix(c, "focus:") {
			lastFocus = strings.TrimPrefix(c, "focus:")
		}
	}
	if lastFocus != "surface:10" {
		t.Errorf("pane left on %q, want surface:10 (its first tab)", lastFocus)
	}
}

func TestTypeCommands_RestoresIntendedFocusAfterTypingIntoTabs(t *testing.T) {
	// Focusing a tab also focuses its pane, so typing into pane 1's tabs must
	// not leave pane 1 focused when the layout marks pane 0 as focused.
	tree := &client.TreeResponse{
		Windows: []client.TreeWindow{{
			Ref: "window:1",
			Workspaces: []client.TreeWorkspace{{
				Ref: "workspace:new", Title: "proj",
				Panes: []client.TreePane{
					{Index: 0, Surfaces: []client.TreeSurface{{Ref: "surface:20", Type: "terminal"}}},
					{Index: 1, Surfaces: []client.TreeSurface{
						{Ref: "surface:21", Type: "terminal"},
						{Ref: "surface:22", Type: "terminal"},
					}},
				},
			}},
		}},
	}
	mc := &layoutFocusMock{
		focusRecorderMock: focusRecorderMock{mockClient: mockClient{sidebarCWDs: map[string]string{}}},
		tree:              tree,
	}
	r := &Restorer{Client: mc}
	result := &RestoreResult{}

	ws := model.Workspace{
		Title: "proj", CWD: "/w",
		Panes: []model.Pane{
			{Type: "terminal", Index: 0, CWD: "/w", Focus: true},
			{Type: "terminal", Index: 1, CWD: "/w", Split: "right",
				Surfaces: []model.Surface{{Type: "terminal", CWD: "/w/b", Command: "claude --resume bbb"}}},
		},
	}
	r.typeCommands(ws, "workspace:new", []int{0, 1}, result)

	lastFocus := ""
	for _, c := range mc.calls {
		if strings.HasPrefix(c, "focus:") {
			lastFocus = strings.TrimPrefix(c, "focus:")
		}
	}
	if lastFocus != "surface:20" {
		t.Errorf("workspace left focused on %q, want surface:20 (the pane the layout marks focus = true)", lastFocus)
	}
}

// ghosttyLikeMock models a backend WITHOUT sub-tabs (Ghostty): NewSurface is
// unsupported, but splits work fine.
type ghosttyLikeMock struct {
	mockClient
	splits []string // "<direction>:<target surface>"
	sends  []string // "<surface>:<text>"
	seq    int
}

func (m *ghosttyLikeMock) NewSurface(paneRef, workspaceRef string) (string, error) {
	return "", client.ErrNotSupported
}

func (m *ghosttyLikeMock) NewSplit(dir, wsRef, surfRef string) (string, error) {
	m.seq++
	m.splits = append(m.splits, dir+":"+surfRef)
	return fmt.Sprintf("terminal:%d", m.seq+10), nil
}

func (m *ghosttyLikeMock) Send(wsRef, surfRef, text string) error {
	m.sends = append(m.sends, surfRef+":"+strings.TrimSpace(text))
	return nil
}

// TestRestoreSurfaces_FallsBackToSplitsWhenTabsUnsupported: a pane's extra tabs
// used to be dropped entirely on Ghostty — the shell, its folder and its AI
// session simply vanished from the restored layout. Ghostty has no sub-tabs but
// it does have splits, so the work stays visible instead of being lost.
func TestRestoreSurfaces_FallsBackToSplitsWhenTabsUnsupported(t *testing.T) {
	mc := &ghosttyLikeMock{mockClient: mockClient{
		treeResp:    &client.TreeResponse{},
		sidebarCWDs: map[string]string{},
	}}
	r := &Restorer{Client: mc}
	result := &RestoreResult{}

	pane := model.Pane{
		Type: "terminal",
		Surfaces: []model.Surface{
			{Type: "terminal", CWD: "/w/a", Command: "claude --resume aaa"},
			{Type: "terminal", CWD: "/w/b", Command: "claude --resume bbb"},
		},
	}
	r.restoreSurfaces(pane, "pane:0", "workspace:1", result, 0, "terminal:1")

	if len(mc.splits) != 2 {
		t.Fatalf("splits = %v, want one per lost tab", mc.splits)
	}
	// The first tab splits off the pane itself; the next stacks beside it, so
	// they form a row instead of repeatedly halving the original pane.
	if mc.splits[0] != "right:terminal:1" {
		t.Errorf("first split = %q, want it anchored on the pane (right:terminal:1)", mc.splits[0])
	}
	if mc.splits[1] != "right:terminal:11" {
		t.Errorf("second split = %q, want it anchored on the previous split", mc.splits[1])
	}
	// Every tab keeps its own folder and its own session command.
	for _, want := range []string{"/w/a", "claude --resume aaa", "/w/b", "claude --resume bbb"} {
		found := false
		for _, s := range mc.sends {
			if strings.Contains(s, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no send carried %q — the tab's content was lost.\nsends: %v", want, mc.sends)
		}
	}
}

// splitlessMock supports neither sub-tabs nor splits: the warning path must
// still work and must not panic.
type splitlessMock struct {
	ghosttyLikeMock
}

func (m *splitlessMock) NewSplit(string, string, string) (string, error) {
	return "", client.ErrNotSupported
}

func TestRestoreSurfaces_WarnsWhenNeitherTabsNorSplitsWork(t *testing.T) {
	mc := &splitlessMock{ghosttyLikeMock{mockClient: mockClient{
		treeResp:    &client.TreeResponse{},
		sidebarCWDs: map[string]string{},
	}}}
	var progress []string
	r := &Restorer{Client: mc, OnProgress: func(msg string, _ int, _ error) {
		progress = append(progress, msg)
	}}
	result := &RestoreResult{}

	pane := model.Pane{Type: "terminal", Surfaces: []model.Surface{{Type: "terminal", CWD: "/w/a"}}}
	r.restoreSurfaces(pane, "pane:0", "workspace:1", result, 0, "terminal:1")

	if len(progress) == 0 {
		t.Error("expected a warning when a backend can restore neither tabs nor splits")
	}
}
