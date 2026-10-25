package app

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

// ExportMargin is the margin of Swift's ExportManager: the image is the scene grown by 30 on every side.
const ExportMargin = 30

// EmbedBundleName is the file of the interactive HTML export (make embed builds it from web/packages/embed).
const EmbedBundleName = "tenniarb-embed.min.js"

// exportOptions render the default way: dark mode off, shadows in scene units (not the screen ones).
func exportOptions() render.Options {
	m := float64(ExportMargin)
	return render.Options{Padding: &m}
}

// exportBackground is the fill of a PNG: the light canvas colour when Prefs ask for it, else transparent.
func (p Prefs) exportBackground() gfx.Color {
	if !p.ExportBackground {
		return gfx.Color{}
	}
	c, _ := render.ParseColor(p.Background)
	return c
}

// ExportPNG renders element at scale device pixels per unit.
func ExportPNG(e *model.Element, p Prefs, scale float64) ([]byte, error) {
	return render.PNG(e, exportOptions(), scale, p.exportBackground())
}

// ExportSVG renders element as an SVG document.
func ExportSVG(e *model.Element, p Prefs) ([]byte, error) {
	var buf bytes.Buffer
	if err := render.SVGBackground(&buf, e, exportOptions(), 1, p.exportBackground()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExportHTML is Swift's "Export as HTML": the PNG as an inline image.
func ExportHTML(e *model.Element, p Prefs, scale float64) ([]byte, error) {
	png, err := ExportPNG(e, p, scale)
	if err != nil {
		return nil, err
	}
	size := render.SceneSize(e, exportOptions())
	return fmt.Appendf(nil, "<html>\n\t<body>\n\t\t<img style=\"border: 1px solid #eeeeee;\" width=\"%.0f\" height=\"%.0f\" src=\"data:image/png;base64,%s\"/>\n\t</body>\n</html>",
		size.Width, size.Height, base64.StdEncoding.EncodeToString(png)), nil
}

// ErrNoEmbedBundle: the interactive HTML export needs tenniarb-embed.min.js, which is a build product.
var ErrNoEmbedBundle = errors.New("tenniarb-embed.min.js not found: build it with `make embed` and put it next to the application, or set TENNIARB_EMBED")

// FindEmbedBundle looks for the embed bundle: $TENNIARB_EMBED, then next to the executable and in the
// Resources of a macOS bundle, then in the config directory.
func FindEmbedBundle(exe, configDir string) (string, error) {
	var cands []string
	if p := os.Getenv("TENNIARB_EMBED"); p != "" {
		cands = append(cands, p)
	}
	if exe != "" {
		d := filepath.Dir(exe)
		cands = append(cands, filepath.Join(d, EmbedBundleName), filepath.Join(d, "..", "Resources", EmbedBundleName))
	}
	cands = append(cands, filepath.Join(configDir, EmbedBundleName))
	for _, c := range cands {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, nil
		}
	}
	return "", ErrNoEmbedBundle
}

// InteractiveHTML is Swift's generateInteractiveHtml: the embed bundle and the whole document as base64
// text, opened at the element.
func InteractiveHTML(bundle, docText string, e *model.Element) []byte {
	attr := strings.NewReplacer("&", "&amp;", "\"", "&quot;", "<", "&lt;").Replace
	return fmt.Appendf(nil, `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>%s</title>
<style>html, body { margin: 0; height: 100%%; }</style>
</head>
<body>
<script>%s</script>
<script type="text/x-tenn" data-encoding="base64" style="height: 100vh" data-element="%s">%s</script>
</body>
</html>
`, attr(e.Name), bundle, attr(strings.Join(PathOf(e), "/")), base64.StdEncoding.EncodeToString([]byte(docText)))
}

// ExportName is the default file name of an export of e.
func ExportName(e *model.Element, ext string) string {
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, e.Name)
	if name == "" {
		name = "Untitled"
	}
	return name + ext
}
