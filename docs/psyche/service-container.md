# 核心服务实例装配基线

对应实施计划：V3.1 第6步。

## 目标

消除 Chat、Memory、Profile、Episodic、WorldBook、Companion 等核心服务在启动入口、路由和系统接口中重复构造造成的状态分裂。

## 当前实现

新增 `backend/cmd/server/services.go`，统一构造 `AppServices`：

| 字段 | 实例 |
|---|---|
| `Chat` | 主聊天服务，持有共享 Memory/Profile/Episodic/WorldBook/Vision/Compressor |
| `Memory` | 记忆服务 |
| `Profile` | 用户画像服务 |
| `Episodic` | 情景记忆服务 |
| `WorldBook` | 世界书服务 |
| `Companion` | 生活与主动消息服务 |
| `Vision` | 图片配置服务 |
| `Graph` | 图谱服务 |

`backend/cmd/server/main.go` 构造一次 `AppServices`，随后将同一组实例传给：

- `setupRouter`
- 主动消息 cron
- 工具保存回调
- 初始频道会话创建
- 消息计数重算
- 当日主动任务生成

以下路由不再内部重复构造核心服务：

- `memory.RegisterMemoryRouter`
- `profile.RegisterProfileRouter`
- `episodic.RegisterEpisodicRouter`
- `worldbook.RegisterWorldBookRouter`
- `agent.RegisterAgentRouter`
- `system.RegisterSystemRouter`
- `companion.RegisterCompanionRouter`

## 验收证据

执行：

```powershell
pwsh -NoLogo -NoProfile -Command 'go test ./cmd/server ./internal/migration ./internal/psyche_testdata'
```

通过条件：

- `cmd/server` 编译通过。
- `TestNewAppServicesBuildsCoreServicesOnce` 证明核心实例由同一容器创建。
- 检索 `chat.NewService`、`memory.NewService`、`profile.NewService`、`episodic.NewService`、`worldbook.NewService`、`companion.NewService`、`NewCompressor`，核心启动路径只剩 `cmd/server/services.go`。

