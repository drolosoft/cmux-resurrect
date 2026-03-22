package config

import (
	"os"
	"path/filepath"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// Config holds global configuration for cmres.
type Config struct {
	LayoutsDir       string        `toml:"layouts_dir"`
	WorkspaceFile    string        `toml:"workspace_file"`
	WatchInterval    time.Duration `toml:"-"`
	WatchIntervalStr string        `toml:"watch_interval"`
	MaxAutosaves     int           `toml:"max_autosaves"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		LayoutsDir:       DefaultLayoutsDir(),
		WorkspaceFile:    DefaultWorkspaceFile(),
		WatchInterval:    5 * time.Minute,
		WatchIntervalStr: "5m",
		MaxAutosaves:     10,
	}
}

// DefaultLayoutsDir returns ~/.config/cmres/layouts.
func DefaultLayoutsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cmres", "layouts")
}

// DefaultConfigPath returns ~/.config/cmres/config.toml.
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cmres", "config.toml")
}

// DefaultWorkspaceFile returns ~/.config/cmres/workspaces.md.
func DefaultWorkspaceFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cmres", "workspaces.md")
}

// ExpandHome expands ~ to the user's home directory.
func ExpandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

// Load reads config from a TOML file, falling back to defaults.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.WatchIntervalStr != "" {
		if d, err := time.ParseDuration(cfg.WatchIntervalStr); err == nil {
			cfg.WatchInterval = d
		}
	}
	// Expand ~ in paths.
	cfg.WorkspaceFile = ExpandHome(cfg.WorkspaceFile)
	cfg.LayoutsDir = ExpandHome(cfg.LayoutsDir)
	return cfg, nil
}
