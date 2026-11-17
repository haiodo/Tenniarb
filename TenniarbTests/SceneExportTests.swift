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
        XCTAssertEqual(SceneExport.clampScale(0), SceneExport.defaultScale)
        XCTAssertEqual(SceneExport.clampScale(-1), SceneExport.minScale)
        XCTAssertEqual(SceneExport.clampScale(9), SceneExport.maxScale)
    }
}
