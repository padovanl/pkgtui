//go:build e2e

package e2e

import (
	"testing"
	"time"
)

// TestSearchAndViewDetails covers the catalog search flow: "/" to search,
// results render, enter opens details for the selected package, esc goes
// back to the list. Searches for "bash", which is present on essentially
// every Ubuntu image (including GitHub's ubuntu-latest runners), so the
// result — and its details — are there regardless of environment.
func TestSearchAndViewDetails(t *testing.T) {
	h := newHarness(t)
	h.waitReady()

	h.sendText("/")
	h.waitFor("search packages", 3*time.Second)

	h.sendText("bash")
	h.sendKey("enter")
	h.waitFor("results for", 5*time.Second)
	h.waitFor("bash", 3*time.Second)

	h.sendKey("enter") // open details for the selected (first) result
	h.waitFor("Package:", 3*time.Second)
	h.waitFor("esc/enter: back", 2*time.Second)

	h.sendKey("esc")
	h.waitForAbsent("Package:", 3*time.Second)
	h.waitFor("results for", 2*time.Second) // back on the results list
}
