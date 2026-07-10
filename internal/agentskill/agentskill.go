// Package agentskill embeds and installs the crex agent skill — a reference
// document that teaches AI coding agents (Claude Code, Codex, and compatible
// tools) how to drive crex: save/restore semantics, safe scripting flags,
// AI-session resume, and programmatic layout queries.
package agentskill

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed SKILL.md
var skill string

// Content returns the embedded SKILL.md.
func Content() string { return skill }

// Install writes the skill to <baseDir>/crex/SKILL.md, creating directories
// as needed and refreshing any existing copy (the embedded version is the
// source of truth). It returns the written path.
func Install(baseDir string) (string, error) {
	dir := filepath.Join(baseDir, "crex")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(skill), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ClaudeDir returns Claude Code's personal skills directory.
func ClaudeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "skills")
}

// CodexDir returns the Codex-compatible agents skills directory.
func CodexDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".agents", "skills")
}
