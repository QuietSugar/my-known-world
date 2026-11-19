package cmdb

import (
	"context"
	"errors"
	"testing"
)

type testProvider struct {
	calls []Action
	value []byte
}

func (p *testProvider) Create(_ context.Context, r SecretRequest) (Metadata, error) {
	p.calls = append(p.calls, CreateSecret)
	p.value = append([]byte(nil), r.Value...)
	return Metadata{Provider: "test", OpaqueID: "opaque-1"}, nil
}
func (p *testProvider) Read(_ context.Context, id string) (SecretResult, error) {
	p.calls = append(p.calls, ReadSecret)
	return SecretResult{Metadata: Metadata{Provider: "test", OpaqueID: id}, Value: append([]byte(nil), p.value...)}, nil
}
func (p *testProvider) Update(_ context.Context, _ string, r SecretRequest) (Metadata, error) {
	p.calls = append(p.calls, UpdateSecret)
	p.value = append([]byte(nil), r.Value...)
	return Metadata{Provider: "test", OpaqueID: "opaque-1"}, nil
}
func (p *testProvider) Rotate(_ context.Context, _ string, r SecretRequest) (Metadata, error) {
	p.calls = append(p.calls, RotateSecret)
	p.value = append([]byte(nil), r.Value...)
	return Metadata{Provider: "test", OpaqueID: "opaque-1", Status: "rotated"}, nil
}
func (p *testProvider) Disable(_ context.Context, id string) (Metadata, error) {
	p.calls = append(p.calls, DisableSecret)
	return Metadata{Provider: "test", OpaqueID: id, Status: "disabled"}, nil
}

type testAuth struct{ allowed bool }

func (a testAuth) Authorize(context.Context, Action, string) error {
	if !a.allowed {
		return errors.New("denied")
	}
	return nil
}
func TestSecretServiceAuthorizesIndependentlyAndDelegatesLifecycle(t *testing.T) {
	p := &testProvider{}
	service := SecretService{Provider: p, Authorizer: testAuth{allowed: false}}
	if _, err := service.Execute(context.Background(), ReadSecret, "opaque-1", SecretRequest{}); err == nil {
		t.Fatal("unauthorized secret read succeeded")
	}
	if len(p.calls) != 0 {
		t.Fatal("provider invoked before authorization")
	}
	service.Authorizer = testAuth{allowed: true}
	for _, action := range []Action{CreateSecret, ReadSecret, UpdateSecret, RotateSecret, DisableSecret} {
		req := SecretRequest{}
		ref := "opaque-1"
		if action == CreateSecret {
			ref = ""
		}
		if action == CreateSecret || action == UpdateSecret || action == RotateSecret {
			req.Value = []byte("synthetic-only")
		}
		if _, err := service.Execute(context.Background(), action, ref, req); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}
	if len(p.calls) != 5 {
		t.Fatalf("provider calls=%v", p.calls)
	}
}
func TestSecretServiceRejectsMissingProviderAndInvalidSecrets(t *testing.T) {
	if _, err := (SecretService{}).Execute(context.Background(), ReadSecret, "ref", SecretRequest{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}
	service := SecretService{Provider: &testProvider{}, Authorizer: testAuth{allowed: true}}
	if _, err := service.Execute(context.Background(), CreateSecret, "", SecretRequest{}); err == nil {
		t.Fatal("empty secret accepted")
	}
	if _, err := service.Execute(context.Background(), ReadSecret, "bad\nref", SecretRequest{}); err == nil {
		t.Fatal("unsafe opaque reference accepted")
	}
}
