//
//  SyncPanelTests.swift
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

private let syncSource = """
    element Root {
        item Alpha {
            pos 0 0
        }
    }
    """

private let syncSourceWithConfigs = """
    element Root {
        sync {
            config "first" {
                command "echo one"
            }
            config "second" {
                command "echo two"
            }
        }
        item Alpha {
            pos 0 0
        }
    }
    """

@MainActor
private func makeController(_ source: String = syncSource) -> (SyncViewController, ViewController, Element) {
    let model = ElementModel.parseTenn(node: TennParser().parse(source))
    let store = ElementModelStore(model)

    let host = ViewController()
    let window = NSWindow(
        contentRect: NSRect(x: 0, y: 0, width: 944, height: 764),
        styleMask: [.titled, .resizable], backing: .buffered, defer: false)
    window.contentViewController = host
    host.loadViewIfNeeded()
    host.viewDidLoad()
    host.setElementModel(elementStore: store)

    let diagram = model.elements[0]
    host.onElementSelected(diagram)

    // setElement must land before the view loads: viewDidLoad reads the element to build
    // its entry list, and loadViewIfNeeded already triggers it.
    let controller = SyncViewController()
    controller.setElement(element: diagram)
    controller.setViewController(host)
    controller.loadViewIfNeeded()
    return (controller, host, diagram)
}

// MARK: - Sync panel

@MainActor
class SyncPanelTests: XCTestCase {

    func testViewIsBuiltProgrammatically() {
        let (controller, _, _) = makeController()
        XCTAssertNotNil(controller.view)
        XCTAssertNotNil(controller.syncOutline)
    }

    func testLoadingListsOnlyTheAddEntryWithoutConfigs() {
        let (controller, _, _) = makeController()

        XCTAssertEqual(controller.syncTypes.count, 1)
        XCTAssertEqual(controller.syncTypes[0].operation, .AddSyncConfig)
    }

    func testDeclaredConfigsAreListed() {
        let (controller, _, _) = makeController(syncSourceWithConfigs)

        let names = controller.syncTypes.map { $0.name }
        XCTAssertTrue(names.contains("Sync - first"))
        XCTAssertTrue(names.contains("Sync - second"))
        XCTAssertEqual(controller.syncTypes.filter { $0.operation == .Sync }.count, 2)
    }

    func testLoadingWiresUpTheOutline() {
        let (controller, _, _) = makeController(syncSourceWithConfigs)

        XCTAssertNotNil(controller.delegate)
        XCTAssertTrue(controller.syncOutline.dataSource === controller.delegate)
        XCTAssertEqual(controller.syncOutline.numberOfRows, controller.syncTypes.count)
    }

    func testLoadingSizesTheViewToItsContent() {
        let (controller, _, _) = makeController(syncSourceWithConfigs)
        XCTAssertGreaterThan(controller.view.frame.height, 0)
    }

    func testCellViewIsProducedForEveryEntry() {
        let (controller, _, _) = makeController(syncSourceWithConfigs)
        let delegate = controller.delegate!

        for entry in controller.syncTypes {
            let cell = delegate.outlineView(controller.syncOutline, viewFor: nil, item: entry) as? NSTableCellView
            XCTAssertEqual(cell?.textField?.stringValue, entry.name)
        }
    }

    func testCellViewIsNilForUnknownItems() {
        let (controller, _, _) = makeController()

        XCTAssertNil(controller.delegate!.outlineView(controller.syncOutline, viewFor: nil, item: "not an entry"))
    }

    func testObjectValueIsTheEntryName() {
        let (controller, _, _) = makeController(syncSourceWithConfigs)
        let delegate = controller.delegate!

        let entry = controller.syncTypes[0]
        XCTAssertEqual(delegate.outlineView(controller.syncOutline, objectValueFor: nil, byItem: entry) as? String, entry.name)
        XCTAssertNil(delegate.outlineView(controller.syncOutline, objectValueFor: nil, byItem: "other"))
    }

    func testChildLookupOutOfRangeFallsBackToEmpty() {
        let (controller, _, _) = makeController()
        let delegate = controller.delegate!

        let value = delegate.outlineView(controller.syncOutline, child: 99, ofItem: nil)
        XCTAssertEqual(value as? String, "")
    }

    func testNothingIsExpandable() {
        let (controller, _, _) = makeController(syncSourceWithConfigs)
        let delegate = controller.delegate!

        XCTAssertFalse(delegate.outlineView(controller.syncOutline, isItemExpandable: controller.syncTypes[0]))
    }

    // Selecting a row ends in dismissViewController, which throws unless the controller
    // was actually presented, so the selection path is not exercised here.

    func testControllerWithoutAnElementIsHarmless() {
        let controller = SyncViewController()
        controller.loadViewIfNeeded()

        XCTAssertTrue(controller.syncTypes.contains(where: { $0.operation == .AddSyncConfig }))
    }
}
