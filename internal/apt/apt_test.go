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

	got := parseUpgradableOutput(out, map[string]bool{"bash": true})

	want := []pkg.Package{
		{Name: "bash", Version: "5.1-6ubuntu1.1", Installed: "5.1-6ubuntu1", Source: "apt", Status: pkg.StatusUpgradable, Held: true},
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

// TestUpgradeCmdUpgradesAllUsesDistUpgrade guards against a regression to
// plain "apt-get upgrade" for the upgrade-all action: it refuses to touch
// a package whose new version needs a dependency installed or removed,
// silently leaving it behind instead — which showed up live as "upgrade
// all" needing two runs to actually finish everything. dist-upgrade
// resolves those dependency changes in the same transaction. Checks just
// the tail of the command, not the sudo prefix (that's MaybeSudo's own
// concern, tested separately in internal/pkg, and depends on the euid
// actually running this test).
func TestUpgradeCmdUpgradesAllUsesDistUpgrade(t *testing.T) {
	got := (&Manager{}).UpgradeCmd("")
	want := []string{"apt-get", "dist-upgrade", "-y"}
	if n := len(got); n < len(want) || !reflect.DeepEqual(got[n-len(want):], want) {
		t.Errorf("UpgradeCmd(\"\") = %v, want it to end with %v", got, want)
	}
}

func TestParseResidualConfigOutput(t *testing.T) {
	out := "curl\t7.81.0-1ubuntu1.25\t566\tinstall ok installed\n" +
		"leftover-conffiles\t1.0\t3\tdeinstall ok config-files\n" +
		"another-leftover\t2.0\t0\tdeinstall ok config-files\n"

	got := parseResidualConfigOutput(out)
	want := map[string]dpkgEntry{
		"leftover-conffiles": {Version: "1.0", SizeKB: 3},
		"another-leftover":   {Version: "2.0", SizeKB: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseResidualConfigOutput() = %#v, want %#v", got, want)
	}
}

func TestKernelVersionKeyStripsFlavor(t *testing.T) {
	cases := map[string]string{
		"5.15.0-91-generic": "5.15.0",
		"5.15.0-91":         "5.15.0",
		"6.8.0-40-generic":  "6.8.0",
	}
	for in, want := range cases {
		if got := kernelVersionKey(in); got != want {
			t.Errorf("kernelVersionKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKernelVersionLessComparesNumerically(t *testing.T) {
	// A plain string compare gets this backwards: "5.4.0" > "5.15.0"
	// lexicographically, since '4' > '1'.
	if !kernelVersionLess("5.4.0", "5.15.0") {
		t.Error("kernelVersionLess(5.4.0, 5.15.0) = false, want true")
	}
	if kernelVersionLess("5.15.0", "5.4.0") {
		t.Error("kernelVersionLess(5.15.0, 5.4.0) = true, want false")
	}
}

// TestKernelDiskItemsKeepsRunningAndNewest guards the actual safety
// property this feature exists for: never flag the kernel currently booted,
// or the newest one installed, as reclaimable — only strictly older, not
// currently running releases are safe to suggest purging.
func TestKernelDiskItemsKeepsRunningAndNewest(t *testing.T) {
	installed := map[string]dpkgEntry{
		"linux-image-5.4.0-100-generic":   {SizeKB: 300000},
		"linux-headers-5.4.0-100-generic": {SizeKB: 10000},
		"linux-headers-5.4.0-100":         {SizeKB: 5000},
		"linux-image-5.15.0-91-generic":   {SizeKB: 320000},
		"linux-headers-5.15.0-91-generic": {SizeKB: 11000},
		"linux-headers-5.15.0-91":         {SizeKB: 5000},
		"linux-image-6.8.0-40-generic":    {SizeKB: 340000},
		"linux-headers-6.8.0-40-generic":  {SizeKB: 12000},
		"linux-headers-6.8.0-40":          {SizeKB: 5000},
		"curl":                            {SizeKB: 500}, // not a kernel package, must be ignored
	}
	// Currently booted into the middle release, not the newest installed.
	got := kernelDiskItems(installed, "5.15.0-91-generic")

	gotNames := map[string]bool{}
	for _, it := range got {
		gotNames[it.Name] = true
	}

	for _, mustBeFlagged := range []string{
		"linux-image-5.4.0-100-generic", "linux-headers-5.4.0-100-generic", "linux-headers-5.4.0-100",
	} {
		if !gotNames[mustBeFlagged] {
			t.Errorf("kernelDiskItems() missing old-kernel package %q, got %v", mustBeFlagged, gotNames)
		}
	}
	for _, mustNotBeFlagged := range []string{
		"linux-image-5.15.0-91-generic", "linux-headers-5.15.0-91-generic", "linux-headers-5.15.0-91", // running
		"linux-image-6.8.0-40-generic", "linux-headers-6.8.0-40-generic", "linux-headers-6.8.0-40", // newest
		"curl",
	} {
		if gotNames[mustNotBeFlagged] {
			t.Errorf("kernelDiskItems() wrongly flagged %q (running or newest kernel, or not a kernel package at all)", mustNotBeFlagged)
		}
	}
}

// TestKernelDiskItemsSingleKernelReportsNothing guards against flagging a
// system's only installed kernel as "old" simply because there's nothing
// newer to compare it against yet.
func TestKernelDiskItemsSingleKernelReportsNothing(t *testing.T) {
	installed := map[string]dpkgEntry{
		"linux-image-6.8.0-40-generic": {SizeKB: 340000},
	}
	if got := kernelDiskItems(installed, "6.8.0-40-generic"); len(got) != 0 {
		t.Errorf("kernelDiskItems() with a single installed kernel = %v, want empty", got)
	}
}

func TestParseAutoUpgradesEnabled(t *testing.T) {
	cases := map[string]bool{
		`APT::Periodic::Update-Package-Lists "1";` + "\n" + `APT::Periodic::Unattended-Upgrade "1";`: true,
		`APT::Periodic::Unattended-Upgrade "0";`:                                                     false,
		`// nothing here`:                                                                            false,
	}
	for conf, want := range cases {
		if got := parseAutoUpgradesEnabled(conf); got != want {
			t.Errorf("parseAutoUpgradesEnabled(%q) = %v, want %v", conf, got, want)
		}
	}
}

func TestParseUnattendedUpgradesLog(t *testing.T) {
	log := "2026-08-10 06:27:01,912 INFO Initial blacklisted packages: \n" +
		"2026-08-10 06:27:02,146 INFO Allowed origins are: o=Ubuntu\n" +
		"2026-08-10 06:27:03,001 INFO Packages that will be upgraded: libssl3 curl\n" +
		"2026-08-10 06:27:15,552 INFO Writing dpkg log to /var/log/unattended-upgrades/unattended-upgrades-dpkg.log\n" +
		"2026-08-11 06:27:03,001 INFO Packages that will be upgraded: bash\n"

	ts, pkgs := parseUnattendedUpgradesLog(log)
	// Must pick up the LAST matching run, not the first.
	if ts != "2026-08-11 06:27:03" {
		t.Errorf("timestamp = %q, want %q", ts, "2026-08-11 06:27:03")
	}
	if want := []string{"bash"}; !reflect.DeepEqual(pkgs, want) {
		t.Errorf("packages = %v, want %v", pkgs, want)
	}
}

func TestParseMadisonOutput(t *testing.T) {
	out := " curl | 7.81.0-1ubuntu1.20 | http://archive.ubuntu.com/ubuntu jammy-updates/main amd64 Packages\n" +
		" curl | 7.81.0-1ubuntu1 | http://archive.ubuntu.com/ubuntu jammy/main amd64 Packages\n" +
		" curl | 7.81.0-1ubuntu1.20 | http://archive.ubuntu.com/ubuntu jammy-security/main amd64 Packages\n" + // same version via a second origin — must collapse, not duplicate
		"garbage line with no separators\n"

	got := parseMadisonOutput(out, "7.81.0-1ubuntu1")
	if len(got) != 2 {
		t.Fatalf("parseMadisonOutput() returned %d versions, want 2 (deduped): %#v", len(got), got)
	}
	if got[0].Version != "7.81.0-1ubuntu1.20" || got[0].Current {
		t.Errorf("got[0] = %#v, want version 7.81.0-1ubuntu1.20, not current", got[0])
	}
	if got[1].Version != "7.81.0-1ubuntu1" || !got[1].Current {
		t.Errorf("got[1] = %#v, want version 7.81.0-1ubuntu1, marked current", got[1])
	}
	if got[1].Origin == "" {
		t.Error("got[1].Origin is empty, want the repo URL/component text")
	}
}

func TestParseKeptBackOutput(t *testing.T) {
	out := `Reading package lists...
Building dependency tree...
Reading state information...
Calculating upgrade...
The following packages have been kept back:
  linux-generic linux-headers-generic linux-image-generic
  another-package
0 upgraded, 0 newly installed, 0 to remove and 4 not upgraded.
`
	got := parseKeptBackOutput(out)
	want := []string{"linux-generic", "linux-headers-generic", "linux-image-generic", "another-package"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseKeptBackOutput() = %v, want %v", got, want)
	}
}

func TestParseKeptBackOutputNothingKeptBack(t *testing.T) {
	out := "Reading package lists...\nBuilding dependency tree...\n0 upgraded, 0 newly installed, 0 to remove and 0 not upgraded.\n"
	if got := parseKeptBackOutput(out); len(got) != 0 {
		t.Errorf("parseKeptBackOutput() = %v, want empty", got)
	}
}

func TestParseUnattendedUpgradesLogNoMatches(t *testing.T) {
	ts, pkgs := parseUnattendedUpgradesLog("nothing relevant here\n")
	if ts != "" || pkgs != nil {
		t.Errorf("parseUnattendedUpgradesLog() = (%q, %v), want (\"\", nil)", ts, pkgs)
	}
}
