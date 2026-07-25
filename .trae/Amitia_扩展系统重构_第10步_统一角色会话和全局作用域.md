# Amitia 扩展系统重构第 10 步实施文档

## 第 10 步：统一角色、会话和全局作用域

---

## 一、步骤目标

在第 9 步已经建立统一 Permission Broker 的基础上，重构当前分散在 Skill、Agent Skill、MCP、Plugin、Workflow、Package、Tool Registry、角色绑定和聊天上下文中的作用域逻辑，建立唯一的 Scope Model 与 Scope Manager。

本步骤的目标是：

> 让所有扩展、模块、Tool、Agent Skill、Workflow、MCP Server、后台任务和 UI Contribution 都使用同一套作用域定义、绑定、继承、解析和失效规则。

当前系统中可能同时存在：

- 全局 Enabled；
-角色 Enabled；
-会话临时启用；
-MCP Server 角色绑定；
-MCP Tool Enabled；
-Skill Scope Binding；
-Agent Skill 全局或角色作用域；
-Plugin Scope State；
-Workflow 角色上下文；
-Package Binding；
-前端当前角色状态；
-聊天当前会话状态；
-后台任务创建时上下文；
-权限 Scope。

这些状态职责混杂，导致：

- “是否安装”“是否启用”“是否允许当前角色使用”“是否有权限”难以区分；
- 角色切换后缓存仍沿用旧作用域；
- MCP Server 已绑定角色但 Tool Registry 未同步；
- Agent Skill 全局启用但当前角色不可用；
- Plugin 全局启用后所有角色都能看到；
- Workflow 子调用缺少会话 Scope；
- 后台任务错误继承前台临时上下文；
- 删除角色后残留 Binding、Schedule、Cache 和 Grant；
- 同一能力在前端显示可用，执行时却被另一套 Scope 拒绝。

本步骤完成后，系统必须形成统一作用域链：

```text
Scope Definition
→ Scope Binding
→ Scope Resolution
→ Scope Inheritance
→ Scope Evaluation
→ Scope Snapshot
→ Scope Invalidation
→ Audit
```

---

## 二、作用域系统职责边界

Scope Manager 负责：

- 定义作用域类型；
-作用域对象解析；
-作用域绑定；
-作用域继承；
-作用域收窄；
-作用域匹配；
-作用域快照；
-临时作用域；
-作用域失效；
-角色删除清理；
-会话结束清理；
-扩展禁用清理；
-作用域缓存；
-作用域审计；
-向 Tool、Permission、Agent Skill、Workflow、MCP 和 UI 提供统一查询。

Scope Manager 不负责：

- 扩展安装；
-模块启用；
-Tool 权限授权；
-MCP 连接；
-Plugin Runtime 健康；
-Workflow 执行；
-Agent Skill 激活；
-用户身份认证；
-前端路由；
-具体角色业务数据；
-会话消息持久化。

必须明确：

```text
Scope 决定“在哪个范围内可用”
Permission 决定“是否被允许”
Availability 决定“当前是否能执行”
Enabled 决定“资源是否启用”
```

四者不得继续混用。

---

## 三、统一作用域类型

建议定义：

```go
type ScopeType string

const (
    ScopeGlobal       ScopeType = "global"
    ScopeCharacter    ScopeType = "character"
    ScopeConversation ScopeType = "conversation"
    ScopeExtension    ScopeType = "extension"
    ScopeModule       ScopeType = "module"
    ScopeResource     ScopeType = "resource"
    ScopeInvocation   ScopeType = "invocation"
    ScopeSession      ScopeType = "session"
)
```

---

## 四、各作用域含义

## 1. Global Scope

表示对当前 Amitia 单用户实例全局可用。

适用于：

- 系统内置 Tool；
-全局扩展配置；
-用户手动创建的共享 MCP Server；
-全局 Provider；
-全局 Agent Skill；
-全局 Workflow；
-系统级 UI Contribution。

全局 Scope 不代表：

- 自动拥有权限；
-所有角色必须启用；
-所有会话必须暴露；
-后台任务可以任意访问角色数据。

---

## 2. Character Scope

绑定指定角色。

适用于：

- 角色专属 Agent Skill；
-角色专属 MCP；
-角色专属 Workflow；
-角色专属插件模块；
-角色专属 UI；
-角色专属工具；
-角色专属配置；
-角色专属记忆访问。

Character Scope 必须使用稳定 Character ID，不得使用显示名称。

支持：

```text
current
specific character ID
all characters
```

其中 `current` 只能在解析时使用，不得持久化为真实 Binding。

---

## 3. Conversation Scope

绑定指定会话。

适用于：

- 会话临时 Tool；
-会话专属 Agent Skill；
-临时 MCP Tool；
-会话 Workflow；
-会话 UI 面板；
-当前聊天上下文贡献；
-临时授权；
-短期调试工具。

Conversation Scope 必须关联：

- Conversation ID；
-Character ID；
-创建时间；
-结束或失效条件。

会话删除或归档后必须执行清理或失效。

---

## 4. Extension Scope

绑定指定扩展。

适用于：

- 扩展私有 Tool；
-扩展内部 IPC；
-扩展配置；
-扩展存储；
-扩展私有事件；
-扩展内部后台任务。

Extension Scope 不等于用户可见 Scope。

一个 Extension Scope 下的 Tool 仍可能同时要求 Character 或 Conversation Scope。

---

## 5. Module Scope

绑定扩展中的模块。

适用于：

- 扩展子模块启停；
-模块私有 Tool；
-模块 UI；
-模块事件；
-模块配置；
-模块 Worker；
-模块 Secret。

Module Scope 必须包含：

```text
extension_id
module_id
```

---

## 6. Resource Scope

绑定某个具体资源。

例如：

- MCP Server；
-Workflow；
-Agent Skill；
-Provider；
-文件目录；
-数据库逻辑资源；
-消息频道；
-桌面窗口；
-模型配置。

Resource Scope 必须包含：

```text
resource_type
resource_id
owner
```

---

## 7. Invocation Scope

绑定单次 Tool 调用。

用于：

- 仅本次授权；
-仅本次临时数据；
-临时资源；
-审批结果；
-一次性 MCP Session；
-临时 Tool 暴露。

Invocation Scope 随 Invocation 结束立即失效。

---

## 8. Session Scope

表示一个短期运行会话，不等同于 Conversation。

可用于：

- 开发者模式调试；
-插件预览；
-扩展安装预览；
-临时沙箱；
-用户临时操作流程；
-一次桌面控制会话。

必须有：

- Session ID；
-创建时间；
-过期时间；
-所有者；
-清理函数。

---

## 五、Scope Identity

建议定义：

```go
type ScopeRef struct {
    Type           ScopeType
    CharacterID    string
    ConversationID string
    ExtensionID    string
    ModuleID       string
    ResourceType   string
    ResourceID     string
    InvocationID   string
    SessionID      string
}
```

要求：

- 每种 ScopeType 只允许对应字段；
-无效组合必须拒绝；
-不得将多个 Scope 混入一个未约束 JSON；
-不得使用名称代替 ID；
-持久化前必须规范化；
-比较时必须结构化比较；
-禁止拼接字符串后模糊匹配。

---

## 六、Scope Set

单个能力通常需要多个 Scope 共同约束。

建议：

```go
type ScopeSet struct {
    Global       bool
    Characters   []string
    Conversations []string
    Extensions   []string
    Modules      []ModuleRef
    Resources    []ResourceRef
    Invocations  []string
    Sessions     []string
}
```

但执行时应使用规范化 Scope Expression，而不是直接依赖数组交集。

---

## 七、Scope Expression

建议支持：

```text
AND
OR
NOT
CURRENT_CHARACTER
CURRENT_CONVERSATION
OWNER_EXTENSION
OWNER_MODULE
```

示例：

```text
global
AND character(current)
AND extension(com.example.weather)
```

或：

```text
character(character-a)
OR character(character-b)
```

需要避免过度复杂，第一版建议只支持：

- 单 Scope；
-多 Scope AND；
-同类型列表 OR；
-Current 占位符；
-Owner 占位符。

不得在第一版引入任意脚本表达式。

---

## 八、Scope Binding

Scope Binding 表示某对象与作用域的持久关联。

建议：

```go
type ScopeBinding struct {
    BindingID     string
    SubjectType   ScopeSubjectType
    SubjectID     string
    Scope         ScopeRef
    State         ScopeBindingState
    Source        ScopeBindingSource
    CreatedAt     time.Time
    UpdatedAt     time.Time
    ExpiresAt     *time.Time
    Metadata      map[string]any
}
```

SubjectType：

```text
extension
module
tool
agent_skill
workflow
mcp_server
mcp_tool
ui_contribution
background_task
provider
```

State：

```text
active
inactive
expired
revoked
pending
```

Source：

```text
system
user
package
migration
runtime
temporary
```

---

## 九、Enabled 与 Binding 分离

必须建立清晰规则：

### Enabled

表示对象本身是否启用。

例如：

```text
Extension Enabled
Module Enabled
Tool Enabled
Agent Skill Enabled
Workflow Enabled
MCP Server Enabled
```

### Scope Binding

表示对象在哪些范围可用。

例如：

```text
Tool Enabled = true
Character Binding = character-a
```

含义是：

> Tool 已启用，但只对 character-a 可用。

不得继续使用：

```text
Scope Enabled
```

这种将启用状态和作用域混在一起的字段。

---

## 十、Scope Manager 接口

建议：

```go
type ScopeManager interface {
    Bind(
        ctx context.Context,
        request ScopeBindRequest,
    ) (ScopeBinding, error)

    Unbind(
        ctx context.Context,
        bindingID string,
    ) error

    Resolve(
        ctx context.Context,
        request ScopeResolveRequest,
    ) ScopeResolution

    Evaluate(
        ctx context.Context,
        request ScopeEvaluationRequest,
    ) ScopeDecision

    Snapshot(
        ctx context.Context,
        request ScopeSnapshotRequest,
    ) ScopeSnapshot

    Invalidate(
        ctx context.Context,
        filter ScopeInvalidationFilter,
    ) error

    ListBindings(
        ctx context.Context,
        filter ScopeBindingFilter,
    ) ([]ScopeBinding, error)
}
```

---

## 十一、Scope Resolution

Scope Resolution 将运行时占位符转换成真实 Scope。

输入可能包含：

```text
current character
current conversation
owner extension
owner module
```

解析需要依赖：

- ToolInvocationContext；
-当前 Extension；
-当前 Module；
-当前角色；
-当前会话；
-资源所有者；
-父 Invocation。

输出必须是稳定、不可变的 Scope Snapshot。

---

## 十二、Scope Snapshot

执行开始后必须生成 Scope Snapshot。

建议：

```go
type ScopeSnapshot struct {
    SnapshotID     string
    InvocationID   string
    ResolvedScopes []ScopeRef
    CharacterID    string
    ConversationID string
    ExtensionID    string
    ModuleID       string
    CreatedAt      time.Time
    ExpiresAt      *time.Time
}
```

用途：

- 执行审计；
-后台任务；
-父子调用；
-权限判断；
-重试；
-异步任务；
-故障恢复。

执行过程中不得因用户切换角色而改变当前 Invocation Scope。

---

## 十三、Scope Evaluation

建议：

```go
type ScopeDecision struct {
    Allowed bool
    Reasons []ScopeReason
    Matched []ScopeBinding
}
```

Reason：

```text
no_binding
character_mismatch
conversation_mismatch
extension_mismatch
module_disabled
resource_mismatch
binding_expired
binding_revoked
parent_scope_too_narrow
conversation_not_owned_by_character
```

评估必须使用：

- 对象 Binding；
-Invocation Context；
-父调用 Snapshot；
-角色与会话关系；
-资源所有权；
-临时 Scope；
-系统 Scope 规则。

---

## 十四、作用域继承

父子调用必须满足：

```text
child scope ⊆ parent scope
```

例如：

```text
角色 A 的 Plugin Tool
→ 调用 Workflow
→ 调用 MCP Tool
```

所有子调用必须仍在角色 A 范围。

如果子 Tool 声明 Global，但父调用仅角色 A：

```text
实际子 Scope = 角色 A
```

不得扩大为 Global。

---

## 十五、角色与会话关系

必须校验：

- Conversation 属于哪个 Character；
-当前 Character 与 Conversation 是否一致；
-会话是否已删除；
-会话是否已归档；
-角色是否已删除；
-会话 Scope 是否允许后台继续使用；
-跨角色会话访问是否明确授权。

禁止：

- 只提供 Conversation ID 而不验证 Character；
-角色切换后继续使用旧 Conversation；
-后台任务使用已删除会话；
-插件通过 Conversation ID 访问其他角色。

---

## 十六、Agent Skill 作用域

Agent Skill 使用独立 Catalog，但作用域由 Scope Manager 管理。

支持：

- 全局 Agent Skill；
-角色 Agent Skill；
-会话临时 Agent Skill；
-扩展私有 Agent Skill；
-包内 Agent Skill。

激活时必须检查：

```text
Agent Skill Enabled
Scope Binding 匹配
依赖 Tool 在当前 Scope 可用
Token Budget 允许
```

Agent Skill 不得自行维护另一套角色绑定表。

---

## 十七、MCP 作用域

MCP Server 与 MCP Tool 作用域必须拆分。

### MCP Server Scope

决定：

- 哪些角色或会话可以使用该 Server；
-后台任务是否可使用；
-扩展是否私有拥有；
-用户是否全局共享。

### MCP Tool Scope

默认继承 Server Scope，但可以进一步收窄。

禁止：

- Tool Scope 大于 Server Scope；
-连接成功后自动全局可用；
-角色绑定和 Tool Enabled 互相覆盖；
-MCP API 直接写 Tool Registry Scope。

---

## 十八、Plugin 作用域

Extension 与 Module 需要分别 Binding。

示例：

```text
Extension 全局安装
Module A 对角色 1 启用
Module B 对角色 2 启用
```

Plugin Runtime 可以全局运行，但其 Contribution 必须按 Scope 过滤。

必须区分：

- Runtime 是否启动；
-Contribution 是否启用；
-Contribution 对哪个 Scope 可见；
-Host API 当前 Invocation Scope。

---

## 十九、Workflow 作用域

Workflow Definition 可绑定：

- 全局；
-角色；
-会话；
-扩展；
-模块。

Workflow 被 Agent 调用时：

```text
Workflow Tool Scope
∩
Invocation Scope
∩
Workflow 内部节点 Scope
```

子节点不得扩大 Scope。

Schedule 创建时必须保存 Scope Snapshot，而不是运行时读取“当前角色”。

---

## 二十、UI Contribution 作用域

UI Contribution 也必须受 Scope 管理。

示例：

- 某角色专属聊天按钮；
-某会话专属侧边面板；
-某扩展设置页；
-某模块桌面小组件；
-某角色消息渲染器。

前端渲染时应向后端或统一前端 Registry 查询：

```text
当前 Scope 可见 Contributions
```

不得让插件前端自行判断角色 ID。

---

## 二十一、后台任务与 Schedule 作用域

后台任务创建时必须持久化：

```text
scope_snapshot
owner
permission_snapshot
created_by_invocation
expiry
```

执行时必须重新验证：

- 角色是否仍存在；
-会话是否仍有效；
-扩展是否启用；
-模块是否启用；
-Scope Binding 是否仍 active；
-Permission Grant 是否仍有效。

禁止后台任务仅凭旧 Snapshot 永久执行。

---

## 二十二、临时作用域

临时 Scope 可用于：

- 一次 Tool 调用；
-开发者测试；
-扩展预览；
-会话临时启用；
-安装向导；
-临时 MCP 连接；
-桌面控制会话。

临时 Scope 必须：

- 有过期时间；
-有所有者；
-有清理任务；
-不可自动转持久；
-应用重启后默认失效；
-除非明确声明可恢复。

---

## 二十三、作用域持久化

建议建立统一表：

```text
scope_bindings
scope_snapshots
scope_audits
scope_migrations
```

Scope Binding 至少包含：

- Binding ID；
-Subject Type；
-Subject ID；
-Scope Type；
-Scope Value；
-State；
-Source；
-Owner；
-Expires At；
-Created At；
-Updated At。

不得继续为每个系统新增：

- plugin_scope_bindings；
-mcp_character_bindings；
-agent_skill_character_bindings；
-workflow_scope；
-tool_scope_enabled。

旧表后续迁移后删除。

---

## 二十四、缓存与失效

Scope Manager 可缓存：

- Subject Binding；
-角色与会话关系；
-当前 Scope Resolution；
-Contribution 可见性。

触发失效：

- Binding 新增；
-Binding 删除；
-Binding 过期；
-角色删除；
-会话删除；
-会话归档；
-扩展禁用；
-模块禁用；
-Tool 禁用；
-MCP Server 删除；
-Workflow 删除；
-Agent Skill 删除；
-资源所有权变化；
-包升级；
-用户切换角色不影响持久缓存，但影响当前查询键。

---

## 二十五、角色删除处理

删除角色前必须预览关联资源：

- Scope Binding；
-Agent Skill；
-MCP Server Binding；
-MCP Tool Binding；
-Workflow；
-Schedule；
-后台任务；
-UI Contribution；
-Permission Grant；
-缓存；
-临时会话；
-执行中的 Invocation。

删除策略：

- 用户资产默认保留但解除角色绑定；
-角色专属扩展资源按策略删除或迁移；
-运行任务取消；
-后台任务禁用；
-历史审计保留；
-缓存立即失效。

不得数据库级无条件级联删除所有扩展资产。

---

## 二十六、会话删除与归档处理

### 删除

- Conversation Binding 失效；
-临时 Tool 移除；
-临时 Agent Skill 移除；
-临时权限失效；
-临时 UI 移除；
-后台任务取消；
-审计保留。

### 归档

根据策略：

- 可继续读取；
-不得主动执行高风险 Tool；
-后台任务默认暂停；
-临时 UI 不再显示；
-会话 Scope 不自动删除。

---

## 二十七、Extension/Module 禁用处理

禁用 Extension：

- Runtime 可停止；
-全部 Contribution 不可见；
-Tool 不可执行；
-Agent Skill 不可激活；
-Workflow 不可运行；
-UI 不可展示；
-后台任务暂停；
-Scope Binding 保留；
-权限 Grant 保留但不可使用；
-数据保留。

禁用 Module：

- 仅影响该 Module；
-不得禁用整个 Extension；
-模块 Binding 保留；
-模块恢复后可恢复可见性。

---

## 二十八、作用域与前端状态

前端应接收统一结果：

```json
{
  "subject": {
    "type": "tool",
    "id": "plugin/com.example.weather/query_weather"
  },
  "bindings": [
    {
      "type": "character",
      "characterId": "character-a",
      "state": "active"
    }
  ],
  "current": {
    "characterId": "character-a",
    "conversationId": "conversation-1"
  },
  "decision": {
    "allowed": true,
    "reasons": []
  }
}
```

前端不得：

- 自行合并 MCP 角色绑定；
-自行判断 Agent Skill Global；
-自行拼接 Tool Scope；
-缓存角色切换结果而不失效；
-把无 Binding 显示为权限拒绝。

---

## 二十九、迁移旧作用域

必须建立映射。

### 旧 Skill Scope Binding

```text
→ scope_bindings(subject=tool)
```

### Agent Skill 角色绑定

```text
→ scope_bindings(subject=agent_skill)
```

### MCP Server 角色绑定

```text
→ scope_bindings(subject=mcp_server)
```

### MCP Tool Scope Enabled

拆分为：

```text
Tool Enabled
+
Tool Scope Binding
```

### Plugin Enabled Scope

拆分为：

```text
Module Enabled
+
Contribution Scope Binding
```

### Workflow Scope

```text
→ scope_bindings(subject=workflow)
```

### Package Binding

Package 本身一般不绑定角色，包内 Contribution 分别绑定。

---

## 三十、迁移约束

迁移期间：

- 新 Binding 只写统一 `scope_bindings`；
-旧表只读；
-禁止双写；
-禁止新代码查询旧表；
-通过 Migration Adapter 将旧 Binding 转换；
-记录未映射数据；
-记录冲突；
-记录无效角色和会话；
-记录重复 Binding；
-记录 Scope 扩大风险。

---

## 三十一、Scope Audit

必须记录：

- 创建 Binding；
-更新 Binding；
-撤销 Binding；
-Binding 过期；
-角色删除清理；
-会话删除清理；
-扩展禁用影响；
-模块禁用影响；
-Scope 拒绝；
-父子 Scope 收窄；
-迁移转换。

审计不得记录完整用户内容。

---

## 三十二、测试要求

必须新增：

### 1. ScopeRef 校验

覆盖所有合法与非法组合。

### 2. Binding 测试

- 创建；
-重复；
-撤销；
-过期；
-查询；
-所有者；
-临时；
-迁移。

### 3. Resolution 测试

覆盖 Current Character、Current Conversation、Owner Extension、Owner Module。

### 4. Evaluation 测试

覆盖：

- Global；
-Character；
-Conversation；
-Extension；
-Module；
-Resource；
-Invocation；
-Session。

### 5. 继承测试

验证子 Scope 不扩大。

### 6. 角色与会话关系测试

覆盖跨角色越权。

### 7. Agent Skill 测试

验证只在匹配 Scope 激活。

### 8. MCP 测试

验证 Server 和 Tool Scope 收窄。

### 9. Plugin 测试

验证 Runtime 全局运行但 Contribution 按 Scope 过滤。

### 10. Workflow 测试

验证节点与 Schedule Scope。

### 11. UI 测试

验证角色切换后 Contribution 实时变化。

### 12. 删除清理测试

角色、会话、Extension、Module、Resource 删除。

### 13. 缓存失效测试

验证所有失效触发。

### 14. 迁移测试

旧 Binding 转新 Binding 语义不扩大。

---

## 三十三、实施任务

### Task 1：定义 Scope 领域模型

完成 ScopeType、ScopeRef、ScopeBinding、ScopeSnapshot、ScopeDecision。

### Task 2：建立 Scope Manager

完成 Bind、Unbind、Resolve、Evaluate、Snapshot、Invalidate、List。

### Task 3：建立统一 Scope 持久化

实现 Binding、Snapshot、Audit。

### Task 4：接入 Tool Availability

Tool 可见性与可执行性使用 Scope Manager。

### Task 5：接入 Permission Broker

Permission Scope 使用统一 ScopeRef。

### Task 6：迁移 Agent Skill Scope

移除独立角色绑定逻辑。

### Task 7：迁移 MCP Scope

统一 Server 和 Tool Binding。

### Task 8：迁移 Plugin Scope

统一 Extension、Module 和 Contribution Scope。

### Task 9：迁移 Workflow Scope

统一 Workflow 与 Schedule Scope。

### Task 10：接入 UI Contribution 预留接口

为后续 UI Registry 使用统一 Scope 查询。

### Task 11：建立后台任务 Scope Snapshot

确保异步任务作用域稳定且可重新验证。

### Task 12：实现角色和会话清理

处理 Binding、任务、缓存和临时资源。

### Task 13：建立迁移适配器

只读转换旧 Scope 数据。

### Task 14：增加迁移统计

统计仍使用旧 Binding 的入口。

### Task 15：完成回归和越权测试

确保无 Scope 扩大。

---

## 三十四、建议目录结构

建议：

```text
backend/internal/extension/kernel/scope/
├── type.go
├── ref.go
├── binding.go
├── expression.go
├── resolution.go
├── snapshot.go
├── decision.go
├── manager.go
├── storage.go
├── cache.go
├── cleanup.go
├── migration.go
└── audit.go
```

前端：

```text
front/src/views/extensions/scopes/
├── ScopeBindingList.vue
├── ScopeBindingEditor.vue
├── ScopeDecisionView.vue
└── ScopeMigrationReport.vue
```

目录仅为建议。

---

## 三十五、风险控制

### P0：跨角色数据泄露

- Conversation 与 Character 未校验；
-子调用扩大 Scope；
-后台任务使用旧角色上下文；
-MCP Tool 全局暴露；
-Plugin UI 读取其他角色。

### P1：状态漂移

- 新旧 Binding 不一致；
-角色删除后 Binding 残留；
-缓存未失效；
-前端仍显示旧角色 Tool。

### P2：过度限制

- Global Tool 被错误限制；
-共享 MCP 无法复用；
-Workflow 子调用被错误拒绝；
-后台任务全部失效。

### P3：模型复杂度

- Scope Expression 过度复杂；
-前端难以解释；
-迁移规则难以验证。

---

## 三十六、本步骤不做的事情

本步骤明确不做：

- 不重构用户账号体系；
-不实现多用户 Scope；
-不实现移动端 Scope；
-不实现完整 `.amitiax` v2；
-不实现第三方插件 Runtime；
-不实现 UI Contribution；
-不删除旧 Scope 表；
-不删除旧角色绑定 API；
-不实现插件市场；
-不增加新角色功能；
-不改变现有角色和会话业务语义。

---

## 三十七、验收产物

完成后必须提交：

### 1. 统一 Scope 主文档

```text
docs/extension-kernel/10-unified-scope-model.md
```

### 2. Scope 领域类型

至少包含：

- ScopeType；
-ScopeRef；
-ScopeBinding；
-ScopeSnapshot；
-ScopeDecision；
-ScopeReason。

### 3. Scope Manager

实现：

- Bind；
-Unbind；
-Resolve；
-Evaluate；
-Snapshot；
-Invalidate；
-List。

### 4. Scope 持久化

包含 Binding、Snapshot 和 Audit。

### 5. 旧作用域迁移映射

覆盖：

- Skill；
-Agent Skill；
-MCP；
-Plugin；
-Workflow；
-Package。

### 6. 删除与失效规则

覆盖：

- 角色；
-会话；
-扩展；
-模块；
-Tool；
-MCP Server；
-Workflow；
-Agent Skill。

### 7. 前端统一状态接口

可展示 Scope Binding 和当前 Decision。

### 8. 迁移统计报告

列出：

- 已迁移 Binding；
-重复 Binding；
-无效角色；
-无效会话；
-Scope 扩大风险；
-仍使用旧表的入口。

### 9. 测试报告

覆盖：

- 解析；
-绑定；
-评估；
-继承；
-清理；
-缓存；
-迁移；
-越权。

---

## 三十八、验收标准

本步骤通过必须满足：

1. 全局、角色、会话、扩展、模块、资源、Invocation 和 Session Scope 已统一定义。
2. Scope 与 Enabled 已分离。
3. Scope 与 Permission 已分离。
4. Scope 与 Availability 已分离。
5. Scope Manager 成为唯一作用域判定入口。
6. Tool、Agent Skill、MCP、Plugin、Workflow 均使用统一 Scope。
7. 父子调用 Scope 不得扩大。
8. Conversation 必须校验 Character 归属。
9. 后台任务保存并重新验证 Scope Snapshot。
10. 角色和会话删除有完整清理规则。
11. Extension 和 Module 禁用不删除 Binding。
12. 新 Binding 只写统一存储。
13. 旧 Binding 只读迁移。
14. 前端不再自行组合作用域状态。
15. 缓存失效正确。
16. 跨角色越权测试通过。
17. 后续第 11 步可以统一运行记录与审计模型。

---

## 三十九、退出条件

只有满足以下条件后，才能进入第 11 步“统一运行记录与审计模型”：

- Scope 类型已锁定；
-Scope Manager 已落地；
-统一 Binding 已落地；
-Tool Availability 已接入；
-Permission Broker 已接入；
-Agent Skill、MCP、Plugin、Workflow 已接入；
-角色和会话清理已实现；
-旧 Scope 写入已停止；
-迁移统计可用；
-越权与继承测试通过。

---

## 四十、执行约束

执行本步骤时必须遵守：

> 作用域只负责表达“某对象在哪个范围内有效”，不得继续承担权限、启用、连接、安装或健康状态。

禁止出现：

- 有角色 Binding 就自动授权；
-MCP Server 已连接就自动全局可用；
-Extension 全局安装就自动对所有角色启用；
-Tool Enabled 字段存角色 ID；
-前端当前角色直接修改后端持久 Binding；
-后台任务运行时读取“当前角色”；
-Conversation ID 不验证 Character；
-子调用创建更宽 Scope；
-新旧 Scope 双写；
-旧 Binding 长期保留为回退逻辑。

本步骤完成后，Amitia 必须具备一套统一、稳定、可继承、可收窄、可失效、可审计的作用域基础。
