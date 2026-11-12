#!/usr/bin/env bash
# Check a finished run against its criteria with a second agent; writes verdict.json.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BENCH="$ROOT/bench"
RUN="${1:?usage: bench/verify.sh <run-dir> [model]}"
MODEL="${2:-opencode-go/deepseek-v4.1-flash}"
[ -d "$RUN/workspace" ] || { echo "no workspace in $RUN" >&2; exit 2; }

( cd "$RUN/workspace" && opencode run -m "$MODEL" --auto --archived --format json "$(cat "$BENCH/verify-prompt.md")" ) \
  > "$RUN/verifier.jsonl" 2> "$RUN/verifier.err" || true

python3 - "$RUN" <<'PY'
import json, re, sys
run = sys.argv[1]
texts = []
for line in open(run + "/verifier.jsonl"):
    try:
        e = json.loads(line)
    except Exception:
        continue
    if e.get("type") == "text":
        texts.append(e.get("part", {}).get("text", ""))
text = "\n".join(texts).strip()
text = re.sub(r"^```(?:json)?|```$", "", text, flags=re.M).strip()
m = re.search(r"\{.*\}", text, re.S)
if not m:
    open(run + "/verdict.json", "w").write(json.dumps({"pass": False, "error": "no JSON in the answer",
        "answer": text[:2000]}, indent=1) + "\n")
    print("verdict: no JSON")
    raise SystemExit(1)
v = json.loads(m.group(0))
open(run + "/verdict.json", "w").write(json.dumps(v, indent=1) + "\n")
print(json.dumps({"pass": v.get("pass"), "criteria": v.get("criteria"), "notes": v.get("notes")}))
PY
