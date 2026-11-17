//
//  PenTheme.swift
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

// A pen theme is a named axis of the file - "themes": {"direction": ["home", "sale"]} - and a node names
// the value of it for its subtree. That is the context of .tenn, so the theme becomes `context <key>
// <value>`, and a variable whose value the file writes per theme becomes an expression that reads it back.

extension PenImport {
    /// One `context <key> <value>` declaration.
    static func contextProp(_ key: String, _ value: String) -> PenProp {
        let arg = simpleIdent(value) ? wordArg(value) : strArg(value)
        return prop("context", wordArg(key), arg)
    }

    /// The theme a node declares for its subtree, as context commands.
    static func themeProps(_ m: PenMap) -> [PenProp] {
        guard let th = m["theme"] as? PenMap else { return [] }
        var ps: [PenProp] = []
        for k in sortedKeys(th) {
            let v = toStr(th[k])
            if !v.isEmpty {
                ps.append(contextProp(k, v))
            }
        }
        return ps
    }

    /// The files of the import graph of f, f first.
    static func importGraph(_ f: PenFile) -> [PenFile] {
        var out: [PenFile] = []
        var seen: Set<ObjectIdentifier> = []
        func visit(_ g: PenFile) {
            if !seen.insert(ObjectIdentifier(g)).inserted { return }
            out.append(g)
            for a in sortedKeys(g.imports) {
                if let lib = g.imports[a] { visit(lib) }
            }
        }
        visit(f)
        return out
    }
}

extension PenFile {
    /// The first value of every theme axis at the root of the file: a node that names no theme sees it,
    /// and a themed variable has a context to read wherever it is written. The axes of the imported
    /// libraries are here too: a variable declared there is written as an expression in this file.
    var rootContext: [PenProp] {
        var seen: Set<String> = []
        var ps: [PenProp] = []
        let files = PenImport.importGraph(self).sorted { $0.name < $1.name }
        for g in files {
            guard let th = g.raw["themes"] as? PenMap else { continue }
            for k in PenImport.sortedKeys(th) {
                let v = g.themeDefault(k)
                if !v.isEmpty && !seen.contains(k) {
                    seen.insert(k)
                    ps.append(PenImport.contextProp(k, v))
                }
            }
        }
        return ps
    }

    /// The first value of an axis: what a node that names no theme sees.
    func themeDefault(_ key: String) -> String {
        guard let th = raw["themes"] as? PenMap, let list = th[key] as? [Any], let first = list.first else {
            return ""
        }
        return PenImport.toStr(first)
    }
}

extension PenConvert {
    /// A color the file writes once per theme value, as an expression of the context:
    /// ${ctx.direction === "sale" ? "#a1b2c3" : "#c6f24e"}. The last branch is the first value of the axis,
    /// so a node outside any theme keeps the color the file would show there. nil when the values do not
    /// share one key of one axis, and the caller resolves the color for one theme instead.
    func themedColor(_ f: PenFile, _ v: PenMap) -> String? {
        guard let items = v["value"] as? [Any], items.count >= 2 else { return nil }
        var key = ""
        for it in items {
            let im = it as? PenMap
            guard let th = im?["theme"] as? PenMap, th.count == 1 else { return nil }
            for k in th.keys {
                if !key.isEmpty && k != key { return nil }
                key = k
            }
        }
        let def = f.themeDefault(key)
        if def.isEmpty { return nil }
        var all: [(val: String, col: String)] = []
        var fallback = ""
        for it in items {
            let im = it as? PenMap
            let th = im?["theme"] as? PenMap
            guard let col = color(f, PenImport.toStr(im?["value"]), "", false), !col.hasPrefix("${") else {
                return nil
            }
            if PenImport.toStr(th?[key]) == def { fallback = col }
            all.append((PenImport.toStr(th?[key]), col))
        }
        if fallback.isEmpty { return nil }
        var out = "${"
        var conds = 0
        for b in all where b.val != def && b.col != fallback {
            out += "ctx.\(key) === \"\(b.val)\" ? \"\(b.col)\" : "
            conds += 1
        }
        if conds == 0 { return nil }
        out += "\"\(fallback)\"}"
        return out
    }
}
