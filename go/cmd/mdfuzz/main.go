// mdfuzz is a differential fuzz of go/markdown against the Swift cmkdown sources (compiled standalone
// with web/packages/markdown/fuzz/main.swift). Usage: go run ./cmd/mdfuzz [-n 2000] [-seed 1].
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
	"strings"

	"github.com/haiodo/tenniarb/go/markdown"
	"github.com/haiodo/tenniarb/go/tenn"
)

var colors = []string{"red", "#ff0000", "#abc", "grey-200", "gray-200", "amber", "amber-500", "deeppurple", "bluegrey", "blue-grey-500", "#EEEEEE", "#FFC107", "clear", "transparent", "#0xff", "# ff", "0xff", "#ffffffffffffffffffff", "#12345678", "#", "RED", "constructor", "__proto__", "zz"}

var nums = []string{"14", "16", "20.5", "1e5", "0x10", "inf", "nan", "+5", ".5", "5.", "1e400", "-0", "1e-7", "1_0", " 5", "1e16", "1e15", "123456789012345678", "0.0001", "0.00001", "abc", "0x1p3", "Infinity", "-3", "007"}

var images = []string{"a", "nope", "logo", "i|20", "i|x30", "i|100x", "i|40x40", "i| 5 x 6 ", "i|+7x+8", "i|ax", "i|9x9x9", "i|1x1", "i|x", "i|", "ii|2.5"}

var ascii = []string{"a", "b", "1", " ", " ", "\n", "\n", "\t", "\r", "\r\n", "*", "*", "_", "<", ">", "~", "#", "## ", "`", "@(", "!(", "&(", "$(", "${", ")", "}", "|", "\\", "\\*", "\\`", "&", "'", `"`, "x", "* ", "\x00"}

var uni = []string{"\u00e9", "\u65e5\u672c", "\u0301", "\U0001F600", "\u200d", "\U0001F468\u200d\U0001F469", "\U0001F1FA\U0001F1F8", "\u00a0", "\u2028", "\u0085", "\ufeff", "\u180e", "\u3000", "\u0600"}

func structural() []string {
	out := []string{}
	for _, c := range colors {
		out = append(out, "!("+c+")", "!("+c+"|w)")
	}
	out = append(out, "!()", "!( )", "!(|x)")
	for _, n := range nums {
		out = append(out, "&("+n+")", "&("+n+"|w)")
	}
	out = append(out, "&()", "&( )")
	for _, i := range images {
		out = append(out, "@("+i+")")
	}
	return append(out, "@()", "${e}", "$(a)", "`c`", "```a\nb```", "`a\nb`", "`a\nb\n`", "\n`x\ny`\n", "# T\n", "## T", "### x\n", "* item\n", " * item\n", "*b*", "_i_", "~s~", "<u>", "word ")
}

type rng struct{ state uint32 }

func (r *rng) next() float64 {
	r.state = r.state*1664525 + 1013904223
	return float64(r.state) / (1 << 32)
}

func concat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func generate(n int, seed uint32) []string {
	r := &rng{seed}
	st := structural()
	alphas := [][]string{
		ascii,
		concat(ascii, st),
		concat(ascii, st, uni),
		concat(uni, ascii, []string{"|", "*", " ", "\n", "&", "<", "'", "!(", "#"}),
	}
	out := make([]string, 0, n+len(colors)+len(nums)+2)
	for i := 0; i < n; i++ {
		alpha := alphas[i%len(alphas)]
		length := 1 + int(r.next()*30)
		var sb strings.Builder
		for j := 0; j < length; j++ {
			sb.WriteString(alpha[int(r.next()*float64(len(alpha)))])
		}
		out = append(out, sb.String())
	}
	return concat(out, colors, nums, []string{"", "\x00"})
}

func buildSwift(root string) (string, string, error) {
	dir, err := os.MkdirTemp("", "md-gofuzz-")
	if err != nil {
		return "", "", err
	}
	bin := filepath.Join(dir, "mdref-swift")
	args := []string{"-O", "-suppress-warnings", "-swift-version", "5"}
	for _, f := range []string{"Types", "MarkdownLexer", "HTMLPrinter", "ColorUtils", "Images"} {
		args = append(args, filepath.Join(root, "build/SourcePackages/checkouts/cmkdown/Sources/cmkdown", f+".swift"))
	}
	args = append(args, filepath.Join(root, "web/packages/markdown/fuzz/main.swift"), "-o", bin)
	cmd := exec.Command("swiftc", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return bin, dir, cmd.Run()
}

func runSwift(bin string, inputs []string) ([]map[string]any, error) {
	var in bytes.Buffer
	for _, s := range inputs {
		in.WriteString(base64.StdEncoding.EncodeToString([]byte(s)))
		in.WriteByte('\n')
	}
	cmd := exec.Command(bin)
	cmd.Stdin = &in
	cmd.Env = append(os.Environ(), "SWIFT_DETERMINISTIC_HASHING=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var res []map[string]any
	for _, l := range strings.Split(stdout.String(), "\n") {
		if !strings.HasPrefix(l, "{") {
			continue
		}
		var v map[string]any
		if e := json.Unmarshal([]byte(l), &v); e != nil {
			return nil, e
		}
		res = append(res, v)
	}
	if err != nil || len(res) != len(inputs) {
		last := ""
		if len(res) < len(inputs) {
			last = fmt.Sprintf("%q", inputs[len(res)])
		}
		return nil, fmt.Errorf("swift failed: %v, %d/%d results, last input %s\n%s", err, len(res), len(inputs), last, stderr.String())
	}
	return res, nil
}

// Same fixed-size provider as main.swift.
func provider() *markdown.ImageProvider {
	p := markdown.NewImageProvider(2)
	p.Resolve = func(name string) *markdown.CachedImage {
		if strings.HasPrefix(name, "n") {
			return nil
		}
		n := tenn.GraphemeCount(name)
		return &markdown.CachedImage{Size: markdown.Size{Width: float64(40 + 10*(n%7)), Height: float64(20 + 5*(n%5))}}
	}
	return p
}

// canonName: Swift picks any name sharing a hex, Go the first one in table order. A name that
// ColorToHex maps back to a name is replaced by that name, anything else is kept.
func canonName(n string) string {
	if _, ok := markdown.ColorNames[n]; ok {
		if back := markdown.ColorToHex(markdown.ParseColor(n, 1)); !strings.HasPrefix(back, "#") {
			return back
		}
	}
	return n
}

var colorRe = regexp.MustCompile(`(color: )([A-Za-z0-9#-]+)`)

func canonHTML(s string) string {
	return colorRe.ReplaceAllStringFunc(s, func(m string) string {
		return "color: " + canonName(colorRe.FindStringSubmatch(m)[2])
	})
}

// runGo builds the same document as main.swift, tokens and error positions in Swift Character units.
func runGo(source string) map[string]any {
	errs := []any{}
	l := markdown.NewLexer(source)
	l.SetErrorHandler(func(e markdown.LexerError, a, b int) {
		errs = append(errs, map[string]any{"error": string(e), "start": tenn.GraphemeCount(source[:a]), "pos": tenn.GraphemeCount(source[:b])})
	})
	var tokens []markdown.Token
	var out []any
	for t := l.GetToken(); t != nil; t = l.GetToken() {
		tokens = append(tokens, *t)
		out = append(out, map[string]any{
			"type": string(t.Type), "literal": t.Literal, "line": t.Line, "col": t.Col,
			"pos": tenn.GraphemeCount(source[:t.Pos]), "size": tenn.GraphemeCount(t.Literal),
		})
	}
	c := markdown.ParseColor(source, 0.5)
	return map[string]any{
		"tokens": out, "errors": errs,
		"html":  canonHTML(markdown.ToHTML(tokens, 16, "black", provider())),
		"color": []any{c.R, c.G, c.B, c.A},
		"hex":   canonName(markdown.ColorToHex(c)),
	}
}

// roundTrip makes Go values comparable with decoded Swift JSON (ints become float64).
func roundTrip(v map[string]any) map[string]any {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}

func main() {
	n := flag.Int("n", 2000, "number of inputs")
	seed := flag.Uint("seed", 1, "seed")
	root := flag.String("root", "..", "repository root")
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
	sw, err := runSwift(bin, inputs)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	bad, skipped := 0, 0
	for i, s := range inputs {
		a, b := sw[i], roundTrip(runGo(s))
		// Swift traps on unknown color names and huge hex values, main.swift reports those as null/missing.
		if a["html"] == nil {
			skipped++
			delete(a, "html")
			delete(b, "html")
		} else {
			a["html"] = canonHTML(a["html"].(string))
		}
		if _, ok := a["color"]; !ok {
			delete(b, "color")
		}
		if h, ok := a["hex"].(string); ok {
			a["hex"] = canonName(h)
		} else {
			delete(b, "hex")
		}
		var keys []string
		for _, k := range []string{"tokens", "errors", "html", "color", "hex"} {
			if !reflect.DeepEqual(a[k], b[k]) {
				keys = append(keys, k)
			}
		}
		if len(keys) == 0 {
			continue
		}
		bad++
		if bad <= 20 {
			fmt.Printf("DIFF %s %q\n", strings.Join(keys, ","), s)
			for _, k := range keys {
				fmt.Printf("  swift %s: %v\n  go    %s: %v\n", k, a[k], k, b[k])
			}
		}
	}
	fmt.Printf("mdfuzz: %d inputs, seed %d, %d mismatches (%d html skipped: unknown color or huge hex traps Swift)\n", len(inputs), *seed, bad, skipped)
	if bad > 0 {
		os.Exit(1)
	}
}
