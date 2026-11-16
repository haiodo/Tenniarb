//
//  SceneColor.swift
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
import cmkdown

extension Scene {
    /// A colour as gfx.Color: channels 0..1, alpha 0 when unset.
    struct Color: Equatable {
        var r = 0.0
        var g = 0.0
        var b = 0.0
        var a = 0.0

        static let white = Color(r: 1, g: 1, b: 1, a: 1)
        static let black = Color(a: 1)
    }

    /// A colour name from the cmkdown table, hex digits, 6 (RGB) or 8 (RGBA) with an optional "#", or the
    /// channels of an array of numbers, as go/scene/color.go parseColor reads them.
    static func parseColor(_ s: String) -> Color? {
        let s = s.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        if s == "none" || s == "transparent" {  // no fill, no border: what alpha 0 draws anyway
            return Color()
        }
        if s.contains(",") {  // a colour array of an expression, as text
            return colorChannels(s)
        }
        var hex = s
        if !hex.isEmpty, let named = ColorNames[hex], !named.isEmpty {
            hex = named
        }
        hex = hex.replacingOccurrences(of: "#", with: "")
        guard hex.count == 6 || hex.count == 8, let v = UInt64(hex, radix: 16) else { return nil }
        let value = hex.count == 6 ? v << 8 | 0xff : v
        let ch = { (shift: UInt64) in Double(value >> shift & 0xff) / 255 }
        return Color(r: ch(24), g: ch(16), b: ch(8), a: ch(0))
    }

    /// getTextColorBasedOn: black on a light fill, white on a dark one.
    static func contrast(_ c: Color) -> Color {
        let sum = (c.r * 255 * 299 + c.g * 255 * 587 + c.b * 255 * 114) / 1000
        if sum + 0.5 >= 129 {
            return Color(a: 1)
        }
        return Color(r: 1, g: 1, b: 1, a: 1)
    }
}

/// Reads "r,g,b" or "r,g,b,a" with channels 0..255: the text go/render gives for a colour array of an
/// expression. r,g,b is opaque.
private func colorChannels(_ s: String) -> Scene.Color? {
    let parts = s.components(separatedBy: ",")
    guard parts.count == 3 || parts.count == 4 else { return nil }
    var v = [0.0, 0.0, 0.0, 1.0]
    for (i, part) in parts.enumerated() {
        guard let f = Double(part.trimmingCharacters(in: .whitespacesAndNewlines)) else { return nil }
        v[i] = f / 255
    }
    return Scene.Color(r: v[0], g: v[1], b: v[2], a: v[3])
}
