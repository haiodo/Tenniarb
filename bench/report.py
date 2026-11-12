#!/usr/bin/env python3
"""Print the runs of the scenario bench: one line per run and a pass rate per model."""
import glob
import json
import os
import sys

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "runs")


def load(path):
    try:
        with open(path) as f:
            return json.load(f)
    except Exception:
        return {}


def main():
    runs = []
    for path in sorted(glob.glob(os.path.join(ROOT, "*", "*", "verdict.json"))):
        run = os.path.dirname(path)
        model = os.path.basename(os.path.dirname(run))
        sid = os.path.basename(run)
        verdict = load(path)
        stats = load(os.path.join(run, "stats.json"))
        failed = [c.get("name") for c in verdict.get("criteria", []) if not c.get("pass")]
        runs.append({"model": model, "id": sid, "pass": bool(verdict.get("pass")), "failed": failed,
                     "calls": stats.get("mcp_calls", 0), "bytes": stats.get("mcp_out_bytes", 0),
                     "tokens": stats.get("tokens", 0), "peak": stats.get("peak_tokens", 0),
                     "ctx": stats.get("ctx", 0), "over": stats.get("ctx_over", False),
                     "tools": stats.get("tools", {})})

    print(f"{'model':34} {'scenario':16} {'pass':4} {'calls':5} {'KB':>6} {'tokens':>7} {'peak':>7} {'ctx':>6}  failed")
    per_model = {}
    for r in runs:
        per_model.setdefault(r["model"], []).append(r)
        print(f"{r['model']:34} {r['id']:16} {'yes' if r['pass'] else 'NO':4} {r['calls']:5} "
              f"{r['bytes'] // 1024:6} {r['tokens']:7} {r['peak']:7} {r['ctx']:6}  {','.join(r['failed'])}"
              + (" OVER" if r['over'] else ""))
    print()
    for model, rs in per_model.items():
        good = sum(1 for r in rs if r["pass"])
        calls = sum(r["calls"] for r in rs) / len(rs)
        tokens = sum(r["tokens"] for r in rs) / len(rs)
        print(f"{model}: {good}/{len(rs)} pass, {calls:.1f} calls and {tokens:.0f} tokens per run on average")
    return 0 if all(r["pass"] for r in runs) else 1


if __name__ == "__main__":
    sys.exit(main())
