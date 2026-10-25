// tenniarb-mcp is a stdio MCP server for a .tenn workspace.
//
//	tenniarb-mcp -root DIR
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/haiodo/tenniarb/go/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	root := flag.String("root", "", "workspace root directory (required); every path argument stays inside it")
	flag.Parse()
	if *root == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: tenniarb-mcp -root DIR")
		os.Exit(2)
	}
	s, err := mcpserver.New(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tenniarb-mcp:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := s.MCP().Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "tenniarb-mcp:", err)
		os.Exit(1)
	}
}
