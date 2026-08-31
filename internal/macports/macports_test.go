package macports

import (
	"reflect"
	"testing"

	"github.com/padovanl/pkgtui/internal/pkg"
)

func TestParseInstalledOutput(t *testing.T) {
	// Without -q, "port installed" prefixes the listing with this header
	// line; it must not turn into a port of its own.
	out := "The following ports are currently installed:\n" +
		"  cmake @3.27.9_0\n" +
		"  cmake @3.28.1_0 (active)\n" +
		"  git @2.43.0_1 (active)\n"

	got := parseInstalledOutput(out)

	want := []installedEntry{
		{Name: "cmake", Version: "3.27.9_0"},
		{Name: "cmake", Version: "3.28.1_0", Active: true},
		{Name: "git", Version: "2.43.0_1", Active: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseInstalledOutput() = %#v, want %#v", got, want)
	}
}

func TestParseOutdatedOutput(t *testing.T) {
	out := "The following installed ports are outdated:\n" +
		"cmake                          3.28.1_0 < 3.29.0_0\n" +
		"git                            2.43.0_1 < 2.44.0_0\n"

	got := parseOutdatedOutput(out)

	want := []pkg.Package{
		{Name: "cmake", Installed: "3.28.1_0", Version: "3.29.0_0", Source: "macports", Status: pkg.StatusUpgradable},
		{Name: "git", Installed: "2.43.0_1", Version: "2.44.0_0", Source: "macports", Status: pkg.StatusUpgradable},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseOutdatedOutput() = %#v, want %#v", got, want)
	}
}

func TestParseSearchOutput(t *testing.T) {
	out := "vim @9.0.2000 (editors)\n" +
		"    Vi \"workalike\" with many additional features\n" +
		"neovim @0.9.5 (editors)\n" +
		"    Vim-fork focused on extensibility and usability\n" +
		"\n" +
		"Found 2 ports.\n"

	installed := map[string]pkg.Package{
		"vim": {Name: "vim", Installed: "9.0.1900_0", Source: "macports", Status: pkg.StatusInstalled},
	}

	got := parseSearchOutput(out, installed, map[string]bool{"vim": true})

	want := []pkg.Package{
		{
			Name:      "vim",
			Version:   "9.0.2000",
			Installed: "9.0.1900_0",
			Summary:   `Vi "workalike" with many additional features`,
			Source:    "macports",
			Status:    pkg.StatusUpgradable,
		},
		{
			Name:    "neovim",
			Version: "0.9.5",
			Summary: "Vim-fork focused on extensibility and usability",
			Source:  "macports",
			Status:  pkg.StatusAvailable,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSearchOutput() = %#v, want %#v", got, want)
	}
}

func TestParseEchoOutput(t *testing.T) {
	out := "libidn2 @2.3.4_0\nlibunistring @1.1_0\n"

	got := parseEchoOutput(out)

	want := []string{"libidn2", "libunistring"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseEchoOutput() = %#v, want %#v", got, want)
	}
}

func TestInactiveDiskItems(t *testing.T) {
	entries := []installedEntry{
		{Name: "cmake", Version: "3.27.9_0"},
		{Name: "cmake", Version: "3.28.1_0", Active: true},
		{Name: "git", Version: "2.42.0_0"},
	}

	got := inactiveDiskItems(entries)

	if len(got) != 2 {
		t.Fatalf("inactiveDiskItems() returned %d items, want 2: %#v", len(got), got)
	}
	if got[0].Name != "cmake @3.27.9_0" {
		t.Errorf("first item name = %q, want %q", got[0].Name, "cmake @3.27.9_0")
	}
	// The version has to reach "port uninstall" as a separate "@version"
	// argument: that's how MacPorts addresses one specific installed
	// version rather than every version of the port.
	want := []string{"port", "uninstall", "cmake", "@3.27.9_0"}
	argv := got[0].Argv
	if len(argv) > 0 && argv[0] == "sudo" {
		argv = argv[1:]
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("first item argv = %v, want %v", argv, want)
	}
}

func TestParseDependentsOutput(t *testing.T) {
	out := "The following ports are dependent on libidn2:\n" +
		"  curl\n" +
		"  wget\n"

	got := parseDependentsOutput(out)

	want := []string{"curl", "wget"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDependentsOutput() = %#v, want %#v", got, want)
	}
}

// "port dependents" says so in a sentence when there are none, and that
// sentence must not be read as a dependent named "vim".
func TestParseDependentsOutputNone(t *testing.T) {
	if got := parseDependentsOutput("vim has no dependents.\n"); got != nil {
		t.Errorf("parseDependentsOutput() = %#v, want nil", got)
	}
}
