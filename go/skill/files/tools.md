# Tool details

Tool behaviour that costs a round when guessed. The loop itself is in `SKILL.md`.

## design

`design {file, ops: [...]}` changes nodes by path: `{op: "set", node: "S.Button", name: "color",
value: "#1f2129"}`, `{op: "move", node, dx, dy}`, `{op: "insert", parent, text}`, `{op: "delete", node}`,
`{op: "replace_all", find, replace}`. The parent of an insert is an item, an element or a use; empty is
the file root. `replace_all` with a `node` searches only that node, without one the whole file. One call
is one undo step; an edit and a delete of the same node in one call is refused - split it.

A property written more than once (`path`, `shadow`) is the last one unless `set` takes `index`: 0-based,
and an index outside the list fails saying how many the node has. The answer names the index it changed.

## read and layout

`read {file, path: "Lib.Forms"}` follows `--from-file`: a path that ends on a mounting element gives the
content of the mounted file, not its one-line declaration.

`layout` and `render {inspect: true}` report the same geometry; the first draws no picture. Ask for
`layout` before the first render: it finds the overflow that the image would only show you.

## refactor

`refactor {file, node, rename: "NewName"}` renames a node and every `use`, `at` and `replace` path that
names it, in every file that mounts it; `parent` moves it into an item, an element or `.` (the file
root), and the paths are written again from the entry document. A move refuses when the node is named
inside a `use` by an `at` or `replace` path: those are relative to the instance, not to the file.
