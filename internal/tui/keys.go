package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elleryq/inv-editor/internal/inventory"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// when terminal is too small, only allow quit
	if m.tooSmall {
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

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
	case "i":
		return m.startImport()
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
	case "esc":
		if m.clipboard.kind != clipNone {
			m.clipboard = clipboardState{}
			m.selectedHosts = nil
			m.statusMsg = "Clipboard cleared"
			return m, nil
		}
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
			m.groupHScroll = 0
			m.ensureGroupVisible()
			m.rebuildHosts()
		}
	case "down", "j":
		if m.groupIdx < len(m.treeNodes)-1 {
			m.groupIdx++
			m.groupHScroll = 0
			m.ensureGroupVisible()
			m.rebuildHosts()
		}
	case "shift+right":
		m.groupHScroll += hScrollStep
	case "shift+left":
		m.groupHScroll = max(0, m.groupHScroll-hScrollStep)
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
			m.groupHScroll = 0
			m.ensureGroupVisible()
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
			m.varScroll = 0
			m.rebuildVars()
			m.focus = panelVars
		}
	case "c":
		if len(m.treeNodes) > 0 && m.currentGroupName() != "all" {
			m.clipboard = clipboardState{kind: clipGroup, mode: clipModeCopy, groupName: m.currentGroupName()}
			m.statusMsg = fmt.Sprintf("Marked group %q to copy — select target group and press p", m.currentGroupName())
		}
	case "m":
		if len(m.treeNodes) > 0 && m.currentGroupName() != "all" {
			m.clipboard = clipboardState{kind: clipGroup, mode: clipModeMove, groupName: m.currentGroupName()}
			m.statusMsg = fmt.Sprintf("Marked group %q to move — select target group and press p", m.currentGroupName())
		}
	case "p":
		return m.pasteClipboard()
	}
	return m, nil
}

// --- hosts ---

func (m Model) handleHostsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.hostIdx > 0 {
			m.hostIdx--
			m.hostHScroll = 0
			m.ensureHostVisible()
		}
	case "down", "j":
		if m.hostIdx < len(m.hostNames)-1 {
			m.hostIdx++
			m.hostHScroll = 0
			m.ensureHostVisible()
		}
	case "shift+right":
		m.hostHScroll += hScrollStep
	case "shift+left":
		m.hostHScroll = max(0, m.hostHScroll-hScrollStep)
	case "n":
		m.startInput(inputNewHost, "New Host", []inputField{
			{label: "Hostname", placeholder: "e.g. web01"},
			{label: "IP (ansible_host)", placeholder: "optional, e.g. 192.168.1.10"},
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
			m.varScroll = 0
			m.rebuildVars()
			m.focus = panelVars
		}
	case " ":
		if len(m.hostNames) > 0 {
			name := m.currentHostName()
			if m.selectedHosts == nil {
				m.selectedHosts = make(map[string]bool)
			}
			if m.selectedHosts[name] {
				delete(m.selectedHosts, name)
			} else {
				m.selectedHosts[name] = true
			}
		}
	case "c":
		if len(m.hostNames) > 0 {
			hosts := m.markedHosts()
			m.clipboard = clipboardState{kind: clipHosts, mode: clipModeCopy, hostNames: hosts, sourceGroup: m.currentGroupName()}
			m.selectedHosts = nil
			m.statusMsg = fmt.Sprintf("Marked %d host(s) to copy — select target group and press p", len(hosts))
		}
	case "m":
		if len(m.hostNames) > 0 {
			hosts := m.markedHosts()
			m.clipboard = clipboardState{kind: clipHosts, mode: clipModeMove, hostNames: hosts, sourceGroup: m.currentGroupName()}
			m.selectedHosts = nil
			m.statusMsg = fmt.Sprintf("Marked %d host(s) to move — select target group and press p", len(hosts))
		}
	}
	return m, nil
}

// markedHosts returns the multi-selected hosts in display order, or just the
// host under the cursor if nothing is multi-selected.
func (m Model) markedHosts() []string {
	if len(m.selectedHosts) == 0 {
		return []string{m.currentHostName()}
	}
	var out []string
	for _, n := range m.hostNames {
		if m.selectedHosts[n] {
			out = append(out, n)
		}
	}
	return out
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
			m.varHScroll = 0
			m.ensureVarVisible()
		}
	case "down", "j":
		if m.varIdx < len(m.varKeys)-1 {
			m.varIdx++
			m.varHScroll = 0
			m.ensureVarVisible()
		}
	case "shift+right":
		m.varHScroll += hScrollStep
	case "shift+left":
		m.varHScroll = max(0, m.varHScroll-hScrollStep)
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
				m.ensureGroupVisible()
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
				m.ensureGroupVisible()
			}
		}
	case inputNewHost:
		hostName := strings.TrimSpace(m.inputFields[0].value)
		ip := strings.TrimSpace(m.inputFields[1].value)
		groupName := m.currentGroupName()
		if hostName != "" && groupName != "" {
			m.inv.AddHost(hostName, groupName)
			if ip != "" {
				m.inv.SetHostVar(hostName, "ansible_host", ip)
			}
			m.modified = true
			m.rebuildHosts()
			for i, n := range m.hostNames {
				if n == hostName {
					m.hostIdx = i
					break
				}
			}
			m.ensureHostVisible()
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
			m.ensureVarVisible()
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
			m.ensureVarVisible()
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
	case inputCopyGroupName:
		newName := strings.TrimSpace(m.inputFields[0].value)
		if newName != "" {
			src := m.clipboard.groupName
			target := m.pasteTargetGroup
			if got := m.inv.CopyGroupDeep(src, target, newName); got != "" {
				m.modified = true
				m.statusMsg = fmt.Sprintf("Copied group %q to %q", src, got)
				m.clipboard = clipboardState{}
				m.rebuildGroups()
				for i, n := range m.treeNodes {
					if n.name == got {
						m.groupIdx = i
						break
					}
				}
				m.ensureGroupVisible()
			} else {
				m.statusMsg = fmt.Sprintf("Copy failed: name %q already in use", newName)
			}
		}
	case inputImportPath:
		path := strings.TrimSpace(m.inputFields[0].value)
		if path != "" {
			other, _, err := inventory.Load(path)
			if err != nil {
				m.statusMsg = fmt.Sprintf("Import failed: %v", err)
			} else {
				newGroups, newHosts := m.inv.MergeFrom(other)
				m.modified = true
				m.statusMsg = fmt.Sprintf("Imported %s: %d new group(s), %d new host(s)", path, newGroups, newHosts)
				m.rebuildGroups()
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

// --- import ---

func (m Model) startImport() (tea.Model, tea.Cmd) {
	m.startInput(inputImportPath, "Import Inventory", []inputField{
		{label: "Path", placeholder: "e.g. other-inventory.yaml"},
	})
	return m, nil
}

// --- clipboard paste ---

// pasteClipboard applies the pending clipboard (marked hosts or a marked
// group) to the currently selected group in the Groups panel.
func (m Model) pasteClipboard() (tea.Model, tea.Cmd) {
	if m.clipboard.kind == clipNone {
		return m, nil
	}
	target := m.currentGroupName()
	if target == "" {
		return m, nil
	}

	switch m.clipboard.kind {
	case clipHosts:
		count := 0
		switch m.clipboard.mode {
		case clipModeCopy:
			for _, h := range m.clipboard.hostNames {
				if m.inv.CopyHostToGroup(h, target) {
					count++
				}
			}
			m.statusMsg = fmt.Sprintf("Copied %d host(s) to %q", count, target)
		case clipModeMove:
			for _, h := range m.clipboard.hostNames {
				if m.inv.MoveHost(h, m.clipboard.sourceGroup, target) {
					count++
				}
			}
			m.statusMsg = fmt.Sprintf("Moved %d host(s) to %q", count, target)
		}
		m.modified = true
		m.clipboard = clipboardState{}
		m.rebuildHosts()

	case clipGroup:
		name := m.clipboard.groupName
		switch m.clipboard.mode {
		case clipModeMove:
			if m.inv.ReparentGroup(name, target) {
				m.modified = true
				m.statusMsg = fmt.Sprintf("Moved group %q under %q", name, target)
				m.clipboard = clipboardState{}
				m.rebuildGroups()
				for i, n := range m.treeNodes {
					if n.name == name {
						m.groupIdx = i
						break
					}
				}
				m.ensureGroupVisible()
			} else {
				m.statusMsg = fmt.Sprintf("Cannot move %q under %q (cycle?)", name, target)
			}
		case clipModeCopy:
			m.pasteTargetGroup = target
			m.startInput(inputCopyGroupName,
				fmt.Sprintf("Copy group %q to %q as:", name, target),
				[]inputField{{label: "New group name", value: m.inv.UniqueGroupName(name)}},
			)
		}
	}
	return m, nil
}

// Run starts the TUI.
func Run(inv *inventory.Inventory, filePath string, format inventory.Format) error {
	m := NewModel(inv, filePath, format)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
