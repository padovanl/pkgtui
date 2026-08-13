#!/bin/sh
# Refreshes the desktop entry/icon caches so the launcher icon shows up
# immediately, without needing to log out and back in. Both tools are
# desktop-environment packages, not dependencies of pkgtui itself (a
# headless/server install has neither, and won't ever look for either
# cache), so this is a no-op there.
set -e

if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database -q /usr/share/applications || true
fi

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
	gtk-update-icon-cache -q /usr/share/icons/hicolor || true
fi

exit 0
