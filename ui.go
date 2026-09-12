package main

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// webAssets contains the production Vite build so normal deployment remains
// a single self-hosted Go binary.
//
//go:embed web/dist
var webAssets embed.FS

var webDist = func() fs.FS {
	dist, err := fs.Sub(webAssets, "web/dist")
	if err != nil {
		panic(err)
	}
	return dist
}()

// Kept temporarily for source compatibility with the retired dashboard path;
// ServeHTTP routes all non-RPC requests through serveUI before that branch.
const uiHTML = ""

func serveUI(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	requested := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if requested == "." || requested == "" {
		requested = "index.html"
	}
	if info, err := fs.Stat(webDist, requested); err == nil && !info.IsDir() {
		if strings.HasPrefix(requested, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.FileServer(http.FS(webDist)).ServeHTTP(w, r)
		return true
	}
	// React Router owns application paths. Returning the shell makes nested
	// routes bookmarkable and safe to refresh in production.
	index, err := fs.ReadFile(webDist, "index.html")
	if err != nil {
		http.Error(w, "human application unavailable", http.StatusServiceUnavailable)
		return true
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(index)
	return true
}
