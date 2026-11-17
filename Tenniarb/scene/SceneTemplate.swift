//
//  SceneTemplate.swift
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
    /// The code a component element generates (spec 5c): text where ${name} is the value of the parameter
    /// name and <slot/> or <slot name/> is the content a use passes.
    struct Template {
        var text: String  // as written between the braces, the indentation of the component cut
        var pos = Pos()  // the template command
    }

    /// The errors of a render, in the order the template pass found them. They are not in the tree
    /// diagnostics: rendering is a read of the file, not a load of it.
    struct TemplateError: Error, CustomStringConvertible {
        var errors: [Diagnostic]

        var description: String {
            errors.map { "\($0.pos): \($0.message)" }.joined(separator: "\n")
        }
    }
}

/// Stops a component that prints itself through the slot of its own use.
private let maxTemplateDepth = 64

extension Scene.Tree {
    /// The text the template of node generates (spec 5c), a component element or a use of one: the values and
    /// slots come from the block of the use, values override both, and without eval a ${...} is an error.
    func render(_ node: Scene.Node, values: [String: String] = [:], eval: Scene.EvalFunc? = nil) throws -> String {
        var errs: [Scene.Diagnostic] = []
        let text = renderUse(node, values, eval, 0, false, &errs)
        if !errs.isEmpty {
            throw Scene.TemplateError(errors: errs)
        }
        return text
    }

    /// The code an element generates: the template of every use below it in file order, nested in the slots,
    /// with tpl in place of the default slot; a plain item prints the code of its content where it stands.
    func renderElement(
        _ el: Scene.Node, template tpl: Scene.Template? = nil, values: [String: String] = [:],
        eval: Scene.EvalFunc? = nil
    ) throws -> String {
        var errs: [Scene.Diagnostic] = []
        let body = itemCode(el, eval, 0, &errs)
        if body.isEmpty && el.template != nil {
            // No use of its own: the element prints its template with the composition its slots fall back to.
            return try render(el, values: values, eval: eval)
        }
        guard let tpl = tpl ?? el.template else {
            if !errs.isEmpty {
                throw Scene.TemplateError(errors: errs)
            }
            return body
        }
        let vars = renderVars(el, nil, values, eval, &errs)
        let text = renderText(tpl, vars, ["": body], eval, &errs)
        if !errs.isEmpty {
            throw Scene.TemplateError(errors: errs)
        }
        return text
    }

    /// render for a node that is an element or a use of one, once the evaluator is known.
    private func renderUse(
        _ node: Scene.Node, _ values: [String: String], _ eval: Scene.EvalFunc?, _ depth: Int, _ code: Bool,
        _ errs: inout [Scene.Diagnostic]
    ) -> String {
        var target = node
        var use: Scene.Node?
        if node.kind == .use {
            use = node
            guard let parent = node.parent else { return "" }  // the loader gives every use a scope
            do {
                target = try resolve(parent, node.path)
            } catch {
                errs.append(Scene.Diagnostic(pos: node.pos, message: "\(error)", warn: false))
                return ""
            }
        }
        return renderAt(target, use, values, eval, depth, code, &errs)
    }

    /// The text of the template of target, with the values and the slots the use gives.
    private func renderAt(
        _ target: Scene.Node, _ use: Scene.Node?, _ values: [String: String], _ eval: Scene.EvalFunc?,
        _ depth: Int, _ code: Bool, _ errs: inout [Scene.Diagnostic]
    ) -> String {
        guard let tpl = target.template else {
            errs.append(
                Scene.Diagnostic(
                    pos: target.pos, message: "the \(target.kind.description) \"\(target.name)\" has no template",
                    warn: false))
            return ""
        }
        if depth > maxTemplateDepth {
            errs.append(
                Scene.Diagnostic(
                    pos: target.pos, message: "the templates nest too deep: a use inside the slot of itself?",
                    warn: false))
            return ""
        }
        let vars = renderVars(target, use, values, eval, &errs)
        let slots = renderSlots(target, use, eval, depth, code, &errs)
        return renderText(tpl, vars, slots, eval, &errs)
    }

    /// The code of the uses below an item or an element, one per line: a use of a component with no template
    /// prints nothing, a plain item prints the code of its content where it stands.
    private func itemCode(
        _ n: Scene.Node, _ eval: Scene.EvalFunc?, _ depth: Int, _ errs: inout [Scene.Diagnostic]
    ) -> String {
        var parts: [String] = []
        for c in n.content {
            switch c.kind {
            case .use:
                guard let parent = c.parent else { continue }
                let target: Scene.Node
                do {
                    target = try resolve(parent, c.path)
                } catch {
                    errs.append(Scene.Diagnostic(pos: c.pos, message: "\(error)", warn: false))
                    continue
                }
                if target.template == nil {
                    continue
                }
                let s = renderAt(target, c, [:], eval, 1, true, &errs)
                if !s.isEmpty {
                    parts.append(s)
                }
            case .item:
                let s = itemCode(c, eval, depth + 1, &errs)
                if !s.isEmpty {
                    parts.append(s)
                }
            default:
                break
            }
        }
        return parts.joined(separator: "\n")
    }

    /// The slot content the block of use passes, by normalized name ("" for the default) in file order, and
    /// the element's own "slot { ... }" of that name (spec 4.7a) where empty.
    private func renderSlots(
        _ target: Scene.Node, _ use: Scene.Node?, _ eval: Scene.EvalFunc?, _ depth: Int, _ code: Bool,
        _ errs: inout [Scene.Diagnostic]
    ) -> [String: String] {
        var slots: [String: String] = [:]
        for ov in use?.block ?? [] {
            switch ov.kind {
            case .node:
                guard let n = ov.node else { continue }
                slots[""] = joinSlot(slots[""] ?? "", nodeText(n, eval, depth, code, &errs))
            case .slot:
                let name = normalizeName(ov.slot)
                if slots[name] == nil {
                    slots[name] = ""  // a slot filled with no node still counts as filled
                }
                for sub in ov.block where sub.kind == .node {
                    guard let n = sub.node else { continue }
                    slots[name] = joinSlot(slots[name] ?? "", nodeText(n, eval, depth, code, &errs))
                }
            default:
                break
            }
        }
        return fallbackSlots(target, slots, eval, depth, code, &errs)
    }

    /// Fills the slots the use leaves empty with the fallback the element writes for them: the
    /// "slot { ... }" of the item that holds the slots (spec 4.7a).
    private func fallbackSlots(
        _ target: Scene.Node?, _ slots: [String: String], _ eval: Scene.EvalFunc?, _ depth: Int, _ code: Bool,
        _ errs: inout [Scene.Diagnostic]
    ) -> [String: String] {
        var slots = slots
        if let target {
            walkSlots(target, &slots, eval, depth, code, &errs)
        }
        return slots
    }

    private func walkSlots(
        _ n: Scene.Node, _ slots: inout [String: String], _ eval: Scene.EvalFunc?, _ depth: Int, _ code: Bool,
        _ errs: inout [Scene.Diagnostic]
    ) {
        for c in n.content {
            guard c.kind == .slot else {
                walkSlots(c, &slots, eval, depth, code, &errs)
                continue
            }
            if slots[c.key] != nil {
                continue
            }
            var fill = ""
            for sub in c.content {
                fill = joinSlot(fill, nodeText(sub, eval, depth, code, &errs))
            }
            if !fill.isEmpty {  // an empty fallback leaves the slot unfilled, and that is an error
                slots[c.key] = fill
            }
        }
    }

    /// A node the use passes to a slot as code: its title, the template of the component it uses, the code of
    /// the items below a plain item when the whole element prints, else its name.
    private func nodeText(
        _ n: Scene.Node, _ eval: Scene.EvalFunc?, _ depth: Int, _ code: Bool,
        _ errs: inout [Scene.Diagnostic]
    ) -> String {
        if n.kind == .use, let parent = n.parent, let target = try? resolve(parent, n.path), target.template != nil {
            return renderAt(target, n, [:], eval, depth + 1, code, &errs)
        }
        var title: Scene.Prop?
        for pr in n.props where pr.name == "title" {
            title = pr
        }
        guard let title else {
            if code && n.kind == .item {
                return itemCode(n, eval, depth + 1, &errs)
            }
            return n.name
        }
        let (v, ok) = evalWords(eval, title, nil)
        if !ok {
            errs.append(
                Scene.Diagnostic(
                    pos: title.pos, message: "the title of the slot content is not evaluated", warn: false))
        }
        return v
    }
}

/// The value the block of a use gives the parameter name, the last one written; nil for a use without it.
private func useProp(_ use: Scene.Node?, _ name: String) -> Scene.Prop? {
    guard let use else { return nil }
    var found: Scene.Prop?
    for ov in use.block where ov.kind == .prop && ov.prop?.name == name {
        found = ov.prop
    }
    return found
}

/// The value of every parameter of target: values gives it, the block of the use writes it, or the default;
/// a parameter without one, and a name the element does not declare, are errors.
private func renderVars(
    _ target: Scene.Node, _ use: Scene.Node?, _ values: [String: String], _ eval: Scene.EvalFunc?,
    _ errs: inout [Scene.Diagnostic]
) -> [String: String] {
    var vars: [String: String] = [:]
    for prm in target.params {
        if let v = values[prm.name] {
            vars[prm.name] = v
            continue
        }
        guard let pr = useProp(use, prm.name) ?? prm.defaultValue else {
            errs.append(
                Scene.Diagnostic(
                    pos: target.pos,
                    message: "element \"\(target.name)\" needs \"\(prm.name)\": set it in the block of the use or with -set",
                    warn: false))
            continue
        }
        let (v, ok) = evalWords(eval, pr, nil)
        if !ok {
            errs.append(
                Scene.Diagnostic(
                    pos: pr.pos, message: "parameter \"\(prm.name)\": the expression is not evaluated", warn: false))
        }
        vars[prm.name] = v
    }
    for name in values.keys.filter({ target.param($0) == nil }).sorted() {
        var declared = "it declares no parameters"
        if !target.params.isEmpty {
            declared = "parameters: " + target.params.map { "\"\($0.name)\"" }.joined(separator: ", ")
        }
        errs.append(
            Scene.Diagnostic(pos: target.pos, message: "unknown parameter \"\(name)\" (\(declared))", warn: false))
    }
    return vars
}

/// One more node in a slot: one node per line.
private func joinSlot(_ slot: String, _ text: String) -> String {
    if slot.isEmpty || text.isEmpty {
        return slot + text
    }
    return slot + "\n" + text
}

/// The text of a template with ${name} replaced by its value and <slot/> by the content.
private func renderText(
    _ tpl: Scene.Template, _ vars: [String: String], _ slots: [String: String], _ eval: Scene.EvalFunc?,
    _ errs: inout [Scene.Diagnostic]
) -> String {
    let text = Array(tpl.text)
    var out = ""
    var i = 0
    while i < text.count {
        if text[i] == "$", i + 1 < text.count, text[i + 1] == "{" {
            if let j = text[(i + 2)...].firstIndex(of: "}") {
                let code = String(text[(i + 2)..<j])
                let (v, ok) = eval?(code, vars) ?? ("", false)
                if ok {
                    out += v
                } else {
                    errs.append(
                        Scene.Diagnostic(
                            pos: tpl.pos, message: "${\(code)}: the expression is not evaluated", warn: false))
                }
                i = j + 1
                continue
            }
        }
        if let (name, n) = slotTag(text, i) {
            if let s = slots[name] {
                out += s
            } else if name.isEmpty {
                errs.append(
                    Scene.Diagnostic(
                        pos: tpl.pos, message: "the default slot has no content: pass nodes in the block of the use",
                        warn: false))
            } else {
                errs.append(
                    Scene.Diagnostic(
                        pos: tpl.pos,
                        message: "slot \"\(name)\" has no content: pass nodes in the block of the use", warn: false))
            }
            i += n
            continue
        }
        out.append(text[i])
        i += 1
    }
    return out
}

/// Reads "<slot/>" or "<slot name/>" at at: the normalized slot name, "" for the default slot, and the
/// length of the tag. Any other form, such as <slot name="x"/>, stays text.
private func slotTag(_ s: [Character], _ at: Int) -> (name: String, length: Int)? {
    let tail = s[at...]
    guard tail.starts(with: "<slot"), let end = tail.firstIndex(of: ">") else { return nil }
    var inner = String(s[(at + "<slot".count)..<end])
    if inner.hasSuffix("/") {
        inner.removeLast()
    }
    let name = inner.trimmingCharacters(in: .whitespacesAndNewlines)
    for ch in name where !(ch.isASCII && (ch.isLetter || ch.isNumber || ch == "_" || ch == "-")) {
        return nil  // a self-closing slot of another form: the text stays
    }
    return (normalizeName(name), end - at + 1)
}
