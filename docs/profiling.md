# Profiling Tenniarb

The Go application has an opt-in `net/http/pprof` endpoint. It serves CPU, heap, allocation and goroutine
profiles without stopping the process. The listener is off by default.

Use a loopback address only. The endpoint has no authentication and exposes stack traces, source paths and
allocation metadata. Do not bind it to `0.0.0.0` or another externally reachable address.

## Start the application with pprof

Build a separate app without replacing the installed one:

```bash
make app
mkdir -p .work
TENNIARB_PPROF=127.0.0.1:6060 \
  build/go/app/Tenniarb.app/Contents/MacOS/tenniarb \
  > .work/tenniarb-pprof.log 2>&1 &
```

For an installed build, replace the binary path with `/Applications/Tenniarb.app/Contents/MacOS/tenniarb`.
Launch the binary directly: `open -a Tenniarb` does not set `TENNIARB_PPROF` for the new process.

The startup log must contain:

```text
tenniarb: pprof: http://127.0.0.1:6060/debug/pprof/
```

Check that the endpoint answers:

```bash
curl -fsS http://127.0.0.1:6060/debug/pprof/ -o /dev/null
```

`make install` stops the running application and MCP processes. Save open documents before running it.
An already running process cannot be given the pprof endpoint retroactively; start a new instrumented
process and reproduce the workload there.

## CPU profile

Record 30 seconds while the problem is visible:

```bash
curl -fsS 'http://127.0.0.1:6060/debug/pprof/profile?seconds=30' \
  -o .work/tenniarb-cpu-30s.pprof
```

Inspect the result:

```bash
go tool pprof -top -nodecount=30 .work/tenniarb-cpu-30s.pprof
go tool pprof -top -cum -nodecount=30 .work/tenniarb-cpu-30s.pprof
```

Open the interactive call graph when needed:

```bash
go tool pprof -http=127.0.0.1:8080 .work/tenniarb-cpu-30s.pprof
```

A profile dominated by `gcBgMarkWorker`, `gcDrain`, `scanSpan` or `scanObjectSmall` is CPU spent in the Go
collector. It is not evidence that rendering is the immediate cause. Use heap and allocation profiles to
find what creates the collector's work.

## Heap and allocation profiles

Take a heap profile:

```bash
curl -fsS http://127.0.0.1:6060/debug/pprof/heap \
  -o .work/tenniarb-heap-current.pprof
```

The same file supports live objects and cumulative allocations:

```bash
go tool pprof -top -inuse_space -nodecount=30 .work/tenniarb-heap-current.pprof
go tool pprof -top -inuse_objects -nodecount=30 .work/tenniarb-heap-current.pprof
go tool pprof -top -alloc_space -nodecount=30 .work/tenniarb-heap-current.pprof
```

- `inuse_space`: memory retained by live Go objects at the profile point. Use it to investigate retention.
- `inuse_objects`: the number of retained objects.
- `alloc_space`: memory allocated over the process lifetime, including objects already collected. High
  allocation churn can drive GC even when live memory is small.

Compare two snapshots taken after the same workload:

```bash
go tool pprof -top -inuse_space \
  -diff_base=.work/tenniarb-heap-before.pprof \
  .work/tenniarb-heap-current.pprof
```

Positive values show growth between the profiles. Take several snapshots: one pair can include temporary
objects and a GC cycle that has not finished yet.

A pprof heap profile contains sampled allocation metadata, not the raw bytes of the Go heap. It does not
account for all process RSS: AppKit allocations, `IOSurface`, CoreAnimation, images, mapped libraries,
runtime reservations and swapped memory are outside it.

The application sets `GOGC=50` and `GOMEMLIMIT=1GiB` when those variables are not already set.
`GOMEMLIMIT` is a soft runtime limit, not a cap on process RSS.

## Goroutine dump

Capture all goroutine stacks:

```bash
curl -fsS 'http://127.0.0.1:6060/debug/pprof/goroutine?debug=2' \
  -o .work/tenniarb-goroutines.txt
```

Use `debug=1` for the shorter table with goroutine counts by stack. Look for growing numbers of goroutines,
blocked channel operations, repeated MCP requests and work that never returns.

Do not use `SIGQUIT` as a routine alternative: a Go process prints its goroutine stacks and then exits.

## macOS process snapshots

Find the process and record its CPU and resident memory:

```bash
pgrep -fl 'Tenniarb|tenniarb-service'
ps -p PID -o pid,ppid,%cpu,%mem,rss,etime,time,command
```

Record a native stack sample and a memory map:

```bash
sample PID 10 -file .work/tenniarb-sample.txt
vmmap -summary PID > .work/tenniarb-vmmap.txt
```

`sample` is useful for native waits such as `__psynch_cvwait`, `read` and `kevent`. Go function names can be
missing from installed binaries built with `-s -w`; prefer the pprof CPU endpoint for Go-level attribution.
`vmmap` shows the process footprint and non-Go regions, but does not identify the Go objects that retain
memory.

## Tool call log

The MCP tools live in `tenniarb-service`, so the service is where a run is recorded: one JSON line per
tool call, appended to `TENNIARB_MCP_LOG` (`off` silences it). `make install`, `make opencode-mcp`, `make codex-mcp` and
`make claude-mcp` set it to `~/Library/Logs/Tenniarb/mcp-calls.log` in the agent config, so every session
opencode, codex or Claude Code starts writes the log without a variable to remember; `MCP_LOG` overrides the path in
the Makefile. The proxy passes the environment to the service it starts.

```json
{"at":"2026-11-07T22:15:39.2+07:00","pid":99111,"client":"codex","root":"/src/design","tool":"edit","ok":true,"in":4180,"out":612,"ms":37,"args":"{\"file\":\"Board.tenn\",\"edits\":[{\"find\":\"item Card\",…"}
```

- `client` and `root` tell the sessions apart: the service is one process for every agent of the user.
- `in` is the byte size of the whole request, `args` its first 2000 bytes; `out` the answer, the way it
  costs the agent's context — see [Measuring an agent](mcp-bench.md).
- `ok` is false for a tool error, so a run's failures are in the same file as its successes.
- The file is rolled to `.1` and started again at 8 MiB.

Summarize a session:

```bash
LOG=~/Library/Logs/Tenniarb/mcp-calls.log
jq -r 'select(.ok|not) | [.client,.tool,.args] | @tsv' "$LOG" | tail -20   # what failed last
jq -s 'group_by(.tool) | map({tool:.[0].tool, calls:length, out:(map(.out)|add)})' "$LOG"
```

The log is sampled per call, not per allocation: it answers "what did the agent ask and what did it get",
not "where did the memory go" — that is the heap profile above.

## tenniarb-service

The service accepts the same `TENNIARB_PPROF` variable. `make install`, `make opencode-mcp`, `make codex-mcp`
and `make claude-mcp` register the MCP proxy with `TENNIARB_PPROF=127.0.0.1:6061`; `MCP_PPROF` overrides that
address in the Makefile. The proxy passes the environment to the service it starts.

The listener starts only with a new service process. Use the same CPU, heap and goroutine commands as for
the application, replacing port `6060` with `6061`. For example:

```bash
curl -fsS http://127.0.0.1:6061/debug/pprof/heap \
  -o .work/tenniarb-service-heap.pprof
go tool pprof -top -inuse_space -nodecount=30 .work/tenniarb-service-heap.pprof
```

When an older service has no listener, preserve the available process evidence instead of restarting it just
for instrumentation:

```bash
ps -p SERVICE_PID -o pid,ppid,%cpu,%mem,rss,etime,time,command
sample SERVICE_PID 5 -file .work/tenniarb-service-sample.txt
vmmap -summary SERVICE_PID > .work/tenniarb-service-vmmap.txt
vmmap SERVICE_PID > .work/tenniarb-service-vmmap-full.txt
```

The service started by MCP writes its own log to the service directory described in
[Service lifecycle](mcp.md#service-lifecycle).
