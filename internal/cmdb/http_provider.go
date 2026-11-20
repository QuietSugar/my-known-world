package cmdb

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPSecretProvider lets an operator supply their own secret service instead of
// using the built-in file provider. It speaks the documented JSON protocol in
// docs/secret-providers.md over TLS with an independent bearer credential.
//
// The adapter never logs, caches, or persists values: they are forwarded to the
// authorized caller and then discarded. Provider errors are reduced to generic
// sentinels because a remote service may accidentally echo secret material.
type HTTPSecretProvider struct {
	BaseURL string
	Token   string
	// AllowPlaintextHTTP exists for loopback testing only. Production
	// deployments must terminate TLS before the endpoint.
	AllowPlaintextHTTP bool
	Client             *http.Client
	// Timeout bounds each provider round trip; zero means 10 seconds.
	Timeout time.Duration
}

type httpProviderSecret struct {
	Metadata Metadata `json:"metadata"`
	Value    string   `json:"value,omitempty"`
}

type httpProviderRequest struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

const (
	httpProviderMaxResponse = 256 << 10
	httpProviderNameLimit   = 64
)

func (p *HTTPSecretProvider) Create(ctx context.Context, r SecretRequest) (Metadata, error) {
	var response httpProviderSecret
	if err := p.call(ctx, http.MethodPost, "/v1/secrets", httpProviderRequest{Name: r.Name, Value: encodeSecretValue(r.Value)}, &response); err != nil {
		return Metadata{}, err
	}
	meta, err := validateProviderMetadata(response.Metadata)
	if err != nil {
		return Metadata{}, err
	}
	if meta.OpaqueID == "" {
		return Metadata{}, errors.New("provider returned no opaque reference")
	}
	return meta, nil
}

func (p *HTTPSecretProvider) Read(ctx context.Context, id string) (SecretResult, error) {
	var response httpProviderSecret
	if err := p.call(ctx, http.MethodGet, secretPath(id), nil, &response); err != nil {
		return SecretResult{}, err
	}
	meta, err := validateProviderMetadata(response.Metadata)
	if err != nil {
		return SecretResult{}, err
	}
	if meta.OpaqueID != id {
		return SecretResult{}, errors.New("provider returned a mismatched reference")
	}
	value, err := decodeSecretValue(response.Value)
	if err != nil {
		return SecretResult{}, err
	}
	return SecretResult{Metadata: meta, Value: value}, nil
}

func (p *HTTPSecretProvider) Update(ctx context.Context, id string, r SecretRequest) (Metadata, error) {
	return p.mutate(ctx, http.MethodPut, id, "", r)
}

func (p *HTTPSecretProvider) Rotate(ctx context.Context, id string, r SecretRequest) (Metadata, error) {
	return p.mutate(ctx, http.MethodPost, id, "/rotate", r)
}

func (p *HTTPSecretProvider) Disable(ctx context.Context, id string) (Metadata, error) {
	var response httpProviderSecret
	if err := p.call(ctx, http.MethodPost, secretPath(id)+"/disable", nil, &response); err != nil {
		return Metadata{}, err
	}
	meta, err := validateProviderMetadata(response.Metadata)
	if err != nil {
		return Metadata{}, err
	}
	if meta.OpaqueID != id {
		return Metadata{}, errors.New("provider returned a mismatched reference")
	}
	return meta, nil
}

func (p *HTTPSecretProvider) mutate(ctx context.Context, method, id, suffix string, r SecretRequest) (Metadata, error) {
	var response httpProviderSecret
	body := httpProviderRequest{Name: r.Name, Value: encodeSecretValue(r.Value)}
	if err := p.call(ctx, method, secretPath(id)+suffix, body, &response); err != nil {
		return Metadata{}, err
	}
	meta, err := validateProviderMetadata(response.Metadata)
	if err != nil {
		return Metadata{}, err
	}
	if meta.OpaqueID != id {
		return Metadata{}, errors.New("provider returned a mismatched reference")
	}
	return meta, nil
}

func (p *HTTPSecretProvider) call(ctx context.Context, method, path string, body any, out *httpProviderSecret) error {
	endpoint, err := p.endpoint(path)
	if err != nil {
		return err
	}
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return errors.New("secret provider request could not be encoded")
		}
		payload = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, payload)
	if err != nil {
		return errors.New("secret provider request could not be created")
	}
	request.Header.Set("Authorization", "Bearer "+p.Token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Cache-Control", "no-store")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	client := p.client()
	response, err := client.Do(request)
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusNotFound, http.StatusGone:
		return ErrSecretUnavailable
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("secret provider rejected the application credential")
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		// Never surface a provider body: it may contain secret material.
		return ErrUnavailable
	}
	if out == nil {
		return nil
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, httpProviderMaxResponse))
	if err := decoder.Decode(out); err != nil {
		return errors.New("secret provider response was invalid")
	}
	return nil
}

func (p *HTTPSecretProvider) endpoint(path string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(p.BaseURL))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("secret provider URL is invalid")
	}
	if parsed.Scheme != "https" {
		if parsed.Scheme != "http" || !p.AllowPlaintextHTTP {
			return "", errors.New("secret provider URL must use https")
		}
	}
	if len(p.Token) < 32 || strings.ContainsAny(p.Token, "\r\n") {
		return "", errors.New("secret provider token must contain at least 32 characters")
	}
	base := strings.TrimSuffix(parsed.String(), "/")
	return base + path, nil
}

func (p *HTTPSecretProvider) client() *http.Client {
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{
		Timeout: timeout,
		// A redirect would forward the bearer credential to another origin.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func secretPath(id string) string {
	return "/v1/secrets/" + url.PathEscape(id)
}

func encodeSecretValue(value []byte) string {
	return base64.StdEncoding.EncodeToString(value)
}

func decodeSecretValue(value string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, errors.New("secret provider returned an invalid value encoding")
	}
	if len(decoded) == 0 || len(decoded) > 64*1024 {
		return nil, errors.New("secret provider returned an unsupported value size")
	}
	return decoded, nil
}

func validateProviderMetadata(meta Metadata) (Metadata, error) {
	meta.Provider = strings.TrimSpace(meta.Provider)
	meta.OpaqueID = strings.TrimSpace(meta.OpaqueID)
	meta.AccessReference = strings.TrimSpace(meta.AccessReference)
	meta.RotationAt = strings.TrimSpace(meta.RotationAt)
	meta.Status = strings.TrimSpace(meta.Status)
	if meta.Provider == "" || len(meta.Provider) > httpProviderNameLimit {
		return Metadata{}, errors.New("secret provider returned invalid metadata")
	}
	if len(meta.OpaqueID) > 2048 || len(meta.AccessReference) > 2048 || len(meta.RotationAt) > 64 || len(meta.Status) > 32 {
		return Metadata{}, errors.New("secret provider returned oversized metadata")
	}
	if strings.ContainsAny(meta.Provider+meta.OpaqueID+meta.AccessReference+meta.RotationAt+meta.Status, "\r\n") {
		return Metadata{}, errors.New("secret provider returned unsafe metadata")
	}
	return meta, nil
}
