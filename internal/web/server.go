package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sync"

	"github.com/elleryq/inv-editor/internal/inventory"
)

//go:embed templates static
var assets embed.FS

// Server holds the shared state for the web interface.
type Server struct {
	inv            *inventory.Inventory
	filePath       string
	format         inventory.Format
	modified       bool
	readOnly       bool
	expandedGroups map[string]bool
	selectedGroup  string
	varCtx         string // "group", "host", or ""
	varName        string
	statusMsg      string
	mu             sync.Mutex
	tmpl           *template.Template
}

// Options configures the web server.
type Options struct {
	Host     string
	Port     int
	ReadOnly bool
}

func NewServer(inv *inventory.Inventory, filePath string, format inventory.Format, opts Options) (*Server, error) {
	funcs := template.FuncMap{
		"mul":       func(a, b int) int { return a * b },
		"urlEncode": url.QueryEscape,
		"nSlice":    func(n int) []struct{} { return make([]struct{}, n) },
		"hasErr": func(s string) bool {
			return len(s) > 0 &&
				(s[0] == 'e' || s[0] == 'c' || s[0] == 'i' || // error, cannot, invalid
					len(s) >= 5 && (s[:5] == "save " || s[:4] == "fail"))
		},
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Server{
		inv:            inv,
		filePath:       filePath,
		format:         format,
		readOnly:       opts.ReadOnly,
		expandedGroups: make(map[string]bool),
		tmpl:           tmpl,
	}, nil
}

// Start listens on host:port and blocks until the server stops.
func (s *Server) Start(host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	mode := "read-write"
	if s.readOnly {
		mode = "read-only"
	}
	fmt.Printf("inv-editor web  %s  →  http://%s  [%s]\n", s.filePath, addr, mode)
	if host == "0.0.0.0" {
		fmt.Println("⚠  Listening on all interfaces — anyone on the network can access this editor.")
	}
	return http.ListenAndServe(addr, s.handler())
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()

	// Static assets (CSS, etc.)
	mux.Handle("/static/", http.FileServerFS(assets))

	// Download current inventory as YAML
	mux.HandleFunc("GET /download", s.handleDownload)

	// Full page
	mux.HandleFunc("GET /{$}", s.handleIndex)

	// Panel fragments (read)
	mux.HandleFunc("GET /api/groups", s.handleGetGroups)
	mux.HandleFunc("GET /api/hosts", s.handleGetHosts)
	mux.HandleFunc("GET /api/vars", s.handleGetVars)

	// Group mutations
	mux.HandleFunc("POST /api/groups/add", s.handleAddGroup)
	mux.HandleFunc("POST /api/groups/rename", s.handleRenameGroup)
	mux.HandleFunc("POST /api/groups/delete", s.handleDeleteGroup)
	mux.HandleFunc("POST /api/groups/expand", s.handleToggleExpand)
	mux.HandleFunc("POST /api/groups/reparent", s.handleReparentGroup)
	mux.HandleFunc("POST /api/groups/copy", s.handleCopyGroupDeep)
	mux.HandleFunc("POST /api/groups/select", s.handleSelectGroup)

	// Host mutations
	mux.HandleFunc("POST /api/hosts/add", s.handleAddHost)
	mux.HandleFunc("POST /api/hosts/rename", s.handleRenameHost)
	mux.HandleFunc("POST /api/hosts/delete", s.handleDeleteHost)
	mux.HandleFunc("POST /api/hosts/move-bulk", s.handleBulkMoveHosts)
	mux.HandleFunc("POST /api/hosts/copy-bulk", s.handleBulkCopyHosts)
	mux.HandleFunc("POST /api/hosts/vars", s.handleSelectHostVars)

	// Var mutations
	mux.HandleFunc("POST /api/vars/set", s.handleSetVar)
	mux.HandleFunc("POST /api/vars/delete", s.handleDeleteVar)

	// File operations
	mux.HandleFunc("POST /api/save", s.handleSave)
	mux.HandleFunc("POST /api/export", s.handleExport)
	mux.HandleFunc("POST /api/import", s.handleImport)

	return mux
}

// guardWrite returns true and writes a 403 if the server is in read-only mode.
func (s *Server) guardWrite(w http.ResponseWriter) bool {
	if s.readOnly {
		http.Error(w, "read-only mode", http.StatusForbidden)
		return true
	}
	return false
}

// render executes a named template with data.
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
