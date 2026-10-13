package persist

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
	toml "github.com/pelletier/go-toml/v2"
)

// legacyLayout is a frozen copy of the layout format as it was before
// workspace groups existed. It stands in for an older crex binary so the
// tests can prove both directions of compatibility without checking out an
// old release: what the old struct writes, and what it accepts when it reads
// a file written by a newer crex.
type legacyLayout struct {
	Name        string            `toml:"name"`
	Description string            `toml:"description,omitempty"`
	Version     int               `toml:"version"`
	Revision    uint64            `toml:"revision"`
	SavedAt     time.Time         `toml:"saved_at"`
	Workspaces  []legacyWorkspace `toml:"workspace"`
}

// legacyWorkspace is the pre-groups Workspace, field for field.
type legacyWorkspace struct {
	Title       string       `toml:"title"`
	Description string       `toml:"description,omitempty"`
	CWD         string       `toml:"cwd"`
	Pinned      bool         `toml:"pinned"`
	Index       int          `toml:"index"`
	Active      bool         `toml:"active,omitempty"`
	Panes       []legacyPane `toml:"pane"`
}

// legacyPane is the pre-groups Pane, field for field.
type legacyPane struct {
	Type        string          `toml:"type"`
	Split       string          `toml:"split,omitempty"`
	CWD         string          `toml:"cwd,omitempty"`
	Name        string          `toml:"name,omitempty"`
	Command     string          `toml:"command,omitempty"`
	Focus       bool            `toml:"focus,omitempty"`
	URL         string          `toml:"url,omitempty"`
	Profile     string          `toml:"profile,omitempty"`
	Index       int             `toml:"index,omitempty"`
	FocusTarget int             `toml:"focus_target,omitempty"`
	SplitRatio  float64         `toml:"split_ratio,omitempty"`
	Surfaces    []legacySurface `toml:"surface,omitempty"`
}

// legacySurface is the pre-groups Surface, field for field.
type legacySurface struct {
	Type    string `toml:"type,omitempty"`
	Name    string `toml:"name,omitempty"`
	Command string `toml:"command,omitempty"`
	CWD     string `toml:"cwd,omitempty"`
	URL     string `toml:"url,omitempty"`
	Profile string `toml:"profile,omitempty"`
	Focus   bool   `toml:"focus,omitempty"`
}

// groupKeyPattern matches any TOML key or table header that belongs to the
// groups feature: a [[group]] table, a group = ... key or a group_anchor key.
var groupKeyPattern = regexp.MustCompile(`(?m)^\s*(\[\[group\]\]|group\s*=|group_anchor\s*=)`)

// groupedLayout builds a layout with two sidebar groups (one plain, one
// collapsed, pinned, coloured and with an icon), their anchors and members,
// and two loose workspaces around them, so every new field carries a value.
func groupedLayout() *model.Layout {
	return &model.Layout{
		Name:    "grouped",
		Version: 1,
		SavedAt: time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC),
		Groups: []model.Group{
			{Name: "🎮 Cmux"},
			{
				Name:      "🔥 Hot fixes",
				Collapsed: true,
				Pinned:    true,
				Color:     "#FF8800",
				Icon:      "flame",
			},
		},
		Workspaces: []model.Workspace{
			{
				Title: "📓 notes",
				CWD:   "/tmp/notes",
				Index: 0,
				Panes: []model.Pane{{Type: "terminal", Focus: true}},
			},
			{
				Title:       "Group 7",
				CWD:         "/tmp/cmux",
				Index:       1,
				Group:       "🎮 Cmux",
				GroupAnchor: true,
				Panes:       []model.Pane{{Type: "terminal", Focus: true}},
			},
			{
				Title: "cmux tests",
				CWD:   "/tmp/cmux",
				Index: 2,
				Group: "🎮 Cmux",
				Panes: []model.Pane{
					{Type: "terminal", Focus: true},
					{Type: "terminal", Split: "right", Command: "go test ./..."},
				},
			},
			{
				Title:       "Group 8",
				CWD:         "/tmp/fixes",
				Index:       3,
				Active:      true,
				Group:       "🔥 Hot fixes",
				GroupAnchor: true,
				Panes:       []model.Pane{{Type: "terminal", Focus: true}},
			},
			{
				Title: "⚙️ api",
				CWD:   "/tmp/api",
				Index: 4,
				Panes: []model.Pane{{Type: "browser", URL: "http://localhost:3000"}},
			},
		},
	}
}

func TestFileStore_GroupsRoundTrip(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	original := groupedLayout()
	if err := store.Save("grouped", original); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Load("grouped")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if !reflect.DeepEqual(loaded, original) {
		data, _ := os.ReadFile(store.Path("grouped"))
		t.Fatalf("round trip changed the layout\nsaved:  %+v\nloaded: %+v\nfile:\n%s", original, loaded, data)
	}
}

func TestFileStore_LoadCurrentFormatWithoutGroups(t *testing.T) {
	store := &FileStore{Dir: "../../testdata/layouts"}

	layout, err := store.Load("my-day")
	if err != nil {
		t.Fatalf("load my-day fixture: %v", err)
	}

	// This is the fixture spelled out by hand: a layout written before groups
	// existed must decode to exactly this, with no group data invented.
	want := &model.Layout{
		Name:        "my-day",
		Description: "Example layout — try: crex restore my-day",
		Version:     1,
		SavedAt:     time.Date(2026, 3, 26, 0, 0, 0, 0, time.UTC),
		Workspaces: []model.Workspace{
			{
				Title:  "🌐 webapp",
				CWD:    "~/projects/webapp",
				Pinned: true,
				Index:  0,
				Active: true,
				Panes: []model.Pane{
					{Type: "terminal", Focus: true},
					{Type: "terminal", Split: "right", Command: "npm run dev"},
				},
			},
			{
				Title:  "⚙️ api",
				CWD:    "~/projects/api",
				Pinned: true,
				Index:  1,
				Panes: []model.Pane{
					{Type: "terminal", Focus: true},
					{Type: "terminal", Split: "right", Command: "go test ./..."},
				},
			},
			{
				Title: "📓 docs",
				CWD:   "~/docs",
				Index: 2,
				Panes: []model.Pane{
					{Type: "terminal", Focus: true},
				},
			},
		},
	}

	if layout.Groups != nil {
		t.Errorf("Groups = %+v, want nil", layout.Groups)
	}
	for _, workspace := range layout.Workspaces {
		if workspace.Group != "" || workspace.GroupAnchor {
			t.Errorf("workspace %q has Group = %q, GroupAnchor = %v; want none", workspace.Title, workspace.Group, workspace.GroupAnchor)
		}
	}
	if !reflect.DeepEqual(layout, want) {
		t.Errorf("fixture decoded differently\ngot:  %+v\nwant: %+v", layout, want)
	}
}

func TestFileStore_SaveWithoutGroupsWritesNoGroupKey(t *testing.T) {
	fixtures := &FileStore{Dir: "../../testdata/layouts"}
	layout, err := fixtures.Load("my-day")
	if err != nil {
		t.Fatalf("load my-day fixture: %v", err)
	}

	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.Save("my-day", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	saved, err := os.ReadFile(filepath.Join(store.Dir, "my-day.toml"))
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if groupKeyPattern.Match(saved) {
		t.Errorf("a layout without groups was saved with a group key:\n%s", saved)
	}

	// Byte for byte, the new model must serialise an ungrouped layout exactly
	// as the old one did, so re-saving an old layout never changes its shape.
	raw, err := os.ReadFile(fixtures.Path("my-day"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var legacy legacyLayout
	if err := toml.Unmarshal(raw, &legacy); err != nil {
		t.Fatalf("decode fixture into legacy struct: %v", err)
	}
	legacyBytes, err := toml.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy: %v", err)
	}
	currentBytes, err := toml.Marshal(layout)
	if err != nil {
		t.Fatalf("marshal current: %v", err)
	}
	if !bytes.Equal(currentBytes, legacyBytes) {
		t.Errorf("serialisation drifted from the pre-groups format\nlegacy:\n%s\ncurrent:\n%s", legacyBytes, currentBytes)
	}
}

func TestFileStore_OldCrexReadsGroupedLayout(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.Save("grouped", groupedLayout()); err != nil {
		t.Fatalf("save: %v", err)
	}

	raw, err := os.ReadFile(store.Path("grouped"))
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}

	// The file really carries the new keys; otherwise this test proves nothing.
	if !groupKeyPattern.Match(raw) {
		t.Fatalf("grouped layout was saved without group keys:\n%s", raw)
	}

	// A strict decoder must reject the file, which proves the new keys are
	// genuinely unknown to the legacy struct and the lenient pass below is
	// skipping them rather than matching them by accident.
	strict := toml.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	var strictLegacy legacyLayout
	if err := strict.Decode(&strictLegacy); err == nil {
		t.Fatal("strict decoding into the legacy struct succeeded; the test lost its meaning")
	}

	// An old crex decodes with plain toml.Unmarshal, the same call Load makes.
	// go-toml v2 only rejects unknown keys when a Decoder is told to with
	// DisallowUnknownFields, so the new keys must be skipped silently.
	var legacy legacyLayout
	if err := toml.Unmarshal(raw, &legacy); err != nil {
		t.Fatalf("old crex failed to read a grouped layout: %v", err)
	}

	if legacy.Name != "grouped" {
		t.Errorf("Name = %q, want grouped", legacy.Name)
	}
	if len(legacy.Workspaces) != 5 {
		t.Fatalf("Workspaces = %d, want 5", len(legacy.Workspaces))
	}
	if legacy.Workspaces[2].Title != "cmux tests" || len(legacy.Workspaces[2].Panes) != 2 {
		t.Errorf("member workspace decoded as %+v", legacy.Workspaces[2])
	}
}
