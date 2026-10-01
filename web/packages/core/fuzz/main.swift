// Reads base64-encoded UTF-8 inputs (one per line; JSON would drop a leading U+FEFF), prints one JSON result per line: tokens, tree, printed, errors.
import Foundation

func tj(_ t: TennToken) -> [String: Any] {
  ["type": "\(t.type)", "literal": t.literal, "line": t.line, "col": t.col, "pos": t.pos, "size": t.size]
}
func nj(_ n: TennNode) -> [String: Any] {
  var r: [String: Any] = ["kind": "\(n.kind)"]
  if let t = n.token { r["token"] = tj(t) }
  if let c = n.children { r["children"] = c.map(nj) }
  return r
}

while let line = readLine() {
  let src = String(decoding: Data(base64Encoded: line)!, as: UTF8.self)
  var toks: [[String: Any]] = []
  let lx = TennLexer(src)
  while let t = lx.getToken() {
    toks.append(tj(t))
    if t.type == .eof || toks.count > 100000 { break }
  }
  let p = TennParser()
  let tree = p.parse(src)
  let errs: [[String: Any]] = p.errors.errors.map {
    ["code": "\($0.errorCode)", "message": $0.message, "line": $0.line, "col": $0.col]
  }
  let doc: [String: Any] = [
    "tokens": toks, "tree": nj(tree), "printed": tree.toStr(), "printedClean": tree.toStr(0, true), "errors": errs,
  ]
  let d = try! JSONSerialization.data(withJSONObject: doc, options: [.sortedKeys, .withoutEscapingSlashes])
  print(String(decoding: d, as: UTF8.self))
}
