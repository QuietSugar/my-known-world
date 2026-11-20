package cmdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// HTTPSecretProvider connects the CMDB layer to an operator-supplied secret
// service. The service is fully decoupled: this repository knows only the small
// JSON contract in docs/secret-providers.md and nothing about the vendor,
// storage engine, or encryption scheme behind the endpoint. Any Vaultwarden,
// Vault, KeePass, cloud KMS, or local sidecar gateway can satisfy the contract.
//
// The adapter never logs, caches, or persists values: they are forwarded to the
// authorized caller and then discarded. Provider errors are reduced to generic
// sentinels because a remote service may accidentally echo secret material.
type HTTPSecretProvider struct {
	BaseURL string
	Token   string
	// AllowInsecureHTTP permits plaintext HTTP to a non-loopback host. Loopback
	// endpoints are allowed without it so a local sidecar works out of the box.
	AllowInsecureHTTP bool
	Client            *http.Client
	// Timeout bounds each provider round trip; zero means 10 seconds.
	Timeout time.Duration
}

type httpProviderSecret struct {
	// Minimal contract fields.
	ID    string `json:"id,omitempty"`
	Ref   string `json:"ref,omitempty"`
	Value string `json:"value,omitempty"`
	// Optional richer metadata, accepted when a provider wants to report it.
	Metadata *Metadata `json:"metadata,omitempty"`
}

type httpProviderRequest struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value"`
}

const (
	httpProviderMaxResponse = 256 << 10
	httpProviderName        = "http"
	httpProviderNameLimit   = 64
)

func (p *HTTPSecretProvider) Create(ctx context.Context, r SecretRequest) (Metadata, error) {
	body, err := p.requestBody(r)
	if err != nil {
		return Metadata{}, err
	}
	var response httpProviderSecret
	if err := p.call(ctx, http.MethodPost, "/secrets", body, &response); err != nil {
		return Metadata{}, err
	}
	id := strings.TrimSpace(response.ID)
	if id == "" {
		id = strings.TrimSpace(response.Ref)
	}
	if id == "" || len(id) > 2048 || strings.ContainsAny(id, "\r\n") {
		return Metadata{}, errors.New("secret provider returned an invalid reference")
	}
	meta := p.metadata(response, id)
	meta.OpaqueID = id
	if meta.AccessReference == "" {
		meta.AccessReference = strings.TrimSpace(response.Ref)
	}
	if meta.Status == "" {
		meta.Status = "active"
	}
	return validateProviderMetadata(meta)
}

func (p *HTTPSecretProvider) Read(ctx context.Context, id string) (SecretResult, error) {
	var response httpProviderSecret
	if err := p.call(ctx, http.MethodGet, secretPath(id), nil, &response); err != nil {
		return SecretResult{}, err
	}
	meta := p.metadata(response, id)
	meta.OpaqueID = id
	value := []byte(response.Value)
	if len(value) == 0 || len(value) > 64*1024 {
		return SecretResult{}, errors.New("secret provider returned an unsupported value size")
	}
	meta, err := validateProviderMetadata(meta)
	if err != nil {
		return SecretResult{}, err
	}
	return SecretResult{Metadata: meta, Value: value}, nil
}

func (p *HTTPSecretProvider) Update(ctx context.Context, id string, r SecretRequest) (Metadata, error) {
	return p.mutate(ctx, http.MethodPut, id, "", "active", r)
}

func (p *HTTPSecretProvider) Rotate(ctx context.Context, id string, r SecretRequest) (Metadata, error) {
	return p.mutate(ctx, http.MethodPost, id, "/rotate", "rotated", r)
}

func (p *HTTPSecretProvider) Disable(ctx context.Context, id string) (Metadata, error) {
	var response httpProviderSecret
	if err := p.call(ctx, http.MethodDelete, secretPath(id), nil, &response); err != nil {
		return Metadata{}, err
	}
	meta := p.metadata(response, id)
	meta.OpaqueID = id
	meta.Status = "disabled"
	return validateProviderMetadata(meta)
}

func (p *HTTPSecretProvider) mutate(ctx context.Context, method, id, suffix, defaultStatus string, r SecretRequest) (Metadata, error) {
	body, err := p.requestBody(r)
	if err != nil {
		return Metadata{}, err
	}
	var response httpProviderSecret
	if err := p.call(ctx, method, secretPath(id)+suffix, body, &response); err != nil {
		return Metadata{}, err
	}
	meta := p.metadata(response, id)
	meta.OpaqueID = id
	if meta.Status == "" {
		meta.Status = defaultStatus
	}
	if defaultStatus == "rotated" && meta.RotationAt == "" {
		// The adapter records the confirmed rotation time when the provider does
		// not report one itself.
		meta.RotationAt = time.Now().UTC().Format(time.RFC3339)
	}
	return validateProviderMetadata(meta)
}

// metadata uses provider-reported metadata when present and otherwise derives
// only non-secret fields from the response.
func (p *HTTPSecretProvider) metadata(response httpProviderSecret, id string) Metadata {
	if response.Metadata != nil {
		meta := *response.Metadata
		if meta.Provider == "" {
			meta.Provider = httpProviderName
		}
		meta.OpaqueID = id
		return meta
	}
	// The top-level "status":"ok" of the minimal contract confirms the
	// operation; it is not credential status, so only metadata may carry that.
	return Metadata{Provider: httpProviderName, OpaqueID: id}
}

func (p *HTTPSecretProvider) requestBody(r SecretRequest) (httpProviderRequest, error) {
	// Values cross the wire as UTF-8 text. Refusing other bytes is safer than
	// letting JSON encoding silently replace them.
	if !utf8.Valid(r.Value) {
		return httpProviderRequest{}, errors.New("secret value must be UTF-8 text for an HTTP secret provider")
	}
	return httpProviderRequest{Name: r.Name, Value: string(r.Value)}, nil
}

func (p *HTTPSecretProvider) call(ctx context.Context, method, path string, body any, out *httpProviderSecret) error {
	endpoint, err := p.endpoint(path)
	if err != nil {
		return err
	}
	var payload io.Reader
	if method != http.MethodGet && method != http.MethodDelete {
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
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := p.client().Do(request)
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
	if out == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	// A provider may answer plain confirmations without a JSON body.
	data, err := io.ReadAll(io.LimitReader(response.Body, httpProviderMaxResponse))
	if err != nil {
		return ErrUnavailable
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return errors.New("secret provider response was invalid")
	}
	return nil
}

func (p *HTTPSecretProvider) endpoint(path string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(p.BaseURL))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("secret provider URL is invalid")
	}
	switch parsed.Scheme {
	case "https":
	case "http":
		if !p.AllowInsecureHTTP && !isLoopbackHost(parsed.Hostname()) {
			return "", errors.New("secret provider URL must use https unless the endpoint is loopback or CMDB_SECRET_HTTP_ALLOW_INSECURE=1")
		}
	default:
		return "", errors.New("secret provider URL must be an http or https URL")
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

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func secretPath(id string) string {
	return "/secrets/" + url.PathEscape(id)
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
