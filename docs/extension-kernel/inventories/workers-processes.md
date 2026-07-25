# Worker、goroutine、Timer 与子进程清单

> 审计日期: 2026-07-25

## 一、Plugin Manager Workers

### 1. PluginManager.Start() → 4 个 goroutine worker

| Worker | goroutine 函数 | 用途 | 队列来源 | 停止条件 |
|---|---|---|---|---|
| afterReplyWorker | `func (m *PluginManager) afterReplyWorker(ctx)` | 处理 AfterReply 队列中的 Hook 调用 | `afterReplyQ chan` | `ctx.Done()` |
| eventIngressWorker | `func (m *PluginManager) eventIngressWorker(ctx)` | 从 `eventIngress` 读取事件并持久化到 DB | `eventIngress chan` | `ctx.Done()` |
| eventWorker | `func (m *PluginManager) eventWorker(ctx)` | 从 DB 读取待处理事件并分发 | DB polling (Ticker) | `ctx.Done()` |
| scheduleWorker | `func (m *PluginManager) scheduleWorker(ctx)` | 检查到期 Schedule 并触发 | DB polling (Ticker) | `ctx.Done()` |

| 字段 | 内容 |
|---|---|
| 所有者 | `PluginManager` |
| 启动条件 | `PluginManager.Start(ctx)` |
| 停止条件 | `PluginManager.Stop(ctx)` → cancel root context |
| 队列容量 | afterReplyQ: 无界 channel；eventIngress: 无界 channel |
| 重试策略 | Event Delivery 有重试（`UpdateDelivery` 记录） |
| 异常处理 | goroutine panic 不会传播；delivery 失败标记为 dead_letter |
| 应用崩溃后恢复 | Event 和 Schedule 通过 DB 恢复；afterReplyQ 丢失（内存队列） |
| 禁用扩展后 | Schedule Worker 不检查 enabled 状态 ← **问题** |
| 卸载扩展后 | 关联的 Event 和 Schedule 不自动清理 ← **问题** |
| 目标归属 | Runtime Supervisor |

### 2. Plugin Circuit Breaker

| 字段 | 内容 |
|---|---|
| 资源类型 | 熔断器 |
| 结构 | `extension/plugin_circuit.go` |
| 状态 | closed → open（连续失败达阈值） → half_open（超时后尝试） |
| 持久化 | 否（内存） |
| 应用崩溃后 | 丢失熔断状态，所有插件重新尝试 |
| 目标归属 | Runtime Supervisor |

---

## 二、MCP Manager Workers

### 3. MCP Reconnect goroutine

| 字段 | 内容 |
|---|---|
| 资源类型 | goroutine（每个断线 Server 一个） |
| 触发条件 | `scheduleReconnect()`（连接失败或连接断开） |
| 重试策略 | 6 次退避重试：1s, 2s, 5s, 10s, 30s, 60s |
| 停止条件 | 重连成功 / 达到最大次数 / Server 被禁用 / Manager 关闭 |
| 防重入 | `reconnecting map` 确保同一 Server 只有一个重连 goroutine |
| 异常处理 | 6 次失败后标记 `status: "degraded"`, `error: "MCP_RECONNECT_LIMIT_REACHED"` |
| 当前问题 | Server 禁用后重连 goroutine 通过 `GetServer` 检查 enabled 状态后退出 |
| 目标归属 | Runtime Supervisor |

### 4. MCP Connection Done 监听 goroutine

| 字段 | 内容 |
|---|---|
| 资源类型 | goroutine（每个连接一个） |
| 触发条件 | `connect()` 成功后 `go func() { <-connection.Done() }` |
| 用途 | 检测连接断开，触发 `scheduleReconnect` |
| 停止条件 | 连接正常关闭（StateStopping）时不重连 |
| 目标归属 | Runtime Supervisor |

### 5. MCP Ready Handler 通知 goroutine

| 字段 | 内容 |
|---|---|
| 资源类型 | goroutine（每个 handler 一个） |
| 触发条件 | `connect()` 成功后 `go handler(m.root, server.ID)` |
| 用途 | 通知 Discovery Service / Skill Runtime / Host Service 执行初始化 |
| 目标归属 | Runtime Supervisor |

---

## 三、MCP Transport 子进程

### 6. MCP stdio 子进程

| 字段 | 内容 |
|---|---|
| 资源类型 | 操作系统子进程 |
| 创建位置 | `transport.NewStdio()` → `Start()` → `exec.Command` |
| 进程标识 | 不记录 PID；通过 `cmd.Process` 管理 |
| 创建者 | `DefaultFactory.Build()` → `Manager.connect()` |
| 终止者 | `StdioTransport.Close()` → `cmd.Process.Kill()` |
| 异常退出检测 | `cmd.Wait()` 返回错误 → `connection.Done()` → `scheduleReconnect` |
| 重启后清理 | **无孤儿进程清理**（进程随父进程退出自动终止，但在非正常退出时可能残留） |
| 升级/回滚时 | 断开连接 → 重新连接（重新启动子进程） |
| 卸载时 | `DeleteServer()` → `Disconnect()` → 终止子进程 |
| 多实例冲突 | 无检测机制 |
| 环境变量注入 | 仅 `stdio_env` 认证类型 |
| 安全性 | `process_windows.go` 包含平台特定安全控制 |
| 目标归属 | Runtime Supervisor |

### 7. MCP Streamable HTTP 连接

| 字段 | 内容 |
|---|---|
| 资源类型 | HTTP 长连接 |
| 创建位置 | `transport.NewStreamableHTTP()` → `Start()` |
| 端点策略 | `EndpointPolicy{AllowLoopback: true, AllowPrivate: 需 capability, MaxRedirects: 3}` |
| 超时 | 连接超时 30s |
| 终止者 | `StreamableHTTP.Close()` |
| 目标归属 | Runtime Supervisor |

---

## 四、应用层 Worker（扩展系统间接相关）

### 8. Delivery Worker

| 字段 | 内容 |
|---|---|
| 资源类型 | goroutine worker |
| 结构 | `delivery.Worker` |
| 用途 | 消息投递（微信、QQ、Web），Workflow Notification 通过此 Worker 发送 |
| 目标归属 | 独立服务（不归 Extension Kernel） |

### 9. Outbox Worker

| 字段 | 内容 |
|---|---|
| 资源类型 | goroutine worker |
| 结构 | `newoutbox.Worker` |
| 用途 | 异步事件发布（chat 后处理 pipeline） |
| 目标归属 | 独立服务（不归 Extension Kernel） |

### 10. DataLifecycle Coordinator

| 字段 | 内容 |
|---|---|
| 资源类型 | 后台清理协调器 |
| 结构 | `mindruntime.DataLifecycleCoordinator` |
| 用途 | 定期清理过期数据 |
| 目标归属 | 独立服务（不归 Extension Kernel） |

---

## 五、启动与关闭顺序

### 启动顺序（services.go `NewAppServices()`）

1. `extension.NewRuntime()` → AgentSkills.Restore(), PluginManager.Start(), Workshop.Restore(), Packages.Restore()
2. `mcpmanager.New()` + `connectionManager.Restore()` → 恢复所有启用 Server 连接
3. `skillRuntime.RegisterAll()` → 注册已有 MCP Tool 到 Extension Registry

### 关闭顺序

1. `PluginManager.Stop()` → cancel root context → 4 个 worker 退出
2. `Manager.Close()` → cancel root context → 断开所有 MCP 连接
3. 其他服务 `Close()`

---

## 六、应用崩溃恢复能力

| 资源 | 是否持久化 | 崩溃后恢复 |
|---|---|---|
| Plugin afterReplyQ | 否（内存） | 丢失 |
| Plugin Events | 是（DB） | 从 DB 恢复未处理事件 |
| Plugin Schedules | 是（DB） | 从 DB 恢复未到期 Schedule |
| Plugin Circuit Breaker | 否（内存） | 重置为 closed |
| MCP 连接状态 | 是（DB） | Manager.Restore() 重建连接 |
| MCP Discovery 缓存 | 是（DB） | 从 DB 恢复 |
| Agent Skill Catalog | 否（内存） | 重新构建 |
| Agent Skill Artifact 缓存 | 否（内存） | 重新从 DB 加载 |
