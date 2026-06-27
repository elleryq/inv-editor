package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elleryq/inv-editor/internal/inventory"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// quit dialog takes priority
	if m.showQuitDialog {
		return m.handleQuitDialogKey(msg)
	}
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}
	if m.inputMode != inputNone {
		return m.handleInputKey(msg)
	}
	if m.confirmMsg != "" {
		return m.handleConfirmKey(msg)
	}

	key := msg.String()

	switch key {
	case "?":
		m.showHelp = true
		return m, nil
	case "q", "ctrl+c":
		if m.modified {
			m.showQuitDialog = true
			return m, nil
		}
		return m, tea.Quit
	case "s":
		return m.doSave()
	case "x":
		return m.startExport()
	case "tab":
		m.focus = (m.focus + 1) % 3
		return m, nil
	case "shift+tab":
		m.focus = (m.focus + 2) % 3
		return m, nil
	case "G":
		m.focus = panelGroups
		return m, nil
	case "H":
		m.focus = panelHosts
		return m, nil
	case "V":
		m.focus = panelVars
		return m, nil
	}

	switch m.focus {
	case panelGroups:
		return m.handleGroupsKey(key)
	case panelHosts:
		return m.handleHostsKey(key)
	case panelVars:
		return m.handleVarsKey(key)
	}
	return m, nil
}

// --- groups ---

func (m Model) handleGroupsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.groupIdx > 0 {
			m.groupIdx--
			m.rebuildHosts()
		}
	case "down", "j":
		if m.groupIdx < len(m.treeNodes)-1 {
			m.groupIdx++
			m.rebuildHosts()
		}
	case "right", "l":
		// expand current node if it has children
		node := m.treeNodes[m.groupIdx]
		if node.hasChildren && !node.expanded {
			m.expandedGroups[node.name] = true
			m.rebuildGroups()
		}
	case "left", "h":
		node := m.treeNodes[m.groupIdx]
		if node.hasChildren && node.expanded {
			// collapse
			m.expandedGroups[node.name] = false
			m.rebuildGroups()
		} else if node.depth > 1 {
			// jump to parent
			grp := m.inv.Group(node.name)
			if grp == nil {
				break
			}
			parentName := grp.Parent
			for i, n := range m.treeNodes {
				if n.name == parentName {
					m.groupIdx = i
					break
				}
			}
			m.rebuildHosts()
		}
	case "n":
		currentName := m.currentGroupName()
		// build parent toggle options: top-level or under current group
		parentOpts := []string{"(top level)"}
		if currentName != "all" {
			parentOpts = append(parentOpts, currentName)
		}
		m.startInput(inputNewGroup, "New Group", []inputField{
			{label: "Group name", placeholder: "e.g. frontend"},
			{
				label:      "Parent",
				isToggle:   true,
				toggleOpts: parentOpts,
				toggleIdx:  0,
				value:      parentOpts[0],
			},
		})
	case "e", "enter":
		if len(m.treeNodes) > 0 && m.currentGroupName() != "all" {
			m.startInput(inputEditGroup, "Rename Group", []inputField{
				{label: "Group name", value: m.currentGroupName()},
			})
		}
	case "d", "delete":
		if len(m.treeNodes) > 0 && m.currentGroupName() != "all" {
			name := m.currentGroupName()
			g := m.inv.Group(name)
			if g == nil {
				break
			}
			var extra string
			if len(g.Children) > 0 {
				parentLabel := g.Parent
				if parentLabel == "" {
					parentLabel = "top level"
				}
				extra = fmt.Sprintf(" Child groups will be re-parented to %q.", parentLabel)
			}
			m.confirmMsg = fmt.Sprintf("Delete group %q?%s", name, extra)
			m.confirmAction = func(mdl *Model) {
				mdl.inv.DeleteGroup(name)
				mdl.modified = true
				mdl.rebuildGroups()
			}
		}
	case "v":
		if len(m.treeNodes) > 0 {
			m.varCtx = varCtxGroup
			m.varCtxName = m.currentGroupName()
			m.varIdx = 0
			m.rebuildVars()
			m.focus = panelVars
		}
	case "M":
		if len(m.treeNodes) > 0 && m.currentGroupName() != "all" {
			m.startMoveGroup()
		}
	}
	return m, nil
}

// --- hosts ---

func (m Model) handleHostsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.hostIdx > 0 {
			m.hostIdx--
		}
	case "down", "j":
		if m.hostIdx < len(m.hostNames)-1 {
			m.hostIdx++
		}
	case "n":
		m.startInput(inputNewHost, "New Host", []inputField{
			{label: "Host address", placeholder: "e.g. 192.168.1.10 or web01.example.com"},
		})
	case "e", "enter":
		if len(m.hostNames) > 0 {
			m.startInput(inputEditHost, "Edit Host", []inputField{
				{label: "Host address", value: m.currentHostName()},
			})
		}
	case "d", "delete":
		if len(m.hostNames) > 0 {
			name := m.currentHostName()
			group := m.currentGroupName()
			m.confirmMsg = fmt.Sprintf("Remove host %q from group %q?", name, group)
			m.confirmAction = func(mdl *Model) {
				mdl.inv.DeleteHost(name, group, false)
				mdl.modified = true
				mdl.rebuildHosts()
			}
		}
	case "v":
		if len(m.hostNames) > 0 {
			m.varCtx = varCtxHost
			m.varCtxName = m.currentHostName()
			m.varIdx = 0
			m.rebuildVars()
			m.focus = panelVars
		}
	case "m":
		if len(m.hostNames) > 0 {
			m.startMoveHost()
		}
	case "c":
		if len(m.hostNames) > 0 {
			m.startCopyHost()
		}
	}
	return m, nil
}

// --- vars ---

func (m Model) handleVarsKey(key string) (tea.Model, tea.Cmd) {
	if m.varCtx == varCtxNone {
		return m, nil
	}
	switch key {
	case "up", "k":
		if m.varIdx > 0 {
			m.varIdx--
		}
	case "down", "j":
		if m.varIdx < len(m.varKeys)-1 {
			m.varIdx++
		}
	case "n":
		m.startInput(inputNewVar, "New Variable", []inputField{
			{label: "Key"},
			{label: "Value"},
		})
	case "e", "enter":
		if len(m.varKeys) > 0 {
			k := m.currentVarKey()
			v := m.currentVarValue()
			m.startInput(inputEditVar, "Edit Variable", []inputField{
				{label: "Key", value: k},
				{label: "Value", value: v},
			})
		}
	case "d", "delete":
		if len(m.varKeys) > 0 {
			k := m.currentVarKey()
			ctx := m.varCtx
			ctxName := m.varCtxName
			m.confirmMsg = fmt.Sprintf("Delete variable %q?", k)
			m.confirmAction = func(mdl *Model) {
				switch ctx {
				case varCtxGroup:
					mdl.inv.DeleteGroupVar(ctxName, k)
				case varCtxHost:
					mdl.inv.DeleteHostVar(ctxName, k)
				}
				mdl.modified = true
				mdl.rebuildVars()
			}
		}
	}
	return m, nil
}

// --- input dialog ---

func (m *Model) startInput(mode inputDialogMode, title string, fields []inputField) {
	m.inputMode = mode
	m.inputTitle = title
	m.inputFields = fields
	m.inputFocused = 0
	// set cursor at end of existing value
	for i := range m.inputFields {
		m.inputFields[i].cursorPos = len(m.inputFields[i].value)
	}
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	f := &m.inputFields[m.inputFocused]

	// toggle field navigation
	if f.isToggle {
		switch key {
		case "esc":
			m.inputMode = inputNone
			return m, nil
		case "tab", "down":
			m.inputFocused = (m.inputFocused + 1) % len(m.inputFields)
			return m, nil
		case "shift+tab", "up":
			m.inputFocused = (m.inputFocused + len(m.inputFields) - 1) % len(m.inputFields)
			return m, nil
		case " ", "right", "l":
			f.toggleIdx = (f.toggleIdx + 1) % len(f.toggleOpts)
			f.value = f.toggleOpts[f.toggleIdx]
			// sync exportFmt when format toggle changes
			if m.inputMode == inputExportPath {
				m.exportFmt = inventory.DetectFormat("." + strings.ToLower(f.value))
			}
			return m, nil
		case "left", "h":
			f.toggleIdx = (f.toggleIdx + len(f.toggleOpts) - 1) % len(f.toggleOpts)
			f.value = f.toggleOpts[f.toggleIdx]
			if m.inputMode == inputExportPath {
				m.exportFmt = inventory.DetectFormat("." + strings.ToLower(f.value))
			}
			return m, nil
		case "enter":
			if m.inputFocused < len(m.inputFields)-1 {
				m.inputFocused++
				return m, nil
			}
			return m.commitInput()
		}
		return m, nil
	}

	// text field
	switch key {
	case "esc":
		m.inputMode = inputNone
		return m, nil
	case "tab", "down":
		m.inputFocused = (m.inputFocused + 1) % len(m.inputFields)
		return m, nil
	case "shift+tab", "up":
		m.inputFocused = (m.inputFocused + len(m.inputFields) - 1) % len(m.inputFields)
		return m, nil
	case "enter":
		if m.inputFocused < len(m.inputFields)-1 {
			m.inputFocused++
			return m, nil
		}
		return m.commitInput()
	case "backspace":
		if len(f.value) > 0 {
			f.value = f.value[:len(f.value)-1]
		}
		return m, nil
	default:
		if len(msg.Runes) == 1 {
			f.value += string(msg.Runes)
		}
	}
	return m, nil
}

func (m Model) commitInput() (tea.Model, tea.Cmd) {
	switch m.inputMode {
	case inputNewGroup:
		name := strings.TrimSpace(m.inputFields[0].value)
		parentToggle := m.inputFields[1].value // "(top level)" or group name
		if name != "" {
			var ok bool
			if parentToggle == "(top level)" || parentToggle == "" {
				ok = m.inv.AddGroup(name)
			} else {
				ok = m.inv.AddGroupUnder(parentToggle, name)
				m.expandedGroups[parentToggle] = true // auto-expand parent
			}
			if ok {
				m.modified = true
				m.rebuildGroups()
				for i, n := range m.treeNodes {
					if n.name == name {
						m.groupIdx = i
						break
					}
				}
				m.rebuildHosts()
			}
		}
	case inputEditGroup:
		newName := strings.TrimSpace(m.inputFields[0].value)
		if newName != "" && newName != m.currentGroupName() {
			old := m.currentGroupName()
			if m.inv.RenameGroup(old, newName) {
				// carry over expanded state
				if m.expandedGroups[old] {
					m.expandedGroups[newName] = true
					delete(m.expandedGroups, old)
				}
				m.modified = true
				m.rebuildGroups()
				for i, n := range m.treeNodes {
					if n.name == newName {
						m.groupIdx = i
						break
					}
				}
			}
		}
	case inputNewHost:
		hostName := strings.TrimSpace(m.inputFields[0].value)
		groupName := m.currentGroupName()
		if hostName != "" && groupName != "" {
			m.inv.AddHost(hostName, groupName)
			m.modified = true
			m.rebuildHosts()
			for i, n := range m.hostNames {
				if n == hostName {
					m.hostIdx = i
					break
				}
			}
		}
	case inputEditHost:
		newName := strings.TrimSpace(m.inputFields[0].value)
		if newName != "" {
			m.inv.RenameHost(m.currentHostName(), newName)
			m.modified = true
			m.rebuildHosts()
		}
	case inputNewVar:
		k := strings.TrimSpace(m.inputFields[0].value)
		v := strings.TrimSpace(m.inputFields[1].value)
		if k != "" {
			switch m.varCtx {
			case varCtxGroup:
				m.inv.SetGroupVar(m.varCtxName, k, v)
			case varCtxHost:
				m.inv.SetHostVar(m.varCtxName, k, v)
			}
			m.modified = true
			m.rebuildVars()
		}
	case inputEditVar:
		oldKey := m.currentVarKey()
		newKey := strings.TrimSpace(m.inputFields[0].value)
		newVal := strings.TrimSpace(m.inputFields[1].value)
		if newKey != "" {
			switch m.varCtx {
			case varCtxGroup:
				if oldKey != newKey {
					m.inv.DeleteGroupVar(m.varCtxName, oldKey)
				}
				m.inv.SetGroupVar(m.varCtxName, newKey, newVal)
			case varCtxHost:
				if oldKey != newKey {
					m.inv.DeleteHostVar(m.varCtxName, oldKey)
				}
				m.inv.SetHostVar(m.varCtxName, newKey, newVal)
			}
			m.modified = true
			m.rebuildVars()
		}
	case inputExportPath:
		// field 0 = format toggle, field 1 = path
		fmtField := m.inputFields[0]
		path := strings.TrimSpace(m.inputFields[1].value)
		var exportFmt inventory.Format
		if strings.ToLower(fmtField.value) == "yaml" {
			exportFmt = inventory.FormatYAML
		} else {
			exportFmt = inventory.FormatINI
		}
		if path != "" {
			if err := inventory.Save(m.inv, path, exportFmt); err != nil {
				m.statusMsg = fmt.Sprintf("Export failed: %v", err)
			} else {
				m.statusMsg = fmt.Sprintf("Exported %s to %s", exportFmt.String(), path)
				m.exportPath = path
				m.exportFmt = exportFmt
			}
		}
	case inputMoveHost:
		target := strings.TrimSpace(m.inputFields[0].value)
		from := m.currentGroupName()
		if target != "" && target != from {
			if m.inv.MoveHost(m.moveHostName, from, target) {
				m.modified = true
				m.rebuildHosts()
			} else {
				m.statusMsg = fmt.Sprintf("Group %q not found", target)
			}
		}
	case inputMoveGroup:
		rawTarget := strings.TrimSpace(m.inputFields[0].value)
		name := m.currentGroupName()
		newParent := rawTarget
		if rawTarget == "(top level)" {
			newParent = ""
		}
		if m.inv.ReparentGroup(name, newParent) {
			m.modified = true
			m.rebuildGroups()
			// re-select the moved group
			for i, n := range m.treeNodes {
				if n.name == name {
					m.groupIdx = i
					break
				}
			}
		} else {
			m.statusMsg = fmt.Sprintf("Cannot move %q to %q (cycle or not found)", name, rawTarget)
		}
	case inputCopyHost:
		target := strings.TrimSpace(m.inputFields[0].value)
		hostName := m.currentHostName()
		if target != "" {
			if m.inv.CopyHostToGroup(hostName, target) {
				m.modified = true
				m.statusMsg = fmt.Sprintf("Copied %q to group %q", hostName, target)
			} else {
				m.statusMsg = fmt.Sprintf("Host already in %q or group not found", target)
			}
		}
	}
	m.inputMode = inputNone
	return m, nil
}

// --- quit dialog ---

func (m Model) handleQuitDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s":
		// save then quit
		if err := inventory.Save(m.inv, m.filePath, m.format); err != nil {
			m.showQuitDialog = false
			m.statusMsg = fmt.Sprintf("Save failed: %v", err)
			return m, nil
		}
		return m, tea.Quit
	case "d":
		// discard and quit
		return m, tea.Quit
	case "esc", "c", "q":
		m.showQuitDialog = false
	}
	return m, nil
}

// --- confirm dialog ---

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y":
		if m.confirmAction != nil {
			m.confirmAction(&m)
		}
		m.confirmMsg = ""
		m.confirmAction = nil
	case "esc", "n", "q":
		m.confirmMsg = ""
		m.confirmAction = nil
	}
	return m, nil
}

// --- save / export ---

func (m Model) doSave() (tea.Model, tea.Cmd) {
	err := inventory.Save(m.inv, m.filePath, m.format)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Save failed: %v", err)
	} else {
		m.modified = false
		m.statusMsg = fmt.Sprintf("Saved to %s", m.filePath)
	}
	return m, nil
}

func (m Model) startExport() (tea.Model, tea.Cmd) {
	fmtOpts := []string{"INI", "YAML"}
	fmtIdx := 0
	if m.exportFmt == inventory.FormatYAML {
		fmtIdx = 1
	}
	// suggest path extension matching the current format
	suggestedPath := suggestExportPath(m.exportPath, m.exportFmt)
	m.startInput(inputExportPath, "Export Inventory", []inputField{
		{
			label:      "Format",
			isToggle:   true,
			toggleOpts: fmtOpts,
			toggleIdx:  fmtIdx,
			value:      fmtOpts[fmtIdx],
		},
		{label: "Path", value: suggestedPath},
	})
	return m, nil
}

// suggestExportPath returns path with extension matching fmt.
func suggestExportPath(path string, fmt inventory.Format) string {
	switch fmt {
	case inventory.FormatYAML:
		return replaceExt(path, ".yaml")
	default:
		return replaceExt(path, ".ini")
	}
}

// --- move host ---

func (m *Model) startMoveHost() {
	m.moveHostName = m.currentHostName()
	m.moveGroupIdx = 0
	m.moveGroupNames = nil
	current := m.currentGroupName()
	for _, g := range m.inv.Groups() {
		if g.Name != current && g.Name != "all" {
			m.moveGroupNames = append(m.moveGroupNames, g.Name)
		}
	}
	if len(m.moveGroupNames) == 0 {
		m.statusMsg = "No other groups to move to"
		return
	}
	m.startInput(inputMoveHost,
		fmt.Sprintf("Move %q to group:", m.moveHostName),
		[]inputField{{label: "Target group", value: m.moveGroupNames[0]}},
	)
}

// startMoveGroup opens dialog to reparent the current group.
func (m *Model) startMoveGroup() {
	name := m.currentGroupName()
	g := m.inv.Group(name)
	if g == nil {
		return
	}
	currentParent := g.Parent
	if currentParent == "" {
		currentParent = "(top level)"
	}
	// build options: top-level + all groups except self and own descendants
	opts := []string{"(top level)"}
	for _, grp := range m.inv.Groups() {
		if grp.Name == "all" || grp.Name == name {
			continue
		}
		if m.inv.IsDescendant(name, grp.Name) {
			continue
		}
		opts = append(opts, grp.Name)
	}
	m.startInput(inputMoveGroup,
		fmt.Sprintf("Move group %q under:", name),
		[]inputField{{label: "New parent", value: currentParent}},
	)
}

// startCopyHost opens dialog to copy the current host to another group.
func (m *Model) startCopyHost() {
	name := m.currentHostName()
	current := m.currentGroupName()
	m.startInput(inputCopyHost,
		fmt.Sprintf("Copy %q to group:", name),
		[]inputField{{label: "Target group", value: ""}},
	)
	_ = current
}

// Run starts the TUI.
func Run(inv *inventory.Inventory, filePath string, format inventory.Format) error {
	m := NewModel(inv, filePath, format)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
