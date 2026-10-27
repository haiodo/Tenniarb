// tenniarb-mcp is the stdio MCP server an agent starts. It is a proxy: it connects to
// tenniarb-service (starting it when none runs) and pumps bytes both ways, so every agent session
// costs one small process while the tools, caches and renderer live once in the service.
//
//	tenniarb-mcp [-root DIR] [-dir SERVICE_DIR]
//
// It must link the standard library and servicewire only: no go/scene, go/render, gowt, goja or
// the MCP SDK (TestProxyDeps). Measured 2026-10-27, macOS arm64, -s -w: binary 2.7 MB, RSS 5.8-6 MB
// connected and idle (a bare Go stdio pump is 4.9 MB); the service is 20 MB and 31-36 MB RSS once
// per user, where the in-process server took 27 MB RSS in every session.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/haiodo/tenniarb/go/servicewire"
)

func main() {
	root := flag.String("root", "", "workspace root directory (default: the current directory); every path argument stays inside it")
	dir := flag.String("dir", "", "service directory (default: $"+servicewire.EnvDir+" or the user's config directory)")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: tenniarb-mcp [-root DIR] [-dir SERVICE_DIR]")
		os.Exit(2)
	}
	if err := run(*root, *dir); err != nil {
		fmt.Fprintln(os.Stderr, "tenniarb-mcp:", err)
		os.Exit(1)
	}
}

func run(root, dir string) error {
	var err error
	if root == "" {
		if root, err = os.Getwd(); err != nil {
			return err
		}
	}
	if root, err = filepath.Abs(root); err != nil {
		return err
	}
	if dir == "" {
		if dir, err = servicewire.Dir(); err != nil {
			return err
		}
	}
	bin, err := servicewire.ServiceBin()
	if err != nil {
		return fmt.Errorf("tenniarb-service not found next to tenniarb-mcp or on PATH (or set $%s): %v", servicewire.EnvBin, err)
	}
	version, err := servicewire.BinaryVersion(bin)
	if err != nil {
		return err
	}
	h := servicewire.Hello{Role: servicewire.RoleMCP, Version: version, Root: root}
	conn, err := servicewire.Connect(dir, h, func() error { return servicewire.Start(bin, dir) }, 15*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	fromService := make(chan error, 1)
	go func() {
		_, err := io.Copy(os.Stdout, conn)
		fromService <- err
	}()
	fromClient := make(chan struct{})
	go func() {
		io.Copy(conn, os.Stdin)
		conn.CloseWrite()
		close(fromClient)
	}()
	select {
	case err := <-fromService:
		if err == nil {
			err = io.EOF
		}
		return fmt.Errorf("tenniarb-service closed the session: %v", err)
	case <-fromClient:
		// The client is gone; let the service finish the answers in flight.
		select {
		case <-fromService:
		case <-time.After(5 * time.Second):
		}
		return nil
	}
}
