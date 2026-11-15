package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyAuthenticatedNodeDeleteRequiresPreview(t *testing.T) {
	g := fixture()
	repo := &memoryRepository{g: g}
	token := strings.Repeat("z", 32)
	h := &Handler{Graph: g, Repository: repo, WriteToken: token}
	for _, id := range []string{"machine", "self"} {
		req := httptest.NewRequest(http.MethodDelete, "/api/nodes/"+id, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusPreconditionRequired {
			t.Fatalf("delete %s should require preview, got %d %s", id, w.Code, w.Body.String())
		}
	}
	if _, ok := repo.g.Node("machine"); !ok {
		t.Fatal("legacy endpoint deleted machine")
	}
}
