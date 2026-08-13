package snap

import (
	"reflect"
	"testing"

	"github.com/padovanl/pkgtui/internal/pkg"
)

func TestParseListOutput(t *testing.T) {
	out := "Name                       Version          Rev    Tracking       Publisher   Notes\n" +
		"bare                       1.0              5      latest/stable  canonical✓  base\n" +
		"core22                     20260225         1612   latest/stable  canonical✓  base\n" +
		"snapd                      2.63             21759  latest/stable  canonical✓  snapd\n"

	got := parseListOutput(out)

	want := map[string]pkg.Package{
		"bare":   {Name: "bare", Installed: "1.0", Source: "snap", Status: pkg.StatusInstalled},
		"core22": {Name: "core22", Installed: "20260225", Source: "snap", Status: pkg.StatusInstalled},
		"snapd":  {Name: "snapd", Installed: "2.63", Source: "snap", Status: pkg.StatusInstalled},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseListOutput() = %#v, want %#v", got, want)
	}
}

func TestParseFindOutput(t *testing.T) {
	out := "Name             Version    Publisher      Notes    Summary\n" +
		"hello            2.10       canonical✓     -        GNU Hello, the \"hello world\" snap\n" +
		"hello-world      6.4        canonical✓     -        The 'hello-world' of snaps\n"

	installed := map[string]pkg.Package{
		"hello": {Name: "hello", Installed: "2.9", Source: "snap", Status: pkg.StatusInstalled},
	}

	got := parseFindOutput(out, installed)

	want := []pkg.Package{
		{Name: "hello", Version: "2.10", Summary: "GNU Hello, the \"hello world\" snap", Source: "snap", Status: pkg.StatusUpgradable, Installed: "2.9"},
		{Name: "hello-world", Version: "6.4", Summary: "The 'hello-world' of snaps", Source: "snap", Status: pkg.StatusAvailable},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseFindOutput() = %#v, want %#v", got, want)
	}
}

func TestParseRefreshListOutput(t *testing.T) {
	out := "Name    Version  Rev   Size   Publisher   Notes\n" +
		"snapd   2.64     21800  32MB  canonical✓  snapd\n"

	got := parseRefreshListOutput(out)
	want := []pkg.Package{
		{Name: "snapd", Version: "2.64", Source: "snap", Status: pkg.StatusUpgradable},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseRefreshListOutput() = %#v, want %#v", got, want)
	}
}

func TestParseRefreshListOutputUpToDate(t *testing.T) {
	out := "All snaps up to date.\n"
	got := parseRefreshListOutput(out)
	if len(got) != 0 {
		t.Errorf("parseRefreshListOutput() = %v, want empty", got)
	}
}

func TestInstallChannelCmd(t *testing.T) {
	m := New()
	if got := m.InstallChannelCmd("hello", "stable"); !reflect.DeepEqual(got, []string{"sudo", "snap", "install", "hello"}) {
		t.Errorf("InstallChannelCmd(stable) = %v", got)
	}
	if got := m.InstallChannelCmd("hello", "edge"); !reflect.DeepEqual(got, []string{"sudo", "snap", "install", "--channel=edge", "hello"}) {
		t.Errorf("InstallChannelCmd(edge) = %v", got)
	}
}
