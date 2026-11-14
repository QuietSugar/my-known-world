package api

import (
	"context"
	"github.com/QuietSugar/my-known-world/internal/graph"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type resolutionRepository struct {
	memoryRepository
	stored graph.FieldResolution
}

func (r *resolutionRepository) ResolveField(_ context.Context, v graph.FieldResolution) (graph.FieldResolution, error) {
	r.stored = v
	r.stored.ID = 9
	return r.stored, nil
}
func (r *resolutionRepository) FieldResolutions(context.Context, string) ([]graph.FieldResolution, error) {
	return []graph.FieldResolution{r.stored}, nil
}
func TestFieldResolutionEndpointRequiresAuthAndAppendsDecision(t *testing.T) {
	repo := &resolutionRepository{memoryRepository: memoryRepository{g: fixture()}}
	token := strings.Repeat("t", 32)
	h := Handler{Graph: repo.g, Repository: repo, WriteToken: token}
	body := `{"nodeId":"m","fieldPath":"machine.hostname","selectedAssertionIds":["a1"],"rationaleReference":"ticket:12"}`
	request := func(auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/assertions/resolve", strings.NewReader(body))
		if auth {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if got := request(false); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status %d", got.Code)
	}
	if got := request(true); got.Code != http.StatusCreated || !strings.Contains(got.Body.String(), `"id":9`) {
		t.Fatalf("resolution failed: %d %s", got.Code, got.Body.String())
	}
}
