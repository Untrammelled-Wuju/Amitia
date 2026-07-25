# Amitia 扩展系统重构第 15 步实施文档

## 第 15 步：重构 Agent Skill Loader

---

## 一、步骤目标

在第 6 步已经解除 Skill 概念过载、第 7 步已经建立统一 Tool/Capability 模型、第 10 步已经统一 Scope、第 12 步已经统一资源所有权、第 13 步已经抽取包安全基础设施、第 14 步已经抽取 MCP 协议基础设施的基础上，正式重构 Amitia 的 Agent Skill Loader。

本步骤的目标是：

> 将 Agent Skill 从旧 Skill Registry、旧 SkillDefinition、旧 MCP 依赖安装链和分散缓存中彻底分离，建立独立的 Agent Skill Definition、Catalog、Loader、Activation、Resource Index、Token Budget、Dependency Resolution 和 Runtime Access 模型。

完成本步骤后，Agent Skill 必须被明确定位为：

```text
指令 + 知识 + 资源 + 激活规则 + 依赖声明
```

而不是：

```text
可执行 Tool
```

本步骤必须解决以下历史问题：

1. Agent Skill 本体被包装成 `SkillDefinition`。
2. Agent Skill 可能存在 `Handler == nil` 的伪 Tool。
3. Agent Skill 激活、资源读取和 MCP 依赖混在一个 Runtime 中。
4. Agent Skill 的全局、角色和会话作用域使用独立绑定逻辑。
5. SKILL.md、Frontmatter、资源索引、Artifact 和缓存之间缺乏唯一源数据。
6. Token 预算、Prompt 注入和资源渐进加载缺乏统一规则。
7. Agent Skill 删除时可能遗留 MCP Dependency、Cache、Round State 和 Artifact。
8. Agent Skill 更新时旧资源、旧依赖和旧索引可能不一致。
9. Agent Skill 可能通过文本或安装过程间接获得 Tool 权限。
10. Agent Skill 的 MCP 声明可能直接创建和拥有 MCP Server，所有权不清。
11. 安装预览、激活和资源读取缺乏统一审计。
12. 同一 Agent Skill 可能被重复导入、重复缓存或重复激活。
13. 前端把 Agent Skill 与 Tool 混在“技能”概念中。
14. Agent Skill 的版本、兼容性和资源变更缺乏稳定对比。

本步骤完成后，系统必须形成以下唯一链路：

```text
Secure Package Input
→ Agent Skill Parser
→ Agent Skill Definition
→ Resource Index
→ Compatibility Validation
→ Ownership Registration
→ Catalog Registration
→ Scope Binding
→ Dependency Resolution
→ Activation Evaluation
→ Prompt Contribution
→ On-demand Resource Access
→ Audit
```

---

## 二、职责边界

Agent Skill Loader 负责：

- 读取安全层提供的 Sealed Staging；
-定位和读取 `SKILL.md`；
-解析 Frontmatter；
-解析指令正文；
-建立资源索引；
-建立资源类型；
-计算内容 Hash；
-校验版本和兼容性；
-校验 Tool 引用；
-校验 MCP 引用；
-生成 AgentSkillDefinition；
-注册 Agent Skill Catalog；
-管理 Definition Cache；
-管理资源索引 Cache；
-提供激活所需元数据；
-提供按需资源读取；
-提供安装、更新、删除所需资源清单；
-生成兼容性报告；
-生成变更报告；
-写入统一审计；
-接入统一 Scope；
-接入统一资源所有权。

Agent Skill Loader 不负责：

- 执行 Tool；
-授予 Tool 权限；
-启动 MCP Server；
-管理 MCP 连接；
-管理 Workflow；
-直接向模型发送 Prompt；
-管理扩展包事务；
-写入旧 Skill Registry；
-注册 `SkillDefinition`；
-直接操作前端；
-决定扩展是否启用；
-执行脚本；
-运行 Agent Skill 包内代码。

---

## 三、目标组件

建议拆分为：

```text
AgentSkillLoader
├── SkillSourceReader
├── SkillManifestParser
├── SkillInstructionParser
├── SkillResourceScanner
├── SkillResourceIndexer
├── SkillDependencyParser
├── SkillCompatibilityValidator
├── SkillIntegrityValidator
├── SkillDefinitionBuilder
├── AgentSkillCatalog
├── AgentSkillActivationService
├── AgentSkillResourceService
├── AgentSkillTokenBudgeter
├── AgentSkillCache
├── AgentSkillChangeDetector
├── AgentSkillMigrationAdapter
└── AgentSkillAuditWriter
```

---

## 四、Agent Skill 领域定义

建议定义：

```go
type AgentSkillDefinition struct {
    ID              string
    ExtensionID     string
    ModuleID        string
    Name            LocalizedText
    Description     LocalizedText
    Version         string
    SchemaVersion   int

    Instructions    SkillInstructionRef
    Activation      SkillActivationRule
    Resources       []SkillResourceDescriptor
    RequiredTools   []ToolReference
    RequiredMCP     []MCPReference
    TokenPolicy     SkillTokenPolicy
    Compatibility   SkillCompatibility
    ScopeRule       ScopeRule
    Integrity       SkillIntegrity
    Metadata        map[string]any
}
```

要求：

- 不包含 Handler；
-不包含 Tool Executor；
-不包含运行时闭包；
-不包含 MCP Client；
-不包含 Permission Grant；
-不包含角色当前状态；
-不包含当前会话；
-不包含 Cache 实例；
-不包含完整资源内容。

---

## 五、稳定 ID

Agent Skill ID 建议格式：

```text
agent-skill/<owner-namespace>/<skill-name>
```

示例：

```text
agent-skill/com.example.weather/weather-assistant
agent-skill/user/local-research
agent-skill/system/default-conversation
```

要求：

- 全局稳定；
-不依赖显示名称；
-不依赖角色；
-不依赖数据库自增 ID；
-不因版本升级变化；
-包卸载重装后可恢复；
-用户 Fork 后必须生成新的 ID；
-不同 Owner 不得共享同一 ID；
-旧 Agent Skill ID 必须有迁移映射。

---

## 六、SKILL.md 入口规则

每个 Agent Skill 必须有唯一主入口：

```text
SKILL.md
```

允许位置应由包结构规则明确，例如：

```text
agent-skills/<skill-id>/SKILL.md
```

或独立导入包根目录：

```text
SKILL.md
```

禁止：

- 多个未声明主入口；
-大小写变体冲突；
-隐藏目录中自动发现；
-通过软链接指向包外；
-动态脚本生成 SKILL.md；
-安装后修改入口文件；
-预览和安装使用不同入口。

---

## 七、Frontmatter 模型

建议使用 YAML Frontmatter，但必须限制支持范围。

示例：

```yaml
---
id: weather-assistant
name: 天气助手
description: 提供天气查询与出行建议
version: 1.0.0
schemaVersion: 2
activation:
  mode: auto
  keywords:
    - 天气
    - 下雨
requiredTools:
  - mcp/weather-server/get_forecast
resources:
  - path: references/weather-rules.md
    type: reference
tokenPolicy:
  maxInstructionTokens: 1200
  maxResourceTokensPerTurn: 2000
---
```

必须校验：

- 未知字段；
-重复字段；
-字段类型；
-版本；
-ID；
-资源路径；
-Tool ID；
-MCP Reference；
-Token；
-激活规则；
-兼容版本；
-权限声明非法项。

---

## 八、Frontmatter 安全限制

禁止支持：

- YAML 自定义对象；
-任意类型反序列化；
-Anchor 递归炸弹；
-超深嵌套；
-超大数组；
-执行表达式；
-动态环境变量读取；
-脚本插值；
-文件包含；
-远程引用；
-宿主函数调用。

解析器必须使用安全模式。

---

## 九、指令正文

SKILL.md 正文表示 Agent Skill 的主要指令。

必须：

- 使用确定性 Markdown 解析；
-保留原文 Hash；
-限制最大字节；
-限制最大 Token；
-识别标题结构；
-支持引用资源；
-支持 Tool 引用说明；
-支持变量占位符，但只能使用宿主允许的结构化变量；
-不得执行 HTML；
-不得执行脚本；
-不得读取未声明文件；
-不得将 Frontmatter 之外内容视为权限声明。

---

## 十、变量占位符

第一版建议仅支持受控变量：

```text
{{character.name}}
{{character.identity}}
{{conversation.id}}
{{current_time}}
{{extension.id}}
{{skill.id}}
```

变量必须由 Prompt 构建阶段解析。

禁止：

- 任意表达式；
-文件读取；
-环境变量；
-Secret；
-SQL；
-网络请求；
-调用 Tool；
-调用宿主函数。

未知变量必须：

- 报告兼容性问题；
-不得静默替换为空字符串。

---

## 十一、资源类型

建议支持：

```go
type SkillResourceType string

const (
    SkillResourceReference SkillResourceType = "reference"
    SkillResourceAsset     SkillResourceType = "asset"
    SkillResourceTemplate  SkillResourceType = "template"
    SkillResourceExample   SkillResourceType = "example"
    SkillResourceSchema    SkillResourceType = "schema"
)
```

第一阶段不把脚本作为可执行资源。

若旧 Agent Skill 存在 Scripts：

```text
只索引
不执行
标记 legacy_script
后续由插件 Runtime 或 Workflow 模型迁移
```

---

## 十二、资源描述

建议：

```go
type SkillResourceDescriptor struct {
    ResourceID    string
    SkillID       string
    Type          SkillResourceType
    Path          string
    MIMEType      string
    SizeBytes     int64
    Hash          string
    TokenEstimate int
    Title         string
    Description   string
    Metadata      map[string]any
}
```

要求：

- Resource ID 稳定；
-路径使用 Package Security 规范化结果；
-Hash 来自 Sealed Staging；
-不直接保存完整内容；
-不允许包外路径；
-不允许链接；
-资源类型与 MIME 一致；
-资源大小受限。

---

## 十三、资源索引

Agent Skill Resource Index 必须包含：

- Resource ID；
-Path；
-Type；
-MIME；
-Hash；
-Size；
-Token Estimate；
-标题；
-摘要；
-关联章节；
-是否可直接注入；
-是否需要 Tool 读取；
-是否包含敏感内容；
-是否适合模型上下文。

资源索引是派生资源，可重建，但不能成为唯一源数据。

---

## 十四、渐进加载

Agent Skill 不应在每次激活时把全部资源注入 Prompt。

建议分层：

### Level 0：Catalog 摘要

包含：

- ID；
-名称；
-描述；
-激活条件；
-少量 Token。

### Level 1：Instructions

激活后注入主指令。

### Level 2：Resource Index

模型可看到资源目录摘要。

### Level 3：On-demand Resource

通过内部 Resource Tool 按需读取。

### Level 4：Large/Binary Asset

返回受控资源引用，不直接进入 Prompt。

---

## 十五、内部 Agent Skill Tools

Agent Skill 本体不进入 ToolRegistry，但可以保留系统内部 Tool：

```text
internal/agent-skill/list
internal/agent-skill/activate
internal/agent-skill/list-resources
internal/agent-skill/read-resource
internal/agent-skill/get-asset
```

这些 Tool：

- 属于 system；
-使用 InternalRuntimeAdapter；
-经过 Execution Security Kernel；
-经过 Scope；
-经过 Permission；
-写审计；
-不代表某个 Agent Skill 本体；
-不得返回未授权资源；
-不得读取包外文件。

---

## 十六、Catalog 模型

建议定义：

```go
type AgentSkillCatalog interface {
    Register(ctx context.Context, skill AgentSkillDefinition) error
    Replace(ctx context.Context, skill AgentSkillDefinition) error
    Unregister(ctx context.Context, skillID string) error
    Get(ctx context.Context, skillID string) (AgentSkillDefinition, error)
    List(ctx context.Context, filter AgentSkillFilter) ([]AgentSkillDefinition, error)
    Search(ctx context.Context, query AgentSkillQuery) ([]AgentSkillMatch, error)
}
```

Catalog 负责：

- Definition 索引；
-作用域过滤；
-启用状态查询；
-版本；
-Owner；
-资源摘要；
-激活候选；
-缓存。

Catalog 不负责：

- Tool 执行；
-MCP 连接；
-Permission Grant；
-Prompt 拼接；
-包安装；
-资源文件写入。

---

## 十七、Catalog 搜索

第一阶段建议支持：

- 精确 ID；
-名称；
-描述；
-关键字；
-标签；
-作用域；
-Owner；
-Extension；
-版本；
-激活模式。

可选支持向量检索，但：

- 不作为唯一激活机制；
-索引可重建；
-不把完整敏感资源写入向量库；
-必须有数据来源与删除清理；
-本步骤不强制实现。

---

## 十八、激活规则

建议：

```go
type SkillActivationRule struct {
    Mode             SkillActivationMode
    Keywords         []string
    IntentHints      []string
    RequiredContext  []string
    ExcludedContext  []string
    Priority         int
    MaxActivationsPerTurn int
}
```

Mode：

```text
manual
auto
tool_requested
extension_controlled
system
```

---

## 十九、激活评估

AgentSkillActivationService 输入：

```go
type SkillActivationRequest struct {
    CharacterID    string
    ConversationID string
    UserMessage    string
    CurrentContext ContextSummary
    ExplicitSkillIDs []string
    AvailableToolIDs []string
}
```

输出：

```go
type SkillActivationResult struct {
    Activated []ActivatedSkill
    Rejected  []RejectedSkill
    TokenPlan SkillTokenPlan
}
```

评估必须考虑：

- Agent Skill Enabled；
-Scope；
-Tool 依赖；
-MCP 依赖；
-Token Budget；
-优先级；
-显式激活；
-自动激活；
-冲突；
-每轮上限；
-角色与会话；
-Extension/Module 状态。

---

## 二十、激活不是权限

激活只表示：

```text
当前轮次应将该 Agent Skill 的指令或资源纳入上下文
```

激活不表示：

- Tool 已授权；
-MCP 已授权；
-角色数据可读；
-后台任务可执行；
-扩展获得宿主权限。

Tool 调用仍由 Permission Broker 决定。

---

## 二十一、Token Budget

建议：

```go
type SkillTokenPolicy struct {
    CatalogSummaryTokens    int
    MaxInstructionTokens    int
    MaxResourceTokensPerTurn int
    MaxTotalTokensPerTurn   int
    Priority                int
    TruncationPolicy        string
}
```

Token 预算必须统一考虑：

- 系统 Prompt；
-角色 Prompt；
-记忆；
-聊天历史；
-Agent Skill Instructions；
-Agent Skill Resource；
-Tool Definitions；
-模型上下文长度；
-当前用户消息；
-模型输出预留。

不得只在 Agent Skill 内部自行判断总 Token。

---

## 二十二、Token 分配策略

建议顺序：

```text
1. 核心系统指令
2. 当前角色核心设定
3. 当前用户消息
4. 必要记忆
5. 显式激活 Agent Skill
6. 自动激活 Agent Skill
7. Agent Skill Resources
8. Tool Definitions
9. 可选上下文
```

Agent Skill 之间发生竞争时：

- 显式优先；
-高优先级优先；
-更高相关性优先；
-依赖不可用则不分配；
-超过预算时进行结构化裁剪；
-不得任意截断到破坏 Markdown 结构。

---

## 二十三、Instruction 裁剪

允许策略：

```text
none
section_priority
tail_drop
resource_only
summary_fallback
```

推荐：

- 主指令尽量完整；
-低优先级章节可裁剪；
-资源目录优先保留；
-不得截断 YAML Frontmatter；
-不得把截断后的不完整指令伪装成完整；
-需要记录裁剪事件；
-前端或开发者控制台可查看。

---

## 二十四、依赖模型

Agent Skill 可声明：

```text
required_tools
optional_tools
required_mcp
optional_mcp
```

建议：

```go
type ToolReference struct {
    ToolID       string
    Required     bool
    VersionRange string
}

type MCPReference struct {
    ReferenceID  string
    ServerKey    string
    RequiredTools []string
    Required     bool
    OwnershipHint string
}
```

---

## 二十五、Tool 依赖解析

Tool 依赖必须通过 ToolRegistry。

检查：

- Tool 是否存在；
-版本；
-当前 Scope 是否可见；
-当前 Runtime 是否可用；
-是否启用；
-是否为 Optional；
-是否来自预期 Owner；
-是否发生 ID 迁移。

Agent Skill 不得直接持有 Handler。

---

## 二十六、MCP 依赖解析

MCP 依赖通过 MCPDependencyService，但其职责必须调整为：

- 解析 Server Definition；
-识别现有 Server；
-生成安装或复用计划；
-生成 Tool Allowlist；
-建立 Resource Reference；
-交由用户确认；
-交由 MCP Service 创建或复用；
-不自动授权 Tool；
-不让 Agent Skill 直接拥有连接。

---

## 二十七、MCP 所有权规则

### 用户导入独立 Agent Skill

若其需要新建 MCP Server：

- 默认 Owner 应为 user；
-Agent Skill 只引用；
-删除 Agent Skill 时不自动删除，除非用户确认且无其他引用。

### Extension Package 内 Agent Skill

若 MCP Definition 属于同一 Extension：

- Owner 为 Extension/Module；
-Agent Skill 只是引用者；
-包卸载时统一 Release Plan。

### 共享 Server

- Owner 为 shared 或 user；
-多个 Agent Skill 引用；
-删除单一 Skill 不删除 Server。

---

## 二十八、兼容性模型

建议：

```go
type SkillCompatibility struct {
    MinHostVersion string
    MaxHostVersion string
    Platforms      []string
    RequiredFeatures []string
    RequiredSchemaVersion int
}
```

必须检查：

- Amitia 版本；
-Agent Skill Schema Version；
-平台；
-Tool Feature；
-MCP Feature；
-模型能力；
-资源类型；
-Token 需求；
-不支持字段。

---

## 二十九、完整性模型

建议：

```go
type SkillIntegrity struct {
    SkillFileHash   string
    ResourceTreeHash string
    PackageHash     string
    SignatureState string
}
```

Hash 来自 Package Security。

Loader 不重新定义另一套 Hash 规则。

---

## 三十、变更检测

更新 Agent Skill 时必须比较：

- Version；
-Instructions Hash；
-Resource Hash；
-激活规则；
-Tool 依赖；
-MCP 依赖；
-Token Policy；
-Scope Rule；
-兼容性；
-Owner；
-签名；
-资源删除；
-资源新增。

输出：

```go
type AgentSkillChangeReport struct {
    BreakingChanges []SkillChange
    PermissionRelevant []SkillChange
    DependencyChanges []SkillChange
    ResourceChanges []SkillChange
    TokenChanges []SkillChange
    Warnings []SkillChange
}
```

---

## 三十一、更新策略

更新流程：

```text
Secure Staging
→ Parse New Definition
→ Validate Compatibility
→ Build Change Report
→ Resolve Dependencies
→ Create Resource Snapshot
→ Register Pending Version
→ Atomic Catalog Replace
→ Invalidate Cache
→ Update Scope/References
→ Commit
→ Audit
```

失败时恢复旧 Definition 和 Resource Index。

不得先删除旧资源再解析新版本。

---

## 三十二、删除策略

删除 Agent Skill 前必须预览：

- Owner；
-Extension；
-Scope Binding；
-Tool References；
-MCP References；
-共享 MCP；
-用户修改；
-Resource；
-Activation History；
-Cache；
-Artifact；
-依赖者。

删除顺序：

```text
1. 禁止新激活
2. 注销 Catalog
3. 取消当前轮次后的延迟激活
4. 清理 Scope Binding
5. 删除 Resource Reference
6. 处理 MCP Reference
7. 清理 Cache
8. 删除 Extension-owned Artifact
9. 保留 Audit/Activation History
10. 标记 deleted
```

---

## 三十三、缓存模型

允许缓存：

- Definition；
-Resource Index；
-资源 Token Estimate；
-激活候选；
-兼容性结果；
-Change Report。

缓存必须绑定：

- Skill ID；
-Version；
-Definition Hash；
-Scope；
-Host Version。

触发失效：

- 更新；
-删除；
-Scope 变化；
-Extension 禁用；
-Module 禁用；
-Tool 依赖变化；
-MCP Descriptor 变化；
-Host Version 变化；
-角色删除；
-会话结束。

---

## 三十四、Round State

旧系统如存在 Agent Skill Round State，应改造成：

```text
Prompt Assembly State
```

而不是 Agent Skill Runtime 的长期状态。

每轮仅记录：

- Activated Skill IDs；
-Resource Reads；
-Token Allocation；
-Explicit Activation；
-Activation Reasons；
-Tool Dependencies；
-Trace ID。

会话结束后可按策略清理。

不得作为 Agent Skill Definition 真值。

---

## 三十五、资源读取服务

建议：

```go
type AgentSkillResourceService interface {
    ListResources(
        ctx context.Context,
        request SkillResourceListRequest,
    ) ([]SkillResourceDescriptor, error)

    ReadResource(
        ctx context.Context,
        request SkillResourceReadRequest,
    ) (SkillResourceContent, error)

    GetAsset(
        ctx context.Context,
        request SkillAssetRequest,
    ) (ResourceReference, error)
}
```

必须检查：

- Skill 是否存在；
-Enabled；
-Scope；
-资源属于 Skill；
-路径安全；
-大小；
-MIME；
-Permission；
-当前 Invocation；
-Token 预算；
-审计。

---

## 三十六、资源读取结果

建议：

```go
type SkillResourceContent struct {
    ResourceID    string
    MIMEType      string
    Text          string
    URI           string
    SizeBytes     int64
    TokenEstimate int
    Truncated     bool
    Hash          string
}
```

二进制资源默认返回 URI/Reference，不直接返回 Base64。

---

## 三十七、Prompt Contribution

Agent Skill 激活后应产生结构化 Contribution：

```go
type AgentSkillPromptContribution struct {
    SkillID       string
    Instructions  string
    ResourceIndex []SkillResourceSummary
    TokenUsage    int
    Priority      int
    Truncated     bool
    Metadata      map[string]any
}
```

Prompt Builder 负责最终顺序和拼接。

Loader 不直接修改最终 Prompt 字符串。

---

## 三十八、Prompt 安全

Agent Skill 内容属于外部扩展内容，不得自动视为系统最高优先级。

必须：

- 标记来源；
-限制优先级；
-隔离系统核心指令；
-处理 Prompt Injection 风险；
-禁止声明绕过 Permission；
-禁止声明读取 Secret；
-禁止覆盖用户安全设置；
-支持用户查看 Agent Skill 内容；
-支持禁用；
-写入激活审计。

---

## 三十九、Agent Skill 与 Plugin 的关系

未来 `.amitiax` Extension 可包含 Agent Skill Module。

Plugin 可以贡献 Agent Skill Definition，但：

- 必须通过 Loader/Definition Validator；
-不得在运行时动态生成不受控指令；
-动态 Contribution 需受 Manifest 声明；
-需要稳定 ID；
-需要 Owner；
-需要 Scope；
-卸载时由 Resource Ownership 清理。

---

## 四十、Agent Skill 与 Workflow 的关系

Agent Skill 可以引用 Workflow Tool ID，但不直接嵌入 Workflow Runtime。

如果包内同时包含：

- Agent Skill；
-Workflow；

两者通过稳定 Tool/Workflow Reference 关联。

删除 Agent Skill 不删除 Workflow，除非 Resource Ownership 说明 Workflow 仅属于该 Skill 且无外部引用。

---

## 四十一、Agent Skill 与 Package 的关系

Package 负责安装事务和 Owner。

Loader 只负责：

- 解析；
-校验；
-构建 Definition；
-注册 Catalog；
-生成 Resource 清单。

禁止 Loader：

- 直接 Commit 文件；
-直接管理 Package Version；
-直接删除 Extension；
-直接创建 Rollback Snapshot；
-直接信任发布者。

---

## 四十二、持久化模型

建议目标表：

```text
agent_skill_definitions
agent_skill_versions
agent_skill_resources
agent_skill_dependencies
agent_skill_activations
agent_skill_resource_access
agent_skill_migrations
```

是否复用旧表需结合第 4 步分类决定。

原则：

- Definition 与 Activation 分离；
-Resource Index 与 Artifact 分离；
-依赖与 Grant 分离；
-Scope 使用统一 Scope Binding；
-Owner 使用统一 Resource Ownership；
-新数据不写旧 Skill 表；
-旧数据只读迁移。

---

## 四十三、旧数据迁移

旧 Agent Skill 数据需迁移：

- Metadata；
-SKILL.md；
-Artifact；
-Resources；
-Scope；
-Enabled；
-Activation History；
-MCP Dependency；
-Tool Mapping；
-Round State；
-Cache。

迁移规则：

- Agent Skill 本体迁入 Definition；
-伪 SkillDefinition 不迁入 Tool；
-内部控制工具保留为系统 Tool；
-旧 MCP Dependency 转为 Resource Reference；
-旧角色绑定转统一 Scope；
-旧 Cache 不迁移，重建；
-旧 Round State 不长期迁移；
-旧 Activation History 可迁入统一审计或专用历史表；
-无法解析资源进入 migration warning。

---

## 四十四、旧 Handler 与 Runtime 处理

重点对象：

```text
backend/internal/extension/agent_skill_runtime.go
backend/internal/extension/agent_skill_handler.go
```

处理原则：

- 将 Catalog、Activation、Resource、Round State 拆分；
-删除 Agent Skill 本体 Handler 语义；
-内部资源工具迁入 InternalRuntimeAdapter；
-旧 Runtime 仅保留迁移入口；
-不新增功能；
-记录调用次数；
-后续删除。

---

## 四十五、API 边界

建议：

```text
GET    /api/extensions/agent-skills
GET    /api/extensions/agent-skills/:id
POST   /api/extensions/agent-skills/import/preview
POST   /api/extensions/agent-skills/import/commit
POST   /api/extensions/agent-skills/:id/enable
POST   /api/extensions/agent-skills/:id/disable
GET    /api/extensions/agent-skills/:id/resources
GET    /api/extensions/agent-skills/:id/dependencies
GET    /api/extensions/agent-skills/:id/compatibility
GET    /api/extensions/agent-skills/:id/changes
DELETE /api/extensions/agent-skills/:id
```

Scope、Permission、Resource、Audit 使用统一系统接口，不在 Agent Skill API 重复实现。

---

## 四十六、前端术语和页面

前端必须明确区分：

### Agent Skill

展示：

- 指令；
-资源；
-激活；
-依赖；
-Token；
-作用域；
-Owner；
-版本；
-兼容性。

### Tool

展示可执行能力。

不得再把 MCP Tool、Workflow 和 Agent Skill 放在同一“技能列表”中。

Agent Skill 详情页至少显示：

- Name；
-Description；
-Version；
-Source；
-Owner；
-Status；
-Scope；
-Activation；
-Instructions Preview；
-Resources；
-Required Tools；
-Required MCP；
-Token Policy；
-Compatibility；
-Integrity；
-Activation History；
-Resource Access；
-Update Changes。

---

## 四十七、开发者诊断

开发者控制台应支持：

- 查看解析后的 Definition；
-查看 Frontmatter；
-查看 Resource Index；
-查看 Hash；
-查看 Token Estimate；
-查看依赖解析；
-查看激活原因；
-查看裁剪；
-查看 Scope；
-查看 Cache；
-查看 Migration Warning；
-查看未解析字段。

不得显示 Secret。

---

## 四十八、测试要求

必须新增：

### 1. SKILL.md Parser

- 合法；
-无 Frontmatter；
-未知字段；
-重复字段；
-错误类型；
-超长；
-非法编码；
-YAML Anchor；
-深度；
-多语言；
-特殊字符。

### 2. 入口检测

- 单入口；
-多入口；
-大小写冲突；
-根目录；
-包内目录；
-链接；
-路径穿越。

### 3. Resource Scanner

- Reference；
-Asset；
-Template；
-Example；
-Schema；
-MIME；
-Hash；
-大小；
-重复；
-非法路径；
-legacy script。

### 4. Definition Builder

- ID；
-Version；
-Owner；
-Scope；
-Integrity；
-Dependencies；
-Token；
-Compatibility。

### 5. Catalog

- Register；
-Replace；
-Unregister；
-Search；
-Scope；
-Enabled；
-Owner；
-并发；
-冲突。

### 6. Activation

- manual；
-auto；
-explicit；
-keyword；
-intent；
-优先级；
-冲突；
-每轮上限；
-依赖缺失；
-Scope 不匹配；
-Extension 禁用。

### 7. Token Budget

- 多 Skill；
-显式优先；
-裁剪；
-资源预算；
-模型上下文；
-极小上下文；
-超限；
-稳定性。

### 8. Resource Access

- List；
-Read；
-Asset；
-不存在；
-越权；
-大小；
-MIME；
-Token；
-删除后；
-并发；
-审计。

### 9. Tool Dependency

- 存在；
-缺失；
-版本；
-禁用；
-Scope；
-Runtime 不可用；
-ID 迁移。

### 10. MCP Dependency

- 复用；
-新建；
-共享；
-用户 Owner；
-Extension Owner；
-删除；
-引用计数；
-无自动权限。

### 11. Update

- 指令变化；
-资源变化；
-依赖变化；
-Token 变化；
-兼容变化；
-失败回滚；
-Cache 失效。

### 12. Delete

- Catalog；
-Scope；
-Resource；
-MCP Reference；
-Cache；
-Artifact；
-Audit；
-共享资源保护。

### 13. Migration

- 旧 Metadata；
-旧 Artifact；
-旧 Scope；
-旧伪 Skill；
-旧 MCP Dependency；
-旧 Round State；
-损坏数据。

### 14. Prompt Security

- 注入覆盖尝试；
-权限绕过文本；
-Secret 请求；
-系统指令覆盖；
-超长指令；
-危险 HTML。

---

## 四十九、实施任务

### Task 1：定义 Agent Skill 领域模型

完成 Definition、Resource、Activation、Token、Dependency、Compatibility、Integrity。

### Task 2：重构 SKILL.md Parser

使用安全 Frontmatter 与确定性 Markdown 解析。

### Task 3：接入 Package Security

只读取 Sealed Staging。

### Task 4：实现 Resource Scanner

建立统一资源类型和路径校验。

### Task 5：实现 Resource Indexer

生成 Hash、MIME、Size、Token Estimate 和摘要。

### Task 6：实现 Definition Builder

生成稳定 AgentSkillDefinition。

### Task 7：实现 Compatibility Validator

校验 Host、平台、Schema、Feature 和版本。

### Task 8：实现 AgentSkillCatalog

完成注册、替换、查询、搜索和注销。

### Task 9：实现 ActivationService

支持显式、自动和系统激活。

### Task 10：实现 TokenBudgeter

接入统一 Prompt Token 预算。

### Task 11：实现 ResourceService

提供安全按需读取。

### Task 12：迁移内部控制 Tool

统一注册为 Internal Tool。

### Task 13：接入 ToolRegistry

只解析 Tool Reference，不注册 Agent Skill 本体。

### Task 14：接入 MCPDependencyService

统一引用和所有权。

### Task 15：接入 Scope Manager

移除独立角色绑定。

### Task 16：接入 Resource Ownership

登记 Definition、Artifact、Resource 和 Reference。

### Task 17：接入统一 Audit

记录导入、激活、资源读取、更新和删除。

### Task 18：实现 Change Detector

生成版本变更报告。

### Task 19：实现更新与删除事务接口

供 Package 生命周期调用。

### Task 20：建立旧数据迁移器

停止新写旧表。

### Task 21：重构前端 Agent Skill 页面

与 Tool 页面分离。

### Task 22：增加旧 Runtime 调用统计

识别剩余依赖。

### Task 23：完成安全、回归和性能测试

确保 Prompt、资源和缓存稳定。

---

## 五十、建议目录结构

建议：

```text
backend/internal/extension/kernel/agent_skill/
├── definition.go
├── id.go
├── parser.go
├── frontmatter.go
├── instruction.go
├── resource.go
├── scanner.go
├── indexer.go
├── dependency.go
├── compatibility.go
├── integrity.go
├── catalog.go
├── activation.go
├── token_budget.go
├── resource_service.go
├── change_detector.go
├── cache.go
├── migration.go
└── audit.go
```

内部 Tool：

```text
backend/internal/extension/kernel/adapters/internal_agent_skill.go
```

前端：

```text
front/src/views/extensions/agent-skills/
├── AgentSkillListView.vue
├── AgentSkillDetailView.vue
├── AgentSkillImportPreview.vue
├── AgentSkillResources.vue
├── AgentSkillDependencies.vue
├── AgentSkillActivationHistory.vue
└── AgentSkillCompatibility.vue
```

目录仅为建议。

---

## 五十一、性能要求

建议：

- Catalog 查询不读取完整 SKILL.md；
-资源内容按需读取；
-Token Estimate 可缓存；
-更新时只重建变化资源；
-Resource Hash 复用 Package Security；
-激活候选不全量读取正文；
-大量 Agent Skill 使用索引搜索；
-每轮激活数量有上限；
-资源读取有大小限制；
-Cache 有界；
-角色切换只失效相关 Scope Cache；
-删除和更新不全表扫描。

---

## 五十二、风险控制

### P0：Prompt 与数据安全

- Agent Skill 覆盖系统安全指令；
-读取包外文件；
-通过文本获得权限；
-跨角色资源泄露；
-未授权 MCP 自动执行；
-Secret 进入资源或审计。

### P1：领域边界回退

- Agent Skill 继续进入 ToolRegistry；
-伪 Handler 保留；
-旧 Runtime 继续成为主入口；
-MCP 依赖继续拥有 Server；
-独立 Scope 继续写入。

### P2：一致性问题

- Definition 与 Artifact 不一致；
-Resource Index 过期；
-更新后 Cache 未失效；
-删除后资源仍可读；
-Tool Reference 漂移。

### P3：性能与体验

- 每轮注入过多；
-Token 预算不稳定；
-资源读取过慢；
-前端信息复杂；
-自动激活误触发。

---

## 五十三、本步骤不做的事情

本步骤明确不做：

- 不实现新的 Agent Skill 标准；
-不执行包内脚本；
-不实现完整 Workflow 重构；
-不实现第三方插件 Runtime；
-不实现 `.amitiax` v2 完整 Manifest；
-不删除旧 Agent Skill 表；
-不删除旧 Runtime；
-不实现移动端；
-不实现 Agent Skill 商店；
-不自动信任外部 Prompt；
-不让 Agent Skill 自动获取 Tool 权限；
-不改变模型核心安全策略。

---

## 五十四、验收产物

完成后必须提交：

### 1. Agent Skill Loader 主文档

```text
docs/extension-kernel/15-agent-skill-loader.md
```

### 2. Agent Skill 领域类型

至少包含：

- AgentSkillDefinition；
-SkillResourceDescriptor；
-SkillActivationRule；
-SkillTokenPolicy；
-ToolReference；
-MCPReference；
-SkillCompatibility；
-SkillIntegrity。

### 3. 安全 Parser

包含：

- Frontmatter；
-Markdown；
-入口检测；
-字段校验；
-兼容性报告。

### 4. Resource Scanner 与 Indexer

输出稳定资源索引。

### 5. AgentSkillCatalog

支持注册、替换、查询、搜索和注销。

### 6. ActivationService

支持显式、自动和系统激活。

### 7. TokenBudgeter

支持统一上下文预算和裁剪。

### 8. ResourceService

支持安全列表、读取和 Asset Reference。

### 9. Tool/MCP Dependency

只声明引用，不自动授权。

### 10. Scope 与 Ownership 接入

不再维护独立角色绑定和资源所有权。

### 11. 统一 Audit 接入

导入、激活、资源读取、更新和删除可追踪。

### 12. 旧数据迁移报告

列出：

- 已迁移 Definition；
-旧伪 Skill；
-旧 Scope；
-旧 MCP Dependency；
-旧 Artifact；
-损坏数据；
-仍使用旧 Runtime 的入口。

### 13. 前端 Agent Skill 页面

与 Tool、MCP、Workflow 页面明确分离。

### 14. 测试报告

覆盖 Parser、Resource、Catalog、Activation、Token、Dependency、Update、Delete、Migration、Prompt Security 和性能。

---

## 五十五、验收标准

本步骤通过必须满足：

1. Agent Skill 已有独立领域模型。
2. Agent Skill 本体不再新增到 ToolRegistry。
3. Agent Skill 本体不包含 Handler。
4. SKILL.md 只从 Sealed Staging 读取。
5. Frontmatter 使用安全解析。
6. Resource Index 统一且可重建。
7. Agent Skill 支持渐进加载。
8. 激活与权限已分离。
9. 激活与 Tool 执行已分离。
10. Tool 依赖通过稳定 Tool ID。
11. MCP 依赖通过 Resource Reference。
12. Agent Skill 不直接拥有 MCP 连接。
13. Token Budget 接入统一 Prompt 预算。
14. Scope 使用统一 Scope Manager。
15. Owner 使用统一 Resource Ownership。
16. 更新和删除具有变更报告与回滚边界。
17. 旧 Cache 和 Round State 不再作为真值。
18. 前端不再将 Agent Skill 与 Tool 混为一类。
19. 新数据不再写旧 Skill 表。
20. 安全与回归测试通过。
21. 后续第 16 步可以在此基础上重构 Workflow Executor。

---

## 五十六、退出条件

只有满足以下条件后，才能进入第 16 步“重构 Workflow 执行器”：

- AgentSkillDefinition 已落地；
-AgentSkillCatalog 已落地；
-安全 Parser 已落地；
-Resource Index 已落地；
-ActivationService 已落地；
-TokenBudgeter 已落地；
-ResourceService 已落地；
-Tool/MCP Dependency 已接入；
-Scope 和 Ownership 已接入；
-新 Agent Skill 不再进入旧 Skill Registry；
-旧 Runtime 只剩迁移用途；
-前端概念已分离；
-关键测试通过。

---

## 五十七、执行约束

执行本步骤时必须遵守：

> Agent Skill 是可激活的指令与资源集合，不是可执行 Tool，不拥有宿主权限，不拥有 MCP 连接，也不直接管理 Workflow。

禁止出现：

- Agent Skill Definition 保存 Handler；
-Agent Skill 本体注册 Tool；
-安装 Agent Skill 自动授予 Tool 权限；
-Agent Skill 删除时无条件删除共享 MCP；
-Agent Skill 直接执行包内脚本；
-Loader 直接提交最终扩展目录；
-Loader 自行管理 Package Version；
-前端继续统一显示为“技能工具”；
-新旧 Agent Skill 数据长期双写；
-旧 Runtime 继续成为新功能入口；
-资源读取绕过 Scope、Permission 和 Audit。

本步骤完成后，Amitia 必须具备一套独立、稳定、安全、可渐进加载、可审计、可迁移的 Agent Skill 加载与激活基础。
