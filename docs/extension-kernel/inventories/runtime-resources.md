# 运行时资源清单

> 审计日期: 2026-07-25

## 一、内存 Registry

### 1. Extension Registry

| 字段 | 内容 |
|---|---|
| 资源类型 | 内存 Registry |
| 结构 | `extension/registry.go` - `Registry` |
| 内部结构 | `items map[string]*registryItem`（key=skillID）, `modelNames map[string]string`（modelName→skillID）, `sync.RWMutex` |
| 创建位置 | `NewRegistry()` → `NewRuntime()` |
| 写入位置 | `Register()`, `Unregister()`, `UpsertDefinition()`, AgentSkill Import, Workshop Install, Package Install |
| 读取位置 | `Get()`, `GetByModelName()`, `Available()`, `List()`, `Resolve()` |
| 失效位置 | `Unregister()` 移除 item |
| 关闭时清理 | 不适用（内存回收） |
| 重启后恢复 | `NewRuntime()` → `agentSkills.Restore()`, `workshop.Restore()`, `packages.Restore()` → 从数据库重新注册 |
| 与数据库状态不一致时 | Registry 为准（运行时可覆盖 DB 状态） |
| 当前问题 | Registry 更新和 DB 更新不是原子的，但无明显漂移 |
| 目标归属 | Tool Registry |

### 2. Plugin Registry

| 字段 | 内容 |
|---|---|
| 资源类型 | 内存 Registry |
| 结构 | `extension/plugin_registry.go` - `PluginRegistry` |
| 内部结构 | `plugins map[string]*pluginEntry` |
| 创建位置 | `NewPluginRegistry()` |
| 写入位置 | `Register()`, `Unregister()` |
| 读取位置 | `Get()`, `List()`, PluginManager |
| 关闭时清理 | 内存回收 |
| 重启后恢复 | 内置插件（diagnostic）在 NewRuntime 时注册；用户插件通过 DB 恢复 |
| 目标归属 | Extension Kernel Plugin Registry |

---

## 二、内存缓存

### 3. Agent Skill Artifact 缓存

| 字段 | 内容 |
|---|---|
| 资源类型 | 内存缓存 |
| 结构 | `agentSkillArtifactCacheEntry` （definition + report + files map） |
| 容器 | `AgentSkillService.artifacts map[string]agentSkillArtifactCacheEntry` |
| Key | 由 scope 和 content 派生的 hash |
| 创建位置 | `AgentSkillService` 懒加载，`getArtifact()` 首次从 DB 读取后缓存 |
| 失效位置 | `AgentSkillService` 重新创建时（new AgentSkillService） |
| 目标归属 | Agent Skill Catalog |

### 4. Agent Skill Catalog 缓存

| 字段 | 内容 |
|---|---|
| 资源类型 | 内存缓存 |
| 结构 | `AgentSkillService.catalogs map[string][]AgentSkillCatalogEntry` |
| Key | scope 派生 |
| 创建位置 | `ResolveCatalog()` 首次调用 |
| 失效位置 | Agent Skill 启用/禁用/删除时 `invalidate()` |
| 目标归属 | Agent Skill Catalog |

### 5. Agent Skill Preview 缓存

| 字段 | 内容 |
|---|---|
| 资源类型 | 内存缓存 |
| 结构 | `AgentSkillService.previews map[string]agentSkillPreviewState` |
| 用途 | 暂存用户上传 ZIP 的解析预览（Import 前确认） |
| 生命周期 | 临时，60 分钟过期 |
| 目标归属 | Agent Skill Catalog（可改为 session 存储） |

### 6. Agent Skill Round 状态

| 字段 | 内容 |
|---|---|
| 资源类型 | 内存运行时状态 |
| 结构 | `AgentSkillService.rounds map[string]*agentSkillRoundState` |
| 用途 | 追踪当前活跃的 Agent Skill 轮次 |
| 生命周期 | 单次对话轮次 |
| 目标归属 | Runtime Supervisor |

---

## 三、队列与 Channel

### 7. Plugin AfterReply 队列

| 字段 | 内容 |
|---|---|
| 资源类型 | Channel |
| 结构 | `PluginManager.afterReplyQ chan afterReplyTask` |
| 创建位置 | `NewPluginManager()` |
| 写入位置 | `DispatchAfterReply()` |
| 消费位置 | `afterReplyWorker` goroutine |
| 关闭 | `PluginManager.Stop()` → close channel |
| 目标归属 | Runtime Supervisor Event Bus |

### 8. Plugin Event 通道

| 字段 | 内容 |
|---|---|
| 资源类型 | Channel（2 个） |
| 结构 | `PluginManager.eventWake chan struct{}`（信号）, `eventIngress chan ExtensionEvent`（事件） |
| 创建位置 | `NewPluginManager()` |
| 写入位置 | `EmitSystemEvent()` → eventIngress |
| 消费位置 | `eventIngressWorker` 和 `eventWorker` goroutines |
| 关闭 | `PluginManager.Stop()` |
| 目标归属 | Runtime Supervisor Event Bus |

---

## 四、MCP 连接管理

### 9. MCP Connection Map

| 字段 | 内容 |
|---|---|
| 资源类型 | 内存连接映射 |
| 结构 | `Manager.connections map[string]*client.Connection` |
| 并发控制 | `sync.RWMutex` |
| 创建位置 | `New()` |
| 写入位置 | `connect()` 成功后 |
| 删除位置 | `Disconnect()`, `Close()` |
| 读取位置 | `Connection()`, `Call()` |
| 目标归属 | MCP Manager |

### 10. MCP Reconnecting Map

| 字段 | 内容 |
|---|---|
| 资源类型 | 内存状态映射 |
| 结构 | `Manager.reconnecting map[string]bool` |
| 用途 | 防止同一 Server 被并发的重连 goroutine 竞争 |
| 目标归属 | MCP Manager |

### 11. MCP Ready Handlers

| 字段 | 内容 |
|---|---|
| 资源类型 | 回调函数列表 |
| 结构 | `Manager.readyHandlers []func(context.Context, string)` |
| 注册位置 | `RegisterReadyHandler()` |
| 触发位置 | `connect()` 连接成功后 |
| 当前注册 | Discovery Service, Skill Runtime, Host Service |
| 目标归属 | MCP Manager |

---

## 六、连接与子进程（见 workers-processes.md）

详见 [workers-processes.md](workers-processes.md)。
