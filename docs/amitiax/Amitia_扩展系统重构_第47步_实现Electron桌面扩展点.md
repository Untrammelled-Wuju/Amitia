# Amitia 扩展系统重构第 47 步实施文档

## 第 47 步：实现 Electron 桌面扩展点

---

## 一、步骤目标

为 Amitia Electron 桌面端建立受控 Desktop Contribution 协议，使扩展可以贡献命令、菜单、托盘项、快捷键、通知动作、独立窗口入口和受限系统集成，但不能直接进入 Electron Main Process 或获得任意 IPC、Shell、文件和输入控制权限。

本步骤目标：

> 将桌面集成能力收敛到 Electron Desktop Extension Host，由 Extension Kernel、Permission Broker、Runtime Supervisor 和 Resource Ownership 统一管理。

---

## 二、允许的 Desktop Contribution

建议第一阶段开放：

```text
desktop.command
desktop.application_menu.item
desktop.context_menu.item
desktop.tray.item
desktop.shortcut
desktop.notification.action
desktop.window.page
desktop.protocol.handler
desktop.file_open.action
desktop.status.item
```

暂不开放：

-任意 Electron Main 脚本；
-任意 BrowserWindow 创建；
-任意 IPC Channel；
-任意原生菜单模板；
-任意全局键盘监听；
-系统输入注入；
-任意 Shell 命令；
-任意系统服务注册。

---

## 三、核心架构

```text
DesktopContributionDefinition
→ Contribution Registry
→ Desktop Contribution Adapter
→ Electron Desktop Extension Host
→ Permission / Conflict / Platform
→ Native Electron API
```

插件 Main Runtime 不能直接调用 Electron。

---

## 四、Desktop Host 边界

Electron Main 中只存在宿主实现：

```text
DesktopExtensionHost
DesktopCommandRegistry
DesktopMenuManager
DesktopTrayManager
DesktopShortcutManager
DesktopWindowHost
DesktopNotificationHost
```

第三方代码不进入 Main。

---

## 五、Desktop Command

定义：

```go
type DesktopCommandSpec struct {
    CommandID      string
    Title          LocalizedText
    Description    LocalizedText
    Icon           string
    Action         UIActionTarget
    Availability   UIVisibilityRule
    RiskLevel      RiskLevel
}
```

命令可被：

-菜单；
-托盘；
-快捷键；
-命令面板；
-按钮；

复用。

---

## 六、命令执行链

```text
Desktop Trigger
→ Resolve Command
→ Effective State
→ Scope
→ Permission
→ Confirmation
→ Tool / Workflow / Runtime / Navigation
→ Result Notification
```

Desktop Host 不直接调用插件 Handler。

---

## 七、Application Menu

扩展可贡献菜单项到稳定位置：

```text
extensions
tools
view
help
context.chat
context.message
context.extension
```

不能任意重排核心菜单。

---

## 八、菜单定义

```go
type DesktopMenuItemSpec struct {
    MenuID       string
    ParentSlot   string
    CommandID    string
    Group        string
    Order        int
    Platforms    []string
    Separator    string
}
```

菜单由宿主生成。

---

## 九、Context Menu

Context Menu 必须绑定 Context Contract：

-消息；
-输入框；
-角色；
-扩展；
-资源；
-窗口。

只提供最小 ID 和能力。

---

## 十、Tray Item

扩展可在 Amitia 托盘菜单的 Extension 分组添加项。

限制：

-数量；
-图标；
-标题；
-状态更新频率；
-不创建独立系统托盘图标，第一阶段默认禁止；
-卸载完整清理。

---

## 十一、独立托盘图标

如未来开放，必须是高风险 Desktop Capability：

-明确用户启用；
-资源；
-生命周期；
-退出；
-图标；
-冲突；
-系统限制。

本步骤不默认开放。

---

## 十二、Shortcut

支持：

```text
application shortcut
global shortcut
```

### Application Shortcut

仅 Amitia 聚焦时，风险较低。

### Global Shortcut

系统级注册，需要独立 Permission 和用户确认。

---

## 十三、快捷键冲突

必须检测：

-核心快捷键；
-其他 Extension；
-操作系统保留；
-布局差异；
-平台差异。

冲突时：

-不自动覆盖；
-要求用户选择；
-提供替代；
-记录。

---

## 十四、快捷键安全

全局快捷键只能触发已声明 Desktop Command。

不能将按键内容发送给插件。

不得实现全局键盘监听。

---

## 十五、Notification Action

扩展可请求宿主通知：

```text
desktop.notification.show
```

并声明 Action。

限制：

-频率；
-用户设置；
-角色/会话 Scope；
-敏感内容；
-通知预览；
-夜间模式；
-点击导航；
-审计。

---

## 十六、Window Page

扩展可声明独立窗口入口，但内容仍是 Extension Page Host。

定义：

```go
type DesktopWindowPageSpec struct {
    PageID        string
    WindowRole    string
    DefaultSize   WindowSize
    MinSize       WindowSize
    MaxSize       WindowSize
    Resizable     bool
    Singleton     bool
    AlwaysOnTop   bool
}
```

宿主可收紧所有字段。

---

## 十七、窗口权限

普通独立窗口不需要特殊高风险权限，但以下需单独控制：

-Always on Top；
-透明窗口；
-无边框；
-屏幕捕获；
-覆盖层；
-点击穿透；
-隐藏任务栏；
-多显示器定位。

第一阶段禁止第三方使用高风险窗口特性。

---

## 十八、窗口创建

```text
Desktop Command
→ DesktopWindowHost
→ Resolve Extension Page
→ Create BrowserWindow with safe defaults
→ Load Host Route
```

不加载扩展文件 URL。

---

## 十九、窗口安全默认

-Node Integration=false；
-Context Isolation=true；
-Sandbox=true；
-Preload 为宿主；
-导航拦截；
-新窗口拦截；
-下载拦截；
-DevTools 按模式；
-Session 隔离；
-Content Security。

---

## 二十、窗口生命周期

Extension Disable/Update/Uninstall：

-通知；
-关闭或 Suspended；
-销毁 UI Session；
-释放窗口资源；
-移除菜单和快捷键；
-不残留隐藏窗口。

---

## 二十一、Protocol Handler

扩展可声明 Amitia 内部协议子路径：

```text
amitia://extension/<extension-id>/...
```

不得注册操作系统全局新协议，第一阶段禁止。

---

## 二十二、File Open Action

扩展可声明处理某些文件类型的动作，但：

-不注册系统文件关联，第一阶段；
-用户在 Amitia 内选择文件；
-返回 Resource Handle；
-按 MIME/扩展名；
-权限；
-大小；
-不提供真实路径。

---

## 二十三、Clipboard

Desktop Command 可调用 Clipboard Host API。

读写分离，读取通常要求用户动作。

---

## 二十四、System Open

打开外部 URL 或文件：

-URL 白名单；
-协议；
-用户确认；
-文件必须来自 Resource Handle；
-不能任意 `shell.openPath`。

---

## 二十五、Auto-start

Extension 不得自行配置 Amitia 开机自启动。

只能请求：

```text
desktop.autostart.request
```

但开机自启动属于 Amitia 全局设置，不应由普通 Extension 控制。

第一阶段禁止扩展修改。

---

## 二十六、后台运行

Extension Module 的后台运行由 Runtime/Lifecycle 控制，不由 Electron 窗口是否存在决定。

桌面 Contribution 不等于后台服务权限。

---

## 二十七、Desktop Permission

建议拆分：

```text
desktop.command.register
desktop.menu.register
desktop.tray.register
desktop.shortcut.application
desktop.shortcut.global
desktop.notification.show
desktop.window.open
desktop.clipboard.read
desktop.clipboard.write
desktop.external_url.open
desktop.resource.save
```

禁止万能 `desktop.full_access`。

---

## 二十八、平台适配

### Windows

-菜单习惯；
-托盘；
-全局快捷键；
-文件锁；
-DPI；
-任务栏。

### macOS

-应用菜单；
-Dock；
-系统快捷键；
-沙箱；
-窗口行为；
-菜单栏。

### Linux

-桌面环境差异；
-托盘支持；
-快捷键；
-通知服务；
-Wayland/X11。

Contribution 应声明平台兼容。

---

## 二十九、Electron IPC

Renderer 或 UI 只能调用宿主固定 IPC。

Extension ID、Contribution ID 和 Session 由 Host 注入。

禁止动态 IPC Channel：

```text
plugin:<id>:<anything>
```

---

## 三十、资源所有权

登记：

-菜单项；
-快捷键；
-托盘项；
-窗口；
-通知订阅；
-协议路由；
-状态项。

卸载时可完整释放。

---

## 三十一、冲突与排序

Desktop Contribution 使用第 48 步统一规则：

-菜单组；
-顺序；
-快捷键冲突；
-单例窗口；
-命令 ID；
-托盘数量；
-核心保留项。

---

## 三十二、前端与开发者诊断

Extension 详情展示：

-Desktop Commands；
-菜单；
-快捷键；
-托盘；
-窗口；
-权限；
-平台；
-冲突；
-当前注册状态。

---

## 三十三、测试要求

覆盖：

-Command；
-Menu；
-Context Menu；
-Tray；
-Application Shortcut；
-Global Shortcut；
-冲突；
-Notification；
-Window；
-安全默认；
-Disable；
-Update；
-Uninstall；
-平台差异；
-Wayland；
-DPI；
-IPC 伪造；
-旧 Session；
-资源泄漏；
-高频状态更新。

---

## 三十四、实施任务

1. 定义 Desktop Contribution Spec。
2. 实现 Desktop Adapter。
3. 建立 Command Registry。
4. 建立 Menu Manager。
5. 建立 Tray Manager。
6. 建立 Shortcut Manager。
7. 建立 Notification Host。
8. 建立 Desktop Window Host。
9. 接入 Extension Page Host。
10. 定义固定 Electron IPC。
11. 接入 Permission/Scope。
12. 接入 Resource Ownership。
13. 实现平台 Capability。
14. 实现冲突检测。
15. 迁移现有 Plugin 菜单/托盘入口。
16. 完成跨平台和安全测试。

---

## 三十五、验收标准

1. 第三方代码不进入 Electron Main。
2. Desktop Contribution 通过统一 Host。
3.命令是所有桌面入口的执行基础。
4.菜单由宿主生成。
5.全局快捷键需独立授权。
6.不实现键盘监听。
7.窗口加载 Extension Page Host。
8.安全 BrowserWindow 默认固定。
9.IPC 不动态开放。
10.卸载完整释放桌面资源。
11.平台差异可解释。
12.可进入第 48 步冲突与排序规则。

---

## 三十六、执行约束

> Desktop Contribution 允许扩展在宿主规定的位置增加桌面入口，但不允许扩展获得 Electron Main、任意 IPC、Shell、系统输入或窗口底层控制权。

禁止：

-主进程插件脚本；
-动态 IPC；
-任意 BrowserWindow；
-任意 shell；
-全局键盘监听；
-默认独立托盘图标；
-扩展修改开机自启动；
-不受控 Always-on-top/Overlay；
-卸载后残留快捷键。
