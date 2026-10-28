package main

import (
	"strings"
	"testing"
)

func TestPreserveSpace(t *testing.T) {
	got := string(preserveSpace([]byte(`<svg xmlns="x"><text>  a</text></svg>`)))
	if !strings.HasPrefix(got, `<svg xml:space="preserve" xmlns="x">`) {
		t.Errorf("got %s", got)
	}
}
