export interface PresetSecretItem {
  username?: string
  password?: string
  notes?: string
}

/**
 * 生产安全标准：前端代码库中不维护明文密码字典。
 * 所有密码托管在后端的 PostgreSQL 加密数据库 (cmdb_local_secrets) 或上游 Bitwarden / FileVault 中，
 * 前端一律通过 `/api/credentials/secret` 接口安全读取。
 */
export const xascPresetSecrets: Record<string, PresetSecretItem> = {}

export function getPresetSecret(_idOrRef: string, _fallbackRef?: string): PresetSecretItem | undefined {
  return undefined
}
