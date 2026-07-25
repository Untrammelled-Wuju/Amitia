# Amitia 扩展系统重构第 6 步实施文档

## 第 6 步：解除 Skill 概念过载

---

## 一、步骤目标

当前 Amitia 将多种本质不同的对象统一包装为 `SkillDefinition`，包括：

- 内置可执行工具；
- Plugin 注册工具；
- MCP Tool；
- Workflow；
- Agent Skill 指令包；
- Agent Skill 激活工具；
- Agent Skill 资源读取工具；
- 历史 Legacy Tool。

这种设计虽然统一了模型调用出口，但导致：

- 领域语义混乱；
- 执行对象与指令对象混在一起；
- 不同生命周期被迫塞进同一 Registry；
- 权限、启用状态和作用域重复；
- Agent Skill 被伪装成没有 Handler 的工具；
- MCP Tool 必须经过额外适配层；
- Workflow 被当作 Skill 注册；
- 前端难以解释“技能”到底是什么；
- 新插件系统无法建立清晰的 Contribution 模型。

本步骤的目标是：

> 将现有 Skill 概念拆分为明确的领域对象，为后续统一 Tool/Capability 模型和 Extension Kernel 建立正确基础。

本步骤完成后，代码中必须能够清晰区分：

```text
Tool
Agent Skill
Workflow
MCP Tool
Plugin Contribution
Built-in Capability
Internal Control Tool
```

但本步骤暂不完成全部执行链迁移，也不删除旧系统。

---

## 二、核心原则

### 1. Skill 不再作为总称

新架构中：

```text
Skill
```

只能表示：

> 提供给 Agent 的指令、知识、流程说明和资源引用。

不得再表示：

- 可执行函数；
- MCP Tool；
- Workflow；
- Plugin；
- Model Provider；
- UI Contribution；
- 后台任务；
-系统内置工具。

### 2. 可执行对象统一称为 Tool 或 Capability

建议区分：

```text
Capability
└── Tool
```

其中：

- Capability：系统内部统一能力对象；
- Tool：可被 Agent 直接调用的 Capability；
- 其他 Capability 可包括 Workflow Entry、Provider Action、Desktop Action 等。

第一阶段可先统一到 `ToolDefinition`，后续再抽象成完整 `CapabilityDefinition`。

### 3. Agent Skill 独立于 Tool Registry

Agent Skill 应进入：

```text
Agent Skill Catalog
```

而不是 Tool Registry。

Agent Skill 负责：

- 指令；
-知识；
-资源；
-激活条件；
-依赖声明；
-Token 预算；
-工具引用。

Agent Skill 不负责：

- 执行 Tool；
-直接提供 Handler；
-管理 MCP 生命周期；
-管理 Workflow 生命周期。

### 4. MCP Tool 只是 Tool 的一种来源

MCP Tool 不再转换为 Skill，而应转换为：

```text
ToolDefinition {
    Source: MCP
}
```

MCP Server 仍由 MCP Manager 管理连接，但 Tool 注册进入统一 Tool Registry。

### 5. Workflow 是独立执行对象

Workflow 应拥有独立定义和执行语义。

当 Workflow 需要被 Agent 调用时，应生成：

```text
Workflow Tool Adapter
```

而不是将 Workflow 本身伪装成 Skill。

### 6. Plugin 是 Contribution 容器

Plugin 不等于 Tool，也不等于 Skill。

Plugin 可以贡献：

- Tools；
-Agent Skills；
-Workflows；
-MCP Definitions；
-UI；
-Hooks；
-Background Tasks；
-Providers。

---

## 三、目标领域模型

本步骤需要定义以下核心对象。

## 1. ToolDefinition

表示可被 Agent 或其他运行时调用的工具。

建议字段：

```go
type ToolDefinition struct {
    ID             string
    ExtensionID    string
    ModuleID       string
    Source         ToolSource
    Name           string
    Description    string
    InputSchema    json.RawMessage
    OutputSchema   json.RawMessage
    Permissions    []PermissionRequirement
    RiskLevel      RiskLevel
    SideEffect     SideEffectLevel
    Scope          ScopeRule
    Executor       ToolExecutor
    Metadata       map[string]any
}
```

来源：

```go
type ToolSource string

const (
    ToolSourceBuiltin  ToolSource = "builtin"
    ToolSourcePlugin   ToolSource = "plugin"
    ToolSourceMCP      ToolSource = "mcp"
    ToolSourceWorkflow ToolSource = "workflow"
    ToolSourceInternal ToolSource = "internal"
)
```

---

## 2. AgentSkillDefinition

表示 Agent 指令能力。

建议字段：

```go
type AgentSkillDefinition struct {
    ID              string
    ExtensionID     string
    ModuleID        string
    Name            string
    Description     string
    Instructions    string
    Activation      ActivationRule
    RequiredTools   []ToolReference
    RequiredMCP     []MCPReference
    Resources       []SkillResource
    TokenBudget     int
    Scope           ScopeRule
    Metadata        map[string]any
}
```

不得包含：

- Tool Handler；
-Tool Executor；
-直接执行函数；
-Registry 执行状态。

---

## 3. WorkflowDefinition

表示工作流定义。

```go
type WorkflowDefinition struct {
    ID             string
    ExtensionID    string
    ModuleID       string
    Name           string
    Description    string
    InputSchema    json.RawMessage
    OutputSchema   json.RawMessage
    Nodes          []WorkflowNode
    Permissions    []PermissionRequirement
    Scope          ScopeRule
    CallableByAgent bool
}
```

当 `CallableByAgent == true` 时，由 Workflow Adapter 生成 ToolDefinition。

---

## 4. MCPToolDescriptor

表示 MCP Server 发现的原始工具描述。

```go
type MCPToolDescriptor struct {
    ServerID     string
    ToolName     string
    Description  string
    InputSchema  json.RawMessage
    OutputSchema json.RawMessage
    Metadata     map[string]any
}
```

之后通过 MCP Tool Adapter 转换为 ToolDefinition。

注意：

```text
MCPToolDescriptor
```

不是系统最终执行对象，只是协议发现结果。

---

## 5. Contribution

所有扩展能力统一通过 Contribution 表达。

```go
type Contribution interface {
    ContributionID() string
    ContributionType() ContributionType
    ExtensionID() string
    ModuleID() string
}
```

类型包括：

```text
tool
agent_skill
workflow
mcp
ui
hook
background_task
provider
asset
```

---

## 四、旧概念映射

必须建立旧对象到新对象的映射。

| 旧对象 | 新对象 |
|---|---|
| Legacy Tool | ToolDefinition |
| SkillDefinition(SourceLegacy) | ToolDefinition |
| SkillDefinition(SourcePlugin) | ToolDefinition |
| SkillDefinition(SourceMCP) | ToolDefinition |
| SkillDefinition(SourceWorkflow) | WorkflowDefinition + Tool Adapter |
| SkillDefinition(SourceInstructions) | AgentSkillDefinition |
| Agent Skill Activation Tool | Internal ToolDefinition |
| Agent Skill Resource Tool | Internal ToolDefinition |
| Plugin | Extension Runtime Module |
| Workflow Skill | Workflow Contribution |
| MCP Skill Runtime | MCP Tool Adapter |
| Skill Registry | Tool Registry + Agent Skill Catalog + Workflow Registry |

---

## 五、代码范围

重点审查并修改：

```text
backend/internal/extension/registry.go
backend/internal/extension/runtime.go
backend/internal/extension/executor.go
backend/internal/extension/permission.go
backend/internal/extension/legacy_tool_adapter.go
backend/internal/extension/agent_skill_runtime.go
backend/internal/extension/agent_skill_handler.go
backend/internal/mcp/skill/runtime.go
backend/internal/extension/workflow_compiler.go
backend/internal/extension/workflow_executor.go
backend/internal/extension/plugin_host.go
```

同时审查：

```text
front/src/views/extensions/
front/src/views/mcp/
front/src/types/
```

---

## 六、实施策略

本步骤采用渐进式重构，不一次删除旧 SkillDefinition。

### 阶段 A：新增新领域类型

先新增：

```text
ToolDefinition
AgentSkillDefinition
WorkflowDefinition
Contribution
```

不改变现有运行行为。

### 阶段 B：建立转换层

建立临时转换：

```text
SkillDefinition → ToolDefinition
SkillDefinition → AgentSkillDefinition
SkillDefinition → WorkflowDefinition
```

仅用于迁移期间。

### 阶段 C：新代码禁止继续使用 SkillDefinition

新增代码必须使用：

- ToolDefinition；
-AgentSkillDefinition；
-WorkflowDefinition；
-Contribution。

### 阶段 D：逐类迁移

迁移顺序：

1. Legacy Tool；
2. MCP Tool；
3. Plugin Tool；
4. Workflow；
5. Agent Skill；
6. 内部控制工具。

### 阶段 E：删除旧 SkillDefinition

在全部调用链完成迁移后，于后续步骤删除。

---

## 七、命名规范

禁止继续使用以下含糊命名：

```text
Skill
SkillItem
SkillRuntime
SkillHandler
SkillSource
SkillRegistry
```

除非对象明确表示 Agent Skill。

推荐命名：

```text
ToolRegistry
ToolExecutor
ToolDefinition
ToolInvocation
ToolSource
AgentSkillCatalog
AgentSkillActivator
WorkflowRegistry
WorkflowExecutor
MCPToolAdapter
ContributionRegistry
```

内部控制工具命名：

```text
InternalTool
SystemTool
ControlTool
```

不得叫 Agent Skill 本体。

---

## 八、Tool ID 规范

建立统一 Tool ID：

```text
<source>/<namespace>/<tool-name>
```

示例：

```text
builtin/files/read
plugin/com.example.weather/query_weather
mcp/server-123/search
workflow/com.example.daily-summary/run
internal/agent-skill/activate
```

要求：

- 全局唯一；
-稳定；
-不依赖显示名称；
-不依赖数据库自增 ID；
-不因角色变化；
-不因连接重启；
-支持日志追踪；
-支持权限绑定；
-支持迁移。

模型可见名称可单独生成，但必须与 Tool ID 映射稳定。

---

## 九、来源与所有权

每个 ToolDefinition 必须明确：

```text
source
owner_extension_id
owner_module_id
runtime_owner
resource_owner
```

示例：

### 内置工具

```text
source: builtin
owner: system
runtime_owner: core
```

### Plugin Tool

```text
source: plugin
owner_extension_id: com.example.weather
runtime_owner: extension runtime
```

### MCP Tool

```text
source: mcp
owner_extension_id: 可选
resource_owner: mcp server owner
runtime_owner: mcp manager
```

### Workflow Tool

```text
source: workflow
resource_owner: workflow owner
runtime_owner: workflow engine
```

---

## 十、Agent Skill 调整

### 1. 移除伪 Tool 语义

当前 Agent Skill 若注册为：

```text
SkillDefinition {
    Handler: nil
}
```

必须标记为迁移对象。

新逻辑：

```text
Agent Skill
→ Agent Skill Catalog
→ Activation
→ Prompt Context
```

不得进入 Tool Registry。

### 2. 保留内部控制工具

以下能力仍可作为 Tool：

```text
agent_skill_activate
agent_skill_list_resources
agent_skill_read_resource
agent_skill_get_asset
```

但必须改名并标记为：

```text
Internal Tool
```

这些 Tool 的职责是操作 Agent Skill Catalog，而不是代表 Agent Skill 本体。

### 3. 工具依赖

Agent Skill 通过稳定 Tool ID 声明依赖：

```json
{
  "requiredTools": [
    "mcp/server-123/search",
    "builtin/files/read"
  ]
}
```

不得依赖显示名称或旧 Skill ID。

---

## 十一、MCP 调整

### 1. 保留协议描述

MCP Discovery 仍产生协议层 Tool Descriptor。

### 2. 改造适配器

将：

```text
MCP Tool
→ SkillDefinition
```

改为：

```text
MCP Tool Descriptor
→ ToolDefinition
```

### 3. 删除 Skill Source MCP

新 ToolSource 使用：

```text
ToolSourceMCP
```

不再存在：

```text
SkillSourceMCP
```

### 4. Tool 可用状态

MCP Tool 可执行必须满足：

```text
Tool 已注册
MCP Server 已启用
当前作用域允许
MCP Connection Ready
Tool Enabled
权限已授权
```

这些状态不能继续混成 Skill Enabled。

---

## 十二、Workflow 调整

### 1. Workflow 本体独立

WorkflowDefinition 进入 Workflow Registry。

### 2. Agent 可调用适配

只有声明：

```text
callableByAgent: true
```

的 Workflow 才生成 ToolDefinition。

### 3. Tool Adapter

Tool Handler 只负责：

```text
校验输入
→ 调用 Workflow Executor
→ 返回结果
```

Workflow 生命周期不由 Tool Registry 管理。

### 4. 所有权

Workflow Tool 被删除时，不得自动删除 WorkflowDefinition。

只有资源所有者可删除 Workflow。

---

## 十三、Plugin 调整

### 1. Plugin 注册 Tool

Plugin Host API 调整为：

```go
RegisterTool(ctx, ToolDefinition)
```

而不是：

```go
RegisterSkill(...)
```

### 2. Plugin 注册 Agent Skill

未来允许：

```go
RegisterAgentSkill(...)
```

但进入 Agent Skill Catalog。

### 3. Plugin 注册 Workflow

未来允许：

```go
RegisterWorkflow(...)
```

但进入 Workflow Registry。

### 4. Plugin Manifest

Plugin Manifest 不再声明笼统的 Skills，而是声明 Contributions。

---

## 十四、Registry 拆分

当前 Skill Registry 必须拆分为：

```text
Tool Registry
Agent Skill Catalog
Workflow Registry
Contribution Registry
```

各自职责：

### Tool Registry

负责：

- Tool 注册；
-Tool 查询；
-模型暴露；
-作用域过滤；
-执行定位。

### Agent Skill Catalog

负责：

- Agent Skill 索引；
-激活；
-资源；
-Token；
-依赖；
-角色作用域。

### Workflow Registry

负责：

- Workflow 定义；
-版本；
-可调用状态；
-依赖；
-所有权。

### Contribution Registry

负责：

- Extension Contribution 的统一索引；
-扩展与模块关系；
-启用；
-卸载；
-资源归属。

---

## 十五、API 调整原则

本步骤暂不删除旧 API，但新增内部接口应使用新命名。

旧接口：

```text
/api/extensions/skills
```

后续目标：

```text
/api/extensions/tools
/api/extensions/agent-skills
/api/extensions/workflows
```

MCP Tool 仍可在：

```text
/api/mcp/servers/:id/tools
```

展示，但其执行能力来自 Tool Registry。

旧 Skill API 进入迁移状态，不继续增加字段。

---

## 十六、前端术语调整

本步骤需建立前端术语规范。

### Tool

中文：

```text
工具
```

用于：

- 模型可调用能力；
-内置工具；
-MCP 工具；
-插件工具；
-Workflow 工具入口。

### Agent Skill

中文：

```text
Agent Skill
```

或：

```text
代理技能
```

建议产品层保留 Agent Skill 原名，避免与普通工具混淆。

### Workflow

中文：

```text
工作流
```

### Extension

中文：

```text
扩展
```

### Plugin

中文：

```text
插件模块
```

插件不是整个扩展包，而是扩展包中的运行时代码模块。

---

## 十七、数据库与持久化影响

本步骤暂不正式迁移数据库，但必须设计映射。

现有 Skill 相关字段需分类：

```text
skill_id
skill_source
skill_type
skill_enabled
skill_scope
skill_handler
```

目标映射：

```text
tool_id
tool_source
capability_type
contribution_id
extension_id
module_id
scope_binding
runtime_binding
```

Agent Skill 数据不得迁入 Tool 表。

Workflow 数据不得迁入 Tool 表，只存 Tool Adapter 关系。

---

## 十八、兼容层约束

临时兼容层可以存在，但必须满足：

- 单向；
-只从旧模型转换到新模型；
-不双写；
-不从新模型回写旧模型；
-有删除步骤；
-有调用统计；
-有测试；
-禁止新业务依赖。

推荐：

```go
type LegacySkillAdapter struct {
    // migration-only
}
```

禁止：

```text
新 Tool Registry
↔ 旧 Skill Registry
```

双向同步。

---

## 十九、测试要求

必须新增：

### 1. 类型映射测试

覆盖：

- Legacy Tool → ToolDefinition；
-MCP Tool → ToolDefinition；
-Plugin Tool → ToolDefinition；
-Workflow → WorkflowDefinition；
-Agent Skill → AgentSkillDefinition。

### 2. ID 稳定性测试

验证 Tool ID：

- 重启不变；
-重连不变；
-角色切换不变；
-显示名称变化不变；
-版本升级规则明确。

### 3. Registry 隔离测试

验证：

- Agent Skill 不进入 Tool Registry；
-Workflow 本体不进入 Tool Registry；
-MCP Descriptor 不直接执行；
-Plugin Contribution 正确进入对应 Registry。

### 4. 兼容测试

旧模型工具列表与新 Tool Registry 在迁移阶段保持必要等价。

### 5. 已知错误消除测试

未来需验证：

- Agent Skill 不再是 nil Handler Skill；
-MCP Tool 不再经过 Skill Runtime；
-Workflow 不再作为 SkillDefinition。

---

## 二十、实施任务

### Task 1：建立术语与领域模型文档

定义：

- Tool；
-Capability；
-Agent Skill；
-Workflow；
-MCP Tool；
-Contribution；
-Extension；
-Plugin Module。

### Task 2：新增新领域类型

新增：

```text
ToolDefinition
AgentSkillDefinition
WorkflowDefinition
Contribution
```

暂不替换旧执行链。

### Task 3：定义稳定 ID 规范

为 Tool、Agent Skill、Workflow、Contribution 建立稳定 ID。

### Task 4：建立旧 Skill 分类器

将所有现有 SkillDefinition 按来源分类，并生成迁移报告。

### Task 5：迁移 Legacy Tool 类型

让 Legacy Adapter 输出 ToolDefinition。

### Task 6：迁移 MCP Tool 类型

让 MCP Discovery 转换为 ToolDefinition，不再产生新的 SkillDefinition。

### Task 7：迁移 Plugin Tool 类型

将 Plugin Host 的注册接口改为 Tool 注册。

### Task 8：迁移 Workflow 类型

将 Workflow 本体迁出 Skill Registry，建立 Workflow Registry 与 Tool Adapter。

### Task 9：迁移 Agent Skill 类型

将 Agent Skill 本体迁入 Agent Skill Catalog，保留内部控制 Tool。

### Task 10：建立新 Registry 骨架

建立 Tool Registry、Agent Skill Catalog、Workflow Registry 和 Contribution Registry。

### Task 11：增加兼容读取层

在迁移期间支持读取旧 SkillDefinition，但禁止新写入。

### Task 12：更新前端术语和类型

先更新类型定义和文档，不重建页面结构。

### Task 13：增加迁移统计

记录：

- 仍由旧 SkillDefinition 提供的对象数量；
-已迁移 Tool 数量；
-已迁移 Agent Skill 数量；
-已迁移 Workflow 数量；
-旧 Adapter 调用次数。

### Task 14：完成回归测试

确保现有模型工具调用和 Agent Skill 使用未被破坏。

---

## 二十一、建议目录结构

建议新增：

```text
backend/internal/extension/kernel/
├── capability/
│   ├── tool.go
│   ├── source.go
│   ├── executor.go
│   └── registry.go
├── contribution/
│   ├── contribution.go
│   ├── type.go
│   └── registry.go
├── agent_skill/
│   ├── definition.go
│   └── catalog.go
└── workflow/
    ├── definition.go
    └── registry.go
```

迁移适配器：

```text
backend/internal/extension/migration/
├── legacy_skill_adapter.go
├── mcp_skill_adapter.go
├── workflow_skill_adapter.go
└── agent_skill_adapter.go
```

目录仅作为建议，必须结合现有项目结构实施，不得为了目录形式进行无意义搬迁。

---

## 二十二、风险控制

### P0：行为中断

包括：

- 模型工具列表减少；
-Tool ID 变化；
-权限映射丢失；
-Agent Skill 无法激活；
-MCP Tool 无法调用；
-Workflow 无法执行。

### P1：双 Registry 漂移

包括：

- 新旧 Registry 不一致；
-删除只发生在一侧；
-启用状态不同步；
-重复 Tool 暴露。

### P2：概念迁移不完整

包括：

- 新代码继续使用 SkillDefinition；
-Agent Skill 仍有 Handler；
-Workflow 仍直接注册；
-前端继续统一叫技能。

### P3：命名问题

包括：

- Tool 与 Capability 混用；
-Plugin 与 Extension 混用；
-Agent Skill 与 Tool 混用。

---

## 二十三、本步骤不做的事情

本步骤明确不做：

- 不建立完整 Extension Kernel 生命周期；
-不实现 `.amitiax` v2；
-不实现第三方 JavaScript Runtime；
-不实现 UI Contribution；
-不删除旧 Skill Registry；
-不删除旧数据库表；
-不删除旧 API；
-不重建扩展中心；
-不改变 MCP 协议实现；
-不改变 Agent Skill 文件格式；
-不改变 Workflow Schema；
-不迁移用户数据。

---

## 二十四、验收产物

完成后必须提交：

### 1. 领域模型主文档

```text
docs/extension-kernel/06-skill-concept-separation.md
```

### 2. 术语表

必须定义：

- Extension；
-Plugin；
-Module；
-Contribution；
-Tool；
-Capability；
-Agent Skill；
-Workflow；
-MCP Tool；
-Internal Tool。

### 3. 旧 Skill 分类清单

列出所有 SkillDefinition 来源和目标类型。

### 4. 新领域类型代码

至少包含：

- ToolDefinition；
-AgentSkillDefinition；
-WorkflowDefinition；
-Contribution。

### 5. Registry 骨架

至少建立：

- Tool Registry；
-Agent Skill Catalog；
-Workflow Registry；
-Contribution Registry。

### 6. 迁移适配器

明确旧对象到新对象的单向转换。

### 7. Tool ID 规范

包含格式、稳定性、冲突处理和迁移规则。

### 8. 测试报告

覆盖：

- 类型映射；
-ID；
-Registry 隔离；
-兼容行为；
-旧链路回归。

### 9. 剩余旧 Skill 使用报告

列出所有仍依赖 SkillDefinition 的文件和调用路径。

---

## 二十五、验收标准

本步骤通过必须满足：

1. 已正式定义 Tool、Agent Skill、Workflow 和 Contribution。
2. Skill 不再作为所有能力的总称。
3. 新代码禁止新增 SkillDefinition。
4. Legacy Tool 已可转换为 ToolDefinition。
5. MCP Tool 已可转换为 ToolDefinition。
6. Plugin Tool 已可使用 ToolDefinition。
7. Workflow 本体已具备独立定义。
8. Agent Skill 本体已具备独立定义。
9. 已建立 Tool Registry 骨架。
10. 已建立 Agent Skill Catalog 骨架。
11. 已建立 Workflow Registry 骨架。
12. 已建立 Contribution Registry 骨架。
13. Tool ID 稳定且有测试。
14. 兼容层为单向且可删除。
15. 当前功能未出现回归。
16. 旧 Skill 使用位置已有完整清单。
17. 后续第 7 步可以正式建立统一 Tool/Capability 模型。

---

## 二十六、退出条件

只有满足以下条件后，才能进入第 7 步“建立统一 Tool/Capability 模型”：

- 新领域对象已定义；
-旧 SkillDefinition 已全部分类；
-新代码不再写入旧 SkillDefinition；
-Legacy、MCP、Plugin、Workflow 和 Agent Skill 均有迁移路径；
-Tool ID 规则已锁定；
-Registry 骨架已建立；
-兼容行为测试通过；
-剩余旧依赖已有删除计划；
-未引入双向同步；
-未扩大旧系统职责。

---

## 二十七、执行约束

执行本步骤时必须遵守：

> 本步骤不是简单重命名，而是重新建立领域边界。

禁止只做以下表面修改：

- 将 `SkillDefinition` 改名为 `ToolDefinition`；
-将 `SkillRegistry` 改名为 `ToolRegistry`；
-继续让 Agent Skill、Workflow 和 MCP 全部共用同一个类型；
-保留旧 Source 枚举但换名字；
-继续让 nil Handler 对象进入 Tool Registry。

真正完成的标准是：

```text
可执行能力
≠
指令能力
≠
工作流定义
≠
协议发现结果
≠
插件容器
```

只有这些对象在类型、Registry、生命周期和所有权上真正分离，本步骤才算完成。
