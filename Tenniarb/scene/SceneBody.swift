//
//  SceneBody.swift
//  Tenniarb
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

import Foundation
import Tenn
import cmkdown

extension Scene {
    /// The body of an item is the text under its title, as in go/render (spec 5d): markdown, 2 points
    /// smaller than the title, laid out in the item and grown to fit when no height is given.
    static let bodyFontDrop = 2.0
    static let bodyBlockGap = 5.0
    static let bodyMonoFace = "Menlo"
    static let bodyListIndent = 16.0
    static let bodyQuoteIndent = 12.0
    static let bodyCodePad = 8.0
    /// The width of the bar a quote of a body draws on the left.
    static let bodyBarW = 3.0
    static let bodyHeadSize1 = 6.0  // above the body, h2 +4, h3 +2
    static let bodyHeadGap1 = 10.0
    static let bodyHeadGap2 = 6.0

    /// One span of a line in one look: a markdown token after its glyphs are decided, or an inline picture
    /// of a classic diagram.
    struct BodyRun: Equatable {
        var text = ""
        var font = Font()
        var color = Color()  // the colour of the markup at this point of the line
        var code = false  // drawn over the code background
        var link = ""  // the target of the link the run is the label of, "" for plain text
        var img: TextImage?  // an inline picture: text is empty
    }

    /// The blocks of a body: a heading, a paragraph, a list, a quote, a code block and the table of
    /// contents of the headings.
    enum BodyBlockKind: UInt8 {
        case heading
        case paragraph
        case list
        case quote
        case code
        case toc
    }

    /// One line of the table of contents: a heading level and its text.
    struct BodyTOCItem {
        var level = 0
        var runs: [BodyRun] = []
    }

    struct BodyBlock {
        var kind = BodyBlockKind.paragraph
        var level = 0  // heading level 1..3
        var anchor = ""  // heading: the link target of the heading, see anchorOf
        var runs: [BodyRun] = []  // heading, paragraph, quote
        var items: [[BodyRun]] = []  // list: one run list per bullet
        var code: [String] = []  // code: source lines, wrapped at the width
        var toc: [BodyTOCItem] = []  // the table of contents, built by toc
    }

    /// One laid out line of the body.
    struct BodyLine {
        var x = 0.0  // indent of the line
        var w = 0.0  // width of its runs
        var asc = 0.0
        var desc = 0.0
        var h = 0.0  // line box height
        var gap = 0.0  // extra space before the line, between blocks
        var bullet = false  // a line of a list, measured with the head indent of its bullet
        var quote = false  // a bar on the left
        var code = false  // a background across the width
        var runs: [BodyRun] = []
    }

    /// The markdown body of an item, ready to be measured and drawn.
    final class BodyText {
        let m: TextMeasurer
        var font: Font  // body text, already 2 points below the title
        var mono: Font  // code
        var color: Color
        var lineH = 0.0  // 0: the font's rounded line box
        var gap = bodyBlockGap  // between blocks
        var padX = 0.0
        var padY = 0.0
        /// sep joins the lines of a paragraph: a space in a scene, where a line break is soft, and a
        /// newline in a classic diagram, where go/render breaks the line as written.
        var sep = ""
        var blocks: [BodyBlock] = []
        private var metrics: [Font: TextMetrics] = [:]
        private var widths: [WidthKey: Double] = [:]
        var pos = Pos()  // the body command, for a warning about its height
        var from = ""  // the markdown file the text was read from; "" for text written in the .tenn
        /// The classic inline pictures the body may name with @(name).
        var imgs: [String: ItemImage] = [:]
        /// The build this body belongs to: its measurer of line boxes and its classic profile.
        unowned let p: Prep

        private struct WidthKey: Hashable {
            var font: Font
            var text: String
        }

        init(m: TextMeasurer, font: Font, color: Color, p: Prep) {
            self.m = m
            self.font = font
            self.mono = font
            self.color = color
            self.p = p
        }

        /// Whether this body is the body of a classic diagram.
        var classic: Bool { p.classic }

        func metric(_ f: Font) -> TextMetrics {
            if let cached = metrics[f] {
                return cached
            }
            let out = m.measure("Hg", f)
            metrics[f] = out
            return out
        }

        func widthOf(_ s: String, _ f: Font) -> Double {
            let key = WidthKey(font: f, text: s)
            if let cached = widths[key] {
                return cached
            }
            let out = m.measure(s, f).width
            widths[key] = out
            return out
        }

        // MARK: - Blocks

        /// Splits markdown into blocks and their inline runs.
        func parse(_ text: String) {
            var blocks: [BodyBlock] = []
            var para: [String] = []
            var quote: [String] = []
            var list: BodyBlock?
            var code: [String] = []
            var inCode = false
            var seen: [String: Int] = [:]  // heading anchors seen, for the -2, -3 of a repeat
            func flushPara() {
                if !para.isEmpty {
                    blocks.append(BodyBlock(kind: .paragraph, runs: inline(para.joined(separator: lineSep()))))
                    para = []
                }
            }
            func flushQuote() {
                if !quote.isEmpty {
                    blocks.append(BodyBlock(kind: .quote, runs: inline(quote.joined(separator: lineSep()))))
                    quote = []
                }
            }
            func flushList() {
                if let current = list {
                    blocks.append(current)
                    list = nil
                }
            }
            for line in text.components(separatedBy: "\n") {
                let trimmed = line.trimmingCharacters(in: .whitespacesAndNewlines)
                if inCode {
                    if trimmed.hasPrefix("```") {
                        blocks.append(BodyBlock(kind: .code, code: code))
                        code = []
                        inCode = false
                    } else {
                        code.append(line)
                    }
                    continue
                }
                if trimmed.hasPrefix("```") {
                    flushPara()
                    flushQuote()
                    flushList()
                    inCode = true
                    continue
                }
                if trimmed.isEmpty {
                    flushPara()
                    flushQuote()
                    flushList()
                    continue
                }
                if let rest = quoteLine(trimmed) {
                    flushPara()
                    flushList()
                    quote.append(rest)
                    continue
                }
                if let (level, rest) = headingLine(trimmed) {
                    flushPara()
                    flushQuote()
                    flushList()
                    var runs = inline(rest)
                    let size = font.size + Scene.bodyHeadSize1 - 2 * Double(level - 1)
                    for i in runs.indices {
                        runs[i].font = weighted(runs[i].font, size)
                    }
                    let anchor = uniqueAnchor(&seen, anchorOf(runsText(runs)))
                    blocks.append(BodyBlock(kind: .heading, level: level, anchor: anchor, runs: runs))
                    continue
                }
                if let item = bulletLine(trimmed) {
                    flushPara()
                    flushQuote()
                    if list == nil {
                        list = BodyBlock(kind: .list)
                    }
                    list?.items.append(inline(item))
                    continue
                }
                flushList()
                flushQuote()
                para.append(trimmed)
            }
            flushPara()
            flushQuote()
            flushList()
            if inCode && !code.isEmpty {  // an unclosed fence is still code: the scene must not lose it
                blocks.append(BodyBlock(kind: .code, code: code))
            }
            self.blocks = blocks
        }

        /// Puts the headings before the body: levels up to depth, the body font less 2 points.
        func toc(_ depth: Int) {
            if depth <= 0 {
                return
            }
            let size = max(font.size - Scene.bodyFontDrop, 1)
            var items: [BodyTOCItem] = []
            for blk in blocks where blk.kind == .heading && blk.level <= depth {
                var runs = blk.runs
                for i in runs.indices {
                    runs[i].font.size = size
                }
                items.append(BodyTOCItem(level: blk.level, runs: runs))
            }
            if !items.isEmpty {
                blocks.insert(BodyBlock(kind: .toc, toc: items), at: 0)
            }
        }

        /// The lines of the body at width; a width of 0 or less does not wrap.
        func lines(_ width: Double) -> [BodyLine] {
            var out: [BodyLine] = []
            for blk in blocks {
                var lines: [BodyLine] = []
                switch blk.kind {
                case .heading, .paragraph:
                    lines = wrap(blk.runs, width, 0, 0)
                case .quote:
                    // The bar go/render draws on the left; only its indent moves the text.
                    lines = wrap(blk.runs, width, Scene.bodyQuoteIndent, Scene.bodyQuoteIndent)
                    for i in lines.indices {
                        lines[i].quote = true
                    }
                case .list:
                    let bulletW = widthOf("• ", font)
                    // go/render hangs no indent on a list of a classic body: the bullet stands where the text
                    // does and a wrapped line returns to the left edge.
                    let first = classic ? 0.0 : Scene.bodyListIndent
                    for (j, item) in blk.items.enumerated() {
                        var ls = wrap(
                            [BodyRun(text: "• ", font: font, color: color)] + item, width, first, first + bulletW)
                        if j == 0 && !out.isEmpty && !ls.isEmpty {
                            ls[0].gap = gap
                        }
                        for i in ls.indices {
                            ls[i].bullet = true
                        }
                        lines += ls
                    }
                    if lines.isEmpty {
                        continue
                    }
                case .toc:
                    for item in blk.toc {
                        let x = Double(item.level - 1) * Scene.bodyListIndent
                        lines += wrap(item.runs, width, x, x)
                    }
                    if lines.isEmpty {
                        continue
                    }
                case .code:
                    let inner = width - 2 * Scene.bodyCodePad
                    for src in blk.code {
                        for part in codeLines(src, inner) {
                            var line = BodyLine(
                                x: Scene.bodyCodePad, runs: [BodyRun(text: part, font: mono, color: color, code: true)])
                            finishLine(&line)
                            line.code = true
                            lines.append(line)
                        }
                    }
                    if lines.isEmpty {
                        continue
                    }
                }
                if !out.isEmpty && blk.kind != .list && !lines.isEmpty {
                    lines[0].gap = gap
                    if blk.kind == .heading {
                        switch blk.level {
                        case 1: lines[0].gap += Scene.bodyHeadGap1
                        case 2: lines[0].gap += Scene.bodyHeadGap2
                        default: break
                        }
                    }
                }
                out += lines
            }
            return out
        }

        /// The height of the body at width, its block gaps included.
        func height(_ width: Double) -> Double {
            var h = 0.0
            for line in lines(width) {
                h += line.gap + line.h
            }
            return h
        }

        /// The width of the widest line when nothing wraps.
        func natural() -> Double {
            var w = 0.0
            let indent = classicListIndent()
            for line in lines(.infinity) {
                var width = line.x + line.w
                if line.bullet {
                    width += indent
                }
                w = max(w, width)
            }
            return w
        }

        /// The head indent go/render puts on the paragraph of a bullet: five points, so the box of an item
        /// with a list is wider than its words by that much.
        private func classicListIndent() -> Double {
            classic ? 5 : 0
        }

        /// Joins the lines of one paragraph: a space where a line break is soft, a newline in a classic diagram.
        private func lineSep() -> String {
            sep.isEmpty ? " " : sep
        }

        // MARK: - Lines

        /// Breaks runs into lines greedily at spaces; text wider than the width stays on its line. A classic
        /// body breaks a word that does not fit, as go/render did. first is the indent of the first line, cont
        /// the indent of the continuation lines.
        private func wrap(_ runs: [BodyRun], _ width: Double, _ first: Double, _ cont: Double) -> [BodyLine] {
            var lines: [BodyLine] = []
            var line = BodyLine(x: first)
            var started = false
            func flush() {
                finishLine(&line)
                lines.append(line)
                line = BodyLine(x: cont)
                started = false
            }
            func put(_ text: String, _ run: BodyRun, _ w: Double) {
                if let last = line.runs.last, last.img == nil, last.font == run.font, last.color == run.color,
                    last.code == run.code, last.link == run.link
                {
                    line.runs[line.runs.count - 1].text += text
                } else {
                    line.runs.append(BodyRun(text: text, font: run.font, color: run.color, code: run.code, link: run.link))
                }
                line.w += w
                started = true
            }
            // classicWord breaks a word wider than the width by characters, at most the line each time.
            func classicWord(_ input: String, _ run: BodyRun, _ w: Double) -> Bool {
                var word = input
                if !classic || width <= 0 || w <= width + 1e-9 {
                    return false
                }
                while !word.isEmpty {
                    let avail = width - (line.x + line.w)
                    let whole = widthOf(word, run.font)
                    if whole <= avail + 1e-9 {
                        put(word, run, whole)
                        return true
                    }
                    var cut = fitPrefix(word, run.font, avail)
                    if cut.isEmpty {
                        if !started {
                            cut = firstGrapheme(word)
                        } else {
                            flush()
                            continue
                        }
                    }
                    put(cut, run, widthOf(cut, run.font))
                    word = String(word.dropFirst(cut.count))
                    flush()
                }
                return true
            }
            for run in runs {
                if let img = run.img {
                    if started && width > 0 && line.x + line.w + img.w > width + 1e-9 {
                        flush()
                    }
                    line.runs.append(run)
                    line.w += img.w
                    started = true
                    continue
                }
                for (i, seg) in run.text.components(separatedBy: "\n").enumerated() {
                    if i > 0 && started {
                        flush()  // a newline of the text is a break, not a space
                    }
                    for tok in splitSpaces(seg) {
                        let space = tok.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
                        let w = widthOf(tok, run.font)
                        if space && !started {
                            continue  // leading spaces of a line are not drawn
                        }
                        if !space && started && width > 0 && line.x + line.w + w > width + 1e-9 {
                            flush()
                        }
                        if !space && classicWord(tok, run, w) {
                            continue
                        }
                        put(tok, run, w)
                    }
                }
            }
            if started || lines.isEmpty {
                flush()
            }
            return lines
        }

        private func finishLine(_ line: inout BodyLine) {
            if p.classic {
                for run in line.runs {
                    let box = p.lineBox(run.font)
                    line.asc = max(line.asc, box.asc)
                    line.desc = max(line.desc, box.desc)
                }
                line.h = floor(line.asc + line.desc) + 1
            } else {
                for run in line.runs {
                    let m = metric(run.font)
                    line.asc = max(line.asc, m.ascent)
                    line.desc = max(line.desc, m.descent)
                }
                line.h = (line.asc + line.desc).rounded()
            }
            if lineH > 0 {
                line.h = lineH
            }
            for run in line.runs {
                if let img = run.img {
                    line.h = max(line.h, img.h)
                }
            }
        }

        /// The longest prefix of s that fits avail, in whole characters; "" when not even one does.
        private func fitPrefix(_ s: String, _ f: Font, _ avail: Double) -> String {
            if avail <= 0 {
                return ""
            }
            var cut = ""
            for g in s {
                if widthOf(cut + String(g), f) > avail + 1e-9 {
                    break
                }
                cut += String(g)
            }
            return cut
        }

        /// Breaks a code line at the width by characters: code has no spaces to break at. A width of 0 or less
        /// keeps the line whole.
        private func codeLines(_ src: String, _ width: Double) -> [String] {
            if width <= 0 || widthOf(src, mono) <= width + 1e-9 {
                return [src]
            }
            var out: [String] = []
            var line = ""
            for g in src {
                let next = line + String(g)
                if !line.isEmpty && widthOf(next, mono) > width + 1e-9 {
                    out.append(line)
                    line = String(g)
                    continue
                }
                line = next
            }
            out.append(line)
            return out
        }

        // MARK: - Inline

        /// Turns one line of markdown into runs, the runs of a link label carrying its target.
        func inline(_ s: String) -> [BodyRun] {
            var st = InlineState(font: font, color: color)
            var runs: [BodyRun] = []
            for piece in splitLinks(s) {
                runs += linkRuns(piece.text, piece.link, &st)
            }
            return runs
        }

        /// The colour and font in force inside a line: what !(...) and &(...) set until !() and &().
        private struct InlineState {
            var font = Font()
            var color = Color()
        }

        /// inline of text that is the label of a link, or plain text when link is "".
        private func linkRuns(_ s: String, _ link: String, _ st: inout InlineState) -> [BodyRun] {
            var runs: [BodyRun] = []
            func add(_ text: String, _ font: Font, _ color: Color, _ code: Bool) {
                if text.isEmpty {
                    return
                }
                if let last = runs.last, last.font == font, last.color == color, last.code == code, last.link == link {
                    runs[runs.count - 1].text += text
                    return
                }
                runs.append(BodyRun(text: text, font: font, color: color, code: code, link: link))
            }
            for t in MarkdownLexer.getTokens(code: normalizeInline(s)) {
                switch t.type {
                case .bold:
                    add(t.literal, styled(st.font, 700, false), st.color, false)
                case .italic:
                    add(t.literal, styled(st.font, 0, true), st.color, false)
                case .code:
                    add(t.literal, mono, st.color, true)
                case .image:
                    if let img = resolveInline(imgs, t.literal) {
                        runs.append(BodyRun(img: img))
                    } else {
                        add("@(" + t.literal + ")", st.font, st.color, false)
                    }
                case .color:
                    let (before, text, inline) = cut(t.literal, "|")
                    if inline {
                        add(text, st.font, markupColor(before) ?? color, false)
                    } else if t.literal.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                        st.color = color
                    } else if let c = markupColor(t.literal) {
                        st.color = c
                    } else {
                        st.color = color
                    }
                case .font:
                    let (size, text, inline) = cut(t.literal, "|")
                    if inline {
                        if let v = markupSize(size) {
                            add(text, sizedFont(st.font, v), st.color, false)
                        }
                    } else if let v = markupSize(t.literal) {
                        st.font = sizedFont(font, v)
                    } else {
                        st.font = font
                    }
                case .underline:
                    add("<" + t.literal + ">", st.font, st.color, false)
                case .scratch:
                    add("~" + t.literal + "~", st.font, st.color, false)
                case .eof:
                    break
                default:
                    add(t.literal, st.font, st.color, false)
                }
            }
            return runs
        }

        /// Reads the colour of !(...): a name of the cmkdown table or hex digits, reported once per build.
        private func markupColor(_ name: String) -> Color? {
            p.markupColor(pos, name)
        }

        /// A run font with a weight or italic. A face Name leaves both to the face, so the family takes over;
        /// otherwise **bold** would stay thin.
        private func styled(_ f: Font, _ weight: Int, _ italic: Bool) -> Font {
            var f = f
            if !f.name.isEmpty {
                f.family = cut(f.name, ",").0.trimmingCharacters(in: .whitespaces)
                f.name = ""
            }
            if weight != 0 {
                f.weight = weight
            }
            if italic {
                f.italic = true
            }
            return f
        }

        /// A heading run: the body weight of bold and a size of its own.
        private func weighted(_ f: Font, _ size: Double) -> Font {
            var f = styled(f, 700, false)
            f.size = size
            return f
        }
    }
}

// MARK: - Inline forms

/// A piece of a line: plain text, or the label of a link and its target.
private struct BodyPiece {
    let text: String
    let link: String  // the text inside ( ), as bodyLinks reads it; "" for plain text
}

/// The first character of s, so a line always makes progress.
private func firstGrapheme(_ s: String) -> String {
    s.first.map(String.init) ?? s
}

/// strings.Cut: the text before and after the first separator, and whether it was there.
func cut(_ s: String, _ sep: String) -> (String, String, Bool) {
    guard let r = s.range(of: sep) else { return (s, "", false) }
    return (String(s[..<r.lowerBound]), String(s[r.upperBound...]), true)
}

/// Splits text into runs of spaces and runs of other characters, as the renderer wraps.
private func splitSpaces(_ s: String) -> [String] {
    var out: [String] = []
    var current = ""
    var spaces: Bool?
    for ch in s {
        let isSpace = ch == " " || ch == "\t"
        if let was = spaces, was != isSpace {
            out.append(current)
            current = ""
        }
        spaces = isSpace
        current.append(ch)
    }
    if !current.isEmpty {
        out.append(current)
    }
    return out
}

/// Cuts a line at the explicit links [label](target), code spans left whole. An image is not a link.
private func splitLinks(_ s: String) -> [BodyPiece] {
    let chars = Array(s)
    var out: [BodyPiece] = []
    var plain = 0
    func flush(_ end: Int) {
        if end > plain {
            out.append(BodyPiece(text: String(chars[plain..<end]), link: ""))
        }
    }
    var i = 0
    while i < chars.count {
        if chars[i] == "`" {
            if let j = chars[(i + 1)...].firstIndex(of: "`") {
                i = j + 1
            } else {
                i += 1
            }
            continue
        }
        if chars[i] != "[" || (i > 0 && chars[i - 1] == "!") {
            i += 1
            continue
        }
        guard let mid = indexOf(chars, "](", from: i) else {
            i += 1
            continue
        }
        guard let end = chars[(mid + 2)...].firstIndex(of: ")") else {
            i += 1
            continue
        }
        var target = String(chars[(mid + 2)..<end]).trimmingCharacters(in: .whitespacesAndNewlines)
        if let sp = target.firstIndex(where: { $0 == " " || $0 == "\t" }) {  // an optional title after the path
            target = String(target[..<sp])
        }
        flush(i)
        out.append(BodyPiece(text: String(chars[(i + 1)..<mid]), link: target))
        i = end + 1
        plain = i
    }
    flush(chars.count)
    return out
}

/// The index of needle in chars at or after from.
private func indexOf(_ chars: [Character], _ needle: String, from: Int) -> Int? {
    let n = Array(needle)
    if n.isEmpty || chars.count < n.count {
        return nil
    }
    var i = from
    while i + n.count <= chars.count {
        if Array(chars[i..<(i + n.count)]) == n {
            return i
        }
        i += 1
    }
    return nil
}

/// Maps **bold**, *italic* and [label](url) to the forms the markdown lexer reads. A run of asterisks that
/// is not at the start of a word is left alone, as the lexer would.
private func normalizeInline(_ s: String) -> String {
    let chars = Array(s)
    func boundary(_ i: Int) -> Bool { i == 0 || chars[i - 1] == " " || chars[i - 1] == "\t" }
    var out = ""
    var i = 0
    while i < chars.count {
        if chars[i] == "`" {
            if let j = chars[(i + 1)...].firstIndex(of: "`") {
                out += String(chars[i...j])
                i = j + 1
            } else {
                out.append(chars[i])
                i += 1
            }
            continue
        }
        if matchesAt(chars, "**", i) && boundary(i) {
            if let j = indexOf(chars, "**", from: i + 2), j > i + 2 {
                out += "*" + String(chars[(i + 2)..<j]) + "*"
                i = j + 2
            } else {
                out.append(chars[i])
                i += 1
            }
            continue
        }
        if chars[i] == "*" && boundary(i) {
            if let j = chars[(i + 1)...].firstIndex(of: "*"), j > i + 1 {
                out += "_" + String(chars[(i + 1)..<j]) + "_"
                i = j + 1
            } else {
                out.append(chars[i])
                i += 1
            }
            continue
        }
        if chars[i] == "[" {
            if let mid = indexOf(chars, "](", from: i) {
                if let end = chars[(mid + 2)...].firstIndex(of: ")") {
                    out += String(chars[(i + 1)..<mid])
                    i = end + 1
                    continue
                }
            }
            out.append(chars[i])
            i += 1
            continue
        }
        out.append(chars[i])
        i += 1
    }
    return out
}

private func matchesAt(_ chars: [Character], _ prefix: String, _ at: Int) -> Bool {
    let p = Array(prefix)
    guard at + p.count <= chars.count else { return false }
    return Array(chars[at..<(at + p.count)]) == p
}

/// reads the size of &(...); anything that is not a number resets the size, as go/render does.
func markupSize(_ s: String) -> Double? {
    guard let v = Double(s.trimmingCharacters(in: .whitespacesAndNewlines)), v.isFinite else {
        return nil
    }
    return v
}

/// f at another size, its family and weight kept.
func sizedFont(_ f: Scene.Font, _ size: Double) -> Scene.Font {
    var f = f
    f.size = size
    return f
}

/// Reads "# ", "## " or "### "; more hashes are an ordinary paragraph.
private func headingLine(_ s: String) -> (Int, String)? {
    let chars = Array(s)
    var n = 0
    while n < chars.count && chars[n] == "#" {
        n += 1
    }
    if n == 0 || n > 3 || n == chars.count || chars[n] != " " {
        return nil
    }
    return (n, String(chars[(n + 1)...]).trimmingCharacters(in: .whitespacesAndNewlines))
}

/// Reads "- item", "* item" or "+ item".
private func bulletLine(_ s: String) -> String? {
    let chars = Array(s)
    if chars.count > 2, chars[0] == "-" || chars[0] == "*" || chars[0] == "+", chars[1] == " " {
        return String(chars[2...]).trimmingCharacters(in: .whitespacesAndNewlines)
    }
    return nil
}

/// Reads "> text": one level of quote, the bar is drawn by the layout.
private func quoteLine(_ s: String) -> String? {
    if s == ">" {
        return ""
    }
    let chars = Array(s)
    if chars.count > 1, chars[0] == ">", chars[1] == " " {
        return String(chars[2...]).trimmingCharacters(in: .whitespacesAndNewlines)
    }
    return nil
}

/// Reads an ATX heading: one to six hashes and a space.
private func headingAt(_ line: String) -> (Int, String)? {
    let chars = Array(line.trimmingCharacters(in: .whitespacesAndNewlines))
    var n = 0
    while n < chars.count && chars[n] == "#" {
        n += 1
    }
    if n == 0 || n > 6 || n == chars.count || chars[n] != " " {
        return nil
    }
    return (n, String(chars[(n + 1)...]))
}

/// A heading name as sections compare it: spaces collapsed, the closing hashes of an ATX heading dropped.
private func headingKey(_ s: String) -> String {
    var s = s.trimmingCharacters(in: .whitespacesAndNewlines)
    while s.hasSuffix("#") {
        s.removeLast()
    }
    return s.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
}

/// The link target of a heading: lowercase, a space becomes "-", other characters are dropped.
/// Cyrillic stays; "" when nothing is left.
func anchorOf(_ s: String) -> String {
    var out = ""
    var dash = false
    for u in s.unicodeScalars {
        switch u.properties.generalCategory {
        case .uppercaseLetter, .lowercaseLetter, .titlecaseLetter, .modifierLetter, .otherLetter,
            .decimalNumber:
            out += u.properties.lowercaseMapping
            dash = false
        default:
            guard u == "-" || u.properties.isWhitespace else { continue }
            if !out.isEmpty && !dash {
                out += "-"
                dash = true
            }
        }
    }
    return out.trimmingCharacters(in: CharacterSet(charactersIn: "-"))
}

/// Numbers a repeated anchor -2, -3 and so on in document order.
func uniqueAnchor(_ seen: inout [String: Int], _ base: String) -> String {
    if base.isEmpty {
        return ""
    }
    let n = seen[base, default: 0] + 1
    seen[base] = n
    return n > 1 ? "\(base)-\(n)" : base
}

/// The text of runs without their look.
func runsText(_ runs: [Scene.BodyRun]) -> String {
    runs.map(\.text).joined()
}

/// Lists the headings of markdown text as a reader sees them, with their anchors.
func headingsOf(_ text: String) -> (names: [String], anchors: Set<String>) {
    var names: [String] = []
    var anchors: Set<String> = []
    var seen: [String: Int] = [:]
    for line in text.components(separatedBy: "\n") {
        guard let (_, title) = headingAt(line) else { continue }
        let name = headingKey(title)
        if name.isEmpty {
            continue
        }
        names.append(name)
        let anchor = uniqueAnchor(&seen, anchorOf(name))
        if !anchor.isEmpty {
            anchors.insert(anchor)
        }
    }
    return (names, anchors)
}

/// The targets of the explicit links [label](target) of markdown text, code spans and fenced blocks skipped.
func bodyLinks(_ text: String) -> [String] {
    var out: [String] = []
    var inCode = false
    for line in text.components(separatedBy: "\n") {
        if line.trimmingCharacters(in: .whitespacesAndNewlines).hasPrefix("```") {
            inCode.toggle()
            continue
        }
        if inCode {
            continue
        }
        let chars = Array(line)
        var i = 0
        while i < chars.count {
            if chars[i] == "`" {
                if let j = chars[(i + 1)...].firstIndex(of: "`") {
                    i = j + 1
                } else {
                    i += 1
                }
                continue
            }
            if chars[i] != "[" || (i > 0 && chars[i - 1] == "!") {  // an image is not a section link
                i += 1
                continue
            }
            guard let mid = indexOf(chars, "](", from: i) else {
                i += 1
                continue
            }
            guard let end = chars[(mid + 2)...].firstIndex(of: ")") else {
                i += 1
                continue
            }
            var target = String(chars[(mid + 2)..<end]).trimmingCharacters(in: .whitespacesAndNewlines)
            if let sp = target.firstIndex(where: { $0 == " " || $0 == "\t" }) {  // an optional title after the path
                target = String(target[..<sp])
            }
            out.append(target)
            i = end + 1
        }
    }
    return out
}

/// Cuts the section under the heading name: the heading line and everything below it up to the next heading
/// of the same or a higher level, as a reader of the markdown file sees it.
private func bodySection(_ text: String, _ name: String) -> String? {
    let want = headingKey(name)
    let lines = text.components(separatedBy: "\n")
    var start = -1
    var level = 0
    for (i, line) in lines.enumerated() {
        guard let (n, title) = headingAt(line) else { continue }
        if start < 0 {
            if headingKey(title) == want {
                start = i
                level = n
            }
            continue
        }
        if n <= level {
            return lines[start..<i].joined(separator: "\n")
        }
    }
    if start < 0 {
        return nil
    }
    return lines[start...].joined(separator: "\n")
}

/// Lists the headings of a markdown file, for the message about a section that is not there.
private func bodyHeadings(_ text: String) -> [String] {
    var out: [String] = []
    for line in text.components(separatedBy: "\n") {
        if let (_, title) = headingAt(line) {
            out.append(headingKey(title))
        }
    }
    return out
}

/// Cuts the blank lines around text and the indentation its lines share, so the code of a body starts at
/// column 0 and reads the way it is written (load.go dedent).
private func dedent(_ text: String) -> String {
    let lines = text.trimmingCharacters(in: CharacterSet(charactersIn: "\n")).components(separatedBy: "\n")
    var indent = -1
    for line in lines where !line.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
        let n = line.prefix(while: { $0 == " " || $0 == "\t" }).count
        if indent < 0 || n < indent {
            indent = n
        }
    }
    if indent < 0 {
        indent = 0
    }
    var out: [String] = []
    for line in lines {
        if line.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            out.append("")
        } else {
            out.append(String(line.dropFirst(indent)))
        }
    }
    while let first = out.first, first.isEmpty { out.removeFirst() }
    while let last = out.last, last.isEmpty { out.removeLast() }
    return out.joined(separator: "\n")
}

/// Whether text holds a ${...} no backslash escapes - a help page writes its syntax so.
private func hasPlainExpr(_ text: String) -> Bool {
    let chars = Array(text)
    for i in 0..<chars.count {
        if chars[i] != "$" || i + 1 >= chars.count || chars[i + 1] != "{" {
            continue
        }
        if i > 0 && chars[i - 1] == "\\" {
            continue
        }
        return true
    }
    return false
}

/// Keeps the newlines go/render breaks at - a token's own newline or the end of a block token - and turns the
/// ones the lexer swallowed into an emphasis run into spaces.
private func classicBreaks(_ text: String) -> String {
    let chars = Array(text)
    var breaks: Set<Int> = []
    for t in MarkdownLexer.getTokens(code: text) {
        if t.type == .title || t.type == .bullet {
            breaks.insert(t.pos + t.size)  // the newline that closes the block is at its end
        }
        if !t.literal.contains("\n") {
            continue
        }
        // The lexer puts Pos at the literal it returns, so a newline of it is at Pos plus its offset.
        for (i, ch) in t.literal.enumerated() where ch == "\n" {
            breaks.insert(t.pos + i)
        }
    }
    var out = ""
    for (i, ch) in chars.enumerated() {
        if ch == "\n" && !breaks.contains(i) {
            out.append(" ")
            continue
        }
        out.append(ch)
    }
    return out
}

// MARK: - Reading the command

extension Scene.Prep {
    /// Reads the body command of rn and parses its markdown; nil when rn has no body or an empty one. The
    /// command has the forms of the classic renderer: body "text", body %{ ... }, body { text "..."; font-size 14; ... }
    /// and body --from-file "page.md", which reads the text from a markdown file.
    func bodyInfo(_ rn: Scene.RNode, _ st: Scene.Look, _ inCtx: Bool) -> Scene.BodyText? {
        guard let pr = prop(rn, "body") else {
            if let toc = prop(rn, "toc") {
                t.warn(toc.pos, "toc needs a body: it lists the headings of the body of the same item")
            }
            return nil
        }
        var size = 0.0
        var lineH = 0.0
        var blockGap = Scene.bodyBlockGap
        var family = ""
        var mono = Scene.bodyMonoFace
        var color = st.text
        var text = ""
        var from = ""
        func textArg(_ i: Int) -> String {
            if let val = pr.val {
                return i < val.count ? val[i] : ""
            }
            if i < pr.args.count {
                return markdownArg(pr.pos.file, pr.args[i])
            }
            return ""
        }
        if let block = pr.block {
            for c in bodyCommands(block) {
                let name = bodyCommandName(c)
                let args = bodyCommandArgs(c)
                switch name {
                case "text":
                    if !args.isEmpty {
                        text = markdownArg(pr.pos.file, args[0])
                    }
                case "font-size":
                    if let v = bodyArgNum(args, 0), v > 0 {
                        size = v
                    } else {
                        t.warn(pr.pos, "body font-size: not a number above 0, ignored")
                    }
                case "font-family":
                    family = bodyArgText(args, 0)
                case "mono":
                    let s = bodyArgText(args, 0)
                    if !s.isEmpty {
                        mono = s
                    }
                case "text-color":
                    let s = bodyArgText(args, 0)
                    if let c = Scene.parseColor(s) {
                        color = c
                    } else {
                        t.warn(pr.pos, "body text-color: \"\(s)\" is not a color")
                    }
                case "line-height":
                    if let v = bodyArgNum(args, 0), v > 0 {
                        lineH = v
                    } else {
                        t.warn(pr.pos, "body line-height: not a number above 0, ignored")
                    }
                case "gap":
                    if let v = bodyArgNum(args, 0), v >= 0 {
                        blockGap = v
                    } else {
                        t.warn(pr.pos, "body gap: not a number from 0, ignored")
                    }
                default:
                    break
                }
            }
        }
        let fromFile = bodyFileArg(pr.args)
        if fromFile.ok {
            let file = fromFile.path
            guard let read = bodyFileText(pr, file, true) else {
                return nil
            }
            text = read
            from = file
        } else if pr.block == nil {
            text = textArg(0)
        }
        if text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            return nil
        }
        if classic {
            // go/render reads a text `\n` of a body as a newline; a scene keeps the backslash (spec 4.3).
            text = text.replacingOccurrences(of: "\\n", with: "\n")
            // The ${...} of an old body is an expression of the element model. A body that writes \${...} shows
            // the syntax itself, and the model would swallow it, so such a text is kept as written.
            if hasPlainExpr(text) {
                text = evalText(rn, pr, text)
            }
        }
        text = dedent(text).replacingOccurrences(of: "\t", with: "    ")
        if classic {
            // After the dedent, so the offsets of the lexer are the offsets of this text.
            text = classicBreaks(text)
        }

        if size == 0 {
            size = titleSize(rn) - Scene.bodyFontDrop
        }
        if size <= 0 {
            size = 1
        }
        let body = Scene.BodyText(m: measurer, font: fontOf(rn, size), color: color, p: self)
        body.lineH = lineH
        body.gap = blockGap
        body.pos = pr.pos
        body.from = from
        if classic {
            body.imgs = inlineImages(rn)
            body.sep = "\n"  // a line break of a classic body is a break, as in go/render
        }
        if !family.isEmpty {
            body.font.family = family
            body.font.name = ""
        }
        body.mono = Scene.Font(family: mono, size: size)
        switch true {
        case inCtx:
            body.padX = 0
            body.padY = 0
        case classic:
            // go/render insets the body of a classic item by the same 4 points as its title.
            body.padX = Scene.classicTextPad
            body.padY = Scene.classicTextPad
        default:
            body.padX = Scene.textPadX
            body.padY = Scene.textPadY
        }
        body.parse(text)
        if let toc = prop(rn, "toc") {
            body.toc(tocDepth(toc))
        }
        var base = pr.pos.file
        if !from.isEmpty {
            let (file, _, _) = cut(from, "#")
            if !file.isEmpty {
                let (_, rel, inside) = t.resolveIn(pr.pos.file, file)
                if inside {
                    base = rel
                }
            }
        }
        checkBodyLinks(pr.pos, base, text)
        // skip: the hit boxes of the links and anchors of a body are bodylink.go.
        return body
    }

    /// Reads the depth of a toc command: how many heading levels it lists, 3 when it gives none.
    func tocDepth(_ pr: Scene.Prop) -> Int {
        if pr.args.isEmpty {
            return 3
        }
        guard let v = pr.num(0), v >= 1, v <= 6 else {
            t.fail(pr.pos, "toc takes a depth from 1 to 6, got \"\(bodyArgText(pr.args, 0))\"")
            return 0
        }
        return Int(v)
    }

    /// font-size of the item, the size the title is drawn with and the body is based on.
    func titleSize(_ rn: Scene.RNode) -> Double {
        if let fs = prop(rn, "font-size"), let v = fs.num(0), v > 0 {
            return v
        }
        return Scene.defaultFontSize
    }

    /// The text of a string or markdown literal argument. "%{ ... %}" is what a reader writes: the literal
    /// closes at "}", so the percent of the closing marker lands in the text and is cut here.
    func markdownArg(_ file: String, _ a: TennNode?) -> String {
        guard let a else { return "" }
        var text = a.getIdentText() ?? ""
        if let token = a.token, text.hasSuffix("%") {
            let at = token.pos + text.count
            if let src = t.source(file), at < src.count, src[src.index(src.startIndex, offsetBy: at)] == "}" {
                text.removeLast()
            }
        }
        return text
    }

    /// The path after --from-file among the arguments of a body command; ok is false when the command has no
    /// such flag, and the path is "" when the flag has none.
    func bodyFileArg(_ args: [TennNode]) -> (path: String, ok: Bool) {
        for (i, a) in args.enumerated() where a.getIdentText() == SceneLoader.fromFileFlag {
            return (bodyArgText(args, i + 1), true)
        }
        return ("", false)
    }

    /// The text of "body --from-file <path>": the whole markdown file, or the section named after "#". The
    /// path is relative to the file that writes the body and may not leave the workspace root.
    func bodyFileText(_ pr: Scene.Prop, _ ref: String, _ report: Bool) -> String? {
        let (file, section, hasSection) = cut(ref, "#")
        let flag = SceneLoader.fromFileFlag
        if file.isEmpty {
            if report {
                t.fail(pr.pos, "body \(flag) needs a file: body \(flag) \"docs/plan.md\"")
            }
            return nil
        }
        let (abs, rel, inside) = t.resolveIn(pr.pos.file, file)
        guard inside else {
            if report {
                t.fail(pr.pos, "body \(flag) \"\(file)\" leaves the workspace root")
            }
            return nil
        }
        // shortcut: the file is read on every load and build, no cache by modification time; add one if a
        // large page is rebuilt in a loop.
        guard let text = t.read(abs) else {
            if report {
                t.fail(pr.pos, "body \(flag) \"\(file)\": no such file")
            }
            return nil
        }
        if !hasSection {
            return text
        }
        if let s = bodySection(text, section) {
            return s
        }
        if report {
            let all = bodyHeadings(text)
            if !all.isEmpty {
                t.fail(
                    pr.pos,
                    "body \(flag) \"\(ref)\": \(rel) has no heading \"\(section)\"; headings: \(all.joined(separator: ", "))"
                )
            } else {
                t.fail(
                    pr.pos, "body \(flag) \"\(ref)\": \(rel) has no heading \"\(section)\" and no headings at all")
            }
        }
        return nil
    }

    /// Reports a link [label](file#anchor) whose file or section is not there. base is the markdown file
    /// the text came from, or the .tenn that writes it; pos is the body command, which the reader fixes.
    func checkBodyLinks(_ pos: Scene.Pos, _ base: String, _ text: String) {
        var own: Set<String>?
        var ownNames: [String] = []
        for target in bodyLinks(text) {
            let (file, anchorText, _) = cut(target, "#")
            let anchor = anchorText.trimmingCharacters(in: .whitespacesAndNewlines)
            if anchor.isEmpty || file.contains("://") {
                continue
            }
            if file.isEmpty {
                if own == nil {
                    // A fragment names a heading of the markdown file, not only of the section the body shows.
                    var src = text
                    if base != pos.file, let s = t.read(t.root.appendingPathComponent(base)) {
                        src = s
                    }
                    let found = headingsOf(src)
                    ownNames = found.names
                    own = found.anchors
                }
                if own?.contains(anchorOf(anchor)) != true {
                    noSection(pos, target, anchor, base, ownNames)
                }
                continue
            }
            let (abs, rel, inside) = t.resolveIn(base, file)
            guard inside else {
                t.fail(pos, "link \"\(target)\" leaves the workspace root")
                continue
            }
            guard let src = t.read(abs) else {
                t.fail(pos, "link \"\(target)\": no such file \(rel)")
                continue
            }
            let (names, anchors) = headingsOf(src)
            if !anchors.contains(anchorOf(anchor)) {
                noSection(pos, target, anchor, rel, names)
            }
        }
    }

    func noSection(_ pos: Scene.Pos, _ target: String, _ anchor: String, _ rel: String, _ names: [String]) {
        if !names.isEmpty {
            t.fail(
                pos,
                "link \"\(target)\": no section \"\(anchor)\" in \(rel) (headings: \(names.joined(separator: ", ")))")
            return
        }
        t.fail(pos, "link \"\(target)\": no section \"\(anchor)\" in \(rel) (no headings)")
    }

    /// Reads the colour of !(...): a name of the cmkdown table or hex digits, reported once per build.
    func markupColor(_ pos: Scene.Pos, _ name: String) -> Scene.Color? {
        guard let c = Scene.parseColor(name) else {
            warnColor(pos, name)
            return nil
        }
        return c
    }

    /// Reports an unknown colour once per build: the same typo on many lines must not bury the rest.
    func warnColor(_ pos: Scene.Pos, _ name: String) {
        if warned.contains(name) {
            return
        }
        warned.insert(name)
        t.warn(pos, "!(\(name)): no such color, the default is used")
    }

    /// Warns when a body read from a file needs more room than the item gives it.
    func warnBodyFit(_ rn: Scene.RNode) {
        if let body = bodies[ObjectIdentifier(rn)], !body.from.isEmpty, let ln = frames[ObjectIdentifier(rn)] {
            var width = ln.rw - 2 * body.padX
            var top = body.padY
            let have = ln.rh
            if let bn = bodyAt[ObjectIdentifier(rn)] {
                width = bn.rw
                top = 0
            } else if let ti = texts[ObjectIdentifier(rn)] {
                top = ti.padY + ti.height(ln.rw - 2 * ti.padX)
            }
            let room = have - top - body.padY
            let need = body.height(width)
            if need > room + 0.5 {
                t.warn(
                    body.pos,
                    "body \(SceneLoader.fromFileFlag) \"\(body.from)\" needs \(percentG(need)) points of height, the item leaves \(percentG(room)): raise height or shorten the text"
                )
            }
        }
        for k in rn.kids where k.kind == .item {
            warnBodyFit(k)
        }
    }
}

/// A number as Go writes %g: at most six significant digits, no trailing zeros.
private func percentG(_ v: Double) -> String {
    String(format: "%g", v)
}

/// The commands of a block, its nested blocks excluded.
private func bodyCommands(_ block: TennNode) -> [TennNode] {
    (block.children ?? []).filter { $0.kind != .BlockExpr }
}

/// The name of a command; "" for a node that is not one.
private func bodyCommandName(_ c: TennNode) -> String {
    guard c.kind == .Command else { return "" }
    return c.getIdent(0) ?? ""
}

/// The arguments of a command, its block excluded.
func bodyCommandArgs(_ c: TennNode) -> [TennNode] {
    Array((c.children ?? []).dropFirst().filter { $0.kind != .BlockExpr })
}

/// The text of argument i of a command.
private func bodyArgText(_ args: [TennNode], _ i: Int) -> String {
    guard i < args.count else { return "" }
    return args[i].getIdentText() ?? ""
}

/// The number of argument i of a command read outside a Prop.
private func bodyArgNum(_ args: [TennNode], _ i: Int) -> Double? {
    if i >= args.count {
        return nil
    }
    return Scene.Prop(name: "", args: args, pos: Scene.Pos(), range: Scene.Range()).num(i)
}
