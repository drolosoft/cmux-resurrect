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

// cmux ≥0.64 lazily spawns background surfaces and flips runtime_surface_ready
// at first RENDER, before the shell accepts input; it also started reporting
// each terminal's tty. When the backend reports ttys, a surface without one
// is a shell that hasn't started — it must NOT count as ready (typing into it
// loses the input; the 2026-07-11 audit caught cds vanishing on 0.64).
const debugTerminals064Sample = `{
  "count": 3,
  "terminals": [
    {"surface_ref":"surface:1","current_directory":"/Users/x/a","runtime_surface_ready":true,"tty":"ttys004"},
    {"surface_ref":"surface:2","current_directory":"/Users/x/b","runtime_surface_ready":true,"tty":null},
    {"surface_ref":"surface:3","current_directory":"/Users/x/c","runtime_surface_ready":false,"tty":null}
  ]
}`

func TestParseSurfaceState_Cmux064RequiresTTYWhenReported(t *testing.T) {
	// Backend reports ttys (surface:1 has one) → rendered-but-ttyless
	// surface:2 is NOT ready yet.
	st, err := parseSurfaceState(debugTerminals064Sample, "surface:2")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if st == nil || st.Ready {
		t.Fatalf("surface:2 (ready flag set, no tty, ttys reported) must not be Ready, got %+v", st)
	}
	// A surface with flag + tty is ready.
	st, _ = parseSurfaceState(debugTerminals064Sample, "surface:1")
	if st == nil || !st.Ready {
		t.Fatalf("surface:1 should be Ready, got %+v", st)
	}
}

func TestParseSurfaceState_Cmux063NoTTYsKeepsFlagSemantics(t *testing.T) {
	// On cmux ≤0.63 tty is always null for every terminal — the flag alone
	// must keep meaning ready (the old, proven behavior).
	st, _ := parseSurfaceState(debugTerminalsSample, "surface:1")
	if st == nil || !st.Ready {
		t.Fatalf("surface:1 on 0.63-style output should be Ready, got %+v", st)
	}
}
