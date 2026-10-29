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

OpenCode: `make opencode-mcp` registers the same command in the global config with the full path
(`opencode mcp add --global tenniarb -- <GOBIN>/tenniarb-mcp`); `make install` runs it when `opencode`
is on PATH. Restart OpenCode afterwards.

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
| `select` | Select (and reveal) nodes in the app; `file` may be a link, see [links.md](links.md) |
| `comments` | The comments the user left on a file: id, author, text, anchor (`element`, `node`, `point`, `link_text` that `select` takes), replies; `state` `open` (default) or `all` |
| `reply` | Answer a comment (`file`, `id`, `text`); it shows in the thread in the app at once |
| `resolve` | Mark a comment done (`file`, `id`, optional closing `text`) |
| `wait_comment` | Block until the user leaves a new comment, reply or reopens one (`file` or any document, `timeout_s` default 60, at most 600); returns a list of events, `[]` on timeout |

**Design skill.** `read_skill` serves `go/mcpserver/skill/` (embedded): `SKILL.md` with the loop (get_state,
find components, write in compact form, render, fix) and `format.md`, `components.md`, `layout.md`,
`diagrams.md`, `review.md`. `TestSkillExamples` loads and renders every `tenn` block of it.

**get_state output** is text, one line a node: `path kind line [renderer] [+N]`, where `+N` counts the nodes
cut off by `depth`. With `path` the lines are relative to that node, as `at`/`replace` address them in its
instance; slots are listed as `slot` lines (`default` for the unnamed one). `path` through a mounting element
lists the mounted file. `find` matches the last path segment, case-insensitive, at most 100 matches; with
`depth` each match is followed by its parts. A converted library of 3000 nodes lists in 7 KB by default.

**Warnings of mounted files.** `edit`, `validate {file}` and `render` list the warnings of the file itself and
only count those of the files it mounts (a library warning repeats once per instance); `validate` of the
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

## Check without MCP

`tenniarb check [-json] [-root DIR] FILE...` validates files headless with the same `scene.Diagnose` as `validate`
(parse errors, scene diagnostics, `--from-file` mounts, cycles), for agents that edit files directly. It opens no
window. One line per problem, `path:line:col: error|warning: message`, paths relative to the working directory,
then `N files checked, E errors, W warnings`; `-json` prints an array of `{file, line, col, severity, message}`.
Exit code: 0 no errors (warnings allowed), 1 errors, 2 usage or an unreadable file. The root for `--from-file` is
`-root`, else the working directory (as `tenniarb-mcp`), else the directory of a file outside it. Unlike `validate {file}`
it lists the warnings of mounted files too. The command is in the app binary (`Tenniarb.app/Contents/MacOS/tenniarb check`),
not in `tenniarb-mcp`, which links the standard library only.

## `tenniarb` command line

| Command | Does |
|---|---|
| `tenniarb [FILE.tenn ...]` | opens the app with the files (without files: the last documents, or a new one) |
| `tenniarb check [-json] [-root DIR] FILE...` | validates headless, see above |
| `tenniarb help`, `-h`, `--help` | prints the usage to stdout, exit 0; no window opens |

Any other argument that starts with `-` prints the usage to stderr and exits 2 before the GUI starts. The
arguments AppKit adds itself (`-psn_*`, `-NS*`, `-Apple*`) are ignored.

## Agent presence and Follow

The app shows which agents work on an open document, and can follow them.

- **Tracking.** Every tool call that names a file open in the app (`get_state` with `file`, `read`, `edit`,
  `validate` with `file`, `render`, `select`) records for its session: tool, time and place. The place is
  the dotted path (spec 4.2) of the node and of its element, with lines: the `path`/`element`/first node
  argument, or for `edit` the first changed place of the new text. A file the app does not have open is not
  tracked. The agent name is `clientInfo.name` of the MCP initialize, read at the first tool call.
- **Drop-out.** A session counts for a document for 10 minutes after its last action there
  (`servicewire.PresenceTTL`, `Service.PresenceTTL`); a closed session drops out at once. The service checks
  every `TTL/10` (1 s to 30 s).
- **Wire.** The service sends the app the notification `presence` (`servicewire.Presence`) for each changed
  document: agent count, name, tool, element, node, lines, time `at` (Unix ms) and `seq`. Messages are
  handled concurrently, so the app drops one whose `seq` is below the last it saw. `v` is 1
  (`PresenceVersion`): new optional fields keep it, a reader drops a higher one. The hello protocol is
  unchanged: an app without the method ignores the notification, a service without it never sends one. The
  proxy stays standard-library only. Documents are marked again when an app registers them, so a window
  opened later gets the current state. Provider side: the optional `servicewire.PresenceReceiver`.
- **Indicator.** While an agent works on the file, the toolbar next to the title shows `● claude-code`
  (`● Agent` when the client sent no name, `(2)` for two agents). The dot is filled for 10 s after the last
  action and empty after; gowt text items have no colour, so green and grey are filled and empty. Clicking it
  toggles Follow.
- **Follow.** The `Follow` button (`Following` when on; kept while the window lives, not saved) shows every
  action of the agent: the element goes on the canvas, the node is selected and scrolled into view. It is
  an ordinary selection change, so Back returns to where the user was. The window does not come to front
  and the keyboard stays put; while the user types (inline editor, a text field with the keyboard, the code
  panel) the last place waits and is shown after the typing stops. Off, only the indicator updates.
- **Not covered.** Actions without a place (`validate`, whole-file `read`) update the indicator only.
  Presence comes from the service's sessions only: an agent that edits files directly is not seen.

## Comments

The user places comments on a design in the app (comment mode, key `C` on the canvas); an agent answers them
over MCP. After you finish a task call `comments` for the file and answer each open comment with `reply` or
`resolve`. To stay on duty loop `wait_comment`: it returns when the user writes something new and `[]` after
`timeout_s`, so call it again. A session is told only of what happened after it started, and each event once.
Your own replies do not wake it. `reply` and `resolve` show the agent at the anchor in the app (presence).

- **Storage.** Only in the config folder of the user (`$TENNIARB_CONFIG_DIR`, else the app config folder),
  `comments/<hash>.json`, one file per document, written atomically. Never in the repository. The document is
  named by its link identity (`go/link` `DocKey`: normalised remote plus repository path, else the absolute
  path), so another clone of the repository finds the same comments.
- **Format.** `{v, key, rev, comments: [{id, created, author, text, anchor, state, replies}]}`. `author` is
  `user` or the MCP client name. `anchor` is `{element: [names], node, point}`: `node` is the selection path
  (`a/b` reaches into an instance) or the diagram item name; `point` is measured from the top left of the
  node box, or from the element content origin without a node. `state` is `open` or `resolved`; threads are
  one level deep.
- **Owner.** `tenniarb-service` is the only writer. The app changes comments with the provider call
  `comments.op` (`servicewire.CommentsOp`: `add`, `reply`, `resolve`, `reopen`, `delete`, `list`; the author is
  always `user`) and receives every change as the `comments` notification (`CommentsUpdate`, versioned by
  `CommentsVersion`, ordered by `rev`). The service also sends the stored comments when the app registers a
  document. The app reads the files itself when it opens a document, and writes them itself only while no
  service runs.

## Links

`selection` returns `link` (a `tenniarb://open?...` URL) and `link_text` (`dir/File.tenn#Element/node`) for the
selection of the front window, to share with a person or another agent. `select` takes either form in `file`
(and the nodes from the link when `nodes` is empty), so an agent can show the place a person pasted into the
chat. The path of a link is relative to the git repository root; when the workspace is a folder of it, the
file must be inside the workspace, and a link for another repository is refused. A scene path cannot carry
`#n` or an instance part: a repeated name gives the usual `ambiguous path` error, and a link into an instance
selects the `use` that holds it. Format: [links.md](links.md).

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
  - `Presence(p)` (optional, `PresenceReceiver`): agents working on a document, see above.
  - `Comments(u)` (optional, `CommentsReceiver`): the comments of a document changed, see Comments.
- `go/servicewire/wiretest.Fake` is an in-memory provider for tests.
