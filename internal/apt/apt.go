// Package apt implements pkg.Manager on top of the apt/dpkg command-line
// tools available on Debian-based systems.
package apt

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

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
}

// parseDpkgQueryOutput parses the output of:
//
//	dpkg-query -W -f '${Package}\t${Version}\t${Installed-Size}\t${Status}\n'
//
// keeping only packages whose status is "installed".
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
		result[parts[0]] = dpkgEntry{Version: parts[1], SizeKB: sizeKB}
	}
	return result
}

func installedEntries() (map[string]dpkgEntry, error) {
	out, err := exec.Command("dpkg-query", "-W", "-f", "${Package}\t${Version}\t${Installed-Size}\t${Status}\n").Output()
	if err != nil {
		// dpkg-query exits non-zero if the local package db is empty; the
		// output up to that point is still usable.
		if len(out) == 0 {
			return nil, err
		}
	}
	return parseDpkgQueryOutput(string(out)), nil
}

// parseUpgradableOutput parses the output of "apt list --upgradable".
func parseUpgradableOutput(out string) []pkg.Package {
	var results []pkg.Package
	for _, line := range strings.Split(out, "\n") {
		if line == "" || strings.HasPrefix(line, "Listing...") {
			continue
		}
		nameRepo, rest, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(nameRepo, "/")
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
		})
	}
	return results
}

func fetchUpgradable() ([]pkg.Package, error) {
	out, err := exec.Command("apt", "list", "--upgradable").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("apt list --upgradable: %w", err)
	}
	return parseUpgradableOutput(string(out)), nil
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
	return []string{"sudo", "apt-get", "install", "-y", name}
}

func (m *Manager) RemoveCmd(name string) []string {
	return []string{"sudo", "apt-get", "remove", "-y", name}
}

func (m *Manager) UpgradeCmd(name string) []string {
	if name == "" {
		return []string{"sudo", "apt-get", "upgrade", "-y"}
	}
	return []string{"sudo", "apt-get", "install", "--only-upgrade", "-y", name}
}

func (m *Manager) UpdateCmd() []string {
	return []string{"sudo", "apt-get", "update"}
}

// InstallManyCmd installs several packages in one apt-get invocation.
func (m *Manager) InstallManyCmd(names []string) []string {
	return append([]string{"sudo", "apt-get", "install", "-y"}, names...)
}

// RemoveManyCmd removes several packages in one apt-get invocation.
func (m *Manager) RemoveManyCmd(names []string) []string {
	return append([]string{"sudo", "apt-get", "remove", "-y"}, names...)
}

var _ pkg.Manager = (*Manager)(nil)
var _ pkg.OrphanLister = (*Manager)(nil)
var _ pkg.BatchManager = (*Manager)(nil)
