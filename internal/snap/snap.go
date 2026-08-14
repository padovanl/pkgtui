// Package snap implements pkg.Manager on top of the snapd command-line
// client ("snap").
package snap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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

// parseSnapListAllOutput extracts disabled old revisions from "snap list
// --all" output. snapd always keeps the previous revision or two of every
// snap around as an automatic rollback safety net — genuinely useful right
// after a bad refresh, but nobody ever comes back to reclaim the space once
// satisfied the new revision is fine, and there's no autoremove equivalent
// for snap that does it on its own.
func parseSnapListAllOutput(out string) []pkg.DiskItem {
	var items []pkg.DiskItem
	for i, line := range strings.Split(out, "\n") {
		if i == 0 || line == "" {
			continue // header row
		}
		fields := columnSplit(line, 6)
		if len(fields) < 6 {
			continue
		}
		name := strings.TrimSpace(fields[0])
		rev := strings.TrimSpace(fields[2])
		notes := strings.TrimSpace(fields[5])
		if !strings.Contains(notes, "disabled") {
			continue
		}
		items = append(items, pkg.DiskItem{
			Name:   name + " (revision " + rev + ")",
			Reason: "disabled old revision, kept as a rollback safety net",
			Argv:   pkg.MaybeSudo([]string{"snap", "remove", name, "--revision=" + rev}),
		})
	}
	return items
}

// DiskReport surfaces disabled old snap revisions still taking up disk
// space. Implements pkg.DiskAnalyzer.
func (m *Manager) DiskReport() ([]pkg.DiskItem, error) {
	out, err := exec.Command("snap", "list", "--all").Output()
	if err != nil && len(out) == 0 {
		return nil, nil
	}
	return parseSnapListAllOutput(string(out)), nil
}

// parseListRevisions extracts each installed snap's active revision number
// from "snap list" output (the same listing installedMap already parses,
// just keeping the Rev column that one throws away) — the key needed to
// look up its on-disk file and, from that, when it actually last changed.
func parseListRevisions(out string) map[string]string {
	result := make(map[string]string)
	for i, line := range strings.Split(out, "\n") {
		if i == 0 || line == "" {
			continue // header row
		}
		fields := columnSplit(line, 6)
		if len(fields) < 3 {
			continue
		}
		result[strings.TrimSpace(fields[0])] = strings.TrimSpace(fields[2])
	}
	return result
}

// InstalledRevisions maps each installed snap's name to its active
// revision. Implements pkg.Staler.
func (m *Manager) InstalledRevisions() (map[string]string, error) {
	out, err := exec.Command("snap", "list").Output()
	if err != nil && len(out) == 0 {
		return map[string]string{}, nil
	}
	return parseListRevisions(string(out)), nil
}

// snapsDir is where snapd stores each installed revision's actual squashfs
// image, named "<name>_<revision>.snap" — its mtime is this revision's real
// last-refreshed time. Neither "snap list" nor "snap info" exposes a
// reliably machine-parseable date of their own (snap info's "refresh-date"
// is locale-formatted free text meant for a human, not for parsing), so
// this sidesteps that entirely: a filesystem timestamp needs no parsing and
// can't drift out of format across snapd versions or locales.
var snapsDir = "/var/lib/snapd/snaps"

// RefreshTime returns when name's given revision was actually applied.
// Implements pkg.Staler.
func (m *Manager) RefreshTime(name, revision string) (time.Time, error) {
	path := filepath.Join(snapsDir, name+"_"+revision+".snap")
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// RevertCmd reverts name to its previous revision, restoring that
// revision's data/config along with its binary — snap's own idiomatic
// "undo the last refresh," not just installing an older version number.
// Implements pkg.Reverter. If there's no previous revision to go back to,
// the command itself fails at run time with a clear message, surfaced the
// same way any other failed action already is (red border, error text in
// the live-output box) — not something worth a separate pre-check for.
func (m *Manager) RevertCmd(name string) []string {
	return pkg.MaybeSudo([]string{"snap", "revert", name})
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
var _ pkg.DiskAnalyzer = (*Manager)(nil)
var _ pkg.Staler = (*Manager)(nil)
var _ pkg.Reverter = (*Manager)(nil)
