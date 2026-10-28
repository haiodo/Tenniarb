package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestDispatch(t *testing.T) {
	cases := []struct {
		args   []string
		launch bool
		files  []string
		code   int
		out    string // substring of stdout
		errOut string // substring of stderr
	}{
		{args: nil, launch: true},
		{args: []string{"a.tenn", "b.tenn"}, launch: true, files: []string{"a.tenn", "b.tenn"}},
		{args: []string{"--help"}, out: "usage:"},
		{args: []string{"-h"}, out: "usage:"},
		{args: []string{"help"}, out: "tenniarb check"},
		{args: []string{"--bogus"}, code: 2, errOut: "unknown flag --bogus"},
		{args: []string{"a.tenn", "-x"}, code: 2, errOut: "unknown flag -x"},
		{args: []string{"-psn_0_123", "a.tenn"}, launch: true, files: []string{"a.tenn"}},
		{args: []string{"check"}, code: 2, errOut: "usage: tenniarb check"},
	}
	for _, c := range cases {
		var out, errOut bytes.Buffer
		launch, files, code := dispatch(c.args, &out, &errOut)
		if launch != c.launch || code != c.code || !reflect.DeepEqual(files, c.files) {
			t.Errorf("%v: launch %v files %v code %d", c.args, launch, files, code)
		}
		if !strings.Contains(out.String(), c.out) || !strings.Contains(errOut.String(), c.errOut) {
			t.Errorf("%v: stdout %q stderr %q", c.args, out.String(), errOut.String())
		}
	}
}
