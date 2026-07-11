package orchestrate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	model2 "github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

var errUnsupported = errors.New("unknown flag: --layout")

// decode a built layout back into a generic map for assertions.
func mustDecode(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("built layout is not valid JSON: %v\n%s", err, s)
	}
	return m
}

func child(t *testing.T, node map[string]any, i int) map[string]any {
	t.Helper()
	kids, ok := node["children"].([]any)
	if !ok || len(kids) <= i {
		t.Fatalf("node has no child %d: %v", i, node)
	}
	return kids[i].(map[string]any)
}

func firstSurface(t *testing.T, node map[string]any) map[string]any {
	t.Helper()
	pane, ok := node["pane"].(map[string]any)
	if !ok {
		t.Fatalf("node is not a pane leaf: %v", node)
	}
	surfs := pane["surfaces"].([]any)
	return surfs[0].(map[string]any)
}

func TestBuildCmuxLayout_SinglePaneNotWorthIt(t *testing.T) {
	ws := model.Workspace{
		Title: "one",
		CWD:   "/tmp",
		Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}},
	}
	if _, _, ok := buildCmuxLayout(ws); ok {
		t.Error("single plain pane should not use a layout (plain --cwd path is simpler)")
	}
}

func TestBuildCmuxLayout_Aside(t *testing.T) {
	// pane0, pane1 split right (cwd A), pane2 split down targeting pane1
	// implicitly (focus -1 = previously created pane).
	ws := model.Workspace{
		Title:  "aside",
		Pinned: true,
		CWD:    "/home/u",
		Panes: []model.Pane{
			{Type: "terminal", Focus: true, FocusTarget: -1},
			{Type: "terminal", Split: "right", CWD: "/home/u/a", FocusTarget: 0},
			{Type: "terminal", Split: "down", CWD: "/home/u/b", FocusTarget: 1},
		},
	}
	s, _, ok := buildCmuxLayout(ws)
	if !ok {
		t.Fatal("expected a layout")
	}
	root := mustDecode(t, s)
	if root["direction"] != "horizontal" {
		t.Errorf("root direction = %v, want horizontal (crex right)", root["direction"])
	}
	// left child: pane0 gets the workspace cwd explicitly (empty leaf cwds
	// would depend on cmux spawn defaults).
	left := child(t, root, 0)
	if s0 := firstSurface(t, left); s0["cwd"] != "/home/u" {
		t.Errorf("pane0 cwd = %v, want explicit workspace cwd /home/u", s0["cwd"])
	}
	// right child: vertical split of pane1 (top) and pane2 (bottom)
	right := child(t, root, 1)
	if right["direction"] != "vertical" {
		t.Fatalf("right child direction = %v, want vertical (crex down)", right["direction"])
	}
	if got := firstSurface(t, child(t, right, 0))["cwd"]; got != "/home/u/a" {
		t.Errorf("top-right cwd = %v, want /home/u/a", got)
	}
	if got := firstSurface(t, child(t, right, 1))["cwd"]; got != "/home/u/b" {
		t.Errorf("bottom-right cwd = %v, want /home/u/b", got)
	}
}

func TestBuildCmuxLayout_QuadWithLiveIndexTargets(t *testing.T) {
	// v1.22.x creation order for a 2x2: [P0, P2(right), P1(down targeting live 0), P3(down targeting live 2)]
	ws := model.Workspace{
		Title: "quad",
		CWD:   "/h",
		Panes: []model.Pane{
			{Type: "terminal", Index: 0, Focus: true, FocusTarget: -1},
			{Type: "terminal", Index: 2, Split: "right", CWD: "/h/tr", FocusTarget: 0},
			{Type: "terminal", Index: 1, Split: "down", CWD: "/h/bl", FocusTarget: 0},
			{Type: "terminal", Index: 3, Split: "down", CWD: "/h/br", FocusTarget: 2},
		},
	}
	s, visual, ok := buildCmuxLayout(ws)
	if !ok {
		t.Fatal("expected a layout")
	}
	// Creation order [P0, TR, BL, BR] → visual (x,y) ranks [0, 2, 1, 3].
	if want := []int{0, 2, 1, 3}; len(visual) != 4 || visual[0] != want[0] || visual[1] != want[1] || visual[2] != want[2] || visual[3] != want[3] {
		t.Errorf("visual mapping = %v, want %v", visual, want)
	}
	root := mustDecode(t, s)
	// Expected tree: horizontal( vertical(P0, P1), vertical(P2, P3) )
	if root["direction"] != "horizontal" {
		t.Fatalf("root = %v, want horizontal", root["direction"])
	}
	lcol, rcol := child(t, root, 0), child(t, root, 1)
	if lcol["direction"] != "vertical" || rcol["direction"] != "vertical" {
		t.Fatalf("columns must both be vertical splits: %v / %v", lcol["direction"], rcol["direction"])
	}
	if got := firstSurface(t, child(t, lcol, 1))["cwd"]; got != "/h/bl" {
		t.Errorf("bottom-left cwd = %v, want /h/bl", got)
	}
	if got := firstSurface(t, child(t, rcol, 0))["cwd"]; got != "/h/tr" {
		t.Errorf("top-right cwd = %v, want /h/tr", got)
	}
	if got := firstSurface(t, child(t, rcol, 1))["cwd"]; got != "/h/br" {
		t.Errorf("bottom-right cwd = %v, want /h/br", got)
	}
}

func TestBuildCmuxLayout_RatioMapsToFirstChildFraction(t *testing.T) {
	// crex SplitRatio = fraction the NEW pane occupies; cmux split = FIRST child's fraction.
	ws := model.Workspace{
		Title: "ratio",
		CWD:   "/h",
		Panes: []model.Pane{
			{Type: "terminal", FocusTarget: -1},
			{Type: "terminal", Split: "right", SplitRatio: 0.3, FocusTarget: 0},
		},
	}
	s, _, ok := buildCmuxLayout(ws)
	if !ok {
		t.Fatal("expected a layout")
	}
	root := mustDecode(t, s)
	if got := root["split"].(float64); got < 0.69 || got > 0.71 {
		t.Errorf("split = %v, want 0.7 (1 - new pane's 0.3)", got)
	}
}

func TestBuildCmuxLayout_ExtrasAndSafety(t *testing.T) {
	home, _ := os.UserHomeDir()
	ws := model.Workspace{
		Title: "extra",
		CWD:   "~",
		Panes: []model.Pane{
			{Type: "terminal", Focus: true, FocusTarget: -1, Name: "main", CWD: "~/Documents",
				Surfaces: []model.Surface{{Type: "terminal", Name: "tab2", CWD: "~/Downloads"}}},
			{Type: "browser", Split: "right", URL: "http://localhost:3000", FocusTarget: 0},
		},
	}
	s, _, ok := buildCmuxLayout(ws)
	if !ok {
		t.Fatal("expected a layout")
	}
	root := mustDecode(t, s)
	left := child(t, root, 0)
	surfs := left["pane"].(map[string]any)["surfaces"].([]any)
	if len(surfs) != 2 {
		t.Fatalf("pane0 surfaces = %d, want 2 (multi-surface tabs)", len(surfs))
	}
	s0, s1 := surfs[0].(map[string]any), surfs[1].(map[string]any)
	if s0["cwd"] != filepath.Join(home, "Documents") {
		t.Errorf("surface0 cwd = %v, want expanded ~/Documents", s0["cwd"])
	}
	if s0["name"] != "main" || s0["focus"] != true {
		t.Errorf("surface0 name/focus = %v/%v", s0["name"], s0["focus"])
	}
	if s1["cwd"] != filepath.Join(home, "Downloads") || s1["name"] != "tab2" {
		t.Errorf("surface1 = %v", s1)
	}
	// Commands must NOT be in the layout — they're typed after creation so
	// the shell persists when the command exits (AI resume semantics).
	if s0["command"] != nil || s1["command"] != nil {
		t.Error("commands must not be embedded in the layout")
	}
	right := firstSurface(t, child(t, root, 1))
	if right["type"] != "browser" || right["url"] != "http://localhost:3000" {
		t.Errorf("browser surface = %v", right)
	}
}

func TestBuildCmuxLayout_DemoLayoutFile(t *testing.T) {
	// The shipped demo must be representable (files workspace: pane + right split).
	ws := model.Workspace{
		Title: "📁 files",
		CWD:   "~/Documents",
		Panes: []model.Pane{
			{Type: "terminal", Focus: true, FocusTarget: -1},
			{Type: "terminal", Split: "right", CWD: "~/Downloads", Index: 1, FocusTarget: 0},
		},
	}
	s, _, ok := buildCmuxLayout(ws)
	if !ok {
		t.Fatal("expected a layout for the demo files workspace")
	}
	root := mustDecode(t, s)
	home, _ := os.UserHomeDir()
	if got := firstSurface(t, child(t, root, 1))["cwd"]; got != filepath.Join(home, "Downloads") {
		t.Errorf("split cwd = %v, want expanded ~/Downloads", got)
	}
}

// layoutMock records whether workspaces were created atomically or sequentially.
type layoutMock struct {
	readinessMockClient
	layoutCalls []string // layout JSONs received
	splitCalls  int
	sends       []string
	renames     []string
	pins        int
	failLayout  bool
}

func (m *layoutMock) NewWorkspace(opts model2.NewWorkspaceOpts) (string, error) {
	return "workspace:9", nil
}
func (m *layoutMock) NewWorkspaceLayout(opts model2.NewWorkspaceOpts, layoutJSON string) (string, error) {
	if m.failLayout {
		return "", errUnsupported
	}
	m.layoutCalls = append(m.layoutCalls, layoutJSON)
	return "workspace:9", nil
}
func (m *layoutMock) NewSplit(dir, ref, surfRef string) (string, error) {
	m.splitCalls++
	return "surface:9", nil
}
func (m *layoutMock) Send(ws, surf, text string) error {
	m.sends = append(m.sends, surf+"|"+text)
	return nil
}

// Tree resolves the atomically created workspace like a real backend would:
// typeCommands refuses to send when it can't resolve a pane's surface (a
// blank target would go to the focused pane), so the mock must expose them.
func (m *layoutMock) Tree() (*model2.TreeResponse, error) {
	return &model2.TreeResponse{Windows: []model2.TreeWindow{{
		Workspaces: []model2.TreeWorkspace{{
			Ref: "workspace:9",
			Panes: []model2.TreePane{
				{Index: 0, Surfaces: []model2.TreeSurface{{Ref: "surface:90", Type: "terminal"}}},
				{Index: 1, Surfaces: []model2.TreeSurface{{Ref: "surface:91", Type: "terminal"}}},
			},
		}},
	}}}, nil
}
func (m *layoutMock) SurfaceState(_, ref string) (*model2.SurfaceState, error) {
	return &model2.SurfaceState{Ref: ref, CWD: "/anything", Ready: true}, nil
}
func (m *layoutMock) RenameWorkspace(ref, title string) error {
	m.renames = append(m.renames, title)
	return nil
}
func (m *layoutMock) PinWorkspace(ref string) error {
	m.pins++
	return nil
}

func TestRestoreWorkspace_UsesAtomicLayout(t *testing.T) {
	ws := model.Workspace{
		Title:  "aside",
		Pinned: true,
		CWD:    "/home/u",
		Panes: []model.Pane{
			{Type: "terminal", Focus: true, FocusTarget: -1},
			{Type: "terminal", Split: "right", CWD: "/home/u/a", Index: 1, FocusTarget: 0},
		},
	}
	m := &layoutMock{}
	r := &Restorer{Client: m}
	result := &RestoreResult{}
	if _, err := r.restoreWorkspace(ws, false, result); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(m.layoutCalls) != 1 {
		t.Fatalf("layout creations = %d, want 1", len(m.layoutCalls))
	}
	if m.splitCalls != 0 {
		t.Errorf("NewSplit called %d times — atomic path must not split", m.splitCalls)
	}
	for _, s := range m.sends {
		if strings.Contains(s, "cd ") {
			t.Errorf("a cd was typed despite atomic layout: %q", s)
		}
	}
	if len(m.renames) != 1 || m.renames[0] != "aside" {
		t.Errorf("renames = %v, want [aside] (deferred rename must run on the atomic path too)", m.renames)
	}
	if m.pins != 1 {
		t.Errorf("pins = %d, want 1 (pinned workspace)", m.pins)
	}
}

func TestRestoreWorkspace_AtomicStillTypesCommands(t *testing.T) {
	ws := model.Workspace{
		Title: "ai",
		CWD:   "/home/u",
		Panes: []model.Pane{
			{Type: "terminal", Focus: true, FocusTarget: -1, Command: "claude --resume abc"},
			{Type: "terminal", Split: "right", Index: 1, FocusTarget: 0},
		},
	}
	m := &layoutMock{}
	r := &Restorer{Client: m}
	result := &RestoreResult{}
	if _, err := r.restoreWorkspace(ws, false, result); err != nil {
		t.Fatalf("restore: %v", err)
	}
	found := false
	for _, s := range m.sends {
		if strings.Contains(s, "claude --resume abc") {
			found = true
			if strings.Contains(s, "cd ") {
				t.Errorf("command send must not carry a cd (cwd is native): %q", s)
			}
			if !strings.HasPrefix(s, "surface:90|") {
				t.Errorf("command must target pane 0's resolved surface, got %q", s)
			}
		}
	}
	if !found {
		t.Error("pane command was not typed after atomic creation")
	}
}

func TestRestoreWorkspace_FallsBackWhenLayoutUnsupported(t *testing.T) {
	ws := model.Workspace{
		Title: "fallback",
		CWD:   "/home/u",
		Panes: []model.Pane{
			{Type: "terminal", Focus: true, FocusTarget: -1},
			{Type: "terminal", Split: "right", CWD: "/home/u/a", Index: 1, FocusTarget: 0},
		},
	}
	m := &layoutMock{failLayout: true}
	r := &Restorer{Client: m}
	result := &RestoreResult{}
	if _, err := r.restoreWorkspace(ws, false, result); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if m.splitCalls != 1 {
		t.Errorf("NewSplit calls = %d, want 1 (sequential fallback)", m.splitCalls)
	}
}

func TestBuildCmuxLayout_EmptyLeafCWDFallsBackToWorkspaceCWD(t *testing.T) {
	// Layouts saved before the audit fix elide a pane's cwd when it equals
	// the workspace cwd. The atomic layout must not emit empty leaf cwds for
	// those panes — every terminal leaf gets an explicit cwd so the result
	// never depends on cmux's spawn defaults.
	ws := model.Workspace{
		Title: "resave",
		CWD:   "/home/u/downloads",
		Panes: []model.Pane{
			{Type: "terminal", Focus: true, FocusTarget: -1, CWD: "/home/u"},
			{Type: "terminal", Split: "right", CWD: "/home/u/docs", FocusTarget: 0},
			{Type: "terminal", Split: "down", FocusTarget: 1}, // cwd elided by an older save
		},
	}
	s, _, ok := buildCmuxLayout(ws)
	if !ok {
		t.Fatal("expected a layout")
	}
	root := mustDecode(t, s)
	if got := firstSurface(t, child(t, root, 0))["cwd"]; got != "/home/u" {
		t.Errorf("pane0 cwd = %v, want explicit /home/u", got)
	}
	right := child(t, root, 1)
	if got := firstSurface(t, child(t, right, 0))["cwd"]; got != "/home/u/docs" {
		t.Errorf("pane1 cwd = %v, want /home/u/docs", got)
	}
	if got := firstSurface(t, child(t, right, 1))["cwd"]; got != "/home/u/downloads" {
		t.Errorf("pane2 cwd = %v, want workspace fallback /home/u/downloads", got)
	}
}
