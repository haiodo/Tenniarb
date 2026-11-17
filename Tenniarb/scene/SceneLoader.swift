//
//  SceneLoader.swift
//  Tenniarb
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

import Foundation
import Tenn

/// An error the loader cannot put into the diagnostics of a tree: the main file is unreadable or outside the root.
struct SceneLoadError: Error, CustomStringConvertible {
    let message: String

    var description: String { message }
}

extension Scene {
    struct Diagnostic: Equatable {
        var pos: Pos
        var message: String
        var warn: Bool
    }

    /// A loaded workspace file with everything it imports.
    final class Tree {
        let root: URL  // absolute, symlinks resolved
        var main: Node?  // the file loaded for; with an entry, a file mounted below the entry
        var top: Node?  // the document lookups end at; main when loaded alone
        var files: [String: Node] = [:]  // file roots by slash path relative to the root
        var order: [Node] = []  // file roots in load order
        var texts: [String: String] = [:]  // text of every loaded file
        var diagnostics: [Diagnostic] = []
        var ctxs: [ObjectIdentifier: MountCtx] = [:]  // file root -> the first chain of mounts that reaches it

        init(root: URL) {
            self.root = root
        }

        func file(_ rel: String) -> Node? { files[rel] }

        func source(_ rel: String) -> String? { texts[rel] }

        /// The entry document the tree was loaded in the context of; "" when the file was loaded alone.
        var entry: String { top === main ? "" : top?.pos.file ?? "" }

        /// How many diagnostics are errors; a warning does not stop a render.
        var fatalCount: Int { diagnostics.filter { !$0.warn }.count }

        func report(_ pos: Pos, warn: Bool, _ message: String) {
            diagnostics.append(Diagnostic(pos: pos, message: message, warn: warn))
        }

        /// Finds rel relative to file at, a slash path in the tree; ok is false when the result leaves the root.
        func resolveIn(_ at: String, _ rel: String) -> (url: URL, rel: String, ok: Bool) {
            let dir = (at as NSString).deletingLastPathComponent
            let base = dir.isEmpty ? root : root.appendingPathComponent(dir)
            let path = base.appendingPathComponent(rel).standardizedFileURL.resolvingSymlinksInPath()
            let inside = path.path == root.path || path.path.hasPrefix(root.path + "/")
            return (path, relative(path), inside)
        }

        /// The text of a file the tree reads: a body or a template that names a markdown file of the workspace.
        func read(_ abs: URL) -> String? {
            try? String(contentsOf: abs, encoding: .utf8)
        }

        /// The path of abs relative to the root, with slash separators.
        private func relative(_ abs: URL) -> String {
            let prefix = root.path + "/"
            guard abs.path.hasPrefix(prefix) else { return abs.path }
            return String(abs.path.dropFirst(prefix.count))
        }

        func fail(_ pos: Pos, _ message: String) { report(pos, warn: false, message) }

        func warn(_ pos: Pos, _ message: String) { report(pos, warn: true, message) }
    }
}

/// Reads a .tenn file and the files it mounts, as go/scene/load.go does.
final class SceneLoader {
    static let fromFileFlag = "--from-file"

    let root: URL
    /// Text of files that stands in for the disk, by slash path from the root: an edit is loaded before
    /// it is saved.
    var overlay: [String: String] = [:]
    private var tree: Scene.Tree!
    private var currentFile = ""
    private var stack: [(abs: String, rel: String)] = []
    private var loaded: [String: Scene.Node] = [:]
    private var blockEnds: [String: [Int: Int]] = [:]  // closing brace of every block, by file and brace

    init(root: URL) {
        self.root = root.standardizedFileURL.resolvingSymlinksInPath()
    }

    /// Loads file (relative to the root, or absolute) and the files it mounts.
    func load(_ file: String) throws -> Scene.Tree {
        try loadIn(file, entry: "")
    }

    /// Loads file in the context of an entry document (spec 5a.1): the entry is loaded, and a name the
    /// file does not define itself is looked up where the file is mounted, up to the entry. The entry
    /// must mount file, directly or through other files.
    func loadIn(_ file: String, entry: String) throws -> Scene.Tree {
        let tree = Scene.Tree(root: root)
        self.tree = tree
        currentFile = ""
        stack = []
        loaded = [:]
        blockEnds = [:]

        let (abs, inRoot) = inside(absolute(file))
        guard inRoot else {
            throw SceneLoadError(message: "\(file) is outside the workspace root \(root.path)")
        }
        let (entryAbs, entryInside) = inside(entry.isEmpty ? abs : absolute(entry))
        guard entryInside else {
            throw SceneLoadError(message: "\(entry) is outside the workspace root \(root.path)")
        }
        guard let top = loadFile(entryAbs, at: nil) else {
            throw SceneLoadError(message: "cannot read \(relative(entryAbs))")
        }
        tree.top = top
        tree.mapMounts()
        guard let main = tree.file(relative(abs)) else {
            throw SceneLoadError(message: "\(relative(entryAbs)) does not mount \(relative(abs))")
        }
        tree.main = main
        return tree
    }

    // MARK: - Files

    private func absolute(_ file: String) -> URL {
        file.hasPrefix("/") ? URL(fileURLWithPath: file) : root.appendingPathComponent(file)
    }

    // Cleans the path and reports whether it lies in the workspace root, symlinks resolved when it exists.
    private func inside(_ url: URL) -> (URL, Bool) {
        let path = url.standardizedFileURL.resolvingSymlinksInPath()
        if path.path == root.path {
            return (path, true)
        }
        return (path, path.path.hasPrefix(root.path + "/"))
    }

    private func relative(_ abs: URL) -> String {
        let prefix = root.path + "/"
        guard abs.path.hasPrefix(prefix) else { return abs.path }
        return String(abs.path.dropFirst(prefix.count))
    }

    private func loadFile(_ abs: URL, at: Scene.Pos?) -> Scene.Node? {
        let key = abs.path
        if let i = stack.firstIndex(where: { $0.abs == key }) {
            let hops = stack[i...].map(\.rel) + [relative(abs)]
            tree.fail(at ?? Scene.Pos(file: relative(abs), line: 1, col: 1), "import cycle: \(hops.joined(separator: " -> "))")
            return nil
        }
        if let node = loaded[key] {
            return node
        }
        let rel = relative(abs)
        let text: String
        if let over = overlay[rel] {
            text = over
        } else {
            do {
                text = try String(contentsOf: abs, encoding: .utf8)
            } catch {
                tree.fail(at ?? Scene.Pos(file: rel, line: 1, col: 1), "cannot read \(rel): \(error.localizedDescription)")
                return nil
            }
        }

        stack.append((key, rel))
        let file = Scene.Node(kind: .file, pos: Scene.Pos(file: rel, line: 1, col: 1))
        tree.texts[rel] = text
        blockEnds[rel] = tennBlockEnds(text)
        let parser = TennParser()
        let ast = parser.parse(text)
        if parser.errors.hasErrors() {
            tree.fail(Scene.Pos(file: rel, line: 1, col: 1), "the file has parse errors; the tree is partial")
        }
        let saved = currentFile
        currentFile = rel
        joinedBlocks(ast.children ?? [])
        fill(file, ast.children ?? [])
        currentFile = saved
        stack.removeLast()

        loaded[key] = file
        tree.files[rel] = file
        tree.order.append(file)
        return file
    }

    /// Warns on a command with two blocks: the format joins them into one, so the second name is lost.
    private func joinedBlocks(_ ns: [TennNode]) {
        for n in ns {
            if n.kind == .Command {
                let blocks = (n.children ?? []).filter { $0.kind == .BlockExpr }.count
                if blocks > 1 {
                    tree.warn(
                        pos(n),
                        "\(blocks) blocks in one command \"\(cmdName(n) ?? "")\" are joined: put each block on its own line")
                }
            }
            joinedBlocks(n.children ?? [])
        }
    }

    /// Finds rel relative to the file being read; reports a path that leaves the workspace root.
    private func resolveFile(_ rel: String, _ pos: Scene.Pos, _ what: String) -> (URL, Bool) {
        let (abs, inside) = resolveIn(currentFile, rel)
        if !inside {
            tree.fail(pos, "\(what) \"\(rel)\" leaves the workspace root")
        }
        return (abs, inside)
    }

    /// Finds rel relative to file at, a slash path in the tree; ok is false when the result leaves the root.
    private func resolveIn(_ at: String, _ rel: String) -> (URL, Bool) {
        let found = tree.resolveIn(at, rel)
        return (found.url, found.ok)
    }

    // MARK: - Commands

    /// Sorts the commands of an element, item or file into structure and properties.
    private func fill(_ owner: Scene.Node, _ cmds: [TennNode]) {
        for c in cmds {
            guard let name = cmdName(c) else { continue }
            if name == "element" && owner.isContainer() {
                addElement(owner, c)
            } else if addContent(owner, c, name) != nil {
            } else if name == "styles" && (owner.isContainer() || owner.kind == .item) {
                addStyles(owner, c)
            } else if name == "kind" {
                let (args, _) = split(c)
                setKind(owner, argText(args, 0), pos(c))
            } else if name == "export" && (owner.kind == .element || owner.kind == .item) {
                addParam(owner, c)
            } else {
                owner.props.append(prop(c))
            }
        }
    }

    private func addElement(_ owner: Scene.Node, _ c: TennNode) {
        let n = newNode(.element, owner, c)
        owner.elems.append(n)
        let (args, block) = split(c)
        if let (_, atNode) = kindOf(args) {
            tree.warn(pos(atNode), "kind goes inside the element block: \"element \\\"\(n.name)\\\" { kind Component }\"")
        }
        if let rel = fromFile(args) {
            if !block.isEmpty {
                tree.fail(n.pos, "element \"\(n.name)\" takes its content from \(rel): remove the block")
            }
            mountFile(n, rel)
            return
        }
        fill(n, block)
    }

    /// Makes the content of the file the content of n: the lists are shared, so one file mounted twice is one subtree.
    private func mountFile(_ n: Scene.Node, _ rel: String) {
        let (abs, inside) = resolveFile(rel, n.pos, "element --from-file")
        guard inside, let file = loadFile(abs, at: n.pos) else { return }
        n.mount = file
        n.props = file.props
        n.content = file.content
        n.elems = file.elems
        if n.type.isEmpty {  // the kind of the mounted file, unless the mounting element says its own
            n.type = file.type
        }
    }

    /// Reads "export let name = value": a parameter, with its default; without "=" the use has to set it.
    private func addParam(_ owner: Scene.Node, _ c: TennNode) {
        let (args, _) = split(c)
        let pos = pos(c)
        let wellFormed = (args.count == 2 || args.count == 4) && argText(args, 0) == "let" && (args.count != 4 || argText(args, 2) == "=")
        guard wellFormed else {
            tree.fail(pos, "a parameter is written: export let <name> or export let <name> = <value>")
            return
        }
        let name = argText(args, 1)
        if name == "ctx" {
            tree.warn(self.pos(args[1]), "ctx is the context of the scene (spec 5f), not a parameter")
            return
        }
        if owner.param(name) != nil {
            tree.warn(self.pos(args[1]), "parameter \"\(name)\" is declared twice")
            return
        }
        var param = Scene.Param(name: name, pos: self.pos(args[1]))
        if args.count == 4 {
            let value = Array(args[3...])
            param.defaultValue = Scene.Prop(name: name, args: value, pos: self.pos(args[3]), range: spanOf(value))
        }
        owner.params.append(param)
    }

    /// Reads "styles { name { properties } ... }": the named styles an element gives its nodes.
    private func addStyles(_ owner: Scene.Node, _ c: TennNode) {
        let block = parseStyles(c)
        if owner.styles == nil {
            owner.styles = [:]
        }
        for (name, style) in block.styles {
            owner.styles?[name] = style
        }
        owner.styleGroups.append(contentsOf: block.groups)
    }

    private func parseStyles(_ c: TennNode) -> Scene.StyleBlock {
        var block = Scene.StyleBlock()
        let (_, cmds) = split(c)
        for cmd in cmds {
            guard let name = cmdName(cmd) else { continue }
            if !hasBlock(cmd) {
                if let group = styleGroup(cmd) {
                    block.groups.append(group)
                }
                continue
            }
            block.names.append(name)
            block.styles[normalizeName(name)] = readStyle(cmd)
        }
        return block
    }

    /// Reads a group declaration "name one-of a b" or "name any-of a b"; nil for any other command without a block.
    private func styleGroup(_ c: TennNode) -> Scene.StyleGroup? {
        let (args, _) = split(c)
        let mode = argText(args, 0)
        guard mode == "one-of" || mode == "any-of" else { return nil }
        var group = Scene.StyleGroup(name: normalizeName(cmdName(c) ?? ""), anyOf: mode == "any-of", styles: [])
        for i in 1..<args.count {
            group.styles.append(normalizeName(argText(args, i)))
        }
        return group
    }

    /// Reads one style: its properties and the blocks of kid names in it.
    private func readStyle(_ c: TennNode) -> Scene.Style {
        var style = Scene.Style()
        let (_, cmds) = split(c)
        for cmd in cmds {
            guard let name = cmdName(cmd) else { continue }
            if hasBlock(cmd) {
                if name == "animation" {
                    tree.warn(pos(cmd), "the keyframes block goes in the element, not in a style: write it next to styles")
                    continue
                }
                style.kids.append(Scene.StyleKid(key: normalizeName(name), style: readStyle(cmd)))
                continue
            }
            style.props.append(prop(cmd))
        }
        return style
    }

    /// Adds an item, link, use or slot command to owner; nil when c is not one, so the caller keeps it as a property.
    private func addContent(_ owner: Scene.Node, _ c: TennNode, _ name: String) -> Scene.Node? {
        guard owner.holdsContent() || owner.kind == .use else { return nil }
        let (args, block) = split(c)
        var n: Scene.Node?
        switch name {
        case "item":
            let item = newNode(.item, owner, c)
            fill(item, block)
            n = item
        case "slot":
            guard owner.kind == .item else { return nil }
            let slot = newNode(.slot, owner, c)
            fill(slot, block)
            n = slot
        case "link":
            let link = Scene.Node(kind: .link, pos: pos(c), range: range(c), body: body(c), parent: owner)
            link.from = argText(args, 0)
            link.to = argText(args, 1)
            for b in block where cmdName(b) != nil {
                link.props.append(prop(b))
            }
            n = link
        case "use":
            n = newUse(owner, c, args, block)
        default:
            return nil
        }
        if let n {
            owner.content.append(n)
        }
        return n
    }

    /// Reads "use <path> as <name> { overrides }". A bare use stays a property.
    private func newUse(_ owner: Scene.Node, _ c: TennNode, _ args: [TennNode], _ block: [TennNode]) -> Scene.Node? {
        guard !args.isEmpty else { return nil }
        let pos = pos(c)
        var asIndex = -1
        for (i, a) in args.enumerated() where a.kind == .Ident && argText(args, i) == "as" {
            asIndex = i
            break
        }
        guard asIndex >= 1, asIndex + 2 == args.count else {
            tree.fail(pos, "use needs the form: use <path> as <name>")
            return nil
        }
        guard let path = pathText(Array(args[..<asIndex])) else {
            let written = args[..<asIndex].map { argText([$0], 0) }.joined(separator: " ")
            tree.fail(pos, "bad path \"\(written)\": separate segments with dots, and write a name with spaces in quotes")
            return nil
        }
        let n = Scene.Node(kind: .use, name: argText(args, asIndex + 1), pos: pos, range: range(c), body: body(c), parent: owner)
        n.path = path
        n.block = overrides(n, block)
        return n
    }

    /// Reads the block of a use. Nodes inside it, however deep, get the use as their scope.
    private func overrides(_ use: Scene.Node, _ cmds: [TennNode]) -> [Scene.Override] {
        var out: [Scene.Override] = []
        for c in cmds {
            guard let name = cmdName(c) else { continue }
            let (args, block) = split(c)
            var ov = Scene.Override(pos: pos(c), range: range(c), body: body(c))
            switch name {
            case "at", "replace":
                guard let path = pathText(args) else {
                    let written = args.map { argText([$0], 0) }.joined(separator: " ")
                    tree.fail(ov.pos, "bad path \"\(written)\": separate segments with dots, and write a name with spaces in quotes")
                    continue
                }
                ov.kind = name == "replace" ? .replace : .at
                ov.path = path
                ov.block = overrides(use, block)
            case "slot":
                ov.kind = .slot
                ov.slot = argText(args, 0)
                ov.block = overrides(use, block)
            default:
                if let n = addContent(use, c, name) {
                    ov.kind = .node
                    ov.node = n
                } else {
                    ov.kind = .prop
                    ov.prop = prop(c)
                }
            }
            out.append(ov)
        }
        return out
    }

    // MARK: - Commands and arguments

    private func newNode(_ kind: Scene.Kind, _ owner: Scene.Node, _ c: TennNode) -> Scene.Node {
        let children = c.children ?? []
        var name = ""
        if children.count > 1, children[1].kind != .BlockExpr {
            name = children[1].getIdentText() ?? ""
        }
        return Scene.Node(kind: kind, name: name, pos: pos(c), range: range(c), body: body(c), parent: owner)
    }

    private func prop(_ c: TennNode) -> Scene.Prop {
        let (args, _) = split(c)
        return Scene.Prop(name: cmdName(c) ?? "", args: args, block: lastBlock(c), pos: pos(c), range: range(c))
    }

    private func lastBlock(_ c: TennNode) -> TennNode? {
        guard let last = c.children?.last, last.kind == .BlockExpr else { return nil }
        return last
    }

    private func hasBlock(_ c: TennNode) -> Bool {
        lastBlock(c) != nil
    }

    private func pos(_ c: TennNode) -> Scene.Pos { nodePos(currentFile, c) }

    // Go's Node.Range: the command starts at its name and ends after its last token, a block counted to
    // its closing brace (tenn parseCommand keeps End that way).
    private func range(_ c: TennNode) -> Scene.Range {
        guard let token = c.token else { return Scene.Range() }
        var end = Self.endPos(token)
        for k in c.children ?? [] {
            end = max(end, endOf(k))
        }
        return Scene.Range(start: token.pos, end: end)
    }

    private func endOf(_ k: TennNode) -> Int {
        if k.kind == .BlockExpr, let open = k.token, let close = blockEnds[currentFile]?[open.pos] {
            return close
        }
        return range(k).end
    }

    private func body(_ c: TennNode) -> Scene.Range {
        guard let block = lastBlock(c), let token = block.token else { return Scene.Range() }
        return Scene.Range(start: token.pos, end: endOf(block))
    }

    // The Tenn lexer counts size in Characters and pos from a byte offset, so the two cancel: pos + size
    // is the byte just after the token, and the +1 is its closing delimiter.
    private static func endPos(_ token: TennToken) -> Int {
        switch token.type {
        case .stringLit, .expression, .expressionBlock, .markdownLit, .imageData:
            return token.pos + token.size + 1
        default:
            return token.pos + token.size
        }
    }

    private func spanOf(_ args: [TennNode]) -> Scene.Range {
        guard let first = args.first?.token, let last = args.last?.token else { return Scene.Range() }
        return Scene.Range(start: first.pos, end: Self.endPos(last))
    }

    /// Reads "kind <name>" among the arguments of an element command; the kind goes inside the block.
    private func kindOf(_ args: [TennNode]) -> (String, TennNode)? {
        for (i, _) in args.enumerated() where argText(args, i) == "kind" && i + 1 < args.count {
            return (argText(args, i + 1), args[i + 1])
        }
        return nil
    }

    private func setKind(_ n: Scene.Node, _ name: String, _ pos: Scene.Pos) {
        n.type = name.lowercased()
        switch n.type {
        case SceneType.component, SceneType.screen, SceneType.library, SceneType.application:
            return
        default:
            tree.warn(pos, "kind \"\(name)\" is not one of component, screen, library, application")
        }
    }

    /// The path a command writes, from its arguments: bare words and names in double quotes, joined by dots.
    private func pathText(_ args: [TennNode]) -> String? {
        guard !args.isEmpty else { return nil }
        var out = ""
        for a in args {
            guard let s = a.getIdentText() else { return nil }
            out += a.kind == .StringLit ? quoteName(s) : s
        }
        return splitPath(out) != nil ? out : nil
    }
}

// The reading of commands that entry discovery shares with the loader, as the package-level ones of go/scene.

func cmdName(_ c: TennNode) -> String? {
    guard c.kind == .Command else { return nil }
    let name = c.getIdent(0) ?? ""
    return name.isEmpty ? nil : name
}

/// Separates the arguments of a command from its block.
func split(_ c: TennNode) -> (args: [TennNode], block: [TennNode]) {
    var args: [TennNode] = []
    var block: [TennNode] = []
    for a in (c.children ?? []).dropFirst() {
        if a.kind == .BlockExpr {
            block.append(contentsOf: a.children ?? [])
        } else {
            args.append(a)
        }
    }
    return (args, block)
}

func argText(_ args: [TennNode], _ i: Int) -> String {
    guard i < args.count else { return "" }
    return args[i].getIdentText() ?? ""
}

/// Reads "--from-file <path>" among the arguments of an element command.
func fromFile(_ args: [TennNode]) -> String? {
    for (i, _) in args.enumerated() where argText(args, i) == SceneLoader.fromFileFlag && i + 1 < args.count {
        return argText(args, i + 1)
    }
    return nil
}

/// The position of a command of the file named file, 1-based.
func nodePos(_ file: String, _ c: TennNode) -> Scene.Pos {
    guard let token = c.token else { return Scene.Pos(file: file, line: 1, col: 1) }
    return Scene.Pos(file: file, line: token.line + 1, col: token.col - token.size + 1)
}
