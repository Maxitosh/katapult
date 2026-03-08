package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSPAHandler(t *testing.T) {
	// Set up temp directory with static files.
	dir := t.TempDir()

	indexContent := "<!DOCTYPE html><html>SPA</html>"
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(indexContent), 0o644); err != nil {
		t.Fatal(err)
	}

	assetsDir := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	jsContent := "console.log('ok')"
	if err := os.WriteFile(filepath.Join(assetsDir, "main.js"), []byte(jsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := spaHandler(dir)

	tests := []struct {
		name           string
		path           string
		wantStatus     int
		wantBodySubstr string
	}{
		{
			name:           "root serves index.html",
			path:           "/",
			wantStatus:     http.StatusOK,
			wantBodySubstr: indexContent,
		},
		{
			name:           "existing asset is served directly",
			path:           "/assets/main.js",
			wantStatus:     http.StatusOK,
			wantBodySubstr: jsContent,
		},
		{
			name:           "SPA fallback for unknown nested path",
			path:           "/transfers/123",
			wantStatus:     http.StatusOK,
			wantBodySubstr: indexContent,
		},
		{
			name:           "SPA fallback for unknown top-level path",
			path:           "/agents",
			wantStatus:     http.StatusOK,
			wantBodySubstr: indexContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			body := rec.Body.String()
			if !strings.Contains(body, tt.wantBodySubstr) {
				t.Errorf("body = %q, want substring %q", body, tt.wantBodySubstr)
			}
		})
	}
}
