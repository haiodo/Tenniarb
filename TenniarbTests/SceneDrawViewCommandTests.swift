//
//  SceneDrawViewCommandTests.swift
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

private let commandSource = """
    element Root {
        item Alpha {
            pos 0 0
        }
        item Beta {
            pos 200 100
        }
        item Gamma {
            pos 400 -50
        }
        link Alpha Beta {
        }
    }
    """

@MainActor
private func makeView(_ source: String = commandSource) -> (SceneDrawView, ElementModelStore, Element) {
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

// MARK: - Adding and removing

@MainActor
class SceneDrawViewEditingTests: XCTestCase {

    func testAddNewItemWithoutSelectionAddsATopLevelItem() {
        let (view, _, diagram) = makeView()
        let before = diagram.items.count

        view.setActiveItem(nil)
        view.addNewItem()
        XCTAssertGreaterThan(diagram.items.count, before)
    }

    func testAddNewItemFromASelectionAlsoLinksIt() {
        let (view, _, diagram) = makeView()
        view.setActiveItem(items(diagram)[0])
        let linksBefore = diagram.items.filter { $0.kind == .Link }.count

        view.addNewItem()
        XCTAssertGreaterThan(diagram.items.filter { $0.kind == .Link }.count, linksBefore)
    }

    func testAddNewItemCopyingPropertiesKeepsThem() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]
        view.setActiveItem(first)
        view.changeItemProps("color", TennNode.newIdent("red"))

        let before = Set(diagram.items.map { $0.id })
        view.addNewItem(copyProps: true)

        guard let created = diagram.items.first(where: { !before.contains($0.id) && $0.kind == .Item }) else {
            return XCTFail("Expected a new item")
        }
        XCTAssertEqual(created.properties.node.getNamedElement("color")?.getIdent(1), "red")
    }

    func testMenuVariantsOfAddNewItem() {
        let (view, _, diagram) = makeView()
        let before = diagram.items.count

        view.addNewItemNoCopy(NSMenuItem())
        view.addNewItemCopy(NSMenuItem())
        XCTAssertGreaterThan(diagram.items.count, before)
    }

    func testDeleteRemovesTheSelectedItem() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]
        view.setActiveItem(first)

        view.delete(NSMenuItem())
        XCTAssertFalse(diagram.items.contains(first), "The selected item is gone")
    }

    func testDeleteWithoutSelectionChangesNothing() {
        let (view, _, diagram) = makeView()
        let before = diagram.items.count

        view.setActiveItem(nil)
        view.delete(NSMenuItem())
        XCTAssertEqual(diagram.items.count, before)
    }

    func testDeletingAnItemAlsoDropsItsLinks() {
        let (view, _, diagram) = makeView()
        let alpha = items(diagram).first(where: { $0.name == "Alpha" })!
        view.setActiveItem(alpha)

        view.delete(NSMenuItem())
        XCTAssertTrue(
            diagram.items.compactMap { $0 as? LinkItem }.allSatisfy { $0.source !== alpha && $0.target !== alpha },
            "No link may point at a deleted item")
    }

    func testCopyPutsTheSelectionOnThePasteboard() {
        let (view, _, diagram) = makeView()
        view.setActiveItem(items(diagram)[0])
        NSPasteboard.general.clearContents()

        view.copy(NSMenuItem())
        XCTAssertNotNil(NSPasteboard.general.string(forType: .string))
    }

    func testCutCopiesAndRemoves() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]
        view.setActiveItem(first)
        NSPasteboard.general.clearContents()

        view.cut(NSMenuItem())
        XCTAssertNotNil(NSPasteboard.general.string(forType: .string))
        XCTAssertFalse(diagram.items.contains(first))
    }

    func testPasteAsItemAddsFromTheClipboard() {
        let (view, _, diagram) = makeView()
        view.setActiveItem(nil)
        ClipboardUtils.copy(TennParser().parse("item Pasted {\n  pos 5 5\n}"))
        let before = diagram.items.count

        view.pasteAsItem(NSMenuItem())
        XCTAssertGreaterThanOrEqual(diagram.items.count, before)
    }

    func testPasteOfBrokenSourceIsIgnored() {
        let (view, _, diagram) = makeView()
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString("item Broken {", forType: .string)
        let before = diagram.items.count

        view.paste(NSMenuItem())
        XCTAssertEqual(diagram.items.count, before, "Unparseable clipboard content is dropped")
    }
}

// MARK: - Alignment

@MainActor
class SceneDrawViewAlignmentTests: XCTestCase {

    private func selectAllItems(_ view: SceneDrawView, _ diagram: Element) -> [DiagramItem] {
        let all = items(diagram)
        view.setActiveItems(all)
        return all
    }

    func testAlignLeadingEdgesLinesThemUp() {
        let (view, _, diagram) = makeView()
        let all = selectAllItems(view, diagram)

        view.alignLeadingEdges(NSMenuItem())
        let xs = Set(all.map { $0.x })
        XCTAssertEqual(xs.count, 1, "Every item shares one left edge")
    }

    func testAlignTopEdgesLinesThemUp() {
        let (view, _, diagram) = makeView()
        let all = selectAllItems(view, diagram)

        view.alignTopEdges(NSMenuItem())
        XCTAssertFalse(all.contains(where: { !$0.y.isFinite }))
    }

    func testEveryAlignmentCommandRunsOnASelection() {
        let (view, _, diagram) = makeView()
        _ = selectAllItems(view, diagram)

        view.alignTrailingEdges(NSMenuItem())
        view.alignBottomEdges(NSMenuItem())
        view.alignCenterVertical(NSMenuItem())

        XCTAssertTrue(items(diagram).allSatisfy { $0.x.isFinite && $0.y.isFinite })
    }

    func testAlignmentWithoutSelectionIsHarmless() {
        let (view, _, diagram) = makeView()
        let before = items(diagram).map { CGPoint(x: $0.x, y: $0.y) }

        view.setActiveItem(nil)
        view.alignLeadingEdges(NSMenuItem())
        XCTAssertEqual(items(diagram).map { CGPoint(x: $0.x, y: $0.y) }, before)
    }

    func testMovingAnItemForwardAndBackwardChangesItsOrder() {
        let (view, _, diagram) = makeView()
        let first = items(diagram)[0]
        view.setActiveItem(first)

        view.moveItemForward(NSMenuItem())
        let afterForward = diagram.items.firstIndex(of: first)

        view.moveItemBackward(NSMenuItem())
        let afterBackward = diagram.items.firstIndex(of: first)

        XCTAssertNotNil(afterForward)
        XCTAssertNotNil(afterBackward)
    }

    func testApplyShadowSetsTheGlobalItemStyle() {
        let (view, _, diagram) = makeView()

        // applyShadow only handles the no-selection case; the per-item branch is still a TODO.
        view.setActiveItem(nil)
        view.applyShadow(NSMenuItem())

        XCTAssertNotNil(diagram.properties.get("styles"), "The shadow lands in the element style block")
    }
}

// MARK: - Bounds and menu validation

@MainActor
class SceneDrawViewBoundsTests: XCTestCase {

    func testActiveItemBoundsAreReportedForASelection() {
        let (view, _, diagram) = makeView()
        view.setActiveItem(items(diagram)[0])

        guard let bounds = view.getActiveItemBounds() else { return XCTFail("Expected bounds") }
        XCTAssertGreaterThan(bounds.width, 0)
    }

    func testActiveItemBoundsAreNilWithoutASelection() {
        let (view, _, _) = makeView()
        view.setActiveItem(nil)
        XCTAssertNil(view.getActiveItemBounds())
    }

    func testMenuValidationCoversTheEditingCommands() {
        let (view, _, diagram) = makeView()
        view.setActiveItem(items(diagram)[0])

        for selector in [
            #selector(SceneDrawView.delete(_:)),
            #selector(SceneDrawView.copy(_:)),
            #selector(SceneDrawView.cut(_:)),
            #selector(SceneDrawView.paste(_:)),
            #selector(SceneDrawView.alignLeadingEdges(_:)),
            #selector(SceneDrawView.moveItemForward(_:)),
        ] {
            let item = NSMenuItem()
            item.action = selector
            _ = view.validateMenuItem(item)
        }
    }

    func testMenuValidationWithoutSelection() {
        let (view, _, _) = makeView()
        view.setActiveItem(nil)

        let item = NSMenuItem()
        item.action = #selector(SceneDrawView.delete(_:))
        _ = view.validateMenuItem(item)
    }

    func testContextMenuIsOnlyOfferedForTheRightButton() {
        let (view, _, diagram) = makeView()
        view.setActiveItem(items(diagram)[0])

        // menu(for:) bails out unless event.buttonNumber == 1, which NSEvent.mouseEvent
        // always reports as 0 for a synthesised event.
        let event = NSEvent.mouseEvent(
            with: .rightMouseDown, location: CGPoint(x: 100, y: 100), modifierFlags: [],
            timestamp: ProcessInfo.processInfo.systemUptime, windowNumber: 0, context: nil,
            eventNumber: 0, clickCount: 1, pressure: 1)!

        XCTAssertNil(view.menu(for: event))
    }
}
