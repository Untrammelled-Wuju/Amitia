# Amitia 扩展系统重构第 45 步实施文档

## 第 45 步：实现 Extension Page Host

---

## 一、步骤目标

在第 41—44 步完成 UI Contribution 协议、Schema UI、沙箱 Web UI 和前端 Extension Slots 后，实现 Amitia 统一 Extension Page Host。

本步骤目标是：

> 为扩展提供统一、稳定、可导航、可恢复、可授权、可主题化、可诊断的完整页面宿主，替代插件自行注入 Vue Router、动态 Vue Component、独立管理页和前端特殊分支。

统一页面入口：

```text
/extension/:extensionId/page/:pageId
```

页面解析链：

```text
Route Parameters
→ Extension Page Registry
→ Contribution Registry
→ Effective State
→ Scope / Permission
→ Page Contract
→ Schema UI Host / Sandbox Web UI Host
→ Render
```

---

## 二、为什么需要独立 Page Host

Extension Slot 适合局部挂载，但完整页面还需要解决：

-路由稳定性；
-页面导航；
-标题；
-面包屑；
-权限；
-角色与会话 Scope；
-加载与错误；
-浏览历史；
-刷新恢复；
-深链接；
-页面 Session；
-扩展更新；
-运行时崩溃；
-窗口和弹层；
-诊断；
-卸载后退路。

如果每个扩展自行注册页面，会重新产生：

-路由冲突；
-内部组件依赖；
-状态泄漏；
-无法卸载；
-更新后旧路由残留；
-主题和布局不一致。

---

## 三、页面 Contribution

页面仍属于 UI Contribution。

建议 Kind：

```text
schema_page
web_page
```

Page Spec：

```go
type ExtensionPageSpec struct {
    PageID          string
    RouteKey        string
    Title           LocalizedText
    Description     LocalizedText
    Icon            string
    Navigation      PageNavigationDefinition
    Entry           UIEntryDefinition
    Layout          ExtensionPageLayout
    ScopeRule       ScopeRule
    Permissions     []PermissionRequirement
    DeepLinkPolicy  DeepLinkPolicy
    StatePolicy     PageStatePolicy
}
```

---

## 四、稳定 Page ID

Page ID 在 Module 内唯一。

全局页面标识：

```text
<extension-id>#<module-id>/ui/page/<page-id>
```

URL 不包含文件路径和 Runtime Entry。

---

## 五、统一 Route

建议：

```text
/extension/:extensionId/page/:pageId
```

可选查询参数只用于宿主定义的：

-角色；
-会话；
-资源；
-Tab；
-只读筛选。

禁止扩展自行定义顶层 Route Pattern。

---

## 六、路由解析

流程：

1.解析 Extension ID。
2.解析 Page ID。
3.查询 Contribution Registry。
4.校验 Kind。
5.校验 Effective State。
6.校验 Contract Version。
7.解析 Scope。
8.校验 Permission。
9.创建 Page Session。
10.渲染 Host。

---

## 七、Page Registry

建议：

```go
type ExtensionPageRegistry interface {
    Register(
        ctx context.Context,
        contribution UIContributionDefinition,
    ) error

    Resolve(
        ctx context.Context,
        extensionID ExtensionID,
        pageID string,
    ) (ExtensionPageDefinition, error)

    ListNavigation(
        ctx context.Context,
        query PageNavigationQuery,
    ) ([]ExtensionPageNavigationItem, error)
}
```

Page Registry 是 UI Contribution Registry 的读模型或适配层，不独立成为第二真值。

---

## 八、页面布局

宿主控制外层：

```text
App Shell
├── Global Navigation
├── Page Header
├── Breadcrumb
├── Extension Identity
├── Page Actions
├── Page Content Host
└── Diagnostics/Error
```

扩展只控制 Content Host 内部。

---

## 九、Extension Identity

页面顶部应显示：

-Extension Icon；
-名称；
-页面名称；
-运行状态；
-故障提示；
-开发模式标识；
-未知发布者或高风险标识，必要时。

避免用户误以为扩展页面是 Amitia 核心页面。

---

## 十、页面导航

扩展可声明内部导航树：

```go
type PageNavigationDefinition struct {
    Group       string
    Order       int
    ParentPage  string
    Hidden      bool
}
```

宿主生成：

-侧栏；
-Tab；
-面包屑；
-返回。

扩展不得自行覆盖全局导航。

---

## 十一、页面动作

页面 Header Action 仍属于 UI Action。

例如：

-刷新；
-保存；
-导出；
-运行 Workflow；
-连接 MCP；
-打开诊断。

全部经过 Action Dispatcher。

---

## 十二、Scope

Page 可声明：

```text
global
character
conversation
```

如果需要角色：

-未选角色显示选择状态；
-切换角色时更新 Session；
-旧数据清理；
-不得自动读取默认角色以外内容。

如果需要会话：

-校验会话属于角色；
-会话失效后页面进入受控空状态。

---

## 十三、Permission

Page 本身可访问的 Data Source 和 Action 需要单独 Permission。

打开页面不等于授予页面全部功能。

页面可以部分显示：

```text
基本信息可见
危险操作被锁定
```

---

## 十四、Page Session

建议：

```go
type ExtensionPageSession struct {
    SessionID      string
    ContributionID ContributionID
    ExtensionID    ExtensionID
    ModuleID       ModuleID
    PageID         string
    Generation     int64
    ScopeSnapshot  string
    Contract       int
    CreatedAt      time.Time
    LastActiveAt   time.Time
}
```

Session 绑定 Generation，更新后旧 Session 失效。

---

## 十五、刷新恢复

浏览器刷新或 Electron Renderer 重载：

-从 Route 重新解析；
-不依赖内存对象；
-重建 Page Session；
-恢复允许持久化的页面状态；
-重新检查 Scope 和 Permission。

---

## 十六、页面状态策略

支持：

```text
ephemeral
session
persistent_preferences
```

### ephemeral

刷新丢失。

### session

同一 UI Session 保留。

### persistent_preferences

仅保存非敏感布局偏好到宿主 UI Store。

业务数据必须走 Storage Broker。

---

## 十七、Deep Link

可支持：

```text
amitia://extension/<extension-id>/page/<page-id>
```

Deep Link 处理：

-解析；
-验证 Extension 已安装；
-验证页面；
-权限；
-Scope；
-用户确认外部来源；
-导航。

不得允许 Deep Link 携带 Secret 或任意脚本参数。

---

## 十八、页面参数

Page Contract 声明允许参数：

```go
type PageParameterDefinition struct {
    Name         string
    Schema       json.RawMessage
    Required     bool
    Sensitive    bool
}
```

未知参数忽略或拒绝。

---

## 十九、加载状态

统一：

```text
resolving
permission_check
runtime_starting
loading
ready
degraded
failed
disabled
not_installed
incompatible
```

页面不应在 Runtime 启动时无限空白。

---

## 二十、Runtime 按需启动

若页面需要 Main Runtime：

```text
Open Page
→ Effective State
→ Runtime Desired State already running or request reconcile
→ Wait bounded time
→ Render
```

Page Host 不直接启动进程，而是请求 Lifecycle/Runtime Reconcile。

---

## 二十一、Runtime 故障

Runtime Crash：

-Page Host 保留宿主外壳；
-显示故障；
-提供重试；
-提供诊断；
-取消 Data Subscription；
-不刷新整个应用。

---

## 二十二、Extension 更新

更新时：

1.页面进入 suspended。
2.取消请求。
3.销毁旧 Session。
4.Generation 切换。
5.重新解析 Page Contribution。
6.如果 Contract 兼容则重载。
7.不兼容则返回 Extension 详情或显示更新提示。

---

## 二十三、Extension 禁用与卸载

### 禁用

-页面显示已禁用；
-停止 UI Session；
-保留返回和启用入口；
-不继续后台请求。

### 卸载

-Route 失效；
-销毁 Session；
-导航回 Extension Center；
-历史浏览返回时显示已卸载状态；
-释放资源。

---

## 二十四、多窗口

若未来支持 Extension 独立窗口：

-仍使用 Page Host；
-窗口属于 Desktop Contribution；
-共享或独立 Page Session 按策略；
-不复制 Runtime；
-窗口关闭清理；
-权限不扩大。

---

## 二十五、Dialog 和 Drawer 页面

可复用 Page Host 的轻量模式：

```text
mode=dialog
mode=drawer
mode=fullpage
```

宿主决定尺寸、焦点和 Overlay。

---

## 二十六、页面菜单

扩展 Page 可声明局部菜单，但只能在 Page Host 内。

禁止修改全局应用菜单，除非使用 Desktop Menu Contribution。

---

## 二十七、错误边界

分层错误：

-Route；
-Registry；
-Contract；
-Scope；
-Permission；
-Runtime；
-Renderer；
-Bridge；
-Action；
-Data Source。

每层显示不同可解释错误。

---

## 二十八、诊断信息

开发者模式显示：

-Page Contribution；
-Slot；
-Contract；
-Generation；
-Scope；
-Permission；
-Runtime；
-UI Session；
-Data Source；
-Bridge；
-性能；
-错误。

普通用户只显示必要解释。

---

## 二十九、性能

-页面定义批量缓存；
-按需加载 Bundle；
-隐藏页面暂停；
-Session 有界；
-大页面资源懒加载；
-避免 Main Runtime 重复启动；
-导航不重复解析全部 Extension；
-错误日志限流。

---

## 三十、安全

必须防止：

-Route 参数注入；
-跨 Extension Page ID；
-Deep Link 越权；
-页面参数泄密；
-旧 Generation Session；
-隐藏页面继续读取；
-页面返回时重用旧 Scope；
-扩展伪装核心页面。

---

## 三十一、API

建议：

```text
GET /api/extensions/:extensionId/pages
GET /api/extensions/:extensionId/pages/:pageId
POST /api/extensions/ui/page-sessions
DELETE /api/extensions/ui/page-sessions/:id
```

前端可通过统一 Snapshot 获取，减少请求。

---

## 三十二、测试要求

覆盖：

-Route；
-Page Registry；
-Schema Page；
-Web Page；
-Scope；
-Permission；
-角色切换；
-会话失效；
-Deep Link；
-参数；
-刷新；
-Back/Forward；
-Runtime 启动；
-Crash；
-更新；
-禁用；
-卸载；
-多窗口预留；
-Dialog；
-错误边界；
-性能；
-安全参数 Fuzz。

---

## 三十三、实施任务

1. 定义 Extension Page Spec。
2. 建立 Page Registry Read Model。
3. 建立统一 Route。
4. 实现 Route Resolver。
5. 实现 Page Shell。
6. 实现 Page Session。
7. 实现 Scope/Permission 检查。
8. 接入 Schema UI Host。
9. 接入 Sandbox Web UI Host。
10. 实现 Header/Navigation/Action。
11. 实现 Deep Link。
12. 实现状态恢复。
13. 实现更新、禁用、卸载处理。
14. 实现错误边界。
15. 实现诊断。
16. 迁移旧 Plugin 独立管理页。
17. 完成安全和性能测试。

---

## 三十四、验收标准

1. 所有扩展完整页面使用统一 Page Host。
2. 扩展不直接注册 Vue Router。
3. URL 不含 Entry 文件。
4. Page Session 绑定 Generation 和 Scope。
5.刷新可重建。
6. Deep Link 可校验。
7. Runtime 故障不破坏 App Shell。
8.更新可安全销毁旧 Session。
9.卸载后路由不残留。
10.页面明确标识扩展身份。
11.旧独立插件页已有迁移报告。
12.可进入第 46 步聊天与消息 UI 扩展。

---

## 三十五、执行约束

> Extension Page Host 是扩展完整页面的唯一宿主，扩展只能声明页面和局部导航，不能注册任意路由、覆盖 App Shell 或伪装 Amitia 核心页面。

禁止：

-动态 addRoute；
-Entry 路径进 URL；
-页面直接启动 Runtime；
-页面直接 IPC；
-Deep Link 携带 Secret；
-扩展覆盖全局导航；
-刷新依赖内存 Handler；
-卸载后保留路由。
