#!/bin/sh
# Mirrors postinstall.sh: refreshes the same caches on removal so the
# launcher entry/icon actually disappears instead of lingering stale.
set -e

if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database -q /usr/share/applications || true
fi

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
	gtk-update-icon-cache -q /usr/share/icons/hicolor || true
fi

exit 0
