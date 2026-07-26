# 解除 Skill 概念过载 — 领域模型与术语表

## 1. 领域对象定义

### Tool
可被 Agent 或其他运行时调用的执行能力。

- 有明确的 Handler/Executor
- 有 InputSchema / OutputSchema
- 有执行生命周期（调用→执行→结果）
- 可按来源分类：builtin / plugin / mcp / workflow / internal / legacy_tool
- 概念聚类：**执行对象**

### Agent Skill
提供给 Agent 的指令、知识、流程说明和资源引用包。

- 无 Handler/Executor
- 有 Instructions（提示文本）
- 有激活条件 ActivationRule
- 有依赖声明 RequiredTools / RequiredMCP
- 有 TokenBudget、Resources
- 概念聚类：**指令对象**

### Workflow
工作流定义，描述多步骤、有条件的执行流程。

- 有 Nodes / Steps 定义
- 有独立的 Workflow Registry
- 当 CallableByAgent = true 时，生成 Tool Adapter 进入 Tool Registry
- Workflow 本体生命周期归 Workflow Engine 管理
- 概念聚类：**编排对象**

### MCP Tool Descriptor
MCP 协议发现的原始工具描述。

- 由 MCP Manager 从 Server 发现
- 是协议发现结果，不是最终执行对象
- 通过 MCP Tool Adapter 转换为 ToolDefinition
- 概念聚类：**协议发现结果**

### Contribution
扩展向系统贡献的能力声明。

- 统一索引 Extension 贡献的所有能力
- 类型包括：tool / agent_skill / workflow / mcp / ui / hook / background_task / provider / asset
- Extension 通过 Contribution Registry 声明其含有哪些能力
- 概念聚类：**扩展能力清单**

### Extension
扩展包，包含一个或多个 Contribution。

- 位于 extension 包或外部
- 不同来源：bundled / plugin / workshop / agent_skill 等
- 包含 Module，每个 Module 可以贡献多个 Contribution
- 概念聚类：**扩展单元**

### Plugin Module
扩展包中的可执行代码模块。

- JavaScript / Go 运行时
- 通过 Plugin Host 注册到 Tool Registry
- 概念聚类：**可执行插件代码**

## 2. 术语表

| 术语 | 英文 | 中文 | 说明 |
|------|------|------|------|
| Tool | Tool | 工具 | 可被 Agent 调用的执行能力 |
| Agent Skill | Agent Skill | Agent Skill | 提供给 Agent 的指令和知识包 |
| Workflow | Workflow | 工作流 | 多步骤执行流程定义 |
| MCP Tool | MCP Tool | MCP 工具 | 由 MCP 协议发现的远程工具 |
| Internal Tool | Internal Tool | 内部工具 | 系统内部控制工具 |
| Contribution | Contribution | 贡献 | 扩展向系统注册的能力声明 |
| Extension | Extension | 扩展 | 包含一个或多个 Contribution 的扩展包 |
| Plugin | Plugin | 插件模块 | 扩展包中的可执行代码模块 |
| Module | Module | 模块 | 扩展内的独立功能单元 |
| Registry | Registry | 注册表 | 管理某类对象的索引与生命周期 |

## 3. 旧概念到新概念映射

| 旧对象 | 新对象 | 迁移状态 |
|--------|--------|----------|
| SkillDefinition(SourceLegacy) | ToolDefinition(source: legacy_tool) | 适配器已建立 |
| SkillDefinition(SourceBuiltin) | ToolDefinition(source: builtin) | 适配器已建立 |
| SkillDefinition(SourcePlugin) | ToolDefinition(source: plugin) | 适配器已建立 |
| SkillDefinition(SourceMCP) | ToolDefinition(source: mcp) | 适配器已建立 |
| SkillDefinition(SourceWorkflow) | WorkflowDefinition + Tool Adapter | 适配器已建立 |
| SkillDefinition(SourceInstructions) | AgentSkillDefinition | 适配器已建立 |
| Agent Skill Activation Tool | Internal ToolDefinition | 适配器已建立 |
| Agent Skill Resource Tool | Internal ToolDefinition | 适配器已建立 |
| SkillRegistry | ToolRegistry + AgentSkillCatalog + WorkflowRegistry + ContributionRegistry | Registry 骨架已建立 |

## 4. Tool ID 规范

格式：`<source>/<namespace>/<tool-name>`

示例：
- `builtin/files/read` — 内置文件读取工具
- `builtin/agent-skill/agent_skill_activate` — 内部控制工具
- `plugin/com.example.weather/query_weather` — 插件工具
- `mcp/server-uuid/search` — MCP 工具
- `workflow/com.example.daily-summary/run` — 工作流工具
- `internal/agent-skill/agent_skill_activate` — 内部控制工具
- `legacy_tool/amitia/get_current_time` — 迁移中的旧工具

要求：
- 全局唯一，不依赖数据库自增 ID
- 重启不变、重连不变、角色切换不变
- 不依赖显示名称变化
- 支持日志追踪和权限绑定

## 5. 新增代码文件

### 新领域类型（kernel/）
- `capability/tool.go` — ToolDefinition, ToolSource, BuildToolID
- `capability/registry.go` — ToolRegistry
- `agent_skill/definition.go` — AgentSkillDefinition, AgentSkillCatalogEntry
- `agent_skill/catalog.go` — AgentSkillCatalog
- `workflow/definition.go` — WorkflowDefinition, WorkflowNode
- `workflow/registry.go` — WorkflowRegistry
- `contribution/contribution.go` — Contribution 接口和具体类型
- `contribution/registry.go` — ContributionRegistry

### 迁移适配器（migration/）
- `legacy_skill_adapter.go` — Legacy/Builtin/Plugin/Internal → ToolDefinition
- `mcp_skill_adapter.go` — MCP Skill → ToolDefinition
- `workflow_skill_adapter.go` — Old WorkflowDefinition → New WorkflowDefinition
- `agent_skill_adapter.go` — Old AgentSkillDefinition → New AgentSkillDefinition

### 前端类型定义
- `front/src/types/index.ts` — 新增 ToolDefinition, AgentSkillDefinition, WorkflowDefinition, Contribution 等类型

## 6. 旧系统文件（保持不变，仅标记 Deprecated）

| 文件 | 标记 |
|------|------|
| `extension/protocol.go` | Deprecated |
| `extension/registry.go` | Deprecated |
| `extension/runtime.go` | Deprecated |
| `extension/executor.go` | Deprecated |
| `extension/legacy_tool_adapter.go` | Deprecated |
| `extension/agent_skill_runtime.go` | Deprecated |
| `extension/agent_skill_service.go` | Deprecated |
| `extension/agent_skill_protocol.go` | Deprecated |
| `mcp/skill/runtime.go` | Deprecated |
| `extension/workflow_compiler.go` | Deprecated |
| `extension/workshop_protocol.go` | Deprecated |
| `extension/plugin_host.go` | Deprecated |
