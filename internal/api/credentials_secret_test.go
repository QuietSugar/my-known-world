package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/bitwarden"
	"github.com/QuietSugar/my-known-world/internal/cmdb"
	"github.com/QuietSugar/my-known-world/internal/graph"
)

func TestCredentialSecretAPI(t *testing.T) {
	g := graph.New()
	provider := cmdb.NewBitwardenProvider(bitwarden.Config{})
	provider.Store().SetItems([]bitwarden.Item{
		{
			ID:       "item-uuid-1",
			Name:     "xasc/test-biz-mysql",
			Username: "deployer",
			Password: "MySQLSecretPass999",
		},
	})

	h := &Handler{
		Graph:          g,
		SecretProvider: provider,
	}

	// 1. 测试成功读取
	req := httptest.NewRequest(http.MethodGet, "/api/credentials/secret?ref=vaultwarden:xasc/test-biz-mysql", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Status   string `json:"status"`
		Ref      string `json:"ref"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if res.Password != "MySQLSecretPass999" {
		t.Fatalf("expected password MySQLSecretPass999, got %s", res.Password)
	}

	// 2. 测试不存在的凭据
	req = httptest.NewRequest(http.MethodGet, "/api/credentials/secret?ref=vaultwarden:not-found", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	// 3. 测试缺少 ref 参数
	req = httptest.NewRequest(http.MethodGet, "/api/credentials/secret", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCredentialSecretAPIUnavailable(t *testing.T) {
	g := graph.New()
	h := &Handler{
		Graph:          g,
		SecretProvider: nil,
	}
	req := httptest.NewRequest(http.MethodGet, "/api/credentials/secret?ref=anything", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when SecretProvider is nil, got %d", rec.Code)
	}
}

func TestCredentialStoreRequiresWriteToken(t *testing.T) {
	h := &Handler{Graph: graph.New(), SecretProvider: cmdb.NewBitwardenProvider(bitwarden.Config{})}
	body := strings.NewReader(`{"ref":"credentials/test/samba","password":"s3cret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/credentials/secret", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("writes must stay disabled without a token, got %d: %s", rec.Code, rec.Body.String())
	}

	h.WriteToken = strings.Repeat("t", 40)
	req = httptest.NewRequest(http.MethodPost, "/api/credentials/secret", strings.NewReader(`{"ref":"credentials/test/samba","password":"s3cret"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer token must be rejected, got %d", rec.Code)
	}
}

func TestCredentialStoreValidatesInput(t *testing.T) {
	h := &Handler{Graph: graph.New(), SecretProvider: cmdb.NewBitwardenProvider(bitwarden.Config{}), WriteToken: strings.Repeat("t", 40)}
	cases := []string{
		`{"ref":"","password":"x"}`,
		`{"ref":"credentials/test/x","password":""}`,
	}
	for _, payload := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/credentials/secret", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+h.WriteToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("payload %s should be rejected with 400, got %d", payload, rec.Code)
		}
	}
}
