// Package cmdb contains application-layer contracts. It deliberately does not
// persist secret material in the core graph or import PostgreSQL internals.
package cmdb

import (
	"context"
	"errors"
	"strings"
)

type Action string

const (
	CreateSecret  Action = "create"
	ReadSecret    Action = "read"
	UpdateSecret  Action = "update"
	RotateSecret  Action = "rotate"
	DisableSecret Action = "disable"
)

type Metadata struct {
	Provider        string `json:"provider"`
	OpaqueID        string `json:"opaqueId"`
	AccessReference string `json:"accessReference,omitempty"`
	RotationAt      string `json:"rotationAt,omitempty"`
	Status          string `json:"status,omitempty"`
}
type SecretRequest struct {
	Name  string
	Value []byte
}
type SecretResult struct {
	Metadata Metadata
	Value    []byte
}

// SecretProvider is implemented by a separately configured provider (for example,
// a Vaultwarden-compatible adapter). Values are returned only to the authorized
// caller and must never be copied into graph records, history, logs, or exports.
type SecretProvider interface {
	Create(context.Context, SecretRequest) (Metadata, error)
	Read(context.Context, string) (SecretResult, error)
	Update(context.Context, string, SecretRequest) (Metadata, error)
	Rotate(context.Context, string, SecretRequest) (Metadata, error)
	Disable(context.Context, string) (Metadata, error)
}

// Authorizer is independent from core graph write authorization.
type Authorizer interface {
	Authorize(context.Context, Action, string) error
}

type authorizedApplicationKey struct{}

// RequestSecretAuthorizer permits operations only inside HTTP requests that have
// already passed the independent CMDB application bearer check.
type RequestSecretAuthorizer struct{}

func (RequestSecretAuthorizer) Authorize(ctx context.Context, _ Action, _ string) error {
	if authorized, _ := ctx.Value(authorizedApplicationKey{}).(bool); !authorized {
		return errors.New("authorized CMDB application context required")
	}
	return nil
}

type SecretService struct {
	Provider   SecretProvider
	Authorizer Authorizer
}

var ErrUnavailable = errors.New("secret provider is not configured")

func (s SecretService) Execute(ctx context.Context, action Action, ref string, request SecretRequest) (SecretResult, error) {
	if s.Provider == nil {
		return SecretResult{}, ErrUnavailable
	}
	if s.Authorizer == nil {
		return SecretResult{}, errors.New("secret authorization is not configured")
	}
	if err := validateRequest(action, ref, request); err != nil {
		return SecretResult{}, err
	}
	if err := s.Authorizer.Authorize(ctx, action, ref); err != nil {
		return SecretResult{}, err
	}
	switch action {
	case CreateSecret:
		m, err := s.Provider.Create(ctx, request)
		return SecretResult{Metadata: m}, err
	case ReadSecret:
		return s.Provider.Read(ctx, ref)
	case UpdateSecret:
		m, err := s.Provider.Update(ctx, ref, request)
		return SecretResult{Metadata: m}, err
	case RotateSecret:
		m, err := s.Provider.Rotate(ctx, ref, request)
		return SecretResult{Metadata: m}, err
	case DisableSecret:
		m, err := s.Provider.Disable(ctx, ref)
		return SecretResult{Metadata: m}, err
	default:
		return SecretResult{}, errors.New("unsupported secret action")
	}
}
func validateRequest(action Action, ref string, request SecretRequest) error {
	if action != CreateSecret && strings.TrimSpace(ref) == "" {
		return errors.New("opaque provider reference is required")
	}
	if len(ref) > 2048 || strings.ContainsAny(ref, "\r\n") {
		return errors.New("provider reference is invalid")
	}
	if action == CreateSecret && (strings.TrimSpace(request.Name) == "" || len(request.Name) > 256 || strings.ContainsAny(request.Name, "\r\n")) {
		return errors.New("secret name is required and must be safe")
	}
	if action == CreateSecret || action == UpdateSecret || action == RotateSecret {
		if len(request.Value) == 0 || len(request.Value) > 64*1024 {
			return errors.New("secret value is required and must not exceed 64 KiB")
		}
	}
	return nil
}
