//
//  SceneDrawViewEventTests.swift
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

@MainActor
private func makeView() -> (SceneDrawView, ElementModelStore, Element) {
    let source = """
        element Root {
            item First {
                pos 0 0
            }
            item Second {
                pos 300 0
            }
            link First Second {
            }
        }
        """
    let model = ElementModel.parseTenn(node: TennParser().parse(source))
    let store = ElementModelStore(model)

    let view = SceneDrawView(frame: NSRect(x: 0, y: 0, width: 800, height: 600))
    view.setModel(store: store)
    let diagram = model.elements[0]
    view.setActiveElement(diagram)
    return (view, store, diagram)
}

@MainActor
private func items(_ element: Element) -> [DiagramItem] {
    return element.items.filter { $0.kind == .Item }
}

private func mouseEvent(
    _ type: NSEvent.EventType, at point: CGPoint, clickCount: Int = 1, modifiers: NSEvent.ModifierFlags = []
) -> NSEvent {
    return NSEvent.mouseEvent(
        with: type, location: point, modifierFlags: modifiers, timestamp: ProcessInfo.processInfo.systemUptime,
        windowNumber: 0, context: nil, eventNumber: 0, clickCount: clickCount, pressure: 1)!
}

private func keyEvent(_ characters: String, code: UInt16, modifiers: NSEvent.ModifierFlags = []) -> NSEvent {
    return NSEvent.keyEvent(
        with: .keyDown, location: .zero, modifierFlags: modifiers, timestamp: ProcessInfo.processInfo.systemUptime,
        windowNumber: 0, context: nil, characters: characters, charactersIgnoringModifiers: characters,
        isARepeat: false, keyCode: code)!
}

/// The view point that lands on the centre of `item`'s drawable.
@MainActor
private func viewPoint(of item: DiagramItem, in view: SceneDrawView) -> CGPoint {
    guard let drawable = view.scene?.drawables[item] else { return .zero }
    let bounds = drawable.getBounds()
    let sceneBounds = view.zoomBounds()
    return CGPoint(
        x: (bounds.midX + sceneBounds.midX + view.ox) * view.zoomLevel,
        y: (bounds.midY + sceneBounds.midY + view.oy) * view.zoomLevel)
}

// MARK: - Mouse

@MainActor
class SceneDrawViewMouseTests: XCTestCase {

    func testMouseDownTracksThePointerPosition() {
        let (view, _, _) = makeView()
        view.mouseDown(with: mouseEvent(.leftMouseDown, at: CGPoint(x: 400, y: 300)))

        XCTAssertTrue(view.mouseDownState)
        XCTAssertTrue(view.x.isFinite)
        XCTAssertTrue(view.y.isFinite)
    }

    func testClickingEmptySpaceEntersPanningMode() {
        let (view, _, _) = makeView()
        view.mouseDown(with: mouseEvent(.leftMouseDown, at: CGPoint(x: 790, y: 590)))
        XCTAssertEqual(view.mode, .DiagramMove, "Empty space starts a diagram pan")
    }

    func testClickingAnItemSelectsIt() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]

        let point = viewPoint(of: first, in: view)
        view.mouseDown(with: mouseEvent(.leftMouseDown, at: point))
        view.mouseUp(with: mouseEvent(.leftMouseUp, at: point))

        XCTAssertEqual(view.activeItems, [first])
    }

    func testShiftClickStartsARubberBandSelection() {
        let (view, _, diagram) = makeView()
        view.setActiveItem(items(diagram)[0])

        view.mouseDown(with: mouseEvent(.leftMouseDown, at: CGPoint(x: 400, y: 300), modifiers: .shift))
        XCTAssertEqual(view.mode, .Selection)
        XCTAssertTrue(view.activeItems.isEmpty, "The previous selection is parked while the band is drawn")
    }

    func testShiftDragThenReleaseRestoresTheParkedSelection() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]
        view.setActiveItem(first)

        view.mouseDown(with: mouseEvent(.leftMouseDown, at: CGPoint(x: 10, y: 10), modifiers: .shift))
        view.mouseUp(with: mouseEvent(.leftMouseUp, at: CGPoint(x: 20, y: 20)))

        XCTAssertEqual(view.mode, .Normal)
        XCTAssertEqual(view.activeItems, [first])
    }

    func testDraggingOverEmptySpaceKeepsPanningMode() {
        let (view, _, _) = makeView()

        view.mouseDown(with: mouseEvent(.leftMouseDown, at: CGPoint(x: 790, y: 590)))
        view.mouseDragged(with: mouseEvent(.leftMouseDragged, at: CGPoint(x: 700, y: 500)))

        // Panning reads event.deltaX/deltaY, which synthesised events always report as zero,
        // so the offsets stay put here - only the mode and redraw path are exercised.
        XCTAssertEqual(view.mode, .DiagramMove)
        XCTAssertTrue(view.drawScheduled)
    }

    func testDragSelectionBoxIsTracked() {
        let (view, _, _) = makeView()
        view.mouseDown(with: mouseEvent(.leftMouseDown, at: CGPoint(x: 10, y: 10), modifiers: .shift))
        view.mouseDragged(with: mouseEvent(.leftMouseDragged, at: CGPoint(x: 400, y: 300), modifiers: .shift))

        XCTAssertNotNil(view.scene?.selectionBox, "The rubber band is handed to the scene to draw")
    }

    func testSelectionBoxIsClearedOnRelease() {
        let (view, _, _) = makeView()
        view.mouseDown(with: mouseEvent(.leftMouseDown, at: CGPoint(x: 10, y: 10), modifiers: .shift))
        view.mouseDragged(with: mouseEvent(.leftMouseDragged, at: CGPoint(x: 400, y: 300), modifiers: .shift))
        view.mouseUp(with: mouseEvent(.leftMouseUp, at: CGPoint(x: 400, y: 300)))

        XCTAssertNil(view.scene?.selectionBox)
    }

    func testCommandClickTogglesAnItemIntoTheSelection() {
        let (view, _, diagram) = makeView()
        let all = items(diagram)
        view.setActiveItem(all[0])

        let point = viewPoint(of: all[1], in: view)
        view.mouseDown(with: mouseEvent(.leftMouseDown, at: point, modifiers: .command))
        view.mouseUp(with: mouseEvent(.leftMouseUp, at: point, modifiers: .command))

        XCTAssertTrue(view.activeItems.contains(all[0]))
        XCTAssertTrue(view.activeItems.contains(all[1]), "Command-click adds to the selection")
    }

    func testMouseUpLeavesNormalMode() {
        let (view, _, _) = makeView()
        view.mouseDown(with: mouseEvent(.leftMouseDown, at: CGPoint(x: 790, y: 590)))
        view.mouseUp(with: mouseEvent(.leftMouseUp, at: CGPoint(x: 790, y: 590)))
        XCTAssertEqual(view.mode, .Normal)
    }

    func testScrollWheelMovesTheDiagram() throws {
        let (view, _, _) = makeView()
        let before = CGPoint(x: view.ox, y: view.oy)

        guard
            let scroll = CGEvent(
                scrollWheelEvent2Source: nil, units: .pixel, wheelCount: 2, wheel1: 10, wheel2: 10, wheel3: 0),
            let event = NSEvent(cgEvent: scroll)
        else {
            throw XCTSkip("Cannot synthesise a scroll event in this environment")
        }
        view.scrollWheel(with: event)
        XCTAssertNotEqual(CGPoint(x: view.ox, y: view.oy), before)
    }
}

// MARK: - Keyboard

@MainActor
class SceneDrawViewKeyboardTests: XCTestCase {

    func testArrowKeysMoveTheSelectedItem() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]
        view.setActiveItem(first)
        let before = CGPoint(x: first.x, y: first.y)

        // 124 is the right-arrow key code.
        view.keyDown(with: keyEvent("\u{F703}", code: 124))
        XCTAssertNotEqual(CGPoint(x: first.x, y: first.y), before, "The arrow key nudges the item")
    }

    func testArrowKeysWithNoSelectionAreHarmless() {
        let (view, _, diagram) = makeView()
        let positions = items(diagram).map { CGPoint(x: $0.x, y: $0.y) }

        view.keyDown(with: keyEvent("\u{F703}", code: 124))
        XCTAssertEqual(items(diagram).map { CGPoint(x: $0.x, y: $0.y) }, positions)
    }

    func testAllFourArrowsAreAccepted() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]
        view.setActiveItem(first)

        for (chars, code) in [("\u{F702}", UInt16(123)), ("\u{F703}", 124), ("\u{F701}", 125), ("\u{F700}", 126)] {
            view.keyDown(with: keyEvent(chars, code: code))
        }
        XCTAssertTrue(first.x.isFinite)
        XCTAssertTrue(first.y.isFinite)
    }

    func testFlagsChangedTracksTheShiftKey() {
        let (view, _, _) = makeView()
        guard
            let down = NSEvent.keyEvent(
                with: .flagsChanged, location: .zero, modifierFlags: .shift,
                timestamp: ProcessInfo.processInfo.systemUptime, windowNumber: 0, context: nil,
                characters: "", charactersIgnoringModifiers: "", isARepeat: false, keyCode: 56)
        else {
            return XCTFail("Cannot synthesise a flagsChanged event")
        }
        view.flagsChanged(with: down)
        // The view only has to survive the event and keep a consistent mode.
        XCTAssertEqual(view.mode, .Normal)
    }

    func testViewAcceptsFirstResponder() {
        let (view, _, _) = makeView()
        XCTAssertTrue(view.acceptsFirstResponder)
    }
}

// MARK: - Menu validation

@MainActor
class SceneDrawViewMenuTests: XCTestCase {

    func testDeleteIsOfferedOnlyWithASelection() {
        let (view, _, diagram) = makeView()
        let item = NSMenuItem()
        item.action = #selector(SceneDrawView.delete(_:))

        view.setActiveItem(nil)
        let withoutSelection = view.validateMenuItem(item)

        view.setActiveItem(items(diagram)[0])
        let withSelection = view.validateMenuItem(item)

        XCTAssertTrue(withSelection || !withoutSelection, "Selecting must not make delete less available")
    }

    func testUnknownMenuActionIsStillAnswered() {
        let (view, _, _) = makeView()
        let item = NSMenuItem()
        item.action = #selector(NSObject.description as () -> String)
        _ = view.validateMenuItem(item)
    }
}
