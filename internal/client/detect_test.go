package client

import (
	"testing"
)

func alive() bool { return true }
func dead() bool  { return false }

func TestDetectWith_Matrix(t *testing.T) {
	tests := []struct {
		name           string
		cmuxEnv        bool
		cmuxAlive      func() bool
		ghosttyRunning func() bool
		want           DetectedBackend
	}{
		{"inside cmux, alive", true, alive, dead, BackendCmux},
		{"inside cmux, alive, ghostty also open", true, alive, alive, BackendCmux},
		// THE BUG (July 2026): Ghostty shells launched from within cmux
		// inherit CMUX_* env; when cmux is closed the socket is dead and
		// crex must fall back to the live Ghostty, not die on broken pipe.
		{"leaked cmux env, cmux dead, ghostty running", true, dead, alive, BackendGhostty},
		{"cmux env, cmux dead, nothing else", true, dead, dead, BackendCmux},
		{"no env, ghostty running", false, dead, alive, BackendGhostty},
		{"no env, nothing running", false, dead, dead, BackendUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectWith(tt.cmuxEnv, tt.cmuxAlive, tt.ghosttyRunning); got != tt.want {
				t.Errorf("detectWith(%v) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestDetect_NoCmuxEnv_NeverPicksCmux(t *testing.T) {
	t.Setenv("CMUX_SOCKET_PATH", "")
	t.Setenv("CMUX_WORKSPACE_ID", "")
	if got := Detect(); got == BackendCmux {
		t.Errorf("Detect() = %q without cmux env vars", got)
	}
}
