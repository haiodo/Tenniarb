//
//  ViewControllerTests.swift
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

private let diagramSource = """
    element Root {
        element Nested {
            item Inner {
                pos 0 0
            }
        }
        item First {
            pos 10 20
        }
        item Second {
            pos 200 40
        }
        link First Second {
        }
    }
    """

/// A fully loaded controller inside an offscreen window, so window-dependent
/// code paths (backing scale factor, rendering) have something real to read.
@MainActor
private func makeController(_ source: String = diagramSource) -> (ViewController, ElementModelStore, Element) {
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

    // setElementModel activates the root model on the scene; picking the diagram is a
    // separate step the outline normally performs.
    let diagram = model.elements[0]
    controller.onElementSelected(diagram)
    window.makeFirstResponder(controller.scene)
    return (controller, store, diagram)
}

@MainActor
private func items(_ element: Element) -> [DiagramItem] {
    return element.items.filter { $0.kind == .Item }
}

// MARK: - Wiring

@MainActor
class ViewControllerTests: XCTestCase {

    func testControllerBuildsItsSubviews() {
        let (controller, _, _) = makeController()

        XCTAssertNotNil(controller.scene)
        XCTAssertNotNil(controller.worldTree)
        XCTAssertNotNil(controller.textView)
        XCTAssertNotNil(controller.view.window, "The fixture puts the controller in a window")
    }

    func testSettingTheModelSelectsTheFirstDiagram() {
        let (controller, _, diagram) = makeController()

        XCTAssertNotNil(controller.elementStore)
        XCTAssertEqual(controller.selectedElement, diagram)
        XCTAssertNotNil(controller.scene.scene, "The drawing view has a scene to show")
    }

    func testControllerRegistersAsAModelListener() {
        let (controller, store, _) = makeController()
        XCTAssertTrue(store.onUpdate.contains(where: { ($0 as AnyObject) === (controller as AnyObject) }))
    }

    func testOutlineShowsTheRootElements() {
        let (controller, _, _) = makeController()
        XCTAssertGreaterThan(controller.worldTree.numberOfRows, 0, "The tree lists at least the root diagram")
    }

    func testSelectingAnElementUpdatesTheScene() {
        let (controller, _, diagram) = makeController()
        let nested = diagram.elements[0]

        controller.onElementSelected(nested)
        XCTAssertEqual(controller.selectedElement, nested)
        XCTAssertEqual(controller.scene.element, nested)
    }

    func testSelectingNothingIsHandled() {
        let (controller, _, _) = makeController()
        controller.onElementSelected(nil)
    }

    func testTextPropertiesFollowTheSelection() {
        let (controller, _, diagram) = makeController()
        controller.scene.setActiveItem(items(diagram)[0])
        controller.activeItems = [items(diagram)[0]]
        controller.updateTextProperties()

        // updateTextProperties hands the work to the main queue, so let it drain first.
        let filled = expectation(description: "properties pane filled")
        DispatchQueue.main.async { filled.fulfill() }
        wait(for: [filled], timeout: 5)

        XCTAssertFalse(controller.textView.string.isEmpty, "The properties pane shows the item source")
    }

    func testWindowTitleIsUpdated() {
        let (controller, _, _) = makeController()
        controller.updateWindowTitle()
    }

    func testZoomLabelTracksTheScene() {
        let (controller, _, _) = makeController()
        controller.updateZoomLabel(150)
    }

    func testZoomActionsRunThroughTheController() {
        let (controller, _, _) = makeController()
        let start = controller.scene.zoomLevel

        controller.zoomInAction(NSButton())
        XCTAssertGreaterThan(controller.scene.zoomLevel, start)

        controller.resetZoomAction(NSButton())
        XCTAssertEqual(controller.scene.zoomLevel, 1, accuracy: 0.001)
    }

    func testSelectAllAndSelectNoneItems() {
        let (controller, _, diagram) = makeController()

        controller.selectAllItems(NSMenuItem())
        XCTAssertEqual(Set(controller.scene.activeItems), Set(diagram.items))

        controller.selectNoneItems(NSMenuItem())
        XCTAssertTrue(controller.scene.activeItems.isEmpty)
    }

    func testSelectAllLinksPicksOnlyLinks() {
        let (controller, _, _) = makeController()
        controller.selectAllLinks(NSMenuItem())

        XCTAssertFalse(controller.scene.activeItems.isEmpty)
        XCTAssertTrue(controller.scene.activeItems.allSatisfy { $0.kind == .Link })
    }

    func testSelectAllItemsKindSkipsLinks() {
        let (controller, _, diagram) = makeController()
        controller.selectAllItemsKind(NSMenuItem())
        XCTAssertEqual(Set(controller.scene.activeItems), Set(items(diagram)))
    }

    func testAddingAFreeItemGrowsTheDiagram() {
        let (controller, _, diagram) = makeController()
        let before = diagram.items.count

        controller.addFreeItem(NSMenuItem())
        XCTAssertGreaterThan(controller.scene.element!.items.count, before, "A new item lands on the diagram")
    }

    func testAddingALinkedItemAlsoAddsALink() {
        let (controller, _, diagram) = makeController()
        controller.scene.setActiveItem(items(diagram)[0])
        let linksBefore = diagram.items.filter { $0.kind == .Link }.count

        controller.addLinkedItem(NSMenuItem())
        let linksAfter = controller.scene.element!.items.filter { $0.kind == .Link }.count
        XCTAssertGreaterThan(linksAfter, linksBefore)
    }

    func testAddElementAddsANestedDiagram() {
        let (controller, _, diagram) = makeController()
        let before = diagram.elements.count

        controller.onElementSelected(diagram)
        controller.handleAddElement()
        XCTAssertGreaterThan(diagram.elements.count, before)
    }

    func testStructureChangesAreForwardedToTheController() {
        let (controller, _, diagram) = makeController()
        controller.notifyChanges(ModelEvent(kind: .Structure, element: diagram))
        controller.notifyChanges(ModelEvent(kind: .Layout, element: diagram))
    }

    func testMenuValidationAnswersForEveryKnownAction() {
        let (controller, _, diagram) = makeController()
        controller.scene.setActiveItem(items(diagram)[0])

        for selector in [
            #selector(ViewController.selectAllItems(_:)),
            #selector(ViewController.selectNoneItems(_:)),
            #selector(ViewController.selectAllLinks(_:)),
            #selector(ViewController.editTitle(_:)),
            #selector(ViewController.addFreeItem(_:)),
        ] {
            let item = NSMenuItem()
            item.action = selector
            _ = controller.validateMenuItem(item)
        }
    }

    func testSplitViewConstraintsAreAnswered() throws {
        let (controller, _, _) = makeController()
        guard let split = controller.view.subviews.first?.subviews.first as? NSSplitView else {
            throw XCTSkip("Split view layout changed")
        }
        _ = controller.splitView(split, constrainMinCoordinate: 0, ofSubviewAt: 0)
        _ = controller.splitView(split, constrainMaxCoordinate: 900, ofSubviewAt: 0)
        _ = controller.splitView(split, canCollapseSubview: split.subviews[0])
    }

    func testShowAndHideOperationBox() {
        let (controller, _, _) = makeController()
        controller.showOperationBox()
        controller.hideOperationBox()
    }
}

// MARK: - Export

@MainActor
class ExportManagerTests: XCTestCase {

    private func exporter(_ controller: ViewController) -> ExportManager {
        let manager = ExportManager()
        manager.setViewController(controller)
        return manager
    }

    func testExportMenuListsEveryKind() {
        let (controller, _, _) = makeController()
        let menu = exporter(controller).createMenu()

        XCTAssertGreaterThan(menu.items.count, 0)
        XCTAssertTrue(menu.items.contains(where: { $0.title.contains("HTML") }))
        XCTAssertTrue(menu.items.contains(where: { $0.title.contains("PNG") }))
        XCTAssertTrue(menu.items.contains(where: { $0.title.contains("JSON") }))
    }

    func testSelectedElementIsTakenFromTheController() {
        let (controller, _, diagram) = makeController()
        XCTAssertEqual(exporter(controller).element, diagram)
    }

    func testActiveItemsAreNilWhenNothingIsSelected() {
        let (controller, _, _) = makeController()
        controller.scene.setActiveItem(nil)
        XCTAssertNil(exporter(controller).activeItems)
    }

    func testActiveItemsFollowTheSelection() {
        let (controller, _, diagram) = makeController()
        let first = items(diagram)[0]
        controller.scene.setActiveItems([first])
        controller.activeItems = [first]

        XCTAssertEqual(exporter(controller).activeItems, [first])
    }

    func testRenderImageProducesABitmap() {
        let (controller, _, _) = makeController()
        let (image, bounds) = exporter(controller).renderImage()

        XCTAssertGreaterThan(image.size.width, 0)
        XCTAssertGreaterThan(image.size.height, 0)
        XCTAssertGreaterThan(bounds.width, 0)
    }

    func testGeneratedHtmlEmbedsThePngAsBase64() {
        let (controller, _, _) = makeController()
        guard let html = exporter(controller).generateHtml() else {
            return XCTFail("Expected HTML output")
        }

        XCTAssertTrue(html.hasPrefix("<html>"))
        XCTAssertTrue(html.contains("data:image/png;base64,"))
        XCTAssertTrue(html.hasSuffix("</html>"))
    }

    func testGeneratedHtmlCarriesTheDiagramSize() {
        let (controller, _, _) = makeController()
        guard let html = exporter(controller).generateHtml() else {
            return XCTFail("Expected HTML output")
        }
        XCTAssertTrue(html.contains("width="))
        XCTAssertTrue(html.contains("height="))
    }

    func testJsonExportToClipboardRoundTrips() throws {
        let (controller, _, _) = makeController()
        exporter(controller).exportJson(false)

        guard let text = NSPasteboard.general.string(forType: .string) else {
            return XCTFail("Expected JSON on the pasteboard")
        }
        let decoded = try JSONDecoder().decode(SyncElement.self, from: Data(text.utf8))
        XCTAssertEqual(decoded.name, "Root")
    }

    func testExportWithoutAControllerIsANoOp() {
        let manager = ExportManager()
        XCTAssertNil(manager.element)
        XCTAssertNil(manager.activeItems)
        manager.exportJson(false)
    }
}
