package app

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestPNGBase64ConvertsJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(1, 1, color.RGBA{R: 200, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "a.jpg")
	os.WriteFile(path, buf.Bytes(), 0o644)
	s, err := PNGBase64(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(raw))
	if err != nil || got.Bounds().Dx() != 4 || got.Bounds().Dy() != 3 {
		t.Fatalf("not the same picture as a PNG: %v %v", got, err)
	}
	if _, err := PNGBase64(filepath.Join(t.TempDir(), "none.png")); err == nil {
		t.Fatal("no error for a missing file")
	}
}
