# Amitia 扩展系统重构第 9 步实施文档

## 第 9 步：重构统一权限代理

---

## 一、步骤目标

在第 8 步已经建立统一执行安全内核的基础上，重构 Amitia 当前分散在 Skill、Plugin、MCP、Workflow、Agent Skill、扩展包、前端审批和桌面能力中的权限逻辑，建立唯一的 Permission Broker。

本步骤的目标是：

> 让所有扩展能力的权限声明、授权、审批、作用域、撤销、过期、审计和前端展示都经过同一套权限系统。

当前系统中的权限判断可能分别存在于：

- Skill Permission；
- Plugin Host API；
- MCP Server 与 Tool 启用；
- Agent Skill 依赖确认；
- Workflow 权限；
- Package 安装风险确认；
- Scope Binding；
-角色绑定；
-聊天调用链；
-前端手动确认；
-Electron 桌面能力；
-Computer Use 审批模式。

这些机制必须统一为：

```text
Permission Declaration
→ Permission Resolution
→ Scope Evaluation
→ Policy Evaluation
→ Approval Decision
→ Grant Storage
→ Runtime Enforcement
→ Audit
```

本步骤完成后，必须消除以下问题：

- 同一能力被多套权限系统重复判断；
- Manifest 声明与运行调用不一致；
- Tool Enabled 被误当成权限；
- MCP 连接状态被误当成授权；
- 角色绑定与权限混在一起；
- 前端弹窗决定实际权限结果；
- Plugin Host API 自行做授权；
- Workflow 子调用权限提升；
- Agent Skill 安装 MCP 后默认拥有过多权限；
- 权限撤销后缓存未失效；
- 高风险权限没有输入或目标绑定；
- 长期授权没有过期与范围；
- 扩展升级新增权限未重新确认。

---

## 二、权限系统职责边界

Permission Broker 负责：

- 权限命名与分类；
-权限声明；
-权限请求；
-权限解析；
-授权存储；
-作用域绑定；
-授权有效期；
-一次性授权；
-持久授权；
-拒绝；
-撤销；
-权限升级检测；
-高风险审批；
-可信扩展策略；
-调用时强制校验；
-权限缓存失效；
-权限审计；
-前端权限状态输出。

Permission Broker 不负责：

- Tool 是否安装；
-Tool 是否启用；
-Runtime 是否 Ready；
-MCP 是否连接；
-Plugin 是否崩溃；
-Workflow 是否有效；
-扩展包安装事务；
-UI 渲染；
-具体业务操作；
-Tool 输入 Schema；
-执行结果校验。

权限与可用性必须分离。

---

## 三、核心术语

## 1. Permission Definition

表示 Amitia 支持的一项权限能力。

示例：

```text
network.request
files.read
files.write
files.delete
conversation.read
conversation.write
message.send
memory.read
memory.write
character.read
character.write
scheduler.create
notification.send
desktop.capture
desktop.control
process.spawn
mcp.manage
workflow.manage
provider.use
secrets.read
ui.contribute
```

每个权限必须有：

- 稳定 ID；
-显示名称；
-描述；
-风险等级；
-允许作用域；
-默认审批策略；
-是否可持久授权；
-是否必须每次确认；
-是否允许后台调用；
-是否允许子调用继承；
-是否允许扩展声明；
-是否只允许官方模块。

---

## 2. Permission Requirement

表示某个 Tool、Hook、Provider 或 UI Contribution 执行前所需权限。

建议：

```go
type PermissionRequirement struct {
    PermissionID string
    Scope        PermissionScopeRequirement
    Conditions   []PermissionCondition
    Optional     bool
}
```

---

## 3. Permission Grant

表示用户或系统已经授予的权限。

建议：

```go
type PermissionGrant struct {
    GrantID       string
    Subject       PermissionSubject
    PermissionID  string
    Scope         PermissionScope
    Decision      PermissionDecision
    InputBinding  *InputBinding
    TargetBinding *TargetBinding
    IssuedAt      time.Time
    ExpiresAt     *time.Time
    IssuedBy      GrantIssuer
    Reason        string
    RevokedAt     *time.Time
    Metadata      map[string]any
}
```

---

## 4. Permission Subject

权限主体可以是：

```text
system
user
extension
module
tool
workflow
mcp_server
provider
runtime
```

权限授权必须绑定明确主体，不能只保存“已允许某权限”。

---

## 5. Permission Scope

权限作用域建议支持：

```text
global
character
conversation
extension
module
tool
resource
target
invocation
temporary_session
```

例如：

```text
允许扩展 com.example.weather
在当前角色范围
读取当前位置
仅调用 api.weather.example
```

---

## 四、权限分类

建议划分为以下类别。

## 1. 宿主数据权限

包括：

```text
character.read
character.write
conversation.read
conversation.write
message.read
message.send
memory.read
memory.write
memory.delete
model.context.read
```

---

## 2. 文件与系统权限

包括：

```text
files.read
files.write
files.delete
files.list
files.watch
process.spawn
process.signal
system.info
clipboard.read
clipboard.write
```

---

## 3. 网络权限

包括：

```text
network.request
network.websocket
network.listen
network.download
network.upload
```

必须支持域名、协议、端口和方法范围。

---

## 4. 桌面能力权限

包括：

```text
desktop.capture
desktop.input
desktop.window.read
desktop.window.control
desktop.notification
desktop.tray
desktop.widget
computer_use.observe
computer_use.control
```

---

## 5. 扩展平台权限

包括：

```text
extensions.read
extensions.install
extensions.enable
extensions.disable
extensions.update
extensions.uninstall
extensions.invoke
ui.contribute
hooks.register
events.subscribe
scheduler.create
storage.read
storage.write
secrets.read
secrets.write
```

---

## 6. MCP 权限

包括：

```text
mcp.server.create
mcp.server.connect
mcp.server.manage
mcp.tools.invoke
mcp.resources.read
mcp.prompts.read
mcp.sampling.request
mcp.elicitation.request
```

---

## 7. Workflow 权限

包括：

```text
workflow.read
workflow.execute
workflow.create
workflow.update
workflow.delete
workflow.schedule
```

---

## 8. Provider 权限

包括：

```text
provider.use
provider.configure
provider.secrets.read
provider.register
```

---

## 五、权限命名规范

权限 ID 统一采用：

```text
<domain>.<action>
```

必要时增加层级：

```text
<domain>.<resource>.<action>
```

示例：

```text
mcp.server.connect
desktop.window.control
memory.long_term.write
extensions.package.install
```

要求：

- 小写；
-ASCII；
-稳定；
-不因显示名称变化；
-不因平台变化；
-禁止数据库自增 ID；
-禁止复用旧 Skill Permission 名称而不审查语义；
-权限重命名必须提供迁移映射。

---

## 六、Permission Definition Registry

建立系统级 Permission Definition Registry。

建议：

```go
type PermissionDefinition struct {
    ID                   string
    Name                 LocalizedText
    Description          LocalizedText
    Category             PermissionCategory
    RiskLevel            RiskLevel
    AllowedScopes        []PermissionScopeType
    PersistentGrantable  bool
    RequiresPerUse       bool
    BackgroundAllowed    bool
    ChildInvocationPolicy ChildInvocationPolicy
    TrustedOnly          bool
    DefaultApproval      ApprovalMode
}
```

Registry 负责：

- 权限定义；
-权限元数据；
-风险；
-作用域；
-默认策略；
-兼容映射；
-前端展示信息。

Registry 不负责：

- 存储授权；
-执行 Tool；
-判断 Runtime；
-管理 Scope Binding。

---

## 七、权限声明来源

权限声明可以来自：

- 系统内置 Tool；
-`.amitiax` Manifest；
-插件 Tool Contribution；
-MCP Tool Adapter；
-Workflow Definition；
-Provider Contribution；
-Computer Use 能力；
-内部 Tool。

所有声明必须在注册时校验。

禁止：

- Runtime 执行过程中临时声明新权限；
-Plugin 代码自行拼接未知权限；
-MCP Server 返回任意宿主权限；
-Agent Skill 通过文字说明获得宿主权限；
-前端直接增加权限定义。

---

## 八、Manifest 权限声明

未来 `.amitiax` Manifest 中权限应区分：

```text
declaredPermissions
optionalPermissions
runtimePermissions
contributionPermissions
```

示例：

```json
{
  "permissions": [
    {
      "id": "network.request",
      "scope": {
        "domains": ["api.weather.example"],
        "methods": ["GET"]
      },
      "required": true
    },
    {
      "id": "ui.contribute",
      "scope": {
        "slots": ["chat.composer.actions"]
      },
      "required": false
    }
  ]
}
```

要求：

- 未声明权限运行时不得请求；
-权限范围不得大于 Manifest；
-升级扩大权限时必须重新确认；
-缩小权限可自动应用；
-高风险权限必须单独显示；
-权限声明必须进入签名校验范围。

---

## 九、Permission Broker 接口

建议定义：

```go
type PermissionBroker interface {
    Evaluate(
        ctx context.Context,
        request PermissionEvaluationRequest,
    ) PermissionEvaluationResult

    Grant(
        ctx context.Context,
        grant PermissionGrantRequest,
    ) (PermissionGrant, error)

    Revoke(
        ctx context.Context,
        grantID string,
    ) error

    ListGrants(
        ctx context.Context,
        filter PermissionGrantFilter,
    ) ([]PermissionGrant, error)

    Explain(
        ctx context.Context,
        request PermissionEvaluationRequest,
    ) PermissionExplanation
}
```

---

## 十、权限评估请求

建议：

```go
type PermissionEvaluationRequest struct {
    Subject       PermissionSubject
    Requirements  []PermissionRequirement
    Invocation    ToolInvocationContext
    Input         json.RawMessage
    Target        PermissionTarget
    RiskLevel     RiskLevel
    SideEffects   []ExpectedSideEffect
}
```

评估结果：

```go
type PermissionEvaluationResult struct {
    Decision        PermissionDecision
    Missing         []PermissionRequirement
    MatchedGrants   []PermissionGrant
    ApprovalRequest *ApprovalRequest
    Reasons         []PermissionReason
}
```

---

## 十一、权限决策

统一决策：

```text
allow
deny
require_approval
allow_once
allow_until_expiry
allow_persistent
```

内部最终执行阶段只能接受：

```text
allow
deny
require_approval
```

其他决策属于用户授权结果。

---

## 十二、作用域匹配

Permission Broker 必须与第 10 步将建立的统一 Scope 模型兼容。

当前先实现权限作用域匹配规则：

### 全局

适用于所有角色、会话和资源。

### 角色

仅指定角色。

### 会话

仅指定会话。

### 扩展

仅指定扩展。

### 模块

仅指定扩展模块。

### Tool

仅指定 Tool。

### 资源

例如：

- 指定文件目录；
-指定 MCP Server；
-指定记忆库；
-指定频道；
-指定 Provider。

### 目标

例如：

- 指定域名；
-指定窗口；
-指定联系人；
-指定路径；
-指定 URL。

---

## 十三、条件权限

权限必须支持结构化条件。

示例：

### 网络

```json
{
  "domains": ["api.weather.example"],
  "methods": ["GET"],
  "ports": [443]
}
```

### 文件

```json
{
  "paths": ["${extensionData}/**"],
  "operations": ["read", "write"]
}
```

### 会话

```json
{
  "characterIds": ["current"],
  "conversationIds": ["active"],
  "historyLimit": 20
}
```

### 消息发送

```json
{
  "channels": ["web"],
  "characterIds": ["current"],
  "maxPerHour": 5
}
```

### UI

```json
{
  "slots": [
    "chat.composer.actions",
    "settings.sections"
  ]
}
```

权限条件必须由宿主解析，不允许插件自行解释。

---

## 十四、授权模式

支持以下用户授权模式：

### 1. 仅本次

绑定：

```text
invocation_id
input_hash
target
```

执行结束立即失效。

### 2. 本次会话

绑定：

```text
conversation_id
或 temporary_session_id
```

会话结束后失效。

### 3. 当前角色

绑定指定 Character。

### 4. 当前扩展

绑定 Extension 和 Permission Scope。

### 5. 始终允许

仅对允许持久授权的权限。

### 6. 拒绝

可分为：

- 本次拒绝；
-持续拒绝；
-禁止再次请求；
-由系统策略拒绝。

---

## 十五、高风险权限规则

以下类型默认不得静默持久授权：

- 文件删除；
-进程启动；
-系统控制；
-Computer Use；
-消息批量发送；
-记忆删除；
-扩展安装；
-扩展卸载；
-MCP stdio 命令执行；
-网络监听；
-Secret 读取；
-桌面输入控制；
-Provider Secret 访问。

可以根据用户选择的审批模式放宽，但必须：

- 记录模式；
-显示风险；
-支持随时撤销；
-作用域最小化；
-高风险操作仍保留审计。

---

## 十六、权限继承

父子调用权限规则：

```text
子调用不得扩大父调用权限
```

例如：

```text
Plugin Tool
→ 调用 Workflow
→ 调用 MCP Tool
```

子调用可使用：

- 父调用已授予权限；
-子调用自身更小范围权限；
-系统明确允许继承的权限。

禁止：

- Workflow 使用 Plugin 未声明权限；
-MCP Tool 继承用户全局 Secret；
-后台任务继承前台一次性授权；
-子调用将角色 Scope 扩大为全局。

---

## 十七、后台与定时任务权限

后台任务必须有独立权限快照。

不得直接依赖：

- 当前前端页面；
-当前会话临时授权；
-刚刚一次审批；
-用户当前登录状态；
-运行时内存缓存。

后台任务创建时必须记录：

```text
subject
permissions
scope
expiry
created_by
approval_reference
```

权限撤销后：

- 尚未执行任务必须失效或暂停；
-正在执行任务按策略取消；
-不得继续使用过期 Grant。

---

## 十八、Agent Skill 权限规则

Agent Skill 本身不拥有执行权限。

Agent Skill 只可以声明：

- 需要哪些 Tool；
-需要哪些 MCP；
-需要哪些资源；
-激活条件。

真正执行 Tool 时，仍由：

```text
Tool
→ Permission Broker
```

进行授权。

禁止：

- 安装 Agent Skill 即自动获得所有 Tool 权限；
-Agent Skill 文本绕过权限；
-Agent Skill MCP 依赖自动开启高风险 stdio；
-Agent Skill 通过内部激活 Tool 获取宿主数据权限。

---

## 十九、MCP 权限规则

MCP 相关权限分为三层：

### 1. Server 生命周期权限

- 创建；
-连接；
-断开；
-修改；
-删除。

### 2. 协议能力权限

- Tools；
-Resources；
-Prompts；
-Sampling；
-Elicitation；
-Roots。

### 3. Tool 执行权限

每个 MCP Tool 映射到 ToolDefinition 后，使用统一 Tool 权限。

不得仅因为 MCP Server 已连接，就默认允许全部 Tool。

---

## 二十、Plugin Host API 权限规则

Plugin Host API 每个方法必须绑定权限。

示例：

```text
amitia.files.read → files.read
amitia.network.request → network.request
amitia.message.send → message.send
amitia.memory.search → memory.read
amitia.memory.write → memory.write
amitia.scheduler.create → scheduler.create
amitia.ui.register → ui.contribute
```

Host API Gateway 必须：

- 验证扩展身份；
-验证 Manifest 声明；
-验证 Grant；
-验证 Scope；
-记录审计；
-禁止 Plugin 自行绕过；
-禁止直接访问 Go Service。

---

## 二十一、UI Contribution 权限

UI 扩展也必须受权限约束。

例如：

- 注册导航；
-注册聊天按钮；
-注册消息渲染器；
-打开独立窗口；
-使用系统托盘；
-显示通知；
-覆盖 UI Slot；
-读取当前角色；
-读取消息内容。

UI 可见不代表可以读取宿主数据。

插件 UI 只能通过 Host API 获取数据，并再次经过 Permission Broker。

---

## 二十二、权限持久化

建议建立统一权限表：

```text
permission_definitions
permission_grants
permission_denials
permission_audits
permission_migrations
```

若已有表可复用，应在后续迁移步骤处理。

Grant 至少持久化：

- Grant ID；
-Subject；
-Permission ID；
-Scope；
-Conditions；
-Decision；
-Issued At；
-Expires At；
-Revoked At；
-Issued By；
-Manifest Version；
-Input Binding；
-Target Binding；
-Approval Reference；
-Metadata。

---

## 二十三、缓存与失效

Permission Broker 可缓存评估结果，但必须支持失效。

触发失效：

- Grant 新增；
-Grant 撤销；
-Grant 过期；
-扩展禁用；
-模块禁用；
-角色删除；
-会话结束；
-Manifest 权限变化；
-Tool Definition 变化；
-用户审批模式变化；
-系统策略变化；
-扩展卸载。

不得使用长期不可失效的内存授权缓存。

---

## 二十四、权限升级检测

扩展更新时必须比较：

- 新增权限；
-扩大 Scope；
-增加域名；
-增加路径；
-增加 Tool；
-提高风险等级；
-允许后台执行；
-允许持久授权；
-增加 Secret；
-增加 Computer Use。

任何权限扩大必须：

- 阻止自动启用；
-展示差异；
-重新授权；
-保留旧版本可回滚；
-记录审计。

权限缩小可以自动生效，但必须清理多余 Grant。

---

## 二十五、权限撤销

支持：

- 按 Grant 撤销；
-按扩展撤销；
-按模块撤销；
-按 Tool 撤销；
-按权限撤销；
-按角色撤销；
-按会话撤销；
-全部撤销。

撤销后必须：

- 立即失效缓存；
-阻止新调用；
-处理排队调用；
-处理后台任务；
-处理长任务；
-更新前端；
-写审计；
-不删除历史记录。

---

## 二十六、Permission Explanation

前端和开发者控制台需要可解释权限结果。

示例：

```json
{
  "decision": "require_approval",
  "reasons": [
    {
      "code": "missing_persistent_grant",
      "permission": "files.write"
    },
    {
      "code": "target_outside_allowed_path",
      "target": "C:/Users/..."
    }
  ],
  "matchedGrants": [],
  "requiredAction": "manual_approval"
}
```

不得只返回：

```text
permission denied
```

---

## 二十七、前端权限管理

前端至少需要：

- 扩展安装权限预览；
-扩展详情权限列表；
-已授权范围；
-拒绝记录；
-过期时间；
-撤销按钮；
-权限变化提示；
-高风险标记；
-后台权限标记；
-审批历史；
-按角色和会话查看；
-搜索与筛选。

前端不得自行决定授权结果。

---

## 二十八、与 Execution Security Kernel 的关系

第 8 步中的：

```text
PermissionGate
ApprovalGate
```

应调用 Permission Broker。

执行链：

```text
ExecutionSecurityKernel
→ PermissionGate
→ PermissionBroker.Evaluate
→ ApprovalGate
→ PermissionBroker.Grant/Record
→ Runtime Dispatch
```

Permission Broker 不得调用 ToolExecutor，避免循环依赖。

---

## 二十九、迁移旧权限

需要建立迁移映射：

### Skill Permission

```text
旧 Skill Permission
→ Tool Permission Requirement
```

### Plugin Permission

```text
Plugin Host Capability
→ Permission Definition + Grant
```

### MCP Tool Enabled

不得迁移为 Grant，只迁移为 Capability Enabled。

### MCP Server 角色绑定

迁移为 Scope Binding，不迁移为权限。

### Package 安装确认

拆分为：

- 安装风险确认；
-权限授权；
-运行时风险确认。

### Agent Skill MCP 确认

拆分为：

- 依赖安装确认；
-MCP 生命周期权限；
-Tool 执行权限。

---

## 三十、兼容层约束

迁移期间允许：

```text
旧 PermissionEvaluator
→ 转换 PermissionEvaluationRequest
→ PermissionBroker
```

禁止：

```text
PermissionBroker
→ 旧 PermissionEvaluator
→ 新 PermissionBroker
```

禁止双写：

```text
旧权限表
+
新权限表
```

新 Grant 只写新系统。

旧数据通过只读迁移转换。

---

## 三十一、测试要求

必须新增：

### 1. Permission Definition 测试

- ID；
-分类；
-风险；
-Scope；
-持久授权；
-后台；
-继承；
-可信限制。

### 2. Grant 测试

- 创建；
-读取；
-过期；
-撤销；
-作用域；
-输入绑定；
-目标绑定；
-重复；
-冲突。

### 3. 评估测试

覆盖：

- Allow；
-Deny；
-Approval；
-多权限；
-Optional；
-条件；
-角色；
-会话；
-扩展；
-Tool；
-资源。

### 4. 继承测试

覆盖父子调用权限收窄。

### 5. 后台任务测试

覆盖 Grant 快照、过期、撤销。

### 6. MCP 测试

覆盖 Server、协议能力和 Tool 执行三层权限。

### 7. Plugin Host API 测试

每个 Host API 必须有权限映射。

### 8. 权限升级测试

覆盖新增权限、扩大 Scope、提高风险。

### 9. 缓存失效测试

覆盖撤销、过期、禁用、升级、角色删除。

### 10. 敏感信息测试

权限解释和审计不得泄露 Secret。

---

## 三十二、实施任务

### Task 1：定义权限术语与模型

完成 PermissionDefinition、Requirement、Grant、Subject、Scope、Decision。

### Task 2：建立 Permission Definition Registry

注册系统内置权限定义。

### Task 3：建立 Permission Broker

完成 Evaluate、Grant、Revoke、List、Explain。

### Task 4：建立权限持久化

实现 Grant、Denial、Audit 存储。

### Task 5：建立作用域与条件匹配

支持角色、会话、扩展、Tool、资源和目标。

### Task 6：接入 Execution Security Kernel

替换旧 PermissionGate 实现。

### Task 7：迁移内置 Tool 权限

将旧 Skill 权限转换为 Tool Requirement。

### Task 8：迁移 MCP 权限

分离 Server、协议能力和 Tool 权限。

### Task 9：迁移 Plugin Host API 权限

为每个 API 建立权限映射。

### Task 10：迁移 Workflow 权限

确保 Workflow 与子 Tool 不提升权限。

### Task 11：迁移 Agent Skill 权限

移除 Agent Skill 自动权限语义。

### Task 12：建立权限升级检测

用于后续 `.amitiax` 更新。

### Task 13：建立权限撤销与缓存失效

确保实时生效。

### Task 14：建立统一权限 API

为前端和开发者控制台提供查询。

### Task 15：增加迁移统计

统计仍使用旧 PermissionEvaluator 的入口。

### Task 16：完成安全与回归测试

验证权限无绕过。

---

## 三十三、建议目录结构

建议：

```text
backend/internal/extension/kernel/permission/
├── definition.go
├── registry.go
├── subject.go
├── scope.go
├── condition.go
├── requirement.go
├── grant.go
├── decision.go
├── broker.go
├── evaluator.go
├── storage.go
├── cache.go
├── migration.go
├── audit.go
└── explanation.go
```

前端：

```text
front/src/views/extensions/permissions/
├── PermissionOverview.vue
├── PermissionGrantList.vue
├── PermissionDetail.vue
├── PermissionApprovalDialog.vue
└── PermissionChangeReview.vue
```

目录仅为建议，不得为了目录形式搬迁无关代码。

---

## 三十四、风险控制

### P0：越权

- 未声明权限可调用；
-撤销后仍可执行；
-子调用提升权限；
-后台任务使用前台临时授权；
-目标范围绕过；
-Plugin Host API 漏校验。

### P1：错误授权

- Tool Enabled 被迁移为 Grant；
-MCP 连接被视为授权；
-角色绑定被视为权限；
-输入变化仍复用一次授权；
-扩展升级不重新确认。

### P2：状态漂移

- 前端显示已授权但后端拒绝；
-缓存未失效；
-多份 Grant 冲突；
-旧权限表继续写入。

### P3：可用性问题

- 权限过细导致频繁弹窗；
-解释不清；
-持久授权范围过大；
-开发者无法定位拒绝原因。

---

## 三十五、本步骤不做的事情

本步骤明确不做：

- 不完成统一 Scope 数据模型全部重构；
-不实现 `.amitiax` v2；
-不实现完整插件 Runtime；
-不实现 UI Contribution；
-不重建扩展中心全部页面；
-不删除旧权限表；
-不删除旧 PermissionEvaluator；
-不实现移动端权限；
-不实现操作系统级沙箱；
-不实现插件市场审核；
-不增加新的高风险能力。

---

## 三十六、验收产物

完成后必须提交：

### 1. 权限代理主文档

```text
docs/extension-kernel/09-unified-permission-broker.md
```

### 2. 权限定义清单

包含全部 Permission ID、分类、风险、Scope 和默认策略。

### 3. Permission Broker 代码

包含：

- Evaluate；
-Grant；
-Revoke；
-List；
-Explain。

### 4. 权限持久化

包含 Grant、Denial、Audit。

### 5. Tool 权限映射

列出所有现有 Tool 的权限要求。

### 6. MCP 权限映射

区分 Server、协议能力和 Tool 执行。

### 7. Plugin Host API 权限映射

每个 API 必须有明确权限。

### 8. 权限升级检测

可比较旧 Manifest 与新 Manifest 权限差异。

### 9. 前端权限查询接口

能够展示授权、范围、过期、风险和撤销。

### 10. 迁移报告

列出：

- 已迁移权限；
-仍使用旧系统；
-旧表读取；
-重复 Grant；
-无法映射项。

### 11. 安全测试报告

覆盖越权、继承、撤销、过期、后台、升级和缓存失效。

---

## 三十七、验收标准

本步骤通过必须满足：

1. 所有权限有稳定 Permission ID。
2. 权限定义与授权数据分离。
3. Permission Broker 成为唯一权限判定入口。
4. Tool Enabled 不再表示授权。
5. Scope Binding 不再表示授权。
6. MCP Connection 不再表示授权。
7. Agent Skill 不再自动获得 Tool 权限。
8. Plugin Host API 全部经过 Permission Broker。
9. Workflow 子调用不得提升权限。
10. 一次性授权绑定 Invocation、Input 和 Target。
11. 持久授权有 Scope 和撤销能力。
12. 权限升级可被检测并重新确认。
13. 撤销和过期可实时失效。
14. 前端可解释权限结果。
15. 旧 PermissionEvaluator 不再接收新逻辑。
16. 安全测试通过。
17. 后续第 10 步可以正式统一角色、会话和全局作用域。

---

## 三十八、退出条件

只有满足以下条件后，才能进入第 10 步“统一角色、会话和全局作用域”：

- Permission Definition Registry 已建立；
-Permission Broker 已落地；
-Grant 持久化已落地；
-Execution Security Kernel 已接入；
-内置、MCP、Plugin、Workflow、Agent Skill 权限语义已拆分；
-权限升级和撤销可用；
-缓存失效正确；
-旧权限入口已有迁移统计；
-未新增双写；
-关键安全测试通过。

---

## 三十九、执行约束

执行本步骤时必须遵守：

> 权限系统只回答“谁在什么范围内能否做什么”，不得继续承担启用状态、连接状态、生命周期状态或运行健康状态。

禁止出现：

- Permission Broker 启动 MCP；
-Permission Broker 启用 Tool；
-Permission Broker 修改扩展安装状态；
-前端审批结果直接绕过后端；
-Plugin Runtime 自行缓存永久授权；
-Workflow 使用父调用之外的权限；
-Agent Skill 文本声明被视为授权；
-扩展升级后静默扩大权限；
-一次性授权无限复用；
-权限撤销后继续执行后台任务。

本步骤完成后，Amitia 必须具备一套统一、最小授权、可解释、可撤销、可审计的权限基础。
