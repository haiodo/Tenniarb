// tennbench compares lexer/parser speed of the Swift, Go and TypeScript Tenn implementations on
// identical inputs and prints markdown tables. Usage: go run ./cmd/tennbench [-runs 15] [-root ..].
package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"github.com/haiodo/tenniarb/go/tenn"
)

//go:embed bench.swift
var benchSwift []byte

//go:embed bench.ts
var benchTS []byte

type result struct {
	Tokens int     `json:"tokens"`
	Nodes  int     `json:"nodes"`
	Errors int     `json:"errors"`
	First  float64 `json:"first"`
	Median float64 `json:"median"`
	Min    float64 `json:"min"`
	Allocs uint64  `json:"allocs"`
	Bytes  uint64  `json:"bytes"`
}

var sink int

func countNodes(n *tenn.Node) int {
	c := 1
	for _, ch := range n.Children {
		c += countNodes(ch)
	}
	return c
}

func lexOnce(src string) int {
	lx := tenn.NewLexer(src)
	n := 0
	for t := lx.GetToken(); t != nil; t = lx.GetToken() {
		n++
		if t.Type == tenn.TokEOF {
			break
		}
	}
	return n
}

// worker measures the Go implementation in a fresh process, like the Swift and Node workers.
func worker(mode, file string, runs int, cpuprofile string) {
	data, err := os.ReadFile(file)
	if err != nil {
		fatal(err)
	}
	src := string(data)
	once := func() {
		if mode == "lex" {
			sink += lexOnce(src)
		} else {
			tree, _ := tenn.Parse(src)
			sink += len(tree.Children)
		}
	}
	var r result
	t0 := time.Now()
	once()
	r.First = ms(time.Since(t0))
	for i := 0; i < 3; i++ {
		once()
	}
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			fatal(err)
		}
		defer pprof.StopCPUProfile()
	}
	times := make([]float64, runs)
	var allocs, bytes uint64
	var m0, m1 runtime.MemStats
	for i := range times {
		if cpuprofile == "" {
			runtime.GC()
		}
		runtime.ReadMemStats(&m0)
		s := time.Now()
		once()
		times[i] = ms(time.Since(s))
		runtime.ReadMemStats(&m1)
		allocs += m1.Mallocs - m0.Mallocs
		bytes += m1.TotalAlloc - m0.TotalAlloc
	}
	sort.Float64s(times)
	r.Median, r.Min = times[len(times)/2], times[0]
	r.Allocs, r.Bytes = allocs/uint64(runs), bytes/uint64(runs)
	if mode == "lex" {
		r.Tokens = lexOnce(src)
	} else {
		tree, p := tenn.Parse(src)
		r.Nodes = countNodes(tree)
		if p.Errors.HasErrors() {
			r.Errors = len(p.Errors.Errors)
		}
	}
	b, _ := json.Marshal(r)
	fmt.Println(string(b))
}

func ms(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "tennbench:", err)
	os.Exit(1)
}

func run(name string, args ...string) result {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Fprintln(os.Stderr, string(ee.Stderr))
		}
		fatal(fmt.Errorf("%s %v: %w", name, args, err))
	}
	var r result
	if err := json.Unmarshal(out, &r); err != nil {
		fatal(fmt.Errorf("%s output %q: %w", name, out, err))
	}
	return r
}

func capture(name string, args ...string) string {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return "n/a"
	}
	return strings.TrimSpace(string(out))
}

func buildSwift(root, dir string) string {
	mainSwift := filepath.Join(dir, "main.swift")
	if err := os.WriteFile(mainSwift, benchSwift, 0o644); err != nil {
		fatal(err)
	}
	bin := filepath.Join(dir, "bench-swift")
	args := []string{"-O", "-wmo"}
	for _, f := range []string{"TennLexer", "TennLexerModel", "TennModel", "TennParser", "TennPrinter"} {
		args = append(args, filepath.Join(root, "Tenniarb/document/tenn", f+".swift"))
	}
	args = append(args, mainSwift, "-o", bin)
	cmd := exec.Command("swiftc", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fatal(err)
	}
	return bin
}

func main() {
	runs := flag.Int("runs", 15, "measured runs per input and mode (after 3 warm-up runs)")
	root := flag.String("root", "..", "repository root")
	doProf := flag.Bool("pprof", true, "print Go pprof top 10 for lex+parse of the 10k input")
	wMode := flag.String("worker", "", "internal: lex|parse")
	wFile := flag.String("file", "", "internal: input file")
	wProf := flag.String("cpuprofile", "", "internal: cpu profile output")
	flag.Parse()

	if *wMode != "" {
		worker(*wMode, *wFile, *runs, *wProf)
		return
	}
	if *runs < 10 {
		fatal(fmt.Errorf("-runs must be >= 10"))
	}

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fatal(err)
	}
	dir, err := os.MkdirTemp("", "tennbench-")
	if err != nil {
		fatal(err)
	}
	defer os.RemoveAll(dir)

	inputs := []struct{ name, path string }{
		{"Example.tenn", filepath.Join(absRoot, "docs/Example.tenn")},
	}
	for _, g := range []struct {
		name string
		n    int
		gen  func(int) string
	}{
		{"synthetic-1k", 1000, genDiagram},
		{"synthetic-10k", 10000, genDiagram},
		{"emoji-3k", 3000, genEmoji},
	} {
		p := filepath.Join(dir, g.name+".tenn")
		if err := os.WriteFile(p, []byte(g.gen(g.n)), 0o644); err != nil {
			fatal(err)
		}
		inputs = append(inputs, struct{ name, path string }{g.name, p})
	}

	fmt.Fprintln(os.Stderr, "building Swift (-O)...")
	swiftBin := buildSwift(absRoot, dir)
	tsFile := filepath.Join(dir, "bench.ts")
	if err := os.WriteFile(tsFile, benchTS, 0o644); err != nil {
		fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		fatal(err)
	}
	coreSrc := filepath.Join(absRoot, "web/packages/core/src")
	rn := fmt.Sprint(*runs)

	type row struct {
		name, mode   string
		size         int
		goR, sw, tsR result
	}
	var rows []row
	for _, in := range inputs {
		st, err := os.Stat(in.path)
		if err != nil {
			fatal(err)
		}
		for _, mode := range []string{"lex", "parse"} {
			fmt.Fprintf(os.Stderr, "%s %s...\n", in.name, mode)
			rows = append(rows, row{
				name: in.name, mode: mode, size: int(st.Size()),
				goR: run(self, "-worker", mode, "-file", in.path, "-runs", rn),
				sw:  run(swiftBin, mode, in.path, rn),
				tsR: run("node", tsFile, coreSrc, mode, in.path, rn),
			})
		}
	}

	fmt.Printf("# tennbench: %d measured runs after 3 warm-up runs, median; each worker is a fresh process\n\n", *runs)
	fmt.Print("## Machine\n\n")
	fmt.Println("- CPU: " + capture("sysctl", "-n", "machdep.cpu.brand_string"))
	fmt.Println("- OS: " + capture("sw_vers", "-productName") + " " + capture("sw_vers", "-productVersion") + " (" + capture("uname", "-m") + ")")
	fmt.Println("- Go: " + runtime.Version())
	fmt.Println("- Swift: " + strings.SplitN(capture("swiftc", "--version"), "\n", 2)[0])
	fmt.Println("- Node: " + capture("node", "-v"))

	mbps := func(size int, msv float64) float64 { return float64(size) / 1e6 / (msv / 1e3) }
	fmt.Print("\n## Time\n\n")
	fmt.Println("| input | size, KB | mode | Go ms | Go MB/s | Swift ms | Swift MB/s | TS warm ms | TS warm MB/s | TS cold ms | Go time / Swift time | TS warm / Go |")
	fmt.Println("|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
	for _, r := range rows {
		m := map[string]string{"lex": "lex", "parse": "lex+parse"}[r.mode]
		fmt.Printf("| %s | %d | %s | %.2f | %.0f | %.2f | %.0f | %.2f | %.0f | %.2f | %.2fx | %.2fx |\n",
			r.name, r.size/1024, m,
			r.goR.Median, mbps(r.size, r.goR.Median),
			r.sw.Median, mbps(r.size, r.sw.Median),
			r.tsR.Median, mbps(r.size, r.tsR.Median), r.tsR.First,
			r.goR.Median/r.sw.Median, r.tsR.Median/r.goR.Median)
	}
	fmt.Print("\n## Go first run and allocations per op\n\n")
	fmt.Println("| input | mode | Go first ms | Swift first ms | allocs/op | KB/op |")
	fmt.Println("|---|---|---:|---:|---:|---:|")
	for _, r := range rows {
		fmt.Printf("| %s | %s | %.2f | %.2f | %d | %d |\n", r.name, r.mode, r.goR.First, r.sw.First, r.goR.Allocs, r.goR.Bytes/1024)
	}
	fmt.Print("\n## Equivalence check (tokens incl. eof; nodes in the full tree; parse errors)\n\n")
	fmt.Println("| input | mode | Go | Swift | TS | match |")
	fmt.Println("|---|---|---:|---:|---:|---|")
	ok := true
	for _, r := range rows {
		a, b, c := r.goR.Tokens, r.sw.Tokens, r.tsR.Tokens
		if r.mode == "parse" {
			a, b, c = r.goR.Nodes, r.sw.Nodes, r.tsR.Nodes
		}
		match := a == b && b == c && r.goR.Errors+r.sw.Errors+r.tsR.Errors == 0
		ok = ok && match
		what := "tokens"
		if r.mode == "parse" {
			what = fmt.Sprintf("nodes, errors %d/%d/%d", r.goR.Errors, r.sw.Errors, r.tsR.Errors)
		}
		fmt.Printf("| %s | %s | %d | %d | %d | %v |\n", r.name, what, a, b, c, match)
	}
	if !ok {
		fmt.Println("\n**MISMATCH: implementations did not do equivalent work**")
	}

	if *doProf {
		prof := filepath.Join(dir, "cpu.prof")
		big := filepath.Join(dir, "synthetic-10k.tenn")
		run(self, "-worker", "parse", "-file", big, "-runs", "200", "-cpuprofile", prof)
		fmt.Print("\n## Go pprof top 10 (lex+parse, synthetic-10k, 200 runs)\n\n")
		fmt.Println("```")
		fmt.Println(capture("go", "tool", "pprof", "-top", "-nodecount=10", self, prof))
		fmt.Println("```")
	}
}
