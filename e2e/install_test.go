//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestInstallConfirmShowsLiveOutputThenDismisses covers the privileged-
// action flow end to end: confirm dialog, the real pty-backed live output
// box (sudo prompt or real apt output, depending on whether the test
// environment has passwordless sudo), ctrl+c interrupting the child
// rather than the whole app, the red "failed" box staying up until a
// keypress, and dismissing back to the list.
//
// Deliberately never lets the install actually finish: ctrl+c is sent as
// soon as the live output box appears, so this never installs anything
// for real, regardless of environment (a real completed install would
// need passwordless sudo to be deterministic in CI and would leave a
// package behind — neither is something this suite should depend on or
// cause). Searches "cowsay", present in every Ubuntu apt catalog and
// vanishingly unlikely to already be installed.
func TestInstallConfirmShowsLiveOutputThenDismisses(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	h.sendText("/")
	h.waitFor("search packages", 3*time.Second)
	h.sendText("cowsay")
	h.sendKey("enter")
	h.waitFor("results for", 5*time.Second)
	// The search box echoes the typed query ("🔍 cowsay") above the
	// results, and also contains "cowsay" — skip it explicitly so this
	// checks the actual result row's status symbol, not the query box.
	line := h.waitForResultRow("cowsay", 3*time.Second)
	if !strings.Contains(line, "○") {
		t.Fatalf("expected \"cowsay\" to show as not installed (○) on a clean test environment, got: %q — a previous test run (or something else on this machine) likely left it installed; `apt-get remove cowsay` and retry", line)
	}

	h.sendText("i")
	h.waitFor("Install cowsay?", 3*time.Second)

	h.sendText("y")
	h.waitFor(" — running ", 5*time.Second)

	h.sendText("\x03") // ctrl+c: interrupt the child, not pkgtui itself
	h.waitForAny([]string{" — failed ", " — done "}, 5*time.Second)
	h.waitFor("press any key to continue", 3*time.Second)

	h.sendKey("enter")
	h.waitForAbsent("press any key to continue", 3*time.Second)
	h.waitFor("results for", 3*time.Second)
}
