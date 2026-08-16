//
//  PanelControllerTests.swift
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

private let panelSource = """
    element Root {
        item Alpha {
            pos 0 0
            body %{first body text}
        }
        item Beta {
            pos 100 0
        }
        item Gamma {
            pos 200 0
            body %{alpha appears in this body}
        }
        link Alpha Beta {
        }
    }
    """

@MainActor
private func makeStore(_ source: String = panelSource) -> (ElementModelStore, Element) {
    let model = ElementModel.parseTenn(node: TennParser().parse(source))
    return (ElementModelStore(model), model.elements[0])
}

@MainActor
private func makeSceneView(_ store: ElementModelStore, _ diagram: Element) -> SceneDrawView {
    let view = SceneDrawView(frame: NSRect(x: 0, y: 0, width: 800, height: 600))
    view.setModel(store: store)
    view.setActiveElement(diagram)
    return view
}

@MainActor
private func items(_ element: Element) -> [DiagramItem] {
    return element.items.filter { $0.kind == .Item }
}

/// Let one turn of the main queue run, for the controllers that defer their work.
@MainActor
private func drainMainQueue(_ test: XCTestCase, after delay: TimeInterval = 0) {
    let done = test.expectation(description: "main queue drained")
    DispatchQueue.main.asyncAfter(deadline: .now() + delay) { done.fulfill() }
    test.wait(for: [done], timeout: 5)
}

// MARK: - StyleManager

@MainActor
class StyleManagerTests: XCTestCase {

    private func manager(_ source: String = panelSource) -> (StyleManager, ElementModelStore, Element, SceneDrawView) {
        let (store, diagram) = makeStore(source)
        let view = makeSceneView(store, diagram)
        return (StyleManager(scene: view), store, diagram, view)
    }

    func testManagerReadsItsStateFromTheScene() {
        let (styles, store, diagram, _) = manager()

        XCTAssertEqual(styles.element, diagram)
        XCTAssertTrue(styles.elementStore === store)
        XCTAssertTrue(styles.activeItems.isEmpty)
    }

    func testWithoutDeclaredStylesOnlyTheDefineEntryIsOffered() {
        let (styles, _, _, _) = manager()
        styles.update()

        XCTAssertEqual(styles.styleTypes.count, 1)
        XCTAssertEqual(styles.styleTypes[0].name, "Define new style")
        XCTAssertEqual(styles.styleTypes[0].operation, .AddStyleConfig)
    }

    func testDeclaredStylesAreListedWithASeparator() {
        let (styles, _, _, _) = manager(
            """
            element Root {
                styles {
                    highlight {
                        color red
                    }
                    muted {
                        color gray
                    }
                }
                item Alpha {
                    pos 0 0
                }
            }
            """)
        styles.update()

        let names = styles.styleTypes.map { $0.name }
        XCTAssertTrue(names.contains("highlight"))
        XCTAssertTrue(names.contains("muted"))
        XCTAssertTrue(names.contains("-"), "A separator sits between the styles and the define entry")
        XCTAssertEqual(styles.styleTypes.last?.operation, .AddStyleConfig)
    }

    func testItemAndLineDefaultsAreNotOfferedAsStyles() {
        let (styles, _, _, _) = manager(
            """
            element Root {
                styles {
                    item {
                        font-size 20
                    }
                    line {
                        line-width 2
                    }
                    highlight {
                        color red
                    }
                }
                item Alpha {
                    pos 0 0
                }
            }
            """)
        styles.update()

        let names = styles.styleTypes.map { $0.name }
        XCTAssertFalse(names.contains("item"), "item is the default style, not a named one")
        XCTAssertFalse(names.contains("line"))
        XCTAssertTrue(names.contains("highlight"))
    }

    func testUpdateStartsFromACleanList() {
        let (styles, _, _, _) = manager()
        styles.update()
        styles.update()
        XCTAssertEqual(styles.styleTypes.count, 1, "Repeated updates must not accumulate entries")
    }

    func testMenuIsBuiltFromTheStyleList() {
        let (styles, _, _, _) = manager()
        let menu = styles.createMenu()
        XCTAssertGreaterThan(menu.items.count, 0)
    }

    func testDefineNewStyleAddsItToTheElement() {
        let (styles, _, diagram, _) = manager()
        styles.update()

        guard let index = styles.styleTypes.firstIndex(where: { $0.operation == .AddStyleConfig }) else {
            return XCTFail("Expected the define entry")
        }
        let menuItem = NSMenuItem()
        menuItem.tag = index
        styles.performAction(menuItem)

        XCTAssertNotNil(diagram.properties.get("styles"), "A styles block appears on the element")
    }

    func testActiveItemsFollowTheSceneSelection() {
        let (styles, _, diagram, view) = manager()
        let first = items(diagram)[0]
        view.setActiveItem(first)

        XCTAssertEqual(styles.activeItems, [first])
    }

    func testApplyingAStyleTagsTheSelectedItem() {
        let (styles, _, diagram, view) = manager(
            """
            element Root {
                styles {
                    highlight {
                        color red
                    }
                }
                item Alpha {
                    pos 0 0
                }
            }
            """)
        styles.update()
        let first = items(diagram)[0]
        view.setActiveItem(first)

        guard let index = styles.styleTypes.firstIndex(where: { $0.operation == .Apply }) else {
            return XCTFail("Expected a style to apply")
        }
        let menuItem = NSMenuItem()
        menuItem.tag = index
        styles.performAction(menuItem)

        XCTAssertEqual(first.properties.node.getNamedElement("use-style")?.getIdent(1), "highlight")
    }
}

// MARK: - OperationController

@MainActor
class OperationControllerTests: XCTestCase {

    private func controller() -> (OperationController, ElementModelStore, Element) {
        let (store, diagram) = makeStore()
        let controller = OperationController()
        controller.loadViewIfNeeded()
        controller.setStore(store)
        controller.setElement(diagram)
        return (controller, store, diagram)
    }

    func testSettingItemsAccumulates() {
        let (controller, _, diagram) = controller()
        let all = items(diagram)

        controller.setItems([all[0]])
        controller.setItems([all[1]])
        XCTAssertEqual(controller.items, [all[0], all[1]], "setItems appends rather than replaces")
    }

    func testCreateOperationAddsAMissingProperty() {
        let (controller, _, diagram) = controller()
        let first = items(diagram)[0]

        let operation = controller.createOperation(first, TennParser().parse("color red"))
        XCTAssertNotNil(operation)

        operation?.apply()
        XCTAssertEqual(first.properties.node.getNamedElement("color")?.getIdent(1), "red")
    }

    func testCreateOperationReplacesAnExistingProperty() {
        let (controller, _, diagram) = controller()
        let first = items(diagram)[0]

        controller.createOperation(first, TennParser().parse("color red"))?.apply()
        controller.createOperation(first, TennParser().parse("color blue"))?.apply()

        XCTAssertEqual(first.properties.node.getNamedElement("color")?.getIdent(1), "blue")
    }

    func testMinusPrefixRemovesAProperty() {
        let (controller, _, diagram) = controller()
        let first = items(diagram)[0]
        controller.createOperation(first, TennParser().parse("color red"))?.apply()

        let removal = controller.createOperation(first, TennParser().parse("-color"))
        XCTAssertNotNil(removal, "Removing an existing property is a change")

        removal?.apply()
        XCTAssertNil(first.properties.node.getNamedElement("color"))
    }

    func testRemovingAnAbsentPropertyIsNotAChange() {
        let (controller, _, diagram) = controller()
        let first = items(diagram)[0]

        XCTAssertNil(controller.createOperation(first, TennParser().parse("-nosuchproperty")))
    }

    func testOperationIsUndoable() {
        let (controller, _, diagram) = controller()
        let first = items(diagram)[0]

        guard let operation = controller.createOperation(first, TennParser().parse("color red")) else {
            return XCTFail("Expected an operation")
        }
        operation.apply()
        operation.undo()

        XCTAssertNil(first.properties.node.getNamedElement("color"), "Undo takes the property back out")
    }
}

// MARK: - SearchBoxViewController

@MainActor
class SearchBoxTests: XCTestCase {

    private func searchBox() -> (SearchBoxViewController, Element) {
        let (_, diagram) = makeStore()
        let controller = SearchBoxViewController()
        controller.loadViewIfNeeded()
        controller.setElement(diagram)
        controller.searchResultDelegate = SearchBoxResultDelegate(controller)
        return (controller, diagram)
    }

    func testViewIsBuiltProgrammatically() {
        let (controller, _) = searchBox()
        XCTAssertNotNil(controller.searchBox)
        XCTAssertNotNil(controller.resultView)
    }

    func testSearchingByNameFindsTheItem() {
        let (controller, _) = searchBox()
        controller.searchBox.stringValue = "beta"
        controller.controlTextDidChange(Notification(name: NSControl.textDidChangeNotification))

        drainMainQueue(self, after: 0.4)
        XCTAssertTrue(controller.currentItems.contains(where: { $0.name == "Beta" }))
    }

    func testSearchingAlsoLooksInsideTheBody() {
        let (controller, _) = searchBox()
        controller.searchBox.stringValue = "alpha"
        controller.controlTextDidChange(Notification(name: NSControl.textDidChangeNotification))

        drainMainQueue(self, after: 0.4)
        XCTAssertTrue(
            controller.currentItems.contains(where: { $0.name == "Gamma" }),
            "Gamma has 'alpha' only in its body text")
    }

    func testResultsAreSortedByName() {
        let (controller, _) = searchBox()
        controller.searchBox.stringValue = "a"
        controller.controlTextDidChange(Notification(name: NSControl.textDidChangeNotification))

        drainMainQueue(self, after: 0.4)
        let names = controller.currentItems.map { $0.name }
        XCTAssertEqual(names, names.sorted { $0.lexicographicallyPrecedes($1) })
    }

    func testSearchingForNothingMatchingClearsTheList() {
        let (controller, _) = searchBox()
        controller.searchBox.stringValue = "zzzznotpresent"
        controller.controlTextDidChange(Notification(name: NSControl.textDidChangeNotification))

        drainMainQueue(self, after: 0.4)
        XCTAssertTrue(controller.currentItems.isEmpty)
    }

    func testOnlyTheLastKeystrokeRuns() {
        let (controller, _) = searchBox()
        controller.searchBox.stringValue = "alpha"
        controller.controlTextDidChange(Notification(name: NSControl.textDidChangeNotification))
        controller.searchBox.stringValue = "beta"
        controller.controlTextDidChange(Notification(name: NSControl.textDidChangeNotification))

        drainMainQueue(self, after: 0.4)
        XCTAssertFalse(controller.currentItems.contains(where: { $0.name == "Gamma" }), "The stale query is discarded")
    }

    func testCloseInvokesTheCloseAction() {
        let (controller, _) = searchBox()
        var closed = false
        controller.closeAction = { closed = true }

        controller.close()
        XCTAssertTrue(closed)
    }

    func testCloseWithoutAnActionIsHarmless() {
        let (controller, _) = searchBox()
        controller.close()
    }

    func testResultDelegateCountsOnlyTopLevelItems() {
        let (controller, diagram) = searchBox()
        controller.currentItems = items(diagram)
        let delegate = SearchBoxResultDelegate(controller)
        let outline = NSOutlineView()

        XCTAssertEqual(delegate.outlineView(outline, numberOfChildrenOfItem: nil), controller.currentItems.count)
        XCTAssertEqual(delegate.outlineView(outline, numberOfChildrenOfItem: controller.currentItems[0]), 0)
    }

    func testResultDelegateReturnsItemsAndNames() {
        let (controller, diagram) = searchBox()
        controller.currentItems = items(diagram)
        let delegate = SearchBoxResultDelegate(controller)
        let outline = NSOutlineView()

        let first = delegate.outlineView(outline, child: 0, ofItem: nil)
        XCTAssertTrue(first is DiagramItem)
        XCTAssertEqual(delegate.outlineView(outline, objectValueFor: nil, byItem: first) as? String, controller.currentItems[0].name)
        XCTAssertFalse(delegate.outlineView(outline, isItemExpandable: first))
    }

    func testResultDelegateIgnoresUnknownItems() {
        let (controller, _) = searchBox()
        let delegate = SearchBoxResultDelegate(controller)
        let outline = NSOutlineView()

        XCTAssertNil(delegate.outlineView(outline, objectValueFor: nil, byItem: "not an item"))
    }
}

// MARK: - SyncViewController

@MainActor
class SyncViewControllerTests: XCTestCase {

    func testSyncInfoKeepsWhatItWasGiven() {
        let node = TennNode.newCommand("config", TennNode.newIdent("remote"))
        let info = SyncInfo("Sync - remote", node, .Sync)

        XCTAssertEqual(info.name, "Sync - remote")
        XCTAssertEqual(info.operation, .Sync)
        XCTAssertNotNil(info.node)
    }

    func testControllerStartsWithoutAnElement() {
        let controller = SyncViewController()
        XCTAssertNil(controller.element)
        XCTAssertTrue(controller.syncTypes.isEmpty)
    }

    func testSettingTheElementIsRemembered() {
        let (_, diagram) = makeStore()
        let controller = SyncViewController()
        controller.setElement(element: diagram)
        XCTAssertEqual(controller.element, diagram)
    }

    func testDelegateCountsTheConfiguredEntries() {
        let controller = SyncViewController()
        controller.syncTypes = [
            SyncInfo("Add sync config...", nil, .AddSyncConfig),
            SyncInfo("Sync - remote", nil, .Sync),
        ]
        let delegate = SyncViewControllerDelegate(controller)
        let outline = NSOutlineView()

        XCTAssertEqual(delegate.outlineView(outline, numberOfChildrenOfItem: nil), 2)
        XCTAssertEqual(delegate.outlineView(outline, numberOfChildrenOfItem: controller.syncTypes[0]), 0)
    }

    func testDelegateReturnsEntriesInOrder() {
        let controller = SyncViewController()
        controller.syncTypes = [
            SyncInfo("first", nil, .AddSyncConfig),
            SyncInfo("second", nil, .Sync),
        ]
        let delegate = SyncViewControllerDelegate(controller)
        let outline = NSOutlineView()

        let entry = delegate.outlineView(outline, child: 1, ofItem: nil) as? SyncInfo
        XCTAssertEqual(entry?.name, "second")
    }
}
