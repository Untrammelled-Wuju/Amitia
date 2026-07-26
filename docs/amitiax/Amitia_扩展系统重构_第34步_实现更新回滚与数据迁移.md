# Amitia 扩展系统重构第 34 步实施文档

## 第 34 步：实现更新、回滚与数据迁移

---

## 一、步骤目标

在唯一 Lifecycle Manager、不可变 Artifact、Definition Version、签名信任和 Storage Broker 基础上，实现完整 Extension 更新、回滚和数据迁移体系。

目标：

```text
New Package
→ Verify
→ Parse
→ Definition Diff
→ Dependency/Permission/Scope/Trust Diff
→ Data Migration Plan
→ Snapshot
→ New Generation Install
→ Runtime Switch
→ Verify
→ Retain Rollback Point
```

---

## 二、更新类型

```text
patch
minor
major
security
repair
development
```

类型仅辅助策略，最终以实际 Diff 为准。

---

## 三、Definition Diff

必须比较：

-Extension 元数据；
-Modules；
-Contributions；
-Runtimes；
-Dependencies；
-Permissions；
-Scope；
-Resources；
-Storage Schema；
-Secrets；
-Migrations；
-UI；
-平台；
-Publisher；
-签名；
-Trust。

---

## 四、Breaking Change

包括：

-删除 Contribution；
-更改稳定 ID；
-Input/Output Schema 破坏；
-Storage Schema 不兼容；
-删除 Runtime Entry；
-权限扩大；
-Scope 扩大；
-Provider 接口变化；
-Workflow 输出变化；
-Agent Skill 资源删除；
-MCP Tool 映射变化。

---

## 五、更新计划

输出：

-新旧版本；
-Diff；
-风险；
-新增权限；
-新增 Scope；
-依赖；
-数据迁移；
-用户资产；
-运行中影响；
-回滚能力；
-预计停机类型；
-确认项。

---

## 六、不可变版本

新版本安装到新目录。

禁止原地覆盖旧版本。

---

## 七、更新代际

```text
generation N active
generation N+1 preparing
generation N+1 validated
generation N+1 runtime ready
atomic contribution switch
generation N draining
generation N stopped
```

---

## 八、运行中 Invocation

运行中的 Invocation 固定旧：

-Definition Hash；
-Runtime Generation；
-Dependency Snapshot；
-Scope Snapshot；
-Permission Decision。

更新不得中途切换。

---

## 九、切换策略

```text
stop_then_start
start_then_switch
parallel_canary
manual
```

默认：

```text
stop_then_start
```

只有无共享副作用且 Runtime 支持时允许 start_then_switch。

---

## 十、数据迁移模型

```go
type DataMigrationDefinition struct {
    MigrationID   string
    FromRange     string
    ToRange       string
    RuntimeType   string
    Entry         string
    Reversible    bool
    Idempotent    bool
    Timeout       time.Duration
    AffectedNamespaces []string
}
```

---

## 十一、迁移执行

```text
Validate
→ Snapshot affected namespaces
→ Acquire storage lock
→ Run in Task Runtime
→ Validate output/schema
→ Commit
→ Release lock
```

---

## 十二、迁移权限

Migration 只能访问声明的 Storage Namespace。

禁止：

-网络；
-桌面；
-其他 Extension；
-Secret 明文；
-任意文件；
-Tool；
-用户聊天；

除非有极少数专门审核的宿主迁移类型。

---

## 十三、迁移幂等

每个 Migration 保存：

```text
migration_id
source_version
target_version
input_hash
output_hash
status
attempt
```

重复执行必须安全或被阻止。

---

## 十四、不可逆迁移

必须：

-高风险提示；
-完整备份；
-用户确认；
-回滚声明为受限；
-更新后保留更长 Snapshot；
-禁止自动静默更新。

---

## 十五、回滚点

包含：

-旧 Artifact；
-旧 Definition；
-旧 Installation；
-旧 Module；
-旧 Contribution；
-旧 Runtime Definitions；
-Storage Snapshot；
-Resource Graph；
-Permission Requirements；
-Scope 引用；
-Generation；
-签名记录。

---

## 十六、回滚能力等级

```text
full
code_only
data_snapshot_required
manual
not_supported
```

前端必须显示。

---

## 十七、回滚流程

1. 验证回滚点。
2. 停止新调用。
3. 暂停 Schedule。
4. Drain 当前 Generation。
5. 停止 Runtime。
6. 恢复旧 Artifact/Definition 引用。
7.执行逆向 Migration 或恢复 Snapshot。
8.恢复 Resource Graph。
9.保留用户资产。
10.启动旧 Runtime。
11.原子切换 Contribution。
12.验证。
13.审计。

---

## 十八、更新失败

失败阶段不同处理：

-验证前：无状态变化；
-新 Artifact 提交后：删除未引用视图；
-数据迁移前：可直接回旧；
-数据迁移后：恢复 Snapshot；
-切换后：停止新 Generation 并回切；
-未知副作用：recovery_required。

---

## 十九、Permission 变化

新增 Permission：

-不自动 Grant；
-可安装新版本但保持部分不可用；
-或阻止更新；
-原 Grant 不扩大。

删除 Permission：

-解除不再需要的 Requirement；
-是否撤销共享 Grant 由引用决定。

---

## 二十、Scope 变化

新版本不得扩大具体绑定。

收窄 Scope 可提示并应用。

扩大必须用户重新确认。

---

## 二十一、依赖更新

依赖版本变化必须检查所有 Dependents。

共享依赖升级可生成联合 Lifecycle Plan。

---

## 二十二、用户资产

更新不得覆盖：

-Fork Workflow；
-用户修改 Agent Skill；
-用户 MCP；
-用户 Storage；
-用户 Secret；
-用户 UI 设置。

需要 Merge 时生成冲突报告。

---

## 二十三、自动更新

自动更新只允许：

-Trust 合格；
-无权限扩大；
-无 Scope 扩大；
-无不可逆 Migration；
-无 Major Breaking；
-用户策略允许；
-可回滚。

自动更新仍使用完整 Lifecycle Plan，只是确认策略自动满足。

---

## 二十四、回滚保留策略

按：

-数量；
-时间；
-磁盘；
-安全版本；
-当前版本；
-用户固定；
-不可逆 Migration。

不得删除唯一可用回滚点而不提示。

---

## 二十五、前端

展示：

-版本 Diff；
-模块变化；
-Contribution 变化；
-权限；
-Scope；
-依赖；
-数据迁移；
-回滚等级；
-停机策略；
-用户资产；
-签名/Publisher；
-操作进度；
-恢复入口。

---

## 二十六、持久化

建议：

```text
extension_update_plans
extension_definition_diffs
extension_data_migrations
extension_migration_runs
extension_rollback_points
extension_generation_switches
extension_update_conflicts
```

---

## 二十七、测试要求

覆盖：

-无变化；
-Patch；
-Major；
-权限扩大；
-Scope 扩大；
-依赖变化；
-Module 增删；
-Runtime 切换；
-数据迁移；
-不可逆迁移；
-用户资产；
-更新每阶段崩溃；
-切换失败；
-回滚；
-回滚点损坏；
-自动更新；
-磁盘不足；
-并发调用；
-跨平台锁。

---

## 二十八、实施任务

1. 实现 Definition Diff。
2. 实现风险分类。
3. 实现 Update Planner。
4. 实现 Generation Prepare/Switch。
5. 实现 Data Migration Registry。
6. 实现受限 Migration Task。
7. 实现 Storage Snapshot。
8. 实现 Rollback Point。
9. 实现 Rollback Executor。
10. 实现失败恢复。
11. 接入 Permission/Scope/Dependency Diff。
12. 接入 Trust/Auto Update。
13. 实现用户资产冲突。
14. 改造前端。
15. 完成故障注入。

---

## 二十九、验收标准

1. 更新不原地覆盖。
2. Diff 覆盖全部领域对象。
3. 运行中 Invocation 不漂移。
4. 数据迁移受限且有 Snapshot。
5. 不可逆迁移需确认。
6. 新权限不自动 Grant。
7. Scope 不自动扩大。
8. 用户资产受保护。
9. Generation 可原子切换。
10. 回滚点完整。
11. 更新失败可恢复。
12. 自动更新有严格资格。
13. 包系统阶段完成，可进入 Runtime 选择与实现阶段。

---

## 三十、执行约束

> 更新是安装新版本并进行代际切换，回滚是恢复已验证快照；二者都不能通过覆盖目录和修改几张状态表完成。

禁止：

-原地覆盖；
-运行中换 Definition；
-自动 Grant；
-自动扩大 Scope；
-无 Snapshot 迁移；
-覆盖用户资产；
-删除唯一回滚点；
-更新失败后保持半激活。
