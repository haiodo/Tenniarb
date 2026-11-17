//
//  SceneCanvasEditTests.swift
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

import XCTest

@testable import Tenniarb

/// Input on the scene canvas: a click selects the drawn item under it, a double click edits its title
/// through the document, a part of a library is edited through its use, and a node of a mounted file
/// shown as it is is refused.
@MainActor
final class SceneCanvasEditTests: SceneDocumentTestCase {

    private let repo = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent()

    // MARK: - Fixtures

    /// A scratch workspace: the fixtures are copied, never opened in place.
    private func workspace() throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-canvas-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    private func copy(_ fixture: String, to root: URL) throws -> URL {
        let url = root.appendingPathComponent((fixture as NSString).lastPathComponent)
        try FileManager.default.copyItem(at: repo.appendingPathComponent(fixture), to: url)
        return url
    }

    /// A canvas of a fixture at 100%: no document, only the picture and the hit-test.
    private func canvas(_ fixture: String) throws -> SceneCanvasView {
        let root = try workspace()
        let url = try copy(fixture, to: root)
        let view = SceneCanvasView(frame: .zero)
        view.show(try SceneLoader(root: root).load(url.lastPathComponent))
        let screen = try XCTUnwrap(view.screen)
        view.frame = NSRect(origin: .zero, size: SceneCanvasView.contentSize(screen))
        return view
    }

    /// A canvas that writes through the document it draws.
    private func canvas(_ document: Document) throws -> SceneCanvasView {
        let view = SceneCanvasView(frame: .zero)
        view.document = document
        view.show(document.sceneTree)
        let screen = try XCTUnwrap(view.screen)
        view.frame = NSRect(origin: .zero, size: SceneCanvasView.contentSize(screen))
        return view
    }

    /// A canvas over a .tenn written into a scratch workspace, no document: the picture and the hit-test.
    private func canvas(text: String) throws -> SceneCanvasView {
        let root = try workspace()
        let url = root.appendingPathComponent("a.tenn")
        try text.write(to: url, atomically: true, encoding: .utf8)
        let view = SceneCanvasView(frame: .zero)
        view.show(try SceneLoader(root: root).load("a.tenn"))
        let screen = try XCTUnwrap(view.screen)
        view.frame = NSRect(origin: .zero, size: SceneCanvasView.contentSize(screen))
        return view
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

    /// Whether pl is drawn in the instance named name, itself or one of its parts.
    private func inside(_ pl: Scene.Placed, _ name: String) -> Bool {
        var p: Scene.Placed? = pl
        while let c = p {
            if c.r.name == name { return true }
            p = c.parent
        }
        return false
    }

    // MARK: - Bitmap

    private func bitmap(_ view: NSView) throws -> (rep: NSBitmapImageRep, scale: CGFloat) {
        let rep = try XCTUnwrap(view.bitmapImageRepForCachingDisplay(in: view.bounds))
        view.cacheDisplay(in: view.bounds, to: rep)
        return (rep, CGFloat(rep.pixelsWide) / view.bounds.width)
    }

    /// The colour at a point of the view, y from the top as the canvas draws it.
    private func color(_ bitmap: (rep: NSBitmapImageRep, scale: CGFloat), _ x: Double, _ y: Double) throws -> NSColor {
        let px = Int((x * bitmap.scale).rounded(.down))
        let py = Int((y * bitmap.scale).rounded(.down))
        return try XCTUnwrap(bitmap.rep.colorAt(x: px, y: py)?.usingColorSpace(.deviceRGB))
    }

    private func isAccent(_ c: NSColor) -> Bool {
        c.blueComponent > c.redComponent + 0.2 && c.blueComponent > 0.6
    }

    // MARK: - Selection

    /// scene-text.tenn: Stage covers the 640 by 420 screen, LeafTitle is a 120 by 40 leaf at 160,20.
    func testAHitPicksTheDeepestDrawnItemAndEmptySpaceHitsNothing() throws {
        let canvas = try canvas("web/fixtures/scene/scene-text.tenn")

        let leaf = try XCTUnwrap(canvas.hitItem(Scene.Point(x: 220, y: 40)), "the middle of LeafTitle")
        XCTAssertEqual(canvas.placed(at: leaf)?.r.name, "LeafTitle", "the leaf over the Stage fill wins")
        XCTAssertTrue(canvas.placed(at: leaf)?.r.kids.isEmpty ?? false)

        let stage = try XCTUnwrap(canvas.hitItem(Scene.Point(x: 600, y: 380)), "the empty part of Stage")
        XCTAssertEqual(canvas.placed(at: stage)?.r.name, "Stage", "a fill with no child under it")
        XCTAssertNil(canvas.hitItem(Scene.Point(x: 700, y: 450)), "below the screen")
        XCTAssertNil(canvas.hitItem(Scene.Point(x: -10, y: 20)), "left of the screen")

        canvas.select(leaf)
        XCTAssertEqual(canvas.selection, leaf)
        canvas.select(nil)
        XCTAssertNil(canvas.selection)
    }

    func testTheSelectionStrokeIsDrawnAroundTheItem() throws {
        let canvas = try canvas("web/fixtures/scene/scene-text.tenn")
        let path = try XCTUnwrap(canvas.hitItem(Scene.Point(x: 220, y: 40)))
        canvas.select(path)

        let shot = try bitmap(canvas)
        let corner = canvas.viewPoint(Scene.Point(x: 160, y: 20))
        let left = (x: corner.x - 2, y: corner.y + 22)  // the grown box of LeafTitle, middle of its left edge
        XCTAssertTrue(isAccent(try color(shot, left.x, left.y)), "the selection stroke is not on the box edge")
        XCTAssertFalse(isAccent(try color(shot, left.x - 6, left.y)), "the stroke is thin, not a fill")
        XCTAssertFalse(isAccent(try color(shot, left.x + 6, left.y)), "the stroke does not cover the item")

        canvas.select(nil)
        let clean = try bitmap(canvas)
        XCTAssertFalse(isAccent(try color(clean, left.x, left.y)), "no stroke without a selection")
    }

    // MARK: - Inline title

    func testACommitWritesTheTitleAndUndoBringsItBack() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/scene-text.tenn", to: root)
        let document = try openTestDocument(url)
        let before = try XCTUnwrap(document.sceneText)
        let canvas = try canvas(document)

        let path = try XCTUnwrap(canvas.hitItem(Scene.Point(x: 220, y: 40)))
        canvas.select(path)
        canvas.beginTitleEdit()
        let field = try XCTUnwrap(canvas.editField, "a selected item opens the field")
        XCTAssertEqual(field.stringValue, "Titled", "the field holds the text the layout drew")

        field.stringValue = "Edited title"
        canvas.endTitleEdit(commit: true)

        let old = #"title "Titled"; color #eeeeee"#
        let new = #"title "Edited title"; color #eeeeee"#
        XCTAssertTrue(before.contains(old), "the fixture still writes the old title")
        XCTAssertEqual(
            document.sceneText, before.replacingOccurrences(of: old, with: new), "only the title changes")
        XCTAssertTrue(document.isDocumentEdited)
        XCTAssertNil(canvas.editField)

        XCTAssertEqual(canvas.selection, path, "the canvas keeps the selection")
        XCTAssertEqual(
            canvas.placed(at: path)?.r.prop("title")?.text(0), "Edited title", "the canvas draws the new title")

        document.undoManager?.undo()
        XCTAssertEqual(document.sceneText, before)
        XCTAssertFalse(document.isDocumentEdited, "an undo back to the saved text clears the mark")
        // The window controller reloads the scene tree; a canvas with no window is told here.
        canvas.show(document.sceneTree)
        XCTAssertEqual(
            canvas.placed(at: path)?.r.prop("title")?.text(0), "Titled", "the old title is drawn again")
        XCTAssertEqual(canvas.selection, path)
    }

    func testEscapeLeavesTheTitleAlone() throws {
        let root = try workspace()
        let document = try openTestDocument(copy("web/fixtures/scene/scene-text.tenn", to: root))
        let before = try XCTUnwrap(document.sceneText)
        let canvas = try canvas(document)

        let path = try XCTUnwrap(canvas.hitItem(Scene.Point(x: 220, y: 40)))
        canvas.select(path)
        canvas.beginTitleEdit()
        canvas.editField?.stringValue = "Dropped"
        canvas.endTitleEdit(commit: false)

        XCTAssertEqual(document.sceneText, before)
        XCTAssertFalse(document.isDocumentEdited)
        XCTAssertNil(canvas.editField)
        XCTAssertEqual(canvas.selection, path, "Escape keeps the selection and drops the text")
    }

    func testATitleOfALibraryPartGoesToTheUseAroundIt() throws {
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

        let path = try XCTUnwrap(
            path(in: canvas) { $0.src?.key == "t" && $0.src?.pos.file == "design-kit.tenn" },
            "the label of a badge, a part of the mounted library")
        canvas.select(path)
        canvas.beginTitleEdit()
        let field = try XCTUnwrap(canvas.editField, "a part of a component opens the field like an item")
        XCTAssertEqual(field.stringValue, "New", "prefilled with the title the layout drew")
        field.stringValue = "Nope"
        canvas.endTitleEdit(commit: true)

        let text = try XCTUnwrap(document.sceneText)
        XCTAssertTrue(
            text.contains("\(commandText("at", "Box.t")) { \(commandText("title", "Nope")) }"), text)
        XCTAssertEqual(try Data(contentsOf: library), libraryBefore, "the mounted library is not touched")
        XCTAssertEqual(
            canvas.placed(at: path)?.r.prop("title")?.text(0), "Nope", "the part draws the override")
        XCTAssertEqual(canvas.selection, path)
    }

    /// library-screen.tenn shows the mount element `kit` first, whose content is written in design-kit.tenn:
    /// an item of that file is edited there, and the document refuses it.
    func testANodeOfAMountedFileIsRefused() throws {
        let root = try workspace()
        let library = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let libraryBefore = try Data(contentsOf: library)
        let document = try openTestDocument(copy("web/fixtures/scene/library-screen.tenn", to: root))
        let before = try XCTUnwrap(document.sceneText)
        let canvas = try canvas(document)

        let path = try XCTUnwrap(
            path(in: canvas) { $0.src?.key == "t" && $0.src?.pos.file == "design-kit.tenn" },
            "a drawn node of the mounted file")
        canvas.select(path)
        canvas.beginTitleEdit()
        let field = try XCTUnwrap(canvas.editField)
        field.stringValue = "Nope"
        canvas.endTitleEdit(commit: true)

        XCTAssertEqual(canvas.message, "design-kit.tenn is a mounted file: open it to edit it")
        XCTAssertEqual(document.sceneText, before)
        XCTAssertFalse(document.isDocumentEdited)
        XCTAssertEqual(try Data(contentsOf: library), libraryBefore)
        XCTAssertEqual(canvas.selection, path, "a refused edit keeps the selection")
    }

    // MARK: - Parts of a component

    /// The template and two copies of it, as go/app/sceneedit_test.go sharedDoc: one command per copy.
    private let copiesDoc = """
        element Screen {
            auto-layout row
            use tpl.Card as one
            use tpl.Card as two { pos 5 -5 }
        }

        element tpl {
            item Card {
                auto-layout column
                width 80
                item Title { title Hi; font-size 14 }
                item Row { item Cell }
            }
        }
        """

    /// Two copies side by side: a click selects a copy whole, the next double click steps one level in.
    private let pickDoc = """
        element Screen {
            item Row {
                auto-layout row
                gap 20
                use tpl.Card as one
                use tpl.Card as two
            }
        }

        element tpl {
            item Card {
                auto-layout column
                width 80
                item Title { title Hi; font-size 14 }
                item Row { item Cell }
            }
        }
        """

    func testAClickSelectsTheCopyAndADoubleClickStepsIn() throws {
        let canvas = try canvas(text: pickDoc)
        let title = try XCTUnwrap(path(in: canvas) { $0.src?.key == "Title" && inside($0, "two") })
        let box = try XCTUnwrap(canvas.placed(at: title))
        let p = Scene.Point(x: box.x + box.w / 2, y: box.y + box.h / 2)

        canvas.click(p, clicks: 1)
        XCTAssertEqual(canvas.placed(at: canvas.selection)?.r.name, "two", "the copy is selected whole")
        XCTAssertEqual(canvas.placed(at: canvas.selection)?.src?.kind, .use)

        canvas.click(p, clicks: 2)
        XCTAssertEqual(canvas.placed(at: canvas.selection)?.src?.key, "Title", "the next click steps in")

        let other = try XCTUnwrap(path(in: canvas) { $0.src?.key == "Title" && inside($0, "one") })
        let otherBox = try XCTUnwrap(canvas.placed(at: other))
        canvas.click(Scene.Point(x: otherBox.x + otherBox.w / 2, y: otherBox.y + otherBox.h / 2), clicks: 1)
        XCTAssertEqual(canvas.placed(at: canvas.selection)?.r.name, "one", "the other copy is its own")
    }

    /// The title of a part is an at override in the use around it, so the template and the other copy keep
    /// theirs; a second edit updates that at instead of writing another.
    func testATitleOfAPartStaysInItsUse() throws {
        let document = try document(copiesDoc)
        let before = try XCTUnwrap(document.sceneText)
        let canvas = try canvas(document)
        let part = try XCTUnwrap(path(in: canvas) { $0.src?.key == "Title" && inside($0, "two") })
        let at = commandText("at", "Title")

        canvas.select(part)
        canvas.beginTitleEdit()
        try XCTUnwrap(canvas.editField).stringValue = "Bye"
        canvas.endTitleEdit(commit: true)

        XCTAssertEqual(
            document.sceneText,
            before.replacingOccurrences(
                of: "use tpl.Card as two { pos 5 -5 }",
                with: "use tpl.Card as two { pos 5 -5; \(at) { title Bye } }"))
        XCTAssertEqual(canvas.placed(at: part)?.r.prop("title")?.text(0), "Bye", "the part draws the override")
        XCTAssertEqual(canvas.selection, part, "the edit keeps the selection")
        let other = try XCTUnwrap(path(in: canvas) { $0.src?.key == "Title" && inside($0, "one") })
        XCTAssertEqual(canvas.placed(at: other)?.r.prop("title")?.text(0), "Hi", "the other copy keeps its title")

        canvas.beginTitleEdit()
        try XCTUnwrap(canvas.editField).stringValue = "Again"
        canvas.endTitleEdit(commit: true)
        let text = try XCTUnwrap(document.sceneText)
        XCTAssertTrue(text.contains("\(at) { title Again }"), text)
        XCTAssertEqual(text.components(separatedBy: at).count - 1, 1, "one at, not two")

        document.undoManager?.undo()
        document.undoManager?.undo()
        XCTAssertEqual(document.sceneText, before)
        canvas.show(document.sceneTree)
        XCTAssertEqual(canvas.placed(at: part)?.r.prop("title")?.text(0), "Hi", "undo draws the old title")
    }

    /// The fallback of a slot is reached through the slot, as go/app TestAtPathThroughASlot.
    func testAPartReachedThroughASlotIsNamedThroughTheSlot() throws {
        let document = try document(
            """
            element Screen {
                use tpl.Box as b
            }

            element tpl {
                item Box {
                    item Head
                    slot body { item Fallback { title a } }
                }
            }
            """)
        let canvas = try canvas(document)
        let part = try XCTUnwrap(path(in: canvas) { $0.src?.key == "Fallback" })

        canvas.select(part)
        canvas.beginTitleEdit()
        try XCTUnwrap(canvas.editField).stringValue = "z"
        canvas.endTitleEdit(commit: true)

        XCTAssertTrue(
            document.sceneText?.contains("\(commandText("at", "body.Fallback")) { title z }") ?? false,
            document.sceneText ?? "")
        XCTAssertEqual(canvas.placed(at: part)?.r.prop("title")?.text(0), "z")
        XCTAssertNil(canvas.message)
    }

    /// Two parts with one name cannot be told apart by an at path, so the canvas refuses and keeps the text.
    func testAPartWithNoUniqueNameIsRefused() throws {
        let document = try document(
            """
            element Screen {
                use tpl.Box as b
            }

            element tpl {
                item Box {
                    item Chip { title A }
                    item Chip { title B }
                }
            }
            """)
        let before = try XCTUnwrap(document.sceneText)
        let canvas = try canvas(document)
        let instance = try XCTUnwrap(path(in: canvas) { $0.inst })
        let second = instance + [1]  // the second Chip among the drawn siblings: its name is chip#1

        canvas.select(second)
        canvas.beginTitleEdit()
        try XCTUnwrap(canvas.editField).stringValue = "C"
        canvas.endTitleEdit(commit: true)

        XCTAssertEqual(
            canvas.message, "Not applied: the item has no unique name inside the instance, edit it in the use")
        XCTAssertEqual(document.sceneText, before)
        XCTAssertFalse(document.isDocumentEdited)
        XCTAssertEqual(canvas.selection, second, "a refused edit keeps the selection")
    }

    // MARK: - Window

    /// The double click, on the canvas of a real window, reaches the document the canvas was given.
    func testADoubleClickOpensTheTitleField() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/scene-text.tenn", to: root)
        let document = try openTestDocument(url)
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        vc.view.window?.contentView?.layoutSubtreeIfNeeded()
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)
        XCTAssertTrue(canvas.document === document, "the canvas writes through the document of its window")
        XCTAssertNotNil(canvas.screen)

        let at = canvas.convert(canvas.viewPoint(Scene.Point(x: 220, y: 40)), to: nil)
        let event = try XCTUnwrap(
            NSEvent.mouseEvent(
                with: .leftMouseDown, location: at, modifierFlags: [], timestamp: 0,
                windowNumber: vc.view.window?.windowNumber ?? 0, context: nil, eventNumber: 1, clickCount: 2,
                pressure: 1))
        canvas.mouseDown(with: event)

        XCTAssertEqual(canvas.placed(at: canvas.selection)?.r.name, "LeafTitle")
        let field = try XCTUnwrap(canvas.editField, "a double click opens the title field")
        XCTAssertEqual(field.stringValue, "Titled")
        // The field sits on the box the canvas draws for the item, not on a text rect of its own.
        let corner = canvas.viewPoint(Scene.Point(x: 160, y: 20))
        XCTAssertEqual(field.frame.minX, corner.x, accuracy: 0.01)
        XCTAssertEqual(field.frame.minY, corner.y, accuracy: 0.01)
        XCTAssertEqual(field.frame.width, 120 * canvas.zoom, accuracy: 0.01)

        let before = try String(contentsOf: url, encoding: .utf8)
        field.stringValue = "Edited title"
        canvas.endTitleEdit(commit: true)
        XCTAssertNotEqual(document.sceneText, before, "the double click commits through the document")
        XCTAssertEqual(
            canvas.placed(at: canvas.selection)?.r.prop("title")?.text(0), "Edited title",
            "the window reloads the canvas")

        document.undoManager?.undo()
        XCTAssertEqual(document.sceneText, before)
        XCTAssertEqual(
            canvas.placed(at: canvas.selection)?.r.prop("title")?.text(0), "Titled", "the undo reaches the canvas")
    }
}
