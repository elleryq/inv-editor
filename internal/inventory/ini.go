package inventory

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ParseINIFile parses an Ansible INI inventory file.
//
// Supported section types:
//
//	[groupname]           — host entries
//	[groupname:vars]      — group variables
//	[groupname:children]  — child group names
func ParseINIFile(path string) (*Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseINI(data)
}

type sectionKind int

const (
	kindHosts    sectionKind = iota
	kindVars
	kindChildren
)

func ParseINI(data []byte) (*Inventory, error) {
	inv := New()

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var currentGroup string
	var currentKind sectionKind

	// collect children relationships to wire up after all groups are created
	type childRel struct{ parent, child string }
	var childRels []childRel

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := line[1 : len(line)-1]
			switch {
			case strings.HasSuffix(section, ":vars"):
				currentGroup = strings.TrimSuffix(section, ":vars")
				currentKind = kindVars
			case strings.HasSuffix(section, ":children"):
				currentGroup = strings.TrimSuffix(section, ":children")
				currentKind = kindChildren
				// ensure parent group exists
				if currentGroup != "all" && inv.Group(currentGroup) == nil {
					inv.AddGroup(currentGroup)
				}
			default:
				currentGroup = section
				currentKind = kindHosts
				if currentGroup != "all" && inv.Group(currentGroup) == nil {
					inv.AddGroup(currentGroup)
				}
			}
			continue
		}

		if currentGroup == "" {
			continue
		}

		switch currentKind {
		case kindHosts:
			parts := splitVars(line)
			if len(parts) == 0 {
				continue
			}
			hostName := parts[0]
			inv.AddHost(hostName, currentGroup)
			for _, pair := range parts[1:] {
				k, v, ok := strings.Cut(pair, "=")
				if ok {
					inv.SetHostVar(hostName, strings.TrimSpace(k), unquote(strings.TrimSpace(v)))
				}
			}

		case kindVars:
			k, v, ok := strings.Cut(line, "=")
			if ok {
				inv.SetGroupVar(currentGroup, strings.TrimSpace(k), strings.TrimSpace(v))
			}

		case kindChildren:
			childName := strings.TrimSpace(line)
			if childName != "" {
				childRels = append(childRels, childRel{parent: currentGroup, child: childName})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse INI: %w", err)
	}

	// wire up parent/child relationships
	for _, rel := range childRels {
		child := inv.Group(rel.child)
		if child == nil {
			// child group may not have been declared yet — create it
			inv.AddGroup(rel.child)
			child = inv.Group(rel.child)
		}
		parent := inv.Group(rel.parent)
		if parent == nil {
			continue
		}
		child.Parent = rel.parent
		if !containsStr(parent.Children, rel.child) {
			parent.Children = append(parent.Children, rel.child)
		}
	}

	return inv, nil
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// unquote strips surrounding single or double quotes.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// splitVars splits "host key1=val1 key2=val2" respecting quoted values.
func splitVars(s string) []string {
	var parts []string
	var cur strings.Builder
	inQuote := false
	quoteChar := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			cur.WriteByte(c)
			if c == quoteChar {
				inQuote = false
			}
		} else if c == '"' || c == '\'' {
			inQuote = true
			quoteChar = c
			cur.WriteByte(c)
		} else if c == ' ' || c == '\t' {
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		} else {
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// WriteINIFile writes the inventory to path in Ansible INI format.
// Groups are written with their hosts, then :vars, then :children sections.
func WriteINIFile(inv *Inventory, path string) error {
	var sb strings.Builder

	for _, g := range inv.Groups() {
		if g.Name == "all" {
			continue
		}

		// [group]
		fmt.Fprintf(&sb, "[%s]\n", g.Name)
		for _, hostName := range g.Hosts {
			h := inv.Host(hostName)
			if h == nil {
				continue
			}
			sb.WriteString(hostName)
			for _, k := range sortedKeys(h.Vars) {
				fmt.Fprintf(&sb, " %s=%s", k, h.Vars[k])
			}
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')

		// [group:vars]
		if len(g.Vars) > 0 {
			fmt.Fprintf(&sb, "[%s:vars]\n", g.Name)
			for _, k := range sortedKeys(g.Vars) {
				fmt.Fprintf(&sb, "%s=%s\n", k, g.Vars[k])
			}
			sb.WriteByte('\n')
		}

		// [group:children]
		if len(g.Children) > 0 {
			fmt.Fprintf(&sb, "[%s:children]\n", g.Name)
			for _, child := range g.Children {
				fmt.Fprintf(&sb, "%s\n", child)
			}
			sb.WriteByte('\n')
		}
	}

	// [all:vars]
	allGroup := inv.Group("all")
	if allGroup != nil && len(allGroup.Vars) > 0 {
		sb.WriteString("[all:vars]\n")
		for _, k := range sortedKeys(allGroup.Vars) {
			fmt.Fprintf(&sb, "%s=%s\n", k, allGroup.Vars[k])
		}
		sb.WriteByte('\n')
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
