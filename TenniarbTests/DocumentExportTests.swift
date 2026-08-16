//
//  DocumentExportTests.swift
//  TenniarbTests
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
//  See the License for the specific language governing permissions and
//  limitations under the License.

import XCTest

@testable import Tenniarb

// MARK: - Helpers

private let docSource = """
    element Root {
        item Alpha {
            pos 10 20
            color red
        }
        item Beta {
            pos 200 40
        }
        link Alpha Beta {
        }
    }
    """

private func writeTempFile(_ contents: String, name: String = "fixture.tenn") throws -> URL {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent(
        "tenniarb-tests-\(ProcessInfo.processInfo.globallyUniqueString)")
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    let url = directory.appendingPathComponent(name)
    try contents.write(to: url, atomically: true, encoding: .utf8)
    return url
}

@MainActor
private func makeController(_ source: String = docSource) -> (ViewController, ElementModelStore, Element) {
    let model = ElementModel.parseTenn(node: TennParser().parse(source))
    let store = ElementModelStore(model)

    let controller = ViewController()
    let window = NSWindow(
        contentRect: NSRect(x: 0, y: 0, width: 944, height: 764),
        styleMask: [.titled, .resizable], backing: .buffered, defer: false)
    window.contentViewController = controller
    controller.loadViewIfNeeded()
    controller.viewDidLoad()
    controller.setElementModel(elementStore: store)

    let diagram = model.elements[0]
    controller.onElementSelected(diagram)
    window.makeFirstResponder(controller.scene)
    return (controller, store, diagram)
}

// MARK: - Document

@MainActor
class DocumentTests: XCTestCase {

    func testReadingAFileBuildsTheModel() throws {
        let url = try writeTempFile(docSource)
        let document = Document()

        try document.read(from: url, ofType: "tenn")

        XCTAssertNotNil(document.store)
        XCTAssertEqual(document.store?.model.modelName, "fixture.tenn")
        XCTAssertEqual(document.store?.model.elements.first?.name, "Root")
    }

    func testReadingBrokenSourceKeepsTheStarterModel() throws {
        let url = try writeTempFile("element Root {")
        let document = Document()
        let before = document.store?.model.modelName

        try document.read(from: url, ofType: "tenn")
        XCTAssertEqual(document.store?.model.modelName, before, "A parse error leaves the previous model in place")
    }

    func testReadingAMissingFileIsSurvived() throws {
        let document = Document()
        let missing = FileManager.default.temporaryDirectory.appendingPathComponent("does-not-exist.tenn")

        try document.read(from: missing, ofType: "tenn")
        XCTAssertEqual(document.store?.model.modelName, "Unnamed", "A failed read keeps the starter model")
    }

    func testWritingProducesReadableSource() throws {
        let url = try writeTempFile(docSource)
        let document = Document()
        try document.read(from: url, ofType: "tenn")

        let out = url.deletingLastPathComponent().appendingPathComponent("out.tenn")
        try document.write(to: out, ofType: "tenn")

        let written = try String(contentsOf: out, encoding: .utf8)
        XCTAssertTrue(written.contains("Alpha"))
        XCTAssertTrue(written.contains("Beta"))
    }

    func testWriteReadRoundTripKeepsTheItems() throws {
        let url = try writeTempFile(docSource)
        let first = Document()
        try first.read(from: url, ofType: "tenn")

        let out = url.deletingLastPathComponent().appendingPathComponent("round.tenn")
        try first.write(to: out, ofType: "tenn")

        let second = Document()
        try second.read(from: out, ofType: "tenn")

        XCTAssertEqual(
            second.store?.model.elements.first?.items.count,
            first.store?.model.elements.first?.items.count)
    }

    func testANewDocumentStartsFromAStarterModel() {
        let document = Document()
        XCTAssertNotNil(document.store, "The initialiser installs a sample model")
        XCTAssertEqual(document.store?.model.modelName, "Unnamed")
        XCTAssertFalse(document.isDocumentEdited)
    }

    func testFreshlyReadDocumentIsNotEdited() throws {
        let url = try writeTempFile(docSource)
        let document = Document()
        try document.read(from: url, ofType: "tenn")

        XCTAssertFalse(document.isDocumentEdited)
    }

    func testModelChangesMarkTheDocumentEdited() throws {
        let url = try writeTempFile(docSource)
        let document = Document()
        try document.read(from: url, ofType: "tenn")

        guard let store = document.store, let diagram = store.model.elements.first else {
            return XCTFail("Expected a model")
        }
        let notified = expectation(description: "listener notified")
        store.onUpdate.append(RecordingDocumentListener { notified.fulfill() })

        store.add(diagram, DiagramItem(kind: .Item, name: "New"), undoManager: nil, refresh: {})
        wait(for: [notified], timeout: 5)

        XCTAssertTrue(document.isDocumentEdited)
    }
}

@MainActor
private final class RecordingDocumentListener: IElementModelListener {
    private let onEvent: () -> Void
    init(_ onEvent: @escaping () -> Void) { self.onEvent = onEvent }
    func notifyChanges(_ event: ModelEvent) { onEvent() }
}

// MARK: - Export, remaining paths

@MainActor
class ExportManagerRemainingTests: XCTestCase {

    private func exporter(_ controller: ViewController) -> ExportManager {
        let manager = ExportManager()
        manager.setViewController(controller)
        return manager
    }

    func testHtmlCopyPublishesBothFlavours() {
        let (controller, _, _) = makeController()
        NSPasteboard.general.clearContents()

        exporter(controller).exportHtml(false)
        XCTAssertNotNil(NSPasteboard.general.string(forType: .html))
        XCTAssertNotNil(NSPasteboard.general.string(forType: .string))
    }

    func testExportKindsCoverEveryMenuEntry() {
        let (controller, _, _) = makeController()
        let manager = exporter(controller)
        let menu = manager.createMenu()

        for item in menu.items where !item.isSeparatorItem {
            XCTAssertNotNil(ExportKind(rawValue: item.tag), "\(item.title) must map to a known export kind")
        }
    }

    func testRenderingASelectionOnlyCoversThoseItems() {
        let (controller, _, diagram) = makeController()
        let manager = exporter(controller)

        let (full, fullBounds) = manager.renderImage()

        let first = diagram.items.first(where: { $0.kind == .Item })!
        controller.scene.setActiveItems([first])
        controller.activeItems = [first]
        let (partial, partialBounds) = manager.renderImage()

        XCTAssertGreaterThan(full.size.width, 0)
        XCTAssertGreaterThan(partial.size.width, 0)
        XCTAssertLessThanOrEqual(partialBounds.width, fullBounds.width, "One item cannot be wider than the whole diagram")
    }

    func testCopyingJsonForASelection() throws {
        let (controller, _, diagram) = makeController()
        let first = diagram.items.first(where: { $0.kind == .Item })!
        controller.scene.setActiveItems([first])
        controller.activeItems = [first]

        exporter(controller).exportJson(false)
        guard let text = NSPasteboard.general.string(forType: .string) else {
            return XCTFail("Expected JSON on the pasteboard")
        }
        _ = try JSONDecoder().decode(SyncElement.self, from: Data(text.utf8))
    }
}

// MARK: - Scene, remaining paths

@MainActor
class SceneRemainingTests: XCTestCase {

    private func scene(_ source: String) -> (DrawableScene, Element) {
        let element = ElementModel.parseTenn(node: TennParser().parse(source)).elements[0]
        let context = ExecutionContext()
        context.setElement(element)
        return (DrawableScene(element, darkMode: false, executionContext: context), element)
    }

    private func drawOffscreen(_ scene: DrawableScene) {
        let context = CGContext(
            data: nil, width: 600, height: 600, bitsPerComponent: 8, bytesPerRow: 0,
            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
        NSGraphicsContext.current = NSGraphicsContext(cgContext: context, flipped: false)
        let bounds = scene.getBounds()
        scene.layout(bounds, bounds)
        scene.draw(context: context)
    }

    func testEveryDisplayKindRenders() {
        for kind in ["rect", "no-fill", "circle", "stack", "text"] {
            let (built, element) = scene(
                """
                element Root {
                    item Shape {
                        pos 0 0
                        display \(kind)
                    }
                }
                """)
            XCTAssertNotNil(built.drawables[element.items[0]], "\(kind) must produce a drawable")
            drawOffscreen(built)
        }
    }

    func testEveryLineDisplayKindRenders() {
        for kind in ["solid", "arrow", "arrows", "arrow-source"] {
            let (built, _) = scene(
                """
                element Root {
                    item A {
                        pos 0 0
                    }
                    item B {
                        pos 200 0
                    }
                    link A B {
                        display \(kind)
                    }
                }
                """)
            drawOffscreen(built)
        }
    }

    func testLineStylesRender() {
        for style in ["solid", "dashed", "dotted"] {
            let (built, _) = scene(
                """
                element Root {
                    item A {
                        pos 0 0
                    }
                    item B {
                        pos 200 0
                    }
                    link A B {
                        line-style \(style)
                    }
                }
                """)
            drawOffscreen(built)
        }
    }

    func testTitleAndMarkerAreRendered() {
        let (built, element) = scene(
            """
            element Root {
                item Card {
                    pos 0 0
                    title Header
                    marker dot
                    body %{some body}
                }
            }
            """)
        XCTAssertGreaterThan(built.drawables[element.items[0]]!.getBounds().height, 0)
        drawOffscreen(built)
    }

    func testUseStyleIsAppliedFromTheElement() {
        let (built, element) = scene(
            """
            element Root {
                styles {
                    highlight {
                        width 250
                    }
                }
                item Plain {
                    pos 0 0
                }
                item Styled {
                    pos 300 0
                    use-style highlight
                }
            }
            """)

        let plain = built.drawables[element.items.first(where: { $0.name == "Plain" })!]!
        let styled = built.drawables[element.items.first(where: { $0.name == "Styled" })!]!
        XCTAssertGreaterThan(styled.getBounds().width, plain.getBounds().width, "The named style widens the box")
    }

    func testLinkLabelIsRendered() {
        let (built, element) = scene(
            """
            element Root {
                item A {
                    pos 0 0
                }
                item B {
                    pos 300 0
                }
                link A B {
                    label edge
                }
            }
            """)
        let link = element.items.first(where: { $0.kind == .Link })!
        XCTAssertTrue(built.drawables[link] is DrawableLine)
        drawOffscreen(built)
    }

    func testSelectionBoxIsDrawn() {
        let (built, _) = scene(
            """
            element Root {
                item A {
                    pos 0 0
                }
            }
            """)
        built.selectionBox = CGRect(x: -50, y: -50, width: 200, height: 200)
        drawOffscreen(built)
    }

    func testEditingModeIsDrawn() {
        let (built, element) = scene(
            """
            element Root {
                item A {
                    pos 0 0
                }
            }
            """)
        built.updateActiveElements([element.items[0]])
        built.editingMode = true
        drawOffscreen(built)
    }

    func testUpdateLayoutReportsADirtyRegion() {
        let (built, element) = scene(
            """
            element Root {
                item A {
                    pos 0 0
                }
                item B {
                    pos 200 0
                }
            }
            """)
        let bounds = built.getBounds()
        built.layout(bounds, bounds)

        let dirty = built.updateLayout([element.items[0]: CGPoint(x: 50, y: 50)])
        XCTAssertGreaterThanOrEqual(dirty.width, 0)
    }

    func testRemoveLineToClearsThePendingLink() {
        let (built, element) = scene(
            """
            element Root {
                item A {
                    pos 0 0
                }
                item B {
                    pos 200 0
                }
            }
            """)
        _ = built.updateLineTo(element.items[0], CGPoint(x: 100, y: 0))
        built.removeLineTo()
        drawOffscreen(built)
    }

    func testDeeplyNestedElementsAreRendered() {
        let (built, _) = scene(
            """
            element Root {
                item Parent {
                    pos 0 0
                    layout auto
                }
                item Child {
                    pos 50 50
                }
            }
            """)
        drawOffscreen(built)
        XCTAssertGreaterThan(built.getBounds().width, 0)
    }
}
