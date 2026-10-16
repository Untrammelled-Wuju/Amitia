# Batch 20 · Workbench 终端画面与 Worktree 入口

## 本轮交付

本轮直接叠加第 19 轮累计源码，保持既有终端 HTTP、WebSocket、ConPTY 与 Workspace 权限接口不变。新增 `terminalVtScreen.ts` 解析 ANSI/VT 流，`WorkbenchTerminalScreen.vue` 独立负责安全显示和键盘/粘贴事件，`WorkbenchTerminalView.vue` 仅负责会话、增量字节、输入队列和尺寸同步。Git Worktree 页面通过已有 Workspace URI 一键启动独立终端标签，不将终端直接绑定到父仓库。

## 支持的画面能力

- 常见 CSI 光标移动、定位、行/屏幕清除、滚动区域、字符插入/删除。
- SGR 标准色、高亮色、256 色与真彩色；输出以 Vue 文本节点渲染，不注入终端输出 HTML。
- 私有模式的备用屏幕切换、光标显示/隐藏和有限历史滚动（最多 600 行）。
- 可增量解析跨帧的控制序列，并保留 HTTP 与 WebSocket 的输出游标续接。
- 终端容器自动测算行列，向已有终端 Runtime 请求调整 PTY/ConPTY 大小。
- 实时输入不以整个 HTTP `busy` 状态阻塞；WebSocket 不可用时按顺序串行发送 HTTP 输入。
- 终端会话仍以现有认证身份、对话和 Workspace 进行隔离，禁止跨设备任意启动。

## 尚未完成

- 这一实现是内置 VT 兼容解析器，不是完整 xterm.js 引擎。未完全实现 Unicode grapheme、DECSLRM、复杂 ANSI 私有模式和全屏 TUI 的全部转义能力；复杂应用需真实 xterm.js 接入与真机验收。
- npm 仓库在当前环境 DNS 不可用，因此本轮未增加不能完整锁定的 xterm 依赖。现有 pnpm/npm 锁文件保持可重现状态。
- 当前 Linux 容器缺少前端 node_modules、Go 1.26.1、Flutter SDK 和 Windows 原生环境，未执行整个产品的打包、安装和终端 UI 实际交互验收。
- Git Worktree 的“任务终端”基于相同 Workspace URI 启动，但这不是 Agent 调度器拥有终端会话或终端输出成为可信任务验证证据。两者仍需服务端身份确认和审计绑定。
- 未获得任意云端终端执行权限；本地 Shell 仍非沙盒。

## 可运行的定向验证

使用当前 Linux Node 二进制执行 `verify-batch20-terminal.cjs`，对变更 TypeScript / Vue 脚本做转译语法检查，并运行 VT 解码、备用屏幕、ANSI 安全、光标、颜色及有界滚屏的隔离行为测试。源码还包含可由项目正常 Vitest 环境运行的 `terminalVtScreen.test.ts`。
