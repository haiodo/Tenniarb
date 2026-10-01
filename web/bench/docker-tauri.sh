#!/bin/bash
# Tauri on system webkit2gtk. Run from web/bench after `npm run tauri-dist`:
# docker run --rm --platform linux/arm64 -v "$PWD":/w ubuntu:24.04 bash /w/docker-tauri.sh
set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl build-essential pkg-config libwebkit2gtk-4.1-dev libgtk-3-dev libsoup-3.0-dev xvfb xauth libayatana-appindicator3-dev librsvg2-dev ca-certificates >/dev/null
dpkg -l libwebkit2gtk-4.1-0 | tail -1
curl -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal >/dev/null 2>&1
. ~/.cargo/env; rustc --version
mkdir -p /build && cd /build && tar -C /w/tauri --exclude=src-tauri/target -cf - . | tar xf -
cd src-tauri
S=$(date +%s); cargo build --release 2>&1 | tail -3; echo "linux release build s: $(( $(date +%s)-S ))"
ls -la target/release/bench
export BENCH_OUT=/w/results/linux-tauri.json BENCH_EXIT=1
for m in default; do
  xvfb-run -a -s "-screen 0 1280x800x24" bash -c 'S=$(date +%s.%N); ./target/release/bench & P=$!; sleep 8; ps -eo pid,rss,comm | grep -i "bench\|WebKit"; wait $P; echo "runtime s: $(echo "$(date +%s.%N)-$S" | bc)"'
done
ls -la /w/results/linux-tauri.json
