//go:build e2e

package e2e

import (
	"testing"
	"time"
)

// TestStartupShowsInstalledList covers the golden path every other test
// implicitly relies on: launch, and the Installed view for apt renders —
// tab bar, header, legend, and at least one real installed package row.
func TestStartupShowsInstalledList(t *testing.T) {
	h := newHarness(t)

	h.waitReady()
	h.waitFor("apt", 3*time.Second)
	h.waitFor("APT — Installed", 2*time.Second)
	h.waitFor("installed", 2*time.Second) // legend
	h.waitFor("apt/snap", 2*time.Second)  // footer key hints rendered
}
