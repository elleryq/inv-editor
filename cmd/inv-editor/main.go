package main

import (
	"fmt"
	"os"

	"github.com/elleryq/inv-editor/internal/inventory"
	"github.com/elleryq/inv-editor/internal/tui"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: inv-editor <inventory-file>")
		os.Exit(1)
	}
	filePath := os.Args[1]

	var inv *inventory.Inventory
	var format inventory.Format

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// new file — start empty, detect format from extension
		format = inventory.DetectFormat(filePath)
		inv = inventory.New()
	} else {
		var err error
		inv, format, err = inventory.Load(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading %s: %v\n", filePath, err)
			os.Exit(1)
		}
	}

	if err := tui.Run(inv, filePath, format); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
