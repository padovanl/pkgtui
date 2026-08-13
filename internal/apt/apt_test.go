package apt

import (
	"reflect"
	"sort"
	"testing"

	"github.com/padovanl/pkgtui/internal/pkg"
)

func TestParseDpkgQueryOutput(t *testing.T) {
	out := "curl\t7.81.0-1ubuntu1.25\t566\tinstall ok installed\n" +
		"bash\t5.1-6ubuntu1.1\t7069\tinstall ok installed\n" +
		"leftover-conffiles\t1.0\t0\tdeinstall ok config-files\n" +
		"\n"

	got := parseDpkgQueryOutput(out)

	want := map[string]dpkgEntry{
		"curl": {Version: "7.81.0-1ubuntu1.25", SizeKB: 566},
		"bash": {Version: "5.1-6ubuntu1.1", SizeKB: 7069},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDpkgQueryOutput() = %#v, want %#v", got, want)
	}
}

func TestParseUpgradableOutput(t *testing.T) {
	// CombinedOutput folds apt's stderr CLI-stability warning in with the
	// real listing on stdout; the parser must not treat it as a package.
	out := "WARNING: apt does not have a stable CLI interface. Use with caution in scripts.\n" +
		"\n" +
		"Listing...\n" +
		"bash/jammy-updates 5.1-6ubuntu1.1 amd64 [upgradable from: 5.1-6ubuntu1]\n" +
		"openssl/jammy-security 3.0.2-0ubuntu1.10 amd64 [upgradable from: 3.0.2-0ubuntu1.9]\n" +
		"\n"

	got := parseUpgradableOutput(out)

	want := []pkg.Package{
		{Name: "bash", Version: "5.1-6ubuntu1.1", Installed: "5.1-6ubuntu1", Source: "apt", Status: pkg.StatusUpgradable},
		{Name: "openssl", Version: "3.0.2-0ubuntu1.10", Installed: "3.0.2-0ubuntu1.9", Source: "apt", Status: pkg.StatusUpgradable, Security: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseUpgradableOutput() = %#v, want %#v", got, want)
	}
}

func TestParseSearchOutput(t *testing.T) {
	out := "curl - command line tool for transferring data with URL syntax\n" +
		"htop - interactive processes viewer\n" +
		"bad line without separator\n"

	installed := map[string]dpkgEntry{
		"curl": {Version: "7.81.0-1ubuntu1.25", SizeKB: 566},
	}
	upgradable := map[string]bool{"curl": true}

	got := parseSearchOutput(out, installed, upgradable)

	want := []pkg.Package{
		{
			Name: "curl", Summary: "command line tool for transferring data with URL syntax",
			Source: "apt", Status: pkg.StatusUpgradable, Installed: "7.81.0-1ubuntu1.25", Size: 566 * 1024,
		},
		{
			Name: "htop", Summary: "interactive processes viewer",
			Source: "apt", Status: pkg.StatusAvailable,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSearchOutput() = %#v, want %#v", got, want)
	}
}

func TestParseAutoremoveOutput(t *testing.T) {
	out := `Reading package lists...
Building dependency tree...
Reading state information...
The following packages will be REMOVED:
  linux-headers-5.15.0-25 linux-headers-5.15.0-25-generic
0 upgraded, 0 newly installed, 2 to remove and 0 not upgraded.
Remv linux-headers-5.15.0-25 [5.15.0-25.25]
Remv linux-headers-5.15.0-25-generic [5.15.0-25.25]
`
	got := parseAutoremoveOutput(out)
	want := []string{"linux-headers-5.15.0-25", "linux-headers-5.15.0-25-generic"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseAutoremoveOutput() = %v, want %v", got, want)
	}
}

func TestParseAutoremoveOutputNothingToRemove(t *testing.T) {
	out := "Reading package lists...\nBuilding dependency tree...\n0 upgraded, 0 newly installed, 0 to remove and 0 not upgraded.\n"
	got := parseAutoremoveOutput(out)
	if len(got) != 0 {
		t.Errorf("parseAutoremoveOutput() = %v, want empty", got)
	}
}

func TestParseRdependsOutput(t *testing.T) {
	out := "libssl3\n" +
		"Reverse Depends:\n" +
		"  curl\n" +
		"  |openssh-server\n" +
		"  python3.10,libpython3.10\n" +
		"  curl\n"

	got := parseRdependsOutput(out)
	want := []string{"curl", "openssh-server", "python3.10"}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseRdependsOutput() = %v, want %v", got, want)
	}
}
