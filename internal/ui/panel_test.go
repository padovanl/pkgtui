package ui

import (
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
