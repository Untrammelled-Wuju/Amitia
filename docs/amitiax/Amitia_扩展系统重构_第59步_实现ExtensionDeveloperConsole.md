# Amitia 扩展系统重构第 59 步实施文档

## 第 59 步：实现 Extension Developer Console

---

## 一、步骤目标

在 TypeScript SDK、Plugin CLI、开发模式和热重载完成后，实现 Amitia 内置 Extension Developer Console。

本步骤目标是：

> 为开发者提供统一、可视、可筛选、可追踪的扩展开发诊断界面，用于查看 Extension、Module、Contribution、Runtime、Host API、Tool、Event、Hook、Task、UI Session、Storage、Permission、Scope、资源、日志、性能、错误和生命周期操作，而不暴露宿主内部数据库、Secret 或不受控调试入口。

Developer Console 是开发和诊断工具，不是生产运行的业务控制面。

---

## 二、适用用户

面向：

- Amitia 核心开发者；
-Extension 开发者；
-测试人员；
-高级诊断用户。

普通用户默认不显示。

---

## 三、入口

建议：

```text
设置 → 开发者 → Extension Developer Console
```

或快捷入口：

```text
/diagnostics/extensions
```

仅开发模式或明确启用开发者选项时可见。

---

## 四、总体信息架构

建议分区：

```text
Overview
Extensions
Modules
Contributions
Runtimes
Invocations
Host API
Events
Hooks
Tasks
UI Sessions
Storage
Permissions
Scopes
Resources
Lifecycle
Logs
Performance
Migration
Compatibility
```

---

## 五、Overview

展示：

-已安装 Extension 数量；
-开发工作区；
-运行中 Runtime；
-故障；
-Quarantine；
-Circuit Open；
-活跃 Invocation；
-Event Queue；
-Dead Letter；
-Task；
-UI Session；
-资源泄漏；
-旧系统残留入口；
-迁移就绪状态。

---

## 六、Extension Inspector

按 Extension 查看：

-Definition；
-Version；
-Hash；
-Publisher；
-Trust；
-Installation；
-Enabled；
-Modules；
-Contributions；
-Runtimes；
-Dependencies；
-Permissions；
-Scopes；
-Resources；
-Storage；
-Secrets 元数据；
-UI；
-Lifecycle；
-Migration Mapping。

---

## 七、Module Inspector

展示：

-Module ID；
-Type；
-Enabled；
-Runtime；
-Contribution；
-依赖；
-资源；
-状态；
-Generation；
-故障；
-最近重载。

---

## 八、Contribution Inspector

展示：

-类型；
-稳定 ID；
-Definition Hash；
-Registration State；
-Effective State；
-不可用原因；
-RuntimeBinding；
-依赖；
-权限；
-Scope；
-Exposure；
-排序和冲突；
-最近调用；
-Owner。

---

## 九、Runtime Inspector

展示：

-Instance ID；
-Runtime Type；
-Definition；
-Generation；
-Desired/Actual；
-Health；
-Circuit；
-Quarantine；
-PID 或抽象进程信息；
-内存；
-CPU；
-队列；
-并发；
-Host API Session；
-重启；
-资源；
-启动日志；
-停止原因。

不得展示 Secret 环境变量。

---

## 十、Invocation Trace

建立 Trace Viewer：

```text
Operation
→ Invocation
→ Attempt
→ Runtime Call
→ Host API Call
→ Tool
→ Event
→ Hook
→ SideEffect
```

支持：

-按 Trace ID；
-时间；
-Extension；
-Tool；
-Workflow；
-错误；
-角色；
-会话；
-状态；
-风险；

查询。

敏感输入输出默认摘要化。

---

## 十一、Host API Inspector

展示：

-Method；
-Version；
-Runtime；
-调用次数；
-成功率；
-拒绝；
-Permission；
-Scope；
-耗时；
-Rate Limit；
-错误；
-Deprecated API。

不得提供“绕过权限重放”。

---

## 十二、Tool 调试

开发模式可手动调用 Extension Tool。

流程：

```text
选择 Tool
→ 生成 Schema Form
→ 选择测试 Scope
→ Permission Preview
→ Dry Run/Invoke
→ 查看 Trace
```

高风险 Tool 仍需审批。

不得使用调试器绕过 ExecutionSecurityKernel。

---

## 十三、Event Inspector

展示：

-Event Schema；
-发布；
-订阅；
-Delivery；
-重试；
-Dead Letter；
-Depth；
-Partition；
-过滤；
-Payload 摘要。

开发模式可发送测试 Event，但：

-标记 source=developer；
-符合 Schema；
-不能伪造系统安全事件；
-经过权限和 Scope。

---

## 十四、Hook Inspector

展示：

-Hook Point；
-Phase；
-排序；
-输入输出 Schema；
-修改字段；
-执行耗时；
-失败策略；
-Circuit；
-最近 Diff。

支持模拟测试，不允许直接修改生产输入。

---

## 十五、Task Inspector

展示：

-Task Definition；
-Operation；
-状态；
-Progress；
-Checkpoint；
-资源；
-结果 Artifact；
-取消；
-恢复；
-失败。

可在开发模式运行测试 Task。

---

## 十六、UI Inspector

展示：

-Slot；
-Page；
-Contribution；
-Sandbox；
-Contract；
-UI Session；
-Origin；
-Bridge；
-Action；
-Data Source；
-性能；
-错误；
-排序；
-冲突；
-Theme；
-Locale。

Restricted Web UI 可在开发模式打开独立 DevTools。

---

## 十七、Storage Inspector

展示：

-Namespace；
-数据分类；
-Key；
-Version；
-大小；
-Schema；
-Scope；
-使用量；
-配额；
-迁移；
-Snapshot。

默认不显示完整敏感 Value。

修改 Storage 需要：

-开发模式；
-确认；
-CAS；
-审计；
-禁止 Secret。

---

## 十八、Secret Inspector

只显示元数据：

-Secret ID；
-Owner；
-Purpose；
-Scope；
-状态；
-引用；
-轮换；
-撤销；
-最后使用。

绝不显示明文。

---

## 十九、Permission Inspector

展示：

-Requirement；
-Grant；
-来源；
-Scope；
-条件；
-过期；
-撤销；
-最近 Decision；
-拒绝原因；
-审批历史。

开发者可发起 Permission Request，但不能直接写 Grant 表。

---

## 二十、Scope Inspector

展示：

-Binding；
-Subject；
-角色；
-会话；
-来源；
-State；
-Generation；
-孤儿；
-最近决策。

支持 Scope 模拟评估，不直接扩大真实 Scope。

---

## 二十一、Resource Inspector

展示：

-Resource；
-Type；
-Owner；
-References；
-State；
-Release Policy；
-路径摘要；
-大小；
-生命周期；
-Leak；
-Cleanup。

允许生成 Release Dry Run。

禁止直接强删共享资源。

---

## 二十二、Lifecycle Inspector

展示：

-Command；
-Plan；
-Hash；
-Snapshot；
-Steps；
-Journal；
-Confirmation；
-Compensation；
-Recovery；
-冲突；
-结果。

可触发：

-重新 Plan；
-Recover；
-Cancel；
-Repair；

但仍通过 Lifecycle Manager。

---

## 二十三、日志

统一日志查询：

-Extension；
-Module；
-Runtime；
-Trace；
-Level；
-时间；
-Source；
-Message；
-字段。

支持：

-暂停；
-过滤；
-导出脱敏日志；
-最大保留；
-速率；
-Dropped Count。

---

## 二十四、性能

展示：

-Runtime 启动；
-Invocation 延迟；
-Host API；
-Tool；
-Event；
-Hook；
-Task；
-UI；
-内存；
-CPU；
-队列；
-缓存；
-Storage；
-资源；
-包加载。

---

## 二十五、性能时间线

建议：

```text
Timeline
Flame-like Trace Summary
Latency Histogram
Queue Depth
Memory Trend
Event Loop Lag
UI Render Duration
```

不必第一版实现完整 Profiler，但需提供可扩展指标模型。

---

## 二十六、兼容性 Inspector

展示：

-Manifest；
-SDK；
-Host API；
-RPC；
-Runtime；
-平台；
-架构；
-Deprecated；
-不兼容；
-迁移建议。

---

## 二十七、Migration Inspector

展示第 20、49—55 步数据：

-旧 ID；
-新 ID；
-Source；
-状态；
-冲突；
-旧写入；
-旧 Runtime；
-未迁数据；
-Cutover Readiness。

---

## 二十八、操作权限

Developer Console 自身属于高权限系统界面。

必须区分：

```text
read diagnostics
invoke test
modify development state
manage lifecycle
inspect storage
```

普通扩展不能调用 Developer Console 内部 API。

---

## 二十九、生产模式

即使用户开启 Console，生产模式仍限制：

-不显示 Secret；
-不允许任意 Storage 编辑；
-不允许伪造系统事件；
-不允许绕过 Permission；
-不允许直接 Kill 数据库状态；
-不允许加载未验证代码。

---

## 三十、前后端接口

建议建立只读聚合 API：

```text
GET /api/developer/extensions/overview
GET /api/developer/extensions/:id
GET /api/developer/runtimes
GET /api/developer/traces
GET /api/developer/events
GET /api/developer/hooks
GET /api/developer/tasks
GET /api/developer/ui-sessions
GET /api/developer/resources
```

写操作全部委托正式服务。

---

## 三十一、实时更新

使用统一 Event Bus 或内部诊断流：

-状态变化；
-日志；
-Trace；
-Runtime；
-Task；
-UI；
-Lifecycle。

需要：

-背压；
-过滤；
-断线重连；
-速率限制；
-页面不可见暂停。

---

## 三十二、导出诊断包

生成：

```text
diagnostic-bundle.zip
```

可包含：

-系统版本；
-Extension Summary；
-Definition Hash；
-状态；
-错误；
-脱敏日志；
-Trace 摘要；
-资源报告；
-迁移报告；
-平台信息。

不包含：

-Secret；
-完整聊天；
-完整记忆；
-私钥；
-Token；
-用户文件内容，除非明确选择。

---

## 三十三、测试要求

覆盖：

-Overview；
-Extension；
-Module；
-Contribution；
-Runtime；
-Trace；
-Host API；
-Tool 调试；
-Event；
-Hook；
-Task；
-UI；
-Storage；
-Secret 元数据；
-Permission；
-Scope；
-Resource；
-Lifecycle；
-日志；
-性能；
-实时流；
-诊断包；
-权限；
-生产限制；
-大量数据性能。

---

## 三十四、实施任务

1. 定义 Developer Console 信息架构。
2.建立诊断 Read Model。
3.实现 Overview。
4.实现 Extension/Module/Contribution Inspector。
5.实现 Runtime Inspector。
6.实现 Trace Viewer。
7.实现 Host API/Tool 调试。
8.实现 Event/Hook Inspector。
9.实现 Task/UI Inspector。
10.实现 Storage/Secret 元数据 Inspector。
11.实现 Permission/Scope/Resource Inspector。
12.实现 Lifecycle Inspector。
13.实现日志和性能。
14.实现实时诊断流。
15.实现诊断包导出。
16.接入开发模式。
17.建立权限和生产限制。
18.完成性能和安全测试。

---

## 三十五、验收标准

1. Extension Kernel 关键状态可统一诊断。
2. Trace 可跨 Tool/Runtime/Host API/Event。
3.调试调用不绕过安全内核。
4.Secret 不显示。
5.Storage 编辑受控。
6.生命周期操作仍通过正式 Manager。
7.UI/Runtime/Task 可查看。
8.迁移残留可查看。
9.实时流有背压。
10.诊断包脱敏。
11.普通扩展无法访问内部诊断 API。
12.可进入第 60 步扩展中心重建。

---

## 三十六、执行约束

> Developer Console 提供可观测性和受控调试，不是直接修改数据库、跳过授权、伪造系统事件或执行内部 Handler 的万能后台。

禁止：

-显示 Secret；
-直接写 Grant；
-直接改 Actual Runtime；
-直接删共享资源；
-绕过 Tool Executor；
-伪造安全事件；
-开放远程无认证访问；
-诊断包包含用户敏感全文。
