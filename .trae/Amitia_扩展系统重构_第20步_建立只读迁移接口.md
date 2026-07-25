# Amitia 扩展系统重构第 20 步实施文档

## 第 20 步：建立只读迁移接口

---

## 一、步骤目标

在前 19 步已经完成概念拆分、统一 Tool 模型、统一执行安全、统一 Permission、统一 Scope、统一运行审计、统一资源所有权、包安全、MCP 协议、Agent Skill、Workflow、Plugin Runtime 安全、生命周期和 Enabled 状态清理的基础上，建立旧扩展系统到新 Extension Kernel 的统一只读迁移接口。

本步骤的目标是：

> 让旧 Skill、Plugin、MCP、Workflow、Package、Agent Skill、Schedule、Scope、Permission、运行记录和资源所有权数据，可以通过稳定、只读、可审计、可重放、可校验的迁移接口被新系统读取和转换，同时禁止新系统继续依赖旧系统运行逻辑，也禁止迁移阶段出现新旧双写。

本步骤不是正式删除旧系统，而是为后续：

- Extension Kernel 领域模型落地；
-单一生命周期管理器；
-Contribution Registry；
-Dependency Resolver；
-Runtime Supervisor；
-Host API Gateway；
-Manifest v2；
-旧数据迁移；
-旧系统删除；

建立可控边界。

当前若直接迁移，可能存在：

1. 旧 Repository 与旧 Manager 混合提供数据。
2. 读取旧数据会触发隐式初始化、连接或注册。
3. 旧 Plugin Manager 查询可能顺带启动 Plugin。
4. 旧 MCP Manager 读取可能顺带重连。
5. 旧 Workflow 查询可能返回运行时闭包或 Handler。
6. 旧 Skill 查询混合 Tool、Agent Skill、Workflow 和 MCP Tool。
7. 旧 Package Parser 可能重新解析原始包。
8. 旧 Enabled、Scope、Permission 状态存在冲突。
9. 旧运行记录父子关系不完整。
10. 旧资源所有权无法直接确定。
11. 旧 API 在兼容期间仍可能写旧表。
12. 新系统读取旧数据后又回写旧模型，形成循环依赖。
13. 迁移脚本可能直接访问内部表，难以测试和审计。
14. 一次性迁移失败后无法稳定重放。
15. 数据变化期间迁移结果不一致。
16. 用户在迁移期间修改旧系统，导致迁移快照漂移。
17. 不同平台数据目录和 Artifact 状态不同。
18. 损坏数据可能阻断全部迁移。

本步骤完成后，必须形成唯一方向：

```text
Legacy Persistent Data
→ Legacy Read-Only Gateway
→ Canonical Migration DTO
→ Migration Validator
→ Migration Planner
→ New Kernel Import Boundary
```

禁止形成：

```text
New Kernel
→ Legacy Manager
→ Legacy Runtime
```

也禁止：

```text
New Kernel Write
→ Legacy Tables
```

---

## 二、核心原则

### 1. 单向迁移

迁移方向只能是：

```text
旧系统 → 新系统
```

禁止新系统为了兼容而反向写回旧系统。

---

### 2. 只读

Legacy Gateway 只允许：

- 查询；
-分页；
-快照；
-导出；
-校验；
-统计。

禁止：

- Enable；
-Disable；
-Connect；
-Start；
-Stop；
-Install；
-Update；
-Delete；
-Grant；
-Bind；
-Execute；
-Repair。

---

### 3. 不调用旧 Manager 业务逻辑

迁移读取优先直接使用：

- 只读 Repository；
-数据库 Snapshot；
-Artifact Reader；
-安全 Parser 输出；
-旧数据导出器。

禁止通过可能产生副作用的 Manager 读取。

---

### 4. 迁移 DTO 与旧领域模型分离

不得将旧 Go Struct 直接传入新 Kernel。

必须转换为：

```text
Canonical Migration DTO
```

这样可以：

- 固定字段语义；
-处理缺失；
-处理冲突；
-记录来源；
-记录证据；
-版本化；
-稳定测试。

---

### 5. 数据迁移与运行切换分离

本步骤只建立读取与转换边界。

不在本步骤：

- 切换主运行链；
-删除旧表；
-启动新 Runtime；
-迁移全部用户数据；
-自动修复所有冲突。

---

### 6. 更安全迁移

当旧数据冲突或不确定时：

- 不扩大 Enabled；
-不扩大 Scope；
-不自动授予 Permission；
-不自动启动 Runtime；
-不自动连接 MCP；
-不自动执行 Workflow；
-不自动删除资源。

默认进入：

```text
disabled
unresolved
manual_review
migration_warning
```

---

## 三、目标架构

建议形成：

```text
Legacy Migration Layer
├── LegacyReadOnlyGateway
├── LegacySnapshotService
├── LegacyArtifactReader
├── LegacySourceAdapters
│   ├── SkillAdapter
│   ├── AgentSkillAdapter
│   ├── PluginAdapter
│   ├── MCPAdapter
│   ├── WorkflowAdapter
│   ├── PackageAdapter
│   ├── PermissionAdapter
│   ├── ScopeAdapter
│   ├── RuntimeRecordAdapter
│   └── ResourceOwnershipAdapter
├── CanonicalMigrationDTO
├── MigrationNormalizer
├── MigrationValidator
├── MigrationConflictDetector
├── MigrationPlanner
├── MigrationReportStore
└── NewKernelImportBoundary
```

---

## 四、Legacy Read-Only Gateway

建议接口：

```go
type LegacyReadOnlyGateway interface {
    CreateSnapshot(
        ctx context.Context,
        request LegacySnapshotRequest,
    ) (LegacySnapshot, error)

    ListEntities(
        ctx context.Context,
        snapshotID string,
        query LegacyEntityQuery,
    ) (LegacyEntityPage, error)

    GetEntity(
        ctx context.Context,
        snapshotID string,
        entityType LegacyEntityType,
        legacyID string,
    ) (LegacyEntityRecord, error)

    ReadArtifact(
        ctx context.Context,
        snapshotID string,
        artifactRef string,
    ) (LegacyArtifactRecord, error)

    GetStatistics(
        ctx context.Context,
        snapshotID string,
    ) (LegacyMigrationStatistics, error)
}
```

要求：

- 所有读取绑定 Snapshot；
-无写方法；
-无运行方法；
-不返回 Handler；
-不返回数据库连接；
-不返回内部 Service；
-不触发 Runtime；
-不修改旧缓存；
-不自动修复旧数据。

---

## 五、Snapshot 模型

建议：

```go
type LegacySnapshot struct {
    SnapshotID       string
    SchemaVersion    string
    ApplicationVersion string
    CreatedAt        time.Time
    SourceDatabaseID string
    DataRevision     string
    ArtifactRevision string
    EntityCounts     map[string]int64
    IntegrityHash    string
    Status           string
}
```

Snapshot 用于保证一次迁移读取的一致性。

---

## 六、Snapshot 实现策略

可根据数据库能力选择：

### 方案 A：数据库一致性读事务

适用于迁移时间较短。

### 方案 B：只读数据库副本

复制必要旧表到 Migration Snapshot Database。

### 方案 C：导出文件

导出为结构化快照。

推荐：

```text
本地单用户环境
→ 创建只读迁移快照数据库或导出文件
```

避免长时间锁住主数据库。

---

## 七、Snapshot 内容

至少包含：

- 旧 Schema 版本；
-旧应用版本；
-旧扩展定义；
-旧 Enabled；
-旧 Scope；
-旧 Permission；
-旧运行记录；
-旧资源所有权；
-旧 Package 元数据；
-Artifact 引用；
-旧 MCP 配置；
-旧 Workflow；
-旧 Agent Skill；
-旧 Plugin；
-旧 Schedule；
-迁移时数据统计；
-Hash。

Secret 内容不进入普通 Snapshot。

Secret 只保存：

```text
secret_reference
owner
purpose
scope
存在性
```

---

## 八、Snapshot 冻结语义

创建 Snapshot 后：

- 后续迁移读取只读取 Snapshot；
-不继续读取实时旧表；
-用户后续修改不影响本次迁移；
-若需要包含新修改，创建新 Snapshot；
-每个 Snapshot 有 Revision；
-迁移结果绑定 Snapshot ID；
-重放时使用同一 Snapshot。

---

## 九、Artifact 读取

旧 Artifact 包括：

- `.amitiax`；
-SKILL.md；
-Workflow JSON/YAML；
-Plugin Definition；
-MCP 配置；
-资源文件；
-历史版本；
-旧缓存；
-导入包。

Artifact Reader 必须：

- 只读；
-校验路径；
-校验 Hash；
-不执行；
-不解压到最终目录；
-必要时接入 Package Security；
-记录缺失；
-记录损坏；
-限制大小；
-不跟随链接。

---

## 十、Legacy Entity Type

建议：

```go
type LegacyEntityType string

const (
    LegacyEntityPackage        LegacyEntityType = "package"
    LegacyEntitySkill          LegacyEntityType = "skill"
    LegacyEntityAgentSkill     LegacyEntityType = "agent_skill"
    LegacyEntityPlugin         LegacyEntityType = "plugin"
    LegacyEntityMCPServer      LegacyEntityType = "mcp_server"
    LegacyEntityMCPTool        LegacyEntityType = "mcp_tool"
    LegacyEntityWorkflow       LegacyEntityType = "workflow"
    LegacyEntityWorkflowRun    LegacyEntityType = "workflow_run"
    LegacyEntitySchedule       LegacyEntityType = "schedule"
    LegacyEntityPermission     LegacyEntityType = "permission"
    LegacyEntityScope          LegacyEntityType = "scope"
    LegacyEntityRun            LegacyEntityType = "run"
    LegacyEntityAudit          LegacyEntityType = "audit"
    LegacyEntityResource       LegacyEntityType = "resource"
    LegacyEntityArtifact       LegacyEntityType = "artifact"
)
```

---

## 十一、Canonical Migration DTO

建议统一基础字段：

```go
type MigrationEntity struct {
    MigrationID      string
    EntityType       string
    LegacySource     LegacySourceReference
    CanonicalID      string
    Owner            MigrationOwner
    Version          string
    Definition       json.RawMessage
    Enablement       MigrationEnablement
    Scope            []MigrationScopeBinding
    Permissions      []MigrationPermissionReference
    Dependencies     []MigrationDependency
    Resources        []MigrationResourceReference
    RuntimeHints     []MigrationRuntimeHint
    Integrity        MigrationIntegrity
    Warnings         []MigrationWarning
    Conflicts        []MigrationConflict
    Metadata         map[string]any
}
```

---

## 十二、Legacy Source Reference

建议：

```go
type LegacySourceReference struct {
    SnapshotID    string
    SourceTable   string
    LegacyID      string
    LegacyType    string
    SourceVersion string
    RowHash       string
    ArtifactRefs  []string
}
```

用途：

- 追溯；
-重放；
-校验；
-冲突定位；
-用户诊断；
-迁移审计。

---

## 十三、Canonical ID 生成

新 Canonical ID 必须稳定。

规则：

- 优先使用已有稳定标识；
-缺失时根据 Owner Namespace + Name 生成；
-冲突时使用确定性后缀；
-不得使用随机 ID 作为 Definition 主 ID；
-数据库记录 ID 与 Stable ID 分离；
-记录 Legacy ID 映射。

建议维护：

```text
legacy_id_mappings
```

---

## 十四、ID 映射

建议：

```go
type LegacyIDMapping struct {
    SnapshotID     string
    LegacyType     string
    LegacyID       string
    CanonicalType  string
    CanonicalID    string
    MappingRule    string
    Confidence     string
}
```

Confidence：

```text
exact
derived
conflicted
manual
unknown
```

---

## 十五、Skill Adapter

旧 Skill 需要先分类：

```text
Executable Tool
Agent Skill
Workflow Wrapper
MCP Tool Wrapper
Internal Control Tool
Unknown
```

输出不得继续使用通用 Skill DTO。

转换规则：

### Executable Tool

迁入 Tool Definition DTO。

### Agent Skill

迁入 Agent Skill DTO。

### Workflow Wrapper

迁入 Workflow Tool Exposure，不迁入 Agent Skill。

### MCP Tool Wrapper

迁入 MCP Tool Reference，不重复创建 Tool Definition。

### Internal Control Tool

映射为 System Internal Tool。

### Unknown

标记冲突，默认不启用。

---

## 十六、Agent Skill Adapter

读取：

- Metadata；
-SKILL.md；
-Frontmatter；
-Artifact；
-Resources；
-旧 Scope；
-旧 Enabled；
-MCP Dependency；
-Tool Reference；
-版本；
-激活历史。

转换：

- AgentSkillDefinition DTO；
-Resource Index DTO；
-Scope Binding DTO；
-Enablement；
-Dependency；
-Owner；
-Integrity；
-Warning。

不迁移：

- Runtime Handler；
-旧 Cache；
-旧 Round State 作为长期真值；
-伪 Tool Handler。

---

## 十七、Plugin Adapter

旧 Go Plugin 迁移为：

```text
Extension Module
+ LegacyGoPluginRuntime Definition
+ Contributions
```

读取：

- Plugin ID；
-版本；
-注册入口；
-Tool；
-Hook；
-Event；
-Schedule；
-State Namespace；
-Circuit；
-资源；
-权限；
-Owner。

不读取或输出：

- 运行时闭包；
-函数指针；
-Repository 实例；
-内部 Service 实例。

---

## 十八、MCP Adapter

读取：

- Server Definition；
-Transport；
-Command；
-Args；
-URL；
-Credential Reference；
-Environment Reference；
-Headers Reference；
-Enabled；
-角色绑定；
-Tool Descriptor Snapshot；
-OAuth 元数据；
-Owner；
-Agent Skill Dependency；
-旧连接状态；
-任务记录。

转换：

- MCP Server Definition DTO；
-Resource Ownership；
-Scope Binding；
-Enablement；
-Secret Reference；
-MCP Tool Descriptor DTO；
-Dependency Reference；
-历史 Runtime Record。

不迁移旧 Session 为可恢复 Session。

---

## 十九、Workflow Adapter

读取：

- Definition；
-节点；
-边；
-输入输出；
-Tool 引用；
-旧 Skill Wrapper；
-Schedule；
-运行记录；
-节点记录；
-角色 Scope；
-版本；
-Owner；
-Artifact。

转换：

- WorkflowDefinition DTO；
-Tool Exposure DTO；
-Schedule DTO；
-Execution History DTO；
-Scope；
-Enablement；
-Dependencies；
-Integrity。

不迁移：

- Handler；
-旧运行内存 Context；
-旧缓存；
-未持久化闭包。

---

## 二十、Package Adapter

读取：

- Package ID；
-版本；
-Manifest；
-Artifact；
-Checksum；
-签名状态；
-安装状态；
-资源；
-Owner；
-回滚信息；
-模块；
-旧类型声明。

转换：

- Legacy Package DTO；
-未来 Extension Definition Import DTO；
-Artifact Reference；
-Integrity；
-Publisher Hint；
-Resource Ownership；
-Warning。

不得将旧 Manifest 直接视为 v2 Manifest。

---

## 二十一、Permission Adapter

只迁移：

- 权限定义引用；
-Grant；
-Subject；
-Scope；
-来源；
-时间；
-状态；
-审批；
-条件。

不得自动扩大或合并权限。

旧权限无法准确映射时：

```text
grant_state=unresolved
effective=deny
```

---

## 二十二、Scope Adapter

将旧：

- Global；
-Character；
-Conversation；
-Plugin Scope；
-MCP Tool Scope；
-Agent Skill Binding；
-Workflow Binding；

转换为统一 Scope Binding DTO。

要求：

- Conversation 校验 Character；
-角色不存在时标记孤儿；
-旧 `scope_enabled` 拆分为 Enablement + Scope；
-不将角色限定迁移为 Global；
-不自动补全缺失会话关系。

---

## 二十三、运行记录 Adapter

旧 Run、Plugin Run、MCP Operation、Workflow Run 等转换为：

- Operation；
-Invocation；
-Attempt；
-Runtime Event；
-Audit Event；
-Side Effect。

允许字段缺失，但必须明确：

```text
correlation_incomplete
parent_unknown
outcome_unknown
legacy_status_unknown
```

不得伪造父子关系。

---

## 二十四、资源所有权 Adapter

旧资源需转换：

- Owner；
-Resource Type；
-Reference；
-State；
-Delete Policy；
-Runtime Manager；
-Storage Manager；
-Artifact；
-Secret；
-Schedule；
-Process；
-Connection。

Owner 无法确认：

```text
owner_type=migration
state=orphaned
```

不自动归属 Extension。

---

## 二十五、Normalization

MigrationNormalizer 负责：

- ID 格式；
-版本格式；
-时间格式；
-空值；
-布尔值；
-枚举；
-路径；
-平台；
-MIME；
-状态；
-Owner；
-名称；
-字符编码；
-JSON；
-Hash；
-旧字段合并。

Normalization 不进行权限扩大和业务推断。

---

## 二十六、Validation

MigrationValidator 检查：

- Canonical ID；
-Definition Schema；
-Owner；
-Artifact；
-Hash；
-Dependency；
-Scope；
-Permission；
-Enablement；
-版本；
-兼容性；
-引用；
-资源；
-运行记录；
-平台。

Validation 输出：

```text
valid
valid_with_warnings
blocked
manual_review
```

---

## 二十七、Conflict Detector

冲突类型至少包括：

```text
duplicate_id
duplicate_name
owner_conflict
enablement_conflict
scope_conflict
permission_conflict
artifact_missing
artifact_hash_mismatch
definition_corrupted
dependency_missing
reference_cycle
version_conflict
resource_owner_unknown
runtime_state_conflict
legacy_type_ambiguous
```

---

## 二十八、冲突处理原则

### 可自动解决

仅限确定性、无权限扩大的情况。

例如：

- 大小写规范；
-空字段默认；
-稳定时间格式；
-重复完全相同记录。

### 默认禁用

适用于：

- Enabled 冲突；
-类型不确定；
-Owner 不确定；
-依赖缺失；
-Definition 损坏。

### 人工确认

适用于：

- 用户资产 Owner；
-共享 MCP；
-Workflow 用户修改；
-权限 Grant；
-资源删除策略；
-重复 Stable ID。

---

## 二十九、Migration Plan

建议：

```go
type MigrationPlan struct {
    PlanID            string
    SnapshotID        string
    TargetKernelVersion string
    Entities          []MigrationEntityPlan
    BlockingConflicts []MigrationConflict
    Warnings          []MigrationWarning
    Statistics        MigrationPlanStatistics
    PlanHash          string
    CreatedAt         time.Time
}
```

每个 Entity Plan：

```go
type MigrationEntityPlan struct {
    MigrationID     string
    Action          string
    TargetType      string
    TargetID        string
    Dependencies    []string
    Enablement      EnablementState
    RequiresReview  bool
    ImportPayload   json.RawMessage
}
```

---

## 三十、Migration Action

支持：

```text
import
skip
merge
replace
retain_legacy
manual_review
quarantine
orphan
```

本步骤只生成计划，不执行正式生产导入。

---

## 三十一、Dependency Order

Migration Planner 必须按依赖排序：

```text
Package/Extension
→ Module
→ Definitions
→ Resources
→ MCP Server
→ Tool
→ Agent Skill
→ Workflow
→ Scope
→ Permission
→ Schedule
→ Runtime History
→ Audit
```

实际顺序需根据依赖图确定。

---

## 三十二、New Kernel Import Boundary

本步骤定义新系统导入接口，但不要求完整实现所有新领域。

建议：

```go
type NewKernelImportBoundary interface {
    ValidateImport(
        ctx context.Context,
        plan MigrationPlan,
    ) ImportValidationResult

    DryRunImport(
        ctx context.Context,
        plan MigrationPlan,
    ) ImportDryRunResult
}
```

后续正式迁移时再增加：

```text
CommitImport
RollbackImport
```

当前禁止直接 Commit 生产数据。

---

## 三十三、Dry Run

Dry Run 必须验证：

- ID 冲突；
-依赖顺序；
-Schema；
-Owner；
-Scope；
-Permission；
-资源路径；
-Artifact；
-容量；
-版本；
-数据库约束；
-重复导入；
-回滚可能性。

Dry Run 不写正式业务表。

允许写：

- 临时验证表；
-迁移报告；
-审计；
-测试数据库。

---

## 三十四、幂等与重放

每个 Migration Entity 必须有：

```text
snapshot_id
migration_id
payload_hash
plan_hash
```

相同 Snapshot 和 Plan 重放应得到一致结果。

不得：

- 每次生成不同 Canonical ID；
-依赖当前时间决定 Owner；
-依赖当前 Runtime 状态决定 Definition；
-读取实时旧表；
-随机合并。

---

## 三十五、迁移批次

建议支持批次：

```text
inventory
definitions
resources
mcp
tools
agent_skills
workflows
scope_permission
schedules
history
audit
```

每批可独立：

- Validate；
-Dry Run；
-Report；
-Retry。

---

## 三十六、迁移锁

生成 Snapshot 和正式迁移阶段需防止：

- 同一数据源并发迁移；
-多个 Plan 同时写目标；
-用户重复点击；
-后台自动迁移与手动迁移冲突。

本步骤建立锁模型，不执行正式 Commit。

---

## 三十七、数据变化处理

Snapshot 创建后旧系统仍可能继续运行。

策略：

- 本步骤允许旧系统继续工作；
-本次 Plan 只代表 Snapshot 时刻；
-正式切换前需创建最终增量 Snapshot；
-或进入维护窗口；
-禁止边迁移边长期双写。

---

## 三十八、增量迁移预留

建议为后续预留：

```text
base_snapshot_id
delta_snapshot_id
change_sequence
```

但本步骤不实现完整 CDC。

本地单用户优先采用：

```text
最终停写窗口 + 最终 Snapshot
```

而不是复杂持续双写。

---

## 三十九、只读 API

建议内部 API：

```text
POST /internal/migration/legacy/snapshots
GET  /internal/migration/legacy/snapshots
GET  /internal/migration/legacy/snapshots/:id
GET  /internal/migration/legacy/snapshots/:id/entities
GET  /internal/migration/legacy/snapshots/:id/entities/:type/:legacyId
POST /internal/migration/legacy/snapshots/:id/validate
POST /internal/migration/legacy/snapshots/:id/plans
GET  /internal/migration/plans/:id
POST /internal/migration/plans/:id/dry-run
GET  /internal/migration/reports/:id
```

必须仅本地可信访问。

---

## 四十、API 安全

要求：

- 本地认证；
-不允许远程公开；
-分页限制；
-导出大小限制；
-敏感字段脱敏；
-不返回 Secret；
-Artifact 下载受控；
-路径不暴露完整用户目录；
-每次操作写审计；
-禁止执行类方法。

---

## 四十一、旧 API 兼容

旧前端和旧代码可继续调用旧查询 API，但迁移模块不依赖它们。

禁止：

```text
Migration Adapter
→ HTTP 调用旧业务 API
```

优先：

```text
Migration Adapter
→ Read-Only Repository/Snapshot
```

---

## 四十二、Repository 隔离

建议为旧表建立专用只读 Repository：

```go
type LegacySkillReadRepository interface
type LegacyPluginReadRepository interface
type LegacyMCPReadRepository interface
type LegacyWorkflowReadRepository interface
type LegacyPackageReadRepository interface
```

要求：

- 无 Create；
-无 Update；
-无 Delete；
-数据库连接设置只读；
-不暴露事务写方法；
-命名明确；
-禁止复用旧可写 Repository 接口。

---

## 四十三、数据库只读保护

可采用：

- 只读连接；
-SQLite immutable/只读模式；
-数据库用户权限；
-事务级 read-only；
-代码接口限制；
-测试中检测写入。

不要只依赖开发约定。

---

## 四十四、迁移报告

建议包含：

### 总体统计

- 旧对象数量；
-可迁移；
-警告；
-阻塞；
-孤儿；
-重复；
-损坏；
-手动确认。

### 分类统计

- Tool；
-Agent Skill；
-Plugin；
-MCP；
-Workflow；
-Package；
-Scope；
-Permission；
-Resource；
-Run；
-Audit。

### 风险

- 权限扩大风险；
-Scope 扩大风险；
-资源误删风险；
-重复执行风险；
-状态冲突；
-Artifact 缺失。

---

## 四十五、Migration Warning

统一 Warning Code：

```text
legacy_field_ignored
legacy_handler_not_migrated
legacy_cache_discarded
legacy_runtime_state_discarded
legacy_scope_ambiguous
legacy_enablement_conflict
legacy_permission_unresolved
legacy_artifact_missing
legacy_hash_missing
legacy_owner_unknown
legacy_version_invalid
legacy_reference_broken
legacy_status_unknown
```

---

## 四十六、迁移可观测性

每次 Snapshot、Validate、Plan、Dry Run 需要：

- Operation；
-Trace；
-状态；
-耗时；
-对象数量；
-错误；
-警告；
-内存；
-磁盘；
-取消；
-审计。

大批量记录避免逐条写过量日志，可写摘要和错误样本。

---

## 四十七、取消与恢复

Snapshot、Validation 和 Plan 生成必须支持取消。

取消后：

- 标记 cancelled；
-清理临时文件；
-保留已生成报告；
-不留下半完成可用 Plan；
-可重新运行；
-不修改旧数据。

---

## 四十八、Snapshot 保留

Snapshot 是临时迁移资产。

必须记录：

- Owner；
-创建时间；
-大小；
-Hash；
-来源；
-引用 Plan；
-过期时间；
-清理状态。

被 Plan 引用时不得删除。

---

## 四十九、资源所有权接入

迁移资源纳入第 12 步模型：

### Snapshot

```text
resource_type=artifact
owner=migration
```

### Migration Report

```text
resource_type=artifact
owner=migration
```

### Temporary Database

```text
resource_type=temporary_directory/file
owner=migration
```

### Mapping

属于迁移持久数据。

---

## 五十、统一审计接入

必须记录：

- Snapshot 创建；
-Snapshot 读取；
-Artifact 读取；
-Validation；
-Conflict；
-Plan；
-Dry Run；
-取消；
-删除 Snapshot；
-人工确认；
-只读保护违规；
-旧表写入检测。

不得记录 Secret 内容。

---

## 五十一、旧写入检测

本步骤必须增加检测：

- 哪些代码仍写旧 Skill 表；
-哪些代码仍写旧 Plugin 表；
-哪些代码仍写旧 MCP 状态；
-哪些代码仍写旧 Workflow；
-哪些代码仍写旧 Enabled；
-哪些 API 仍直接操作旧 Manager；
-哪些启动流程仍更新旧状态。

输出：

```text
legacy_write_report
```

---

## 五十二、禁止双写

新领域 Service 一旦开始落地：

```text
只能写新表
```

兼容旧查询必须通过：

- 兼容视图；
-只读转换；
-新表派生旧响应。

禁止：

```text
new_service.save()
→ new_repository.save()
→ legacy_repository.save()
```

---

## 五十三、CI 约束

建议增加静态检查或测试：

- 新 Kernel 包不得 import 旧 Manager；
- Migration 包只能 import 旧 Read Repository；
-旧可写 Repository 不得被 Migration 包引用；
-新 Service 不得写旧表；
-禁止 `legacy` 反向调用 `kernel` 后再回写；
-迁移 Adapter 无 Handler 调用；
-迁移测试检测数据库无变化。

---

## 五十四、包依赖方向

推荐：

```text
legacy/readmodel
        ↓
migration/adapters
        ↓
migration/canonical
        ↓
migration/planner
        ↓
kernel/import
```

禁止：

```text
kernel
→ legacy manager
```

允许旧兼容层依赖 Kernel 查询接口，但不得成为新 Kernel 的依赖。

---

## 五十五、前端迁移页面

本步骤可以建立最小诊断页面，展示：

- Snapshot；
-数据统计；
-迁移可用性；
-冲突；
-警告；
-Plan；
-Dry Run；
-未迁移对象；
-旧写入检测；
-Artifact 缺失。

不在本步骤提供“立即正式迁移”按钮。

---

## 五十六、用户确认项

未来正式迁移前需要用户确认：

- 冲突 Stable ID；
-未知 Owner；
-共享 MCP；
-用户修改 Workflow；
-权限 Grant；
-无法识别 Skill；
-损坏 Artifact；
-孤儿资源；
-是否保留旧历史；
-是否禁用不兼容扩展。

本步骤只生成确认清单。

---

## 五十七、错误隔离

单个损坏对象不得阻断所有 Snapshot。

策略：

- 记录 Entity Error；
-跳过或隔离；
-继续读取其他对象；
-总报告标记 partial；
-关键根对象损坏时阻塞其依赖；
-不伪造空 Definition。

---

## 五十八、版本化

Migration DTO 必须有 Schema Version：

```text
migration_schema_version
```

Adapter 需声明支持：

```text
legacy_schema_range
target_migration_schema
```

未来旧数据版本变化时增加 Adapter，不修改历史 Snapshot 解释方式。

---

## 五十九、测试要求

必须新增：

### 1. Read-Only Gateway

- Snapshot；
-分页；
-单项；
-统计；
-取消；
-并发；
-只读写入拒绝。

### 2. Snapshot

- 一致性；
-Hash；
-重复创建；
-损坏；
-删除；
-引用保护；
-大数据。

### 3. Artifact Reader

- 正常；
-缺失；
-Hash 错误；
-路径越界；
-链接；
-大文件；
-损坏包；
-Sealed Staging。

### 4. Skill Adapter

- Tool；
-Agent Skill；
-Workflow Wrapper；
-MCP Tool Wrapper；
-Internal Tool；
-Unknown。

### 5. Agent Skill Adapter

- SKILL.md；
-Resource；
-MCP Dependency；
-Scope；
-Enabled；
-损坏 Artifact；
-旧 Handler。

### 6. Plugin Adapter

- Go Plugin；
-Tool；
-Hook；
-Event；
-Schedule；
-State；
-Circuit；
-资源；
-无函数指针输出。

### 7. MCP Adapter

- stdio；
-HTTP；
-OAuth；
-Tool；
-Scope；
-Enabled；
-手动断开；
-旧 Session；
-共享依赖。

### 8. Workflow Adapter

- Definition；
-Skill Wrapper；
-Schedule；
-Run；
-Node；
-Scope；
-旧 Handler。

### 9. Package Adapter

- Manifest；
-Artifact；
-Hash；
-签名；
-资源；
-历史版本；
-损坏状态。

### 10. Permission/Scope

- 精确映射；
-冲突；
-不扩大；
-孤儿；
-角色/会话关系。

### 11. Run/Audit

- 父子完整；
-父子缺失；
-状态未知；
-副作用；
-重复记录。

### 12. Ownership

- 明确 Owner；
-未知 Owner；
-共享；
-引用；
-孤儿；
-删除策略。

### 13. Normalizer

- 时间；
-版本；
-ID；
-路径；
-编码；
-枚举；
-空值；
-跨平台。

### 14. Conflict Detector

覆盖全部冲突代码。

### 15. Planner

- 依赖顺序；
-阻塞；
-跳过；
-手动确认；
-Plan Hash；
-稳定重放。

### 16. Dry Run

- 无写入；
-ID 冲突；
-依赖缺失；
-容量；
-重复导入；
-取消。

### 17. CI Boundary

- 禁止 import；
-禁止写旧表；
-禁止调用 Handler；
-禁止双写。

### 18. 性能

- 大量 Tool；
-大量 Run；
-大量 Artifact；
-分页；
-内存；
-磁盘；
-并发 Snapshot。

---

## 六十、实施任务

### Task 1：建立旧数据 Schema 清单

确认所有旧表、Artifact、版本和依赖。

### Task 2：定义 LegacyReadOnlyGateway

只提供 Snapshot 和查询能力。

### Task 3：建立只读 Repository

替换迁移代码对旧可写 Repository 的依赖。

### Task 4：实现 SnapshotService

生成一致性快照和 Hash。

### Task 5：实现 LegacyArtifactReader

接入 Package Security 和路径保护。

### Task 6：定义 Canonical Migration DTO

统一所有迁移对象。

### Task 7：实现 Stable ID Mapping

建立 Legacy 到 Canonical 映射。

### Task 8：实现 SkillAdapter

拆分旧 Skill 类型。

### Task 9：实现 AgentSkillAdapter

迁移 Definition、Resource、Dependency 和 Scope。

### Task 10：实现 PluginAdapter

迁移 Legacy Go Runtime 和 Contributions。

### Task 11：实现 MCPAdapter

迁移 Server、Tool、Credential Reference 和 Scope。

### Task 12：实现 WorkflowAdapter

迁移 Definition、Tool Exposure、Schedule 和历史。

### Task 13：实现 PackageAdapter

迁移 Manifest、Artifact、资源和版本提示。

### Task 14：实现 PermissionAdapter

不扩大 Grant。

### Task 15：实现 ScopeAdapter

不扩大 Scope。

### Task 16：实现 RuntimeRecordAdapter

统一 Operation、Invocation、Attempt 和 Audit。

### Task 17：实现 OwnershipAdapter

迁移 Owner、Reference 和孤儿状态。

### Task 18：实现 MigrationNormalizer

统一格式和枚举。

### Task 19：实现 MigrationValidator

生成 Valid、Warning、Blocked 和 Review。

### Task 20：实现 ConflictDetector

输出结构化冲突。

### Task 21：实现 MigrationPlanner

生成稳定、可重放 Plan。

### Task 22：定义 NewKernelImportBoundary

先实现 Validate 和 Dry Run。

### Task 23：实现 Migration Report Store

保存统计、冲突和警告。

### Task 24：接入 Resource Ownership

登记 Snapshot、Report 和临时文件。

### Task 25：接入统一 Audit

追踪迁移读取和计划生成。

### Task 26：冻结新旧双写

增加运行时和 CI 检测。

### Task 27：建立 Legacy Write Report

统计旧写入入口。

### Task 28：建立最小迁移诊断页面

只提供 Snapshot、Plan 和 Dry Run。

### Task 29：完成数据损坏和故障注入测试

确保单个损坏对象不会拖垮全部迁移。

### Task 30：输出迁移准备报告

作为进入第 21 步的前置产物。

---

## 六十一、建议目录结构

建议：

```text
backend/internal/extension/migration/
├── gateway/
│   ├── read_only.go
│   ├── snapshot.go
│   ├── artifact_reader.go
│   └── statistics.go
├── legacy/
│   ├── skill_repository.go
│   ├── agent_skill_repository.go
│   ├── plugin_repository.go
│   ├── mcp_repository.go
│   ├── workflow_repository.go
│   ├── package_repository.go
│   ├── permission_repository.go
│   ├── scope_repository.go
│   └── runtime_repository.go
├── canonical/
│   ├── entity.go
│   ├── source.go
│   ├── owner.go
│   ├── dependency.go
│   ├── resource.go
│   ├── conflict.go
│   └── warning.go
├── adapters/
│   ├── skill.go
│   ├── agent_skill.go
│   ├── plugin.go
│   ├── mcp.go
│   ├── workflow.go
│   ├── package.go
│   ├── permission.go
│   ├── scope.go
│   ├── runtime.go
│   └── ownership.go
├── normalize/
│   ├── normalizer.go
│   ├── id.go
│   ├── version.go
│   ├── time.go
│   └── path.go
├── validate/
│   ├── validator.go
│   ├── conflict_detector.go
│   └── integrity.go
├── plan/
│   ├── planner.go
│   ├── dependency_order.go
│   ├── dry_run.go
│   └── report.go
└── boundary/
    └── kernel_import.go
```

前端：

```text
front/src/views/extensions/migration/
├── MigrationSnapshotView.vue
├── MigrationInventoryView.vue
├── MigrationConflictView.vue
├── MigrationPlanView.vue
├── MigrationDryRunView.vue
└── LegacyWriteReportView.vue
```

目录仅为建议。

---

## 六十二、性能要求

建议：

- 所有 List 接口分页；
-Snapshot 流式导出；
-Artifact 按需读取；
-大 Run 历史分批；
-Hash 流式计算；
-Adapter 批处理；
-冲突检测使用索引；
-Plan 生成避免全量嵌套查询；
-报告保存摘要；
-错误样本限制数量；
-并发读取有界；
-取消可快速停止；
-不在内存中同时加载全部 Artifact。

---

## 六十三、风险控制

### P0：迁移扩大权限或作用域

- 多个旧 Enabled 取 OR；
-角色 Scope 迁为 Global；
-未知 Permission 自动 Allow；
-共享资源归单一 Extension；
-旧 Runtime Ready 被迁为 Enabled。

### P1：迁移产生副作用

- 读取触发 MCP 连接；
-读取触发 Plugin Start；
-读取触发 Workflow 恢复；
-Artifact Reader 执行代码；
-旧 Manager 修复数据；
-新系统回写旧表。

### P2：数据不可重放

- 实时读取旧表；
-随机 ID；
-依赖当前时间；
-Plan 无 Hash；
-Snapshot 无 Revision；
-Artifact 变化未检测。

### P3：损坏与性能

- 单个损坏对象阻断全部；
-大量 Run 占满内存；
-Snapshot 锁表过久；
-Artifact 扫描全盘；
-报告过大。

---

## 六十四、本步骤不做的事情

本步骤明确不做：

- 不正式迁移全部生产数据；
-不切换 Extension Kernel 为主运行链；
-不删除旧表；
-不删除旧 Manager；
-不实现 CommitImport；
-不实现完整 RollbackImport；
-不启用新 Runtime；
-不修改用户 Permission；
-不修改用户 Scope；
-不自动修复未知 Owner；
-不持续双写；
-不实现复杂 CDC；
-不开放远程迁移 API；
-不把旧 Manifest 直接升级为 v2。

---

## 六十五、验收产物

完成后必须提交：

### 1. 只读迁移接口主文档

```text
docs/extension-kernel/20-read-only-migration-boundary.md
```

### 2. LegacyReadOnlyGateway

支持：

- Snapshot；
-分页；
-单对象；
-Artifact；
-统计。

### 3. 只读 Repository

覆盖旧：

- Skill；
-Agent Skill；
-Plugin；
-MCP；
-Workflow；
-Package；
-Permission；
-Scope；
-Run；
-Resource。

### 4. Canonical Migration DTO

不依赖旧领域 Struct。

### 5. 全部 Adapter

至少包含：

- Skill；
-Agent Skill；
-Plugin；
-MCP；
-Workflow；
-Package；
-Permission；
-Scope；
-Runtime；
-Ownership。

### 6. Normalizer、Validator、Conflict Detector

输出稳定结果。

### 7. Migration Planner

支持依赖排序、阻塞、警告、手动确认和 Plan Hash。

### 8. NewKernelImportBoundary

至少支持 Validate 和 Dry Run。

### 9. Migration Report

包含统计、冲突、警告、孤儿和权限风险。

### 10. Legacy Write Report

列出所有剩余旧写入入口。

### 11. CI 约束

阻止：

- 新 Kernel import 旧 Manager；
-Migration import 旧可写 Repository；
-新 Service 写旧表；
-迁移调用 Handler；
-新旧双写。

### 12. 迁移诊断页面

只提供 Snapshot、Inventory、Conflict、Plan 和 Dry Run。

### 13. 测试报告

覆盖只读、Snapshot、Artifact、Adapter、Normalization、Conflict、Planner、Dry Run、幂等、损坏、性能和边界约束。

---

## 六十六、验收标准

本步骤通过必须满足：

1. 旧数据读取只有统一 LegacyReadOnlyGateway。
2. Migration 包不依赖旧 Manager。
3. Migration 包不调用旧 Handler。
4. 所有迁移读取绑定 Snapshot。
5. Snapshot 可稳定重放。
6. Artifact 读取经过路径和完整性校验。
7. 旧 Skill 已被分类，不再统一迁为 Skill。
8. Canonical Migration DTO 与旧领域 Struct 分离。
9. Stable ID Mapping 可追溯。
10. Enabled、Scope 和 Permission 迁移遵循不扩大原则。
11. 未知 Owner 进入 migration/orphaned。
12. 旧 Session、Cache、闭包和内存 Runtime 不迁移为有效状态。
13. 迁移冲突有结构化报告。
14. 单个损坏对象不会阻断全部快照。
15. Migration Plan 具有稳定 Hash 和依赖顺序。
16. Dry Run 不写正式业务表。
17. 新系统不回写旧表。
18. 新旧系统不存在双写。
19. CI 可以阻止反向依赖。
20. Legacy Write Report 可以列出剩余旧入口。
21. 关键测试通过。
22. 后续第 21 步可以正式定义 Extension Kernel 领域模型。

---

## 六十七、退出条件

只有满足以下条件后，才能进入第 21 步“定义 Extension Kernel 领域模型”：

- LegacyReadOnlyGateway 已落地；
-SnapshotService 已落地；
-旧只读 Repository 已落地；
-Canonical Migration DTO 已落地；
-全部主要 Adapter 已落地；
-Normalizer、Validator、Conflict Detector 已落地；
-Migration Planner 已落地；
-NewKernelImportBoundary 的 Validate/Dry Run 已落地；
-Legacy Write Report 已生成；
-CI 反向依赖约束已生效；
-新旧双写已禁止；
-迁移准备报告通过验收。

---

## 六十八、执行约束

执行本步骤时必须遵守：

> 迁移层只能读取旧系统的持久化事实并转换为新系统可理解的中立 DTO，不能调用旧运行逻辑，不能修复旧系统，不能回写旧系统，也不能提前替新 Kernel 决定最终运行状态。

禁止出现：

- Migration Adapter 调用 PluginManager；
-Migration Adapter 调用 MCP Connect；
-Migration Adapter 调用 Workflow Executor；
-Migration Adapter 读取 Handler；
-新 Kernel 通过 Legacy Gateway 执行功能；
-Snapshot 创建后继续混读实时旧表；
-旧 Enabled 冲突取 OR；
-未知 Permission 默认 Allow；
-未知 Scope 默认 Global；
-未知 Owner 默认归 Extension；
-Dry Run 写正式业务表；
-新旧数据长期双写；
-兼容层成为新系统运行依赖。

本步骤完成后，Amitia 必须具备一套单向、只读、可追溯、可重放、可校验、无副作用且不会扩大权限的旧系统迁移边界。
