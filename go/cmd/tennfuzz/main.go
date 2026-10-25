// tennfuzz is a differential fuzz of go/tenn against the Swift tenn sources (compiled standalone
// with web/packages/core/fuzz/main.swift). Usage: go run ./cmd/tennfuzz [-n 2000] [-seed 1].
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/haiodo/tenniarb/go/internal/ref"
)

var ascii = []string{"a", "b", "1", "2.5", " ", "\n", "\n", ";", "{", "}", `"x"`, "'y'", "$(1)", "%{m}", "@(i)", "//c\n", "/*c*/", "\r\n", "-", "\\", `"`, "'", "$", "%", "@(", "\x00", "\t"}

var uni = []string{"\u00e9", "\u65e5\u672c", "\u0663", "\U0001F600", "\ufeff", "\u00e9", "\u200b", "\u0085", "\U0001F468\u200d\U0001F469"}

var grapheme = []string{
	"e\u0301", "a\u0323\u0308", "\u0301", "\u0915\u094d\u0937", "\u0915\u093f", "\u0600\u0661", "\u0600a",
	"\u0d4e\u0d15", "\u06dd1", "\U000110bd1", "\u110b\u1161\u11ab", "\u1100\u1161", "\u1100\u1100", "\uac00\u11a8", "\uac01\u11a8",
	"\ud55c", "\u1161", "\u11a8", "\U0001F44D\U0001F3FD", "\U0001F3FD", "\U0001F1FA\U0001F1F8", "\U0001F1FA", "\U0001F1F8",
	"\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466", "\u200d\U0001F466", "1\ufe0f\u20e3", "#\ufe0f\u20e3",
	"\u2764\ufe0f", "\u2764", "\u270c\ufe0f\U0001F3FD", "\u00a9\ufe0f", "\u0e01\u0e33", "\u0e01\u0e34", "\u0e40\u0e01",
	"\u65e5\u672c", "\u6f22\ufe0e", "\U000e0100", "\r", "\n", "\r\n", "\u0085", "\u200d", "\u200c", "\u2028", "\u2029", "\u00ad",
	"\u200b", "\ufeff", "\U0001F3F4\U000e0067\U000e0062\U000e007f", "\U0001F600", "\u00e9", "\u0e01\u0e32",
}

var structural = []string{" ", "\n", "{", "}", `"`, ";", "a", "$(", "'", "//", "/*", "%{", "@("}

// scalarRanges are blocks where grapheme break classes change; randomScalars samples them.
var scalarRanges = [][2]rune{
	{0x300, 0x36f}, {0x600, 0x6ff}, {0x900, 0xdff}, {0xe00, 0xeff}, {0x1000, 0x109f}, {0x1100, 0x11ff},
	{0x2000, 0x206f}, {0x20d0, 0x20ff}, {0x2190, 0x2bff}, {0x3000, 0x303f}, {0xa960, 0xa97f},
	{0xac00, 0xd7ff}, {0xfe00, 0xfe0f}, {0xff00, 0xffef}, {0x1f000, 0x1faff}, {0xe0000, 0xe01ff},
}

func randomScalars(r *rng, n int) []string {
	out := make([]string, n)
	for i := range out {
		rg := scalarRanges[int(r.next()*float64(len(scalarRanges)))]
		c := rg[0] + rune(r.next()*float64(rg[1]-rg[0]+1))
		if c >= 0xd800 && c <= 0xdfff {
			c = 'a'
		}
		out[i] = string(c)
	}
	return out
}

func alphabets() [][]string {
	scalars := randomScalars(&rng{12345}, 300)
	mixed := append(append([]string{}, grapheme...), structural...)
	return [][]string{
		ascii,
		append(append([]string{}, ascii...), uni...),
		append(append([]string{}, uni...), " ", "\n", "{", "}", `"`, ";", "a", "$("),
		grapheme,
		mixed,
		mixed,
		scalars,
		append(append([]string{}, scalars...), structural...),
	}
}

type rng struct{ state uint32 }

func (r *rng) next() float64 {
	r.state = r.state*1664525 + 1013904223
	return float64(r.state) / (1 << 32)
}

func generate(n int, seed uint32) []string {
	r := &rng{seed}
	alphas := alphabets()
	out := make([]string, n)
	for i := range out {
		alpha := alphas[i%len(alphas)]
		length := 1 + int(r.next()*40)
		var sb strings.Builder
		for j := 0; j < length; j++ {
			sb.WriteString(alpha[int(r.next()*float64(len(alpha)))])
		}
		out[i] = sb.String()
	}
	return out
}

func buildSwift(root string) (string, string, error) {
	dir, err := os.MkdirTemp("", "tenn-gofuzz-")
	if err != nil {
		return "", "", err
	}
	bin := filepath.Join(dir, "tennref-swift")
	args := []string{"-O"}
	for _, f := range []string{"TennLexer", "TennLexerModel", "TennModel", "TennParser", "TennPrinter"} {
		args = append(args, filepath.Join(root, "Tenniarb/document/tenn", f+".swift"))
	}
	args = append(args, filepath.Join(root, "web/packages/core/fuzz/main.swift"), "-o", bin)
	cmd := exec.Command("swiftc", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return bin, dir, cmd.Run()
}

func runSwift(bin string, inputs []string) ([]any, error) {
	var in bytes.Buffer
	for _, s := range inputs {
		in.WriteString(base64.StdEncoding.EncodeToString([]byte(s)))
		in.WriteByte('\n')
	}
	cmd := exec.Command(bin)
	cmd.Stdin = &in
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var res []any
	// The Swift lexer also prints diagnostics to stdout; results are the JSON objects.
	for _, l := range strings.Split(stdout.String(), "\n") {
		if !strings.HasPrefix(l, "{") {
			continue
		}
		var v any
		if e := json.Unmarshal([]byte(l), &v); e != nil {
			return nil, e
		}
		res = append(res, v)
	}
	if err != nil || len(res) != len(inputs) {
		return nil, fmt.Errorf("swift failed: %v, %d/%d results\n%s", err, len(res), len(inputs), stderr.String())
	}
	return res, nil
}

func runGo(s string) any {
	data, err := json.Marshal(ref.Run(s))
	if err != nil {
		panic(err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		panic(err)
	}
	return v
}

// firstDiff returns the path of the first difference, empty when equal.
func firstDiff(path string, a, b any) string {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok {
			return path
		}
		return diffMap(path, x, y)
	case []any:
		y, ok := b.([]any)
		if !ok {
			return path
		}
		return diffSlice(path, x, y)
	}
	if reflect.DeepEqual(a, b) {
		return ""
	}
	return path
}

func diffMap(path string, x, y map[string]any) string {
	keys := map[string]bool{}
	for k := range x {
		keys[k] = true
	}
	for k := range y {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		if d := firstDiff(path+"."+k, x[k], y[k]); d != "" {
			return d
		}
	}
	return ""
}

func diffSlice(path string, x, y []any) string {
	for i := 0; i < len(x) && i < len(y); i++ {
		if d := firstDiff(fmt.Sprintf("%s[%d]", path, i), x[i], y[i]); d != "" {
			return d
		}
	}
	if len(x) != len(y) {
		return path + ".length"
	}
	return ""
}

var indexRe = regexp.MustCompile(`\[\d+\]`)

func classOf(diff string) string {
	c := indexRe.ReplaceAllString(diff, "[]")
	return strings.ReplaceAll(c, ".children[]", "*")
}

// shrink repeatedly drops single runes while the input still differs in the same class.
func shrink(bin, s, class string) (string, error) {
	for {
		runes := []rune(s)
		cands := make([]string, 0, len(runes))
		for i := range runes {
			cands = append(cands, string(runes[:i])+string(runes[i+1:]))
		}
		sw, err := runSwift(bin, cands)
		if err != nil {
			return s, err
		}
		next := ""
		for i, c := range cands {
			if d := firstDiff("", sw[i], runGo(c)); d != "" && classOf(d) == class {
				next = c
				break
			}
		}
		if next == "" {
			return s, nil
		}
		s = next
	}
}

// sweep builds probes for every scalar c that separate the grapheme break classes: c as the left
// and right side of a break, in front of ZWJ and after a ZWJ pictograph.
const sweepKinds = 13

func sweep() ([]string, []rune) {
	var out []string
	var cps []rune
	for c := rune(0x80); c <= 0x2ffff || c >= 0xe0000 && c <= 0xe0fff; c++ {
		if c >= 0xd800 && c <= 0xdfff {
			continue
		}
		s := string(c)
		probes := []string{"a" + s, s + "a", s + s, s + "\u0301", "\U0001F466\u200d" + s, s + "\u200d\U0001F466", "\u0915\u094d" + s,
			"\u1100" + s, s + "\u1161", "\u1100\u1161" + s, s + "\u11a8", "\U0001F1FA" + s, "\U0001F1FA\U0001F1F8" + s}
		out = append(out, probes...)
		for range probes {
			cps = append(cps, c)
		}
		if c == 0x2ffff {
			c = 0xdffff
		}
	}
	return out, cps
}

// reportSweep prints, per probe kind, the scalar ranges where Go and Swift disagree.
func reportSweep(inputs []string, cps []rune, sw []any) {
	kinds := make([][]rune, sweepKinds)
	for i, s := range inputs {
		if firstDiff("", sw[i], runGo(s)) != "" {
			kinds[i%sweepKinds] = append(kinds[i%sweepKinds], cps[i])
		}
	}
	for k, list := range kinds {
		fmt.Printf("probe %d: %d scalars %s\n", k, len(list), ranges(list))
	}
}

func ranges(list []rune) string {
	var sb strings.Builder
	for i := 0; i < len(list); i++ {
		j := i
		for j+1 < len(list) && list[j+1] == list[j]+1 {
			j++
		}
		if j == i {
			fmt.Fprintf(&sb, "%X ", list[i])
		} else {
			fmt.Fprintf(&sb, "%X-%X ", list[i], list[j])
		}
		i = j
	}
	return sb.String()
}

type found struct {
	count int
	first string
}

func main() {
	n := flag.Int("n", 2000, "number of inputs")
	seed := flag.Uint("seed", 1, "seed")
	root := flag.String("root", "..", "repository root")
	doSweep := flag.Bool("sweep", false, "probe every scalar instead of random inputs")
	flag.Parse()
	bin, dir, err := buildSwift(*root)
	if dir != "" {
		defer os.RemoveAll(dir)
	}
	if err != nil {
		fmt.Println("swiftc failed:", err)
		os.Exit(1)
	}
	inputs := generate(*n, uint32(*seed))
	var cps []rune
	if *doSweep {
		inputs, cps = sweep()
		*n = len(inputs)
	}
	sw, err := runSwift(bin, inputs)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if *doSweep {
		reportSweep(inputs, cps, sw)
		return
	}
	classes := map[string]*found{}
	bad := 0
	for i, s := range inputs {
		d := firstDiff("", sw[i], runGo(s))
		if d == "" {
			continue
		}
		bad++
		c := classOf(d)
		if classes[c] == nil {
			classes[c] = &found{first: s}
		}
		classes[c].count++
	}
	report(bin, classes)
	fmt.Printf("gofuzz: %d inputs, seed %d, %d mismatches, %d classes\n", *n, *seed, bad, len(classes))
	if bad > 0 {
		os.Exit(1)
	}
}

func report(bin string, classes map[string]*found) {
	names := make([]string, 0, len(classes))
	for c := range classes {
		names = append(names, c)
	}
	sort.Strings(names)
	for _, c := range names {
		f := classes[c]
		min, err := shrink(bin, f.first, c)
		if err != nil {
			fmt.Println("shrink failed:", err)
		}
		fmt.Printf("CLASS %s: %d\n  example %q\n  minimized %q (% x)\n", c, f.count, f.first, min, min)
	}
}
