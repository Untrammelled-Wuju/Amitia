# 阶段5：切换唯一 Tool/MCP/Legacy 主链 — 实施计划

## Context

当前 Chat 服务通过 `*extension.Runtime`（旧 Runtime）获取和执行工具。旧 Runtime 在 `NewRuntime` 中调用 `pluginManager.Start(ctx)`，通过旧 Registry 注册 Legacy Tools 和 MCP 工具。Kernel 已有完整的 `ExecutionPipeline`、`capability.ToolRegistry`、`ContributionRegistry` 等基础设施，但 `ExecutionPipeline.ToolResolver` 未设置，`ToolRegistry` 未加入 Container，未连接到 Chat。

阶段5目标：建立 Kernel Tool Facade 新链路，让 Chat 通过接口依赖而非具体类型使用工具，MCP 工具通过 Kernel 注册，最终禁用 PluginManager.Start，新增诊断计数器。

## 关键约束

1. **循环依赖**：`extension` 包 import `kernel` 包（`Kernel *kernelruntime.Runtime`），所以 `kernel` 包**不能** import `extension` 包。ToolFacade 不能直接持有 `*extension.Runtime`，必须通过接口解耦。
2. **渐进式迁移**：方案文档要求"先建立新链真实可用证据，再切断旧链"。过渡期保留旧 Runtime 作为回退。
3. **Chat 6 个调用点**：`PrepareAgentSkillPrompt`、`EndAgentSkillRound`、`BeforePrompt`、`ModelTools`、`ExecuteModelTool`、`AfterReply` 都需要覆盖。

## 实施步骤

### 步骤 1：定义 Chat 接口（解耦 Chat 与 extension）

**新建** `backend/internal/chat/tool_runtime.go`

定义 chat 包自己的接口和类型，不 import extension：
- `ModelToolRuntime` 接口：包含上述 6 个方法
- `SkillScope` 结构体：替代 `extension.ExecutionScope`
- `ToolResult` 结构体：替代 `extension.SkillResult`
- `ContextContribution` 结构体：替代 `extension.ContextContribution`
- `ReplyView` 结构体：替代 `extension.ReplyView`
- `ActivatedSkill` 结构体：替代 `extension.ActivatedAgentSkill`
- `ToolError` 结构体：替代 `extension.ExtensionError`

**修改** `backend/internal/chat/service.go`
- 新增 `toolRuntime ModelToolRuntime` 字段
- 新增 `SetToolRuntime(ModelToolRuntime)` 方法
- 保留 `skillRuntime *extension.Runtime` 作为过渡期回退

**修改** `backend/internal/chat/compute.go`、`message_llm.go`、`message_pipeline.go`
- 6 个调用点改为：先调用 `toolRuntime`（若非 nil），nil 时回退 `skillRuntime`

### 步骤 2：定义 Legacy 适配接口（解决循环依赖）

**新建** `backend/internal/extension/kernel/legacy_runtime_iface.go`

在 kernel 包中定义接口，避免 import extension：
```go
type LegacyToolDispatcher interface {
    PrepareAgentSkillPrompt(ctx, scope LegacyScope, message string) (string, []LegacyActivatedSkill, []string)
    EndAgentSkillRound(scope LegacyScope)
    BeforePrompt(ctx, scope LegacyScope) []LegacyContextContribution
    ModelTools(ctx, scope LegacyScope) ([]tool.Tool, error)
    ExecuteModelTool(ctx, modelName, input, scope, idempotencyKey) (LegacyToolResult, bool)
    AfterReply(scope, reply LegacyReplyView) bool
}
```
或更简单地使用函数类型回调，避免定义过多中间类型。

### 步骤 3：扩展 Container

**修改** `backend/internal/extension/kernel/container.go`
- Container 新增字段：`ToolRegistry *capability.ToolRegistry`、`AdapterRegistry *capability.RuntimeAdapterRegistry`

**修改** `backend/internal/extension/kernel/container_builder.go`
- 创建 `ToolRegistry` 和 `AdapterRegistry` 实例
- 设置 `executionKernel.ToolResolver`（初始为从 ToolRegistry 解析）
- 将 `ToolRegistry` 和 `AdapterRegistry` 加入 Container

### 步骤 4：实现 Kernel Tool Facade

**新建** `backend/internal/extension/kernel/tool_facade.go`

`ToolFacade` 实现 `chat.ModelToolRuntime` 接口（通过 duck typing，不需要显式 import chat 包中的接口——Go 的隐式接口实现）：
- `ModelTools`：从 `capability.ToolRegistry` 获取工具，通过 `ToolDefinition.ModelToolView()` 转换为 `tool.Tool`，按 Permission/Scope/State 过滤
- `ExecuteModelTool`：通过 `ExecutionPipeline.Execute` 执行
- `BeforePrompt`/`AfterReply`：通过 `HookService.Invoke` 触发 Hook Point
- `PrepareAgentSkillPrompt`/`EndAgentSkillRound`：通过 `AgentSkillCatalog` 或回退 Legacy
- 持有 `LegacyToolDispatcher` 接口（非 `*extension.Runtime`）作为回退
- 设置 `ExecutionPipeline.ToolResolver = f.resolveTool`

**新建** `backend/internal/extension/kernel/tool_facade_counters.go`
- `ToolFacadeCounters` 结构体，使用 `atomic` 计数器
- 计数项：`legacy_dispatch_calls`、`pipeline_executions`、`pipeline_failures`、`legacy_fallback_*`、`mcp_tool_sync_count` 等
- `Snapshot()` 方法返回 `map[string]int64`

**新建** `backend/internal/extension/kernel/tool_facade_legacy_sync.go`
- `SyncLegacyTools(ctx)` 方法：通过 `LegacyToolDispatcher` 获取旧工具列表，转为 `capability.ToolDefinition` 写入 `ToolRegistry`

### 步骤 5：MCP 工具迁移

**新建** `backend/internal/extension/kernel/tool_facade_mcp.go`
- `SyncMCPTools(ctx, serverID, descriptors)` 方法：使用 `MCPToolAdapter` 将 MCP 工具适配为 `ToolDefinition`，写入 `ToolRegistry` 和 `ContributionRegistry`
- `UnregisterMCPTools(ctx, serverID)` 方法：清理

**修改** `backend/internal/mcp/skill/runtime.go`
- 构造函数新增可选 `ToolFacadeSyncer` 接口参数
- `RegisterServer` 中双写：旧 Registry + 新 ToolRegistry（通过 `ToolFacadeSyncer`）
- 保留旧路径确保过渡期一致性

### 步骤 6：services.go 接线

**修改** `backend/cmd/server/services.go`
- 创建 `LegacyToolDispatcher` 适配器（包装 `extensionRuntime`）
- 创建 `ToolFacade` 实例（注入 Container、LegacyDispatcher、Config）
- `chatSvc.SetToolRuntime(toolFacade)` 替代 `chatSvc.SetSkillRuntime(extensionRuntime)`（保留 SetSkillRuntime 作为回退）
- `mcpskill.New(mcpRepository, connectionManager, extensionRuntime, toolFacade)` 传入 ToolFacade

### 步骤 7：禁用 PluginManager.Start

**修改** `backend/internal/extension/runtime.go`
- `NewRuntime` 新增 `RuntimeOptions` 参数，包含 `SkipPluginManagerStart bool`
- 当 `SkipPluginManagerStart=true` 时跳过 `pluginManager.Start(ctx)`
- 保留旧默认行为用于测试兼容

**修改** `backend/cmd/server/services.go`
- 生产启动使用 `NewRuntimeWithOptions(ctx, db, version, RuntimeOptions{SkipPluginManagerStart: true})`
- ToolFacade 的 HookService 接管 BeforePrompt/AfterReply

### 步骤 8：诊断计数器接入

**修改** `backend/internal/extension/kernel/developer_console/service.go`
- `ConsoleOverview` 新增 `ToolFacadeCounters map[string]int64` 字段
- 新增 `ToolFacadeSummaryProvider` 接口
- 在 services.go 中注入 ToolFacade 作为 provider

### 步骤 9：Legacy Runtime 拆分（只读化）

**修改** `backend/internal/extension/runtime.go`
- 保留 `NewRuntime` 用于测试
- 新增 `NewLegacyMigrationReader`、`NewLegacyReadOnlyFacade` 用于生产只读查询
- 生产启动不再直接使用 `NewRuntime`，改用 `NewRuntimeWithOptions`

## 关键文件清单

**新建文件**：
- `backend/internal/chat/tool_runtime.go` — Chat 接口定义
- `backend/internal/extension/kernel/legacy_runtime_iface.go` — Legacy 适配接口
- `backend/internal/extension/kernel/tool_facade.go` — ToolFacade 主实现
- `backend/internal/extension/kernel/tool_facade_counters.go` — 诊断计数器
- `backend/internal/extension/kernel/tool_facade_legacy_sync.go` — Legacy 工具同步
- `backend/internal/extension/kernel/tool_facade_mcp.go` — MCP 工具迁移

**修改文件**：
- `backend/internal/chat/service.go` — 新增 toolRuntime 字段和 SetToolRuntime
- `backend/internal/chat/compute.go` — 调用点改造（PrepareAgentSkillPrompt/EndAgentSkillRound/BeforePrompt/ModelTools）
- `backend/internal/chat/message_llm.go` — 调用点改造（ExecuteModelTool）
- `backend/internal/chat/message_pipeline.go` — 调用点改造（AfterReply）
- `backend/internal/extension/kernel/container.go` — 新增 ToolRegistry/AdapterRegistry 字段
- `backend/internal/extension/kernel/container_builder.go` — 创建并注入 ToolRegistry/AdapterRegistry，设置 ToolResolver
- `backend/internal/extension/runtime.go` — NewRuntimeWithOptions，SkipPluginManagerStart
- `backend/internal/mcp/skill/runtime.go` — 双写支持
- `backend/cmd/server/services.go` — ToolFacade 接线
- `backend/internal/extension/kernel/developer_console/service.go` — 诊断计数器接入

## 验证方法

1. `go build ./...` — 编译通过
2. `go vet ./internal/extension/kernel/... ./internal/chat/...` — 静态检查通过
3. `go test ./internal/extension/kernel/event/...` — Event E2E 不回归
4. `go test ./internal/chat/...` — Chat 测试不回归
5. `go test ./internal/extension/...` — Extension 测试不回归
6. 重启完整服务后：
   - Chat 能正常对话和调用工具（get_current_time 等）
   - Developer Console 显示 ToolFacade 计数器
   - MCP 工具注册后同时出现在旧 Registry 和新 ToolRegistry
   - `legacy_fallback_*` 计数器在生产流量下逐渐为 0（过渡期验证）
