package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
