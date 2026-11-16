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

extension Scene {
    /// A face of a text: the part of gfx.Font the layout reads.
    struct Font {
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
        private let m: TextMeasurer

        init(font: Font, text: String, m: TextMeasurer) {
            self.font = font
            self.text = text
            self.m = m
        }

        func width(_ s: String) -> Double { m.measure(s, font).width }

        /// The width of the widest line when only "\n" breaks lines.
        func natural() -> Double {
            var w = 0.0
            for line in text.components(separatedBy: "\n") {
                w = max(w, width(line))
            }
            return w
        }

        /// The height of the text at width w.
        func height(_ w: Double) -> Double {
            Double(wrap(w).count) * lineH
        }

        /// The lines of the text at width w, broken at spaces and after a hyphen inside a word; a piece wider
        /// than w stays on its line.
        func wrap(_ w: Double) -> [String] {
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
            // evalText of layout.go returns the text as written without an evaluator.
            title = pr.text(0) ?? ""
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
        let ti = Scene.TextInfo(font: fontOf(rn, size), text: title, m: measurer)
        ti.padX = padX
        ti.padY = padY
        let box = lineBox(ti.font)
        ti.asc = box.asc
        ti.desc = box.desc
        ti.lineH = box.h
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
