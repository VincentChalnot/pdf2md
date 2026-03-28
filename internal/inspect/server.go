package inspect

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"sort"

	"github.com/user/pdf2md/internal/model"
)

//go:embed templates/*
var templateFS embed.FS

// Server serves the inspect UI for a Document.
type Server struct {
	Doc  *model.Document
	Port int
	tmpl *template.Template
}

// NewServer creates a new inspect server.
func NewServer(doc *model.Document, port int) (*Server, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	return &Server{Doc: doc, Port: port, tmpl: tmpl}, nil
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.Port)
	fmt.Printf("Inspect server listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, s.Handler())
}

// Handler returns the HTTP handler for the inspect server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/page/", s.handlePage)
	mux.HandleFunc("/fonts", s.handleFonts)
	mux.HandleFunc("/outline", s.handleOutline)
	return mux
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/page/1", http.StatusFound)
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	// Parse /page/{n} or /page/{n}/raw
	path := r.URL.Path
	var suffix string
	// Remove "/page/" prefix
	rest := path[len("/page/"):]
	// Check for /raw suffix
	if len(rest) > 4 && rest[len(rest)-4:] == "/raw" {
		suffix = "raw"
		rest = rest[:len(rest)-4]
	}

	pageNum, err := strconv.Atoi(rest)
	if err != nil || pageNum < 1 || pageNum > len(s.Doc.Pages) {
		http.Error(w, fmt.Sprintf("Invalid page number. Valid range: 1-%d", len(s.Doc.Pages)), http.StatusBadRequest)
		return
	}

	page := s.Doc.Pages[pageNum-1]

	if suffix == "raw" {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(page)
		return
	}

	verbose := r.URL.Query().Get("verbose") == "1"

	data := pageData{
		Page:       page,
		PageNum:    pageNum,
		TotalPages: len(s.Doc.Pages),
		Verbose:    verbose,
		FontMap:    s.Doc.FontMap,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "page.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type pageData struct {
	Page       model.Page
	PageNum    int
	TotalPages int
	Verbose    bool
	FontMap    map[string]model.FontSpec
}

func (s *Server) handleFonts(w http.ResponseWriter, r *http.Request) {
	// Sort fonts by NbChars DESC.
	type fontRow struct {
		model.FontSpec
	}
	var fonts []fontRow
	for _, fs := range s.Doc.FontMap {
		fonts = append(fonts, fontRow{fs})
	}
	sort.Slice(fonts, func(i, j int) bool {
		return fonts[i].NbChars > fonts[j].NbChars
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "fonts.html", fonts); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleOutline(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Outline []model.OutlineItem
	}{
		Outline: s.Doc.Outline,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "outline.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
