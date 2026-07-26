# Amitia 扩展系统重构第 43 步实施文档

## 第 43 步：实现沙箱 Web UI

---

## 一、步骤目标

为无法由 Schema UI 满足的复杂扩展界面实现 Restricted Web UI Runtime，使扩展可运行独立 HTML/CSS/JavaScript Bundle，但无法直接访问 Amitia Renderer、Electron、Node、文件、网络、数据库和内部 Store。

本步骤目标：

> 建立基于强隔离 Frame/WebContents、严格 CSP、受认证 UI Bridge、最小 Host API、资源限制、导航限制和生命周期管理的复杂 UI 沙箱。

---

## 二、适用范围

-复杂图表；
-可视化编辑器；
-流程图；
-自定义资源浏览；
-复杂 Provider 配置；
-复杂数据展示；
-多步骤向导；
-高级交互面板。

普通设置和表单优先使用 Schema UI。

---

## 三、技术路线

优先级：

```text
1. sandboxed iframe + isolated origin
2. Electron BrowserView/WebContents with strict preferences
3. WebView 仅在可严格控制时使用
```

必须根据 Electron 版本与当前架构做原型验证。

无论实现方式，必须满足：

-Node Integration=false；
-Context Isolation=true；
-Sandbox=true；
-Remote Module 不可用；
-Preload 仅宿主受控；
-导航拦截；
-下载拦截；
-新窗口拦截；
-CSP；
-Origin 隔离；
-Session 隔离。

---

## 四、独立 Origin

每个 Extension UI 使用逻辑 Origin：

```text
amitia-extension://<extension-id>/<module-id>/
```

由宿主协议处理器提供包内只读资源。

禁止 `file://` 直接加载。

---

## 五、资源协议

自定义协议处理：

-路径安全；
-包内；
-MIME；
-Hash；
-缓存；
-Range；
-禁止目录遍历；
-禁止跨 Extension；
-禁止动态写入；
-禁止执行未声明文件。

---

## 六、CSP

默认：

```text
default-src 'none'
script-src 'self'
style-src 'self'
img-src 'self' amitia-resource:
font-src 'self'
connect-src 'none'
frame-src 'none'
object-src 'none'
base-uri 'none'
form-action 'none'
```

需要网络时不直接放开 `connect-src`，仍使用 Host Bridge 网络 API。

---

## 七、JavaScript 能力

允许页面自身 JS。

禁止：

-Node；
-Electron；
-动态远程脚本；
-eval；
-new Function；
-WASM 默认关闭或单独声明；
-SharedArrayBuffer；
-Service Worker 默认关闭；
-WebRTC 默认关闭；
-Clipboard 直接访问；
-Notification 直接访问；
-Filesystem API；
-WebUSB/WebSerial/WebBluetooth；
-本地端口扫描。

---

## 八、Bridge 注入

Preload 只暴露：

```ts
window.amitiaUI = {
  ready(),
  invokeAction(),
  requestData(),
  subscribeData(),
  navigate(),
  requestResize(),
  openResource(),
  log()
}
```

对象冻结，不能暴露底层 IPC。

---

## 九、Bridge Session

每次 Mount 生成：

-UI Session ID；
-Contribution ID；
-Extension；
-Module；
-Generation；
-Slot；
-Origin；
-Nonce；
-过期时间。

消息必须绑定 Session。

---

## 十、消息协议

所有消息：

-Method；
-Version；
-ID；
-Input Schema；
-Output Schema；
-Deadline；
-大小；
-方向；
-Session。

禁止任意 Channel 名。

---

## 十一、Action

Web UI 只能调用 Manifest 声明的 UI Action。

不能自行构造 Tool ID 调用。

---

## 十二、Data Subscription

订阅必须：

-声明 Data Source；
-速率限制；
-可见时激活；
-隐藏时暂停；
-取消；
-最大 Payload；
-敏感字段裁剪。

---

## 十三、网络

Web UI 网络请求走：

```text
ui.network.request
→ Host Bridge
→ Host API Gateway
```

受域名、方法、Header、Secret、大小和审计限制。

---

## 十四、存储

Web Storage：

-LocalStorage 默认禁用或隔离且不作为业务存储；
-IndexedDB 默认禁用或明确隔离；
-Cookie 禁止；
-持久状态走 Storage Broker。

---

## 十五、剪贴板

通过 UI Action：

```text
ui.clipboard.read
ui.clipboard.write
```

独立 Permission 与用户动作要求。

---

## 十六、文件选择

由宿主打开文件选择器，返回受控 Resource Handle。

Web UI 不获得真实路径。

---

## 十七、下载

禁止浏览器默认下载。

需要导出：

```text
UI Action
→ Artifact
→ Host Save Dialog
```

---

## 十八、导航

允许：

-自身内部路径；
-声明的 Extension 子页面；
-宿主稳定 Route ID；
-外部链接请求宿主打开。

禁止直接修改顶层窗口。

---

## 十九、新窗口

全部拦截。

外部链接：

-显示目标；
-协议白名单；
-用户确认策略；
-使用系统浏览器；
-审计可选。

---

## 二十、样式与主题

宿主通过 Bridge 传 Theme Snapshot。

Web UI 使用 CSS Variables：

```text
--amitia-color-surface
--amitia-color-text
--amitia-spacing-md
...
```

不得读取宿主 DOM。

---

## 二十一、尺寸

Web UI 可请求 Resize，但宿主决定。

限制：

-最小/最大；
-频率；
-Slot；
-避免抖动；
-避免全屏劫持。

---

## 二十二、焦点

宿主管理：

-进入；
-退出；
-Tab 顺序；
-Escape；
-快捷键；
-模态；
-屏幕阅读器。

扩展不得拦截系统保留快捷键。

---

## 二十三、性能限制

-最大 Bundle；
-最大内存；
-CPU；
-Frame Rate；
-后台节流；
-消息速率；
-DOM 节点建议；
-图片；
-定时器；
-隐藏暂停。

---

## 二十四、崩溃处理

Frame Crash：

-销毁 Session；
-取消请求；
-释放 Resource Handle；
-记录；
-显示错误卡；
-允许重新加载；
-高频 Crash 打开 UI Circuit。

不影响宿主页面。

---

## 二十五、开发模式

支持：

-DevTools；
-Source Map；
-热重载；
-Bridge Mock；
-CSP 报告；
-性能；
-网络请求审计。

生产默认关闭 DevTools，或仅用户显式开发模式开启。

---

## 二十六、完整性

所有 HTML/JS/CSS/Asset：

-包内；
-Hash；
-Content Tree；
-不可运行时替换；
-更新使用新 Generation。

---

## 二十七、UI Runtime 与 Main Runtime 通信

Web UI 不直接连 Main Runtime。

链路：

```text
Web UI
→ UI Host Bridge
→ Runtime Supervisor
→ Main Runtime Entry
```

这样保留 Scope、Permission、Trace、Deadline 和审计。

---

## 二十八、前端 Host

建立：

```text
SandboxWebUIHost
```

负责：

-创建；
-加载；
-Session；
-Bridge；
-Theme；
-Lifecycle；
-资源；
-错误；
-销毁。

---

## 二十九、持久化

建议：

```text
ui_web_sessions
ui_web_failures
ui_web_resource_handles
ui_web_action_invocations
ui_web_performance_snapshots
```

不保存浏览器 Cookie。

---

## 三十、安全测试

必须测试：

-Node；
-Electron；
-Preload 逃逸；
-postMessage；
-Origin；
-CSP；
-eval；
-远程脚本；
-导航；
-新窗口；
-下载；
-文件路径；
-Clipboard；
-网络；
-Service Worker；
-IndexedDB；
-跨 Extension；
-旧 Session；
-Bridge Fuzz；
-资源耗尽；
-Crash；
-DevTools；
-更新代际。

---

## 三十一、实施任务

1. 选择并验证隔离容器。
2. 实现自定义 Extension 协议。
3. 实现资源加载器。
4. 实现 CSP。
5. 实现受控 Preload/Bridge。
6. 实现 Session Auth。
7. 实现 Action/Data 通信。
8. 实现 Theme/Locale。
9. 实现导航/窗口/下载拦截。
10. 实现 Resource Handle。
11. 实现网络代理边界。
12. 实现生命周期和销毁。
13. 实现性能监控。
14. 实现 Crash/Circuit。
15. 接入 Contribution Registry。
16. 接入 Runtime Supervisor。
17. 建立安全测试包。
18. 完成跨平台验证。

---

## 三十二、验收标准

1. Web UI 不具备 Node/Electron。
2. 不使用 `file://`。
3. CSP 严格。
4. Bridge 有 Session 和 Schema。
5. 只能执行声明 Action。
6. 网络、文件、剪贴板受控。
7. 不持久化业务状态到浏览器 Storage。
8. 导航和新窗口被拦截。
9. UI Crash 不影响宿主。
10. 更新代际隔离。
11. 安全测试通过。
12. 可进入第 44 步前端扩展槽。

---

## 三十三、执行约束

> Restricted Web UI 允许复杂前端代码，但其权限必须比插件 Main Runtime更小，且不能借助浏览器或 Electron 能力绕过 Host Bridge。

禁止：

-Node Integration；
-contextIsolation=false；
-sandbox=false；
-file://；
-任意 preload；
-远程代码；
-直接 fetch；
-直接 IPC；
-默认 Cookie/Service Worker；
-共享 Session；
-跨 Extension Origin。
