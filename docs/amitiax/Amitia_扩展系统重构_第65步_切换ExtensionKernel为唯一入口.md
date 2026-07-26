# Amitia 扩展系统重构第 65 步实施文档

## 第 65 步：切换 Extension Kernel 为唯一入口

---

## 一、步骤目标

在第 62—64 步完成等价性、桌面稳定性和安全验收后，正式将 Extension Kernel 切换为 Amitia 全部扩展能力的唯一生产入口。

本步骤目标：

> 关闭旧 Tool、Skill、MCP、Workflow、Plugin、Package、UI、Schedule 和状态系统的生产写入与执行入口，将所有前端、后端、Electron、模型 Prompt、启动恢复、API、CLI 和迁移兼容请求统一路由到 Extension Kernel。

这是不可逆架构切换点，但仍保留旧数据只读和回滚快照，不在本步骤立即删除旧代码和表。

---

## 二、切换前置条件

必须全部满足：

-第 62 步 P0 清零；
-第 63 步 P0 清零；
-第 64 步 P0 清零；
-第 55 步 Cutover Readiness 通过；
-旧写入冻结；
-旧 Scheduler 停止；
-旧 MCP 自动重连停止；
-旧 Plugin Init 停止；
-旧 Tool Handler 停止；
-新 Registry 可重建；
-回滚 Snapshot 完整；
-三平台启动关闭通过；
-正式迁移演练通过。

---

## 三、切换范围

包括：

### 后端

-Tool；
-Agent Skill；
-MCP；
-Workflow；
-Plugin；
-Package；
-Lifecycle；
-Schedule；
-Event；
-Hook；
-Permission；
-Scope；
-Storage；
-Secret；
-Resource；
-运行记录。

### 前端

-Extension Center；
-详情；
-旧 Skill 页面；
-旧 MCP 页面；
-旧 Workflow 页面；
-旧 Plugin 页面；
-旧 Package 导入；
-设置；
-聊天 UI；
-桌面 UI。

### Electron

-Backend 启动；
-Runtime；
-MCP；
-托盘；
-菜单；
-快捷键；
-窗口；
-退出。

### 模型

-Tool Projection；
-Agent Skill Context；
-Workflow Tool；
-MCP Tool；
-Prompt Trace。

---

## 四、切换模式

建议：

```text
maintenance cutover
```

步骤：

1.进入维护模式。
2.停止旧写入。
3.停止旧新任务。
4.等待运行中关键任务。
5.创建最终 Snapshot。
6.执行最终增量迁移。
7.对账。
8.关闭旧执行入口。
9.启动 Extension Kernel。
10.重建 Registry。
11.启动必要 Runtime。
12.恢复 Schedule。
13.运行冒烟测试。
14.退出维护模式。

---

## 五、Feature Flag

建议总开关：

```text
extension_kernel_primary=true
legacy_extension_execution=false
legacy_extension_write=false
legacy_extension_read_only=true
```

切换后应用启动必须验证这些值。

禁止按用户或随机流量长期双主。

---

## 六、旧入口策略

### 写入口

全部拒绝或转换为新 Command。

### 执行入口

全部转换为新 Invocation。

### 查询入口

可暂时从新 Read Model 返回，或读取旧历史。

### 管理入口

跳转 Extension Center。

---

## 七、API 路由切换

旧 API 分类：

```text
redirected
adapter_to_kernel
read_only_legacy
gone
```

每个 API 必须有迁移表。

禁止旧 API 继续写旧 Repository。

---

## 八、模型 Tool 切换

Prompt Builder 只能读取：

```text
ToolRegistry
→ EffectiveStateResolver
→ Model Projection
```

删除或禁用：

-旧 Skill Tool 拼接；
-Plugin Tool 拼接；
-MCP Tool 拼接；
-Workflow Tool 拼接；
-内置 Tool 手工列表。

---

## 九、Agent Skill 切换

Prompt Assembly 只能使用：

```text
AgentSkillCatalog
→ Selector
→ ContextProvider
```

旧 Prompt 注入链全部关闭。

---

## 十、MCP 切换

-新 Runtime Supervisor 管理连接；
-旧 Manager 不启动；
-旧连接全部停止；
-新 Connection Generation；
-Discovery 注册新 Registry；
-验证无重复 Tool。

---

## 十一、Workflow 切换

-旧 Executor 拒绝新运行；
-旧 Scheduler 停止；
-新 Scheduler 恢复；
-旧 Event Trigger 停止；
-运行中实例按迁移分类处理；
-新运行只写新 Operation。

---

## 十二、Plugin 切换

-旧 Plugin Init 不执行；
-官方 Plugin 由 Extension Kernel 启动；
-LegacyGoRuntime 仅由 Supervisor 调用；
-旧 PluginManager 只读诊断；
-旧 Event/Schedule/Tool 注册关闭。

---

## 十三、Package 切换

-所有 `.amitiax` 导入使用新 Package Security/Parser/Lifecycle；
-旧 PackageService 安装 API 禁止；
-旧包进入 Migration Preview；
-更新/卸载只走 Lifecycle。

---

## 十四、UI 切换

-Extension Center 成为唯一入口；
-旧页面只跳转；
-UI Slot 使用新 Registry；
-旧动态 Vue 注入关闭；
-旧 Plugin Schema UI 关闭；
-扩展页面使用 Page Host。

---

## 十五、Desktop 切换

-Electron Desktop Host 注册菜单/托盘/快捷键；
-旧 Plugin 直接 IPC 关闭；
-旧窗口入口关闭；
-退出时只调用新 Shutdown Coordinator。

---

## 十六、数据写入切换

写入只允许：

-新 Definition；
-Installation；
-Enablement；
-Scope；
-Permission；
-Storage；
-Secret；
-Resource；
-Operation；
-Registry State；
-Lifecycle Journal。

旧表设置数据库层保护：

-只读连接；
-触发器拒绝可选；
-Repository Panic/错误；
-指标。

---

## 十七、旧读兼容

历史查询可以：

-使用 Legacy Read Gateway；
-通过 ID Mapping；
-明确 Legacy；
-不触发旧业务逻辑；
-不修改。

---

## 十八、运行中任务

切换前分类：

```text
finish_before_cutover
cancel_and_restart
migrate_state
manual_resolution
```

高风险未知结果必须人工处理。

---

## 十九、Schedule 恢复

切换后：

-只由新 Scheduler；
-读取 Next Run；
-应用 Missed Policy；
-防 Stampede；
-使用 Idempotency；
-验证 Scope/Permission；
-先小批恢复。

---

## 二十、冒烟测试

切换后立即验证：

-启动；
-Extension Center；
-系统 Tool；
-消息发送；
-Agent Skill；
-MCP；
-Workflow；
-官方 Plugin；
-UI；
-桌面菜单；
-Permission；
-Scope；
-安装包；
-禁用/启用；
-日志；
-关闭。

---

## 二十一、监控窗口

切换后重点监控：

-旧入口调用；
-重复消息；
-MCP 进程；
-Schedule；
-Event；
-Runtime Crash；
-Permission Deny；
-Scope Deny；
-Registry；
-资源泄漏；
-启动；
-关闭；
-用户数据；
-性能。

---

## 二十二、回滚条件

切换后出现以下可触发回滚：

-核心消息链不可用；
-数据损坏；
-大规模扩展无法启动；
-跨角色数据问题；
-重复副作用；
-应用无法退出；
-关键 Secret 迁移错误；
-Registry 不可恢复；
-系统 Tool 大面积失败。

---

## 二十三、回滚方式

回滚目标不是长期恢复旧双主，而是：

```text
restore snapshot
→ disable new runtime
→ re-enable approved legacy read/execute baseline
→ maintenance mode
→ repair
```

若旧系统已知会造成重复副作用，不能直接全量重启。

回滚必须使用预定义脚本和顺序。

---

## 二十四、切换审计

记录：

-操作人；
-版本；
-Snapshot；
-数据 Hash；
-Feature Flag；
-停止旧系统；
-启动新系统；
-Registry；
-Runtime；
-Schedule；
-冒烟测试；
-错误；
-回滚决定。

---

## 二十五、旧入口统计

切换后任何旧入口调用都记录：

```text
legacy_entry_called
```

包含：

-调用方；
-API；
-堆栈摘要；
-时间；
-是否阻止；
-映射结果。

用于第 66—69 步删除。

---

## 二十六、用户提示

正式版本说明：

-扩展系统已升级；
-旧包已迁移；
-需要重新确认部分权限；
-部分扩展可能暂时禁用；
-用户数据保留；
-可在“需要处理”查看问题。

避免使用技术术语恐吓普通用户。

---

## 二十七、实施任务

1. 冻结发布分支。
2.确认前置验收。
3.生成最终 Cutover Plan。
4.进入维护模式。
5.创建最终 Snapshot。
6.执行最终增量迁移。
7.完成数据对账。
8.关闭旧写入。
9.关闭旧执行。
10.切换 API。
11.切换模型 Tool/Agent Skill。
12.切换 MCP/Workflow/Plugin。
13.切换 UI/Desktop。
14.启动 Extension Kernel。
15.重建 Registry。
16.恢复 Runtime/Schedule。
17.执行冒烟测试。
18.退出维护模式。
19.监控旧入口和核心指标。
20.输出切换报告。

---

## 二十八、验收标准

1.Extension Kernel 是唯一写入口。
2.Extension Kernel 是唯一执行入口。
3.旧系统仅允许只读历史。
4.模型 Tool 只来自新 Registry。
5.Agent Skill 只来自新 Catalog。
6.MCP 无旧连接。
7.Workflow 无旧调度。
8.Plugin 无旧 Init。
9.Package 无旧安装。
10.UI 无旧动态注入。
11.旧表无新写。
12.冒烟测试通过。
13.回滚点有效。
14.旧入口调用可追踪。
15.可进入第 66 步删除旧 Plugin Runtime。

---

## 二十九、执行约束

> 第 65 步是唯一主链切换，不允许以“灰度”为名长期保留新旧双执行、双调度、双连接或双写。

禁止：

-双主；
-生产消息影子执行；
-旧 Scheduler 备用运行；
-旧 MCP 保持连接；
-旧 Tool Handler 兜底直接执行；
-旧表继续写；
-未对账切换；
-无回滚计划切换。
