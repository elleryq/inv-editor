package main

import (
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

const manPagePath = "../../docs/man/inv-editor.1.md"

// manSections splits the man page source into top-level "# NAME" sections.
func manSections(t *testing.T) (title string, sections map[string]string) {
	t.Helper()
	data, err := os.ReadFile(manPagePath)
	if err != nil {
		t.Fatalf("read man page: %v", err)
	}
	title, body, _ := strings.Cut(string(data), "\n")
	sections = map[string]string{}
	var name string
	for line := range strings.Lines(body) {
		if heading, ok := strings.CutPrefix(line, "# "); ok {
			name = strings.TrimSpace(heading)
			continue
		}
		sections[name] += line
	}
	for _, want := range []string{"NAME", "SYNOPSIS", "COMMANDS", "OPTIONS", "TUI KEYS"} {
		if _, ok := sections[want]; !ok {
			t.Fatalf("man page has no %q section", want)
		}
	}
	return title, sections
}

func TestManPageTitlePlaceholders(t *testing.T) {
	title, _ := manSections(t)
	// packaging/build.sh substitutes these when generating the man page.
	for _, p := range []string{"@VERSION@", "@DATE@"} {
		if !strings.Contains(title, p) {
			t.Errorf("man page title %q lacks placeholder %s", title, p)
		}
	}
}

func TestManPageDocumentsServeFlags(t *testing.T) {
	_, sections := manSections(t)
	fs, _ := newServeFlags(io.Discard)
	fs.VisitAll(func(f *flag.Flag) {
		bold := "**--" + f.Name + "**"
		for _, sec := range []string{"SYNOPSIS", "OPTIONS"} {
			if !strings.Contains(sections[sec], bold) {
				t.Errorf("man page %s does not mention %s", sec, bold)
			}
		}
		if !strings.Contains(helpText, "--"+f.Name) {
			t.Errorf("--help text does not mention --%s", f.Name)
		}
	})
}

func TestManPageDocumentsCommands(t *testing.T) {
	_, sections := manSections(t)
	// Keep in sync with the switch in main().
	commands := []string{"serve", "help", "--help", "-h", "version", "--version"}
	for _, c := range commands {
		bold := "**" + c + "**"
		// SYNOPSIS lines start with "**inv-editor <command>**".
		if !strings.Contains(sections["SYNOPSIS"], bold) &&
			!strings.Contains(sections["SYNOPSIS"], "**inv-editor "+c+"**") {
			t.Errorf("man page SYNOPSIS does not mention %s", c)
		}
		if !strings.Contains(sections["COMMANDS"], bold) {
			t.Errorf("man page COMMANDS does not mention %s", bold)
		}
	}
}
