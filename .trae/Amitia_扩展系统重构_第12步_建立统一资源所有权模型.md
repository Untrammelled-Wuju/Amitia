# Amitia 扩展系统重构第 12 步实施文档

## 第 12 步：建立统一资源所有权模型

---

## 一、步骤目标

在第 3 步已经完成数据表与资源归属盘点、第 10 步已经统一 Scope、第 11 步已经统一运行记录与审计模型的基础上，正式建立 Amitia 唯一的 Resource Ownership Model。

本步骤的目标是：

> 让每一个由扩展、模块、Tool、Agent Skill、MCP、Workflow、后台任务、UI Contribution、Provider 或系统运行时创建的资源，都有唯一所有者、明确引用关系、明确生命周期和明确清理责任。

当前系统中资源所有权可能分散在：

- Package；
-Plugin；
-Agent Skill；
-MCP Dependency；
-MCP Server；
-Workflow；
-Owned Resource；
-Scope Binding；
-Permission Grant；
-Artifact；
-Secret；
-Plugin Event；
-Plugin Schedule；
-运行时 Cache；
-子进程；
-临时目录；
-前端本地缓存；
-角色绑定；
-用户手动配置。

这种分散设计会导致：

- 扩展卸载时无法判断哪些资源应删除；
- Agent Skill 创建的 MCP Server 被用户修改后所有权不清；
- 多个扩展共享同一个 MCP Server 时引用计数不可靠；
- Workflow 被包安装后又被用户复制，源资产与用户资产混淆；
- Plugin 禁用后后台任务仍运行；
- Package 回滚后旧资源没有恢复；
- Secret 删除不完整；
- 角色删除误删用户资产；
- 临时文件、Timer、Worker 和子进程泄漏；
- Registry 已清理但数据库或文件仍残留；
- 数据库存在 Owned Resource 记录，但实际资源创建链未统一接入。

本步骤完成后，系统必须形成统一资源生命周期：

```text
Resource Created
→ Ownership Assigned
→ References Registered
→ Scope Bound
→ Runtime Activated
→ Updated / Shared
→ Disabled / Suspended
→ Migrated / Upgraded
→ Uninstalled / Released
→ Retained / Deleted / Orphaned
→ Audited
```

---

## 二、核心原则

### 1. 每项资源必须只有一个生命周期所有者

资源可以被多个对象引用，但只能有一个对象负责：

- 创建；
-升级；
-迁移；
-删除；
-最终清理；
-恢复；
-回滚。

不得使用：

```text
由 Package 和 MCP 共同管理
```

作为最终结论。

必须明确：

```text
所有者：Extension Package
管理器：MCP Manager
引用者：Agent Skill、角色、Workflow
```

其中：

- 所有者决定删除；
-管理器负责运行；
-引用者只持有引用。

---

### 2. 所有权、运行责任、存储责任分离

建议区分：

```text
Owner
Runtime Manager
Storage Manager
Reference Holder
Scope Holder
Permission Subject
```

例如 MCP Server：

```text
Owner：用户或 Extension
Runtime Manager：MCP Manager
Storage Manager：MCP Repository + Secret Broker
Reference Holder：Agent Skill / Workflow / Extension Module
Scope Holder：Scope Manager
Permission Subject：MCP Server / Tool
```

---

### 3. 禁用不等于删除

禁用资源时：

- 定义和数据保留；
-运行时停止或不可用；
-引用保留；
-Scope Binding 保留；
-Permission Grant 保留但不可使用；
-审计保留。

卸载或删除时才进入资源释放流程。

---

### 4. 用户资产优先保护

任何资源只要经过用户明确编辑、复制、转移或脱离扩展管理，就不能再被扩展卸载直接删除。

必须支持：

```text
Extension-owned
→ User-adopted
```

所有权转移。

---

### 5. 共享资源必须引用计数或引用图确认

删除共享资源前必须确认：

- 当前所有者；
-全部引用者；
-是否存在运行中任务；
-是否存在历史回滚依赖；
-是否存在其他 Extension 依赖；
-是否存在用户资产依赖。

不得只根据一个 `ref_count` 字段做最终判断，引用计数必须可由关系表重新计算。

---

## 三、资源领域模型

建议定义：

```go
type ResourceRecord struct {
    ResourceID      string
    ResourceType    ResourceType
    Owner           ResourceOwner
    RuntimeManager  string
    StorageManager  string
    State           ResourceState
    ScopeRefs       []ScopeRef
    Version         string
    CreatedAt       time.Time
    UpdatedAt       time.Time
    DeletedAt       *time.Time
    Metadata        map[string]any
}
```

---

## 四、资源类型

建议至少支持：

```go
type ResourceType string

const (
    ResourceExtensionPackage ResourceType = "extension_package"
    ResourceExtensionModule  ResourceType = "extension_module"
    ResourceTool             ResourceType = "tool"
    ResourceAgentSkill       ResourceType = "agent_skill"
    ResourceMCPServer        ResourceType = "mcp_server"
    ResourceMCPTool          ResourceType = "mcp_tool"
    ResourceWorkflow         ResourceType = "workflow"
    ResourceUIContribution   ResourceType = "ui_contribution"
    ResourceHook             ResourceType = "hook"
    ResourceBackgroundTask   ResourceType = "background_task"
    ResourceSchedule         ResourceType = "schedule"
    ResourceEventSubscription ResourceType = "event_subscription"
    ResourceProvider         ResourceType = "provider"
    ResourceSecret           ResourceType = "secret"
    ResourceStorageNamespace ResourceType = "storage_namespace"
    ResourceFile             ResourceType = "file"
    ResourceArtifact         ResourceType = "artifact"
    ResourceCache            ResourceType = "cache"
    ResourceProcess          ResourceType = "process"
    ResourceConnection       ResourceType = "connection"
    ResourceTemporaryDirectory ResourceType = "temporary_directory"
    ResourceWindow           ResourceType = "window"
    ResourceTrayAction       ResourceType = "tray_action"
)
```

---

## 五、Owner 模型

建议：

```go
type ResourceOwner struct {
    OwnerType   OwnerType
    OwnerID     string
    ExtensionID string
    ModuleID    string
}
```

OwnerType：

```text
system
user
extension
module
shared
temporary
migration
```

---

## 六、Owner 类型含义

### System

Amitia 内置资源。

特点：

- 不允许普通扩展卸载；
-版本随宿主升级；
-权限由系统策略定义；
-资源 ID 稳定。

### User

用户手动创建或已接管的资源。

特点：

- 扩展卸载不得删除；
-可被多个扩展引用；
-删除必须由用户明确操作；
-配置优先由用户控制。

### Extension

属于某个 `.amitiax` 包。

特点：

- 安装时创建；
-升级时迁移；
-回滚时恢复；
-卸载时进入释放流程。

### Module

属于扩展中的特定模块。

特点：

- 模块禁用时停止；
-扩展卸载时删除；
-模块卸载或移除时单独释放。

### Shared

由共享资源管理器持有。

特点：

- 多个 Extension 或用户对象引用；
-删除需引用图为空；
-不能由单个引用者直接删除。

### Temporary

运行时临时资源。

特点：

- 必须有过期时间；
-必须有清理函数；
-应用退出时清理；
-默认不可恢复。

### Migration

仅在旧系统迁移期间存在。

特点：

- 迁移完成后转移所有权；
-或删除；
-不得成为长期业务资源。

---

## 七、资源状态

建议统一：

```go
type ResourceState string

const (
    ResourceStatePending     ResourceState = "pending"
    ResourceStateActive      ResourceState = "active"
    ResourceStateDisabled    ResourceState = "disabled"
    ResourceStateSuspended   ResourceState = "suspended"
    ResourceStateUpdating    ResourceState = "updating"
    ResourceStateRollingBack ResourceState = "rolling_back"
    ResourceStateDeleting    ResourceState = "deleting"
    ResourceStateDeleted     ResourceState = "deleted"
    ResourceStateFailed      ResourceState = "failed"
    ResourceStateOrphaned    ResourceState = "orphaned"
    ResourceStateRetained    ResourceState = "retained"
)
```

---

## 八、资源引用模型

建议：

```go
type ResourceReference struct {
    ReferenceID      string
    SourceResourceID string
    TargetResourceID string
    ReferenceType    ReferenceType
    Required         bool
    OwnershipEffect  OwnershipEffect
    CreatedAt        time.Time
    Metadata         map[string]any
}
```

ReferenceType：

```text
depends_on
contains
uses
generated_from
installed_by
owned_by
scoped_by
secured_by
scheduled_by
rendered_by
runtime_managed_by
```

OwnershipEffect：

```text
none
retain_target
block_delete
cascade_delete
transfer_on_delete
prompt_user
```

---

## 九、引用图规则

删除资源前必须构建引用图。

例如：

```text
Extension
├── Module
│   ├── Tool
│   ├── Workflow
│   ├── UI Contribution
│   └── MCP Server
└── Agent Skill
    └── depends_on MCP Server
```

删除 Extension 时：

1. 查询所有直接拥有资源；
2. 查询外部引用；
3. 区分包内引用与外部引用；
4. 识别用户修改；
5. 识别共享资源；
6. 识别历史版本依赖；
7. 生成删除预览；
8. 执行用户选择；
9. 记录审计；
10. 进行原子释放或补偿。

---

## 十、资源所有权服务

建议定义：

```go
type ResourceOwnershipService interface {
    Register(
        ctx context.Context,
        resource ResourceRecord,
    ) error

    AddReference(
        ctx context.Context,
        ref ResourceReference,
    ) error

    RemoveReference(
        ctx context.Context,
        referenceID string,
    ) error

    TransferOwnership(
        ctx context.Context,
        request OwnershipTransferRequest,
    ) error

    PlanRelease(
        ctx context.Context,
        request ResourceReleaseRequest,
    ) ResourceReleasePlan

    ExecuteRelease(
        ctx context.Context,
        plan ResourceReleasePlan,
    ) ResourceReleaseResult

    ListOwned(
        ctx context.Context,
        owner ResourceOwner,
    ) ([]ResourceRecord, error)

    ListReferences(
        ctx context.Context,
        resourceID string,
    ) ([]ResourceReference, error)
}
```

---

## 十一、资源注册时机

所有资源必须在创建成功后立即注册所有权。

推荐顺序：

```text
1. 创建 Pending Resource Record
2. 创建实际资源
3. 注册引用
4. 绑定 Scope
5. 注册 Permission Subject
6. 激活 Runtime
7. 标记 Active
8. 写 Audit
```

如果实际资源创建失败：

- 删除 Pending Record；
-清理临时文件；
-撤销已创建引用；
-不得残留半激活资源。

---

## 十二、资源释放计划

建议：

```go
type ResourceReleasePlan struct {
    PlanID             string
    RootResourceID     string
    DeleteResources    []ResourceAction
    RetainResources    []ResourceAction
    TransferResources  []ResourceAction
    Blockers           []ResourceBlocker
    UserDecisions      []RequiredUserDecision
    RollbackSnapshotID string
}
```

必须支持：

- Dry Run；
-预览；
-阻塞项；
-用户选择；
-所有权转移；
-保留数据；
-删除数据；
-清理 Runtime；
-回滚。

---

## 十三、删除策略

每项资源必须指定：

```text
cascade
retain
transfer
prompt
block
rebuildable
```

### Cascade

只适用于完全私有、无外部引用资源。

### Retain

保留数据或历史记录。

### Transfer

转为用户或 Shared 所有。

### Prompt

需要用户决定。

### Block

存在依赖，禁止删除。

### Rebuildable

删除派生资源，后续可重建。

---

## 十四、用户接管

用户对扩展资源进行以下操作后，应评估是否发生所有权转移：

- 修改 MCP Server 配置；
-复制 Workflow；
-编辑 Agent Skill；
-增加自定义资源；
-修改 Provider 配置；
-将包内配置设为共享；
-导出后重新导入；
-解除扩展关联。

支持：

```text
adopt
clone
detach
```

### Adopt

原资源转为用户所有。

### Clone

创建用户副本，原资源仍属 Extension。

### Detach

保留资源，但解除扩展更新与删除关系。

---

## 十五、Extension Package 所有权

`.amitiax` Package 是根所有者之一。

一个 Package 可拥有：

- Module；
-Tool；
-Agent Skill；
-MCP Definition；
-Workflow；
-UI；
-Hook；
-Background Task；
-Provider；
-Artifact；
-Secret Namespace；
-Storage Namespace。

Package 卸载时必须通过 Resource Release Plan。

不得在 PackageService 内硬编码：

```text
if workflow ...
if instructions ...
```

而应按 Resource Ownership 图处理。

---

## 十六、Module 所有权

Module 可以拥有：

- Tool；
-UI；
-Hook；
-Worker；
-Schedule；
-事件订阅；
-配置；
-缓存；
-Secret；
-子进程。

Module 禁用：

- Runtime 资源停止；
-持久资源保留；
-Contribution 不可见；
-引用保留。

Module 删除：

- 释放其私有资源；
-检查外部引用；
-必要时转移资源。

---

## 十七、Tool 所有权

ToolDefinition 属于 Extension、Module、System、User 或 Shared。

Tool 本身通常是定义资源，执行产生的业务资源不能自动归 Tool 所有。

例如：

```text
Tool 创建一个日程
```

日程的所有者应是：

- 用户；
-当前 Workflow；
-当前 Extension；
-或系统业务模块；

必须由 Tool 结果明确声明。

Tool 注销时：

- 删除定义；
-取消排队调用；
-运行中调用按策略结束；
-审计保留；
-执行产生的用户资产默认保留。

---

## 十八、Agent Skill 所有权

Agent Skill 可以来自：

- 用户导入；
-Extension Package；
-系统内置；
-迁移。

Agent Skill 资源包括：

- SKILL.md；
-References；
-Assets；
-Tool Mapping；
-MCP References；
-Activation Metadata；
-Catalog Cache。

卸载 Extension 时：

- 包内未修改 Agent Skill 可删除；
-用户已修改或接管时需保留或转移；
-共享 MCP Server 不自动删除；
-激活历史保留；
-派生 Cache 删除。

---

## 十九、MCP Server 所有权

MCP Server 所有权必须明确区分：

### 用户创建

```text
owner=user
```

Extension 只能引用，不能删除。

### Extension 创建

```text
owner=extension/module
```

卸载时：

- 若无外部引用，可删除；
-若用户已接管，转为 user；
-若其他 Extension 引用，转为 shared 或阻止删除。

### Agent Skill 依赖自动创建

所有者不应是 Agent Skill 本体，而应根据安装来源确定：

- 包内 Agent Skill：Extension；
-用户导入 Agent Skill：User 或 temporary pending；
-共享依赖：Shared。

MCP Tool 默认由 MCP Server 拥有。

---

## 二十、Workflow 所有权

Workflow 来源：

- 用户创建；
-Extension Package；
-Workshop；
-系统内置；
-导入；
-迁移。

需要区分：

```text
definition owner
runtime execution owner
generated business resource owner
```

包内 Workflow 被用户编辑时应支持：

- Fork 为用户 Workflow；
-继续跟随包更新；
-拒绝更新；
-合并冲突。

不能静默覆盖用户修改。

---

## 二十一、UI Contribution 所有权

UI Contribution 属于 Extension Module。

资源包括：

- 导航项；
-页面；
-Slot；
-Widget；
-消息渲染器；
-输入栏按钮；
-托盘动作；
-窗口。

禁用 Module 时：

- 立即从 Registry 移除；
-已打开页面显示卸载/不可用状态；
-独立窗口关闭；
-缓存清理；
-用户输入按策略保存。

卸载时不得遗留静态路由和前端 Store。

---

## 二十二、Hook、Event 与 Schedule 所有权

### Hook

属于 Module。

禁用时立即注销。

### Event Subscription

属于 Module 或 Background Service。

禁用和卸载时必须取消订阅。

### Schedule

属于：

- Extension；
-Module；
-Workflow；
-User。

Schedule 必须记录 Owner。

Extension 卸载时：

- Extension-owned Schedule 删除；
-User-owned Schedule 保留并禁用或重新绑定；
-运行中任务按策略取消。

---

## 二十三、Secret 所有权

Secret 必须通过 Secret Broker 管理。

每个 Secret 必须记录：

- Secret ID；
-Owner；
-用途；
-作用域；
-读取权限；
-创建时间；
-更新时间；
-是否可导出；
-删除策略；
-轮换策略。

Extension 卸载时：

- Extension 私有 Secret 删除；
-共享 Secret 解除引用；
-用户 Secret 保留；
-审计保留；
-不得在日志中记录内容。

---

## 二十四、Storage Namespace 所有权

每个 Extension/Module 使用隔离存储命名空间。

例如：

```text
extensions/com.example.weather/global/
extensions/com.example.weather/modules/main/
extensions/com.example.weather/characters/<id>/
extensions/com.example.weather/conversations/<id>/
```

必须由 Storage Broker 分配。

插件不得自行构造主数据目录路径。

卸载时支持：

- 删除全部；
-保留配置；
-保留用户数据；
-导出；
-转移为用户资产。

---

## 二十五、Artifact 所有权

Artifact 包括：

- 原始 `.amitiax`；
-解压内容；
-历史版本；
-回滚快照；
-编译产物；
-迁移备份；
-Workshop 产物。

所有者可能是：

- Package；
-User；
-Migration；
-System。

历史版本 Artifact 在以下情况下不可删除：

- 当前可回滚；
-迁移未完成；
-审计引用；
-签名校验需要；
-用户明确保留。

---

## 二十六、Cache 所有权

Cache 必须标记源资源。

例如：

```text
Agent Skill Catalog Cache
→ generated_from Agent Skill

MCP Discovery Cache
→ generated_from MCP Server

UI Schema Cache
→ generated_from UI Contribution
```

Cache 默认：

- 可删除；
-不可作为唯一真值；
-源资源变化时失效；
-Extension 卸载时清理；
-应用启动可重建。

---

## 二十七、Process 与 Connection 所有权

子进程和连接属于 Runtime Manager，但所有权仍归创建资源。

例如：

```text
MCP stdio process
owner_resource = MCP Server
runtime_manager = MCP Manager
```

或：

```text
Plugin service process
owner_resource = Extension Module
runtime_manager = Runtime Supervisor
```

必须记录：

- PID/Connection ID；
-Owner Resource；
-启动时间；
-健康状态；
-关闭函数；
-崩溃恢复；
-孤儿检测；
-应用退出清理。

---

## 二十八、Temporary Resource

临时资源包括：

- 解压目录；
-上传文件；
-安装 Session；
-预览文件；
-开发者热重载目录；
-临时 MCP；
-临时窗口；
-临时端口；
-锁文件。

必须有：

```text
owner
expires_at
cleanup_handler
recovery_policy
```

应用异常退出后，启动恢复阶段必须扫描并清理过期临时资源。

---

## 二十九、资源所有权持久化

建议目标表：

```text
owned_resources
resource_references
resource_ownership_transfers
resource_release_plans
resource_release_operations
resource_orphan_reports
resource_cleanup_jobs
```

若当前已有 `owned_resource_repository`，应评估后改造复用。

Owned Resource 记录不得只保存：

```text
resource_type + resource_id
```

还必须保存：

- Owner；
-State；
-Runtime Manager；
-Storage Manager；
-Version；
-删除策略；
-作用域；
-引用关系；
-审计关联。

---

## 三十、孤儿资源检测

系统必须能识别：

- Owner 不存在；
-Target 不存在；
-Reference 指向删除资源；
-数据库存在但文件缺失；
-文件存在但数据库无记录；
-进程存在但 Owner 已删除；
-Secret 无引用；
-Schedule Owner 不存在；
-Tool Owner 不存在；
-MCP Tool 对应 Server 不存在；
-Cache 源资源不存在。

建议提供：

```text
orphan scan
orphan report
safe cleanup
manual review
```

高风险孤儿资源不得自动删除。

---

## 三十一、资源清理顺序

推荐：

```text
1. 阻止新调用
2. 标记 deleting
3. 停止 Runtime
4. 取消 Schedule/Worker
5. 关闭 Connection/Process
6. 注销 Tool/UI/Hook
7. 解除 Scope
8. 撤销或冻结 Permission
9. 处理外部引用
10. 转移或保留用户资产
11. 删除 Secret
12. 删除 Storage
13. 删除 Artifact
14. 删除 Definition
15. 保留 Audit
16. 标记 deleted
```

不得先删除定义，再尝试关闭运行时。

---

## 三十二、升级与回滚

升级前必须生成：

- 当前资源快照；
-Owner 图；
-Reference 图；
-配置快照；
-Secret 引用；
-运行时状态；
-版本 Artifact。

升级时：

- 新资源注册为 pending；
-迁移；
-切换引用；
-激活；
-旧资源转 retained 或删除；
-失败时回滚。

回滚必须恢复：

- 定义；
-Owner；
-Reference；
-Scope；
-Permission Requirement；
-Artifact；
-配置；
-运行时绑定。

---

## 三十三、Extension 卸载预览

前端必须展示：

```text
将删除
将保留
将转移
需要确认
阻塞卸载
```

示例：

```text
删除：
- 3 个插件工具
- 1 个 UI 页面
- 2 个后台任务

保留：
- 用户修改后的 Workflow
- 用户手动创建的 MCP Server

转移：
- 1 个共享 MCP Server → 用户所有

阻塞：
- 另一个扩展仍依赖该 Provider
```

---

## 三十四、与 Scope、Permission、Audit 的关系

### Scope

资源可以拥有 Scope Binding，但 Scope Manager 不拥有资源。

### Permission

资源可以作为 Permission Subject 或 Target，但 Permission Broker 不拥有资源。

### Audit

资源生命周期操作必须写 Audit Event，但 Audit Store 不拥有资源。

### Runtime

Runtime Manager 负责运行，不决定资源是否删除。

---

## 三十五、旧系统迁移

必须迁移：

- Package Owned Resources；
-Agent Skill MCP Dependency；
-Plugin Schedule；
-Plugin Event Subscription；
-MCP Server Ownership；
-Workflow Package Relation；
-Artifact Relation；
-Secret Relation；
-Scope Binding；
-运行时 Process；
-Cache；
-临时文件。

旧资源无法确定 Owner 时：

```text
owner_type = migration
state = orphaned
```

必须进入人工或规则化确认，不得自动归 Extension。

---

## 三十六、兼容层约束

迁移期间允许旧系统查询新所有权服务。

禁止：

```text
旧 PackageService 维护一份所有权
+
新 ResourceOwnershipService 再维护一份
```

新资源只注册新所有权模型。

旧资源通过迁移器导入。

---

## 三十七、前端资源管理

扩展详情页需要增加：

- 资源列表；
-Owner；
-引用者；
-State；
-作用域；
-存储占用；
-Secret 数量；
-后台任务；
-进程；
-连接；
-卸载策略；
-转移入口；
-孤儿状态。

开发者控制台需要支持：

- 查看插件创建资源；
-查看清理失败；
-查看泄漏；
-手动触发扫描；
-导出诊断信息。

---

## 三十八、测试要求

必须新增：

### 1. Owner 注册测试

- System；
-User；
-Extension；
-Module；
-Shared；
-Temporary；
-Migration。

### 2. Reference 图测试

- depends_on；
-contains；
-uses；
-generated_from；
-循环引用；
-重复引用。

### 3. Release Plan 测试

- 无引用删除；
-共享资源；
-用户接管；
-阻塞；
-转移；
-保留；
-回滚。

### 4. Extension 卸载测试

覆盖 Tool、Agent Skill、MCP、Workflow、UI、Hook、Schedule、Secret、Storage。

### 5. Module 禁用与删除测试

验证禁用不删除。

### 6. 用户资产保护测试

确保卸载不误删。

### 7. MCP 共享测试

多个扩展和用户引用。

### 8. Workflow Fork 测试

用户副本与包内定义分离。

### 9. Process/Connection 清理测试

验证无孤儿进程。

### 10. Temporary Resource 测试

过期和异常退出清理。

### 11. Upgrade/Rollback 测试

验证 Owner 与 Reference 恢复。

### 12. Orphan Scan 测试

数据库、文件、进程和 Secret 孤儿。

### 13. 并发删除测试

防止重复释放。

### 14. 审计测试

所有生命周期动作均可追踪。

---

## 三十九、实施任务

### Task 1：定义资源领域模型

完成 ResourceRecord、ResourceType、ResourceOwner、ResourceState、ResourceReference。

### Task 2：建立 ResourceOwnershipService

实现 Register、Reference、Transfer、PlanRelease、ExecuteRelease。

### Task 3：建立统一资源持久化

完成 Owned Resource、Reference、Transfer、Release Plan、Cleanup Job。

### Task 4：接入 Package

Package 成为根资源所有者之一。

### Task 5：接入 Module

统一模块资源。

### Task 6：接入 Tool

注册 Tool 所有权与注销关系。

### Task 7：接入 Agent Skill

统一包内、用户和迁移来源。

### Task 8：接入 MCP

统一 Server、Tool、Process、Connection 和 Secret。

### Task 9：接入 Workflow

统一 Definition、Schedule、Fork 和用户接管。

### Task 10：接入 UI、Hook、Event、Schedule

确保禁用与卸载完整清理。

### Task 11：接入 Storage 与 Secret Broker

统一命名空间所有权。

### Task 12：接入 Runtime Supervisor 预留接口

管理 Process、Connection、Worker 和临时资源。

### Task 13：实现 Release Plan

支持 Dry Run、预览、用户决策、阻塞和回滚。

### Task 14：实现 Ownership Transfer

支持 Adopt、Clone、Detach。

### Task 15：实现 Orphan Scanner

识别数据库、文件、Secret、进程和引用孤儿。

### Task 16：迁移旧 Owned Resource 数据

只读导入并标记不确定项。

### Task 17：重建卸载预览接口

前端显示删除、保留、转移和阻塞。

### Task 18：增加迁移统计

记录未纳入新模型的资源创建入口。

### Task 19：完成故障注入与清理测试

验证部分失败与补偿。

---

## 四十、建议目录结构

建议：

```text
backend/internal/extension/kernel/resource/
├── resource.go
├── type.go
├── owner.go
├── state.go
├── reference.go
├── service.go
├── storage.go
├── transfer.go
├── release_plan.go
├── release_executor.go
├── cleanup.go
├── orphan_scanner.go
├── migration.go
└── audit.go
```

前端：

```text
front/src/views/extensions/resources/
├── ResourceList.vue
├── ResourceGraph.vue
├── ResourceDetail.vue
├── OwnershipTransferDialog.vue
├── UninstallPreview.vue
└── OrphanReport.vue
```

目录仅为建议。

---

## 四十一、性能要求

资源图查询必须可控。

建议：

- Owner 和 Target 建索引；
-Reference 图按需加载；
-卸载预览避免全库扫描；
-大扩展资源分页；
-孤儿扫描增量执行；
-Process/Connection 由运行时索引；
-文件扫描限制目录；
-Secret 不读取内容；
-审计异步但高风险操作可靠写入。

---

## 四十二、风险控制

### P0：误删用户资产

- Extension 卸载删除用户 MCP；
-删除用户 Workflow；
-删除共享 Secret；
-级联删除错误。

### P1：资源泄漏

- 子进程；
-连接；
-Worker；
-Timer；
-临时目录；
-Secret；
-Tool 注册；
-UI 页面。

### P2：所有权漂移

- 数据库 Owner 与实际创建者不同；
-用户修改未触发接管；
-升级覆盖用户修改；
-共享资源引用不完整。

### P3：性能问题

- 卸载预览过慢；
-资源图过大；
-孤儿扫描频繁全盘；
-前端一次加载全部资源。

---

## 四十三、本步骤不做的事情

本步骤明确不做：

- 不实现完整 Extension Kernel 生命周期；
-不实现 `.amitiax` v2 Manifest；
-不实现第三方插件 Runtime；
-不实现 UI Contribution Registry；
-不实现完整 Storage Broker；
-不实现完整 Secret Broker；
-不删除旧 Owned Resource 表；
-不立即迁移生产数据；
-不实现插件市场；
-不实现移动端；
-不重构用户业务资产系统。

---

## 四十四、验收产物

完成后必须提交：

### 1. 资源所有权主文档

```text
docs/extension-kernel/12-unified-resource-ownership.md
```

### 2. 核心领域类型

至少包含：

- ResourceRecord；
-ResourceType；
-ResourceOwner；
-ResourceState；
-ResourceReference；
-ResourceReleasePlan。

### 3. ResourceOwnershipService

实现：

- Register；
-Add/Remove Reference；
-Transfer；
-PlanRelease；
-ExecuteRelease；
-ListOwned；
-ListReferences。

### 4. 统一资源持久化

包含：

- Owned Resources；
-References；
-Transfers；
-Release Plans；
-Cleanup Jobs；
-Orphan Reports。

### 5. 各系统接入报告

覆盖：

- Package；
-Module；
-Tool；
-Agent Skill；
-MCP；
-Workflow；
-UI；
-Hook；
-Schedule；
-Secret；
-Storage；
-Process；
-Connection。

### 6. 卸载预览接口

能够展示：

- 删除；
-保留；
-转移；
-阻塞；
-用户决策。

### 7. 所有权迁移报告

列出：

- 已迁移资源；
-不确定 Owner；
-共享资源；
-用户接管；
-孤儿资源；
-仍未注册新模型的创建入口。

### 8. Orphan Scanner

能够检测数据库、文件、Secret、Process、Connection 和引用孤儿。

### 9. 测试报告

覆盖：

- Owner；
-Reference；
-Release；
-Transfer；
-Uninstall；
-Upgrade；
-Rollback；
-Orphan；
-资源清理；
-用户资产保护。

---

## 四十五、验收标准

本步骤通过必须满足：

1. 每类扩展资源都有统一 ResourceType。
2. 每项资源都有唯一 Owner。
3. 所有权、运行管理、存储管理和引用关系已分离。
4. 共享资源有可重建的引用图。
5. 禁用不再等于删除。
6. Extension 和 Module 卸载通过统一 Release Plan。
7. 用户资产不会被扩展卸载直接删除。
8. 支持 Adopt、Clone 和 Detach。
9. MCP Server、Workflow、Agent Skill、Secret、Storage、Schedule、Process 和 Connection 已纳入。
10. 升级和回滚可恢复所有权与引用。
11. 临时资源有过期与清理策略。
12. Orphan Scanner 可识别主要孤儿资源。
13. 新资源只写统一所有权模型。
14. 旧资源只读迁移。
15. 前端可展示资源与卸载影响。
16. 故障注入和清理测试通过。
17. 后续第 13 步可以抽取通用包安全基础设施。

---

## 四十六、退出条件

只有满足以下条件后，才能进入第 13 步“抽取包安全基础设施”：

- Resource Ownership 模型已落地；
-Package、Module、Tool、Agent Skill、MCP、Workflow 已接入；
-Release Plan 已落地；
-用户接管已落地；
-共享资源引用图可用；
-Process、Connection、Schedule 和临时资源有清理责任；
-旧 Owned Resource 写入已停止；
-迁移统计可用；
-孤儿资源可检测；
-关键卸载与回滚测试通过。

---

## 四十七、执行约束

执行本步骤时必须遵守：

> 资源所有权系统负责回答“谁拥有、谁引用、谁运行、谁存储、谁清理”，不得继续依赖 Package、Plugin、MCP、Workflow 各自维护一套隐式所有权。

禁止出现：

- PackageService 直接硬编码删除 Agent Skill 与 Workflow；
-MCP Dependency Service 自行决定共享 Server 删除；
-PluginManager 自行删除 Secret；
-Workflow 删除时级联删除用户业务资源；
-禁用扩展时删除持久数据；
-用户修改资源后仍由 Extension 强制覆盖；
-只有 ref_count 没有引用关系；
-先删数据库再停止进程；
-新旧 Owned Resource 长期双写；
-资源清理失败不记录审计。

本步骤完成后，Amitia 必须具备一套统一、可追踪、可转移、可预览、可回滚、可安全卸载的资源所有权基础。
