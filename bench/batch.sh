#!/usr/bin/env bash
# Run and verify every scenario that has a check, at most JOBS agents at once, and write the baseline.
set -euo pipefail
BENCH="$(cd "$(dirname "$0")" && pwd)"
MODEL="${1:-opencode-go/deepseek-v4.1-flash}"
JOBS="${JOBS:-2}"
FORCE="${FORCE:-0}"
OUT="$BENCH/baselines/${MODEL//\//_}"
mkdir -p "$OUT"
LOG="$OUT/log.txt"
: > "$LOG"

run_one() {
  local sid="$1"
  local run="$BENCH/runs/${MODEL//\//_}/$sid"
  local log="$OUT/$sid.log"
  printf '[%s] start %s\n' "$(date +%H:%M:%S)" "$sid" >> "$LOG"
  mkdir -p "$run"
  "$BENCH/run.sh" "$sid" "$MODEL" > "$log" 2>&1 || true
  "$BENCH/verify.sh" "$run" "$MODEL" >> "$log" 2>&1 || true
  "$BENCH/snapshot.sh" "$MODEL" "$sid" || true
  printf '[%s] done  %s %s\n' "$(date +%H:%M:%S)" "$sid" \
    "$(python3 -c "import json,sys;print(json.load(open(sys.argv[1])).get('pass','?'))" "$OUT/$sid/verdict.json" 2>/dev/null || echo no-verdict)" >> "$LOG"
}

running=0
for d in "$BENCH"/scenarios/*/; do
  sid="$(basename "$d")"
  [ -f "$d/check.json" ] || continue
  if [ "$FORCE" = 0 ] && [ -f "$OUT/$sid/verdict.json" ]; then continue; fi
  run_one "$sid" &
  running=$((running + 1))
  if [ "$running" -ge "$JOBS" ]; then
    wait -n || true
    running=$((running - 1))
  fi
done
wait
"$BENCH/report.py" > "$OUT/summary.txt" 2>&1 || true
