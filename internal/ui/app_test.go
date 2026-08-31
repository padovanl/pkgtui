package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/padovanl/pkgtui/internal/pkg"
)

// namedManager is a fakeManager with a name and an availability answer of
// its own, for exercising which backends end up as tabs.
type namedManager struct {
	fakeManager
	name      string
	available bool
}

func (m namedManager) Name() string    { return m.name }
func (m namedManager) Available() bool { return m.available }

func TestFilterAvailableKeepsOnlyPresentBackends(t *testing.T) {
	all := []pkg.Manager{
		namedManager{name: "apt", available: false},
		namedManager{name: "brew", available: true},
		namedManager{name: "macports", available: false},
	}

	got := filterAvailable(all)

	if len(got) != 1 || got[0].Name() != "brew" {
		t.Errorf("filterAvailable() kept %v, want just brew", backendNames(got))
	}
}

// A machine with none of the supported managers installed (a fresh macOS,
// say) must still get tabs: each one then explains that it isn't available,
// which beats an empty window that looks broken.
func TestFilterAvailableFallsBackToAllBackends(t *testing.T) {
	all := []pkg.Manager{
		namedManager{name: "apt", available: false},
		namedManager{name: "snap", available: false},
	}

	if got := filterAvailable(all); len(got) != len(all) {
		t.Errorf("filterAvailable() kept %v, want all of them", backendNames(got))
	}
}

func backendNames(mgrs []pkg.Manager) []string {
	names := make([]string, 0, len(mgrs))
	for _, m := range mgrs {
		names = append(names, m.Name())
	}
	return names
}

// TestOverlapScreenTogglesAndDoesNotCrash exercises the app-wide overlap
// overlay end to end: it spans every panel at once (unlike every other
// screen, which belongs to a single Panel), so it's driven by the root App
// directly instead of going through Panel.handleKey/Update.
func TestOverlapScreenTogglesAndDoesNotCrash(t *testing.T) {
	a := &App{panels: []*Panel{NewPanel(fakeManager{}), NewPanel(fakeManager{})}}
	m, _ := a.Update(tea.WindowSizeMsg{Width: 100, Height: 34})
	a = m.(*App)

	m, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("O")})
	a = m.(*App)
	if a.overlap == nil {
		t.Fatal("expected the overlap screen to open on 'O'")
	}
	if cmd == nil {
		t.Fatal("expected loadOverlapCmd to be returned")
	}

	m, _ = a.Update(cmd())
	a = m.(*App)
	if a.overlap.loading {
		t.Error("overlap screen still loading after its result message was delivered")
	}
	if a.overlap.err != nil {
		t.Errorf("overlap screen error = %v, want nil (fakeManager never errors)", a.overlap.err)
	}

	// Must render without panicking while the overlay is open.
	_ = a.View()

	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(*App)
	if a.overlap != nil {
		t.Error("esc did not close the overlap screen")
	}
}
