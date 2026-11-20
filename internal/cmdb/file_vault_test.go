package cmdb

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
	vault = &FileVault{Dir: filepath.Join(t.TempDir(), "vault"), Key: key}
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

func TestFileVaultKeyFilesAreOwnerOnlyAndParsed(t *testing.T) {
	dir := t.TempDir()
	base64Key := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	path := filepath.Join(dir, "vault.key")
	if err := os.WriteFile(path, []byte(base64Key+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadVaultKeyFile(path); err == nil {
		t.Fatal("world-readable key file was accepted")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	key, err := LoadVaultKeyFile(path)
	if err != nil || string(key) != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("base64 key file: %q %v", key, err)
	}
	if key, err = ParseVaultKey([]byte("3031323334353637383961626364656630313233343536373839616263646566")); err != nil || string(key) != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("hex key: %q %v", key, err)
	}
	if _, err = ParseVaultKey([]byte("short")); err == nil {
		t.Fatal("short key accepted")
	}
	link := filepath.Join(dir, "vault.link")
	if err = os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadVaultKeyFile(link); err == nil {
		t.Fatal("symlinked key file was accepted")
	}
}

func TestFileVaultRejectsUnknownAndDisabledReferences(t *testing.T) {
	vault := &FileVault{Dir: filepath.Join(t.TempDir(), "vault"), Key: []byte("0123456789abcdef0123456789abcdef")}
	if _, err := vault.Read(context.Background(), "missing"); !errors.Is(err, ErrSecretUnavailable) {
		t.Fatalf("unknown reference error: %v", err)
	}
	meta, err := vault.Create(context.Background(), SecretRequest{Name: "synthetic", Value: []byte("synthetic")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = vault.Disable(context.Background(), meta.OpaqueID); err != nil {
		t.Fatal(err)
	}
	if _, err = vault.Read(context.Background(), meta.OpaqueID); !errors.Is(err, ErrSecretUnavailable) {
		t.Fatalf("disabled reference error: %v", err)
	}
	if _, err = vault.Update(context.Background(), meta.OpaqueID, SecretRequest{Value: []byte("x")}); !errors.Is(err, ErrSecretUnavailable) {
		t.Fatalf("disabled reference update error: %v", err)
	}
}

func TestFileVaultRekeyReencryptsWithoutLosingRecords(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vault")
	oldKey := []byte("0123456789abcdef0123456789abcdef")
	newKey := []byte("fedcba9876543210fedcba9876543210")
	ctx := context.Background()
	vault := &FileVault{Dir: dir, Key: append([]byte(nil), oldKey...)}
	meta, err := vault.Create(ctx, SecretRequest{Name: "synthetic", Value: []byte("synthetic-value")})
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := vault.Create(ctx, SecretRequest{Name: "synthetic-disabled", Value: []byte("synthetic-disabled")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = vault.Disable(ctx, disabled.OpaqueID); err != nil {
		t.Fatal(err)
	}
	if err = vault.Rekey(ctx, append([]byte(nil), oldKey...)); err == nil {
		t.Fatal("rekey accepted the current key")
	}
	if err = vault.Rekey(ctx, []byte("too-short")); err == nil {
		t.Fatal("rekey accepted an invalid key")
	}
	if err = vault.Rekey(ctx, newKey); err != nil {
		t.Fatal(err)
	}
	if _, err = (&FileVault{Dir: dir, Key: append([]byte(nil), oldKey...)}).Read(ctx, meta.OpaqueID); err == nil {
		t.Fatal("record remained readable with the retired key")
	}
	reopened := &FileVault{Dir: dir, Key: append([]byte(nil), newKey...)}
	got, err := reopened.Read(ctx, meta.OpaqueID)
	if err != nil || string(got.Value) != "synthetic-value" {
		t.Fatalf("read after rekey: %q %v", got.Value, err)
	}
	if _, err = reopened.Read(ctx, disabled.OpaqueID); !errors.Is(err, ErrSecretUnavailable) {
		t.Fatalf("disabled record after rekey: %v", err)
	}
}

func TestFileVaultSerializesWritersWithAnAdvisoryLock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vault")
	vault := &FileVault{Dir: dir, Key: []byte("0123456789abcdef0123456789abcdef")}
	if err := vault.prepareDir(); err != nil {
		t.Fatal(err)
	}
	unlock, err := vault.acquireFileLock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	// A second process (a distinct open file description) must not be able to
	// take the lock while it is held, which keeps read-modify-write atomic.
	file, err := os.OpenFile(filepath.Join(dir, "vault.lock"), os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
		t.Fatal("second writer acquired the vault lock")
	}
}
