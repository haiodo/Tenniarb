# Measuring an agent on a design task

What costs time and tokens in a run is not the file I/O: it is how many times the agent has to come back
for a picture, a coordinate or a piece of a file. These runs make that visible and comparable.

## Turning on the log

`TENNIARB_MCP_LOG=/tmp/run.log` makes every MCP tool call append one JSON line:

```json
{"tool":"layout","in":58,"out":1680,"ms":21}
```

`in` and `out` are bytes of the request and the answer; `out/4` is a rough token count of what the answer
adds to the agent's context. Images count as their PNG size here, so a run that renders is cheaper in this
log than it is in tokens — read the render lines as "at least this much".

```bash
TENNIARB_MCP_LOG=/tmp/run.log <agent with the tenniarb MCP>
jq -s 'group_by(.tool) | map({tool: .[0].tool, calls: length, out: (map(.out) | add)})' /tmp/run.log
jq -s 'map(.out) | add' /tmp/run.log            # the whole context cost of the run
jq -s '[.[] | select(.tool != "read")][0]' /tmp/run.log   # the first call that is not a read: the start of work
```

## The tasks

All four are on the platform design, and each has a check that needs no judgement call.

| Task | What the agent is asked | Check |
|---|---|---|
| `plates` | every card of `Libraries.ui_kit.Forms` sits inside its plate | `layout` reports no `sits on … and leaves it` |
| `dark` | a dark twin of one section: a `§ Name/dark` plate with the same components on it | `read` shows the new plate, `render` shows the components on it, `validate` has 0 errors |
| `rename` | rename a component and keep every `use` valid | `validate` 0 errors, `get_state {find}` lists the same uses |
| `build` | a screen of 1200x760 from three components of the library, no warnings | `render` gives that size and 0 warnings |

## This repository's own comparison

`BENCHDST=<workspace with PlatformDesign.tenn> go test ./mcpserver -run TestBenchWorkflows -v` runs the
same job twice on a copy of the workspace: reading the files whole and looking at pictures, and the node
tools (`layout`, `design`). It prints the calls and the bytes of each way.

The first number to watch is the first call: `get_state` with no file is what a run pays to see the
workspace at all. On this tree it was 107 317 bytes (≈27 000 tokens) before the cap and 12 038 (≈3 000)
after, for the same next step, because the cut names `get_state {file}` and `find`.

Numbers of 2026-11-05 on the platform design: the files-and-pictures way took 4 calls and 138 735 bytes
(≈34 700 tokens), the node way 4 calls and 13 985 bytes (≈3 500 tokens) for the same answer and the same
change.
