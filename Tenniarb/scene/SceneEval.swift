//
//  SceneEval.swift
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
import JavaScriptCore

extension Scene {
    /// Options.EvalVars: value of an expression with the instance's parameters in scope (spec 5c); nil: no
    /// expression runs.
    typealias EvalFunc = (_ code: String, _ vars: [String: String]?) -> (String, Bool)
}

/// SceneEvalVars of go/exec: the value of a ${...} (spec 6, 5c) with the instance's parameters and ctx.* keys
/// (spec 5f) in scope, a variable a number only when it is written as one.
///
/// shortcut: no round limit, so a runaway expression hangs the build; add an interrupt when scenes take files
/// from outside the workspace.
func sceneEvalVars(_ code: String, _ vars: [String: String]?) -> (String, Bool) {
    // A fresh context per call, as SceneEvalVars does: one expression sees no binding of another.
    guard let j = JSContext() else { return ("", false) }
    var ctx: [String: Any] = [:]
    for (name, v) in vars ?? [:] {
        if name.hasPrefix("ctx.") {
            ctx[String(name.dropFirst("ctx.".count))] = sceneValue(v)
            continue
        }
        // A failed write (a read-only global such as `undefined`) changes nothing, as JSObjectSetProperty.
        j.setObject(sceneValue(v), forKeyedSubscript: name as NSString)
    }
    if !ctx.isEmpty, let object = JSValue(newObjectIn: j) {
        for (key, v) in ctx {
            object.setObject(v, forKeyedSubscript: key as NSString)
        }
        j.setObject(object, forKeyedSubscript: "ctx" as NSString)
    }
    var error: JSValue?
    j.exceptionHandler = { _, e in error = e }
    guard let v = j.evaluateScript(code), error == nil else { return ("", false) }
    return (jsToString(v), true)
}

/// sceneValue of go/exec: v as a JS value, a number when it is written as one, else the text.
func sceneValue(_ v: String) -> Any {
    // "007" or "1.50" stay the text they are, or they would come back changed as 7 and 1.5.
    if let f = Double(v), canonicalNumber(f) == v {
        return f
    }
    return v
}

/// FormatFloat(f, 'f', -1, 64) of Go: the shortest digits that read back as f, never an exponent.
private func canonicalNumber(_ f: Double) -> String {
    if f.isNaN { return "NaN" }
    if f.isInfinite { return f < 0 ? "-Inf" : "+Inf" }
    var text = f.description  // the shortest digits that read back as f
    var sign = ""
    if text.hasPrefix("-") {
        sign = "-"
        text.removeFirst()
    }
    var digits = text
    var point = 0
    if let e = text.firstIndex(where: { $0 == "e" || $0 == "E" }) {
        digits = String(text[..<e])
        point = Int(text[text.index(after: e)...]) ?? 0
    }
    let dot = digits.firstIndex(of: ".")
    let whole = dot.map { String(digits[..<$0]) } ?? digits
    let frac = dot.map { String(digits[digits.index(after: $0)...]) } ?? ""
    point += whole.count
    var all = whole + frac
    while all.count > point && all.hasSuffix("0") { all.removeLast() }
    if point <= 0 {
        return sign + "0." + String(repeating: "0", count: -point) + all
    }
    if point >= all.count {
        return sign + all + String(repeating: "0", count: point - all.count)
    }
    return sign + String(all.prefix(point)) + "." + String(all.dropFirst(point))
}

/// JSValue.toString(); a throwing toString gives "".
private func jsToString(_ v: JSValue) -> String {
    v.toString() ?? ""
}
