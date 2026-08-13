package cmd

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

func TestResolveBackendChoice(t *testing.T) {
	det := func(b client.DetectedBackend) func() client.DetectedBackend {
		return func() client.DetectedBackend { return b }
	}
	cases := []struct {
		name          string
		envOverride   string
		configBackend string
		caller        client.DetectedBackend // terminal crex is running INSIDE
		detected      client.DetectedBackend
		callerRunning bool   // caller's app is running even if unreachable
		want          string // backend Name()
	}{
		{"env wins over everything", "ghostty", "cmux", client.BackendCmux, client.BackendCmux, false, "ghostty"},
		{"config wins over detection (external)", "", "ghostty", client.BackendUnknown, client.BackendCmux, false, "ghostty"},
		{"config cmux over ghostty detection", "", "cmux", client.BackendUnknown, client.BackendGhostty, false, "cmux"},
		{"empty config falls back to detection", "", "", client.BackendUnknown, client.BackendGhostty, false, "ghostty"},
		{"invalid config falls back to detection", "", "bogus", client.BackendUnknown, client.BackendCmux, false, "cmux"},
		{"invalid env falls back (config empty)", "bogus", "", client.BackendUnknown, client.BackendGhostty, false, "ghostty"},
		// The footgun guard: inside a LIVE session, config must NOT hijack.
		{"inside live cmux ignores ghostty config", "", "ghostty", client.BackendCmux, client.BackendCmux, false, "cmux"},
		// Leaked-but-dead cmux env (detection says ghostty) → config still applies.
		{"inside cmux env but dead, config applies", "", "ghostty", client.BackendCmux, client.BackendGhostty, false, "ghostty"},

		// Symmetry (field report): running crex from a Ghostty window must act
		// on Ghostty even when the config pins cmux as the default. The default
		// is a tie-breaker for external launchers, not an override of the
		// terminal the user is actually sitting in.
		{"inside live ghostty ignores cmux config", "", "cmux", client.BackendGhostty, client.BackendGhostty, false, "ghostty"},
		{"inside ghostty, empty config", "", "", client.BackendGhostty, client.BackendGhostty, false, "ghostty"},
		{"env still wins over the ghostty caller", "cmux", "", client.BackendGhostty, client.BackendGhostty, false, "cmux"},
		// Ghostty env but the app is gone → config/detection decide.
		{"ghostty caller but app dead, config applies", "", "cmux", client.BackendGhostty, client.BackendCmux, false, "cmux"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cl := resolveBackendChoice(tc.envOverride, tc.configBackend, tc.caller, det(tc.detected),
				func(client.DetectedBackend) bool { return tc.callerRunning })
			if cl == nil {
				t.Fatal("nil client")
			}
			if got := backendName(cl); got != tc.want {
				t.Errorf("resolveBackendChoice(env=%q cfg=%q caller=%q det=%q) = %q, want %q",
					tc.envOverride, tc.configBackend, tc.caller, tc.detected, got, tc.want)
			}
		})
	}
}

func TestCallerBackendFromEnv(t *testing.T) {
	cases := []struct {
		name        string
		cmuxWS      string
		cmuxSurface string
		termProgram string
		want        client.DetectedBackend
	}{
		{"cmux shell", "WS-1", "", "ghostty", client.BackendCmux},
		{"cmux shell via surface id", "", "SURF-1", "ghostty", client.BackendCmux},
		// cmux is built on Ghostty and sets TERM_PROGRAM=ghostty for its own
		// shells, so TERM_PROGRAM alone must never decide — the CMUX_* vars do.
		{"plain ghostty window", "", "", "ghostty", client.BackendGhostty},
		{"ghostty capitalized", "", "", "Ghostty", client.BackendGhostty},
		{"other terminal", "", "", "Apple_Terminal", client.BackendUnknown},
		{"no terminal context (cron, alfred)", "", "", "", client.BackendUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CMUX_WORKSPACE_ID", tc.cmuxWS)
			t.Setenv("CMUX_SURFACE_ID", tc.cmuxSurface)
			t.Setenv("TERM_PROGRAM", tc.termProgram)
			if got := callerBackend(); got != tc.want {
				t.Errorf("callerBackend() = %q, want %q", got, tc.want)
			}
		})
	}
}

// backendName maps a Backend to a comparable name for the test.
func backendName(cl client.Backend) string {
	switch cl.(type) {
	case *client.GhosttyClient:
		return "ghostty"
	case *client.CLIClient:
		return "cmux"
	default:
		return "unknown"
	}
}

// TestResolveBackendChoice_UnreachableButRunningCallerWins: cmux's socket can be
// unreachable while the app is wide open (Socket Control Mode off or password
// protected). Sitting in a cmux tab, crex must NOT silently drive Ghostty — it
// must target cmux and let the error name the right backend.
func TestResolveBackendChoice_UnreachableButRunningCallerWins(t *testing.T) {
	det := func(b client.DetectedBackend) func() client.DetectedBackend {
		return func() client.DetectedBackend { return b }
	}
	cases := []struct {
		name          string
		configBackend string
		caller        client.DetectedBackend
		detected      client.DetectedBackend
		callerRunning bool
		want          string
	}{
		{
			name: "cmux open but socket unreachable, ghostty live",
			// Detection points at Ghostty, but we are inside cmux and cmux is
			// running: never hand the command to another app.
			configBackend: "", caller: client.BackendCmux, detected: client.BackendGhostty,
			callerRunning: true, want: "cmux",
		},
		{
			name:          "same, with a ghostty default configured",
			configBackend: "ghostty", caller: client.BackendCmux, detected: client.BackendGhostty,
			callerRunning: true, want: "cmux",
		},
		{
			// The deliberate escape hatch: a Ghostty shell that inherited
			// CMUX_* vars from a cmux that has since been CLOSED. cmux is not
			// running, so falling through to the live Ghostty is correct.
			name:          "leaked cmux env, cmux closed → live ghostty",
			configBackend: "", caller: client.BackendCmux, detected: client.BackendGhostty,
			callerRunning: false, want: "ghostty",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cl := resolveBackendChoice("", tc.configBackend, tc.caller, det(tc.detected),
				func(client.DetectedBackend) bool { return tc.callerRunning })
			if got := backendName(cl); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
