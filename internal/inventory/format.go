package inventory

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Format int

const (
	FormatINI Format = iota
	FormatYAML
)

func DetectFormat(path string) Format {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yml", ".yaml":
		return FormatYAML
	default:
		return FormatINI
	}
}

func (f Format) String() string {
	switch f {
	case FormatYAML:
		return "YAML"
	default:
		return "INI"
	}
}

func (f Format) Extension() string {
	switch f {
	case FormatYAML:
		return ".yaml"
	default:
		return ".ini"
	}
}

// Load reads an inventory from disk, auto-detecting format.
func Load(path string) (*Inventory, Format, error) {
	format := DetectFormat(path)
	var inv *Inventory
	var err error
	switch format {
	case FormatYAML:
		inv, err = ParseYAMLFile(path)
	default:
		inv, err = ParseINIFile(path)
	}
	if err != nil {
		return nil, format, fmt.Errorf("load %s: %w", path, err)
	}
	return inv, format, nil
}

// Save writes the inventory to disk in the given format.
func Save(inv *Inventory, path string, format Format) error {
	switch format {
	case FormatYAML:
		return WriteYAMLFile(inv, path)
	default:
		return WriteINIFile(inv, path)
	}
}
