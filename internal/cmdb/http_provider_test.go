package cmdb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testProviderToken = "0123456789abcdef0123456789abcdef"

// fakeRemoteProvider implements the documented provider protocol so the
// adapter can be exercised without any real secret service.
type fakeRemoteProvider struct {
	values    map[string]string
	seenAuth  []string
	seenPaths []string
}

func (f *fakeRemoteProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.seenAuth = append(f.seenAuth, r.Header.Get("Authorization"))
	f.seenPaths = append(f.seenPaths, r.Method+" "+r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/secrets":
		var body httpProviderRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.values["opaque-1"] = body.Value
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"metadata": Metadata{Provider: "remote", OpaqueID: "opaque-1", Status: "active"}})
	case r.Method == http.MethodGet:
		value, ok := f.values[strings.TrimPrefix(r.URL.Path, "/v1/secrets/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(httpProviderSecret{Metadata: Metadata{Provider: "remote", OpaqueID: strings.TrimPrefix(r.URL.Path, "/v1/secrets/")}, Value: value})
	case r.Method == http.MethodPut || strings.HasSuffix(r.URL.Path, "/rotate"):
		var body httpProviderRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.values["opaque-1"] = body.Value
		meta := Metadata{Provider: "remote", OpaqueID: "opaque-1", Status: "active"}
		if strings.HasSuffix(r.URL.Path, "/rotate") {
			meta.Status = "rotated"
			meta.RotationAt = "2026-01-01T00:00:00Z"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"metadata": meta})
	case strings.HasSuffix(r.URL.Path, "/disable"):
		_ = json.NewEncoder(w).Encode(map[string]any{"metadata": Metadata{Provider: "remote", OpaqueID: "opaque-1", Status: "disabled"}})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestHTTPProviderImplementsTheDocumentedLifecycle(t *testing.T) {
	fake := &fakeRemoteProvider{values: map[string]string{}}
	server := httptest.NewServer(fake)
	defer server.Close()
	provider := &HTTPSecretProvider{BaseURL: server.URL, Token: testProviderToken, AllowPlaintextHTTP: true}
	ctx := context.Background()
	meta, err := provider.Create(ctx, SecretRequest{Name: "synthetic", Value: []byte("synthetic-secret")})
	if err != nil || meta.OpaqueID != "opaque-1" || meta.Provider != "remote" {
		t.Fatalf("create: %+v %v", meta, err)
	}
	if fake.values["opaque-1"] != base64.StdEncoding.EncodeToString([]byte("synthetic-secret")) {
		t.Fatalf("value was not base64 encoded on the wire: %q", fake.values["opaque-1"])
	}
	got, err := provider.Read(ctx, "opaque-1")
	if err != nil || string(got.Value) != "synthetic-secret" {
		t.Fatalf("read: %q %v", got.Value, err)
	}
	if _, err = provider.Update(ctx, "opaque-1", SecretRequest{Value: []byte("updated")}); err != nil {
		t.Fatal(err)
	}
	rotated, err := provider.Rotate(ctx, "opaque-1", SecretRequest{Value: []byte("rotated")})
	if err != nil || rotated.RotationAt == "" {
		t.Fatalf("rotate: %+v %v", rotated, err)
	}
	disabled, err := provider.Disable(ctx, "opaque-1")
	if err != nil || disabled.Status != "disabled" {
		t.Fatalf("disable: %+v %v", disabled, err)
	}
	for _, header := range fake.seenAuth {
		if header != "Bearer "+testProviderToken {
			t.Fatalf("provider credential not forwarded: %q", header)
		}
	}
	expected := []string{"POST /v1/secrets", "GET /v1/secrets/opaque-1", "PUT /v1/secrets/opaque-1", "POST /v1/secrets/opaque-1/rotate", "POST /v1/secrets/opaque-1/disable"}
	if strings.Join(fake.seenPaths, ",") != strings.Join(expected, ",") {
		t.Fatalf("unexpected protocol paths: %v", fake.seenPaths)
	}
}

func TestHTTPProviderRejectsUnsafeConfiguration(t *testing.T) {
	ctx := context.Background()
	cases := []*HTTPSecretProvider{
		{BaseURL: "http://127.0.0.1:9", Token: testProviderToken},
		{BaseURL: "https://provider.example", Token: "short"},
		{BaseURL: "https://user:pass@provider.example", Token: testProviderToken},
		{BaseURL: "https://provider.example/?tenant=1", Token: testProviderToken},
		{BaseURL: "not-a-url", Token: testProviderToken},
	}
	for _, provider := range cases {
		if _, err := provider.Read(ctx, "opaque-1"); err == nil {
			t.Fatalf("unsafe provider configuration was accepted: %q", provider.BaseURL)
		}
	}
}

func TestHTTPProviderFailuresStayGenericAndNeverLeakValues(t *testing.T) {
	const leaked = "synthetic-secret-must-not-escape"
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr error
		allowed bool
	}{
		{name: "missing", status: http.StatusNotFound, body: leaked, wantErr: ErrSecretUnavailable},
		{name: "gone", status: http.StatusGone, body: leaked, wantErr: ErrSecretUnavailable},
		{name: "downtime", status: http.StatusServiceUnavailable, body: leaked, wantErr: ErrUnavailable},
		{name: "unauthorized", status: http.StatusUnauthorized, body: leaked},
		{name: "invalid-json", status: http.StatusOK, body: leaked},
		{name: "empty-metadata", status: http.StatusOK, body: `{"metadata":{},"value":"c2VjcmV0"}`},
		{name: "canned", status: http.StatusOK, body: `{"metadata":{"provider":"remote","opaqueId":"other"},"value":"c2VjcmV0"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			provider := &HTTPSecretProvider{BaseURL: server.URL, Token: testProviderToken, AllowPlaintextHTTP: true}
			_, err := provider.Read(context.Background(), "opaque-1")
			if err == nil {
				t.Fatal("provider failure was not surfaced")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error %v does not match %v", err, tc.wantErr)
			}
			if strings.Contains(err.Error(), leaked) || strings.Contains(err.Error(), "c2VjcmV0") {
				t.Fatalf("provider error leaked secret material: %v", err)
			}
		})
	}
}

func TestHTTPProviderDoesNotFollowRedirectsWithItsCredential(t *testing.T) {
	var forwarded bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			forwarded = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()
	provider := &HTTPSecretProvider{BaseURL: redirector.URL, Token: testProviderToken, AllowPlaintextHTTP: true}
	if _, err := provider.Read(context.Background(), "opaque-1"); err == nil {
		t.Fatal("redirected provider response was accepted")
	}
	if forwarded {
		t.Fatal("bearer credential was forwarded to a redirect target")
	}
}

func TestSecretHTTPMapsUnavailableReferencesToNotFound(t *testing.T) {
	vault := &FileVault{Dir: t.TempDir(), Key: []byte("0123456789abcdef0123456789abcdef")}
	handler := HTTPHandler{Secrets: SecretService{Provider: vault, Authorizer: RequestSecretAuthorizer{}}, Authorizer: BearerAuthorizer{Token: testProviderToken}}
	request := httptest.NewRequest(http.MethodGet, "/credentials/unknown-reference", nil)
	request.Header.Set("Authorization", "Bearer "+testProviderToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unavailable reference status %d", response.Code)
	}
}
