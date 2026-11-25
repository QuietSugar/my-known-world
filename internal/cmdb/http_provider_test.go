package cmdb

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testProviderToken = "0123456789abcdef0123456789abcdef"

// fakeSecretService implements the documented generic contract so the adapter
// can be exercised without any real secret service.
type fakeSecretService struct {
	values    map[string]string
	seenAuth  []string
	seenPaths []string
}

func (f *fakeSecretService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.seenAuth = append(f.seenAuth, r.Header.Get("Authorization"))
	f.seenPaths = append(f.seenPaths, r.Method+" "+r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/secrets":
		var body httpProviderRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.values["opaque-1"] = body.Value
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "opaque-1", "ref": "gw://opaque-1"})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/secrets/"):
		value, ok := f.values[strings.TrimPrefix(r.URL.Path, "/secrets/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"value": value})
	case (r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/secrets/")) || (r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/rotate")):
		var body httpProviderRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.values["opaque-1"] = body.Value
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/secrets/"):
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestHTTPProviderImplementsTheGenericContract(t *testing.T) {
	fake := &fakeSecretService{values: map[string]string{}}
	server := httptest.NewServer(fake)
	defer server.Close()
	provider := &HTTPSecretProvider{BaseURL: server.URL, Token: testProviderToken}
	ctx := context.Background()
	meta, err := provider.Create(ctx, SecretRequest{Name: "synthetic", Value: []byte("synthetic-secret")})
	if err != nil || meta.OpaqueID != "opaque-1" || meta.Provider != "http" || meta.AccessReference != "gw://opaque-1" {
		t.Fatalf("create: %+v %v", meta, err)
	}
	if fake.values["opaque-1"] != "synthetic-secret" {
		t.Fatalf("value was not forwarded as text: %q", fake.values["opaque-1"])
	}
	got, err := provider.Read(ctx, "opaque-1")
	if err != nil || string(got.Value) != "synthetic-secret" {
		t.Fatalf("read: %q %v", got.Value, err)
	}
	if _, err = provider.Update(ctx, "opaque-1", SecretRequest{Value: []byte("updated")}); err != nil {
		t.Fatal(err)
	}
	rotated, err := provider.Rotate(ctx, "opaque-1", SecretRequest{Value: []byte("rotated")})
	if err != nil || rotated.RotationAt == "" || rotated.Status != "rotated" {
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
	expected := []string{
		"POST /secrets",
		"GET /secrets/opaque-1",
		"PUT /secrets/opaque-1",
		"POST /secrets/opaque-1/rotate",
		"DELETE /secrets/opaque-1",
	}
	if strings.Join(fake.seenPaths, ",") != strings.Join(expected, ",") {
		t.Fatalf("unexpected protocol paths: %v", fake.seenPaths)
	}
}

func TestHTTPProviderAcceptsOptionalProviderMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"opaque-2","metadata":{"provider":"hashicorp-vault","opaqueId":"opaque-2","status":"active"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"value":"synthetic","metadata":{"provider":"hashicorp-vault","opaqueId":"opaque-2"}}`))
	}))
	defer server.Close()
	provider := &HTTPSecretProvider{BaseURL: server.URL, Token: testProviderToken}
	meta, err := provider.Create(context.Background(), SecretRequest{Name: "synthetic", Value: []byte("synthetic")})
	if err != nil || meta.Provider != "hashicorp-vault" {
		t.Fatalf("provider metadata was ignored: %+v %v", meta, err)
	}
	result, err := provider.Read(context.Background(), "opaque-2")
	if err != nil || result.Metadata.Provider != "hashicorp-vault" || string(result.Value) != "synthetic" {
		t.Fatalf("read: %+v %v", result.Metadata, err)
	}
}

func TestHTTPProviderRejectsUnsafeConfiguration(t *testing.T) {
	ctx := context.Background()
	cases := []*HTTPSecretProvider{
		{BaseURL: "http://provider.example", Token: testProviderToken},
		{BaseURL: "https://provider.example", Token: "short"},
		{BaseURL: "https://user:pass@provider.example", Token: testProviderToken},
		{BaseURL: "https://provider.example/?tenant=1", Token: testProviderToken},
		{BaseURL: "not-a-url", Token: testProviderToken},
		{BaseURL: "ftp://provider.example", Token: testProviderToken},
	}
	for _, provider := range cases {
		if _, err := provider.Read(ctx, "opaque-1"); err == nil {
			t.Fatalf("unsafe provider configuration was accepted: %q", provider.BaseURL)
		}
	}
	// Loopback sidecars work without an opt-in; remote plaintext needs one.
	if provider := (&HTTPSecretProvider{BaseURL: "http://127.0.0.1:9", Token: testProviderToken}); func() bool {
		_, err := provider.endpoint("/secrets")
		return err != nil
	}() {
		t.Fatal("loopback http endpoint was rejected")
	}
	if _, err := (&HTTPSecretProvider{BaseURL: "http://provider.example", Token: testProviderToken, AllowInsecureHTTP: true}).endpoint("/secrets"); err != nil {
		t.Fatalf("explicit insecure opt-in failed: %v", err)
	}
}

func TestHTTPProviderFailuresStayGenericAndNeverLeakValues(t *testing.T) {
	const leaked = "synthetic-secret-must-not-escape"
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{name: "missing", status: http.StatusNotFound, body: leaked, wantErr: ErrSecretUnavailable},
		{name: "gone", status: http.StatusGone, body: leaked, wantErr: ErrSecretUnavailable},
		{name: "downtime", status: http.StatusServiceUnavailable, body: leaked, wantErr: ErrUnavailable},
		{name: "unauthorized", status: http.StatusUnauthorized, body: leaked},
		{name: "invalid-json", status: http.StatusOK, body: leaked},
		{name: "empty-value", status: http.StatusOK, body: `{"value":""}`},
		{name: "unsafe-reference", status: http.StatusOK, body: `{"id":"bad\nref"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			provider := &HTTPSecretProvider{BaseURL: server.URL, Token: testProviderToken}
			var err error
			if tc.name == "unsafe-reference" {
				_, err = provider.Create(context.Background(), SecretRequest{Name: "synthetic", Value: []byte("synthetic")})
			} else {
				_, err = provider.Read(context.Background(), "opaque-1")
			}
			if err == nil {
				t.Fatal("provider failure was not surfaced")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error %v does not match %v", err, tc.wantErr)
			}
			if strings.Contains(err.Error(), leaked) {
				t.Fatalf("provider error leaked secret material: %v", err)
			}
		})
	}
}

func TestHTTPProviderRejectsNonUTF8Values(t *testing.T) {
	provider := &HTTPSecretProvider{BaseURL: "https://provider.example", Token: testProviderToken}
	if _, err := provider.Create(context.Background(), SecretRequest{Name: "binary", Value: []byte{0xff, 0xfe, 0xfd}}); err == nil {
		t.Fatal("non-UTF-8 value was accepted for a text protocol")
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
	provider := &HTTPSecretProvider{BaseURL: redirector.URL, Token: testProviderToken}
	if _, err := provider.Read(context.Background(), "opaque-1"); err == nil {
		t.Fatal("redirected provider response was accepted")
	}
	if forwarded {
		t.Fatal("bearer credential was forwarded to a redirect target")
	}
}
