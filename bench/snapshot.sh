#!/usr/bin/env bash
# Copy one finished run into the baseline of its model: the verdict, the stats, the picture numbers.
set -euo pipefail
BENCH="$(cd "$(dirname "$0")" && pwd)"
MODEL="${1:?model}"
SID="${2:?scenario}"
RUN="$BENCH/runs/${MODEL//\//_}/$SID"
DST="$BENCH/baselines/${MODEL//\//_}/$SID"
mkdir -p "$DST"
for f in verdict.json stats.json compare.json render.png diff.png; do
  if [ -f "$RUN/$f" ]; then cp "$RUN/$f" "$DST/"; fi
done
