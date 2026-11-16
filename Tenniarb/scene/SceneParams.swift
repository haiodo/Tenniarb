//
//  SceneParams.swift
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

extension Scene.Expander {
    /// The parameters of instance target in use: a value the use sets by name over the default from the
    /// declaration (spec 5c). used holds the overrides that are parameter values, so they do not become
    /// properties of the instance root. A parameter without a default that the use does not set is an error.
    func params(_ target: Scene.Node, _ use: Scene.Node) -> (values: [String: String]?, used: Set<Int>) {
        if target.params.isEmpty {
            return (nil, [])
        }
        var used: Set<Int> = []
        var given: [String: Scene.Prop] = [:]
        for (i, ov) in use.block.enumerated() where ov.kind == .prop {
            guard let p = ov.prop, target.param(p.name) != nil else { continue }
            given[p.name] = p
            used.insert(i)
        }
        var values: [String: String] = [:]
        var missing: [String] = []
        for prm in target.params {
            if let pr = given[prm.name] {
                values[prm.name] = value(pr)
            } else if let def = prm.defaultValue {
                values[prm.name] = value(def)
            } else {
                missing.append(prm.name)
            }
        }
        if !missing.isEmpty {
            t.fail(use.pos, "element \"\(target.name)\" needs \(quoteAll(missing)): set it in the block of the use")
        }
        return (values, used)
    }

    /// The parameter values of an element expanded on its own, not through a use: a component shown as a screen
    /// looks like an instance that sets nothing.
    func defaults(_ n: Scene.Node) -> [String: String]? {
        if n.params.isEmpty {
            return nil
        }
        var values: [String: String] = [:]
        for prm in n.params {
            if let def = prm.defaultValue {
                values[prm.name] = value(def)
                continue
            }
            values[prm.name] = ""  // a required parameter has no value on its own, and no warning either
        }
        return values
    }

    /// The value of one parameter: its arguments as words.
    /// shortcut: expressions are not evaluated, so a ${...} argument ends the value with the words before it.
    func value(_ pr: Scene.Prop) -> String {
        var words: [String] = []
        for a in pr.args {
            if a.kind == .Expression || a.kind == .ExpressionBlock {
                break
            }
            let code = a.getIdentText() ?? ""
            if a.kind == .StringLit && code.contains("${") {
                break
            }
            words.append(code)
        }
        return words.joined(separator: " ")
    }
}

private func quoteAll(_ names: [String]) -> String {
    names.map { "\"\($0)\"" }.joined(separator: ", ")
}
