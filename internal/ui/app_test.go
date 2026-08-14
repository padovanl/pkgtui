package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestOverlapScreenTogglesAndDoesNotCrash exercises the app-wide apt+snap
// overlap overlay end to end: it spans both panels at once (unlike every
// other screen, which belongs to a single Panel), so it's driven by the
// root App directly instead of going through Panel.handleKey/Update.
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
