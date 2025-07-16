package panel

import (
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

// Handler represents the panel HTTP handler
type Handler struct {
	staticFS http.FileSystem
}

// NewHandler creates a new panel handler
func NewHandler() *Handler {
	// Create sub-filesystem for static files
	staticSubFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("failed to create static sub-filesystem: " + err.Error())
	}

	return &Handler{
		staticFS: http.FS(staticSubFS),
	}
}

// ServeHTTP handles HTTP requests for the panel
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Handle root path - serve index.html
	if r.URL.Path == "/panel" || r.URL.Path == "/panel/" {
		h.serveIndex(w, r)
		return
	}

	// Handle static files
	if strings.HasPrefix(r.URL.Path, "/panel/") {
		// Remove /panel prefix to serve from static directory
		path := strings.TrimPrefix(r.URL.Path, "/panel")
		if path == "" || path == "/" {
			h.serveIndex(w, r)
			return
		}

		// Serve static file
		h.serveStatic(w, r, path)
		return
	}

	// 404 for other paths
	http.NotFound(w, r)
}

// serveIndex serves the main panel HTML page
func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	file, err := h.staticFS.Open("templates/index.html")
	if err != nil {
		http.Error(w, "Panel not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// Get file info for ModTime
	info, err := file.Stat()
	if err != nil {
		http.Error(w, "File error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", info.ModTime(), file)
}

// serveStatic serves static files (CSS, JS, etc.)
func (h *Handler) serveStatic(w http.ResponseWriter, r *http.Request, path string) {
	// Clean the path
	path = filepath.Clean(path)
	if path == "." || path == "/" {
		h.serveIndex(w, r)
		return
	}

	// Try to open the file
	file, err := h.staticFS.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		http.Error(w, "File error", http.StatusInternalServerError)
		return
	}

	// Don't serve directories
	if info.IsDir() {
		http.NotFound(w, r)
		return
	}

	// Set content type based on file extension
	ext := filepath.Ext(path)
	switch ext {
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	// Serve the file
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}
