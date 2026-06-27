package inventory

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// yamlGroup mirrors the Ansible YAML inventory group structure.
// Both Hosts and Children use map[string]*yamlGroup / map[string]map[string]string
// so they can be nil when empty (omitempty).
type yamlGroup struct {
	Hosts    map[string]map[string]string `yaml:"hosts,omitempty"`
	Vars     map[string]string            `yaml:"vars,omitempty"`
	Children map[string]*yamlGroup        `yaml:"children,omitempty"`
}

type yamlInventory struct {
	All yamlGroup `yaml:"all"`
}

func ParseYAMLFile(path string) (*Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseYAML(data)
}

func ParseYAML(data []byte) (*Inventory, error) {
	var raw yamlInventory
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	inv := New()

	// all-group vars
	for k, v := range raw.All.Vars {
		inv.SetGroupVar("all", k, v)
	}

	// hosts directly under "all" (ungrouped)
	for hostName, hostVars := range raw.All.Hosts {
		inv.AddHost(hostName, "all")
		for k, v := range hostVars {
			inv.SetHostVar(hostName, k, v)
		}
	}

	// recursively parse children
	for groupName, groupData := range raw.All.Children {
		parseYAMLGroup(inv, groupName, "", groupData)
	}

	return inv, nil
}

func parseYAMLGroup(inv *Inventory, name, parentName string, data *yamlGroup) {
	if data == nil {
		data = &yamlGroup{}
	}

	// create group under parent
	if parentName == "" || parentName == "all" {
		inv.AddGroup(name)
	} else {
		inv.AddGroupUnder(parentName, name)
	}

	for k, v := range data.Vars {
		inv.SetGroupVar(name, k, v)
	}
	for hostName, hostVars := range data.Hosts {
		inv.AddHost(hostName, name)
		for k, v := range hostVars {
			inv.SetHostVar(hostName, k, v)
		}
	}
	for childName, childData := range data.Children {
		parseYAMLGroup(inv, childName, name, childData)
	}
}

// WriteYAMLFile writes the inventory following the single-source-of-truth principle:
//   - all.hosts  : every host in the inventory with its full vars (ansible_host, etc.)
//   - child groups: list host names only — no duplicated vars
func WriteYAMLFile(inv *Inventory, path string) error {
	allGroup := inv.Group("all")
	raw := yamlInventory{
		All: yamlGroup{
			Vars:     nilIfEmpty(allGroup.Vars),
			Hosts:    buildYAMLAllHosts(inv),
			Children: buildYAMLChildren(inv, allGroup),
		},
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal YAML: %w", err)
	}
	return os.WriteFile(path, out, 0644)
}

// buildYAMLAllHosts collects every unique host in the inventory with full vars.
// Written under all.hosts as the single source of truth for host variables.
func buildYAMLAllHosts(inv *Inventory) map[string]map[string]string {
	seen := make(map[string]bool)
	hosts := make(map[string]map[string]string)
	for _, g := range inv.Groups() {
		for _, hostName := range g.Hosts {
			if seen[hostName] {
				continue
			}
			seen[hostName] = true
			h := inv.Host(hostName)
			if h == nil {
				continue
			}
			if len(h.Vars) == 0 {
				hosts[hostName] = nil
			} else {
				hosts[hostName] = h.Vars
			}
		}
	}
	if len(hosts) == 0 {
		return nil
	}
	return hosts
}

// buildYAMLChildren recursively builds the children map for a group.
// Child group hosts are written as names only (no vars) per the single-source-of-truth rule.
func buildYAMLChildren(inv *Inventory, g *Group) map[string]*yamlGroup {
	var childNames []string
	if g.Name == "all" {
		for _, tg := range inv.TopLevelGroups() {
			childNames = append(childNames, tg.Name)
		}
	} else {
		childNames = g.Children
	}

	if len(childNames) == 0 {
		return nil
	}

	children := make(map[string]*yamlGroup)
	for _, childName := range childNames {
		child := inv.Group(childName)
		if child == nil {
			continue
		}
		yg := &yamlGroup{
			Vars:     nilIfEmpty(child.Vars),
			Hosts:    buildYAMLChildHosts(child),
			Children: buildYAMLChildren(inv, child),
		}
		children[childName] = yg
	}
	if len(children) == 0 {
		return nil
	}
	return children
}

// buildYAMLChildHosts writes only host names (nil vars) for a non-all group.
// Vars are written once in all.hosts, not repeated here.
func buildYAMLChildHosts(g *Group) map[string]map[string]string {
	if len(g.Hosts) == 0 {
		return nil
	}
	hosts := make(map[string]map[string]string, len(g.Hosts))
	for _, hostName := range g.Hosts {
		hosts[hostName] = nil
	}
	return hosts
}

func nilIfEmpty(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return m
}

// sortedStringKeys returns sorted keys (kept for potential future use).
func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
