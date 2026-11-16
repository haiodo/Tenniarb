//
//  ScenePlaced.swift
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

extension Scene {
    /// An item of a built screen: where it is drawn and the command written for it.
    final class Placed {
        /// The item or the slot content written in a file, the use for the root of an instance, the element
        /// or file for the screen itself.
        let src: Node?
        /// The root of an instance: the items below it come from the component, except those the use passes in.
        let inst: Bool
        /// The position comes from pos (the parent has no flow layout, or the item is absolute).
        let free: Bool
        let parent: Placed?
        var kids: [Placed] = []  // in z order
        let x, y, w, h: Double  // in the coordinates of the screen

        let r: RNode

        init(src: Node?, inst: Bool, free: Bool, parent: Placed?, x: Double, y: Double, w: Double, h: Double, r: RNode) {
            self.src = src
            self.inst = inst
            self.free = free
            self.parent = parent
            self.x = x
            self.y = y
            self.w = w
            self.h = h
            self.r = r
        }

        /// The property in effect, the overrides of the uses around applied; nil when unset.
        func prop(_ name: String) -> Prop? { r.prop(name) }

        /// The properties in effect, the overrides of the uses around applied, in file order.
        func props() -> [Prop] { r.props }
    }

    /// The laid out rectangle of one item, relative to its parent.
    struct Box {
        var path = ""  // normalized names from the element, joined by "."
        var name = ""
        var x = 0.0
        var y = 0.0
        var w = 0.0
        var h = 0.0
        var depth = 0
        var lines: [String] = []  // wrapped text lines of an item with a title
    }

    /// One element built into the scene: its size, its laid out tree and what the drawing keeps.
    final class Screen {
        let name: String
        /// The extent of the content from the origin of the element, as the layout sees it.
        let w, h: Double
        let root: LayoutNode
        let tree: RNode
        let p: Prep

        init(name: String, w: Double, h: Double, root: LayoutNode, tree: RNode, p: Prep) {
            self.name = name
            self.w = w
            self.h = h
            self.root = root
            self.tree = tree
            self.p = p
        }

        /// Whether the screen is a classic diagram rather than a scene.
        var classic: Bool { p.classic }

        /// The items of the screen in paint order, the element itself first.
        func boxes() -> [Box] {
            var out: [Box] = []
            func walk(_ rn: RNode, _ path: String, _ depth: Int) {
                guard let ln = p.frames[ObjectIdentifier(rn)] else { return }
                var b = Box()
                b.path = path
                b.name = rn.name
                b.x = ln.rx
                b.y = ln.ry
                b.w = ln.rw
                b.h = ln.rh
                b.depth = depth
                if let ti = p.texts[ObjectIdentifier(rn)] {
                    b.lines = ti.wrap(ln.rw - 2 * ti.padX)
                }
                out.append(b)
                if p.styles[ObjectIdentifier(rn)]?.display == "none" {  // not drawn, its children with it
                    return
                }
                for k in p.paintOrder(rn) where k.kind == .item {
                    guard p.frames[ObjectIdentifier(k)] != nil else { continue }
                    let kp = path.isEmpty ? k.key : path + "." + k.key
                    walk(k, kp, depth + 1)
                }
            }
            walk(tree, "", 0)
            return out
        }

        /// The lines drawn for the links of the screen, in paint order, each with the path of the item whose
        /// coordinates it is drawn in.
        func links() -> [Link] {
            var out: [Link] = []
            func walk(_ rn: RNode, _ path: String) {
                if rn !== tree, p.styles[ObjectIdentifier(rn)]?.display == "none" {
                    return  // not drawn, its children with it
                }
                for k in p.paintOrder(rn) {
                    switch k.kind {
                    case .item:
                        guard p.frames[ObjectIdentifier(k)] != nil else { continue }
                        walk(k, path.isEmpty ? k.key : path + "." + k.key)
                    case .link:
                        p.checkRoute(k)
                        guard var link = p.linkPicture(rn, k) else { continue }
                        link.parent = path
                        out.append(link)
                    case .slot:
                        break  // gone after finalize
                    }
                }
            }
            walk(tree, "")
            return out
        }

        /// The tree of the drawn items, the screen itself at the root. Hidden items are not in it.
        func placed() -> Placed {
            func walk(_ rn: RNode, _ parent: Placed?, _ ox: Double, _ oy: Double, _ parentDir: Dir) -> Placed {
                let ln = p.frames[ObjectIdentifier(rn)]!
                let pl = Placed(
                    src: rn.src, inst: rn.inst, free: parentDir == .abs || ln.absolute, parent: parent,
                    x: ox + ln.rx, y: oy + ln.ry, w: ln.rw, h: ln.rh, r: rn)
                if p.styles[ObjectIdentifier(rn)]?.display == "none" {  // not drawn, its children with it
                    return pl
                }
                for k in p.paintOrder(rn) where k.kind == .item && p.frames[ObjectIdentifier(k)] != nil {
                    pl.kids.append(walk(k, pl, pl.x, pl.y, ln.dir))
                }
                return pl
            }
            return walk(tree, nil, 0, 0, .abs)
        }
    }
}

extension Scene.Prep {
    /// Lists the kids in draw order: background items under the rest, hover items over them, the file order
    /// within each pass. links keep the place of an item of no layer.
    func paintOrder(_ rn: Scene.RNode) -> [Scene.RNode] {
        guard rn.kids.count >= 2 else { return rn.kids }
        return rn.kids.enumerated()
            .sorted { (layerKey($0.element), $0.offset) < (layerKey($1.element), $1.offset) }
            .map(\.element)
    }

    /// The position of a node among its siblings; the zero layer is the middle pass. A classic diagram
    /// draws its links first, so a line passes under the boxes it crosses.
    func layerKey(_ rn: Scene.RNode) -> Int {
        guard rn.kind == .item else { return classic ? 0 : 1 }
        guard let layer = styles[ObjectIdentifier(rn)]?.layer else { return 1 }
        switch layer {
        case layerBackground: return 0
        case layerHover: return 2
        default: return 1
        }
    }
}

extension Scene.Tree {
    /// Builds the element at path ("" is the content of the file itself): expanded, styled and laid out.
    /// The error holds what is wrong with the element; the screen is nil then.
    func build(
        _ path: String = "", measurer: Scene.TextMeasurer = Scene.TextMeasurer(), eval: Scene.EvalFunc? = nil,
        classic: Scene.ClassicFunc? = nil
    ) -> Scene.Screen? {
        build(measurer, eval, classic) { self.element(path, eval: eval) }
    }

    /// buildElement is build of an element or file root of the tree, for a caller that holds the node:
    /// a path of names cannot tell elements with one name apart.
    func buildElement(
        _ el: Scene.Node, measurer: Scene.TextMeasurer = Scene.TextMeasurer(), eval: Scene.EvalFunc? = nil,
        classic: Scene.ClassicFunc? = nil
    ) -> Scene.Screen? {
        build(measurer, eval, classic) {
            guard el.isContainer() else {
                self.fail(el.pos, "a \(el.kind.description) is not an element")
                return nil
            }
            return self.expandElement(el, self.ctxOf(el), eval: eval)
        }
    }

    private func build(
        _ measurer: Scene.TextMeasurer, _ eval: Scene.EvalFunc?, _ classic: Scene.ClassicFunc?, _ expand: () -> Scene.RNode?
    ) -> Scene.Screen? {
        let failures = diagnostics.filter { !$0.warn }.count
        guard let rn = expand() else { return nil }
        let p = Scene.Prep(self, measurer: measurer, eval: eval, classic: classic)
        p.classic = !sceneRoot(rn)
        // The default look of a classic diagram is settled before the styles: a node may still name its own.
        if p.classic {
            classicDefaults(rn)
            // The element model names every node of a classic diagram, for Options.Classic.
            if classic != nil {
                p.classicRefs(rn)
            }
        }
        // The expressions of the screen come before the styles: use-style ${kind} picks a style by its value.
        if p.canEval() {
            p.evalTree(rn)
        }
        applyStyles(rn)
        let root = p.root(rn)
        if diagnostics.filter({ !$0.warn }).count > failures {
            return nil
        }
        root.layout()
        p.warnOutside(rn)
        p.warnBodyFit(rn)
        return Scene.Screen(name: rn.name, w: root.rw, h: root.rh, root: root, tree: rn, p: p)
    }
}
