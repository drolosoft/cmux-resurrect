package client

import (
	"testing"
)

func TestDetect_CmuxSocketPath(t *testing.T) {
	t.Setenv("CMUX_SOCKET_PATH", "/tmp/cmux.sock")
	t.Setenv("CMUX_WORKSPACE_ID", "")
	if got := Detect(); got != BackendCmux {
		t.Errorf("Detect() = %q, want %q", got, BackendCmux)
	}
}

func TestDetect_CmuxWorkspaceID(t *testing.T) {
	t.Setenv("CMUX_SOCKET_PATH", "")
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	if got := Detect(); got != BackendCmux {
		t.Errorf("Detect() = %q, want %q", got, BackendCmux)
	}
}

func TestDetect_NoCmuxEnv(t *testing.T) {
	t.Setenv("CMUX_SOCKET_PATH", "")
	t.Setenv("CMUX_WORKSPACE_ID", "")
	got := Detect()
	// Without cmux env vars, result depends on whether Ghostty is running.
	// In CI/test, Ghostty is not running, so expect Unknown.
	if got == BackendCmux {
		t.Errorf("Detect() = %q without cmux env vars", got)
	}
}

func TestDetect_CmuxTakesPriority(t *testing.T) {
	// Even if Ghostty is running, cmux env vars take priority.
	t.Setenv("CMUX_SOCKET_PATH", "/tmp/cmux.sock")
	if got := Detect(); got != BackendCmux {
		t.Errorf("Detect() = %q, want %q (cmux should take priority)", got, BackendCmux)
	}
}
