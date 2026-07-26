# Amitia 扩展系统重构第 44 步实施文档

## 第 44 步：建立前端 Extension Slots

---

## 一、步骤目标

在 UI Contribution 协议、Schema UI 与沙箱 Web UI 基础上，改造 Amitia Vue 前端，使所有可扩展位置通过稳定 Extension Slot 暴露，而不是依赖 DOM、组件路径或路由实现细节。

本步骤目标：

> 建立统一 `<ExtensionSlot>` 宿主组件、Slot Registry、Slot Contract、Context Provider、Loading/Error Boundary、排序、可见性和生命周期，使扩展 UI 能以稳定协议挂载到前端。

---

## 二、核心原则

### 1. Slot 是正式 API

Slot ID 稳定，不使用 Vue 文件名或 CSS Selector。

### 2. 宿主掌握布局

扩展只能在 Slot 边界内渲染。

### 3. Slot Context 最小化

不同 Slot 只提供所需 Context。

### 4. UI 类型统一承载

Schema UI、Web UI、Host-native Action 使用同一 Slot Resolver。

---

## 三、核心组件

建议：

```text
ExtensionSlot
ExtensionSlotRegistry
ExtensionSlotResolver
ExtensionContributionHost
ExtensionSlotContextProvider
ExtensionSlotErrorBoundary
ExtensionSlotSkeleton
ExtensionSlotDiagnostics
```

---

## 四、Vue 组件

示例：

```vue
<ExtensionSlot
  slot-id="chat.header.action"
  :context="slotContext"
  layout="inline"
  fallback="none"
/>
```

前端页面不直接查询具体 Extension。

---

## 五、Slot Definition

```ts
interface ExtensionSlotDefinition {
  slotId: string;
  contractVersion: number;
  supportedKinds: UIContributionKind[];
  multiplicity: SlotMultiplicity;
  layout: SlotLayout;
  contextSchema: JsonSchema;
  performanceBudget: UIPerformanceBudget;
  fallbackPolicy: string;
}
```

---

## 六、初始 Slot 清单

建议第一阶段：

### Extension 管理

```text
extension.center.header.action
extension.center.card.badge
extension.detail.tab
extension.detail.action
extension.settings.page
extension.settings.section
```

### Chat

```text
chat.header.action
chat.sidebar.panel
chat.message.action
chat.message.renderer
chat.composer.action
chat.composer.attachment
chat.empty_state.card
```

### Character

```text
character.detail.tab
character.detail.action
character.sidebar.card
```

### System

```text
system.status.item
system.settings.section
system.diagnostics.tab
```

### Desktop

```text
desktop.command
desktop.menu.item
desktop.tray.item
desktop.window.page
```

---

## 七、Slot Context

每个 Slot 由类型专用 Builder 构建。

例如消息 Action：

```ts
interface ChatMessageActionContext {
  messageId: string;
  messageType: string;
  direction: "incoming" | "outgoing";
  characterId: string;
  conversationId: string;
  capabilities: string[];
}
```

不提供完整 Message Body，除非 Contribution 有权限且通过 Data Request 获取。

---

## 八、Context Schema

Slot Context 必须有 Schema 和版本。

扩展只能读取 Contract 声明字段。

宿主内部对象变化不影响扩展。

---

## 九、Context 生成

```text
Page State
→ Slot Context Builder
→ Scope Filter
→ Sensitive Field Reduction
→ Immutable Snapshot
→ UI Contribution
```

禁止直接传递 Reactive Store。

---

## 十、Slot Resolver

流程：

```text
Slot ID
→ Contribution Registry Query
→ Effective State
→ Contract Compatibility
→ Visibility Rule
→ Ordering
→ Conflict
→ Render List
```

---

## 十一、渲染 Host

根据 UI 类型：

```text
schema_page → SchemaUIHost
web_page → SandboxWebUIHost
action → HostNativeActionHost
panel/card → 对应 Host
```

---

## 十二、错误边界

每个 Contribution 独立 Error Boundary。

错误时：

-只影响自身；
-显示可控占位；
-记录；
-可禁用；
-可重试；
-不破坏 Slot 其他项。

---

## 十三、Loading

不同 UI 使用：

-Host-native：立即；
-Schema UI：轻量 Skeleton；
-Web UI：异步加载；
-隐藏 Slot 不预加载；
-高频页面避免重复启动 Session。

---

## 十四、缓存与复用

可按：

```text
contribution_id + generation + slot_context_key
```

复用。

涉及角色/会话时必须正确销毁或重新绑定 Scope。

---

## 十五、Keep Alive

仅允许 Slot Contract 明确支持。

禁止所有 Extension Page 默认 KeepAlive，防止：

-内存泄漏；
-旧 Scope；
-旧 Session；
-后台活动；
-隐藏网络请求。

---

## 十六、可见性

结合：

-Extension/Module；
-Contribution Override；
-平台；
-当前页面；
-角色；
-会话；
-消息类型；
-Runtime；
-Permission；
-Scope；
-用户偏好。

---

## 十七、性能

Slot Resolver 应批量查询，不为每个 Slot 单独请求后端。

建议前端获取：

```text
UI Contribution Snapshot
```

后端状态变化增量推送。

---

## 十八、前端状态管理

建立专用：

```text
extensionUIStore
```

只保存：

-Definition Summary；
-Registration；
-Effective State；
-Slot Mapping；
-UI Session；
-错误；
-用户布局偏好。

不保存后端业务真值。

---

## 十九、Router 扩展

扩展页面不直接向 Vue Router 动态添加任意路由。

统一使用：

```text
/extension/:extensionId/page/:pageId
```

由 Extension Page Host 决定渲染。

---

## 二十、Dialog/Drawer

扩展请求打开 Dialog：

```text
UI Action
→ Host Dialog Service
→ Extension UI Host
```

不得直接 Teleport 到宿主 Body。

---

## 二十一、Z-index

扩展不能设置全局 Z-index。

所有弹层由宿主 Overlay Manager 承载。

---

## 二十二、快捷键

前端 Slot 只展示快捷键绑定结果。

真正注册由 Desktop Extension Point 管理。

---

## 二十三、主题变化

Theme Store 变化：

-更新 Schema UI；
-向 Web UI 推送 Theme Snapshot；
-不重载整个 Runtime；
-不暴露内部 Store。

---

## 二十四、Locale 变化

更新本地化资源和 UI Context。

Web UI 接收 Locale Changed 消息。

---

## 二十五、Scope 切换

角色或会话切换：

-取消旧 Data Subscription；
-失效旧 Context；
-更新 UI Session；
-销毁不支持重绑定的 Web UI；
-防止跨角色数据残留。

---

## 二十六、卸载

Contribution Registry 事件：

```text
Deactivate
→ Slot Resolver 更新
→ UI Unmount
→ Session Destroy
→ Resource Handle Release
→ Store Cleanup
```

---

## 二十七、开发者诊断

页面可显示：

-Slot ID；
-Contract；
-匹配 Contribution；
-排序；
-被隐藏原因；
-冲突；
-渲染类型；
-Session；
-性能；
-错误。

---

## 二十八、测试要求

覆盖：

-所有 Slot；
-无 Contribution；
-单个；
-多个；
-冲突；
-排序；
-Schema/Web/Action；
-Context；
-Scope 切换；
-角色切换；
-主题；
-Locale；
-Error Boundary；
-Runtime Crash；
-Disable；
-Uninstall；
-缓存；
-KeepAlive；
-批量状态；
-性能。

---

## 二十九、实施任务

1. 定义 Slot Registry。
2. 实现 `<ExtensionSlot>`。
3. 实现 Slot Context Builder。
4. 实现 Slot Resolver。
5. 实现 UI Host Dispatcher。
6. 实现 Error Boundary/Skeleton。
7. 建立 extensionUIStore。
8. 建立后端 Snapshot API。
9. 接入状态增量事件。
10. 建立统一 Extension Route。
11. 实现 Dialog/Drawer Host。
12. 实现 Theme/Locale 推送。
13. 实现 Scope 切换清理。
14. 替换现有硬编码 Plugin UI 注入。
15. 建立 Slot Diagnostics。
16. 完成性能和回归测试。

---

## 三十、验收标准

1. 前端所有扩展位置使用稳定 Slot。
2. Slot 不依赖 DOM/CSS Selector。
3. Context 有 Schema。
4. 不传递 Reactive Store。
5. Schema/Web/Action 统一承载。
6. 每个 Contribution 有错误边界。
7. 扩展页面使用统一 Host Route。
8. Dialog/Overlay 由宿主管理。
9. Scope 切换不会泄露旧数据。
10. 卸载可完整清理。
11. 前端批量查询状态。
12. 可进入第 45 步扩展页面宿主。

---

## 三十一、执行约束

> Extension Slot 是前端唯一扩展挂载入口，页面不得直接识别具体插件，也不得允许扩展依赖 Vue 组件路径、Store 结构或 DOM。

禁止：

-动态 addRoute 任意路径；
-动态 import 插件组件；
-直接 Pinia；
-直接 Teleport；
-全局 Z-index；
-Context 传完整对象；
-每 Slot 单独轮询；
-卸载后残留 Session。
