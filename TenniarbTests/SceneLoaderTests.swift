//
//  SceneLoaderTests.swift
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

final class SceneLoaderTests: XCTestCase {

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

    func testLoadsFileWithElementItemsAndProperties() throws {
        let root = try workspace([
            "board.tenn": """
            element "Board" {
                item "Title" {
                    title "Hello"
                    width 120
                    height 40
                }
                item "Empty"
            }
            """
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("board.tenn")
        let file = try XCTUnwrap(tree.main)
        XCTAssertEqual(file.kind, .file)
        XCTAssertEqual(file.elems.count, 1)

        let board = try XCTUnwrap(file.elems.first)
        XCTAssertEqual(board.kind, .element)
        XCTAssertEqual(board.name, "Board")
        XCTAssertEqual(board.key, "Board")
        XCTAssertTrue(board.parent === file)
        XCTAssertEqual(board.content.count, 2)

        let title = try XCTUnwrap(board.content.first)
        XCTAssertEqual(title.kind, .item)
        XCTAssertEqual(title.name, "Title")
        XCTAssertEqual(title.prop("title")?.text(0), "Hello")
        XCTAssertEqual(title.prop("width")?.num(0), 120)
        XCTAssertEqual(board.content[1].name, "Empty")
        XCTAssertTrue(board.content[1].props.isEmpty)
    }

    func testUseKeepsExportedParamsAndOverrides() throws {
        let root = try workspace([
            "lib.tenn": """
            element "Kit" {
                element "Button" {
                    export let title
                    export let size = 12
                    item "Label" {
                        title "Label"
                    }
                }
            }
            """,
            "board.tenn": """
            element "Board" {
                use Kit.Button as ok {
                    title "Войти"
                    size 16
                }
            }
            """,
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let lib = try SceneLoader(root: root).load("lib.tenn")
        let kit = try XCTUnwrap(lib.main?.elems.first)
        let button = try XCTUnwrap(kit.elems.first)
        XCTAssertEqual(button.params.count, 2)
        XCTAssertNil(button.param("title")?.defaultValue, "a parameter without a value is required")
        XCTAssertEqual(button.param("size")?.defaultValue?.num(0), 12)

        let tree = try SceneLoader(root: root).load("board.tenn")
        let board = try XCTUnwrap(tree.main?.elems.first)
        let use = try XCTUnwrap(board.content.first)
        XCTAssertEqual(use.kind, .use)
        XCTAssertEqual(use.path, "Kit.Button")
        XCTAssertEqual(use.name, "ok")
        XCTAssertEqual(use.block.count, 2)
        XCTAssertEqual(use.block[0].kind, .prop)
        XCTAssertEqual(use.block[0].prop?.name, "title")
        XCTAssertEqual(use.block[0].prop?.text(0), "Войти")
        XCTAssertEqual(use.block[1].prop?.num(0), 16)
    }

    func testNamedAndDefaultSlots() throws {
        let root = try workspace([
            "board.tenn": """
            element "Window" {
                item "Root" {
                    slot "toolbar"
                    slot {
                        item "Fallback" {
                            title "Empty"
                        }
                    }
                }
            }
            element "Board" {
                use Window as win {
                    slot toolbar {
                        item "Help"
                    }
                }
            }
            """
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("board.tenn")
        let window = try XCTUnwrap(tree.main?.elems.first)
        let rootItem = try XCTUnwrap(window.content.first)
        XCTAssertEqual(rootItem.content.count, 2)

        let named = rootItem.content[0]
        XCTAssertEqual(named.kind, .slot)
        XCTAssertEqual(named.name, "toolbar")
        XCTAssertTrue(named.content.isEmpty)

        let fallback = rootItem.content[1]
        XCTAssertEqual(fallback.kind, .slot)
        XCTAssertEqual(fallback.name, "")
        XCTAssertEqual(fallback.content.first?.name, "Fallback")

        let board = try XCTUnwrap(tree.main?.elems.last)
        let fill = try XCTUnwrap(board.content.first)
        XCTAssertEqual(fill.kind, .use)
        XCTAssertEqual(fill.block.count, 1)
        XCTAssertEqual(fill.block[0].kind, .slot)
        XCTAssertEqual(fill.block[0].slot, "toolbar")
        XCTAssertEqual(fill.block[0].block.count, 1)
        XCTAssertEqual(fill.block[0].block[0].node?.name, "Help")
    }

    func testMountResolvesRelativeToTheMountingFile() throws {
        let root = try workspace([
            "main.tenn": """
            element "Board" --from-file "sub/a.tenn"
            """,
            "sub/a.tenn": """
            element "A" {
                element "Inner" --from-file "b.tenn"
            }
            """,
            "sub/b.tenn": """
            element "B" {
                item "Leaf" {
                    title "B"
                }
            }
            """,
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("main.tenn")
        XCTAssertEqual(tree.order.count, 3)
        let aFile = try XCTUnwrap(tree.file("sub/a.tenn"))
        let bFile = try XCTUnwrap(tree.file("sub/b.tenn"))

        let board = try XCTUnwrap(tree.main?.elems.first)
        XCTAssertEqual(board.name, "Board")
        XCTAssertNotNil(board.mount, "the element mounts a file")
        XCTAssertTrue(board.mount === aFile)

        let a = try XCTUnwrap(board.elems.first)
        XCTAssertEqual(a.name, "A")
        XCTAssertTrue(a.parent === aFile, "mounted content keeps the scope of its own file")
        let inner = try XCTUnwrap(a.elems.first)
        XCTAssertEqual(inner.name, "Inner")
        XCTAssertTrue(inner.mount === bFile)
        XCTAssertEqual(inner.elems.first?.name, "B")
        XCTAssertEqual(inner.elems.first?.content.first?.name, "Leaf")
    }

    /// Two blocks on one line are joined into one command by the parser, so the second name is lost;
    /// the loader says so instead of dropping it quietly (go/scene/load.go joinedBlocks).
    func testTwoBlocksInOneCommandAreReported() throws {
        let root = try workspace([
            "a.tenn": "element A { item a { width 1 } item b { width 2 } }\n"
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("a.tenn")
        let warning = try XCTUnwrap(tree.diagnostics.first { $0.message.contains("blocks in one command") })
        XCTAssertTrue(warning.warn)
        XCTAssertEqual(
            warning.message, "2 blocks in one command \"item\" are joined: put each block on its own line")
        XCTAssertEqual(warning.pos.file, "a.tenn")
    }

    /// A ${...} that is not JavaScript is a load-time diagnostic with the line of the command, not an
    /// element that silently draws nothing (go/scene/exprcheck.go).
    func testBrokenExpressionIsReportedAndTheFileStillLoads() throws {
        let root = try workspace([
            "a.tenn": """
            element "Board" {
                item A {
                    width ${1 +}
                }
            }
            """
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("a.tenn")
        let reported = tree.diagnostics.filter { $0.message.contains("expression") }
        XCTAssertEqual(reported.count, 1)
        let broken = try XCTUnwrap(reported.first)
        XCTAssertTrue(broken.message.hasPrefix("expression \"1 +\": "), broken.message)
        XCTAssertGreaterThan(broken.message.count, "expression \"1 +\": ".count, "the compiler says why")
        XCTAssertEqual(broken.pos.file, "a.tenn")
        XCTAssertEqual(broken.pos.line, 3, "the line of the width command")
        XCTAssertEqual(broken.pos.col, 17, "the column of the expression text, as Go's")

        let board = try XCTUnwrap(tree.main?.elems.first)
        XCTAssertEqual(board.name, "Board")
        let item = try XCTUnwrap(board.content.first)
        XCTAssertEqual(item.name, "A")
        XCTAssertNotNil(item.prop("width"), "the diagnostic does not stop the load")
    }

    func testValidExpressionsAreNotReported() throws {
        let root = try workspace([
            "a.tenn": """
            element "Board" {
                item A {
                    width ${1 + 2}
                    title "${1 + 2}"
                }
            }
            """
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("a.tenn")
        XCTAssertTrue(tree.diagnostics.filter { $0.message.contains("expression") }.isEmpty)
    }

    /// The text of a template is code a component generates, not an expression of this file.
    func testBrokenExpressionInsideTemplateIsIgnored() throws {
        let root = try workspace([
            "a.tenn": """
            element "Board" {
                template {
                    width ${1 +}
                }
            }
            """
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("a.tenn")
        XCTAssertTrue(tree.diagnostics.filter { $0.message.contains("expression") }.isEmpty)
        let board = try XCTUnwrap(tree.main?.elems.first)
        XCTAssertEqual(board.template?.text, "width ${1 +}")
    }

    /// A library the board mounts is a file of its own: the diagnostic names it, as Go's does.
    func testBrokenExpressionInMountedLibraryNamesTheLibrary() throws {
        let root = try workspace([
            "board.tenn": """
            element "Board" {
                element "Kit" --from-file "tenniarb-library://exprcheck"
            }
            """,
            "libraries/exprcheck.tenn": """
            item A {
                width ${1 +}
            }
            """,
        ])
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }

        let tree = try SceneLoader(root: root).load("board.tenn")
        let reported = tree.diagnostics.filter { $0.message.contains("expression") }
        XCTAssertEqual(reported.count, 1)
        XCTAssertEqual(reported.first?.pos.file, ".tenniarb-library/exprcheck.tenn")

        let kit = try XCTUnwrap(tree.main?.elems.first?.elems.first)
        XCTAssertEqual(kit.name, "Kit")
        XCTAssertEqual(kit.content.first?.name, "A", "the library still loads its items")
    }

    /// The scene fixtures of web/ are files the engine must load without a false expression error.
    func testSceneFixturesHaveNoExpressionDiagnostics() throws {
        let fixtures = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("web/fixtures/scene")
        for name in ["context.tenn", "classic-values.tenn", "hero.tenn", "theme-styles.tenn"] {
            let tree = try SceneLoader(root: fixtures).load(name)
            let found = tree.diagnostics.filter { $0.message.hasPrefix("expression \"") }
            XCTAssertTrue(found.isEmpty, "\(name): \(found)")
        }
    }

}
