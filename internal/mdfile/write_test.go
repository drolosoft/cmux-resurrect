package mdfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/juanatsap/cmux-resurrect/internal/model"
)

func TestWriteRoundTrip(t *testing.T) {
	// Parse the test MD.
	srcPath := writeTempMD(t, testMD)
	wf, err := Parse(srcPath)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Write it back.
	outPath := filepath.Join(t.TempDir(), "out.md")
	if err := Write(outPath, wf); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Parse the written file.
	wf2, err := Parse(outPath)
	if err != nil {
		t.Fatalf("parse written: %v", err)
	}

	if len(wf2.Projects) != len(wf.Projects) {
		t.Errorf("projects: %d vs %d", len(wf2.Projects), len(wf.Projects))
	}
	for i, p := range wf2.Projects {
		orig := wf.Projects[i]
		if p.Name != orig.Name {
			t.Errorf("project[%d] name: %q vs %q", i, p.Name, orig.Name)
		}
		if p.Enabled != orig.Enabled {
			t.Errorf("project[%d] enabled: %v vs %v", i, p.Enabled, orig.Enabled)
		}
		if p.Icon != orig.Icon {
			t.Errorf("project[%d] icon: %q vs %q", i, p.Icon, orig.Icon)
		}
		if p.Template != orig.Template {
			t.Errorf("project[%d] template: %q vs %q", i, p.Template, orig.Template)
		}
	}

	if len(wf2.Templates) != len(wf.Templates) {
		t.Errorf("templates: %d vs %d", len(wf2.Templates), len(wf.Templates))
	}
}

func TestAddProject(t *testing.T) {
	path := writeTempMD(t, testMD)

	p := model.Project{
		Enabled:  true,
		Icon:     "🚀",
		Name:     "NewProject",
		Template: "dev",
		Pin:      true,
		Path:     "~/Git/new-project",
	}
	if err := AddProject(path, p); err != nil {
		t.Fatalf("add: %v", err)
	}

	wf, _ := Parse(path)
	if len(wf.Projects) != 5 {
		t.Fatalf("projects = %d, want 5", len(wf.Projects))
	}
	last := wf.Projects[4]
	if last.Name != "NewProject" {
		t.Errorf("last name = %q", last.Name)
	}
}

func TestAddProject_Duplicate(t *testing.T) {
	path := writeTempMD(t, testMD)
	p := model.Project{Name: "LaPorrA", Icon: "🏟️", Template: "dev", Path: "/tmp"}
	err := AddProject(path, p)
	if err == nil {
		t.Error("expected duplicate error")
	}
}

func TestRemoveProject(t *testing.T) {
	path := writeTempMD(t, testMD)

	if err := RemoveProject(path, "Gallery"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	wf, _ := Parse(path)
	if len(wf.Projects) != 3 {
		t.Fatalf("projects = %d, want 3", len(wf.Projects))
	}
	for _, p := range wf.Projects {
		if p.Name == "Gallery" {
			t.Error("Gallery should have been removed")
		}
	}
}

func TestRemoveProject_NotFound(t *testing.T) {
	path := writeTempMD(t, testMD)
	err := RemoveProject(path, "NoSuchProject")
	if err == nil {
		t.Error("expected not found error")
	}
}

func TestToggleProject(t *testing.T) {
	path := writeTempMD(t, testMD)

	// LaPorrA is enabled, toggle to disabled.
	newState, err := ToggleProject(path, "LaPorrA")
	if err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if newState {
		t.Error("should be disabled after toggle")
	}

	// Toggle back to enabled.
	newState, err = ToggleProject(path, "LaPorrA")
	if err != nil {
		t.Fatalf("toggle back: %v", err)
	}
	if !newState {
		t.Error("should be enabled after second toggle")
	}
}

func TestWrite_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.md")

	wf := &model.WorkspaceFile{
		Projects: []model.Project{
			{Enabled: true, Icon: "🔥", Name: "test", Template: "single", Pin: true, Path: "/tmp"},
		},
		Templates: defaultTemplates(),
	}

	if err := Write(path, wf); err != nil {
		t.Fatalf("write: %v", err)
	}

	content, _ := os.ReadFile(path)
	s := string(content)
	if !strings.Contains(s, "## Projects") {
		t.Error("missing Projects header")
	}
	if !strings.Contains(s, "🔥") {
		t.Error("missing icon")
	}
	if !strings.Contains(s, "## Templates") {
		t.Error("missing Templates header")
	}
}
