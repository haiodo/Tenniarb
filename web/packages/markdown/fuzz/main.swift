// Reads base64-encoded UTF-8 inputs (one per line), prints one JSON result per line:
// tokens, lexer errors, HTML (null where Swift would trap) and parseColor/colorToHex of the input itself.
import Cocoa
import Foundation

// Fixed sizes derived from the name; names starting with "n" are unknown.
class P: ImageProvider {
  override func resolveImage(name: String) -> CachedImage? {
    if name.hasPrefix("n") { return nil }
    let s = CGSize(width: 40 + 10 * CGFloat(name.count % 7), height: 20 + 5 * CGFloat(name.count % 5))
    return CachedImage(image: NSImage(size: s), size: s)
  }
}

func tj(_ t: MarkdownToken) -> [String: Any] {
  ["type": "\(t.type)", "literal": t.literal, "line": t.line, "col": t.col, "pos": t.pos, "size": t.size]
}

// colorToHex reads components[2] and traps on the 2-component gray CGColor.black that parseColor returns for unknown names.
func nameTraps(_ s: String) -> Bool {
  !s.starts(with: "#") && ColorNames[s] == nil
}

// hexStringToUIColor does Int(color) on the saturated UInt64 and traps from 2^63 up.
func hexTraps(_ s: String) -> Bool {
  guard s.starts(with: "#") else { return false }
  var h = s.trimmingCharacters(in: .whitespacesAndNewlines)
  if h.hasPrefix("#") { h = String(h.suffix(from: h.index(h.startIndex, offsetBy: 1))) }
  var v: UInt64 = 0
  Scanner(string: h).scanHexInt64(&v)
  return v > UInt64(Int.max)
}

func comps(_ c: CGColor) -> [CGFloat] {
  let k = c.components!
  return k.count == 4 ? k : [0, 0, 0, k[1]]
}

setvbuf(stdout, nil, _IOLBF, 0)

while let line = readLine() {
  let src = String(decoding: Data(base64Encoded: line)!, as: UTF8.self)
  var errs: [[String: Any]] = []
  let lx = MarkdownLexer(src)
  lx.errorHandler = { e, a, b in errs.append(["error": "\(e)", "start": a, "pos": b]) }
  var toks: [MarkdownToken] = []
  while let t = lx.getToken() { toks.append(t) }

  var skip = false
  for t in toks where t.type == .color {
    let l = t.literal
    if let i = l.firstIndex(of: "|") {
      if nameTraps(String(l.prefix(upTo: i))) || hexTraps(String(l.prefix(upTo: i))) { skip = true }
    } else if l.trimmingCharacters(in: .whitespacesAndNewlines).count != 0, (nameTraps(l) || hexTraps(l)) {
      skip = true
    }
  }
  var doc: [String: Any] = ["tokens": toks.map(tj), "errors": errs]
  doc["html"] = skip ? NSNull() : HTMLPrinter.toHTML(toks, originalFont: 16.0, textColor: "black", imageProvider: P(2))
  if !hexTraps(src) {
    let c = parseColor(src, alpha: 0.5)
    doc["color"] = comps(c).map { Double($0) }
    if !nameTraps(src) { doc["hex"] = colorToHex(c) }
  }

  let d = try! JSONSerialization.data(withJSONObject: doc, options: [.sortedKeys, .withoutEscapingSlashes])
  print(String(decoding: d, as: UTF8.self))
}
