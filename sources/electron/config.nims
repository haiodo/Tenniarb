import strutils, ospaths

mode = ScriptMode.Verbose

task build, "Build browser part":
  exec "nim js -o=index.js index.nim"
