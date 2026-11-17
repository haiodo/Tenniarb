//
//  PenEmit.swift
//  Tenniarb
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

import Foundation
import Tenn

extension PenImport {
    static func argNode(_ a: PenArg) -> TennNode {
        a.quoted ? TennNode.newStrNode(a.s) : TennNode.newIdent(a.s)
    }

    static func propNode(_ p: PenProp) -> TennNode {
        let cmd = TennNode.newCommand(p.name)
        for a in p.args {
            cmd.add(argNode(a))
        }
        return cmd
    }

    static func withBlock(_ cmd: TennNode, _ body: [TennNode]) -> TennNode {
        if !body.isEmpty {
            let blk = TennNode(kind: .BlockExpr)
            for n in body {
                blk.add(n)
            }
            cmd.add(blk)
        }
        return cmd
    }

    static func emitNode(_ n: PenNode) -> TennNode {
        var body: [TennNode] = []
        for p in n.props {
            body.append(propNode(p))
        }
        switch n.kind {
        case .slot:
            for k in n.kids {
                body.append(emitNode(k))
            }
            return withBlock(TennNode.newCommand("slot", TennNode.newIdent(n.slot)), body)
        case .use:
            for ov in n.block {
                body.append(emitOvr(ov))
            }
            let target = n.target.map { pathTo(n.file, $0) } ?? ""
            let asNode = simpleIdent(n.name) ? TennNode.newIdent(n.name) : TennNode.newStrNode(n.name)
            return withBlock(
                TennNode.newCommand("use", TennNode.newIdent(target), TennNode.newIdent("as"), asNode), body)
        case .item, .file:
            break
        }
        var leaf = n.type != "text"
        for k in n.kids {
            leaf = leaf && k.kind == .slot
            body.append(emitNode(k))
        }
        if leaf && !n.b.layout && n.type != "frame" && !hasProp(n.props, "path") {
            // outside a layout the name of an item is its text, and a leaf is as big as its text
            body.insert(propNode(prop("title", strArg(""))), at: 0)
        }
        return withBlock(TennNode.newCommand("item", TennNode.newStrNode(n.name)), body)
    }

    static func chainPath(_ chain: [PenNode]) -> String {
        chain.compactMap(pathSeg).joined(separator: ".")
    }

    static func emitOvr(_ ov: PenOvr) -> TennNode {
        var body: [TennNode] = []
        switch ov.kind {
        case .at:
            for p in ov.props {
                body.append(propNode(p))
            }
            return withBlock(TennNode.newCommand("at", TennNode.newIdent(chainPath(ov.chain))), body)
        case .replace:
            for n in ov.nodes {
                body.append(emitNode(n))
            }
            return withBlock(TennNode.newCommand("replace", TennNode.newIdent(chainPath(ov.chain))), body)
        case .slot:
            break
        }
        for n in ov.nodes {
            body.append(emitNode(n))
        }
        let fill = withBlock(TennNode.newCommand("slot", TennNode.newIdent(ov.slot?.slot ?? "")), body)
        if !ov.chain.isEmpty {
            return withBlock(TennNode.newCommand("at", TennNode.newIdent(chainPath(ov.chain))), [fill])
        }
        return fill
    }
}

extension PenResult {
    /// Prints the converted file.
    func tenn(of f: PenFile) -> String {
        let stmts = TennNode.newNode(kind: .Statements)
        let imports = f.imports.values.sorted { $0.name < $1.name }
        for lib in imports {
            stmts.add(
                TennNode.newCommand(
                    "element", TennNode.newStrNode(f.mountAlias(lib)), TennNode.newIdent(SceneLoader.fromFileFlag),
                    TennNode.newStrNode(lib.name + ".tenn")))
        }
        for p in f.rootContext {
            stmts.add(PenImport.propNode(p))
        }
        for n in f.root.kids {
            stmts.add(PenImport.emitNode(n))
        }
        PenImport.dropDefaults(stmts.children ?? [], false)
        return PenCompact.compact(stmts, indent: "\t", limit: PenCompact.limit, pack: true) + "\n"
    }

    /// Saves the .tenn of every converted file into dir, and nothing else.
    func write(to dir: String) throws {
        let base = URL(fileURLWithPath: dir)
        try FileManager.default.createDirectory(at: base, withIntermediateDirectories: true)
        for f in files {
            let file = base.appendingPathComponent(f.name + ".tenn")
            do {
                try tenn(of: f).write(to: file, atomically: true, encoding: .utf8)
            } catch {
                throw PenImportFailure(message: "\(file.path): \(error.localizedDescription)")
            }
        }
    }
}

// MARK: - Defaults

// Properties that repeat what go/scene assumes are dropped. Inside a layout (an auto-layout on the item or
// on its parent) an item has no fill and no border; outside, a white fill and a black border of 1.

extension PenImport {
    static func hasCmd(_ cmds: [TennNode], _ name: String) -> Bool {
        cmds.contains { cmdName($0) == name }
    }

    static func hasKids(_ cmds: [TennNode]) -> Bool {
        hasCmd(cmds, "item") || hasCmd(cmds, "use") || hasCmd(cmds, "slot")
    }

    /// Read from an item only when it has an auto-layout.
    static let containerProps = ["padding", "gap", "line-gap", "wrap", "align", "justify"]

    /// Tells whether the plain command p of an item holds the default of its context.
    static func defaultValue(_ p: TennNode, _ cmds: [TennNode], _ parentAuto: Bool) -> Bool {
        let auto = hasCmd(cmds, "auto-layout")
        switch cmdName(p) {
        case "color":
            return argText(p, 1) == "#00000000" && (auto || parentAuto)
        case "line-width":
            guard let w = Double(argText(p, 1)) else { return false }
            return w == 1 || (auto || parentAuto) && !hasCmd(cmds, "border-color")
        case "text-color":
            return argText(p, 1) == "#000000" && !hasCmd(cmds, "color")
        case "auto-layout":
            if argText(p, 1) != "row" || !parentAuto || hasKids(cmds) {
                return false
            }
            for name in containerProps where hasCmd(cmds, name) {
                return false
            }
            return true
        default:
            return false
        }
    }

    /// Walks cmds, the content of a parent with an auto-layout when parentAuto. The text does not tell
    /// where the content of a use lands in the component: it counts as outside a layout.
    static func dropDefaults(_ cmds: [TennNode], _ parentAuto: Bool) {
        for c in cmds {
            guard let blk = blockOf(c) else { continue }
            switch cmdName(c) {
            case "item":
                var kept: [TennNode] = []
                let children = blk.children ?? []
                let auto = hasCmd(children, "auto-layout") || parentAuto
                for p in children {
                    if isStructure(cmdName(p)) || !defaultValue(p, children, parentAuto) {
                        kept.append(p)
                    }
                }
                dropDefaults(kept, auto)
                blk.children = kept
            case "slot":
                dropDefaults(blk.children ?? [], parentAuto)
            default:
                dropDefaults(blk.children ?? [], false)
            }
        }
    }

    static func cmdName(_ c: TennNode) -> String {
        c.getIdent(0) ?? ""
    }

    static func blockOf(_ c: TennNode) -> TennNode? {
        let children = c.children ?? []
        if children.count > 1, children[children.count - 1].kind == .BlockExpr {
            return children[children.count - 1]
        }
        return nil
    }

    static func isStructure(_ name: String) -> Bool {
        ["item", "use", "at", "replace", "slot", "element"].contains(name)
    }

    static func argText(_ c: TennNode, _ i: Int) -> String {
        c.getChild(i)?.getIdentText() ?? ""
    }
}
