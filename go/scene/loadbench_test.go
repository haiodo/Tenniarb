package scene_test

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/haiodo/tenniarb/go/scene"
)

// benchRoot is the workspace BENCHDST names: a folder with PlatformDesign.tenn and what it mounts.
func benchRoot(b *testing.B) string {
	b.Helper()
	root := os.Getenv("BENCHDST")
	if root == "" {
		b.Skip("BENCHDST not set: a workspace with PlatformDesign.tenn")
	}
	return root
}

// BenchmarkLoadAuto loads a whole workspace through go/scene, as an edit of the app does.
func BenchmarkLoadAuto(b *testing.B) {
	root := benchRoot(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree, err := scene.LoadAuto(root, "PlatformDesign.tenn", nil)
		if err != nil || tree == nil {
			b.Fatalf("load: %v", err)
		}
	}
}

// TestLoadAutoMem is the same load measured once, with what a profile of the app reports: the time, the
// bytes allocated and the live heap with one tree held. Not a test: it fails only when the file does.
func TestLoadAutoMem(t *testing.T) {
	root := os.Getenv("BENCHDST")
	if root == "" {
		t.Skip("BENCHDST not set: a workspace with PlatformDesign.tenn")
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	tree, err := scene.LoadAuto(root, "PlatformDesign.tenn", nil)
	if err != nil || tree == nil {
		t.Fatalf("load: %v", err)
	}
	ms := time.Since(started)
	runtime.GC()
	runtime.ReadMemStats(&after)
	t.Logf("LoadAuto: %d ms, %.1f MB allocated, live %.1f MB, %d objects",
		ms.Milliseconds(), mb(after.TotalAlloc-before.TotalAlloc), mb(after.HeapAlloc-before.HeapAlloc), after.HeapObjects-before.HeapObjects)
	runtime.KeepAlive(tree)
}

func mb(b uint64) float64 { return float64(b) / (1 << 20) }
