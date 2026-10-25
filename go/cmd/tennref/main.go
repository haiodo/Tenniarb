// tennref is the Go counterpart of web/packages/core/fuzz/main.swift: base64 UTF-8 inputs on
// stdin, one line each, one JSON result per line on stdout.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/haiodo/tenniarb/go/internal/ref"
)

func main() {
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for {
		line, err := in.ReadString('\n')
		if len(line) > 0 {
			if code := emit(out, line); code != 0 {
				out.Flush()
				os.Exit(code)
			}
		}
		if err != nil {
			return
		}
	}
}

func emit(out *bufio.Writer, line string) int {
	for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r') {
		line = line[:len(line)-1]
	}
	raw, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad base64:", err)
		return 1
	}
	data, err := json.Marshal(ref.Run(string(raw)))
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal:", err)
		return 1
	}
	out.Write(data)
	out.WriteByte('\n')
	return 0
}
