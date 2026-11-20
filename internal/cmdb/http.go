package cmdb

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// BearerAuthorizer owns a credential independent of the core graph write token.
type BearerAuthorizer struct{ Token string }

func (a BearerAuthorizer) Authorize(r *http.Request) bool {
	if len(a.Token) < 32 {
		return false
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	raw := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if len(raw) != len(a.Token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(raw), []byte(a.Token)) == 1
}

// HTTPHandler is intended to be mounted on a separately authorized application
// listener. Provider errors are deliberately not returned, because providers may
// accidentally include secret material in their error strings.
type HTTPHandler struct {
	Secrets    SecretService
	Catalog    Catalog
	Resources  ResourceCatalog
	Authorizer BearerAuthorizer
}
type secretHTTPRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (h HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		writeSecretJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if !h.Authorizer.Authorize(r) {
		writeHTTPError(w, http.StatusUnauthorized, "authorized application access required")
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), authorizedApplicationKey{}, true))
	path := strings.TrimSuffix(r.URL.EscapedPath(), "/")
	if path == "" {
		path = "/"
	}
	if path == "/services" || strings.HasPrefix(path, "/services/") {
		h.serveResources(w, r, ServiceResource, path)
		return
	}
	if path == "/accounts" || strings.HasPrefix(path, "/accounts/") {
		h.serveResources(w, r, AccountResource, path)
		return
	}
	if strings.HasPrefix(path, "/environments") {
		h.serveEnvironment(w, r, path)
		return
	}
	if path == "/credentials" && r.Method == http.MethodPost {
		var body secretHTTPRequest
		if err := decodeSecretBody(w, r, &body); err != nil {
			writeHTTPError(w, http.StatusBadRequest, "invalid secret request")
			return
		}
		result, err := h.Secrets.Execute(r.Context(), CreateSecret, "", SecretRequest{Name: body.Name, Value: []byte(body.Value)})
		if err != nil {
			writeSecretServiceError(w, err)
			return
		}
		writeSecretJSON(w, http.StatusCreated, map[string]any{"metadata": result.Metadata})
		return
	}
	if !strings.HasPrefix(path, "/credentials/") {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/credentials/"), "/")
	if len(parts) < 1 || len(parts) > 2 {
		http.NotFound(w, r)
		return
	}
	ref, err := url.PathUnescape(parts[0])
	if err != nil || ref == "" {
		http.NotFound(w, r)
		return
	}
	action := Action("")
	var body secretHTTPRequest
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		action = ReadSecret
	case len(parts) == 1 && r.Method == http.MethodPut:
		action = UpdateSecret
	case len(parts) == 2 && parts[1] == "rotate" && r.Method == http.MethodPost:
		action = RotateSecret
	case len(parts) == 2 && parts[1] == "disable" && r.Method == http.MethodPost:
		action = DisableSecret
	case len(parts) == 1 && r.Method == http.MethodDelete:
		action = DisableSecret
	default:
		w.Header().Set("Allow", "GET, PUT, POST, DELETE")
		writeHTTPError(w, http.StatusMethodNotAllowed, "unsupported method")
		return
	}
	if action == UpdateSecret || action == RotateSecret {
		if err := decodeSecretBody(w, r, &body); err != nil {
			writeHTTPError(w, http.StatusBadRequest, "invalid secret request")
			return
		}
	}
	result, err := h.Secrets.Execute(r.Context(), action, ref, SecretRequest{Name: body.Name, Value: []byte(body.Value)})
	if err != nil {
		writeSecretServiceError(w, err)
		return
	}
	response := map[string]any{"metadata": result.Metadata}
	if action == ReadSecret {
		response["value"] = string(result.Value)
	}
	writeSecretJSON(w, http.StatusOK, response)
}

func (h HTTPHandler) serveEnvironment(w http.ResponseWriter, r *http.Request, path string) {
	if path == "/environments" {
		switch r.Method {
		case http.MethodGet:
			items, err := h.Catalog.Search(r.Context(), EnvironmentFilter{Query: r.URL.Query().Get("q"), Tag: r.URL.Query().Get("tag"), ServiceID: r.URL.Query().Get("serviceId"), AccountID: r.URL.Query().Get("accountId")})
			if err != nil {
				writeHTTPError(w, http.StatusServiceUnavailable, "environment catalog unavailable")
				return
			}
			writeSecretJSON(w, http.StatusOK, map[string]any{"items": items})
			return
		case http.MethodPost:
			var environment Environment
			if err := decodeSecretBody(w, r, &environment); err != nil {
				writeHTTPError(w, http.StatusBadRequest, "invalid environment request")
				return
			}
			if err := h.Catalog.Save(r.Context(), environment); err != nil {
				writeHTTPError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeSecretJSON(w, http.StatusCreated, environment)
			return
		default:
			w.Header().Set("Allow", "GET, POST")
			writeHTTPError(w, http.StatusMethodNotAllowed, "unsupported method")
			return
		}
	}
	if !strings.HasPrefix(path, "/environments/") {
		http.NotFound(w, r)
		return
	}
	id, err := url.PathUnescape(strings.TrimPrefix(path, "/environments/"))
	if err != nil || id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var environment Environment
		if err := decodeSecretBody(w, r, &environment); err != nil || environment.ID != id {
			writeHTTPError(w, http.StatusBadRequest, "environment ID must match request path")
			return
		}
		if err := h.Catalog.Save(r.Context(), environment); err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeSecretJSON(w, http.StatusOK, environment)
	case http.MethodDelete:
		if err := h.Catalog.Remove(r.Context(), id); err != nil {
			writeHTTPError(w, http.StatusServiceUnavailable, "environment catalog unavailable")
			return
		}
		writeSecretJSON(w, http.StatusOK, map[string]string{"deleted": id})
	default:
		w.Header().Set("Allow", "PUT, DELETE")
		writeHTTPError(w, http.StatusMethodNotAllowed, "unsupported method")
	}
}

func decodeSecretBody(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON value")
	}
	return nil
}
func writeSecretServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUnavailable):
		writeHTTPError(w, http.StatusServiceUnavailable, "secret provider unavailable")
	case errors.Is(err, ErrSecretUnavailable):
		writeHTTPError(w, http.StatusNotFound, "secret unavailable")
	default:
		writeHTTPError(w, http.StatusBadGateway, "secret operation failed")
	}
}
func writeHTTPError(w http.ResponseWriter, status int, message string) {
	writeSecretJSON(w, status, map[string]string{"error": message})
}
func writeSecretJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h HTTPHandler) serveResources(w http.ResponseWriter, r *http.Request, kind ResourceKind, path string) {
	base := "/services"
	if kind == AccountResource {
		base = "/accounts"
	}
	if path == base {
		switch r.Method {
		case http.MethodGet:
			items, err := h.Resources.List(r.Context(), kind)
			if err != nil {
				writeHTTPError(w, http.StatusServiceUnavailable, "application catalog unavailable")
				return
			}
			writeSecretJSON(w, http.StatusOK, map[string]any{"items": items})
		case http.MethodPost:
			var item Resource
			if err := decodeSecretBody(w, r, &item); err != nil {
				writeHTTPError(w, http.StatusBadRequest, "invalid catalog request")
				return
			}
			item.Kind = kind
			if err := h.Resources.Save(r.Context(), item); err != nil {
				writeHTTPError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeSecretJSON(w, http.StatusCreated, item)
		default:
			w.Header().Set("Allow", "GET, POST")
			writeHTTPError(w, http.StatusMethodNotAllowed, "unsupported method")
		}
		return
	}
	id, err := url.PathUnescape(strings.TrimPrefix(path, base+"/"))
	if err != nil || id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var item Resource
		if err := decodeSecretBody(w, r, &item); err != nil || item.ID != id {
			writeHTTPError(w, http.StatusBadRequest, "resource ID must match request path")
			return
		}
		item.Kind = kind
		if err := h.Resources.Save(r.Context(), item); err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeSecretJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := h.Resources.Remove(r.Context(), kind, id); err != nil {
			writeHTTPError(w, http.StatusServiceUnavailable, "application catalog unavailable")
			return
		}
		writeSecretJSON(w, http.StatusOK, map[string]string{"deleted": id})
	default:
		w.Header().Set("Allow", "PUT, DELETE")
		writeHTTPError(w, http.StatusMethodNotAllowed, "unsupported method")
	}
}
