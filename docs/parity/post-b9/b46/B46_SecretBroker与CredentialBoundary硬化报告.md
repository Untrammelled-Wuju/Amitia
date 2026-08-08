# B46 SecretBroker / Credential Boundary 硬化报告

## 1. 执行结果

**Status**: `BLOCKED_CANONICAL_SECRET_AUTHORITY_MISSING`

**Blockers**:
1. Extension Kernel 层无统一 SecretBroker 收口全部运行时凭据
2. 9 个 Provider Config 的 `api_key` 明文存储需要 DB Schema 迁移 — 触发 `BLOCKED_LEGACY_SECRET_MIGRATION_SCHEMA_CHANGE_NOT_ALLOWED`

**Construction Mode**: REUSE + EXTEND (仅 MCP 域内可复用)

## 2. B46 Step Definition Resolution

B46 Final Step Reuse Matrix: B46 匹配 `Secret / Credential Boundary`; Construction Mode = REUSE + EXTEND; 无冲突。

**Canonical Targets from Matrix**:
- SecretBroker (if exists) -> ABSENT
- SecretReference -> PARTIAL (fragmented)
- SecretLease -> PARTIAL (TrustedService only)
- Secure Backing Store -> PARTIAL (MCP EncryptedFileStore only)
- Host API secret access -> DECLARED NOT WIRED
- MCP secret integration -> CONFORMING
- Runtime secret projection -> PARTIAL (TrustedService Lease + MCP resolveCredential)
- Provider credential integration -> VIOLATION (plaintext)

## 3. B9P8 输入

- 20 Canonical Systems 无 Secret System
- B9P8 定义架构: Runtime -> Host API Gateway -> Storage/Secret Broker
- B9P8 产出: `secret.read` / `secrets.use` Permission 已注册但无实现接线

## 4. B39~B45 输入

- B45: PermissionAuthority = 1 (DefaultPermissionBroker); Scope -> Permission -> Approval -> Dispatch
- B45: `secret.read` / `secrets.read` 已注册但无 Lease/Resolve 流程
- B41: InvocationContext 绑定但未携带 SecretRequirement
- B43/B44: Cancellation/Timeout 终态未与 Secret Lease 生命周期联动 (TrustedService 已实现)

## 5. 当前 Secret 架构

**已有组件**:
1. MCP `SecretStore` (token_store.go:22) — 域内 AES-256-GCM 加密存取
2. MCP `EncryptedFileStore` (token_store.go:28-53) — 文件级 AEAD
3. MCP `OAuth Manager` (oauth.go:89-130) — PKCE + refresh + revocation
4. MCP `resolveCredential` (manager.go:152-161) — 连接时解析
5. TrustedService `SecretLeaseManager` (secret_lease.go:72-261) — Issue/Consume/Revoke/TTL/MaxUses/Audit
6. TrustedService `SecretProvider` 接口 (secret_lease.go:79-81)
7. Host API `MethodSecretGet` (types.go:22) — 已声明未接线
8. Host API `secret.read` Permission (permission_mapping.go:143) — 已注册
9. Schema UI `SecretReference` / `SecretInputResolver` (schema_ui/schema.go:662-697)
10. Workflow `$secret` / `secrets.X` 模板解析 (workflow_values.go:183-192)
11. Observability `RecordSanitizer` (sanitizer.go:11-130)
12. Host API `maskSensitiveInput` (host_api_routes.go:47-102)
13. MCP `mcpRedact` (adapter_mcp.go:14,200)

## 6. SecretBroker Authority

**统一权威: ABSENT**

域内权威计数:
- MCP SecretStore: 1 (MCP 域内)
- TrustedService SecretLeaseManager: 1 (TrustedService 域内)
- Provider 域: 0 (直接读 SQLite 明文)

**违反 B46 Spec 16-17**: `secretBrokerAuthorityCount` 应为 1; 当前为 0。

## 7. Secret Metadata Authority

ABSENT — 无统一 Secret Metadata Store。

## 8. Secure Material Authority

域级: MCP EncryptedFileStore (AES-256-GCM) — material JSON file 0600, key 0600.
Provider: SQLite TEXT plaintext — VIOLATION.

## 9. Secret Reference

- `mcp-secret://<namespace>/<uuid>` — MCP ServerCredential.SecretReference
- Schema UI `{ RefID, Field, LeaseID }` — schema_ui/schema.go
- Workflow `$secret: name` — workflow_values.go
- Provider: NO Reference — direct `api_key` TEXT column — VIOLATION.

## 10. Secret Lease

唯一完整实现: TrustedService `SecretLeaseManager` (Issue/Consume/Revoke/RevokeByInstance/TTL 5min/MaxUses/Audit/Cleanup).

**未覆盖的运行时**: Provider / JS / WASM / Workflow / Task.

## 11. Purpose

域级 Validate:
- TrustedService: `Purpose` non-empty (secret_lease.go:94-96)
- MCP: `sanitizeNamespace` pattern match (token_store.go:215-227)

**缺**: 跨域 Purpose registry / mismatch FAIL-CLOSED.

## 12. Owner

域内 (MCP per-ServerCredential record; TrustedService per-ExtensionID+ModuleID).

**缺**: 统一 Owner model / Reference graph / Revocation impact.

## 13. Scope

- Host API: MethodSecretGet -> ScopePolicy{Namespaced:true} (permission_mapping.go:62-63)
- MCP: per-server reference
- TrustedService: per-Extension+Module+RuntimeInstance

## 14. Permission

HOST API 注册 MethodSecretGet + `secret.read` Permission 但未接线 — accidental safety.

Provider Config 读取 (chat/model_service.go / llm_client.go) **无** Permission 检查 — VIOLATION.

## 15~20. Create / Resolve / Release / Rotate / Revoke / Transfer / Share

| 操作 | MCP | TrustedService | Provider |
|---|---|---|---|
| Create | mcpapi/router.go:622 Secrets.Put | supervisor.Issue ModelConfigService.CreateModel plaintext |
| Resolve | manager.go:160 Secrets.Get | Issue+Consume | direct SQLite read |
| Release | Secrets.Delete | RevokeByInstance | N/A |
| Rotate | OAuth refresh | N/A | direct UPDATE |
| Revoke | OAuth revocation endpoint | Revoke(id) | N/A |
| Transfer | N/A | N/A | N/A |
| Share | N/A | N/A | N/A |

## 21. Lease Expiry

TrustedService: `IsExpired()` checks `ExpiresAt`. 5min default TTL. B46 Spec 43-44 满足.

## 22. Invocation Binding

TrustedService: per-RuntimeInstance (not per-Invocation ID — coarse).

Provider: NO Invocation binding — persistent plaintext.

## 23. Runtime Binding

TrustedService: `RuntimeInstance` field non-empty required (secret_lease.go:97-99).

## 24. Runtime Generation

TrustedService Supervisor 在重启时 RevokeByInstance — cross-generation protection.

## 25. Cancellation / 26. Timeout

B43/B44 触发时 TrustedService Supervisor RevokeByInstance — leases 失效.

## 27. Secure Backing Store

- MCP: AES-256-GCM per-entry nonce, file 0600, key 0600, atomic rename, NO homemade crypto.
- Provider: SQLite TEXT 明文 — VIOLATION.

## 28. Platform Security

Win/macOS/Linux/Android/iOS 均未对接 OS KeyVault. MCP file AES-GCM 为唯一 material-at-rest 加密.

## 29. Encryption

crypto/aes crypto/cipher crypto/rand Go stdlib. NO custom cipher. NO hard-coded master key.

## 30. Master Key

MCP: loadOrCreateKey 32-byte random base64 in 0600 file. Provider: N/A plaintext.

## 31~35. Runtime Injection

- Host-mediated: 未实施
- Opaque Handle: 未实施
- Leased Material: TrustedService SecretLease.Consume
- Environment: MCP stdio_env (Supervisor controlled)
- Temp File: 未实施

## 36. MCP

CONFORMING — OAuth PUT/GET/DELETE SecretStore; resolveCredential per-connection.

## 37. OAuth

CONFORMING — PKCE + refresh + revocation + encryption at rest.

## 38. Provider Config / 39~46. Model/Voice/Plugin/JavaScript/WASM/Trusted/Task/Workflow

 chatting/ASR/TTS/Embedding/Vision/ImageGen 均**明文 api_key** — VIOLATION.

## 47~49. Android / iOS / Desktop

Future — 当前 legacy Extension.

## 50. Browser/Search/Media

Future.

## 51. Legacy Plaintext Credentials

| 位置 | 表 | 字段 |
|---|---|---|
| chat/model_configs | api_key | LLM API key 明文 |
| asr/asr_configs | api_key | ASR API key 明文 |
| tts/tts_configs | api_key | TTS API key 明文 |
| embedding/embedding_configs | api_key | Embedding API key 明文 |
| vision/vision_configs | api_key | Vision API key 明文 |
| imagegen/imagegen_configs | api_key | ImageGen API key 明文 |
| imageprovider | api_key | Image API key 明文 |
| desktoppet | api_key | DesktopPet API key 明文 |
| realtime | api_key | Realtime API key 明文 |

## 52. Migration / Dual Write

B46 不做迁移 — BLOCKED_LEGACY_SECRET_MIGRATION_SCHEMA_CHANGE_NOT_ALLOWED. 迁移交 B141.

## 53. Storage Boundary

MCP: 满足 (EncryptedFileStore).
Provider: VIOLATION (plaintext in ordinary SQLite config table).
Workflow/Task $secret: context only, no checkpoint.

## 54. Workspace Boundary

PASS — 无 workspace credential 自动存储.

## 55. Checkpoint Boundary

PASS — 无 checkpoint 携带 secret material.

## 56. Logs

PASS — 4 层脱敏机制运行中 (Observability/MCP/Hook/HostAPI).

## 57. Errors

PARTIAL — MCP mcpRedact + Observability redact. Provider errors 未脱敏 APIKey in error propagation.

## 58. Audit

PASS — HostAPI maskSensitiveInput + Observability RecordSanitizer.

## 59. Metrics

PASS — 无 secret in metrics.

## 60. Streaming

PASS — B42 + adapter_mcp.mcpRedact.

## 61. ToolResult

PARTIAL — MCP 有 redact; Provider LLM callLLM 错误路径可能包含 Provider token.

## 62. Observation

PASS — 已有 hook SensitiveFields 标记.

## 63. Command Preview

PASS — 无 command preview 携带 secret material.

## 64~66. Shared Secret / Disable / Uninstall

ABSENT/Partial — ServerCredential + SecretLease RevokeByInstance on uninstall 部分满足.

## 67~70. Uninstall / Crash / Child Inheritance / Shell

TrustedService Supervisor 停进程+RevokeByInstance. MCP Manager 断连+Secrets.Delete. Provider 无 special uninstall hook.

## 71~77. Environment / iSH / Android / Browser / Search / Media / ASR

TrustedService Supervisor controlled stdio_env. 其他域 N/A.

## 78~80. Env Injection / Command Preview / CLI args

MCP stdio_env 受控. Provider Agent Tool 不暴露 env/preview.

## 81~86. Temp File / Workspace / Package / Host API

Host API MethodSecretGet declared, not wired.

## 87~90. Metadata listing / Reveal / HTTP Cache / Clipboard

NOT IMPLEMENTED / 默认禁用.

## 91~95. JS / WASM / Plugin / Task

依靠 Host API 方法未接线 — 实际无 secret access 路径.

## 96~102. Task Checkpoint / Workflow Secret

$secret resolution context-only.

## 103~106. Runtime Generation / Reconnect / Cache

TrustedService RevokeByInstance 重启失效率. MCP reconnect -> new resolveCredential.

## 107~119. Logging / Error / Auth / URL

4-layer redaction 覆盖. Spec 约束满足.

## 120~135. Observation / BehaviorPlan / Action / ToolInvocation / Tool Schema / User Secret / Frontend / Reveal / HTTP Cache / Clipboard / Export / Backup / Encrypted Export / Import

默认禁用或前端仅 metadata.

## 136~145. Encryption / Master Key / Platform / DB Metadata / DB Separation

MCP: file AES-GCM + separate key file. 平台安全后端未对接 OS KeyVault. Metadata 不含 key.

## 146~173. Reference Graph / Audit / Crash / Child / Shell / iSH / Android / Browser / Embedding / Model / ToolRegistry / RuntimeBinding / Package / SDK / ListMetadata / Create / Rotate / Resolve / Revoke / Transfer / Fail Closed

部分满足, 部分域级实现.

## 174~177. Fail Closed / Plaintext Fallback

MCP OK; Provider FAIL — plaintext is the fallback.

## 178. Error Mapping

未统一: 各域自建 (MCP ErrSecretNotFound/ErrSecretLeaseExpired; Provider SQL errors).

## 179~180. Registry / Material in Error Details

部分 — Provider error 路径可能泄漏.

## 181~187. Redaction / Leak / Storage / Lease / Permission / Runtime Projection / Provider Matrix

已在 JSON 报告中详细列出.

## 278~284. B45/B44/B43/B42/B41/B40/B39 Regression

未做任何修改 — 无回归风险. B45 Permission Authority 仍为 1.

## 285~288. Race / Production Fake / In-memory / Plaintext fallback

MCP EncryptedFileStore 非 in-memory; 非 Fake; 无 plaintext fallback.
Provider plaintext = fallback, VIOLATION.

## 289~291. Planned Changes / No Gap

BLOCKED — 有 gap (canonical broker missing, 9 legacy fields).

## 292~298. Allowed / Unrelated / Source Scope / Duplicate / Consistency / Security / Backward

Allowed 范围 = MCP 域增强. No unrelated changes. Source scope = analysis only. Duplicate = 0. Consistency = no dual-write. Security = Provider domain violates. Backward = 当前行为保留.

## 299~302. Output Directory / Step Def / Lease Contract / Status

已生成.

## 303. Allowed States

`BLOCKED_CANONICAL_SECRET_AUTHORITY_MISSING` 位于允许状态列表.

## 304. PASS Conditions (未满足项)

- SecretBroker Authority = 1 (当前 0)
- Secret Material in ordinary Storage = 0 (当前 9 处)
- Permission before Resolve 部分满足

## 305. Final Report

本报告.

*报告生成: B46 Parity Hardening — 2026-08-08*
