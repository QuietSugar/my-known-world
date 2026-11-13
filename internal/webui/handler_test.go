package webui

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServesIndexAndFallsBackForClientRoutes(t *testing.T) {
	root := t.TempDir()
	index := []byte("<!doctype html><title>test app</title>")
	if err := os.WriteFile(filepath.Join(root, "index.html"), index, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := Handler{Root: root}
	for _, route := range []string{"/", "/objects/example"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", route, nil))
		if response.Code != 200 || response.Body.String() != string(index) {
			t.Fatalf("%s: got %d %q", route, response.Code, response.Body.String())
		}
		if response.Header().Get("Content-Security-Policy") == "" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("security headers missing on %s", route)
		}
	}
}

func TestMissingBuildAndTraversalAreNotServed(t *testing.T) {
	root := t.TempDir()
	handler := Handler{Root: root}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
	if response.Code != 404 || !strings.Contains(response.Body.String(), "web application is not built") {
		t.Fatalf("unexpected missing build response: %d %q", response.Code, response.Body.String())
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/../../etc/passwd", nil))
	if response.Code != 404 || strings.Contains(response.Body.String(), "root:") {
		t.Fatalf("traversal was not rejected: %d %q", response.Code, response.Body.String())
	}
}
