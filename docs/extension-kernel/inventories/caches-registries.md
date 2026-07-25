# 缓存与 Registry 汇总

> 审计日期: 2026-07-25

## 一、Registry 汇总

| 名称 | 文件 | 存储 | 数据来源 | 写入者 | 读取者 |
|---|---|---|---|---|---|
| Extension Registry | `extension/registry.go` | 内存 `map[string]*registryItem` | DB (extensions) → Restore | UpsertDefinition, AgentSkill Import, Workshop Install, Package Install | Executor, Service, API, Chat |
| Plugin Registry | `extension/plugin_registry.go` | 内存 `map[string]*pluginEntry` | 硬编码 + DB | Register, Unregister | PluginManager |

## 二、缓存汇总

| 名称 | 结构 | 键 | 失效策略 | 持久化 | 崩溃恢复 |
|---|---|---|---|---|---|
| Agent Skill Artifact | `map[string]agentSkillArtifactCacheEntry` | scope+content hash | new AgentSkillService 时清空 | 否 | 从 DB 重新加载 |
| Agent Skill Catalog | `map[string][]AgentSkillCatalogEntry` | scope 派生 | Agent Skill 启用/禁用/删除时 invalidate | 否 | 重新构建 |
| Agent Skill Preview | `map[string]agentSkillPreviewState` | session id | 60 分钟后过期 | 否 | 丢失 |

## 三、连接状态汇总

| 名称 | 结构 | 持久化 |
|---|---|---|
| MCP Connection Map | `map[string]*client.Connection` + `sync.RWMutex` | 否（但连接状态通过 `mcp_servers.status` 持久化） |
| MCP Reconnecting Map | `map[string]bool` | 否 |

## 四、与数据库一致性分析

| 资源 | DB 状态 | 内存状态 | 一致性策略 | 漂移风险 |
|---|---|---|---|---|
| Extension Registry | `extensions` 表 | `Registry.items` | Restore 时以 DB 为准；运行时写入同时更新双方 | 低（写入时同步） |
| Plugin Registry | `extensions` 表（enabled） | `PluginRegistry.plugins` | 启动时注册内置插件；用户插件通过 DB 恢复 | 中（DB 已启用但 Registry 未注册） |
| Agent Skill Catalog | `extension_agent_skill_metadata` | `AgentSkillService.catalogs` | 启用/禁用时 invalidate | 中（需依赖 AgentSkillService 正确触发 invalidate） |
| MCP Connection Map | `mcp_servers.status` | `Manager.connections` | DB 为权威（`Manager.connect()` 更新 DB） | 低（Manager 负责同步） |
| MCP Discovery 缓存 | `mcp_tools/mcp_resources/mcp_prompts` | 无独立内存缓存 | DB 为权威 | 低（每次调用直接读 DB） |

## 五、缓存 vs 数据库：以谁为准

统一原则：**数据库为准，内存为加速副本**。

以下场景的权威来源：

| 场景 | 权威来源 | 说明 |
|---|---|---|
| 扩展是否启用 | `extensions.enabled` (DB) | Registry 在 Restore 和 SetEnabled 后同步 |
| Tool 是否可用 | `mcp_tools.enabled` (DB) | 每次调用 `Call` 前通过 Manager 检查 |
| MCP Server 是否连接 | `mcp_servers.status` (DB) | Manager.connect() 写入；内存 Connection Map 同步 |
| Agent Skill Artifact | `extension_artifacts.content_blob` (DB) | 内存缓存仅加速读取 |
