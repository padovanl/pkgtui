package ui

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/padovanl/pkgtui/internal/pkg"
)

// fakeManager is a minimal pkg.Manager stub, just enough to construct a
// Panel in tests without touching a real apt/snap installation.
type fakeManager struct{}

func (fakeManager) Name() string                               { return "fake" }
func (fakeManager) Available() bool                            { return true }
func (fakeManager) Search(query string) ([]pkg.Package, error) { return nil, nil }
func (fakeManager) ListInstalled() ([]pkg.Package, error)      { return nil, nil }
func (fakeManager) ListUpgradable() ([]pkg.Package, error)     { return nil, nil }
func (fakeManager) Info(name string) (string, error)           { return "", nil }
func (fakeManager) InstallCmd(name string) []string            { return nil }
func (fakeManager) RemoveCmd(name string) []string             { return nil }
func (fakeManager) UpgradeCmd(name string) []string            { return nil }
func (fakeManager) UpdateCmd() []string                        { return nil }

func (fakeManager) DiskReport() ([]pkg.DiskItem, error) {
	return []pkg.DiskItem{
		{
			Name:   "linux-image-5.4.0-100-generic",
			Reason: "old kernel (5.4.0)",
			Size:   300 * 1024 * 1024,
			Argv:   []string{"apt-get", "purge", "-y", "linux-image-5.4.0-100-generic"},
		},
	}, nil
}

func (fakeManager) Provenance(name string) (pkg.Provenance, error) {
	return pkg.Provenance{Manual: false, ReverseDeps: []string{"curl", "wget"}}, nil
}

func (fakeManager) UnattendedUpgradesStatus() (pkg.UnattendedUpgradesStatus, error) {
	return pkg.UnattendedUpgradesStatus{
		Enabled:      true,
		LastRunTime:  "2026-08-10 06:27:03",
		LastPackages: []string{"bash"},
		NextRunTime:  "Fri 2026-08-15 06:12:34 UTC",
	}, nil
}

// fakeVersionManager and fakeReverterManager each wrap fakeManager to add
// exactly one of pkg.VersionLister/pkg.Reverter, kept off fakeManager
// itself and split into two separate types rather than both living on one:
// a real backend is never both at once (apt has one concept, snap the
// other), and openVersionAction branches on "which one, if either" — a
// fake implementing both at once couldn't exercise that branch at all.
type fakeVersionManager struct{ fakeManager }

func (fakeVersionManager) AvailableVersions(name string) ([]pkg.PackageVersion, error) {
	return []pkg.PackageVersion{
		{Version: "2.0", Origin: "jammy-updates", Current: false},
		{Version: "1.0", Origin: "jammy", Current: true},
	}, nil
}

func (fakeVersionManager) InstallVersionCmd(name, version string) []string {
	return []string{"apt-get", "install", "-y", name + "=" + version}
}

type fakeReverterManager struct{ fakeManager }

func (fakeReverterManager) RevertCmd(name string) []string {
	return []string{"snap", "revert", name}
}

// fakeMetricsManager wraps fakeManager to return real installed-package
// data (fakeManager's own ListInstalled always returns nil) for testing
// the metrics dashboard's sort order.
type fakeMetricsManager struct{ fakeManager }

func (fakeMetricsManager) ListInstalled() ([]pkg.Package, error) {
	return []pkg.Package{
		{Name: "small", Size: 100},
		{Name: "big", Size: 900000},
		{Name: "medium", Size: 5000},
	}, nil
}

// fakeConflictManager wraps fakeManager to additionally implement
// pkg.ConflictReporter.
type fakeConflictManager struct{ fakeManager }

func (fakeConflictManager) UpgradeConflicts() ([]pkg.UpgradeConflict, error) {
	return []pkg.UpgradeConflict{{Name: "linux-generic", Reason: "needs a dependency change"}}, nil
}

// TestSearchKeyRecomputesListHeight guards against a regression where
// pressing "/" switched into search mode without recomputing the list's
// height budget for the newly-visible 3-row search box: the list kept
// rendering as if the search box weren't there, overflowing the terminal
// by 3 rows and scrolling the tab bar/title/legend off the top of the
// screen. cycleMode() (Tab) already recomputed correctly, which is why
// the bug only showed up when entering search via "/".
func TestSearchKeyRecomputesListHeight(t *testing.T) {
	p := NewPanel(fakeManager{})
	p.setSize(100, 30)

	before := p.listTopOffset
	np, _ := p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	p = np

	if p.mode != viewSearch {
		t.Fatalf("mode = %v, want viewSearch", p.mode)
	}
	const wantOffset = 1 /* app tab bar */ + 1 /* header */ + 1 /* legend */ + 3 /* search box */
	if p.listTopOffset != wantOffset {
		t.Errorf("listTopOffset after entering search = %d, want %d (unchanged from %d: setSize wasn't recomputed)", p.listTopOffset, wantOffset, before)
	}
}

// TestMouseClickBelowListDoesNotPanic guards against a crash reported live:
// clicking below the last real row (row index computed from the click's Y
// position landing past the end of the items slice) called list.Select with
// an out-of-range index. bubbles/list doesn't validate that itself, so the
// next render panicked inside its own populatedView with a slice-bounds
// error. Reproduces on an empty list (no items loaded yet — the fastest way
// to trigger it, any click at all is "past the end") and on a short one.
func TestMouseClickBelowListDoesNotPanic(t *testing.T) {
	p := NewPanel(fakeManager{})
	p.setSize(100, 30)

	click := tea.MouseMsg{X: 5, Y: 20, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	p, _ = p.handleMouse(click) // empty list: must not panic

	p.list.SetItems(itemsFrom([]pkg.Package{{Name: "a"}, {Name: "b"}}))
	p, _ = p.handleMouse(click) // short list, click past the last row: must not panic

	if got := p.list.Index(); got != 0 {
		t.Errorf("list.Index() = %d, want 0 (unchanged: an out-of-range click is ignored, not applied)", got)
	}
}

// TestLongConfirmLabelWraps guards against a crash reported live: "upgrade
// all" joins every package name into one unbroken line, and modalStyle has
// no width of its own, so without wrapping that line first, it runs
// straight past the box border and off the edge of the terminal instead of
// wrapping inside it. Checks both directions: a long label wraps to fit
// the terminal width, and a short one — the common "Install x?" case —
// stays naturally compact instead of getting padded out to the wrap width
// too.
func TestLongConfirmLabelWraps(t *testing.T) {
	const termWidth = 100

	names := make([]string, 60)
	for i := range names {
		names[i] = "some-fairly-long-package-name-" + string(rune('a'+i%26))
	}
	longLabel := "Upgrade 60 apt packages?\n" + strings.Join(names, ", ")

	p := NewPanel(fakeManager{})
	p.setSize(termWidth, 34)
	p.screen = screenConfirm
	p.pending = &pendingAction{label: longLabel}

	for _, line := range strings.Split(p.View(), "\n") {
		if w := lipgloss.Width(line); w > termWidth {
			t.Fatalf("line %d chars wide, wider than the %d-column terminal: %q", w, termWidth, line)
		}
	}

	p.pending = &pendingAction{label: "Install cowsay?"}
	var boxTop string
	for _, line := range strings.Split(p.View(), "\n") {
		if strings.Contains(line, "╭") {
			boxTop = line
			break
		}
	}
	// lipgloss.Place centers the whole modal in the full terminal width,
	// so the *view* naturally has plenty of padding around a small box —
	// that's not the wrapping bug this guards against. What matters is
	// the box itself: for one short line, it should stay close to its
	// natural content width, not get stretched out to the wrap cap.
	if w := lipgloss.Width(strings.TrimSpace(boxTop)); w > 40 {
		t.Errorf("short confirm's box is %d chars wide, wider than it should need to be for one short line: %q", w, boxTop)
	}
}

// TestDiskScreenOpensAndPurgeAsksConfirmation exercises the disk cleanup
// explorer end to end: opening it loads findings, and purging one goes
// through the same confirm-then-run flow as every other privileged action,
// returning to the disk screen (not the package list) afterwards.
func TestDiskScreenOpensAndPurgeAsksConfirmation(t *testing.T) {
	p := NewPanel(fakeManager{})
	p.setSize(100, 30)

	np, _ := p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")})
	p = np
	if p.screen != screenDisk {
		t.Fatalf("screen = %v, want screenDisk", p.screen)
	}

	p, _ = p.Update(p.loadDiskReportCmd()())
	if len(p.diskItems) != 1 {
		t.Fatalf("diskItems = %v, want 1 item", p.diskItems)
	}

	np, _ = p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	p = np
	if p.screen != screenConfirm || p.pending == nil {
		t.Fatalf("screen = %v, pending = %v, want screenConfirm with a pending purge", p.screen, p.pending)
	}
	if p.returnScreen != screenDisk {
		t.Errorf("returnScreen = %v, want screenDisk, so cancelling/confirming returns here rather than the package list", p.returnScreen)
	}
}

// TestProvenanceDrillDownAndBack exercises the "why is this installed"
// screen's navigation stack: drilling into a reverse dependency, then
// stepping back out one level at a time with esc instead of leaving
// straight to the package list.
func TestProvenanceDrillDownAndBack(t *testing.T) {
	p := NewPanel(fakeManager{})
	p.setSize(100, 30)
	p.list.SetItems(itemsFrom([]pkg.Package{{Name: "curl", Status: pkg.StatusInstalled}}))

	np, _ := p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("W")})
	p = np
	if p.screen != screenProvenance || p.provenanceName != "curl" {
		t.Fatalf("screen = %v, provenanceName = %q, want screenProvenance for curl", p.screen, p.provenanceName)
	}

	p, _ = p.Update(p.loadProvenanceCmd("curl")())
	if len(p.provenance.ReverseDeps) != 2 {
		t.Fatalf("ReverseDeps = %v, want 2 entries", p.provenance.ReverseDeps)
	}
	firstDep := p.provenance.ReverseDeps[0]

	np, _ = p.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	p = np
	if p.provenanceName != firstDep {
		t.Fatalf("after drilling in, provenanceName = %q, want %q", p.provenanceName, firstDep)
	}
	if want := []string{"curl"}; !reflect.DeepEqual(p.provenanceStack, want) {
		t.Fatalf("provenanceStack = %v, want %v", p.provenanceStack, want)
	}

	np, _ = p.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	p = np
	if p.screen != screenProvenance || p.provenanceName != "curl" {
		t.Fatalf("after one esc: screen = %v, provenanceName = %q, want to have stepped back to curl, not left the screen", p.screen, p.provenanceName)
	}

	np, _ = p.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	p = np
	if p.screen != screenList {
		t.Fatalf("after a second esc (empty stack): screen = %v, want screenList", p.screen)
	}
}

// TestVersionPickerInstallsSelectedVersion exercises "V" on a backend that
// implements pkg.VersionLister (apt): it opens a picker over every version
// apt-cache madison reports, and confirming one builds an install/downgrade
// command for that exact version, not just re-running the default install.
func TestVersionPickerInstallsSelectedVersion(t *testing.T) {
	p := NewPanel(fakeVersionManager{})
	p.setSize(100, 30)
	p.list.SetItems(itemsFrom([]pkg.Package{{Name: "curl", Status: pkg.StatusInstalled}}))

	np, _ := p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	p = np
	if p.screen != screenVersion || p.versionPkgName != "curl" {
		t.Fatalf("screen = %v, versionPkgName = %q, want screenVersion for curl", p.screen, p.versionPkgName)
	}

	p, _ = p.Update(p.loadVersionsCmd("curl")())
	if len(p.versions) != 2 {
		t.Fatalf("versions = %v, want 2 entries", p.versions)
	}

	// Cursor starts at 0 (the newer, non-current version); confirm it.
	np, _ = p.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	p = np
	if p.screen != screenConfirm || p.pending == nil {
		t.Fatalf("screen = %v, pending = %v, want screenConfirm with a pending version install", p.screen, p.pending)
	}
	wantArgv := []string{"apt-get", "install", "-y", "curl=2.0"}
	if !reflect.DeepEqual(p.pending.argv, wantArgv) {
		t.Errorf("pending.argv = %v, want %v", p.pending.argv, wantArgv)
	}
}

// TestRevertGoesStraightToConfirm exercises "V" on a backend that
// implements pkg.Reverter instead (snap): unlike the apt version picker,
// there's nothing to browse — it goes straight to a confirmation for
// reverting the selected package to its previous revision.
func TestRevertGoesStraightToConfirm(t *testing.T) {
	p := NewPanel(fakeReverterManager{})
	p.setSize(100, 30)
	p.list.SetItems(itemsFrom([]pkg.Package{{Name: "firefox", Status: pkg.StatusInstalled}}))

	np, _ := p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	p = np
	if p.screen != screenConfirm || p.pending == nil {
		t.Fatalf("screen = %v, pending = %v, want screenConfirm with a pending revert", p.screen, p.pending)
	}
	wantArgv := []string{"snap", "revert", "firefox"}
	if !reflect.DeepEqual(p.pending.argv, wantArgv) {
		t.Errorf("pending.argv = %v, want %v", p.pending.argv, wantArgv)
	}
}

// TestProvenanceListScrollsWithManyReverseDeps guards a bug reported live:
// a package with enough reverse dependencies (cyrus-common, in the wild)
// rendered every single one unconditionally, with no height bound at all —
// the exact same overflow class as the help/settings screens, just missed
// there because those had regression tests and this screen didn't. The
// title (and the cursor's own row) must stay visible regardless of how
// long the underlying list actually is.
func TestProvenanceListScrollsWithManyReverseDeps(t *testing.T) {
	const termHeight = 30

	names := make([]string, 60)
	for i := range names {
		names[i] = fmt.Sprintf("some-reverse-dep-%02d", i)
	}

	p := NewPanel(fakeManager{})
	p.setSize(100, termHeight)
	p.screen = screenProvenance
	p.provenanceName = "cyrus-common"
	p.provenance = pkg.Provenance{ReverseDeps: names}
	p.provenanceCursor = 45 // deep into the list, not just the first screenful

	lines := strings.Split(p.View(), "\n")
	if len(lines) > termHeight+2 { // small allowance, matching the box-chrome screens
		t.Fatalf("rendered %d lines for a %d-row terminal, want it bounded", len(lines), termHeight)
	}
	if !strings.Contains(p.View(), "Dependency tree") || !strings.Contains(p.View(), "cyrus-common") {
		t.Error("title is missing from the rendered output (pushed off-screen by the unbounded list?)")
	}
	if !strings.Contains(p.View(), names[p.provenanceCursor]) {
		t.Errorf("the selected row (%q) isn't visible in the rendered output; the scroll window didn't follow the cursor", names[p.provenanceCursor])
	}
}

// TestDiskListScrollsWithManyItems mirrors
// TestProvenanceListScrollsWithManyReverseDeps for the disk cleanup screen,
// which had the identical unbounded-list gap.
func TestDiskListScrollsWithManyItems(t *testing.T) {
	const termHeight = 30

	items := make([]pkg.DiskItem, 60)
	for i := range items {
		items[i] = pkg.DiskItem{Name: fmt.Sprintf("old-kernel-pkg-%02d", i), Reason: "old kernel"}
	}

	p := NewPanel(fakeManager{})
	p.setSize(100, termHeight)
	p.screen = screenDisk
	p.diskItems = items
	p.diskCursor = 45

	lines := strings.Split(p.View(), "\n")
	if len(lines) > termHeight+2 {
		t.Fatalf("rendered %d lines for a %d-row terminal, want it bounded", len(lines), termHeight)
	}
	if !strings.Contains(p.View(), items[p.diskCursor].Name) {
		t.Errorf("the selected row (%q) isn't visible in the rendered output; the scroll window didn't follow the cursor", items[p.diskCursor].Name)
	}
}

// TestMetricsSelectedRowBarUsesAccentColor guards a bug reported live: the
// cursor lands on index 0 by default, which — since the list is sorted by
// size descending — is always the very first row a user sees, highlighted.
// The highlighted-row rendering used to drop the bar's color entirely
// (rendering it as plain block characters instead of the theme's accent
// color), so the one row anyone actually looks at right after pressing "M"
// looked like theming had never been applied to the chart at all, even
// though scrolling down to any other row would have shown it correctly
// colored.
func TestMetricsSelectedRowBarUsesAccentColor(t *testing.T) {
	p := NewPanel(fakeManager{})
	p.setSize(100, 30)
	p.screen = screenMetrics
	p.metrics = []pkg.Package{{Name: "curl", Size: 1000}}
	p.metricsCursor = 0

	want := lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(colorAccent).Bold(true).Render("█")
	if !strings.Contains(p.View(), want) {
		t.Error("selected row's bar doesn't carry the theme's accent color — it renders as plain, uncolored text instead")
	}
}

// TestMetricsSortedBySizeDescending exercises the metrics dashboard's whole
// point: it's a ranking, not just a listing.
func TestMetricsSortedBySizeDescending(t *testing.T) {
	p := NewPanel(fakeMetricsManager{})
	p.setSize(100, 30)

	np, cmd := p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("M")})
	p = np
	if p.screen != screenMetrics {
		t.Fatalf("screen = %v, want screenMetrics", p.screen)
	}
	if cmd == nil {
		t.Fatal("expected a load command")
	}

	p, _ = p.Update(p.loadMetricsCmd()())
	if len(p.metrics) != 3 {
		t.Fatalf("metrics = %v, want 3 entries", p.metrics)
	}
	for i := 1; i < len(p.metrics); i++ {
		if p.metrics[i-1].Size < p.metrics[i].Size {
			t.Fatalf("metrics not sorted descending by size: %v", p.metrics)
		}
	}
	if p.metrics[0].Name != "big" {
		t.Errorf("metrics[0] = %q, want %q (the largest)", p.metrics[0].Name, "big")
	}
}

// TestConflictsScreenLoadsWhenSupported exercises "X" on a backend that
// implements pkg.ConflictReporter.
func TestConflictsScreenLoadsWhenSupported(t *testing.T) {
	p := NewPanel(fakeConflictManager{})
	p.setSize(100, 30)

	np, _ := p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	p = np
	if p.screen != screenConflicts {
		t.Fatalf("screen = %v, want screenConflicts", p.screen)
	}

	p, _ = p.Update(p.loadConflictsCmd()())
	if len(p.conflicts) != 1 || p.conflicts[0].Name != "linux-generic" {
		t.Fatalf("conflicts = %v, want a single linux-generic entry", p.conflicts)
	}
}

// TestConflictsUnavailableShowsMessage guards the fallback for a backend
// without pkg.ConflictReporter (snap): "X" must not switch screens, only
// explain why.
func TestConflictsUnavailableShowsMessage(t *testing.T) {
	p := NewPanel(fakeManager{})
	p.setSize(100, 30)

	np, _ := p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	p = np
	if p.screen != screenList {
		t.Fatalf("screen = %v, want screenList (unavailable, shouldn't switch screens)", p.screen)
	}
	if p.statusMsg == "" {
		t.Error("expected a status message explaining conflicts aren't available")
	}
}

// TestActionLogRecordsAndDisplays exercises the whole path: dismissRunning
// logs an action (success or failure), and the log screen ("L") shows it.
func TestActionLogRecordsAndDisplays(t *testing.T) {
	old := sessionLog
	sessionLog = nil
	defer func() { sessionLog = old }()

	logAction("apt", []string{"sudo", "apt-get", "install", "-y", "curl"}, nil)
	logAction("apt", []string{"sudo", "apt-get", "remove", "-y", "cowsay"}, errors.New("boom"))

	if len(sessionLog) != 2 {
		t.Fatalf("sessionLog has %d entries, want 2", len(sessionLog))
	}
	if sessionLog[0].summary != "apt-get install -y curl" {
		t.Errorf("summary = %q, want the sudo prefix stripped", sessionLog[0].summary)
	}
	if !sessionLog[0].ok {
		t.Error("first entry should be marked successful")
	}
	if sessionLog[1].ok {
		t.Error("second entry should be marked failed")
	}

	p := NewPanel(fakeManager{})
	p.setSize(100, 30)
	np, _ := p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	p = np
	if p.screen != screenLog {
		t.Fatalf("screen = %v, want screenLog", p.screen)
	}
	view := p.View()
	if !strings.Contains(view, "curl") || !strings.Contains(view, "cowsay") {
		t.Errorf("log view is missing expected entries: %q", view)
	}
}

// TestMetricsListScrollsWithManyPackages, TestConflictsListScrollsWithManyEntries
// and TestLogScrollsWithManyEntries guard these three new screens against
// the exact overflow bug that hit the disk-cleanup and provenance screens
// live (see TestProvenanceListScrollsWithManyReverseDeps): an unbounded
// list pushing the screen's own title off the top of the terminal.
func TestMetricsListScrollsWithManyPackages(t *testing.T) {
	const termHeight = 30
	pkgs := make([]pkg.Package, 60)
	for i := range pkgs {
		pkgs[i] = pkg.Package{Name: fmt.Sprintf("pkg-%02d", i), Size: int64(60-i) * 1024}
	}
	p := NewPanel(fakeManager{})
	p.setSize(100, termHeight)
	p.screen = screenMetrics
	p.metrics = pkgs
	p.metricsCursor = 45

	lines := strings.Split(p.View(), "\n")
	if len(lines) > termHeight+2 {
		t.Fatalf("rendered %d lines for a %d-row terminal, want it bounded", len(lines), termHeight)
	}
	if !strings.Contains(p.View(), pkgs[p.metricsCursor].Name) {
		t.Errorf("selected row %q not visible in the rendered output", pkgs[p.metricsCursor].Name)
	}
}

func TestConflictsListScrollsWithManyEntries(t *testing.T) {
	const termHeight = 30
	items := make([]pkg.UpgradeConflict, 60)
	for i := range items {
		items[i] = pkg.UpgradeConflict{Name: fmt.Sprintf("pkg-%02d", i), Reason: "blocked"}
	}
	p := NewPanel(fakeManager{})
	p.setSize(100, termHeight)
	p.screen = screenConflicts
	p.conflicts = items
	p.conflictsCursor = 45

	lines := strings.Split(p.View(), "\n")
	if len(lines) > termHeight+2 {
		t.Fatalf("rendered %d lines for a %d-row terminal, want it bounded", len(lines), termHeight)
	}
	if !strings.Contains(p.View(), items[p.conflictsCursor].Name) {
		t.Errorf("selected row %q not visible in the rendered output", items[p.conflictsCursor].Name)
	}
}

func TestLogScrollsWithManyEntries(t *testing.T) {
	const termHeight = 30
	old := sessionLog
	sessionLog = nil
	defer func() { sessionLog = old }()
	for i := range 60 {
		logAction("apt", []string{"apt-get", "install", "-y", fmt.Sprintf("pkg-%02d", i)}, nil)
	}

	p := NewPanel(fakeManager{})
	p.setSize(100, termHeight)
	p.screen = screenLog
	p.logCursor = 45

	lines := strings.Split(p.View(), "\n")
	if len(lines) > termHeight+2 {
		t.Fatalf("rendered %d lines for a %d-row terminal, want it bounded", len(lines), termHeight)
	}
	if !strings.Contains(p.View(), "pkg-45") {
		t.Error("selected row not visible in the rendered output")
	}
}

// TestHelpScreenTitleVisibleAt100x34 guards the same regression class that
// broke renderHelp once already (see the "not height-aware" history in
// helpContent/renderHelp): at the exact terminal size that caught it before,
// with even more content now that new optional features add their own rows,
// the title must still be the first thing rendered, not pushed off-screen
// by everything above it. Structurally guaranteed now that the body is a
// bounded viewport rather than an unbounded block of rows, but still worth
// asserting directly since that's the property that broke last time.
func TestHelpScreenTitleVisibleAt100x34(t *testing.T) {
	p := NewPanel(fakeManager{})
	p.setSize(100, 34)

	np, _ := p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	p = np

	if !strings.Contains(p.View(), "pkgtui — help") {
		t.Error("help screen's title is missing from the rendered output (pushed off-screen?)")
	}
}

// TestHelpScreenScrolls guards the fix itself: with content now taller than
// a small terminal can show at once, the down arrow must scroll the help
// body instead of doing nothing (or, per the old non-viewport rendering,
// silently losing rows off the bottom with no way to reach them at all).
func TestHelpScreenScrolls(t *testing.T) {
	p := NewPanel(fakeManager{})
	p.setSize(100, 20) // short enough that the full help content can't fit at once

	np, _ := p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	p = np
	if p.viewport.TotalLineCount() <= p.viewport.Height {
		t.Fatalf("test setup: help content (%d lines) already fits in the viewport (%d lines); can't exercise scrolling", p.viewport.TotalLineCount(), p.viewport.Height)
	}

	np, _ = p.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	p = np
	if p.screen != screenHelp {
		t.Fatal("down arrow closed the help screen instead of scrolling it")
	}
	if p.viewport.YOffset == 0 {
		t.Error("down arrow did not scroll the help viewport (YOffset still 0)")
	}
}
