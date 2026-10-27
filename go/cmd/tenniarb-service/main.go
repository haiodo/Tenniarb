// tenniarb-service hosts the Tenniarb MCP tools for every agent session of the user in one process.
// tenniarb-mcp starts it on demand; the desktop app connects to it to share its open documents.
//
//	tenniarb-service [-dir DIR] [-idle 30m]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/haiodo/tenniarb/go/mcpserver"
	"github.com/haiodo/tenniarb/go/servicewire"
)

func main() {
	def, err := servicewire.Dir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tenniarb-service:", err)
		os.Exit(1)
	}
	idleDef := 30 * time.Minute
	if v := os.Getenv("TENNIARB_SERVICE_IDLE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			idleDef = d
		}
	}
	dir := flag.String("dir", def, "service directory: endpoint, lock, socket and log ($"+servicewire.EnvDir+")")
	idle := flag.Duration("idle", idleDef, "exit after this long without MCP sessions; 0 never ($TENNIARB_SERVICE_IDLE)")
	flag.Parse()
	logger := log.New(os.Stderr, fmt.Sprintf("tenniarb-service[%d] ", os.Getpid()), log.LstdFlags)

	version := ""
	if exe, err := os.Executable(); err == nil {
		version, _ = servicewire.BinaryVersion(exe)
	}
	ln, err := servicewire.Listen(*dir, version)
	if errors.Is(err, servicewire.ErrRunning) {
		// Two proxies started a service at once; the other one listens.
		return
	}
	if err != nil {
		logger.Fatal(err)
	}
	logger.Printf("listening in %s, version %s, idle exit %v", *dir, version, *idle)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	sv := &mcpserver.Service{Version: version, Idle: *idle, Log: logger}
	if err := sv.Serve(ctx, ln); err != nil {
		logger.Fatal(err)
	}
	logger.Print("stopped")
}
