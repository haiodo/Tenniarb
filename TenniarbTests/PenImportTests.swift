//
//  PenImportTests.swift
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
//

import Tenn
import XCTest

@testable import Tenniarb

/// The text go/penimport writes for go/penimport/testdata/layout.pen.
private let layoutText = #"""
    item Root {
    	auto-layout column; gap 8; padding 10 20; align center; width 300; color #ffffff; border-color #000000; line-width 2; corner-radius 6; pos 10 -20
    	item Title { title "Hello \"quoted\" world"; font-size 20; line-height 24; layout left; text-color #102030 }
    	item Row {
    		auto-layout row; gap 4; justify space-between; width fill; height 40
    		item A { width 30; height 20; color #ff0000 }
    		item B { width fill; height 20; color #00ff00 }
    		item C { width fill; height fill; color #0000ff }
    	}
    	item Hug {
    		auto-layout row; gap 3; padding 5; align end
    		item Label { title "Some longer label that wraps over lines"; font-size 12; layout left; width 80 }
    		item Small { title x; font-size 10; layout left }
    	}
    	item Abs {
    		auto-layout abs; width 200; height 100; clip yes
    		item A1 { width 50; height 20; pos 5 -7 }
    		item Dot { width 20; height 20; corner-radius 10; color #00ff00; pos 60 -7 }
    		item A2 {
    			auto-layout column; justify center; width 80; height fill; pos 100 -50
    			item A3 { width fill; height 10 }
    		}
    	}
    	item Empty { width 10; height 12; shadow 0 -2 4 #00000040 }
    }
    item Note {
    	title "line one
    line two"
    	font-size 14; layout left; auto-layout row; pos 400 0
    }
    """# + "\n"

/// The text go/penimport writes for go/penimport/testdata/components.pen.
private let componentsText = #"""
    item Library {
    	auto-layout column; gap 20
    	item Card {
    		auto-layout column; gap 4; padding 8; width 200; color #eeeeee
    		item Title { title "Card title"; font-size 14; layout left; text-color #111111 }
    		item Body {
    			auto-layout column; gap 2
    			item Line { title "Body text"; font-size 12; layout left }
    			item Ghost { hidden yes; title hidden; font-size 14; layout left }
    		}
    		item Footer {
    			auto-layout row; gap 4
    			item Ok { width 40; height 20; color #00aa00 }
    			item Cancel { hidden yes; width 40; height 20; color #aa0000 }
    		}
    	}
    	item Wrapper {
    		auto-layout column; gap 6
    		use Library.Card as "Inner card" {
    			at Title { title "Inner title" }
    			at Footer.Ok { width 55 }
    		}
    		item Caption { title caption; font-size 10; layout left }
    	}
    }
    item Screen {
    	auto-layout abs; width 900; height 600; pos 500 0
    	use Library.Card as Plain { pos 10 -10 }
    	use Library.Card as Changed {
    		width 220; pos 250 -10
    		at Title { title "A much longer card title here"; font-size 18 }
    		at Footer.Ok { width 60; color #0000ff }
    		at Footer.Cancel { hidden no }
    	}
    	use Library.Card as "No footer" {
    		pos 10 -200
    		at Body { padding 4 6 }
    		at Footer { hidden yes }
    	}
    	use Library.Card as Replaced {
    		pos 250 -200
    		replace Footer.Ok {
    			item Fresh {
    				auto-layout column; width 10; height 30
    				item Inside { width 4; height 4 }
    			}
    		}
    	}
    	use Library.Wrapper as Nested {
    		pos 10 -400
    		at Inner_card { width 150 }
    		at Inner_card.Title { title "Deep title" }
    		at Inner_card.Footer.Ok { width 70 }
    	}
    	use Screen.Plain as "Plain 2" { pos 250 -400 }
    }
    """# + "\n"

/// The text go/penimport writes for go/penimport/testdata/slots.pen.
private let slotsText = #"""
    item Window {
    	auto-layout column; gap 6; padding 10; width 300; color #ffffff
    	item Title { title Window; font-size 16; layout left }
    	item Content {
    		auto-layout column; gap 4; width fill
    		slot Content {
    			item Placeholder { title "Nothing here"; font-size 12; layout left }
    		}
    	}
    	item Extra { auto-layout row; gap 3; slot Extra }
    	item Bar { auto-layout row; gap 3; slot Bar }
    }
    item Screens {
    	auto-layout column; gap 30; pos 500 0
    	use Window as Filled {
    		at Title { title "Filled window" }
    		slot Content {
    			item One { width fill; height 20; color #ff0000; line-width 0 }
    			item Two { width fill; height 30; color #00ff00; line-width 0 }
    		}
    		slot Extra {
    			item Added { width 15; height 15 }
    		}
    	}
    	use Window as Emptied {
    		slot Content
    		slot Bar {
    			item Btn { width 40; height 16 }
    		}
    	}
    	use Window as Default
    }
    """# + "\n"

/// The text go/penimport writes for go/penimport/testdata/strokes.pen.
private let strokesText = #"""
    item Screen {
    	auto-layout column; gap 4; padding 4; width 300; height 300
    	item Uniform { width 40; height 12; color #ffffff; border-color #ff0000; line-width 2 }
    	item Top { width 40; height 12; color #ffffff; border-color #00ff00 }
    	item Zero { width 40; height 12; color #ffffff; border-color #0000ff; line-width 0 }
    	item NoWidth { width 40; height 12; color #ffffff; border-color #0000ff; line-width 0 }
    	item Clear { width 40; height 12; color #ffffff; border-color #ff000000 }
    	item Shape { width 40; height 12; path "M0 0L40 12"; view-box 0 0 40 12; color #ffffff; border-color #ff0000; line-width 2 }
    	item Inner { width 40; height 12; color #ffffff; border-color #ff0000; line-width 2 }
    	item Center { width 40; height 12; color #ffffff; border-color #ff0000; line-width 2 }
    	item Outer { width 40; height 12; color #ffffff; border-color #ff0000; line-width 2 }
    }
    """# + "\n"

/// The text go/penimport writes for go/penimport/testdata/imports.pen.
private let importsText = #"""
    element lib --from-file lib.tenn
    context mode light
    item Page {
    	auto-layout column; gap 12; padding 16; width 400; pos 20 -30
    	use lib.Kit_Buttons.Button as Save {
    		at Icon { hidden no }
    		at Label { title Save }
    	}
    	use lib.Kit_Buttons.Button as Cancel {
    		at Label { title Cancel; text-color #222222 }
    	}
    	use lib.Kit_Buttons.Row as Pair {
    		at Button { height 40 }
    	}
    	item "missing: Gone" { width 20; height 20; absolute yes }
    	use Local_top as Local { width 100 }
    }
    item "Local top" {
    	auto-layout row; padding 4; pos 600 0
    	item Name { title "2024"; font-size 12; layout left }
    }
    """# + "\n"

/// The text go/penimport writes for go/penimport/testdata/lib.pen (converted with imports.pen).
private let importsLibText = #"""
    context mode light
    item "Kit/Buttons" {
    	auto-layout column; gap 10
    	item Button {
    		auto-layout row; gap 6; padding 6 12; align center; color #3366ff; corner-radius 4
    		item Icon { hidden yes; width 12; height 12; color #ffffff }
    		item Label { title Button; font-size 14; layout left; text-color ${ctx.mode === "dark" ? "#111111" : "#ffffff"} }
    	}
    	item Row {
    		auto-layout row; gap 8; use Kit_Buttons.Button as Button
    		use Kit_Buttons.Button as "Button 2" {
    			at Label { title Second }
    		}
    	}
    }
    """# + "\n"

/// The text go/penimport writes for go/penimport/testdata/lib.pen.
private let libText = #"""
    context mode light
    item "Kit/Buttons" {
    	auto-layout column; gap 10
    	item Button {
    		auto-layout row; gap 6; padding 6 12; align center; color #3366ff; corner-radius 4
    		item Icon { hidden yes; width 12; height 12; color #ffffff }
    		item Label { title Button; font-size 14; layout left; text-color ${ctx.mode === "dark" ? "#111111" : "#ffffff"} }
    	}
    	item Row {
    		auto-layout row; gap 8; use Kit_Buttons.Button as Button
    		use Kit_Buttons.Button as Button {
    			at Label { title Second }
    		}
    	}
    }
    """# + "\n"

final class PenImportTests: XCTestCase {

    // MARK: - Fixture conversion

    private func fixture(_ name: String) throws -> String {
        let bundle = Bundle(for: PenImportTests.self)
        guard let url = bundle.url(forResource: name, withExtension: "pen", subdirectory: "PenFixtures") else {
            throw XCTSkip("no fixture \(name).pen in \(bundle.bundlePath)")
        }
        return url.path
    }

    private func convert(_ name: String) throws -> (PenImport.Result, URL) {
        try convert(paths: [try fixture(name)])
    }

    /// Converts the files and writes them the way the import does, into a directory of its own.
    private func convert(paths: [String]) throws -> (PenImport.Result, URL) {
        let result = try PenImport.convert(paths: paths)
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("pen-\(UUID().uuidString)")
        addTeardownBlock { try? FileManager.default.removeItem(at: dir) }
        try result.write(to: dir.path)
        return (result, dir)
    }

    /// convert for a fixture written in the test.
    private func convertJSON(_ body: String) throws -> (PenImport.Result, URL, PenImport.File) {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("pen-inline-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: dir) }
        let src = dir.appendingPathComponent("inline.pen")
        try body.write(to: src, atomically: true, encoding: .utf8)
        let result = try PenImport.convert(paths: [src.path])
        let out = dir.appendingPathComponent("out")
        try result.write(to: out.path)
        return (result, out, result.files[result.files.count - 1])
    }

    private func fileNamed(_ res: PenImport.Result, _ name: String) -> PenImport.File? {
        res.files.first { $0.name == name }
    }

    /// Loads the converted file the way a document does: a diagnostic that is not a warning fails.
    @discardableResult
    private func load(_ dir: URL, _ f: PenImport.File) throws -> Scene.Tree {
        let tree = try SceneLoader(root: dir).load(f.name + ".tenn")
        XCTAssertEqual(tree.fatalCount, 0, "\(f.name): \(tree.diagnostics)")
        return tree
    }

    // MARK: - The fixtures

    func testLayout() throws {
        let (res, dir) = try convert("layout")
        let f = try XCTUnwrap(fileNamed(res, "layout"))
        try load(dir, f)
        let out = res.tenn(of: f)
        XCTAssertEqual(out, layoutText)
        // y points up: pen y 20 is pos y -20
        for want in [
            "pos 10 -20", "auto-layout column", "justify space-between", "padding 10 20", "width fill",
            "height fill", #"title "Hello \"quoted\" world""#, "auto-layout abs", "clip yes", "shadow 0 -2 4 #00000040",
            "corner-radius 10", "pos 5 -7",
        ] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
        XCTAssertEqual(f.unsupported["type group"], 1)
        XCTAssertEqual(f.unsupported["size fallback in fill_container"], 1)
        XCTAssertEqual(f.unsupported["ellipse of flexible width"], 1)
    }

    func testComponents() throws {
        let (res, dir) = try convert("components")
        let f = try XCTUnwrap(fileNamed(res, "components"))
        try load(dir, f)
        let out = res.tenn(of: f)
        XCTAssertEqual(out, componentsText)
        for want in [
            "use Library.Card as Plain", "at Title", "replace Footer.Ok", "at Footer.Ok",
            "use Library.Wrapper as Nested", "at Inner_card", "use Screen.Plain as \"Plain 2\"",
            "at Footer.Cancel { hidden no }", "replace Footer",
        ] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
        // a disabled node stays in the component, hidden, so that an instance can switch it on
        XCTAssertTrue(out.contains("item Ghost { hidden yes;"), "\(out)")
        XCTAssertFalse(out.contains("slot opt_"), "\(out)")
    }

    func testSlots() throws {
        let (res, dir) = try convert("slots")
        let f = try XCTUnwrap(fileNamed(res, "slots"))
        try load(dir, f)
        let out = res.tenn(of: f)
        XCTAssertEqual(out, slotsText)
        for want in ["slot Content {", "slot Bar {", "slot Extra {"] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
    }

    func testImports() throws {
        let (res, dir) = try convert("imports")
        let lib = try XCTUnwrap(fileNamed(res, "lib"))
        let f = try XCTUnwrap(fileNamed(res, "imports"))
        try load(dir, lib)
        try load(dir, f)
        // the library has to come first: the file mounts it
        XCTAssertTrue(res.files[0] === lib, "the library must come first")
        let out = res.tenn(of: f)
        XCTAssertEqual(out, importsText)
        for want in [
            "element lib --from-file lib.tenn", "use lib.Kit_Buttons.Button as Save", "at Icon { hidden no }",
            "missing: Gone",
        ] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
        XCTAssertEqual(f.placeholders.count, 1)
        XCTAssertEqual(f.warnings.count, 1)
        XCTAssertTrue(f.variables["accent"] == nil, "\(f.variables)")
        XCTAssertTrue(lib.variables["accent"] != 0, "\(lib.variables)")
        XCTAssertTrue(lib.variables["bg"] != 0, "\(lib.variables)")
        XCTAssertEqual(res.tenn(of: lib), importsLibText)
        // a value the file writes per theme stays a switch on the context; the first value of the axis is
        // the one a node outside a theme sees, so the light theme keeps its color and the dark one is reachable
        let libOut = res.tenn(of: lib)
        XCTAssertTrue(libOut.contains("color #3366ff"), "\(libOut)")
        XCTAssertTrue(
            libOut.contains(##"text-color ${ctx.mode === "dark" ? "#111111" : "#ffffff"}"##), "\(libOut)")
        XCTAssertEqual(f.unsupported["override of an id that is not in the file or library"], 1)
    }

    /// The names of a library are settled by the files that import it: one converted alone keeps two
    /// instances of Button as they are.
    func testLibraryAlone() throws {
        let (res, _) = try convert("lib")
        let f = try XCTUnwrap(fileNamed(res, "lib"))
        XCTAssertEqual(res.tenn(of: f), libText)
    }

    /// A copy of the pen files is written by the importer, so the fixtures need only the .pen source.
    func testEveryFixtureConvertsAndLoads() throws {
        var problems: [String] = []
        for name in ["layout", "components", "slots", "strokes", "imports", "lib"] {
            do {
                let (res, dir) = try convert(name)
                for f in res.files {
                    do {
                        let tree = try SceneLoader(root: dir).load(f.name + ".tenn")
                        if tree.fatalCount > 0 {
                            problems.append("\(name)/\(f.name): \(tree.diagnostics)")
                        }
                    } catch {
                        problems.append("\(name)/\(f.name): \(error)")
                    }
                }
            } catch {
                problems.append("\(name): \(error)")
            }
        }
        XCTAssertEqual(problems, [], problems.joined(separator: "\n"))
    }

    func testOutputParses() throws {
        for name in ["layout", "components", "slots", "strokes", "imports"] {
            let (res, _) = try convert(name)
            for f in res.files {
                let out = res.tenn(of: f)
                let node = TennParser().parse(out)
                let errors = parseErrors(out)
                XCTAssertEqual(errors, [], "\(f.name): \(errors)\n\(out)")
                let again = PenCompact.compact(node, indent: "\t", limit: PenCompact.limit, pack: true) + "\n"
                XCTAssertEqual(again, out, "\(f.name): printing the parsed file gives other text")
            }
        }
    }

    func testOutputIsIndentedWithTabs() throws {
        let (res, _) = try convert("layout")
        for f in res.files {
            let out = res.tenn(of: f)
            XCTAssertTrue(out.contains("\n\t"), "\(f.name) is not indented with tabs:\n\(out)")
            XCTAssertFalse(out.contains("\n  "), "\(f.name) is not indented with tabs:\n\(out)")
        }
    }

    // MARK: - Layout

    func testAbsoluteChildLeavesTheFlow() throws {
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"frame","id":"p","name":"Parent","layout":"vertical","gap":5,"padding":10,"children":[
              {"type":"rectangle","id":"a","name":"A","width":40,"height":20},
              {"type":"rectangle","id":"b","name":"Badge","layoutPosition":"absolute","x":30,"y":40,"width":8,"height":9},
              {"type":"rectangle","id":"c","name":"C","width":40,"height":20}]}]}
            """#)
        try load(dir, f)
        let out = res.tenn(of: f)
        XCTAssertTrue(out.contains("absolute yes"), "\(out)")
        XCTAssertTrue(out.contains("pos 30 -40"), "\(out)")
    }

    func testNamesStayUnambiguous() throws {
        // two siblings with one name, names that normalize alike, a name that starts with a digit,
        // and a decoy named like the first segment of a path
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"frame","id":"c","name":"Card","reusable":true,"layout":"vertical","children":[
              {"type":"rectangle","id":"r1","name":"Row","width":10,"height":10},
              {"type":"rectangle","id":"r2","name":"Row","width":11,"height":11},
              {"type":"rectangle","id":"r3","name":"a b","width":12,"height":12},
              {"type":"rectangle","id":"r4","name":"a_b","width":13,"height":13},
              {"type":"rectangle","id":"r5","name":"2024","width":14,"height":14}]},
             {"type":"frame","id":"pg","name":"Page","x":300,"layout":"vertical","children":[
              {"type":"frame","id":"decoy","name":"Card","children":[
               {"type":"ref","id":"u","name":"Use","ref":"c","descendants":{"r2":{"width":30},"r4":{"height":31},"r5":{"width":32},"r1":{"height":33}}}]}]}]}
            """#)
        try load(dir, f)
        let out = res.tenn(of: f)
        XCTAssertEqual(out.components(separatedBy: "item Row {").count - 1, 1, "\(out)")
        XCTAssertTrue(out.contains(#"item "Row 2""#), "\(out)")
    }

    func testTextLineHeightAndAlign() throws {
        // 14 * 1.25 = 17.5 rounds to 18 in gfx/pen; the file carries the box, not a difference to the font's own
        let (res, _, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[{"type":"frame","id":"p","name":"P","width":400,"height":300,"children":[
             {"type":"text","id":"a","name":"A","content":"x","fontSize":14,"lineHeight":1.25},
             {"type":"text","id":"b","name":"B","content":"x","textAlign":"center"},
             {"type":"text","id":"c","name":"C","content":"x","textAlign":"right"},
             {"type":"text","id":"d","name":"D","content":"x"}]}]}
            """#)
        let out = res.tenn(of: f)
        for want in ["line-height 18", "layout middle", "layout right", "layout left"] {
            XCTAssertTrue(out.contains(want), "no \(want) in:\n\(out)")
        }
        XCTAssertFalse(out.contains("line-spacing"), "\(out)")
    }

    func testRefCyclesAndMissingRefsGivePlaceholders() throws {
        let (_, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"frame","id":"a","name":"A","reusable":true,"children":[{"type":"ref","id":"ab","name":"ToB","ref":"b"}]},
             {"type":"frame","id":"b","name":"B","reusable":true,"children":[{"type":"ref","id":"ba","name":"ToA","ref":"a"}]},
             {"type":"ref","id":"x","name":"Use A","ref":"a","x":100},
             {"type":"ref","id":"y","name":"Nowhere","ref":"zzz","x":200}]}
            """#)
        XCTAssertFalse(f.warnings.isEmpty, "\(f.warnings)")
        XCTAssertTrue(f.unsupported["unresolved ref", default: 0] > 0, "\(f.unsupported)")
        try load(dir, f)
    }

    func testTextWithBackslashesParses() throws {
        let (res, _, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[{"type":"text","id":"t","name":"T","content":"a\\\"b\\","fontSize":14}]}
            """#)
        let errors = parseErrors(res.tenn(of: f))
        XCTAssertTrue(errors.isEmpty, "\(errors)\n\(res.tenn(of: f))")
    }

    func testOverrideReachesSlotFallbackOfNestedInstance() throws {
        // Row holds an instance of Cell; Cell has a slot with fallback content. The screen changes the
        // fallback text of one Row and fills the slot of the other, which needs an at on the nested instance.
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"frame","id":"cell","name":"Cell","reusable":true,"layout":"vertical","children":[
              {"type":"frame","id":"cs","name":"Cells","layout":"vertical","slot":[],"children":[
               {"type":"text","id":"fb","name":"Fallback","content":"short","fontSize":14}]}]},
             {"type":"frame","id":"row","name":"Row","reusable":true,"layout":"vertical","children":[
              {"type":"ref","id":"r1","name":"Inner","ref":"cell"}]},
             {"type":"frame","id":"scr","name":"Screen","x":300,"layout":"vertical","gap":10,"children":[
              {"type":"ref","id":"a","name":"Edited","ref":"row","descendants":{"fb":{"content":"a much longer fallback text"}}},
              {"type":"ref","id":"b","name":"Filled","ref":"row","descendants":{"cs":{"children":[
               {"type":"rectangle","id":"n1","name":"Box","width":50,"height":40}]}}}]}]}
            """#)
        try load(dir, f)
        let out = res.tenn(of: f)
        for want in ["at Inner.Cells.Cells.Fallback { title", "at Inner {\n", "slot Cells {\n"] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
        for (k, n) in f.unsupported where k.contains("fallback") || k.contains("not found") {
            XCTFail("unsupported \(k) (\(n))")
        }
    }

    func testPathOpacityAndTitle() throws {
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"rectangle","id":"top","name":"Top","width":20,"height":20},
             {"type":"frame","id":"fr","name":"Frame","x":100,"layout":"horizontal","opacity":0.5,"children":[
              {"type":"rectangle","id":"in","name":"Inside","width":20,"height":20},
              {"type":"path","id":"ic","name":"Icon","geometry":"M0 0L10 10","viewBox":[0,0,10,10],"fillRule":"evenodd","width":16,"height":16,"fill":"#112233"},
              {"type":"path","id":"ic2","name":"Line","geometry":"M0 0L8 8","width":8,"height":8,"stroke":"#445566","strokeWidth":2}]}]}
            """#)
        let out = res.tenn(of: f)
        for want in [
            #"item Top { title """#, "opacity 0.5", #"path "M0 0L10 10""#, "view-box 0 0 10 10", "fill-rule evenodd",
            "color #112233", #"path "M0 0L8 8""#, "view-box 0 0 8 8", "border-color #445566",
        ] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
        XCTAssertEqual(out.components(separatedBy: #"title """#).count - 1, 1, "\(out)")
        try load(dir, f)
        XCTAssertEqual(f.unsupported["opacity"], nil)
        XCTAssertEqual(f.unsupported["path geometry"], nil)
    }

    func testFontAndPerCornerRadius() throws {
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"frame","id":"cmp","name":"Cmp","reusable":true,"layout":"vertical","cornerRadius":[8,8,0,0],"children":[
              {"type":"text","id":"t","name":"T","content":"hello","fontFamily":"Menlo","fontWeight":"700","fontStyle":"italic","letterSpacing":0.5,"fontSize":14},
              {"type":"text","id":"p","name":"Plain","content":"plain","fontFamily":"Menlo","fontWeight":"normal","fontStyle":"normal","letterSpacing":0,"fontSize":14}]},
             {"type":"frame","id":"scr","name":"Screen","x":300,"layout":"vertical","children":[
              {"type":"ref","id":"a","name":"Inst","ref":"cmp","descendants":{"t":{"fontWeight":"normal","letterSpacing":0},"p":{"fontWeight":600}}}]}]}
            """#)
        let out = res.tenn(of: f)
        for want in [
            "font-family Menlo", "font-weight 700", "font-style italic", "letter-spacing 0.5", "corner-radius 8 8 0 0",
            "font-weight normal", "font-weight 600",
        ] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
        // defaults are written only in overrides
        XCTAssertEqual(out.components(separatedBy: "font-weight normal").count - 1, 1, "\(out)")
        XCTAssertFalse(out.contains("font-style normal"), "\(out)")
        try load(dir, f)
        for k in ["fontFamily", "fontWeight", "fontStyle", "letterSpacing", "per-corner radius"] {
            XCTAssertNil(f.unsupported[k], "unsupported \(k)")
        }
    }

    func testGradientFill() throws {
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","variables":{"bg":{"type":"color","value":"#112233"}},"children":[
             {"type":"frame","id":"h","name":"Header","width":100,"height":40,"fill":{"type":"gradient","gradientType":"linear","enabled":true,"rotation":270,"size":{"height":1},
              "colors":[{"color":"$bg","position":0},{"color":"#ffffff","position":1}]}},
             {"type":"frame","id":"d","name":"Diag","y":50,"width":100,"height":40,"fill":[{"type":"gradient","rotation":135,
              "colors":[{"color":"#000000","position":0},{"color":"#808080","position":0.33},{"color":"#ffffff","position":1}]}]},
             {"type":"frame","id":"r","name":"Radial","y":100,"width":100,"height":40,"fill":{"type":"gradient","gradientType":"radial",
              "colors":[{"color":"#000000","position":0},{"color":"#ffffff","position":1}]}},
             {"type":"text","id":"t","name":"T","y":150,"content":"x","fill":{"type":"gradient","rotation":0,
              "colors":[{"color":"#111111","position":0},{"color":"#ffffff","position":1}]}}]}
            """#)
        let out = res.tenn(of: f)
        for want in [
            "color linear 90 #112233 0 #ffffff 1", "color linear 225 #000000 0 #808080 0.33 #ffffff 1",
            "color #000000;", "text-color #111111",
        ] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
        // the radial fill and the text color
        XCTAssertEqual(f.unsupported["gradient fill (first stop)"], 2, "\(f.unsupported)")
        try load(dir, f)
    }

    func testInstanceIgnoresPosOfComponent() throws {
        // the component sits at y 300 of the pen canvas; an instance at 0 0 in an absolute parent says no pos
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"frame","id":"c","name":"Comp","reusable":true,"x":0,"y":300,"width":50,"height":40,"layout":"none"},
             {"type":"frame","id":"scr","name":"Screen","x":500,"layout":"none","width":200,"height":100,"clip":true,"children":[
              {"type":"ref","id":"a","name":"Inst","ref":"c","x":0,"y":0}]}]}
            """#)
        try load(dir, f)
        let out = res.tenn(of: f)
        XCTAssertFalse(out.contains("use Comp as Inst { pos"), "\(out)")
        XCTAssertFalse(out.contains("pos 0 0"), "\(out)")
    }

    // Pen addresses a node inside a nested instance by the path "outer/inner": the bare id would not say
    // which instance it belongs to. Reading the path as an id lost the override and counted a reference to
    // a node the file holds as a reference to one it does not.
    func testNestedOverrideByPath() throws {
        let (res, _, f) = try convertJSON(
            #"""
            {"version":"2.22","children":[
             {"type":"frame","id":"cap","name":"Cap","reusable":true,"width":100,"height":20,"children":[
              {"type":"text","id":"label","name":"Label","content":"Button","fontSize":14,"fill":"#000000"}]},
             {"type":"frame","id":"card","name":"Card","reusable":true,"layout":"vertical","width":120,"height":40,"children":[
              {"type":"ref","id":"chip","name":"Chip","ref":"cap"}]},
             {"type":"ref","id":"inst","name":"Instance","ref":"card","x":0,"y":0,"descendants":{"chip/label":{"content":"nested"}}}]}
            """#)
        let out = res.tenn(of: f)
        XCTAssertTrue(out.contains("at Chip.Label {"), "\(out)")
        XCTAssertTrue(out.contains("title nested"), "\(out)")
        XCTAssertEqual(f.unsupported["override of an id that is not in the file or library"], nil, "\(f.unsupported)")
    }

    // MARK: - Themes

    // A pen theme is the context of .tenn: a node names the value for its subtree, and a variable the file
    // writes per theme reads it back, so an instance keeps the colors of its theme instead of the ones of
    // the file's first. Writing one frozen set lost the other themes of every instance.
    func testThemeBecomesContext() throws {
        let (res, _, f) = try convertJSON(
            #"""
            {"version":"2.22","themes":{"direction":["home","app"]},
             "variables":{"accent":{"type":"color","value":[
              {"value":"#c6f24e","theme":{"direction":"home"}},{"value":"#5b6cff","theme":{"direction":"app"}}]}},
             "children":[
              {"type":"frame","id":"card","name":"Card","reusable":true,"width":100,"height":40,"fill":"$accent"},
              {"type":"ref","id":"i1","name":"App card","ref":"card","x":0,"y":0,"theme":{"direction":"app"}}]}
            """#)
        let out = res.tenn(of: f)
        for want in [
            "context direction home",
            #"use Card as "App card" { context direction app }"#,
            ##"color ${ctx.direction === "app" ? "#5b6cff" : "#c6f24e"}"##,
        ] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
        XCTAssertNil(f.unsupported["theme on an instance (colors stay light)"], "\(f.unsupported)")
    }

    // The axes of an imported library are declared at the root of the file too: a variable declared there is
    // written into this file as an expression, and ctx has to exist for it to read.
    func testLibraryAxesReachTheRootContext() throws {
        let lib = PenFile(path: "", name: "lib", alias: "lib", raw: ["themes": ["mode": ["light", "dark"]] as PenMap])
        let f = PenFile(path: "", name: "app", alias: "app", raw: [:])
        f.imports = ["K": lib]
        let got = f.rootContext
        XCTAssertEqual(got.count, 1)
        XCTAssertEqual(got[0].name, "context")
        XCTAssertEqual(got[0].args.count, 2)
        XCTAssertEqual(got[0].args[0].s, "mode")
        XCTAssertEqual(got[0].args[1].s, "light")
    }

    // MARK: - Defaults and overrides

    func testDefaultsAreLeftOut() throws {
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"rectangle","id":"top","name":"Top","width":20,"height":20,"fill":"#ffffff"},
             {"type":"frame","id":"p","name":"P","x":100,"layout":"vertical","width":400,"height":300,"children":[
              {"type":"rectangle","id":"r","name":"R","width":20,"height":20,"fill":"#112233"},
              {"type":"rectangle","id":"s","name":"S","width":20,"height":20,"stroke":"#445566","strokeWidth":1},
              {"type":"ellipse","id":"e","name":"E","width":20,"height":20},
              {"type":"text","id":"t","name":"T","content":"x","fill":"#000000"},
              {"type":"text","id":"k","name":"K","content":"y","fill":"#ffffff"},
              {"type":"frame","id":"d","name":"D","padding":3,"layout":"horizontal","children":[
               {"type":"text","id":"w","name":"W","content":"z"}]}]}]}
            """#)
        let out = res.tenn(of: f)
        for gone in ["display", "color #00000000", "text-color #000000", "line-width 1", "item T { auto-layout"] {
            XCTAssertFalse(out.contains(gone), "\(gone) is written:\n\(out)")
        }
        // outside a layout the item has a white fill and a black border: line-width 0 says there is none
        XCTAssertEqual(out.components(separatedBy: "line-width 0").count - 1, 1, "\(out)")
        XCTAssertTrue(
            out.contains(#"item Top { title ""; width 20; height 20; color #ffffff; line-width 0 }"#), "\(out)")
        // a color on a dark fill and the layout of a node that has children stay
        XCTAssertTrue(out.contains("text-color #ffffff"), "\(out)")
        XCTAssertTrue(out.contains("item D {\n\t\tauto-layout row"), "\(out)")
        try load(dir, f)
    }

    func testOverrideRepeatingTheComponentIsLeftOut() throws {
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"frame","id":"cmp","name":"Cmp","reusable":true,"layout":"vertical","width":100,"height":50,"fill":"$bg","children":[
              {"type":"text","id":"lbl","name":"Lbl","content":"a","fontSize":14,"fill":"#222222"}]},
             {"type":"frame","id":"scr","name":"Screen","x":300,"layout":"vertical","children":[
              {"type":"ref","id":"a","name":"Same","ref":"cmp","width":100,"height":60,"fill":"$bg","descendants":{"lbl":{"content":"b","fontSize":14,"fill":"#333333"}}}]}],
             "variables":{"bg":{"type":"color","value":"#eeeeee"}}}
            """#)
        let out = res.tenn(of: f)
        // a value that names a variable stays: the theme of the instance may be another
        for want in ["use Cmp as Same {", "height 60", "color #eeeeee", "at Lbl { title b; text-color #333333 }"] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
        XCTAssertEqual(out.components(separatedBy: "color #eeeeee").count - 1, 2, "\(out)")
        guard let at = out.range(of: "use Cmp as Same") else {
            return XCTFail("no use in\n\(out)")
        }
        let block = String(out[at.lowerBound...])
        XCTAssertFalse(block.contains("width"), "\(block)")
        XCTAssertFalse(block.contains("font-size"), "\(block)")
        try load(dir, f)
    }

    // MARK: - Unsupported

    // A ref to a component the converter does not build leaves a placeholder of the room the component asks
    // for, and the warning names the type: an empty box of the right size keeps the layout of the frame
    // around it, and "the component is not there" would be a lie about a node the file holds.
    func testUnsupportedComponentKeepsItsRoom() throws {
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"group","id":"grp","name":"Group","reusable":true,"width":32,"height":32},
             {"type":"frame","id":"row","name":"Row","layout":"horizontal","width":200,"height":40,"children":[
              {"type":"ref","id":"u1","name":"Icon","ref":"grp","width":32,"height":32}]}]}
            """#)
        let out = res.tenn(of: f)
        XCTAssertTrue(out.contains(#"item "missing: Icon" { width 32; height 32; absolute yes }"#), "\(out)")
        let joined = f.warnings.joined(separator: "\n")
        XCTAssertTrue(
            joined.contains(#"the component is a "group", a type this converter does not build"#), "\(joined)")
        XCTAssertFalse(joined.contains("the component is not there"), "\(joined)")
        try load(dir, f)
    }

    // A text component carries the width of its sample and says it grows in a fixed width; an instance of it
    // that grows of itself must say so, or it takes the width of the sample and sticks out of the frame it
    // was placed in - which is what moved the taglines of the footer out of their column.
    func testTextInstanceDoesNotTakeTheSampleWidth() throws {
        let (res, _, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"text","id":"body14","name":"Body 14","reusable":true,"textGrowth":"fixed-width","width":640,
              "content":"Продаём, устанавливаем и обслуживаем зарядные станции","fontSize":14,"fill":"#000000"},
             {"type":"frame","id":"brand","name":"Brand","layout":"vertical","width":300,"height":200,"children":[
              {"type":"ref","id":"t1","name":"Tagline","ref":"body14","textGrowth":"auto","content":"Короткий текст","fill":"#000000"}]}]}
            """#)
        let out = res.tenn(of: f)
        XCTAssertTrue(out.contains("use "), "\(out)")
        XCTAssertTrue(out.contains("width hug"), "\(out)")
        if out.contains("width 640") && out.contains("Tagline") {
            for line in out.components(separatedBy: "\n") where line.contains("Tagline") && line.contains("640") {
                XCTFail("the sample width went into the instance: \(line)")
            }
        }
    }

    func testWriteLeavesOnlyTheDocument() throws {
        let (res, _, _) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"frame","id":"f","name":"Screen","width":100,"height":50}]}
            """#)
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("pen-write-\(UUID().uuidString)")
        addTeardownBlock { try? FileManager.default.removeItem(at: dir) }
        try res.write(to: dir.path)
        let names = try FileManager.default.contentsOfDirectory(atPath: dir.path)
        XCTAssertEqual(names.count, 1, "\(names)")
        XCTAssertTrue(names[0].hasSuffix(".tenn"), "\(names)")
    }

    // A stroke the file cannot draw as pen draws it is counted, and one that comes out the same is not: a
    // width the same on all four sides is a plain border, zero width and a colour of no alpha draw nothing,
    // and a path is stroked along its geometry, where pen centers the stroke too.
    func testStrokesThatComeOutTheSame() throws {
        let (res, dir) = try convert("strokes")
        let f = try XCTUnwrap(fileNamed(res, "strokes"))
        try load(dir, f)
        let out = res.tenn(of: f)
        for want in [
            #"item Uniform { width 40; height 12; color #ffffff; border-color #ff0000; line-width 2 }"#,
            #"item Zero { width 40; height 12; color #ffffff; border-color #0000ff; line-width 0 }"#,
            #"item NoWidth { width 40; height 12; color #ffffff; border-color #0000ff; line-width 0 }"#,
            #"path "M0 0L40 12"; view-box 0 0 40 12; color #ffffff; border-color #ff0000; line-width 2"#,
        ] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
        let want = [
            "per-side stroke width (a 1 point border on all four sides is drawn)": 1,
            "stroke alignment center (.tenn draws the stroke inside the box)": 3,
            "stroke alignment outer (.tenn draws the stroke inside the box)": 1,
        ]
        for (k, n) in want {
            XCTAssertEqual(f.unsupported[k], n, "\(k) is counted wrong in \(f.unsupported)")
        }
        let found = want.values.filter { $0 > 0 }.count
        XCTAssertEqual(f.unsupported.count, found, "more is counted than the strokes that are lost: \(f.unsupported)")
    }

    // A pen fill that is a picture is the picture of the box: .tenn has an image on an item and no paint
    // that is one, so the fill becomes `image ... fit`. A fill mode the format does not know keeps a warning.
    func testFillImageBecomesTheImageOfTheItem() throws {
        let (res, _, f) = try convertJSON(
            #"""
            {"version":"2.22","children":[
             {"type":"frame","id":"ph","name":"Photo","width":200,"height":100,
              "fill":{"type":"image","enabled":true,"url":"assets/photo.png","mode":"cover"}},
             {"type":"rectangle","id":"tp","name":"Tiled","width":100,"height":100,
              "fill":{"type":"image","enabled":true,"url":"assets/a.png","mode":"tile"}}]}
            """#)
        let out = res.tenn(of: f)
        XCTAssertTrue(out.contains(#"image "assets/photo.png" fit cover"#), "\(out)")
        XCTAssertFalse(out.contains(#"item Photo { width 200; height 100; color"#), "\(out)")
        XCTAssertEqual(f.unsupported["fill image mode tile"], 1, "\(f.unsupported)")
    }

    // MARK: - Icons

    // A pen icon node names a glyph of a library and keeps no geometry: the converter writes the path of
    // the glyph into the item, which is how go/scene draws a vector, so the .tenn file needs no icon
    // property and no file beside it.
    func testIconBecomesAPath() throws {
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","variables":{"ink":{"type":"color","value":"#112233"}},
             "children":[{"type":"icon","id":"ico","name":"Zap","reusable":true,"icon":"zap","library":"lucide",
              "width":32,"height":32,"fill":"$ink"}]}
            """#)
        let out = res.tenn(of: f)
        let d = try iconPath("zap")
        for want in [
            "item Zap {", "width 32; height 32", "path \"" + d + "\"", "view-box 0 0 24 24", "border-color #112233",
            "line-width 2.67",
        ] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
        // The stroke of the library is in the units of a 24 wide view box and the icon is 32 wide, so it
        // would be a third too thick at its own width.
        XCTAssertFalse(out.contains("line-width 2;"), "\(out)")
        XCTAssertFalse(out.contains("line-width 2\n"), "\(out)")
        XCTAssertTrue(f.placeholders.isEmpty, "\(f.placeholders)")
        XCTAssertEqual(f.unsupported["unresolved ref"], nil, "\(f.unsupported)")
        XCTAssertEqual(f.unsupported["type icon"], nil, "\(f.unsupported)")
        try load(dir, f)
    }

    // An instance of an icon component carries a glyph of its own - the size presets of a design are one
    // component showing different icons - and a size of its own. Both override the component.
    func testIconRefCarriesItsOwnGlyphAndSize() throws {
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","variables":{"ink":{"type":"color","value":"#112233"}},
             "children":[{"type":"icon","id":"ico","name":"UI / Icon / 20","reusable":true,"icon":"zap","library":"lucide",
              "width":20,"height":20,"fill":"$ink"},
             {"type":"frame","id":"row","name":"Row","x":100,"layout":"horizontal","gap":8,"children":[
              {"type":"ref","id":"u1","name":"Plus","ref":"ico","icon":"plus"},
              {"type":"ref","id":"u2","name":"Small","ref":"ico","icon":"check","width":12,"height":12}]}]}
            """#)
        let out = res.tenn(of: f)
        let plus = try iconPath("plus")
        let check = try iconPath("check")
        for want in [
            "use UI___Icon___20 as Plus { view-box 0 0 24 24; path \"" + plus + "\"", "path \"" + check + "\"",
            "width 12; height 12", "line-width 1",
        ] {
            XCTAssertTrue(out.contains(want), "no \(want) in\n\(out)")
        }
        XCTAssertTrue(f.placeholders.isEmpty, "\(f.placeholders)")
        XCTAssertEqual(f.unsupported["type icon"], nil, "\(f.unsupported)")
        try load(dir, f)
    }

    // A name the table does not have is a warning naming the icon, and the item keeps its room as an empty
    // box: the layout around it stays as it was drawn, and nothing panics.
    func testUnknownIconKeepsItsRoom() throws {
        let (res, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"icon","id":"ico","name":"Widget","reusable":true,"icon":"no-such-glyph","library":"lucide",
              "width":24,"height":24},
             {"type":"frame","id":"row","name":"Row","x":100,"layout":"horizontal","gap":8,"children":[
              {"type":"ref","id":"a","name":"One","ref":"ico"},
              {"type":"ref","id":"b","name":"Two","ref":"ico"}]}]}
            """#)
        let out = res.tenn(of: f)
        let joined = f.warnings.joined(separator: "\n")
        XCTAssertTrue(joined.contains(#"icon "no-such-glyph" is not in the icons library"#), "\(joined)")
        // One warning for the name, not one for the icon node and one for each of its instances.
        XCTAssertEqual(joined.components(separatedBy: "no-such-glyph").count - 1, 1, "\(joined)")
        XCTAssertFalse(out.contains("path \""), "\(out)")
        XCTAssertTrue(out.contains(#"item Widget { title ""; width 24; height 24 }"#), "\(out)")
        XCTAssertTrue(f.placeholders.isEmpty, "\(f.placeholders)")
        try load(dir, f)
    }

    // A library other than lucide is not in the table: the item keeps its room and the report says which
    // library was left out, instead of drawing an icon of the wrong set.
    func testUnknownIconLibrary() throws {
        let (_, dir, f) = try convertJSON(
            #"""
            {"version":"2.21","children":[
             {"type":"icon","id":"ico","name":"Widget","icon":"zap","library":"material","width":20,"height":20}]}
            """#)
        XCTAssertEqual(f.unsupported["icon library material"], 1, "\(f.unsupported)")
        try load(dir, f)
    }

    // The glyphs are the items of the shipped libraries/icons.tenn, not a table of the binary.
    func testIconsComeFromTheLibrary() throws {
        let set = PenIcons.table
        if let err = set.error { throw XCTSkip("the icons library is not readable: \(err)") }
        XCTAssertGreaterThanOrEqual(set.glyphs.count, 2000)
        let zap = try XCTUnwrap(set.glyphs["zap"])
        XCTAssertFalse(zap.paths.isEmpty)
        XCTAssertTrue(zap.paths[0].d.hasPrefix("M"), "\(zap.paths[0].d)")
        // A glyph with a filled detail keeps it as a path of its own, and the contour stays stroked.
        let tag = try XCTUnwrap(set.glyphs["tag"])
        XCTAssertEqual(tag.paths.count, 2)
        XCTAssertFalse(tag.paths[0].filled)
        XCTAssertTrue(tag.paths[1].filled)
        XCTAssertEqual(set.viewBox, [0, 0, 24, 24])
        XCTAssertEqual(set.stroke, 2)
    }

    private func iconPath(_ name: String) throws -> String {
        let set = PenIcons.table
        if let err = set.error { throw XCTSkip("the icons library is not readable: \(err)") }
        let g = try XCTUnwrap(set.glyphs[name], "the icons library has no \(name)")
        return g.paths[0].d
    }
}

/// The diagnostics of a parse, for the tests that only ask whether the text reads back.
private func parseErrors(_ text: String) -> [String] {
    let parser = TennParser()
    _ = parser.parse(text)
    return parser.errors.errors.map { "line \($0.line): \($0.message)" }
}
