# Amitia 扩展系统重构第 66 步实施文档

## 第 66 步：删除旧 Plugin Runtime

---

## 一、步骤目标

在第 65 步已经完成 Extension Kernel 唯一入口切换，并确认旧 PluginManager、旧 Plugin Init、旧 Tool/Event/Schedule 注册入口不再承担生产业务后，正式删除旧 Plugin Runtime 体系。

本步骤目标：

> 删除旧 PluginManager、Plugin Registry、Plugin Runtime、Plugin State、Plugin Event Dispatcher、Plugin Schedule、Plugin Run、直接 Service 注入和旧前端 Plugin 执行入口，仅保留已迁移到 LegacyGoRuntime 的官方内置 Handler 实现，且这些 Handler 只能由新 Runtime Supervisor 调用。

本步骤删除的是“旧 Plugin 运行和管理体系”，不是删除所有官方插件功能。

---

## 二、删除前置条件

必须满足：

-第 53 步官方 Plugin 全部完成迁移分类；
-第 62 步 Plugin 等价性通过；
-第 63 步进程和资源稳定性通过；
-第 64 步安全验收通过；
-第 65 步旧 Plugin Init 已关闭；
-旧 Plugin Tool 无调用；
-旧 Event 无投递；
-旧 Schedule 无触发；
-旧 Plugin State 无写入；
-旧 API 已映射新系统；
-旧入口统计连续观察无有效生产调用；
-回滚快照存在。

---

## 三、删除对象清单

必须通过代码搜索和运行时统计确认，至少包括：

```text
PluginManager
PluginRegistry
PluginRuntime
PluginLoader
PluginInitializer
PluginEventDispatcher
PluginHookDispatcher
PluginScheduler
PluginWorkerManager
PluginStateRepository
PluginRunRepository
PluginEnabledRepository
PluginResourceTracker
PluginUIRegistry
PluginDesktopBridge
PluginServiceContainer
PluginContext
PluginHandlerRegistry
```

实际名称以项目为准。

---

## 四、保留对象

可以保留：

-迁移后的官方 Plugin 业务 Handler；
-`LegacyGoRuntimeFactory`；
-稳定 Entry 映射；
-新 Extension/Module/Contribution Definition；
-迁移 ID Mapping；
-旧历史只读 DTO；
-旧数据导出工具；
-审计记录。

保留 Handler 时必须从旧 Manager 解耦。

---

## 五、LegacyGoRuntime 边界

保留的 Go Handler 必须满足：

```text
Runtime Supervisor
→ LegacyGoRuntimeFactory
→ Stable Entry
→ Safety Guard
→ Host API / Broker
```

禁止：

```text
PluginManager.Start(plugin)
```

---

## 六、直接 Service 依赖清理

删除前搜索：

-构造函数注入；
-全局 Service Locator；
-Repository；
-数据库；
-Electron IPC；
-Event Bus；
-Scheduler；
-文件；
-Secret。

每个保留 Handler 必须：

-使用受控 Adapter；
-或有明确临时兼容接口；
-不能因删除 Manager 恢复直接访问。

---

## 七、旧 Init 清理

删除所有：

```text
init()
RegisterPlugin()
LoadPlugins()
StartAllPlugins()
AutoEnablePlugins()
RestorePluginRuntime()
```

或改为 Extension Kernel Bootstrap Participant。

禁止保留空壳但仍有副作用的 `init()`。

---

## 八、旧 Tool 注册清理

删除：

-Plugin Tool Registry；
-模型 Tool 拼接；
-旧 Tool Enabled；
-旧 Handler 映射执行；
-重复 Schema；
-重复 Tool 列表 API。

所有 Tool 已由 Contribution Registry 提供。

---

## 九、旧 Event/Hook 清理

删除：

-Plugin Event Bus；
-Plugin Hook Registry；
-直接 Handler 调用；
-旧优先级；
-旧 Depth；
-旧重试；
-旧错误记录。

全部由统一 Event Bus/Hook Pipeline 管理。

---

## 十、旧 Schedule/Worker 清理

删除：

-Plugin Timer；
-Cron；
-Goroutine Worker；
-启动时自动创建；
-旧 Schedule Table 写入；
-旧恢复逻辑。

必须验证无运行中旧 Timer。

---

## 十一、旧 State 清理

代码层删除：

-旧 State Repository 写入；
-旧 Namespace；
-直接 KV；
-直接数据库表访问；
-旧 CAS；
-旧 Secret 存储。

数据表是否删除留到第 69 步。

---

## 十二、旧资源清理

删除旧 Runtime 创建的：

-进程；
-连接；
-Timer；
-Watcher；
-Window；
-Tray；
-Shortcut；
-临时目录；
-Cache；
-锁；
-Event Subscription。

使用 Resource Ownership 扫描验证无 Owner=legacy_plugin 的 Active 资源。

---

## 十三、旧前端清理

删除：

-PluginManager 前端 Store；
-旧 Plugin 启停 API；
-旧 Plugin 页面业务写逻辑；
-动态 Vue Plugin 加载；
-旧 Schema UI Host；
-旧 Runtime 状态；
-旧菜单/托盘注入。

旧 Route 可暂时保留跳转。

---

## 十四、旧 API

分类：

### 删除

无兼容价值的内部 API。

### 返回 Gone

外部历史 API，返回迁移说明。

### Adapter

仍有调用的旧 API 映射新 Extension Kernel。

Adapter 不依赖旧 Manager。

---

## 十五、构建依赖

删除：

-旧 Plugin 包依赖；
-反射；
-动态加载；
-不再使用的库；
-旧测试夹具；
-旧前端类型；
-旧生成代码。

必须确认无第三方依赖因误删影响其他系统。

---

## 十六、数据库访问封禁

在旧 Repository 删除前，可先增加：

```text
panic/explicit error on write in test
metric on read
```

删除代码后确保没有 SQL 直接访问。

---

## 十七、测试迁移

旧 Plugin 测试：

-有业务价值的迁为 LegacyGoRuntime/Contribution 集成测试；
-验证旧 Manager 行为的测试删除；
-新增编译期禁止依赖测试；
-新增无旧入口扫描。

---

## 十八、静态检查

CI 增加禁止导入：

```text
internal/plugin/manager
internal/plugin/runtime
internal/plugin/registry
```

实际路径按项目调整。

禁止字符串和类型重新出现。

---

## 十九、运行时检查

应用启动时短期保留诊断：

-旧 Plugin 进程；
-旧 Timer；
-旧表写；
-旧 API；
-旧 Handler 直接调用。

发现即启动失败或进入安全模式，取决于严重度。

---

## 二十、删除顺序

推荐：

1.删除前端写入口。
2.删除旧 API 写 Handler。
3.删除旧启动/Init。
4.删除旧 Tool 注册。
5.删除旧 Event/Hook。
6.删除旧 Schedule/Worker。
7.删除旧 State 写入。
8.删除旧 Runtime/Manager。
9.删除旧资源追踪。
10.删除不再使用依赖。
11.迁移测试。
12.执行全量回归。

---

## 二十一、回滚

代码删除回滚必须基于版本控制。

不允许运行时 Feature Flag 重新启用旧 Plugin 主链。

如发现缺失功能：

-恢复具体业务 Handler；
-重新通过 LegacyGoRuntime 接入；
-不能恢复整个 PluginManager。

---

## 二十二、验收搜索

必须搜索：

```text
PluginManager
RegisterPlugin
StartAllPlugins
plugin_enabled
plugin_runs
plugin_state
legacy plugin event
legacy plugin schedule
```

并人工判断所有剩余引用。

---

## 二十三、测试要求

覆盖：

-所有官方 Plugin；
-应用启动；
-Tool；
-Event；
-Hook；
-Schedule；
-State；
-Secret；
-UI；
-Desktop；
-启停；
-更新；
-卸载；
-Runtime Crash；
-应用关闭；
-旧 API；
-无旧进程；
-无旧表写；
-无重复副作用；
-三平台。

---

## 二十四、实施任务

1. 输出旧 Plugin Runtime 删除清单。
2.确认零生产调用。
3.备份旧映射和历史读取。
4.删除前端旧写入口。
5.删除旧 API 执行入口。
6.删除 Plugin Init。
7.删除 Tool/Event/Hook/Schedule 注册。
8.删除 State/Resource 写入。
9.删除 PluginManager/Runtime。
10.保留并整理 LegacyGo Handler。
11.删除无用依赖。
12.增加 CI 禁止导入。
13.执行资源扫描。
14.执行全量回归。
15.输出删除报告。

---

## 二十五、验收标准

1.项目中不存在可运行的旧 PluginManager。
2.旧 Plugin Init 不存在。
3.旧 Tool/Event/Hook/Schedule 不存在。
4.官方功能由 LegacyGoRuntime 或新 Runtime 承载。
5.无旧 State 写入。
6.无旧 Plugin 资源。
7.前端不调用旧 Plugin API 写操作。
8.CI 阻止重新引入。
9.三平台测试通过。
10.可进入第 67 步删除旧 Skill 兼容层。

---

## 二十六、执行约束

> 删除旧 Plugin Runtime 时，只允许保留已经纳入新 Runtime Supervisor 的具体业务 Handler，不得保留可再次启动、注册、调度或写状态的旧 Plugin 框架。

禁止：

-保留备用 PluginManager；
-Feature Flag 可重启旧主链；
-保留旧 Scheduler；
-保留旧 Event Dispatcher；
-保留旧 State 双写；
-直接 Service 回潮；
-因功能缺失恢复整套旧系统。
