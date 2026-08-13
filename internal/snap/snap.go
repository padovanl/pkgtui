// Package snap implements pkg.Manager on top of the snapd command-line
// client ("snap").
package snap

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/padovanl/pkgtui/internal/pkg"
)

type Manager struct{}

func New() *Manager { return &Manager{} }

func (m *Manager) Name() string { return "snap" }

func (m *Manager) Available() bool {
	_, err := exec.LookPath("snap")
	return err == nil
}

// columnSplit splits a line of tabular snap CLI output on runs of two or
// more spaces, which is how these commands align their columns. maxFields
// caps the number of resulting fields, so the last one (usually a free-text
// summary/description) keeps any single spaces it contains.
var columnRe = regexp.MustCompile(`\s{2,}`)

func columnSplit(line string, maxFields int) []string {
	return columnRe.Split(strings.TrimRight(line, " "), maxFields)
}

// parseListOutput parses "snap list" output into name -> installed package.
func parseListOutput(out string) map[string]pkg.Package {
	result := make(map[string]pkg.Package)
	for i, line := range strings.Split(out, "\n") {
		if i == 0 || line == "" {
			continue // header row
		}
		fields := columnSplit(line, 6)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSpace(fields[0])
		version := strings.TrimSpace(fields[1])
		result[name] = pkg.Package{
			Name:      name,
			Installed: version,
			Source:    "snap",
			Status:    pkg.StatusInstalled,
		}
	}
	return result
}

func (m *Manager) installedMap() (map[string]pkg.Package, error) {
	out, err := exec.Command("snap", "list").Output()
	if err != nil {
		// "no snaps installed" makes snap list exit non-zero on some versions.
		if len(out) == 0 {
			return map[string]pkg.Package{}, nil
		}
	}
	return parseListOutput(string(out)), nil
}

// parseFindOutput parses "snap find <query>" output, annotating results
// with install/upgrade status from the already-installed set.
func parseFindOutput(out string, installed map[string]pkg.Package) []pkg.Package {
	var results []pkg.Package
	for i, line := range strings.Split(out, "\n") {
		if i == 0 || line == "" {
			continue // header row
		}
		fields := columnSplit(line, 5)
		if len(fields) < 5 {
			continue
		}
		name := strings.TrimSpace(fields[0])
		version := strings.TrimSpace(fields[1])
		summary := strings.TrimSpace(fields[4])
		p := pkg.Package{
			Name:    name,
			Version: version,
			Summary: summary,
			Source:  "snap",
			Status:  pkg.StatusAvailable,
		}
		if inst, ok := installed[name]; ok {
			p.Installed = inst.Installed
			p.Status = pkg.StatusInstalled
			if inst.Installed != version {
				p.Status = pkg.StatusUpgradable
			}
		}
		results = append(results, p)
	}
	return results
}

func (m *Manager) Search(query string) ([]pkg.Package, error) {
	out, err := exec.Command("snap", "find", query).Output()
	if err != nil {
		return nil, fmt.Errorf("snap find: %w", err)
	}
	installed, err := m.installedMap()
	if err != nil {
		installed = map[string]pkg.Package{}
	}
	return parseFindOutput(string(out), installed), nil
}

func (m *Manager) ListInstalled() ([]pkg.Package, error) {
	installed, err := m.installedMap()
	if err != nil {
		return nil, err
	}
	results := make([]pkg.Package, 0, len(installed))
	for _, p := range installed {
		results = append(results, p)
	}
	return results, nil
}

// parseRefreshListOutput parses "snap refresh --list" output into the set
// of packages with an upgrade available.
func parseRefreshListOutput(out string) []pkg.Package {
	var results []pkg.Package
	for i, line := range strings.Split(out, "\n") {
		if i == 0 || line == "" || strings.Contains(line, "up to date") {
			continue
		}
		fields := columnSplit(line, 6)
		if len(fields) < 2 {
			continue
		}
		results = append(results, pkg.Package{
			Name:    strings.TrimSpace(fields[0]),
			Version: strings.TrimSpace(fields[1]),
			Source:  "snap",
			Status:  pkg.StatusUpgradable,
		})
	}
	return results
}

func (m *Manager) ListUpgradable() ([]pkg.Package, error) {
	out, err := exec.Command("snap", "refresh", "--list").CombinedOutput()
	if err != nil {
		// "All snaps up to date." exits 0; other errors we surface, but an
		// empty list is not an error condition worth failing on.
		if !strings.Contains(string(out), "up to date") {
			return nil, fmt.Errorf("snap refresh --list: %w", err)
		}
	}
	return parseRefreshListOutput(string(out)), nil
}

func (m *Manager) Info(name string) (string, error) {
	out, err := exec.Command("snap", "info", name).Output()
	if err != nil {
		return "", fmt.Errorf("snap info %s: %w", name, err)
	}
	return string(out), nil
}

func (m *Manager) InstallCmd(name string) []string {
	return pkg.MaybeSudo([]string{"snap", "install", name})
}

func (m *Manager) RemoveCmd(name string) []string {
	return pkg.MaybeSudo([]string{"snap", "remove", name})
}

func (m *Manager) UpgradeCmd(name string) []string {
	if name == "" {
		return pkg.MaybeSudo([]string{"snap", "refresh"})
	}
	return pkg.MaybeSudo([]string{"snap", "refresh", name})
}

func (m *Manager) UpdateCmd() []string {
	return nil
}

// InstallManyCmd installs several snaps in one invocation.
func (m *Manager) InstallManyCmd(names []string) []string {
	return pkg.MaybeSudo(append([]string{"snap", "install"}, names...))
}

// RemoveManyCmd removes several snaps in one invocation.
func (m *Manager) RemoveManyCmd(names []string) []string {
	return pkg.MaybeSudo(append([]string{"snap", "remove"}, names...))
}

// Channels lists the standard snap risk levels, most to least stable.
func (m *Manager) Channels() []string {
	return []string{"stable", "candidate", "beta", "edge"}
}

// InstallChannelCmd installs name from a specific channel/risk level.
func (m *Manager) InstallChannelCmd(name, channel string) []string {
	if channel == "" || channel == "stable" {
		return m.InstallCmd(name)
	}
	return pkg.MaybeSudo([]string{"snap", "install", "--channel=" + channel, name})
}

var _ pkg.Manager = (*Manager)(nil)
var _ pkg.BatchManager = (*Manager)(nil)
var _ pkg.ChannelInstaller = (*Manager)(nil)
