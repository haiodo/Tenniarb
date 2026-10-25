// Worker: bench <lex|parse> <file> <runs>; prints one JSON line.
import Foundation

var sink = 0

func now() -> Double { Double(DispatchTime.now().uptimeNanoseconds) / 1e6 }

func countNodes(_ n: TennNode) -> Int {
  var c = 1
  if let ch = n.children { for x in ch { c += countNodes(x) } }
  return c
}

func lexOnce(_ src: String) -> Int {
  let lx = TennLexer(src)
  var n = 0
  while let t = lx.getToken() {
    n += 1
    if t.type == .eof { break }
  }
  return n
}

func parseOnce(_ src: String) -> (TennNode, Int) {
  let p = TennParser()
  let tree = p.parse(src)
  return (tree, p.errors.errors.count)
}

let args = CommandLine.arguments
let mode = args[1]
let src = String(decoding: FileManager.default.contents(atPath: args[2])!, as: UTF8.self)
let runs = Int(args[3])!

func once() {
  if mode == "lex" { sink &+= lexOnce(src) } else { sink &+= parseOnce(src).0.children?.count ?? 0 }
}

var times: [Double] = []
let t0 = now()
once()
let first = now() - t0
for _ in 0..<3 { once() }
for _ in 0..<runs {
  let s = now()
  once()
  times.append(now() - s)
}
times.sort()
var tokens = 0, nodes = 0, errors = 0
if mode == "lex" { tokens = lexOnce(src) } else {
  let (t, e) = parseOnce(src)
  nodes = countNodes(t)
  errors = e
}
print("{\"tokens\":\(tokens),\"nodes\":\(nodes),\"errors\":\(errors),\"first\":\(first),\"median\":\(times[times.count / 2]),\"min\":\(times[0]),\"runs\":\(runs),\"sink\":\(sink)}")
