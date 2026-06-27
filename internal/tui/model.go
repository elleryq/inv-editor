package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/elleryq/inv-editor/internal/inventory"
)

type panel int

const (
	panelGroups panel = iota
	panelHosts
	panelVars
)

type mode int

const (
	modeNormal mode = iota
	modeInput       // text input dialog
	modeConfirm     // yes/no confirmation dialog
	modeExport      // export dialog
	modeHelp        // help overlay
	modeMove        // move host: pick target group
)

// varContext records which entity the Variables panel is currently showing.
type varContext int

const (
	varCtxNone varContext = iota
	varCtxGroup
	varCtxHost
)

// treeNode is one row in the Groups panel tree.
type treeNode struct {
	name        string
	depth       int
	hasChildren bool
	expanded    bool
}

type Model struct {
	inv      *inventory.Inventory
	filePath string
	format   inventory.Format
	modified bool

	showHelp bool

	// panel state
	focus      panel
	groupIdx   int // index into treeNodes
	hostIdx    int
	varIdx     int
	varCtx     varContext
	varCtxName string // group or host name

	// tree state
	treeNodes      []treeNode
	expandedGroups map[string]bool

	// cached
	hostNames []string // hosts in selected group
	varKeys   []string // vars for current varCtx entity

	// terminal size
	width  int
	height int

	// input dialog
	inputMode    inputDialogMode
	inputTitle   string
	inputFields  []inputField
	inputFocused int

	// confirm dialog (generic yes/no)
	confirmMsg    string
	confirmAction func(*Model)

	// quit dialog (save / discard / cancel)
	showQuitDialog bool

	// export
	exportFmt  inventory.Format
	exportPath string

	// move host dialog
	moveHostName   string
	moveGroupNames []string
	moveGroupIdx   int

	// status message
	statusMsg string
}

type inputDialogMode int

const (
	inputNone inputDialogMode = iota
	inputNewGroup
	inputEditGroup
	inputNewHost
	inputEditHost
	inputNewVar
	inputEditVar
	inputExportPath
	inputMoveHost
	inputMoveGroup
	inputCopyHost
)

type inputField struct {
	label       string
	value       string
	cursorPos   int
	placeholder string
	// toggle fields cycle through options with Space; no free-text input
	isToggle   bool
	toggleOpts []string
	toggleIdx  int
}

func NewModel(inv *inventory.Inventory, filePath string, format inventory.Format) Model {
	m := Model{
		inv:            inv,
		filePath:       filePath,
		format:         format,
		focus:          panelGroups,
		varCtx:         varCtxNone,
		exportFmt:      format,
		exportPath:     replaceExt(filePath, format.Extension()),
		expandedGroups: make(map[string]bool),
	}
	m.rebuildGroups()
	return m
}

// buildTree performs DFS from "all" and returns a flat ordered list of rows.
func (m *Model) buildTree() []treeNode {
	var nodes []treeNode
	var visit func(name string, depth int)
	visit = func(name string, depth int) {
		g := m.inv.Group(name)
		if g == nil {
			return
		}
		expanded := m.expandedGroups[name]
		nodes = append(nodes, treeNode{
			name:        name,
			depth:       depth,
			hasChildren: len(g.Children) > 0,
			expanded:    expanded,
		})
		if expanded {
			for _, child := range g.Children {
				visit(child, depth+1)
			}
		}
	}

	// "all" at depth 0, then top-level groups at depth 1
	visit("all", 0)
	for _, g := range m.inv.TopLevelGroups() {
		visit(g.Name, 1)
	}
	return nodes
}

func (m *Model) rebuildGroups() {
	m.treeNodes = m.buildTree()
	if m.groupIdx >= len(m.treeNodes) {
		m.groupIdx = max(0, len(m.treeNodes)-1)
	}
	m.rebuildHosts()
}

func (m *Model) rebuildHosts() {
	if len(m.treeNodes) == 0 {
		m.hostNames = nil
		m.hostIdx = 0
		return
	}
	hosts := m.inv.HostsInGroup(m.currentGroupName())
	m.hostNames = make([]string, len(hosts))
	for i, h := range hosts {
		m.hostNames[i] = h.Name
	}
	if m.hostIdx >= len(m.hostNames) {
		m.hostIdx = max(0, len(m.hostNames)-1)
	}
}

func (m *Model) rebuildVars() {
	switch m.varCtx {
	case varCtxGroup:
		g := m.inv.Group(m.varCtxName)
		if g == nil {
			m.varKeys = nil
			return
		}
		m.varKeys = sortedMapKeys(g.Vars)
	case varCtxHost:
		h := m.inv.Host(m.varCtxName)
		if h == nil {
			m.varKeys = nil
			return
		}
		m.varKeys = sortedMapKeys(h.Vars)
	default:
		m.varKeys = nil
	}
	if m.varIdx >= len(m.varKeys) {
		m.varIdx = max(0, len(m.varKeys)-1)
	}
}

func (m *Model) currentGroupName() string {
	if len(m.treeNodes) == 0 {
		return ""
	}
	return m.treeNodes[m.groupIdx].name
}

func (m *Model) currentHostName() string {
	if len(m.hostNames) == 0 {
		return ""
	}
	return m.hostNames[m.hostIdx]
}

func (m *Model) currentVarKey() string {
	if len(m.varKeys) == 0 {
		return ""
	}
	return m.varKeys[m.varIdx]
}

func (m *Model) currentVarValue() string {
	key := m.currentVarKey()
	if key == "" {
		return ""
	}
	switch m.varCtx {
	case varCtxGroup:
		g := m.inv.Group(m.varCtxName)
		if g != nil {
			return g.Vars[key]
		}
	case varCtxHost:
		h := m.inv.Host(m.varCtxName)
		if h != nil {
			return h.Vars[key]
		}
	}
	return ""
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	main := m.viewMain()

	switch {
	case m.showQuitDialog:
		return overlayCenter(main, m.viewQuitDialog(), m.width, m.height)
	case m.showHelp:
		return overlayCenter(main, m.viewHelp(), m.width, m.height)
	case m.inputMode != inputNone:
		return overlayCenter(main, m.viewInputDialog(), m.width, m.height)
	case m.confirmMsg != "":
		return overlayCenter(main, m.viewConfirmDialog(), m.width, m.height)
	}
	return main
}

func (m Model) viewMain() string {
	header := m.viewHeader()
	statusBar := m.viewStatusBar()

	// reserve rows for header(1) + border(2) and statusbar(1) + border(2) for panels
	availH := m.height - lipgloss.Height(header) - lipgloss.Height(statusBar)

	topH := availH * 2 / 3
	botH := availH - topH

	leftW := m.width / 3
	rightW := m.width - leftW

	groupsPanel := m.viewGroupsPanel(leftW-2, topH-2)
	hostsPanel := m.viewHostsPanel(rightW-2, topH-2)
	varsPanel := m.viewVarsPanel(m.width-2, botH-2)

	top := lipgloss.JoinHorizontal(lipgloss.Top, groupsPanel, hostsPanel)
	body := lipgloss.JoinVertical(lipgloss.Left, top, varsPanel)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, statusBar)
}

func (m Model) viewHeader() string {
	title := fmt.Sprintf("inv-editor: %s", m.filePath)
	if m.modified {
		title += styleModified.Render("  [modified]")
	}
	help := styleDim.Render("Press ? for help")
	gap := strings.Repeat(" ", max(0, m.width-lipgloss.Width(title)-lipgloss.Width(help)-2))
	return styleHeader.Width(m.width).Render(title + gap + help)
}

func (m Model) viewStatusBar() string {
	if m.statusMsg != "" {
		return styleStatusBar.Render(m.statusMsg)
	}
	keys := []struct{ k, v string }{
		{"Tab", "next panel"},
		{"n", "new"},
		{"e", "edit"},
		{"d", "del"},
		{"v", "vars"},
		{"s", "save"},
		{"x", "export"},
		{"q", "quit"},
	}
	var parts []string
	for _, kv := range keys {
		parts = append(parts, styleKey.Render(kv.k)+" "+styleKeyLabel.Render(kv.v))
	}
	return styleStatusBar.Render(strings.Join(parts, "  "))
}

func (m Model) viewGroupsPanel(w, h int) string {
	active := m.focus == panelGroups
	title := panelTitle("GROUPS", "G", active)

	lines := make([]string, 0, len(m.treeNodes)+1)
	for i, node := range m.treeNodes {
		indent := strings.Repeat("  ", node.depth)
		var icon string
		switch {
		case node.hasChildren && node.expanded:
			icon = "▼ "
		case node.hasChildren:
			icon = "▶ "
		default:
			icon = "  "
		}
		label := indent + icon + node.name
		if i == m.groupIdx {
			if active {
				lines = append(lines, styleSelected.Render(">"+label))
			} else {
				lines = append(lines, " "+styleDim.Render(label))
			}
		} else {
			lines = append(lines, " "+label)
		}
	}
	lines = append(lines, styleAdd.Render("  [+ New Group]"))

	return renderPanel(title, lines, w, h, active)
}

func (m Model) viewHostsPanel(w, h int) string {
	active := m.focus == panelHosts
	groupName := m.currentGroupName()
	title := panelTitle(fmt.Sprintf("HOSTS (%s)", groupName), "H", active)

	lines := make([]string, 0, len(m.hostNames)+1)
	for i, name := range m.hostNames {
		line := name
		if i == m.hostIdx {
			if active {
				line = styleSelected.Render("> " + name)
			} else {
				line = "  " + styleDim.Render(name)
			}
		} else {
			line = "  " + name
		}
		lines = append(lines, line)
	}
	lines = append(lines, styleAdd.Render("  [+ New Host]"))

	return renderPanel(title, lines, w, h, active)
}

func (m Model) viewVarsPanel(w, h int) string {
	active := m.focus == panelVars

	var ctxLabel string
	switch m.varCtx {
	case varCtxGroup:
		ctxLabel = fmt.Sprintf("Group: %s", m.varCtxName)
	case varCtxHost:
		ctxLabel = fmt.Sprintf("Host: %s", m.varCtxName)
	default:
		ctxLabel = "press v on a group or host"
	}
	title := panelTitle(fmt.Sprintf("VARIABLES (%s)", ctxLabel), "V", active)

	lines := make([]string, 0, len(m.varKeys)+1)
	for i, k := range m.varKeys {
		var val string
		switch m.varCtx {
		case varCtxGroup:
			if g := m.inv.Group(m.varCtxName); g != nil {
				val = g.Vars[k]
			}
		case varCtxHost:
			if h := m.inv.Host(m.varCtxName); h != nil {
				val = h.Vars[k]
			}
		}
		entry := fmt.Sprintf("%s = %s", k, val)
		if i == m.varIdx {
			if active {
				entry = styleSelected.Render("> " + entry)
			} else {
				entry = "  " + styleDim.Render(entry)
			}
		} else {
			entry = "  " + entry
		}
		lines = append(lines, entry)
	}
	if m.varCtx != varCtxNone {
		lines = append(lines, styleAdd.Render("  [+ New Variable]"))
	}

	return renderPanel(title, lines, w, h, active)
}

// --- dialog views ---

func (m Model) viewInputDialog() string {
	var sb strings.Builder
	sb.WriteString(styleTitle.Render(m.inputTitle) + "\n\n")

	for i, f := range m.inputFields {
		label := f.label + ":"
		active := i == m.inputFocused
		if f.isToggle {
			val := f.toggleOpts[f.toggleIdx]
			left := styleDim.Render("◀")
			right := styleDim.Render("▶")
			if active {
				sb.WriteString(fmt.Sprintf("%s %s %s %s\n",
					styleSelected.Render(label),
					left, styleSelected.Render(" "+val+" "), right))
			} else {
				sb.WriteString(fmt.Sprintf("%s %s %s %s\n",
					label, left, styleDim.Render(" "+val+" "), right))
			}
		} else {
			val := f.value
			if active {
				val = val + "█"
				sb.WriteString(fmt.Sprintf("%s [%s]\n", styleSelected.Render(label), val))
			} else {
				sb.WriteString(fmt.Sprintf("%s [%s]\n", label, styleDim.Render(val)))
			}
		}
	}

	hint := "\n" + styleDim.Render("Tab") + " next field  " +
		styleDim.Render("Enter") + " confirm  " +
		styleDim.Render("Esc") + " cancel"
	if hasToggle(m.inputFields) {
		hint += "  " + styleDim.Render("Space/←/→") + " toggle"
	}
	sb.WriteString(hint)
	return dialogBox(sb.String(), 54)
}

func hasToggle(fields []inputField) bool {
	for _, f := range fields {
		if f.isToggle {
			return true
		}
	}
	return false
}

func (m Model) viewQuitDialog() string {
	var sb strings.Builder
	sb.WriteString(styleTitle.Render("Unsaved Changes") + "\n\n")
	sb.WriteString("You have unsaved changes to:\n")
	sb.WriteString(styleDim.Render("  "+m.filePath) + "\n\n")
	sb.WriteString(styleSelected.Render("[s]") + " Save & Quit\n")
	sb.WriteString(styleDanger.Render("[d]") + " Discard & Quit\n")
	sb.WriteString(styleDim.Render("[Esc]") + " Cancel\n")
	return dialogBox(sb.String(), 46)
}

func (m Model) viewConfirmDialog() string {
	var sb strings.Builder
	sb.WriteString(m.confirmMsg + "\n\n")
	sb.WriteString(styleDanger.Render("Enter") + " confirm  " + styleDim.Render("Esc") + " cancel")
	return dialogBox(sb.String(), 50)
}

func (m Model) viewHelp() string {
	help := `NAVIGATION
  Tab / Shift+Tab   cycle panels
  G / H / V         jump to panel
  ↑ ↓  or  j k      move cursor

GROUPS PANEL
  → / l             expand subgroups
  ← / h             collapse / jump to parent
  n                 new group (choose parent in dialog)
  e / Enter         rename group
  M                 move group (reparent)
  d / Delete        delete group (children re-parented)
  v                 open group variables

HOSTS PANEL
  n                 new host
  e / Enter         rename host
  m                 move host to another group (removes here)
  c                 copy host to another group (keeps here)
  d / Delete        remove host from this group
  v                 open host variables

FILE
  s  save   x  export   q  quit

Press any key to close`
	return dialogBox(help, 52)
}

// --- helpers ---

func panelTitle(label, key string, active bool) string {
	k := styleDim.Render("[" + key + "]")
	if active {
		return styleTitleActive.Render(label) + " " + k
	}
	return styleTitle.Render(label) + " " + k
}

func renderPanel(title string, lines []string, w, h int, active bool) string {
	inner := title + "\n" + strings.Repeat("─", w) + "\n"
	for _, l := range lines {
		inner += l + "\n"
	}
	// pad to height
	used := strings.Count(inner, "\n")
	for used < h {
		inner += "\n"
		used++
	}
	s := stylePanel
	if active {
		s = stylePanelActive
	}
	return s.Width(w).Height(h).Render(inner)
}

func dialogBox(content string, w int) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorActiveBorder).
		Padding(1, 2).
		Width(w).
		Render(content)
}

func overlayCenter(bg, overlay string, totalW, totalH int) string {
	ow := lipgloss.Width(overlay)
	oh := lipgloss.Height(overlay)
	col := max(0, (totalW-ow)/2)
	row := max(0, (totalH-oh)/2)

	bgLines := strings.Split(bg, "\n")
	ovLines := strings.Split(overlay, "\n")

	for r, ovLine := range ovLines {
		bgRow := row + r
		if bgRow >= len(bgLines) {
			break
		}
		bg := bgLines[bgRow]
		// pad background line to at least col+ow
		bgRunes := []rune(stripANSI(bg))
		for len(bgRunes) < col+ow {
			bgRunes = append(bgRunes, ' ')
		}
		bgLines[bgRow] = string(bgRunes[:col]) + ovLine + string(bgRunes[col+ow:])
		_ = col
	}
	return strings.Join(bgLines, "\n")
}

// stripANSI is a minimal ANSI escape stripper for width calculation.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
		} else {
			out.WriteByte(s[i])
			i++
		}
	}
	return out.String()
}

func replaceExt(path, ext string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[:i] + ext
		}
		if path[i] == '/' {
			break
		}
	}
	return path + ext
}

func sortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
