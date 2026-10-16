# Workbench Batch03 — 实施与验证记录

## 开发基线

基于 Batch02 累积源码继续增量实施。所有改变写入项目实际目录，替换包内包含完整文件而非局部补丁。本轮没有修改原始上传文件本身，也没有推送远端仓库。

## 新增服务端 API

路由挂载在现有认证 `/api` 分组，且只允许 Device Runtime 提供（Cloud Core 不提供浏览器实例）：

- `GET /api/workbench/browser/capabilities` 查询后端截图能力。
- `POST /api/workbench/browser/open` 创建或恢复按对话、设备身份及工作台标签隔离的 Chromium 会话。
- `GET /api/workbench/browser/:conversationId/:viewId` 获取标签当前状态。
- `POST /api/workbench/browser/:conversationId/:viewId/navigate` 导航 HTTP(S) URL。
- `POST /api/workbench/browser/:conversationId/:viewId/action` 前进、后退、刷新和停止。
- `POST /api/workbench/browser/:conversationId/:viewId/input` 有界的屏幕点击、滚动和文本输入。
- `GET /api/workbench/browser/:conversationId/:viewId/screenshot` 返回通过认证的 JPEG 截图流，禁止客户端直接读取临时文件路径。
- `GET /api/workbench/browser/:conversationId/:viewId/dom` 返回有界 DOM 快照。
- `DELETE /api/workbench/browser/:conversationId/:viewId` 释放标签对应的会话。

服务端使用现有 `BrowserProvider`/CDP 引擎而不是重复启动独立 Chromium 逻辑。每个会话以 `SpaceID/PrincipalType/DeviceID/SessionID/ConversationID/ViewID` 作为作用域。单个对话限制 8 个、单个 Runtime 限制 128 个会话。异常退出后重新打开标签时会检测失效会话并创建替代实例。

## 前端与移动端

- Web/Electron Workbench 浏览器标签新增真实 Chromium 截图、导航、DOM、点击、滚动、文字输入 UI，以及无法启动时的明确轻量预览回退。
- 关闭非固定浏览器标签会请求释放服务端会话。
- Git 提交历史支持展开父提交 Diff 与统计。
- Flutter 聊天头部入口支持先打开 80% 高度抽屉、再切换全屏。
- Flutter 工作区状态异步恢复增加版本序号，防止旧读结果覆盖新状态。
- Flutter `AgentUIEvent(workbench.open)` 能新增或复用文件/Git/Diff/网页标签，操作去重、后台打开且不抢占正在阅读的标签。
- Electron/Web 与 Flutter 的浏览器 API 都显式路由到设备 Runtime，避免云端环境误请求 Cloud Core。

## 未实现/未通过验收的范围

- 此轮浏览器 UI 连接的是当前设备的独立 Chromium 会话，并不等价于直接接管某个已存在 Agent Browser 会话；Agent 同页查看与共享控制权仍需实现。
- Flutter 工作区使用系统 WebView 浏览网页，没有与后端 Chromium 同页同步；远程跨设备设备授权与资源代理未完成。
- 浏览器会话映射位于服务进程内存；服务重启时需要重新打开标签；生产级持久化和生命周期回收仍需设计。
- 由于构建环境缺少 Go 1.26.1 工具链、Flutter SDK 和前端 node_modules，未运行 Go 单元测试、Flutter analyze、Flutter Release、Electron 实际启动和浏览器/CDP 真机联调。此处不得宣称生产可用。
- 后端运行时还未自动产出 `workbench.open` 事件；前端与手机端只是消费端已接入。

## 后续建议

先补浏览器同页复用与 Agent 事件发布，随后覆盖 Flutter 本机/远程浏览器观察、跨设备授权、服务端持久化、终端 PTY 与多 Agent 工作区联动。必须在完整工具链可用的设备上重新构建和回归。
