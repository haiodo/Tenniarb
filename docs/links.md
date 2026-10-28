# Links to a place in a file

A link names an element and, optionally, a node of a `.tenn` file so that someone else, who has the same
repository checked out at another path, can open exactly that place. No form contains an absolute path.
Code: package `go/link` (format, remotes, resolution) and `go/app/links*.go` (copy, open).

## Forms

**URL**, what Copy Link puts on the clipboard:

```
tenniarb://open?repo=github.com/haiodo/foundation&path=design/PlatformDesign.tenn&el=Tracker.Boards&node=Screen.Kanban
```

| Parameter | Meaning |
|---|---|
| `repo` | Git remote of the repository, normalised: `host/owner/name`, lower case, no scheme, user, port or `.git`. `git@github.com:haiodo/foundation.git` and `https://github.com/haiodo/foundation` give the same value. Taken from `origin`, else the first remote. |
| `home` | `1` when the file is in no repository with a remote. `path` is then relative to the home folder of the user, `repo` is absent. Such a link works only for people who keep the file at the same place under home; Copy Link says so in the banner. |
| `path` | Slash-separated path of the file from the repository root. Must end in `.tenn`, must not be absolute and must not contain `..`; a link that breaks this is refused. |
| `el` | The element on the canvas: the names from the root, joined by `.`, as the app writes element paths (`PathOf`): a repeated name among siblings gets `#n`, n from 1 for the second one. In a name `%`, `.` and `/` are written `%25`, `%2E`, `%2F`. Absent: the root. |
| `node` | The selected node of that element, as the app writes selection paths (`NodePath`): keys joined by `.`, `#n` for repeats, `/` reaches into an instance (`Screen.login/Header.Title`). In a diagram element it is the name of the selected item. Absent: no selection. |

Values are URL-encoded; the slash is left readable.

**Plain text**, for chats and agents (MCP `select` takes it, so does Goto and Open Link):

```
design/PlatformDesign.tenn#Tracker.Boards/Screen.Kanban
```

`path#el/node`: the file from the repository root, then the element and the node as above. The first `/` after
`#` ends the element, so `/` inside the node is the instance separator. It carries no repository: the file is
looked up in the clone that holds it. A file name with `#` cannot be written in this form (use the URL). The
element names of a link made from MCP are the scene names (`Tracker_2` for `Tracker 2`); the app accepts both.

## Copy

Edit > Copy Link (Cmd+Alt+C; Ctrl+Alt+C elsewhere) and Copy Link at the end of the context menus of the canvas,
the element outline and the item list. The link names the element on the canvas and the first selected node or
item. An unsaved untitled document has no file and gives no link. MCP `selection` returns the same as `link`
(URL) and `link_text` (plain form; absent for a home link).

## Open

File > Open Link... (Cmd+Shift+O) takes a link of either form; a link on the clipboard is filled in. Pasting a link
into Goto Item opens it too. The system gives the URL to the app through the `tenniarb` scheme where it is
registered (see below) or as a command-line argument: `tenniarb 'tenniarb://open?...'`.

1. Find the file. The clones are tried in this order: the folder remembered for the repository
   (`repos.json` in the config directory), the repositories of the open documents, of Open Recent and of every file
   that has a saved window frame (`windows.json`). A clone matches when its remote equals `repo` and the file
   exists in it. A plain link takes the first clone that has the file.
2. Not found: Tenniarb asks for the folder of the clone (a folder dialog) and remembers it for the repository.
   The folder must contain `path`.
3. The file is opened, or its window is brought to the front. The element goes on the canvas, the node is selected
   and scrolled into view. It is an ordinary selection change, so Back returns to where the user was.
4. An element that does not exist leaves the document where it is and says so in the banner; an unknown node
   shows the element with nothing selected and says so.

## Registering the scheme

- macOS: `CFBundleURLTypes` with the scheme `tenniarb`, written into Info.plist by `go/packaging/macos.sh`.
- Linux: `x-scheme-handler/tenniarb` in `MimeType` of the `.desktop` file (`go/packaging/linux.sh`), and
  `xdg-mime default` in `install.sh`. `Exec` has `%U`, so the URL arrives as an argument.
- Windows: not done (a `.reg` with `URL Protocol`).

### Delivery of the URL (state with gowt v0.13.5)

- Command-line argument: works on every OS (the app takes `tenniarb://` arguments itself). The app does not
  call `gowt.SingleInstance`, so a second launch is a second process, not a link into the running window.
- macOS Apple event: **does not arrive**. gowt registers the delegate method as `application:openUrls:`;
  AppKit calls `application:openURLs:`. The app listens to `swt.OpenUrl` already, so a fix in gowt is enough.
- Needed in gowt: register `application:openURLs:` (or the `kAEGetURL` handler) on macOS and raise `swt.OpenUrl`
  with the URL in `Event.Text`; a facade `App.OnOpenURLs(func(urls []string))` with the queue of `OnOpenFiles`;
  and `SingleInstance` forwarding of arguments that are not files (`argFiles` drops them; so does the `Open`
  of the D-Bus interface on Linux).
