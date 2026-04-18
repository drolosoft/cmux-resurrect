package orchestrate

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDefaultPIDPath(t *testing.T) {
	got := DefaultPIDPath()
	if !strings.HasSuffix(got, filepath.Join(".config", "crex", "crex.pid")) {
		t.Errorf("DefaultPIDPath() = %q, want suffix .config/crex/crex.pid", got)
	}
}

func TestDefaultLogPath(t *testing.T) {
	got := DefaultLogPath()
	if !strings.HasSuffix(got, filepath.Join(".config", "crex", "watch.log")) {
		t.Errorf("DefaultLogPath() = %q, want suffix .config/crex/watch.log", got)
	}
}

func TestWritePIDFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "crex.pid")

	if err := WritePIDFile(path, 12345); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after write: %v", err)
	}

	got, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse PID: %v", err)
	}
	if got != 12345 {
		t.Errorf("written PID = %d, want 12345", got)
	}
}

func TestReadPIDFile(t *testing.T) {
	t.Run("missing file returns error", func(t *testing.T) {
		_, err := ReadPIDFile("/nonexistent/path/crex.pid")
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
	})

	t.Run("valid file returns PID", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "crex.pid")

		if err := WritePIDFile(path, 12345); err != nil {
			t.Fatalf("WritePIDFile: %v", err)
		}

		pid, err := ReadPIDFile(path)
		if err != nil {
			t.Fatalf("ReadPIDFile: %v", err)
		}
		if pid != 12345 {
			t.Errorf("ReadPIDFile() = %d, want 12345", pid)
		}
	})
}

func TestRemovePIDFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crex.pid")

	if err := WritePIDFile(path, 99); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	RemovePIDFile(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted, got: %v", err)
	}
}

func TestOpenLogWriter_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watch.log")

	w, err := OpenLogWriter(path, 1024*1024)
	if err != nil {
		t.Fatalf("OpenLogWriter: %v", err)
	}
	defer func() { _ = w.Close() }()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("log file should be created")
	}
}

func TestOpenLogWriter_RotatesWhenLarge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watch.log")

	// Write a file larger than the threshold.
	bigData := make([]byte, 100)
	if err := os.WriteFile(path, bigData, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	w, err := OpenLogWriter(path, 50) // threshold = 50 bytes
	if err != nil {
		t.Fatalf("OpenLogWriter: %v", err)
	}
	defer func() { _ = w.Close() }()

	// Old file should be renamed to .old
	oldPath := path + ".old"
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		t.Error("old log file should exist after rotation")
	}
}

func TestDaemonRunning_NotRunning(t *testing.T) {
	t.Run("no PID file means not running", func(t *testing.T) {
		running, pid := IsDaemonRunning("/nonexistent/path/crex.pid")
		if running {
			t.Errorf("expected not running for missing PID file, got running with pid=%d", pid)
		}
		if pid != 0 {
			t.Errorf("expected pid=0 for missing file, got %d", pid)
		}
	})

	t.Run("stale PID means not running", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "crex.pid")

		// PID 99999999 is virtually guaranteed to not exist
		const stalePID = 99999999
		if err := WritePIDFile(path, stalePID); err != nil {
			t.Fatalf("WritePIDFile: %v", err)
		}

		running, pid := IsDaemonRunning(path)
		if running {
			t.Errorf("expected stale PID %d to be not running", stalePID)
		}
		if pid != stalePID {
			t.Errorf("expected pid=%d, got %d", stalePID, pid)
		}
	})
}
