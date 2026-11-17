//
//  InspectorTests.swift
//  TenniarbTests
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

import XCTest

@testable import Tenniarb

/// The inspector of a scene: the rows of the selected node read its own commands, a field writes through
/// the document as one undo step, and a part of a component is edited in the use around it.
@MainActor
final class InspectorTests: SceneDocumentTestCase {

    private let repo = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent()

    // MARK: - Fixtures

    /// A scratch workspace: fixtures are copied, never opened in place.
    private func workspace() throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-inspector-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    private func copy(_ fixture: String, to root: URL) throws -> URL {
        let url = root.appendingPathComponent((fixture as NSString).lastPathComponent)
        try FileManager.default.copyItem(at: repo.appendingPathComponent(fixture), to: url)
        return url
    }

    /// A scene document on text written into a scratch workspace.
    private func document(_ text: String, in root: URL? = nil) throws -> Document {
        let dir: URL
        if let root {
            dir = root
        } else {
            dir = try workspace()
        }
        let url = dir.appendingPathComponent("a.tenn")
        try text.write(to: url, atomically: true, encoding: .utf8)
        return try openTestDocument(url)
    }

    private func canvas(_ document: Document) throws -> SceneCanvasView {
        let view = SceneCanvasView(frame: .zero)
        view.document = document
        view.show(document.sceneTree)
        let screen = try XCTUnwrap(view.screen)
        view.frame = NSRect(origin: .zero, size: SceneCanvasView.contentSize(screen))
        return view
    }

    private func inspector(_ document: Document, _ canvas: SceneCanvasView) -> InspectorController {
        let controller = InspectorController()
        controller.attach(canvas: canvas, document: document)
        return controller
    }

    /// The index path of the first drawn item below the screen root that matches.
    private func path(in canvas: SceneCanvasView, where match: (Scene.Placed) -> Bool) -> [Int]? {
        guard let root = canvas.placed else { return nil }
        func walk(_ pl: Scene.Placed, _ p: [Int]) -> [Int]? {
            for i in pl.kids.indices {
                if match(pl.kids[i]) { return p + [i] }
                if let found = walk(pl.kids[i], p + [i]) { return found }
            }
            return nil
        }
        return walk(root, [])
    }

    private func inside(_ pl: Scene.Placed, _ name: String) -> Bool {
        var p: Scene.Placed? = pl
        while let c = p {
            if c.r.name == name { return true }
            p = c.parent
        }
        return false
    }

    // MARK: - Rows

    func testTheRowsOfASelectedItemReadItsWrittenProps() throws {
        let root = try workspace()
        let document = try openTestDocument(copy("web/fixtures/scene/scene-text.tenn", to: root))
        let canvas = try canvas(document)
        let leaf = try XCTUnwrap(path(in: canvas) { $0.src?.key == "LeafTitle" })
        canvas.select(leaf)
        let inspector = inspector(document, canvas)
        inspector.createView().layoutSubtreeIfNeeded()

        XCTAssertEqual(inspector.title, "Item")
        let sections = Set(inspector.rows.map(\.section))
        XCTAssertTrue(
            sections.isSuperset(of: ["Header", "Layout", "Fill", "Text", "Border", "Shape", "Effects"]),
            "\(sections)")
        let labels = Set(inspector.rows.map(\.label))
        for label in [
            "Auto layout", "Gap", "Align", "Justify", "Clip", "Position X", "Position Y", "Size W", "Size H",
            "Padding", "Color", "Title", "Font", "Weight", "Size", "Width", "Style", "Radius", "Display",
            "Layer", "Opacity", "Hidden",
        ] {
            XCTAssertTrue(labels.contains(label), "no \(label) row")
        }

        XCTAssertEqual(inspector.row("Auto layout")?.kind, .popup)
        XCTAssertEqual(inspector.row("Auto layout")?.choices, ["none", "row", "column", "abs"])
        XCTAssertEqual(inspector.row("Position X")?.text, "160", "the written pos x")
        XCTAssertEqual(inspector.row("Position Y")?.text, "-20", "the written pos y")
        XCTAssertEqual(inspector.row("Size W")?.text, "120", "the written width")
        XCTAssertEqual(inspector.row("Size H")?.text, "40", "the written height")
        XCTAssertEqual(inspector.row("Text.Title")?.text, "Titled")
        XCTAssertEqual(inspector.row("Header.Name")?.text, "LeafTitle")
    }

    // MARK: - Writing

    func testSettingWidthWritesTheFileAndUndoRestoresIt() throws {
        let document = try document(
            "element Screen { item Box { auto-layout abs; width 100; height 40; color #ffffff } }")
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Box" }))
        let inspector = inspector(document, canvas)

        XCTAssertEqual(inspector.row("Size W")?.text, "100")
        inspector.setText("123", for: "Size W")

        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("width 123"), document.sceneText ?? "")
        XCTAssertEqual(inspector.row("Size W")?.text, "123", "the row follows the edited tree")

        document.undoManager?.undo()
        canvas.show(document.sceneTree)
        inspector.refresh()
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("width 100"), "undo removes the edit")
        XCTAssertEqual(inspector.row("Size W")?.text, "100", "the row shows the value of the undone text")
    }

    /// A component's gap is the value in effect; the use writes its own only when the field is set.
    func testAnUnsetGapShowsTheEffectiveValueAndClearingRemovesTheCommand() throws {
        let document = try document(
            """
            element Screen {
                use tpl.Card as card
            }

            element tpl {
                item Card { gap 12; width 100; height 50 }
            }
            """)
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.kind == .use }))
        let inspector = inspector(document, canvas)

        XCTAssertEqual(inspector.row("Gap")?.text, "", "the use writes no gap")
        XCTAssertEqual(inspector.row("Gap")?.placeholder, "12", "the component's gap is the hint")

        inspector.setText("20", for: "Gap")
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("gap 20"), "the use writes its gap")
        XCTAssertEqual(inspector.row("Gap")?.text, "20")
        XCTAssertEqual(inspector.row("Gap")?.placeholder, "")

        inspector.setText("", for: "Gap")
        XCTAssertFalse(try XCTUnwrap(document.sceneText).contains("gap 20"), "an empty field removes it")
        XCTAssertEqual(inspector.row("Gap")?.text, "")
        XCTAssertEqual(inspector.row("Gap")?.placeholder, "12", "the effective gap is back")
    }

    func testTheAutoLayoutPopupWritesAndNoneRemovesTheCommand() throws {
        let document = try document(
            "element Screen { item Box { auto-layout abs; width 100; height 40; color #ffffff } }")
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Box" }))
        let inspector = inspector(document, canvas)

        inspector.setChoice("column", for: "Auto layout")
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("auto-layout column"))
        XCTAssertEqual(inspector.row("Auto layout")?.text, "column")

        inspector.setChoice("none", for: "Auto layout")
        XCTAssertFalse(
            try XCTUnwrap(document.sceneText).contains("auto-layout"), "none removes the command")
        XCTAssertEqual(inspector.row("Auto layout")?.text, "none")
    }

    func testTheClipToggleWritesYesAndRemovesIt() throws {
        let document = try document(
            "element Screen { item Box { auto-layout abs; width 100; height 40; color #ffffff } }")
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Box" }))
        let inspector = inspector(document, canvas)

        inspector.toggle("Clip")
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("clip yes"))
        XCTAssertEqual(inspector.row("Clip")?.text, "yes")

        inspector.toggle("Clip")
        XCTAssertFalse(try XCTUnwrap(document.sceneText).contains("clip"))
        XCTAssertEqual(inspector.row("Clip")?.text, "off")
    }

    // MARK: - Parts and mounted files

    /// A part of a component has no node of its own: the title goes into an at override of the use, and the
    /// library keeps its text.
    func testATitleOfAPartIsWrittenInTheUse() throws {
        let root = try workspace()
        let library = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let libraryBefore = try Data(contentsOf: library)
        let document = try document(
            """
            element Screen {
                use kit.Badge as pending
            }

            element kit --from-file "design-kit.tenn"
            """, in: root)
        let canvas = try canvas(document)
        let part = try XCTUnwrap(path(in: canvas) { $0.src?.key == "t" })
        XCTAssertTrue(inside(try XCTUnwrap(canvas.placed(at: part)), "pending"))
        XCTAssertNotNil(canvas.part(at: part))
        canvas.select(part)
        let inspector = inspector(document, canvas)

        inspector.setText("Nope", for: "Title")

        let at = commandText("at", "Box.t") + " { " + commandText("title", "Nope") + " }"
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains(at), document.sceneText ?? "")
        XCTAssertEqual(try Data(contentsOf: library), libraryBefore, "the library is not touched")
        XCTAssertEqual(inspector.row("Title")?.text, "Nope", "the part draws the override")
    }

    /// library-screen.tenn shows the mount element first: its items are written in design-kit.tenn, and the
    /// document refuses an edit of them.
    func testANodeOfAMountedFileIsRefused() throws {
        let root = try workspace()
        let library = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let libraryBefore = try Data(contentsOf: library)
        let document = try openTestDocument(copy("web/fixtures/scene/library-screen.tenn", to: root))
        let before = try XCTUnwrap(document.sceneText)
        let canvas = try canvas(document)
        let mounted = try XCTUnwrap(
            path(in: canvas) { $0.src?.pos.file == "design-kit.tenn" && $0.src?.kind == .item })
        XCTAssertNil(canvas.part(at: mounted), "a written node of the mounted file")
        canvas.select(mounted)
        let inspector = inspector(document, canvas)

        inspector.setText("200", for: "Size W")

        XCTAssertEqual(canvas.message, "design-kit.tenn is a mounted file: open it to edit it")
        XCTAssertEqual(document.sceneText, before, "a refused edit leaves the text as it was")
        XCTAssertEqual(try Data(contentsOf: library), libraryBefore)
    }

    // MARK: - Instances and the screen

    func testASelectedInstanceShowsItsComponentAndEditsTheUseBlock() throws {
        let document = try document(
            """
            element Screen {
                item Stage {
                    width 400; height 300; auto-layout abs
                    use tpl.Card as card { pos 10 20 }
                }
            }

            element tpl {
                item Card { width 100; height 80; gap 4 }
            }
            """)
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.kind == .use }))
        let inspector = inspector(document, canvas)

        XCTAssertEqual(inspector.title, "Instance")
        XCTAssertEqual(inspector.row("Component")?.text, "tpl.Card")
        XCTAssertEqual(inspector.row("Component")?.kind, .info)
        XCTAssertEqual(inspector.row("Position X")?.text, "10")

        inspector.setText("30", for: "Position X")
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("pos 30 20"), document.sceneText ?? "")
        XCTAssertEqual(inspector.row("Position X")?.text, "30")

        inspector.setText("Hi", for: "Title")
        let use = commandText("use", "tpl.Card", "as", "card")
        XCTAssertTrue(
            try XCTUnwrap(document.sceneText).contains("\(use) { pos 30 20; title Hi }"),
            document.sceneText ?? "")
        XCTAssertEqual(inspector.row("Title")?.text, "Hi")
    }

    func testNoSelectionShowsTheScreenNameAndSize() throws {
        let document = try document(
            "element Rules { item Stage { width 640; height 420; auto-layout abs } }")
        let canvas = try canvas(document)
        canvas.select(nil)
        let inspector = inspector(document, canvas)

        XCTAssertEqual(inspector.title, "Screen")
        XCTAssertEqual(inspector.row("Header.Name")?.text, "Rules")
        XCTAssertEqual(inspector.row("Header.Size")?.text, "640 x 420")
    }
}
