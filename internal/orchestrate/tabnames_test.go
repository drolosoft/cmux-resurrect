package orchestrate

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// renamingMock embeds the base mockClient and records RenameSurface calls,
// so it satisfies the optional client.SurfaceRenamer interface.
type renamingMock struct {
	*mockClient
	renames []rename
}

type rename struct{ ws, surf, title string }

func (m *renamingMock) RenameSurface(ws, surf, title string) error {
	m.renames = append(m.renames, rename{ws, surf, title})
	return nil
}

func TestApplyName_RenamesWhenSupported(t *testing.T) {
	rm := &renamingMock{mockClient: &mockClient{}}
	r := &Restorer{Client: rm}

	r.applyName("workspace:1", "surface:3", "QA") // named → recorded
	r.applyName("workspace:1", "surface:4", "")   // empty name → ignored
	r.applyName("workspace:2", "", "Plan")        // first surface (empty ref) → recorded

	if len(rm.renames) != 2 {
		t.Fatalf("got %d renames, want 2: %+v", len(rm.renames), rm.renames)
	}
	if rm.renames[0] != (rename{"workspace:1", "surface:3", "QA"}) {
		t.Errorf("rename[0] = %+v", rm.renames[0])
	}
	if rm.renames[1] != (rename{"workspace:2", "", "Plan"}) {
		t.Errorf("rename[1] = %+v", rm.renames[1])
	}
}

func TestApplyName_NoopWhenUnsupported(t *testing.T) {
	// Plain mockClient does NOT implement SurfaceRenamer — must not panic.
	r := &Restorer{Client: &mockClient{}}
	r.applyName("workspace:1", "surface:1", "Plan") // should silently no-op
}

func TestImportFromMD_AppliesTabNames(t *testing.T) {
	rm := &renamingMock{mockClient: &mockClient{}}

	wf := &model.WorkspaceFile{
		Templates: map[string]*model.Template{
			"named": {
				Name: "named",
				Panes: []model.TemplatePan{
					{Enabled: true, IsMain: true, Type: "terminal", Name: "Plan", Command: "claude", FocusTarget: -1},
					{Enabled: true, Split: "right", Type: "terminal", Name: "Diff", Command: "git diff", FocusTarget: -1},
				},
			},
		},
		Projects: []model.Project{
			{Enabled: true, Icon: "🛠️", Name: "proj", Template: "named", Path: "/tmp/proj"},
		},
	}

	im := &Importer{Client: rm}
	if _, err := im.ImportFromMD(wf, false); err != nil {
		t.Fatalf("import: %v", err)
	}

	got := map[string]bool{}
	for _, r := range rm.renames {
		got[r.title] = true
	}
	for _, want := range []string{"Plan", "Diff"} {
		if !got[want] {
			t.Errorf("expected a RenameSurface for %q; got renames %+v", want, rm.renames)
		}
	}
}
