# Workbench 第 19 轮：双端终端、可读输出与会话重新挂接

## 代码分层

- 后端 `workbenchterminal/session_list.go`：只列出 Manager 内属于当前 owner 的终端，不读取或暴露其他主体会话；按创建时间倒序排序，不通过列举行为延长闲置会话寿命。
- 后端 `workbenchterminal/session_list_handler.go`：经过与终端读写相同的可信本机用户、认证会话与 conversationId 校验，提供 `GET /api/workbench/terminal/sessions?conversationId=...`。
- Web `terminalTextBuffer.ts`：流式处理 ANSI CSI/OSC、回车、退格、行清除和截断。该模块属于有限的纯文本展示，不是完整 VT100/xterm 模拟器。
- Web `WorkbenchTerminalView.vue`：在输出区直接键盘输入方向键、Ctrl 组合键；新增已有终端手动恢复；组件退出不再自动关闭后端进程，用户点击结束、关闭标签或闲置到期时才释放。
- Flutter `workbench_terminal_client.dart`：隔离 API 合同、输入大小、Base64、Cursor 与终端身份校验。
- Flutter `workbench_terminal_output.dart`：保留跨帧 UTF-8 字符，限制文本长度，过滤常见 ANSI 控制码。
- Flutter `workbench_terminal_view.dart`：本机工作区筛选、启动、命令输入、输出、退出码、显式结束、后台轮询暂停、手动恢复当前会话。
- Flutter `workbench_page.dart`：终端标签与 IndexedStack 页面集成；`workbench_mobile_events.dart` 支持 AI 后台打开终端标签，但不会自动执行命令。
- 新增独立 Go/TS/Dart 测试和 `terminal-session-list-v1.schema.json`。

## 边界

- 该终端是具有本机用户权限的 Shell，**不是**进程沙盒。仅注册的可信本机 Runtime 允许创建；跨设备/Cloud Core 不提供执行能力。
- 终端持久化范围是服务进程的内存；前端刷新或短时间内重新挂接可以恢复已有进程，后端重启后不会重建原有进程。
- 主体校验与原有终端接口共用 `terminalOwner`；不同登录会话、对话、设备、Space 均无法列举或操作其他终端。
- Flutter 在 Android 原生环境可能没有可写的本地 Go Runtime 工作区；此时启动按服务端规则拒绝，而不是自动路由到不受信任的 Cloud Core。
- Flutter 展示采用简化文本输出，不支持完整全屏 TUI；Web 输出同样没有加载 xterm.js，因此不能宣称完整光标控制和 ANSI 兼容。
- 尚未完成完整 Go 1.26.1 工程编译、Flutter/Dart SDK 分析、移动设备真机以及 WebSocket 端到端认证联调。
