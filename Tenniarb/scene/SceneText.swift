//
//  SceneText.swift
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

import Cocoa
import cmkdown

extension Scene {
    /// A face of a text: the part of gfx.Font the layout reads.
    struct Font: Hashable {
        var name = ""  // the exact face; wins over family
        var family = ""
        var weight = 0  // a CSS weight 100..900, 0 when not set
        var italic = false
        var size = 0.0
        var spacing = 0.0  // letter-spacing
    }

    /// The extents of one line in points, as gfx.TextMetrics: ascent and descent are positive.
    struct TextMetrics {
        var width = 0.0
        var ascent = 0.0
        var descent = 0.0
    }

    /// Measures text as the window draws it, one line at a time: gfx.TextMeasurer over a CTLine.
    struct TextMeasurer {
        /// Estimates from the code points instead of measuring, as gfx.EstimateText: the parity stand and
        /// the goldens of go/cmd/tennlayout need that, the window keeps CoreText.
        var estimated = false

        func measure(_ s: String, _ f: Font) -> TextMetrics {
            if estimated {
                let n = Double(s.unicodeScalars.count)
                return TextMetrics(width: n * (f.size * 0.6 + f.spacing), ascent: f.size * 0.8, descent: f.size * 0.25)
            }
            var attrs: [NSAttributedString.Key: Any] = [.font: f.nsFont]
            if f.spacing != 0 {
                attrs[.kern] = f.spacing
            }
            let line = CTLineCreateWithAttributedString(NSAttributedString(string: s, attributes: attrs))
            var ascent: CGFloat = 0
            var descent: CGFloat = 0
            var leading: CGFloat = 0
            let width = CTLineGetTypographicBounds(line, &ascent, &descent, &leading)
            return TextMetrics(width: Double(width), ascent: Double(ascent), descent: Double(descent))
        }
    }

    /// How the lines of a leaf's text sit in its box: `layout left/right`, middle by default.
    enum TextAlign {
        case left
        case middle
        case right
    }

    /// One text of a node: the font, the line box and the words, what textInfo of layout.go keeps.
    final class TextInfo {
        let font: Font
        let text: String
        var padX = 0.0
        var padY = 0.0
        var lineH = 0.0
        var asc = 0.0
        var desc = 0.0
        var align = TextAlign.middle
        /// Set when the classic title carries markup: each line is laid out and drawn part by part.
        var parts: [TitlePart]?
        /// The build this text belongs to: it decides the line box of a classic title.
        unowned let p: Prep
        private let m: TextMeasurer

        init(font: Font, text: String, m: TextMeasurer, p: Prep) {
            self.font = font
            self.text = text
            self.m = m
            self.p = p
        }

        func width(_ s: String) -> Double { m.measure(s, font).width }

        /// The width of the widest line when only "\n" breaks lines.
        func natural() -> Double {
            if parts != nil {
                var w = 0.0
                for ln in layout(.infinity) {
                    w = max(w, ln.w)
                }
                return w
            }
            var w = 0.0
            for line in text.components(separatedBy: "\n") {
                w = max(w, width(line))
            }
            return w
        }

        /// The height of the text at width w; a run of another size is taller than its line of text.
        func height(_ w: Double) -> Double {
            guard parts != nil else {
                return Double(wrap(w).count) * lineH
            }
            var h = 0.0
            for ln in layout(w) {
                h += ln.h
            }
            return h
        }

        /// The lines of the text at width w, broken at spaces and after a hyphen inside a word; a piece wider
        /// than w stays on its line.
        func wrap(_ w: Double) -> [String] {
            if parts != nil {
                return layout(w).map(\.text)
            }
            var out: [String] = []
            for para in text.components(separatedBy: "\n") {
                var line = ""
                for (i, word) in para.components(separatedBy: " ").enumerated() {
                    for (j, piece) in hyphenPieces(word).enumerated() {
                        // The joined line is measured whole: a sum of word widths misses kerning and wraps
                        // text that fits as one line, as its natural width says.
                        var cand = line + piece
                        if j == 0 && i > 0 {
                            cand = line + " " + piece
                        }
                        switch true {
                        case i == 0 && j == 0:
                            line = piece
                        case width(cand) <= w + 1e-9:
                            line = cand
                        default:
                            out.append(line)
                            line = piece
                        }
                    }
                }
                out.append(line)
            }
            return out
        }

        /// The font of a part, the title font for a part that names none.
        func partFont(_ part: TitlePart) -> Font {
            part.font.size == 0 ? font : part.font
        }

        /// Breaks the title at width w: a marked-up title keeps its runs, and a word wider than a classic line
        /// is broken by characters.
        func layout(_ w: Double) -> [TitleLine] {
            var lines: [TitleLine] = []
            var line = TitleLine(h: lineH, asc: asc, desc: desc)
            var started = false
            func flush() {
                lines.append(line)
                line = TitleLine(h: lineH, asc: asc, desc: desc)
                started = false
            }
            func add(_ part: TitlePart) {
                line.parts.append(part)
                if let img = part.img {
                    // go/render kept the advance of the `@` that names the picture as a space beside it;
                    // dropping it made an item with a picture in its title five points narrower.
                    line.w += img.w + m.measure(" ", partFont(part)).width
                    line.h = max(line.h, img.h)
                    started = true
                    return
                }
                let f = partFont(part)
                line.w += m.measure(part.text, f).width
                if f != font {
                    let box = p.lineBox(f)
                    line.asc = max(line.asc, box.asc)
                    line.desc = max(line.desc, box.desc)
                    line.h = max(line.h, box.h)
                }
                started = started || !part.text.isEmpty
            }
            for part in parts ?? [] {
                if let img = part.img {
                    if started && w > 0 && line.w + img.w > w + 1e-9 {
                        flush()
                    }
                    add(part)
                    continue
                }
                let pf = partFont(part)
                for (pi, para) in part.text.components(separatedBy: "\n").enumerated() {
                    if pi > 0 {
                        flush()
                    }
                    for (i, word) in para.components(separatedBy: " ").enumerated() {
                        for (j, piece) in hyphenPieces(word).enumerated() {
                            var text = piece
                            if j == 0 && i > 0 {
                                text = " " + piece
                            }
                            let blank = text.trimmingCharacters(in: .whitespaces).isEmpty
                            if text.isEmpty || (!started && blank) {
                                continue
                            }
                            if started && !blank && w > 0 && line.w + m.measure(text, pf).width > w + 1e-9 {
                                flush()
                                text = piece
                            }
                            // A word wider than the line is broken by characters in a classic diagram, as
                            // go/render drew the title of an item: the box grows down instead of over.
                            if p.classic && w > 0 && !blank {
                                var rest = text
                                while !rest.isEmpty {
                                    if m.measure(rest, pf).width <= w - line.w + 1e-9 {
                                        add(TitlePart(text: rest, font: part.font, color: part.color, hasColor: part.hasColor))
                                        break
                                    }
                                    var cut = p.fitText(rest, pf, w - line.w)
                                    if cut.isEmpty {
                                        if !started {
                                            cut = p.firstText(rest)
                                        } else {
                                            flush()
                                            continue
                                        }
                                    }
                                    add(TitlePart(text: cut, font: part.font, color: part.color, hasColor: part.hasColor))
                                    rest = String(rest.dropFirst(cut.count))
                                    flush()
                                }
                                continue
                            }
                            if !text.isEmpty {
                                add(TitlePart(text: text, font: part.font, color: part.color, hasColor: part.hasColor))
                            }
                        }
                    }
                }
            }
            if started || lines.isEmpty {
                flush()
            }
            return lines
        }
    }

    /// A span of a title: text in the item font or in a size and colour of its own, set by the markdown markup,
    /// or an inline picture of a classic diagram.
    struct TitlePart {
        var text = ""
        var font = Font()  // zero: the font of the title
        var color = Color()
        var hasColor = false
        var img: TextImage?  // an inline picture: text is empty
    }

    /// One laid out line of a title; h grows above lineH for a larger run, and asc/desc follow the largest run.
    struct TitleLine {
        var parts: [TitlePart] = []
        var w = 0.0
        var h = 0.0
        var asc = 0.0
        var desc = 0.0

        var text: String { parts.map(\.text).joined() }
    }
}

extension Scene.Font {
    /// The face f asks for: an exact name, else the closest face of the family, else Helvetica, as gfx/cg picks it.
    /// shortcut: the face of a family comes from AppKit's weight steps, not from the CoreText trait table of gowt.
    var nsFont: NSFont {
        let fallback = NSFont(name: "Helvetica", size: size) ?? NSFont.systemFont(ofSize: size)
        if !name.isEmpty {
            return NSFont(name: name, size: size) ?? fallback
        }
        if family.isEmpty && weight == 0 && !italic {
            return fallback  // the empty face of gfx.Font is Helvetica in CoreText
        }
        var family = self.family
        if family.isEmpty {
            family = "Helvetica Neue"  // the sans face of gowt when only a weight or a style is set
        }
        let traits: NSFontTraitMask = italic ? .italicFontMask : []
        let weight = self.weight >= 700 ? 9 : self.weight >= 500 ? 7 : self.weight <= 300 ? 3 : 5
        return NSFontManager.shared.font(withFamily: family, traits: traits, weight: weight, size: size) ?? fallback
    }
}

/// The size of plain text at fontSize: the widest line across, the sum of the line boxes down, both rounded up;
/// the classic evaluator asks for it (`utils.textSize`).
func measureText(_ m: Scene.TextMeasurer, _ text: String, _ fontSize: Double) -> (w: Double, h: Double) {
    let font = Scene.Font(size: fontSize)
    let metrics = m.measure("Hg", font)
    var w = 0.0
    var h = 0.0
    for line in text.replacingOccurrences(of: "\\n", with: "\n").components(separatedBy: "\n") {
        if line.isEmpty {
            h += metrics.ascent + metrics.descent
            continue
        }
        w = max(w, m.measure(line, font).width)
        h += metrics.ascent + metrics.descent
    }
    return (ceil(w), ceil(h))
}

extension Scene.Prep {
    /// The text of a title with each ${...} replaced by its value: the element model hands the whole title of a
    /// classic item, a scene asks the engine about each piece.
    func evalText(_ rn: Scene.RNode, _ pr: Scene.Prop, _ text: String) -> String {
        if !text.contains("${") {
            return text
        }
        if let vals = modelVals(rn, pr), let first = vals.first {
            return first
        }
        guard let eval else { return text }
        var out = ""
        var i = text.startIndex
        while i < text.endIndex {
            if text[i...].hasPrefix("${") {
                let from = text.index(i, offsetBy: 2)
                if let close = text[from...].firstIndex(of: "}") {
                    let (v, ok) = evalValue(eval, String(text[from..<close]), rn.vars)
                    if ok {
                        out += v
                        i = text.index(after: close)
                        continue
                    }
                }
            }
            out.append(text[i])
            i = text.index(after: i)
        }
        return out
    }

    /// The ascent, the descent and the height of one line: from the measurer in a scene, from the ratios and the
    /// `floor(ascent+descent)+1` rounding of go/render in a classic diagram.
    func lineBox(_ f: Scene.Font) -> (asc: Double, desc: Double, h: Double) {
        if classic {
            let asc = f.size * Scene.classicAscentRatio
            let desc = f.size * Scene.classicDescentRatio
            return (asc, desc, floor(asc + desc) + 1)
        }
        let m = measurer.measure("Hg", f)
        return (m.ascent, m.descent, (m.ascent + m.descent).rounded())
    }

    /// The text of a leaf: its name, or the title over it, in the font the node asks for.
    func textOf(_ rn: Scene.RNode, _ inCtx: Bool) -> Scene.TextInfo? {
        // The name of a leaf is its text, as in a classic diagram, and a title overrides it. A node that holds
        // items is a box for them: its name is only a path segment, the text is a title if one is written.
        var title = rn.name
        if inCtx || holdsItems(rn) {
            title = ""
        }
        if let pr = prop(rn, "title") {
            title = pr.text(0) ?? ""
            title = evalText(rn, pr, title)
            // A bare text with spaces is several arguments, and only the first would be drawn: name the
            // mistake instead of dropping the words. A classic diagram drops them, as go/render always has.
            if pr.args.count > 1 && !classic {
                t.fail(pr.pos, "title takes one value: quote the words, \(quotedTitle(pr)) (tenniarb check -fix does it)")
            }
        }
        if let marker = styles[ObjectIdentifier(rn)]?.marker, !marker.isEmpty {
            // A marker is drawn in front of the text, with a space, as go/render does it.
            title = marker + " " + title
        }
        if title.isEmpty {
            return nil
        }
        if classic {
            // go/render reads a text `\n` as a newline; the block of a title keeps the indentation of its edges.
            title = title.replacingOccurrences(of: "\\n", with: "\n")
            title = title.trimmingCharacters(in: CharacterSet(charactersIn: " \t\r\n"))
        }
        var size = Scene.defaultFontSize
        if let fs = prop(rn, "font-size"), let v = fs.num(0) {
            if v > 0 {
                size = v
            } else {
                t.warn(fs.pos, "font-size: \(v) is not above 0, the default is used")
            }
        }
        var padX = classic ? Scene.classicTextPad : Scene.textPadX
        var padY = classic ? Scene.classicTextPad : Scene.textPadY
        if inCtx {
            padX = 0
            padY = 0
        }
        let ti = Scene.TextInfo(font: fontOf(rn, size), text: title, m: measurer, p: self)
        ti.padX = padX
        ti.padY = padY
        let box = lineBox(ti.font)
        ti.asc = box.asc
        ti.desc = box.desc
        ti.lineH = box.h
        if classic {
            ti.parts = titleMarkupParts(rn, ti)
            if ti.parts == nil {
                ti.parts = splitTitleImages(ti.text, inlineImages(rn))
            }
        }
        if let lh = prop(rn, "line-height") {
            if let v = lh.num(0), v > 0 {
                ti.lineH = v
            } else {
                t.warn(lh.pos, "line-height: not a number above 0, ignored")
            }
        }
        if let pr = prop(rn, "layout") {
            // Other words are the layout algorithms of diagram elements.
            switch pr.text(0) ?? "" {
            case "left": ti.align = .left
            case "right": ti.align = .right
            default: break
            }
        }
        if let ls = prop(rn, "line-spacing"), let v = ls.num(0) {
            ti.lineH += v
        }
        return ti
    }

    /// Reads font-family, font-weight, font-style and letter-spacing; the default face is Helvetica.
    func fontOf(_ rn: Scene.RNode, _ size: Double) -> Scene.Font {
        var f = Scene.Font(size: size)
        if let ls = prop(rn, "letter-spacing") {
            if let v = ls.num(0) {
                f.spacing = v
            } else {
                t.warn(ls.pos, "letter-spacing: not a number, ignored")
            }
        }
        if let fam = prop(rn, "font-family") {
            f.family = fam.text(0) ?? ""
        }
        if let pr = prop(rn, "font-weight") {
            f.weight = weight(pr)
        }
        if let pr = prop(rn, "font-style") {
            switch pr.text(0) ?? "" {
            case "italic", "oblique": f.italic = true
            case "normal": break
            default: t.warn(pr.pos, "font-style: \"\(pr.text(0) ?? "")\" is not italic or normal, ignored")
            }
        }
        if f.family.isEmpty && f.weight == 0 && !f.italic {
            f.name = "Helvetica"  // Options.FontName of layout.go
        } else if f.family.isEmpty {
            // FontName may be a CSS list for SVG (tenn-render); a family is one name.
            f.family = "Helvetica"
        }
        return f
    }

    /// Reads font-weight as a CSS number 100..900 or the words normal and bold; 0 when it is none of them.
    private func weight(_ pr: Scene.Prop) -> Int {
        switch pr.text(0) ?? "" {
        case "normal": return 400
        case "bold": return 700
        default: break
        }
        guard let v = pr.num(0), v >= 100, v <= 900 else {
            t.warn(pr.pos, "font-weight: takes 100..900, normal or bold, ignored")
            return 0
        }
        return Int(v)
    }

    /// The longest prefix of s that fits avail, in whole characters; "" when not even one does.
    func fitText(_ s: String, _ f: Scene.Font, _ avail: Double) -> String {
        if avail <= 0 {
            return ""
        }
        var cut = ""
        for g in s {
            if measurer.measure(cut + String(g), f).width > avail + 1e-9 {
                break
            }
            cut += String(g)
        }
        return cut
    }

    /// The first character of s, so a line always makes progress.
    func firstText(_ s: String) -> String {
        s.first.map(String.init) ?? s
    }

    /// Cuts a classic title at the !(colour), &(size) and @(picture) of the markdown markup; nil when the
    /// title writes none of them, so its text stays as written.
    func titleMarkupParts(_ rn: Scene.RNode, _ ti: Scene.TextInfo) -> [Scene.TitlePart]? {
        let title = Array(ti.text)
        if let parts = titleEmphasisParts(ti) {
            return parts
        }
        if !ti.text.contains("!(") && !ti.text.contains("&(") {
            return nil
        }
        var color = Scene.Color()
        var hasColor = false
        var font = ti.font
        var parts: [Scene.TitlePart] = []
        var plain = ""
        let imgs = inlineImages(rn)
        func flush() {
            if plain.isEmpty {
                return
            }
            parts.append(Scene.TitlePart(text: plain, font: font, color: color, hasColor: hasColor))
            plain = ""
        }
        var i = 0
        while i < title.count {
            if title[i] == "\\", i + 1 < title.count, "@$*_#<~!".contains(title[i + 1]) {
                plain.append(title[i + 1])
                i += 2
                continue
            }
            guard let (mark, arg, next) = parenAt(title, i) else {
                plain.append(title[i])
                i += 1
                continue
            }
            switch mark {
            case "!":
                flush()
                let (before, text, inline) = cut(arg, "|")
                if inline {
                    let c = markupColor(rn.pos, before)
                    parts.append(
                        Scene.TitlePart(text: text, font: font, color: c ?? Scene.Color(), hasColor: c != nil))
                } else if arg.trimmingCharacters(in: .whitespaces).isEmpty {
                    color = Scene.Color()
                    hasColor = false
                } else if let c = markupColor(rn.pos, arg) {
                    color = c
                    hasColor = true
                } else {
                    color = Scene.Color()
                    hasColor = false
                }
            case "&":
                flush()
                let (size, text, inline) = cut(arg, "|")
                if inline {
                    if let v = markupSize(size) {
                        parts.append(
                            Scene.TitlePart(
                                text: text, font: sizedFont(font, v), color: color, hasColor: hasColor))
                    }
                } else if let v = markupSize(arg) {
                    font = sizedFont(ti.font, v)
                } else {
                    font = ti.font
                }
            case "@":
                if let img = resolveInline(imgs, arg) {
                    flush()
                    parts.append(Scene.TitlePart(font: font, img: img))
                } else {
                    plain += String(title[i..<next])
                }
            default:
                plain += String(title[i..<next])
            }
            i = next
        }
        flush()
        return parts
    }

    /// The runs of a title with the emphasis go/render reads in it: **bold**, *italic*, `code`. nil when the
    /// title writes none, so a plain title keeps its characters.
    func titleEmphasisParts(_ ti: Scene.TextInfo) -> [Scene.TitlePart]? {
        let text = ti.text
        if text.rangeOfCharacter(from: CharacterSet(charactersIn: "*_`")) == nil {
            return nil
        }
        // The raw text, as go/render lexes it: **bold** is bold and *italic* is italic, which the normalised
        // form of a body would swap.
        let toks = MarkdownLexer.getTokens(code: text)
        if toks.isEmpty {
            return nil
        }
        func styled(_ weight: Int, _ italic: Bool) -> Scene.Font {
            var f = ti.font
            if !f.name.isEmpty {
                f.family = cut(f.name, ",").0.trimmingCharacters(in: .whitespaces)
                f.name = ""
            }
            if weight != 0 {
                f.weight = weight
            }
            f.italic = italic
            return f
        }
        var parts: [Scene.TitlePart] = []
        var plain = ""
        func flush() {
            if plain.isEmpty {
                return
            }
            parts.append(Scene.TitlePart(text: plain, font: ti.font))
            plain = ""
        }
        var marked = false
        for t in toks {
            switch t.type {
            case .bold:
                flush()
                parts.append(Scene.TitlePart(text: t.literal, font: styled(700, false)))
                marked = true
            case .italic:
                flush()
                parts.append(Scene.TitlePart(text: t.literal, font: styled(0, true)))
                marked = true
            case .code:
                flush()
                var f = ti.font
                f.family = Scene.bodyMonoFace
                f.name = ""
                parts.append(Scene.TitlePart(text: t.literal, font: f))
                marked = true
            case .eof:
                break
            default:
                plain += t.literal
            }
        }
        flush()
        if !marked {
            return nil
        }
        return parts
    }
}

/// reads a marker of the markup at i: "!", "&" or "@" followed by "(...)", the content balanced the way the
/// lexer reads it, and the index after the closing ")".
private func parenAt(_ s: [Character], _ i: Int) -> (String, String, Int)? {
    guard i + 1 < s.count, s[i + 1] == "(", s[i] == "!" || s[i] == "&" || s[i] == "@" else {
        return nil
    }
    var depth = 1
    var j = i + 2
    while j < s.count {
        if s[j] == "(" {
            depth += 1
        } else if s[j] == ")" {
            depth -= 1
            if depth == 0 {
                return (String(s[i]), String(s[(i + 2)..<j]), j + 1)
            }
        }
        j += 1
    }
    return nil
}

/// The title as the file should write it: every argument in one quoted text.
private func quotedTitle(_ pr: Scene.Prop) -> String {
    var words: [String] = []
    for i in 0..<pr.args.count {
        if let text = pr.text(i), !text.isEmpty {
            words.append(text)
        }
    }
    return "title \"" + words.joined(separator: " ") + "\""
}

/// Splits a word after each "-" that sits between a letter or digit and another character.
private func hyphenPieces(_ word: String) -> [String] {
    if !word.contains("-") {
        return [word]
    }
    var out: [String] = []
    let rs = Array(word)
    var start = 0
    if rs.count > 2 {
        for i in 1..<(rs.count - 1) {
            let prev = rs[i - 1]
            if rs[i] == "-" && (prev.isLetter || prev.isWholeNumber) && rs[i + 1] != "-" {
                out.append(String(rs[start...i]))
                start = i + 1
            }
        }
    }
    out.append(String(rs[start...]))
    return out
}
