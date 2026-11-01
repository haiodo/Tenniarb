package mcpserver

import (
	"strings"
	"testing"
)

// TestGuideReference guards the reference: non-empty, short, sections present, no leftovers of the
// draft spec that agents used to be given.
func TestGuideReference(t *testing.T) {
	if n := strings.Count(refText, "\n"); n > 210 {
		t.Errorf("reference has %d lines, want at most 210", n)
	}
	for _, want := range []string{"## 1. Commands", "## 7. styles and use-style", "## 10. Not supported", "auto-layout", "--from-file"} {
		if !strings.Contains(refText, want) {
			t.Errorf("reference lacks %q", want)
		}
	}
	for _, bad := range []string{"frame ", "pen.dev", "предложение"} {
		if strings.Contains(refText, bad) {
			t.Errorf("reference mentions %q", bad)
		}
	}
	styles := guide("styles")
	if !strings.Contains(styles, "use-style") || !strings.Contains(styles, "one-of") {
		t.Errorf("guide(styles) lacks the styles section: %q", styles)
	}
	for _, bad := range []string{"## 1.", "## 10", "Not supported", "Y axis"} {
		if strings.Contains(styles, bad) {
			t.Errorf("guide(styles) returned another section too: %q", bad)
		}
	}
}
