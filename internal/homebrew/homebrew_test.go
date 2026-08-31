package homebrew

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/padovanl/pkgtui/internal/pkg"
)

func TestParseListVersionsOutput(t *testing.T) {
	// "git 2.43.0 2.43.1" is a keg with two versions installed side by
	// side, which brew lists on one line, oldest first.
	out := "cmake 3.28.1\ngit 2.43.0 2.43.1\nfirefox 121.0\n"

	got := parseListVersionsOutput(out, map[string]bool{"cmake": true})

	want := map[string]pkg.Package{
		"cmake":   {Name: "cmake", Installed: "3.28.1", Source: "brew", Status: pkg.StatusInstalled, Held: true},
		"git":     {Name: "git", Installed: "2.43.1", Source: "brew", Status: pkg.StatusInstalled},
		"firefox": {Name: "firefox", Installed: "121.0", Source: "brew", Status: pkg.StatusInstalled},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseListVersionsOutput() = %#v, want %#v", got, want)
	}
}

func TestParseOutdatedJSON(t *testing.T) {
	data := `{
	  "formulae": [
	    {"name": "cmake", "installed_versions": ["3.28.1"], "current_version": "3.29.0", "pinned": false},
	    {"name": "git", "installed_versions": ["2.43.0", "2.43.1"], "current_version": "2.44.0", "pinned": true}
	  ],
	  "casks": [
	    {"name": "firefox", "installed_versions": ["121.0"], "current_version": "122.0"}
	  ]
	}`

	got, err := parseOutdatedJSON(data)
	if err != nil {
		t.Fatalf("parseOutdatedJSON() error: %v", err)
	}

	want := []pkg.Package{
		{Name: "cmake", Version: "3.29.0", Installed: "3.28.1", Source: "brew", Status: pkg.StatusUpgradable},
		{Name: "firefox", Version: "122.0", Installed: "121.0", Source: "brew", Status: pkg.StatusUpgradable},
		{Name: "git", Version: "2.44.0", Installed: "2.43.1", Source: "brew", Status: pkg.StatusUpgradable, Held: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseOutdatedJSON() = %#v, want %#v", got, want)
	}
}

// Nothing outdated is the common case, and brew still prints valid JSON for
// it — an empty result, not an error.
func TestParseOutdatedJSONEmpty(t *testing.T) {
	got, err := parseOutdatedJSON(`{"formulae": [], "casks": []}`)
	if err != nil {
		t.Fatalf("parseOutdatedJSON() error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("parseOutdatedJSON() = %#v, want empty", got)
	}
}

func TestParseSearchOutput(t *testing.T) {
	out := "==> Formulae\ncmake\ncmake-docs\n\n==> Casks\ncmake-app\n"

	installed := map[string]pkg.Package{
		"cmake": {Name: "cmake", Installed: "3.28.1", Size: 4096, Source: "brew", Status: pkg.StatusInstalled},
	}
	outdated := map[string]pkg.Package{
		"cmake": {Name: "cmake", Version: "3.29.0", Source: "brew", Status: pkg.StatusUpgradable},
	}

	got := parseSearchOutput(out, installed, outdated)

	want := []pkg.Package{
		{Name: "cmake", Version: "3.29.0", Installed: "3.28.1", Summary: "formula", Size: 4096, Source: "brew", Status: pkg.StatusUpgradable},
		{Name: "cmake-docs", Summary: "formula", Source: "brew", Status: pkg.StatusAvailable},
		{Name: "cmake-app", Summary: "cask", Source: "brew", Status: pkg.StatusAvailable},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSearchOutput() = %#v, want %#v", got, want)
	}
}

func TestParseAutoremoveOutput(t *testing.T) {
	out := "==> Would autoremove 2 unneeded formulae:\nlibidn2\nlibunistring\n"

	got := parseAutoremoveOutput(out)

	want := []string{"libidn2", "libunistring"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseAutoremoveOutput() = %#v, want %#v", got, want)
	}
}

func TestParseCleanupOutput(t *testing.T) {
	out := "Would remove: /Users/me/Library/Caches/Homebrew/cmake--3.28.1.tar.gz (10.2MB)\n" +
		"Would remove: /opt/homebrew/Cellar/git/2.43.0 (1,543 files, 52.1MB)\n" +
		"==> This operation would free approximately 62.3MB of disk space.\n"

	count, total := parseCleanupOutput(out)

	if count != 2 {
		t.Errorf("parseCleanupOutput() count = %d, want 2", count)
	}
	// brew's own "MB" is 2^20, so 62.3MB is 62.3 * 1048576 truncated.
	if want := int64(65326284); total != want {
		t.Errorf("parseCleanupOutput() total = %d, want %d", total, want)
	}
}

// With nothing to clean up brew prints only its closing line, and a report
// with no items behind it would be a row promising space it can't reclaim.
func TestParseCleanupOutputNothingToDo(t *testing.T) {
	count, total := parseCleanupOutput("==> This operation would free approximately 0B of disk space.\n")
	if count != 0 || total != 0 {
		t.Errorf("parseCleanupOutput() = (%d, %d), want (0, 0)", count, total)
	}
}

func TestParseInstalledOnRequest(t *testing.T) {
	onRequest := `{"formulae": [{"installed": [{"installed_on_request": true}]}], "casks": []}`
	asDependency := `{"formulae": [{"installed": [{"installed_on_request": false}]}], "casks": []}`

	if !parseInstalledOnRequest(onRequest) {
		t.Error("parseInstalledOnRequest(on request) = false, want true")
	}
	if parseInstalledOnRequest(asDependency) {
		t.Error("parseInstalledOnRequest(as dependency) = true, want false")
	}
	if parseInstalledOnRequest("not json") {
		t.Error("parseInstalledOnRequest(garbage) = true, want false")
	}
}

func TestDirSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "tool"), make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), make([]byte, 512), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := dirSize(dir); got != 1536 {
		t.Errorf("dirSize() = %d, want 1536", got)
	}
	if got := dirSize(filepath.Join(dir, "nope")); got != 0 {
		t.Errorf("dirSize(missing) = %d, want 0", got)
	}
}

// Homebrew refuses to run as root, so no command it produces may ever be
// prefixed with sudo.
func TestCommandsNeverUseSudo(t *testing.T) {
	m := New()
	argvs := [][]string{
		m.InstallCmd("cmake"),
		m.RemoveCmd("cmake"),
		m.UpgradeCmd(""),
		m.UpgradeCmd("cmake"),
		m.UpdateCmd(),
		m.InstallManyCmd([]string{"a", "b"}),
		m.RemoveManyCmd([]string{"a", "b"}),
		m.HoldCmd("cmake"),
		m.UnholdCmd("cmake"),
	}
	for _, argv := range argvs {
		if len(argv) == 0 || argv[0] != "brew" {
			t.Errorf("argv = %v, want it to start with brew", argv)
		}
	}
}
