//
//  LexerConformanceTests.swift
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

/// Both lexers implement TennLexerProtocol and must agree on every token stream,
/// so each case here runs twice - once per implementation.
class LexerConformanceTests: XCTestCase {

    private typealias Factory = (String) -> TennLexerProtocol

    private var factories: [(String, Factory)] {
        return [
            ("TennLexer", { TennLexer($0) }),
            ("SlowLexer", { SlowLexer($0) }),
        ]
    }

    /// Drain a lexer into a token list, stopping at eof.
    private func tokenize(_ lexer: TennLexerProtocol, limit: Int = 512) -> [TennToken] {
        var result: [TennToken] = []
        while result.count < limit {
            guard let token = lexer.getToken() else { break }
            result.append(token)
            if token.type == .eof { break }
        }
        return result
    }

    /// Run `body` against every lexer implementation, labelling failures with the name.
    private func forEachLexer(_ source: String, _ body: (TennLexerProtocol, String) -> Void) {
        for (name, make) in factories {
            body(make(source), name)
        }
    }

    private func types(_ source: String, _ make: Factory) -> [TennTokenType] {
        return tokenize(make(source)).map { $0.type }
    }

    private func literals(_ source: String, _ make: Factory) -> [String] {
        return tokenize(make(source)).filter { $0.type != .eof }.map { $0.literal }
    }

    // MARK: Symbols and numbers

    func testSymbolsAndEof() {
        forEachLexer("qwe asd") { lexer, name in
            let tokens = self.tokenize(lexer)
            XCTAssertEqual(tokens.map { $0.literal }, ["qwe", "asd", "\0"], name)
            XCTAssertEqual(tokens.map { $0.type }, [.symbol, .symbol, .eof], name)
            XCTAssertNil(lexer.getToken(), "\(name): nothing follows eof")
        }
    }

    func testIntegerLiterals() {
        forEachLexer("1 42 -7") { lexer, name in
            let tokens = self.tokenize(lexer).filter { $0.type != .eof }
            XCTAssertEqual(tokens.map { $0.literal }, ["1", "42", "-7"], name)
            XCTAssertEqual(tokens.map { $0.type }, [.intLit, .intLit, .intLit], name)
        }
    }

    func testFloatLiterals() {
        forEachLexer("1.5 -0.25") { lexer, name in
            let tokens = self.tokenize(lexer).filter { $0.type != .eof }
            XCTAssertEqual(tokens.map { $0.literal }, ["1.5", "-0.25"], name)
            XCTAssertEqual(tokens.map { $0.type }, [.floatLit, .floatLit], name)
        }
    }

    func testDigitsFollowedByLettersAreSymbols() {
        forEachLexer("12ab 1.2.3") { lexer, name in
            let tokens = self.tokenize(lexer).filter { $0.type != .eof }
            XCTAssertEqual(tokens.map { $0.type }, [.symbol, .symbol], "\(name): not valid numbers")
        }
    }

    func testEmojiIsASymbol() {
        forEachLexer("qwe 😈") { lexer, name in
            let tokens = self.tokenize(lexer).filter { $0.type != .eof }
            XCTAssertEqual(tokens.map { $0.literal }, ["qwe", "😈"], name)
            XCTAssertEqual(tokens.map { $0.type }, [.symbol, .symbol], name)
        }
    }

    // MARK: Strings

    func testDoubleQuotedString() {
        for (name, make) in factories {
            let tokens = tokenize(make("name \"hello world\"")).filter { $0.type != .eof }
            XCTAssertEqual(tokens.map { $0.type }, [.symbol, .stringLit], name)
            XCTAssertEqual(tokens[1].literal, "hello world", name)
        }
    }

    func testSingleQuotedString() {
        for (name, make) in factories {
            let tokens = tokenize(make("name 'hello'")).filter { $0.type != .eof }
            XCTAssertEqual(tokens.map { $0.type }, [.symbol, .stringLit], name)
            XCTAssertEqual(tokens[1].literal, "hello", name)
        }
    }

    func testUnterminatedStringReportsError() {
        for (name, make) in factories {
            var errors: [LexerError] = []
            var lexer = make("name \"unterminated")
            lexer.errorHandler = { error, _, _ in errors.append(error) }
            _ = tokenize(lexer)
            XCTAssertTrue(errors.contains(where: { if case .EndOfLineReadString = $0 { return true } else { return false } }), name)
        }
    }

    // MARK: Blocks and separators

    func testCurlyBlockTokens() {
        for (name, make) in factories {
            XCTAssertEqual(types("a { b }", make), [.symbol, .curlyLe, .symbol, .curlyRi, .eof], name)
        }
    }

    func testNestedCurlyBlocks() {
        for (name, make) in factories {
            let got = types("a { b { c } }", make)
            XCTAssertEqual(got.filter { $0 == .curlyLe }.count, 2, name)
            XCTAssertEqual(got.filter { $0 == .curlyRi }.count, 2, name)
        }
    }

    func testSemicolonSeparatesStatements() {
        for (name, make) in factories {
            XCTAssertEqual(types("a; b", make), [.symbol, .semiColon, .symbol, .eof], name)
        }
    }

    func testNewlineSeparatesStatements() {
        for (name, make) in factories {
            XCTAssertEqual(literals("a\nb", make), ["a", "\n", "b"], "\(name): newline emits a statement separator")
        }
    }

    // MARK: Comments

    func testLineComment() {
        for (name, make) in factories {
            XCTAssertEqual(literals("a // ignored\nb", make).filter { $0 != "\n" }, ["a", "b"], name)
        }
    }

    func testBlockComment() {
        for (name, make) in factories {
            XCTAssertEqual(literals("a /* ignored */ b", make), ["a", "b"], name)
        }
    }

    func testMultilineBlockComment() {
        for (name, make) in factories {
            XCTAssertEqual(literals("a /* line one\nline two */ b", make).filter { $0 != "\n" }, ["a", "b"], name)
        }
    }

    // MARK: Expressions, markdown and images

    func testInlineExpression() {
        for (name, make) in factories {
            let tokens = tokenize(make("x $(1 + 2)")).filter { $0.type != .eof }
            XCTAssertEqual(tokens.map { $0.type }, [.symbol, .expression], name)
            XCTAssertEqual(tokens[1].literal, "1 + 2", name)
        }
    }

    func testExpressionBlock() {
        for (name, make) in factories {
            let tokens = tokenize(make("x ${var a = 1; a}")).filter { $0.type != .eof }
            XCTAssertEqual(tokens.map { $0.type }, [.symbol, .expressionBlock], name)
            XCTAssertEqual(tokens[1].literal, "var a = 1; a", name)
        }
    }

    func testMarkdownLiteral() {
        for (name, make) in factories {
            let tokens = tokenize(make("doc %{**bold**}")).filter { $0.type != .eof }
            XCTAssertEqual(tokens.map { $0.type }, [.symbol, .markdownLit], name)
            XCTAssertEqual(tokens[1].literal, "**bold**", name)
        }
    }

    func testImageData() {
        for (name, make) in factories {
            let tokens = tokenize(make("img @(AAAA)")).filter { $0.type != .eof }
            XCTAssertEqual(tokens.map { $0.type }, [.symbol, .imageData], name)
            XCTAssertEqual(tokens[1].literal, "AAAA", name)
        }
    }

    func testDollarWithoutBracketIsPlainText() {
        for (name, make) in factories {
            let tokens = tokenize(make("$x")).filter { $0.type != .eof }
            XCTAssertFalse(tokens.contains(where: { $0.type == .expression }), "\(name): bare $ is not an expression")
        }
    }

    // MARK: Positions and revert

    func testTokenCarriesLineAndPosition() {
        for (name, make) in factories {
            let tokens = tokenize(make("a\nbb")).filter { $0.type == .symbol }
            XCTAssertEqual(tokens.count, 2, name)
            XCTAssertEqual(tokens[0].line, 0, name)
            XCTAssertEqual(tokens[1].line, 1, "\(name): second symbol is on the next line")
            XCTAssertEqual(tokens[1].size, 2, "\(name): token size is its literal length")
        }
    }

    func testRevertPutsTokenBack() {
        for (name, make) in factories {
            let lexer = make("a b")
            guard let first = lexer.getToken() else {
                XCTFail("\(name): expected a token")
                continue
            }
            lexer.revert(tok: first)
            let again = lexer.getToken()
            XCTAssertEqual(again?.literal, first.literal, "\(name): reverted token comes back first")
            XCTAssertEqual(lexer.getToken()?.literal, "b", name)
        }
    }

    func testEmptyInputYieldsOnlyEof() {
        for (name, make) in factories {
            XCTAssertEqual(types("", make), [.eof], name)
        }
    }

    func testWhitespaceOnlyInputYieldsOnlyEof() {
        for (name, make) in factories {
            XCTAssertEqual(types("   \t  ", make), [.eof], name)
        }
    }

    // MARK: Both lexers agree on a realistic document

    func testBothLexersProduceTheSameStreamForADiagram() {
        let source = """
            element "Root" {
                item First {
                    pos 10 20
                    color red
                    value $(1 + 2)
                }
                item Second {
                    // a comment
                    doc %{**text**}
                }
            }
            """
        let fast = tokenize(TennLexer(source))
        let slow = tokenize(SlowLexer(source))

        XCTAssertEqual(fast.map { $0.type }, slow.map { $0.type }, "Token types must match")
        XCTAssertEqual(fast.map { $0.literal }, slow.map { $0.literal }, "Token literals must match")
        XCTAssertEqual(fast.map { $0.line }, slow.map { $0.line }, "Line numbers must match")
    }

    func testSlowLexerParsesTheSameModelAsTennLexer() {
        let source = "element Root {\n  item A {\n    pos 1 2\n  }\n}"

        let fastParser = TennParser()
        let fast = fastParser.parse(source)

        let slowParser = TennParser()
        slowParser.factory = { SlowLexer($0) }
        let slow = slowParser.parse(source)

        XCTAssertFalse(fastParser.errors.hasErrors())
        XCTAssertFalse(slowParser.errors.hasErrors())
        XCTAssertEqual(fast.toStr(), slow.toStr(), "Both lexers must drive the parser to the same tree")
    }
}
