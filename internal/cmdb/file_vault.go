package cmdb

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// FileVault is an encrypted, single-process local provider intended for
// development and functional testing. Production deployments should inject a
// managed vault implementation. Key must be a dedicated 32-byte secret.
type FileVault struct {
	Dir string
	Key []byte
	mu  sync.Mutex
}
type fileVaultEntry struct {
	Metadata   Metadata `json:"metadata"`
	Nonce      []byte   `json:"nonce"`
	Ciphertext []byte   `json:"ciphertext"`
}

func (v *FileVault) Create(ctx context.Context, r SecretRequest) (Metadata, error) {
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	entries, err := v.load()
	if err != nil {
		return Metadata{}, err
	}
	raw := make([]byte, 24)
	if _, err = io.ReadFull(rand.Reader, raw); err != nil {
		return Metadata{}, errors.New("unable to allocate provider reference")
	}
	id := hex.EncodeToString(raw)
	meta := Metadata{Provider: "local-file", OpaqueID: id, AccessReference: "local-file://" + id, Status: "active"}
	entry, err := v.seal(meta, r.Value)
	if err != nil {
		return Metadata{}, err
	}
	entries[id] = entry
	if err = v.save(entries); err != nil {
		return Metadata{}, err
	}
	return meta, nil
}
func (v *FileVault) Read(ctx context.Context, id string) (SecretResult, error) {
	if err := ctx.Err(); err != nil {
		return SecretResult{}, err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	entries, err := v.load()
	if err != nil {
		return SecretResult{}, err
	}
	entry, ok := entries[id]
	if !ok || entry.Metadata.Status == "disabled" {
		return SecretResult{}, errors.New("secret unavailable")
	}
	value, err := v.open(entry)
	if err != nil {
		return SecretResult{}, errors.New("secret unavailable")
	}
	return SecretResult{Metadata: entry.Metadata, Value: value}, nil
}
func (v *FileVault) Update(ctx context.Context, id string, r SecretRequest) (Metadata, error) {
	return v.replace(ctx, id, r, false)
}
func (v *FileVault) Rotate(ctx context.Context, id string, r SecretRequest) (Metadata, error) {
	return v.replace(ctx, id, r, true)
}
func (v *FileVault) replace(ctx context.Context, id string, r SecretRequest, rotate bool) (Metadata, error) {
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	entries, err := v.load()
	if err != nil {
		return Metadata{}, err
	}
	old, ok := entries[id]
	if !ok || old.Metadata.Status == "disabled" {
		return Metadata{}, errors.New("secret unavailable")
	}
	meta := old.Metadata
	if rotate {
		now := time.Now().UTC().Format(time.RFC3339)
		meta.RotationAt = now
		meta.Status = "rotated"
	}
	entry, err := v.seal(meta, r.Value)
	if err != nil {
		return Metadata{}, err
	}
	entries[id] = entry
	if err = v.save(entries); err != nil {
		return Metadata{}, err
	}
	return meta, nil
}
func (v *FileVault) Disable(ctx context.Context, id string) (Metadata, error) {
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	entries, err := v.load()
	if err != nil {
		return Metadata{}, err
	}
	entry, ok := entries[id]
	if !ok {
		return Metadata{}, errors.New("secret unavailable")
	}
	entry.Metadata.Status = "disabled"
	entry.Nonce = nil
	entry.Ciphertext = nil
	entries[id] = entry
	if err = v.save(entries); err != nil {
		return Metadata{}, err
	}
	return entry.Metadata, nil
}
func (v *FileVault) seal(meta Metadata, value []byte) (fileVaultEntry, error) {
	block, err := aes.NewCipher(v.Key)
	if err != nil {
		return fileVaultEntry{}, errors.New("vault encryption key must be 32 bytes")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return fileVaultEntry{}, errors.New("vault encryption unavailable")
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fileVaultEntry{}, errors.New("vault encryption unavailable")
	}
	return fileVaultEntry{Metadata: meta, Nonce: nonce, Ciphertext: aead.Seal(nil, nonce, value, []byte(meta.OpaqueID))}, nil
}
func (v *FileVault) open(entry fileVaultEntry) ([]byte, error) {
	block, err := aes.NewCipher(v.Key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, entry.Nonce, entry.Ciphertext, []byte(entry.Metadata.OpaqueID))
}
func (v *FileVault) load() (map[string]fileVaultEntry, error) {
	if len(v.Key) != 32 || strings.TrimSpace(v.Dir) == "" {
		return nil, errors.New("file vault configuration is invalid")
	}
	if err := os.MkdirAll(v.Dir, 0700); err != nil {
		return nil, errors.New("file vault storage unavailable")
	}
	info, err := os.Lstat(v.Dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("file vault directory is unsafe")
	}
	path := filepath.Join(v.Dir, "vault.json")
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if os.IsNotExist(err) {
		return map[string]fileVaultEntry{}, nil
	}
	if err != nil {
		return nil, errors.New("file vault storage unavailable")
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("file vault file permissions are unsafe")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.New("file vault storage unavailable")
	}
	var entries map[string]fileVaultEntry
	if json.Unmarshal(data, &entries) != nil || entries == nil {
		return nil, errors.New("file vault data is invalid")
	}
	return entries, nil
}
func (v *FileVault) save(entries map[string]fileVaultEntry) error {
	data, err := json.Marshal(entries)
	if err != nil {
		return errors.New("file vault serialization failed")
	}
	tmp, err := os.CreateTemp(v.Dir, ".vault-*.tmp")
	if err != nil {
		return errors.New("file vault storage unavailable")
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil || closeErr != nil {
		return errors.New("file vault storage unavailable")
	}
	if err = os.Rename(name, filepath.Join(v.Dir, "vault.json")); err != nil {
		return errors.New("file vault storage unavailable")
	}
	return nil
}
