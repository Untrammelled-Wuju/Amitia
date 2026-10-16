# Workbench Batch 05 — Agent 浏览器临时控制权（实现记录）

基线：`U-Ai_Workbench_Batch04_累计完整源码包.zip`。本文件对应本轮真实源码变更，不代表整个 Workbench 已完成。

## 实际功能

1. 后端 `control` API：在 `POST /api/workbench/browser/:conversationId/:viewId/control` 提供 `acquire`、`renew` 和 `release`。必须先使用有效且未撤销的 `browserRef` 绑定 Agent 浏览器观察标签；控制权属于当前认证设备/会话/对话/标签，不能跨标签续租或释放。
2. 默认只读。控制权采用 45 秒的内存租约；只能被拥有 `workbench.browser.control` 权限且属于本地可信 UI 或受信设备的主体申请。该权限仅加入 OwnerDevicePermissions，默认标准设备权限不会自动获得。
3. 同一个 Agent 标签最多一个观察者持有输入租约。控制中时，其他观察者的写操作仍由后端拒绝；同一个观察者可手动释放/续租。`DELETE` 观察标签只解除绑定，不销毁 Agent 原生 session。
4. Agent BrowserAgent 在浏览器导航、点击、输入、选择和滚动工具的调用边界等待手动控制结束。另有 in-flight 标志：当 Agent 已经执行一个变更型操作时拒绝同时颁发新租约；手动接管不会强行取消已经发出的操作。
5. `RevokeAgentTab` 立即撤销绑定的控制权。租约超时、观察权限失效和标签关闭后恢复只读；浏览器实例独立会话无需临时接管。
6. Web/Electron 工作区与 Flutter Chromium 工作区新增手动申请、释放、续租和可控/只读 UI；不可控时禁止点击、滚动、输入和导航。
7. 浏览器截图响应最大 6 MiB、DOM 观察最大 1 MiB（UTF-8 完整截断，返回 truncated）；为 DOM 响应增加 no-store。
8. 新增租约独占、到期恢复、正在执行的 Agent 操作互斥、权限校验、跨设备观察者隔离及观察引用撤销测试。

## 重要限制

- 此次实现限定 **同一个 Go 进程中的 BrowserAgent / BrowserProvider**。跨设备 Device Mesh 转发、授权委托、断线恢复和云端共享尚未做完。移动端若访问不到实际运行 Chromium 的设备 Runtime，不能凭界面按钮获得远程控制。
- BrowserAgent 之外的独立直接浏览器工具调用，尚未全部纳入同一临界区；本轮仅保证当前 BrowserAgent 通过 browserAgentExecute 路径执行的相关动作在工具边界协调。不可宣传为全平台排它控制。
- 临时控制权限不等于网页支付、凭证或敏感系统的高风险操作授权；上线前仍需要服务器侧权限审计、限流、DNS/网络沙箱和 Device Mesh 委托证明。
- Agent 原生会话及观察引用仍驻留内存，不支持进程重启后的自动恢复；长期会话重连、跨设备控制及正式 Chromium/Flutter 真机测试仍待后续开发。
- Go SDK 需要 1.26.1，当前 Linux Go 1.23.2 无法运行项目完整编译；Flutter SDK 当前不可用。Go 语法、逻辑定向测试与 TS 脚本检查不能代替完整构建。

## 变更文件

- `backend/internal/auth/permissions.go`
- `backend/internal/agent/browser_agent.go`
- `backend/internal/agent/browser_agent_control_test.go`
- `backend/internal/workbenchbrowser/agent_bridge.go`
- `backend/internal/workbenchbrowser/handler.go`
- `backend/internal/workbenchbrowser/handler_test.go`
- `backend/internal/workbenchbrowser/control_lease.go`
- `backend/internal/workbenchbrowser/control_lease_test.go`
- `backend/internal/workbenchbrowser/control_handler_test.go`
- `front/src/components/workbench/WorkbenchBrowserView.vue`
- `mobile_app/lib/features/workbench/presentation/pages/workbench_remote_browser_view.dart`
- `docs/workbench/BATCH05_IMPLEMENTATION.md`
