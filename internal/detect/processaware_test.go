package detect

import (
	"testing"
)

func TestDetectProcessAware(t *testing.T) {
	detect := detectProcessAware("mytool")
	s := detect("/tmp/project", "12345")
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.Tool != "mytool" {
		t.Errorf("Tool = %q, want mytool", s.Tool)
	}
	if s.CWD != "/tmp/project" {
		t.Errorf("CWD = %q, want /tmp/project", s.CWD)
	}
	if s.Command != "mytool" {
		t.Errorf("Command = %q, want mytool", s.Command)
	}
}
