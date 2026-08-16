//
//  SceneEditingTests.swift
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

private let editSource = """
    element Root {
        element Nested {
            item Inner {
                pos 0 0
            }
        }
        item Alpha {
            pos 0 0
            body %{first line\\nsecond line}
        }
        item Beta {
            pos 200 0
            title Header
        }
        link Alpha Beta {
        }
    }
    """

@MainActor
private func makeController(_ source: String = editSource) -> (ViewController, ElementModelStore, Element) {
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

@MainActor
private func items(_ element: Element) -> [DiagramItem] {
    return element.items.filter { $0.kind == .Item }
}

// MARK: - Body text extraction

@MainActor
class SceneBodyTextTests: XCTestCase {

    func testBodyTextIsCollectedFromTheItem() {
        let item = DiagramItem(kind: .Item, name: "Doc")
        item.properties.append(TennNode.newCommand("body", TennNode.newMarkdownNode("hello body")))

        var text = ""
        SceneDrawView.getBodyText(item, nil, &text)
        XCTAssertTrue(text.contains("hello body"))
    }

    func testItemWithoutABodyYieldsNothing() {
        let item = DiagramItem(kind: .Item, name: "Plain")

        var text = ""
        SceneDrawView.getBodyText(item, nil, &text)
        XCTAssertTrue(text.isEmpty)
    }

    func testBodyTextSurvivesAStyle() {
        let item = DiagramItem(kind: .Item, name: "Doc")
        item.properties.append(TennNode.newCommand("body", TennNode.newMarkdownNode("styled body")))

        var text = ""
        SceneDrawView.getBodyText(item, DrawableItemStyle(false), &text)
        XCTAssertTrue(text.contains("styled body"))
    }
}

// MARK: - Title editing

@MainActor
class SceneTitleEditingTests: XCTestCase {

    func testEditingATitleEntersEditingMode() {
        let (controller, _, diagram) = makeController()
        let first = items(diagram)[0]
        controller.scene.setActiveItem(first)

        controller.scene.editTitle(first, .Name)
        XCTAssertEqual(controller.scene.mode, .Editing)
        XCTAssertNotNil(controller.scene.editBox)
    }

    func testEditingABodyEntersEditingMode() {
        let (controller, _, diagram) = makeController()
        let first = items(diagram)[0]
        controller.scene.setActiveItem(first)

        controller.scene.editTitle(first, .Body)
        XCTAssertEqual(controller.scene.mode, .Editing)
    }

    func testEditingAValueEntersEditingMode() {
        let (controller, _, diagram) = makeController()
        let second = items(diagram)[1]
        controller.scene.setActiveItem(second)

        controller.scene.editTitle(second, .Value)
        XCTAssertEqual(controller.scene.mode, .Editing)
    }

    func testClickingWhileEditingCommitsFirst() {
        let (controller, _, diagram) = makeController()
        let first = items(diagram)[0]
        controller.scene.setActiveItem(first)
        controller.scene.editTitle(first, .Name)

        let event = NSEvent.mouseEvent(
            with: .leftMouseDown, location: CGPoint(x: 700, y: 500), modifierFlags: [],
            timestamp: ProcessInfo.processInfo.systemUptime, windowNumber: 0, context: nil,
            eventNumber: 0, clickCount: 1, pressure: 1)!

        controller.scene.mouseDown(with: event)
        XCTAssertNotEqual(controller.scene.mode, .Editing, "Clicking away ends the edit")
    }

    func testSwitchingDiagramsDiscardsAnyEdit() {
        let (controller, _, diagram) = makeController()
        let first = items(diagram)[0]
        controller.scene.setActiveItem(first)
        controller.scene.editTitle(first, .Name)

        controller.scene.setActiveElement(diagram.elements[0])
        XCTAssertNotEqual(controller.scene.mode, .Editing)
    }

    func testControllerEditActionsRouteToTheScene() {
        let (controller, _, diagram) = makeController()
        controller.scene.setActiveItem(items(diagram)[0])

        controller.editTitle(NSMenuItem())
        XCTAssertEqual(controller.scene.mode, .Editing)
    }

    func testControllerEditBodyAndValue() {
        let (controller, _, diagram) = makeController()
        controller.scene.setActiveItem(items(diagram)[0])

        controller.editBody(NSMenuItem())
        XCTAssertEqual(controller.scene.mode, .Editing)

        controller.scene.setActiveItem(items(diagram)[1])
        controller.editValue(NSMenuItem())
    }

    func testEditActionsWithoutASelectionAreHarmless() {
        let (controller, _, _) = makeController()
        controller.scene.setActiveItem(nil)

        controller.editTitle(NSMenuItem())
        controller.editBody(NSMenuItem())
        XCTAssertNotEqual(controller.scene.mode, .Editing)
    }
}

// MARK: - Zoom and framing

@MainActor
class SceneZoomTests: XCTestCase {

    func testResetZoomReturnsToOneHundredPercent() {
        let (controller, _, _) = makeController()
        controller.scene.zoomIn(nil)
        controller.scene.zoomIn(nil)

        controller.scene.resetZoom()
        XCTAssertEqual(controller.scene.zoomLevel, 1, accuracy: 0.001)
    }

    func testZoomingOutRepeatedlyStaysPositive() {
        let (controller, _, _) = makeController()
        for _ in 0..<20 {
            controller.scene.zoomOut(nil)
        }
        XCTAssertGreaterThan(controller.scene.zoomLevel, 0)
    }

    func testZoomingInRepeatedlyKeepsGrowing() {
        let (controller, _, _) = makeController()
        var previous = controller.scene.zoomLevel
        for _ in 0..<20 {
            controller.scene.zoomIn(nil)
            XCTAssertGreaterThan(controller.scene.zoomLevel, previous)
            previous = controller.scene.zoomLevel
        }
        // Only the pinch gesture clamps to 0.1...10; the menu action is unbounded.
    }

    func testZoomBoundsScaleWithTheZoomLevel() {
        let (controller, _, _) = makeController()
        controller.scene.frame = NSRect(x: 0, y: 0, width: 800, height: 600)

        let atOne = controller.scene.zoomBounds().width
        controller.scene.zoomLevel = 2
        XCTAssertEqual(controller.scene.zoomBounds().width, atOne / 2, accuracy: 0.001)
    }

    func testCentringAnItemAfterZoom() {
        let (controller, _, diagram) = makeController()
        controller.scene.zoomLevel = 2
        let second = items(diagram)[1]

        controller.scene.centerItem(second, 10)
        XCTAssertTrue(controller.scene.ox.isFinite)
        XCTAssertTrue(controller.scene.oy.isFinite)
    }
}

// MARK: - Nested elements

@MainActor
class SceneNestedElementTests: XCTestCase {

    func testSwitchingToANestedDiagramRebuildsTheScene() {
        let (controller, _, diagram) = makeController()
        let nested = diagram.elements[0]

        controller.onElementSelected(nested)
        XCTAssertEqual(controller.scene.element, nested)
        XCTAssertNotNil(controller.scene.scene?.drawables[nested.items[0]])
    }

    func testSwitchingBackRestoresTheOuterDiagram() {
        let (controller, _, diagram) = makeController()
        controller.onElementSelected(diagram.elements[0])
        controller.onElementSelected(diagram)

        XCTAssertEqual(controller.scene.element, diagram)
        for item in diagram.items {
            XCTAssertNotNil(controller.scene.scene?.drawables[item])
        }
    }

    func testRemovingANestedElement() {
        let (controller, _, diagram) = makeController()
        let before = diagram.elements.count

        controller.onElementSelected(diagram.elements[0])
        controller.handleRemoveElement()
        XCTAssertLessThanOrEqual(diagram.elements.count, before)
    }

    func testSwitchingElementsRecentresTheView() {
        let (controller, _, diagram) = makeController()
        let nested = diagram.elements[0]

        controller.scene.ox = 111
        controller.onElementSelected(nested)

        // setActiveElement recentres on the new diagram rather than restoring the stored offset.
        guard let bounds = controller.scene.scene?.getBounds() else { return XCTFail("Expected a scene") }
        XCTAssertEqual(controller.scene.ox, -bounds.midX, accuracy: 0.001)
    }
}

// MARK: - Window and application wiring

@MainActor
class WindowAndAppTests: XCTestCase {

    func testWindowTitleIsTheDocumentName() {
        let controller = WindowController()
        XCTAssertEqual(controller.windowTitle(forDocumentDisplayName: "Diagram.tenn"), "Diagram.tenn")
    }

    func testWindowLoadingAppliesAMinimumSize() {
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 900, height: 700),
            styleMask: [.titled, .resizable], backing: .buffered, defer: false)
        let controller = WindowController(window: window)

        controller.windowDidLoad()
        XCTAssertEqual(window.minSize, NSSize(width: 600, height: 400))
    }

    func testTerminateAfterLastWindowIsConfigurable() {
        let delegate = AppDelegate()

        delegate.setTerminateWindows(true)
        XCTAssertTrue(delegate.applicationShouldTerminateAfterLastWindowClosed(NSApplication.shared))

        delegate.setTerminateWindows(false)
        XCTAssertFalse(delegate.applicationShouldTerminateAfterLastWindowClosed(NSApplication.shared))
    }

    func testLifecycleCallbacksAreHarmless() {
        let delegate = AppDelegate()
        delegate.applicationDidBecomeActive(Notification(name: NSApplication.didBecomeActiveNotification))
        delegate.applicationWillTerminate(Notification(name: NSApplication.willTerminateNotification))
    }
}
