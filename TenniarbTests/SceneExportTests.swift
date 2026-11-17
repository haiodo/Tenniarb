//
//  SceneExportTests.swift
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

import CoreGraphics
import Foundation
import XCTest

@testable import Tenniarb

@MainActor
final class SceneExportTests: SceneDocumentTestCase {

    private let repo = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent()

    // MARK: - Fixtures

    /// A scratch workspace: the fixtures are copied, never opened in place.
    private func workspace() throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-export-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    private func copy(_ fixture: String, to root: URL) throws -> URL {
        let url = root.appendingPathComponent((fixture as NSString).lastPathComponent)
        try FileManager.default.copyItem(at: repo.appendingPathComponent(fixture), to: url)
        return url
    }

    /// A document and a canvas of the first screen it shows, at 100%.
    private func scene(_ url: URL) throws -> (Document, SceneCanvasView) {
        let document = try openTestDocument(url)
        let canvas = SceneCanvasView(frame: .zero)
        canvas.document = document
        canvas.show(document.sceneTree)
        let screen = try XCTUnwrap(canvas.screen, "the fixture must build")
        canvas.frame = NSRect(origin: .zero, size: SceneCanvasView.contentSize(screen))
        return (document, canvas)
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

    private func bitmap(_ url: URL) throws -> NSBitmapImageRep {
        let data = try Data(contentsOf: url)
        return try XCTUnwrap(NSBitmapImageRep(data: data), "\(url.lastPathComponent) must be a PNG")
    }

    /// Whether anything was drawn at all: an empty PNG passes a size check just as well.
    private func hasVisiblePixels(_ rep: NSBitmapImageRep) -> Bool {
        for y in stride(from: 0, to: rep.pixelsHigh, by: 2) {
            for x in stride(from: 0, to: rep.pixelsWide, by: 2) {
                if let color = rep.colorAt(x: x, y: y), color.alphaComponent > 0 {
                    return true
                }
            }
        }
        return false
    }

    // MARK: - Selection

    /// The library screen: its first element is the mount, the screen comes next.
    private func libraryScreen() throws -> (Document, SceneCanvasView, URL) {
        let root = try workspace()
        _ = try copy("web/fixtures/scene/design-kit.tenn", to: root)
        let url = try copy("web/fixtures/scene/library-screen.tenn", to: root)
        let (document, canvas) = try scene(url)
        let tree = try XCTUnwrap(document.sceneTree)
        let index = try XCTUnwrap(tree.main?.elems.firstIndex { $0.name == "Inbox" }, "the fixture shows Inbox")
        canvas.showElement([index])
        return (document, canvas, root)
    }

    func testSelectionPNGIsTheItemBoxAtEveryScale() throws {
        let (document, canvas, root) = try libraryScreen()

        // A title-only item draws no fill, so the corners of its box stay empty.
        let itemPath = try XCTUnwrap(
            path(in: canvas) { $0.prop("title") != nil && $0.prop("color") == nil && $0.w >= 20 && $0.h >= 10 })
        canvas.select(itemPath)
        let pl = try XCTUnwrap(canvas.placed(at: itemPath))
        let exporter = SceneExport(document: document, canvas: canvas)
        XCTAssertEqual(exporter.selectionFileName, "Title.png")

        for scale in [1, 2] {
            let url = root.appendingPathComponent("selection-\(scale).png")
            XCTAssertTrue(exporter.exportSelectionPNG(to: url, scale: scale))
            let rep = try bitmap(url)
            XCTAssertEqual(rep.pixelsWide, Int(ceil(pl.w * Double(scale))), "\(scale)x width")
            XCTAssertEqual(rep.pixelsHigh, Int(ceil(pl.h * Double(scale))), "\(scale)x height")
            XCTAssertTrue(hasVisiblePixels(rep), "the item must be drawn, not an empty image")
            let corner = try XCTUnwrap(rep.colorAt(x: 0, y: 0))
            XCTAssertEqual(corner.alphaComponent, 0, accuracy: 0.01, "the background is transparent")
        }
    }

    func testWholeScreenIsExportedWhenNothingIsSelected() throws {
        let (document, canvas, root) = try libraryScreen()
        let screen = try XCTUnwrap(canvas.screen)
        XCTAssertNil(canvas.selection)

        let exporter = SceneExport(document: document, canvas: canvas)
        XCTAssertEqual(exporter.selectionFileName, "Inbox.png")

        let url = root.appendingPathComponent("screen.png")
        XCTAssertTrue(exporter.exportSelectionPNG(to: url, scale: 1))
        let rep = try bitmap(url)
        XCTAssertEqual(rep.pixelsWide, Int(ceil(screen.w)))
        XCTAssertEqual(rep.pixelsHigh, Int(ceil(screen.h)))
        XCTAssertGreaterThan(rep.pixelsWide * rep.pixelsHigh, 0)
        XCTAssertTrue(hasVisiblePixels(rep))
    }

    // MARK: - Pages

    /// Two elements, one nested in the other: the scene engine draws them, so the document needs a scene.
    private let nestedScene = """
        element "A" {
            item frame {
                auto-layout column
                width 40
                height 30
                item red {
                    width 40
                    height 30
                    color #ff0000
                }
            }
            element "B" {
                item frame {
                    auto-layout column
                    width 20
                    height 20
                    item green {
                        width 20
                        height 20
                        color #00ff00
                    }
                }
            }
        }
        """

    private let classicTwoElements = """
        element "A" {
            item alpha {
                pos 10 20
                color red
            }
        }
        element "B" {
            item beta {
                pos 10 20
            }
        }
        """

    private func document(_ text: String, in root: URL) throws -> Document {
        let url = root.appendingPathComponent("pages.tenn")
        try text.write(to: url, atomically: true, encoding: .utf8)
        return try openTestDocument(url)
    }

    func testPagesPNGWritesOneFilePerSceneElement() throws {
        let root = try workspace()
        let document = try document(nestedScene, in: root)
        XCTAssertNotNil(document.sceneTree, "the fixture is a scene")

        let out = root.appendingPathComponent("png", isDirectory: true)
        try FileManager.default.createDirectory(at: out, withIntermediateDirectories: true)
        let result = SceneExport(document: document, canvas: nil).exportPagesPNG(to: out)

        XCTAssertEqual(result.total, 2)
        XCTAssertEqual(result.written, 2)
        XCTAssertTrue(result.skipped.isEmpty, "skipped: \(result.skipped)")
        for name in ["01-A.png", "02-B.png"] {
            let rep = try bitmap(out.appendingPathComponent(name))
            XCTAssertGreaterThan(rep.pixelsWide, 0, name)
            XCTAssertGreaterThan(rep.pixelsHigh, 0, name)
            XCTAssertTrue(hasVisiblePixels(rep), name)
        }
    }

    func testPagesPDFHasOnePagePerSceneElement() throws {
        let root = try workspace()
        let document = try document(nestedScene, in: root)
        let url = root.appendingPathComponent("pages.pdf")

        let result = SceneExport(document: document, canvas: nil).exportPagesPDF(to: url)
        XCTAssertEqual(result.written, 2)
        XCTAssertTrue(result.skipped.isEmpty, "skipped: \(result.skipped)")

        let pdf = try XCTUnwrap(CGPDFDocument(url as CFURL))
        XCTAssertEqual(pdf.numberOfPages, 2)
        // Each page is the content of its element with the canvas padding: A is 40x30, B is 20x20.
        let first = try XCTUnwrap(pdf.page(at: 1)).getBoxRect(.mediaBox)
        XCTAssertEqual(first.width, 70)
        XCTAssertEqual(first.height, 60)
        let second = try XCTUnwrap(pdf.page(at: 2)).getBoxRect(.mediaBox)
        XCTAssertEqual(second.width, 50)
        XCTAssertEqual(second.height, 50)
    }

    func testClassicPagesAreWrittenOnePerElement() throws {
        let root = try workspace()
        let document = try document(classicTwoElements, in: root)
        XCTAssertNil(document.sceneTree, "a diagram with plain items is classic")

        let out = root.appendingPathComponent("png", isDirectory: true)
        try FileManager.default.createDirectory(at: out, withIntermediateDirectories: true)
        let result = SceneExport(document: document, canvas: nil).exportPagesPNG(to: out)

        XCTAssertEqual(result.total, 2)
        XCTAssertEqual(result.written, 2)
        XCTAssertTrue(result.skipped.isEmpty, "skipped: \(result.skipped)")
        for name in ["01-A.png", "02-B.png"] {
            let rep = try bitmap(out.appendingPathComponent(name))
            XCTAssertGreaterThan(rep.pixelsWide, 0, name)
            XCTAssertGreaterThan(rep.pixelsHigh, 0, name)
            XCTAssertTrue(hasVisiblePixels(rep), name)
        }
    }

    // MARK: - Names

    func testPageNamesFollowTheGoRules() {
        XCTAssertEqual(SceneExport.pageName("Design team: A/B"), "Design_team__A_B")
        XCTAssertEqual(SceneExport.pageName(" .hidden. "), "hidden")
        XCTAssertEqual(SceneExport.pageName("   "), "Untitled")
        XCTAssertEqual(SceneExport.pageFileName(1, 12, "x", ".png"), "01-x.png")
        XCTAssertEqual(SceneExport.pageFileName(1, 100, "x", ".png"), "001-x.png")
        XCTAssertEqual(SceneExport.selectionFileName("Selection"), "Selection.png")
        XCTAssertEqual(SceneExport.selectionFileName("a/b:c"), "a_b_c.png")
        XCTAssertEqual(SceneExport.selectionFileName(" . "), "Untitled.png")
        XCTAssertEqual(SceneExport.svgFileName("Inbox"), "Inbox.svg")
        XCTAssertEqual(SceneExport.clampScale(0), SceneExport.defaultScale)
        XCTAssertEqual(SceneExport.clampScale(-1), SceneExport.minScale)
        XCTAssertEqual(SceneExport.clampScale(9), SceneExport.maxScale)
    }

    // MARK: - SVG

    /// The width and height of the document root, the first attributes of the svg tag.
    private func dimensions(_ svg: String) throws -> (width: Double, height: Double) {
        func value(_ name: String) throws -> Double {
            let range = try XCTUnwrap(svg.range(of: "\(name)=\""), "no \(name) in \(svg.prefix(160))")
            let rest = svg[range.upperBound...]
            let end = try XCTUnwrap(rest.firstIndex(of: "\""), "unterminated \(name)")
            return try XCTUnwrap(Double(rest[..<end]), "\(name) is not a number")
        }
        return (try value("width"), try value("height"))
    }

    func testSceneSVGIsWellFormedAndDrawsTheScreen() throws {
        let (_, canvas, _) = try libraryScreen()
        let screen = try XCTUnwrap(canvas.screen)

        let svg = SceneSVG.document(screen)
        let parser = XMLParser(data: Data(svg.utf8))
        XCTAssertTrue(parser.parse(), parser.parserError?.localizedDescription ?? "the SVG must parse")
        let size = try dimensions(svg)
        XCTAssertEqual(size.width, ceil(screen.w))
        XCTAssertEqual(size.height, ceil(screen.h))
        XCTAssertTrue(svg.contains("<rect") || svg.contains("<path"), "the screen has boxes")
        XCTAssertTrue(svg.contains("<text"), "the screen has titles")
        XCTAssertTrue(svg.contains("Invites"), "a title of the Inbox screen is in the document")
    }

    func testButtonIconIsDrawnAsAPath() throws {
        let root = try workspace()
        let url = try copy("web/fixtures/scene/button-styles.tenn", to: root)
        let (_, canvas) = try scene(url)
        let screen = try XCTUnwrap(canvas.screen)
        XCTAssertEqual(screen.name, "Button")

        let svg = SceneSVG.document(screen)
        XCTAssertTrue(svg.contains("<path"), "the button and its icon are paths")
        XCTAssertTrue(svg.contains("M8 3v10M3 8h10"), "the icon path of the fixture is written")
        XCTAssertTrue(svg.contains("fill=\"rgb("), "the button is filled")
        XCTAssertTrue(svg.contains("<text"), "the label of the button is drawn")
    }

    func testRelativeArcPathIsWrittenAsItIs() throws {
        let root = try workspace()
        let document = try document(
            """
            element "Arc" {
                item frame {
                    auto-layout column
                    item curve {
                        width 24
                        height 24
                        path "M2 2a8 8 0 1 0 16 0"
                        border-color #000000
                    }
                }
            }
            """, in: root)
        let tree = try XCTUnwrap(document.sceneTree)
        let node = try XCTUnwrap(tree.main?.elems.first)
        let screen = try XCTUnwrap(tree.buildElement(node, measurer: Scene.TextMeasurer(), eval: sceneEvalVars))

        let svg = SceneSVG.document(screen)
        XCTAssertTrue(svg.contains("a8 8 0 1 0"), "the relative arc is kept in the path")
        let parser = XMLParser(data: Data(svg.utf8))
        XCTAssertTrue(parser.parse(), parser.parserError?.localizedDescription ?? "the SVG must parse")
    }

    func testSVGEscapesTheTitle() throws {
        let root = try workspace()
        let document = try document(
            """
            element "Esc" {
                item frame {
                    auto-layout column
                    item t { width 220; height 40; title "A <&\\" B"; font-size 16 }
                }
            }
            """, in: root)
        let tree = try XCTUnwrap(document.sceneTree)
        let node = try XCTUnwrap(tree.main?.elems.first)
        let screen = try XCTUnwrap(tree.buildElement(node, measurer: Scene.TextMeasurer(), eval: sceneEvalVars))

        let svg = SceneSVG.document(screen)
        XCTAssertTrue(svg.contains("A &lt;&amp;&quot; B"), "the title is escaped")
        XCTAssertFalse(svg.contains("A <&\" B"), "the raw title is not in the document")
        let parser = XMLParser(data: Data(svg.utf8))
        XCTAssertTrue(parser.parse(), parser.parserError?.localizedDescription ?? "the SVG must parse")
    }

    func testExportSVGWritesTheElementFile() throws {
        let (document, canvas, root) = try libraryScreen()
        let exporter = SceneExport(document: document, canvas: canvas)
        XCTAssertEqual(exporter.svgFileName, "Inbox.svg")

        let name = try XCTUnwrap(exporter.svgFileName)
        let url = root.appendingPathComponent(name)
        try exporter.exportSVG(to: url)
        let svg = try String(contentsOf: url, encoding: .utf8)
        XCTAssertTrue(svg.hasPrefix("<svg"), "the written file is an SVG document")
        XCTAssertTrue(svg.contains("xmlns=\"http://www.w3.org/2000/svg\""))
        XCTAssertGreaterThan(svg.utf8.count, 0)
    }

    func testExportSVGRefusesAClassicDocument() throws {
        let root = try workspace()
        let document = try document(classicTwoElements, in: root)
        let exporter = SceneExport(document: document, canvas: nil)
        XCTAssertNil(exporter.svgFileName)
        XCTAssertThrowsError(try exporter.exportSVG(to: root.appendingPathComponent("classic.svg")))
    }

    /// The root size of the SVG go/cmd/tenn-render writes for the fixture: gfx and this writer lay out the
    /// markup differently, so only the document size is comparable.
    private func goDimensions(fixture: String, element: String) throws -> (width: Double, height: Double) {
        let bin = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenn-render-\(UUID().uuidString)")
        addTeardownBlock { try? FileManager.default.removeItem(at: bin) }

        let candidates = ["/opt/homebrew/bin/go", "/usr/local/go/bin/go", "/usr/local/bin/go", "/usr/bin/go"]
        guard let go = candidates.first(where: { FileManager.default.isExecutableFile(atPath: $0) }) else {
            throw XCTSkip("the Go toolchain is not installed")
        }
        let build = Process()
        build.executableURL = URL(fileURLWithPath: go)
        build.arguments = ["build", "-o", bin.path, "./cmd/tenn-render"]
        build.currentDirectoryURL = repo.appendingPathComponent("go")
        do {
            try build.run()
        } catch {
            throw XCTSkip("the Go toolchain is not available: \(error.localizedDescription)")
        }
        build.waitUntilExit()
        guard build.terminationStatus == 0 else {
            XCTFail("go build ./cmd/tenn-render failed with \(build.terminationStatus)")
            throw XCTSkip("the renderer is not built")
        }

        let render = Process()
        render.executableURL = bin
        render.arguments = [
            "-root", repo.appendingPathComponent("web/fixtures/scene").path, fixture, element,
        ]
        let pipe = Pipe()
        render.standardOutput = pipe
        render.standardError = Pipe()
        do {
            try render.run()
        } catch {
            throw XCTSkip("the renderer cannot start: \(error.localizedDescription)")
        }
        render.waitUntilExit()
        guard render.terminationStatus == 0 else {
            XCTFail("tenn-render \(fixture) \(element) failed with \(render.terminationStatus)")
            throw XCTSkip("the fixture is not rendered")
        }
        let svg = String(decoding: pipe.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
        return try dimensions(svg)
    }

    func testSVGDimensionsMatchTheGoRenderer() throws {
        let (_, canvas, _) = try libraryScreen()
        let screen = try XCTUnwrap(canvas.screen)
        let ours = try dimensions(SceneSVG.document(screen))
        let go = try goDimensions(fixture: "library-screen.tenn", element: "Inbox")

        XCTAssertEqual(
            ours.width, go.width, "width: Go writes \(go.width), Swift writes \(ours.width)")
        XCTAssertEqual(
            ours.height, go.height, "height: Go writes \(go.height), Swift writes \(ours.height)")
    }

}
