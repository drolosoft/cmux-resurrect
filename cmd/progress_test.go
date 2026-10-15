package cmd

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
)

// defaultGroupTitle matches the "Group N" title cmux gives a group's anchor,
// which save and restore must never print.
var defaultGroupTitle = regexp.MustCompile(`Group \d+`)

// progressGroupedLayout has two groups, one anchored by a cmux "Group 2"
// placeholder and one by a workspace the user renamed, plus a loose
// workspace at the end of the sidebar.
func progressGroupedLayout() *model.Layout {
	return &model.Layout{
		Name: "grouped",
		Groups: []model.Group{
			{Name: "🎮 Cmux", Pinned: true, Collapsed: true, Color: "#FF8800", Icon: "flame"},
			{Name: "Docs"},
		},
		Workspaces: []model.Workspace{
			{Title: "crex", Index: 1, Group: "🎮 Cmux", Panes: []model.Pane{{Type: "terminal"}, {Type: "browser"}}},
			{Title: "Group 2", Index: 0, Group: "🎮 Cmux", GroupAnchor: true, Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "manual", Index: 2, Group: "Docs", GroupAnchor: true, Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "notes", Index: 3, Pinned: true, Panes: []model.Pane{{Type: "terminal"}}},
		},
	}
}

// assertNoDefaultGroupTitle fails when any line shows a "Group N" title.
func assertNoDefaultGroupTitle(t *testing.T, output string) {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if defaultGroupTitle.MatchString(line) {
			t.Errorf("line %q shows a cmux placeholder title", line)
		}
	}
}

func TestSaveSummary_GroupedLayout(t *testing.T) {
	useCmuxUnits(t)
	var out bytes.Buffer

	renderSaveSummary(&out, progressGroupedLayout(), "/layouts/grouped.toml")

	want := "\n" +
		"  🎮 Cmux 📌 ▸ collapsed ■ #FF8800 icon flame\n" +
		"    OK  (anchor)\n" +
		"    OK  crex (2 panes)\n" +
		"  Docs\n" +
		"    OK  manual anchor\n" +
		"  OK  notes 📌\n" +
		"\n" +
		"✅ Saved 4 workspaces to /layouts/grouped.toml (1 browser)\n" +
		"\n"
	if out.String() != want {
		t.Errorf("save summary:\n%s\nwant\n%s", out.String(), want)
	}
	assertNoDefaultGroupTitle(t, out.String())
}

// TestAnchorNamedAfterGroup_PrintsOnce: save and restore now name a
// placeholder anchor after its group, and printing that name under a header
// carrying the same name would only repeat it, so it reads "(anchor)".
func TestAnchorNamedAfterGroup_PrintsOnce(t *testing.T) {
	useCmuxUnits(t)
	layout := &model.Layout{
		Name:   "named",
		Groups: []model.Group{{Name: "Backend"}},
		Workspaces: []model.Workspace{
			{Title: "Backend", Index: 0, Group: "Backend", GroupAnchor: true, Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "api", Index: 1, Group: "Backend", Panes: []model.Pane{{Type: "terminal"}}},
		},
	}

	var saved bytes.Buffer
	renderSaveSummary(&saved, layout, "/layouts/named.toml")

	wantSaved := "\n" +
		"  Backend\n" +
		"    OK  (anchor)\n" +
		"    OK  api\n" +
		"\n" +
		"✅ Saved 2 workspaces to /layouts/named.toml\n" +
		"\n"
	if saved.String() != wantSaved {
		t.Errorf("save summary:\n%s\nwant\n%s", saved.String(), wantSaved)
	}

	var restored bytes.Buffer
	progress := newRestoreProgress(&restored)
	progress.workspace(progressEvent(layout, "Backend", nil, false))
	progress.workspace(progressEvent(layout, "api", nil, false))

	wantRestored := "" +
		"  Backend\n" +
		"    OK  (anchor) (1 panes)\n" +
		"    OK  api (1 panes)\n"
	if restored.String() != wantRestored {
		t.Errorf("restore progress:\n%s\nwant\n%s", restored.String(), wantRestored)
	}
}

// TestSaveSummary_NoGroupsIsUnchanged pins the flat list a layout without
// groups printed before groups existed, in slice order.
func TestSaveSummary_NoGroupsIsUnchanged(t *testing.T) {
	useCmuxUnits(t)
	var out bytes.Buffer
	layout := &model.Layout{
		Name: "plain",
		Workspaces: []model.Workspace{
			{Title: "api", Index: 1, Pinned: true, Panes: []model.Pane{{Type: "terminal"}, {Type: "terminal"}}},
			{Title: "notes", Index: 0, Panes: []model.Pane{{Type: "terminal"}}},
		},
	}

	renderSaveSummary(&out, layout, "/layouts/plain.toml")

	want := "\n" +
		"  OK  api (2 panes) 📌\n" +
		"  OK  notes\n" +
		"\n" +
		"✅ Saved 2 workspaces to /layouts/plain.toml\n" +
		"\n"
	if out.String() != want {
		t.Errorf("save summary:\n%q\nwant\n%q", out.String(), want)
	}
}

// progressEvent builds the event Restore reports for one workspace of the
// grouped layout.
func progressEvent(layout *model.Layout, title string, err error, skipped bool) orchestrate.WorkspaceProgress {
	for _, workspace := range layout.Workspaces {
		if workspace.Title == title {
			return orchestrate.WorkspaceProgress{
				Workspace: workspace,
				Group:     layout.GroupNamed(workspace.Group),
				Err:       err,
				Skipped:   skipped,
			}
		}
	}

	return orchestrate.WorkspaceProgress{}
}

func TestRestoreProgress_GroupedLayout(t *testing.T) {
	layout := progressGroupedLayout()
	var out bytes.Buffer
	progress := newRestoreProgress(&out)

	// Restore walks the sidebar in Index order and reports each workspace
	// after handling it; the notice comes from inside a workspace.
	progress.workspace(progressEvent(layout, "Group 2", nil, false))
	progress.notice("⚡ auto-accept: claude --dangerously-skip-permissions", 0, nil)
	progress.workspace(progressEvent(layout, "crex", errors.New("already open, skipped"), true))
	progress.workspace(progressEvent(layout, "manual", errors.New("new-workspace: boom"), false))
	progress.workspace(progressEvent(layout, "notes", nil, false))

	want := "" +
		"  🎮 Cmux 📌 ▸ collapsed ■ #FF8800 icon flame\n" +
		"    OK  (anchor) (1 panes)\n" +
		"    INFO  ⚡ auto-accept: claude --dangerously-skip-permissions\n" +
		"    SKIP  crex (already open, skipped)\n" +
		"  Docs\n" +
		"    FAIL  manual anchor: new-workspace: boom\n" +
		"  OK  notes (1 panes)\n"
	if out.String() != want {
		t.Errorf("restore progress:\n%s\nwant\n%s", out.String(), want)
	}
	assertNoDefaultGroupTitle(t, out.String())
}

// TestRestoreProgress_LooseAnchor covers a restore that could not recreate
// groups: the anchor arrives loose and still must not read "Group 2".
func TestRestoreProgress_LooseAnchor(t *testing.T) {
	var out bytes.Buffer
	progress := newRestoreProgress(&out)

	progress.workspace(orchestrate.WorkspaceProgress{
		Workspace: model.Workspace{Title: "Group 2", GroupAnchor: true, Panes: []model.Pane{{Type: "terminal"}}},
	})

	want := "  OK  (group anchor) (1 panes)\n"
	if out.String() != want {
		t.Errorf("restore progress = %q, want %q", out.String(), want)
	}
}

// TestRestoreProgress_NoGroupsIsUnchanged pins the flat lines a layout
// without groups printed before groups existed.
func TestRestoreProgress_NoGroupsIsUnchanged(t *testing.T) {
	var out bytes.Buffer
	progress := newRestoreProgress(&out)

	progress.workspace(orchestrate.WorkspaceProgress{Workspace: model.Workspace{Title: "api", Panes: []model.Pane{{}, {}}}})
	progress.workspace(orchestrate.WorkspaceProgress{
		Workspace: model.Workspace{Title: "notes", Panes: []model.Pane{{}}},
		Err:       errors.New("already open, skipped"),
		Skipped:   true,
	})

	want := "  OK  api (2 panes)\n  SKIP  notes (already open, skipped)\n"
	if out.String() != want {
		t.Errorf("restore progress = %q, want %q", out.String(), want)
	}
}
