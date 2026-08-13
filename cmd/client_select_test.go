package cmd

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

// TestClientFor_OverrideWinsOverDetection ensures CREX_BACKEND takes precedence
// over auto-detection and that the detection callback isn't even consulted when
// a valid override is set (so commands like `crex setup` honor the env var —
// the M5 bug where doFirstSave silently ignored CREX_BACKEND).
func TestClientFor_OverrideWinsOverDetection(t *testing.T) {
	t.Setenv("CREX_BACKEND", "ghostty")

	detectCalled := false
	cl := clientFor(func() client.DetectedBackend {
		detectCalled = true
		return client.BackendCmux // detection says cmux...
	})

	if _, ok := cl.(*client.GhosttyClient); !ok {
		t.Fatalf("clientFor with CREX_BACKEND=ghostty returned %T, want *client.GhosttyClient", cl)
	}
	if detectCalled {
		t.Error("clientFor consulted detection despite a valid override being set")
	}
}

// clearCallerContext removes the env that identifies the terminal crex is
// running inside, so a test's outcome doesn't depend on which terminal the
// suite happens to run in (the caller's own session outranks detection).
func clearCallerContext(t *testing.T) {
	t.Helper()
	t.Setenv("CMUX_WORKSPACE_ID", "")
	t.Setenv("CMUX_SURFACE_ID", "")
	t.Setenv("TERM_PROGRAM", "")
}

// TestClientFor_FallsBackToDetection ensures that with no override and no
// caller context, the detected backend is used.
func TestClientFor_FallsBackToDetection(t *testing.T) {
	t.Setenv("CREX_BACKEND", "")
	clearCallerContext(t)

	cl := clientFor(func() client.DetectedBackend { return client.BackendGhostty })
	if _, ok := cl.(*client.GhosttyClient); !ok {
		t.Fatalf("clientFor with detected=ghostty returned %T, want *client.GhosttyClient", cl)
	}
}

// TestClientFor_UnknownOverrideFallsBack ensures an unrecognized CREX_BACKEND
// falls back to detection rather than failing.
func TestClientFor_UnknownOverrideFallsBack(t *testing.T) {
	t.Setenv("CREX_BACKEND", "kitty")
	clearCallerContext(t)

	cl := clientFor(func() client.DetectedBackend { return client.BackendCmux })
	if _, ok := cl.(*client.CLIClient); !ok {
		t.Fatalf("clientFor with unknown override returned %T, want *client.CLIClient (detected fallback)", cl)
	}
}
