//
//  SceneRefsTests.swift
//  TenniarbTests
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//  WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//  License for the specific language governing permissions and limitations under
//  the License.
//

import XCTest

@testable import Tenniarb

/// The Swift side of go/scene/refs_test.go and of the planning half of go/app/rename.go.
final class SceneRefsTests: XCTestCase {

    // MARK: - Stand

    /// A workspace on disk; the loader reads mounts from it, as go/scene does.
    private func workspace(_ files: [String: String]) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-refs-\(UUID().uuidString)", isDirectory: true)
        for (path, text) in files {
            let url = root.appendingPathComponent(path)
            try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try text.write(to: url, atomically: true, encoding: .utf8)
        }
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    private func load(_ files: [String: String], _ main: String = "main.tenn") throws -> Scene.Tree {
        try SceneLoader(root: try workspace(files)).load(main)
    }

    private func resolve(_ tree: Scene.Tree, _ path: String, in file: String) throws -> Scene.Node {
        try tree.resolve(try XCTUnwrap(tree.file(file), "\(file) is loaded"), path)
    }

    /// The text of one file after the edits of a plan.
    private func applied(
        _ text: String, _ plan: Scene.RenamePlan, _ file: String = "main.tenn", update: Bool = true
    ) throws -> String {
        try applyEdits(text, plan.edits(update: update).filter { $0.file == file }.map(\.edit))
    }

    /// The refs as file:line and the argument as written, the way refs_test.go lists them.
    private func list(_ tree: Scene.Tree, _ refs: [Scene.Ref]) -> [String] {
        refs.map { r in
            let bytes = Array((tree.source(r.file) ?? "").utf8)
            let line = bytes.prefix(r.arg.start).filter { $0 == 0x0A }.count + 1
            return "\(r.file):\(line) \(String(decoding: bytes[r.arg.start..<r.arg.end], as: UTF8.self))"
        }
    }

    // MARK: - References

    /// A path counts when it goes through the element, short or full, inside the mounted file or outside it.
    /// The same name in another scope is left alone.
    func testReferencesToElement() throws {
        let tree = try load([
            "kit.tenn": "element Parts {\n  item Button { width 40 }\n}\nelement Demo {\n  use Parts.Button as a\n}\n",
            "main.tenn": "element kit --from-file kit.tenn\n" + "element Screen {\n  use kit.Parts.Button as b\n  use kit.Demo.a as c\n}\n"
                + "element Other {\n  item Parts { }\n  use Parts as p\n}\n",
        ])
        let before = tree.diagnostics.map(\.message)
        let parts = try resolve(tree, "kit.Parts", in: "main.tenn")
        let (refs, blocked) = tree.references(parts)
        XCTAssertEqual(list(tree, refs), ["kit.tenn:5 Parts.Button", "main.tenn:3 kit.Parts.Button"])
        XCTAssertNil(blocked)
        XCTAssertEqual(refs[0].seg, 0)
        XCTAssertEqual(refs[1].seg, 1)

        let other = try resolve(tree, "Other.Parts", in: "main.tenn")
        XCTAssertEqual(list(tree, tree.references(other).refs), ["main.tenn:8 Parts"])

        let alias = try resolve(tree, "kit", in: "main.tenn")
        XCTAssertEqual(
            list(tree, tree.references(alias).refs), ["main.tenn:3 kit.Parts.Button", "main.tenn:4 kit.Demo.a"])
        XCTAssertEqual(tree.diagnostics.map(\.message), before, "finding references changed the diagnostics")
    }

    /// A part opened on its own is loaded in the context of its entry: the references in the entry and in the
    /// other parts are found from there.
    func testReferencesThroughEntry() throws {
        let root = try workspace([
            "design.tenn":
                "element ui --from-file ui.tenn\nelement p --from-file \"apps/part.tenn\"\nelement q --from-file \"apps/other.tenn\"\n",
            "ui.tenn": "item Button { width 40 }\n",
            "apps/part.tenn": "element Screen { use ui.Button as b }\n",
            "apps/other.tenn": "element Screen2 { use ui.Button as b }\n",
        ])
        let tree = try SceneLoader(root: root).loadIn("apps/part.tenn", entry: "design.tenn")
        let btn = try resolve(tree, "ui.Button", in: "design.tenn")
        XCTAssertEqual(
            list(tree, tree.references(btn).refs), ["apps/other.tenn:1 ui.Button", "apps/part.tenn:1 ui.Button"])
    }

    private let componentText = """
        element S {
          item Card { item title { }; item body { item line { } } }
          use Card as c1 { at title { color red }; at body.line { color blue } }
          use Card as c2 { replace body.line { item other { } } }
          use c1 as c3 { at body { }; at title { } }
          item A { }
          item B { }
          link A B
          link "B" A
        }

        """

    /// at and replace are followed through the instances; a use is a component too.
    func testReferencesThroughInstances() throws {
        let tree = try load(["m.tenn": componentText], "m.tenn")
        let title = try resolve(tree, "S.Card.title", in: "m.tenn")
        XCTAssertEqual(list(tree, tree.references(title).refs), ["m.tenn:3 title", "m.tenn:5 title"])

        let line = try resolve(tree, "S.Card.body.line", in: "m.tenn")
        let refs = tree.references(line).refs
        XCTAssertEqual(list(tree, refs), ["m.tenn:3 body.line", "m.tenn:4 body.line"])
        XCTAssertEqual(refs[0].kind, .at)
        XCTAssertEqual(refs[1].kind, .replace)
        XCTAssertEqual(refs[0].seg, 1)

        let c1 = try resolve(tree, "S.c1", in: "m.tenn")
        XCTAssertEqual(list(tree, tree.references(c1).refs), ["m.tenn:5 c1"])

        let card = try resolve(tree, "S.Card", in: "m.tenn")
        XCTAssertEqual(list(tree, tree.references(card).refs), ["m.tenn:3 Card", "m.tenn:4 Card"])
    }

    /// A link end names a sibling by its name; with several of that name the rename is blocked.
    func testReferencesLinkEnds() throws {
        let tree = try load(
            [
                "m.tenn": componentText
                    + "element T {\n  item X { }\n  item X { }\n  link X X { target-index 1 }\n  item Y { }\n  link Y Y\n}\n"
            ], "m.tenn")
        let a = try resolve(tree, "S.A", in: "m.tenn")
        let (refs, blocked) = tree.references(a)
        XCTAssertEqual(list(tree, refs), ["m.tenn:8 A", "m.tenn:9 A"])
        XCTAssertNil(blocked)
        XCTAssertEqual(refs[0].kind, .link)
        XCTAssertEqual(refs[0].seg, -1)

        let b = try resolve(tree, "S.B", in: "m.tenn")
        XCTAssertEqual(list(tree, tree.references(b).refs), ["m.tenn:8 B", "m.tenn:9 \"B\""])

        let x = try XCTUnwrap(try resolve(tree, "T", in: "m.tenn").content.first)
        let block = tree.references(x).blocked
        XCTAssertTrue(block?.contains("several") ?? false, "an indexed link end must block the rename, got \(block ?? "nil")")

        let y = try resolve(tree, "T.Y", in: "m.tenn")
        let (both, none) = tree.references(y)
        XCTAssertEqual(both.count, 2, "both ends of the link Y Y")
        XCTAssertNil(none)
    }

    // MARK: - Rename plan

    /// The plan writes the name of the use and the path segment of every use that names the node, in the file
    /// the argument is written in.
    func testPlanRenamesTheUsePaths() throws {
        let text = """
            element Parts {
              item Button { }
            }
            element Screen {
              use Parts.Button as b
              item Box { use Parts.Button as c }
            }

            """
        let tree = try load(["main.tenn": text])
        let button = try resolve(tree, "Parts.Button", in: "main.tenn")
        let plan = try tree.planRename(button, "Icon")
        XCTAssertEqual(plan.name, "Icon")
        XCTAssertEqual(plan.total, 2)
        XCTAssertEqual(plan.refEdits.count, 2)
        let edits = plan.edits()
        XCTAssertEqual(edits.count, 3)
        XCTAssertEqual(
            try applied(text, plan),
            """
            element Parts {
              item Icon { }
            }
            element Screen {
              use Parts.Icon as b
              item Box { use Parts.Icon as c }
            }

            """)
    }

    /// A name with a space is written quoted where it is a name and as its key in a path.
    func testPlanQuotesTheNameAndKeysThePath() throws {
        let text = "element Parts {\n  item Button { }\n}\nuse Parts.Button as b\n"
        let tree = try load(["main.tenn": text])
        let plan = try tree.planRename(try resolve(tree, "Parts.Button", in: "main.tenn"), "My Button")
        XCTAssertEqual(
            try applied(text, plan),
            """
            element Parts {
              item "My Button" { }
            }
            use Parts.My_Button as b

            """)
    }

    /// The span of a name takes its quotes with it, also for a non-ASCII name the lexer's pos is short of.
    func testPlanReplacesQuotedAndNonASCIIRanges() throws {
        let text = "element Parts {\n  item \"Кнопка\" { }\n}\nuse Parts.\"Кнопка\" as b\n"
        let tree = try load(["main.tenn": text])
        let button = try resolve(tree, "Parts.\"Кнопка\"", in: "main.tenn")
        let plan = try tree.planRename(button, "Button")
        XCTAssertEqual(
            try applied(text, plan),
            "element Parts {\n  item Button { }\n}\nuse Parts.Button as b\n")
    }

    /// at and replace paths follow the node into the instances; a link end is written by its name.
    func testPlanRenamesAtReplaceAndLinkEnds() throws {
        let text = """
            element Card {
              item title { }
              item body { item line { } }
            }
            element S {
              use Card as c1 { at body.line { width 1 } }
              item A { }
              item B { }
              link A B
              link "B" A
            }

            """
        let tree = try load(["main.tenn": text])
        let line = try resolve(tree, "Card.body.line", in: "main.tenn")
        let at = try tree.planRename(line, "row")
        XCTAssertEqual(
            try applied(text, at),
            """
            element Card {
              item title { }
              item body { item row { } }
            }
            element S {
              use Card as c1 { at body.row { width 1 } }
              item A { }
              item B { }
              link A B
              link "B" A
            }

            """)

        let b = try resolve(tree, "S.B", in: "main.tenn")
        let link = try tree.planRename(b, "C")
        XCTAssertEqual(
            try applied(text, link),
            """
            element Card {
              item title { }
              item body { item line { } }
            }
            element S {
              use Card as c1 { at body.line { width 1 } }
              item A { }
              item C { }
              link A C
              link C A
            }

            """)
        // Without update the references keep the old name.
        let own = try applied(text, link, update: false)
        XCTAssertTrue(own.contains("item C { }"))
        XCTAssertTrue(own.contains("link A B"))
    }

    /// A plan of a node of a mounted file edits both files.
    func testPlanRenamesAcrossMountedFiles() throws {
        let kit = "element Parts {\n  item Button { }\n}\nelement Demo {\n  use Parts.Button as a\n}\n"
        let main = "element kit --from-file kit.tenn\nelement Screen {\n  use kit.Parts.Button as b\n}\n"
        let tree = try load(["kit.tenn": kit, "main.tenn": main])
        let plan = try tree.planRename(try resolve(tree, "kit.Parts.Button", in: "main.tenn"), "Icon")
        let edits = plan.edits()
        XCTAssertEqual(edits.map(\.file), ["kit.tenn", "kit.tenn", "main.tenn"])
        XCTAssertEqual(
            try applied(kit, plan, "kit.tenn"),
            "element Parts {\n  item Icon { }\n}\nelement Demo {\n  use Parts.Icon as a\n}\n")
        XCTAssertEqual(
            try applied(main, plan, "main.tenn"),
            "element kit --from-file kit.tenn\nelement Screen {\n  use kit.Parts.Icon as b\n}\n")
    }

    func testPlanRefusesAKeyTakenByASibling() throws {
        let tree = try load(["m.tenn": "element S {\n  item A { }\n  item B { }\n}\n"], "m.tenn")
        let a = try resolve(tree, "S.A", in: "m.tenn")
        XCTAssertThrowsError(try tree.planRename(a, "B")) { error in
            XCTAssertEqual((error as? Scene.RenameError)?.message, "There is something named \"B\" here already.")
        }
    }

    func testPlanRefusesAnEmptyNameAndAnUnnamedNode() throws {
        let tree = try load(["m.tenn": "element S {\n  item A { }\n  item { }\n}\n"], "m.tenn")
        let a = try resolve(tree, "S.A", in: "m.tenn")
        XCTAssertThrowsError(try tree.planRename(a, "  ")) { error in
            XCTAssertEqual((error as? Scene.RenameError)?.message, "The name cannot be empty.")
        }
        let unnamed = try XCTUnwrap(try resolve(tree, "S", in: "m.tenn").content.last)
        XCTAssertThrowsError(try tree.planRename(unnamed, "B")) { error in
            XCTAssertEqual((error as? Scene.RenameError)?.message, "An item without a name cannot be renamed here.")
        }
    }

    func testPlanRefusesALinkThatNamesSeveralSiblings() throws {
        let tree = try load(["m.tenn": "element T {\n  item X { }\n  item X { }\n  link X X\n}\n"], "m.tenn")
        let x = try XCTUnwrap(try resolve(tree, "T", in: "m.tenn").content.first)
        XCTAssertThrowsError(try tree.planRename(x, "Z")) { error in
            XCTAssertEqual(
                (error as? Scene.RenameError)?.message,
                "Not renamed: several items are named X and a link addresses them by index.")
        }
    }

    // MARK: - Edits

    /// A path segment is replaced by the key of the name; a link end by the name as the printer writes it.
    func testRefEditWritesTheSegmentAndQuotesTheName() throws {
        let segment = Scene.Ref(kind: .use, file: "m.tenn", arg: Scene.Range(start: 10, end: 20), text: "Parts.Button", seg: 1)
        let edit = try XCTUnwrap(refEdit(segment, "My Button"))
        XCTAssertEqual(edit.start, 10)
        XCTAssertEqual(edit.end, 20)
        XCTAssertEqual(edit.new, "Parts.My_Button")

        let link = Scene.Ref(kind: .link, file: "m.tenn", arg: Scene.Range(start: 3, end: 6), text: "B", seg: -1)
        XCTAssertEqual(refEdit(link, "My Button")?.new, "\"My Button\"")
        XCTAssertNil(refEdit(link, "B"), "the text already names the node")
    }

    func testRefEditLeavesAPathThatAlreadyNamesTheNode() throws {
        let r = Scene.Ref(kind: .use, file: "m.tenn", arg: Scene.Range(start: 0, end: 5), text: "some_name", seg: 0)
        XCTAssertNil(refEdit(r, "some name"))
        let bad = Scene.Ref(kind: .use, file: "m.tenn", arg: Scene.Range(start: 0, end: 5), text: "a..b", seg: 0)
        XCTAssertNil(refEdit(bad, "c"), "a path the format cannot read is left alone")
    }

    func testApplyEditsAppliesFromTheEndAndRefusesOverlap() throws {
        let text = "abc"
        let edits = [
            Scene.TextEdit(start: 0, end: 1, new: "A"),
            Scene.TextEdit(start: 2, end: 3, new: "C"),
        ]
        XCTAssertEqual(try applyEdits(text, edits), "AbC")
        XCTAssertEqual(try applyEdits(text, edits.reversed()), "AbC", "the order the edits come in does not matter")
        XCTAssertThrowsError(
            try applyEdits(text, [Scene.TextEdit(start: 0, end: 2, new: ""), Scene.TextEdit(start: 1, end: 3, new: "")])
        ) { error in
            XCTAssertEqual((error as? Scene.RenameError)?.message, "The references overlap.")
        }
        XCTAssertThrowsError(try applyEdits(text, [Scene.TextEdit(start: 0, end: 9, new: "")]))
    }
}
