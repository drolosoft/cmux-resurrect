package client

import "testing"

const debugTerminalsSample = `{
  "count": 3,
  "terminals": [
    {"surface_ref":"surface:1","current_directory":"/Users/x/a","runtime_surface_ready":true},
    {"surface_ref":"surface:2","current_directory":"/Users/x/b","runtime_surface_ready":false},
    {"surface_ref":"surface:3","current_directory":"/private/tmp","runtime_surface_ready":true}
  ]
}`

func TestParseSurfaceState(t *testing.T) {
	// Found + ready, with cwd.
	st, err := parseSurfaceState(debugTerminalsSample, "surface:3")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if st == nil || st.CWD != "/private/tmp" || !st.Ready {
		t.Fatalf("surface:3 = %+v, want cwd=/private/tmp ready=true", st)
	}
	// Found but not ready.
	st, _ = parseSurfaceState(debugTerminalsSample, "surface:2")
	if st == nil || st.Ready {
		t.Fatalf("surface:2 should be found and not ready, got %+v", st)
	}
	// Not present → nil, no error.
	st, err = parseSurfaceState(debugTerminalsSample, "surface:99")
	if err != nil || st != nil {
		t.Fatalf("missing surface should be (nil,nil), got (%+v,%v)", st, err)
	}
	// Garbage JSON → error.
	if _, err := parseSurfaceState("not json", "surface:1"); err == nil {
		t.Fatal("expected parse error on bad JSON")
	}
}
