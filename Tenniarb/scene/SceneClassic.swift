//
//  SceneClassic.swift
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
    /// ClassicRef names a node of a classic diagram as the element model sees it: the element, an item by
    /// name and index, or a link by its two ends. Nth numbers the nodes of that name in file order from 0.
    struct ClassicRef: Hashable {
        var kind = RKind.item  // .item: the element or an item; .link: a link
        var name = ""  // .item: the name
        var from = ""
        var to = ""  // .link: the names of the ends
        var nth = 0
        var root = false  // .item: the element itself, not an item
    }

    /// Options.Classic: the text of every argument of prop on the node at, from the element model of a
    /// classic diagram; nil when the model does not hold it and the scene evaluates the property itself.
    typealias ClassicFunc = (_ at: ClassicRef, _ prop: String) -> [String]?
}

extension Scene.Prep {
    /// Whether an expression can become a value: an evaluator of the scene, or the values of the element
    /// model in a classic diagram.
    func canEval() -> Bool { eval != nil || (classic && classicVals != nil) }

    /// classicRefs numbers the nodes of a classic diagram the way the element model names them, so
    /// Options.Classic can find the item or link each drawn node stands for.
    func classicRefs(_ root: Scene.RNode) {
        classicAt = [:]
        var items: [String: Int] = [:]
        var links: [[String]: Int] = [:]
        func walk(_ rn: Scene.RNode, _ isRoot: Bool) {
            switch rn.kind {
            case .item:
                let key = rn.name.precomposedStringWithCanonicalMapping
                var at = Scene.ClassicRef(kind: .item, name: rn.name, root: isRoot)
                if !isRoot {  // the element is not one of the items the model numbers
                    at.nth = items[key] ?? 0
                    items[key] = at.nth + 1
                }
                classicAt[ObjectIdentifier(rn)] = at
            case .link:
                let key = [rn.from.precomposedStringWithCanonicalMapping, rn.to.precomposedStringWithCanonicalMapping]
                classicAt[ObjectIdentifier(rn)] = Scene.ClassicRef(
                    kind: .link, from: rn.from, to: rn.to, nth: links[key] ?? 0)
                links[key] = (links[key] ?? 0) + 1
            case .slot:
                return
            }
            for k in rn.kids {
                walk(k, false)
            }
        }
        walk(root, true)
    }

    /// modelVals is the property of a classic diagram as Options.Classic gives it; a node or a property the
    /// model does not hold gives nothing, and the arguments fall back to the usual evaluation.
    func modelVals(_ rn: Scene.RNode, _ pr: Scene.Prop) -> [String]? {
        guard let classicVals, !usesCtx(pr) else { return nil }
        guard let at = classicAt[ObjectIdentifier(rn)] else { return nil }
        guard let vals = classicVals(at, pr.name), vals.count == pr.args.count else { return nil }
        return vals
    }
}
