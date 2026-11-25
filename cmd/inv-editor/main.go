package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/elleryq/inv-editor/internal/inventory"
	"github.com/elleryq/inv-editor/internal/tui"
	"github.com/elleryq/inv-editor/internal/web"
)

const helpText = `inv-editor — terminal Ansible inventory editor

USAGE
  inv-editor <inventory-file>
  inv-editor serve <inventory-file> [flags]
  inv-editor help | --help | -h
  inv-editor version | --version

SUBCOMMANDS
  (none)   Open inventory file in the terminal UI (TUI).
           Create a new file if it does not exist.
  serve    Start a web interface for the inventory file.
  version  Print the version and exit.

TUI CONTROLS
  Tab / Shift+Tab   Cycle between panels (Groups → Hosts → Variables)
  G / H / V         Jump to Groups / Hosts / Variables panel
  ↑ ↓  or  j k      Navigate items
  Shift+← →         Scroll panel left / right (horizontal)
  n                 New item (group, host, or variable)
  e / Enter         Edit selected item
  d / Delete        Delete selected item
  m                 Move host to another group
  c                 Copy host to another group
  M                 Move (reparent) group under a different parent
  v                 Open variables for the selected group or host
  s                 Save to original file
  x                 Export to a different format / path
  q                 Quit (prompts to save if modified)
  ?                 Toggle help overlay

SERVE FLAGS
  --host string     Address to listen on (default "127.0.0.1")
  --port int        Port to listen on (default 8080)
  --readonly        Start in read-only mode (mutations return 403)

EXAMPLES
  inv-editor vc8.ini
  inv-editor new-inventory.yaml
  inv-editor serve vc8.yaml --port 9090
  inv-editor serve vc8.yaml --host 127.0.0.1
  inv-editor serve vc8.yaml --host 0.0.0.0 --readonly

SUPPORTED FORMATS
  Read/write: INI (.ini, .cfg)  |  YAML (.yml, .yaml)
  /download endpoint always returns YAML.
`

func main() {
	if len(os.Args) < 2 {
		printUsageShort()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "help", "--help", "-h":
		fmt.Print(helpText)
		os.Exit(0)
	case "version", "--version":
		fmt.Println("inv-editor", buildVersion())
		os.Exit(0)
	case "serve":
		runServe(os.Args[2:])
	default:
		runTUI(os.Args[1])
	}
}

func printUsageShort() {
	fmt.Fprintln(os.Stderr, "usage: inv-editor <inventory-file>")
	fmt.Fprintln(os.Stderr, "       inv-editor serve <inventory-file> [--host H] [--port P] [--readonly]")
	fmt.Fprintln(os.Stderr, "       inv-editor --help | --version")
}

func runTUI(filePath string) {
	inv, format := loadOrNew(filePath)
	if err := tui.Run(inv, filePath, format); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// serveOptions holds the parsed arguments of the serve subcommand.
type serveOptions struct {
	filePath string
	host     string
	port     int
	readonly bool
}

var errMissingInventoryFile = errors.New("missing inventory file")

// parseServeArgs parses the serve subcommand's arguments. Flags may be given
// before or after the inventory file ("serve --readonly inv.yaml" and
// "serve inv.yaml --readonly" are equivalent). The standard flag package
// stops at the first non-flag argument, so flags after the file are parsed
// in a second pass instead of being silently ignored.
// Parse errors and usage are written to output.
func parseServeArgs(args []string, output io.Writer) (*serveOptions, error) {
	opts := &serveOptions{}
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&opts.host, "host", "127.0.0.1", "address to listen on")
	fs.IntVar(&opts.port, "port", 8080, "port to listen on")
	fs.BoolVar(&opts.readonly, "readonly", false, "start in read-only mode")
	fs.Usage = func() {
		fmt.Fprintln(output, "usage: inv-editor serve <inventory-file> [flags]")
		fmt.Fprintln(output)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() < 1 {
		fs.Usage()
		return nil, errMissingInventoryFile
	}
	opts.filePath = fs.Arg(0)

	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		err := fmt.Errorf("unexpected arguments after inventory file: %s", strings.Join(fs.Args(), " "))
		fmt.Fprintln(output, err)
		fs.Usage()
		return nil, err
	}
	return opts, nil
}

func runServe(args []string) {
	opts, err := parseServeArgs(args, os.Stderr)
	switch {
	case errors.Is(err, flag.ErrHelp):
		os.Exit(0)
	case errors.Is(err, errMissingInventoryFile):
		os.Exit(1)
	case err != nil:
		os.Exit(2) // same as flag.ExitOnError
	}

	inv, format := loadOrNew(opts.filePath)

	srv, err := web.NewServer(inv, opts.filePath, format, web.Options{
		Host:     opts.host,
		Port:     opts.port,
		ReadOnly: opts.readonly,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "web server init error: %v\n", err)
		os.Exit(1)
	}
	if err := srv.Start(opts.host, opts.port); err != nil {
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
