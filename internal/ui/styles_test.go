package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestContrastingBadgeTextPicksTheBetterOption guards against badge text
// being hardcoded to a single color regardless of the background: black
// reads fine on a light/bright badge and poorly on a dark one, and vice
// versa for white, so the picker has to actually choose per background.
func TestContrastingBadgeTextPicksTheBetterOption(t *testing.T) {
	cases := []struct {
		name string
		bg   string
		want lipgloss.Color
	}{
		{"near-black background needs white text", "234", badgeTextWhite},
		{"bright yellow background needs black text", "220", badgeTextBlack},
		{"near-white background needs black text", "255", badgeTextBlack},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := contrastingBadgeText(c.bg); got != c.want {
				t.Errorf("contrastingBadgeText(%q) = %v, want %v", c.bg, got, c.want)
			}
		})
	}
}

// TestContrastingBadgeTextMeetsMinimumForEveryTheme is the Go-side twin of
// scripts/check-theme-contrast.py: every built-in theme's badge
// backgrounds (accent/highlight/danger) must pair with whichever of
// black/white contrastingBadgeText picks at a ratio a human can actually
// read, not just barely over the WCAG "large text" floor.
func TestContrastingBadgeTextMeetsMinimumForEveryTheme(t *testing.T) {
	const minRatio = 4.5
	for _, name := range ThemeNames() {
		p, ok := palettes[name]
		if !ok {
			t.Fatalf("theme %q not in palettes map", name)
		}
		for _, bg := range []struct {
			label string
			code  string
		}{
			{"highlight", p.highlight},
			{"accent", p.accent},
			{"danger", p.danger},
		} {
			text := contrastingBadgeText(bg.code)
			ratio := contrastRatio(xterm256Luminance(string(text)), xterm256Luminance(bg.code))
			if ratio < minRatio {
				t.Errorf("theme %q: badge text on %s (bg=%s) = %.2f, want >= %.1f", name, bg.label, bg.code, ratio, minRatio)
			}
		}
	}
}
