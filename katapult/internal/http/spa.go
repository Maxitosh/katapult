package http

// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1

// SPA handler that serves static files from a directory.
// If a file exists, serve it. If not, serve index.html (for SPA client-side routing).

import (
	"net/http"
	"os"
	"path/filepath"
)

// spaFileSystem wraps http.Dir to fall back to index.html for missing paths,
// enabling client-side routing in single-page applications.
type spaFileSystem struct {
	root http.Dir
}

// Open tries to open the requested file. If the file does not exist or is a
// directory without its own index.html, it returns /index.html instead.
func (fs spaFileSystem) Open(name string) (http.File, error) {
	f, err := fs.root.Open(name)
	if os.IsNotExist(err) {
		return fs.root.Open("/index.html")
	}
	if err != nil {
		return nil, err
	}

	// If the path is a directory, check for an index.html inside it.
	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if stat.IsDir() {
		indexPath := filepath.Join(name, "index.html")
		if _, err := fs.root.Open(indexPath); os.IsNotExist(err) {
			_ = f.Close()
			return fs.root.Open("/index.html")
		}
	}

	return f, nil
}

// spaHandler returns an http.Handler that serves static files from dir,
// falling back to index.html for paths that do not match a real file.
func spaHandler(dir string) http.Handler {
	return http.FileServer(spaFileSystem{root: http.Dir(dir)})
}
