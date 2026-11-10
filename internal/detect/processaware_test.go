package detect

import (
	"testing"
)

func TestDetectProcessAware(t *testing.T) {
	detect := detectProcessAware("factory", "droid")
	s := detect("/tmp/project", "12345")
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.Tool != "factory" {
		t.Errorf("Tool = %q, want factory", s.Tool)
	}
	if s.CWD != "/tmp/project" {
		t.Errorf("CWD = %q, want /tmp/project", s.CWD)
	}
	if s.Command != "droid" {
		t.Errorf("Command = %q, want droid", s.Command)
	}
}

func TestProcessAware_FillsDetect(t *testing.T) {
	entry := processAware(detector{Name: "rovo", Command: "acli rovodev run"})
	if entry.Detect == nil {
		t.Fatal("processAware left Detect nil")
	}
	if s := entry.Detect("/tmp/project", ""); s.Command != "acli rovodev run" {
		t.Errorf("Command = %q, want the launch command", s.Command)
	}

	entry = processAware(detector{Name: "aider"})
	if s := entry.Detect("/tmp/project", ""); s.Command != "aider" {
		t.Errorf("Command = %q, want the tool name when no command is set", s.Command)
	}
}
