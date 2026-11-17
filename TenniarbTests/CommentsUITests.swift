//
//  CommentsUITests.swift
//  TenniarbTests
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.  You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//  WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
//  License for the specific language governing permissions and limitations under
//  the License.
//

import XCTest

@testable import Tenniarb

/// The comments UI without a window: the node paths the Go service reads, the store, the pins and the
/// anchors of a click.
@MainActor
final class CommentsUITests: SceneDocumentTestCase {

    /// 2026-10-28T10:00:00Z, the time of the Go tests.
    private let t0 = Date(timeIntervalSince1970: 1_793_181_600)

    private func scratch(_ prefix: String) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("\(prefix)-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    /// The config folder a test controller writes under.
    private func configDir() throws -> URL {
        try scratch("tenniarb-comments-ui").appendingPathComponent("cfg", isDirectory: true)
    }

    private func tree(_ text: String) throws -> Scene.Tree {
        let root = try scratch("tenniarb-comments-scene")
        try text.write(to: root.appendingPathComponent("scene.tenn"), atomically: true, encoding: .utf8)
        return try SceneLoader(root: root).load("scene.tenn")
    }

    /// The canvas of the first element of a scratch file, at the size of its content.
    private func canvas(_ text: String) throws -> SceneCanvasView {
        let tree = try tree(text)
        let view = SceneCanvasView(frame: .zero)
        view.show(tree)
        let screen = try XCTUnwrap(view.screen, "the scene does not build: \(tree.diagnostics)")
        view.frame = NSRect(origin: .zero, size: SceneCanvasView.contentSize(screen))
        return view
    }

    // MARK: - Node paths

    func testNodePathRepeatsAndNests() throws {
        let tree = try tree(
            """
            element "Screen" {
                item A { width 10; height 10 }
                item A { width 10; height 10 }
                item Frame { width 10; height 10; item Badge { width 5; height 5 } }
            }
            """)
        let el = try XCTUnwrap(tree.main?.elems.first)
        let firstA = el.content[0]
        let secondA = el.content[1]
        let frame = el.content[2]
        let badge = frame.content[0]

        XCTAssertEqual(CommentPaths.nodePath(of: firstA, from: el), "A")
        XCTAssertEqual(
            CommentPaths.nodePath(of: secondA, from: el), "A#1",
            "Go's nodeSeg counts the earlier siblings of the key: the second A is A#1")
        XCTAssertEqual(CommentPaths.nodePath(of: frame, from: el), "Frame")
        XCTAssertEqual(CommentPaths.nodePath(of: badge, from: el), "Frame.Badge")

        XCTAssertTrue(CommentPaths.node(at: "A", from: el) === firstA)
        XCTAssertTrue(CommentPaths.node(at: "A#1", from: el) === secondA)
        XCTAssertTrue(CommentPaths.node(at: "Frame.Badge", from: el) === badge)
        XCTAssertNil(CommentPaths.node(at: "A#2", from: el), "a repeat names the n-th earlier sibling")
        XCTAssertNil(CommentPaths.node(at: "Frame.Nope", from: el))
    }

    func testNodePathGoesThroughSlots() throws {
        let tree = try tree(
            """
            element "Screen" {
                item Frame {
                    width 200; height 100
                    slot actions { item Badge { width 10; height 10 } }
                    slot { item Fallback { width 10; height 10 } }
                }
            }
            """)
        let el = try XCTUnwrap(tree.main?.elems.first)
        let frame = el.content[0]
        let badge = try XCTUnwrap(frame.content.first { $0.kind == .slot }?.content.first)
        let fallback = try XCTUnwrap(frame.content.last { $0.kind == .slot }?.content.first)

        XCTAssertEqual(CommentPaths.nodePath(of: badge, from: el), "Frame.actions.Badge")
        XCTAssertEqual(CommentPaths.nodePath(of: fallback, from: el), "Frame..Fallback")
        XCTAssertTrue(CommentPaths.node(at: "Frame.actions.Badge", from: el) === badge)
        XCTAssertTrue(CommentPaths.node(at: "Frame..Fallback", from: el) === fallback)
    }

    func testElementPathNamesNestedAndRepeatedElements() throws {
        let tree = try tree(
            """
            element "Root" {
                item Frame { width 10; height 10 }
                element "Nested" { item One { width 5; height 5 } }
                element "Nested" { item Two { width 5; height 5 } }
            }
            """)
        let root = try XCTUnwrap(tree.main?.elems.first)
        XCTAssertEqual(CommentPaths.elementPath(of: root), ["Root"])
        XCTAssertEqual(CommentPaths.elementPath(of: root.elems[0]), ["Root", "Nested"])
        XCTAssertEqual(CommentPaths.elementPath(of: root.elems[1]), ["Root", "Nested#1"])
        XCTAssertEqual(CommentPaths.elementPath(of: try XCTUnwrap(tree.main)), [], "the file root is not an element")
    }

    // MARK: - The store through the controller

    func testControllerWritesTheFileAndReloadsIt() throws {
        let cfg = try configDir()
        let key = "path:/design/a.tenn"
        let anchor = Comments.Anchor(element: ["Screen"], node: "Frame", point: Comments.Point(x: 1, y: 2))
        let comments = CommentsController(key: key, configDir: cfg)
        XCTAssertEqual(comments.comments(), [])

        let added = try comments.addComment(text: "  too tight\n", anchor: anchor, now: t0)
        XCTAssertEqual(added.text, "too tight")
        XCTAssertTrue(FileManager.default.fileExists(atPath: Comments.Store(dir: Comments.dirIn(cfg)).file(for: key).path))

        let again = CommentsController(key: key, configDir: cfg)
        XCTAssertEqual(again.comments(), [added])

        try again.reply(id: added.id, text: "padding is 4 now", now: t0.addingTimeInterval(60))
        try again.resolve(id: added.id, text: "fixed", now: t0.addingTimeInterval(120))
        var stored = try Comments.Store(dir: Comments.dirIn(cfg)).load(key)
        XCTAssertEqual(stored.comments.first?.state, Comments.resolved)
        XCTAssertEqual(stored.comments.first?.replies?.map(\.text), ["padding is 4 now", "fixed"])

        try again.reopen(id: added.id, now: t0.addingTimeInterval(180))
        stored = try Comments.Store(dir: Comments.dirIn(cfg)).load(key)
        XCTAssertEqual(stored.comments.first?.state, Comments.open)

        try again.delete(id: added.id)
        XCTAssertTrue(try Comments.Store(dir: Comments.dirIn(cfg)).load(key).comments.isEmpty)
        XCTAssertTrue(again.comments().isEmpty)
    }

    func testDeletingAnotherAuthorsCommentKeepsTheFile() throws {
        let cfg = try configDir()
        let key = "path:/design/a.tenn"
        let store = Comments.Store(dir: Comments.dirIn(cfg))
        let other = Comments.Comment(
            id: "deadbeef", created: t0, author: "claude-code", text: "mine", anchor: Comments.Anchor(),
            state: Comments.open)
        _ = try store.update(key) { $0.comments.append(other) }

        let comments = CommentsController(key: key, configDir: cfg)
        XCTAssertThrowsError(try comments.delete(id: other.id)) {
            XCTAssertEqual($0 as? Comments.Error, Comments.Error.notYours)
        }
        XCTAssertEqual(try store.load(key).comments.map(\.id), [other.id])
    }

    func testAnUntitledDocumentCannotWrite() throws {
        let cfg = try configDir()
        let comments = CommentsController(key: nil, configDir: cfg)
        XCTAssertThrowsError(
            try comments.addComment(text: "x", anchor: Comments.Anchor(), now: t0)
        ) {
            XCTAssertEqual($0 as? CommentsController.Error, CommentsController.Error.untitled)
        }
        XCTAssertFalse(FileManager.default.fileExists(atPath: Comments.dirIn(cfg).path))
    }

    // MARK: - Pins

    func testPinsFollowTheGoPlacementRule() throws {
        let comments = CommentsController(key: "path:/a.tenn", configDir: try configDir())
        let element = ["Screen"]
        let boxes = ["Frame.Badge": CGRect(x: 100, y: 50, width: 40, height: 20)]
        let corner = try comments.addComment(
            text: "corner", anchor: Comments.Anchor(element: element, node: "Frame.Badge"),
            now: t0)
        let offset = try comments.addComment(
            text: "offset",
            anchor: Comments.Anchor(element: element, node: "Frame.Badge", point: Comments.Point(x: 5, y: 7)),
            now: t0)
        let origin = try comments.addComment(
            text: "origin",
            anchor: Comments.Anchor(element: element, node: "", point: Comments.Point(x: 3, y: 4)), now: t0)
        let gone = try comments.addComment(
            text: "gone", anchor: Comments.Anchor(element: element, node: "Gone"), now: t0)

        let pins = comments.pins(
            element: element, box: { boxes[$0] }, origin: CGPoint(x: 10, y: 20))
        XCTAssertEqual(pins.map(\.id), [corner.id, offset.id, origin.id, gone.id])
        XCTAssertEqual(pins.map(\.number), [1, 2, 3, 4])
        XCTAssertEqual(pins[0].point, CGPoint(x: 140, y: 50), "a node without a point sits at its top right")
        XCTAssertEqual(pins[1].point, CGPoint(x: 105, y: 57), "a point is measured from the top left of the box")
        XCTAssertEqual(pins[2].point, CGPoint(x: 13, y: 24), "without a node it is measured from the origin")
        XCTAssertEqual(pins[3].point, CGPoint(x: 10, y: 20), "a node that is gone sits at the origin")
        XCTAssertFalse(pins[0].detached)
        XCTAssertTrue(pins[3].detached)
        XCTAssertEqual(comments.pin(at: CGPoint(x: 140, y: 50))?.id, corner.id)
        XCTAssertNil(comments.pin(at: CGPoint(x: 400, y: 400)))
    }

    func testResolvedPinsAreHiddenUnlessAsked() throws {
        let comments = CommentsController(key: "path:/a.tenn", configDir: try configDir())
        let element = ["Screen"]
        let first = try comments.addComment(
            text: "open", anchor: Comments.Anchor(element: element, node: "", point: Comments.Point(x: 0, y: 0)),
            now: t0)
        let second = try comments.addComment(
            text: "done", anchor: Comments.Anchor(element: element, node: "", point: Comments.Point(x: 20, y: 20)),
            now: t0)
        try comments.resolve(id: second.id, now: t0.addingTimeInterval(60))

        XCTAssertEqual(comments.pins(element: element, box: { _ in nil }).map(\.id), [first.id])
        let all = comments.pins(element: element, box: { _ in nil }, showResolved: true)
        XCTAssertEqual(all.map(\.id), [first.id, second.id])
        XCTAssertEqual(all.map(\.number), [1, 2])
        XCTAssertTrue(all[1].resolved)
    }

    func testPinHitTestPicksTheTopmost() throws {
        let comments = CommentsController(key: "path:/a.tenn", configDir: try configDir())
        let anchor = Comments.Anchor(element: ["Screen"], node: "", point: Comments.Point(x: 0, y: 0))
        let first = try comments.addComment(text: "first", anchor: anchor, now: t0)
        let second = try comments.addComment(text: "second", anchor: anchor, now: t0)
        let pins = comments.pins(element: ["Screen"], box: { _ in nil })

        XCTAssertEqual(pins.count, 2)
        XCTAssertEqual(comments.pin(at: CGPoint(x: 0, y: 0))?.id, second.id, "the last drawn wins")
        XCTAssertEqual(comments.pin(at: CGPoint(x: 0, y: 0), radius: 0)?.id, second.id)
        XCTAssertNil(comments.pin(at: CGPoint(x: 40, y: 0)))
        XCTAssertNotEqual(first.id, second.id)
    }

    // MARK: - The canvas

    func testCanvasBuildsTheAnchorOfAClick() throws {
        let canvas = try canvas(
            """
            element "Screen" {
                item Frame { pos 20 -40; width 100; height 50 }
            }
            """)
        var got: (text: String, anchor: Comments.Anchor)?
        canvas.onAddComment = { got = (text: $0, anchor: $1) }

        canvas.addComment(text: "  near the corner\n", at: Scene.Point(x: 50, y: 60))
        let (text, anchor) = try XCTUnwrap(got)
        XCTAssertEqual(text, "near the corner")
        XCTAssertEqual(anchor.element, ["Screen"])
        XCTAssertEqual(anchor.node, "Frame")
        XCTAssertEqual(try XCTUnwrap(anchor.point).x, 30, accuracy: 1e-6)
        XCTAssertEqual(try XCTUnwrap(anchor.point).y, 20, accuracy: 1e-6)

        got = nil
        canvas.addComment(text: "empty space", at: Scene.Point(x: 400, y: 300))
        let (_, empty) = try XCTUnwrap(got)
        XCTAssertEqual(empty.element, ["Screen"])
        XCTAssertEqual(empty.node, "")
        XCTAssertEqual(try XCTUnwrap(empty.point).x, 400, accuracy: 1e-6)
        XCTAssertEqual(try XCTUnwrap(empty.point).y, 300, accuracy: 1e-6)
    }

    func testCanvasPinHitTestAndCallback() throws {
        let canvas = try canvas(
            """
            element "Screen" {
                item Frame { pos 0 0; width 100; height 50 }
            }
            """)
        let pins = [
            CommentPin(id: "a", number: 1, point: CGPoint(x: 10, y: 10), detached: false, resolved: false),
            CommentPin(id: "b", number: 2, point: CGPoint(x: 10, y: 10), detached: true, resolved: false),
        ]
        canvas.setCommentPins(pins)

        let hit = try XCTUnwrap(canvas.pin(at: canvas.pinCenter(pins[1])))
        XCTAssertEqual(hit.id, "b", "the hit test is the same list in draw order, topmost first")
        XCTAssertNil(canvas.pin(at: Scene.Point(x: 300, y: 300)))

        var clicked: String?
        canvas.onPinClick = { clicked = $0 }
        canvas.onPinClick?(hit.id)
        XCTAssertEqual(clicked, "b")
    }

    // MARK: - The thread

    func testThreadTextShowsTheRepliesAndTheState() throws {
        let comments = CommentsController(key: "path:/a.tenn", configDir: try configDir())
        let anchor = Comments.Anchor(element: ["Screen"], node: "Frame")
        let added = try comments.addComment(text: "first line\nsecond line", anchor: anchor, now: t0)

        var lines = comments.threadText(id: added.id, now: t0.addingTimeInterval(3600))
        XCTAssertTrue(lines[0].hasPrefix("#1  user  "), lines[0])
        XCTAssertEqual(lines[1], "Screen / Frame")
        XCTAssertEqual(lines[2], "first line\nsecond line")

        try comments.reply(id: added.id, text: "answered", now: t0.addingTimeInterval(60))
        lines = comments.threadText(id: added.id, now: t0.addingTimeInterval(3600))
        XCTAssertTrue(lines.contains("answered"))
        XCTAssertTrue(lines.contains { $0.hasPrefix("user  ") }, "a reply line holds its author and time")

        try comments.resolve(id: added.id, text: "fixed", now: t0.addingTimeInterval(120))
        lines = comments.threadText(id: added.id, now: t0.addingTimeInterval(3600))
        XCTAssertTrue(lines[0].hasSuffix("(resolved)"), lines[0])
        XCTAssertTrue(lines.contains("fixed"))

        try comments.reopen(id: added.id, now: t0.addingTimeInterval(180))
        XCTAssertFalse(comments.threadText(id: added.id)[0].contains("(resolved)"))

        try comments.delete(id: added.id)
        XCTAssertTrue(comments.threadText(id: added.id).isEmpty)
    }

    // MARK: - Comment mode

    /// A key event as AppKit delivers it to the first responder.
    private func keyEvent(
        _ characters: String, code: UInt16, modifiers: NSEvent.ModifierFlags = []
    ) -> NSEvent {
        NSEvent.keyEvent(
            with: .keyDown, location: .zero, modifierFlags: modifiers, timestamp: 0, windowNumber: 0,
            context: nil, characters: characters, charactersIgnoringModifiers: characters,
            isARepeat: false, keyCode: code)!
    }

    /// A mouse event at a point of the canvas, in window coordinates.
    private func mouseEvent(
        _ type: NSEvent.EventType, at p: NSPoint, in view: NSView, clicks: Int = 1
    ) throws -> NSEvent {
        try XCTUnwrap(
            NSEvent.mouseEvent(
                with: type, location: view.convert(p, to: nil), modifierFlags: [], timestamp: 0,
                windowNumber: view.window?.windowNumber ?? 0, context: nil, eventNumber: 1,
                clickCount: clicks, pressure: 1))
    }

    /// A scene document on text in a scratch workspace, closed by the teardown of the base case.
    private func openDocument(_ text: String) throws -> Document {
        let url = try scratch("tenniarb-comments-doc").appendingPathComponent("a.tenn")
        try text.write(to: url, atomically: true, encoding: .utf8)
        return try openTestDocument(url)
    }

    /// The canvas over a document, so a drag writes where the test can read it.
    private func canvas(_ document: Document) throws -> SceneCanvasView {
        let view = SceneCanvasView(frame: .zero)
        view.document = document
        view.show(document.sceneTree)
        let screen = try XCTUnwrap(view.screen)
        view.frame = NSRect(origin: .zero, size: SceneCanvasView.contentSize(screen))
        return view
    }

    func testCKeyTogglesTheCommentModeAndEscLeavesIt() throws {
        let canvas = try canvas(
            """
            element "Screen" {
                item Frame { pos 0 0; width 100; height 50 }
            }
            """)
        XCTAssertFalse(canvas.commentMode)
        XCTAssertNil(canvas.message)

        canvas.keyDown(with: keyEvent("c", code: 8))
        XCTAssertTrue(canvas.commentMode)
        XCTAssertEqual(canvas.message, "Comment mode: click to place a comment, Esc to leave")

        canvas.keyDown(with: keyEvent("\u{1b}", code: 53))
        XCTAssertFalse(canvas.commentMode)
        XCTAssertNil(canvas.message, "leaving the mode clears its banner")

        canvas.keyDown(with: keyEvent("c", code: 8))
        XCTAssertTrue(canvas.commentMode)
        canvas.keyDown(with: keyEvent("c", code: 8))
        XCTAssertFalse(canvas.commentMode, "the same key leaves the mode")
    }

    func testAClickInCommentModePlacesACommentAndKeepsTheSelection() throws {
        let canvas = try canvas(
            """
            element "Screen" {
                item Frame { pos 20 -40; width 100; height 50 }
            }
            """)
        canvas.select([0])
        var added: [(text: String, anchor: Comments.Anchor)] = []
        canvas.onAddComment = { added.append((text: $0, anchor: $1)) }
        canvas.commentPrompt = { $0("too tight") }
        canvas.setCommentMode(true)

        let point = canvas.viewPoint(Scene.Point(x: 50, y: 60))
        canvas.mouseDown(with: try mouseEvent(.leftMouseDown, at: point, in: canvas))
        canvas.mouseUp(with: try mouseEvent(.leftMouseUp, at: point, in: canvas))

        XCTAssertEqual(added.count, 1)
        let (text, anchor) = try XCTUnwrap(added.first)
        XCTAssertEqual(text, "too tight")
        XCTAssertEqual(anchor.element, ["Screen"])
        XCTAssertEqual(anchor.node, "Frame")
        XCTAssertEqual(try XCTUnwrap(anchor.point).x, 30, accuracy: 1e-6)
        XCTAssertEqual(try XCTUnwrap(anchor.point).y, 20, accuracy: 1e-6)
        XCTAssertEqual(canvas.selection, [0], "a comment click leaves the selection")

        canvas.setCommentMode(false)
        canvas.select(nil)
        canvas.mouseDown(with: try mouseEvent(.leftMouseDown, at: point, in: canvas))
        canvas.mouseUp(with: try mouseEvent(.leftMouseUp, at: point, in: canvas))
        XCTAssertEqual(canvas.selection, [0], "with the mode off the same click selects")
        XCTAssertEqual(added.count, 1, "a plain click adds no comment")
    }

    func testADragInCommentModeLeavesTheItemWhereItWas() throws {
        let document = try openDocument(
            """
            element "Screen" {
                item Frame { pos 20 -40; width 100; height 50; item Leaf { width 10; height 10 } }
            }
            """)
        let canvas = try canvas(document)
        canvas.select([0])
        canvas.commentPrompt = { $0("") }  // the click takes the mode path and writes nothing
        canvas.setCommentMode(true)

        let from = canvas.viewPoint(Scene.Point(x: 50, y: 60))
        let to = canvas.viewPoint(Scene.Point(x: 150, y: 120))
        canvas.mouseDown(with: try mouseEvent(.leftMouseDown, at: from, in: canvas))
        canvas.mouseDragged(with: try mouseEvent(.leftMouseDragged, at: to, in: canvas))
        canvas.mouseUp(with: try mouseEvent(.leftMouseUp, at: to, in: canvas))

        let placed = try XCTUnwrap(canvas.placed(at: [0]))
        XCTAssertEqual(placed.x, 20)
        XCTAssertEqual(placed.y, 40)
        XCTAssertTrue(document.sceneText?.contains("pos 20 -40") ?? false, document.sceneText ?? "")

        canvas.setCommentMode(false)
        canvas.mouseDown(with: try mouseEvent(.leftMouseDown, at: from, in: canvas))
        canvas.mouseDragged(with: try mouseEvent(.leftMouseDragged, at: to, in: canvas))
        canvas.mouseUp(with: try mouseEvent(.leftMouseUp, at: to, in: canvas))
        XCTAssertEqual(
            try XCTUnwrap(canvas.placed(at: [0])).x, 120, "leaving the mode restores the drag")
    }

    func testAClickOnAPinInCommentModeOpensTheThread() throws {
        let canvas = try canvas(
            """
            element "Screen" {
                item Frame { pos 0 0; width 100; height 50 }
            }
            """)
        let pin = CommentPin(id: "a", number: 1, point: CGPoint(x: 10, y: 10), detached: false, resolved: false)
        canvas.setCommentPins([pin])
        var opened: String?
        var added = 0
        canvas.onPinClick = { opened = $0 }
        canvas.onAddComment = { _, _ in added += 1 }
        canvas.commentPrompt = { $0("note") }
        canvas.setCommentMode(true)

        let at = canvas.viewPoint(canvas.pinCenter(pin))
        canvas.mouseDown(with: try mouseEvent(.leftMouseDown, at: at, in: canvas))

        XCTAssertEqual(opened, "a")
        XCTAssertEqual(added, 0, "a pin wins over placing a comment")
    }

    func testShowingAnotherElementLeavesTheCommentMode() throws {
        let canvas = try canvas(
            """
            element "First" {
                item A { width 10; height 10 }
            }
            element "Second" {
                item B { width 10; height 10 }
            }
            """)
        canvas.setCommentMode(true)
        XCTAssertTrue(canvas.commentMode)
        XCTAssertEqual(canvas.message, "Comment mode: click to place a comment, Esc to leave")

        canvas.showElement([1])
        XCTAssertFalse(canvas.commentMode)
        XCTAssertNil(canvas.message)
        XCTAssertEqual(canvas.screenNode?.name, "Second")
    }

    // MARK: - The list

    func testListRowsShowTheNumberTheStartAndThePlace() throws {
        let comments = CommentsController(key: "path:/a.tenn", configDir: try configDir())
        let node = try comments.addComment(
            text: "  too tight\nsecond line",
            anchor: Comments.Anchor(element: ["Screen"], node: "Frame.Badge"), now: t0)
        let loose = try comments.addComment(
            text: "near the corner", anchor: Comments.Anchor(element: ["Screen"], node: ""), now: t0)
        try comments.resolve(id: loose.id, now: t0.addingTimeInterval(60))

        let now = t0.addingTimeInterval(3600)
        let rows = comments.listRows(now: now)
        XCTAssertEqual(rows.map { $0.id }, [node.id, loose.id], "the rows keep the order of the document")
        XCTAssertEqual(rows.map { $0.number }, [1, 2])
        XCTAssertEqual(rows.map { $0.resolved }, [false, true])
        XCTAssertTrue(rows[0].label.hasPrefix("1  "), rows[0].label)
        XCTAssertTrue(rows[0].label.contains(CommentsController.whenText(node.created, now: now)), rows[0].label)
        XCTAssertTrue(rows[0].label.contains("user: too tight"), rows[0].label)
        XCTAssertFalse(rows[0].label.contains("second line"), "a row keeps the first line of the text")
        XCTAssertTrue(rows[0].label.contains("Screen / Frame.Badge"), rows[0].label)
        XCTAssertTrue(rows[1].label.contains("near the corner"), rows[1].label)
        XCTAssertTrue(rows[1].label.hasSuffix("(resolved)"), rows[1].label)
        XCTAssertFalse(rows[1].label.contains(" / "), "a comment without a node names no node")
    }

    func testJumpTargetNamesThePlaceOfAComment() throws {
        let comments = CommentsController(key: "path:/a.tenn", configDir: try configDir())
        let node = try comments.addComment(
            text: "node",
            anchor: Comments.Anchor(element: ["Screen"], node: "Frame", point: Comments.Point(x: 3, y: 4)),
            now: t0)
        let loose = try comments.addComment(
            text: "loose",
            anchor: Comments.Anchor(element: ["Screen"], node: "", point: Comments.Point(x: 7, y: 8)), now: t0)
        let bare = try comments.addComment(
            text: "bare", anchor: Comments.Anchor(element: ["Screen"], node: ""), now: t0)

        let target = try XCTUnwrap(comments.jumpTarget(id: node.id))
        XCTAssertEqual(target.element, ["Screen"])
        XCTAssertEqual(target.node, "Frame")
        XCTAssertEqual(target.point, Comments.Point(x: 3, y: 4))

        let withoutNode = try XCTUnwrap(comments.jumpTarget(id: loose.id))
        XCTAssertEqual(withoutNode.element, ["Screen"])
        XCTAssertEqual(withoutNode.node, "")
        XCTAssertEqual(withoutNode.point, Comments.Point(x: 7, y: 8))

        let noPoint = try XCTUnwrap(comments.jumpTarget(id: bare.id))
        XCTAssertNil(noPoint.point, "a comment placed on the element names no point")
        XCTAssertNil(comments.jumpTarget(id: "no-such-comment"))
    }

    func testShowResolvedBringsTheResolvedPinsBack() throws {
        let comments = CommentsController(key: "path:/a.tenn", configDir: try configDir())
        let element = ["Screen"]
        let open = try comments.addComment(
            text: "open", anchor: Comments.Anchor(element: element, node: "", point: Comments.Point(x: 0, y: 0)),
            now: t0)
        let done = try comments.addComment(
            text: "done", anchor: Comments.Anchor(element: element, node: "", point: Comments.Point(x: 20, y: 20)),
            now: t0)
        try comments.resolve(id: done.id, now: t0.addingTimeInterval(60))
        var reloads = 0
        comments.onChange = { reloads += 1 }

        let shown = comments.pins(element: element, box: { _ in nil }, showResolved: comments.showResolved)
        XCTAssertEqual(shown.map { $0.id }, [open.id], "resolved comments are absent until asked for")
        XCTAssertNil(comments.pin(at: CGPoint(x: 20, y: 20)))

        comments.setShowResolved(true)
        XCTAssertTrue(comments.showResolved)
        XCTAssertEqual(reloads, 1, "the canvas is asked to place the pins again")
        let all = comments.pins(element: element, box: { _ in nil }, showResolved: comments.showResolved)
        XCTAssertEqual(all.map { $0.id }, [open.id, done.id])
        XCTAssertEqual(all.map { $0.resolved }, [false, true])
        XCTAssertEqual(comments.pin(at: CGPoint(x: 20, y: 20))?.id, done.id, "a resolved pin is hit-testable")

        let canvas = try canvas(
            """
            element "Screen" {
                item Frame { width 100; height 50 }
            }
            """)
        canvas.setCommentPins(all)
        let pin = try XCTUnwrap(canvas.pin(at: canvas.pinCenter(try XCTUnwrap(all.last))))
        XCTAssertEqual(pin.id, done.id)
        XCTAssertTrue(pin.resolved, "the canvas draws the pin of a resolved thread faded")

        comments.setShowResolved(false)
        XCTAssertEqual(reloads, 2)
        XCTAssertEqual(
            comments.pins(element: element, box: { _ in nil }, showResolved: comments.showResolved).map { $0.id },
            [open.id])
    }

    func testJumpShowsTheOtherElementAndSelectsItsNode() throws {
        let document = try openDocument(
            """
            element "First" {
                item A { auto-layout abs; width 10; height 10 }
            }
            element "Second" {
                item B { auto-layout abs; pos 20 -10; width 10; height 10 }
            }
            """)
        let canvas = try canvas(document)
        let tree = try XCTUnwrap(document.sceneTree)
        let second = try XCTUnwrap(tree.main?.elems.last)
        let b = try XCTUnwrap(CommentPaths.node(at: "B", from: second))
        XCTAssertEqual(CommentPaths.elementPath(of: second), ["Second"])

        let place = try XCTUnwrap(LinkOpen.target(in: document, element: ["Second"], node: "B"))
        XCTAssertEqual(place.elementPath, [1], "the element names the second row of the file")
        XCTAssertEqual(place.nodeOffset, b.range.start)

        canvas.showElement([1])
        canvas.selectSource(at: b.range.start)
        XCTAssertEqual(canvas.screenNode?.name, "Second", "the jump shows the element of the comment")
        XCTAssertEqual(canvas.selection, [0], "the node of the comment is selected")
    }
}
