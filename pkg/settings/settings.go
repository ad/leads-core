package settings

import (
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

// Handler represents the settings HTTP handler
type Handler struct {
	staticFS http.FileSystem
}

// NewHandler creates a new settings handler
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

// ServeHTTP handles HTTP requests for the settings
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Handle root path - serve index.html
	if r.URL.Path == "/settings" || r.URL.Path == "/settings/" {
		h.serveIndex(w, r)
		return
	}

	// Handle static files
	if strings.HasPrefix(r.URL.Path, "/settings/") {
		// Remove /settings prefix to serve from static directory
		path := strings.TrimPrefix(r.URL.Path, "/settings")
		if path == "" || path == "/" {
			h.serveIndex(w, r)
			return
		}

		// Check if it's a static file request (CSS, JS, images, etc.)
		if h.isStaticFile(path) {
			h.serveStatic(w, r, path)
			return
		}

		// For non-static files under /settings/, serve index.html (SPA routing)
		h.serveIndex(w, r)
		return
	}

	// 404 for other paths
	http.NotFound(w, r)
}

// serveIndex serves the main settings HTML page
func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	file, err := h.staticFS.Open("templates/index.html")
	if err != nil {
		http.Error(w, "Settings not found", http.StatusNotFound)
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

// isStaticFile checks if the given path is a static file (CSS, JS, images, etc.)
func (h *Handler) isStaticFile(path string) bool {
	// Try to open the file first
	file, err := h.staticFS.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return false
	}

	// Don't treat directories as static files
	if info.IsDir() {
		return false
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".css", ".js", ".html", ".json", ".ico", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".woff", ".woff2", ".ttf", ".eot":
		return true
	default:
		return false
	}
}
