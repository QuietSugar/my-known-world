package cmdb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSecretProviderSelectsOnlyConfiguredProviders(t *testing.T) {
	if provider, err := NewSecretProvider(ProviderOptions{}); provider != nil || err != nil {
		t.Fatalf("disabled provider: %v %v", provider, err)
	}
	key := []byte("0123456789abcdef0123456789abcdef")
	provider, err := NewSecretProvider(ProviderOptions{Kind: ProviderFile, Directory: t.TempDir(), Key: key})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := provider.(*FileVault); !ok {
		t.Fatalf("file provider kind returned %T", provider)
	}
	if _, err = NewSecretProvider(ProviderOptions{Kind: ProviderFile, Key: key}); err == nil {
		t.Fatal("file provider without a directory was accepted")
	}
	if _, err = NewSecretProvider(ProviderOptions{Kind: ProviderFile, Directory: t.TempDir()}); err == nil {
		t.Fatal("file provider without a key was accepted")
	}
	if _, err = NewSecretProvider(ProviderOptions{Kind: ProviderFile, Directory: t.TempDir(), Key: []byte("short")}); err == nil {
		t.Fatal("file provider with a short key was accepted")
	}
	if _, err = NewSecretProvider(ProviderOptions{Kind: ProviderKind("vaultwarden")}); err == nil {
		t.Fatal("unknown provider kind was accepted")
	}
	if _, err = NewSecretProvider(ProviderOptions{Kind: ProviderHTTP, URL: "http://provider.example", Token: testProviderToken}); err == nil {
		t.Fatal("plaintext remote HTTP provider was accepted without opt-in")
	}
	if _, err = NewSecretProvider(ProviderOptions{Kind: ProviderHTTP, URL: "http://127.0.0.1:9000", Token: testProviderToken}); err != nil {
		t.Fatalf("loopback sidecar endpoint was rejected: %v", err)
	}
	if _, err = NewSecretProvider(ProviderOptions{Kind: ProviderHTTP, URL: "https://provider.example", Token: "short"}); err == nil {
		t.Fatal("provider with an unsafe token was accepted")
	}
	remote, err := NewSecretProvider(ProviderOptions{Kind: ProviderHTTP, URL: "https://provider.example/", Token: testProviderToken})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := remote.(*HTTPSecretProvider); !ok {
		t.Fatalf("http provider kind returned %T", remote)
	}
}

func TestFileProviderLoadsAnOwnerOnlyKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "vault.key")
	if err := os.WriteFile(keyPath, []byte("fedcba9876543210fedcba9876543210"), 0600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewSecretProvider(ProviderOptions{Kind: ProviderFile, Directory: filepath.Join(dir, "store"), KeyFile: keyPath})
	if err != nil {
		t.Fatal(err)
	}
	vault, ok := provider.(*FileVault)
	if !ok || len(vault.Key) != 32 {
		t.Fatalf("key file was not loaded: %#v", provider)
	}
}

func TestVaultKeyFromEnvironmentRejectsAmbiguousConfiguration(t *testing.T) {
	if _, _, err := VaultKeyFromEnvironment("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=", "/tmp/vault.key"); err == nil {
		t.Fatal("both key sources were accepted")
	}
	key, keyFile, err := VaultKeyFromEnvironment("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=", "")
	if err != nil || keyFile != "" || string(key) != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("base64 key: %q %q %v", key, keyFile, err)
	}
	key, keyFile, err = VaultKeyFromEnvironment("", "/tmp/vault.key")
	if err != nil || key != nil || keyFile != "/tmp/vault.key" {
		t.Fatalf("key file: %q %q %v", key, keyFile, err)
	}
	if _, _, err = VaultKeyFromEnvironment("not-a-key", ""); err == nil {
		t.Fatal("invalid key material was accepted")
	}
}
