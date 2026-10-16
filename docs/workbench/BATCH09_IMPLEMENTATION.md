# Batch 09 — 跨设备浏览器错误协议、权限收口与失效恢复

## 基线

本轮以 `U-Ai_Workbench_Batch08_累计完整源码包.zip` 为基线，保持既有 Device Mesh、Workbench BrowserProvider 与双端工作区体系，不建立新的 Chromium Runtime，不开放跨设备写操作。

## 服务端

- 为 Device Mesh 的浏览器只读 RPC 增加应用级结构化失败结果，包含 `errorCode`、`error`、`retryable`。固定类别为 `invalid_request`、`capability_revoked`、`runtime_unavailable`、`unsupported_operation` 和 `resource_exceeded`。这类失败作为成功送达的 RPC 应用结果返回，然后由 Cloud 网关转换成 HTTP 状态，不把设备内部错误直接回传客户端。
- Cloud 网关返回 410 表示观察许可失效或真实 Agent 页面已结束；400 表示请求无效；501 表示能力不支持；503 表示暂时不可用；502 表示资源异常或无效设备响应。权限不足仍为 403，离线仍为 503，权限校验仍先于发起 Device Mesh 调用。
- 当 BrowserProvider 的 Tab 或 Session 已不可用且属于终态时，不将其当成网络波动无限重试；运行时启动、短期读取故障按暂时不可用处理。
- 在读取 DOM/截图后复核观察引用；引用在执行过程中撤销时不返回已经读取的数据。
- 清理闲置的本地观察标签时，仅释放该标签持有的手动控制租约，不撤销同一 Agent 浏览器标签上其他观察者的租约。
- 限制设备端错误消息长度，继续对远程截图施加 2 MiB 限制；远程链路返回 `Cache-Control: no-store`。

## Web/Electron

- 引入纯函数错误分类器，将永久错误与暂时性离线分别处理，并添加定向测试。
- 远程观察权限失效后停止轮询、清空截图和 DOM，避免继续展示撤权前缓存的界面；设备离线和超时仍使用有界退避重试。
- 浏览器 DOM 读取路径也遵循同一错误策略，不仅依赖截图和状态轮询触发。

## Flutter Android/iOS

- 通过现有 BackendTransportError / Dio 响应解析 HTTP 状态及安全错误字段。永久权限错误停止重连、清除截图与 DOM；暂时故障沿用有界退避。
- 既保留显式退出的状态，也保留明确的手动重新连接入口。不额外持久化观察令牌到新的系统位置。
- 新增错误分类器单元测试，便于在 Flutter SDK 可用时执行。

## 验证与尚未完成

- 独立 Go GOPATH 镜像执行 BrowserRef / 控制租约 / RemoteRead 单元测试，并启用 `go test -race`；这不是完整 Go 1.26.1 项目构建。
- 修改的 Go 源码通过 `gofmt`、`go/parser`，Web TypeScript/Vue 脚本解析和 12 项分类逻辑断言通过。
- `Flutter test`、完整 Vue 类型检查、Electron 启动、双设备真实 Chromium 联调尚未完成；需要对应工具链与在线设备。
- 跨设备仍为只读观察；用户控制权不能跨设备转发；Runtime 重启不会自动恢复已经消失的 Agent Chrome 实例。
