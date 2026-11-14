package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestEntityHistoryReadEndpoint(t *testing.T) {
	repo := &memoryRepository{g: fixture()}
	h := Handler{Graph: repo.g, Repository: repo}
	w := request(h, http.MethodGet, "/api/entities/node/machine/history")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"action":"created"`) {
		t.Fatalf("history response invalid: %d %s", w.Code, w.Body.String())
	}
}
