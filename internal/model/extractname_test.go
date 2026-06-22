package model

import "testing"

func TestExtractPaneName(t *testing.T) {
	tests := []struct {
		in       string
		wantName string
		wantRest string
	}{
		{`main terminal "Plan":`, "Plan", "main terminal:"},
		{`split right "Diff":`, "Diff", "split right:"},
		{`tab 2 "QA":`, "QA", "tab 2:"},
		{`split right "Diff"`, "Diff", "split right"},
		{`split down "Logs":`, "Logs", "split down:"},
		// No name → unchanged (backward compatible).
		{`split right:`, "", "split right:"},
		{`main terminal`, "", "main terminal"},
		{`tab 2:`, "", "tab 2:"},
		// Name with spaces.
		{`split right "My Logs":`, "My Logs", "split right:"},
		// Unterminated quote → treated as no name.
		{`split right "oops`, "", `split right "oops`},
	}
	for _, tt := range tests {
		name, rest := ExtractPaneName(tt.in)
		if name != tt.wantName || rest != tt.wantRest {
			t.Errorf("ExtractPaneName(%q) = (%q, %q), want (%q, %q)", tt.in, name, rest, tt.wantName, tt.wantRest)
		}
	}
}
