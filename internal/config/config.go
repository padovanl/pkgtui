// Package config handles pkgtui's persisted settings: theme choice,
// keybinding overrides and last-used view, stored as JSON under the user's
// config directory so the in-app settings screen can edit them and have
// changes survive a restart.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is the on-disk settings file.
type Config struct {
	Theme       string            `json:"theme,omitempty"`
	Keybindings map[string]string `json:"keybindings,omitempty"` // action name -> key
	LastBackend string            `json:"last_backend,omitempty"`
	LastView    map[string]string `json:"last_view,omitempty"` // backend -> view name
}

// Dir returns the directory pkgtui's config file lives in, without
// creating it.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pkgtui"), nil
}

func path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config file, returning a zero-value Config (not an error)
// if it doesn't exist yet.
func Load() (*Config, error) {
	p, err := path()
	if err != nil {
		return &Config{}, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return &Config{}, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return &Config{}, err
	}
	return &c, nil
}

// Save writes the config file, creating its directory if needed.
func (c *Config) Save() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	p, err := path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
