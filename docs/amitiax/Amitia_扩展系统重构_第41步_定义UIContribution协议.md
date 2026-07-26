# Amitia 扩展系统重构第 41 步实施文档

## 第 41 步：定义 UI Contribution 协议

---

## 一、步骤目标

在第 21—40 步已经完成 Extension Kernel 领域、生命周期、Contribution Registry、Runtime Supervisor、Host API Gateway、`.amitiax` v2 与多 Runtime 基础后，正式定义 Amitia 的 UI Contribution 协议。

本步骤目标是：

> 建立 Extension 向 Amitia 前端、Electron 桌面壳和特定业务页面贡献界面能力的唯一协议，使 UI 扩展具备稳定 ID、明确 Slot、版本化 Contract、沙箱类型、权限边界、Scope、生命周期、数据输入输出、事件通信、排序和故障隔离。

完成后，扩展不得再通过以下方式修改 UI：

- 直接修改 Vue Router；
- 直接 import 第三方扩展代码；
- 运行时动态注入 Vue Component；
- 直接访问 Pinia Store；
- 直接操作 DOM；
- 直接调用 Electron Main；
- 直接读取数据库；
- 修改宿主 CSS；
- 覆盖全局主题变量；
- 通过 PluginManager 向前端发送任意组件路径；
- 使用未经声明的 iframe 或 WebView；
- 通过前端本地配置绕过 Contribution Registry。

唯一链路：

```text
UIContributionDefinition
→ Contribution Registry
→ UI Contribution Adapter
→ UI Contract Validator
→ UI Host
→ Slot Resolver
→ Sandbox / Schema Renderer
→ Scoped Host Bridge
→ Render
```

---

## 二、核心设计原则

### 1. UI Contribution 是声明，不是前端源码注入

UI Contribution 描述：

- 贡献到哪里；
- 使用哪种渲染模式；
- 需要哪些数据；
- 可触发哪些动作；
- 需要哪些权限；
- 如何排序；
- 何时显示；
- 如何卸载。

它不允许第三方直接控制宿主前端。

---

### 2. UI Runtime 与 Main Runtime 分离

主插件 Runtime：

```text
JavaScript Main Runtime
```

负责 Tool、Event、Hook、Provider 等逻辑。

UI Runtime：

```text
Schema UI
Restricted Web UI
Host-native Declarative UI
```

负责可视界面。

禁止把主 Runtime 直接运行在 Renderer。

---

### 3. UI 只通过 Host Bridge 通信

扩展 UI 不能直接访问：

- Window 全局对象中的宿主内部接口；
-Pinia；
-Axios 全局实例；
-Electron IPC；
-数据库；
-文件系统；
-Host API Session；
-其他扩展 UI。

统一通信：

```text
UI Host Bridge
→ Runtime Identity
→ Permission / Scope
→ Host API Gateway / Extension Runtime
```

---

### 4. UI Contract 版本化

每个 Slot 和 UI Contribution 类型必须有独立 Contract Version。

例如：

```text
extension.settings.page/1
chat.message.action/1
desktop.tray.item/1
```

宿主升级时不能依赖隐式 DOM 结构。

---

## 三、UI Contribution 类型

建议统一类型：

```go
type UIContributionKind string

const (
    UIContributionSchemaPage      UIContributionKind = "schema_page"
    UIContributionWebPage         UIContributionKind = "web_page"
    UIContributionPanel           UIContributionKind = "panel"
    UIContributionCard            UIContributionKind = "card"
    UIContributionAction          UIContributionKind = "action"
    UIContributionMenuItem        UIContributionKind = "menu_item"
    UIContributionToolbarItem     UIContributionKind = "toolbar_item"
    UIContributionStatusItem      UIContributionKind = "status_item"
    UIContributionMessageAction   UIContributionKind = "message_action"
    UIContributionMessageRenderer UIContributionKind = "message_renderer"
    UIContributionComposerAction  UIContributionKind = "composer_action"
    UIContributionSettingsSection UIContributionKind = "settings_section"
    UIContributionDesktopCommand  UIContributionKind = "desktop_command"
)
```

---

## 四、UI Contribution Definition

建议：

```go
type UIContributionDefinition struct {
    ContributionID ContributionID
    ExtensionID    ExtensionID
    ModuleID       ModuleID

    Kind            UIContributionKind
    Slot            UISlotReference
    ContractVersion int
    Display         UIDisplayMetadata
    Entry           UIEntryDefinition

    Visibility      UIVisibilityRule
    DataContract    UIDataContract
    Actions         []UIActionDefinition
    Permissions     []PermissionRequirement
    ScopeRule       ScopeRule

    Ordering        UIOrderingRule
    ConflictPolicy  UIConflictPolicy
    Sandbox         UISandboxPolicy
    Lifecycle       UILifecyclePolicy
    Integrity       ContributionIntegrity
}
```

---

## 五、UI Slot Reference

建议：

```go
type UISlotReference struct {
    SlotID          string
    ContractVersion int
}
```

Slot ID 使用稳定命名：

```text
extension.settings.page
extension.detail.tab
chat.sidebar.panel
chat.header.action
chat.message.action
chat.message.renderer
chat.composer.action
desktop.tray.item
desktop.menu.item
desktop.command
system.status.item
```

---

## 六、Slot Contract

每个 Slot 必须定义：

```go
type UISlotContract struct {
    SlotID              string
    Version             int
    SupportedKinds      []UIContributionKind
    InputSchema         json.RawMessage
    OutputSchema        json.RawMessage
    AllowedActions      []string
    AllowedSandboxes    []UISandboxType
    Multiplicity        string
    OrderingPolicy      string
    FailurePolicy       string
    PerformanceBudget   UIPerformanceBudget
}
```

---

## 七、Slot Multiplicity

支持：

```text
single
multiple
ordered_multiple
replaceable_single
exclusive
```

### single

最多一个 Contribution。

### multiple

允许多个并列。

### ordered_multiple

允许多个并稳定排序。

### replaceable_single

宿主有默认实现，扩展可按策略替代。

### exclusive

仅系统或明确授权的一个扩展可占用。

---

## 八、渲染模式

建议三种正式模式：

### 1. Schema UI

由扩展提供声明式 JSON Schema/组件树，宿主使用原生 Vue 组件渲染。

适合：

-设置页；
-表单；
-状态卡片；
-简单列表；
-按钮；
-只读详情。

### 2. Restricted Web UI

扩展提供 HTML/CSS/JS Bundle，在严格隔离 WebView/iframe 中运行。

适合：

-复杂交互；
-图表；
-富媒体；
-复杂配置器。

### 3. Host-native Action

扩展只声明按钮、菜单、命令、图标、快捷键，由宿主原生渲染。

适合：

-工具栏；
-消息操作；
-托盘；
-菜单；
-状态项。

---

## 九、UI Entry Definition

建议：

```go
type UIEntryDefinition struct {
    Type        UISandboxType
    Path        string
    SchemaPath  string
    RuntimeID   RuntimeDefinitionID
    EntryName   string
    ContentHash string
}
```

要求：

-Path 包内；
-完整性绑定；
-类型与 Kind 匹配；
-不得引用远程页面；
-不得在 Manifest 中内嵌大段代码。

---

## 十、Sandbox 类型

建议：

```text
host_native
schema_renderer
web_restricted
web_isolated
```

### host_native

完全由宿主渲染。

### schema_renderer

声明式 UI。

### web_restricted

隔离 Web UI，有受控 Bridge。

### web_isolated

更严格，不允许调用扩展 Main Runtime，仅允许静态展示或极少 Host API。

---

## 十一、显示元数据

```go
type UIDisplayMetadata struct {
    Title       LocalizedText
    Description LocalizedText
    Icon        string
    Badge       *UIBadgeDefinition
    Category    string
    Keywords    []string
}
```

Icon：

-包内资源；
-受 Integrity；
-尺寸和格式限制；
-不得远程加载。

---

## 十二、Visibility Rule

允许受限规则：

```text
extension enabled
module enabled
character selected
conversation active
message type matches
platform matches
runtime ready
feature enabled
user setting
```

禁止：

-任意 JavaScript 表达式；
-直接读取 Store；
-读取 Secret；
-网络判断；
-任意 DOM 判断。

建议：

```go
type UIVisibilityRule struct {
    RequiredContext []string
    Platforms       []string
    MessageTypes    []string
    Conditions      []UICondition
}
```

---

## 十三、UI Data Contract

定义宿主向 UI 提供的最小数据。

```go
type UIDataContract struct {
    InputSchema      json.RawMessage
    OutputSchema     json.RawMessage
    RefreshPolicy    string
    SensitiveFields []string
    MaxPayloadBytes  int64
}
```

不得默认提供完整 Character、Conversation、Message 或 Memory 对象。

---

## 十四、UI Context

建议统一：

```ts
interface UIContext {
  contributionId: string;
  extensionId: string;
  moduleId: string;
  slotId: string;
  contractVersion: number;
  platform: "windows" | "macos" | "linux";
  theme: ThemeSnapshot;
  locale: string;
  scope: ScopeSummary;
  data: unknown;
}
```

不包含：

-真实文件路径；
-数据库 ID 内部字段；
-Secret；
-Host API Token；
-Electron 对象；
-Pinia Store；
-系统 Prompt。

---

## 十五、UI Action

建议：

```go
type UIActionDefinition struct {
    ActionID      string
    Title         LocalizedText
    Icon          string
    InputSchema   json.RawMessage
    Target        UIActionTarget
    RiskLevel     RiskLevel
    Confirmation  string
}
```

Target 类型：

```text
host_command
extension_runtime
tool
workflow
navigation
dialog
copy
open_resource
```

---

## 十六、Action 执行链

```text
User Click
→ UI Host
→ Validate Action
→ Scope/Permission
→ Confirmation
→ Host Command / Tool / Runtime
→ Result
→ UI Feedback
```

UI 不能直接执行 Runtime。

---

## 十七、导航

扩展只能导航到：

-自身 Extension Page；
-宿主允许的稳定 Route ID；
-自身声明的子页面；
-受控资源页。

禁止任意 URL 或内部 Vue 路由路径。

---

## 十八、主题协议

宿主向 UI 提供 Theme Snapshot：

```ts
interface ThemeSnapshot {
  mode: "light" | "dark";
  density: "comfortable" | "compact";
  tokens: Record<string, string | number>;
}
```

扩展不得：

-覆盖全局 CSS；
-修改 `:root`；
-注入全局字体；
-使用宿主私有 Class；
-依赖 DOM 层级。

---

## 十九、样式隔离

Schema UI 使用宿主组件，无样式注入。

Restricted Web UI：

-Shadow DOM 或 iframe/WebView 隔离；
-自身 CSS；
-禁止影响宿主；
-限制字体和资源；
-禁用外链 CSS；
-CSP。

---

## 二十、本地化

UI 文案：

-Manifest/本地化文件；
-宿主 Locale；
-缺失语言回退；
-不运行动态代码生成基础文案。

扩展可在 Runtime 返回动态内容，但仍需长度和安全限制。

---

## 二十一、无障碍

UI Contribution 必须满足：

-键盘可用；
-焦点管理；
-ARIA；
-对比度；
-缩放；
-屏幕阅读器；
-不只用颜色表达；
-Reduced Motion。

Schema Renderer 自动提供基础保障。

---

## 二十二、性能预算

每个 Slot 规定：

-首屏加载；
-Bundle 大小；
-内存；
-事件速率；
-更新频率；
-渲染耗时；
-并发请求；
-后台不可见暂停。

超预算可降级或停用 UI Contribution。

---

## 二十三、生命周期

UI 状态：

```text
registered
loading
mounted
visible
hidden
suspended
failed
unmounted
```

Extension Disable：

-隐藏；
-取消请求；
-卸载；
-释放资源。

Runtime Crash：

-显示降级或错误；
-不让页面崩溃；
-允许重试。

---

## 二十四、状态持久化

UI 临时状态：

-由 UI Host 或沙箱内存管理；
-不可作为业务真值。

需要持久化：

```text
Storage Broker
```

---

## 二十五、UI Bridge

核心方法：

```text
ui.ready
ui.action.invoke
ui.data.request
ui.data.subscribe
ui.resize.request
ui.navigation.request
ui.dialog.request
ui.resource.open
ui.log
```

每个方法有 Schema 和权限。

---

## 二十六、消息来源验证

Web UI Bridge 必须验证：

-Frame/Window；
-Contribution ID；
-Session；
-Origin；
-Generation；
-Message Schema；
-Contract Version。

禁止使用通用 `window.postMessage("*")` 无校验。

---

## 二十七、错误模型

```text
ui_contract_invalid
slot_unsupported
sandbox_unsupported
entry_missing
bundle_integrity_failed
bridge_auth_failed
action_not_declared
permission_denied
scope_denied
payload_invalid
render_timeout
runtime_unavailable
ui_crashed
```

---

## 二十八、Contribution Registry 接入

UI Contribution 只能通过统一 Registry 注册。

Registry 调用 UI Adapter：

```text
Register Definition
→ Validate Slot
→ Register with UI Host
→ Activate when Effective
```

---

## 二十九、Host API 接入

复杂 UI 不能直接使用完整 Host API。

使用 UI Host Bridge 的受限子集。

需要执行 Tool/Workflow 时由 UI Host 创建正式 Invocation。

---

## 三十、持久化建议

```text
ui_slot_definitions
ui_contribution_registration_states
ui_contribution_sessions
ui_contribution_failures
ui_action_invocations
ui_layout_preferences
```

Definition 仍属于 Extension Domain。

---

## 三十一、开发者工具

支持：

-查看 Slot；
-查看 Contract；
-查看 Contribution；
-查看 Sandbox；
-查看消息；
-查看性能；
-模拟主题；
-模拟平台；
-模拟 Scope；
-重载；
-错误诊断。

不得允许直接注入生产页面脚本。

---

## 三十二、测试要求

必须覆盖：

-全部 Kind；
-Slot Contract；
-Contract Version；
-Schema UI；
-Web UI；
-Host Action；
-Visibility；
-Scope；
-Permission；
-主题；
-本地化；
-无障碍；
-Action；
-导航；
-Bridge 认证；
-旧 Generation；
-大 Payload；
-性能；
-Runtime Crash；
-Disable；
-Uninstall；
-多扩展冲突；
-跨平台。

---

## 三十三、实施任务

1. 定义 UI Contribution Domain Spec。
2. 定义 Slot Contract。
3. 定义 Kind 与 Sandbox。
4. 定义 UI Data Contract。
5. 定义 UI Action。
6. 定义 UI Context。
7. 定义 Theme/Locale Contract。
8. 定义 UI Bridge。
9. 实现 UI Contribution Validator。
10. 实现 Contribution Adapter。
11. 接入 EffectiveStateResolver。
12. 接入 Permission/Scope。
13. 接入 Runtime Supervisor。
14. 建立 UI Host 基础。
15. 实现开发者诊断。
16. 建立 Contract 兼容测试。
17. 输出全部 Slot 初始清单。
18. 完成安全与性能测试。

---

## 三十四、验收标准

1. UI Contribution 有独立协议。
2. UI 不直接注入 Vue 组件。
3. UI Runtime 与 Main Runtime 分离。
4. Slot 和 Contract 版本化。
5. 支持 Schema、Restricted Web、Host-native 三类模式。
6. UI Context 最小化。
7. Action 经过统一执行链。
8. 主题和样式隔离。
9. Bridge 有身份与 Generation。
10. UI Contribution 只通过 Registry。
11. 故障不会拖垮宿主页面。
12. 可进入第 42 步 Schema UI。

---

## 三十五、执行约束

> UI Contribution 只允许在宿主定义的稳定扩展点内渲染和交互，不得获得对 Vue 应用、Electron 主进程、全局 DOM 或内部 Store 的直接控制权。

禁止：

-动态 import 扩展 Vue；
-Node Integration；
-全局 CSS；
-任意 postMessage；
-远程页面；
-直接 IPC；
-UI 自行执行 Tool；
-前端路由路径作为稳定 Contract；
-新旧 UI 注入体系并行。
