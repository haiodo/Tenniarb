#!/usr/bin/env bash
# One run of a design scenario: copy the fixture out of the repository, run an agent on the task, keep
# its trace and result. The workspace lives in a temp dir so the repository's AGENTS.md and config do
# not reach the agent; only the fixture does.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BENCH="$ROOT/bench"
MCP_BIN="${TENNIARB_MCP_BIN:-$HOME/Develop/go/bin/tenniarb-mcp}"
CLI="${TENNIARB_BIN:-$(command -v tenniarb || true)}"

SID="${1:?usage: bench/run.sh <scenario> [model]}"
MODEL="${2:-opencode-go/deepseek-v4.1-flash}"
SCEN="$BENCH/scenarios/$SID"
[ -d "$SCEN" ] || { echo "no scenario $SID" >&2; exit 2; }

OUT="$BENCH/runs/${MODEL//\//_}/$SID"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/tenniarb-bench.XXXXXX")"
WORK="$TMP/workspace"
trap 'rm -rf "$TMP"' EXIT
WORK_REAL="$(mkdir -p "$WORK" && cd "$WORK" && pwd -P)"
# The shared service fixes its log path at start; a bench-owned one always logs where the run looks.
SERVICE_DIR="$BENCH/.service"
MCP_LOG="$BENCH/.mcp-calls.log"
BEFORE=0
[ -f "$MCP_LOG" ] && BEFORE="$(wc -l < "$MCP_LOG" | tr -d ' ')"

rm -rf "$OUT"
mkdir -p "$OUT" "$WORK/.opencode"
cp -R "$BENCH/workspace/." "$WORK/"
if [ -d "$SCEN/seed" ]; then cp -R "$SCEN/seed/." "$WORK/"; fi
cp -R "$WORK" "$OUT/baseline"
rm -rf "$OUT/baseline/.opencode"
cp "$SCEN/task.md" "$OUT/task.md"

cat > "$WORK/.opencode/opencode.json" <<JSON
{
  "\$schema": "https://opencode.ai/config.json",
  "model": "$MODEL",
  "mcp": {
    "fusion": { "enabled": false },
    "chrome-devtools-brave": { "enabled": false },
    "servers": {
      "tenniarb": {
        "type": "local",
        "command": ["$MCP_BIN", "-dir", "$SERVICE_DIR"],
        "environment": { "TENNIARB_MCP_LOG": "$MCP_LOG" }
      }
    }
  }
}
JSON

att=()
if [ -f "$SCEN/reference.png" ]; then
  cp "$SCEN/reference.png" "$OUT/reference.png"
  att=(--file "$SCEN/reference.png")
fi

status=0
( cd "$WORK" && opencode run -m "$MODEL" --auto --archived --format json "${att[@]}" "$(cat "$SCEN/task.md")" ) \
  > "$OUT/agent.jsonl" 2> "$OUT/agent.err" || status=$?
echo "$status" > "$OUT/agent.status"

if [ -f "$MCP_LOG" ]; then
  tail -n +$((BEFORE + 1)) "$MCP_LOG" | python3 -c '
import json, sys
root = sys.argv[1]
for line in sys.stdin:
    try:
        if json.loads(line).get("root") == root:
            print(line, end="")
    except Exception:
        pass
' "$WORK_REAL" > "$OUT/mcp.jsonl"
fi

mkdir -p "$OUT/workspace/.bench"
cp -R "$WORK/." "$OUT/workspace/"
cp "$SCEN/task.md" "$SCEN/check.json" "$OUT/workspace/.bench/"
cp "$SCEN/check.json" "$OUT/check.criteria.json"

file="$(python3 -c "import json,sys;print(json.load(open(sys.argv[1])).get('file',''))" "$SCEN/check.json")"
if [ -n "$file" ] && [ -n "$CLI" ]; then
  ( cd "$OUT/workspace" && "$CLI" check -json "$file" ) > "$OUT/check.json" 2> "$OUT/check.err" || true
fi

visual="$(python3 -c "
import json,sys
v=json.load(open(sys.argv[1])).get('visual')
print(json.dumps(v) if v else '')" "$SCEN/check.json")"
if [ -n "$visual" ] && [ -n "$file" ]; then
  vfile="$(python3 -c 'import json,sys;print(json.loads(sys.argv[1])["file"])' "$visual")"
  velem="$(python3 -c 'import json,sys;print(json.loads(sys.argv[1])["element"])' "$visual")"
  if "$ROOT/bench/render.py" "$OUT/workspace" "$vfile" "$velem" "$OUT/render.png" 1 > "$OUT/render.out" 2> "$OUT/render.err"; then
    "$ROOT/bench/compare.py" "$SCEN/reference.png" "$OUT/render.png" --out "$OUT/diff.png" > "$OUT/compare.json" || true
  fi
fi

python3 - "$OUT" <<'PY'
import json, os, sys
out = sys.argv[1]
calls = errors = out_bytes = 0
path = os.path.join(out, "mcp.jsonl")
for line in open(path) if os.path.exists(path) else []:
    try:
        c = json.loads(line)
    except Exception:
        continue
    calls += 1
    out_bytes += c.get("out", 0)
    errors += 0 if c.get("ok") else 1
tokens = cost = peak = 0
tools = {}
for line in open(os.path.join(out, "agent.jsonl")):
    try:
        e = json.loads(line)
    except Exception:
        continue
    if e.get("type") == "step_finish":
        p = e.get("part", {})
        t = p.get("tokens", {})
        c = t.get("cache", {})
        tokens += t.get("input", 0) + t.get("output", 0)
        peak = max(peak, t.get("input", 0) + t.get("output", 0) + c.get("read", 0) + c.get("write", 0))
        cost += int(round(p.get("cost", 0) * 1000000))
    if e.get("type") == "tool_use":
        p = e.get("part", {})
        name = p.get("tool", "?")
        tools[name] = tools.get(name, 0) + 1
        for c in (p.get("state", {}).get("metadata", {}) or {}).get("metadata", {}).get("toolCalls", []) or []:
            n = c.get("tool", "?")
            tools[n] = tools.get(n, 0) + 1
budget = 0
try:
    budget = json.load(open(os.path.join(out, "check.criteria.json"))).get("ctx", 0)
except Exception:
    pass
open(os.path.join(out, "stats.json"), "w").write(json.dumps({"mcp_calls": calls, "mcp_failed": errors,
    "mcp_out_bytes": out_bytes, "tokens": tokens, "peak_tokens": peak, "ctx": budget,
    "ctx_over": bool(budget and peak > budget), "cost_micro_usd": cost, "tools": tools}, indent=1) + "\n")
print(json.dumps({"run": out, "mcp_calls": calls, "mcp_out_bytes": out_bytes, "tokens": tokens,
    "tools": tools}))
PY
