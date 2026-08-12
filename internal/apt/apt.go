// Package apt implements pkg.Manager on top of the apt/dpkg command-line
// tools available on Debian-based systems.
package apt

import (
	"bufio"
	"fmt"
	"os/exec"
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

// installedVersions returns a map of installed package name -> version for
// packages currently in the "installed" state.
func installedVersions() (map[string]string, error) {
	out, err := exec.Command("dpkg-query", "-W", "-f", "${Package}\t${Version}\t${Status}\n").Output()
	if err != nil {
		// dpkg-query exits non-zero if the local package db is empty; the
		// output up to that point is still usable.
		if len(out) == 0 {
			return nil, err
		}
	}
	result := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "\t", 3)
		if len(parts) != 3 {
			continue
		}
		if !strings.Contains(parts[2], "installed") {
			continue
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

func (m *Manager) Search(query string) ([]pkg.Package, error) {
	out, err := exec.Command("apt-cache", "search", query).Output()
	if err != nil {
		return nil, fmt.Errorf("apt-cache search: %w", err)
	}
	installed, err := installedVersions()
	if err != nil {
		installed = map[string]string{}
	}
	upgradable, err := upgradableSet()
	if err != nil {
		upgradable = map[string]bool{}
	}

	var results []pkg.Package
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
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
		if v, ok := installed[p.Name]; ok {
			p.Installed = v
			p.Status = pkg.StatusInstalled
			if upgradable[p.Name] {
				p.Status = pkg.StatusUpgradable
			}
		}
		results = append(results, p)
	}
	return results, nil
}

func (m *Manager) ListInstalled() ([]pkg.Package, error) {
	installed, err := installedVersions()
	if err != nil {
		return nil, err
	}
	upgradable, err := upgradableSet()
	if err != nil {
		upgradable = map[string]bool{}
	}
	results := make([]pkg.Package, 0, len(installed))
	for name, version := range installed {
		p := pkg.Package{
			Name:      name,
			Installed: version,
			Source:    "apt",
			Status:    pkg.StatusInstalled,
		}
		if upgradable[name] {
			p.Status = pkg.StatusUpgradable
		}
		results = append(results, p)
	}
	return results, nil
}

// upgradableSet returns the set of installed packages that have an upgrade
// available, as reported by "apt list --upgradable".
func upgradableSet() (map[string]bool, error) {
	pkgs, err := parseUpgradable()
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		set[p.Name] = true
	}
	return set, nil
}

func parseUpgradable() ([]pkg.Package, error) {
	out, err := exec.Command("apt", "list", "--upgradable").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("apt list --upgradable: %w", err)
	}
	var results []pkg.Package
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Listing...") || line == "" {
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
	return results, nil
}

func (m *Manager) ListUpgradable() ([]pkg.Package, error) {
	return parseUpgradable()
}

func (m *Manager) Info(name string) (string, error) {
	out, err := exec.Command("apt-cache", "show", name).Output()
	if err != nil {
		return "", fmt.Errorf("apt-cache show %s: %w", name, err)
	}
	// apt-cache show can list multiple versions/stanzas; keep only the first.
	if idx := strings.Index(string(out), "\n\n"); idx != -1 {
		return string(out[:idx]), nil
	}
	return string(out), nil
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

var _ pkg.Manager = (*Manager)(nil)
