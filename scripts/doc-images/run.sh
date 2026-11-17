#!/usr/bin/env bash
# Regenerates the light/dark screenshots in docs/images and .github/images
# against fictional seed data. usage: scripts/doc-images/run.sh [chrome-binary]
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
work=$(mktemp -d)
pids=()
trap 'kill "${pids[@]}" 2>/dev/null || true; rm -rf "$work"' EXIT

chrome=${1:-${CHROME:-$(command -v google-chrome || command -v chromium || echo "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")}}

(cd "$root" && go build -o "$work/delistd" ./cmd/delistd && go run ./scripts/a11y/seed "$work/seeded")
mkdir -p "$work/empty"
(cd "$root/scripts/doc-images" && npm ci --silent)

# seeded home for the dashboard and Automation page, empty home for the setup wizard
HOME="$work/seeded" "$work/delistd" serve -p 3997 >"$work/seeded.log" 2>&1 & pids+=($!)
HOME="$work/empty" "$work/delistd" serve -p 3998 >"$work/empty.log" 2>&1 & pids+=($!)
for port in 3997 3998; do
  for _ in $(seq 50); do curl -fs -o /dev/null "localhost:$port/setup" && break; sleep 0.2; done
done

node "$root/scripts/doc-images/capture.mjs" "$chrome" "$root" http://localhost:3997 http://localhost:3998
