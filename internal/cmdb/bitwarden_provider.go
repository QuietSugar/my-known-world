package cmdb

import (
	"context"
	"errors"

	"github.com/QuietSugar/my-known-world/internal/bitwarden"
)

// BitwardenSecretProvider 适配 bitwarden.Store 为 cmdb.SecretProvider
type BitwardenSecretProvider struct {
	store *bitwarden.Store
}

// NewBitwardenProvider 构造 Provider
func NewBitwardenProvider(cfg bitwarden.Config) *BitwardenSecretProvider {
	return &BitwardenSecretProvider{
		store: bitwarden.NewStore(cfg),
	}
}

// Store 获取底层 Store
func (p *BitwardenSecretProvider) Store() *bitwarden.Store {
	return p.store
}

// Sync 执行全量同步
func (p *BitwardenSecretProvider) Sync(ctx context.Context) error {
	return p.store.Sync(ctx)
}

// Read 查询明文秘密
func (p *BitwardenSecretProvider) Read(ctx context.Context, ref string) (SecretResult, error) {
	item, ok := p.store.Lookup(ref)
	if !ok {
		return SecretResult{}, errors.New("credential secret not found for reference: " + ref)
	}
	return SecretResult{
		Metadata: Metadata{
			Provider:        "bitwarden",
			OpaqueID:        item.ID,
			AccessReference: ref,
			Status:          "active",
		},
		Value: []byte(item.Password),
	}, nil
}

func (p *BitwardenSecretProvider) Create(context.Context, SecretRequest) (Metadata, error) {
	return Metadata{}, errors.New("bitwarden provider is read-only")
}

func (p *BitwardenSecretProvider) Update(context.Context, string, SecretRequest) (Metadata, error) {
	return Metadata{}, errors.New("bitwarden provider is read-only")
}

func (p *BitwardenSecretProvider) Rotate(context.Context, string, SecretRequest) (Metadata, error) {
	return Metadata{}, errors.New("bitwarden provider is read-only")
}

func (p *BitwardenSecretProvider) Disable(context.Context, string) (Metadata, error) {
	return Metadata{}, errors.New("bitwarden provider is read-only")
}
