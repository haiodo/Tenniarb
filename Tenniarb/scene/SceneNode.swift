//
//  SceneNode.swift
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

/// Namespace of the scene engine port: the types inside mirror go/scene one to one.
/// Built straight from Tenn nodes: an adapter from the app model replaces the parser input once it carries the scene format.
enum Scene {}

extension Scene {
    /// The kind of a node of a file, before any use is expanded.
    enum Kind: UInt8 {
        case file  // root of a file: acts as an unnamed element
        case element
        case item
        case link
        case use
        case slot  // slot definition in a component; content is the fallback
    }

    /// A source position, 1-based; file is a slash path relative to the workspace root.
    struct Pos: Equatable {
        var file = ""
        var line = 1
        var col = 1
    }

    /// The byte range [start, end) of a command in the text of its file.
    struct Range: Equatable {
        var start = 0
        var end = 0
    }

    /// A command that is not structure: a property of its node, in file order.
    final class Prop {
        let name: String
        let args: [TennNode]  // arguments after the name, a block excluded
        var val: [String]?  // the evaluated text of every argument, when the property holds an expression
        let block: TennNode?
        let pos: Pos
        let range: Range

        init(name: String, args: [TennNode], val: [String]? = nil, block: TennNode? = nil, pos: Pos, range: Range) {
            self.name = name
            self.args = args
            self.val = val
            self.block = block
            self.pos = pos
            self.range = range
        }

        func text(_ i: Int) -> String? {
            if let val {
                return i < val.count ? val[i] : nil
            }
            guard i < args.count else { return nil }
            return args[i].getIdentText()
        }

        func num(_ i: Int) -> Double? {
            if let val {
                return i < val.count ? Double(val[i]) : nil
            }
            guard i < args.count, args[i].kind == .IntLit || args[i].kind == .FloatLit else {
                return nil
            }
            return Double(args[i].getIdentText() ?? "")
        }
    }

    /// One parameter of a component: "export let Name = value", or "export let Name" when the use must set it.
    struct Param {
        let name: String
        var defaultValue: Prop?
        let pos: Pos
    }

    /// One named style: properties of the node that names it, and of the kids its blocks name.
    struct Style {
        var props: [Prop] = []
        var kids: [StyleKid] = []
    }

    /// The part of a style for a kid with that key, as a CSS descendant selector.
    struct StyleKid {
        var key: String
        var style: Style
    }

    /// One styles block read: the styles it declares in file order and the groups among them.
    struct StyleBlock {
        var names: [String] = []
        var styles: [String: Style] = [:]
        var groups: [StyleGroup] = []
    }

    /// One group declaration, "name one-of a b c" or "name any-of a b".
    struct StyleGroup {
        var name: String
        var anyOf: Bool
        var styles: [String]
    }

    /// The kind of one command in the block of a use.
    enum OvKind: UInt8 {
        case prop  // override of a property of the root
        case node  // item, link or use: goes to the default slot
        case at  // at <path> { ... }
        case replace  // replace <path> { nodes }
        case slot  // slot <name> { nodes }
    }

    /// One command of the block of a use.
    struct Override {
        var kind: OvKind = .prop
        var pos = Pos()
        var range = Range()
        var body = Range()
        var prop: Prop?
        var node: Node?
        var path = ""
        var slot = ""
        var block: [Override] = []
    }

    /// A node of the file tree: what was written, before use is expanded.
    final class Node {
        let kind: Kind
        var name = ""
        var key = ""
        var pos = Pos()
        var range = Range()  // the whole command with its block; zero for a file root
        var body = Range()  // the block of the command from { to after }; zero when it has none
        weak var parent: Node?  // scope: lookups go up this chain, and on from a file root to its mount point

        var props: [Prop] = []
        var content: [Node] = []  // item, link, use, slot in file order
        var elems: [Node] = []  // nested elements

        var mount: Node?  // element with --from-file: the root of the file that holds its content
        var from = ""
        var to = ""
        var path = ""  // use
        var block: [Override] = []  // use

        /// The named styles of an element from its styles block.
        var styles: [String: Style]?
        /// The groups the same block declares, in declaration order.
        var styleGroups: [StyleGroup] = []
        /// The kind the command "kind" gives an element or a file; "" when none is written, which is a screen.
        var type = ""
        /// The parameters the element declares with "export let name = value".
        var params: [Param] = []

        init(kind: Kind, name: String = "", pos: Pos = Pos(), range: Range = Range(), body: Range = Range(), parent: Node? = nil) {
            self.kind = kind
            self.name = name
            self.key = normalizeName(name)
            self.pos = pos
            self.range = range
            self.body = body
            self.parent = parent
        }

        /// The property in effect: the last one written, as go/scene expand.go:44.
        func prop(_ name: String) -> Prop? {
            for pr in props.reversed() where pr.name == name {
                return pr
            }
            return nil
        }

        func param(_ name: String) -> Param? {
            params.first { $0.name == name }
        }

        func holdsContent() -> Bool {
            kind == .file || kind == .element || kind == .item || kind == .slot
        }

        func isContainer() -> Bool {
            kind == .file || kind == .element
        }
    }
}

/// The kind words of an element or a file, as "kind Component" writes them; stored lowercase.
enum SceneType {
    static let component = "component"
    static let screen = "screen"
    static let library = "library"
    static let application = "application"
}

/// NormalizeName of go/scene/names.go: a character that is not a letter, mark or number becomes "_".
func normalizeName(_ name: String) -> String {
    var out = ""
    for scalar in name.unicodeScalars {
        switch scalar.properties.generalCategory {
        case .uppercaseLetter, .lowercaseLetter, .titlecaseLetter, .modifierLetter, .otherLetter,
            .nonspacingMark, .spacingMark, .enclosingMark,
            .decimalNumber, .letterNumber, .otherNumber:
            out.unicodeScalars.append(scalar)
        default:
            out.append("_")
        }
    }
    return out
}
