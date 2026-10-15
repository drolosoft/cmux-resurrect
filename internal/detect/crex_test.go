package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsCrexCommand(t *testing.T) {
	cases := []struct {
		name     string
		cmdline  string
		selfPath string
		want     bool
	}{
		{name: "save from PATH", cmdline: "crex save now", want: true},
		{name: "restore by absolute path", cmdline: "/opt/homebrew/bin/crex restore x", want: true},
		{name: "dev build elsewhere", cmdline: "/some/dev/build/crex save", want: true},
		{name: "bare crex", cmdline: "crex", want: true},
		{name: "env prefixed", cmdline: "FOO=1 crex save", want: true},
		{
			// A dev build renamed on disk is still this program when the pane
			// runs the very file that is executing now.
			name:     "same file as the running executable",
			cmdline:  "/tmp/builds/crex-dev save now",
			selfPath: "/tmp/builds/crex-dev",
			want:     true,
		},
		{
			// Saved commands keep only the binary name, so a renamed build
			// shows up without its directory.
			name:     "running executable's name without a directory",
			cmdline:  "crex-dev restore now",
			selfPath: "/tmp/builds/crex-dev",
			want:     true,
		},
		{name: "crex as an argument", cmdline: "vim crex.toml", want: false},
		{name: "cd into a crex folder", cmdline: "cd crex", want: false},
		{name: "longer binary name", cmdline: "crexx save", want: false},
		{name: "unrelated tool", cmdline: "claude --resume id", selfPath: "/opt/homebrew/bin/crex", want: false},
		{name: "empty", cmdline: "", selfPath: "/opt/homebrew/bin/crex", want: false},
		{
			// Another program that happens to live next to crex is not crex.
			name:     "different file in the same directory",
			cmdline:  "/tmp/builds/other save",
			selfPath: "/tmp/builds/crex-dev",
			want:     false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsCrexCommand(c.cmdline, c.selfPath); got != c.want {
				t.Errorf("IsCrexCommand(%q, %q) = %v, want %v", c.cmdline, c.selfPath, got, c.want)
			}
		})
	}
}

// TestIsCrexCommand_ResolvesSymlinks covers a pane that runs crex through a
// symlink (Homebrew links bin/crex into the Cellar) while os.Executable
// reports the real file, or the other way round.
func TestIsCrexCommand_ResolvesSymlinks(t *testing.T) {
	dir := t.TempDir()

	realBinary := filepath.Join(dir, "cellar-build")
	if err := os.WriteFile(realBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	linkedBinary := filepath.Join(dir, "linked-build")
	if err := os.Symlink(realBinary, linkedBinary); err != nil {
		t.Fatal(err)
	}

	if !IsCrexCommand(linkedBinary+" save now", realBinary) {
		t.Errorf("a symlink to the running executable must count as crex")
	}
	if !IsCrexCommand(realBinary+" save now", linkedBinary) {
		t.Errorf("the target of a symlinked executable must count as crex")
	}
}
