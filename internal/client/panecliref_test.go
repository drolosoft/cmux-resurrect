package client

import "testing"

func TestPaneCLIRef(t *testing.T) {
	tests := []struct {
		paneRef, ws, want string
	}{
		// The #8 bug: with a workspace, "pane:N" must be stripped to "N" so cmux
		// reads it as a workspace-local index (it rejected the un-stripped form).
		{"pane:0", "workspace:17", "0"},
		{"pane:3", "workspace:1", "3"},
		// No workspace → leave as-is (global ref form).
		{"pane:0", "", "pane:0"},
		// Real (non-"pane:") refs must never be mangled.
		{"abc-uuid", "workspace:1", "abc-uuid"},
		{"surface:5", "workspace:1", "surface:5"},
		{"", "workspace:1", ""},
	}
	for _, tt := range tests {
		if got := paneCLIRef(tt.paneRef, tt.ws); got != tt.want {
			t.Errorf("paneCLIRef(%q, %q) = %q, want %q", tt.paneRef, tt.ws, got, tt.want)
		}
	}
}
