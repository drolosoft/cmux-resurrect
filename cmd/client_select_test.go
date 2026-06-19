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

// TestClientFor_FallsBackToDetection ensures that with no override the detected
// backend is used.
func TestClientFor_FallsBackToDetection(t *testing.T) {
	t.Setenv("CREX_BACKEND", "")

	cl := clientFor(func() client.DetectedBackend { return client.BackendGhostty })
	if _, ok := cl.(*client.GhosttyClient); !ok {
		t.Fatalf("clientFor with detected=ghostty returned %T, want *client.GhosttyClient", cl)
	}
}

// TestClientFor_UnknownOverrideFallsBack ensures an unrecognized CREX_BACKEND
// falls back to detection rather than failing.
func TestClientFor_UnknownOverrideFallsBack(t *testing.T) {
	t.Setenv("CREX_BACKEND", "kitty")

	cl := clientFor(func() client.DetectedBackend { return client.BackendCmux })
	if _, ok := cl.(*client.CLIClient); !ok {
		t.Fatalf("clientFor with unknown override returned %T, want *client.CLIClient (detected fallback)", cl)
	}
}
