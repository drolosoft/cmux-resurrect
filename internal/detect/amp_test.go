package detect

import (
	"testing"
)

func TestAmpThreadCache_SeedAndLookup(t *testing.T) {
	resetAmpCache()
	seedAmpThread("17752", "T-aaaa-bbbb")
	if got := ampCache.threadFor("17752"); got != "T-aaaa-bbbb" {
		t.Errorf("threadFor known pid = %q", got)
	}
	if got := ampCache.threadFor("99999"); got != "" {
		t.Errorf("threadFor unknown pid = %q", got)
	}
}

func TestResetAmpCache(t *testing.T) {
	seedAmpThread("123", "T-y")
	resetAmpCache()
	if ampCache.threadFor("123") != "" {
		t.Error("expected empty after reset")
	}
}

func TestDetectAmp_NoPID(t *testing.T) {
	if s := detectAmp("/some/path", ""); s != nil {
		t.Error("expected nil when pid is empty")
	}
}

func TestDetectAmp_UnseededPID(t *testing.T) {
	resetAmpCache()
	if s := detectAmp("/some/path", "12345"); s != nil {
		t.Error("expected nil when pid not in cache")
	}
}

func TestThreadIDFromLogPath(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{
			"/Users/me/.cache/amp/logs/threads/T-019e4c62-93b2-7359-9148-2ecd027fcda0.log",
			"T-019e4c62-93b2-7359-9148-2ecd027fcda0",
		},
		{"/Users/me/.cache/amp/logs/cli.log", ""},                  // not in threads/
		{"/Users/me/.cache/amp/logs/threads/foo.txt", ""},          // wrong suffix
		{"/Users/me/.cache/amp/logs/threads/not-a-thread.log", ""}, // missing T- prefix
		{"/some/nested/path/logs/threads/T-019e4c62-aaaa-bbbb-cccc-ddddeeeeffff.log", "T-019e4c62-aaaa-bbbb-cccc-ddddeeeeffff"},
		{"", ""},
	}
	for _, c := range cases {
		if got := threadIDFromLogPath(c.input); got != c.want {
			t.Errorf("threadIDFromLogPath(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
