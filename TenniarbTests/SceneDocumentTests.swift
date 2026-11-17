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

    // MARK: - Libraries

    /// Add Library appends the element that mounts the file, one undo step, and the outline lists its row.
    func testAddLibraryMountsTheFileAtTheEnd() throws {
        let root = try workspace([
            "screen.tenn": "element Screen { item Box { auto-layout row } }\n",
            "lib.tenn": "item Chip { title c }\n",
        ])
        let document = try openTestDocument(root.appendingPathComponent("screen.tenn"))
        try document.addLibrary(at: root.appendingPathComponent("lib.tenn"))

        XCTAssertEqual(
            document.sceneText,
            "element Screen { item Box { auto-layout row } }\nelement \"lib\" --from-file \"lib.tenn\"\n")
        XCTAssertNotNil(document.sceneTree?.file("lib.tenn"), "the mount is in the tree")
        XCTAssertEqual(document.store?.model.elements.map(\.name), ["Screen", "lib"], "a row for the mount")
        XCTAssertTrue(document.isDocumentEdited, "the edit is the document's")

        // a second mount of the same file takes the next free name
        try document.addLibrary(at: root.appendingPathComponent("lib.tenn"))
        XCTAssertTrue(document.sceneText?.contains(#"element "lib 2" --from-file "lib.tenn""#) ?? false)
        XCTAssertEqual(document.store?.model.elements.map(\.name), ["Screen", "lib", "lib 2"], "a row each")
        document.undoManager?.undo()
        XCTAssertFalse(document.sceneText?.contains("lib 2") ?? true, "undo takes the mount back")
        XCTAssertFalse(document.store?.model.elements.contains { $0.name == "lib 2" } ?? true, "and its row")
    }

    /// The path is written from the folder of the document, with the document in a subfolder of the root.
    func testAddLibraryUsesThePathFromTheDocumentFolder() throws {
        let root = try workspace([
            "tenniarb.tenn": "",
            "sub/screen.tenn": "element Screen { item Box { auto-layout row } }\n",
            "lib.tenn": "item Chip { title c }\n",
        ])
        let document = try openTestDocument(root.appendingPathComponent("sub/screen.tenn"))
        try document.addLibrary(at: root.appendingPathComponent("lib.tenn"))
        XCTAssertTrue(
            document.sceneText?.contains(#"--from-file "../lib.tenn""#) ?? false, document.sceneText ?? "")
    }

    func testAddLibraryRefusesAFileOutsideTheWorkspace() throws {
        let root = try workspace([
            "tenniarb.tenn": "",
            "screen.tenn": "element Screen { item Box { auto-layout row } }\n",
        ])
        let document = try openTestDocument(root.appendingPathComponent("screen.tenn"))
        let outside = FileManager.default.temporaryDirectory.appendingPathComponent("outside-\(UUID()).tenn")
        try "item Chip { title c }\n".write(to: outside, atomically: true, encoding: .utf8)
        addTeardownBlock { try? FileManager.default.removeItem(at: outside) }

        XCTAssertThrowsError(try document.addLibrary(at: outside)) { error in
            XCTAssertTrue(
                error.localizedDescription.contains("is outside the workspace root"),
                error.localizedDescription)
        }
        XCTAssertEqual(document.sceneText, "element Screen { item Box { auto-layout row } }\n")
    }

    // MARK: - Insert Component

    /// A mounted library offers what it says are components: the elements marked kind component and
    /// the items that declare parameters, never a plate or a part.
    func testComponentsOfAMountedLibrary() throws {
        let root = try workspace()
        _ = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let document = try openTestDocument(try copy("web/fixtures/scene/library-screen.tenn", to: root))

        let comps = document.components("", limit: 100)
        XCTAssertEqual(comps.map(\.path), ["kit.Button", "kit.Badge"], "\(comps)")
        XCTAssertEqual(comps.map(\.name), ["Button", "Badge"])
        XCTAssertEqual(comps.map(\.group), ["", ""], "root components have no branch")

        XCTAssertEqual(document.components("bad", limit: 100).map(\.name), ["Badge"], "the query folds case")
        XCTAssertEqual(document.components("", limit: 1).map(\.name), ["Button"], "limit caps the list")
        XCTAssertTrue(document.components("nothing", limit: 100).isEmpty)
    }

    /// The document's own use statements name nodes it can insert again, grouped by the branch they
    /// are defined in, as go/app/component_browser_test.go TestOwnComponentsOfTheDocument.
    func testOwnComponentsAreTheNodesTheDocumentReuses() throws {
        let root = try workspace([
            "own.tenn": """
            element Kit {
                item Icons {
                    item Plus { width 10; height 10 }
                    item Minus { width 10; height 10 }
                }
                item Chrome {
                    item Check { width 10; height 10 }
                }
            }
            element Screen {
                item Frame {
                    use Kit.Icons.Plus as plus
                    use Kit.Chrome.Check as check
                }
            }
            """
        ])
        let document = try openTestDocument(root.appendingPathComponent("own.tenn"))

        let comps = document.components("", limit: 100)
        XCTAssertEqual(comps.map(\.path), ["Kit.Icons.Plus", "Kit.Chrome.Check"])
        XCTAssertEqual(comps.map(\.group), ["Icons", "Chrome"])
        XCTAssertEqual(document.components("plus", limit: 100).map(\.name), ["Plus"], "the query filters")
        XCTAssertTrue(document.components("minus", limit: 100).isEmpty, "an unused node is no component")
    }

    /// A use written into an item lands in its block and takes the next free name.
    func testInsertComponentWritesAUseIntoAnItem() throws {
        let root = try workspace([
            "a.tenn": """
            element Screen {
                item Frame { auto-layout row; width 300; height 200 }
            }

            item Chip { title c }
            """
        ])
        let document = try openTestDocument(root.appendingPathComponent("a.tenn"))
        let tree = try XCTUnwrap(document.sceneTree)
        let main = try XCTUnwrap(tree.main)
        let screen = try XCTUnwrap(try? tree.resolve(main, "Screen"))
        let frame = try XCTUnwrap(try? tree.resolve(main, "Screen.Frame"))

        let offset = try document.insertComponent(
            path: "Chip", name: "Chip", into: frame, slot: "", canvas: screen, spot: (0, 0))
        XCTAssertTrue(
            document.sceneText?.contains("item Frame { auto-layout row; width 300; height 200; use Chip as Chip }") ?? false,
            document.sceneText ?? "")

        let next = try XCTUnwrap(document.sceneTree)
        let nextMain = try XCTUnwrap(next.main)
        let use = try XCTUnwrap(try? next.resolve(nextMain, "Screen.Frame.Chip"))
        XCTAssertEqual(use.kind, .use)
        XCTAssertEqual(use.range.start, offset, "the answer is where the canvas selects")
        XCTAssertEqual(use.key, "Chip")

        // the same name again takes the next free one
        let nextFrame = try XCTUnwrap(try? next.resolve(nextMain, "Screen.Frame"))
        let second = try document.insertComponent(
            path: "Chip", name: "Chip", into: nextFrame, slot: "", canvas: nextMain, spot: (0, 0))
        XCTAssertTrue(document.sceneText?.contains("use Chip as Chip_2") ?? false, document.sceneText ?? "")
        let after = try XCTUnwrap(document.sceneTree)
        let afterMain = try XCTUnwrap(after.main)
        XCTAssertEqual(try? after.resolve(afterMain, "Screen.Frame.Chip_2").range.start, second)
    }

    /// Inserting into an instance puts the use into the slot its component defines.
    func testInsertComponentFillsTheSlotOfAnInstance() throws {
        let root = try workspace([
            "a.tenn": """
            element Screen {
                item Frame { auto-layout row; width 300 }
                use Card as card
            }

            item Card {
                slot header
                slot { item Empty { title e } }
            }

            item Chip { title c }
            """
        ])
        let document = try openTestDocument(root.appendingPathComponent("a.tenn"))
        var tree = try XCTUnwrap(document.sceneTree)
        let main = try XCTUnwrap(tree.main)
        let screen = try XCTUnwrap(try? tree.resolve(main, "Screen"))
        let cardUse = try XCTUnwrap(screen.content.first { $0.kind == .use })
        let card = try XCTUnwrap(try? tree.resolve(main, "Card"))
        XCTAssertEqual(insertTarget(tree, cardUse).slots, ["", "header"], "the default slot comes first")
        XCTAssertEqual(componentSlots(card).named, ["header"])

        _ = try document.insertComponent(
            path: "Chip", name: "Chip", into: cardUse, slot: "header", canvas: main, spot: (0, 0))
        XCTAssertTrue(
            document.sceneText?.contains("use Card as card { slot header { use Chip as Chip } }") ?? false,
            document.sceneText ?? "")
        tree = try XCTUnwrap(document.sceneTree)
        let nextUse = try XCTUnwrap(try XCTUnwrap(tree.main).elems.first?.content.first { $0.kind == .use })
        XCTAssertTrue(nextUse.block.contains { $0.slot == "header" }, "the use sits in the named slot")
    }

    /// On the canvas the use gets the spot the caller picked, and undo takes the whole use back.
    func testInsertComponentOnTheCanvasWritesItsPosition() throws {
        let root = try workspace([
            "a.tenn": """
            element Screen {
                item Frame { auto-layout row; width 300; height 200 }
            }

            item Chip { title c }
            """
        ])
        let document = try openTestDocument(root.appendingPathComponent("a.tenn"))
        let tree = try XCTUnwrap(document.sceneTree)
        let main = try XCTUnwrap(tree.main)
        let screen = try XCTUnwrap(try? tree.resolve(main, "Screen"))

        let offset = try document.insertComponent(
            path: "Chip", name: "Chip", into: nil, slot: "", canvas: screen, spot: (420, 35))
        XCTAssertTrue(
            document.sceneText?.contains("use Chip as Chip { pos 420 35 }") ?? false, document.sceneText ?? "")
        let next = try XCTUnwrap(document.sceneTree)
        XCTAssertEqual(try? next.resolve(try XCTUnwrap(next.main), "Screen.Chip").range.start, offset)

        document.undoManager?.undo()
        XCTAssertFalse(document.sceneText?.contains("use Chip") ?? true, "undo takes the use back")
    }

    /// The items of the screen decide where a new top level node goes: right of the content, at its top.
    func testFreeSpotSitsRightOfTheContent() throws {
        let root = try workspace([
            "a.tenn": """
            element Screen {
                item A { auto-layout abs; pos 10 20; width 100; height 50 }
                item B { auto-layout abs; pos 200 -30; width 80; height 40 }
            }
            """
        ])
        let document = try openTestDocument(root.appendingPathComponent("a.tenn"))
        let view = SceneCanvasView(frame: .zero)
        view.show(document.sceneTree)
        XCTAssertEqual(view.freeSpot().x, 380, "200 + 80 + the 100 gap")
        XCTAssertEqual(view.freeSpot().y, 20, "the top of the placed content is -20")
    }

    /// The picker lists what the libraries offer and writes the chosen use where the selection points:
    /// the sheet, its Insert button and the canvas selection, end to end.
    func testInsertComponentSheetWritesThePickedUse() throws {
        let root = try workspace()
        _ = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let url = root.appendingPathComponent("screen.tenn")
        try """
        element Screen {
            item Frame { auto-layout row; width 300 }
        }

        element kit --from-file "design-kit.tenn"
        """.write(to: url, atomically: true, encoding: .utf8)
        let document = try openTestDocument(url)
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        vc.view.window?.contentView?.layoutSubtreeIfNeeded()
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        canvas.select([0])
        XCTAssertEqual(canvas.selectedNode?.name, "Frame")

        vc.insertComponent(nil)
        let sheet = try XCTUnwrap(vc.view.window?.attachedSheet, "Insert Component… opens a sheet")
        let insert = try XCTUnwrap(button("Insert", in: try XCTUnwrap(sheet.contentView)))
        insert.performClick(nil)

        XCTAssertTrue(
            document.sceneText?.contains("item Frame { auto-layout row; width 300; use kit.Button as Button }")
                ?? false, document.sceneText ?? "")
        XCTAssertNil(vc.view.window?.attachedSheet, "the sheet closes on insert")
        XCTAssertEqual(canvas.selectedNode?.name, "Button", "the inserted use is selected")
    }

    private func button(_ title: String, in view: NSView) -> NSButton? {
        if let b = view as? NSButton, b.title == title {
            return b
        }
        for sub in view.subviews {
            if let b = button(title, in: sub) {
                return b
            }
        }
        return nil
    }

    /// The picker puts a heading where the branch changes and filters by the search text.
    func testThePickerGroupsAndFiltersItsRows() throws {
        let node = Scene.Node(kind: .item)
        let all = [
            Scene.Component(path: "kit.Button", name: "Button", group: "", node: node),
            Scene.Component(path: "kit.Input", name: "Input", group: "Forms", node: node),
            Scene.Component(path: "kit.Tag", name: "Tag", group: "Forms", node: node),
        ]
        var picked: Scene.Component?
        let picker = ComponentPickerController(
            provider: { q in all.filter { q.isEmpty || $0.name.lowercased().contains(q) } },
            onPick: { picked = $0 })

        XCTAssertEqual(picker.rows.map(\.label), ["Button", "Forms", "Input", "Tag"], "one heading per branch")
        XCTAssertEqual(picker.selectedComponent?.path, "kit.Button", "the first component is selected")
        picker.query = "ta"
        XCTAssertEqual(picker.rows.map(\.label), ["Forms", "Tag"])
        XCTAssertEqual(picker.selectedComponent?.path, "kit.Tag")
        picker.query = "nothing"
        XCTAssertTrue(picker.rows.isEmpty)
        XCTAssertNil(picked)
    }

    // MARK: - The Items pane: component browser

    /// A scene that mounts design-kit.tenn, with a Frame on the canvas for an insert to land in.
    private func componentBrowserDocument() throws -> (ViewController, Document) {
        let root = try workspace()
        _ = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let url = root.appendingPathComponent("screen.tenn")
        try """
        element Screen {
            item Frame { auto-layout row; width 300 }
        }

        element kit --from-file "design-kit.tenn"
        """.write(to: url, atomically: true, encoding: .utf8)
        let document = try openTestDocument(url)
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        vc.view.window?.contentView?.layoutSubtreeIfNeeded()
        return (vc, document)
    }

    /// The browser lists what the library offers, a root component under no heading, and the pane returns
    /// to the drawn items of the canvas when switched back.
    func testComponentBrowserRowsAndModeSwitch() throws {
        let (vc, _) = try componentBrowserDocument()
        let sidebar = try XCTUnwrap(vc.outlineSidebarController)
        XCTAssertFalse(sidebar.showsComponents)
        XCTAssertEqual(sidebar.itemsRows.map(\.label), ["Frame"], "the drawn items of the canvas")

        sidebar.setItemsMode(true)
        XCTAssertTrue(sidebar.showsComponents)
        XCTAssertEqual(sidebar.itemsRows.map(\.label), ["Button", "Badge"])
        XCTAssertTrue(sidebar.itemsRows.allSatisfy(\.component))
        XCTAssertFalse(sidebar.itemsRows.contains { $0.kind == .group }, "a root component has no heading")

        sidebar.setItemsMode(false)
        XCTAssertEqual(sidebar.itemsRows.map(\.label), ["Frame"], "the items of the canvas again")
    }

    /// A document that mounts no library says where a library comes from instead of showing an empty list.
    func testComponentBrowserHintsWhenNothingIsOffered() throws {
        let root = try workspace([
            "plain.tenn": """
            element Screen {
                item Frame { auto-layout row; width 100 }
            }
            """
        ])
        let document = try openTestDocument(root.appendingPathComponent("plain.tenn"))
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        let sidebar = try XCTUnwrap(vc.outlineSidebarController)

        sidebar.setItemsMode(true)
        let hint = try XCTUnwrap(sidebar.itemsRows.first)
        XCTAssertEqual(sidebar.itemsRows.count, 1)
        XCTAssertEqual(hint.kind, .hint)
        XCTAssertFalse(hint.component)
        XCTAssertTrue(hint.label.contains("Add Library"), hint.label)
    }

    /// A double click on a component row writes the use where the selection points and selects it.
    func testComponentRowInsertWritesWhereTheSelectionPoints() throws {
        let (vc, document) = try componentBrowserDocument()
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        canvas.select([0])
        XCTAssertEqual(canvas.selectedNode?.name, "Frame")

        let sidebar = try XCTUnwrap(vc.outlineSidebarController)
        sidebar.setItemsMode(true)
        let button = try XCTUnwrap(sidebar.itemsRows.first { $0.usePath == "kit.Button" })
        sidebar.itemsDelegate?.activate(button)

        XCTAssertTrue(
            document.sceneText?.contains("item Frame { auto-layout row; width 300; use kit.Button as Button }") ?? false,
            document.sceneText ?? "")
        XCTAssertEqual(canvas.selectedNode?.name, "Button", "the inserted use is selected")
    }

    /// A component dropped on the canvas takes the spot of the drop; a path the document does not offer
    /// writes nothing.
    func testComponentDropWritesAtTheDropPoint() throws {
        let (vc, document) = try componentBrowserDocument()
        XCTAssertTrue(vc.dropComponent(path: "kit.Button", at: Scene.Point(x: 420, y: 35)))
        XCTAssertTrue(
            document.sceneText?.contains("use kit.Button as Button { pos 420 35 }") ?? false,
            document.sceneText ?? "")
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        XCTAssertEqual(canvas.selectedNode?.name, "Button", "the dropped use is selected")

        let before = document.sceneText
        XCTAssertFalse(vc.dropComponent(path: "kit.Nothing", at: Scene.Point(x: 10, y: 10)))
        XCTAssertEqual(document.sceneText, before, "a component the document does not offer was written")
    }

    /// The picture of a component is the piece it draws; a component of slots does not break the preview.
    func testComponentPreviewDrawsTheComponent() throws {
        let (vc, document) = try componentBrowserDocument()
        let sidebar = try XCTUnwrap(vc.outlineSidebarController)
        sidebar.setItemsMode(true)

        let button = try XCTUnwrap(sidebar.previewImage(path: "kit.Button"), "Button draws something")
        XCTAssertEqual(button.size, NSSize(width: 160, height: 100))
        let bitmap = try XCTUnwrap(NSBitmapImageRep(data: try XCTUnwrap(button.tiffRepresentation)))
        var drawn = false
        for y in stride(from: 0, to: bitmap.pixelsHigh, by: 4) {
            for x in stride(from: 0, to: bitmap.pixelsWide, by: 4)
            where (bitmap.colorAt(x: x, y: y)?.alphaComponent ?? 0) > 0 {
                drawn = true
            }
        }
        XCTAssertTrue(drawn, "the preview shows the button, not an empty box")

        // kit.Card is not offered by the browser; its slots are what the preview has to survive.
        let tree = try XCTUnwrap(document.sceneTree)
        let kit = try XCTUnwrap(tree.file("design-kit.tenn"))
        let card = try XCTUnwrap(try? tree.resolve(kit, "Card"))
        _ = ComponentPreview.image(tree: tree, node: card)
    }

    // MARK: - Element > Move

    /// The text one scene edit makes of text, as applySceneEdits does.
    private func applied(_ text: String, _ edit: Scene.TextEdit) -> String {
        var b = Array(text.utf8)
        b.replaceSubrange(edit.start..<edit.end, with: Array(edit.new.utf8))
        return String(decoding: b, as: UTF8.self)
    }

    private func sceneNode(_ tree: Scene.Tree, _ path: String) throws -> Scene.Node {
        let main = try XCTUnwrap(tree.main)
        return try XCTUnwrap(try? tree.resolve(main, path), "no node \(path)")
    }

    /// Down swaps a node with the sibling after it in one edit, the answer is where it starts in the new
    /// text, and one undo puts the file back.
    func testMoveNodeEditSwapsSiblings() throws {
        let root = try workspace([
            "a.tenn": "element Screen {\n  item A { auto-layout abs; pos 10 20; width 100 }\n"
                + "  item B { width 120 }\n}\n"
        ])
        let document = try openTestDocument(root.appendingPathComponent("a.tenn"))
        let tree = try XCTUnwrap(document.sceneTree)
        let main = try XCTUnwrap(tree.main)
        let text = try XCTUnwrap(tree.source(main.pos.file))
        let a = try sceneNode(tree, "Screen.A")

        let down = try XCTUnwrap(moveNodeEdit(text, a, 1))
        XCTAssertEqual(
            applied(text, down.edit),
            "element Screen {\n  item B { width 120 }\n  item A { auto-layout abs; pos 10 20; width 100 }\n}\n")

        try document.applySceneEdit(down.edit, in: a.pos.file)
        XCTAssertEqual(document.sceneText, applied(text, down.edit))
        XCTAssertEqual(try sceneNode(try XCTUnwrap(document.sceneTree), "Screen.A").range.start, down.at)

        document.undoManager?.undo()
        XCTAssertEqual(document.sceneText, text, "one undo step puts the file back")
    }

    /// The first sibling has nothing above it and the last nothing below: no edit, no change.
    func testMoveNodeEditHasNoPlaceAtTheEnds() throws {
        let root = try workspace([
            "a.tenn": "element Screen {\n  item A { auto-layout abs; width 100 }\n  item B { width 120 }\n}\n"
        ])
        let document = try openTestDocument(root.appendingPathComponent("a.tenn"))
        let tree = try XCTUnwrap(document.sceneTree)
        let text = try XCTUnwrap(tree.source(try XCTUnwrap(tree.main).pos.file))

        XCTAssertNil(moveNodeEdit(text, try sceneNode(tree, "Screen.A"), -1))
        XCTAssertNil(moveNodeEdit(text, try sceneNode(tree, "Screen.B"), 1))
        XCTAssertEqual(document.sceneText, text, "a refused move writes nothing")
    }

    /// A flow layout places its kids by itself, so a node moved into it loses its pos; an absolute parent
    /// keeps it.
    func testMoveNodeEditDropsThePosOfAFlowParent() throws {
        let root = try workspace([
            "a.tenn": "element Flow {\n  item Row {\n    auto-layout row\n"
                + "    item A { pos 5 5; width 10 }\n    item B { width 20 }\n  }\n}\n\n"
                + "element Abs {\n  item A { pos 5 5; width 10 }\n  item B { width 20 }\n}\n"
        ])
        let document = try openTestDocument(root.appendingPathComponent("a.tenn"))
        let tree = try XCTUnwrap(document.sceneTree)
        let text = try XCTUnwrap(tree.source(try XCTUnwrap(tree.main).pos.file))

        let flow = applied(text, try XCTUnwrap(moveNodeEdit(text, try sceneNode(tree, "Flow.Row.A"), 1)).edit)
        XCTAssertTrue(
            flow.hasPrefix(
                "element Flow {\n  item Row {\n    auto-layout row\n    item B { width 20 }\n"
                    + "    item A { width 10 }\n  }\n}"), "a flow parent places the node, its pos goes: \(flow)")

        let abs = applied(text, try XCTUnwrap(moveNodeEdit(text, try sceneNode(tree, "Abs.A"), 1)).edit)
        XCTAssertTrue(abs.contains("item A { pos 5 5; width 10 }"), "an absolute parent keeps the pos")
    }

    /// Left puts the node last inside the sibling before it, right first inside the one after; the answer
    /// is where the node starts in the new text, and one undo puts the file back.
    func testMoveNodeIntoEditTakesTheNeighbour() throws {
        let root = try workspace([
            "a.tenn": "element Screen {\n  item A { pos 1 2; width 10 }\n  item B { width 20 }\n"
                + "  item F {\n    auto-layout column\n  }\n}\n"
        ])
        let document = try openTestDocument(root.appendingPathComponent("a.tenn"))
        let text = try XCTUnwrap(document.sceneText)
        var tree = try XCTUnwrap(document.sceneTree)
        let file = try XCTUnwrap(tree.main).pos.file

        let left = try XCTUnwrap(moveNodeIntoEdit(text, try sceneNode(tree, "Screen.B"), -1))
        try document.applySceneEdit(left.edit, in: file)
        XCTAssertEqual(
            document.sceneText,
            "element Screen {\n  item A { pos 1 2; width 10; item B { width 20 } }\n"
                + "  item F {\n    auto-layout column\n  }\n}\n")
        tree = try XCTUnwrap(document.sceneTree)
        XCTAssertEqual(try sceneNode(tree, "Screen.A.B").range.start, left.at)

        document.undoManager?.undo()
        XCTAssertEqual(document.sceneText, text)
        tree = try XCTUnwrap(document.sceneTree)
        XCTAssertNil(moveNodeIntoEdit(text, try sceneNode(tree, "Screen.A"), -1), "no sibling before A")
        XCTAssertNil(moveNodeIntoEdit(text, try sceneNode(tree, "Screen.F"), 1), "no sibling after the flow")

        let right = try XCTUnwrap(moveNodeIntoEdit(text, try sceneNode(tree, "Screen.A"), 1))
        try document.applySceneEdit(right.edit, in: file)
        XCTAssertEqual(
            document.sceneText,
            "element Screen {\n  item B { item A { pos 1 2; width 10 }; width 20 }\n"
                + "  item F {\n    auto-layout column\n  }\n}\n")
        tree = try XCTUnwrap(document.sceneTree)
        XCTAssertEqual(try sceneNode(tree, "Screen.B.A").range.start, right.at)
    }

    /// The Element menu actions edit through the document and keep the moved node selected.
    func testMoveMenuActionsSwapAndSelectTheNode() throws {
        let root = try workspace([
            "screen.tenn": "element Screen {\n  item A { auto-layout abs; width 100 }\n  item B { width 120 }\n}\n"
        ])
        let document = try openTestDocument(root.appendingPathComponent("screen.tenn"))
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        vc.view.window?.contentView?.layoutSubtreeIfNeeded()
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        let tree = try XCTUnwrap(document.sceneTree)
        canvas.selectSource(at: try sceneNode(tree, "Screen.A").range.start)
        XCTAssertEqual(canvas.selectedNode?.name, "A")
        let up = NSMenuItem(title: "Move Up", action: #selector(ViewController.moveSceneNodeUp(_:)), keyEquivalent: "")
        XCTAssertTrue(vc.validateMenuItem(up), "a written node of the scene can move")

        vc.moveSceneNodeDown(nil)

        XCTAssertEqual(
            document.sceneText,
            "element Screen {\n  item B { width 120 }\n  item A { auto-layout abs; width 100 }\n}\n")
        XCTAssertEqual(canvas.selectedNode?.name, "A", "the moved node stays selected")

        document.undoManager?.undo()
        XCTAssertEqual(
            document.sceneText,
            "element Screen {\n  item A { auto-layout abs; width 100 }\n  item B { width 120 }\n}\n")
        canvas.selectSource(at: try sceneNode(try XCTUnwrap(document.sceneTree), "Screen.A").range.start)
        vc.moveSceneNodeIntoNext(nil)
        XCTAssertEqual(
            document.sceneText,
            "element Screen {\n  item B { item A { auto-layout abs; width 100 }; width 120 }\n}\n")
        XCTAssertEqual(canvas.selectedNode?.name, "A", "the node moved into the sibling is selected")

        canvas.select(nil)
        XCTAssertFalse(vc.validateMenuItem(up), "no selection, no move")
    }

    /// A node written in a mounted file resolves to that file; the document refuses it and the main text
    /// stays as it was.
    func testMoveMenuActionLeavesTheMainTextOfAMountAlone() throws {
        let root = try workspace()
        _ = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let document = try openTestDocument(try copy("web/fixtures/scene/library-screen.tenn", to: root))
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        vc.view.window?.contentView?.layoutSubtreeIfNeeded()
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        XCTAssertEqual(canvas.screen?.name, "kit")

        canvas.select([0])
        let node = try XCTUnwrap(canvas.selectedNode, "the mount draws the items of its file")
        XCTAssertEqual(node.pos.file, "design-kit.tenn")
        let before = document.sceneText

        vc.moveSceneNodeDown(nil)

        XCTAssertEqual(document.sceneText, before, "the main file keeps its text")
        XCTAssertTrue(canvas.message?.contains("mounted file") ?? false, canvas.message ?? "no message")
    }

    // MARK: - Element > Rename

    /// A scene window whose rename dialogs the closures answer, so no alert opens.
    private func renameDocument(_ files: [String: String], _ main: String = "a.tenn") throws -> (ViewController, Document) {
        let document = try openTestDocument(try workspace(files).appendingPathComponent(main))
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        vc.view.window?.contentView?.layoutSubtreeIfNeeded()
        return (vc, document)
    }

    /// Yes writes the node name and the use path that names it, as one undo step (go/app/rename_ui.go).
    func testRenameNodeUpdatesTheUsePathAsOneUndoStep() throws {
        let text = "element Parts {\n  item Button { width 40 }\n}\nelement Screen {\n  use Parts.Button as b\n}\n"
        let (vc, document) = try renameDocument(["a.tenn": text])
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        canvas.select([0])
        XCTAssertEqual(canvas.selectedNode?.name, "Button")
        var asked: Scene.RenamePlan?
        vc.renamePrompt = { _ in "Icon" }
        vc.renameConfirm = { plan in
            asked = plan
            return .yes
        }

        vc.renameSelection(nil)

        XCTAssertEqual(asked?.name, "Icon")
        XCTAssertEqual(asked?.refs.count, 1, "the use path names the item")
        XCTAssertEqual(
            document.sceneText,
            "element Parts {\n  item Icon { width 40 }\n}\nelement Screen {\n  use Parts.Icon as b\n}\n")
        XCTAssertEqual(canvas.selectedNode?.name, "Icon", "the renamed node stays selected")
        XCTAssertEqual(
            canvas.selectedNode?.range.start,
            try sceneNode(try XCTUnwrap(document.sceneTree), "Parts.Icon").range.start,
            "the selection is where the node starts in the new text")

        document.undoManager?.undo()
        XCTAssertEqual(document.sceneText, text, "one undo takes the name and the path back")
    }

    /// No writes the name alone: the use keeps the old name (go/app TestRenameOnlyLeavesReferences).
    func testRenameNodeNoLeavesTheReference() throws {
        let text = "element Parts {\n  item Button { width 40 }\n}\nelement Screen {\n  use Parts.Button as b\n}\n"
        let (vc, document) = try renameDocument(["a.tenn": text])
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        canvas.select([0])
        vc.renamePrompt = { _ in "Icon" }
        vc.renameConfirm = { _ in .no }

        vc.renameSelection(nil)

        XCTAssertEqual(
            document.sceneText,
            "element Parts {\n  item Icon { width 40 }\n}\nelement Screen {\n  use Parts.Button as b\n}\n")
    }

    /// Cancel keeps the old name and writes nothing.
    func testRenameNodeCancelKeepsTheName() throws {
        let text = "element Parts {\n  item Button { width 40 }\n}\nelement Screen {\n  use Parts.Button as b\n}\n"
        let (vc, document) = try renameDocument(["a.tenn": text])
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        canvas.select([0])
        vc.renamePrompt = { _ in "Icon" }
        vc.renameConfirm = { _ in .cancel }

        vc.renameSelection(nil)

        XCTAssertEqual(document.sceneText, text, "Cancel keeps the old name")
    }

    /// Nothing names this node, so the references question is not asked.
    func testRenameNodeWithoutReferencesDoesNotAsk() throws {
        let text = "element Screen {\n  item A { auto-layout abs; width 100 }\n}\n"
        let (vc, document) = try renameDocument(["a.tenn": text])
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        canvas.select([0])
        vc.renamePrompt = { _ in "C" }
        vc.renameConfirm = { _ in
            XCTFail("no references, so no question about them")
            return .cancel
        }

        vc.renameSelection(nil)

        XCTAssertEqual(document.sceneText, "element Screen {\n  item C { auto-layout abs; width 100 }\n}\n")
        XCTAssertEqual(canvas.selectedNode?.name, "C")
    }

    /// A key a sibling holds is refused by the plan, and the plan's message is what the canvas shows.
    func testRenameRefusesAKeyATakenSiblingHolds() throws {
        let text = "element Screen {\n  item A { auto-layout abs }\n  item B { auto-layout abs }\n}\n"
        let (vc, document) = try renameDocument(["a.tenn": text])
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        canvas.select([0])
        vc.renamePrompt = { _ in "B" }

        vc.renameSelection(nil)

        XCTAssertEqual(canvas.message, "There is something named \"B\" here already.")
        XCTAssertEqual(document.sceneText, text)
    }

    /// An empty name is refused by the plan, and nothing is written.
    func testRenameRefusesAnEmptyName() throws {
        let text = "element Screen {\n  item A { auto-layout abs }\n}\n"
        let (vc, document) = try renameDocument(["a.tenn": text])
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        canvas.select([0])
        vc.renamePrompt = { _ in "   " }

        vc.renameSelection(nil)

        XCTAssertEqual(canvas.message, "The name cannot be empty.")
        XCTAssertEqual(document.sceneText, text)
    }

    /// A reference in a mounted file is refused whole, naming the file: this window has no undo for it.
    func testRenameRefusesAReferenceInAMountedFile() throws {
        let main = "element Parts {\n  item Button { width 40 }\n}\n\nelement kit --from-file \"lib.tenn\"\n"
        let lib = "element Screen {\n  use Parts.Button as b\n}\n"
        let (vc, document) = try renameDocument(["a.tenn": main, "lib.tenn": lib])
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        canvas.select([0])
        XCTAssertEqual(canvas.selectedNode?.name, "Button")
        vc.renamePrompt = { _ in "Icon" }
        vc.renameConfirm = { _ in .yes }

        vc.renameSelection(nil)

        XCTAssertEqual(canvas.message, "lib.tenn is a mounted file: open it to edit it")
        XCTAssertEqual(document.sceneText, main, "nothing of the main file is written")
        XCTAssertEqual(document.sceneTree?.source("lib.tenn"), lib, "the mounted file keeps its text")
    }

    /// With no node selected, the element the canvas shows is renamed and the paths that name it follow.
    func testRenameElementFollowsThePathsThatNameIt() throws {
        let text = "element Parts {\n  item Button { width 40 }\n}\nelement Screen {\n  use Parts.Button as b\n}\n"
        let (vc, document) = try renameDocument(["a.tenn": text])
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        XCTAssertEqual(canvas.screenNode?.name, "Parts")
        canvas.select(nil)
        var asked: Scene.RenamePlan?
        vc.renamePrompt = { _ in "Kit" }
        vc.renameConfirm = { plan in
            asked = plan
            return .yes
        }

        vc.renameSelection(nil)

        XCTAssertEqual(asked?.refs.count, 1)
        XCTAssertEqual(
            document.sceneText,
            "element Kit {\n  item Button { width 40 }\n}\nelement Screen {\n  use Kit.Button as b\n}\n")
        XCTAssertEqual(document.store?.model.elements.first?.name, "Kit", "the outline row follows")

        let row = try XCTUnwrap(document.store?.model.elements.first)
        let index = vc.worldTree.row(forItem: row)
        XCTAssertGreaterThanOrEqual(index, 0, "the outline lists the screen")
        if index >= 0 {
            vc.worldTree.selectRowIndexes(IndexSet(integer: index), byExtendingSelection: false)
            let item = NSMenuItem(
                title: "Rename", action: #selector(OutlineViewControllerDelegate.renameElementAction(_:)), keyEquivalent: "")
            XCTAssertTrue(vc.outlineViewDelegate?.validateMenuItem(item) ?? false, "a scene element row offers Rename")
        }
    }

    /// A classic document has no scene text: the selected element is renamed through the model.
    func testRenameClassicElementGoesThroughTheModel() throws {
        let root = try workspace(["classic.tenn": "element D {\n  item A { pos 0 0; width 10 }\n}\n"])
        let document = try openTestDocument(root.appendingPathComponent("classic.tenn"))
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        let element = try XCTUnwrap(document.store?.model.elements.first)
        vc.onElementSelected(element)
        vc.renamePrompt = { _ in "Home" }

        vc.renameSelection(nil)

        XCTAssertEqual(element.name, "Home")
        document.undoManager?.undo()
        XCTAssertEqual(element.name, "D", "the model rename is one undo step")
    }

    // MARK: - Navigate: Back and Forward

    /// A scene of two elements with a node each, in a window whose view is loaded and laid out.
    private func navigationDocument() throws -> (ViewController, Document) {
        let url = try workspace([
            "a.tenn": """
            element A {
                item Box { auto-layout abs; width 100; height 50 }
                item Two { auto-layout abs; pos 120 0; width 100; height 50 }
            }
            element B {
                item Row { auto-layout abs; width 100; height 50 }
            }
            """
        ]).appendingPathComponent("a.tenn")
        let document = try openTestDocument(url)
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        vc.view.window?.contentView?.layoutSubtreeIfNeeded()
        return (vc, document)
    }

    private func validated(_ vc: ViewController, _ action: Selector) -> Bool {
        vc.validateMenuItem(NSMenuItem(title: "", action: action, keyEquivalent: ""))
    }

    /// The selections in the order the canvas showed them; Back returns to the element and the node an
    /// entry named, Forward walks ahead, and the menu offers only the directions that resolve.
    func testBackAndForwardWalkTheSelections() throws {
        let (vc, document) = try navigationDocument()
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        let rows = try XCTUnwrap(document.store?.model.elements)
        XCTAssertEqual(canvas.screenNode?.name, "A", "the first element is shown")
        XCTAssertEqual(vc.navigation.entries.count, 1, "the element on screen is the first entry")
        XCTAssertFalse(validated(vc, #selector(ViewController.navigateBack(_:))), "nothing is behind it")
        XCTAssertFalse(validated(vc, #selector(ViewController.navigateForward(_:))))

        canvas.select([0])
        vc.onElementSelected(rows[1])
        XCTAssertEqual(canvas.screenNode?.name, "B")
        XCTAssertEqual(canvas.selectedNode?.name, "Row")
        XCTAssertEqual(vc.navigation.entries.map(\.element), [["A"], ["A"], ["B"]])
        XCTAssertEqual(vc.navigation.entries.map(\.nodes), [[], ["Box"], ["Row"]])

        vc.navigateBack(nil)
        XCTAssertEqual(canvas.screenNode?.name, "A")
        XCTAssertEqual(canvas.selectedNode?.name, "Box", "the entry keeps the selected node")

        vc.navigateForward(nil)
        XCTAssertEqual(canvas.screenNode?.name, "B")
        XCTAssertEqual(canvas.selectedNode?.name, "Row")
        XCTAssertTrue(validated(vc, #selector(ViewController.navigateBack(_:))))
        XCTAssertFalse(validated(vc, #selector(ViewController.navigateForward(_:))), "nothing is ahead")

        vc.navigateBack(nil)
        vc.navigateBack(nil)
        XCTAssertEqual(canvas.screenNode?.name, "A")
        XCTAssertNil(canvas.selectedNode, "the first entry has no node")
        XCTAssertFalse(validated(vc, #selector(ViewController.navigateBack(_:))), "the start of the history")
        XCTAssertTrue(validated(vc, #selector(ViewController.navigateForward(_:))))
    }

    /// A selection made after Back cuts off the entries that were ahead, as Go's push does.
    func testSelectionAfterBackDropsTheEntriesAhead() throws {
        let (vc, document) = try navigationDocument()
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        let rows = try XCTUnwrap(document.store?.model.elements)
        canvas.select([0])
        vc.onElementSelected(rows[1])
        vc.navigateBack(nil)

        canvas.select([1])

        XCTAssertEqual(canvas.screenNode?.name, "A")
        XCTAssertEqual(canvas.selectedNode?.name, "Two")
        XCTAssertEqual(vc.navigation.entries.map(\.nodes), [[], ["Box"], ["Two"]])
        XCTAssertFalse(validated(vc, #selector(ViewController.navigateForward(_:))), "Forward is gone")
        vc.navigateForward(nil)
        XCTAssertEqual(canvas.selectedNode?.name, "Two", "nothing ahead to show")
    }

    /// The entry already on screen is not written again, so Back does not grow the list.
    func testTheSameEntryIsWrittenOnce() {
        var history = NavigateHistory()
        let entry = NavigationEntry(element: ["A"], nodes: ["Box"])
        history.push(entry)
        history.push(entry)
        XCTAssertEqual(history.entries.count, 1)
        history.push(NavigationEntry(element: ["B"]))
        XCTAssertEqual(history.entries.count, 2)
        XCTAssertEqual(history.at, 1)

        for i in 0..<130 { history.push(NavigationEntry(element: ["E\(i)"])) }
        XCTAssertEqual(history.entries.count, NavigateHistory.limit, "the oldest entries go")
        XCTAssertEqual(history.at, NavigateHistory.limit - 1)
    }

    /// The Navigate menu offers Back and Forward on Go's shortcuts.
    func testTheNavigateMenuOffersBackAndForward() throws {
        let menu = try XCTUnwrap(NSApp.mainMenu?.items.compactMap(\.submenu).first { $0.title == "Navigate" })
        for (action, key) in [
            (#selector(ViewController.navigateBack(_:)), "["),
            (#selector(ViewController.navigateForward(_:)), "]"),
        ] {
            let item = try XCTUnwrap(menu.items.first { $0.action == action })
            XCTAssertEqual(item.keyEquivalent, key)
            XCTAssertEqual(item.keyEquivalentModifierMask, .command)
        }
    }

    /// An entry whose node the file lost is skipped for the older one, and it stays in the list.
    func testAnEntryWhoseNodeIsGoneIsSkipped() throws {
        let (vc, document) = try navigationDocument()
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        let rows = try XCTUnwrap(document.store?.model.elements)
        canvas.select([0])
        vc.onElementSelected(rows[1])
        let box = try sceneNode(try XCTUnwrap(document.sceneTree), "A.Box")
        try document.applySceneEdit(Scene.TextEdit(start: box.range.start, end: box.range.end, new: ""), in: "a.tenn")

        vc.navigateBack(nil)

        XCTAssertEqual(canvas.screenNode?.name, "A")
        XCTAssertNil(canvas.selectedNode)
        XCTAssertEqual(vc.navigation.at, 0)
        XCTAssertEqual(vc.navigation.entries.count, 3, "an entry that no longer resolves stays")
    }

    /// A classic document records the element and the items by name, and Back selects them again.
    func testClassicHistoryRestoresTheElementAndItsItems() throws {
        let root = try workspace([
            "classic.tenn": """
            element D {
                item A { pos 0 0; width 10; height 10 }
                item B { pos 20 0; width 10; height 10 }
            }
            element E {
                item C { pos 0 0; width 10; height 10 }
            }
            """
        ])
        let document = try openTestDocument(root.appendingPathComponent("classic.tenn"))
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        let rows = try XCTUnwrap(document.store?.model.elements)
        XCTAssertNil(document.sceneTree, "a classic document")
        vc.onElementSelected(rows[0])
        vc.scene.setActiveItem(try XCTUnwrap(rows[0].items.first { $0.name == "A" }))
        vc.onElementSelected(rows[1])

        XCTAssertEqual(vc.navigation.entries.map(\.element), [["D"], ["D"], ["E"]])
        XCTAssertEqual(vc.navigation.entries.map(\.items), [[], ["A"], []])

        vc.navigateBack(nil)

        XCTAssertTrue(vc.selectedElement === rows[0])
        XCTAssertEqual(vc.activeItems.map(\.name), ["A"])
        XCTAssertEqual(vc.scene.activeItems.map(\.name), ["A"])
    }

    /// A rename rewrites the entry that named the node, so Back and Forward still resolve it by its new
    /// name.
    func testRenameRewritesTheHistoryNodePath() throws {
        let (vc, document) = try navigationDocument()
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        let rows = try XCTUnwrap(document.store?.model.elements)
        canvas.select([0])
        vc.onElementSelected(rows[1])
        vc.navigateBack(nil)
        XCTAssertEqual(canvas.selectedNode?.name, "Box")

        vc.renamePrompt = { _ in "Crate" }
        vc.renameConfirm = { _ in .yes }
        vc.renameSelection(nil)

        XCTAssertTrue(document.sceneText?.contains("item Crate") ?? false, document.sceneText ?? "")
        XCTAssertEqual(vc.navigation.entries[1].nodes, ["Crate"], "the entry follows the rename")
        XCTAssertEqual(vc.navigation.entries.count, 3, "the rename is no new entry")

        vc.navigateForward(nil)
        XCTAssertEqual(canvas.screenNode?.name, "B")
        vc.navigateBack(nil)
        XCTAssertEqual(canvas.screenNode?.name, "A")
        XCTAssertEqual(canvas.selectedNode?.name, "Crate", "the rewritten entry resolves")
    }

}
