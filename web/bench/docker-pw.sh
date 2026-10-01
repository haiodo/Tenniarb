#!/bin/sh
# Linux Chromium + WebKitGTK numbers. Run from web/bench (PWV must match the image tag):
# docker run --rm --platform linux/arm64 -v "$PWD":/w -e PWV=1.63.0 mcr.microsoft.com/playwright:v1.63.0-noble sh /w/docker-pw.sh
set -e
cd /tmp && mkdir -p w && cp /w/bench.html /w/scene.js /w/pw.mjs . && npm init -y >/dev/null && npm i playwright@$PWV >/dev/null 2>&1
for b in chromium webkit; do xvfb-run -a -s "-screen 0 1920x1080x24" node pw.mjs $b headed 1 /w/results/linux-$b-headed.json; done
