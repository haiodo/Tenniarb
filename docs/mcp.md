# Tenniarb MCP

Agents read, edit, validate and render `.tenn` files (UI scenes and classic diagrams) through MCP tools.
The tools run in one shared process, `tenniarb-service`; every agent session starts only the small
proxy `tenniarb-mcp`.

## Install

```bash
make go-mcp-install          # go install tenniarb-mcp and tenniarb-service into GOBIN
claude mcp add tenniarb -- tenniarb-mcp
# or with an explicit workspace root:
claude mcp add tenniarb -- tenniarb-mcp -root /path/to/project
```

`make go-mcp` builds both into `build/go/bin/`. The two binaries must sit in the same directory: the proxy
starts the service found next to itself (or `$TENNIARB_SERVICE_BIN`, or `tenniarb-service` on `PATH`).

The workspace root is `-root`, or the directory the agent starts the proxy in. Every path argument of every
tool stays inside the root. A root with more than 50 000 entries (a home directory) is refused by
`get_state` and by `validate` of the whole workspace.

## Service lifecycle

- **Start.** The proxy dials the service; when none answers it starts `tenniarb-service` detached (own session,
  output in `service.log`) and retries for up to 15 s. Two proxies starting at once are fine: the lock lets one
  service listen, the other exits.
- **One per user.** Service directory: `$TENNIARB_SERVICE_DIR`, else `$XDG_RUNTIME_DIR/tenniarb` on Linux, else
  `<user config dir>/Tenniarb/service` (`~/Library/Application Support/Tenniarb/service` on macOS,
  `%AppData%\Tenniarb\service` on Windows). It holds `service.lock`, `endpoint`, `service.sock` and `service.log`.
- **Idle exit.** Without MCP sessions for 30 minutes (`-idle`, `$TENNIARB_SERVICE_IDLE`; 0 never) the service
  exits. A connected app does not keep it alive.
- **New version.** The proxy sends the size and modification time of the `tenniarb-service` binary next to it.
  If the running service is another binary, it stops listening and the proxy starts the new one. Sessions
  already open stay on the old process, which exits when the last one closes. Ceiling: two installs in different
  directories used at once restart each other; keep one install.

## Security

- Unix: a unix socket in the service directory, which is created `0700`; the socket and the endpoint are `0600`.
- Windows: loopback TCP on `127.0.0.1` with a random port.
- Both: the endpoint file holds a random 128-bit token that every client sends in its hello; on Windows the
  token, readable only from the user's profile, is what keeps other local users out.
- No network exposure: nothing listens on other interfaces.

## Tools

| Tool | What it does |
|---|---|
| `read_skill` | The design skill: how to draw screens and diagrams with these tools; `path` returns a file it names (`layout.md`) |
| `guide` | Examples, the scenes/diagrams rule and the scene spec (`docs/spec/tenn-scene.md`); `topic` narrows it |
| `get_state` | Files, node trees 2 levels deep, renderer of each element, open/unsaved in the app, diagnostics (10 per file); `file`, `path`, `depth` list one subtree, `find` searches nodes by name |
| `read` | A file or one node by dotted path, with line numbers |
| `edit` | Atomic find/replace; nothing changes on a mismatch, a parse error or a new validation error |
| `validate` | Errors and warnings of one file or the workspace |
| `render` | PNG of an element; `view` renders the app window of the file |
| `selection` | Documents open in the app and the user's selection |
| `select` | Select (and reveal) nodes in the app |

**Design skill.** `read_skill` serves `go/mcpserver/skill/` (embedded): `SKILL.md` with the loop (get_state,
find components, write in compact form, render, fix) and `format.md`, `components.md`, `layout.md`,
`diagrams.md`, `review.md`. `TestSkillExamples` loads and renders every `tenn` block of it.

**get_state output** is text, one line a node: `path kind line [renderer] [+N]`, where `+N` counts the nodes
cut off by `depth`. With `path` the lines are relative to that node, as `at`/`replace` address them in its
instance; slots are listed as `slot` lines (`default` for the unnamed one). `path` through an import alias
lists the imported file. `find` matches the last path segment, case-insensitive, at most 100 matches; with
`depth` each match is followed by its parts. A converted library of 3000 nodes lists in 7 KB by default.

**Warnings of imported files.** `edit`, `validate {file}` and `render` list the warnings of the file itself and
only count those of the files it imports (a library warning repeats once per instance); `validate` of the
library lists them.

**Renderer rule** (`model.SceneElement`, the app's rule, spec 5a.17). An element with a `use`, or an item with
children, a `slot` or `auto-layout`, written directly in it is a UI scene (`go/scene`); the file root follows
the same rule. A `slot` written directly in an element is a plain property, as go/scene reads it. Any other
element is a classic diagram (`go/render`, as the app canvas: `pos`, links, styles, element layouts, `${...}`
evaluated).

**Routing.** When the app has the file open:
- `read`, `get_state`, `validate`, `render` use the app's text, unsaved changes included.
- `edit` is checked against the app's text and sent to the app as one undo step at the version it was made
  for. If the user changed the document in between, the app refuses and the agent reads again. The file on
  disk is never written while the app has it open.
- After a version restart the old service refuses every tool on files the app had open: the app has moved
  to the new service, so the old one cannot know the unsaved text.

Otherwise the tools work on disk. Known gap: the app polls for the service every 2 s, so for up to 2 s after
a service starts an edit of a file the app has open goes to disk.

## Provider protocol (for the app)

Package `go/servicewire`. The app is a "provider": it connects with role `provider`, registers its open
documents and answers calls. Messages are newline-delimited JSON (`id`, `method`, `params`, `result`, `error`).

```go
client, err := servicewire.RunProvider(ctx, "", provider, 0) // dials, re-dials every 2 s, never starts the service
client.SetDocuments([]servicewire.Doc{{Path: abs, Version: v, Dirty: dirty}})
```

- `SetDocuments` sends the full set of open documents. Call it on open, close, save, and every change of a
  text (edit, undo, redo, reload); `Version` must change with the text. It returns `ErrNotConnected` while no
  service runs; the last set is sent again on every reconnect.
- `servicewire.Provider` is what the app implements; calls arrive concurrently from service goroutines:
  - `Text(path)`: current text, version, dirty.
  - `Apply(path, base, text)`: replace the whole text as one undo step if the document is at `base`, return the
    new version; otherwise an error.
  - `Selection()`: document of the key window, element on screen, selected nodes as dotted paths.
  - `Select(path, nodes, reveal)`: select nodes; `reveal` brings the window to front and scrolls to the first.
  - `Render(path, scale)`: PNG of what the window shows.
- `go/servicewire/wiretest.Fake` is an in-memory provider for tests.
