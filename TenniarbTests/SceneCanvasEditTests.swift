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

    /// A mouse event at a point of the view, in window coordinates, as AppKit delivers it.
    private func mouse(_ type: NSEvent.EventType, _ p: NSPoint, clicks: Int = 1, in view: NSView) throws -> NSEvent {
        try XCTUnwrap(
            NSEvent.mouseEvent(
                with: type, location: view.convert(p, to: nil), modifierFlags: [], timestamp: 0,
                windowNumber: view.window?.windowNumber ?? 0, context: nil, eventNumber: 1, clickCount: clicks,
                pressure: 1))
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

    // MARK: - Drag

    /// scene-text.tenn writes LeafTitle at pos 160 -20; a drag of 30 right and 10 down keeps pos y up.
    func testADragWritesThePositionOfAFreeItem() throws {
        let document = try document(String(contentsOf: repo.appendingPathComponent("web/fixtures/scene/scene-text.tenn"), encoding: .utf8))
        let before = try XCTUnwrap(document.sceneText)
        let canvas = try canvas(document)
        let p = Scene.Point(x: 220, y: 40)  // the middle of LeafTitle
        let path = try XCTUnwrap(canvas.hitItem(p))
        canvas.select(path)

        XCTAssertTrue(canvas.beginDrag(p))
        canvas.moveDrag(Scene.Point(x: p.x + 30, y: p.y + 10))
        canvas.endDrag()

        XCTAssertEqual(
            document.sceneText, before.replacingOccurrences(of: "pos 160 -20", with: "pos 190 -30"))
        XCTAssertEqual(canvas.placed(at: path)?.x, 190, "the canvas draws the item where the drag left it")
        XCTAssertEqual(canvas.placed(at: path)?.y, 30, "pos y up, the screen y down")

        document.undoManager?.undo()
        XCTAssertEqual(document.sceneText, before)
        canvas.show(document.sceneTree)
        XCTAssertEqual(canvas.placed(at: path)?.x, 160, "undo puts the item back")
    }

    /// A part with a pos of its own moves by an at override in its use.
    func testADragOfAPartWritesPosInItsUse() throws {
        let document = try document(
            """
            element Screen {
                use tpl.Box as b
            }

            element tpl {
                item Box {
                    width 100
                    height 100
                    item Dot { title d; pos 10 10 }
                }
            }
            """)
        let canvas = try canvas(document)
        let path = try XCTUnwrap(path(in: canvas) { $0.src?.key == "Dot" })
        let dot = try XCTUnwrap(canvas.placed(at: path))
        canvas.select(path)

        XCTAssertTrue(canvas.beginDrag(Scene.Point(x: dot.x + dot.w / 2, y: dot.y + dot.h / 2)))
        canvas.moveDrag(Scene.Point(x: dot.x + dot.w / 2 + 5, y: dot.y + dot.h / 2 - 7))
        canvas.endDrag()

        XCTAssertTrue(
            document.sceneText?.contains("\(commandText("at", "Dot")) { pos 15 17 }") ?? false,
            document.sceneText ?? "")
        XCTAssertEqual(canvas.placed(at: path)?.x, 15, "the part draws the override")
        XCTAssertEqual(canvas.placed(at: path)?.y, -17, "pos y up, the screen y down")
    }

    /// A click on a node inside an auto-layout moves the container it sits in, as movable does: the pos
    /// goes into the use of the instance, not into an at override of the leaf.
    func testADragOfAFlowItemMovesItsContainer() throws {
        let document = try document(
            """
            element Screen {
                use tpl.Col as c
            }

            element tpl {
                item Col {
                    auto-layout column
                    width 50
                    item Leaf { title x }
                }
            }
            """)
        let canvas = try canvas(document)
        let path = try XCTUnwrap(path(in: canvas) { $0.src?.key == "Leaf" })
        let leaf = try XCTUnwrap(canvas.placed(at: path))
        canvas.select(path)

        XCTAssertTrue(canvas.beginDrag(Scene.Point(x: leaf.x + leaf.w / 2, y: leaf.y + leaf.h / 2)))
        canvas.moveDrag(Scene.Point(x: leaf.x + leaf.w / 2 + 4, y: leaf.y + leaf.h / 2 + 6))
        canvas.endDrag()

        XCTAssertTrue(
            document.sceneText?.contains("use tpl.Col as c { pos 4 -6 }") ?? false, document.sceneText ?? "")
        XCTAssertFalse(document.sceneText?.contains("at ") ?? true, "the leaf has no place of its own")
    }

    func testAPressWithoutAMoveWritesNothing() throws {
        let document = try document(String(contentsOf: repo.appendingPathComponent("web/fixtures/scene/scene-text.tenn"), encoding: .utf8))
        let before = try XCTUnwrap(document.sceneText)
        let canvas = try canvas(document)
        let p = Scene.Point(x: 220, y: 40)
        canvas.select(try XCTUnwrap(canvas.hitItem(p)))

        XCTAssertTrue(canvas.beginDrag(p))
        canvas.endDrag()

        XCTAssertEqual(document.sceneText, before)
        XCTAssertFalse(document.isDocumentEdited)
    }

    // MARK: - Drag and links

    /// Two free siblings and a link, B far enough that the edge anchors barely turn as A moves.
    private let linkedDoc = """
        element Screen {
            item Stage {
                auto-layout abs
                width 1200
                height 300
                item A { title a; pos 0 0; width 80; height 40 }
                item B { title b; pos 1000 0; width 80; height 40 }
                link "A" "B"
            }
        }
        """

    /// The points of the one link of the screen, in the coordinates of the item that owns it.
    private func linkPoints(_ canvas: SceneCanvasView) throws -> [Scene.Point] {
        let links = try XCTUnwrap(canvas.screen?.links())
        XCTAssertEqual(links.count, 1, "the fixture writes one link")
        return try XCTUnwrap(links.first).path.map { step in
            switch step {
            case let .move(x, y), let .line(x, y): return Scene.Point(x: x, y: y)
            case let .quad(_, _, x, y): return Scene.Point(x: x, y: y)
            }
        }
    }

    /// The first point is the end near A; it follows the box while the drag is in flight, before the edit.
    /// The anchor slides a little along the edge as the line turns, so the delta is not exact.
    func testALinkFollowsTheDraggedItemBeforeTheMouseIsReleased() throws {
        let document = try document(linkedDoc)
        let canvas = try canvas(document)
        let a = try XCTUnwrap(path(in: canvas) { $0.src?.key == "A" })
        let box = try XCTUnwrap(canvas.placed(at: a))
        let p = Scene.Point(x: box.x + box.w / 2, y: box.y + box.h / 2)
        let before = try linkPoints(canvas)
        canvas.select(a)

        XCTAssertTrue(canvas.beginDrag(p))
        canvas.moveDrag(Scene.Point(x: p.x + 30, y: p.y + 10))

        let after = try linkPoints(canvas)
        let from = try XCTUnwrap(before.first)
        let moved = try XCTUnwrap(after.first)
        XCTAssertEqual(moved.x - from.x, 30, accuracy: 1, "the end near A moves with it")
        XCTAssertEqual(moved.y - from.y, 10, accuracy: 1, "the screen y down, as the pointer")
        XCTAssertEqual(try XCTUnwrap(after.last).x, try XCTUnwrap(before.last).x, accuracy: 1, "B stays")
        canvas.endDrag()
    }

    /// A press that never moves the pointer shifts neither the frame nor the line drawn from it.
    func testAClickOnALinkedItemLeavesTheLinkAlone() throws {
        let document = try document(linkedDoc)
        let canvas = try canvas(document)
        let a = try XCTUnwrap(path(in: canvas) { $0.src?.key == "A" })
        let box = try XCTUnwrap(canvas.placed(at: a))
        let p = Scene.Point(x: box.x + box.w / 2, y: box.y + box.h / 2)
        let before = try linkPoints(canvas)
        let frame = try XCTUnwrap(canvas.screen?.p.frames[ObjectIdentifier(box.r)])
        let x = frame.rx
        let y = frame.ry
        canvas.select(a)

        XCTAssertTrue(canvas.beginDrag(p))
        canvas.moveDrag(p)
        canvas.endDrag()

        XCTAssertEqual(frame.rx, x)
        XCTAssertEqual(frame.ry, y)
        let after = try linkPoints(canvas)
        XCTAssertEqual(try XCTUnwrap(after.first).x, try XCTUnwrap(before.first).x, accuracy: 1e-9)
        XCTAssertEqual(try XCTUnwrap(after.first).y, try XCTUnwrap(before.first).y, accuracy: 1e-9)
    }

    /// The release writes pos and the rebuilt screen draws the link where the drag drew it.
    func testARebuiltScreenLinksTheItemAtItsWrittenPos() throws {
        let document = try document(linkedDoc)
        let canvas = try canvas(document)
        let a = try XCTUnwrap(path(in: canvas) { $0.src?.key == "A" })
        let box = try XCTUnwrap(canvas.placed(at: a))
        let p = Scene.Point(x: box.x + box.w / 2, y: box.y + box.h / 2)
        canvas.select(a)
        XCTAssertTrue(canvas.beginDrag(p))
        canvas.moveDrag(Scene.Point(x: p.x + 30, y: p.y + 10))
        let during = try linkPoints(canvas)
        canvas.endDrag()

        XCTAssertTrue(document.sceneText?.contains("pos 30 -10") ?? false, document.sceneText ?? "")
        let moved = try XCTUnwrap(canvas.placed(at: a))
        XCTAssertEqual(moved.x, 30)
        XCTAssertEqual(moved.y, 10, "pos y up, the screen y down")
        let after = try linkPoints(canvas)
        XCTAssertEqual(try XCTUnwrap(after.first).x, try XCTUnwrap(during.first).x, accuracy: 1e-9)
        XCTAssertEqual(try XCTUnwrap(after.first).y, try XCTUnwrap(during.first).y, accuracy: 1e-9)
    }

    /// A document refuses an item of a mounted file: the rebuilt screen undoes the drag, links with it.
    func testARefusedDragLeavesTheLinkWhereItWas() throws {
        let root = try workspace()
        let library = root.appendingPathComponent("linked-kit.tenn")
        let libraryText = """
            item Card {
                item A { title a; pos 0 0; width 80; height 40 }
                item B { title b; pos 400 0; width 80; height 40 }
                link "A" "B"
            }
            """
        try libraryText.write(to: library, atomically: true, encoding: .utf8)
        let document = try document(
            """
            element lib --from-file "linked-kit.tenn"

            element Screen {
                use lib.Card as card
            }
            """, in: root)
        let before = try XCTUnwrap(document.sceneText)
        let canvas = try canvas(document)
        let a = try XCTUnwrap(path(in: canvas) { $0.src?.key == "A" }, "a drawn node of the mounted file")
        let box = try XCTUnwrap(canvas.placed(at: a))
        let p = Scene.Point(x: box.x + box.w / 2, y: box.y + box.h / 2)
        let link = try linkPoints(canvas)
        canvas.select(a)

        XCTAssertTrue(canvas.beginDrag(p))
        canvas.moveDrag(Scene.Point(x: p.x + 30, y: p.y + 10))
        XCTAssertEqual(
            try XCTUnwrap(linkPoints(canvas).first).x - (try XCTUnwrap(link.first).x), 30, accuracy: 1,
            "the link follows the drag before the edit is refused")
        canvas.endDrag()

        XCTAssertEqual(canvas.message, "linked-kit.tenn is a mounted file: open it to edit it")
        XCTAssertEqual(document.sceneText, before)
        XCTAssertEqual(try String(contentsOf: library, encoding: .utf8), libraryText)
        let after = try linkPoints(canvas)
        XCTAssertEqual(try XCTUnwrap(after.first).x, try XCTUnwrap(link.first).x, accuracy: 1e-9)
        XCTAssertEqual(try XCTUnwrap(after.first).y, try XCTUnwrap(link.first).y, accuracy: 1e-9)
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

    /// A drag of real events through the canvas of a window writes the new pos through the document.
    func testADragInTheWindowWritesThePosition() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/scene-text.tenn", to: root)
        let document = try openTestDocument(url)
        document.makeWindowControllers()
        let vc = try XCTUnwrap(document.vc)
        _ = vc.view
        vc.view.window?.contentView?.layoutSubtreeIfNeeded()
        let canvas = try XCTUnwrap(vc.editorViewController?.sceneCanvas?.canvas)

        let down = canvas.viewPoint(Scene.Point(x: 220, y: 40))
        // the first click selects; the press on the selection is what a drag picks the item up with
        canvas.mouseDown(with: try mouse(.leftMouseDown, down, in: canvas))
        canvas.mouseUp(with: try mouse(.leftMouseUp, down, in: canvas))
        XCTAssertEqual(canvas.placed(at: canvas.selection)?.r.name, "LeafTitle")
        canvas.mouseDown(with: try mouse(.leftMouseDown, down, in: canvas))

        let up = canvas.viewPoint(Scene.Point(x: 250, y: 50))
        canvas.mouseDragged(with: try mouse(.leftMouseDragged, up, in: canvas))
        canvas.mouseUp(with: try mouse(.leftMouseUp, up, in: canvas))

        let text = try XCTUnwrap(document.sceneText)
        XCTAssertTrue(text.contains("pos 190 -30"), text)
        XCTAssertEqual(canvas.placed(at: canvas.selection)?.x, 190, "the drawing follows the drag")
    }

    // MARK: - Items pane

    /// The rows under the element and under an item, as the Items pane lists them: a row per drawn item,
    /// kids only where something is drawn under it.
    func testItemRowsFollowTheDrawnTree() throws {
        let canvas = try canvas(text: pickDoc)
        let top = canvas.itemRows()
        XCTAssertEqual(top.map(\.label), ["Row"])
        XCTAssertTrue(top[0].kids)
        XCTAssertEqual(canvas.itemRows(top[0].path).map(\.label), ["one", "two"])
        XCTAssertEqual(canvas.itemRows([0, 1]).map(\.label), ["Title", "Row"])
        XCTAssertFalse(canvas.itemRows([0, 1])[0].kids, "a leaf opens nothing")
        XCTAssertTrue(canvas.itemRows([0, 1])[1].kids)
        XCTAssertEqual(canvas.itemRows([9]).count, 0, "a stale path lists nothing")
    }

}
