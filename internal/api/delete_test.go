package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthenticatedExplicitNodeDelete(t *testing.T) {
	g := fixture()
	repo := &memoryRepository{g: g}
	token := strings.Repeat("z", 32)
	h := &Handler{Graph: g, Repository: repo, WriteToken: token}
	req := httptest.NewRequest(http.MethodDelete, "/api/nodes/machine", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", w.Code, w.Body.String())
	}
	if _, ok := repo.g.Node("machine"); ok {
		t.Fatal("node remains after delete")
	}
	req = httptest.NewRequest(http.MethodDelete, "/api/nodes/self", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("Self deletion should be rejected, got %d", w.Code)
	}
}
