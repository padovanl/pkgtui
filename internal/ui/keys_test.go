package ui

import "testing"

// TestResetKeybindingsRestoresDefaults guards the settings screen's
// "reset keybindings" row: rebind an action away from its default, then
// confirm ResetKeybindings puts it (and everything else) back, including
// the one that was never touched — a real reset has to restore all of
// them, not just whichever one happened to be rebound most recently.
func TestResetKeybindingsRestoresDefaults(t *testing.T) {
	defer ResetKeybindings() // don't leak a rebind into other tests

	entries := rebindableKeys()
	if len(entries) < 2 {
		t.Fatal("need at least two rebindable actions for this test")
	}
	first, second := entries[0], entries[1]
	firstDefault := defaultKeybindings[first.action]
	secondDefault := defaultKeybindings[second.action]

	if _, ok := RebindKey(first.action, "z"); !ok {
		t.Fatalf("RebindKey(%q, \"z\") failed unexpectedly", first.action)
	}
	if got := first.ptr.Keys()[0]; got != "z" {
		t.Fatalf("after rebind, %s = %q, want \"z\"", first.action, got)
	}

	ResetKeybindings()

	if got := first.ptr.Keys()[0]; got != firstDefault {
		t.Errorf("after reset, %s = %q, want default %q", first.action, got, firstDefault)
	}
	if got := second.ptr.Keys()[0]; got != secondDefault {
		t.Errorf("after reset, untouched action %s = %q, want default %q", second.action, got, secondDefault)
	}
}
