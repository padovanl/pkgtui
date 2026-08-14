#!/usr/bin/env python3
"""Computes WCAG contrast ratios for every color pair pkgtui's themes
actually render (badge text on accent/highlight/danger backgrounds, and
each status/dim color against a dark terminal background), so a new theme
or a tweaked value can be sanity-checked without a screenshot round-trip.

Badge text isn't a fixed color: pick_badge_text mirrors
internal/ui/styles.go's contrastingBadgeText, choosing whichever of black/
white contrasts better against that specific badge background, instead of
assuming black always works. A fixed black looked fine against some
themes' accent/highlight/danger and unreadable against others — this is
what the badges actually do at runtime, not a looser approximation of it.

Run after editing internal/ui/styles.go's `palettes` map (keep the two in
sync manually; this script intentionally has no Go dependency).

Usage: python3 scripts/check-theme-contrast.py
Exits non-zero if any pair falls below its minimum ratio.
"""

import sys

# Mirrors internal/ui/styles.go's palettes map.
PALETTES = {
    "default":     dict(fg="252", accent="42",  warn="220", danger="203", muted="245", highlight="39"),
    "dracula":     dict(fg="253", accent="84",  warn="228", danger="212", muted="245", highlight="141"),
    "nord":        dict(fg="251", accent="108", warn="222", danger="167", muted="245", highlight="110"),
    "solarized":   dict(fg="244", accent="70",  warn="136", danger="196", muted="245", highlight="45"),
    "gruvbox":     dict(fg="223", accent="142", warn="214", danger="167", muted="245", highlight="109"),
    "catppuccin":  dict(fg="189", accent="151", warn="223", danger="210", muted="245", highlight="183"),
    "tokyo-night": dict(fg="195", accent="149", warn="179", danger="204", muted="245", highlight="111"),
    "monokai":     dict(fg="255", accent="154", warn="221", danger="197", muted="245", highlight="80"),
    "darcula":     dict(fg="250", accent="107", warn="215", danger="173", muted="245", highlight="74"),
    "vscode":      dict(fg="254", accent="78",  warn="178", danger="174", muted="245", highlight="33"),
    "ubuntu":      dict(fg="231", accent="64",  warn="100", danger="203", muted="245", highlight="166"),
}
TERMINAL_BG = "235"    # stand-in for a typical dark terminal background
BADGE_MIN_RATIO = 4.5  # WCAG AA, normal text — badge text is short but not bold-large enough to lean on the looser 3:1 minimum
TEXT_MIN_RATIO = 3.0   # WCAG AA, large/bold text — the existing plain-text-on-terminal-bg pairs


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


def pick_badge_text(bg_code):
    """Whichever of black(0)/white(15) contrasts better against bg_code."""
    return "15" if contrast("15", bg_code) > contrast("0", bg_code) else "0"


def main():
    failures = []
    print(f"{'theme':<10} {'pair':<42} {'ratio':>6}")
    for name, p in PALETTES.items():
        badge_pairs = [
            ("badge text on highlight (header/tab/hints)", p["highlight"]),
            ("badge text on accent (active tab)", p["accent"]),
            ("badge text on danger (warning banner)", p["danger"]),
        ]
        text_pairs = [
            ("muted on terminal bg (dim text)", p["muted"], TERMINAL_BG),
            ("warn on terminal bg (upgradable bullet)", p["warn"], TERMINAL_BG),
            ("danger on terminal bg (security bullet)", p["danger"], TERMINAL_BG),
            ("accent on terminal bg (installed bullet)", p["accent"], TERMINAL_BG),
            ("highlight on terminal bg", p["highlight"], TERMINAL_BG),
            ("fg on terminal bg (titles)", p["fg"], TERMINAL_BG),
        ]
        for label, bg in badge_pairs:
            text = pick_badge_text(bg)
            ratio = contrast(text, bg)
            flag = "" if ratio >= BADGE_MIN_RATIO else "  <-- LOW"
            print(f"{name:<10} {label:<42} {ratio:5.2f} (text={text}){flag}")
            if ratio < BADGE_MIN_RATIO:
                failures.append(f"{name}: {label} = {ratio:.2f}")
        for label, fg, bg in text_pairs:
            ratio = contrast(fg, bg)
            flag = "" if ratio >= TEXT_MIN_RATIO else "  <-- LOW"
            print(f"{name:<10} {label:<42} {ratio:5.2f}{flag}")
            if ratio < TEXT_MIN_RATIO:
                failures.append(f"{name}: {label} = {ratio:.2f}")
        print()

    if failures:
        print(f"{len(failures)} pair(s) below their minimum ratio:", file=sys.stderr)
        for f in failures:
            print(f"  {f}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
