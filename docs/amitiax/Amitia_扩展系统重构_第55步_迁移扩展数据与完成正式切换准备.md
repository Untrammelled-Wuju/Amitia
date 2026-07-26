# Amitia 扩展系统重构第 55 步实施文档

## 第 55 步：迁移扩展数据并完成正式切换准备

---

## 一、步骤目标

在第 49—54 步已经完成 Tool、Agent Skill、MCP、Workflow、官方 Plugin 和旧 `.amitiax` 定义迁移后，迁移扩展系统相关的全部持久数据、运行历史、状态、资源引用、用户偏好、Secret、Scope、Permission、Schedule 和前端配置，并完成新旧系统切换前的最终数据一致性准备。

本步骤目标是：

> 将旧扩展系统的业务数据从多套表、多套目录、多套状态和多套资源关系，迁移到 Extension Kernel 的唯一数据模型；完成全量快照、转换、校验、对账、增量停写窗口、回滚点和切换验收，但尚不在本步骤删除旧系统。

本步骤是第 65 步“切换 Extension Kernel 为唯一入口”的数据前置条件。

---

## 二、迁移范围

必须覆盖：

### 定义和安装

-Extension；
-Package；
-Module；
-Contribution；
-Tool；
-Agent Skill；
-MCP；
-Workflow；
-Plugin；
-Provider；
-UI；
-Desktop；
-Schedule。

### 状态

-Installed；
-Enabled；
-Override；
-Desired Runtime；
-旧 Connection；
-Health；
-Circuit；
-Quarantine。

### Scope

-Global；
-Character；
-Conversation；
-Extension；
-Module；
-Tool；
-Agent Skill；
-MCP；
-Workflow。

### Permission

-Definition；
-Requirement；
-Grant；
-Approval；
-条件；
-撤销；
-历史。

### Storage 和 Secret

-Plugin KV；
-配置；
-Cache；
-用户数据；
-Token；
-API Key；
-OAuth；
-Headers；
-Environment。

### 资源

-Artifact；
-安装目录；
-临时目录；
-文件；
-快照；
-进程；
-连接；
-Schedule；
-UI；
-窗口；
-托盘；
-快捷键。

### 历史

-Tool Run；
-Plugin Run；
-MCP Operation；
-Workflow Run；
-Node Run；
-Event Delivery；
-Hook；
-Schedule；
-错误；
-审计；
-SideEffect。

### 前端偏好

-扩展开关；
-角色绑定；
-Tool 开关；
-页面设置；
-布局；
-快捷键；
-UI 排序；
-Provider 选择。

---

## 三、数据迁移原则

### 1. 新系统是目标真值

迁移完成后，所有新写入只进入新模型。

### 2. 迁移不扩大

不得扩大：

-Enabled；
-Scope；
-Permission；
-Owner；
-资源删除范围；
-自动连接；
-自动 Schedule。

### 3. 历史与当前状态分离

历史数据可以不完整，但当前业务状态必须一致。

### 4. Secret 最小暴露

Secret 迁移过程不得进入日志、报告或普通快照。

### 5. 用户资产优先保护

无法判断 Owner 时默认用户/迁移保留，不自动删除。

---

## 四、迁移架构

```text
Final Legacy Snapshot
→ Read-Only Gateway
→ Canonical Migration DTO
→ Domain Builders
→ Import Plan
→ Staging Database / Target Tables
→ Validation
→ Resource/Artifact Migration
→ Secret Migration
→ Reconciliation
→ Cutover Readiness Report
```

---

## 五、迁移批次

建议：

```text
Batch 1  Definitions
Batch 2  Installations
Batch 3  Modules/Contributions
Batch 4  Enablement
Batch 5  Scope
Batch 6  Permission
Batch 7  Storage
Batch 8  Secrets
Batch 9  Dependencies
Batch 10 Schedules
Batch 11 Resources/Ownership
Batch 12 Runtime State
Batch 13 Run History
Batch 14 Audit/SideEffect
Batch 15 UI Preferences
Batch 16 Legacy ID Mapping
```

---

## 六、最终 Snapshot

创建切换前最终 Snapshot：

-数据库；
-旧扩展目录；
-Artifact；
-配置；
-Secret Reference 元数据；
-用户偏好；
-旧版本信息；
-应用版本；
-Schema；
-Hash；
-时间。

Snapshot 必须：

-只读；
-可校验；
-可重放；
-受 Resource Ownership 管理；
-不能包含 Secret 明文导出。

---

## 七、停写窗口

由于不采用长期双写，正式最终迁移需要短停写窗口。

流程：

1.阻止旧扩展配置修改。
2.阻止旧安装/更新/卸载。
3.阻止旧 Enabled/Scope/Permission 修改。
4.停止旧 Scheduler/Event/Runtime 新任务。
5.等待或中断运行中任务。
6.创建最终 Snapshot。
7.执行增量迁移。
8.对账。
9.保持旧系统只读。

本步骤完成停写能力和演练，不一定立即正式切换。

---

## 八、Definition 数据

迁入：

-ExtensionDefinition Version；
-ModuleDefinition；
-ContributionDefinition；
-RuntimeDefinition；
-DependencyDefinition；
-Definition Hash；
-Source Artifact；
-Legacy Mapping。

必须与第 49—54 步迁移结果一致。

---

## 九、Installation 数据

迁入：

-当前版本；
-安装状态；
-Enabled；
-Generation；
-Module Enabled；
-Override；
-Rollback Point；
-安装时间；
-来源；
-Trust；
-迁移状态。

---

## 十、Enabled 状态

应用第 19 步规则。

每个对象只有一个真值。

旧冲突：

-生成 Conflict；
-默认安全状态；
-需要用户确认；
-不静默取 OR。

---

## 十一、Scope 数据

迁移步骤：

1.映射 Subject ID。
2.校验 Character。
3.校验 Conversation。
4.校验 Owner。
5.规范化 Scope Type。
6.写 Scope Binding。
7.计算 Generation。
8.生成孤儿报告。

不存在角色/会话的绑定不自动删除，进入 Orphan Review。

---

## 十二、Permission 数据

迁移：

-Permission ID 映射；
-Subject；
-Scope；
-Grant Mode；
-条件；
-审批；
-过期；
-撤销；
-来源。

旧笼统权限需要拆分时：

-不能自动拆成多个 Allow；
-默认 Unresolved/Deny；
-用户重新确认。

---

## 十三、Storage 数据

旧 Plugin/Extension 数据按分类迁入：

```text
configuration
state
cache
user_data
derived
temporary
```

步骤：

-Owner；
-Namespace；
-Scope；
-Key；
-Version；
-Schema；
-Hash；
-大小；
-分类；
-保留策略。

Cache 可选择不迁，改为重建，但必须记录。

---

## 十四、Secret 数据

Secret 迁移采用受控流：

```text
Read legacy secret
→ Write Secret Broker
→ Verify reference
→ Replace target config
→ Mark legacy field migrated
→ Securely clear legacy plaintext where supported
```

要求：

-单条处理；
-内存生命周期短；
-不写 Snapshot；
-不写报告值；
-失败时保留旧值但禁用相关功能；
-迁移后检查日志和前端。

---

## 十五、MCP OAuth

必须保持：

-Refresh Token；
-Access Token；
-Expiry；
-Client；
-Scopes；
-Server Owner；
-Reference。

若无法安全迁移，要求重新登录，不伪造有效状态。

---

## 十六、Schedule 数据

迁入：

-Definition；
-Enabled；
-Owner；
-Scope；
-Permission Reference；
-Recurrence；
-Timezone；
-Next Run；
-Missed Policy；
-Overlap；
-Last Run。

正式切换前 Scheduler 保持暂停，避免双触发。

---

## 十七、Runtime 状态

不迁移旧内存 Runtime。

只迁移：

-Desired State；
-Quarantine；
-Circuit 可选；
-Health 历史；
-手动 Disconnect；
-恢复策略；
-Cleanup Pending。

Actual Runtime 在新系统启动后重新确定。

---

## 十八、Connection 状态

MCP 旧 Connected 不迁为新 Ready。

迁移：

-Enabled；
-Desired connected/disconnected；
-Credential；
-手动断开；
-重连策略。

Actual Connection 初始为 stopped/disconnected。

---

## 十九、资源所有权

对每个资源建立：

-Resource ID；
-Type；
-Owner；
-State；
-Storage；
-References；
-Delete Policy；
-Runtime Manager；
-来源；
-Legacy Path。

无法判断 Owner：

```text
owner=migration
state=orphaned
```

---

## 二十、文件迁移

包括：

-Agent Skill 资源；
-Workflow；
-Plugin 数据；
-用户文件；
-Cache；
-导出；
-临时；
-快照。

规则：

-包内容进入 Artifact；
-用户数据进入 Data；
-Cache 进入 Cache 或不迁；
-临时不迁；
-Secret 不迁文件；
-路径安全；
-Hash；
-跨平台；
-原文件保留期。

---

## 二十一、运行历史

迁移历史时允许不完整，但必须：

-保留原 ID；
-映射 Trace/Operation/Invocation；
-标记 Legacy；
-状态映射；
-父子缺失明确；
-结果未知明确；
-不生成不存在的 SideEffect。

---

## 二十二、历史保留策略

可分：

```text
full
summary
recent_only
skip
```

建议：

-近期关键运行完整；
-旧低价值高频日志摘要；
-审计和高风险 SideEffect 保留；
-用户可选择是否迁移全部历史。

---

## 二十三、UI Preference

迁移：

-Extension 页面设置；
-Tool 显示；
-UI Slot 排序；
-隐藏；
-Pin；
-快捷键；
-Renderer 选择；
-Provider 选择。

旧 UI 位置无法映射时恢复默认，并记录 Warning。

---

## 二十四、Legacy ID Mapping

必须建立完整映射：

```text
legacy_package_id
legacy_plugin_id
legacy_skill_id
legacy_mcp_id
legacy_workflow_id
legacy_tool_id
legacy_run_id
legacy_resource_id
→ canonical_id
```

所有兼容 API 和历史跳转依赖该映射。

---

## 二十五、数据导入事务

每个 Batch：

```text
Plan
→ Stage
→ Validate
→ Commit
→ Postcondition
→ Journal
```

Batch 失败不应破坏已提交且独立的前置 Batch，但必须可恢复或回滚。

---

## 二十六、幂等

每条迁移记录：

```text
snapshot_id
migration_entity_id
payload_hash
target_id
status
```

重复执行：

-不重复插入；
-验证 Hash；
-不同 Hash 产生冲突；
-不覆盖用户已修改目标。

---

## 二十七、目标已修改

如果迁移演练后新系统目标数据已被用户修改：

-正式迁移必须检测；
-不可覆盖；
-生成 Merge/Manual；
-优先用户最新数据；
-必要时重新创建最终 Snapshot。

---

## 二十八、对账

完成后对账：

### 数量

-Definition；
-Installation；
-Contribution；
-Scope；
-Permission；
-Storage；
-Secret Reference；
-Schedule；
-Resource；
-History。

### 关系

-Owner；
-Dependency；
-Module；
-Tool Source；
-MCP Tool Parent；
-Workflow Tool；
-Agent Skill Dependency；
-用户资产。

### 状态

-Enabled；
-Desired；
-Manual Disconnect；
-Quarantine；
-Schedule Enabled。

---

## 二十九、Postcondition

必须满足：

1.所有当前有效对象有 Canonical ID。
2.所有 Contribution 有 Owner。
3.所有运行能力有 RuntimeBinding。
4.所有写能力有 Permission Requirement。
5.所有角色/会话能力有 Scope。
6.所有 Secret 只有 Reference。
7.所有 Schedule 有固定 Scope。
8.所有共享资源有 Reference。
9.旧表无新增写入。
10.新系统可完全重建 Registry。
11.新 Runtime 启动前不存在旧双运行。
12.回滚 Snapshot 完整。

---

## 三十、切换准备报告

必须输出：

```text
Cutover Readiness Report
```

包含：

-迁移完成率；
-阻塞对象；
-数据冲突；
-孤儿；
-未迁 Secret；
-旧写入入口；
-旧 Runtime；
-旧 Schedule；
-旧 Event；
-旧 Tool；
-用户确认项；
-回滚能力；
-性能；
-磁盘；
-预计停写步骤。

---

## 三十一、阻塞切换条件

以下任一存在，不允许进入第 65 步正式切换：

-旧消息发送 Tool 仍执行；
-旧 Schedule 仍触发；
-旧 MCP 自动重连；
-旧 Plugin Event 双订阅；
-旧 Enabled 仍可写；
-Secret 明文未处理；
-关键 Scope 扩大冲突；
-用户资产 Owner 不明且可能被删除；
-Registry 无法重建；
-回滚 Snapshot 损坏；
-运行中高风险结果未知未处理。

---

## 三十二、回滚策略

本步骤迁移失败时：

-新目标表可清理或恢复；
-旧系统仍保持只读/原状态；
-使用最终 Snapshot；
-不丢用户数据；
-不重新开启双写；
-恢复前必须清理新 Runtime 和 Schedule。

---

## 三十三、前端迁移状态

提供管理员/开发者页面：

-阶段；
-批次；
-进度；
-警告；
-冲突；
-Secret 状态；
-资源；
-历史；
-旧写入；
-对账；
-回滚；
-切换就绪。

普通用户只显示必要迁移提示。

---

## 三十四、性能

需要控制：

-大 Run 历史；
-文件 Hash；
-Secret 逐条；
-数据库索引；
-批量写；
-内存；
-磁盘；
-迁移时间；
-停写窗口。

先演练测量，再确定最终切换窗口。

---

## 三十五、安全

迁移工具本身必须：

-本地可信；
-最小权限；
-不开放远程；
-不记录 Secret；
-输入只读；
-目标写入有审计；
-支持取消；
-防路径攻击；
-防恶意旧数据；
-故障隔离。

---

## 三十六、测试要求

覆盖：

-全量数据；
-空系统；
-大量 Tool；
-大量 Agent Skill；
-大量 MCP；
-大量 Workflow；
-Plugin State；
-Secret；
-OAuth；
-Scope；
-Permission；
-Schedule；
-Owner；
-文件；
-用户资产；
-历史；
-UI Preference；
-重复执行；
-目标修改；
-Batch 崩溃；
-磁盘不足；
-停写；
-对账；
-回滚；
-跨平台。

---

## 三十七、实施任务

1. 建立最终迁移 Schema。
2.实现 Batch Planner。
3.实现 Definition/Installation Import。
4.实现 Enablement Import。
5.实现 Scope Import。
6.实现 Permission Import。
7.实现 Storage Import。
8.实现 Secret Migration。
9.实现 Schedule Import。
10.实现 Resource Ownership Import。
11.实现 Runtime Desired State Import。
12.实现 History Import。
13.实现 UI Preference Import。
14.完成 Legacy ID Mapping。
15.实现幂等 Journal。
16.实现停写模式。
17.实现全量对账。
18.实现 Readiness Report。
19.完成三次以上迁移演练。
20.完成回滚演练。
21.冻结全部旧写入。
22.输出正式切换前签字报告。

---

## 三十八、验收标准

1. 扩展定义和安装数据全部进入新模型。
2.Enabled 只有新真值。
3.Scope 和 Permission 完成迁移。
4.Storage 完成分类迁移。
5.Secret 不存在普通明文目标。
6.Schedule 暂停且可由新系统恢复。
7.Actual Runtime 不从旧状态伪造。
8.资源 Owner/Reference 完整。
9.历史数据可追溯。
10.UI Preference 可映射或安全回退。
11.Legacy ID Mapping 完整。
12.迁移幂等。
13.最终 Snapshot 可恢复。
14.旧写入全部冻结。
15.Cutover Readiness 无 P0 阻塞。
16.迁移和回滚演练通过。
17.第 49—55 步正式迁移阶段完成，可进入第 56 步 SDK 和开发工具阶段。

---

## 三十九、执行约束

> 本步骤的目标不是让新旧系统长期同步，而是在可回滚的短停写窗口内把所有扩展数据迁入唯一新模型，并证明新系统具备正式切换条件。

禁止：

-长期双写；
-Secret 进入普通 Snapshot；
-旧 Connected 迁 Ready；
-旧缓存作为真值；
-Owner 不明自动归扩展；
-Enabled 冲突取 OR；
-迁移覆盖用户新修改；
-未对账即切换；
-旧 Scheduler 未停即启动新 Scheduler。
