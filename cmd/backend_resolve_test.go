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
		insideCmux    bool
		detected      client.DetectedBackend
		want          string // backend Name()
	}{
		{"env wins over everything", "ghostty", "cmux", true, client.BackendCmux, "ghostty"},
		{"config wins over detection (external)", "", "ghostty", false, client.BackendCmux, "ghostty"},
		{"config cmux over ghostty detection", "", "cmux", false, client.BackendGhostty, "cmux"},
		{"empty config falls back to detection", "", "", false, client.BackendGhostty, "ghostty"},
		{"invalid config falls back to detection", "", "bogus", false, client.BackendCmux, "cmux"},
		{"invalid env falls back (config empty)", "bogus", "", false, client.BackendGhostty, "ghostty"},
		// The footgun guard: inside a LIVE cmux session, config must NOT hijack.
		{"inside live cmux ignores ghostty config", "", "ghostty", true, client.BackendCmux, "cmux"},
		// Leaked-but-dead cmux env (detection says ghostty) → config still applies.
		{"inside cmux env but dead, config applies", "", "ghostty", true, client.BackendGhostty, "ghostty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cl := resolveBackendChoice(tc.envOverride, tc.configBackend, tc.insideCmux, det(tc.detected))
			if cl == nil {
				t.Fatal("nil client")
			}
			if got := backendName(cl); got != tc.want {
				t.Errorf("resolveBackendChoice(env=%q cfg=%q det=%q) = %q, want %q",
					tc.envOverride, tc.configBackend, tc.detected, got, tc.want)
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
