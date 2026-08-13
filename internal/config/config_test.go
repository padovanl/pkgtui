package config

import "testing"

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	c := &Config{
		Theme:       "nord",
		Keybindings: map[string]string{"install": "x"},
		LastBackend: "snap",
		LastView:    map[string]string{"apt": "upgradable"},
	}
	if err := c.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Theme != "nord" || got.LastBackend != "snap" {
		t.Errorf("Load() = %+v, want theme=nord last_backend=snap", got)
	}
	if got.Keybindings["install"] != "x" {
		t.Errorf("Keybindings[install] = %q, want %q", got.Keybindings["install"], "x")
	}
	if got.LastView["apt"] != "upgradable" {
		t.Errorf("LastView[apt] = %q, want %q", got.LastView["apt"], "upgradable")
	}
}

func TestLoadMissingFileReturnsZeroValue(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Theme != "" || len(got.Keybindings) != 0 {
		t.Errorf("Load() on missing file = %+v, want zero value", got)
	}
}
