package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

// TestTreeCacheBudget: the cache holds as much as its budget allows, not a count of trees; a tree bigger
// than the budget alone stays, so one huge workspace does not reload on every call.
func TestTreeCacheBudget(t *testing.T) {
	dir := t.TempDir()
	texts := map[string]string{
		"a.tenn": "element \"A\" { item a { width 1 } }\n",
		"b.tenn": "element \"B\" { item b { title \"" + strings.Repeat("x", 40000) + "\" } }\n",
	}
	for name, text := range texts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	load := func(name string) *scene.Tree {
		t.Helper()
		tree, err := scene.Load(dir, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return tree
	}
	a, b := load("a.tenn"), load("b.tenn")
	if treeSize(a) >= treeSize(b) {
		t.Fatalf("sizes %d %d: the test needs a tree smaller than the other", treeSize(a), treeSize(b))
	}

	one := &treeCache{entries: map[cacheKey]*cacheEntry{}, budget: treeSize(a)}
	one.put(dir, "a.tenn", a, nil)
	one.put(dir, "b.tenn", b, nil)
	if len(one.entries) != 1 {
		t.Errorf("a budget of one tree holds %d", len(one.entries))
	}
	if _, ok := one.get(dir, "b.tenn", nil); !ok {
		t.Error("the tree just cached was evicted")
	}

	both := &treeCache{entries: map[cacheKey]*cacheEntry{}, budget: treeSize(a) + treeSize(b)}
	both.put(dir, "a.tenn", a, nil)
	both.put(dir, "b.tenn", b, nil)
	if len(both.entries) != 2 {
		t.Errorf("a budget of two trees holds %d", len(both.entries))
	}
}

// TestCacheBudgetEnv: the budget comes from TENNIARB_TREE_CACHE_MB, in megabytes.
func TestCacheBudgetEnv(t *testing.T) {
	t.Setenv("TENNIARB_TREE_CACHE_MB", "64")
	if got := cacheBudget(); got != 64<<20 {
		t.Errorf("budget %d, want 64 MB", got)
	}
	t.Setenv("TENNIARB_TREE_CACHE_MB", "not a number")
	if got := cacheBudget(); got != int64(defaultCacheMB)<<20 {
		t.Errorf("budget %d for a bad value, want the default", got)
	}
}
