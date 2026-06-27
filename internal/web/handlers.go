package web

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/elleryq/inv-editor/internal/inventory"
)

// ----- template data -----

type treeNode struct {
	Name        string
	Depth       int
	HasChildren bool
	Expanded    bool
	Selected    bool
}

type varEntry struct {
	Key   string
	Value string
}

type pageData struct {
	ReadOnly      bool
	FilePath      string
	Modified      bool
	Status        string
	Groups        []treeNode
	SelectedGroup string
	Hosts         []string
	SelectedHost  string
	VarCtx        string // "group" or "host"
	VarSubject    string // the name whose vars are shown (group name or host name)
	Vars          []varEntry
	AllGroupNames []string // for dropdowns (excluding "all")
	AllGroups     []string // for dropdowns (including "all")
	CurrentGroup  string   // the group whose hosts are shown
}

// ----- tree building -----

func (s *Server) buildTreeNodes(selectedGroup string) []treeNode {
	var nodes []treeNode
	var visit func(name string, depth int)
	visit = func(name string, depth int) {
		g := s.inv.Group(name)
		if g == nil {
			return
		}
		hasChildren := len(g.Children) > 0
		expanded := s.expandedGroups[name]
		nodes = append(nodes, treeNode{
			Name:        name,
			Depth:       depth,
			HasChildren: hasChildren,
			Expanded:    expanded,
			Selected:    name == selectedGroup,
		})
		if expanded {
			for _, child := range g.Children {
				visit(child, depth+1)
			}
		}
	}
	visit("all", 0)
	for _, g := range s.inv.TopLevelGroups() {
		visit(g.Name, 1)
	}
	return nodes
}

// ----- page data assembly -----

func (s *Server) buildPageData(r *http.Request) pageData {
	q := r.URL.Query()
	group := q.Get("group")
	if group == "" {
		group = "all"
	}
	host := q.Get("host")
	varCtx := q.Get("vars") // "group" or "host"
	status := q.Get("msg")

	// consume one-time server status
	s.mu.Lock()
	if s.statusMsg != "" {
		if status == "" {
			status = s.statusMsg
		}
		s.statusMsg = ""
	}
	s.mu.Unlock()

	// hosts for selected group
	var hostNames []string
	if g := s.inv.Group(group); g != nil {
		hostNames = g.Hosts
	}

	// resolve vars
	varSubject := ""
	if varCtx == "group" {
		varSubject = group
	} else if varCtx == "host" {
		varSubject = host
	}

	var vars []varEntry
	if varCtx == "group" {
		if g := s.inv.Group(group); g != nil {
			keys := make([]string, 0, len(g.Vars))
			for k := range g.Vars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				vars = append(vars, varEntry{Key: k, Value: g.Vars[k]})
			}
		}
	} else if varCtx == "host" && host != "" {
		if h := s.inv.Host(host); h != nil {
			keys := make([]string, 0, len(h.Vars))
			for k := range h.Vars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				vars = append(vars, varEntry{Key: k, Value: h.Vars[k]})
			}
		}
	}

	// group name lists for dropdowns
	var allGroupNames []string // excluding "all"
	var allGroups []string     // including "all"
	for _, g := range s.inv.Groups() {
		allGroups = append(allGroups, g.Name)
		if g.Name != "all" {
			allGroupNames = append(allGroupNames, g.Name)
		}
	}

	return pageData{
		ReadOnly:      s.readOnly,
		FilePath:      s.filePath,
		Modified:      s.modified,
		Status:        status,
		Groups:        s.buildTreeNodes(group),
		SelectedGroup: group,
		Hosts:         hostNames,
		SelectedHost:  host,
		VarCtx:        varCtx,
		VarSubject:    varSubject,
		Vars:          vars,
		AllGroupNames: allGroupNames,
		AllGroups:     allGroups,
		CurrentGroup:  group,
	}
}

// ----- redirect helper -----

func redirectBack(w http.ResponseWriter, r *http.Request, group, host, varCtx, msg string) {
	q := url.Values{}
	if group != "" {
		q.Set("group", group)
	}
	if host != "" {
		q.Set("host", host)
	}
	if varCtx != "" {
		q.Set("vars", varCtx)
	}
	if msg != "" {
		q.Set("msg", msg)
	}
	target := "/?" + q.Encode()
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) currentCtx(r *http.Request) (group, host, varCtx string) {
	group = r.FormValue("ctx_group")
	host = r.FormValue("ctx_host")
	varCtx = r.FormValue("ctx_vars")
	if group == "" {
		group = "all"
	}
	return
}

// ----- page handlers -----

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data := s.buildPageData(r)
	s.render(w, "page", data)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmp, err := os.CreateTemp("", "inv-editor-*.yaml")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	if err := inventory.Save(s.inv, tmp.Name(), inventory.FormatYAML); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data, err := os.ReadFile(tmp.Name())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	baseName := s.filePath
	if idx := strings.LastIndexAny(baseName, "/\\"); idx >= 0 {
		baseName = baseName[idx+1:]
	}
	if dot := strings.LastIndex(baseName, "."); dot >= 0 {
		baseName = baseName[:dot]
	}
	baseName += ".yaml"

	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, baseName))
	w.Write(data)
}

// ----- group handlers -----

func (s *Server) handleGetGroups(w http.ResponseWriter, r *http.Request) {
	data := s.buildPageData(r)
	s.render(w, "groups", data)
}

func (s *Server) handleGetHosts(w http.ResponseWriter, r *http.Request) {
	data := s.buildPageData(r)
	s.render(w, "hosts", data)
}

func (s *Server) handleGetVars(w http.ResponseWriter, r *http.Request) {
	data := s.buildPageData(r)
	s.render(w, "vars", data)
}

func (s *Server) handleToggleExpand(w http.ResponseWriter, r *http.Request) {
	group := r.FormValue("group")
	if group != "" {
		s.mu.Lock()
		s.expandedGroups[group] = !s.expandedGroups[group]
		s.mu.Unlock()
	}
	ctxGroup, ctxHost, ctxVars := s.currentCtx(r)
	redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "")
}

func (s *Server) handleSelectGroup(w http.ResponseWriter, r *http.Request) {
	group := r.FormValue("group")
	redirectBack(w, r, group, "", "", "")
}

func (s *Server) handleAddGroup(w http.ResponseWriter, r *http.Request) {
	if s.guardWrite(w) {
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	parent := r.FormValue("parent")
	ctxGroup, ctxHost, ctxVars := s.currentCtx(r)

	if name == "" {
		redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "group name cannot be empty")
		return
	}
	s.mu.Lock()
	ok := s.inv.AddGroupUnder(parent, name)
	if ok {
		s.modified = true
	}
	s.mu.Unlock()
	if !ok {
		redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "group '"+name+"' already exists")
		return
	}
	redirectBack(w, r, name, ctxHost, ctxVars, "group '"+name+"' created")
}

func (s *Server) handleRenameGroup(w http.ResponseWriter, r *http.Request) {
	if s.guardWrite(w) {
		return
	}
	oldName := r.FormValue("name")
	newName := strings.TrimSpace(r.FormValue("newname"))
	ctxGroup, ctxHost, ctxVars := s.currentCtx(r)

	if newName == "" {
		redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "new name cannot be empty")
		return
	}
	s.mu.Lock()
	ok := s.inv.RenameGroup(oldName, newName)
	if ok {
		s.modified = true
	}
	s.mu.Unlock()
	if !ok {
		redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "cannot rename '"+oldName+"'")
		return
	}
	newGroup := ctxGroup
	if ctxGroup == oldName {
		newGroup = newName
	}
	redirectBack(w, r, newGroup, ctxHost, ctxVars, "group renamed to '"+newName+"'")
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if s.guardWrite(w) {
		return
	}
	name := r.FormValue("name")
	ctxGroup, ctxHost, ctxVars := s.currentCtx(r)

	s.mu.Lock()
	ok := s.inv.DeleteGroup(name)
	if ok {
		s.modified = true
		delete(s.expandedGroups, name)
	}
	s.mu.Unlock()
	if !ok {
		redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "cannot delete '"+name+"'")
		return
	}
	newGroup := ctxGroup
	if ctxGroup == name {
		newGroup = "all"
	}
	redirectBack(w, r, newGroup, "", "", "group '"+name+"' deleted")
}

func (s *Server) handleReparentGroup(w http.ResponseWriter, r *http.Request) {
	if s.guardWrite(w) {
		return
	}
	name := r.FormValue("name")
	newParent := r.FormValue("parent")
	ctxGroup, ctxHost, ctxVars := s.currentCtx(r)

	s.mu.Lock()
	ok := s.inv.ReparentGroup(name, newParent)
	if ok {
		s.modified = true
	}
	s.mu.Unlock()
	if !ok {
		redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "cannot reparent '"+name+"'")
		return
	}
	redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "group '"+name+"' moved")
}

// ----- host handlers -----

func (s *Server) handleAddHost(w http.ResponseWriter, r *http.Request) {
	if s.guardWrite(w) {
		return
	}
	hostName := strings.TrimSpace(r.FormValue("name"))
	group := r.FormValue("group")
	ctxGroup, ctxHost, ctxVars := s.currentCtx(r)

	if hostName == "" {
		redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "host name cannot be empty")
		return
	}
	if group == "" {
		group = ctxGroup
	}
	s.mu.Lock()
	ok := s.inv.AddHost(hostName, group)
	// also add to all.hosts for GUIDELINE compliance
	if ok && group != "all" {
		s.inv.AddHost(hostName, "all")
	}
	if ok {
		s.modified = true
	}
	s.mu.Unlock()
	if !ok {
		redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "'"+hostName+"' already in group")
		return
	}
	redirectBack(w, r, ctxGroup, hostName, "host", "host '"+hostName+"' added")
}

func (s *Server) handleRenameHost(w http.ResponseWriter, r *http.Request) {
	if s.guardWrite(w) {
		return
	}
	oldName := r.FormValue("name")
	newName := strings.TrimSpace(r.FormValue("newname"))
	ctxGroup, _, ctxVars := s.currentCtx(r)

	if newName == "" {
		redirectBack(w, r, ctxGroup, oldName, ctxVars, "new name cannot be empty")
		return
	}
	s.mu.Lock()
	ok := s.inv.RenameHost(oldName, newName)
	if ok {
		s.modified = true
	}
	s.mu.Unlock()
	if !ok {
		redirectBack(w, r, ctxGroup, oldName, ctxVars, "cannot rename '"+oldName+"'")
		return
	}
	redirectBack(w, r, ctxGroup, newName, ctxVars, "host renamed to '"+newName+"'")
}

func (s *Server) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	if s.guardWrite(w) {
		return
	}
	hostName := r.FormValue("name")
	fromGroup := r.FormValue("group")
	completely := r.FormValue("completely") == "1"
	ctxGroup, ctxHost, ctxVars := s.currentCtx(r)

	if fromGroup == "" {
		fromGroup = ctxGroup
	}
	s.mu.Lock()
	ok := s.inv.DeleteHost(hostName, fromGroup, completely)
	if ok {
		s.modified = true
	}
	s.mu.Unlock()
	if !ok {
		redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "cannot delete '"+hostName+"'")
		return
	}
	newHost := ctxHost
	if ctxHost == hostName {
		newHost = ""
	}
	redirectBack(w, r, ctxGroup, newHost, "", "host '"+hostName+"' deleted")
}

func (s *Server) handleMoveHost(w http.ResponseWriter, r *http.Request) {
	if s.guardWrite(w) {
		return
	}
	hostName := r.FormValue("name")
	fromGroup := r.FormValue("from")
	toGroup := r.FormValue("to")
	ctxGroup, ctxHost, ctxVars := s.currentCtx(r)

	s.mu.Lock()
	ok := s.inv.MoveHost(hostName, fromGroup, toGroup)
	if ok {
		s.modified = true
	}
	s.mu.Unlock()
	if !ok {
		redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "cannot move '"+hostName+"'")
		return
	}
	redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "host '"+hostName+"' moved to '"+toGroup+"'")
}

func (s *Server) handleCopyHost(w http.ResponseWriter, r *http.Request) {
	if s.guardWrite(w) {
		return
	}
	hostName := r.FormValue("name")
	toGroup := r.FormValue("to")
	ctxGroup, ctxHost, ctxVars := s.currentCtx(r)

	s.mu.Lock()
	ok := s.inv.CopyHostToGroup(hostName, toGroup)
	if ok {
		s.modified = true
	}
	s.mu.Unlock()
	if !ok {
		redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "'"+hostName+"' already in '"+toGroup+"'")
		return
	}
	redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "host '"+hostName+"' copied to '"+toGroup+"'")
}

func (s *Server) handleSelectHostVars(w http.ResponseWriter, r *http.Request) {
	group := r.FormValue("group")
	host := r.FormValue("host")
	redirectBack(w, r, group, host, "host", "")
}

// ----- var handlers -----

func (s *Server) handleSetVar(w http.ResponseWriter, r *http.Request) {
	if s.guardWrite(w) {
		return
	}
	ctx := r.FormValue("ctx")     // "group" or "host"
	subject := r.FormValue("subject") // group name or host name
	key := strings.TrimSpace(r.FormValue("key"))
	value := r.FormValue("value")
	ctxGroup, ctxHost, _ := s.currentCtx(r)

	if key == "" {
		redirectBack(w, r, ctxGroup, ctxHost, ctx, "key cannot be empty")
		return
	}
	s.mu.Lock()
	var ok bool
	if ctx == "group" {
		ok = s.inv.SetGroupVar(subject, key, value)
	} else {
		ok = s.inv.SetHostVar(subject, key, value)
	}
	if ok {
		s.modified = true
	}
	s.mu.Unlock()
	if !ok {
		redirectBack(w, r, ctxGroup, ctxHost, ctx, "cannot set var on '"+subject+"'")
		return
	}
	redirectBack(w, r, ctxGroup, ctxHost, ctx, "var '"+key+"' set")
}

func (s *Server) handleDeleteVar(w http.ResponseWriter, r *http.Request) {
	if s.guardWrite(w) {
		return
	}
	ctx := r.FormValue("ctx")
	subject := r.FormValue("subject")
	key := r.FormValue("key")
	ctxGroup, ctxHost, _ := s.currentCtx(r)

	s.mu.Lock()
	var ok bool
	if ctx == "group" {
		ok = s.inv.DeleteGroupVar(subject, key)
	} else {
		ok = s.inv.DeleteHostVar(subject, key)
	}
	if ok {
		s.modified = true
	}
	s.mu.Unlock()
	if !ok {
		redirectBack(w, r, ctxGroup, ctxHost, ctx, "cannot delete var '"+key+"'")
		return
	}
	redirectBack(w, r, ctxGroup, ctxHost, ctx, "var '"+key+"' deleted")
}

// ----- file handlers -----

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if s.guardWrite(w) {
		return
	}
	ctxGroup, ctxHost, ctxVars := s.currentCtx(r)

	s.mu.Lock()
	err := inventory.Save(s.inv, s.filePath, s.format)
	if err == nil {
		s.modified = false
	}
	s.mu.Unlock()

	if err != nil {
		redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "save error: "+err.Error())
		return
	}
	redirectBack(w, r, ctxGroup, ctxHost, ctxVars, "saved to "+s.filePath)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
