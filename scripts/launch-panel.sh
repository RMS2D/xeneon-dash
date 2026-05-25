#!/usr/bin/env bash
# launch-panel.sh - opens xeneon-dash as a borderless desktop panel on Linux.
# Window geometry persists via per-panel user-data-dir.
# Always-on-top: use your WM (e.g. wmctrl -r :ACTIVE: -b add,above after launch).

set -e

URL="http://localhost:7777"
USER_DATA="${HOME}/.xeneon-dash-panel"

# Find a chromium-family browser.
BROWSER=""
for b in google-chrome google-chrome-stable chromium chromium-browser microsoft-edge; do
  if command -v "$b" >/dev/null 2>&1; then BROWSER="$b"; break; fi
done
if [ -z "$BROWSER" ]; then
  echo "No chromium-family browser found. Install one of: google-chrome, chromium, microsoft-edge." >&2
  exit 1
fi

mkdir -p "$USER_DATA"

ARGS=(
  --app="$URL"
  --user-data-dir="$USER_DATA"
  --no-first-run
  --no-default-browser-check
)

# Hint default geometry on first launch only.
if [ ! -f "$USER_DATA/Default/Preferences" ]; then
  ARGS+=(--window-size=1920,300 --window-position=0,1080)
fi

nohup "$BROWSER" "${ARGS[@]}" >/dev/null 2>&1 &
echo "Panel opened with $BROWSER. Drag/resize freely; geometry remembered for next launch."
echo "Always-on-top (X11): wmctrl -r :ACTIVE: -b add,above"
echo ""
echo "To remove the title bar entirely, install as PWA:"
echo "  Browser menu -> Apps / Install xeneon-dash"
echo "Then launch from your app drawer; opens chromeless."
