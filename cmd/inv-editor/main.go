package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/elleryq/inv-editor/internal/inventory"
	"github.com/elleryq/inv-editor/internal/tui"
	"github.com/elleryq/inv-editor/internal/web"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: inv-editor <inventory-file>")
		fmt.Fprintln(os.Stderr, "       inv-editor serve <inventory-file> [flags]")
		os.Exit(1)
	}

	if os.Args[1] == "serve" {
		runServe(os.Args[2:])
		return
	}

	runTUI(os.Args[1])
}

func runTUI(filePath string) {
	inv, format := loadOrNew(filePath)
	if err := tui.Run(inv, filePath, format); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	host := fs.String("host", "0.0.0.0", "address to listen on")
	port := fs.Int("port", 8080, "port to listen on")
	readonly := fs.Bool("readonly", false, "start in read-only mode")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: inv-editor serve <inventory-file> [--host H] [--port P] [--readonly]")
		os.Exit(1)
	}
	filePath := fs.Arg(0)

	inv, format := loadOrNew(filePath)

	srv, err := web.NewServer(inv, filePath, format, web.Options{
		Host:     *host,
		Port:     *port,
		ReadOnly: *readonly,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "web server init error: %v\n", err)
		os.Exit(1)
	}
	if err := srv.Start(*host, *port); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func loadOrNew(filePath string) (*inventory.Inventory, inventory.Format) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return inventory.New(), inventory.DetectFormat(filePath)
	}
	inv, format, err := inventory.Load(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading %s: %v\n", filePath, err)
		os.Exit(1)
	}
	return inv, format
}
