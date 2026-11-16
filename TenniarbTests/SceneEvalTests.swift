//
//  SceneEvalTests.swift
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

/// The value of a ${...} as text, as go/exec/scene_eval_test.go reads it.
final class SceneEvalTests: XCTestCase {

    func testExpressionGivesItsTextAndAThrowFails() {
        for (code, want) in [("1 + 2", "3"), ("`Hi ${'x'}`", "Hi x"), ("(24 * 2) + 'px'", "48px")] {
            let (got, ok) = sceneEvalVars(code, nil)
            XCTAssertTrue(ok, code)
            XCTAssertEqual(got, want, code)
        }
        XCTAssertFalse(sceneEvalVars("nosuch()", nil).1)
    }

    func testParameterIsAVariableAndANumberOnlyWhenWrittenAsOne() {
        let vars = ["size": "40", "who": "Bob", "kind": "primary", "code": "007"]
        for (code, want) in [("size + 12", "52"), ("kind", "primary"), ("`Hi ${who}`", "Hi Bob"), ("code", "007")] {
            let (got, ok) = sceneEvalVars(code, vars)
            XCTAssertTrue(ok, code)
            XCTAssertEqual(got, want, code)
        }
        XCTAssertFalse(sceneEvalVars("nosuch", vars).1, "a name that is no variable still throws")
    }

    func testValueIsANumberOnlyInCanonicalForm() {
        XCTAssertEqual(sceneValue("44.4") as? Double, 44.4)
        XCTAssertEqual(sceneValue("12") as? Double, 12)
        XCTAssertEqual(sceneValue("007") as? String, "007")
        XCTAssertEqual(sceneValue("1.50") as? String, "1.50")
        XCTAssertEqual(sceneValue("1e5") as? String, "1e5")
        XCTAssertEqual(sceneValue("12pt") as? String, "12pt")
    }

    func testContextKeysAreOneObjectNamedCtx() {
        let vars = ["size": "40", "ctx.theme": "dark", "ctx.radius": "12", "ctx.label": "Save"]
        for (code, want) in [
            ("ctx.theme", "dark"),
            ("ctx.radius * 2", "24"),
            (##"ctx.theme === "dark" ? "#131925" : "#f0f4ff""##, "#131925"),
            ("`${ctx.label}: ${size}`", "Save: 40"),
            ("typeof ctx", "object"),
        ] {
            let (got, ok) = sceneEvalVars(code, vars)
            XCTAssertTrue(ok, code)
            XCTAssertEqual(got, want, code)
        }
        XCTAssertEqual(sceneEvalVars("ctx.nope", vars).0, "undefined", "a missing key is undefined")
    }
}
