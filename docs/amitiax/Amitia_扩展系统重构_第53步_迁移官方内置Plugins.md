# Amitia 扩展系统重构第 53 步实施文档

## 第 53 步：迁移官方内置 Plugins

---

## 一、步骤目标

将 Amitia 当前编译期注册、由 PluginManager 管理的官方内置 Go Plugin，迁移到 Extension Kernel 的系统 Extension、Module、Contribution、LegacyGoRuntime、Host API、Storage Broker、Event Bus、Hook Pipeline 和统一生命周期体系。

本步骤目标是：

> 保留官方内置 Plugin 已经实现且具备业务价值的能力，但彻底取消其独立 Plugin 领域、独立状态、独立运行记录、直接 Service 访问和直接资源管理，使其成为 Extension Kernel 中的正式系统模块。

迁移完成后：

```text
旧 Go Plugin
→ system/amitia-official-extensions
→ ModuleDefinition
→ ContributionDefinition
→ LegacyGoRuntime
→ Host API / Broker / Event / Hook
```

`legacy_go` 仅作为过渡 Runtime，不再新增第三方或新官方插件。

---

## 二、迁移范围

需要盘点所有：

-编译期 Plugin Registry；
-PluginManager；
-Plugin Init；
-Tool Handler；
-Hook Handler；
-Event Handler；
-Schedule；
-Background Worker；
-State；
-Store；
-Circuit；
-Timer；
-窗口；
-托盘；
-菜单；
-Provider；
-渠道扩展；
-诊断扩展；
-资源；
-直接 Repository；
-直接 Service；
-直接数据库；
-直接 Electron IPC；
-旧 Run；
-旧 Enabled；
-旧 Scope；
-旧 Permission；
-卸载和关闭逻辑。

必须按插件逐个建立迁移档案，不允许只迁移 Plugin Tool。

---

## 三、目标 Extension 组织

建议根据职责拆分，而不是把全部官方插件塞进一个巨大 Module。

可以建立：

```text
system/amitia-official-extensions
```

其下按插件建立 Module：

```text
system/amitia-official-extensions#<plugin-id>
```

如果某个官方插件本身具有独立版本、权限、更新和卸载边界，也可建立独立系统 Extension：

```text
official.amitia/<extension-id>
```

判断原则：

-需要独立版本和更新：独立 Extension；
-仅 Amitia 核心内部模块：系统 Extension Module；
-可选但不可外部分发：官方系统 Extension；
-计划未来外置分发：独立 Extension。

---

## 四、迁移档案

每个旧 Plugin 必须输出：

```text
Legacy Plugin ID
Canonical Extension/Module ID
Version
Registration Entry
Tools
Hooks
Events
Schedules
Workers
Providers
UI
Desktop
Storage
Secrets
Permissions
Scopes
Dependencies
Resources
Side Effects
Runtime Assumptions
Direct Service Calls
Migration Risk
```

没有档案不得直接迁移。

---

## 五、Contribution 拆分

旧 Plugin 不能整体变成一个模糊 Contribution。

需要拆分：

```text
Plugin Tool
→ Tool Contribution

Plugin Hook
→ Hook Contribution

Plugin Event Handler
→ Event Subscription Contribution

Plugin Schedule
→ Schedule Contribution

Plugin Background Worker
→ Background Task Contribution

Plugin Provider
→ Provider Contribution

Plugin UI
→ UI Contribution

Plugin Menu/Tray/Shortcut
→ Desktop Contribution

Plugin State
→ Storage Namespace

Plugin Secret
→ Secret Reference
```

---

## 六、LegacyGoRuntime

每个需执行 Go Handler 的 Module 使用：

```text
RuntimeType: legacy_go
```

RuntimeDefinition 指向稳定内置 Runtime Entry ID，不使用反射函数名。

示例：

```text
official.weather/runtime/main
```

LegacyGoRuntimeFactory 内部映射编译期 Handler，但外部领域层只看到稳定 Entry。

---

## 七、直接 Service 访问迁移

旧 Plugin 可能直接持有：

-CharacterService；
-MemoryService；
-MessageService；
-ProviderService；
-FileRepository；
-数据库；
-Electron Bridge。

迁移顺序：

```text
直接 Service
→ Host API Route
→ Permission
→ Scope
→ Audit
```

如果暂时无法一次迁完，可使用内部 Host Adapter，但必须：

-仅 legacy_go；
-有调用清单；
-有删除计划；
-经过 Runtime Identity；
-经过 Scope/Permission；
-不能暴露 Repository 对象。

---

## 八、Tool 迁移

Plugin Tool 使用第 49 步规则：

-正式 Tool Contribution；
-稳定 ID；
-Schema；
-Permission；
-Scope；
-SideEffect；
-HostInternal/LegacyGo RuntimeBinding；
-统一模型暴露。

旧 PluginManager 不再注册 Tool。

---

## 九、Hook 迁移

旧 Hook：

-确认 Hook Point；
-映射 Phase；
-确定优先级；
-输入输出 Schema；
-允许修改字段；
-Timeout；
-Failure Policy；
-Depth；
-Circuit。

不符合 Hook Pipeline 安全模型的 Hook 必须重写，不能简单包装。

---

## 十、Event 迁移

旧 Event Handler：

-映射正式 Event Type；
-建立 Event Subscription Contribution；
-定义 Filter；
-Delivery Policy；
-幂等；
-Retry；
-Scope；
-Permission；
-Entry。

旧 Event Dispatcher 不再直接调用 Handler。

---

## 十一、Schedule 迁移

旧 Timer、Cron 和轮询：

-转换为 Schedule Contribution；
-固定 Scope；
-固定 Entry；
-Recurrence；
-Timezone；
-Overlap；
-Missed Run；
-Owner；
-Permission；
-取消；
-恢复。

禁止 Plugin 在 `Init()` 内创建 `time.Ticker` 作为长期调度。

---

## 十二、Background Worker 迁移

长期 Worker：

-若轻量且属于 Runtime：由 RuntimeSupervisor 管理；
-若可调度任务：Background Task；
-若长期原生服务：重新评估是否需要 Trusted Service；
-若只是轮询：Schedule + Task。

Worker 必须成为可追踪 Resource。

---

## 十三、State 迁移

旧 Plugin State：

-命名空间；
-Key；
-Version；
-类型；
-Scope；
-大小；
-用户数据；
-Cache；
-Secret。

迁移到 Storage Broker。

必须区分：

```text
configuration
state
cache
user_data
derived
temporary
secret
```

Secret 不迁普通 Storage。

---

## 十四、State CAS

旧状态若无版本：

-迁移时生成初始 Version；
-后续写入使用 CAS；
-并发写冲突显式；
-禁止继续直接 Repository Save。

---

## 十五、Secret 迁移

旧 API Key、Token、Header、环境变量：

-写 Secret Broker；
-保存 Reference；
-删除明文；
-检查日志；
-检查数据库历史字段；
-检查前端回显；
-迁移失败默认禁用相关 Contribution。

---

## 十六、Permission 迁移

官方 Plugin 也必须声明权限。

系统信任不等于无限权限。

建立：

```text
Contribution
→ Permission Requirements
→ Existing Grants / System Policy
```

系统内部权限可以由宿主策略预授权，但必须可审计，并且不能被第三方继承。

---

## 十七、Scope 迁移

旧 Plugin 常用全局单例和当前角色。

迁移时：

-删除前端当前角色依赖；
-调用时使用 ScopeSnapshot；
-Schedule 固定 Scope；
-角色状态进入 Character Namespace；
-会话状态进入 Conversation Namespace；
-Global 仅用于真实全局能力。

---

## 十八、Provider 迁移

旧 Plugin 提供模型、语音、图片、向量或渠道 Provider：

-Provider Contribution；
-接口版本；
-能力；
-Runtime Binding；
-配置 Schema；
-Secret；
-Health；
-依赖；
-用户选择。

不得直接写入全局 Provider Map。

---

## 十九、UI 迁移

旧 Plugin 管理页面、Schema 表单和前端特殊组件：

-简单表单 → Schema UI；
-复杂页面 → Restricted Web UI 或宿主重写；
-按钮 → Host-native Action；
-页面 → Extension Page Host；
-菜单/托盘 → Desktop Contribution。

禁止继续动态 import 内置 Plugin Vue 页面作为新架构标准。

---

## 二十、Desktop 迁移

旧菜单、托盘、快捷键、窗口：

-转换 Desktop Contribution；
-使用 DesktopExtensionHost；
-资源所有权；
-冲突规则；
-禁用/卸载清理；
-不直接 Electron IPC。

---

## 二十一、生命周期迁移

旧 Plugin：

```text
Init
Start
Enable
Disable
Stop
Destroy
```

映射：

-Extension/Module Lifecycle；
-Runtime Start/Stop；
-Contribution Activate/Deactivate；
-Resource Cleanup。

不得让 Plugin `Init()` 自行完成全部注册和启动。

---

## 二十二、Enabled 状态

旧：

-Plugin Enabled；
-Feature Enabled；
-Tool Enabled；
-Schedule Enabled；
-UI Enabled。

拆分并迁入：

-Extension/Module Enabled；
-Contribution Override；
-Schedule Enabled；
-Runtime Desired State。

---

## 二十三、Circuit 与 Health

旧 Plugin Circuit/Health 迁入统一：

-PluginRuntimeHealth；
-CircuitBreaker；
-EffectiveState；
-Runtime Supervisor。

旧字段停止新写。

---

## 二十四、运行记录

旧：

-Plugin Run；
-Hook Log；
-Event Delivery；
-Schedule Run；
-Worker Log。

迁入：

-Operation；
-Invocation；
-Attempt；
-Runtime Event；
-Audit；
-SideEffect。

---

## 二十五、资源所有权

登记：

-Storage；
-Secret；
-Worker；
-Timer；
-Schedule；
-Process；
-Connection；
-UI；
-Window；
-Tray；
-File Watcher；
-Cache；
-Artifact。

Module Disable 只释放 Runtime Resource。

Extension Uninstall 按 Release Plan。

---

## 二十六、迁移模式

建议每个 Plugin 分三阶段：

### 阶段 A：包装

-建立 Extension/Module/Contribution；
-使用 LegacyGoRuntime；
-旧 Handler 被新 Supervisor 调用。

### 阶段 B：边界迁移

-直接 Service → Host API；
-State → Broker；
-Event/Hook/Schedule → 新系统；
-UI/Desktop → Contribution。

### 阶段 C：旧系统切断

-旧 PluginManager 不注册；
-旧 Init 不执行；
-旧状态只读；
-兼容 API 映射新系统。

---

## 二十七、官方插件优先级

建议优先迁移：

1.无外部副作用的简单 Plugin；
2.纯 Tool Plugin；
3.状态型 Plugin；
4.Event/Hook Plugin；
5.Schedule/Worker Plugin；
6.Provider；
7.UI/Desktop；
8.高风险系统控制 Plugin。

---

## 二十八、双运行防护

对每个 Plugin 设置：

```text
migration_mode
legacy
wrapped
kernel_active
legacy_disabled
```

同一 Plugin 不允许：

-旧 Init 和新 Runtime 同时启动；
-旧 Event 和新 Event 同时订阅；
-旧 Schedule 和新 Schedule 同时触发；
-旧 Tool 和新 Tool 同时暴露；
-State 双写。

---

## 二十九、回滚

迁移回滚只允许回到：

```text
wrapped LegacyGoRuntime
```

不允许重新启用已证明存在重复副作用的旧完整 PluginManager 主链。

回滚需要：

-状态 Snapshot；
-Tool 映射；
-Event/Schedule 防重；
-资源清理；
-审计。

---

## 三十、前端

扩展详情展示官方插件：

-官方标识；
-系统 Extension；
-Module；
-Contributions；
-Runtime=legacy_go；
-权限；
-Scope；
-Storage；
-事件；
-Schedule；
-UI；
-运行健康；
-迁移状态。

---

## 三十一、测试要求

每个 Plugin 必须覆盖：

-启动；
-停止；
-启用；
-禁用；
-Tool；
-Hook；
-Event；
-Schedule；
-Worker；
-State；
-Secret；
-Permission；
-Scope；
-Provider；
-UI；
-Desktop；
-Runtime Crash；
-应用关闭；
-更新；
-资源清理；
-旧 API；
-双运行；
-行为等价；
-性能。

---

## 三十二、实施任务

1. 输出官方 Plugin 全量清单。
2. 为每个 Plugin 建立迁移档案。
3. 决定 Extension/Module 边界。
4. 建立 Contribution 拆分。
5. 包装 LegacyGoRuntime。
6.迁移 Tool。
7.迁移 Hook/Event。
8.迁移 Schedule/Worker。
9.迁移 State/Secret。
10.迁移 Permission/Scope。
11.迁移 Provider。
12.迁移 UI/Desktop。
13.迁移运行审计。
14.登记资源所有权。
15.关闭旧 Init/Registry。
16.实现兼容 API。
17.完成逐插件验收。
18.输出剩余 LegacyGo 清单和后续重写计划。

---

## 三十三、验收标准

1. 所有官方 Plugin 有 Extension/Module 定义。
2. Plugin 能力已拆为 Contribution。
3.所有执行由 LegacyGoRuntime/Supervisor 承载。
4.旧 PluginManager 不再注册新能力。
5.Tool、Hook、Event、Schedule 进入统一系统。
6.State/Secret 进入 Broker。
7.直接 Service 访问有清单并已迁移或受控。
8.UI/Desktop 使用正式 Contribution。
9.生命周期由 LifecycleManager 管理。
10.旧 Enabled/Run 停止新写。
11.不存在双 Tool、双 Event、双 Schedule。
12.关键插件回归通过。
13.可进入第 54 步旧 `.amitiax` 迁移。

---

## 三十四、执行约束

> 官方内置 Plugin 也必须服从 Extension Kernel。官方身份只能影响信任和系统策略，不能成为绕过 Host API、Permission、Scope、Runtime Safety 和资源所有权的理由。

禁止：

-官方 Plugin 直接数据库；
-旧 Init 自动启动；
-旧 Event 双订阅；
-旧 Schedule 双运行；
-直接 Electron IPC；
-State 双写；
-新增 legacy_go Plugin；
-用官方身份绕过权限。
