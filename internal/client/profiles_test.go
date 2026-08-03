package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const profilesListJSON = `{
  "current_profile_id" : "52B43C05-4A1D-45D3-8FD5-9EF94952E445",
  "profiles" : [
    {
      "built_in_default" : true,
      "current" : true,
      "id" : "52B43C05-4A1D-45D3-8FD5-9EF94952E445",
      "name" : "Default",
      "slug" : "default"
    },
    {
      "built_in_default" : false,
      "current" : false,
      "id" : "29A2424D-B926-4A45-9CE9-1DF3CF3E23D5",
      "name" : "Work Admin",
      "slug" : "work-admin"
    }
  ]
}`

func TestParseBrowserProfiles(t *testing.T) {
	profiles, err := parseBrowserProfiles(profilesListJSON)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles, want 2", len(profiles))
	}
	if !profiles[0].Default || profiles[0].Slug != "default" {
		t.Errorf("profile 0 = %+v, want built-in default with slug 'default'", profiles[0])
	}
	if profiles[1].Default || profiles[1].Slug != "work-admin" || profiles[1].ID != "29A2424D-B926-4A45-9CE9-1DF3CF3E23D5" {
		t.Errorf("profile 1 = %+v, want non-default work-admin", profiles[1])
	}
	if profiles[1].Name != "Work Admin" {
		t.Errorf("profile 1 name = %q, want %q", profiles[1].Name, "Work Admin")
	}
}

// sessionJSON mirrors the real structure of cmux's
// session-com.cmuxterm.app.json: browser panels carry their surface UUID in
// "id" and the assigned profile in "browser.profileID".
const sessionJSON = `{
  "windows": [
    {
      "tabManager": {
        "workspaces": [
          {
            "panels": [
              {
                "id": "AAAAAAAA-0000-0000-0000-000000000001",
                "stableSurfaceId": "BBBBBBBB-0000-0000-0000-000000000001",
                "type": "terminal",
                "title": "Terminal"
              },
              {
                "id": "AAAAAAAA-0000-0000-0000-000000000002",
                "stableSurfaceId": "BBBBBBBB-0000-0000-0000-000000000002",
                "type": "browser",
                "title": "Admin Console",
                "browser": {
                  "urlString": "http://localhost:3000/admin",
                  "profileID": "29A2424D-B926-4A45-9CE9-1DF3CF3E23D5",
                  "pageZoom": 1
                }
              },
              {
                "id": "AAAAAAAA-0000-0000-0000-000000000003",
                "stableSurfaceId": "BBBBBBBB-0000-0000-0000-000000000003",
                "type": "browser",
                "title": "Docs",
                "browser": {
                  "urlString": "https://example.com/",
                  "profileID": "52B43C05-4A1D-45D3-8FD5-9EF94952E445"
                }
              }
            ]
          }
        ]
      }
    }
  ]
}`

func TestParseSessionSurfaceProfiles(t *testing.T) {
	got, err := parseSessionSurfaceProfiles([]byte(sessionJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := map[string]string{
		"AAAAAAAA-0000-0000-0000-000000000002": "29A2424D-B926-4A45-9CE9-1DF3CF3E23D5",
		"AAAAAAAA-0000-0000-0000-000000000003": "52B43C05-4A1D-45D3-8FD5-9EF94952E445",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("panel %s profile = %q, want %q", k, got[k], v)
		}
	}
}

func TestComposeSurfaceProfiles(t *testing.T) {
	profiles := []BrowserProfile{
		{ID: "52B43C05-4A1D-45D3-8FD5-9EF94952E445", Slug: "default", Name: "Default", Default: true},
		{ID: "29A2424D-B926-4A45-9CE9-1DF3CF3E23D5", Slug: "work-admin", Name: "Work Admin"},
	}
	panelProfiles := map[string]string{
		"AAAAAAAA-0000-0000-0000-000000000002": "29A2424D-B926-4A45-9CE9-1DF3CF3E23D5", // work-admin
		"AAAAAAAA-0000-0000-0000-000000000003": "52B43C05-4A1D-45D3-8FD5-9EF94952E445", // default → excluded
		"AAAAAAAA-0000-0000-0000-000000000004": "DEADBEEF-0000-0000-0000-000000000000", // deleted profile → excluded
	}
	idToRef := map[string]string{
		"AAAAAAAA-0000-0000-0000-000000000002": "surface:7",
		"AAAAAAAA-0000-0000-0000-000000000003": "surface:8",
		"AAAAAAAA-0000-0000-0000-000000000004": "surface:9",
	}

	got := composeSurfaceProfiles(panelProfiles, profiles, idToRef)
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1: %v", len(got), got)
	}
	if got["surface:7"] != "work-admin" {
		t.Errorf("surface:7 profile = %q, want %q", got["surface:7"], "work-admin")
	}
}

func TestNewestSessionFile(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "session-com.cmuxterm.app.debug.json")
	cur := filepath.Join(dir, "session-com.cmuxterm.app.json")
	prev := filepath.Join(dir, "session-com.cmuxterm.app-previous.json")
	for _, p := range []string{old, cur, prev} {
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Make "previous" the newest by mtime: it must still be excluded.
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	got, err := newestSessionFile(dir)
	if err != nil {
		t.Fatalf("newestSessionFile: %v", err)
	}
	if got != cur {
		t.Errorf("newestSessionFile = %q, want %q", got, cur)
	}
}

func TestNewestSessionFile_Empty(t *testing.T) {
	if _, err := newestSessionFile(t.TempDir()); err == nil {
		t.Error("expected error for dir without session files")
	}
}

func TestBrowserProfileExists(t *testing.T) {
	profiles := []BrowserProfile{
		{ID: "52B43C05-4A1D-45D3-8FD5-9EF94952E445", Slug: "default", Name: "Default", Default: true},
		{ID: "29A2424D-B926-4A45-9CE9-1DF3CF3E23D5", Slug: "work-admin", Name: "Work Admin"},
	}
	cases := []struct {
		selector string
		want     bool
	}{
		{"work-admin", true},
		{"Work Admin", true},                           // display name
		{"WORK-ADMIN", true},                           // case-insensitive
		{"29A2424D-B926-4A45-9CE9-1DF3CF3E23D5", true}, // UUID
		{"29a2424d-b926-4a45-9ce9-1df3cf3e23d5", true}, // lowercase UUID
		{"default", true},
		{"personal", false},
		{"", false},
	}
	for _, c := range cases {
		if got := browserProfileExists(profiles, c.selector); got != c.want {
			t.Errorf("browserProfileExists(%q) = %v, want %v", c.selector, got, c.want)
		}
	}
}

func TestParseTreeSurfaceIDs(t *testing.T) {
	// Shape of `cmux tree --json --id-format both`: surfaces carry both ref and id.
	treeJSON := `{
	  "windows": [
	    {"ref": "window:1", "workspaces": [
	      {"ref": "workspace:4", "panes": [
	        {"ref": "pane:4", "surfaces": [
	          {"ref": "surface:4", "id": "533ACBE5-CBC0-4071-9403-45E87A6548EC", "type": "terminal"},
	          {"ref": "surface:5", "id": "716F0F44-FCFB-4CE1-BB19-65FCDBB9D240", "type": "browser"}
	        ]}
	      ]}
	    ]}
	  ]
	}`
	got, err := parseTreeSurfaceIDs(treeJSON)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := map[string]string{
		"533ACBE5-CBC0-4071-9403-45E87A6548EC": "surface:4",
		"716F0F44-FCFB-4CE1-BB19-65FCDBB9D240": "surface:5",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("id %s ref = %q, want %q", k, got[k], v)
		}
	}
}

func TestNewPaneArgs_Profile(t *testing.T) {
	got := newPaneArgs(NewPaneOpts{
		Type: "browser", Direction: "right", WorkspaceRef: "workspace:1",
		URL: "http://localhost:3000", Profile: "work-admin",
	})
	want := []string{
		"new-pane", "--type", "browser", "--direction", "right",
		"--workspace", "workspace:1", "--url", "http://localhost:3000",
		"--profile", "work-admin",
	}
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args = %v, want %v", got, want)
		}
	}
}

func TestNewPaneArgs_NoProfileForTerminal(t *testing.T) {
	// cmux >0.64.20 rejects --profile on terminal panes; never emit it.
	got := newPaneArgs(NewPaneOpts{
		Type: "terminal", Direction: "right", WorkspaceRef: "workspace:1",
		Profile: "leaked",
	})
	for _, a := range got {
		if a == "--profile" || a == "leaked" {
			t.Fatalf("terminal pane args contain profile: %v", got)
		}
	}
}

func TestCmuxDryRun_FmtNewPaneProfile(t *testing.T) {
	got := CmuxDryRun{}.FmtNewPane("browser", "right", "workspace:1", "http://localhost:3000", "work-admin")
	want := `cmux new-pane --type browser --direction right --workspace workspace:1 --url "http://localhost:3000" --profile work-admin`
	if got != want {
		t.Errorf("FmtNewPane = %q, want %q", got, want)
	}
	// No profile → no flag.
	got = CmuxDryRun{}.FmtNewPane("browser", "right", "workspace:1", "http://localhost:3000", "")
	if strings.Contains(got, "--profile") {
		t.Errorf("FmtNewPane without profile contains --profile: %q", got)
	}
}

func TestNewPaneArgs_NoProfileStaysClean(t *testing.T) {
	got := newPaneArgs(NewPaneOpts{
		Type: "browser", Direction: "right", WorkspaceRef: "workspace:1",
		URL: "http://localhost:3000",
	})
	for _, a := range got {
		if strings.Contains(a, "profile") {
			t.Fatalf("no-profile browser pane args leak profile: %v", got)
		}
	}
}
