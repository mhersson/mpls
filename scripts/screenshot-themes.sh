#!/bin/bash
#
# Generate screenshots of mpls demo for all available themes
# Usage: ./scripts/screenshot-themes.sh [output_dir]
#

set -e

OUTPUT_DIR="${1:-/tmp/mpls-screenshots}"
PORT=9999
# BROWSER="google-chrome" # Default to Chrome
BROWSER="/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"

# Verify Edge exists
if [[ ! -x "$BROWSER" ]]; then
  echo "Error: Browser not found at $BROWSER"
  exit 1
fi

# Build mpls if needed
if [[ ! -x ./mpls ]]; then
  echo "Building mpls..."
  CGO_ENABLED=1 go build .
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Get list of themes from the themes directory
THEMES_DIR="internal/previewserver/web/themes"
THEMES=$(ls "$THEMES_DIR"/*.css 2>/dev/null | xargs -n1 basename | sed 's/.css$//')

if [[ -z "$THEMES" ]]; then
  echo "Error: No themes found in $THEMES_DIR"
  exit 1
fi

echo "Output directory: $OUTPUT_DIR"
echo "Found themes: $(echo $THEMES | wc -w | tr -d ' ')"
echo ""

for theme in $THEMES; do
  echo "Capturing: $theme"

  # Start demo server in background
  ./mpls demo --theme "$theme" --port $PORT --no-auto --wait 5s &
  MPLS_PID=$!

  # Wait for server to be ready
  sleep 2

  # Take screenshot with Edge headless
  "$BROWSER" --headless --disable-gpu --screenshot="$OUTPUT_DIR/${theme}.png" \
    --window-size=900,1300 "http://localhost:$PORT" 2>/dev/null

  # Wait for mpls to finish (or kill if still running)
  kill $MPLS_PID 2>/dev/null || true
  wait $MPLS_PID 2>/dev/null || true

  echo "  -> $OUTPUT_DIR/${theme}.png"
done

echo ""
echo "Done! Screenshots saved to $OUTPUT_DIR"
ls -la "$OUTPUT_DIR"/*.png
