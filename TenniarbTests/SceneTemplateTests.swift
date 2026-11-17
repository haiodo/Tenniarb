//
//  SceneTemplateTests.swift
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

/// The code components generate (spec 5c), as go/scene/template.go and its tests read it.
final class SceneTemplateTests: XCTestCase {

    /// Three components that generate text: Chip fills ${...} from its parameters, Card puts the nodes of
    /// the use into its slots, Calc evaluates an expression.
    private let templateDoc = """
        element "Chip" {
          kind Component
          export let label = "Chip"
          export let tone = plain
          export let extras = outlined
          template %{<span class="${tone} ${extras}">${label}</span>}
        }

        element "Card" {
          kind Component
          template {
            <div class="card">
              <slot/>
              <slot actions/>
            </div>
          }
          item Box {
            auto-layout column
            item Title { title Card }
            slot
            slot actions
          }
        }

        element "Calc" {
          kind Component
          export let n = 40
          template %{<i>${n + 12}</i>}
        }

        element "Panel" {
          kind Component
          export let width
          template %{<aside style="width: ${width}"></aside>}
        }

        element "Screen" {
          item col {
            auto-layout column
            use Chip as one
            use Chip as two { label "Save"; tone primary; extras rounded outlined }
            use Calc as sum { n 1 }
            use Card as card {
              item Body { title "<p>Hello</p>" }
              slot actions { item Go { title "<button>Go</button>" } }
            }
            use Panel as side { width 320 }
          }
        }
        """

    /// A screen of plain boxes and templated components: the code of the screen is the code of the uses,
    /// the boxes only place them.
    private let elementDoc = """
        element "Item" {
          kind Component
          export let text = "x"
          template %{<span>${text}</span>}
        }

        element "List" {
          kind Component
          template %{<ul><slot/></ul>}
          item Box { auto-layout column; slot }
        }

        element "Screen" {
          template %{<main><slot/></main>}
          item Page {
            auto-layout column
            use List as list {
              use Item as a { text "A" }
              use Item as b { text "B" }
            }
          }
        }
        """

    /// A workspace of one file, loaded alone as go/scene loads it.
    private func workspace(_ doc: String) throws -> Scene.Tree {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("tenniarb-scene-template-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        try doc.write(to: root.appendingPathComponent("tpl.tenn"), atomically: true, encoding: .utf8)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return try SceneLoader(root: root).load("tpl.tenn")
    }

    private func node(_ tree: Scene.Tree, _ path: String) throws -> Scene.Node {
        try tree.resolve(try XCTUnwrap(tree.main), path)
    }

    /// Renders the node at path with the app's evaluator, as "tenniarb gen" does.
    private func render(_ tree: Scene.Tree, _ path: String, _ values: [String: String] = [:]) throws -> String {
        try tree.render(node(tree, path), values: values, eval: sceneEvalVars)
    }

    private func messages(_ error: Error?) -> [String] {
        ((error as? Scene.TemplateError)?.errors ?? []).map(\.message)
    }

    /// The parameters render with their defaults, the values override them, and an undeclared name is an
    /// error that lists the parameters.
    func testDefaultsAndValues() throws {
        let tree = try workspace(templateDoc)
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        let chip = try node(tree, "Chip")
        XCTAssertNotNil(chip.template)
        XCTAssertNil(chip.prop("template"), "the template command is not left as a property")

        XCTAssertEqual(try render(tree, "Chip"), "<span class=\"plain outlined\">Chip</span>")
        XCTAssertEqual(
            try render(tree, "Chip", ["label": "Save", "tone": "primary", "extras": "rounded outlined"]),
            "<span class=\"primary rounded outlined\">Save</span>")

        XCTAssertThrowsError(try render(tree, "Chip", ["nope": "1"])) { error in
            XCTAssertEqual(messages(error), ["unknown parameter \"nope\" (parameters: \"label\", \"tone\", \"extras\")"])
        }
    }

    /// A use of the component gives the values and the slot content; a parameter may be a list of words.
    func testUse() throws {
        let tree = try workspace(templateDoc)
        XCTAssertEqual(try render(tree, "Screen.col.one"), "<span class=\"plain outlined\">Chip</span>")
        XCTAssertEqual(try render(tree, "Screen.col.two"), "<span class=\"primary rounded outlined\">Save</span>")
        XCTAssertEqual(try render(tree, "Screen.col.sum"), "<i>13</i>")
    }

    /// <slot/> takes the nodes the use passes as their titles, named slots too; a slot the use does not
    /// fill is an error.
    func testSlots() throws {
        let tree = try workspace(templateDoc)
        XCTAssertEqual(
            try render(tree, "Screen.col.card"),
            "<div class=\"card\">\n  <p>Hello</p>\n  <button>Go</button>\n</div>")

        XCTAssertThrowsError(try render(tree, "Card")) { error in
            XCTAssertEqual(
                messages(error).first, "the default slot has no content: pass nodes in the block of the use")
        }
    }

    /// A parameter without a default and without a value is an error; a value and the block of a use both
    /// give it.
    func testRequiredParameter() throws {
        let tree = try workspace(templateDoc)
        XCTAssertThrowsError(try render(tree, "Panel")) { error in
            XCTAssertEqual(messages(error).first, "element \"Panel\" needs \"width\": set it in the block of the use or with -set")
        }
        XCTAssertEqual(try render(tree, "Panel", ["width": "100%"]), "<aside style=\"width: 100%\"></aside>")
        XCTAssertEqual(try render(tree, "Screen.col.side"), "<aside style=\"width: 320\"></aside>")
    }

    /// An element without a template generates nothing, and the template command of an item stays a
    /// property: only an element reads it.
    func testNoTemplate() throws {
        let tree = try workspace(
            """
            element "Bare" {
              kind Component
              item Box { title Bare }
            }
            element "Screen" {
              item Box { template %{<b>x</b>}; width 10 }
            }
            """)
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        XCTAssertNil(try node(tree, "Bare").template)
        XCTAssertThrowsError(try render(tree, "Bare")) { error in
            XCTAssertEqual(messages(error), ["the element \"Bare\" has no template"])
        }
        let box = try node(tree, "Screen.Box")
        XCTAssertEqual(box.prop("template")?.name, "template", "template inside an item is a property")
        XCTAssertNil(box.template)
    }

    /// A second template of one element is a warning, and the first one is used.
    func testSecondTemplate() throws {
        let tree = try workspace(
            """
            element "Two" {
              kind Component
              template %{<a/>}
              template %{<b/>}
            }
            """)
        XCTAssertEqual(try node(tree, "Two").template?.text, "<a/>")
        let warning = try XCTUnwrap(tree.diagnostics.first { $0.message.contains("second template") })
        XCTAssertTrue(warning.warn)
        XCTAssertEqual(warning.message, "the element has a second template: the first one is used")
    }

    /// A template command without text is an error of the load, not a property of the element.
    func testTemplateNeedsText() throws {
        let tree = try workspace(
            """
            element "T" {
              kind Component
              template
            }
            """)
        let failure = try XCTUnwrap(tree.diagnostics.first { !$0.warn })
        XCTAssertEqual(failure.message, "template needs text: template { ... } or template %{ ... }")
        XCTAssertNil(try node(tree, "T").template)
    }

    /// A use inside the slot of another prints its own template with the values of its own block, not its
    /// name.
    func testNestedUse() throws {
        let tree = try workspace(elementDoc)
        XCTAssertTrue(tree.diagnostics.isEmpty, "\(tree.diagnostics)")
        XCTAssertEqual(try render(tree, "Screen.Page.list"), "<ul><span>A</span>\n<span>B</span></ul>")
    }

    /// A component that prints itself through the slot of its own use stops at the nesting limit.
    func testRecursionLimit() throws {
        let tree = try workspace(
            """
            element "Loop" {
              kind Component
              template %{<l><slot/></l>}
              item Box {
                slot {
                  use Loop as self
                }
              }
            }
            """)
        XCTAssertThrowsError(try render(tree, "Loop")) { error in
            XCTAssertEqual(messages(error).first, "the templates nest too deep: a use inside the slot of itself?")
        }
        XCTAssertTrue(tree.diagnostics.isEmpty, "a render adds nothing to the tree: \(tree.diagnostics)")
    }

    /// A use of a component with no template prints nothing in the code of an element, and a plain item
    /// prints the code of its content where it stands.
    func testUseWithoutTemplate() throws {
        let tree = try workspace(
            """
            element "Bare" {
              kind Component
              item Box { title Bare }
            }
            element "Plain" {
              kind Component
              export let text = "x"
              template %{<b>${text}</b>}
            }
            element "Screen" {
              item col {
                use Bare as bare
                item Wrapper {
                  item Deep {
                    use Plain as p { text "P" }
                  }
                }
              }
            }
            """)
        let screen = try node(tree, "Screen")
        XCTAssertEqual(try tree.renderElement(screen, eval: sceneEvalVars), "<b>P</b>")
    }

    /// A block template is read as written, with the indentation of the component cut: the code starts at
    /// column 0, and a line deeper than the first keeps its extra indentation.
    func testBlockDedent() throws {
        let tree = try workspace(
            """
            element "Page" {
              kind Component
              export let label = "Hi"
              template {
                <div>
                    <p>${label}</p>
                </div>
              }
            }
            """)
        let page = try node(tree, "Page")
        XCTAssertEqual(page.template?.text, "<div>\n    <p>${label}</p>\n</div>")
        XCTAssertEqual(try render(tree, "Page"), "<div>\n    <p>Hi</p>\n</div>")

        let tabs = try workspace(
            "element \"T\" {\n  kind Component\n  template {\n\t\titem a { title A }\n\t\t\titem b { title B }\n  }\n}\n")
        XCTAssertEqual(try node(tabs, "T").template?.text, "item a { title A }\n\titem b { title B }")
        XCTAssertEqual(try render(tabs, "T"), "item a { title A }\n\titem b { title B }")
    }

    /// "%{ ... %}" is what a reader writes, and the percent of the closing marker does not land in the
    /// generated text.
    func testMarkdownTemplateClosing() throws {
        let tree = try workspace(
            """
            element "B" {
              kind Component
              export let title = "Save"
              template %{
            <button>${title}</button>
            %}
            }
            """)
        let b = try node(tree, "B")
        XCTAssertEqual(b.template?.text, "<button>${title}</button>")
        XCTAssertEqual(try render(tree, "B"), "<button>Save</button>")
    }

    /// A template written as a string is its text, as a markdown literal is.
    func testQuotedTemplate() throws {
        let tree = try workspace(
            """
            element "B" {
              kind Component
              export let label = "x"
              template "<b>${label}</b>"
            }
            """)
        XCTAssertEqual(try node(tree, "B").template?.text, "<b>${label}</b>")
        XCTAssertEqual(try render(tree, "B"), "<b>x</b>")
    }

    /// The template evaluates its ${...} with the evaluator it is given, and reports one it cannot
    /// evaluate instead of printing it.
    func testExpressionAndNoEvaluator() throws {
        let tree = try workspace(
            """
            element "Calc" {
              kind Component
              export let n = 1
              template %{<b>${n + 1}</b>}
            }
            """)
        let calc = try node(tree, "Calc")
        XCTAssertEqual(
            try tree.render(calc, eval: { code, vars in code == "n + 1" && vars?["n"] == "1" ? ("2", true) : ("", false) }),
            "<b>2</b>")

        XCTAssertThrowsError(try tree.render(calc)) { error in
            XCTAssertEqual(messages(error), ["${n + 1}: the expression is not evaluated"])
        }
    }

    /// An element prints the templates of the uses below it in file order, and its own template wraps them
    /// in the default slot; a template given instead replaces it.
    func testRenderElement() throws {
        let tree = try workspace(elementDoc)
        let screen = try node(tree, "Screen")
        XCTAssertEqual(
            try tree.renderElement(screen, eval: sceneEvalVars),
            "<main><ul><span>A</span>\n<span>B</span></ul></main>")
        XCTAssertEqual(
            try tree.renderElement(screen, template: Scene.Template(text: "<x>\n<slot/>\n</x>"), eval: sceneEvalVars),
            "<x>\n<ul><span>A</span>\n<span>B</span></ul>\n</x>")
    }
}
