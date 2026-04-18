package orchestrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// DefaultPIDPath returns the default path for the daemon PID file.
func DefaultPIDPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "crex", "crex.pid")
}

// DefaultLogPath returns the default path for the watch daemon log file.
func DefaultLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "crex", "watch.log")
}

// WritePIDFile writes the given PID to path, creating parent directories as needed.
func WritePIDFile(path string, pid int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create pid dir: %w", err)
	}
	data := []byte(strconv.Itoa(pid) + "\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	return nil
}

// ReadPIDFile reads and parses the PID stored in path.
func ReadPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid: %w", err)
	}
	return pid, nil
}

// RemovePIDFile deletes the PID file at path. Errors are silently ignored.
func RemovePIDFile(path string) {
	_ = os.Remove(path)
}

// OpenLogWriter opens a log file for writing, rotating if it exceeds maxBytes.
// The old log is renamed to path + ".old" (only one backup is kept).
func OpenLogWriter(path string, maxBytes int64) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	// Check if rotation is needed.
	if info, err := os.Stat(path); err == nil && info.Size() >= maxBytes {
		oldPath := path + ".old"
		_ = os.Remove(oldPath)
		_ = os.Rename(path, oldPath)
	}

	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

// IsDaemonRunning checks whether the process recorded in pidPath is alive.
// It returns (true, pid) if running, or (false, pid) if the file exists but
// the process is gone, or (false, 0) if the file cannot be read.
func IsDaemonRunning(pidPath string) (bool, int) {
	pid, err := ReadPIDFile(pidPath)
	if err != nil {
		return false, 0
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, pid
	}

	// On Unix, os.FindProcess always succeeds; signal 0 is the real liveness check.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, pid
	}
	return true, pid
}
