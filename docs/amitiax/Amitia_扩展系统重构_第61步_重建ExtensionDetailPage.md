# Amitia 扩展系统重构第 61 步实施文档

## 第 61 步：重建 Extension Detail Page

---

## 一、步骤目标

在第 60 步重建 Extension Center 后，实现统一 Extension Detail Page，完整展示和管理单个 Extension 的定义、模块、能力、状态、安全、数据、运行、版本和诊断。

本步骤目标是：

> 用一个以 Extension 为聚合根的详情页替代 Plugin Detail、Skill Detail、MCP Detail、Workflow Detail、Package Detail 等重复页面，同时保留各 Contribution 的专业视图和操作入口。

统一路由：

```text
/extensions/:extensionId
```

---

## 二、信息架构

建议 Tab：

```text
概览
模块
能力
权限
作用域
运行
数据与资源
界面
版本与更新
日志与审计
开发者
```

按 Extension 实际能力隐藏无内容 Tab，但 URL Contract 稳定。

---

## 三、概览

展示：

-Icon；
-名称；
-说明；
-Extension ID；
-Version；
-Publisher；
-Trust；
-来源；
-License；
-安装时间；
-Enabled；
-Effective State；
-故障；
-更新；
-平台；
-主要 Contribution；
-风险摘要。

---

## 四、主操作

根据状态显示：

```text
启用
禁用
更新
回滚
修复
卸载
导出
打开页面
打开开发者控制台
退出开发模式
```

所有写操作先 Plan。

---

## 五、状态解释

必须区分：

```text
用户设置：已启用
定义：有效
依赖：缺失 1 项
Runtime：启动失败
可执行：否
```

不能只显示一个总开关。

---

## 六、Module Tab

每个 Module 展示：

-Module ID；
-名称；
-Type；
-Version；
-Enabled；
-Effective State；
-Runtime；
-Contributions；
-Dependencies；
-平台；
-错误；
-资源。

Module 开关只修改 Module Enablement。

---

## 七、Contribution Tab

按类型分组：

```text
Tools
Agent Skills
Workflows
MCP Servers
Providers
Hooks
Events
Schedules
Background Tasks
UI
Desktop
Resources
```

每项展示：

-稳定 ID；
-名称；
-Enabled Override；
-来源；
-Registration；
-Effective；
-RuntimeBinding；
-依赖；
-权限；
-Scope；
-风险；
-最近运行；
-错误。

---

## 八、Tool 详情

Tool 子视图：

-Input/Output Schema；
-模型名称；
-模型可见；
-风险；
-SideEffect；
-幂等；
-并发；
-超时；
-Permission；
-Scope；
-Runtime；
-调用记录；
-测试入口，开发模式。

---

## 九、Agent Skill 详情

展示：

-摘要；
-SKILL.md；
-资源；
-Token 估算；
-激活策略；
-依赖 Tool/MCP；
-Scope；
-Enabled；
-最近选择；
-用户 Fork；
-来源。

不显示执行 Handler。

---

## 十、Workflow 详情

展示：

-Definition；
-节点图；
-输入输出；
-Tool Exposure；
-Schedule；
-Event Trigger；
-运行；
-等待；
-审批；
-补偿；
-依赖；
-用户 Fork。

---

## 十一、MCP 详情

展示：

-Transport；
-Command/URL 摘要；
-Enabled；
-Desired/Actual；
-Connection；
-Health；
-Circuit；
-Secret Reference；
-Scope；
-Permission；
-Tools；
-Resources；
-Prompts；
-依赖引用；
-连接/断开；
-运行记录。

不显示 Secret。

---

## 十二、Provider 详情

展示：

-Provider Interface；
-能力；
-配置；
-Secret；
-Health；
-默认选择；
-模型/服务列表；
-调用；
-成本字段预留；
-错误。

---

## 十三、Hook/Event 详情

展示：

-Hook Point/Event Type；
-Phase；
-优先级；
-Filter；
-Delivery；
-Retry；
-Dead Letter；
-Depth；
-失败策略；
-最近执行；
-Circuit。

---

## 十四、Schedule/Task 详情

展示：

-Recurrence；
-Timezone；
-Enabled；
-Scope；
-Permission；
-Overlap；
-Missed Run；
-Next Run；
-Last Run；
-Task Progress；
-Checkpoint；
-结果；
-错误。

---

## 十五、UI/Desktop 详情

展示：

-Slot；
-Page；
-Sandbox；
-Contract；
-排序；
-用户 Preference；
-冲突；
-性能；
-Session；
-菜单；
-快捷键；
-窗口；
-托盘；
-权限；
-平台。

---

## 十六、权限 Tab

展示：

-Requirement；
-原因；
-风险；
-约束；
-当前 Grant；
-Scope；
-来源；
-审批；
-过期；
-撤销；
-最近使用。

操作：

-Grant；
-拒绝；
-撤销；
-收窄；
-查看影响。

必须通过 Permission Broker。

---

## 十七、权限影响预览

撤销前展示：

-受影响 Tool；
-Workflow；
-MCP；
-UI；
-Task；
-Runtime；
-Schedule。

---

## 十八、作用域 Tab

展示：

-Global；
-Character；
-Conversation；
-Module；
-Contribution；
-来源；
-状态；
-孤儿；
-最近决定。

操作：

-绑定；
-解绑；
-恢复；
-收窄。

Scope 与 Enabled 分离。

---

## 十九、运行 Tab

展示：

-Runtime Definition；
-Instance；
-Desired/Actual；
-Health；
-Circuit；
-Quarantine；
-资源；
-队列；
-并发；
-内存；
-CPU；
-重启；
-启动日志；
-当前调用。

操作：

-重试；
-Repair；
-手动停止/启动仅作为 Lifecycle/Desired State；
-解除 Quarantine 需确认。

---

## 二十、数据与资源 Tab

### Storage

-Namespace；
-分类；
-使用量；
-配额；
-Schema；
-迁移；
-Snapshot。

### Secret

-元数据；
-用途；
-引用；
-轮换；
-撤销。

### Resource

-Owner；
-Reference；
-State；
-Retention；
-文件；
-Artifact；
-Cache；
-临时；
-泄漏。

---

## 二十一、清理操作

提供：

-清理 Cache；
-清理临时资源；
-生成 Release Dry Run；
-导出用户数据；
-修复孤儿；
-Storage Snapshot。

禁止“一键清空所有扩展数据”作为默认。

---

## 二十二、界面 Tab

展示 Extension UI：

-页面；
-Slot；
-Chat；
-Desktop；
-用户布局；
-隐藏；
-Pin；
-Renderer；
-快捷键；
-冲突；
-性能。

可跳转到布局管理。

---

## 二十三、版本与更新 Tab

展示：

-当前版本；
-历史版本；
-Definition Hash；
-Package Hash；
-Signature；
-Publisher Key；
-更新；
-Diff；
-数据 Migration；
-回滚点；
-自动更新设置；
-兼容；
-用户资产冲突。

---

## 二十四、更新操作

先生成 Update Plan，展示：

-Module；
-Contribution；
-Runtime；
-Permission；
-Scope；
-Dependency；
-Storage Migration；
-UI；
-用户资产；
-回滚。

---

## 二十五、回滚操作

展示：

-目标版本；
-回滚能力；
-数据影响；
-不可逆项；
-停机；
-用户资产；
-风险。

---

## 二十六、日志与审计 Tab

分为：

-运行日志；
-Invocation；
-生命周期；
-Permission；
-Scope；
-Event；
-Hook；
-Task；
-UI；
-资源；
-安全。

支持脱敏导出。

---

## 二十七、开发者 Tab

仅开发模式或高级用户显示：

-Manifest；
-Canonical Definition；
-Hash；
-Entry；
-SDK；
-Host API；
-RPC；
-Workspace；
-Revision；
-热重载；
-构建；
-Contract；
-Deprecated；
-迁移映射；
-打开 Developer Console。

---

## 二十八、迁移状态

旧系统迁移期间显示：

-旧 ID；
-迁移来源；
-状态；
-冲突；
-剩余旧入口；
-用户确认；
-Readiness。

迁移完成后可隐藏，但保留诊断入口。

---

## 二十九、系统 Extension 限制

系统 Extension：

-可能禁止卸载；
-可能禁止禁用关键 Module；
-仍显示权限和运行；
-允许重启/修复；
-保护操作必须解释原因。

---

## 三十、Synthetic Extension

用户 Agent Skill、Workflow、MCP：

-仍使用完整详情页；
-可提供简化首屏；
-允许导出；
-允许编辑/Fork；
-显示用户 Owner。

---

## 三十一、危险操作设计

卸载、删除数据、撤销高影响权限、清理 Secret 等：

-Plan；
-影响预览；
-明确选择；
-二次确认；
-审计；
-可恢复性说明。

避免仅用通用确认弹窗。

---

## 三十二、状态实时更新

详情页订阅：

-Enabled；
-Runtime；
-Health；
-Circuit；
-Invocation；
-Lifecycle；
-Permission；
-Scope；
-Task；
-UI。

使用 Generation 防旧事件覆盖。

---

## 三十三、后端 Read Model

建议：

```text
ExtensionDetailReadModel
```

按 Tab 分片：

```text
summary
modules
contributions
security
runtime
data
ui
versions
audit
developer
```

避免一次加载全部历史。

---

## 三十四、前端组件

建议：

```text
ExtensionDetailShell
ExtensionOverview
ExtensionStateMatrix
ModuleList
ContributionExplorer
PermissionPanel
ScopePanel
RuntimePanel
ResourcePanel
UIContributionPanel
VersionPanel
AuditPanel
DeveloperPanel
LifecyclePlanDrawer
```

---

## 三十五、旧详情页迁移

旧页面：

-Plugin；
-Skill；
-MCP；
-Workflow；
-Package。

改为：

```text
Extension Detail
→ 对应 Contribution 聚焦
```

兼容 Route 保留跳转，不保留独立写逻辑。

---

## 三十六、错误与空状态

必须处理：

-Extension 不存在；
-未安装；
-Definition 损坏；
-版本缺失；
-迁移中；
-已卸载；
-不兼容；
-Runtime 故障；
-无权限；
-无 Scope；
-Tab 无数据；
-后端离线。

---

## 三十七、性能

-Tab 懒加载；
-历史分页；
-大 JSON 折叠；
-节点图按需；
-日志虚拟化；
-实时事件过滤；
-Definition 缓存；
-资源树分层；
-避免每个 Contribution 单独请求。

---

## 三十八、无障碍

-Tab 键盘；
-状态文本；
-风险不只颜色；
-表格；
-Dialog 焦点；
-代码块；
-图表文本替代；
-缩放；
-高对比。

---

## 三十九、安全

不得显示：

-Secret；
-私钥；
-Token；
-完整敏感路径；
-完整消息/记忆；
-系统 Prompt；
-Host Session。

开发者原始 Payload 仍需权限和脱敏。

---

## 四十、测试要求

覆盖：

-概览；
-Module；
-全部 Contribution 类型；
-Permission；
-Scope；
-Runtime；
-Storage；
-Secret 元数据；
-Resource；
-UI/Desktop；
-Version；
-Update；
-Rollback；
-Audit；
-Developer；
-Migration；
-System；
-Synthetic；
-实时状态；
-旧 Route；
-危险操作；
-无障碍；
-性能；
-安全脱敏。

---

## 四十一、实施任务

1. 定义 ExtensionDetailReadModel。
2.建立 Tab API。
3.实现 Detail Shell。
4.实现概览和状态矩阵。
5.实现 Module/Contribution Explorer。
6.实现 Tool/Agent Skill/Workflow/MCP 专业视图。
7.实现 Permission/Scope。
8.实现 Runtime。
9.实现 Data/Resource。
10.实现 UI/Desktop。
11.实现 Version/Update/Rollback。
12.实现 Audit。
13.实现 Developer/Migration。
14.接入 Lifecycle Plan Drawer。
15.接入实时状态。
16.迁移旧详情页和 Route。
17.完成性能、无障碍和安全测试。

---

## 四十二、验收标准

1. 单个 Extension 只有一个正式详情页。
2.所有 Contribution 可在详情页查看。
3.状态矩阵可解释。
4.Module/Contribution 开关语义正确。
5.Permission、Scope、Runtime 分离。
6.Secret 不显示。
7.更新/回滚使用 Plan。
8.系统与 Synthetic Extension 可正确展示。
9.旧详情页只保留跳转。
10.实时状态使用 Generation。
11.大数据性能通过。
12.第 56—61 步开发者生态与扩展中心阶段完成，可进入第 62 步等价性与稳定性验收阶段。

---

## 四十三、执行约束

> Extension Detail Page 是 Extension 聚合状态的统一可视化和操作入口，不得在不同 Tab 内重新实现独立 Tool、MCP、Workflow、Plugin 生命周期。

禁止：

-Tab 直接写数据库；
-直接启动 Runtime；
-直接连接 MCP；
-直接删除资源；
-显示 Secret；
-独立旧 API 写入；
-状态混为一个 Enabled；
-旧详情页继续管理；
-跳过 Lifecycle Plan。
