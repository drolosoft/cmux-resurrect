package detect

import "testing"

func TestParseForegroundCommand(t *testing.T) {
	tests := []struct {
		name  string
		psOut string
		want  string
	}{
		{
			name: "plain zsh shell",
			psOut: `  PID  PPID STAT ARGS
 8875   823 Ss   /usr/bin/login
 8876  8875 S+   -/bin/zsh`,
			want: "",
		},
		{
			name: "claude running",
			psOut: `  PID  PPID STAT ARGS
 8875   823 Ss   /usr/bin/login
 8876  8875 S    -/bin/zsh
45032  8876 S+   claude
45358 45032 S+   /usr/bin/python3 -m server`,
			want: "claude",
		},
		{
			name: "htop running",
			psOut: `  PID  PPID STAT ARGS
 1234   823 Ss   /usr/bin/login
 1235  1234 S    -/bin/zsh
 5678  1235 S+   htop`,
			want: "htop",
		},
		{
			name: "vim with file",
			psOut: `  PID  PPID STAT ARGS
 1234   823 Ss   /usr/bin/login
 1235  1234 S    -/bin/zsh
 5678  1235 S+   vim main.go`,
			want: "vim main.go",
		},
		{
			name: "make watch",
			psOut: `  PID  PPID STAT ARGS
 1234   823 Ss   /usr/bin/login
 1235  1234 S    -/bin/bash
 5678  1235 S+   make watch
 5679  5678 S+   go run ./cmd/server`,
			want: "make watch",
		},
		{
			name: "npm run dev via node wrapper",
			psOut: `  PID  PPID STAT ARGS
 1234   823 Ss   /usr/bin/login
 1235  1234 S    -/bin/zsh
 5678  1235 S+   node /usr/local/bin/npm run dev
 5679  5678 S+   node server.js`,
			want: "npm run dev",
		},
		{
			name: "bash shell only",
			psOut: `  PID  PPID STAT ARGS
 1234   823 Ss   /usr/bin/login
 1235  1234 S+   -/bin/bash`,
			want: "",
		},
		{
			name: "fish shell only",
			psOut: `  PID  PPID STAT ARGS
 1234   823 Ss   /usr/bin/login
 1235  1234 S+   /opt/homebrew/bin/fish`,
			want: "",
		},
		{
			name:  "empty output",
			psOut: "",
			want:  "",
		},
		{
			name: "subshell zsh not foreground, claude is",
			psOut: `  PID  PPID STAT ARGS
 8875   823 Ss   /usr/bin/login
 8876  8875 S    -/bin/zsh
45030  8876 SN   -/bin/zsh
45032  8876 S+   claude`,
			want: "claude",
		},
		{
			name: "npm via absolute nvm path",
			psOut: `  PID  PPID STAT ARGS
 1234   823 Ss   /usr/bin/login
 1235  1234 S    -/bin/zsh
 5678  1235 S+   /Users/me/.nvm/versions/node/v23/bin/node /Users/me/.nvm/versions/node/v23/bin/npm run dev`,
			want: "npm run dev",
		},
		{
			name: "go test with flags",
			psOut: `  PID  PPID STAT ARGS
 1234   823 Ss   /usr/bin/login
 1235  1234 S    -/bin/zsh
 5678  1235 S+   go test ./... -v`,
			want: "go test ./... -v",
		},
		{
			name: "long command truncated at 80",
			psOut: `  PID  PPID STAT ARGS
 1234   823 Ss   /usr/bin/login
 1235  1234 S    -/bin/zsh
 5678  1235 S+   python3 /very/long/path/to/some/script.py --arg1 value1 --arg2 value2 --arg3 value3 --verbose --debug`,
			want: "python3 /very/long/path/to/some/script.py --arg1 value1 --arg2 value2 --arg3...",
		},
		{
			name: "caffeinate with flag",
			psOut: `  PID  PPID STAT ARGS
 1234   823 Ss   /usr/bin/login
 1235  1234 S    -/bin/zsh
 5678  1235 S+   caffeinate -d`,
			want: "caffeinate -d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseForegroundCommand(tt.psOut)
			if got != tt.want {
				t.Errorf("parseForegroundCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanCommand(t *testing.T) {
	tests := []struct {
		args string
		want string
	}{
		{"htop", "htop"},
		{"/usr/bin/vim main.go", "vim main.go"},
		{"node /usr/local/bin/npm run dev", "npm run dev"},
		{"claude", "claude"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.args, func(t *testing.T) {
			got := cleanCommand(tt.args)
			if got != tt.want {
				t.Errorf("cleanCommand(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestForegroundCommand_EmptyTTY(t *testing.T) {
	got := ForegroundCommand("")
	if got != "" {
		t.Errorf("ForegroundCommand(\"\") = %q, want \"\"", got)
	}
}

func TestForegroundLeaderPID(t *testing.T) {
	tests := []struct {
		name  string
		psOut string
		want  string
	}{
		{
			name: "running command under shell → command pid",
			psOut: `  PID  PPID STAT ARGS
 8875   823 Ss   /usr/bin/login
 8876  8875 Ss   -/bin/zsh
 9001  8876 S+   node /Users/x/.nvm/bin/claude --resume abc`,
			want: "9001",
		},
		{
			name: "plain shell prompt → shell pid",
			psOut: `  PID  PPID STAT ARGS
 8875   823 Ss   /usr/bin/login
 8876  8875 S+   -/bin/zsh`,
			want: "8876",
		},
		{
			name:  "no processes",
			psOut: `  PID  PPID STAT ARGS`,
			want:  "",
		},
		{
			name: "login only (no shell)",
			psOut: `  PID  PPID STAT ARGS
 8875   823 Ss   /usr/bin/login`,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := foregroundLeaderPID(tt.psOut); got != tt.want {
				t.Fatalf("foregroundLeaderPID() = %q, want %q", got, tt.want)
			}
		})
	}
}
