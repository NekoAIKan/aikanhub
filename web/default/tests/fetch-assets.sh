#!/usr/bin/env bash
# Mirror reference assets into ./assets/ so they can be inspected locally.
# The test harness still references the upstream URLs (Volcano fetches
# reference content from its own side, so it must be publicly reachable).
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p assets

declare -A ASSETS=(
  [first.jpg]="https://picsum.photos/seed/aikanhub-first/800/600"
  [last.jpg]="https://picsum.photos/seed/aikanhub-last/800/600"
  [ref1.jpg]="https://picsum.photos/seed/aikanhub-ref1/800/600"
  [ref2.jpg]="https://picsum.photos/seed/aikanhub-ref2/800/600"
  # 720p, ~10s, ~1 MB — meets Seedance's [300, 6000] px and 409600 pixel-area
  # minimums (the w3schools 320x240 sample is too small).
  [reference.mp4]="https://test-videos.co.uk/vids/bigbuckbunny/mp4/h264/720/Big_Buck_Bunny_720_10s_1MB.mp4"
  # ~7 s WAV, well above Seedance's 1.8s audio minimum.
  [reference.wav]="https://www.kozco.com/tech/piano2.wav"
)

for name in "${!ASSETS[@]}"; do
  url="${ASSETS[$name]}"
  if [ -s "assets/$name" ]; then
    echo "  [skip] assets/$name (already present)"
  else
    echo "  [get ] $url -> assets/$name"
    curl -sSL "$url" -o "assets/$name"
  fi
done

echo
echo "Done. Files:"
ls -lh assets/
