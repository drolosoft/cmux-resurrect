package client

import (
	"strings"
	"testing"
)

func TestValidateSplitDirection(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"empty defaults to right", "", "right", false},
		{"right", "right", "right", false},
		{"left", "left", "left", false},
		{"up", "up", "up", false},
		{"down", "down", "down", false},
		{"uppercase rejected", "Right", "", true},
		{"unknown word rejected", "diagonal", "", true},
		// AppleScript injection attempts via a malicious saved layout's `split` value.
		{"newline injection rejected", "right\nend tell\ndo shell script \"id\"", "", true},
		{"quote breakout rejected", `right"`, "", true},
		{"semicolon rejected", "right; rm -rf /", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateSplitDirection(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateSplitDirection(%q) = %q, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateSplitDirection(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("validateSplitDirection(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestNewSplitRejectsMaliciousDirection ensures the injection guard fires at the
// backend boundary (before any osascript/cmux invocation), so a malicious
// `split` value in a layout cannot reach the AppleScript interpolation.
func TestNewSplitRejectsMaliciousDirection(t *testing.T) {
	malicious := "right\nend tell\ndo shell script \"touch /tmp/pwned\"\ntell application \"Ghostty\""

	g := NewGhosttyClient()
	if _, err := g.NewSplit(malicious, "tab:1", ""); err == nil {
		t.Fatal("GhosttyClient.NewSplit accepted malicious direction; injection guard missing")
	} else if !strings.Contains(err.Error(), "invalid split direction") {
		t.Fatalf("expected invalid-direction error, got: %v", err)
	}

	c := NewCLIClient()
	if _, err := c.NewSplit(malicious, "tab:1", ""); err == nil {
		t.Fatal("CLIClient.NewSplit accepted malicious direction; injection guard missing")
	} else if !strings.Contains(err.Error(), "invalid split direction") {
		t.Fatalf("expected invalid-direction error, got: %v", err)
	}
}
