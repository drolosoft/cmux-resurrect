package client

import "testing"

// sessionDirsJSON mirrors cmux's session file: terminal panels carry the
// surface UUID in "id" and their working directory in "directory". A tab whose
// shell has never spawned still has a directory but no ttyName — that is
// exactly the case the live tree cannot report (GitHub #8).
const sessionDirsJSON = `{
  "windows": [
    {
      "tabManager": {
        "workspaces": [
          {
            "panels": [
              {
                "id": "AAAA0001-0000-0000-0000-000000000001",
                "type": "terminal",
                "ttyName": "ttys001",
                "directory": "/Users/u/proj/main",
                "title": "u@mac: ~/proj/main"
              },
              {
                "id": "AAAA0001-0000-0000-0000-000000000002",
                "type": "terminal",
                "ttyName": null,
                "directory": "/Users/u/proj/feature-a",
                "title": "Terminal"
              },
              {
                "id": "AAAA0001-0000-0000-0000-000000000003",
                "type": "browser",
                "browser": {"urlString": "https://example.com", "profileID": "P"}
              },
              {
                "id": "AAAA0001-0000-0000-0000-000000000004",
                "type": "terminal",
                "directory": ""
              }
            ]
          }
        ]
      }
    }
  ]
}`

func TestParseSessionSurfaceDirectories(t *testing.T) {
	got, err := parseSessionSurfaceDirectories([]byte(sessionDirsJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := map[string]string{
		"AAAA0001-0000-0000-0000-000000000001": "/Users/u/proj/main",
		"AAAA0001-0000-0000-0000-000000000002": "/Users/u/proj/feature-a",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("panel %s directory = %q, want %q", k, got[k], v)
		}
	}
}

func TestComposeSurfaceDirs(t *testing.T) {
	panelDirs := map[string]string{
		"AAAA0001-0000-0000-0000-000000000001": "/Users/u/proj/main",
		"AAAA0001-0000-0000-0000-000000000002": "/Users/u/proj/feature-a",
		"AAAA0001-0000-0000-0000-000000000009": "/Users/u/gone", // not in the live tree
	}
	idToRef := map[string]string{
		"AAAA0001-0000-0000-0000-000000000001": "surface:4",
		"AAAA0001-0000-0000-0000-000000000002": "surface:5",
	}

	got := composeSurfaceDirs(panelDirs, idToRef)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %v", len(got), got)
	}
	if got["surface:4"] != "/Users/u/proj/main" || got["surface:5"] != "/Users/u/proj/feature-a" {
		t.Errorf("composed dirs = %v", got)
	}
}
