//
//  SceneContext.swift
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

extension Scene {
    /// One "context <key> <value>" or "context <key> ?= <value>" (spec 5f).
    struct CtxDecl {
        let key: String
        var val: String
        var soft: Bool  // ?=: only fills a key no layer above has
    }
}

/// isExprArg: whether the argument is an expression: an expression node, or a string with ${...}.
func isExprArg(_ a: TennNode) -> Bool {
    if a.kind == .Expression || a.kind == .ExpressionBlock {
        return true
    }
    if a.kind == .StringLit, let code = a.getIdentText() {
        return code.contains("${")
    }
    return false
}

/// applyCtx: env with the declarations applied; a soft one fills only an absent key.
func applyCtx(_ env: [String: String]?, _ decls: [Scene.CtxDecl]) -> [String: String]? {
    if decls.isEmpty {
        return env
    }
    var out = env ?? [:]
    for d in decls {
        if d.soft && out[d.key] != nil {
            continue
        }
        out[d.key] = d.val
    }
    return out
}

/// blockProps: the properties of a use block in file order; a node of the block is no declaration.
func blockProps(_ block: [Scene.Override]) -> [Scene.Prop] {
    block.compactMap { $0.kind == .prop ? $0.prop : nil }
}

/// ctxEnv: the context part of a scope, the keys with the ctx. prefix.
func ctxEnv(_ vars: [String: String]?) -> [String: String]? {
    var out: [String: String]?
    for (k, v) in vars ?? [:] {
        guard k.hasPrefix("ctx.") else { continue }
        if out == nil {
            out = [:]
        }
        out![String(k.dropFirst("ctx.".count))] = v
    }
    return out
}

/// fileRoot: the root of the file n is written in: the node above every element of it (spec 5f).
func fileRoot(_ n: Scene.Node) -> Scene.Node {
    var n = n
    while let p = n.parent {
        n = p
    }
    return n
}

/// usesCtx: whether pr names ctx in an expression. The element model of a classic diagram has no ctx, so such
/// a property is the scene's to evaluate (spec 5f, 6). Its only caller is modelVals of classic.go, not ported yet.
///
/// shortcut: the code is scanned as text, so a property that mentions ctx inside a string or mixes it with the
/// model scope (sum, items) is left unevaluated; give the model the context when such a file turns up.
func usesCtx(_ pr: Scene.Prop) -> Bool {
    for a in pr.args {
        let isString = a.kind == .StringLit
        guard a.kind == .Expression || a.kind == .ExpressionBlock || isString, let code = a.getIdentText() else {
            continue
        }
        if isString && !code.contains("${") {
            continue
        }
        if code.range(of: "(^|[^.\\w$])ctx\\b", options: .regularExpression) != nil {
            return true
        }
    }
    return false
}

extension Scene.Expander {
    /// scope: the variables of an expression: parameters by name and context keys with the ctx. prefix
    /// (spec 5c, 5f). Without context the map is the one of the parameters, unchanged.
    func scope() -> [String: String]? { scopeOf(vars, env) }

    func scopeOf(_ vars: [String: String]?, _ env: [String: String]?) -> [String: String]? {
        guard let env, !env.isEmpty else {
            return vars
        }
        var out = vars ?? [:]
        for (k, v) in env {
            out["ctx." + k] = v
        }
        return out
    }

    /// context: env with the context commands of a node's own properties applied, their values evaluated in
    /// the parameters and the inherited context.
    func context(_ n: Scene.Node, _ props: [Scene.Prop], _ env: [String: String]?) -> [String: String]? {
        let env = env ?? self.env
        return applyCtx(env, ctxDecls(n, props, scopeOf(vars, env)))
    }

    /// ctxDecls: reads the context commands of props in file order; the value is the rest of the arguments,
    /// expressions evaluated in scope. --from-item reads the properties of an item as context keys.
    func ctxDecls(_ n: Scene.Node, _ props: [Scene.Prop], _ scope: [String: String]?) -> [Scene.CtxDecl] {
        var out: [Scene.CtxDecl] = []
        for pr in props where pr.name == "context" {
            if let first = pr.args.first, (first.getIdentText() ?? "") == "--from-item" {
                guard pr.args.count >= 2 else {
                    t.warn(pr.pos, "context is written: context --from-item <path>")
                    continue
                }
                // The load-time validation has no evaluator: a path expression is the build's to resolve.
                if eval == nil && isExprArg(pr.args[1]) {
                    continue
                }
                let path = value(
                    Scene.Prop(name: "context", args: Array(pr.args[1...]), pos: pr.pos, range: Scene.Range()),
                    scope)
                let item: Scene.Node
                do {
                    item = try t.resolve(n, path)
                } catch {
                    t.fail(pr.pos, "context --from-item \"\(path)\": \(error)")
                    continue
                }
                guard item.kind == .item else {
                    t.fail(pr.pos, "context --from-item \"\(path)\" names a \(item.kind.description), not an item")
                    continue
                }
                for pp in item.props where pp.name != "context" && !pp.args.isEmpty {
                    out.append(Scene.CtxDecl(key: pp.name, val: value(pp, scope), soft: false))
                }
                continue
            }
            guard pr.args.count >= 2 else {
                t.warn(pr.pos, "context is written: context <key> [?=] <value>")
                continue
            }
            var d = Scene.CtxDecl(key: pr.args[0].getIdentText() ?? "", val: "", soft: false)
            var rest = Array(pr.args[1...])
            if (pr.args[1].getIdentText() ?? "") == "?=" {
                d.soft = true
                rest = Array(pr.args[2...])
            }
            guard !rest.isEmpty else {
                t.warn(pr.pos, "context \(d.key) needs a value")
                continue
            }
            d.val = value(Scene.Prop(name: "context", args: rest, pos: pr.pos, range: Scene.Range()), scope)
            out.append(d)
        }
        return out
    }

    /// fileDefaults: the `?=` context of the file a definition is written in, evaluated in the place of the
    /// use: a default travels with the component, a hard declaration binds its own file (spec 5f).
    func fileDefaults(_ target: Scene.Node, _ inherited: [String: String]?) -> [Scene.CtxDecl] {
        let file = fileRoot(target)
        if file === target {
            return []
        }
        return ctxDecls(file, file.props, scopeOf(nil, inherited)).filter(\.soft)
    }
}
