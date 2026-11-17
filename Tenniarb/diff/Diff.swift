//
//  Diff.swift
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

/// Compares two texts of one .tenn file node by node, as go/diff: a node is matched by the names from
/// the file root down; a node that changed parent is matched by its name among the nodes left over.
enum Diff {
    /// What happened to a node between the two versions; nil is a node of both, kept to hold its kids.
    enum Status: String {
        case added
        case removed
        case changed
        case moved
    }

    /// One property that differs; an empty side means the property is not written there.
    struct Prop: Equatable {
        var name: String
        var old: String
        var new: String
    }

    /// One element, item, link or use of the comparison tree.
    final class Node {
        let path: String  // elements joined by "/", then nodes by "."; "" for the file root
        let kind: String  // file, element, item, link, use, slot
        let name: String  // as written in the file, for a row
        var status: Status?
        var props: [Prop] = []
        var note = ""  // where a moved node came from, or why it moved
        /// The element that holds the node (its own path for an element row), and the path inside it;
        /// sel is "" for an element.
        let element: String
        let sel: String
        /// The node in each version; nil on the side it is not written.
        let old: Scene.Node?
        let new: Scene.Node?
        fileprivate(set) var kids: [Node] = []
        fileprivate let parent: String  // the path of the parent

        fileprivate init(
            path: String, kind: String, name: String, element: String, sel: String, parent: String, old: Scene.Node?, new: Scene.Node?
        ) {
            self.path = path
            self.kind = kind
            self.name = name
            self.element = element
            self.sel = sel
            self.parent = parent
            self.old = old
            self.new = new
        }
    }

    /// The comparison of two texts of one file.
    final class Report {
        let root: Node
        let old: String
        let new: String
        private var byPath: [String: Node] = [:]
        private var counts: [Status: Int] = [:]

        fileprivate init(root: Node, old: String, new: String) {
            self.root = root
            self.old = old
            self.new = new
            index(root)
        }

        /// The node of the report at a path, as Node.path writes it.
        func node(_ path: String) -> Node? { byPath[path] }

        /// How many nodes have status s.
        func count(_ status: Status) -> Int { counts[status] ?? 0 }

        /// Whether the two versions differ in any node.
        var changed: Bool { Status.counts.contains { count($0) > 0 } }

        /// The paths of the nodes with status s, in file order.
        func paths(_ status: Status) -> [String] {
            var out: [String] = []
            func walk(_ n: Node) {
                if n.status == status {
                    out.append(n.path)
                }
                for k in n.kids {
                    walk(k)
                }
            }
            walk(root)
            return out
        }

        private func index(_ n: Node) {
            byPath[n.path] = n
            if let status = n.status {
                counts[status, default: 0] += 1
            }
            for k in n.kids {
                index(k)
            }
        }
    }

    /// Compares old and new as two texts of the file rel under the workspace root. Both texts load the
    /// way the document loads them, so a mount of the file is read as it is on disk.
    static func compare(root: URL, rel: String, old: String, new: String) throws -> Report {
        let oldFile: Scene.Node
        do {
            oldFile = try load(root, rel, old)
        } catch {
            throw SceneLoadError(message: "the old version: \(error)")
        }
        let newFile: Scene.Node
        do {
            newFile = try load(root, rel, new)
        } catch {
            throw SceneLoadError(message: "the new version: \(error)")
        }
        let oldRoot = Entry(node: oldFile)
        oldRoot.kind = "file"
        oldRoot.name = baseName(rel)
        oldRoot.kids = collect(oldFile, "", "")
        let newRoot = Entry(node: newFile)
        newRoot.kind = "file"
        newRoot.name = baseName(rel)
        newRoot.kids = collect(newFile, "", "")
        let builder = Builder(movedFrom: matchMoved(oldRoot, newRoot))
        let root = builder.node(oldRoot, newRoot)
        root.kids = builder.kids(oldRoot.kids, newRoot.kids)
        return Report(root: root, old: old, new: new)
    }

    /// The name a row gives the file: its last component without the extension.
    private static func baseName(_ rel: String) -> String {
        ((rel as NSString).lastPathComponent as NSString).deletingPathExtension
    }

    /// Loads one text of the file as the document loader does: from an overlay at its own path.
    private static func load(_ root: URL, _ rel: String, _ text: String) throws -> Scene.Node {
        let loader = SceneLoader(root: root)
        loader.overlay[rel] = text
        let tree = try loader.loadAuto(rel)
        guard let file = tree.main ?? tree.file(rel) else {
            throw SceneLoadError(message: "\(rel) is not part of the tree")
        }
        return file
    }

    // MARK: - The nodes of one version

    /// One node of one version: what the comparison needs about it.
    private final class Entry {
        var path = ""
        var parent = ""
        var elem = ""
        var sel = ""
        var kind = ""
        var name = ""
        var key = ""
        let node: Scene.Node
        var kids: [Entry] = []
        var values: [String: String] = [:]

        init(node: Scene.Node) {
            self.node = node
        }
    }

    /// Lists what container holds in file order: its elements, then the items, uses, links and slots.
    private static func collect(_ c: Scene.Node, _ elemPath: String, _ nodePath: String) -> [Entry] {
        struct At {
            let pos: Int
            let offset: Int
            let entry: Entry
        }
        var all: [At] = []
        for (i, el) in c.elems.enumerated() {
            let seg = segOf(c.elems, el, key: false)
            let path = elemPath.isEmpty ? seg : elemPath + "/" + seg
            let e = Entry(node: el)
            e.path = path
            e.parent = nodePath
            e.elem = path
            e.kind = "element"
            e.name = el.name
            e.key = el.name
            e.kids = collect(el, path, path)
            e.values = values(el)
            all.append(At(pos: el.range.start, offset: i, entry: e))
        }
        for (i, k) in c.content.enumerated() {
            guard let e = nodeEntry(c, k, elemPath, nodePath) else { continue }
            all.append(At(pos: k.range.start, offset: c.elems.count + i, entry: e))
        }
        all.sort { $0.pos == $1.pos ? $0.offset < $1.offset : $0.pos < $1.pos }
        return all.map(\.entry)
    }

    /// An item, use, link or slot of container as an entry; nil for a command that is no node.
    private static func nodeEntry(_ c: Scene.Node, _ k: Scene.Node, _ elemPath: String, _ nodePath: String) -> Entry? {
        switch k.kind {
        case .item, .use, .slot, .link:
            break
        default:
            return nil
        }
        let e = Entry(node: k)
        e.kind = k.kind.description
        e.parent = nodePath
        e.elem = elemPath
        e.values = values(k)
        switch k.kind {
        case .link:
            e.name = k.from + " -> " + k.to
            e.key = "link:" + k.from + "->" + k.to
            e.sel = e.key
        case .slot:
            e.name = "slot " + k.name
            e.key = "slot:" + k.key
            e.sel = e.key
        default:
            e.name = k.name
            e.key = k.key
            e.sel = segOf(sceneKids(c), k, key: true)
        }
        e.path = nodePath.isEmpty ? e.sel : nodePath + "." + e.sel
        e.kids = collect(k, elemPath, e.path)
        return e
    }

    /// The segment of n among its siblings: its name or key, with #n for a repeat.
    private static func segOf(_ siblings: [Scene.Node], _ n: Scene.Node, key: Bool) -> String {
        let base = key ? n.key : n.name
        var k = 0
        for c in siblings {
            if c === n {
                break
            }
            if key && c.key == n.key || !key && c.name == n.name {
                k += 1
            }
        }
        return k > 0 ? base + "#\(k)" : base
    }

    /// The nodes a path goes through under n: its items, uses and slots.
    private static func sceneKids(_ n: Scene.Node) -> [Scene.Node] {
        n.content.filter { $0.kind == .item || $0.kind == .use || $0.kind == .slot }
    }

    /// The properties of n as the file writes them, by name. A repeated property keeps every value;
    /// styles, the kind, parameters, the template and the link ends count as properties too.
    private static func values(_ n: Scene.Node) -> [String: String] {
        var m: [String: String] = [:]
        func add(_ name: String, _ value: String) {
            if let old = m[name] {
                m[name] = old + " " + value
            } else {
                m[name] = value
            }
        }
        switch n.kind {
        case .element:
            if !n.type.isEmpty {
                add("kind", n.type)
            }
            if let mount = n.mount {
                add("mount", mount.pos.file)
            }
            for p in n.params {
                add("param " + p.name, p.defaultValue.map { argsText($0.args) } ?? "(required)")
            }
            if let template = n.template {
                add("template", template.text)
            }
        case .use:
            add("use", n.path)
            for ov in n.block {
                switch ov.kind {
                case .prop:
                    if let prop = ov.prop {
                        add(prop.name, argsText(prop.args))
                    }
                case .at, .replace:
                    add("at " + ov.path, "")
                case .slot, .node:
                    break
                }
            }
        case .link:
            add("link", n.from + " -> " + n.to)
        default:
            break
        }
        for p in n.props {
            add(p.name, argsText(p.args))
        }
        for name in (n.styles ?? [:]).keys.sorted() {
            if let style = n.styles?[name] {
                add("style " + name, styleText(style))
            }
        }
        return m
    }

    /// The arguments of a command as the file writes them, joined by spaces.
    private static func argsText(_ args: [TennNode]) -> String {
        args.map { $0.getIdentText() ?? "" }.joined(separator: " ")
    }

    /// One style as the file writes it, for a comparison.
    private static func styleText(_ style: Scene.Style) -> String {
        var out = ""
        for p in style.props {
            out += p.name
            let text = argsText(p.args)
            if !text.isEmpty {
                out += " " + text
            }
            out += "; "
        }
        for k in style.kids {
            out += k.key + " { " + styleText(k.style) + "} "
        }
        return out.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    // MARK: - Moved nodes

    /// Pairs the nodes that have no counterpart by path with one of the other version by name: a node
    /// that changed parent. It returns the old node of every new node so paired.
    private static func matchMoved(_ oldRoot: Entry, _ newRoot: Entry) -> [ObjectIdentifier: Entry] {
        var oldByPath: [String: Entry] = [:]
        var newByPath: [String: Entry] = [:]
        walk(oldRoot) { oldByPath[$0.path] = $0 }
        walk(newRoot) { newByPath[$0.path] = $0 }
        func byKey(_ root: Entry, matched: (Entry) -> Bool) -> [String: [Entry]] {
            var out: [String: [Entry]] = [:]
            walk(root) { e in
                if matched(e) || e.key.isEmpty {
                    return
                }
                out[e.kind + "\u{0}" + e.key, default: []].append(e)
            }
            return out
        }
        let oldLeft = byKey(oldRoot) { newByPath[$0.path] != nil }
        let newLeft = byKey(newRoot) { oldByPath[$0.path] != nil }
        var out: [ObjectIdentifier: Entry] = [:]
        for (key, nodes) in newLeft {
            // One node of each version with that name: any more and the pair is a guess.
            if let olds = oldLeft[key], nodes.count == 1, olds.count == 1 {
                out[ObjectIdentifier(nodes[0])] = olds[0]
            }
        }
        return out
    }

    private static func walk(_ e: Entry, _ visit: (Entry) -> Void) {
        visit(e)
        for k in e.kids {
            walk(k, visit)
        }
    }

    /// Turns two entry trees into the nodes of the report.
    private final class Builder {
        let movedFrom: [ObjectIdentifier: Entry]

        init(movedFrom: [ObjectIdentifier: Entry]) {
            self.movedFrom = movedFrom
        }

        /// The report node of an entry pair; either side may be nil.
        func node(_ oldE: Entry?, _ newE: Entry?) -> Node {
            guard let e = newE ?? oldE else {
                preconditionFailure("a report node names a side")
            }
            let n = Node(
                path: e.path, kind: e.kind, name: e.name, element: e.elem, sel: e.sel, parent: e.parent, old: oldE?.node, new: newE?.node)
            if oldE == nil {
                n.status = .added
            } else if newE == nil {
                n.status = .removed
            } else if let oldE, let newE {
                let props = propChanges(oldE.values, newE.values)
                if !props.isEmpty {
                    n.status = .changed
                    n.props = props
                }
                if oldE.parent != newE.parent {
                    n.status = .moved
                    n.note = "from " + displayPath(oldE.parent)
                }
            }
            return n
        }

        /// The rows of the children of a paired node: matched by path, then by name when the node moved,
        /// and the nodes the new version no longer has put back where they were.
        func kids(_ oldKids: [Entry], _ newKids: [Entry]) -> [Node] {
            var oldByPath: [String: Entry] = [:]
            for k in oldKids {
                oldByPath[k.path] = k
            }
            var newByPath: [String: Entry] = [:]
            for k in newKids {
                newByPath[k.path] = k
            }
            var out: [Node] = []
            var pairedOld: [ObjectIdentifier: Node] = [:]
            var newSeq: [Node] = []
            for nk in newKids {
                var ok = oldByPath[nk.path]
                if ok == nil, let moved = movedFrom[ObjectIdentifier(nk)] {
                    // another node has its old path now: not the same node
                    ok = newByPath[moved.path] == nil ? moved : nil
                }
                let n = node(ok, nk)
                if let ok {
                    pairedOld[ObjectIdentifier(ok)] = n
                    newSeq.append(n)
                }
                n.kids = kids(ok?.kids ?? [], nk.kids)
                out.append(n)
            }
            for (i, ok) in oldKids.enumerated() {
                if newByPath[ok.path] != nil || isMoved(ok) {
                    continue
                }
                var at = out.count
                for next in oldKids[(i + 1)...] {
                    if let j = out.firstIndex(where: { $0.path == next.path }) {
                        at = j
                        break
                    }
                }
                let n = node(ok, nil)
                n.kids = kids(ok.kids, [])
                out.insert(n, at: at)
            }
            // A node whose place among the siblings both versions hold changed has moved; an added or
            // a removed node around it does not move it.
            markOrder(oldKids, newSeq, pairedOld)
            return out
        }

        /// Flags the paired nodes whose order among the siblings changed: every node outside the
        /// longest run that kept its order has moved.
        private func markOrder(_ oldKids: [Entry], _ newSeq: [Node], _ pairedOld: [ObjectIdentifier: Node]) {
            var pos: [ObjectIdentifier: Int] = [:]
            for (i, ok) in oldKids.enumerated() {
                if let n = pairedOld[ObjectIdentifier(ok)] {
                    pos[ObjectIdentifier(n)] = i
                }
            }
            var idx: [Int] = []
            for n in newSeq {
                guard let p = pos[ObjectIdentifier(n)] else { return }
                idx.append(p)
            }
            let keep = longestIncreasing(idx)
            for (i, n) in newSeq.enumerated() where !keep[i] && n.status == nil {
                n.status = .moved
                n.note = "moved among siblings"
            }
        }

        /// Whether old entry e is written in the new version under another path.
        private func isMoved(_ e: Entry) -> Bool {
            movedFrom.values.contains { $0 === e }
        }
    }

    /// One longest strictly increasing subsequence of a, by index.
    private static func longestIncreasing(_ a: [Int]) -> [Bool] {
        var keep = Array(repeating: false, count: a.count)
        if a.isEmpty {
            return keep
        }
        var prev = Array(repeating: -1, count: a.count)
        var tails: [Int] = []  // indices into a: the smallest tail of a run of each length
        for (i, v) in a.enumerated() {
            var lo = 0
            var hi = tails.count
            while lo < hi {
                let mid = (lo + hi) / 2
                if a[tails[mid]] < v {
                    lo = mid + 1
                } else {
                    hi = mid
                }
            }
            if lo > 0 {
                prev[i] = tails[lo - 1]
            }
            if lo == tails.count {
                tails.append(i)
            } else {
                tails[lo] = i
            }
        }
        var i = tails[tails.count - 1]
        while i >= 0 {
            keep[i] = true
            i = prev[i]
        }
        return keep
    }

    /// The properties that differ, by name.
    private static func propChanges(_ oldV: [String: String], _ newV: [String: String]) -> [Prop] {
        var names = Array(oldV.keys)
        for k in newV.keys where oldV[k] == nil {
            names.append(k)
        }
        names.sort()
        var out: [Prop] = []
        for k in names where oldV[k] != newV[k] {
            out.append(Prop(name: k, old: oldV[k] ?? "", new: newV[k] ?? ""))
        }
        return out
    }

    private static func displayPath(_ path: String) -> String {
        path.isEmpty ? "the file" : path.replacingOccurrences(of: "/", with: " / ")
    }
}

extension Diff.Status {
    /// Every status a report counts, in the order a summary lists them.
    static let counts: [Diff.Status] = [.changed, .added, .removed, .moved]
}

// MARK: - The version in git

extension Diff {
    /// The version a comparison starts from: the last commit.
    static let revHead = "HEAD"

    /// The repository that holds dir: the top folder of the work tree, symlinks as git gives them.
    /// nil when dir is not in a repository, or git is not installed.
    static func repo(_ dir: URL) -> String? {
        guard let out = git(["-C", dir.path, "rev-parse", "--show-toplevel"]), out.status == 0 else {
            return nil
        }
        let root = out.text.trimmingCharacters(in: .whitespacesAndNewlines)
        return root.isEmpty ? nil : root
    }

    /// Reads rel (a slash path from the repository root) as rev has it in the repository; nil when
    /// there is no such version, or git cannot read it.
    static func show(repo: String, rev: String = revHead, rel: String) -> String? {
        let rev = rev.isEmpty ? revHead : rev
        guard let out = git(["-C", repo, "show", rev + ":" + rel]), out.status == 0 else {
            return nil
        }
        return out.text
    }

    /// Runs git and reads what it wrote; nil when git cannot be started.
    private static func git(_ args: [String], timeout: TimeInterval = 3) -> (status: Int32, text: String)? {
        let scratch = FileManager.default.temporaryDirectory.appendingPathComponent("tenniarb-git-\(UUID().uuidString)", isDirectory: true)
        guard (try? FileManager.default.createDirectory(at: scratch, withIntermediateDirectories: true)) != nil else {
            return nil
        }
        defer { try? FileManager.default.removeItem(at: scratch) }
        let outURL = scratch.appendingPathComponent("out")
        let errURL = scratch.appendingPathComponent("err")
        guard FileManager.default.createFile(atPath: outURL.path, contents: nil),
            FileManager.default.createFile(atPath: errURL.path, contents: nil),
            let outHandle = try? FileHandle(forWritingTo: outURL),
            let errHandle = try? FileHandle(forWritingTo: errURL)
        else {
            return nil
        }
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        process.arguments = ["git"] + args
        process.standardOutput = outHandle
        process.standardError = errHandle
        let done = DispatchSemaphore(value: 0)
        process.terminationHandler = { _ in done.signal() }
        do {
            try process.run()
        } catch {
            return nil
        }
        if done.wait(timeout: .now() + timeout) == .timedOut {
            process.terminate()
            _ = done.wait(timeout: .now() + 1)
            return nil
        }
        try? outHandle.close()
        try? errHandle.close()
        let text = (try? String(contentsOf: outURL, encoding: .utf8)) ?? ""
        return (process.terminationStatus, text)
    }
}
