#!/usr/bin/env python3
"""Computes WCAG contrast ratios for every color pair pkgtui's themes
actually render (badge text on accent/highlight backgrounds, and each
status/dim color against a dark terminal background), so a new theme or a
tweaked value can be sanity-checked without a screenshot round-trip.

Run after editing internal/ui/styles.go's `palettes` map (keep the two in
sync manually; this script intentionally has no Go dependency).

Usage: python3 scripts/check-theme-contrast.py
Exits non-zero if any pair falls below the WCAG "large text" minimum (3:1).
"""

import sys

# Mirrors internal/ui/styles.go's palettes map.
PALETTES = {
    "default":   dict(fg="252", accent="42",  warn="220", danger="203", muted="245", highlight="39"),
    "dracula":   dict(fg="253", accent="84",  warn="228", danger="212", muted="245", highlight="141"),
    "nord":      dict(fg="251", accent="108", warn="222", danger="167", muted="245", highlight="110"),
    "solarized": dict(fg="244", accent="70",  warn="136", danger="196", muted="245", highlight="45"),
    "gruvbox":   dict(fg="223", accent="142", warn="214", danger="167", muted="245", highlight="109"),
    "catppuccin":  dict(fg="189", accent="151", warn="223", danger="210", muted="245", highlight="183"),
    "tokyo-night": dict(fg="195", accent="149", warn="179", danger="204", muted="245", highlight="111"),
    "monokai":     dict(fg="255", accent="154", warn="221", danger="197", muted="245", highlight="80"),
}
BADGE_TEXT = "0"     # colorBadgeText: text on accent/highlight/danger badges
TERMINAL_BG = "235"  # stand-in for a typical dark terminal background
MIN_RATIO = 3.0      # WCAG AA, large/bold text


def xterm256_to_rgb(code):
    n = int(code)
    if n < 16:
        table = [
            (0, 0, 0), (205, 0, 0), (0, 205, 0), (205, 205, 0), (0, 0, 238), (205, 0, 205), (0, 205, 205), (229, 229, 229),
            (127, 127, 127), (255, 0, 0), (0, 255, 0), (255, 255, 0), (92, 92, 255), (255, 0, 255), (0, 255, 255), (255, 255, 255),
        ]
        return table[n]
    if n < 232:
        n -= 16
        r, g, b = n // 36, (n % 36) // 6, n % 6
        lvl = lambda x: 0 if x == 0 else 55 + x * 40
        return (lvl(r), lvl(g), lvl(b))
    v = 8 + (n - 232) * 10
    return (v, v, v)


def luminance(rgb):
    def chan(c):
        c = c / 255.0
        return c / 12.92 if c <= 0.03928 else ((c + 0.055) / 1.055) ** 2.4
    r, g, b = rgb
    return 0.2126 * chan(r) + 0.7152 * chan(g) + 0.0722 * chan(b)


def contrast(fg_code, bg_code):
    l1 = luminance(xterm256_to_rgb(fg_code))
    l2 = luminance(xterm256_to_rgb(bg_code))
    l1, l2 = max(l1, l2), min(l1, l2)
    return (l1 + 0.05) / (l2 + 0.05)


def main():
    failures = []
    print(f"{'theme':<10} {'pair':<38} {'ratio':>6}")
    for name, p in PALETTES.items():
        pairs = [
            ("badge text on highlight (header/tab/hints)", BADGE_TEXT, p["highlight"]),
            ("badge text on accent (active tab)", BADGE_TEXT, p["accent"]),
            ("badge text on danger (warning banner)", BADGE_TEXT, p["danger"]),
            ("muted on terminal bg (dim text)", p["muted"], TERMINAL_BG),
            ("warn on terminal bg (upgradable bullet)", p["warn"], TERMINAL_BG),
            ("danger on terminal bg (security bullet)", p["danger"], TERMINAL_BG),
            ("accent on terminal bg (installed bullet)", p["accent"], TERMINAL_BG),
            ("highlight on terminal bg", p["highlight"], TERMINAL_BG),
            ("fg on terminal bg (titles)", p["fg"], TERMINAL_BG),
        ]
        for label, fg, bg in pairs:
            ratio = contrast(fg, bg)
            flag = "" if ratio >= MIN_RATIO else "  <-- LOW"
            print(f"{name:<10} {label:<42} {ratio:5.2f}{flag}")
            if ratio < MIN_RATIO:
                failures.append(f"{name}: {label} = {ratio:.2f}")
        print()

    if failures:
        print(f"{len(failures)} pair(s) below {MIN_RATIO}:1:", file=sys.stderr)
        for f in failures:
            print(f"  {f}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
