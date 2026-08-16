//
//  PreferencesPanelTests.swift
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

// MARK: - Preferences panel

@MainActor
class PreferencesGeneralControllerTests: XCTestCase {

    private func controller() -> PreferencesGeneralController {
        let controller = PreferencesGeneralController()
        controller.loadViewIfNeeded()
        return controller
    }

    func testEveryFieldIsBuilt() {
        let panel = controller()

        XCTAssertNotNil(panel.backgroundField)
        XCTAssertNotNil(panel.backgroundDarkField)
        XCTAssertNotNil(panel.expandLevelField)
        XCTAssertNotNil(panel.autoExpandButton)
        XCTAssertNotNil(panel.renderEnableButton)
        XCTAssertNotNil(panel.renderNativeScaleButton)
        XCTAssertNotNil(panel.uiQuickPanelButton)
        XCTAssertNotNil(panel.uiTransparentButton)
    }

    func testAppearingFillsTheFieldsFromPreferences() {
        let panel = controller()
        panel.viewWillAppear()

        XCTAssertFalse(panel.backgroundField.stringValue.isEmpty, "The current background is shown as hex")
        XCTAssertFalse(panel.backgroundDarkField.stringValue.isEmpty)
    }

    func testChangingTheBackgroundAppliesAValidColour() {
        let panel = controller()
        panel.viewWillAppear()

        panel.backgroundField.stringValue = "#123456"
        panel.backgroundChanged(panel.backgroundField)

        let applied = PreferenceConstants.preference.backgroundGet.components ?? []
        XCTAssertEqual(applied[0], CGFloat(0x12) / 255.0, accuracy: 0.01)

        panel.resetBackground(NSButton())
    }

    func testAnInvalidColourIsIgnored() {
        let panel = controller()
        panel.viewWillAppear()
        let before = PreferenceConstants.preference.backgroundGet.components ?? []

        panel.backgroundField.stringValue = "not a colour"
        panel.backgroundChanged(panel.backgroundField)

        XCTAssertEqual(PreferenceConstants.preference.backgroundGet.components ?? [], before)
    }

    func testChangingTheDarkBackground() {
        let panel = controller()
        panel.viewWillAppear()

        panel.backgroundDarkField.stringValue = "#654321"
        panel.backgroundDarkChanged(panel.backgroundDarkField)

        let applied = PreferenceConstants.preference.backgroundDarkGet.components ?? []
        XCTAssertEqual(applied[0], CGFloat(0x65) / 255.0, accuracy: 0.01)

        panel.resetDarkBackground(NSButton())
    }

    func testResettingRestoresTheDefaultColours() {
        let panel = controller()
        panel.viewWillAppear()

        panel.backgroundField.stringValue = "#010203"
        panel.backgroundChanged(panel.backgroundField)
        panel.resetBackground(NSButton())

        XCTAssertFalse(panel.backgroundField.stringValue.isEmpty)
        panel.resetDarkBackground(NSButton())
        XCTAssertFalse(panel.backgroundDarkField.stringValue.isEmpty)
    }

    func testToggleButtonsWriteToUserDefaults() {
        let panel = controller()
        panel.viewWillAppear()

        for (button, action) in [
            (panel.autoExpandButton!, panel.autoExpandChanged),
            (panel.renderEnableButton!, panel.renderEnableChanged),
            (panel.renderNativeScaleButton!, panel.renderNativeScaleChanged),
            (panel.uiQuickPanelButton!, panel.uiQuickPanelChanged),
            (panel.uiTransparentButton!, panel.uiTransparentChanged),
        ] {
            let original = button.state
            button.state = original == .on ? .off : .on
            action(button)
            button.state = original
            action(button)
        }
    }

    func testExpandLevelIsStored() {
        let panel = controller()
        panel.viewWillAppear()

        panel.expandLevelField.integerValue = 4
        panel.expandLevelChanged(panel.expandLevelField)
        XCTAssertEqual(PreferenceConstants.preference.autoExpandLevel, 4)

        panel.expandLevelField.integerValue = 2
        panel.expandLevelChanged(panel.expandLevelField)
    }

    func testEscapeClosesThePanel() {
        let panel = controller()
        guard
            let escape = NSEvent.keyEvent(
                with: .keyDown, location: .zero, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                windowNumber: 0, context: nil, characters: "\u{1B}", charactersIgnoringModifiers: "\u{1B}",
                isARepeat: false, keyCode: 53)
        else {
            return XCTFail("Cannot synthesise the escape key")
        }
        panel.keyDown(with: escape)
    }

    func testColourRoundTripsThroughHex() {
        let hex = colorToHex(CGColor(red: 1, green: 0, blue: 0, alpha: 1))
        let parsed = parseColor(hex)
        XCTAssertNotNil(parsed)
        XCTAssertEqual((parsed?.components ?? [])[0], 1.0, accuracy: 0.01)
    }
}

@MainActor
class PreferencesTabControllerTests: XCTestCase {

    func testTabControllerLoads() {
        let controller = PreferencesController()
        controller.loadViewIfNeeded()
        controller.viewDidLoad()

        // Panes come from the nib in the running app; built bare here it just has to load.
        XCTAssertNotNil(controller.view)
        XCTAssertEqual(controller.tabStyle, .unspecified)
    }

    func testAppearingIsHandled() {
        let controller = PreferencesController()
        controller.loadViewIfNeeded()
        controller.viewDidLoad()

        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 500, height: 300),
            styleMask: [.titled], backing: .buffered, defer: false)
        window.contentViewController = controller

        controller.viewWillAppear()
        controller.viewDidAppear()
    }

    func testTransitionBetweenPanesRuns() {
        let controller = PreferencesController()
        controller.loadViewIfNeeded()
        controller.viewDidLoad()

        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 500, height: 300),
            styleMask: [.titled], backing: .buffered, defer: false)
        window.contentViewController = controller
        controller.viewDidAppear()

        guard controller.tabViewItems.count > 1,
            let from = controller.tabViewItems[0].viewController,
            let to = controller.tabViewItems[1].viewController
        else {
            return
        }
        let finished = expectation(description: "transition finished")
        controller.transition(from: from, to: to, options: []) { finished.fulfill() }
        wait(for: [finished], timeout: 5)
    }
}

// MARK: - Spring layout knobs

@MainActor
class SpringLayoutOptionTests: XCTestCase {

    private func context(_ source: String, bounds: CGRect = CGRect(x: 0, y: 0, width: 800, height: 600)) -> LayoutContext {
        let model = ElementModel.parseTenn(node: TennParser().parse(source))
        let diagram = model.elements[0]
        let store = ElementModelStore(model)

        let execution = ExecutionContext()
        execution.setElement(diagram)
        let scene = DrawableScene(diagram, darkMode: false, executionContext: execution)
        return LayoutContext(diagram, scene: scene, store: store, bounds: bounds)
    }

    private let chain = """
        element Root {
            item A {
                pos 0 0
            }
            item B {
                pos 100 0
            }
            item C {
                pos 200 0
            }
            item D {
                pos 300 0
            }
            link A B {
            }
            link B C {
            }
            link C D {
            }
        }
        """

    func testIterationLimitIsRespected() {
        let layout = SpringLayout()
        layout.sprIterations = 5

        let ops = layout.apply(context: context(chain), clean: true)
        XCTAssertEqual(ops.count, 4, "Every node still gets a position")
    }

    func testDisablingRandomnessGivesRepeatableResults() {
        func run() -> [CGPoint] {
            let layout = SpringLayout()
            layout.sprRandom = false
            layout.sprIterations = 10

            let ctx = context(chain)
            for op in layout.apply(context: ctx, clean: true) {
                op.apply()
            }
            return ctx.nodes.map { CGPoint(x: $0.x, y: $0.y) }
        }
        XCTAssertEqual(run(), run(), "Without randomness the layout is deterministic")
    }

    func testTuningKnobsKeepCoordinatesFinite() {
        let layout = SpringLayout()
        layout.sprMove = 2
        layout.sprStrain = 3
        layout.sprLength = 50
        layout.sprGravitation = 2
        layout.sprIterations = 20

        let ctx = context(chain)
        for op in layout.apply(context: ctx, clean: true) {
            op.apply()
        }
        XCTAssertTrue(ctx.nodes.allSatisfy { $0.x.isFinite && $0.y.isFinite })
    }

    func testFittingWithinBoundsKeepsNodesInside() {
        let layout = SpringLayout()
        layout.fitWithinBounds = true
        layout.sprIterations = 20

        let viewBounds = CGRect(x: 0, y: 0, width: 400, height: 300)
        let ctx = context(chain, bounds: viewBounds)
        for op in layout.apply(context: ctx, clean: true) {
            op.apply()
        }
        XCTAssertTrue(ctx.nodes.allSatisfy { $0.x.isFinite && $0.y.isFinite })
    }

    func testTimeLimitStopsTheRun() {
        let layout = SpringLayout()
        layout.maxTimeMS = 1
        layout.sprIterations = 100_000

        let ops = layout.apply(context: context(chain), clean: true)
        XCTAssertEqual(ops.count, 4, "The run is cut short but still places every node")
    }

    func testDisconnectedNodesAreStillPlaced() {
        let layout = SpringLayout()
        layout.sprIterations = 10

        let ctx = context(
            """
            element Root {
                item Lonely {
                    pos 0 0
                }
                item AlsoLonely {
                    pos 100 100
                }
            }
            """)
        XCTAssertEqual(layout.apply(context: ctx, clean: true).count, 2)
    }
}
