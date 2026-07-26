# Amitia 扩展系统重构第 70 步实施文档

## 第 70 步：执行 Extension Kernel 最终总验收

---

## 一、步骤目标

完成 Amitia 扩展系统 70 步重构的最终总验收，确认新 Extension Kernel 已成为稳定、唯一、安全、可开发、可扩展、可迁移、可恢复的扩展基础，且旧系统已按计划删除，不存在并行主链和关键遗留风险。

本步骤目标：

> 对领域模型、包系统、生命周期、依赖、贡献注册、Runtime、Host API、Permission、Scope、Storage、Secret、Event、Hook、UI、Desktop、迁移、开发者生态、扩展中心、数据模型、跨平台稳定性和旧系统删除进行最终签字验收，并形成可直接用于正式发布的 Release Readiness Report。

---

## 二、最终验收范围

覆盖全部 70 步成果：

```text
阶段一：冻结、审计和基础抽取
阶段二：Extension Kernel 核心
阶段三：.amitiax 与 Runtime
阶段四：UI Contribution
阶段五：正式迁移
阶段六：开发者生态和扩展中心
阶段七：验证、切换和旧系统删除
```

---

## 三、最终架构

必须确认主链为：

```text
. amitiax / System / Synthetic Extension
→ Package Security
→ Manifest Parser / Domain Builder
→ ExtensionDefinition
→ Lifecycle Manager
→ Dependency Resolver
→ Contribution Registry
→ Runtime Supervisor
→ Host API Gateway
→ Permission / Scope
→ Storage / Secret / Resource
→ Event / Hook / Scheduler
→ Tool / Agent Skill / Workflow / MCP / UI / Desktop
```

不得存在第二条生产主链。

---

## 四、领域模型验收

检查：

-Extension；
-Package；
-Installation；
-Module；
-Contribution；
-RuntimeDefinition；
-RuntimeInstance；
-Dependency；
-Owner；
-Artifact；
-版本；
-Hash；
-Generation；
-State Matrix。

所有不变量测试必须通过。

---

## 五、唯一入口验收

确认：

-安装唯一；
-启用唯一；
-禁用唯一；
-更新唯一；
-回滚唯一；
-卸载唯一；
-Tool 执行唯一；
-Agent Skill 注入唯一；
-MCP 连接唯一；
-Workflow 执行唯一；
-Event/Hook 唯一；
-UI Slot 唯一；
-Desktop Host 唯一；
-数据写入唯一。

---

## 六、包系统验收

确认：

-`.amitiax` 唯一后缀；
-Manifest v2；
-多 Module；
-严格 Parser；
-包安全；
-签名；
-Publisher；
-Trust；
-原子安装；
-更新；
-回滚；
-数据迁移；
-旧包迁移；
-旧 Parser 生产删除。

---

## 七、Runtime 验收

确认：

-JavaScript Main Runtime；
-Task Runtime；
-MCP Runtime；
-Workflow Runtime；
-WASM Runtime；
-Trusted Service Runtime；
-LegacyGoRuntime 仅官方迁移；
-Desired/Actual；
-Health；
-Circuit；
-Quarantine；
-资源限制；
-取消；
-停止；
-进程清理；
-Generation。

---

## 八、Host API 验收

确认：

-唯一 Runtime 边界；
-身份；
-Session；
-版本；
-Schema；
-Permission；
-Scope；
-Deadline；
-Rate；
-Depth；
-Result；
-Resource Handle；
-审计；
-无万能入口；
-无直接 Service。

---

## 九、Permission 验收

确认：

-Requirement 与 Grant 分离；
-默认拒绝；
-Allow/Deny/Approval；
-条件；
-Scope；
-后台；
-撤销；
-过期；
-高风险；
-桌面；
-Service；
-MCP；
-UI；
-官方扩展也受控制。

---

## 十、Scope 验收

确认：

-Global；
-Character；
-Conversation；
-Extension；
-Module；
-Invocation；
-Session；
-子调用只收窄；
-Schedule 固定；
-UI Session；
-角色切换；
-无跨角色数据。

---

## 十一、Storage/Secret 验收

确认：

-Namespace；
-CAS；
-配额；
-分类；
-Schema；
-迁移；
-Snapshot；
-用户数据；
-Secret Broker；
-Reference；
-Lease；
-用途；
-轮换；
-撤销；
-无明文日志。

---

## 十二、Tool/Agent Skill 验收

### Tool

-Schema；
-风险；
-SideEffect；
-幂等；
-执行安全；
-模型 Projection；
-内置/第三方一致。

### Agent Skill

-声明式；
-渐进加载；
-Token Budget；
-Scope；
-依赖；
-唯一 Prompt 注入；
-不执行代码。

---

## 十三、MCP 验收

确认：

-Transport；
-OAuth；
-Secret；
-Discovery；
-动态 Contribution；
-Tool；
-Resources；
-Prompts；
-Tasks；
-Host Callback；
-重连；
-手动断开；
-共享；
-无双连接；
-无重复 Tool。

---

## 十四、Workflow 验收

确认：

-声明式节点；
-Tool；
-Condition；
-Parallel；
-Delay；
-Approval；
-Sub Workflow；
-Compensation；
-Schedule；
-Event；
-恢复；
-未知结果；
-Tool Exposure；
-无旧 Skill Wrapper。

---

## 十五、Event/Hook 验收

确认：

-Event Schema；
-Subscription；
-Delivery；
-幂等；
-重试；
-Dead Letter；
-Depth；
-Hook Phase；
-排序；
-修改白名单；
-失败策略；
-无旧 Dispatcher。

---

## 十六、UI 验收

确认：

-UI Contribution；
-Schema UI；
-Restricted Web UI；
-Extension Slots；
-Page Host；
-Chat UI；
-Desktop；
-冲突排序；
-主题；
-Locale；
-无障碍；
-性能；
-沙箱；
-卸载清理；
-无 Vue 动态注入。

---

## 十七、桌面验收

确认：

-Electron Main 无第三方代码；
-菜单；
-托盘；
-快捷键；
-窗口；
-通知；
-IPC 固定；
-进程管理；
-退出；
-休眠；
-跨平台；
-更新；
-资源无残留。

---

## 十八、生命周期验收

确认每个操作：

-Plan；
-Snapshot；
-Confirmation；
-Lock；
-Step；
-Journal；
-Retry；
-Compensation；
-Recovery；
-Postcondition；
-Audit；
-Outbox。

---

## 十九、Dependency 验收

确认：

-SemVer；
-Required/Optional；
-循环；
-冲突；
-共享；
-Provider；
-平台；
-Host Feature；
-Snapshot；
-依赖丢失；
-不自动安装。

---

## 二十、Resource Ownership 验收

确认所有：

-Artifact；
-Storage；
-Secret；
-Runtime；
-Process；
-Connection；
-Schedule；
-UI；
-Window；
-Shortcut；
-Cache；
-Temp；
-用户资产；

有 Owner 和 Release Policy。

---

## 二十一、迁移验收

确认：

-Tool；
-Agent Skill；
-MCP；
-Workflow；
-Plugin；
-旧包；
-数据；
-ID Mapping；
-用户资产；
-Secret；
-历史；
-UI Preference；
-对账；
-Snapshot；
-旧写入冻结。

---

## 二十二、开发者生态验收

确认：

-TypeScript SDK；
-CLI；
-Manifest 类型；
-Testing SDK；
-Contract Test；
-开发模式；
-热重载；
-Developer Console；
-示例；
-文档；
-版本矩阵；
-签名；
-发布检查。

---

## 二十三、扩展中心验收

确认：

-唯一中心；
-已安装；
-导入；
-更新；
-需要处理；
-开发中；
-Extension 卡片；
-详情页；
-权限；
-Scope；
-Runtime；
-版本；
-日志；
-旧 Route 跳转。

---

## 二十四、旧系统删除验收

确认不存在生产可用：

-PluginManager；
-SkillManager；
-旧 MCP Manager 主链；
-旧 Workflow Executor/Scheduler；
-旧 PackageService；
-旧 Parser 主链；
-旧 Tool 拼接；
-旧 Prompt 注入；
-旧 Enabled 写入；
-旧 Event/Hook；
-旧 UI 注入；
-重复状态表。

---

## 二十五、代码依赖验收

生成依赖图，确认：

```text
new kernel
```

不依赖：

```text
legacy manager
legacy runtime
legacy repository
legacy parser
legacy UI store
```

Migration 工具必须独立隔离。

---

## 二十六、数据库验收

确认：

-Schema Version；
-唯一真值；
-无旧业务表；
-历史归档；
-Integrity Check；
-外键；
-索引；
-Generation；
-Owner；
-Secret；
-备份；
-恢复；
-旧应用版本阻止。

---

## 二十七、性能验收

必须满足项目设定的正式阈值。

至少记录：

-启动；
-Ready；
-扩展中心；
-详情；
-Tool；
-MCP；
-Workflow；
-Prompt；
-UI；
-安装；
-更新；
-关闭；
-内存；
-CPU；
-磁盘；
-100/1000 扩展规模。

---

## 二十八、稳定性验收

至少包含：

-三平台；
-24/72 小时；
-崩溃；
-休眠；
-网络；
-磁盘；
-更新；
-反复启停；
-资源泄漏；
-进程残留；
-大量扩展；
-Renderer 重载。

---

## 二十九、安全验收

确认第 64 步：

-P0 清零；
-P1 清零或正式接受；
-威胁模型；
-Fuzz；
-恶意扩展；
-Secret 扫描；
-依赖漏洞；
-UI 沙箱；
-Service 风险；
-开发模式；
-诊断包。

---

## 三十、发布阻塞条件

以下任一存在禁止发布：

-任何 P0；
-未决定 P1；
-旧系统仍可执行；
-双 Schedule/MCP/Event/Tool；
-跨角色数据；
-Secret 明文；
-卸载误删用户数据；
-更新无法恢复；
-应用无法正常退出；
-数据库回滚不可用；
-三平台核心链路未通过；
-Registry 无法重建；
-关键扩展缺失。

---

## 三十一、正式产物

必须生成：

```text
Extension Kernel Architecture
Extension Developer Guide
Manifest v2 Specification
SDK Documentation
CLI Documentation
Security Model
Permission Catalog
Host API Reference
UI Contribution Reference
Migration Guide
Operations Guide
Troubleshooting Guide
Release Readiness Report
Known Limitations
```

---

## 三十二、版本发布策略

建议新扩展系统发布为明确架构版本：

```text
Extension Kernel v1
Manifest v2
Host API v1
Runtime RPC v1
Schema UI v1
UI Contract v1
SDK v1
```

每个协议单独版本化。

---

## 三十三、已知限制

必须真实列出：

-Trusted Service 隔离能力；
-平台差异；
-未支持的 UI Slot；
-未支持 Runtime；
-旧包迁移限制；
-开发模式风险；
-性能规模；
-在线市场未完成部分；
-移动端未支持。

不得隐藏。

---

## 三十四、后续治理

建立持续规则：

-领域模型变更 ADR；
-Manifest 版本策略；
-Host API 审核；
-Permission 新增审核；
-UI Slot 审核；
-Runtime 安全评审；
-SDK 兼容；
-依赖漏洞；
-扩展发布；
-迁移弃用；
-旧接口删除周期。

---

## 三十五、最终验收角色

建议至少由以下角色签字：

-Extension Kernel 后端；
-Electron；
-前端；
-安全；
-测试；
-数据迁移；
-产品；
-发布负责人。

独立开发时仍需按角色分项自验，不能一项“整体通过”代替。

---

## 三十六、验收矩阵

建议文件：

```text
docs/extension-kernel/final-acceptance-matrix.md
```

字段：

```text
Area
Requirement
Evidence
Status
Owner
Risk
Blocking
Notes
```

---

## 三十七、最终发布演练

发布前执行完整演练：

1.旧版本数据准备。
2.升级应用。
3.迁移。
4.切换。
5.启动。
6.核心扩展运行。
7.安装新包。
8.更新。
9.回滚 Extension。
10.禁用/启用。
11.卸载。
12.应用更新回滚。
13.数据库恢复。
14.诊断包。
15.关闭。

---

## 三十八、实施任务

1.汇总 1—69 步产物。
2.建立最终验收矩阵。
3.执行领域和唯一入口验收。
4.执行包和 Runtime 验收。
5.执行 Permission/Scope/Storage/Secret 验收。
6.执行 Tool/Agent Skill/MCP/Workflow 验收。
7.执行 Event/Hook/UI/Desktop 验收。
8.执行迁移和旧系统删除验收。
9.执行 SDK/CLI/开发者生态验收。
10.执行扩展中心验收。
11.执行数据库验收。
12.执行性能/稳定性/安全验收。
13.执行发布演练。
14.关闭所有阻塞项。
15.输出 Release Readiness Report。
16.锁定 Extension Kernel v1 协议。
17.正式发布。

---

## 三十九、最终验收标准

1.Extension Kernel 是唯一生产主链。
2.旧系统执行、写入和重复状态已删除。
3.全部 70 步关键产物存在。
4.三平台核心功能通过。
5.Permission/Scope 无越权。
6.Secret 无泄漏。
7.包、签名和 Trust 可用。
8.所有 Runtime 可管理和清理。
9.Tool/Agent Skill/MCP/Workflow 行为正确。
10.UI/Desktop 安全稳定。
11.迁移和数据库恢复通过。
12.SDK/CLI/开发模式可用。
13.Extension Center/Detail 完成。
14.性能达到阈值。
15.稳定性达到阈值。
16.P0 清零。
17.P1 清零或正式接受。
18.已知限制公开。
19.发布演练通过。
20.Extension Kernel v1 可以正式作为 Amitia 扩展基础发布。

---

## 四十、执行约束

> 第 70 步只有在架构、功能、数据、安全、稳定性、跨平台、迁移和旧系统删除均有证据通过时才能签字，不能以“主流程能用”代替完整总验收。

禁止：

-带 P0 发布；
-旧系统备用运行；
-数据库无恢复演练；
-安全限制不公开；
-只测开发环境；
-只测 Windows；
-协议未版本化；
-无发布演练；
-用未来修复承诺替代当前阻塞项关闭。
