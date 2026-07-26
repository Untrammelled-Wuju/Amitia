# Amitia 扩展系统重构第 69 步实施文档

## 第 69 步：删除重复生命周期状态表与旧数据模型

---

## 一、步骤目标

在旧 Plugin Runtime、Skill 兼容层和旧 Package 生产链已经删除后，正式删除扩展系统重复的数据库表、状态字段、旧 Repository、旧索引、旧缓存和重复生命周期模型，完成数据层唯一真值收口。

本步骤目标：

> 让 ExtensionDefinition、ExtensionInstallation、InstalledModule、Contribution Override、Scope Binding、Permission Grant、Runtime Desired/Actual、Resource Ownership、Lifecycle Operation 和统一运行审计成为扩展系统唯一数据模型，彻底消除旧 Plugin/Skill/MCP/Workflow/Package 各自保存 Enabled、Run、State、Connection 和 Resource 的重复表。

---

## 二、删除前置条件

必须满足：

-第 55 步数据迁移和对账完成；
-第 65 步唯一入口切换完成；
-第 66—68 步旧代码删除完成；
-旧表无新写入；
-旧表读取仅历史/迁移；
-所有旧 ID 有 Mapping；
-历史查询已迁入新模型或归档；
-最终数据库 Snapshot；
-回滚策略；
-三平台数据库迁移测试。

---

## 三、删除分类

旧数据对象分为：

```text
drop
archive
transform_then_drop
retain_read_only
merge
```

不得所有旧表一律直接 Drop。

---

## 四、目标唯一真值

### Definition

```text
extension_definitions
extension_modules
extension_contributions
extension_runtime_definitions
extension_dependencies
```

### Installation/Enablement

```text
extension_installations
extension_installed_modules
extension_enablement_overrides
```

### Scope/Permission

```text
scope_bindings
permission_grants
permission_requirements
```

### Runtime

```text
runtime_desired_states
runtime_instances
runtime_health
runtime_circuits
```

### Resource/Data

```text
resource_ownership
resource_references
extension_storage_*
extension_secret_*
```

### Operation/Audit

```text
runtime_operations
runtime_invocations
runtime_attempts
audit_events
side_effects
lifecycle_*
```

实际表名以实现为准。

---

## 五、典型旧表

需要盘点：

```text
plugins
plugin_states
plugin_runs
plugin_enabled
plugin_settings
plugin_resources

skills
skill_runs
skill_enabled
skill_scopes
skill_resources

mcp_servers
mcp_tools
mcp_operations
mcp_enabled
mcp_connections
mcp_secrets

workflows
workflow_runs
workflow_node_runs
workflow_enabled
workflow_schedules

packages
package_versions
package_enabled
package_installations
package_resources
```

不得依赖示例名称，必须扫描数据库 Migration 和 ORM。

---

## 六、重复 Enabled 删除

旧 Enabled 字段全部映射到：

-ExtensionInstallation；
-InstalledModule；
-Contribution Override；
-Schedule Enabled。

删除前对账：

```text
old value
canonical value
source
conflict resolution
last updated
```

---

## 七、重复 Connection 状态删除

MCP 旧：

```text
connected
status
last_seen
session
```

Actual 连接只由 RuntimeInstance/Health 表示。

历史连接记录可归档。

---

## 八、重复 Run 表删除

旧 Run 迁为：

-Operation；
-Invocation；
-Attempt；
-Runtime Event；
-Audit；
-SideEffect。

旧完整 Payload 可归档到 Legacy History Store，避免污染新表。

---

## 九、重复 State 删除

Plugin/MCP/Workflow 独立 State：

-业务状态迁 Storage Broker；
-运行状态迁 Runtime；
-Enabled 迁 Enablement；
-Cache 丢弃或重建；
-Secret 迁 Broker。

---

## 十、重复 Scope 删除

旧 Character Binding、Global Flag、Conversation Binding：

-迁 Scope Binding；
-验证 Owner；
-删除旧表；
-孤儿进入归档报告。

---

## 十一、重复 Permission 删除

旧插件权限、MCP 权限、Tool Allowlist：

-迁 Requirement/Grant/Constraint；
-旧布尔字段不再读取；
-高风险模糊权限未确认则不迁 Allow。

---

## 十二、重复 Resource 删除

旧资源表：

-映射 Resource Ownership；
-共享引用；
-Owner；
-State；
-Release Policy。

无法映射的归档，不直接删除文件。

---

## 十三、历史归档

建议建立：

```text
legacy_extension_history_archive
```

或只读数据库文件。

存储：

-旧记录；
-原表；
-原 ID；
-时间；
-Hash；
-迁移 Mapping；
-不完整说明。

生产业务不查询，Developer Console 可按需打开。

---

## 十四、数据库迁移策略

使用多阶段 Migration：

### Phase 1

增加约束、停止旧写。

### Phase 2

最终增量复制与对账。

### Phase 3

重命名旧表为 `_legacy_archive`。

### Phase 4

观察版本周期。

### Phase 5

物理 Drop 或导出归档。

本步骤文档可执行到项目确定的安全阶段，但最终验收前需明确物理删除计划。

---

## 十五、列删除

除了整表，还需删除新表或公共表中的旧字段：

```text
plugin_enabled
skill_type
package_status
mcp_connected
workflow_skill_id
legacy_runtime
```

删除前确认无 ORM、SQL、前端 DTO 使用。

---

## 十六、索引和触发器

删除：

-旧唯一索引；
-旧外键；
-旧触发器；
-旧 View；
-旧 FTS；
-旧缓存表；
-旧同步 Job。

---

## 十七、Repository 删除

删除所有旧 Repository、DAO、Model 和 Query。

历史读取使用独立 Archive Gateway，不能继续使用业务 Repository。

---

## 十八、Migration 可逆性

物理 Drop 不可逆，因此：

-数据库 Snapshot；
-归档导出；
-校验 Hash；
-恢复脚本；
-恢复演练；
-版本标记；
-用户数据说明。

---

## 十九、SQLite 特性

Amitia 使用 SQLite，需要特别处理：

-表重建；
-外键；
-事务；
-WAL；
-磁盘空间；
-锁；
-VACUUM；
-迁移失败恢复；
-大表复制；
-应用关闭；
-版本兼容。

不能假设直接 Drop Column 在所有目标 SQLite 版本稳定可用。

---

## 二十、迁移期间磁盘

SQLite 表重建可能临时占用大量磁盘。

预检：

-数据库大小；
-归档；
-可用空间；
-WAL；
-临时文件；
-备份。

空间不足时禁止开始。

---

## 二十一、VACUUM

Drop 后是否执行 VACUUM：

-不能阻塞首次启动过久；
-可延迟后台维护；
-需要磁盘；
-有进度；
-可取消策略；
-关闭前安全。

---

## 二十二、应用版本兼容

删除旧表后，旧应用版本不能直接读取新数据库。

必须：

-禁止旧版本启动或提示；
-备份；
-更新器版本检查；
-回滚应用时同时回滚数据库；
-不能只回滚二进制。

---

## 二十三、只读历史

若用户需要查看旧运行记录：

```text
Archive Gateway
```

返回 Legacy 标记。

不允许对归档记录执行重试、恢复或修改。

---

## 二十四、前端字段清理

删除 DTO 中：

-旧 Enabled；
-旧 Status；
-旧 Plugin Type；
-旧 Skill Type；
-旧 Package ID；
-旧 Run ID。

保留 Legacy Mapping 展示字段需明确命名。

---

## 二十五、缓存失效

数据库 Schema 变更后清理：

-ORM Statement；
-Read Model；
-Extension Center；
-详情页；
-Registry；
-搜索索引；
-开发者控制台；
-旧前端 LocalStorage。

---

## 二十六、对账

Drop 前后：

-对象数量；
-Enabled；
-Scope；
-Permission；
-Storage；
-Secret Reference；
-Schedule；
-Resource；
-历史；
-用户偏好；
-数据库 Integrity Check。

---

## 二十七、数据库完整性

运行：

-`PRAGMA integrity_check`；
-外键检查；
-索引；
-重复 ID；
-孤儿；
-Hash；
-Generation；
-当前版本；
-Artifact。

---

## 二十八、测试要求

覆盖：

-空数据库；
-小数据库；
-大数据库；
-所有旧表；
-部分迁移；
-损坏旧记录；
-孤儿；
-冲突；
-空间不足；
-迁移中崩溃；
-WAL；
-备份恢复；
-应用更新；
-应用回滚；
-历史查询；
-前端；
-三平台；
-性能。

---

## 二十九、实施任务

1. 输出完整旧表/字段清单。
2.给每项标注 Drop/Archive/Merge。
3.确认零业务访问。
4.创建数据库和文件 Snapshot。
5.执行最终增量对账。
6.归档旧历史。
7.重命名旧表观察。
8.删除旧 Repository/Model。
9.删除旧字段/索引/触发器。
10.执行物理 Drop 或制定版本窗口。
11.清理缓存和 DTO。
12.执行 SQLite 完整性检查。
13.执行恢复演练。
14.执行三平台迁移测试。
15.输出数据模型删除报告。

---

## 三十、验收标准

1.扩展状态只有唯一真值。
2.旧 Enabled 表不再存在或仅归档。
3.旧 Run 表不再参与业务。
4.旧 State/Scope/Permission 已删除。
5.旧 Repository 不存在。
6.历史归档只读。
7.数据库完整性通过。
8.应用旧版本被正确阻止。
9.数据库恢复演练通过。
10.三平台迁移通过。
11.可进入第 70 步最终验收。

---

## 三十一、执行约束

> 删除重复数据模型必须在零业务访问、完整对账和可恢复 Snapshot 的基础上进行，不能为了“表更少”而丢失用户数据、历史审计或共享资源关系。

禁止：

-无 Snapshot Drop；
-Enabled 冲突未处理；
-Secret 归档明文；
-只回滚应用不回滚数据库；
-旧 Repository 继续查询归档表；
-空间不足强制迁移；
-未完整性检查；
-把归档重新作为业务真值。
