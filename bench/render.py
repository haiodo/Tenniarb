#!/usr/bin/env python3
"""Render a .tenn element to PNG through the tenniarb MCP proxy, with the same renderer the agents use.

usage: bench/render.py <workspace> <file> <element> <out.png> [scale]
"""
import json
import os
import subprocess
import sys

MCP = os.environ.get("TENNIARB_MCP_BIN", os.path.expanduser("~/Develop/go/bin/tenniarb-mcp"))


def call(proc, msg):
    proc.stdin.write(json.dumps(msg) + "\n")
    proc.stdin.flush()


def read(proc):
    while True:
        line = proc.stdout.readline()
        if not line:
            raise SystemExit("render: the proxy closed the stream")
        try:
            m = json.loads(line)
        except Exception:
            continue
        if m.get("id") is not None:
            return m


def main():
    if len(sys.argv) < 5:
        raise SystemExit(__doc__)
    workspace, file, element, out = sys.argv[1:5]
    scale = float(sys.argv[5]) if len(sys.argv) > 5 else 1.0
    proc = subprocess.Popen([MCP], cwd=workspace, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, text=True)
    call(proc, {"jsonrpc": "2.0", "id": 1, "method": "initialize",
                "params": {"protocolVersion": "2024-11-05", "capabilities": {},
                           "clientInfo": {"name": "tenniarb-bench", "version": "0"}}})
    read(proc)
    call(proc, {"jsonrpc": "2.0", "method": "notifications/initialized"})
    call(proc, {"jsonrpc": "2.0", "id": 2, "method": "tools/call",
                "params": {"name": "render", "arguments": {"file": file, "element": element, "scale": scale}}})
    m = read(proc)
    proc.stdin.close()
    proc.wait(timeout=10)
    if "error" in m:
        raise SystemExit("render: " + json.dumps(m["error"])[:500])
    content = m.get("result", {}).get("content", [])
    for part in content:
        if part.get("type") == "image":
            import base64
            with open(out, "wb") as f:
                f.write(base64.b64decode(part["data"]))
            print(out)
            return
    raise SystemExit("render: no image in the answer: " + json.dumps(m)[:800])


if __name__ == "__main__":
    main()
