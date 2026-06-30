package inventory

import (
	"fmt"
	"slices"
)

// Host represents a single managed node.
type Host struct {
	Name string
	Vars map[string]string
}

// Group represents an Ansible inventory group.
type Group struct {
	Name     string
	Parent   string   // "" means direct child of "all" (top-level)
	Children []string // ordered child group names
	Hosts    []string // ordered host names (direct members only)
	Vars     map[string]string
}

// Inventory holds the full in-memory state.
type Inventory struct {
	groups []*Group          // insertion-ordered; "all" is always groups[0]
	hosts  map[string]*Host  // keyed by host name
	byName map[string]*Group // keyed by group name
}

func New() *Inventory {
	inv := &Inventory{
		hosts:  make(map[string]*Host),
		byName: make(map[string]*Group),
	}
	inv.addGroupRaw(&Group{Name: "all", Vars: make(map[string]string)})
	return inv
}

// Groups returns all groups in insertion order.
func (inv *Inventory) Groups() []*Group {
	return inv.groups
}

// Group returns the group by name, or nil.
func (inv *Inventory) Group(name string) *Group {
	return inv.byName[name]
}

// Host returns the host by name, or nil.
func (inv *Inventory) Host(name string) *Host {
	return inv.hosts[name]
}

// TopLevelGroups returns groups whose parent is "" (direct children of "all"),
// excluding "all" itself.
func (inv *Inventory) TopLevelGroups() []*Group {
	var out []*Group
	for _, g := range inv.groups {
		if g.Name != "all" && g.Parent == "" {
			out = append(out, g)
		}
	}
	return out
}

// ChildGroups returns the direct child groups of the named group.
func (inv *Inventory) ChildGroups(name string) []*Group {
	g := inv.byName[name]
	if g == nil {
		return nil
	}
	out := make([]*Group, 0, len(g.Children))
	for _, child := range g.Children {
		if c := inv.byName[child]; c != nil {
			out = append(out, c)
		}
	}
	return out
}

// HostsInGroup returns the Host objects that are direct members of the group.
func (inv *Inventory) HostsInGroup(groupName string) []*Host {
	g := inv.byName[groupName]
	if g == nil {
		return nil
	}
	out := make([]*Host, 0, len(g.Hosts))
	for _, name := range g.Hosts {
		if h, ok := inv.hosts[name]; ok {
			out = append(out, h)
		}
	}
	return out
}

// AddGroup adds a new top-level group (parent = "all").
// Returns false if the name already exists.
func (inv *Inventory) AddGroup(name string) bool {
	return inv.AddGroupUnder("", name)
}

// AddGroupUnder adds a new group as a child of parentName.
// Use parentName="" to create a top-level group.
// Returns false if name already exists.
func (inv *Inventory) AddGroupUnder(parentName, name string) bool {
	if inv.byName[name] != nil {
		return false
	}
	g := &Group{Name: name, Vars: make(map[string]string)}

	if parentName == "" || parentName == "all" {
		g.Parent = ""
	} else {
		parent := inv.byName[parentName]
		if parent == nil {
			return false
		}
		g.Parent = parentName
		parent.Children = append(parent.Children, name)
	}
	inv.addGroupRaw(g)
	return true
}

func (inv *Inventory) addGroupRaw(g *Group) {
	if g.Vars == nil {
		g.Vars = make(map[string]string)
	}
	inv.groups = append(inv.groups, g)
	inv.byName[g.Name] = g
}

// RenameGroup renames a group. Returns false if newName already exists or oldName not found.
func (inv *Inventory) RenameGroup(oldName, newName string) bool {
	if oldName == "all" || inv.byName[newName] != nil {
		return false
	}
	g := inv.byName[oldName]
	if g == nil {
		return false
	}
	delete(inv.byName, oldName)
	g.Name = newName
	inv.byName[newName] = g

	// update parent's Children list
	if g.Parent != "" {
		parent := inv.byName[g.Parent]
		if parent != nil {
			for i, c := range parent.Children {
				if c == oldName {
					parent.Children[i] = newName
				}
			}
		}
	}
	// update children's Parent field
	for _, childName := range g.Children {
		if child := inv.byName[childName]; child != nil {
			child.Parent = newName
		}
	}
	return true
}

// DeleteGroup removes a group. Its direct children are re-parented to the
// deleted group's parent (or become top-level). Hosts remain in the inventory.
// Returns false if group not found or group is "all".
func (inv *Inventory) DeleteGroup(name string) bool {
	if name == "all" {
		return false
	}
	g := inv.byName[name]
	if g == nil {
		return false
	}

	// re-parent children
	for _, childName := range g.Children {
		if child := inv.byName[childName]; child != nil {
			child.Parent = g.Parent
			if g.Parent != "" {
				parent := inv.byName[g.Parent]
				if parent != nil {
					parent.Children = append(parent.Children, childName)
				}
			}
		}
	}

	// remove from parent's Children list
	if g.Parent != "" {
		parent := inv.byName[g.Parent]
		if parent != nil {
			parent.Children = slices.DeleteFunc(parent.Children, func(c string) bool { return c == name })
		}
	}

	inv.groups = slices.DeleteFunc(inv.groups, func(x *Group) bool { return x.Name == name })
	delete(inv.byName, name)
	return true
}

// AddHost adds a host to a group, creating the host record if needed.
// Returns false if the host is already in that group.
func (inv *Inventory) AddHost(hostName, groupName string) bool {
	if groupName == "" {
		groupName = "all"
	}
	g := inv.byName[groupName]
	if g == nil {
		return false
	}
	if slices.Contains(g.Hosts, hostName) {
		return false
	}
	if inv.hosts[hostName] == nil {
		inv.hosts[hostName] = &Host{Name: hostName, Vars: make(map[string]string)}
	}
	g.Hosts = append(g.Hosts, hostName)
	return true
}

// RenameHost renames a host across all groups.
func (inv *Inventory) RenameHost(oldName, newName string) bool {
	if inv.hosts[oldName] == nil || inv.hosts[newName] != nil {
		return false
	}
	h := inv.hosts[oldName]
	delete(inv.hosts, oldName)
	h.Name = newName
	inv.hosts[newName] = h
	for _, g := range inv.groups {
		for i, n := range g.Hosts {
			if n == oldName {
				g.Hosts[i] = newName
			}
		}
	}
	return true
}

// DeleteHost removes a host from a specific group.
// If removeCompletely is true, removes from all groups and deletes the host record.
func (inv *Inventory) DeleteHost(hostName, groupName string, removeCompletely bool) bool {
	g := inv.byName[groupName]
	if g == nil {
		return false
	}
	g.Hosts = slices.DeleteFunc(g.Hosts, func(n string) bool { return n == hostName })
	if removeCompletely {
		for _, grp := range inv.groups {
			grp.Hosts = slices.DeleteFunc(grp.Hosts, func(n string) bool { return n == hostName })
		}
		delete(inv.hosts, hostName)
	}
	return true
}

// MoveHost moves a host from one group to another.
func (inv *Inventory) MoveHost(hostName, fromGroup, toGroup string) bool {
	from := inv.byName[fromGroup]
	to := inv.byName[toGroup]
	if from == nil || to == nil || inv.hosts[hostName] == nil {
		return false
	}
	from.Hosts = slices.DeleteFunc(from.Hosts, func(n string) bool { return n == hostName })
	if !slices.Contains(to.Hosts, hostName) {
		to.Hosts = append(to.Hosts, hostName)
	}
	return true
}

// SetGroupVar sets a variable on a group.
func (inv *Inventory) SetGroupVar(groupName, key, value string) bool {
	g := inv.byName[groupName]
	if g == nil {
		return false
	}
	g.Vars[key] = value
	return true
}

// DeleteGroupVar removes a variable from a group.
func (inv *Inventory) DeleteGroupVar(groupName, key string) bool {
	g := inv.byName[groupName]
	if g == nil {
		return false
	}
	delete(g.Vars, key)
	return true
}

// SetHostVar sets a variable on a host.
func (inv *Inventory) SetHostVar(hostName, key, value string) bool {
	h := inv.hosts[hostName]
	if h == nil {
		return false
	}
	h.Vars[key] = value
	return true
}

// DeleteHostVar removes a variable from a host.
func (inv *Inventory) DeleteHostVar(hostName, key string) bool {
	h := inv.hosts[hostName]
	if h == nil {
		return false
	}
	delete(h.Vars, key)
	return true
}

// ReparentGroup moves a group to a new parent (or to top-level if newParent=="").
// Returns false if the group or newParent is not found, or if it would create a cycle.
func (inv *Inventory) ReparentGroup(name, newParent string) bool {
	g := inv.byName[name]
	if g == nil || name == "all" {
		return false
	}
	// prevent cycle: newParent must not be a descendant of name
	if newParent != "" && inv.isDescendant(name, newParent) {
		return false
	}

	// remove from old parent's Children list
	if g.Parent != "" {
		old := inv.byName[g.Parent]
		if old != nil {
			old.Children = slices.DeleteFunc(old.Children, func(c string) bool { return c == name })
		}
	}

	// attach to new parent
	if newParent == "" || newParent == "all" {
		g.Parent = ""
	} else {
		newP := inv.byName[newParent]
		if newP == nil {
			return false
		}
		g.Parent = newParent
		if !slices.Contains(newP.Children, name) {
			newP.Children = append(newP.Children, name)
		}
	}
	return true
}

// IsDescendant returns true if candidate is a descendant of ancestor.
func (inv *Inventory) IsDescendant(ancestor, candidate string) bool {
	return inv.isDescendant(ancestor, candidate)
}

func (inv *Inventory) isDescendant(ancestor, candidate string) bool {
	g := inv.byName[candidate]
	for g != nil && g.Parent != "" {
		if g.Parent == ancestor {
			return true
		}
		g = inv.byName[g.Parent]
	}
	return false
}

// CopyHostToGroup adds hostName to targetGroup without removing it from its current group.
func (inv *Inventory) CopyHostToGroup(hostName, targetGroup string) bool {
	return inv.AddHost(hostName, targetGroup)
}

// UniqueGroupName returns base if it isn't already in use, otherwise base
// suffixed with "-copy", "-copy2", "-copy3", ... until a free name is found.
func (inv *Inventory) UniqueGroupName(base string) string {
	if inv.byName[base] == nil {
		return base
	}
	if inv.byName[base+"-copy"] == nil {
		return base + "-copy"
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-copy%d", base, i)
		if inv.byName[candidate] == nil {
			return candidate
		}
	}
}

// CopyGroupDeep deep-copies srcName, including its descendant groups, hosts
// membership and vars, as a new subtree attached under targetParent
// ("" or "all" for top-level). The copied root is named newName; descendant
// groups keep their original names unless that name is already taken, in
// which case UniqueGroupName resolves the collision. Hosts are not
// duplicated — the cloned groups reference the same host names. Returns the
// new root group's name, or "" if srcName/targetParent is invalid or newName
// is already in use.
func (inv *Inventory) CopyGroupDeep(srcName, targetParent, newName string) string {
	src := inv.byName[srcName]
	if src == nil || srcName == "all" {
		return ""
	}
	if newName == "" || inv.byName[newName] != nil {
		return ""
	}
	if targetParent != "" && targetParent != "all" && inv.byName[targetParent] == nil {
		return ""
	}

	var clone func(orig *Group, parent, name string) string
	clone = func(orig *Group, parent, name string) string {
		// snapshot before AddGroupUnder mutates orig.Children (relevant when
		// parent == orig.Name, i.e. copying a group under itself).
		children := append([]string(nil), orig.Children...)
		hosts := append([]string(nil), orig.Hosts...)
		vars := make(map[string]string, len(orig.Vars))
		for k, v := range orig.Vars {
			vars[k] = v
		}

		if !inv.AddGroupUnder(parent, name) {
			return ""
		}
		ng := inv.byName[name]
		ng.Vars = vars
		ng.Hosts = hosts

		for _, childName := range children {
			child := inv.byName[childName]
			if child == nil {
				continue
			}
			clone(child, name, inv.UniqueGroupName(child.Name))
		}
		return name
	}

	return clone(src, targetParent, newName)
}

// GroupsForHost returns all group names that contain the given host (excluding "all").
func (inv *Inventory) GroupsForHost(hostName string) []string {
	var out []string
	for _, g := range inv.groups {
		if g.Name == "all" {
			continue
		}
		if slices.Contains(g.Hosts, hostName) {
			out = append(out, g.Name)
		}
	}
	return out
}

// MergeFrom merges all groups and hosts from other into inv, matching by
// name. For a group/host that already exists in inv, missing vars are
// filled in (inv's existing values always win on conflict) and host
// membership / group-vars are unioned; the existing group's parent in inv
// is left untouched even if other places it elsewhere. For a group/host
// that doesn't exist yet, it's created using other's parent (resolved by
// name, since other.groups is parent-before-child ordered). Returns the
// number of newly created groups and hosts.
func (inv *Inventory) MergeFrom(other *Inventory) (newGroups, newHosts int) {
	for _, g := range other.groups {
		if g.Name == "all" {
			mergeVars(inv.byName["all"].Vars, g.Vars)
			continue
		}
		existing := inv.byName[g.Name]
		if existing == nil {
			if !inv.AddGroupUnder(g.Parent, g.Name) {
				continue // parent missing/invalid; skip this group
			}
			existing = inv.byName[g.Name]
			newGroups++
		}
		mergeVars(existing.Vars, g.Vars)
		for _, h := range g.Hosts {
			if !slices.Contains(existing.Hosts, h) {
				existing.Hosts = append(existing.Hosts, h)
			}
		}
	}

	for name, h := range other.hosts {
		existing := inv.hosts[name]
		if existing == nil {
			vars := make(map[string]string, len(h.Vars))
			mergeVars(vars, h.Vars)
			inv.hosts[name] = &Host{Name: name, Vars: vars}
			newHosts++
			continue
		}
		mergeVars(existing.Vars, h.Vars)
	}

	return newGroups, newHosts
}

// mergeVars copies entries from src into dst for keys dst doesn't already have.
func mergeVars(dst, src map[string]string) {
	for k, v := range src {
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}
}
