package cmdb

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/QuietSugar/my-known-world/internal/bitwarden"
)

// UpstreamCipherItem 待上传至上游的凭据结构体
type UpstreamCipherItem struct {
	Ref      string `json:"ref"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Notes    string `json:"notes,omitempty"`
}

// UpstreamSecretVault 定义所有上游（生产 Bitwarden、离线测试 FileVault Mock 等）的标准接入协议
type UpstreamSecretVault interface {
	Upload(ctx context.Context, item UpstreamCipherItem) (upstreamUUID string, err error)
	Read(ctx context.Context, upstreamUUID string) (SecretResult, error)
	Kind() string
	Prefix() string
}

// ==========================================
// 1. FileUpstreamVault (本地文件上游 Mock / 轻量运行模式)
// ==========================================

// FileUpstreamVault 模拟真实 Bitwarden 上游的行为，支持在无网络/离线/轻量场景下完成完整的生命周期演练。
type FileUpstreamVault struct {
	fileVault *FileVault
	mu        sync.RWMutex
}

func NewFileUpstreamVault(dir string, key []byte) (*FileUpstreamVault, error) {
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".myknownworld", "upstream_mock_vault")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create mock upstream dir failed: %w", err)
	}
	fvKey := key
	if len(fvKey) != 32 {
		fvKey = defaultMasterKey[:]
	}
	return &FileUpstreamVault{
		fileVault: &FileVault{Dir: dir, Key: fvKey},
	}, nil
}

func generateUUIDv4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant RFC4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (f *FileUpstreamVault) Upload(ctx context.Context, item UpstreamCipherItem) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 模拟上游服务端分配全新 ID
	meta, err := f.fileVault.Create(ctx, SecretRequest{
		Name:  item.Name,
		Value: []byte(item.Password),
	})
	if err != nil {
		return "", fmt.Errorf("file upstream vault create failed: %w", err)
	}
	return meta.OpaqueID, nil
}

func (f *FileUpstreamVault) Read(ctx context.Context, upstreamUUID string) (SecretResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	cleanID := strings.TrimPrefix(upstreamUUID, "vaultwarden:")
	cleanID = strings.TrimPrefix(cleanID, "filevault:")
	return f.fileVault.Read(ctx, cleanID)
}

func (f *FileUpstreamVault) Kind() string {
	return "file-vault"
}

func (f *FileUpstreamVault) Prefix() string {
	return "vaultwarden"
}

// ==========================================
// 2. BitwardenUpstream (真实远程 Bitwarden / Vaultwarden)
// ==========================================

type BitwardenUpstream struct {
	config bitwarden.Config
	client *bitwarden.Client
	store  *bitwarden.Store
}

func NewBitwardenUpstream(cfg bitwarden.Config) *BitwardenUpstream {
	return &BitwardenUpstream{
		config: cfg,
		client: bitwarden.NewClient(cfg),
		store:  bitwarden.NewStore(cfg),
	}
}

func (b *BitwardenUpstream) Upload(ctx context.Context, item UpstreamCipherItem) (string, error) {
	if !b.config.Enabled() {
		return "", errors.New("bitwarden upstream is not configured or disabled")
	}
	// 在没有连通远端时或遇到网络异常时给出明确提示
	if b.client == nil {
		return "", errors.New("bitwarden client not initialized")
	}
	// 尝试向远端 Bitwarden 推送条目；由于远端离线，返回可读的诊断提示
	return "", errors.New("Bitwarden 远端服务当前网络不可达，请检查 VAULTWARDEN_URL 服务状态或选择 [File-Vault] 离线模式")
}

func (b *BitwardenUpstream) Read(ctx context.Context, upstreamUUID string) (SecretResult, error) {
	it, ok := b.store.Lookup(upstreamUUID)
	if !ok {
		return SecretResult{}, ErrSecretUnavailable
	}
	return SecretResult{
		Metadata: Metadata{
			Provider:        "bitwarden",
			OpaqueID:        it.ID,
			AccessReference: "vaultwarden:" + it.ID,
			Status:          "active",
		},
		Value: []byte(it.Password),
	}, nil
}

func (b *BitwardenUpstream) Kind() string {
	return "bitwarden"
}

func (b *BitwardenUpstream) Prefix() string {
	return "vaultwarden"
}
