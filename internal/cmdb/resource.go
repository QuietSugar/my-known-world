package cmdb

import (
	"context"
	"errors"
	"strings"
)

type ResourceKind string

const (
	ServiceResource ResourceKind = "service"
	AccountResource ResourceKind = "account"
)

type Resource struct {
	ID             string       `json:"id"`
	Kind           ResourceKind `json:"kind"`
	Name           string       `json:"name"`
	Description    string       `json:"description,omitempty"`
	EnvironmentIDs []string     `json:"environmentIds,omitempty"`
	CredentialRef  string       `json:"credentialRef,omitempty"`
}
type ResourceRepository interface {
	ListResources(context.Context, ResourceKind) ([]Resource, error)
	PutResource(context.Context, Resource) error
	DeleteResource(context.Context, ResourceKind, string) error
}
type ResourceCatalog struct{ Repository ResourceRepository }

func (c ResourceCatalog) List(ctx context.Context, k ResourceKind) ([]Resource, error) {
	if c.Repository == nil {
		return nil, errors.New("resource repository is not configured")
	}
	if !validResourceKind(k) {
		return nil, errors.New("unsupported resource kind")
	}
	return c.Repository.ListResources(ctx, k)
}
func (c ResourceCatalog) Save(ctx context.Context, r Resource) error {
	if c.Repository == nil {
		return errors.New("resource repository is not configured")
	}
	r.ID = strings.TrimSpace(r.ID)
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	r.CredentialRef = strings.TrimSpace(r.CredentialRef)
	if !validResourceKind(r.Kind) || r.ID == "" || r.Name == "" || len(r.ID) > 256 || len(r.Name) > 256 || strings.ContainsAny(r.ID+r.Name+r.Description+r.CredentialRef, "\r\n") {
		return errors.New("resource kind, ID and name must be safe and required")
	}
	if len(r.Description) > 4096 || len(r.CredentialRef) > 2048 {
		return errors.New("resource field limit exceeded")
	}
	r.EnvironmentIDs = cleanStrings(r.EnvironmentIDs)
	if len(r.EnvironmentIDs) > 100 {
		return errors.New("environment membership limit exceeded")
	}
	return c.Repository.PutResource(ctx, r)
}
func (c ResourceCatalog) Remove(ctx context.Context, k ResourceKind, id string) error {
	if c.Repository == nil {
		return errors.New("resource repository is not configured")
	}
	if !validResourceKind(k) || strings.TrimSpace(id) == "" {
		return errors.New("resource kind and ID are required")
	}
	return c.Repository.DeleteResource(ctx, k, id)
}
func validResourceKind(k ResourceKind) bool { return k == ServiceResource || k == AccountResource }
