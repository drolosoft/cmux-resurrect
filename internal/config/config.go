package config

import (
	"os"
	"path/filepath"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// Config holds global configuration for cmux-persist.
type Config struct {
	LayoutsDir    string        `toml:"layouts_dir"`
	WatchInterval time.Duration `toml:"-"`
	WatchIntervalStr string    `toml:"watch_interval"`
	MaxAutosaves  int           `toml:"max_autosaves"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		LayoutsDir:       DefaultLayoutsDir(),
		WatchInterval:    5 * time.Minute,
		WatchIntervalStr: "5m",
		MaxAutosaves:     10,
	}
}

// DefaultLayoutsDir returns ~/.config/cmux-persist/layouts.
func DefaultLayoutsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cmux-persist", "layouts")
}

// DefaultConfigPath returns ~/.config/cmux-persist/config.toml.
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cmux-persist", "config.toml")
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
	return cfg, nil
}
