// Package apt implements pkg.Manager on top of the apt/dpkg command-line
// tools available on Debian-based systems.
package apt

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/padovanl/pkgtui/internal/pkg"
)

type Manager struct{}

func New() *Manager { return &Manager{} }

func (m *Manager) Name() string { return "apt" }

func (m *Manager) Available() bool {
	_, err := exec.LookPath("apt-cache")
	if err != nil {
		return false
	}
	_, err = exec.LookPath("dpkg-query")
	return err == nil
}

// dpkgEntry is what we know about an installed package from dpkg's own
// database, before cross-referencing with apt for upgradability.
type dpkgEntry struct {
	Version string
	SizeKB  int64
	Held    bool
}

// parseDpkgQueryOutput parses the output of:
//
//	dpkg-query -W -f '${Package}\t${Version}\t${Installed-Size}\t${Status}\n'
//
// keeping only packages whose status is "installed". dpkg's Status field is
// "<want> <flag> <status>" (e.g. "install ok installed" or, once held,
// "hold ok installed"); the want field is what apt-mark hold/unhold flips.
func parseDpkgQueryOutput(out string) map[string]dpkgEntry {
	result := make(map[string]dpkgEntry)
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 {
			continue
		}
		if !strings.Contains(parts[3], "installed") {
			continue
		}
		sizeKB, _ := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
		held := strings.HasPrefix(parts[3], "hold ")
		result[parts[0]] = dpkgEntry{Version: parts[1], SizeKB: sizeKB, Held: held}
	}
	return result
}

// dpkgQueryOutput runs the raw dpkg-query listing both installedEntries and
// DiskReport parse (differently) from, so a single DiskReport call doesn't
// need to shell out to dpkg-query twice for two views of the same data.
func dpkgQueryOutput() (string, error) {
	out, err := exec.Command("dpkg-query", "-W", "-f", "${Package}\t${Version}\t${Installed-Size}\t${Status}\n").Output()
	if err != nil {
		// dpkg-query exits non-zero if the local package db is empty; the
		// output up to that point is still usable.
		if len(out) == 0 {
			return "", err
		}
	}
	return string(out), nil
}

func installedEntries() (map[string]dpkgEntry, error) {
	out, err := dpkgQueryOutput()
	if err != nil {
		return nil, err
	}
	return parseDpkgQueryOutput(out), nil
}

// parseResidualConfigOutput extracts packages dpkg has fully removed but
// left configuration files behind for ("rc" state in dpkg -l output), from
// the same raw dpkg-query output parseDpkgQueryOutput reads — which
// deliberately excludes these, since installedEntries() only wants
// genuinely installed packages. Nobody ever revisits this list on their
// own; dpkg never prunes it automatically either.
func parseResidualConfigOutput(out string) map[string]dpkgEntry {
	result := make(map[string]dpkgEntry)
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 {
			continue
		}
		if strings.TrimSpace(parts[3]) != "deinstall ok config-files" {
			continue
		}
		sizeKB, _ := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
		result[parts[0]] = dpkgEntry{Version: parts[1], SizeKB: sizeKB}
	}
	return result
}

// kernelPkgRe matches the kernel package families apt's own autoremove
// deliberately always leaves at least one previous generation of behind (so
// a bad new kernel can still be booted into) — exactly what then silently
// piles up over time on a small /boot partition if nobody ever revisits it.
var kernelPkgRe = regexp.MustCompile(`^linux-(?:image|headers|modules(?:-extra)?)-(\d[\w.+-]*)$`)

var digitsRe = regexp.MustCompile(`\d+`)

// kernelVersionKey normalizes a kernel package's version suffix down to
// just its numeric release (e.g. "5.15.0-91"), stripping the flavor suffix
// ("-generic", "-generic-64k"...) so linux-image-5.15.0-91-generic and
// linux-headers-5.15.0-91 (headers packages sometimes omit the flavor) group
// under the same kernel release instead of looking like two unrelated ones.
func kernelVersionKey(suffix string) string {
	nums := digitsRe.FindAllString(suffix, -1)
	if len(nums) == 0 {
		return suffix
	}
	if len(nums) > 3 {
		nums = nums[:3]
	}
	return strings.Join(nums, ".")
}

// kernelVersionLess compares two kernelVersionKey outputs numerically,
// component by component: a plain string compare gets this backwards
// ("5.4" sorts after "5.15" lexicographically, the wrong way round).
func kernelVersionLess(a, b string) bool {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		an, _ := strconv.Atoi(as[i])
		bn, _ := strconv.Atoi(bs[i])
		if an != bn {
			return an < bn
		}
	}
	return len(as) < len(bs)
}

// kernelDiskItems flags installed kernel packages (image/headers/modules)
// belonging to a release that's neither the currently running one nor the
// newest installed one.
func kernelDiskItems(installed map[string]dpkgEntry, running string) []pkg.DiskItem {
	runningKey := kernelVersionKey(running)
	byKey := map[string][]string{}
	for name := range installed {
		m := kernelPkgRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		key := kernelVersionKey(m[1])
		byKey[key] = append(byKey[key], name)
	}
	if len(byKey) <= 1 {
		return nil // nothing to compare against, or no kernel packages tracked at all
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return kernelVersionLess(keys[i], keys[j]) })
	newest := keys[len(keys)-1]

	var items []pkg.DiskItem
	for _, key := range keys {
		if key == newest || key == runningKey {
			continue
		}
		for _, name := range byKey[key] {
			e := installed[name]
			items = append(items, pkg.DiskItem{
				Name:   name,
				Reason: "old kernel (" + key + ")",
				Size:   e.SizeKB * 1024,
				Argv:   pkg.MaybeSudo([]string{"apt-get", "purge", "-y", name}),
			})
		}
	}
	return items
}

// DiskReport surfaces installed things taking up disk space without adding
// value any more: old kernel packages and residual config files left
// behind by already-removed packages. Implements pkg.DiskAnalyzer.
func (m *Manager) DiskReport() ([]pkg.DiskItem, error) {
	raw, err := dpkgQueryOutput()
	if err != nil {
		return nil, err
	}
	installed := parseDpkgQueryOutput(raw)
	residual := parseResidualConfigOutput(raw)

	running := ""
	if out, err := exec.Command("uname", "-r").Output(); err == nil {
		running = strings.TrimSpace(string(out))
	}

	items := kernelDiskItems(installed, running)
	for name, e := range residual {
		items = append(items, pkg.DiskItem{
			Name:   name,
			Reason: "leftover config files (package already removed)",
			Size:   e.SizeKB * 1024,
			Argv:   pkg.MaybeSudo([]string{"dpkg", "--purge", name}),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Size != items[j].Size {
			return items[i].Size > items[j].Size
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

// Provenance reports whether name was explicitly installed (as opposed to
// pulled in as a dependency) and what currently depends on it. Implements
// pkg.ProvenanceProvider.
func (m *Manager) Provenance(name string) (pkg.Provenance, error) {
	manual := false
	if out, err := exec.Command("apt-mark", "showmanual", name).Output(); err == nil {
		manual = strings.TrimSpace(string(out)) == name
	}
	var revdeps []string
	if out, err := exec.Command("apt-cache", "rdepends", name).Output(); err == nil {
		revdeps = parseRdependsOutput(string(out))
	}
	return pkg.Provenance{Manual: manual, ReverseDeps: revdeps}, nil
}

// parseMadisonOutput parses "apt-cache madison <pkg>" output, one version
// per line in the form "pkg | version | origin" (apt itself doesn't
// document the exact column widths/whitespace, hence the trimming). The
// same version can appear once per matching origin (e.g. both binary
// Packages and source Sources entries); only the first occurrence is kept.
func parseMadisonOutput(out, installed string) []pkg.PackageVersion {
	var versions []pkg.PackageVersion
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		version := strings.TrimSpace(parts[1])
		origin := strings.TrimSpace(parts[2])
		if version == "" || seen[version] {
			continue
		}
		seen[version] = true
		versions = append(versions, pkg.PackageVersion{Version: version, Origin: origin, Current: version == installed})
	}
	return versions
}

// AvailableVersions lists every version of name apt-cache madison knows
// about (across all configured repos, not just the single candidate apt
// would pick on its own). Implements pkg.VersionLister.
func (m *Manager) AvailableVersions(name string) ([]pkg.PackageVersion, error) {
	out, err := exec.Command("apt-cache", "madison", name).Output()
	if err != nil {
		return nil, fmt.Errorf("apt-cache madison %s: %w", name, err)
	}
	installed := ""
	if entries, err := installedEntries(); err == nil {
		if e, ok := entries[name]; ok {
			installed = e.Version
		}
	}
	return parseMadisonOutput(string(out), installed), nil
}

// InstallVersionCmd installs (or downgrades to) an exact version of name.
// Implements pkg.VersionLister.
func (m *Manager) InstallVersionCmd(name, version string) []string {
	return pkg.MaybeSudo([]string{"apt-get", "install", "-y", name + "=" + version})
}

// parseKeptBackOutput extracts package names from "apt-get upgrade -s"'s
// "The following packages have been kept back:" section: packages with an
// upgrade available that a conservative upgrade won't perform because it
// would need to install or remove something else first. The section is a
// run of "  "-indented lines (names wrapped across several lines for a
// long list), terminated by the first line that isn't indented that way.
func parseKeptBackOutput(out string) []string {
	const marker = "The following packages have been kept back:"
	var names []string
	inSection := false
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, marker) {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			break
		}
		names = append(names, strings.Fields(line)...)
	}
	return names
}

// UpgradeConflicts reports packages a conservative "apt-get upgrade" would
// leave behind (as opposed to "upgrade all" in this UI, which already uses
// dist-upgrade and resolves most of these on its own — see UpgradeCmd).
// Implements pkg.ConflictReporter.
func (m *Manager) UpgradeConflicts() ([]pkg.UpgradeConflict, error) {
	out, err := exec.Command("apt-get", "upgrade", "-s").Output()
	if err != nil {
		return nil, fmt.Errorf("apt-get upgrade -s: %w", err)
	}
	names := parseKeptBackOutput(string(out))
	if len(names) == 0 {
		return nil, nil
	}
	held := map[string]bool{}
	if entries, err := installedEntries(); err == nil {
		for name, e := range entries {
			held[name] = e.Held
		}
	}
	items := make([]pkg.UpgradeConflict, 0, len(names))
	for _, name := range names {
		reason := "needs a dependency change (install/remove something else) a plain upgrade won't perform on its own"
		if held[name] {
			reason = "held (apt-mark hold)"
		}
		items = append(items, pkg.UpgradeConflict{Name: name, Reason: reason})
	}
	return items, nil
}

const autoUpgradesConfigPath = "/etc/apt/apt.conf.d/20auto-upgrades"
const unattendedUpgradesLogPath = "/var/log/unattended-upgrades/unattended-upgrades.log"

var autoUpgradesEnabledRe = regexp.MustCompile(`APT::Periodic::Unattended-Upgrade\s+"(\d+)"`)

// parseAutoUpgradesEnabled reports whether APT::Periodic::Unattended-Upgrade
// is turned on in 20auto-upgrades (the file unattended-upgrades' own
// postinst writes when "enable automatic updates" is accepted at install
// time), e.g. a line like: APT::Periodic::Unattended-Upgrade "1";
func parseAutoUpgradesEnabled(conf string) bool {
	m := autoUpgradesEnabledRe.FindStringSubmatch(conf)
	return m != nil && m[1] != "0"
}

// parseUnattendedUpgradesLog scans an unattended-upgrades.log for the most
// recent "Packages that will be upgraded: ..." line, returning its
// timestamp and the package names — the only per-run summary this log
// actually contains, everything else is verbose noise around it.
func parseUnattendedUpgradesLog(log string) (timestamp string, packages []string) {
	const marker = "Packages that will be upgraded: "
	for _, line := range strings.Split(log, "\n") {
		idx := strings.Index(line, marker)
		if idx == -1 {
			continue
		}
		ts, _, ok := strings.Cut(line, ",") // "2026-08-10 06:27:03,001 INFO ..."
		if !ok {
			continue
		}
		timestamp = ts
		packages = strings.Fields(line[idx+len(marker):])
	}
	return timestamp, packages
}

// UnattendedUpgradesStatus reports on silent background upgrades, which
// otherwise leave no trace anywhere a user is likely to look. Every field
// is best-effort: a missing config file or log just means "unknown", not an
// error worth failing the whole call over. Implements
// pkg.UnattendedUpgradesReporter.
func (m *Manager) UnattendedUpgradesStatus() (pkg.UnattendedUpgradesStatus, error) {
	var status pkg.UnattendedUpgradesStatus
	if conf, err := os.ReadFile(autoUpgradesConfigPath); err == nil {
		status.Enabled = parseAutoUpgradesEnabled(string(conf))
	}
	if logData, err := os.ReadFile(unattendedUpgradesLogPath); err == nil {
		status.LastRunTime, status.LastPackages = parseUnattendedUpgradesLog(string(logData))
	}
	if out, err := exec.Command("systemctl", "show", "apt-daily-upgrade.timer", "-p", "NextElapseUSecRealtime", "--value").Output(); err == nil {
		if next := strings.TrimSpace(string(out)); next != "" && next != "n/a" {
			status.NextRunTime = next
		}
	}
	return status, nil
}

// parseUpgradableOutput parses the output of "apt list --upgradable". The
// suite name after the "/" (e.g. "jammy-security") marks security updates.
// apt (deliberately, per its own docs) has no stable machine-readable
// output format, and prints a "WARNING: apt does not have a stable CLI
// interface" banner on stderr that CombinedOutput folds in with the actual
// listing, so that gets filtered out here too, not just "Listing...".
//
// held comes from dpkg's own selection state (installedEntries), not from
// this command's output: apt still lists a held package here (it does have
// a newer version available, hold just blocks apt from installing it), but
// says nothing about the hold itself.
func parseUpgradableOutput(out string, held map[string]bool) []pkg.Package {
	var results []pkg.Package
	for _, line := range strings.Split(out, "\n") {
		if line == "" || strings.HasPrefix(line, "Listing...") || strings.HasPrefix(line, "WARNING:") {
			continue
		}
		nameRepo, rest, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		name, suite, _ := strings.Cut(nameRepo, "/")
		fields := strings.Fields(rest)
		version := ""
		if len(fields) > 0 {
			version = fields[0]
		}
		installedFrom := ""
		if idx := strings.Index(line, "[upgradable from: "); idx != -1 {
			tail := line[idx+len("[upgradable from: "):]
			installedFrom = strings.TrimSuffix(tail, "]")
		}
		results = append(results, pkg.Package{
			Name:      name,
			Version:   version,
			Installed: installedFrom,
			Source:    "apt",
			Status:    pkg.StatusUpgradable,
			Security:  strings.Contains(suite, "security"),
			Held:      held[name],
		})
	}
	return results
}

func fetchUpgradable() ([]pkg.Package, error) {
	out, err := exec.Command("apt", "list", "--upgradable").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("apt list --upgradable: %w", err)
	}
	installed, err := installedEntries()
	if err != nil {
		installed = map[string]dpkgEntry{}
	}
	held := make(map[string]bool, len(installed))
	for name, e := range installed {
		held[name] = e.Held
	}
	return parseUpgradableOutput(string(out), held), nil
}

func upgradableSet() (map[string]bool, error) {
	pkgs, err := fetchUpgradable()
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		set[p.Name] = true
	}
	return set, nil
}

// parseSearchOutput parses "apt-cache search" output ("name - summary" per
// line) and annotates each result with install/upgrade status.
func parseSearchOutput(out string, installed map[string]dpkgEntry, upgradable map[string]bool) []pkg.Package {
	var results []pkg.Package
	for _, line := range strings.Split(out, "\n") {
		name, summary, ok := strings.Cut(line, " - ")
		if !ok {
			continue
		}
		p := pkg.Package{
			Name:    strings.TrimSpace(name),
			Summary: strings.TrimSpace(summary),
			Source:  "apt",
			Status:  pkg.StatusAvailable,
		}
		if e, ok := installed[p.Name]; ok {
			p.Installed = e.Version
			p.Size = e.SizeKB * 1024
			p.Held = e.Held
			p.Status = pkg.StatusInstalled
			if upgradable[p.Name] {
				p.Status = pkg.StatusUpgradable
			}
		}
		results = append(results, p)
	}
	return results
}

func (m *Manager) Search(query string) ([]pkg.Package, error) {
	out, err := exec.Command("apt-cache", "search", query).Output()
	if err != nil {
		return nil, fmt.Errorf("apt-cache search: %w", err)
	}
	installed, err := installedEntries()
	if err != nil {
		installed = map[string]dpkgEntry{}
	}
	upgradable, err := upgradableSet()
	if err != nil {
		upgradable = map[string]bool{}
	}
	return parseSearchOutput(string(out), installed, upgradable), nil
}

func (m *Manager) ListInstalled() ([]pkg.Package, error) {
	installed, err := installedEntries()
	if err != nil {
		return nil, err
	}
	upgradable, err := upgradableSet()
	if err != nil {
		upgradable = map[string]bool{}
	}
	results := make([]pkg.Package, 0, len(installed))
	for name, e := range installed {
		p := pkg.Package{
			Name:      name,
			Installed: e.Version,
			Size:      e.SizeKB * 1024,
			Held:      e.Held,
			Source:    "apt",
			Status:    pkg.StatusInstalled,
		}
		if upgradable[name] {
			p.Status = pkg.StatusUpgradable
		}
		results = append(results, p)
	}
	// dpkg-query -W's own ordering isn't guaranteed, so pick a deterministic
	// default; the UI re-sorts by size on request.
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

func (m *Manager) ListUpgradable() ([]pkg.Package, error) {
	return fetchUpgradable()
}

// parseAutoremoveOutput parses the output of "apt-get -s autoremove"
// (simulate mode, no root and no changes made), extracting the packages
// that would be removed ("Remv <name> [<version>]" lines).
func parseAutoremoveOutput(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "Remv ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		names = append(names, fields[1])
	}
	return names
}

// ListOrphaned returns installed packages that "apt-get autoremove" would
// remove (no longer required by anything else). Implements the optional
// pkg.OrphanLister interface.
func (m *Manager) ListOrphaned() ([]pkg.Package, error) {
	out, err := exec.Command("apt-get", "-s", "autoremove").Output()
	if err != nil {
		return nil, fmt.Errorf("apt-get -s autoremove: %w", err)
	}
	names := parseAutoremoveOutput(string(out))
	if len(names) == 0 {
		return nil, nil
	}
	installed, err := installedEntries()
	if err != nil {
		installed = map[string]dpkgEntry{}
	}
	results := make([]pkg.Package, 0, len(names))
	for _, name := range names {
		p := pkg.Package{Name: name, Source: "apt", Status: pkg.StatusInstalled, Summary: "no longer required by anything else"}
		if e, ok := installed[name]; ok {
			p.Installed = e.Version
			p.Size = e.SizeKB * 1024
		}
		results = append(results, p)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

// parseRdependsOutput extracts package names from "apt-cache rdepends"
// output, which looks like:
//
//	<name>
//	Reverse Depends:
//	  pkgA
//	  pkgB,pkgC
func parseRdependsOutput(out string) []string {
	lines := strings.Split(out, "\n")
	var names []string
	seen := map[string]bool{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasSuffix(trimmed, "Reverse Depends:") || !strings.HasPrefix(line, "  ") {
			continue
		}
		name := strings.TrimPrefix(trimmed, "|")
		name, _, _ = strings.Cut(name, ",")
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

const maxRdepends = 25

func (m *Manager) Info(name string) (string, error) {
	out, err := exec.Command("apt-cache", "show", name).Output()
	if err != nil {
		return "", fmt.Errorf("apt-cache show %s: %w", name, err)
	}
	text := string(out)
	// apt-cache show can list multiple versions/stanzas; keep only the first.
	if idx := strings.Index(text, "\n\n"); idx != -1 {
		text = text[:idx]
	}

	if rdOut, err := exec.Command("apt-cache", "rdepends", name).Output(); err == nil {
		names := parseRdependsOutput(string(rdOut))
		if len(names) > 0 {
			shown := names
			suffix := ""
			if len(shown) > maxRdepends {
				shown = shown[:maxRdepends]
				suffix = fmt.Sprintf("\n  ... and %d more", len(names)-maxRdepends)
			}
			text += fmt.Sprintf("\n\nReverse Dependencies (%d):\n  %s%s", len(names), strings.Join(shown, ", "), suffix)
		}
	}

	return text, nil
}

func (m *Manager) InstallCmd(name string) []string {
	return pkg.MaybeSudo([]string{"apt-get", "install", "-y", name})
}

func (m *Manager) RemoveCmd(name string) []string {
	return pkg.MaybeSudo([]string{"apt-get", "remove", "-y", name})
}

func (m *Manager) UpgradeCmd(name string) []string {
	if name == "" {
		// dist-upgrade, not plain upgrade: plain "apt-get upgrade" refuses
		// to touch a package whose new version needs a dependency
		// installed or removed, silently leaving it behind rather than
		// erroring — which showed up live as "upgrade all" needing two
		// separate runs to actually finish (the first run's upgrades
		// freed up whatever had been blocking the rest, which then went
		// through cleanly on a second identical run). dist-upgrade
		// resolves those dependency changes as part of the same
		// transaction, so one run is enough.
		return pkg.MaybeSudo([]string{"apt-get", "dist-upgrade", "-y"})
	}
	return pkg.MaybeSudo([]string{"apt-get", "install", "--only-upgrade", "-y", name})
}

func (m *Manager) UpdateCmd() []string {
	return pkg.MaybeSudo([]string{"apt-get", "update"})
}

// InstallManyCmd installs several packages in one apt-get invocation.
func (m *Manager) InstallManyCmd(names []string) []string {
	return pkg.MaybeSudo(append([]string{"apt-get", "install", "-y"}, names...))
}

// RemoveManyCmd removes several packages in one apt-get invocation.
func (m *Manager) RemoveManyCmd(names []string) []string {
	return pkg.MaybeSudo(append([]string{"apt-get", "remove", "-y"}, names...))
}

// HoldCmd pins a package so apt-get upgrade/dist-upgrade skips it.
func (m *Manager) HoldCmd(name string) []string {
	return pkg.MaybeSudo([]string{"apt-mark", "hold", name})
}

// UnholdCmd releases a previous hold.
func (m *Manager) UnholdCmd(name string) []string {
	return pkg.MaybeSudo([]string{"apt-mark", "unhold", name})
}

// changelogTimeout bounds "apt-get changelog", which fetches over the
// network and would otherwise hang the UI indefinitely on a bad connection.
const changelogTimeout = 20 * time.Second

// Changelog fetches a package's changelog from its source repository.
func (m *Manager) Changelog(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), changelogTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "apt-get", "changelog", "--print-uris=false", name).Output()
	if err != nil {
		return "", fmt.Errorf("apt-get changelog %s: %w", name, err)
	}
	return string(out), nil
}

// ppaURLRe matches a PPA's deb line, e.g.:
//
//	deb https://ppa.launchpadcontent.net/someone/something/ubuntu jammy main
//	deb http://ppa.launchpad.net/someone/something/ubuntu jammy main
var ppaURLRe = regexp.MustCompile(`ppa\.launchpad(?:content)?\.net/([^/]+)/([^/]+)/ubuntu`)

// ListPPAs scans /etc/apt/sources.list.d for third-party PPA sources
// add-apt-repository would have created. Implements pkg.PPAManager.
func (m *Manager) ListPPAs() ([]pkg.PPA, error) {
	return listPPAs(), nil
}

func listPPAs() []pkg.PPA {
	entries, err := os.ReadDir("/etc/apt/sources.list.d")
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var ppas []pkg.PPA
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".list") {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/etc/apt/sources.list.d", e.Name()))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			m := ppaURLRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			name := fmt.Sprintf("ppa:%s/%s", m[1], m[2])
			if seen[name] {
				continue
			}
			seen[name] = true
			ppas = append(ppas, pkg.PPA{Name: name, Description: e.Name()})
		}
	}
	sort.Slice(ppas, func(i, j int) bool { return ppas[i].Name < ppas[j].Name })
	return ppas
}

// AddPPACmd adds a third-party PPA and refreshes the package index.
func (m *Manager) AddPPACmd(ppa string) []string {
	return pkg.MaybeSudo([]string{"add-apt-repository", "-y", ppa})
}

// RemovePPACmd removes a previously added PPA.
func (m *Manager) RemovePPACmd(ppa pkg.PPA) []string {
	return pkg.MaybeSudo([]string{"add-apt-repository", "--remove", "-y", ppa.Name})
}

var _ pkg.Manager = (*Manager)(nil)
var _ pkg.OrphanLister = (*Manager)(nil)
var _ pkg.BatchManager = (*Manager)(nil)
var _ pkg.Holder = (*Manager)(nil)
var _ pkg.Changelogger = (*Manager)(nil)
var _ pkg.PPAManager = (*Manager)(nil)
var _ pkg.DiskAnalyzer = (*Manager)(nil)
var _ pkg.ProvenanceProvider = (*Manager)(nil)
var _ pkg.UnattendedUpgradesReporter = (*Manager)(nil)
var _ pkg.VersionLister = (*Manager)(nil)
var _ pkg.ConflictReporter = (*Manager)(nil)
