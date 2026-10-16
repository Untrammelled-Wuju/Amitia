# Workbench 第 55 轮：生产级 HTTP 边界与浏览器执行期限

## 变更目标

本轮以前 54 轮累计源码为基线，对任务、浏览器、终端这三类带身份和运行资源的接口进行安全收口。优先采用共享、可独立测试的安全边界，而不是在每个处理函数中复制响应头及请求体限制。

## 已修改的源码

1. `backend/internal/workbenchhttp/` 增加标准库实现的 HTTP 安全模块。任何响应都设置 `Cache-Control: no-store, private`、`Pragma: no-cache` 和 `X-Content-Type-Options: nosniff`。对含请求体的写操作，同时按声明的 Content-Length 和实际流式读入量限制内容长度，不能通过分块传输绕过。浏览器上限为 96 KiB；终端上限为 16 KiB。
2. `workbenchbrowser.Handler.RegisterRoutes`、`workbenchterminal.Handler.RegisterRoutes` 使用分组中间件，在身份校验失败、业务失败和成功响应时均施加缓存禁止策略；对于已声明超限的请求直接返回 HTTP 413。超过读取上限的未知长度请求将由 `http.MaxBytesReader` 阻断，底层 JSON 绑定仍可能返回 HTTP 400，这是有界拒绝而非成功执行。
3. 对直接注册在 Gin Engine 上的终端 WebSocket `/api/workbench/terminal/stream`，在申请升级之前同样附加不可缓存与 MIME 嗅探防护响应头。一次性票据和消息帧的既有权限、大小、有效期策略保持不变。
4. `/workbench/tasks` 和 `/workbench/tasks/checkpoints` 在进入处理函数前设置一致的保护响应头，包含所有 4xx/5xx 路径。既有认证及对话作用域校验不改变。
5. 浏览器 `state`、`navigate`、`action`、`screenshot`、`dom` 和 `input` 的 Provider 调用补充有界请求上下文，客户端断开连接和执行期限均可触发取消。执行器自身是否能可靠响应取消需在真实 Chromium 中进一步验证。
6. Browser 观察标签创建前主动清理过期观察引用，避免仅通过观察接口创建视图、长时间没有独立 open 请求时，过期观察者占满运行时标签配额。
7. 新增 6 个共享 HTTP 安全专项 Go race 测试，覆盖响应头、请求声明长度、未知长度流式超限、边界值、只读请求和无效上限；这些测试已接入既有研发验证门禁。相关 Go 语法检查包含新增模块。

## 本轮验证与仍需运行的生产级验收

- 在本轮 Linux 工作树上直接执行 `GO111MODULE=off go test -race -count=1 ./backend/internal/workbenchhttp`：6 个专项测试通过。
- 执行隔离浏览器租约与远端观察源代码 Go race 测试、原有终端票据测试、Workbench 相关源码语法门禁，以及集成门禁脚本检查。
- 这些测试可以证明安全模块的大小限制、响应头行为、源代码语法和已有隔离逻辑回归；不能替代真正 Gin HTTP 路由测试或完整后端构建。
- 完整 Go 1.26.1 服务启动、真实授权 HTTP 及 WebSocket、Chromium CDP 取消响应、Windows ConPTY、Electron 和 Flutter Release、Device Mesh 真机与压力/恢复/迁移/发布验收仍未完成，保持未通过状态。

## 进度说明

本轮处于 97%→100% 的安全和生产验收收口阶段。研发范围估算仍与正式生产验收独立，不能用这些定向验证证明项目已达到上线条件。未经过实际测试的能力不得标为已验收。
