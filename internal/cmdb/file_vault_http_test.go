package cmdb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFileVaultIsReachableOnlyAfterCMDBApplicationAuthorization(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef"
	handler := HTTPHandler{Secrets: SecretService{Provider: &FileVault{Dir: t.TempDir(), Key: []byte("0123456789abcdef0123456789abcdef")}, Authorizer: RequestSecretAuthorizer{}}, Authorizer: BearerAuthorizer{Token: token}}
	request := httptest.NewRequest(http.MethodPost, "/credentials", strings.NewReader(`{"name":"synthetic","value":"synthetic-secret"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status %d", response.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/credentials", strings.NewReader(`{"name":"synthetic","value":"synthetic-secret"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || strings.Contains(response.Body.String(), "synthetic-secret") {
		t.Fatalf("authorized create leaked value or failed: %d %s", response.Code, response.Body.String())
	}
}
