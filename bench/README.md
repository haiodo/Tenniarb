# Agent design benchmark

A run is one design task in a copy of `workspace/`: an agent draws in `.tenn` through the tenniarb MCP,
the run records what it did, and a second agent checks the result against written criteria. The point is
to see whether the skill and the tools are enough for the task, and to compare models on the same task.

## Layout

```
workspace/            the fixture: PlatformDesign.tenn, lib/ui_kit.tenn (Button, Card, Avatar, Badge, Panel),
                      lib/forms.tenn (Field, Dialog), apps/home.tenn, apps/empty.tenn, assets/logo.png
scenarios/<id>/       task.md (the prompt), check.json (the criteria), seed/ (optional files over the fixture)
runs/<model>/<id>/    one run: workspace/, agent.jsonl, mcp.jsonl, check.json, verifier.jsonl, verdict.json
run.sh <id> [model]   copy the fixture and run the agent
verify.sh <run> [model]  run the verifier over a finished run
```

## A run

`bench/run.sh b01-card` does this:

1. copies `workspace/` to `runs/<model>/<id>/workspace/`;
2. writes `.bench/task.md`, `.bench/check.json` and a project `opencode.json` that keeps only the
   tenniarb MCP and logs its calls to `runs/<model>/<id>/mcp.jsonl`;
3. runs `opencode run -m <model> --format json "<task>"` in the workspace and keeps the JSON stream in
   `agent.jsonl` (text, tool calls, tokens, cost);
4. runs `tenniarb check -json` on the file of the scenario and keeps the result in `check.json`;
5. leaves the result workspace as the agent left it.

`bench/verify.sh <run-dir>` starts a second agent in the result workspace, gives it the task and the
criteria and asks for only JSON:

```json
{"pass": false, "criteria": [{"name": "card", "pass": true, "note": ""}], "notes": "the title is 18pt"}
```

Both agents run on the model given to the script; the default is `opencode-go/deepseek-v4.1-flash`.

## The criteria

`check.json` names the file and element, the checks a script can make without judgement
(`no_errors`, `no_warnings`), and a list of `criteria` for the verifier. A criterion says what to look
at and what must be true, in the terms of the tools: a path from `get_state`, a property from `read`, a
number from `layout`, a picture from `render`. Write the truth as it would be seen in the app, not as a
particular text: "a card 320 wide" allows any formatting.

The verifier answers every criterion and the run passes only when all of them pass. Machine checks that
fail make the run fail regardless of the verifier.

## Scenarios

Tiers: `b` basic syntax, `l` layout, `c` components, `s` states, `d` diagrams, `t` tool discipline,
`r` a real page of the platform design, `v` a picture to reproduce. The target set is 100 plus the 8
visual tasks in `scenarios.md`.

## Pictures and context

A `v` scenario has `reference.png` next to `task.md`: the runner attaches it to the message, and after the
run it renders the result element through the same MCP renderer (`render.py`) and compares the two with
`compare.py` — `mae` (mean channel difference), `match` (share of pixels within a tolerance) and a diff
image. The thresholds are in `check.json` under `visual`. The first one, a 360x420 login, scored
`mae 0.33, match 0.994` on flash.

Every `check.json` has a `ctx` budget: 50k for the fixture-only tiers, 200k for library work and simple
pictures, 500k for a 1920x1080 page rebuilt from a picture. The runner records the peak request size in
`stats.json` and sets `ctx_over` when the run went past it.

## Model list

`opencode-go/deepseek-v4.1-flash` first, then the other three once the tasks are stable. A model is
compared on: pass rate, tool calls and bytes (`mcp.jsonl`), tokens and cost (`agent.jsonl`), and the
first failing criterion.

## Notes

- The fixture is small on purpose: a run must finish in a few turns so that 100 scenarios are affordable.
  Real pages go in the `r` tier against the platform design and are run less often.
- Do not edit `workspace/` by hand to make a scenario pass; put the setup in `scenarios/<id>/seed/`.
- A scenario that a model can pass by ignoring the skill is not a scenario; make the check depend on the
  library, the entry, or a tool the skill recommends.
- A plain item in an otherwise flat element makes it a classic diagram, and the renderer draws the item's
  name as a label over it. A scene task puts its nodes inside a container with `auto-layout` (or tells the
  agent to add a frame with `auto-layout abs`); the first `b09-image` run showed the name `Logo` printed
  over the picture because the task did not.
