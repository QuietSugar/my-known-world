// Package webui serves the compiled single-page application without a separate
// runtime service. The development Vite server is not used in production.
package webui

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Handler struct{ Root string }

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; font-src 'self' data:")
	root := filepath.Clean(h.Root)
	requested := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if requested == "." || requested == string(filepath.Separator) {
		requested = "index.html"
	}
	candidate := filepath.Join(root, requested)
	if !strings.HasPrefix(candidate, root+string(filepath.Separator)) && candidate != filepath.Join(root, "index.html") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if info, err := os.Stat(candidate); err != nil || info.IsDir() {
		candidate = filepath.Join(root, "index.html")
	}
	if _, err := os.Stat(candidate); err != nil {
		http.Error(w, "web application is not built", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, candidate)
}
