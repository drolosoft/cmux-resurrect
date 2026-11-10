package detect

import (
	"os"
	"path/filepath"
	"testing"
)

// testMatcher builds a matcher over a subset of the real registry: plain
// names, a dashed name, a tool with a generic process name guarded by its
// install path, and one started through a launcher binary.
func testMatcher() processMatcher {
	return newProcessMatcher([]detector{
		{Name: "claude", ProcessNames: []string{"claude"}},
		{Name: "codex", ProcessNames: []string{"codex"}},
		{Name: "opencode", ProcessNames: []string{"opencode"}},
		{Name: "gemini", ProcessNames: []string{"gemini"}},
		{Name: "pi", ProcessNames: []string{"pi"}},
		{Name: "factory", ProcessNames: []string{"droid"}},
		{Name: "cursor", ProcessNames: []string{"agent", "cursor-agent"}, PathMarker: "cursor-agent"},
		{Name: "rovo", ProcessNames: []string{"atlassian_cli_rovodev"}, LaunchPrefixes: []string{"acli rovodev"}},
	})
}

func TestMatchArgs(t *testing.T) {
	matcher := testMatcher()

	tests := []struct {
		name     string
		args     string
		wantTool string
		wantOK   bool
	}{
		// argv[0] basename is the tool — alias expansion, nvm/brew bin shim, wrapper named after the tool.
		{"nvm bin shim", "/Users/dev/.nvm/versions/node/v23.11.0/bin/claude --dangerously-skip-permissions", "claude", true},
		{"plain binary", "claude -c", "claude", true},
		{"alias expanded", "claude --resume abc123", "claude", true},
		{"dashed process name", "/opt/homebrew/bin/cursor-agent chat", "cursor", true},
		{"binary not named after the tool", "/Users/dev/.local/bin/droid", "factory", true},

		// Interpreter launching a tool script — comm would be "node", argv0 misses, path arg matches.
		{"node + script basename", "node /home/u/.local/share/codex/cli.js", "codex", true},
		{"node + path segment", "node /opt/lib/node_modules/opencode/dist/index.js serve", "opencode", true},
		{"bun + script", "bun /Users/x/.bun/install/global/node_modules/gemini/bin.js", "gemini", true},
		{"npx wrapper", "npx /tmp/cache/claude/cli.js", "claude", true},
		{"node in the cursor install dir", "node /Users/x/.local/share/cursor-agent/versions/2026.01/index.js", "cursor", true},

		// A launcher binary followed by the words that start the tool.
		{"launcher subcommand", "acli rovodev run", "rovo", true},
		{"launcher by full path", "/opt/homebrew/bin/acli rovodev run --yolo", "rovo", true},
		{"launcher without run", "acli rovodev", "rovo", true},
		{"tool binary of a launcher", "/Users/x/.rovodev/atlassian_cli_rovodev serve", "rovo", true},

		// False positives that MUST NOT match.
		{"tool as file arg to cat", "cat claude", "", false},
		{"tool as file arg to vim", "vim /Users/x/claude/notes.md", "", false},
		{"unrelated node app", "node /srv/app/server.js --port 3000", "", false},
		{"interpreter flag only", "node --inspect", "", false},
		{"substring not segment", "node /srv/apidocs/index.js", "", false}, // "pi" must not match inside "apidocs"
		{"generic name outside its install dir", "/usr/local/bin/agent --serve", "", false},
		{"generic name bare", "agent", "", false},
		{"generic name as script dir", "node /srv/agent/server.js", "", false},
		{"launcher with another subcommand", "acli jira workitem view", "", false},
		{"launcher alone", "acli", "", false},
		{"empty", "", "", false},
		{"shell only", "-zsh", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := matcher.matchArgs(tt.args)
			if ok != tt.wantOK {
				t.Fatalf("matchArgs(%q) ok = %v, want %v (tool=%q)", tt.args, ok, tt.wantOK, tool)
			}
			if ok && tool != tt.wantTool {
				t.Fatalf("matchArgs(%q) tool = %q, want %q", tt.args, tool, tt.wantTool)
			}
		})
	}
}

func TestMatchComm(t *testing.T) {
	matcher := testMatcher()

	tests := []struct {
		name     string
		comm     string
		wantTool string
		wantOK   bool
	}{
		{"full path", "/Users/dev/.local/bin/claude", "claude", true},
		{"bare name", "codex", "codex", true},
		{"binary not named after the tool", "/Users/dev/.local/bin/droid", "factory", true},
		{"second process name", "cursor-agent", "cursor", true},
		{"generic name inside its install dir", "/Users/dev/.local/share/cursor-agent/versions/2026.01/agent", "cursor", true},
		{"generic name outside its install dir", "/usr/local/bin/agent", "", false},
		{"generic name bare", "agent", "", false},
		{"unknown", "/usr/bin/htop", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := matcher.matchComm(tt.comm)
			if ok != tt.wantOK {
				t.Fatalf("matchComm(%q) ok = %v, want %v (tool=%q)", tt.comm, ok, tt.wantOK, tool)
			}
			if ok && tool != tt.wantTool {
				t.Fatalf("matchComm(%q) tool = %q, want %q", tt.comm, tool, tt.wantTool)
			}
		})
	}
}

// The Cursor installer puts both `agent` and `cursor-agent` in ~/.local/bin
// as links into ~/.local/share/cursor-agent/versions/<version>/. The link
// path says nothing about Cursor; the file it points to does.
func TestMatchArgs_GenericNameThroughSymlink(t *testing.T) {
	home := t.TempDir()
	versionDir := filepath.Join(home, ".local", "share", "cursor-agent", "versions", "2026.01")
	binDir := filepath.Join(home, ".local", "bin")
	for _, dir := range []string{versionDir, binDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	launcher := filepath.Join(versionDir, "cursor-agent")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(binDir, "agent")
	if err := os.Symlink(launcher, link); err != nil {
		t.Fatal(err)
	}

	matcher := testMatcher()

	tool, ok := matcher.matchArgs(link + " --model auto")
	if !ok || tool != "cursor" {
		t.Errorf("matchArgs through the agent link = (%q, %v), want (cursor, true)", tool, ok)
	}

	tool, ok = matcher.matchComm(link)
	if !ok || tool != "cursor" {
		t.Errorf("matchComm through the agent link = (%q, %v), want (cursor, true)", tool, ok)
	}

	// A link named agent that points anywhere else stays unmatched.
	otherTarget := filepath.Join(home, "other-agent")
	if err := os.WriteFile(otherTarget, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	otherLink := filepath.Join(home, "bin-other", "agent")
	if err := os.MkdirAll(filepath.Dir(otherLink), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(otherTarget, otherLink); err != nil {
		t.Fatal(err)
	}
	if tool, ok := matcher.matchArgs(otherLink); ok {
		t.Errorf("matchArgs(%q) = %q, want no match", otherLink, tool)
	}
}
