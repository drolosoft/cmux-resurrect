package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

// nowBackend answers the reads `crex now` makes: the tree and the sidebar
// state. The embedded Backend is nil, so any write panics the test.
type nowBackend struct {
	client.Backend

	window client.TreeWindow
}

// Tree returns a single-window tree with the caller inside it.
func (b *nowBackend) Tree() (*client.TreeResponse, error) {
	return &client.TreeResponse{
		Caller:  &client.CallerInfo{WindowRef: b.window.Ref},
		Windows: []client.TreeWindow{b.window},
	}, nil
}

// SidebarState reports no cwd, which keeps the expected lines short.
func (b *nowBackend) SidebarState(string) (*client.SidebarState, error) {
	return &client.SidebarState{}, nil
}

// groupedNowBackend is a cmux-like backend that also lists groups.
type groupedNowBackend struct {
	nowBackend
	client.GroupManager

	groups []client.WorkspaceGroup
	err    error
}

// ListGroups returns the fixture groups or the configured error.
func (b *groupedNowBackend) ListGroups(string) ([]client.WorkspaceGroup, error) {
	return b.groups, b.err
}

// nowWindowFixture is a live window with a loose workspace before a group of
// two, the group's anchor carrying its own "Group 7" title.
func nowWindowFixture() client.TreeWindow {
	pane := []client.TreePane{{Ref: "pane:1"}}
	return client.TreeWindow{
		Ref: "window:1",
		Workspaces: []client.TreeWorkspace{
			{Ref: "workspace:1", Title: "inbox", Index: 0, Pinned: true, Panes: pane},
			{Ref: "workspace:7", Title: "Group 7", Index: 1, Selected: true, Panes: pane},
			{Ref: "workspace:8", Title: "crex", Index: 2, Panes: pane},
		},
	}
}

// useCmuxUnits pins the unit name to "workspaces" for one test. The backend
// is otherwise detected from the terminal running the tests, and inside
// Ghostty every expected "workspace" would read "tab".
func useCmuxUnits(t *testing.T) {
	t.Helper()
	previous := cachedBackend
	cachedBackend = client.BackendCmux
	t.Cleanup(func() { cachedBackend = previous })
}

// renderNowFixture runs the now renderer against a backend and returns the
// text it printed.
func renderNowFixture(t *testing.T, backend client.Backend) string {
	t.Helper()
	useCmuxUnits(t)
	var out bytes.Buffer
	if err := renderNow(&out, backend); err != nil {
		t.Fatalf("renderNow: %v", err)
	}

	return out.String()
}

// flatNowOutput is what `crex now` printed for the fixture before groups
// existed; every fallback path must keep producing it.
const flatNowOutput = "                    \n  Current Workspaces\n" +
	"  inbox 📌  1 pane(s)\n" +
	"  Group 7 ●  1 pane(s)\n" +
	"  crex  1 pane(s)\n" +
	"\n"

func TestNow_FlatWithoutGroupSupport(t *testing.T) {
	got := renderNowFixture(t, &nowBackend{window: nowWindowFixture()})

	if got != flatNowOutput {
		t.Errorf("now output =\n%q\nwant\n%q", got, flatNowOutput)
	}
}

func TestNow_FlatWhenListGroupsFails(t *testing.T) {
	backend := &groupedNowBackend{nowBackend: nowBackend{window: nowWindowFixture()}, err: errors.New("boom")}

	if got := renderNowFixture(t, backend); got != flatNowOutput {
		t.Errorf("now output =\n%q\nwant\n%q", got, flatNowOutput)
	}
}

func TestNow_GroupedLikeShow(t *testing.T) {
	backend := &groupedNowBackend{
		nowBackend: nowBackend{window: nowWindowFixture()},
		groups: []client.WorkspaceGroup{{
			Ref:        "workspace_group:1",
			Name:       "🎮 Cmux",
			AnchorRef:  "workspace:7",
			MemberRefs: []string{"workspace:7", "workspace:8"},
			Pinned:     true,
			Collapsed:  true,
		}},
	}

	got := renderNowFixture(t, backend)
	want := "                    \n  Current Workspaces\n" +
		"  inbox 📌  1 pane(s)\n" +
		"  🎮 Cmux 📌 ▸ collapsed\n" +
		"    (anchor) ●  1 pane(s)\n" +
		"    crex  1 pane(s)\n" +
		"\n"

	if got != want {
		t.Errorf("now output =\n%q\nwant\n%q", got, want)
	}
}
