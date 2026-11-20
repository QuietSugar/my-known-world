package cmdb

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
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

const (
	fileVaultProvider   = "local-file"
	fileVaultDataFile   = "vault.json"
	fileVaultLockFile   = "vault.lock"
	fileVaultKeyByteLen = 32
)

// FileVault is the first-party encrypted provider. It keeps one AES-256-GCM
// encrypted store in a private directory, owns its own key, and serializes
// mutations with both an in-process mutex and an advisory file lock so that
// several processes on the same host cannot interleave writes.
//
// It is a single-host provider: sharing the directory over a network filesystem
// is unsupported because advisory locks and atomic rename are not reliable
// there. Metadata is stored as plaintext JSON; secret values are never stored
// unencrypted. Key must be a dedicated 32-byte secret and is never written to
// the store directory.
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

// ErrSecretUnavailable reports that a reference is unknown, disabled, or no
// longer decryptable. It intentionally carries no provider detail.
var ErrSecretUnavailable = errors.New("secret unavailable")

func (v *FileVault) Create(ctx context.Context, r SecretRequest) (Metadata, error) {
	var meta Metadata
	err := v.withEntries(ctx, true, func(entries map[string]fileVaultEntry) error {
		raw := make([]byte, 24)
		if _, err := io.ReadFull(rand.Reader, raw); err != nil {
			return errors.New("unable to allocate provider reference")
		}
		id := hex.EncodeToString(raw)
		meta = Metadata{Provider: fileVaultProvider, OpaqueID: id, AccessReference: "local-file://" + id, Status: "active"}
		entry, err := v.seal(v.Key, meta, r.Value)
		if err != nil {
			return err
		}
		entries[id] = entry
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}
	return meta, nil
}

func (v *FileVault) Read(ctx context.Context, id string) (SecretResult, error) {
	var result SecretResult
	err := v.withEntries(ctx, false, func(entries map[string]fileVaultEntry) error {
		entry, ok := entries[id]
		if !ok || entry.Metadata.Status == "disabled" || len(entry.Ciphertext) == 0 {
			return ErrSecretUnavailable
		}
		value, err := v.open(v.Key, entry)
		if err != nil {
			// Fail closed and keep provider detail out of the caller-visible error.
			return ErrSecretUnavailable
		}
		result = SecretResult{Metadata: entry.Metadata, Value: value}
		return nil
	})
	if err != nil {
		return SecretResult{}, err
	}
	return result, nil
}

func (v *FileVault) Update(ctx context.Context, id string, r SecretRequest) (Metadata, error) {
	return v.replace(ctx, id, r, false)
}

func (v *FileVault) Rotate(ctx context.Context, id string, r SecretRequest) (Metadata, error) {
	return v.replace(ctx, id, r, true)
}

func (v *FileVault) replace(ctx context.Context, id string, r SecretRequest, rotate bool) (Metadata, error) {
	var meta Metadata
	err := v.withEntries(ctx, true, func(entries map[string]fileVaultEntry) error {
		old, ok := entries[id]
		if !ok || old.Metadata.Status == "disabled" {
			return ErrSecretUnavailable
		}
		meta = old.Metadata
		if rotate {
			meta.RotationAt = time.Now().UTC().Format(time.RFC3339)
			meta.Status = "rotated"
		}
		entry, err := v.seal(v.Key, meta, r.Value)
		if err != nil {
			return err
		}
		entries[id] = entry
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}
	return meta, nil
}

func (v *FileVault) Disable(ctx context.Context, id string) (Metadata, error) {
	var meta Metadata
	err := v.withEntries(ctx, true, func(entries map[string]fileVaultEntry) error {
		entry, ok := entries[id]
		if !ok {
			return ErrSecretUnavailable
		}
		meta = entry.Metadata
		meta.Status = "disabled"
		// Revocation destroys the ciphertext; the tombstone keeps the reference
		// resolvable as disabled instead of silently unknown.
		entries[id] = fileVaultEntry{Metadata: meta}
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}
	return meta, nil
}

// Rekey re-encrypts every stored value with newKey and swaps the in-memory key
// only after the new store has been written durably. Disabled tombstones are
// preserved without ciphertext.
func (v *FileVault) Rekey(ctx context.Context, newKey []byte) error {
	if len(newKey) != fileVaultKeyByteLen {
		return errors.New("vault encryption key must be 32 bytes")
	}
	if len(v.Key) == fileVaultKeyByteLen && subtle.ConstantTimeCompare(v.Key, newKey) == 1 {
		return errors.New("new vault key must differ from the current key")
	}
	err := v.withEntries(ctx, true, func(entries map[string]fileVaultEntry) error {
		for id, entry := range entries {
			if len(entry.Ciphertext) == 0 {
				continue
			}
			value, err := v.open(v.Key, entry)
			if err != nil {
				return errors.New("vault rekey aborted: an existing record cannot be decrypted")
			}
			resealed, err := v.seal(newKey, entry.Metadata, value)
			zeroBytes(value)
			if err != nil {
				return err
			}
			entries[id] = resealed
		}
		return nil
	})
	if err != nil {
		return err
	}
	v.Key = append([]byte(nil), newKey...)
	return nil
}

func (v *FileVault) withEntries(ctx context.Context, write bool, fn func(map[string]fileVaultEntry) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(v.Key) != fileVaultKeyByteLen {
		return errors.New("vault encryption key must be 32 bytes")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.prepareDir(); err != nil {
		return err
	}
	unlock, err := v.acquireFileLock()
	if err != nil {
		return err
	}
	defer unlock()
	entries, err := v.load()
	if err != nil {
		return err
	}
	if err := fn(entries); err != nil {
		return err
	}
	if !write {
		return nil
	}
	return v.save(entries)
}

// prepareDir creates the store directory with owner-only permissions and
// refuses to continue when an existing path is not a plain, private directory.
func (v *FileVault) prepareDir() error {
	if strings.TrimSpace(v.Dir) == "" {
		return errors.New("file vault configuration is invalid")
	}
	if err := os.MkdirAll(v.Dir, 0700); err != nil {
		return errors.New("file vault storage unavailable")
	}
	if err := checkPrivatePath(v.Dir); err != nil {
		// An operator-created directory may be world-readable; tighten it rather
		// than storing secrets in it. Symlinks and non-directories stay fatal.
		if info, statErr := os.Lstat(v.Dir); statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return err
		}
		if chmodErr := os.Chmod(v.Dir, 0700); chmodErr != nil {
			return err
		}
		return checkPrivatePath(v.Dir)
	}
	return nil
}

// acquireFileLock takes an exclusive advisory lock so that only one process at
// a time can read-modify-write the store. The lock file itself contains no data.
func (v *FileVault) acquireFileLock() (func(), error) {
	path := filepath.Join(v.Dir, fileVaultLockFile)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, errors.New("file vault storage unavailable")
	}
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		file.Close()
		return nil, errors.New("file vault lock file permissions are unsafe")
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, errors.New("file vault lock unavailable")
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

func (v *FileVault) seal(key []byte, meta Metadata, value []byte) (fileVaultEntry, error) {
	aead, err := vaultAEAD(key)
	if err != nil {
		return fileVaultEntry{}, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fileVaultEntry{}, errors.New("vault encryption unavailable")
	}
	return fileVaultEntry{Metadata: meta, Nonce: nonce, Ciphertext: aead.Seal(nil, nonce, value, []byte(meta.OpaqueID))}, nil
}

func (v *FileVault) open(key []byte, entry fileVaultEntry) ([]byte, error) {
	aead, err := vaultAEAD(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, entry.Nonce, entry.Ciphertext, []byte(entry.Metadata.OpaqueID))
}

func vaultAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("vault encryption key must be 32 bytes")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("vault encryption unavailable")
	}
	return aead, nil
}

func (v *FileVault) load() (map[string]fileVaultEntry, error) {
	if len(v.Key) != fileVaultKeyByteLen || strings.TrimSpace(v.Dir) == "" {
		return nil, errors.New("file vault configuration is invalid")
	}
	if err := checkPrivatePath(v.Dir); err != nil {
		return nil, err
	}
	path := filepath.Join(v.Dir, fileVaultDataFile)
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if os.IsNotExist(err) {
		return map[string]fileVaultEntry{}, nil
	}
	if err != nil {
		return nil, errors.New("file vault storage unavailable")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("file vault file permissions are unsafe")
	}
	data, err := io.ReadAll(io.LimitReader(file, 32<<20))
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
	if err = os.Rename(name, filepath.Join(v.Dir, fileVaultDataFile)); err != nil {
		return errors.New("file vault storage unavailable")
	}
	return nil
}

func checkPrivatePath(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return errors.New("file vault directory is unsafe")
	}
	return nil
}

// LoadVaultKeyFile reads a dedicated key file. The file must be a regular,
// owner-only file containing 32 raw bytes or a Base64/hex encoding of them.
func LoadVaultKeyFile(path string) ([]byte, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, errors.New("vault key file is unavailable")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("vault key file must be a regular file readable only by its owner")
	}
	if info.Size() <= 0 || info.Size() > 4096 {
		return nil, errors.New("vault key file has an invalid size")
	}
	data, err := io.ReadAll(io.LimitReader(file, 4096))
	if err != nil {
		return nil, errors.New("vault key file is unavailable")
	}
	return ParseVaultKey(data)
}

// ParseVaultKey accepts 32 raw key bytes or a Base64/hex encoding of them.
func ParseVaultKey(raw []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(raw)
	if trimmed == nil {
		return nil, errors.New("vault key is empty")
	}
	if len(trimmed) == fileVaultKeyByteLen {
		return append([]byte(nil), trimmed...), nil
	}
	text := string(trimmed)
	if decoded, err := base64.StdEncoding.DecodeString(text); err == nil && len(decoded) == fileVaultKeyByteLen {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(text); err == nil && len(decoded) == fileVaultKeyByteLen {
		return decoded, nil
	}
	return nil, errors.New("vault key must be 32 raw bytes or a Base64/hex encoding of them")
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
