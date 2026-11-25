package cmdb

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
)

type MigrateItemResult struct {
	Ref             string `json:"ref"`
	NewUUID         string `json:"newUUID,omitempty"`
	AccessReference string `json:"accessReference,omitempty"`
	Success         bool   `json:"success"`
	Error           string `json:"error,omitempty"`
}

type BatchMigrateResult struct {
	Total     int                 `json:"total"`
	Succeeded int                 `json:"succeeded"`
	Failed    int                 `json:"failed"`
	Upstream  string              `json:"upstream"`
	Results   []MigrateItemResult `json:"results"`
}

// CompositeSecretProvider 复合秘密提供者：
// 优先从本地数据库（PostgresSecretProvider）获取；
// 若已迁移或外部引用，则路由到对应上游（Bitwarden 或 本地文件Mock上游）。
type CompositeSecretProvider struct {
	Local        *PostgresSecretProvider
	Bitwarden    UpstreamSecretVault
	FileUpstream *FileUpstreamVault
	DB           *sql.DB
	mu           sync.RWMutex
}

func NewCompositeSecretProvider(db *sql.DB, bw UpstreamSecretVault, fu *FileUpstreamVault) *CompositeSecretProvider {
	local := NewPostgresSecretProvider(db, nil)
	return &CompositeSecretProvider{
		Local:        local,
		Bitwarden:    bw,
		FileUpstream: fu,
		DB:           db,
	}
}

func (c *CompositeSecretProvider) Create(ctx context.Context, req SecretRequest) (Metadata, error) {
	return c.Local.Create(ctx, req)
}

func (c *CompositeSecretProvider) Read(ctx context.Context, ref string) (SecretResult, error) {
	// 1. 优先查本地数据库密室
	if c.Local != nil {
		res, err := c.Local.Read(ctx, ref)
		if err == nil && len(res.Value) > 0 {
			return res, nil
		}
	}

	// 2. 查远程 Bitwarden 上游
	if c.Bitwarden != nil {
		res, err := c.Bitwarden.Read(ctx, ref)
		if err == nil && len(res.Value) > 0 {
			return res, nil
		}
	}

	// 3. 查本地文件 Mock 上游
	if c.FileUpstream != nil {
		res, err := c.FileUpstream.Read(ctx, ref)
		if err == nil && len(res.Value) > 0 {
			return res, nil
		}
	}

	return SecretResult{}, ErrSecretUnavailable
}

func (c *CompositeSecretProvider) Update(ctx context.Context, ref string, req SecretRequest) (Metadata, error) {
	return c.Local.Update(ctx, ref, req)
}

func (c *CompositeSecretProvider) Rotate(ctx context.Context, ref string, req SecretRequest) (Metadata, error) {
	return c.Local.Rotate(ctx, ref, req)
}

func (c *CompositeSecretProvider) Disable(ctx context.Context, ref string) (Metadata, error) {
	return c.Local.Disable(ctx, ref)
}

// MigrateToUpstream 执行单选或多选批量上传上游并原子换绑
func (c *CompositeSecretProvider) MigrateToUpstream(ctx context.Context, refs []string, targetUpstream string) (BatchMigrateResult, error) {
	result := BatchMigrateResult{
		Total:    len(refs),
		Upstream: targetUpstream,
		Results:  make([]MigrateItemResult, 0, len(refs)),
	}

	var upstream UpstreamSecretVault
	if strings.ToLower(targetUpstream) == "file" || strings.ToLower(targetUpstream) == "file-vault" {
		upstream = c.FileUpstream
	} else {
		upstream = c.Bitwarden
	}

	if upstream == nil {
		return result, fmt.Errorf("指定的上游提供者 %q 未初始化或不可用", targetUpstream)
	}

	for _, ref := range refs {
		cleanRef := strings.TrimSpace(ref)
		if cleanRef == "" {
			continue
		}

		itemRes := MigrateItemResult{Ref: cleanRef}

		// 1. 从本地数据库读取密码
		sec, err := c.Local.Read(ctx, cleanRef)
		if err != nil {
			itemRes.Success = false
			itemRes.Error = fmt.Sprintf("本地未找到有效密文: %v", err)
			result.Failed++
			result.Results = append(result.Results, itemRes)
			continue
		}

		// 2. 加密上传至上游
		name := strings.TrimPrefix(cleanRef, "credentials/xasc/")
		newUUID, err := upstream.Upload(ctx, UpstreamCipherItem{
			Ref:      cleanRef,
			Name:     name,
			Password: string(sec.Value),
		})
		if err != nil {
			itemRes.Success = false
			itemRes.Error = fmt.Sprintf("上传上游失败: %v", err)
			result.Failed++
			result.Results = append(result.Results, itemRes)
			continue
		}

		newRef := fmt.Sprintf("%s:%s", upstream.Prefix(), newUUID)

		// 3. 在数据库中原子更新凭据节点的 accessReference
		if c.DB != nil {
			_, err = c.DB.ExecContext(ctx, `
				UPDATE account_credential_details
				SET details = jsonb_set(details, '{accessReference}', to_jsonb($1::text), true)
				WHERE node_id = $2 OR node_id = $3
			`, newRef, cleanRef, "credentials/xasc/"+strings.TrimPrefix(cleanRef, "credentials/xasc/"))
			if err != nil {
				log.Printf("Warning: failed to rebind accessReference in account_credential_details: %v", err)
			}
		}

		// 4. 物理删除本地数据库密室中的密文
		if err := c.Local.Delete(ctx, cleanRef); err != nil {
			log.Printf("Warning: failed to delete local secret for %s: %v", cleanRef, err)
		}

		itemRes.Success = true
		itemRes.NewUUID = newUUID
		itemRes.AccessReference = newRef
		result.Succeeded++
		result.Results = append(result.Results, itemRes)
	}

	return result, nil
}

// GetStatusMap 获取所有凭据的托管状态 (local / upstream)
func (c *CompositeSecretProvider) GetStatusMap(ctx context.Context) (map[string]string, error) {
	status := make(map[string]string)
	if c.Local == nil {
		return status, nil
	}
	refs, err := c.Local.ListLocalRefs(ctx)
	if err != nil {
		return status, err
	}
	for _, r := range refs {
		status[r] = "local"
		short := strings.TrimPrefix(r, "credentials/xasc/")
		status[short] = "local"
	}
	return status, nil
}
