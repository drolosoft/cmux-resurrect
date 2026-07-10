package client

import (
	"os"
	"testing"
)

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

func TestCwdFromTitle(t *testing.T) {
	tmp := t.TempDir() // a real, existing directory

	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"user@host prefix with absolute path", "txeo@Mac: " + tmp, tmp},
		{"bare absolute path", tmp, tmp},
		{"home tilde alone", "txeo@Mac: ~", homeDirOrEmpty()},
		{"nonexistent path rejected", "txeo@Mac: /no/such/dir-xyz", ""},
		{"file (not dir) rejected", "vim: /etc/hosts", ""},
		{"arbitrary title rejected", "make watch", ""},
		{"empty title", "", ""},
		{"tilde-user form rejected", "x: ~otheruser/dir", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cwdFromTitle(tt.title); got != tt.want {
				t.Errorf("cwdFromTitle(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func homeDirOrEmpty() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}
