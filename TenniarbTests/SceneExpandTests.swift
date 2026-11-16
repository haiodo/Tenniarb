//
//  SceneExpandTests.swift
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

import XCTest

@testable import Tenniarb

final class SceneExpandTests: XCTestCase {

    /// A workspace on disk; the loader reads mounts from it, as go/scene does.
    private func workspace(_ files: [String: String]) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-\(UUID().uuidString)", isDirectory: true)
        for (path, text) in files {
            let url = root.appendingPathComponent(path)
            try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try text.write(to: url, atomically: true, encoding: .utf8)
        }
        return root
    }

    func testUseOfComponentFromMountedLibraryFillsSlots() throws {
        let root = try workspace([
            "main.tenn": """
            element "Kit" --from-file "lib/kit.tenn"
            element "Screen" {
                use Kit.Window as win {
                    slot toolbar { item "Help" { height 5 } }
                    item "Email" { height 10 }
                }
                use Kit.Window as bare
            }
            """,
            "lib/kit.tenn": """
            element "base" --from-file "base.tenn"
            item "Window" {
                item "Title" { height 20 }
                slot "toolbar"
                slot { item "Empty" { height 20 } }
                use base.Dot as dot
            }
            """,
            "lib/base.tenn": """
            item "Dot" { width 6; height 6 }
            """,
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("main.tenn")
        let screen = try XCTUnwrap(tree.element("Screen"))
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")

        let win = try XCTUnwrap(screen.kids.first)
        XCTAssertTrue(win.inst)
        XCTAssertEqual(win.name, "win")
        XCTAssertEqual(win.kids.map(\.name), ["Title", "Help", "Email", "dot"])
        XCTAssertEqual(win.kids[1].prop("height")?.num(0), 5)
        XCTAssertTrue(win.kids[3].inst, "the component's own use of the nested library is an instance too")
        XCTAssertEqual(win.kids[3].def?.name, "Dot")

        let bare = try XCTUnwrap(screen.kids.last)
        XCTAssertEqual(bare.kids.map(\.name), ["Title", "Empty", "dot"], "an unfilled slot keeps its fallback")
    }

    func testParamDefaultAndOverride() throws {
        let root = try workspace([
            "p.tenn": """
            element "Button" {
                kind Component
                export let title = "Button"
                export let size = medium
                item "Label" { height 20 }
            }
            element "Screen" {
                use Button as plain
                use Button as save { title "Save" }
            }
            """
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("p.tenn")
        let screen = try XCTUnwrap(tree.element("Screen"))
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")

        let plain = try XCTUnwrap(screen.kids.first)
        let save = try XCTUnwrap(screen.kids.last)
        XCTAssertEqual(plain.vars, ["title": "Button", "size": "medium"], "the declaration's defaults reach the instance")
        XCTAssertEqual(save.vars, ["title": "Save", "size": "medium"], "the use wins over the default, the rest stays")
        XCTAssertEqual(save.kids.first?.vars?["title"], "Save", "a node of the body sees the values of its own instance")
        XCTAssertNil(save.prop("title"), "a parameter value is no property of the instance root")
        XCTAssertNil(plain.prop("title"))
    }

    func testMissingRequiredParameterIsTheGoDiagnostic() throws {
        let root = try workspace([
            "r.tenn": """
            element "C" {
                kind Component
                export let size
                item "Box" { width 10 }
            }
            element "Screen" { item "col" { use C as x } }
            """
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("r.tenn")
        _ = try XCTUnwrap(tree.element("Screen"))
        let diag = try XCTUnwrap(tree.diagnostics.first)
        XCTAssertEqual(diag.message, #"element "C" needs "size": set it in the block of the use"#)
        XCTAssertFalse(diag.warn)
    }

    func testAtAndReplaceReachIntoTheInstance() throws {
        let root = try workspace([
            "o.tenn": """
            element "Kit" {
                item "Card" {
                    item "Head" { height 10 }
                    item "Body" { height 20 }
                }
            }
            element "Screen" {
                use Kit.Card as c {
                    at Head { height 30 }
                    replace Body { item "New" { height 7 } }
                }
            }
            """
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("o.tenn")
        let screen = try XCTUnwrap(tree.element("Screen"))
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")

        let card = try XCTUnwrap(screen.kids.first)
        XCTAssertEqual(card.kids.map(\.name), ["Head", "New"])
        XCTAssertEqual(card.kids[0].prop("height")?.num(0), 30, "at puts its properties after the ones of the component")
        XCTAssertEqual(card.kids[1].prop("height")?.num(0), 7)
    }

    func testUseCycleIsTheGoDiagnostic() throws {
        let root = try workspace([
            "c.tenn": """
            element "Screen" {
                item "A" { use A as a }
            }
            """
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("c.tenn")
        _ = try XCTUnwrap(tree.element("Screen"))
        let diag = try XCTUnwrap(tree.diagnostics.first)
        XCTAssertEqual(
            diag.message,
            #"cycle: item "A" (c.tenn:2:5) -> use A as a (c.tenn:2:16) -> item "A" (c.tenn:2:5)"#)
    }
}
