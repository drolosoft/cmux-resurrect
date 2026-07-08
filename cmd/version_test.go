package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootVersionFlag(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--version"})
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("crex --version: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, Version) {
		t.Errorf("--version output = %q, want it to contain version %q", out, Version)
	}
	if !strings.Contains(out, Commit) {
		t.Errorf("--version output = %q, want it to contain commit %q", out, Commit)
	}
}
