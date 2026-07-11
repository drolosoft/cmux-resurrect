package config

import (
	"os"
	"path/filepath"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// Config holds global configuration for crex.
type Config struct {
	LayoutsDir        string        `toml:"layouts_dir"`
	WorkspaceFile     string        `toml:"workspace_file"`
	WatchInterval     time.Duration `toml:"-"`
	WatchIntervalStr  string        `toml:"watch_interval"`
	MaxAutosaves      int           `toml:"max_autosaves"` // TODO: not yet enforced — rotation not implemented
	BannerStyle       string        `toml:"banner_style"`  // "flame" (default), "classic", "plain"
	RestoreMode       string        `toml:"restore_mode"`  // "ask" (default when empty), "replace", "add"
	StableDuration    time.Duration `toml:"-"`
	StableDurationStr string        `toml:"stable_duration"`
	AutoAccept        []string      `toml:"auto_accept"`
	// Backend pins the default terminal backend when auto-detection is
	// ambiguous (cmux AND Ghostty both running). Empty means auto-detect.
	// Values: "cmux", "ghostty", "cmux-applescript". CREX_BACKEND overrides it.
	Backend string `toml:"backend,omitempty"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		LayoutsDir:        DefaultLayoutsDir(),
		WorkspaceFile:     DefaultWorkspaceFile(),
		WatchInterval:     5 * time.Minute,
		WatchIntervalStr:  "5m",
		MaxAutosaves:      10,
		StableDuration:    3 * time.Second,
		StableDurationStr: "3s",
	}
}

// DefaultLayoutsDir returns ~/.config/crex/layouts.
func DefaultLayoutsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "crex", "layouts")
}

// DefaultConfigPath returns ~/.config/crex/config.toml.
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "crex", "config.toml")
}

// DefaultWorkspaceFile returns ~/.config/crex/workspaces.md.
func DefaultWorkspaceFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "crex", "workspaces.md")
}

// ExpandHome expands ~ to the user's home directory.
func ExpandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

// Save writes the config to a TOML file, creating parent dirs if needed.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
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
	if cfg.StableDurationStr != "" {
		if d, err := time.ParseDuration(cfg.StableDurationStr); err == nil {
			cfg.StableDuration = d
		}
	}
	// Expand ~ in paths.
	cfg.WorkspaceFile = ExpandHome(cfg.WorkspaceFile)
	cfg.LayoutsDir = ExpandHome(cfg.LayoutsDir)
	return cfg, nil
}
