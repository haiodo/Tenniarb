//
//  SceneDocumentTests.swift
//  TenniarbTests
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//  WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
//  License for the specific language governing permissions and limitations under
//  the License.
//

import Tenn
import XCTest

@testable import Tenniarb

/// The document layer with a scene file: go/model/scenekind.go picks the kind on read, and a scene keeps
/// its text, which an edit changes and a save writes.
@MainActor
final class SceneDocumentTests: SceneDocumentTestCase {

    private let repo = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent()

    /// A scratch workspace: tests never read or write web/fixtures in place.
    private func workspace(_ files: [String: String] = [:]) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-doc-\(UUID().uuidString)", isDirectory: true)
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

    private func model(_ source: String) throws -> ElementModel {
        let parser = TennParser()
        let node = parser.parse(source)
        XCTAssertFalse(parser.errors.hasErrors(), "\(source) does not parse")
        return ElementModel.parseTenn(node: node)
    }

    // MARK: - scenekind.go

    func testSceneElementIsTheGoRule() throws {
        let scene = try model("element A { item Window { auto-layout row } }")
        XCTAssertTrue(sceneElement(try XCTUnwrap(scene.elements.first)), "auto-layout of an item")

        let nested = try model("element A { item B { item C {} } }")
        XCTAssertTrue(sceneElement(try XCTUnwrap(nested.elements.first)), "an item with child nodes")

        let use = try model("element A { use lib.Card as card }")
        XCTAssertTrue(sceneElement(try XCTUnwrap(use.elements.first)), "a use with a path")

        let property = try model("element A { use }")
        XCTAssertFalse(sceneElement(try XCTUnwrap(property.elements.first)), "a bare use is a user property")

        let classic = try model("element A { item B { pos 1 2 } }")
        XCTAssertFalse(sceneElement(try XCTUnwrap(classic.elements.first)))

        let link = try model("element A { item B {} item C {} link B C { auto-layout row } }")
        XCTAssertFalse(sceneElement(try XCTUnwrap(link.elements.first)), "a link gives no child nodes")
    }

    func testKindOfModelClassifiesADocument() throws {
        XCTAssertEqual(try kindOfModel(model("element A { item B {} }")), .diagram)
        XCTAssertEqual(try kindOfModel(model("element A { item B { auto-layout row } }")), .scene)
        XCTAssertEqual(try kindOfModel(model("element A { item B {} }\nelement C { item D { item E {} } }")), .mixed)
        XCTAssertEqual(
            try kindOfModel(model("element Kit --from-file 'kit.tenn'")), .scene,
            "a mounted file makes a document a scene when nothing else decides")
    }

    func testFromFileIsReadFromTheElementArguments() throws {
        let m = try model(#"element "kit" --from-file "design-kit.tenn""#)
        XCTAssertEqual(m.elements.first?.fromFile, "design-kit.tenn")
    }

    // MARK: - Reading

    func testClassicFixtureStaysClassic() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/Example.tenn", to: root)

        let document = try openTestDocument(url)

        XCTAssertNil(document.sceneTree)
        let store = try XCTUnwrap(document.store)
        XCTAssertEqual(store.model.modelName, "Example.tenn")
        XCTAssertFalse(store.model.elements.isEmpty)
        XCTAssertEqual(kindOfModel(store.model), .diagram)
    }

    func testSceneFixtureOpensWithItsTree() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/auto-layout-tour.tenn", to: root)

        let document = try openTestDocument(url)

        let tree = try XCTUnwrap(document.sceneTree)
        XCTAssertEqual(tree.main?.pos.file, "auto-layout-tour.tenn")
        XCTAssertEqual(tree.entry, "")
        XCTAssertEqual(tree.fatalCount, 0, "\(tree.diagnostics)")
        XCTAssertEqual(
            try XCTUnwrap(document.store).model.elements.map(\.name), ["Settings"],
            "the outline lists the elements of the file, not the classic model")
        XCTAssertFalse(document.isDocumentEdited)
    }

    func testSceneMountStaysOutOfTheClassicModel() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/library-screen.tenn", to: root)
        try copy("web/fixtures/scene/design-kit.tenn", to: root)

        let document = try openTestDocument(url)

        let tree = try XCTUnwrap(document.sceneTree)
        XCTAssertNotNil(tree.file("design-kit.tenn"))
        let rows = try XCTUnwrap(document.store).model.elements
        XCTAssertEqual(rows.map(\.name), ["kit", "Inbox"], "a mount is one row, its file is not parsed")
        XCTAssertTrue(rows[0].elements.isEmpty)
        XCTAssertTrue(rows[0].items.isEmpty, "the components of the mounted file stay out of the model")
    }

    func testSceneEntryPropertyIsLoadedInItsContext() throws {
        let root = try workspace([
            "tenniarb.tenn": "element \"Workspace\" {}\n",
            "design.tenn": "element \"p\" --from-file \"apps/part.tenn\"\n",
            "apps/part.tenn": "entry \"../design.tenn\"\n"
                + "element \"Screen\" { item Window { auto-layout row } }\n",
        ])

        let document = try openTestDocument(root.appendingPathComponent("apps/part.tenn"))

        let tree = try XCTUnwrap(document.sceneTree)
        XCTAssertEqual(tree.entry, "design.tenn")
        XCTAssertEqual(tree.main?.pos.file, "apps/part.tenn")
        XCTAssertEqual(tree.fatalCount, 0, "\(tree.diagnostics)")
    }

    func testASceneDocumentOpensAWindow() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/auto-layout-tour.tenn", to: root)
        let document = try openTestDocument(url)

        document.makeWindowControllers()

        XCTAssertNotNil(document.vc)
        let model = try XCTUnwrap(document.vc?.elementStore?.model)
        XCTAssertEqual(model.elements.map(\.name), ["Settings"], "the outline lists the screen")
        XCTAssertTrue(model.elements.allSatisfy { $0.items.isEmpty }, "no classic item is parsed")
    }

    // MARK: - Saving

    func testOpenAndSaveKeepEverySceneByte() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/library-screen.tenn", to: root)
        let library = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let before = try Data(contentsOf: url)
        let libraryBefore = try Data(contentsOf: library)

        let document = try openTestDocument(url)
        // An edit would set this; the save must not print the empty classic model over the scene.
        try XCTUnwrap(document.store).modified = true
        XCTAssertTrue(document.isDocumentEdited)

        try document.write(to: url, ofType: "tenn")

        XCTAssertEqual(try Data(contentsOf: url), before)
        XCTAssertEqual(try Data(contentsOf: library), libraryBefore)
    }

    func testAppKitSaveKeepsTheSceneBytes() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/auto-layout-tour.tenn", to: root)
        let before = try Data(contentsOf: url)

        let document = try openTestDocument(url)

        let saved = expectation(description: "saved")
        document.save(to: url, ofType: "tenn", for: .saveOperation) { error in
            XCTAssertNil(error)
            saved.fulfill()
        }
        wait(for: [saved], timeout: 5)

        XCTAssertEqual(try Data(contentsOf: url), before)
    }

    /// A scene document lists its elements in the outline, nested as the file writes them: the canvas has
    /// no other way to reach an element than the first one.
    func testTheOutlineListsTheElementsOfAScene() throws {
        let root = try workspace()
        let document = try openTestDocument(copy("web/fixtures/scene/library-screen.tenn", to: root))
        let rows = try XCTUnwrap(document.store?.model.elements)

        XCTAssertEqual(rows.map(\.name), ["kit", "Inbox"], "the file writes a mount, then the screen")
        XCTAssertEqual(document.sceneElementPath(of: rows[0]), [0])
        XCTAssertEqual(document.sceneElementPath(of: rows[1]), [1])
        XCTAssertTrue(rows[0].elements.isEmpty, "a mount element holds no element of its own")
    }

    func testNestedElementsNestInTheOutline() throws {
        let root = try workspace([
            "a.tenn": """
            element Screen {
                use kit.Chip as chip
            }

            element kit {
                element Group { item Box { item Dot } }
            }

            element Other { item Row { auto-layout row } }

            item Chip { title c }
            """
        ])
        let document = try openTestDocument(root.appendingPathComponent("a.tenn"))
        let rows = try XCTUnwrap(document.store?.model.elements)

        XCTAssertEqual(rows.map(\.name), ["Screen", "kit", "Other"], "a root item is no outline row")
        XCTAssertEqual(rows[1].elements.map(\.name), ["Group"])
        XCTAssertEqual(document.sceneElementPath(of: rows[1].elements[0]), [1, 0])
        XCTAssertEqual(document.sceneElementPath(of: rows[2]), [2])
    }

    /// A click in the outline puts the element of that row on the canvas: the document opens on its first
    /// element, and the mount of library-screen.tenn comes first.
    func testSelectingAnOutlineRowShowsThatElement() throws {
        let root = try workspace()
        _ = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let document = try openTestDocument(copy("web/fixtures/scene/library-screen.tenn", to: root))
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        vc.view.window?.contentView?.layoutSubtreeIfNeeded()
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)

        XCTAssertEqual(canvas.screen?.name, "kit", "the first element of the file is shown")

        let inbox = try XCTUnwrap((document.store?.model.elements ?? []).last)
        let row = vc.worldTree.row(forItem: inbox)
        XCTAssertGreaterThanOrEqual(row, 0, "the outline has a row for the screen")
        vc.worldTree.selectRowIndexes(IndexSet(integer: row), byExtendingSelection: false)

        XCTAssertEqual(canvas.screen?.name, "Inbox")
        XCTAssertEqual(canvas.screen?.w, 640, "the screen of the document is drawn")
        XCTAssertNotNil(canvas.placed)
    }

    /// The Items pane lists what the canvas draws: the rows of the shown element, and a row selects its
    /// item on the canvas.
    func testTheItemsPaneListsAndSelectsTheDrawnItems() throws {
        let root = try workspace()
        let document = try openTestDocument(copy("web/fixtures/scene/scene-text.tenn", to: root))
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        vc.view.window?.contentView?.layoutSubtreeIfNeeded()
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        let outline = try XCTUnwrap(vc.outlineSidebarController?.itemsOutlineView)

        XCTAssertEqual(outline.numberOfRows, 1, "the element draws one item, Stage")
        let cell = try XCTUnwrap(outline.view(atColumn: 0, row: 0, makeIfNecessary: true) as? NSTableCellView)
        XCTAssertEqual(cell.textField?.stringValue, "Stage")

        outline.selectRowIndexes(IndexSet(integer: 0), byExtendingSelection: false)
        XCTAssertEqual(canvas.placed(at: canvas.selection)?.r.name, "Stage")
    }

}
