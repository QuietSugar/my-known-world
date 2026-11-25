package cmdb

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	_ "github.com/lib/pq"
)

var defaultMasterKey = sha256.Sum256([]byte("myknownworld-local-vault-master-key-2026"))

// PostgresSecretProvider 实现了基于 PostgreSQL 的 AES-256-GCM 本地加密秘密托管引擎。
// 它不依赖外部文件系统，完全在数据库内闭环管理本地密室。
type PostgresSecretProvider struct {
	DB  *sql.DB
	Key []byte
	mu  sync.RWMutex
}

func NewPostgresSecretProvider(db *sql.DB, key []byte) *PostgresSecretProvider {
	k := key
	if len(k) != 32 {
		k = defaultMasterKey[:]
	}
	return &PostgresSecretProvider{
		DB:  db,
		Key: k,
	}
}

func (p *PostgresSecretProvider) EnsureSchema(ctx context.Context) error {
	if p.DB == nil {
		return errors.New("postgres secret provider: db is not configured")
	}
	query := `
	CREATE TABLE IF NOT EXISTS cmdb_local_secrets (
		ref TEXT PRIMARY KEY,
		opaque_id TEXT NOT NULL,
		ciphertext BYTEA NOT NULL,
		nonce BYTEA NOT NULL,
		metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	`
	_, err := p.DB.ExecContext(ctx, query)
	return err
}

func (p *PostgresSecretProvider) Create(ctx context.Context, req SecretRequest) (Metadata, error) {
	if p.DB == nil {
		return Metadata{}, errors.New("database not available")
	}
	ref := strings.TrimSpace(req.Name)
	if ref == "" {
		return Metadata{}, errors.New("secret name/ref cannot be empty")
	}

	block, err := aes.NewCipher(p.Key)
	if err != nil {
		return Metadata{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Metadata{}, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Metadata{}, err
	}

	ciphertext := gcm.Seal(nil, nonce, req.Value, []byte(ref))
	opaqueBytes := sha256.Sum256([]byte(ref))
	opaqueID := hex.EncodeToString(opaqueBytes[:12])

	meta := Metadata{
		Provider:        "postgres-local",
		OpaqueID:        opaqueID,
		AccessReference: ref,
		Status:          "active",
	}

	metaJSON, _ := json.Marshal(meta)

	query := `
	INSERT INTO cmdb_local_secrets (ref, opaque_id, ciphertext, nonce, metadata, updated_at)
	VALUES ($1, $2, $3, $4, $5, now())
	ON CONFLICT (ref) DO UPDATE
	SET ciphertext = EXCLUDED.ciphertext,
		nonce = EXCLUDED.nonce,
		metadata = EXCLUDED.metadata,
		updated_at = now();
	`
	_, err = p.DB.ExecContext(ctx, query, ref, opaqueID, ciphertext, nonce, metaJSON)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to save secret to database: %w", err)
	}

	return meta, nil
}

func (p *PostgresSecretProvider) Read(ctx context.Context, ref string) (SecretResult, error) {
	if p.DB == nil {
		return SecretResult{}, errors.New("database not available")
	}
	cleanRef := strings.TrimSpace(ref)

	// 匹配候选键 (完整 ref, 去前缀 ref, vaultwarden:xasc/ 等)
	candidates := []string{
		cleanRef,
		strings.TrimPrefix(cleanRef, "credentials/xasc/"),
		strings.TrimPrefix(cleanRef, "vaultwarden:xasc/"),
		strings.TrimPrefix(cleanRef, "vaultwarden:"),
		"credentials/xasc/" + strings.TrimPrefix(cleanRef, "vaultwarden:xasc/"),
		"vaultwarden:xasc/" + strings.TrimPrefix(cleanRef, "credentials/xasc/"),
	}

	query := `SELECT ref, opaque_id, ciphertext, nonce, metadata FROM cmdb_local_secrets WHERE ref = ANY($1) LIMIT 1`
	row := p.DB.QueryRowContext(ctx, query, candidates)

	var actualRef, opaqueID string
	var ciphertext, nonce, metaBytes []byte
	err := row.Scan(&actualRef, &opaqueID, &ciphertext, &nonce, &metaBytes)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SecretResult{}, ErrSecretUnavailable
		}
		return SecretResult{}, err
	}

	block, err := aes.NewCipher(p.Key)
	if err != nil {
		return SecretResult{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return SecretResult{}, err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(actualRef))
	if err != nil {
		return SecretResult{}, fmt.Errorf("failed to decrypt secret: %w", err)
	}

	var meta Metadata
	_ = json.Unmarshal(metaBytes, &meta)
	meta.OpaqueID = opaqueID
	meta.AccessReference = actualRef

	return SecretResult{
		Metadata: meta,
		Value:    plaintext,
	}, nil
}

func (p *PostgresSecretProvider) Update(ctx context.Context, ref string, req SecretRequest) (Metadata, error) {
	req.Name = ref
	return p.Create(ctx, req)
}

func (p *PostgresSecretProvider) Rotate(ctx context.Context, ref string, req SecretRequest) (Metadata, error) {
	return p.Update(ctx, ref, req)
}

func (p *PostgresSecretProvider) Disable(ctx context.Context, ref string) (Metadata, error) {
	if p.DB == nil {
		return Metadata{}, errors.New("database not available")
	}
	_, err := p.DB.ExecContext(ctx, "DELETE FROM cmdb_local_secrets WHERE ref = $1", ref)
	return Metadata{Status: "disabled"}, err
}

func (p *PostgresSecretProvider) Delete(ctx context.Context, ref string) error {
	if p.DB == nil {
		return errors.New("database not available")
	}
	cleanRef := strings.TrimSpace(ref)
	candidates := []string{
		cleanRef,
		strings.TrimPrefix(cleanRef, "credentials/xasc/"),
		strings.TrimPrefix(cleanRef, "vaultwarden:xasc/"),
		"credentials/xasc/" + strings.TrimPrefix(cleanRef, "vaultwarden:xasc/"),
		"vaultwarden:xasc/" + strings.TrimPrefix(cleanRef, "credentials/xasc/"),
	}
	_, err := p.DB.ExecContext(ctx, "DELETE FROM cmdb_local_secrets WHERE ref = ANY($1)", candidates)
	return err
}

func (p *PostgresSecretProvider) Has(ctx context.Context, ref string) bool {
	res, err := p.Read(ctx, ref)
	return err == nil && len(res.Value) > 0
}

func (p *PostgresSecretProvider) ListLocalRefs(ctx context.Context) ([]string, error) {
	if p.DB == nil {
		return nil, errors.New("database not available")
	}
	rows, err := p.DB.QueryContext(ctx, "SELECT DISTINCT ref FROM cmdb_local_secrets WHERE ref LIKE 'credentials/%' ORDER BY ref")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err == nil {
			refs = append(refs, r)
		}
	}
	return refs, nil
}
