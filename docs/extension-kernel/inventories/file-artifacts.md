# 文件与 Artifact 清单

> 审计日期: 2026-07-25
> 关键发现: 扩展系统几乎不使用独立文件系统存储，数据主要存储在 SQLite 数据库和加密 JSON 文件中。

## 一、扩展系统使用的文件路径

### 1. MCP Secret Store

| 字段 | 内容 |
|---|---|
| 资源类型 | 加密 JSON 文件 |
| 路径生成 | `{dataDir}/mcp-secrets.json` |
| 密钥文件 | `{dataDir}/mcp-secrets.key` |
| 目录创建者 | `EncryptedFileStore.writeLocked()` → `os.MkdirAll` |
| dataDir 来源 | `config.AppCfg.Storage.DataDir` > SQLite DB 所在目录 > `./data` |
| 唯一所有者 | MCP Secret Store (`mcp/auth/token_store.go`) |
| 创建者 | `NewEncryptedFileStore()` |
| 读写者 | `Put()`, `Get()`, `Delete()` |
| 加密方式 | AES-256-GCM，随机 nonce，每 Secret 一份；文件整体 JSON，每条 value 为 base64(nonce+ciphertext) |
| 密钥生成 | `loadOrCreateKey()`: 首次运行生成 32 字节随机密钥，base64 写入 key 文件（权限 0600） |
| 作用域 | 全局 |
| 启动恢复 | Manager.Restore() 通过 Repository 读取 `secret_reference`，从该文件解密 |
| 禁用行为 | 不适用（全局 Secret Store 不因单个 Server 禁停用） |
| 卸载行为 | 无自动清理，Secret 删除需通过 `Delete()` 显式调用 |
| 敏感性 | 高敏感 |
| 审计要求 | 无（无操作审计） |
| 当前问题 | 1) 删除 MCP Server 后 `secret_reference` 仍指向文件中的 Secret，文件不自动清理；2) 备份/迁移时需同时携带 `.json` 和 `.key` |

### 2. Extension Config Crypto Key

| 字段 | 内容 |
|---|---|
| 资源类型 | 加密密钥文件 |
| 路径生成 | 优先 `AMITIA_EXTENSION_CONFIG_KEY` 环境变量，其次 `{dbPath}.extension-key` |
| dbPath | SQLite 数据库文件路径 |
| 唯一所有者 | Extension Config Crypto (`extension/config_crypto.go`) |
| 加密方式 | AES-GCM，密文前缀 `enc:v1:` |
| 用途 | 加密 `extension_configs.config_json` 和 `extension_states.state_json` |
| 敏感性 | 高敏感 |
| 当前问题 | 密钥文件与数据库可能不在同一备份中，迁移时需额外处理 |

### 3. Electron ConfigStore 文件

| 字段 | 内容 |
|---|---|
| 资源类型 | JSON 配置文件 |
| 路径 | `{AmitiaData}/config/deployment-config.json` |
| 所有者 | Electron `ConfigStore` (`desktop/src/main/config-store.ts`) |
| 用途 | 部署模式配置（local/remote） |
| 敏感性 | 低 |
| 扩展相关 | **不直接存储扩展数据**，但部署模式影响扩展安装来源 |

| 字段 | 内容 |
|---|---|
| 资源类型 | JSON 配置文件 |
| 路径 | `{AmitiaData}/config/auto-launch.json` |
| 所有者 | Electron `ConfigStore` |
| 用途 | 开机自启配置 |
| 敏感性 | 低 |
| 扩展相关 | 否 |

### 4. 前端 localStorage

| 字段 | 内容 |
|---|---|
| key | `uai-user-avatar` |
| 所有者 | `front/src/stores/app.ts` - `useAppStore` |
| 用途 | 用户自定义头像 URL |
| 敏感性 | 低 |
| 扩展相关 | 否 |

| 字段 | 内容 |
|---|---|
| key | `uai-default-char` |
| 所有者 | `front/src/views/extensions/api.ts` |
| 用途 | 扩展管理页面默认角色选择缓存 |
| 敏感性 | 低 |
| 扩展相关 | **是** - 扩展管理视图的角色选择状态 |

### 5. AmitiaData 运行时目录

| 目录 | 用途 | 扩展相关 |
|---|---|---|
| `AmitiaData/config/` | 部署配置、自动启动配置 | 否 |
| `AmitiaData/data/graph.db/` | BadgerDB 图数据库 | 否 |
| `AmitiaData/data/migration_backups/` | SurrealDB 迁移备份 | 否 |
| `AmitiaData/logs/` | 应用日志 | 间接（含扩展错误日志） |
| `AmitiaData/memory/` | 内存数据 | 否 |
| `AmitiaData/qdrant/` | Qdrant 向量数据库 | 否 |
| `AmitiaData/runtime/electron-appdata/` | Electron 应用数据 | 否 |
| `AmitiaData/runtime/electron-localappdata/` | Electron 本地应用数据 | 否 |
| `AmitiaData/runtime/electron-userdata/` | Electron 用户数据（含 Chromium 缓存） | 否 |
| `AmitiaData/surrealdb/` | SurrealDB 数据库（扩展配置存储于此） | **是** |
| `AmitiaData/uploads/` | 用户上传文件 | 否 |

---

## 二、Artifact 存储方案

### Agent Skill Artifact

| 字段 | 内容 |
|---|---|
| 存储方式 | `extension_artifacts` 表的 `content_blob` BLOB 字段 |
| 格式 | ZIP 压缩（`zip.Deflate`） |
| 内容 | Agent Skill 全套文件（SKILL.md + resources + references + scripts） |
| 编码 | `encodeAgentSkillArtifact()` → ZIP bytes → GORM BLOB |
| 解码 | `decodeAgentSkillArtifact()` → ZIP reader → `map[string][]byte` |
| 大小限制 | `DefaultAgentSkillLimits()` 控制 |
| 唯一所有者 | AgentSkillService |
| 创建者 | AgentSkillService.Import() |
| 读取者 | AgentSkillService.GetArtifact() 缓存 |
| 删除者 | AgentSkillService.Remove() → archived_at |
| 启动恢复 | AgentSkills.Restore() → 从 DB 读取 |

### Workshop Artifact

| 字段 | 内容 |
|---|---|
| 存储方式 | `extension_artifacts` 表的 JSON 文本字段 |
| 字段 | `manifest_json`, `workflow_json`, `schemas_json`, `compiled_workflow_json`, `tests_json`, `readme_text` |
| 格式 | JSON 文本（非 BLOB） |
| Checksum | `artifactChecksum()` 计算（字段拼接 SHA-256） |
| 唯一所有者 | WorkshopService |
| 创建者 | WorkshopInstaller.buildArtifact() |
| 读取者 | WorkshopInstaller.definitionFromArtifact(), WorkshopService.GetArtifact() |
| 删除者 | 归档 (`archived_at`) |

### Package Artifact

| 字段 | 内容 |
|---|---|
| 存储方式 | `extension_artifacts` 表 |
| 唯一所有者 | PackageService |
| 注 | 包的原始 `.amitiax` ZIP 在解析后不保留原始文件，解析后的内容存入数据库 |

---

## 三、不存在独立文件存储的子系统

以下子系统**不使用文件系统存储**，全部数据存储在 SQLite 数据库表中：

- Agent Skill 源码文件（ZIP 内容存为 BLOB）
- Plugin 代码（编译到 `server.exe` 或通过 Manifest 定义）
- Workshop 会话与草稿
- Package 版本与 Artifact
- Workflow 定义与编译结果
- 所有 MCP Server 配置（含 stdio 命令/参数/环境变量）
- 所有扩展配置（加密 JSON）和状态（加密 JSON）

---

## 四、已确认不存在的路径

以下在审计计划中假设的路径，在代码中**未找到**对应实现：

- `amitiaData/extensions/` - 不存在，扩展数据全部存数据库
- `amitiaData/plugins/` - 不存在
- `amitiaData/skills/` - 不存在
- `amitiaData/agent-skills/` - 不存在
- `amitiaData/mcp/` - 不存在独立目录（Secret 文件在 dataDir 根目录）
- `amitiaData/workflows/` - 不存在
- `amitiaData/packages/` - 不存在
- `amitiaData/artifacts/` - 不存在独立文件
- `amitiaData/cache/` - 不存在独立扩展缓存
- `amitiaData/secrets/` - 不存在独立目录（Secret 文件在 dataDir 根目录）
