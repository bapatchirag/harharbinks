// Package config loads and persists harharbinks' small on-disk user settings so
// that preferences chosen in the interactive TUI survive across runs. Today it
// stores only the selected theme, but the JSON document is a struct so new
// fields can be added without changing the file format.
//
// Loading is deliberately best-effort: a missing, unreadable, or malformed
// config yields the built-in defaults (via Default), so a bad or absent config
// file can never stop harharbinks from starting. The file always lives at
// ~/.config/hhb/config.json — a fixed, XDG-style path under the user's home
// directory — on every platform, so the location is predictable regardless of OS.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// configRoot is the directory beneath the user's home that holds per-application
// configuration. harharbinks uses a fixed ~/.config path on every OS (rather than
// os.UserConfigDir) so the config location is identical across macOS, Linux, and
// Windows.
const configRoot = ".config"

// appDir is the harharbinks-specific subdirectory under configRoot; it matches
// the installed binary name, "hhb".
const appDir = "hhb"

// fileName is the config file's base name within appDir.
const fileName = "config.json"

// Config is the persisted user configuration. Load fills every field, defaulting
// any that are absent from the file, so callers can read fields directly without
// nil/empty checks.
type Config struct {
	// Theme is the Name of the palette applied across the interface (e.g.
	// "harharbinks-gruvbox"). It defaults to the built-in default palette.
	Theme string `json:"theme"`
}

// Default returns the configuration with every field set to its built-in
// default. It is the base that Load merges the on-disk file onto: any field
// missing from the file (including one introduced in a newer release) takes its
// value from here, and Ensure writes these defaults out for a fresh install.
func Default() Config {
	return Config{
		Theme: theme.Default().Name,
	}
}

// Path returns the absolute path to the config file, ~/.config/hhb/config.json,
// without creating anything. It resolves the user's home directory with the
// cross-platform os.UserHomeDir and joins the fixed ~/.config/hhb location; the
// directory itself is created lazily by Save/Ensure. On Windows this resolves
// under the user profile (e.g. C:\Users\<name>\.config\hhb\config.json).
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configRoot, appDir, fileName), nil
}

// Load reads the config file and merges it onto Default: it starts from the
// built-in defaults, then overwrites the fields present in the file. It is
// best-effort: if the path cannot be resolved, the file is missing or
// unreadable, or its contents are not valid JSON, Load returns the full set of
// defaults so startup always proceeds rather than failing over configuration.
func Load() Config {
	c := Default()
	path, err := Path()
	if err != nil {
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return Default()
	}
	return c
}

// Ensure guarantees a config file exists and is current, returning the effective
// configuration. It loads the file (falling back to defaults for anything
// missing) and writes the result straight back, which creates the file for a
// fresh install and rewrites it so any newly added fields are persisted with
// their defaults. It is idempotent and safe to call on every launch. Any write
// error is returned but callers may treat persistence as best-effort.
func Ensure() (Config, error) {
	c := Load()
	if err := Save(c); err != nil {
		return c, err
	}
	return c, nil
}

// Save writes c to the config file as indented JSON, creating the parent
// directory if needed. It returns any error so callers may report a persistence
// failure, though harharbinks treats saving as best-effort and keeps running
// regardless of the outcome.
func Save(c Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
