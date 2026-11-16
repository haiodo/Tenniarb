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
/// through the document, and a node of a mounted file is refused on the canvas.
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

    func testATitleOfAMountedNodeIsRefused() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/library-screen.tenn", to: root)
        let library = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let libraryBefore = try Data(contentsOf: library)
        let document = try openTestDocument(url)
        let before = try XCTUnwrap(document.sceneText)
        let canvas = try canvas(document)

        let path = try XCTUnwrap(
            path(in: canvas) { $0.src?.pos.file == "design-kit.tenn" && $0.prop("title") != nil },
            "a drawn part of the mounted library")
        canvas.select(path)
        canvas.beginTitleEdit()
        let field = try XCTUnwrap(canvas.editField, "a part of a component opens the field like an item")
        XCTAssertFalse(field.stringValue.isEmpty, "prefilled with the title the layout drew")
        field.stringValue = "Nope"
        canvas.endTitleEdit(commit: true)

        XCTAssertEqual(canvas.message, "design-kit.tenn is a mounted file: open it to edit it")
        XCTAssertEqual(document.sceneText, before)
        XCTAssertFalse(document.isDocumentEdited)
        XCTAssertEqual(try Data(contentsOf: library), libraryBefore)
        XCTAssertEqual(canvas.selection, path, "a refused edit keeps the selection")
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
