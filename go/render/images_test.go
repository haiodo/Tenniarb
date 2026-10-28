package render

import (
	"container/list"
	"crypto/sha256"
	"testing"
)

// A rebuild must get the same Raster pointer for the same payload: the renderer caches by pointer.
func TestStdImageDecoderReusesRaster(t *testing.T) {
	a, b := StdImageDecoder(png1), StdImageDecoder(png1)
	if a == nil || a != b || a.Image != b.Image {
		t.Fatalf("one payload, two images: %p %p", a, b)
	}
	if StdImageDecoder("not-base64!!") != nil {
		t.Error("broken data must not decode")
	}
}

func TestDecodedCacheEvictsLeastRecentlyUsed(t *testing.T) {
	c := &decodedCache{limit: 2 * 16, byKey: map[[sha256.Size]byte]*list.Element{}, order: list.New()}
	img := func() *DecodedImage { return &DecodedImage{Width: 2, Height: 2} } // 16 bytes
	k1, k2, k3 := sha256.Sum256([]byte("1")), sha256.Sum256([]byte("2")), sha256.Sum256([]byte("3"))
	i1 := c.put(k1, img())
	c.put(k2, img())
	c.get(k1)
	c.put(k3, img())
	if c.get(k1) != i1 || c.get(k2) != nil || c.get(k3) == nil {
		t.Error("the least recently used image must go first")
	}
	if big := c.put(sha256.Sum256([]byte("big")), &DecodedImage{Width: 100, Height: 100}); big == nil || len(c.byKey) != 2 {
		t.Error("an image above the limit is returned but not kept")
	}
}
