package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/elleryq/inv-editor/internal/inventory"
)

const (
	minWidth    = 80
	minHeight   = 22
	hScrollStep = 4
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
	width    int
	height   int
	tooSmall bool

	// vertical scroll offsets (first visible item index in each panel)
	groupScroll int
	hostScroll  int
	varScroll   int

	// horizontal scroll offsets (visual columns shifted left in each panel)
	groupHScroll int
	hostHScroll  int
	varHScroll   int

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

	// host multi-select (hosts panel)
	selectedHosts map[string]bool

	// clipboard for copy/move of hosts or a group
	clipboard clipboardState
	// target group captured when paste of a group-copy opens the rename dialog
	pasteTargetGroup string

	// status message
	statusMsg string
}

type clipKind int

const (
	clipNone clipKind = iota
	clipHosts
	clipGroup
)

type clipMode int

const (
	clipModeCopy clipMode = iota
	clipModeMove
)

type clipboardState struct {
	kind clipKind
	mode clipMode

	hostNames   []string // clipHosts
	sourceGroup string   // clipHosts: group hosts were marked from (for move)

	groupName string // clipGroup
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
	inputCopyGroupName
	inputImportPath
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
	m.ensureGroupVisible()
	m.rebuildHosts()
}

func (m *Model) rebuildHosts() {
	m.selectedHosts = nil
	if len(m.treeNodes) == 0 {
		m.hostNames = nil
		m.hostIdx = 0
		m.hostScroll = 0
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
	m.hostScroll = 0
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

// ----- viewport helpers -----

func (m *Model) groupsVisibleRows() int {
	if m.height == 0 {
		return 10
	}
	availH := m.height - 2 // header(1) + statusbar(1)
	topH := availH * 2 / 3
	// h param = topH-2; visible = h − title(1) − separator(1) − [+New](1)
	rows := (topH - 2) - 3
	if rows < 1 {
		return 1
	}
	return rows
}

func (m *Model) hostsVisibleRows() int { return m.groupsVisibleRows() }

func (m *Model) varsVisibleRows() int {
	if m.height == 0 {
		return 5
	}
	availH := m.height - 2
	topH := availH * 2 / 3
	botH := availH - topH
	rows := (botH - 2) - 3
	if rows < 1 {
		return 1
	}
	return rows
}

func (m *Model) ensureGroupVisible() {
	visible := m.groupsVisibleRows()
	if maxS := max(0, len(m.treeNodes)-visible); m.groupScroll > maxS {
		m.groupScroll = maxS
	}
	if m.groupIdx < m.groupScroll {
		m.groupScroll = m.groupIdx
	} else if m.groupIdx >= m.groupScroll+visible {
		m.groupScroll = m.groupIdx - visible + 1
	}
}

func (m *Model) ensureHostVisible() {
	visible := m.hostsVisibleRows()
	if maxS := max(0, len(m.hostNames)-visible); m.hostScroll > maxS {
		m.hostScroll = maxS
	}
	if m.hostIdx < m.hostScroll {
		m.hostScroll = m.hostIdx
	} else if m.hostIdx >= m.hostScroll+visible {
		m.hostScroll = m.hostIdx - visible + 1
	}
}

func (m *Model) ensureVarVisible() {
	visible := m.varsVisibleRows()
	if maxS := max(0, len(m.varKeys)-visible); m.varScroll > maxS {
		m.varScroll = maxS
	}
	if m.varIdx < m.varScroll {
		m.varScroll = m.varIdx
	} else if m.varIdx >= m.varScroll+visible {
		m.varScroll = m.varIdx - visible + 1
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
		m.tooSmall = msg.Width < minWidth || msg.Height < minHeight
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
	if m.tooSmall {
		return fmt.Sprintf(
			"\n  Terminal too small: %d×%d\n  Minimum required:  %d×%d\n\n  Resize the terminal, or press q to quit.",
			m.width, m.height, minWidth, minHeight,
		)
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

	// stylePanel has Border(1 each side) + Padding(0,1 each side) = 4 overhead.
	// Pass content width (w - 4) so rendered panel total equals leftW / rightW / m.width.
	groupsPanel := m.viewGroupsPanel(leftW-4, topH-2)
	hostsPanel := m.viewHostsPanel(rightW-4, topH-2)
	varsPanel := m.viewVarsPanel(m.width-4, botH-2)

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

	visible := max(1, h-3) // h − title(1) − separator(1) − [+New](1)
	scroll := m.groupScroll
	end := min(scroll+visible, len(m.treeNodes))

	lines := make([]string, 0, visible+1)
	for i := scroll; i < end; i++ {
		node := m.treeNodes[i]
		indent := strings.Repeat("  ", node.depth)
		var icon string
		switch {
		case node.hasChildren && node.expanded:
			icon = "- "
		case node.hasChildren:
			icon = "+ "
		default:
			icon = "  "
		}
		// 1 col reserved for the ">" / " " prefix
		label := panelLineView(indent+icon+node.name, m.groupHScroll, w-1)
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

	visible := max(1, h-3) // h − title(1) − separator(1) − [+New](1)
	scroll := m.hostScroll
	end := min(scroll+visible, len(m.hostNames))

	lines := make([]string, 0, visible+1)
	for i := scroll; i < end; i++ {
		box := "[ ]"
		if m.selectedHosts[m.hostNames[i]] {
			box = "[x]"
		}
		// 6 cols reserved for "> " cursor + "[ ] " checkbox prefix
		name := panelLineView(m.hostNames[i], m.hostHScroll, w-6)
		content := box + " " + name
		var line string
		if i == m.hostIdx {
			if active {
				line = styleSelected.Render("> " + content)
			} else {
				line = "  " + styleDim.Render(content)
			}
		} else {
			line = "  " + content
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

	visible := max(1, h-3) // h − title(1) − separator(1) − [+New](1)
	scroll := m.varScroll
	end := min(scroll+visible, len(m.varKeys))

	lines := make([]string, 0, visible+1)
	for i := scroll; i < end; i++ {
		k := m.varKeys[i]
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
		// 2 cols reserved for "> " / "  " prefix
		entry := panelLineView(fmt.Sprintf("%s = %s", k, val), m.varHScroll, w-2)
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
			left := styleDim.Render("<")
			right := styleDim.Render(">")
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
				val = val + "_"
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
  Tab / Shift+Tab      cycle panels
  G / H / V            jump to panel
  ↑ ↓  or  j k         move cursor
  Shift+← →            scroll panel left / right

GROUPS PANEL
  → / l                expand subgroups
  ← / h                collapse / jump to parent
  n                    new group (choose parent in dialog)
  e / Enter            rename group
  c                    mark group to copy (deep copy)
  m                    mark group to move (reparent)
  p                    paste marked group/hosts into selected group
  d / Delete           delete group (children re-parented)
  v                    open group variables

HOSTS PANEL
  n                    new host (hostname + ip)
  e / Enter            rename host
  space                toggle multi-select
  c                    mark selected host(s) to copy
  m                    mark selected host(s) to move
  d / Delete           remove host from this group
  v                    open host variables

  After c/m, switch to the Groups panel, select the target
  group, then press p to paste. Esc clears a pending clipboard.

FILE
  s  save   x  export   i  import   q  quit

IMPORT
  i                    merge another inventory file into this one
                       (matching group/host names are merged, not
                       duplicated; existing values always win)

Press any key to close`
	return dialogBox(help, 56)
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
	// Build row slice: title, separator, then item lines.
	rows := make([]string, 0, h)
	rows = append(rows, title)
	rows = append(rows, strings.Repeat("─", w))
	rows = append(rows, lines...)

	// Hard-clip or pad to exactly h rows so lipgloss never needs to guess.
	// Clip from the bottom — title is always rows[0] and stays visible.
	if len(rows) > h {
		rows = rows[:h]
	}
	for len(rows) < h {
		rows = append(rows, "")
	}

	s := stylePanel
	if active {
		s = stylePanelActive
	}
	// stylePanel has Padding(0,1): lipgloss .Width() budgets content+padding
	// together, so pass w+2 to give the text rows (already sized to w) their
	// 1-col padding on each side without lipgloss wrapping the overflow onto
	// an extra row.
	return s.Width(w + 2).Height(h).Render(strings.Join(rows, "\n"))
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// panelLineView returns the visible slice of plain-text s for a horizontal
// viewport of maxCols visual columns starting at visual column hScroll.
// Uses only ASCII 1-col indicators: '<' (hidden content left), '>' (right).
// All indicator characters are unambiguously 1 visual column wide in every locale.
func panelLineView(s string, hScroll, maxCols int) string {
	if maxCols <= 0 {
		return ""
	}
	if hScroll <= 0 {
		// No scroll: truncate to maxCols with '>' indicator if cut.
		if lipgloss.Width(s) <= maxCols {
			return s
		}
		runes := []rune(s)
		w := 0
		for i, r := range runes {
			rw := lipgloss.Width(string(r))
			if w+rw > maxCols-1 {
				return string(runes[:i]) + ">"
			}
			w += rw
		}
		return s
	}

	// Scrolled right: skip hScroll visual columns from the left.
	runes := []rune(s)
	col := 0
	startIdx := len(runes) // default: fully scrolled past end
	for i, r := range runes {
		if col >= hScroll {
			startIdx = i
			break
		}
		col += lipgloss.Width(string(r))
	}

	avail := maxCols - 1 // reserve 1 col for leading '<'

	var viewRunes []rune
	viewW := 0
	hasRight := false
	for _, r := range runes[startIdx:] {
		rw := lipgloss.Width(string(r))
		if viewW+rw > avail {
			hasRight = true
			break
		}
		viewRunes = append(viewRunes, r)
		viewW += rw
	}

	if hasRight {
		// Reserve 1 more col for trailing '>'.
		for viewW > avail-1 && len(viewRunes) > 0 {
			removed := viewRunes[len(viewRunes)-1]
			viewRunes = viewRunes[:len(viewRunes)-1]
			viewW -= lipgloss.Width(string(removed))
		}
	}

	result := "<" + string(viewRunes)
	if hasRight {
		result += ">"
	}
	return result
}
