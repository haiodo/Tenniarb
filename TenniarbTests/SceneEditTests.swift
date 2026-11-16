//
//  SceneEditTests.swift
//  TenniarbTests
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
//  License for the specific language governing permissions and limitations under
//  the License.
//

import Foundation
import XCTest

@testable import Tenniarb

/// The edit producers of go/app/sceneedit.go against the strings its tests pin, and the document side
/// that applies one: the text is what an undo restores and what a save writes.
@MainActor
final class SceneEditTests: SceneDocumentTestCase {

    private let repo = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent()

    // MARK: - Workspace

    /// A scratch workspace with one file, so a test never reads or writes web/fixtures in place.
    private func load(_ text: String, _ name: String = "a.tenn") throws -> Scene.Tree {
        let root = try workspace([name: text])
        return try SceneLoader(root: root).load(name)
    }

    private func workspace(_ files: [String: String] = [:]) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-edit-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        for (path, text) in files {
            let url = root.appendingPathComponent(path)
            try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try text.write(to: url, atomically: true, encoding: .utf8)
        }
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    private func copy(_ fixture: String, to root: URL) throws -> URL {
        let url = root.appendingPathComponent((fixture as NSString).lastPathComponent)
        try FileManager.default.copyItem(at: repo.appendingPathComponent(fixture), to: url)
        return url
    }

    /// NodeAt of the Go tests: a key path from the file root, items, uses and nested elements by key.
    private func node(_ tree: Scene.Tree, _ path: String) throws -> Scene.Node {
        var n = try XCTUnwrap(tree.main, "no main file")
        for segment in path.components(separatedBy: ".") {
            let next = n.content.first { $0.key == segment } ?? n.elems.first { $0.key == segment }
            n = try XCTUnwrap(next, "no \(segment) in \(path)")
        }
        return n
    }

    /// apply of the Go tests: the text with edit spliced in, byte offsets as Scene.Range.
    private func apply(_ text: String, _ e: Scene.TextEdit) -> String {
        var bytes = Array(text.utf8)
        bytes.replaceSubrange(e.start..<e.end, with: Array(e.new.utf8))
        return String(decoding: bytes, as: UTF8.self)
    }

    // MARK: - Edits

    /// What penimport writes: tabs, and plain commands packed on one line.
    private let tabDoc = "item Page {\n\tcolor white; gap 1\n\titem Row {\n\t\tgap 1\n\t\titem A { title a }\n\t}\n\titem Empty\n}\n"

    func testEditsCopyTabIndent() throws {
        let tree = try load(tabDoc)
        let page = try node(tree, "Page")
        let row = try node(tree, "Page.Row")
        let empty = try node(tree, "Page.Empty")
        let cases: [(String, Scene.TextEdit, String)] = [
            (
                "property into a block", insertEdit(tabDoc, row, "color red"),
                tabDoc.replacingOccurrences(of: "\t\titem A { title a }\n", with: "\t\titem A { title a }\n\t\tcolor red\n")
            ),
            (
                "several lines into a block", insertEdit(tabDoc, row, "item B {\n\ttitle b\n}"),
                tabDoc.replacingOccurrences(
                    of: "\t\titem A { title a }\n", with: "\t\titem A { title a }\n\t\titem B {\n\t\t\ttitle b\n\t\t}\n")
            ),
            (
                "several lines into a node without block", insertEdit(tabDoc, empty, "item B {\n\ttitle b\n}"),
                tabDoc.replacingOccurrences(of: "\titem Empty\n", with: "\titem Empty {\n\t\titem B {\n\t\t\ttitle b\n\t\t}\n\t}\n")
            ),
            (
                "property into a packed block", insertEdit(tabDoc, page, "width 9"),
                tabDoc.replacingOccurrences(of: "\titem Empty\n", with: "\titem Empty\n\twidth 9\n")
            ),
        ]
        for (name, e, want) in cases {
            XCTAssertEqual(apply(tabDoc, e), want, name)
        }
        XCTAssertEqual(indentUnit(tabDoc), "\t")
        // a command packed after another one goes with its ;, a line of its own with the line
        let packed = try XCTUnwrap(setPropEdit(tabDoc, page, "gap"))
        XCTAssertEqual(apply(tabDoc, packed), tabDoc.replacingOccurrences(of: "; gap 1", with: ""))
        let line = try XCTUnwrap(setPropEdit(tabDoc, row, "gap"))
        XCTAssertEqual(apply(tabDoc, line), tabDoc.replacingOccurrences(of: "\t\tgap 1\n", with: ""))
    }

    /// The code section of a node loses the indentation of its first line; the replace and merge halves
    /// of the Go test need ReplaceNodeEdit and the stubs, which are later slices.
    func testNodeTextKeepsTabIndent() throws {
        let tree = try load(tabDoc)
        let row = try node(tree, "Page.Row")
        XCTAssertEqual(nodeText(tabDoc, row), "item Row {\n\tgap 1\n\titem A { title a }\n}")
    }

    /// TestSceneTextEdits of go/app/scenedoc_test.go: quoting, packed lines, deletion and insertion.
    func testSceneTextEdits() throws {
        let text = "item Card {\n  auto-layout column\n  item Title { title Hi; color red }\n  item Empty {}\n  item Bare\n}\n"
        let tree = try load(text)
        let cases: [(String, String, (Scene.Node) -> Scene.TextEdit?, String)] = [
            (
                "Card.Title", "title", { setPropEdit(text, $0, "title", "Sign in") },
                #"item Title { title "Sign in"; color red }"#
            ),
            ("Card.Title", "gap", { setPropEdit(text, $0, "gap", "8") }, "item Title { title Hi; color red; gap 8 }"),
            ("Card.Title", "color", { setPropEdit(text, $0, "color") }, "item Title { title Hi }"),
            ("Card.Title", "title removed", { setPropEdit(text, $0, "title") }, "item Title { color red }"),
            ("Card.Empty", "width", { setPropEdit(text, $0, "width", "fill") }, "item Empty { width fill }"),
            ("Card.Bare", "color", { setPropEdit(text, $0, "color", "#ffffff") }, "item Bare { color #ffffff }"),
            (
                "Card", "gap", { setPropEdit(text, $0, "gap", "4") },
                "  auto-layout column\n  item Title { title Hi; color red }\n  item Empty {}\n  item Bare\n  gap 4\n}"
            ),
            ("Card", "auto-layout", { setPropEdit(text, $0, "auto-layout") }, "item Card {\n  item Title"),
        ]
        for (path, name, edit, want) in cases {
            let e = try XCTUnwrap(edit(try node(tree, path)), "\(path) \(name)")
            let got = apply(text, e)
            XCTAssertTrue(got.contains(want), "\(path) \(name):\n\(got)")
            XCTAssertEqual(try load(got).fatalCount, 0, "\(path) \(name): the edit does not parse")
        }
        let empty = try node(tree, "Card.Empty")
        let deleted = apply(text, deleteEdit(text, empty))
        XCTAssertFalse(deleted.contains("Empty"))
        XCTAssertTrue(deleted.contains("}\n  item Bare"))
        let bare = try node(tree, "Card.Bare")
        let inserted = apply(text, insertEdit(text, bare, "use kit.Button as ok"))
        XCTAssertTrue(inserted.contains("item Bare { use kit.Button as ok }"), inserted)
        XCTAssertEqual(commandText("use", "ui.Button", "as", "Primary button"), #"use ui.Button as "Primary button""#)
    }

    /// TestParamEdit of go/app/params_test.go: a value goes in as its declaration writes it, in place of
    /// the command already there, and a removed value gives the default back.
    private let paramEditDoc = """
        element "Button" kind Component {
          export let title = "Button"
          export let kind = primary
          export let n = 40
          styles {
            primary { color red }
            secondary { color blue }
            kind one-of primary secondary
          }
          item Btn { title ${title} }
        }
        element "Screen" { use Button as save }

        """

    func testParamEdit() throws {
        let tree = try load(paramEditDoc)
        let button = try XCTUnwrap(tree.main?.elems.first)
        let use = try node(tree, "Screen.save")
        let cases: [(String, String, String)] = [
            ("title", "Save", #"use Button as save { title "Save" }"#),
            ("title", "Two words", #"use Button as save { title "Two words" }"#),
            ("kind", "secondary", "use Button as save { kind secondary }"),
            ("n", "48", "use Button as save { n 48 }"),
        ]
        for (name, value, want) in cases {
            let prm = try XCTUnwrap(button.param(name))
            let e = paramEdit(paramEditDoc, use, prm, value)
            XCTAssertTrue(apply(paramEditDoc, e).contains(want), "\(name) \(value)")
        }
        // the command already in the block is replaced, not written twice; no value removes it
        let withKind = #"use Button as save { kind secondary }"#
        let text = paramEditDoc.replacingOccurrences(of: "use Button as save", with: withKind)
        let loaded = try load(text)
        let kindButton = try XCTUnwrap(loaded.main?.elems.first)
        let kindUse = try node(loaded, "Screen.save")
        let prm = try XCTUnwrap(kindButton.param("kind"))
        let replaced = apply(text, paramEdit(text, kindUse, prm, "primary"))
        XCTAssertTrue(replaced.contains("use Button as save { kind primary }"), replaced)
        let removed = apply(text, try XCTUnwrap(removeParamEdit(text, kindUse, prm)))
        XCTAssertTrue(removed.contains("use Button as save"))
        XCTAssertFalse(removed.contains("kind secondary"))
    }

    /// penimport shares a repeated subtree as a template; the copies are uses of it. A part of a copy is
    /// edited by an at override in that use, which stays one command as it changes.
    private let sharedDoc =
        "element tpl {\n\titem Card {\n\t\tauto-layout column; width 80\n\t\titem Title { title Hi; font-size 14 }\n\t\titem Row { item Cell }\n\t}\n}\n"
        + "item Page {\n\tauto-layout row\n\tuse tpl.Card as one\n\tuse tpl.Card as two { pos 5 -5 }\n}\n"

    func testAtPropEditStaysInItsUse() throws {
        func use(_ text: String) throws -> Scene.Node { try node(try load(text), "Page.two") }

        var text = sharedDoc
        let first = try XCTUnwrap(atPropEdit(text, try use(text), "Title", "title", "Bye"))
        text = apply(text, first)
        XCTAssertEqual(
            text, sharedDoc.replacingOccurrences(of: "pos 5 -5 }", with: "pos 5 -5; at Title { title Bye } }"))

        // the same part again: its at is updated in place, and a second property goes into it
        let again = try XCTUnwrap(atPropEdit(text, try use(text), "Title", "title", "Again"))
        text = apply(text, again)
        let size = try XCTUnwrap(atPropEdit(text, try use(text), "Title", "font-size", "20"))
        text = apply(text, size)
        XCTAssertTrue(text.contains("at Title { title Again; font-size 20 }"), text)
        XCTAssertEqual(text.components(separatedBy: "at Title").count - 1, 1, text)
        // the template and the other copy are as they were
        XCTAssertTrue(text.hasPrefix(sharedDoc.components(separatedBy: "use tpl.Card as two")[0]))

        // removing the last property of a part leaves the at, and the part shows the component's value
        let removed = try XCTUnwrap(atPropEdit(text, try use(text), "Title", "font-size"))
        text = apply(text, removed)
        XCTAssertFalse(text.contains("font-size 20"), text)
        XCTAssertTrue(text.contains("at Title { title Again }"), text)
        XCTAssertEqual(try load(text).fatalCount, 0)
    }

    func testNumberText() {
        XCTAssertEqual(numText(1.23456), "1.23")
        XCTAssertEqual(numText(2), "2")
        XCTAssertEqual(numText(-0.005), "-0.01")
        XCTAssertTrue(isNumber("0.01"))
        XCTAssertFalse(isNumber("007"))
    }

    // MARK: - The document applies one

    func testSceneEditAppliesUndoAndSave() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/scene-text.tenn", to: root)
        let document = try openTestDocument(url)

        let text = try XCTUnwrap(document.sceneText)
        let tree = try XCTUnwrap(document.sceneTree)
        let leaf = try node(tree, "Rules.Stage.LeafTitle")
        let edit = try XCTUnwrap(setPropEdit(text, leaf, "title", "Edited"))
        let written = try XCTUnwrap(text.range(of: #"title "Titled""#))
        let start = text.utf8.distance(from: text.startIndex, to: written.lowerBound)
        XCTAssertEqual(edit.start, start)
        XCTAssertEqual(edit.end, start + #"title "Titled""#.utf8.count)
        XCTAssertEqual(edit.new, "title Edited")

        try document.applySceneEdit(edit, in: leaf.pos.file)
        let want = text.replacingOccurrences(of: #"title "Titled""#, with: "title Edited")
        XCTAssertEqual(document.sceneText, want)
        XCTAssertTrue(document.isDocumentEdited, "an applied edit marks the document")
        let next = try XCTUnwrap(document.sceneTree)
        XCTAssertEqual(next.fatalCount, 0, "\(next.diagnostics)")
        XCTAssertEqual(try node(next, "Rules.Stage.LeafTitle").prop("title")?.text(0), "Edited")

        let undo = try XCTUnwrap(document.undoManager)
        undo.undo()
        XCTAssertEqual(document.sceneText, text)
        XCTAssertFalse(document.isDocumentEdited, "an undo back to the saved text clears the mark")
        undo.redo()
        XCTAssertEqual(document.sceneText, want)

        try document.write(to: url, ofType: "tenn")
        XCTAssertEqual(try String(contentsOf: url, encoding: .utf8), want)
        XCTAssertFalse(document.isDocumentEdited)
    }

    func testAnEditOfAMountedFileIsRefused() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/library-screen.tenn", to: root)
        let library = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let libraryBefore = try Data(contentsOf: library)
        let document = try openTestDocument(url)

        let text = try XCTUnwrap(document.sceneText)
        let tree = try XCTUnwrap(document.sceneTree)
        let libraryText = try XCTUnwrap(tree.source("design-kit.tenn"))
        let button = try XCTUnwrap(tree.file("design-kit.tenn")?.elems.first)
        let edit = try XCTUnwrap(setPropEdit(libraryText, button, "width", "10"))

        XCTAssertThrowsError(try document.applySceneEdit(edit, in: button.pos.file)) { error in
            XCTAssertTrue(error.localizedDescription.contains("mounted"), error.localizedDescription)
        }
        // the content of the element that mounts the library is written in that file too
        let mount = try XCTUnwrap(tree.main?.elems.first)
        let content = try XCTUnwrap(mount.mount?.pos.file)
        XCTAssertThrowsError(try document.applySceneEdit(edit, in: content)) { error in
            XCTAssertTrue(error.localizedDescription.contains("mounted"), error.localizedDescription)
        }
        XCTAssertEqual(document.sceneText, text)
        XCTAssertTrue(document.sceneTree === tree)
        XCTAssertFalse(document.isDocumentEdited)
        XCTAssertEqual(try Data(contentsOf: library), libraryBefore)
    }

    func testAnEditThatBreaksTheFileIsRejected() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/scene-text.tenn", to: root)
        let document = try openTestDocument(url)

        let text = try XCTUnwrap(document.sceneText)
        let tree = try XCTUnwrap(document.sceneTree)
        let leaf = try node(tree, "Rules.Stage.LeafTitle")
        let broken = Scene.TextEdit(start: leaf.range.start, end: leaf.range.start, new: "{")

        XCTAssertThrowsError(try document.applySceneEdit(broken, in: leaf.pos.file)) { error in
            XCTAssertTrue(error.localizedDescription.hasPrefix("not applied:"), error.localizedDescription)
        }
        // a text that parses but loads with a scene error is refused the same way
        let missing = Scene.TextEdit(
            start: 0, end: 0, new: "element \"Lost\" --from-file \"missing.tenn\"\n")
        XCTAssertThrowsError(try document.applySceneEdit(missing, in: leaf.pos.file)) { error in
            XCTAssertTrue(error.localizedDescription.hasPrefix("not applied:"), error.localizedDescription)
        }
        XCTAssertEqual(document.sceneText, text)
        XCTAssertTrue(document.sceneTree === tree)
        XCTAssertFalse(document.isDocumentEdited)
        XCTAssertEqual(try String(contentsOf: url, encoding: .utf8), text)
    }
}
