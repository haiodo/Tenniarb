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
import Tenn

/// The text of an argument: as written, or the value of an expression; a string holding ${...} is a template,
/// run as one. ok is false when an expression failed, or when there is no evaluator.
func evalArg(_ eval: Scene.EvalFunc?, _ a: TennNode, _ vars: [String: String]?) -> (String, Bool) {
    var code = a.getIdentText() ?? ""
    switch a.kind {
    case .Expression, .ExpressionBlock:
        break
    case .StringLit where code.contains("${"):
        code = "`" + code + "`"  // a template string, as the classic engine runs it
    default:
        return (code, true)
    }
    guard let eval else { return ("", false) }
    return evalValue(eval, code, vars)
}

/// eval with the rule of spec 5f: undefined and NaN are not a value, so the caller reports the expression
/// instead of drawing their text in the scene.
func evalValue(_ eval: Scene.EvalFunc, _ code: String, _ vars: [String: String]?) -> (String, Bool) {
    let (v, ok) = eval(code, vars)
    if ok && (v == "undefined" || v == "NaN") {
        return ("", false)
    }
    return (v, ok)
}

/// The arguments of a property as words, each an expression evaluated in vars; ok is false when an expression
/// could not be evaluated, and the words before it still come back.
func evalWords(_ eval: Scene.EvalFunc?, _ pr: Scene.Prop, _ vars: [String: String]?) -> (String, Bool) {
    var words: [String] = []
    for a in pr.args {
        let (v, ok) = evalArg(eval, a, vars)
        if !ok {
            return (words.joined(separator: " "), false)
        }
        words.append(v)
    }
    return (words.joined(separator: " "), true)
}

extension Scene.Expander {
    /// The parameters of instance target in use: a value the use sets by name over the default from the
    /// declaration (spec 5c). caller is the scope of the use, out of which the values it sets are read. used
    /// holds the overrides that are parameter values, so they do not become properties of the instance root. A
    /// parameter without a default that the use does not set is an error.
    func params(_ target: Scene.Node, _ use: Scene.Node, _ caller: [String: String]?) -> (
        values: [String: String]?, used: Set<Int>
    ) {
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
                values[prm.name] = value(pr, caller)
            } else if let def = prm.defaultValue {
                values[prm.name] = value(def, nil)
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
                values[prm.name] = value(def, nil)
                continue
            }
            values[prm.name] = ""  // a required parameter has no value on its own, and no warning either
        }
        return values
    }

    /// The value of one parameter: its arguments as words, each an expression evaluated in vars. A list of styles
    /// is written as several words: "extras outlined rounded" (spec 5b, any-of).
    func value(_ pr: Scene.Prop, _ vars: [String: String]?) -> String {
        let (v, ok) = evalWords(eval, pr, vars)
        if !ok && eval != nil {
            t.warn(pr.pos, "\(pr.name): the expression is not evaluated")
        }
        return v
    }
}

private func quoteAll(_ names: [String]) -> String {
    names.map { "\"\($0)\"" }.joined(separator: ", ")
}
