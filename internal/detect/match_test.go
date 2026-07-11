package detect

import "testing"

func TestMatchAIToolArgs(t *testing.T) {
	// Known tool ProcessNames (subset of the real registry, incl. a dashed one).
	known := map[string]bool{
		"claude": true, "codex": true, "opencode": true, "gemini": true,
		"cursor-agent": true, "pi": true,
	}
	isKnown := func(n string) bool { return known[n] }

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
		{"dashed process name", "/opt/homebrew/bin/cursor-agent chat", "cursor-agent", true},

		// Interpreter launching a tool script — comm would be "node", argv0 misses, path arg matches.
		{"node + script basename", "node /home/u/.local/share/codex/cli.js", "codex", true},
		{"node + path segment", "node /opt/lib/node_modules/opencode/dist/index.js serve", "opencode", true},
		{"bun + script", "bun /Users/x/.bun/install/global/node_modules/gemini/bin.js", "gemini", true},
		{"npx wrapper", "npx /tmp/cache/claude/cli.js", "claude", true},

		// False positives that MUST NOT match.
		{"tool as file arg to cat", "cat claude", "", false},
		{"tool as file arg to vim", "vim /Users/x/claude/notes.md", "", false},
		{"unrelated node app", "node /srv/app/server.js --port 3000", "", false},
		{"interpreter flag only", "node --inspect", "", false},
		{"substring not segment", "node /srv/apidocs/index.js", "", false}, // "pi" must not match inside "apidocs"
		{"empty", "", "", false},
		{"shell only", "-zsh", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := matchAIToolArgs(tt.args, isKnown)
			if ok != tt.wantOK {
				t.Fatalf("matchAIToolArgs(%q) ok = %v, want %v (tool=%q)", tt.args, ok, tt.wantOK, tool)
			}
			if ok && tool != tt.wantTool {
				t.Fatalf("matchAIToolArgs(%q) tool = %q, want %q", tt.args, tool, tt.wantTool)
			}
		})
	}
}
