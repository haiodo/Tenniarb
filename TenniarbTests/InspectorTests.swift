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

import Tenn
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

    /// Writes the library file a document mounts.
    @discardableResult
    private func library(_ text: String, in root: URL) throws -> URL {
        let url = root.appendingPathComponent("kit.tenn")
        try text.write(to: url, atomically: true, encoding: .utf8)
        return url
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

    // MARK: - Parameters

    /// The Parameters section of a selected use: a field for a plain parameter, a popup for a one-of group of
    /// the component's styles, each writing one command in the block of the use.
    func testTheParametersOfAUseWriteTheBlockOfTheUse() throws {
        let root = try workspace()
        try library(
            """
            element Button {
                export let label = "Button"
                export let tone = primary
                styles {
                    primary { color #6b3ad3; text-color #ffffff }
                    secondary { color #ffffff; text-color #172547 }
                    tone one-of primary secondary
                }
                item Btn { width 100; height 40; title ${label}; use-style ${tone} }
            }
            """, in: root)
        let document = try document(
            """
            element Screen {
                use kit.Button as button
                item Plain { width 20; height 20 }
            }

            element kit --from-file "kit.tenn"
            """, in: root)
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.kind == .use }))
        let inspector = inspector(document, canvas)

        let sections = Set(inspector.rows.map(\.section))
        XCTAssertTrue(sections.contains("Parameters"), "\(sections)")
        XCTAssertEqual(inspector.row("label")?.kind, .text)
        XCTAssertEqual(inspector.row("label")?.text, "Button", "the declaration's default")
        XCTAssertEqual(inspector.row("tone")?.kind, .popup)
        XCTAssertEqual(inspector.row("tone")?.choices, ["primary", "secondary"])
        XCTAssertEqual(inspector.row("tone")?.text, "primary", "the default the use does not override")

        inspector.setChoice("secondary", for: "tone")
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("tone secondary"), document.sceneText ?? "")
        XCTAssertEqual(inspector.row("tone")?.text, "secondary")

        inspector.setText("Hi", for: "label")
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains(#"label "Hi""#), document.sceneText ?? "")
        XCTAssertEqual(inspector.row("label")?.text, "Hi")

        // A test body is one event for the undo manager: the clear gets a group of its own to be undone alone.
        document.undoManager?.beginUndoGrouping()
        inspector.setText("", for: "label")
        document.undoManager?.endUndoGrouping()
        XCTAssertFalse(try XCTUnwrap(document.sceneText).contains("label"), "an empty field drops the command")
        XCTAssertEqual(inspector.row("label")?.text, "Button", "the default is back")

        document.undoManager?.undoNestedGroup()
        canvas.show(document.sceneTree)
        inspector.refresh()
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains(#"label "Hi""#), "undo restores the value")
        XCTAssertEqual(inspector.row("label")?.text, "Hi")

        // A part of the instance shows the parameters of the use around it; a write goes to that use.
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Btn" }))
        XCTAssertNotNil(canvas.part(at: canvas.selection))
        XCTAssertEqual(inspector.row("label")?.text, "Hi")
        XCTAssertEqual(inspector.row("tone")?.text, "secondary")
        inspector.setChoice("primary", for: "tone")
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("tone primary"), document.sceneText ?? "")
        XCTAssertEqual(inspector.row("tone")?.text, "primary")

        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Plain" }))
        XCTAssertFalse(inspector.rows.contains { $0.section == "Parameters" }, "a plain item has no parameters")
        XCTAssertFalse(inspector.rows.contains { $0.section == "Styles" }, "a plain item names no style")
    }

    /// An any-of parameter: a check box per style of the group, written as the list of the styles it keeps.
    func testAnAnyOfParameterWritesTheStylesItKeeps() throws {
        let root = try workspace()
        try library(
            """
            element Badge {
                export let decor = outlined
                styles {
                    outlined { border-color #1725471a }
                    rounded { corner-radius 8 }
                    decor any-of outlined rounded
                }
                item Face { width 60; height 20 }
            }
            """, in: root)
        let document = try document(
            """
            element Screen {
                use kit.Badge as badge
            }

            element kit --from-file "kit.tenn"
            """, in: root)
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.kind == .use }))
        let inspector = inspector(document, canvas)

        XCTAssertEqual(inspector.row("decor")?.kind, .checks)
        XCTAssertEqual(inspector.row("decor")?.choices, ["outlined", "rounded"])
        XCTAssertEqual(inspector.row("decor")?.text, "outlined", "the default in effect")

        inspector.setCheck("rounded", on: true, for: "decor")
        XCTAssertTrue(
            try XCTUnwrap(document.sceneText).contains("decor outlined rounded"), document.sceneText ?? "")
        XCTAssertEqual(inspector.row("decor")?.text, "outlined rounded")

        inspector.setCheck("rounded", on: false, for: "decor")
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("decor outlined"), document.sceneText ?? "")
        XCTAssertFalse(
            try XCTUnwrap(document.sceneText).contains("decor outlined rounded"), "the drop writes the rest")
        XCTAssertEqual(inspector.row("decor")?.text, "outlined")
    }

    // MARK: - Styles

    /// The Styles section: a one-of group takes the place of the default it replaces, an any-of group adds and
    /// drops its style, and the mounted library keeps its text.
    func testTheStylesOfANodeWriteItsUseStyle() throws {
        let root = try workspace()
        let libraryURL = try library("element Anchors { item Dot { width 2; height 2 } }", in: root)
        let libraryBefore = try Data(contentsOf: libraryURL)
        let document = try document(
            """
            element Screen {
                styles {
                    primary { color #6b3ad3 }
                    secondary { color #ffffff }
                    other { border-color #1725471a }
                    look one-of primary secondary
                    accents any-of other
                }
                item Box { width 100; height 40; use-style "primary other" }
                item Split { width 60; height 20; use-style primary; use-style other }
                item Plain { width 40; height 20 }
            }

            element kit --from-file "kit.tenn"
            """, in: root)
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Box" }))
        let inspector = inspector(document, canvas)

        let sections = Set(inspector.rows.map(\.section))
        XCTAssertTrue(sections.contains("Styles"), "\(sections)")
        XCTAssertEqual(inspector.row("look")?.kind, .popup)
        XCTAssertEqual(inspector.row("look")?.choices, ["primary", "secondary"])
        XCTAssertEqual(inspector.row("look")?.text, "primary")
        XCTAssertEqual(inspector.row("accents")?.kind, .checks)
        XCTAssertEqual(inspector.row("accents")?.text, "other")

        inspector.setChoice("secondary", for: "look")
        XCTAssertTrue(
            try XCTUnwrap(document.sceneText).contains("use-style secondary other"),
            "the choice takes the place of the default: \(document.sceneText ?? "")")

        inspector.setCheck("other", on: false, for: "accents")
        XCTAssertTrue(
            try XCTUnwrap(document.sceneText).contains("use-style secondary"), document.sceneText ?? "")
        XCTAssertFalse(try XCTUnwrap(document.sceneText).contains("use-style secondary other"))
        XCTAssertEqual(inspector.row("accents")?.text, "")

        inspector.setCheck("other", on: true, for: "accents")
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("use-style secondary other"))
        XCTAssertEqual(inspector.row("accents")?.text, "other")
        XCTAssertEqual(try Data(contentsOf: libraryURL), libraryBefore, "the library is not touched")

        // A list split in two use-style commands: the style it drops goes by name, the other command stays.
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Split" }))
        XCTAssertEqual(inspector.row("look")?.text, "primary")
        inspector.setChoice("secondary", for: "look")
        XCTAssertTrue(
            try XCTUnwrap(document.sceneText).contains("use-style -primary secondary other"),
            document.sceneText ?? "")

        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Plain" }))
        XCTAssertFalse(inspector.rows.contains { $0.section == "Styles" }, "a node that names no style")
    }

    /// A part of an instance shows the styles of its component and writes only the difference into the use.
    func testTheStyleOfAPartIsWrittenInTheUse() throws {
        let root = try workspace()
        let libraryURL = try library(
            """
            element Panel {
                styles {
                    primary { color #6b3ad3 }
                    secondary { color #ffffff }
                    look one-of primary secondary
                }
                item Frame {
                    width 200; height 100
                    item Badge { width 100; height 40; use-style primary }
                }
            }
            """, in: root)
        let libraryBefore = try Data(contentsOf: libraryURL)
        let document = try document(
            """
            element Screen {
                use kit.Panel as panel
            }

            element kit --from-file "kit.tenn"
            """, in: root)
        let canvas = try canvas(document)
        let part = try XCTUnwrap(path(in: canvas) { $0.src?.key == "Badge" })
        XCTAssertNotNil(canvas.part(at: part), "a part of the instance")
        canvas.select(part)
        let inspector = inspector(document, canvas)

        XCTAssertEqual(inspector.row("look")?.kind, .popup)
        XCTAssertEqual(inspector.row("look")?.text, "primary", "the default of the component")

        inspector.setChoice("secondary", for: "look")
        XCTAssertTrue(
            try XCTUnwrap(document.sceneText).contains("at Frame.Badge { use-style -primary secondary }"),
            document.sceneText ?? "")
        XCTAssertEqual(inspector.row("look")?.text, "secondary")
        XCTAssertEqual(try Data(contentsOf: libraryURL), libraryBefore, "the library is not touched")
    }

    // MARK: - Animation

    /// An item that plays an animation shows its title, a Play switch and a Frame slider: the switch pauses
    /// the canvas and the slider stops at a frame, neither writes a command.
    func testTheAnimationOfAnItemPlaysAndScrubsWithoutWriting() throws {
        let text = """
            element Screen {
                width 200; height 100
                animation spin {
                    from { rotate 0 }
                    to { rotate 360 }
                }
                item Spinner { auto-layout abs; pos 20 -20; width 40; height 40; color #ff0000; animation spin 1s linear infinite }
                item Plain { auto-layout abs; pos 100 -20; width 40; height 40; color #00ff00 }
            }
            """
        let document = try document(text)
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Spinner" }))
        let inspector = inspector(document, canvas)

        XCTAssertTrue(canvas.hasAnimations)
        let sections = Set(inspector.rows.map(\.section))
        XCTAssertTrue(sections.contains("Animation"), "\(sections)")
        XCTAssertEqual(inspector.row("Animation")?.kind, .info)
        XCTAssertEqual(inspector.row("Animation")?.text, "spin, 1s linear")
        XCTAssertEqual(inspector.row("Play")?.kind, .toggle)
        XCTAssertEqual(inspector.row("Play")?.text, "yes", "the canvas plays by default")
        XCTAssertEqual(inspector.row("Frame")?.kind, .slider)

        inspector.toggle("Play")
        XCTAssertFalse(canvas.animationsPlaying, "the switch pauses the canvas")
        XCTAssertEqual(inspector.row("Play")?.text, "off")

        inspector.scrub(50)
        XCTAssertFalse(canvas.animationsPlaying, "the scrub stops the playback")
        let half = try XCTUnwrap(canvas.animationFrames.values.first)
        XCTAssertEqual(half.rotate, 180, accuracy: 0.5, "half of the turn")
        XCTAssertEqual(try XCTUnwrap(document.sceneText), text, "Play and Frame write nothing")

        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Plain" }))
        XCTAssertFalse(inspector.rows.contains { $0.section == "Animation" }, "an item that plays none has no rows")
        XCTAssertNil(inspector.row("Play"))
    }

    /// The title of the section shows whole seconds as s, anything else as ms, as Go's animTitle.
    func testTheAnimationTitleShowsSecondsOrMilliseconds() throws {
        let document = try document(
            """
            element Screen {
                animation long { from { opacity 1 } to { opacity 0 } }
                animation short { from { opacity 1 } to { opacity 0 } }
                item A { auto-layout abs; pos 0 0; width 10; height 10; animation long 2s linear infinite }
                item B { auto-layout abs; pos 20 0; width 10; height 10; animation short 600ms ease-out infinite }
            }
            """)
        let canvas = try canvas(document)
        let inspector = inspector(document, canvas)

        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "A" }))
        XCTAssertEqual(inspector.row("Animation")?.text, "long, 2s linear")
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "B" }))
        XCTAssertEqual(inspector.row("Animation")?.text, "short, 600ms ease-out")
    }

    // MARK: - Shadows

    /// Runs a write as its own undo group; the body of a test is one event for the undo manager otherwise.
    private func oneUndo(_ document: Document, _ work: () -> Void) {
        document.undoManager?.beginUndoGrouping()
        work()
        document.undoManager?.endUndoGrouping()
    }

    /// A node that writes one shadow shows the custom state; a preset replaces every layer as one undo
    /// step, and none drops them.
    func testAShadowPresetReplacesTheLayersOfTheNode() throws {
        let document = try document(
            """
            element Screen {
                item Card {
                    auto-layout abs
                    width 100
                    height 40
                    shadow 0 -8 16 #00000008
                }
            }

            """)
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Card" }))
        let inspector = inspector(document, canvas)

        XCTAssertEqual(inspector.row("Shadow")?.kind, .popup)
        XCTAssertEqual(inspector.row("Shadow")?.choices, ["none", "soft", "floating", "custom"])
        XCTAssertEqual(inspector.row("Shadow")?.text, "custom", "layers written by hand are no preset")
        XCTAssertEqual(inspector.row("Shadow 1 Color")?.text, "#00000008")

        oneUndo(document) { inspector.setChoice("soft", for: "Shadow") }
        let soft = try XCTUnwrap(document.sceneText)
        XCTAssertTrue(soft.contains("shadow 0 -4 12 #00000014"), soft)
        XCTAssertTrue(soft.contains("shadow 0 -14 32 #00000010"), soft)
        XCTAssertFalse(soft.contains("#00000008"), "the old layer is gone: \(soft)")
        XCTAssertEqual(inspector.row("Shadow")?.text, "soft")
        XCTAssertEqual(inspector.row("Shadow 2 Blur")?.text, "32", "the list shows both layers")

        document.undoManager?.undoNestedGroup()
        canvas.show(document.sceneTree)
        inspector.refresh()
        XCTAssertEqual(inspector.row("Shadow")?.text, "custom", "one undo brings the layer back")
        XCTAssertEqual(inspector.row("Shadow 1 Blur")?.text, "16")
        XCTAssertFalse(inspector.rows.contains { $0.label == "Shadow 2 Blur" })

        oneUndo(document) { inspector.setChoice("none", for: "Shadow") }
        XCTAssertEqual(inspector.row("Shadow")?.text, "none")
        XCTAssertFalse(try XCTUnwrap(document.sceneText).contains("shadow"), "none removes every layer")

        document.undoManager?.undoNestedGroup()
        canvas.show(document.sceneTree)
        inspector.refresh()
        XCTAssertEqual(inspector.row("Shadow")?.text, "custom")
        XCTAssertEqual(inspector.row("Shadow 1 Blur")?.text, "16")
    }

    /// A node that writes no shadow shows none; a preset writes its layers as one undo step.
    func testANodeWithoutShadowsShowsNoneAndAPresetWritesItsLayers() throws {
        let document = try document(
            """
            element Screen {
                item Card {
                    auto-layout abs
                    width 100
                    height 40
                }
            }

            """)
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Card" }))
        let inspector = inspector(document, canvas)

        XCTAssertEqual(inspector.row("Shadow")?.text, "none")
        XCTAssertNil(inspector.row("Shadow 1 X"), "no layer to list")
        XCTAssertEqual(inspector.row("+ Shadow")?.kind, .button, "the list still adds a layer")

        oneUndo(document) { inspector.setChoice("floating", for: "Shadow") }
        let text = try XCTUnwrap(document.sceneText)
        XCTAssertTrue(text.contains("shadow 0 -12 20 #00000012"), text)
        XCTAssertTrue(text.contains("shadow 0 -32 64 #0000000f"), text)
        XCTAssertEqual(inspector.row("Shadow")?.text, "floating")
        XCTAssertEqual(inspector.row("Shadow 2 Y")?.text, "-32")

        document.undoManager?.undoNestedGroup()
        canvas.show(document.sceneTree)
        inspector.refresh()
        XCTAssertEqual(inspector.row("Shadow")?.text, "none")
        XCTAssertFalse(try XCTUnwrap(document.sceneText).contains("shadow"))
    }

    /// The list reads one row per own layer and rewrites the whole list on every change; each change is
    /// one undo step and leaves exactly those commands in the file.
    func testTheShadowListEditsTheWholeListInOneUndoStep() throws {
        let text = """
            element Screen {
                item Card {
                    auto-layout abs
                    width 100
                    height 40
                    shadow 0 -2 4 #00000010
                    shadow 0 -8 16 #00000008
                }
            }

            """
        let document = try document(text)
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Card" }))
        let inspector = inspector(document, canvas)
        let view = inspector.createView()
        view.frame = NSRect(x: 0, y: 0, width: InspectorController.width, height: 900)
        view.layoutSubtreeIfNeeded()

        XCTAssertEqual(inspector.row("Shadow 1 X")?.text, "0")
        XCTAssertEqual(inspector.row("Shadow 1 Y")?.text, "-2")
        XCTAssertEqual(inspector.row("Shadow 1 Blur")?.text, "4")
        XCTAssertEqual(inspector.row("Shadow 1 Color")?.text, "#00000010")
        XCTAssertEqual(inspector.row("Shadow 2 Blur")?.text, "16")

        oneUndo(document) { inspector.setText("20", for: "Shadow 2 Blur") }
        let blurred = text.replacingOccurrences(
            of: "shadow 0 -8 16 #00000008", with: "shadow 0 -8 20 #00000008")
        XCTAssertEqual(try XCTUnwrap(document.sceneText), blurred, "the whole list is written again")
        XCTAssertEqual(inspector.row("Shadow 1 Blur")?.text, "4", "the other layer keeps its value")

        document.undoManager?.undoNestedGroup()
        canvas.show(document.sceneTree)
        inspector.refresh()
        XCTAssertEqual(try XCTUnwrap(document.sceneText), text, "one undo for the layer change")

        oneUndo(document) { inspector.addShadow() }
        let added = text.replacingOccurrences(
            of: "shadow 0 -8 16 #00000008\n",
            with: "shadow 0 -8 16 #00000008\n        shadow 0 -2 4 #00000020\n")
        XCTAssertEqual(try XCTUnwrap(document.sceneText), added, "the added layer comes last")
        XCTAssertEqual(inspector.row("Shadow 3 Color")?.text, "#00000020")

        document.undoManager?.undoNestedGroup()
        canvas.show(document.sceneTree)
        inspector.refresh()
        XCTAssertEqual(try XCTUnwrap(document.sceneText), text, "one undo for the added layer")

        oneUndo(document) { inspector.removeShadow(at: 0) }
        let dropped = text.replacingOccurrences(of: "        shadow 0 -2 4 #00000010\n", with: "")
        XCTAssertEqual(try XCTUnwrap(document.sceneText), dropped, "the dropped layer takes its line with it")
        XCTAssertEqual(inspector.row("Shadow 1 Y")?.text, "-8", "the list starts with what is left")

        document.undoManager?.undoNestedGroup()
        canvas.show(document.sceneTree)
        inspector.refresh()
        XCTAssertEqual(try XCTUnwrap(document.sceneText), text, "one undo for the dropped layer")
    }

    /// A part of an instance has no node of its own: the preset and the list are absent, as Go's own-node
    /// rule.
    func testAPartOfAnInstanceHasNoShadowList() throws {
        let root = try workspace()
        try library(
            """
            element Panel {
                item Frame {
                    width 200; height 100
                    item Badge { width 100; height 40; shadow 0 -2 4 #00000010 }
                }
            }
            """, in: root)
        let document = try document(
            """
            element Screen {
                use kit.Panel as panel
            }

            element kit --from-file "kit.tenn"
            """, in: root)
        let canvas = try canvas(document)
        let part = try XCTUnwrap(path(in: canvas) { $0.src?.key == "Badge" })
        XCTAssertNotNil(canvas.part(at: part), "a part of the instance")
        canvas.select(part)
        let inspector = inspector(document, canvas)

        let labels = inspector.rows.map(\.label)
        XCTAssertFalse(labels.contains("Shadow"), "no preset for a part: \(labels)")
        XCTAssertFalse(labels.contains { $0.hasPrefix("Shadow ") || $0 == "+ Shadow" }, "no list: \(labels)")
        XCTAssertTrue(Set(inspector.rows.map(\.section)).contains("Effects"), "the other Effects rows stay")
    }

    /// The shadow list refuses a node written in a mounted file, as the single-edit rows do.
    func testTheShadowEditOfAMountedFileIsRefused() throws {
        let root = try workspace()
        let library = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let libraryBefore = try Data(contentsOf: library)
        let document = try openTestDocument(copy("web/fixtures/scene/library-screen.tenn", to: root))
        let before = try XCTUnwrap(document.sceneText)
        let canvas = try canvas(document)
        let mounted = try XCTUnwrap(
            path(in: canvas) { $0.src?.pos.file == "design-kit.tenn" && $0.src?.kind == .item })
        canvas.select(mounted)
        let inspector = inspector(document, canvas)

        inspector.addShadow()

        XCTAssertEqual(canvas.message, "design-kit.tenn is a mounted file: open it to edit it")
        XCTAssertEqual(document.sceneText, before, "a refused edit leaves the text as it was")
        XCTAssertEqual(try Data(contentsOf: library), libraryBefore)
    }

    // MARK: - Sizes

    /// Fixed, Hug and Fill of a size row: the word of a mode, or the size the item is drawn with.
    func testTheSizeSegmentsWriteFixedHugAndFill() throws {
        let document = try document(
            """
            element Screen {
                item Card {
                    auto-layout abs
                    width 120.6
                    height 40
                }
            }

            """)
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Card" }))
        let inspector = inspector(document, canvas)
        let view = inspector.createView()
        view.frame = NSRect(x: 0, y: 0, width: InspectorController.width, height: 900)
        view.layoutSubtreeIfNeeded()

        XCTAssertEqual(inspector.row("Size W")?.segments, ["Fixed", "Hug", "Fill"])
        XCTAssertEqual(inspector.row("Size W")?.chosen, 0, "a written size behaves as fixed")

        oneUndo(document) { inspector.setSizeMode("Hug", for: "Size W") }
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("width hug"))
        XCTAssertEqual(inspector.row("Size W")?.text, "hug")
        XCTAssertEqual(inspector.row("Size W")?.chosen, 1)

        document.undoManager?.undoNestedGroup()
        canvas.show(document.sceneTree)
        inspector.refresh()
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("width 120.6"))
        XCTAssertEqual(inspector.row("Size W")?.chosen, 0, "undo selects Fixed again")

        oneUndo(document) { inspector.setSizeMode("Fill", for: "Size H") }
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("height fill"))
        XCTAssertEqual(inspector.row("Size H")?.chosen, 2)

        oneUndo(document) { inspector.setSizeMode("Fixed", for: "Size W") }
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("width 121"), "the measured width, whole")
        XCTAssertEqual(inspector.row("Size W")?.text, "121")

        document.undoManager?.undoNestedGroup()
        canvas.show(document.sceneTree)
        inspector.refresh()
        XCTAssertTrue(try XCTUnwrap(document.sceneText).contains("width 120.6"), "undo restores the number")
    }

    // MARK: - Classic documents

    /// The classic inspector of a document: no scene canvas, the item or link the canvas would select, and
    /// the element the outline names.
    private func classicInspector(
        _ document: Document, items: [DiagramItem] = [], element: Element? = nil
    ) -> InspectorController {
        let controller = InspectorController()
        controller.attach(canvas: nil, document: document)
        controller.showClassic(items: items, element: element ?? items.first?.parent)
        return controller
    }

    private func classicItem(_ name: String, _ document: Document) throws -> DiagramItem {
        let element = try XCTUnwrap(document.store?.model.elements.first)
        return try XCTUnwrap(element.items.first { $0.name == name })
    }

    private func modelText(_ document: Document) -> String {
        document.store?.model.toTennStr() ?? ""
    }

    func testAClassicItemShowsItsOwnCommandsWithoutAScene() throws {
        let document = try document(
            """
            element Screen {
                item Box {
                    pos 10 -20
                    width 120
                    height 40
                    color #ffffff
                    text-color #000000
                    font-size 12
                    border-color #cccccc
                    line-width 1
                    line-style dashed
                    corner-radius 4
                    display circle
                    title "Titled"
                }
            }
            """)
        let item = try classicItem("Box", document)
        let inspector = classicInspector(document, items: [item])
        inspector.createView().layoutSubtreeIfNeeded()

        XCTAssertEqual(inspector.title, "Item")
        XCTAssertEqual(inspector.row("Header.Name")?.kind, .text)
        XCTAssertEqual(inspector.row("Header.Name")?.text, "Box")
        XCTAssertEqual(inspector.row("Header.Kind")?.kind, .info)
        XCTAssertEqual(inspector.row("Header.Kind")?.text, "Item")
        XCTAssertEqual(inspector.row("Header.Title")?.text, "Titled")
        XCTAssertEqual(inspector.row("Position X")?.text, "10")
        XCTAssertEqual(inspector.row("Position Y")?.text, "-20")
        XCTAssertEqual(inspector.row("Size W")?.text, "120")
        XCTAssertEqual(inspector.row("Size H")?.text, "40")
        XCTAssertEqual(inspector.row("Fill.Color")?.text, "#ffffff")
        XCTAssertEqual(inspector.row("Text.Color")?.text, "#000000")
        XCTAssertEqual(inspector.row("Text.Size")?.text, "12")
        XCTAssertEqual(inspector.row("Border.Color")?.text, "#cccccc")
        XCTAssertEqual(inspector.row("Border.Style")?.text, "dashed")
        XCTAssertEqual(inspector.row("Border.Style")?.choices, ["none", "solid", "dashed", "dotted"])
        XCTAssertEqual(inspector.row("Shape.Display")?.text, "circle")
        XCTAssertEqual(inspector.row("Shape.Display")?.choices, ["rect", "no-fill", "circle", "stack", "text"])

        let sections = Set(inspector.rows.map(\.section))
        XCTAssertTrue(
            sections.isSuperset(of: ["Header", "Layout", "Fill", "Text", "Border", "Shape"]), "\(sections)")
    }

    func testAClassicLinkShowsTheLineRows() throws {
        let document = try document(
            """
            element Screen {
                item A { pos 0 0; width 10; height 10 }
                item B { pos 100 0; width 10; height 10 }
                link A B { label "goes"; color #ff0000; line-width 2; line-style dotted; display arrow }
            }
            """)
        let element = try XCTUnwrap(document.store?.model.elements.first)
        let link = try XCTUnwrap(element.items.first { $0 is LinkItem })
        let inspector = classicInspector(document, items: [link])

        XCTAssertEqual(inspector.title, "Link")
        XCTAssertEqual(inspector.row("Header.Label")?.text, "goes")
        XCTAssertEqual(inspector.row("Header.Kind")?.text, "Link")
        XCTAssertEqual(inspector.row("Line.Color")?.text, "#ff0000")
        XCTAssertEqual(inspector.row("Line.Width")?.text, "2")
        XCTAssertEqual(inspector.row("Line.Style")?.text, "dotted")
        XCTAssertEqual(inspector.row("Line.Arrows")?.text, "arrow")
        XCTAssertEqual(inspector.row("Line.Arrows")?.choices, ["solid", "arrow", "arrows", "arrow-source"])

        let sections = Set(inspector.rows.map(\.section))
        XCTAssertTrue(sections.contains("Line"), "\(sections)")
        XCTAssertFalse(sections.contains("Layout"), "a link has no box: \(sections)")
        XCTAssertFalse(sections.contains("Border"), "a link has no box border: \(sections)")
    }

    func testAClassicDocumentWithoutSelectionShowsTheElement() throws {
        let document = try document(
            """
            element Screen {
                grid 10 20
                item Box { width 100; height 40 }
            }
            """)
        let element = try XCTUnwrap(document.store?.model.elements.first)
        let inspector = InspectorController()
        inspector.attach(canvas: nil, document: document)

        XCTAssertEqual(inspector.title, "Element", "the column knows the classic document without a canvas")
        XCTAssertEqual(inspector.row("Header.Name")?.text, "Screen", element.name)
        XCTAssertEqual(inspector.row("Grid X")?.text, "10")
        XCTAssertEqual(inspector.row("Grid Y")?.text, "20")
        XCTAssertNil(inspector.row("Size W"), "no item, no box")
    }

    /// The title is the item's own command; undo puts the old command back.
    func testAClassicTitleWritesTheCommandAndUndoRestoresIt() throws {
        let document = try document(
            "element Screen { item Box { width 100; height 40; title \"Old\" } }")
        let item = try classicItem("Box", document)
        let inspector = classicInspector(document, items: [item])

        inspector.setText("Shown", for: "Header.Title")
        XCTAssertTrue(modelText(document).contains(#"title "Shown""#), modelText(document))
        XCTAssertEqual(inspector.row("Header.Title")?.text, "Shown")

        document.undoManager?.undo()
        inspector.refresh()
        XCTAssertTrue(modelText(document).contains(#"title "Old""#), modelText(document))
        XCTAssertEqual(inspector.row("Header.Title")?.text, "Old")
    }

    /// Position is not a command of the item: it lives in x and y, and the edit goes through
    /// updatePosition so the canvas and the file follow.
    func testAClassicPositionWritesThroughUpdatePosition() throws {
        let document = try document(
            "element Screen { item Box { pos 10 -20; width 100; height 40 } }")
        let item = try classicItem("Box", document)
        let inspector = classicInspector(document, items: [item])

        inspector.setText("30", for: "Position X")
        XCTAssertEqual(item.x, 30)
        XCTAssertTrue(modelText(document).contains("pos 30.0 -20.0"), modelText(document))
        XCTAssertEqual(inspector.row("Position X")?.text, "30")

        document.undoManager?.undo()
        inspector.refresh()
        XCTAssertEqual(item.x, 10, "undo puts the item back")
        XCTAssertEqual(inspector.row("Position X")?.text, "10")
    }

    func testAClassicWidthWritesAndClearingRemovesIt() throws {
        let document = try document(
            "element Screen { item Box { width 100; height 40 } }")
        let item = try classicItem("Box", document)
        let inspector = classicInspector(document, items: [item])

        inspector.setText("123", for: "Size W")
        XCTAssertTrue(modelText(document).contains("width 123"), modelText(document))
        XCTAssertEqual(inspector.row("Size W")?.text, "123")

        inspector.setText("", for: "Size W")
        XCTAssertFalse(modelText(document).contains("width"), "an empty field removes the command")
        XCTAssertEqual(inspector.row("Size W")?.text, "")
    }

    func testAClassicNameWritesThroughUpdateNameAndUndoRestoresIt() throws {
        let document = try document(
            "element Screen { item Box { width 100; height 40 } }")
        let item = try classicItem("Box", document)
        let inspector = classicInspector(document, items: [item])

        inspector.setText("Renamed", for: "Header.Name")
        XCTAssertEqual(item.name, "Renamed")
        XCTAssertTrue(modelText(document).contains(#"item "Renamed""#), modelText(document))
        XCTAssertEqual(inspector.row("Header.Name")?.text, "Renamed")

        document.undoManager?.undo()
        inspector.refresh()
        XCTAssertEqual(item.name, "Box")
        XCTAssertTrue(modelText(document).contains(#"item "Box""#), modelText(document))
    }

    func testAClassicLineStylePopupWritesAndNoneRemovesIt() throws {
        let document = try document(
            "element Screen { item Box { width 100; height 40 } }")
        let item = try classicItem("Box", document)
        let inspector = classicInspector(document, items: [item])

        XCTAssertEqual(inspector.row("Border.Style")?.text, "none", "the item writes no style")

        inspector.setChoice("dashed", for: "Border.Style")
        XCTAssertTrue(modelText(document).contains("line-style dashed"), modelText(document))
        XCTAssertEqual(inspector.row("Border.Style")?.text, "dashed")

        inspector.setChoice("none", for: "Border.Style")
        XCTAssertFalse(modelText(document).contains("line-style"), "none removes the command")
        XCTAssertEqual(inspector.row("Border.Style")?.text, "none")
    }

    /// Align and Vertical pick one word of `layout` each and keep the other axis.
    func testAClassicLayoutChoicesKeepTheOtherAxis() throws {
        let document = try document(
            "element Screen { item Box { width 100; height 40 } }")
        let item = try classicItem("Box", document)
        let inspector = classicInspector(document, items: [item])

        XCTAssertEqual(inspector.row("Text.Align")?.choices, ["none", "left", "center", "right"])
        XCTAssertEqual(inspector.row("Text.Vertical")?.choices, ["none", "top", "middle", "bottom", "fill"])

        inspector.setChoice("center", for: "Text.Align")
        XCTAssertTrue(modelText(document).contains("layout center"), modelText(document))
        XCTAssertEqual(inspector.row("Text.Align")?.text, "center")

        inspector.setChoice("middle", for: "Text.Vertical")
        XCTAssertTrue(modelText(document).contains("layout middle center"), modelText(document))
        XCTAssertEqual(inspector.row("Text.Vertical")?.text, "middle")
        XCTAssertEqual(inspector.row("Text.Align")?.text, "center", "the axis that was not chosen stays")

        inspector.setChoice("none", for: "Text.Align")
        XCTAssertTrue(modelText(document).contains("layout middle"), modelText(document))
        XCTAssertFalse(modelText(document).contains("layout center"), modelText(document))
        XCTAssertEqual(inspector.row("Text.Align")?.text, "none")
    }

    /// A scene document keeps the scene column: its Name is a read-only row and it has the scene's Layout.
    func testASceneDocumentDoesNotShowTheClassicRows() throws {
        let root = try workspace()
        let document = try openTestDocument(copy("web/fixtures/scene/scene-text.tenn", to: root))
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "LeafTitle" }))
        let inspector = inspector(document, canvas)

        XCTAssertEqual(inspector.title, "Item")
        XCTAssertEqual(inspector.row("Header.Name")?.kind, .info, "the scene name is read-only")
        XCTAssertNil(inspector.row("Header.Kind"), "no classic Kind row")
        let labels = Set(inspector.rows.map(\.label))
        XCTAssertTrue(labels.contains("Auto layout"), "the scene Layout rows stay: \(labels)")
        XCTAssertFalse(labels.contains("Grid X"), "no classic element header")

        canvas.select(nil)
        inspector.refresh()
        XCTAssertEqual(inspector.title, "Screen")
    }

    /// A model change refreshes the values; a newly added item can be selected without stale rows.
    func testTheClassicInspectorFollowsAModelChange() throws {
        let document = try document(
            "element Screen { item Box { width 100; height 40; title \"Old\" } }")
        let element = try XCTUnwrap(document.store?.model.elements.first)
        let item = try classicItem("Box", document)
        let inspector = classicInspector(document, items: [item])

        let props = item.toTennAsProps(.BlockExpr)
        props.removeNamed("title")
        props.add(TennNode.newCommand("title", TennNode.newStrNode("Changed")))
        document.store?.setProperties(
            element, item, props, undoManager: document.undoManager, refresh: {})
        inspector.refresh()
        XCTAssertEqual(inspector.row("Header.Title")?.text, "Changed", "the row reads the new tree")

        let added = DiagramItem(kind: .Item, name: "Second")
        document.store?.add(
            element, added, undoManager: document.undoManager, refresh: {})
        inspector.showClassic(items: [added], element: element)
        XCTAssertEqual(inspector.row("Header.Name")?.text, "Second")
        XCTAssertEqual(inspector.row("Header.Title")?.text, "", "no title carried over")
        XCTAssertEqual(inspector.row("Size W")?.text, "", "no stale size")
    }

    // MARK: - Classic body

    /// The body row of a selected item reads the markdown of its `body` command, with the indent
    /// prepareBodyText strips for the inline editor.
    func testAClassicBodyRowShowsTheMarkdownOfTheSelectedItem() throws {
        let root = try workspace()
        let document = try openTestDocument(copy("web/fixtures/scene/classic-body.tenn", to: root))
        let item = try classicItem("Lines", document)
        let inspector = classicInspector(document, items: [item])
        let view = inspector.createView()
        view.frame = NSRect(x: 0, y: 0, width: InspectorController.width, height: 900)
        view.layoutSubtreeIfNeeded()

        XCTAssertEqual(inspector.row("Body.Body")?.kind, .body)
        XCTAssertEqual(inspector.row("Body.Body")?.text, "line one\nline two\nline three")
        XCTAssertEqual(inspector.bodyText, "line one\nline two\nline three")
    }

    /// A body edit is one command written through the item's properties; undo restores the old text and
    /// an empty body drops the command.
    func testAClassicBodyWritesTheCommandAndEmptyRemovesIt() throws {
        let document = try document("element Screen { item Box { width 100; height 40; body \"Old\" } }")
        let item = try classicItem("Box", document)
        let inspector = classicInspector(document, items: [item])

        XCTAssertEqual(inspector.bodyText, "Old")

        oneUndo(document) { inspector.setBody("New\ntext", for: "Body") }
        XCTAssertTrue(modelText(document).contains("New"), modelText(document))
        XCTAssertTrue(modelText(document).contains("text"), modelText(document))
        XCTAssertEqual(inspector.bodyText, "New\ntext")

        document.undoManager?.undoNestedGroup()
        inspector.refresh()
        XCTAssertEqual(inspector.bodyText, "Old", "undo puts the old body back")
        XCTAssertTrue(modelText(document).contains(#"body "Old""#), modelText(document))

        oneUndo(document) { inspector.setBody("", for: "Body") }
        XCTAssertFalse(modelText(document).contains("body"), "an empty body removes the command")
        XCTAssertEqual(inspector.bodyText, "")
    }

    /// With nothing selected the body field is the description of the element; an empty text removes it.
    func testAClassicElementDescriptionIsTheBodyWhenNothingIsSelected() throws {
        let document = try document(
            "element Screen { description \"about D\"; item Box { width 100; height 40 } }")
        let inspector = classicInspector(document)

        XCTAssertEqual(inspector.title, "Element")
        XCTAssertEqual(inspector.row("Description.Body")?.kind, .body)
        XCTAssertEqual(inspector.bodyText, "about D")

        oneUndo(document) { inspector.setBody("new about", for: "Body") }
        XCTAssertTrue(modelText(document).contains(#"description "new about""#), modelText(document))
        XCTAssertEqual(inspector.bodyText, "new about")

        document.undoManager?.undoNestedGroup()
        inspector.refresh()
        XCTAssertEqual(inspector.bodyText, "about D")

        oneUndo(document) { inspector.setBody("", for: "Body") }
        XCTAssertFalse(modelText(document).contains("description"), modelText(document))
        XCTAssertEqual(inspector.bodyText, "")
    }

    // MARK: - Classic shadows

    /// A classic item that writes no shadow shows none; a preset writes its two layers as one undo step
    /// and a layer edited by hand makes the row custom.
    func testAClassicShadowPresetWritesTheLayersOfTheItem() throws {
        let document = try document("element Screen { item Card { width 100; height 40 } }")
        let item = try classicItem("Card", document)
        let inspector = classicInspector(document, items: [item])

        XCTAssertEqual(inspector.row("Shadow")?.kind, .popup)
        XCTAssertEqual(inspector.row("Shadow")?.choices, ["none", "soft", "floating", "custom"])
        XCTAssertEqual(inspector.row("Shadow")?.text, "none")
        XCTAssertNil(inspector.row("Shadow 1 X"), "no layer to list")
        XCTAssertEqual(inspector.row("+ Shadow")?.kind, .button)

        oneUndo(document) { inspector.setChoice("soft", for: "Shadow") }
        let soft = modelText(document)
        XCTAssertTrue(soft.contains("shadow 0 -4 12 #00000014"), soft)
        XCTAssertTrue(soft.contains("shadow 0 -14 32 #00000010"), soft)
        XCTAssertEqual(inspector.row("Shadow")?.text, "soft")
        XCTAssertEqual(inspector.row("Shadow 2 Blur")?.text, "32")

        document.undoManager?.undoNestedGroup()
        inspector.refresh()
        XCTAssertEqual(inspector.row("Shadow")?.text, "none")
        XCTAssertFalse(modelText(document).contains("shadow"), "one undo removes the preset")

        oneUndo(document) { inspector.setChoice("soft", for: "Shadow") }
        oneUndo(document) { inspector.setText("20", for: "Shadow 2 Blur") }
        XCTAssertTrue(modelText(document).contains("shadow 0 -14 20 #00000010"), modelText(document))
        XCTAssertEqual(inspector.row("Shadow")?.text, "custom", "an edited layer is no preset")
        XCTAssertEqual(inspector.row("Shadow 1 Blur")?.text, "12", "the other layer keeps its value")

        oneUndo(document) { inspector.setChoice("none", for: "Shadow") }
        XCTAssertEqual(inspector.row("Shadow")?.text, "none")
        XCTAssertFalse(modelText(document).contains("shadow"), "none removes every layer")
    }

    /// The classic shadow list reads X, Y, blur and colour of each layer and rewrites the whole list on
    /// every change: add, drop and a blur edit are one undo step each.
    func testTheClassicShadowListEditsTheWholeListInOneUndoStep() throws {
        let document = try document(
            """
            element Screen {
                item Card {
                    width 100; height 40
                    shadow 0 -2 4 #00000010
                    shadow 0 -8 16 #00000008
                }
            }
            """)
        let item = try classicItem("Card", document)
        let inspector = classicInspector(document, items: [item])
        inspector.createView().layoutSubtreeIfNeeded()

        XCTAssertEqual(inspector.row("Shadow")?.text, "custom", "layers written by hand")
        XCTAssertEqual(inspector.row("Shadow 1 X")?.text, "0")
        XCTAssertEqual(inspector.row("Shadow 1 Y")?.text, "-2")
        XCTAssertEqual(inspector.row("Shadow 1 Blur")?.text, "4")
        XCTAssertEqual(inspector.row("Shadow 1 Color")?.text, "#00000010")
        XCTAssertEqual(inspector.row("Shadow 1 Color")?.action, "−")
        XCTAssertEqual(inspector.row("Shadow 2 Blur")?.text, "16")

        let before = modelText(document)
        oneUndo(document) { inspector.setText("20", for: "Shadow 2 Blur") }
        let blurred = modelText(document)
        XCTAssertTrue(blurred.contains("shadow 0 -8 20 #00000008"), blurred)
        XCTAssertTrue(blurred.contains("shadow 0 -2 4 #00000010"), "the other layer stays: \(blurred)")
        XCTAssertEqual(inspector.row("Shadow 1 Blur")?.text, "4")

        document.undoManager?.undoNestedGroup()
        inspector.refresh()
        XCTAssertEqual(modelText(document), before, "one undo for the whole list")

        oneUndo(document) { inspector.addShadow() }
        XCTAssertTrue(modelText(document).contains("shadow 0 -2 4 #00000020"), modelText(document))
        XCTAssertEqual(inspector.row("Shadow 3 Color")?.text, "#00000020", "the added layer comes last")

        document.undoManager?.undoNestedGroup()
        inspector.refresh()
        XCTAssertEqual(modelText(document), before, "one undo for the added layer")

        oneUndo(document) { inspector.removeShadow(at: 0) }
        let dropped = modelText(document)
        XCTAssertFalse(dropped.contains("shadow 0 -2 4 #00000010"), dropped)
        XCTAssertTrue(dropped.contains("shadow 0 -8 16 #00000008"), dropped)
        XCTAssertEqual(inspector.row("Shadow 1 Y")?.text, "-8", "the list starts with what is left")

        document.undoManager?.undoNestedGroup()
        inspector.refresh()
        XCTAssertEqual(modelText(document), before, "one undo for the dropped layer")
    }

    // MARK: - Classic styles

    /// With nothing selected the Styles section lists the element's styles: the named ones without the
    /// item and line defaults, and the group declarations as Go's styleGroupText writes them.
    func testAClassicElementListsItsStyleNamesAndGroups() throws {
        let document = try document(
            """
            element Screen {
                styles {
                    item { color red }
                    primary {}
                    tone one-of primary
                }
                item Box { width 100; height 40 }
            }

            element Plain { item Dot { width 10; height 10 } }
            """)
        let screen = try XCTUnwrap(document.store?.model.elements.first)
        let plain = try XCTUnwrap(document.store?.model.elements.last)

        let inspector = classicInspector(document, element: screen)
        XCTAssertEqual(inspector.row("Styles.Named")?.kind, .info)
        XCTAssertEqual(inspector.row("Styles.Named")?.text, "primary", "item is the default, not a name")
        XCTAssertEqual(inspector.row("Styles.Groups")?.kind, .info)
        XCTAssertEqual(inspector.row("Styles.Groups")?.text, "tone (one-of primary)")

        inspector.showClassic(items: [], element: plain)
        XCTAssertEqual(inspector.row("Styles.Named")?.text, "None")
        XCTAssertEqual(inspector.row("Styles.Groups")?.text, "None")
    }

    /// A scene document keeps the scene column: no body field and no classic element Styles rows.
    func testASceneDocumentHasNoClassicBodyOrElementStyles() throws {
        let root = try workspace()
        let document = try openTestDocument(copy("web/fixtures/scene/scene-text.tenn", to: root))
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "LeafTitle" }))
        let inspector = inspector(document, canvas)

        XCTAssertFalse(inspector.rows.contains { $0.kind == .body }, "the scene rows carry no body field")
        XCTAssertNil(inspector.row("Named"), "no classic element Styles rows")
        XCTAssertEqual(inspector.bodyText, "")
    }

    // MARK: - Classic item and link defaults

    /// The document text as one line: the printer breaks a block across lines.
    private func flat(_ text: String) -> String {
        text.split(whereSeparator: \.isWhitespace).joined(separator: " ")
    }

    /// The text of a defaults row; those sections share captions with the item rows.
    private func defaultText(_ section: String, _ label: String, _ inspector: InspectorController) -> String? {
        inspector.rows.first { $0.section == section && $0.label == label }?.text
    }

    /// An element without a styles block shows both defaults sections, the fields empty and the popups on
    /// the word the renderer would use.
    func testAnElementWithoutStylesShowsEmptyItemAndLinkDefaults() throws {
        let document = try document("element Screen { item Box { width 100; height 40 } }")
        let inspector = classicInspector(document)

        for label in ["Fill", "Border", "Text", "Font size", "Padding"] {
            XCTAssertEqual(defaultText("Item defaults", label, inspector), "", label)
        }
        XCTAssertEqual(defaultText("Item defaults", "Display", inspector), "rect")
        XCTAssertEqual(
            inspector.rows.first { $0.section == "Item defaults" && $0.label == "Display" }?.choices,
            ["rect", "no-fill", "circle", "stack", "text"])
        XCTAssertNil(defaultText("Item defaults", "Shadow 1 X", inspector), "no default layer to list")

        for label in ["Color", "Width"] {
            XCTAssertEqual(defaultText("Link defaults", label, inspector), "", label)
        }
        XCTAssertEqual(defaultText("Link defaults", "Style", inspector), "solid")
        XCTAssertEqual(defaultText("Link defaults", "Arrows", inspector), "solid")
    }

    /// A value typed into Item defaults writes `styles { item { ... } }` of the element, and one undo takes
    /// the block it made away again.
    func testItemDefaultsWriteTheGroupAndUndoRemovesIt() throws {
        let document = try document("element Screen { item Box { width 100; height 40 } }")
        let inspector = classicInspector(document)

        oneUndo(document) { inspector.setText("red", for: "Fill") }
        XCTAssertTrue(flat(modelText(document)).contains("styles { item { color red } }"), modelText(document))
        XCTAssertEqual(defaultText("Item defaults", "Fill", inspector), "red")

        document.undoManager?.undoNestedGroup()
        inspector.refresh()
        XCTAssertFalse(modelText(document).contains("styles"), "one undo removes the block the field made")
        XCTAssertEqual(defaultText("Item defaults", "Fill", inspector), "")
    }

    /// Clearing a field drops its command and leaves the other commands of the group.
    func testClearingADefaultFieldKeepsTheRestOfTheGroup() throws {
        let document = try document(
            """
            element Screen {
                styles {
                    item {
                        color red
                        border-color #111111
                        font-size 14
                    }
                }
                item Box { width 100; height 40 }
            }
            """)
        let inspector = classicInspector(document)
        XCTAssertEqual(defaultText("Item defaults", "Border", inspector), "#111111")

        oneUndo(document) { inspector.setText("", for: "Border") }
        let text = flat(modelText(document))
        XCTAssertFalse(text.contains("border-color"), text)
        XCTAssertTrue(text.contains("styles { item { color red font-size 14 } }"), text)
        XCTAssertEqual(defaultText("Item defaults", "Border", inspector), "")
        XCTAssertEqual(defaultText("Item defaults", "Fill", inspector), "red", "the group keeps the rest")
    }

    /// The Link defaults write `styles { line { ... } }` and leave the item group alone.
    func testLinkDefaultsWriteTheLineGroupOnly() throws {
        let document = try document(
            """
            element Screen {
                styles {
                    item { color red }
                }
                item Box { width 100; height 40 }
            }
            """)
        let inspector = classicInspector(document)

        oneUndo(document) { inspector.setText("2", for: "Width") }
        oneUndo(document) { inspector.setChoice("dashed", for: "Style") }
        let text = flat(modelText(document))
        XCTAssertTrue(text.contains("line { line-width 2 line-style dashed }"), text)
        XCTAssertTrue(text.contains("item { color red }"), text)
        XCTAssertEqual(defaultText("Link defaults", "Width", inspector), "2")
        XCTAssertEqual(defaultText("Link defaults", "Style", inspector), "dashed")
        XCTAssertEqual(defaultText("Item defaults", "Fill", inspector), "red", "the item group stays")
    }

    /// A document that already writes both groups shows their values, and an edit keeps the named styles
    /// and the groups beside them.
    func testDefaultsReadBothGroupsAndKeepTheOtherStyles() throws {
        let document = try document(
            """
            element Screen {
                styles {
                    item {
                        color #ffffff
                        font-size 12
                    }
                    line {
                        color #ff0000
                        line-style dotted
                        display arrow
                    }
                    primary { color blue }
                    tone one-of primary
                }
                item Box { width 100; height 40 }
            }
            """)
        let inspector = classicInspector(document)

        XCTAssertEqual(defaultText("Item defaults", "Fill", inspector), "#ffffff")
        XCTAssertEqual(defaultText("Item defaults", "Font size", inspector), "12")
        XCTAssertEqual(defaultText("Item defaults", "Display", inspector), "rect", "the group writes no display")
        XCTAssertEqual(defaultText("Link defaults", "Color", inspector), "#ff0000")
        XCTAssertEqual(defaultText("Link defaults", "Style", inspector), "dotted")
        XCTAssertEqual(defaultText("Link defaults", "Arrows", inspector), "arrow")

        oneUndo(document) { inspector.setText("16", for: "Font size") }
        let text = flat(modelText(document))
        XCTAssertTrue(text.contains("font-size 16"), text)
        XCTAssertTrue(text.contains("primary { color blue }"), text)
        XCTAssertTrue(text.contains("tone one-of primary"), text)
        XCTAssertTrue(text.contains("line { color #ff0000 line-style dotted display arrow }"), text)
    }

    /// A scene document keeps the scene column: no classic defaults at all.
    func testASceneDocumentShowsNoClassicDefaults() throws {
        let root = try workspace()
        let document = try openTestDocument(copy("web/fixtures/scene/scene-text.tenn", to: root))
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "LeafTitle" }))
        let inspector = inspector(document, canvas)

        XCTAssertFalse(
            inspector.rows.contains { $0.section == "Item defaults" || $0.section == "Link defaults" },
            "\(Set(inspector.rows.map(\.section)))")
    }

    /// The shadow list of Item defaults writes its layers into the item group: an added layer, a blur typed
    /// into it, and the − that drops it again.
    func testItemDefaultShadowsWriteIntoTheItemGroup() throws {
        let document = try document("element Screen { item Box { width 100; height 40 } }")
        let inspector = classicInspector(document)

        oneUndo(document) { inspector.addShadow() }
        XCTAssertTrue(
            flat(modelText(document)).contains("styles { item { shadow 0 -2 4 #00000020 } }"),
            modelText(document))
        XCTAssertEqual(defaultText("Item defaults", "Shadow 1 Blur", inspector), "4")

        oneUndo(document) { inspector.setText("20", for: "Shadow 1 Blur") }
        XCTAssertTrue(flat(modelText(document)).contains("shadow 0 -2 20 #00000020"), modelText(document))
        XCTAssertEqual(defaultText("Item defaults", "Shadow 1 Blur", inspector), "20")

        oneUndo(document) { inspector.removeShadow(at: 0) }
        XCTAssertFalse(modelText(document).contains("shadow"), "none leaves the group without a layer")
    }

    // MARK: - The column

    /// The column starts at the top of its scroll view: an unflipped document view would pin a content
    /// shorter than the scroll view to the bottom edge.
    func testTheColumnStartsAtTheTopOfItsScrollView() throws {
        let document = try document(
            "element Screen { item Box { auto-layout abs; width 100; height 40; color #ffffff } }")
        let canvas = try canvas(document)
        canvas.select(try XCTUnwrap(path(in: canvas) { $0.src?.key == "Box" }))
        let inspector = inspector(document, canvas)
        let view = inspector.createView()
        view.frame = NSRect(x: 0, y: 0, width: InspectorController.width, height: 900)
        view.layoutSubtreeIfNeeded()

        let scroll = try XCTUnwrap(view.subviews.compactMap { $0 as? NSScrollView }.first)
        let content = try XCTUnwrap(scroll.documentView)
        XCTAssertTrue(content.isFlipped, "an unflipped document view sits at the bottom")

        let stack = try XCTUnwrap(content.subviews.first)
        XCTAssertEqual(stack.frame.maxY, content.bounds.maxY, accuracy: 1, "the rows begin at the top edge")
        let placed = view.convert(content.bounds, from: content)
        XCTAssertEqual(placed.maxY, view.bounds.maxY, accuracy: 1, "the column sits at the top of the pane")
        XCTAssertEqual(content.frame.width, scroll.contentView.bounds.width, "the rows keep the column width")
    }
}
