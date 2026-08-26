package flatpak

import (
	"reflect"
	"testing"

	"github.com/padovanl/pkgtui/internal/pkg"
)

func TestParseSize(t *testing.T) {
	cases := map[string]int64{
		"1.2 MB":   1200000,
		"455 kB":   455000,
		"3.0 GB":   3000000000,
		"512 B":    512,
		"1.5 GiB":  1610612736,
		"?":        0,
		"":         0,
		"1,2 MB":   0, // LC_ALL=C is what keeps this shape from ever showing up
		"12 apple": 0,
	}
	for in, want := range cases {
		if got := parseSize(in); got != want {
			t.Errorf("parseSize(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseListOutput(t *testing.T) {
	out := "org.gimp.GIMP\t2.10.36\tGNU Image Manipulation Program\tflathub\t1.2 GB\n" +
		"org.videolan.VLC\t3.0.20\tVLC\tflathub\t455 MB\n"

	got := parseListOutput(out, map[string]bool{"org.videolan.VLC": true})

	want := map[string]pkg.Package{
		"org.gimp.GIMP": {
			Name:      "org.gimp.GIMP",
			Installed: "2.10.36",
			Summary:   "GNU Image Manipulation Program (flathub)",
			Size:      1200000000,
			Source:    "flatpak",
			Status:    pkg.StatusInstalled,
		},
		"org.videolan.VLC": {
			Name:      "org.videolan.VLC",
			Installed: "3.0.20",
			Summary:   "VLC (flathub)",
			Size:      455000000,
			Source:    "flatpak",
			Status:    pkg.StatusInstalled,
			Held:      true,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseListOutput() = %#v, want %#v", got, want)
	}
}

// flatpak only prints the column-title row when it thinks it's writing to a
// terminal, which never happens here — but a version that changed its mind
// about that shouldn't turn the header into a bogus package named
// "Application ID".
func TestParseListOutputSkipsHeader(t *testing.T) {
	out := "Application ID\tVersion\tName\tOrigin\tInstalled size\n" +
		"org.gimp.GIMP\t2.10.36\tGNU Image Manipulation Program\tflathub\t1.2 GB\n"

	got := parseListOutput(out, nil)

	if len(got) != 1 {
		t.Fatalf("parseListOutput() returned %d entries, want 1: %#v", len(got), got)
	}
	if _, ok := got["org.gimp.GIMP"]; !ok {
		t.Errorf("parseListOutput() = %#v, want the GIMP entry", got)
	}
}

func TestParseUpdatesOutput(t *testing.T) {
	installed := map[string]pkg.Package{
		"org.gimp.GIMP": {Name: "org.gimp.GIMP", Installed: "2.10.36", Summary: "GNU Image Manipulation Program (flathub)", Size: 1200000000, Source: "flatpak", Status: pkg.StatusInstalled},
	}
	out := "org.gimp.GIMP\t2.10.38\tflathub\n" +
		"org.videolan.VLC\t3.0.21\tflathub\n"

	got := parseUpdatesOutput(out, installed)

	want := []pkg.Package{
		{
			Name:      "org.gimp.GIMP",
			Version:   "2.10.38",
			Installed: "2.10.36",
			Summary:   "GNU Image Manipulation Program (flathub)",
			Size:      1200000000,
			Source:    "flatpak",
			Status:    pkg.StatusUpgradable,
		},
		{
			Name:    "org.videolan.VLC",
			Version: "3.0.21",
			Source:  "flatpak",
			Status:  pkg.StatusUpgradable,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseUpdatesOutput() = %#v, want %#v", got, want)
	}
}

func TestParseSearchOutput(t *testing.T) {
	installed := map[string]pkg.Package{
		"org.gimp.GIMP": {Name: "org.gimp.GIMP", Installed: "2.10.36", Size: 1200000000, Source: "flatpak", Status: pkg.StatusInstalled},
	}
	out := "org.gimp.GIMP\t2.10.38\tGNU Image Manipulation Program\tCreate images and edit photographs\tflathub\n" +
		"org.gimp.GIMP.Manual\t2.10\tGIMP User Manual\tOffline user manual for GIMP\tflathub\n"

	got := parseSearchOutput(out, installed)

	want := []pkg.Package{
		{
			Name:      "org.gimp.GIMP",
			Version:   "2.10.38",
			Installed: "2.10.36",
			Summary:   "GNU Image Manipulation Program — Create images and edit photographs",
			Size:      1200000000,
			Source:    "flatpak",
			Status:    pkg.StatusUpgradable,
		},
		{
			Name:    "org.gimp.GIMP.Manual",
			Version: "2.10",
			Summary: "GIMP User Manual — Offline user manual for GIMP",
			Source:  "flatpak",
			Status:  pkg.StatusAvailable,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSearchOutput() = %#v, want %#v", got, want)
	}
}

func TestParseMaskOutput(t *testing.T) {
	out := "org.videolan.VLC\napp/org.gimp.GIMP/x86_64/stable\nruntime/org.freedesktop.Platform*\n"

	got := parseMaskOutput(out)

	want := map[string]bool{
		"org.videolan.VLC":         true,
		"org.gimp.GIMP":            true,
		"org.freedesktop.Platform": true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseMaskOutput() = %#v, want %#v", got, want)
	}
}

// flatpak prints a plain-English line ("No masks configured") when nothing
// is masked, which must not turn into a mask entry of its own.
func TestParseMaskOutputIgnoresMessages(t *testing.T) {
	if got := parseMaskOutput("No masks configured\n"); len(got) != 0 {
		t.Errorf("parseMaskOutput() = %#v, want empty", got)
	}
}

// Nothing flatpak runs should ever be prefixed with sudo: it asks polkit
// for authorization itself, and running it as root would silently target
// root's own user installation instead.
func TestCommandsNeverUseSudo(t *testing.T) {
	m := New()
	argvs := [][]string{
		m.InstallCmd("org.gimp.GIMP"),
		m.RemoveCmd("org.gimp.GIMP"),
		m.UpgradeCmd(""),
		m.UpgradeCmd("org.gimp.GIMP"),
		m.UpdateCmd(),
		m.InstallManyCmd([]string{"a", "b"}),
		m.RemoveManyCmd([]string{"a", "b"}),
		m.HoldCmd("org.gimp.GIMP"),
		m.UnholdCmd("org.gimp.GIMP"),
	}
	for _, argv := range argvs {
		if len(argv) == 0 || argv[0] != "flatpak" {
			t.Errorf("argv = %v, want it to start with flatpak", argv)
		}
	}
}
