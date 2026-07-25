# Secret 与凭据清单

> 审计日期: 2026-07-25

## 一、Secret 总览

当前扩展系统涉及 **3 类** Secret 存储机制：

| 机制 | 存储位置 | 加密方式 | 密钥来源 |
|---|---|---|---|
| Extension Config Crypto | `extension_configs.config_json` (DB) | AES-GCM | 环境变量 `AMITIA_EXTENSION_CONFIG_KEY` 或 `{dbPath}.extension-key` |
| Plugin State Crypto | `extension_states.state_json` (DB) | AES-GCM | 同上 |
| MCP Secret Store | `{dataDir}/mcp-secrets.json` (文件) | AES-256-GCM | `{dataDir}/mcp-secrets.key` |

---

## 二、Extension Config Crypto

| 字段 | 内容 |
|---|---|
| Secret 类型 | 扩展配置 JSON (`extension_configs.config_json`) |
| 存储位置 | SQLite 数据库 `extension_configs` 表 |
| 加密方式 | AES-GCM，密文前缀 `enc:v1:` |
| 密钥来源 | 优先 `AMITIA_EXTENSION_CONFIG_KEY` 环境变量；否则从 `{dbPath}.extension-key` 读取/生成 |
| 密钥生成 | `loadOrCreateCryptoKey()`：首次运行生成 32 字节随机密钥，base64 写入文件（权限 0600） |
| 读取者 | `GetConfig()`, Plugin Host 执行前 `resolveConfig()` |
| 写入者 | `SetConfig()` |
| 删除者 | `DeleteConfig()` |
| 轮换方式 | 无自动轮换；密钥更换后旧密文无法解密 |
| 日志脱敏 | 无显式脱敏（日志中不直接输出 config_json） |
| 导出时是否包含 | 导出为密文 |
| 卸载时是否保留 | 当前：删除扩展不删除配置（需确认） |
| 敏感性 | 高敏感 |
| 审计要求 | 无操作审计 |
| 当前问题 | 1) 禁用扩展后配置仍可读取；2) 无密钥轮换机制；3) 密钥与数据库分离存储，迁移/备份时可能遗漏 |
| 目标归属 | Secret Broker |

---

## 三、Plugin State Crypto

| 字段 | 内容 |
|---|---|
| Secret 类型 | 插件运行时状态 (`extension_states.state_json`) |
| 存储位置 | SQLite 数据库 `extension_states` 表 |
| 加密方式 | 与 Extension Config Crypto **相同** (AES-GCM, `enc:v1:` 前缀) |
| 密钥来源 | 与 Extension Config Crypto **共享同一密钥** |
| 读取者 | `GetPluginState()`, Plugin Host `requireCapability("storage.own.read")` |
| 写入者 | `UpsertPluginState()` (CAS) |
| 删除者 | 插件卸载时 |
| 敏感性 | 高敏感 |
| 当前问题 | 插件状态与扩展配置共享密钥，权限边界模糊 |
| 目标归属 | Secret Broker |

---

## 四、MCP Secret Store

| 字段 | 内容 |
|---|---|
| Secret 类型 | MCP 凭据（bearer_token, custom_headers, stdio_env, OAuth code_verifier） |
| 存储位置 | `{dataDir}/mcp-secrets.json`（单 JSON 文件，map[string]string） |
| 文件权限 | 600（通过 `os.CreateTemp` + `Chmod`） |
| 加密方式 | AES-256-GCM，每条 value 为 `base64(nonce + ciphertext)`，AAD 为 `secret_reference` |
| 密钥来源 | `{dataDir}/mcp-secrets.key`（32 字节，base64，权限 0600） |
| secret_reference 格式 | `mcp-secret://{namespace}/{uuid}` |
| 读取者 | `DefaultFactory.resolveCredential()` → HTTP headers / stdio env 注入 |
| 写入者 | `EncryptedFileStore.Put()` → `mcp_server_credentials.secret_reference` |
| 删除者 | `EncryptedFileStore.Delete()` |
| 轮换方式 | 无自动轮换 |
| 日志脱敏 | 无（`resolveCredential` 返回原始 bytes 用于 HTTP header 注入，不经过日志） |
| 导出时是否包含 | 不导出 |
| 卸载时是否保留 | **保留**（删除 MCP Server 不删除 Secret Store 中的凭据） |
| 敏感性 | 高敏感 |
| 审计要求 | 无操作审计 |
| 当前问题 | 1) 删除 MCP Server 后 Secret 不清理；2) `secret_reference` 断链后 Secret 成为孤儿数据；3) 无命名空间隔离（所有 Server 共享同一文件）；4) 备份/迁移需 `.json` + `.key` 同时携带 |
| 目标归属 | Secret Broker |

---

## 五、OAuth Token（运行时）

| 字段 | 内容 |
|---|---|
| Secret 类型 | MCP OAuth Access Token / Refresh Token |
| 存储位置 | 不持久化（仅内存） |
| 读取者 | `OAuthTokens.AccessToken()` |
| 写入者 | `OAuthManager` OAuth 流程 |
| 敏感性 | 高敏感 |
| 当前问题 | 应用重启后 OAuth Token 丢失，需重新授权 |
| 目标归属 | Secret Broker（可选持久化） |

---

## 六、Secret 引用链

```text
MCP Server 创建时设置凭据
  → mcp_server_credentials.secret_reference = "mcp-secret://{type}/{uuid}"
  → EncryptedFileStore.Put() → mcp-secrets.json 写入加密值
  → Manager.connect() → Factory.resolveCredential()
    → Repository.CredentialReference() → 读取 secret_reference
    → EncryptedFileStore.Get() → 解密返回明文
    → 注入 HTTP header 或 stdio env

删除 MCP Server
  → DeleteServer() 级联删除 mcp_server_credentials
  → **不删除** mcp-secrets.json 中的对应条目 ← 孤儿 Secret
```

---

## 七、Secret 生命周期汇总

| Secret 类型 | 创建 | 加密 | 读取 | 轮换 | 删除 | 导出 | 卸载保留 |
|---|---|---|---|---|---|---|---|
| Extension Config | SetConfig | AES-GCM | GetConfig / Plugin Host | 无 | DeleteConfig | 密文 | 是 |
| Plugin State | UpsertPluginState | AES-GCM | GetPluginState / Plugin Host | 无 | 插件卸载 | 密文 | 否 |
| MCP Credential | API 创建 | AES-256-GCM (文件) | Factory.resolveCredential | 无 | 无自动清理 | 不导出 | 是 |
| OAuth Token | OAuth 流程 | 不持久化 | OAuthManager | Token 过期重刷 | 不持久化 | 不导出 | 不适用 |
