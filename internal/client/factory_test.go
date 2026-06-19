package client

import "testing"

func TestNewForOverride(t *testing.T) {
	tests := []struct {
		override string
		wantOK   bool
		check    func(Backend) bool
	}{
		{"ghostty", true, func(b Backend) bool { g, ok := b.(*GhosttyClient); return ok && g.appName() == "Ghostty" }},
		{"cmux-applescript", true, func(b Backend) bool { g, ok := b.(*GhosttyClient); return ok && g.appName() == "cmux" }},
		{"cmux", true, func(b Backend) bool { _, ok := b.(*CLIClient); return ok }},
		{"", false, nil},
		{"kitty", false, nil},
		{"GHOSTTY", false, nil}, // case-sensitive on purpose
	}
	for _, tt := range tests {
		t.Run(tt.override, func(t *testing.T) {
			b, ok := NewForOverride(tt.override)
			if ok != tt.wantOK {
				t.Fatalf("NewForOverride(%q) ok = %v, want %v", tt.override, ok, tt.wantOK)
			}
			if !tt.wantOK {
				if b != nil {
					t.Fatalf("NewForOverride(%q) returned non-nil backend on failure", tt.override)
				}
				return
			}
			if !tt.check(b) {
				t.Fatalf("NewForOverride(%q) returned the wrong backend type/app", tt.override)
			}
		})
	}
}

func TestNewForDetected(t *testing.T) {
	if _, ok := NewForDetected(BackendGhostty).(*GhosttyClient); !ok {
		t.Error("NewForDetected(Ghostty) did not return *GhosttyClient")
	}
	if _, ok := NewForDetected(BackendCmux).(*CLIClient); !ok {
		t.Error("NewForDetected(Cmux) did not return *CLIClient")
	}
	// Unknown falls back to the CLI client (the safe default).
	if _, ok := NewForDetected(BackendUnknown).(*CLIClient); !ok {
		t.Error("NewForDetected(Unknown) did not fall back to *CLIClient")
	}
}
