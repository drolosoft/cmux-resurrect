package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/config"
)

func TestSettingsRestoreMode_SetPersistsAndGet(t *testing.T) {
	setupTestConfig(t)
	cfgFile = filepath.Join(t.TempDir(), "config.toml")
	t.Cleanup(func() { cfgFile = "" })

	// set replace
	if err := runRestoreModeSet(settingsRestoreModeSetCmd, []string{"replace"}); err != nil {
		t.Fatalf("set replace: %v", err)
	}
	if cfg.RestoreMode != "replace" {
		t.Fatalf("cfg.RestoreMode = %q, want replace", cfg.RestoreMode)
	}

	// persisted to disk so CLI restore + TUI pick it up
	loaded, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.RestoreMode != "replace" {
		t.Fatalf("persisted RestoreMode = %q, want replace", loaded.RestoreMode)
	}

	// get reflects the new value
	buf := new(bytes.Buffer)
	settingsRestoreModeGetCmd.SetOut(buf)
	t.Cleanup(func() { settingsRestoreModeGetCmd.SetOut(nil) })
	if err := runRestoreModeGet(settingsRestoreModeGetCmd, nil); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(buf.String(), "replace") {
		t.Fatalf("get output %q does not contain the current mode", buf.String())
	}
}

func TestSettingsRestoreMode_InvalidRejected(t *testing.T) {
	setupTestConfig(t)
	cfgFile = filepath.Join(t.TempDir(), "config.toml")
	t.Cleanup(func() { cfgFile = "" })

	if err := runRestoreModeSet(settingsRestoreModeSetCmd, []string{"bogus"}); err == nil {
		t.Fatal("expected error for invalid mode, got nil")
	}
	if cfg.RestoreMode == "bogus" {
		t.Fatal("invalid mode must not be stored")
	}
}

func TestSettingsRestoreMode_GetDefaultsToAsk(t *testing.T) {
	setupTestConfig(t)
	cfg.RestoreMode = "" // unset

	buf := new(bytes.Buffer)
	settingsRestoreModeGetCmd.SetOut(buf)
	t.Cleanup(func() { settingsRestoreModeGetCmd.SetOut(nil) })
	if err := runRestoreModeGet(settingsRestoreModeGetCmd, nil); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(buf.String(), "ask") {
		t.Fatalf("get output %q should default to 'ask'", buf.String())
	}
}

func TestSettingsAndNow_Registered(t *testing.T) {
	var hasSettings, hasNow bool
	for _, c := range rootCmd.Commands() {
		switch c.Name() {
		case "settings":
			hasSettings = true
		case "now":
			hasNow = true
		}
	}
	if !hasSettings {
		t.Error("settings command not registered on root")
	}
	if !hasNow {
		t.Error("now command not registered on root")
	}
}
