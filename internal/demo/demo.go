// Package demo embeds the portable example layout that ships with crex.
// Every path in it is home-relative (~), so it restores on any machine —
// it's the layout the docs use for a safe first try of save/restore.
package demo

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed demo.toml
var layout []byte

// Content returns the embedded demo layout TOML.
func Content() []byte { return layout }

// Install writes the demo layout into layoutsDir as demo.toml, creating the
// directory if needed. It never overwrites an existing demo (the user may
// have edited or re-saved it). Returns whether the file was created.
func Install(layoutsDir string) (bool, error) {
	if err := os.MkdirAll(layoutsDir, 0o755); err != nil {
		return false, err
	}
	path := filepath.Join(layoutsDir, "demo.toml")
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	if err := os.WriteFile(path, layout, 0o644); err != nil {
		return false, err
	}
	return true, nil
}
