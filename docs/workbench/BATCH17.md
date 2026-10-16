# Workbench 第 17 轮：分模块交互终端

## 交付边界

独立 `backend/internal/workbenchterminal` 模块负责进程启动、输出缓冲、会话、认证 HTTP 适配；没有将整个终端实现放进 `router.go`、大体积 Workspace Handler 或单个 Go 文件。

| 模块文件 | 职责 |
|---|---|
| `buffer.go` | 最大 256 KiB 输出缓冲、64 KiB 分片、增量游标和截断识别 |
| `process.go` | 进程接口、Shell 选择、退出码适配 |
| `process_unix.go` | Linux/macOS 真正的 PTY、resize、进程关闭 |
| `process_windows.go` | Windows CMD 管道兼容；不是 ConPTY，不支持完整终端控制序列 |
| `process_other.go` | 其他平台显式不支持 |
| `session.go` | 输入串行化、退出观察、流处理 |
| `manager.go` | 会话配额、闲置回收、并发隔离、生命周期 |
| `handler.go` | 可信本机 UI 身份校验、Workspace 根校验、REST 路由 |
| `front/src/components/workbench/useWorkbenchTerminal.ts` | 终端 API 与 Base64 协议适配 |
| `front/src/components/workbench/WorkbenchTerminalView.vue` | 工作区独立终端 UI、轮询、状态及关闭 |

## 接口

在认证后的 `/api/workbench/terminal` 命名空间下：

- `POST /open`：传 `conversationId`、`workspaceUri`（必须是已注册且可写的本机挂载根 URI）、`rows`、`cols`，返回随机 40 位 Session ID。
- `GET /:id?conversationId=...&cursor=...`：按字节增量返回 Base64 输出、游标、进程状态和截断标志。
- `POST /:id/input`：Base64 编码输入（最多 4096 字节），写到进程 stdin/PTY。
- `POST /:id/resize`：调整 PTY 大小；Windows 管道模式明确拒绝。
- `DELETE /:id?conversationId=...`：结束进程并注销会话。

## 安全策略

1. 后端校验认证 Actor 为 `PrincipalLocalUI` 且 `IsLocalTrusted`，Cloud Core 不注册执行接口。设备端仍通过本机认证，不支持 Remote/SAF/Isolated 挂载直接执行。
2. 会话隔离绑定 Space、设备、认证 Session 和对话；每个会话 ID 为 160 bit 加密安全随机值。
3. 每个 Owner 上限 6 个会话，每个 Runtime 总上限 48；25 分钟无 API 活动或 4 小时绝对期限后关闭。
4. 终端执行使用本机操作系统用户权限。这是**可信本地执行接口**，不是安全沙箱；需要隔离未受信任代码时仍需专门容器/系统沙箱。
5. HTTP 轮询返回 Base64 二进制，前端通过 TextDecoder 做 UTF-8 流式解码；终端输出按纯文本显示，不直接作为 HTML 注入。
6. 会话仅保留在内存；服务重启后需重建会话。关闭标签时请求释放，断线后的孤儿进程通过定时回收关闭。

## 不包含 / 尚未验收

- Windows ConPTY、xterm.js ANSI 完整渲染、WebSocket 低延迟双向流、子进程树跨平台清理、会话跨进程重连。
- Flutter 端原生终端视图及跨设备远程命令执行。
- 完整 Go 1.26.1 编译、Windows 实机运行、桌面端启动、Android Release、真实 Agent Coding 任务端到端测试。
