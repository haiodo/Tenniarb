//
//  ScenePaintTests.swift
//  TenniarbTests
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.  You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//  WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//  License for the specific language governing permissions and limitations under
//  the License.
//

import XCTest

@testable import Tenniarb

/// The scene canvas paints the values the layout produced: a box of a fixture keeps its fill and the
/// text of a leaf leaves its pixels on the bitmap.
@MainActor
final class ScenePaintTests: SceneDocumentTestCase {

    private let repo = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent()

    /// A scratch workspace: the fixtures are copied, never opened in place.
    private func workspace(_ fixtures: [String]) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-paint-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        for fixture in fixtures {
            let name = (fixture as NSString).lastPathComponent
            try FileManager.default.copyItem(at: repo.appendingPathComponent(fixture), to: root.appendingPathComponent(name))
        }
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root
    }

    /// The canvas of the first element of a fixture, at the size of its content.
    private func canvas(_ fixture: String) throws -> SceneCanvasView {
        let name = (fixture as NSString).lastPathComponent
        let root = try workspace([fixture])
        let tree = try SceneLoader(root: root).load(name)
        let view = SceneCanvasView(frame: .zero)
        view.show(tree)
        let screen = try XCTUnwrap(view.screen, "\(name) does not build: \(tree.diagnostics)")
        view.frame = NSRect(origin: .zero, size: SceneCanvasView.contentSize(screen))
        return view
    }

    /// Renders the view offscreen, as the window would, and keeps the pixels.
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

    private func assertColor(_ got: NSColor, _ r: Double, _ g: Double, _ b: Double, _ message: String) {
        XCTAssertEqual(got.redComponent, r, accuracy: 0.08, message)
        XCTAssertEqual(got.greenComponent, g, accuracy: 0.08, message)
        XCTAssertEqual(got.blueComponent, b, accuracy: 0.08, message)
    }

    /// The number of pixels in a rect of the view that are not one colour.
    private func countOther(_ bitmap: (rep: NSBitmapImageRep, scale: CGFloat), _ rect: NSRect, _ color: NSColor)
        -> Int
    {
        var n = 0
        let step = 2.0
        var y = rect.minY
        while y < rect.maxY {
            var x = rect.minX
            while x < rect.maxX {
                if let c = try? self.color(bitmap, x, y),
                    abs(c.redComponent - color.redComponent) > 0.1 || abs(c.greenComponent - color.greenComponent) > 0.1
                        || abs(c.blueComponent - color.blueComponent) > 0.1
                {
                    n += 1
                }
                x += step
            }
            y += step
        }
        return n
    }

    // MARK: - Paint

    func testFilledBoxKeepsTheFixtureColor() throws {
        // padding.tenn: Plain holds a white box of 200 by 60 at the origin of the element.
        let view = try canvas("web/fixtures/scene/padding.tenn")
        let bitmap = try bitmap(view)
        let pad = SceneCanvasView.padding
        let inside = try color(bitmap, pad + 20, pad + 10)
        assertColor(inside, 1, 1, 1, "the fill of Plain")
        let background = try color(bitmap, 2, 2)
        XCTAssertGreaterThan(
            countOther(bitmap, NSRect(x: 0, y: 0, width: view.bounds.width, height: view.bounds.height), background), 100,
            "the bitmap is blank")
    }

    func testTextLeavesPixelsInsideItsLeaf() throws {
        // scene-text.tenn: Stage.LeafTitle holds "Titled" in #eeeeee; LeafName next to it holds no text.
        let view = try canvas("web/fixtures/scene/scene-text.tenn")
        let bitmap = try bitmap(view)
        let pad = SceneCanvasView.padding
        let fill = try color(bitmap, pad + 30, pad + 30)
        assertColor(fill, 0.93, 0.93, 0.93, "the fill of LeafName")
        // The title sits in the middle of the 120 by 40 leaf; the glyphs must show against its fill.
        let textArea = NSRect(x: pad + 172, y: pad + 26, width: 96, height: 28)
        let leafFill = try color(bitmap, pad + 170, pad + 22)
        XCTAssertGreaterThan(countOther(bitmap, textArea, leafFill), 5, "the title drew nothing")
    }

    func testBodyTextAndAFigureAreNotBlank() throws {
        // scene-text.tenn: Box holds no title but a #ddeeff fill, LeafBody holds a title and a body.
        let view = try canvas("web/fixtures/scene/scene-text.tenn")
        let bitmap = try bitmap(view)
        let pad = SceneCanvasView.padding
        let box = try color(bitmap, pad + 30, pad + 120)
        assertColor(box, 0.87, 0.93, 1, "the fill of Box")
        let body = try color(bitmap, pad + 250, pad + 255)
        assertColor(body, 0.93, 0.93, 0.93, "the fill of LeafBody")
    }

    func testAPictureOfTheFileIsPainted() throws {
        // A scene that keeps its picture in a file next to it: the canvas decodes the pixels of the box.
        let root = try workspace(["web/fixtures/scene/padding.tenn"])
        let sheet = try XCTUnwrap(
            NSBitmapImageRep(
                bitmapDataPlanes: nil, pixelsWide: 2, pixelsHigh: 2, bitsPerSample: 8, samplesPerPixel: 4,
                hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0))
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: sheet)
        NSColor.red.setFill()
        NSRect(x: 0, y: 0, width: 2, height: 2).fill()
        NSGraphicsContext.restoreGraphicsState()
        try XCTUnwrap(sheet.representation(using: .png, properties: [:]))
            .write(to: root.appendingPathComponent("pic.png"))

        let text = "element \"E\" { item Pic { pos 0 0; width 40; height 40; image \"pic.png\" } }\n"
        try text.write(to: root.appendingPathComponent("pic.tenn"), atomically: true, encoding: .utf8)
        let tree = try SceneLoader(root: root).load("pic.tenn")
        let view = SceneCanvasView(frame: .zero)
        view.show(tree)
        let screen = try XCTUnwrap(view.screen, "\(tree.diagnostics)")
        view.frame = NSRect(origin: .zero, size: SceneCanvasView.contentSize(screen))

        let bitmap = try bitmap(view)
        let pad = SceneCanvasView.padding
        let middle = try color(bitmap, pad + 20, pad + 20)
        XCTAssertGreaterThan(middle.redComponent, 0.8, "the picture of the file is not painted")
        XCTAssertLessThan(middle.greenComponent, 0.3)
    }

    // MARK: - Window

    func testASceneDocumentShowsTheCanvas() throws {
        let root = try workspace(["web/fixtures/scene/auto-layout-tour.tenn", "web/fixtures/Example.tenn"])
        let scene = root.appendingPathComponent("auto-layout-tour.tenn")
        let document = try openTestDocument(scene)
        document.makeWindowControllers()

        let vc = try XCTUnwrap(document.vc)
        _ = vc.view  // the editor is built with the view
        let editor = try XCTUnwrap(vc.editorViewController)
        XCTAssertFalse(editor.sceneCanvas.isHidden, "a scene document draws on the scene canvas")
        XCTAssertTrue(editor.sceneView.isHidden, "the classic canvas is not in the way")
        XCTAssertNotNil(editor.sceneCanvas.canvas.screen, "the canvas holds the built screen")

        let classic = try openTestDocument(root.appendingPathComponent("Example.tenn"))
        classic.makeWindowControllers()
        let cvc = try XCTUnwrap(classic.vc)
        _ = cvc.view
        let classicEditor = try XCTUnwrap(cvc.editorViewController)
        XCTAssertTrue(classicEditor.sceneCanvas.isHidden, "a classic document keeps the classic canvas")
        XCTAssertFalse(classicEditor.sceneView.isHidden)
    }
}
