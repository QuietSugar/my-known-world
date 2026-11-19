package cmdb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecretHTTPUsesIndependentAuthAndNeverReturnsCreatedSecret(t *testing.T) {
	provider := &testProvider{}
	handler := HTTPHandler{Secrets: SecretService{Provider: provider, Authorizer: testAuth{allowed: true}}, Authorizer: BearerAuthorizer{Token: "0123456789abcdef0123456789abcdef"}}
	request := httptest.NewRequest(http.MethodPost, "/credentials", strings.NewReader(`{"name":"synthetic","value":"test-secret"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || len(provider.calls) != 0 {
		t.Fatalf("request without separate bearer auth reached provider: %d %v", response.Code, provider.calls)
	}
	request = httptest.NewRequest(http.MethodPost, "/credentials", strings.NewReader(`{"name":"synthetic","value":"test-secret"}`))
	request.Header.Set("Authorization", "Bearer 0123456789abcdef0123456789abcdef")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || strings.Contains(response.Body.String(), "test-secret") {
		t.Fatalf("create response leaked secret or failed: %d %s", response.Code, response.Body.String())
	}
	var created struct {
		Metadata Metadata `json:"metadata"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil || created.Metadata.OpaqueID == "" {
		t.Fatalf("opaque provider ID missing: %+v %v", created, err)
	}
	request = httptest.NewRequest(http.MethodGet, "/credentials/opaque-1", nil)
	request.Header.Set("Authorization", "Bearer 0123456789abcdef0123456789abcdef")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"value":"test-secret"`) || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("authorized read failed or cache protection missing: %d %s", response.Code, response.Body.String())
	}
}

func TestCMDBHTTPEnvironmentRoutesUseApplicationRepositoryAndAuth(t *testing.T) {
	catalog := Catalog{Repository: memoryEnvironments{}}
	handler := HTTPHandler{Catalog: catalog, Authorizer: BearerAuthorizer{Token: "0123456789abcdef0123456789abcdef"}}
	request := httptest.NewRequest(http.MethodPost, "/environments", strings.NewReader(`{"id":"test","name":"Synthetic","serviceIds":["svc"],"accountIds":["acct"]}`))
	request.Header.Set("Authorization", "Bearer 0123456789abcdef0123456789abcdef")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create environment: %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/environments?serviceId=svc", nil)
	request.Header.Set("Authorization", "Bearer 0123456789abcdef0123456789abcdef")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Synthetic") {
		t.Fatalf("filter environment: %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/environments", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized environment access: %d", response.Code)
	}
}
