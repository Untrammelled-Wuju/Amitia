# Workbench 第18轮：模块化实时终端与 Windows ConPTY

## 本轮交付

- `workbenchterminal/stream_ticket.go`：短期随机连接凭据，绑定可信调用主体、指定终端会话与浏览器 Origin；每张凭据只消费一次，过期 20 秒；128 张待使用凭据上限。
- `workbenchterminal/stream.go`：受一次性凭据保护的双向 WebSocket；JSON snapshot/input/resize；Base64 字节安全传输、消息大小和寿命限制、Ping/Pong、游标断线续接。升级入口独立于 HTTP Authorization 头，票据只能经受认证的 POST 签发。
- `workbenchterminal/stream_notify.go`：按会话广播更新，多个订阅者能够同时收到唤醒；没有数据时不再周期性拉取 HTTP 输出。
- `workbenchterminal/process_conpty_windows.go`：Windows 10+ 上通过 ConPTY 创建原生伪控制台，支持调整行列、读取进程退出码；无法找到 ConPTY 系统 API 时才回退已有管道适配。平台差异保留独立模块，其他平台不依赖 Windows API。
- Web/Electron `useWorkbenchTerminalStream.ts`：先经原认证链申请票据，再用浏览器 WebSocket 连接 Runtime。断线按游标重新获取输出，指数退避重连，连接失败回退原 HTTP 轮询；双向输入、大小调整与关闭安全处理仍使用统一会话身份。
- `WorkbenchTerminalView.vue`：实时/兼容模式指示和大小调整。仍保留非 xterm 的文本输出 UI，不宣称支持完整 VT 序列渲染。
- 新增 `terminal-stream-v1.schema.json`；更新 `terminal-session-v1.schema.json` 纳入 `conpty` 模式，保持既有 `pty`/`pipe` 兼容。

## 安全及运行限制

- Shell 是本机可信用户的真实操作系统进程，不是执行沙箱，Workspace 根目录并不是 OS 级进程访问边界。
- Cloud Core 不注册本机终端执行接口。WebSocket 升级路径虽然无需浏览器无法附加的自定义认证头，但必须出示受认证 POST 签发的短期、一次性、Origin 绑定票据。
- WebSocket 单连接最大生命周期 4 分钟，超过后由前端重连；服务端会话仍由后台闲置回收机制保护。
- Windows ConPTY 已交叉编译验证，尚未在 Windows 实机验证 CreatePseudoConsole、交互输入、大小调整和关闭进程的完整运行结果。
- Windows 旧系统的 `pipe` 兼容模式不支持真正的控制台大小调整和完整交互；Web UI 尚未使用 xterm.js/完整 ANSI/VT100 解析器。
- 当前 Linux 环境缺乏项目要求的 Go 1.26.1、完整前端依赖及 Flutter SDK。独立 Go race 测试、Windows 交叉编译和 TypeScript 脚本解析不能代替完整 Go 服务、Electron、Android Release 真机联调。
- 实时 WebSocket 当前只针对本机可信 Runtime 终端；不开放跨设备未授权远程 PTY。

## 后续验收

在有完整环境时，必须先完成真实 Gin 服务 HTTP 认证 → 签票 → 同 Origin WebSocket 连接、反向授权与 CSRF 校验、断线续传，随后在 Windows 11 真机完成 ConPTY/Pipe 模式差异回归、进程树资源清理，再引入成熟终端渲染器与任务生命周期集成。
