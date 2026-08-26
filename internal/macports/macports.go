// Package macports implements pkg.Manager on top of MacPorts' "port"
// command-line tool.
package macports

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/padovanl/pkgtui/internal/pkg"
)

type Manager struct{}

func New() *Manager { return &Manager{} }

func (m *Manager) Name() string { return "macports" }

func (m *Manager) Available() bool {
	_, err := exec.LookPath("port")
	return err == nil
}

// installedEntry is one row of "port installed": MacPorts keeps every
// version of a port it has ever installed, with at most one of them active
// (the one whose files are actually linked into the prefix).
type installedEntry struct {
	Name    string
	Version string // "@3.28.1_0", revision included: that's how port itself addresses it
	Active  bool
}

// installedRe matches a "port -q installed" row, e.g.
//
//	cmake @3.28.1_0 (active)
//	cmake @3.27.9_0
var installedRe = regexp.MustCompile(`^\s*(\S+)\s+@(\S+)(\s+\(active\))?\s*$`)

func parseInstalledOutput(out string) []installedEntry {
	var entries []installedEntry
	for _, line := range strings.Split(out, "\n") {
		m := installedRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		entries = append(entries, installedEntry{Name: m[1], Version: m[2], Active: m[3] != ""})
	}
	return entries
}

// installedMap keeps only the active version of each port: the inactive
// ones are old versions MacPorts hangs on to, surfaced separately by
// DiskReport rather than as packages in their own right.
//
// MacPorts reports no size anywhere in its registry ("port space" shells
// out to du per port, one process each), so Size stays 0 — the "unknown"
// the metrics dashboard already handles for anything else that can't
// measure itself.
func (m *Manager) installedMap() (map[string]pkg.Package, error) {
	out, err := exec.Command("port", "-q", "installed").Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("port installed: %w", err)
	}
	result := map[string]pkg.Package{}
	for _, e := range parseInstalledOutput(string(out)) {
		if !e.Active {
			continue
		}
		result[e.Name] = pkg.Package{
			Name:      e.Name,
			Installed: e.Version,
			Source:    "macports",
			Status:    pkg.StatusInstalled,
		}
	}
	return result, nil
}

func (m *Manager) ListInstalled() ([]pkg.Package, error) {
	installed, err := m.installedMap()
	if err != nil {
		return nil, err
	}
	upgradable, err := m.upgradableSet()
	if err != nil {
		upgradable = map[string]bool{}
	}
	results := make([]pkg.Package, 0, len(installed))
	for name, p := range installed {
		if upgradable[name] {
			p.Status = pkg.StatusUpgradable
		}
		results = append(results, p)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

// outdatedRe matches a "port -q outdated" row, e.g.
//
//	cmake                          3.28.1_0 < 3.29.0_0
var outdatedRe = regexp.MustCompile(`^\s*(\S+)\s+(\S+)\s+<\s+(\S+)\s*$`)

func parseOutdatedOutput(out string) []pkg.Package {
	var results []pkg.Package
	for _, line := range strings.Split(out, "\n") {
		m := outdatedRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		results = append(results, pkg.Package{
			Name:      m[1],
			Installed: m[2],
			Version:   m[3],
			Source:    "macports",
			Status:    pkg.StatusUpgradable,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results
}

func (m *Manager) ListUpgradable() ([]pkg.Package, error) {
	out, err := exec.Command("port", "-q", "outdated").Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("port outdated: %w", err)
	}
	return parseOutdatedOutput(string(out)), nil
}

func (m *Manager) upgradableSet() (map[string]bool, error) {
	pkgs, err := m.ListUpgradable()
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		set[p.Name] = true
	}
	return set, nil
}

// searchHeadRe matches the first of the two lines "port search" prints per
// hit, e.g.
//
//	vim @9.0.2000 (editors)
//	    Vi "workalike" with many additional features
//
// The "--line" flag would fold both into one tab-separated row, but which
// columns it emits (and in what order) has changed between MacPorts
// releases, while this two-line shape has not.
var searchHeadRe = regexp.MustCompile(`^(\S+)\s+@(\S+)\s+\((.*)\)\s*$`)

func parseSearchOutput(out string, installed map[string]pkg.Package, upgradable map[string]bool) []pkg.Package {
	var results []pkg.Package
	for _, line := range strings.Split(out, "\n") {
		if m := searchHeadRe.FindStringSubmatch(line); m != nil {
			p := pkg.Package{
				Name:    m[1],
				Version: m[2],
				Summary: m[3], // categories, until the description line replaces it
				Source:  "macports",
				Status:  pkg.StatusAvailable,
			}
			if inst, ok := installed[p.Name]; ok {
				p.Installed = inst.Installed
				p.Status = pkg.StatusInstalled
				if upgradable[p.Name] {
					p.Status = pkg.StatusUpgradable
				}
			}
			results = append(results, p)
			continue
		}
		// The description is an indented continuation of the entry above.
		if len(results) > 0 && strings.HasPrefix(line, " ") {
			if desc := strings.TrimSpace(line); desc != "" {
				results[len(results)-1].Summary = desc
			}
		}
	}
	return results
}

func (m *Manager) Search(query string) ([]pkg.Package, error) {
	// CombinedOutput: a search with no hits prints "No match for <query>
	// found" and exits non-zero, which is an empty result rather than an
	// error worth surfacing.
	out, err := exec.Command("port", "search", query).CombinedOutput()
	installed, instErr := m.installedMap()
	if instErr != nil {
		installed = map[string]pkg.Package{}
	}
	upgradable, upErr := m.upgradableSet()
	if upErr != nil {
		upgradable = map[string]bool{}
	}
	results := parseSearchOutput(string(out), installed, upgradable)
	if err != nil && len(results) == 0 {
		if strings.Contains(string(out), "No match for") {
			return nil, nil
		}
		return nil, fmt.Errorf("port search: %w", err)
	}
	return results, nil
}

func (m *Manager) Info(name string) (string, error) {
	out, err := exec.Command("port", "info", name).Output()
	if err != nil {
		return "", fmt.Errorf("port info %s: %w", name, err)
	}
	return string(out), nil
}

func (m *Manager) InstallCmd(name string) []string {
	return pkg.MaybeSudo([]string{"port", "install", name})
}

func (m *Manager) RemoveCmd(name string) []string {
	return pkg.MaybeSudo([]string{"port", "uninstall", name})
}

func (m *Manager) UpgradeCmd(name string) []string {
	if name == "" {
		// "outdated" is a pseudo-port expanding to everything with a newer
		// version available — MacPorts has no bare "upgrade everything".
		return pkg.MaybeSudo([]string{"port", "upgrade", "outdated"})
	}
	return pkg.MaybeSudo([]string{"port", "upgrade", name})
}

// UpdateCmd refreshes the ports tree. selfupdate rather than plain sync: it
// syncs the tree and updates MacPorts' own base, which is what its
// documentation tells users to run and what "port upgrade" expects to have
// been run.
func (m *Manager) UpdateCmd() []string {
	return pkg.MaybeSudo([]string{"port", "selfupdate"})
}

// InstallManyCmd installs several ports in one invocation.
func (m *Manager) InstallManyCmd(names []string) []string {
	return pkg.MaybeSudo(append([]string{"port", "install"}, names...))
}

// RemoveManyCmd removes several ports in one invocation.
func (m *Manager) RemoveManyCmd(names []string) []string {
	return pkg.MaybeSudo(append([]string{"port", "uninstall"}, names...))
}

// parseEchoOutput reads "port -q echo <pseudo-port>", one "name @version"
// per line.
func parseEchoOutput(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 || strings.HasPrefix(f[0], "@") {
			continue
		}
		names = append(names, f[0])
	}
	return names
}

// ListOrphaned returns "leaves": installed ports that were pulled in as
// dependencies and that nothing installed still depends on. Implements
// pkg.OrphanLister.
func (m *Manager) ListOrphaned() ([]pkg.Package, error) {
	out, err := exec.Command("port", "-q", "echo", "leaves").Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("port echo leaves: %w", err)
	}
	names := parseEchoOutput(string(out))
	if len(names) == 0 {
		return nil, nil
	}
	installed, err := m.installedMap()
	if err != nil {
		installed = map[string]pkg.Package{}
	}
	results := make([]pkg.Package, 0, len(names))
	for _, name := range names {
		p := pkg.Package{Name: name, Source: "macports", Status: pkg.StatusInstalled, Summary: "no longer required by anything else"}
		if inst, ok := installed[name]; ok {
			p.Installed = inst.Installed
		}
		results = append(results, p)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

// parseDependentsOutput reads "port dependents <name>", which prints a
// sentence ("The following ports are dependent on vim:", or "vim has no
// dependents.") followed by one indented port name per dependent. The
// sentence is the only unindented line, which is what separates it from
// the names.
func parseDependentsOutput(out string) []string {
	var names []string
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" || !strings.HasPrefix(line, " ") {
			continue
		}
		f := strings.Fields(line)
		if len(f) == 0 || seen[f[0]] {
			continue
		}
		seen[f[0]] = true
		names = append(names, f[0])
	}
	return names
}

// Provenance reports whether name was asked for by name or only pulled in
// as a dependency, and which installed ports still depend on it. MacPorts
// records the difference itself — the "requested" flag its registry sets
// for anything installed directly — so this is a lookup rather than a
// guess. Implements pkg.ProvenanceProvider.
func (m *Manager) Provenance(name string) (pkg.Provenance, error) {
	manual := false
	if out, err := exec.Command("port", "-q", "echo", "requested").Output(); err == nil {
		for _, requested := range parseEchoOutput(string(out)) {
			if requested == name {
				manual = true
				break
			}
		}
	}
	var revdeps []string
	if out, err := exec.Command("port", "dependents", name).Output(); err == nil {
		revdeps = parseDependentsOutput(string(out))
	}
	return pkg.Provenance{Manual: manual, ReverseDeps: revdeps}, nil
}

// inactiveDiskItems turns every inactive installed version into a
// reclaimable-space finding. MacPorts deliberately keeps the previous
// version of a port around after an upgrade so it can be reactivated
// without a rebuild, and never prunes them on its own.
func inactiveDiskItems(entries []installedEntry) []pkg.DiskItem {
	var items []pkg.DiskItem
	for _, e := range entries {
		if e.Active {
			continue
		}
		items = append(items, pkg.DiskItem{
			Name:   e.Name + " @" + e.Version,
			Reason: "inactive old version, kept so it can be reactivated without a rebuild",
			Argv:   pkg.MaybeSudo([]string{"port", "uninstall", e.Name, "@" + e.Version}),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// DiskReport surfaces inactive port versions still on disk. Implements
// pkg.DiskAnalyzer.
func (m *Manager) DiskReport() ([]pkg.DiskItem, error) {
	out, err := exec.Command("port", "-q", "installed").Output()
	if err != nil && len(out) == 0 {
		return nil, nil
	}
	return inactiveDiskItems(parseInstalledOutput(string(out))), nil
}

var _ pkg.Manager = (*Manager)(nil)
var _ pkg.BatchManager = (*Manager)(nil)
var _ pkg.OrphanLister = (*Manager)(nil)
var _ pkg.DiskAnalyzer = (*Manager)(nil)
var _ pkg.ProvenanceProvider = (*Manager)(nil)
