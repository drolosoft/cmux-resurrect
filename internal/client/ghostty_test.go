package client

import "testing"

func TestParseTabIndex(t *testing.T) {
	tests := []struct {
		ref     string
		want    int
		wantErr bool
	}{
		{"tab:1", 1, false},
		{"tab:5", 5, false},
		{"tab:0", 0, false},
		{"invalid", 0, true},
		{"tab:", 0, true},
		{"tab:abc", 0, true},
	}
	for _, tt := range tests {
		got, err := parseTabIndex(tt.ref)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseTabIndex(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseTabIndex(%q) = %d, want %d", tt.ref, got, tt.want)
		}
	}
}

func TestParseTerminalIndex(t *testing.T) {
	tests := []struct {
		ref     string
		want    int
		wantErr bool
	}{
		// terminal refs are already 1-based — pass through.
		{"terminal:1", 1, false},
		{"terminal:3", 3, false},
		// pane refs are 0-based — convert to 1-based.
		{"pane:0", 1, false},
		{"pane:1", 2, false},
		{"pane:2", 3, false},
		// errors
		{"invalid", 0, true},
		{"pane:", 0, true},
		{"pane:abc", 0, true},
	}
	for _, tt := range tests {
		got, err := parseTerminalIndex(tt.ref)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseTerminalIndex(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseTerminalIndex(%q) = %d, want %d", tt.ref, got, tt.want)
		}
	}
}
