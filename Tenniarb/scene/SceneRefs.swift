//
//  SceneRefs.swift
//  Tenniarb
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//  WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//  License for the specific language governing permissions and limitations under
//  the License.
//

import Foundation
import Tenn

extension Scene {
    /// Where a reference to a node is written.
    enum RefKind: UInt8 {
        case use  // the path of a use
        case at  // the path of an at override
        case replace  // the path of a replace override
        case link  // an end of a link
    }

    /// An argument of a command in the text of file that names a node. text is its value, a dotted path or the
    /// name of a link end; seg is the segment of the path that names the node, -1 for a link end.
    struct Ref {
        var kind: RefKind
        var file: String
        var arg: Range
        var text: String
        var seg: Int
    }

    /// A text edit of one file of the document.
    struct FileEdit {
        var file: String
        var edit: TextEdit
    }

    /// A rename of a node and what refers to it: the own name edit and the edits of the references, one for
    /// each of refs. total is how many places name the node now, those the rename leaves as they are too.
    struct RenamePlan {
        var node: Node
        var name: String
        var own: FileEdit
        var refs: [Ref]
        var refEdits: [FileEdit]
        var total: Int

        /// The edits in file order, then by offset; without update only the name is written.
        func edits(update: Bool = true) -> [FileEdit] {
            let all = (update ? refEdits : []) + [own]
            return all.enumerated().sorted { a, b in
                if a.element.file != b.element.file {
                    return a.element.file < b.element.file
                }
                if a.element.edit.start != b.element.edit.start {
                    return a.element.edit.start < b.element.edit.start
                }
                return a.offset < b.offset
            }.map(\.element)
        }
    }

    /// The refusal of a rename: the message the app shows.
    struct RenameError: Error, CustomStringConvertible {
        let message: String

        var description: String { message }
    }
}

extension Scene.Tree {
    /// The places that name target in every file of the tree: a path counts when it resolves through target,
    /// so the same name in another scope is not found. at and replace are followed into the instances of each
    /// mount context. blocked says why the links cannot follow a rename: several siblings share the name and
    /// the index of a link would shift.
    /// Ceiling: links in the block of a use or in a slot fill are not looked at.
    func references(_ target: Scene.Node?) -> (refs: [Scene.Ref], blocked: String?) {
        guard let target, top != nil else {
            return ([], nil)
        }
        let finder = RefFinder(t: self, target: target)
        if let top {
            finder.expand(top, nil)
        }
        let blocked = finder.links()
        return (finder.sorted(), blocked)
    }

    /// planRename is planNodeRename of go/app/rename.go: what renaming the item or use target changes. A
    /// sibling that holds the new key refuses the rename, and so does a link whose index would shift.
    func planRename(_ target: Scene.Node, _ name: String) throws -> Scene.RenamePlan {
        let name = name.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !name.isEmpty else {
            throw Scene.RenameError(message: "The name cannot be empty.")
        }
        guard !target.name.isEmpty, let own = nameRange(target, self) else {
            throw Scene.RenameError(message: "An item without a name cannot be renamed here.")
        }
        let key = normalizeName(name)
        if key != target.key, keyTaken(target, key) {
            throw Scene.RenameError(message: "There is something named \(String(reflecting: name)) here already.")
        }
        var plan = Scene.RenamePlan(
            node: target, name: name,
            own: Scene.FileEdit(file: target.pos.file, edit: Scene.TextEdit(start: own.start, end: own.end, new: argText(name))),
            refs: [], refEdits: [], total: 0)
        let (found, blocked) = references(target)
        plan.total = found.count
        for r in found {
            guard let edit = refEdit(r, name) else { continue }
            plan.refs.append(r)
            plan.refEdits.append(Scene.FileEdit(file: r.file, edit: edit))
        }
        if let blocked {
            throw Scene.RenameError(message: "Not renamed: " + blocked + ".")
        }
        return plan
    }
}

/// The name as the compact printer writes an argument: bare when it can be, else quoted.
func argText(_ name: String) -> String {
    let printed = commandText("x", name)
    return printed.hasPrefix("x ") ? String(printed.dropFirst(2)) : printed
}

/// The edit that makes r name the node by name; nil when the text already does. A path segment is compared by
/// its normalized form.
func refEdit(_ r: Scene.Ref, _ name: String) -> Scene.TextEdit? {
    var text = name
    if r.seg >= 0 {
        guard let segs = splitPath(r.text), r.seg < segs.count, !segs[r.seg].isName(name) else {
            return nil
        }
        var parts = segs.map(\.text)
        parts[r.seg] = normalizeName(name)
        text = parts.joined(separator: ".")
    } else if r.text == name {
        return nil
    }
    return Scene.TextEdit(start: r.arg.start, end: r.arg.end, new: argText(text))
}

/// The text after edits; throws when two of them overlap or one reaches outside text.
func applyEdits(_ text: String, _ edits: [Scene.TextEdit]) throws -> String {
    let bytes = Array(text.utf8)
    let sorted = edits.enumerated().sorted { a, b in
        a.element.start == b.element.start ? a.offset < b.offset : a.element.start < b.element.start
    }.map(\.element)
    var at = 0
    for e in sorted {
        guard e.start >= at, e.end >= e.start, e.end <= bytes.count else {
            throw Scene.RenameError(message: "The references overlap.")
        }
        at = e.end
    }
    var out = bytes
    for e in sorted.reversed() {  // from the end, so the offsets of the earlier edits stay
        out.replaceSubrange(e.start..<e.end, with: Array(e.new.utf8))
    }
    return String(decoding: out, as: UTF8.self)
}

/// Whether a sibling of n, which a path can name, has the key.
private func keyTaken(_ n: Scene.Node, _ key: String) -> Bool {
    guard let p = n.parent, !key.isEmpty else {
        return false
    }
    return (p.named() + p.content).contains { $0 !== n && $0.key == key }
}

/// The span of the name of n in its file, where a rename writes it; nil for a node without one.
private func nameRange(_ n: Scene.Node, _ tree: Scene.Tree) -> Scene.Range? {
    guard let text = tree.source(n.pos.file) else {
        return nil
    }
    let file = Scene.RefFile(text)
    switch n.kind {
    case .use:
        return file.use(n.range)?.name
    case .item, .element, .slot:
        return file.name(n.range)
    default:
        return nil
    }
}

extension Scene {
    /// The bytes of a file and the tokens the loader read it with. The loader keeps no argument range, so a
    /// reference reads the span of its path or name from the text again.
    fileprivate final class RefFile {
        private let bytes: [UInt8]
        private let tokens: [TennToken]

        init(_ text: String) {
            bytes = Array(text.utf8)
            var found: [TennToken] = []
            let lexer = TennLexer(text)
            while let token = lexer.getToken() {
                if token.type != .semiColon {
                    found.append(token)
                }
            }
            tokens = found
        }

        /// The byte after the token: a literal keeps its closing delimiter, as SceneLoader.endPos reads it.
        private func end(_ t: TennToken) -> Int {
            switch t.type {
            case .stringLit, .expression, .expressionBlock, .markdownLit, .imageData:
                return t.pos + t.size + 1
            default:
                return t.pos + t.size
            }
        }

        /// The byte of the first character of the token. The lexer counts a token's size in characters, so a
        /// non-ASCII literal moves its pos; the end is right and the start is read back from it.
        private func start(_ t: TennToken) -> Int {
            let end = end(t)
            if t.type == .stringLit {
                var i = end - 2
                while i >= 0 {
                    if bytes[i] == 0x22 && !escaped(i) {
                        return i
                    }
                    i -= 1
                }
                return t.pos
            }
            return end - t.literal.utf8.count
        }

        private func escaped(_ i: Int) -> Bool {
            var backslashes = 0
            var j = i - 1
            while j >= 0, bytes[j] == 0x5C {
                backslashes += 1
                j -= 1
            }
            return backslashes % 2 == 1
        }

        private func span(_ t: TennToken) -> Scene.Range {
            Scene.Range(start: start(t), end: end(t))
        }

        private func span(_ list: ArraySlice<TennToken>) -> Scene.Range? {
            guard let first = list.first, let last = list.last else {
                return nil
            }
            return Scene.Range(start: start(first), end: end(last))
        }

        /// The arguments of the command at range, its name dropped: the tokens before its block, which the
        /// loader joins with a second one a command may carry.
        private func args(_ range: Scene.Range) -> [TennToken] {
            var out: [TennToken] = []
            for t in tokens where end(t) > range.start && end(t) <= range.end {
                if t.type == .curlyLe {
                    break
                }
                out.append(t)
            }
            if !out.isEmpty {
                out.removeFirst()
            }
            return out
        }

        /// The path and the name of a use: the path ends at its "as", the name follows it.
        func use(_ range: Scene.Range) -> (path: Scene.Range, name: Scene.Range)? {
            let a = args(range)
            guard let i = a.firstIndex(where: { $0.type == .symbol && $0.literal == "as" }), i >= 1, i + 1 < a.count,
                let path = span(a[..<i])
            else {
                return nil
            }
            return (path, span(a[i + 1]))
        }

        /// The path of an at or replace: every argument of the command is one.
        func path(_ range: Scene.Range) -> Scene.Range? {
            span(args(range)[...])
        }

        /// The name of an item, an element or a slot: the first argument of its command.
        func name(_ range: Scene.Range) -> Scene.Range? {
            guard let first = args(range).first else {
                return nil
            }
            return span(first)
        }

        /// The two ends of a link: its source and its target.
        func link(_ range: Scene.Range) -> (from: Scene.Range, to: Scene.Range)? {
            let a = args(range)
            guard a.count >= 2 else {
                return nil
            }
            return (span(a[0]), span(a[1]))
        }
    }
}

/// A reference being collected, so the same argument of the same file counts once.
private struct RefKey: Hashable {
    let file: String
    let start: Int
    let seg: Int
}

/// Collects the references to target while the files are expanded.
private final class RefFinder {
    private let t: Scene.Tree
    private let target: Scene.Node
    private var seen = Set<RefKey>()
    private var refs: [Scene.Ref] = []
    private var files: [String: Scene.RefFile] = [:]

    init(t: Scene.Tree, target: Scene.Node) {
        self.t = t
        self.target = target
    }

    /// The refs in the order Go lists them: file, then argument start.
    func sorted() -> [Scene.Ref] {
        refs.enumerated().sorted { a, b in
            if a.element.file != b.element.file {
                return a.element.file < b.element.file
            }
            if a.element.arg.start != b.element.arg.start {
                return a.element.arg.start < b.element.arg.start
            }
            return a.offset < b.offset
        }.map(\.element)
    }

    func file(_ name: String) -> Scene.RefFile? {
        if let found = files[name] {
            return found
        }
        guard let text = t.source(name) else {
            return nil
        }
        let found = Scene.RefFile(text)
        files[name] = found
        return found
    }

    func add(_ kind: Scene.RefKind, _ file: String, _ arg: Scene.Range?, _ text: String, _ seg: Int) {
        guard let arg, arg.start != 0 || arg.end != 0 else {
            return
        }
        guard seen.insert(RefKey(file: file, start: arg.start, seg: seg)).inserted else {
            return
        }
        refs.append(Scene.Ref(kind: kind, file: file, arg: arg, text: text, seg: seg))
    }

    /// Expands n and what is written below it as the expander does, in the context of its mount.
    func expand(_ n: Scene.Node, _ c: Scene.MountCtx?) {
        _ = RefWalk(f: self, ctx: c).container(n)
        var c = c
        if n.mount != nil {
            c = c?.push(n) ?? Scene.MountCtx(n, nil)
        }
        for e in n.elems {
            expand(e, c)
        }
    }

    /// Sees the path of a use that resolved from the context of its file. chain names the segments, the last
    /// node included, so the segment that names target is its index.
    func use(_ n: Scene.Node, _ chain: [Scene.Node]) {
        guard let i = chain.firstIndex(where: { $0 === target }), let path = file(n.pos.file)?.use(n.range)?.path
        else {
            return
        }
        add(.use, n.pos.file, path, n.path, i)
    }

    /// Sees the path of an at or replace that reached final from the root of the instance.
    func at(_ ov: Scene.Override, _ root: Scene.RNode, _ final: Scene.RNode) {
        guard let segs = splitPath(ov.path), let path = file(ov.pos.file)?.path(ov.range) else {
            return
        }
        let kind: Scene.RefKind = ov.kind == .replace ? .replace : .at
        for (i, r) in chainTo(root, segs, final).enumerated() where r.src === target {
            add(kind, ov.pos.file, path, ov.path, i)
        }
    }

    /// Adds the ends of the links next to target that name it; a name several siblings share blocks the rename.
    func links() -> String? {
        guard let p = target.parent, target.kind == .item || target.kind == .use else {
            return nil
        }
        let name = target.name
        let same = p.content.filter { ($0.kind == .item || $0.kind == .use) && $0.name == name }
        guard let at = same.firstIndex(where: { $0 === target }) else {
            return nil
        }
        for l in p.content where l.kind == .link {
            guard let ends = file(l.pos.file)?.link(l.range) else {
                continue
            }
            let sides = [(l.from, ends.from, "source-index"), (l.to, ends.to, "target-index")]
            for side in sides where side.0 == name {
                if same.count > 1 {
                    return "several items are named \(name) and a link addresses them by index"
                }
                if linkIndex(l, side.2) == at {
                    add(.link, l.pos.file, side.1, side.0, -1)
                }
            }
        }
        return nil
    }
}

/// The source-index or target-index of a link; the last one written wins, 0 when none.
private func linkIndex(_ l: Scene.Node, _ prop: String) -> Int {
    guard let v = l.prop(prop)?.num(0) else {
        return 0
    }
    return Int(v)
}

/// A node a path names from the root of an instance, with the place it holds in its parent.
private struct RefLoc {
    let parent: Scene.RNode
    let i: Int

    var node: Scene.RNode { parent.kids[i] }
}

/// The structural half of Scene.Expander, kept for the finder: which nodes a use or an override makes. Values,
/// parameters and contexts change no node, so this walk skips them and adds nothing to the diagnostics.
private final class RefWalk {
    private let f: RefFinder
    private var ctx: Scene.MountCtx?
    private var stack: [Scene.Node] = []
    private var depth = 0

    init(f: RefFinder, ctx: Scene.MountCtx?) {
        self.f = f
        self.ctx = ctx
    }

    /// Expands an element or a file: its content is the scene.
    func container(_ n: Scene.Node) -> Scene.RNode? {
        guard enter(n) else {
            return nil
        }
        defer { leave() }
        let saved = ctx
        if n.mount != nil {
            ctx = saved?.push(n) ?? Scene.MountCtx(n, nil)
        }
        defer { ctx = saved }
        let r = rnode(.item, n)
        r.kids = contents(n.content)
        return r
    }

    private func enter(_ n: Scene.Node) -> Bool {
        guard !stack.contains(where: { $0 === n }) else {
            return false
        }
        stack.append(n)
        return true
    }

    private func leave() { stack.removeLast() }

    private func rnode(_ kind: Scene.RKind, _ n: Scene.Node) -> Scene.RNode {
        Scene.RNode(kind: kind, name: n.name, key: n.key, pos: n.pos, props: n.props, src: n)
    }

    private func contents(_ nodes: [Scene.Node]) -> [Scene.RNode] {
        nodes.compactMap { content($0) }
    }

    private func content(_ n: Scene.Node) -> Scene.RNode? {
        switch n.kind {
        case .item:
            return item(n)
        case .use:
            return use(n)
        case .link:
            let r = rnode(.link, n)
            r.name = ""
            r.key = ""
            r.from = n.from
            r.to = n.to
            return r
        case .slot:
            let r = rnode(.slot, n)
            r.slot = n.key
            r.kids = contents(n.content)
            return r
        default:
            return nil
        }
    }

    /// Expands an item written in place: no use sets its parameters, which change no node.
    private func item(_ n: Scene.Node) -> Scene.RNode? {
        guard enter(n) else {
            return nil
        }
        defer { leave() }
        let r = rnode(.item, n)
        r.kids = contents(n.content)
        return r
    }

    /// Expands an instance. As the target of another use its slots stay open for that use.
    private func use(_ n: Scene.Node, asTarget: Bool = false) -> Scene.RNode? {
        guard let parent = n.parent, enter(n) else {
            return nil
        }
        defer { leave() }
        depth += 1
        defer { depth -= 1 }
        let outer = ctx
        guard let hit = lookup(parent, outer, n.path) else {
            return nil
        }
        f.use(n, hit.via + [hit.n])
        ctx = hit.c
        let inst = instance(hit.n)
        ctx = outer
        guard let inst else {
            return nil
        }
        inst.def = hit.n
        inst.name = n.name
        inst.key = n.key
        inst.pos = n.pos
        inst.inst = true
        inst.src = n
        apply(inst, n.block)
        if !asTarget && depth == 1 {
            finalize(inst)
        }
        return inst
    }

    private func instance(_ target: Scene.Node) -> Scene.RNode? {
        switch target.kind {
        case .item:
            return item(target)
        case .link:
            return content(target)
        case .use:
            return use(target, asTarget: true)
        case .element, .file:
            return container(target)
        default:
            return nil
        }
    }

    /// Puts the block of a use onto the instance root: nodes fill the default slot, at and replace reach into
    /// the instance, slot fills a named slot.
    private func apply(_ root: Scene.RNode, _ block: [Scene.Override]) {
        var fills: [RefFill] = []
        func add(_ name: String, _ nodes: [Scene.RNode], _ explicit: Bool) {
            for i in fills.indices where fills[i].name == name {
                fills[i].nodes += nodes
                fills[i].explicit = fills[i].explicit || explicit
                return
            }
            fills.append(RefFill(name: name, nodes: nodes, explicit: explicit))
        }
        for ov in block {
            switch ov.kind {
            case .prop:
                continue
            case .node:
                guard let n = ov.node else { continue }
                add("", contents([n]), false)
            case .slot:
                add(normalizeName(ov.slot), nodes(ov.block, slots: false), true)
            case .at:
                guard let at = find(root, ov.path) else { continue }
                f.at(ov, root, at.node)
                apply(at.node, ov.block)
            case .replace:
                guard let at = find(root, ov.path) else { continue }
                f.at(ov, root, at.node)
                let replacement = nodes(ov.block, slots: true)
                markReplaced(replacement)
                at.parent.kids = splice(at.parent.kids, at.i, replacement)
            }
        }
        for fill in fills {
            fillSlot(root, fill)
        }
    }

    /// Expands the block of a slot or replace, which takes nodes only; slots too in a replace block.
    private func nodes(_ block: [Scene.Override], slots: Bool) -> [Scene.RNode] {
        var out: [Scene.RNode] = []
        for ov in block {
            if ov.kind == .node {
                if let n = ov.node, let r = content(n) {
                    out.append(r)
                }
            } else if ov.kind == .slot && slots {
                let r = Scene.RNode(kind: .slot, name: ov.slot, pos: ov.pos)
                r.slot = normalizeName(ov.slot)
                r.kids = nodes(ov.block, slots: false)
                out.append(r)
            }
        }
        return out
    }

    /// Follows a path of names from the instance root: items, instances and slots, no links.
    private func find(_ root: Scene.RNode, _ path: String) -> RefLoc? {
        guard let segs = splitPath(path) else {
            return nil
        }
        var nodes = [root]
        var locs: [RefLoc] = []
        for seg in segs {
            locs = []
            for p in nodes {
                for (i, k) in p.kids.enumerated() where matchesSeg(k, seg) {
                    locs.append(RefLoc(parent: p, i: i))
                }
            }
            guard !locs.isEmpty else {
                return nil
            }
            nodes = locs.map(\.node)
        }
        guard nodes.count == 1, let loc = locs.first, loc.node.kind != .slot else {
            return nil
        }
        return loc
    }

    private func fillSlot(_ root: Scene.RNode, _ f: RefFill) {
        if f.name.isEmpty && f.nodes.isEmpty && !f.explicit {
            return
        }
        var at = findSlot(root, f.name)
        if at == nil && !f.name.isEmpty {
            let hits = forwardedSlots(root, f.name)
            if hits.count == 1 {
                at = hits[0]
            }
        }
        guard let at else {
            return
        }
        at.parent.kids = splice(at.parent.kids, at.i, f.nodes)
    }

    private func findSlot(_ r: Scene.RNode, _ name: String) -> RefLoc? {
        findSlotIn(r, name, true) ?? findSlotIn(r, name, false)
    }

    private func findSlotIn(_ r: Scene.RNode, _ name: String, _ replaced: Bool) -> RefLoc? {
        for (i, k) in r.kids.enumerated() {
            if k.kind == .slot && k.slot == name && k.replaced == replaced {
                return RefLoc(parent: r, i: i)
            }
            if k.inst {
                continue
            }
            if let l = findSlotIn(k, name, replaced) {
                return l
            }
        }
        return nil
    }

    /// The slots name that live in the components r uses: a wrapper passes a slot of a component it uses through.
    private func forwardedSlots(_ r: Scene.RNode, _ name: String) -> [RefLoc] {
        let replaced = collectSlots(r, name, false, true)
        return replaced.isEmpty ? collectSlots(r, name, false, false) : replaced
    }

    private func collectSlots(_ r: Scene.RNode, _ name: String, _ inInstance: Bool, _ replaced: Bool) -> [RefLoc] {
        var out: [RefLoc] = []
        for (i, k) in r.kids.enumerated() {
            if k.kind == .slot {
                if inInstance && k.slot == name && k.replaced == replaced {
                    out.append(RefLoc(parent: r, i: i))
                }
            } else if k.inst {
                out += collectSlots(k, name, true, replaced)
            } else {
                out += collectSlots(k, name, inInstance, replaced)
            }
        }
        return out
    }
}

/// One slot fill of the block of a use, waiting for all commands of the block to be read.
private struct RefFill {
    let name: String
    var nodes: [Scene.RNode]
    var explicit: Bool  // written as slot [name] { ... }: an empty block still replaces the fallback
}

/// Replaces the slots still open by their fallback content.
private func finalize(_ r: Scene.RNode) {
    var kids: [Scene.RNode] = []
    for k in r.kids {
        finalize(k)
        if k.kind == .slot {
            kids += k.kids
        } else {
            kids.append(k)
        }
    }
    r.kids = kids
}

/// Marks the slots the nodes of a replace declare: a fill reaches them before a slot of the same name the
/// component declares elsewhere (spec 4.7a).
private func markReplaced(_ nodes: [Scene.RNode]) {
    for n in nodes {
        if n.kind == .slot {
            n.replaced = true
        }
        markReplaced(n.kids)
    }
}

private func splice(_ kids: [Scene.RNode], _ i: Int, _ with: [Scene.RNode]) -> [Scene.RNode] {
    Array(kids[..<i]) + with + Array(kids[(i + 1)...])
}

/// Whether a segment of an at or replace path names the drawn node k; a slot is named "default" when it has no
/// name.
private func matchesSeg(_ k: Scene.RNode, _ seg: Scene.Segment) -> Bool {
    switch k.kind {
    case .item:
        if seg.quoted {
            return seg.name.precomposedStringWithCanonicalMapping == k.name.precomposedStringWithCanonicalMapping
        }
        return k.key == seg.key
    case .slot:
        if seg.quoted {
            return normalizeName(seg.name) == k.slot
        }
        return (k.slot.isEmpty ? "default" : k.slot) == seg.key
    default:
        return false
    }
}

/// The nodes the segments name from root down to final.
private func chainTo(_ root: Scene.RNode, _ segs: [Scene.Segment], _ final: Scene.RNode) -> [Scene.RNode] {
    guard let first = segs.first else {
        return []
    }
    for k in root.kids where matchesSeg(k, first) {
        if segs.count == 1 {
            if k === final {
                return [k]
            }
            continue
        }
        let rest = chainTo(k, Array(segs.dropFirst()), final)
        if !rest.isEmpty {
            return [k] + rest
        }
    }
    return []
}

/// A node a path names so far, with the context of the file it lives in.
private struct RefHit {
    let n: Scene.Node
    let c: Scene.MountCtx?
    let via: [Scene.Node]
}

/// The scope after a: its parent, or from the root of a mounted file the place it is mounted at (spec 5a.1).
private func outward(_ a: Scene.Node, _ c: Scene.MountCtx?) -> (Scene.Node?, Scene.MountCtx?) {
    if let p = a.parent {
        return (p, c)
    }
    guard let c else {
        return (nil, nil)
    }
    return (c.at.parent, c.up)
}

/// lookup of go/scene/resolve.go that tracks the nodes of the earlier segments, which a rename needs to see
/// where a path goes through it. Nothing is added to the diagnostics.
private func lookup(_ scope: Scene.Node, _ ctx: Scene.MountCtx?, _ path: String) -> RefHit? {
    guard let segs = splitPath(path), let first = segs.first else {
        return nil
    }
    var a: Scene.Node? = scope
    var c = ctx
    while let cur = a {
        var found = cur.named().filter { first.matches($0) }.map { RefHit(n: $0, c: c, via: []) }
        if !found.isEmpty {
            for seg in segs.dropFirst() {
                var next: [RefHit] = []
                for h in found {
                    var cc = h.c
                    if h.n.mount != nil {
                        cc = cc?.push(h.n) ?? Scene.MountCtx(h.n, nil)
                    }
                    for m in h.n.named() where seg.matches(m) {
                        next.append(RefHit(n: m, c: cc, via: h.via + [h.n]))
                    }
                }
                if next.isEmpty {
                    return nil
                }
                found = next
            }
            return found.count == 1 ? found[0] : nil
        }
        (a, c) = outward(cur, c)
    }
    return nil
}
