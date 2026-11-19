package cmdb

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileVaultEncryptedLifecycleSurvivesRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vault")
	key := []byte("0123456789abcdef0123456789abcdef")
	ctx := context.Background()
	vault := &FileVault{Dir: dir, Key: key}
	meta, err := vault.Create(ctx, SecretRequest{Name: "synthetic-test", Value: []byte("synthetic-secret-value")})
	if err != nil {
		t.Fatal(err)
	}
	if meta.Provider != "local-file" || meta.OpaqueID == "" {
		t.Fatalf("invalid provider metadata: %+v", meta)
	}
	path := filepath.Join(dir, "vault.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "synthetic-secret-value") {
		t.Fatal("plaintext secret persisted")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("vault mode is %o", info.Mode().Perm())
	}
	vault = &FileVault{Dir: dir, Key: key}
	got, err := vault.Read(ctx, meta.OpaqueID)
	if err != nil || string(got.Value) != "synthetic-secret-value" {
		t.Fatalf("read after restart: %q %v", got.Value, err)
	}
	if _, err := vault.Update(ctx, meta.OpaqueID, SecretRequest{Value: []byte("updated-synthetic")}); err != nil {
		t.Fatal(err)
	}
	got, err = vault.Read(ctx, meta.OpaqueID)
	if err != nil || string(got.Value) != "updated-synthetic" {
		t.Fatalf("updated value: %q %v", got.Value, err)
	}
	rotated, err := vault.Rotate(ctx, meta.OpaqueID, SecretRequest{Value: []byte("rotated-synthetic")})
	if err != nil || rotated.RotationAt == "" {
		t.Fatalf("rotation metadata: %+v %v", rotated, err)
	}
	if _, err := vault.Disable(ctx, meta.OpaqueID); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Read(ctx, meta.OpaqueID); err == nil {
		t.Fatal("disabled credential remained readable")
	}
}
func TestFileVaultRejectsBadKeyAndTamperedCiphertext(t *testing.T) {
	ctx := context.Background()
	vault := &FileVault{Dir: t.TempDir(), Key: []byte("too-short")}
	if _, err := vault.Create(ctx, SecretRequest{Name: "x", Value: []byte("synthetic")}); err == nil {
		t.Fatal("accepted an invalid encryption key")
	}
	key := []byte("0123456789abcdef0123456789abcdef")
	vault = &FileVault{Dir: t.TempDir(), Key: key}
	meta, err := vault.Create(ctx, SecretRequest{Name: "x", Value: []byte("synthetic")})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vault.Dir, "vault.json")
	entries, err := vault.load()
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[meta.OpaqueID]
	entry.Ciphertext[0] ^= 0xff
	entries[meta.OpaqueID] = entry
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	// Malformed records fail closed; no provider error reveals a secret value.
	if _, err = vault.Read(ctx, meta.OpaqueID); err == nil {
		t.Fatal("tampered vault record was accepted")
	}
}
