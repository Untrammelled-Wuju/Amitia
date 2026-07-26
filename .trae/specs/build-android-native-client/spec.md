# Amitia Android 原生客户端构建 Spec（第一阶段）

> 来源：`AndroidAPP/stage.md`（共三十二节）+ 仓库实际代码扫描结果
> change-id：`build-android-native-client`
> 阶段：第一阶段（聚焦当前 Amitia 已实现能力的迁移 + 内嵌 Linux Runtime）

---

## Why

Amitia 当前只有 Web（Vue3）和 Electron（Windows）两个客户端，无法在 Android 设备上提供陪伴角色与本地 Agent Runtime 体验。本阶段目标是在仓库内构建一个可编译、可安装、可运行的 Android 原生客户端，通过内嵌无 Root Linux 用户空间运行现有 Go 后端 + SQLite + Qdrant + SurrealDB，复用现有业务逻辑而非重写一套 Kotlin 后端，为后续 Computer Use、MCP、Skill、AmitiaX 扩展预留正确架构。

## What Changes

### 新增（ADDED）

- **Android 原生工程**：在仓库根目录创建 `android/`，使用 Kotlin + Jetpack Compose + Gradle Kotlin DSL + Hilt + Material 3。
- **内嵌 Linux Runtime**：在 Android 应用私有目录建立无 Root Linux 用户空间（优先 PRoot/proot-rs 路线），管理 RootFS、进程、健康检查、状态机。
- **Go 后端 Linux ARM64 构建**：`GOOS=linux GOARCH=arm64` 交叉编译 Amitia Go 后端，复用现有业务代码。
- **Qdrant / SurrealDB Linux ARM64 二进制**：获取或构建 ARM64 版本，由 Runtime 管理启动与持久化。
- **本地/远程双运行模式**：通过统一的 `RuntimeEndpoint` / `ServerEndpoint` 切换 `127.0.0.1` 内嵌后端与远程 HTTPS 服务，共用同一套 Repository / 领域模型 / UI。
- **Android 原生 UI**：首页 / 对话 / 角色 / 能力 / 设置 五大导航；首次启动引导；Runtime 管理页面；深色优先设计系统。
- **Native Capability Bridge**：受控桥接 Android 系统能力（文件/相机/麦克风/通知/剪贴板/分享等）供 Linux 后端未来调用。
- **Room 缓存层 / DataStore 偏好 / Keystore 凭据**：仅作 UI 缓存与偏好，核心业务数据仍由 Go 后端 SQLite 管理。
- **前台服务 + 通知 + 运行策略**（AlwaysOn / OnDemand / RemoteOnly）。
- **文档体系**：`docs/android/01~13 + third-party-licenses.md` 共 14 份。

### 修改（MODIFIED）

- **Go 后端平台抽象**：将 `backend/cmd/server/main.go` 中的 `cmd /c netstat`、`taskkill` 等 Windows 专属调用抽象为 `DesktopRuntime` / `AndroidEmbeddedRuntime` / `ServerRuntime` 平台接口，禁止复制分叉。
- **配置加载**：保持 viper + `CONFIG_PATH` 机制，Linux Runtime 通过环境变量注入 Android 私有目录路径。
- **流式协议**：以现有 SSE/WebSocket 真实事件名为准（`message_start` / `delta` / `message_end` / `tool_call` 等），如有不一致以最小修改方式修复并保持 Web/Electron 兼容。

### 不做（Out of Scope，仅预留架构）

完整 Computer Use、无障碍自动操作、屏幕理解、Root、Shizuku、ADB、完整本地模型推理、完整终端 UI、MCP/Skill/AmitiaX 市场、桌宠、全局悬浮助手、自动编程工作区。本阶段目录与接口设计不得阻碍这些功能未来加入。

## Impact

### 影响的代码与系统

- `backend/cmd/server/main.go` — 平台抽象（Windows 专属代码隔离）
- `backend/internal/mcp/transport/process_windows.go` — Windows 专属进程管理，需平台分支
- `backend/pkg/database/qdrant/manager.go` — Qdrant 启动管理，需支持 Linux ARM64 二进制
- `backend/pkg/database/surrealdb/manager.go` — SurrealDB 启动管理，需支持 Linux ARM64 二进制
- `backend/config/config.yml` — 端口/路径配置（18899/19178/18000），Android 内嵌时通过环境变量覆盖
- `backend/internal/chat/handler.go` — SSE/WebSocket 流式协议（Android 客户端需严格对齐）
- `front/src/composables/useChatSSE.ts` — 前端 SSE 实现参考
- `desktop/src/runtime/process-supervisor.ts` — Electron 进程管理参考
- 新增 `android/` 整个工程目录
- 新增 `docs/android/` 14 份文档

### 端口规约（来自 `backend/config/config.yml`）

| 服务 | 端口 | 备注 |
|------|------|------|
| Go 后端 | 18899 | 127.0.0.1 |
| Qdrant | 19178 | 127.0.0.1 |
| SurrealDB | 18000 | 127.0.0.1，dataPath=data/graph.db |

> Android 内嵌模式必须保持上述端口仅监听 `127.0.0.1`，禁止 `0.0.0.0`。避开 3000 端口（项目规则）。

### 关键技术事实（已通过代码扫描确认）

- Go 版本：`go 1.26.1`，模块路径 `github.com/u-ai/backend`。
- SQLite 驱动：`github.com/glebarez/sqlite`（基于 `modernc.org/sqlite` 纯 Go 实现，**无需 CGO**，Linux ARM64 交叉编译友好）。
- 配置：`viper` + `config/config.yml` + `CONFIG_PATH` 环境变量 + `v.AutomaticEnv()`。
- 数据库迁移：Go 代码迁移（`backend/internal/migration/`）。
- Windows 专属代码：`main.go:killExistingServer` 调用 `cmd /c netstat` 和 `taskkill`，必须平台抽象。
- TTS：`internal/tts/`（edge-tts + 豆包）。
- 主动消息：`internal/proactive/`。
- 角色/记忆/模型/渠道：`internal/character/`、`internal/memory/`、`internal/model/`、`internal/channel/`。

## ADDED Requirements

### Requirement: Android 原生工程骨架

系统 SHALL 在 `android/` 目录创建可编译的 Android 原生工程，使用 Kotlin + Jetpack Compose + Gradle Kotlin DSL + Material 3 + Hilt + Navigation Compose，按 `core/runtime/platform/feature/native` 分层组织模块（可合并但职责清晰），不得使用 Flutter/React Native/WebView 套壳。

#### Scenario: 工程可编译
- **WHEN** 执行 `./gradlew assembleDebug`
- **THEN** 成功生成 Debug APK，无编译错误

#### Scenario: 模块职责清晰
- **WHEN** 审查 `android/` 目录结构
- **THEN** UI 层、Runtime 层、平台桥接层、网络层、数据层分离，不存在大量空模块

### Requirement: 内嵌 Linux Runtime（无 Root）

系统 SHALL 在 Android 应用私有目录建立无 Root Linux 用户空间，优先采用 PRoot/proot-rs 路线，由 `LinuxRootfsManager` 管理 RootFS 安装/升级/校验，由 `LinuxProcessManager` 管理进程生命周期，由 Runtime 状态机（NotInstalled/Installing/Installed/Starting/Running/Degraded/Stopping/Stopped/Failed/Updating）暴露真实状态。

#### Scenario: 首次启动安装 RootFS
- **WHEN** 用户首次选择本地模式
- **THEN** 应用解压 RootFS 到 `files/runtime/rootfs/`，显示真实解压进度（非固定延时），完成后校验完整性

#### Scenario: 升级不丢数据
- **WHEN** RootFS 升级
- **THEN** 用户数据（`files/amitia-data/`）不被删除，仅替换 RootFS

#### Scenario: 服务真实健康后才显示运行成功
- **WHEN** 启动本地 Runtime
- **THEN** 必须通过端口/HTTP 健康检查确认 SurrealDB → Qdrant → Go 后端依次健康后，UI 才显示运行成功；禁止固定延时猜测

### Requirement: Go 后端 Linux ARM64 构建

系统 SHALL 通过 `GOOS=linux GOARCH=arm64` 交叉编译 Amitia Go 后端，复用现有业务代码，不得复制分叉。Windows 专属代码（`cmd`、`taskkill`、`syscall` Windows API、绝对盘符路径）必须抽象为平台接口（`DesktopRuntime` / `AndroidEmbeddedRuntime` / `ServerRuntime`），且保持 Windows Electron、Web 部署、远程部署全部可用。

#### Scenario: 交叉编译成功
- **WHEN** 执行 `GOOS=linux GOARCH=arm64 go build`
- **THEN** 生成 Linux ARM64 二进制，不依赖 CGO（glebarez/sqlite 纯 Go）

#### Scenario: 平台抽象不破坏桌面端
- **WHEN** 后端修改后启动 Windows Electron 客户端
- **THEN** 桌面端启动、Web 连接、远程部署均正常，现有 SQLite/Qdrant/SurrealDB 数据不被破坏

### Requirement: Qdrant 与 SurrealDB Linux ARM64

系统 SHALL 为 Qdrant 和 SurrealDB 准备 Linux ARM64 二进制，由 Runtime 按启动顺序（SurrealDB → Qdrant → Go 后端）启动并持久化数据，不得用假进程替代。若因内核/指令集/PRoot 限制无法运行，MUST 记录真实错误、尝试兼容构建参数或降级版本、评估原生运行，并在不破坏接口前提下提供可替换 Adapter；不得静默忽略或谎报能力正常。

#### Scenario: Qdrant ARM64 持久化
- **WHEN** 启动 Qdrant 并写入向量后重启 Runtime
- **THEN** 重启后数据仍存在，Go 后端可正常连接

#### Scenario: SurrealDB 不可用时进入 Degraded
- **WHEN** SurrealDB 无法启动
- **THEN** Runtime 进入 `Degraded` 状态并明确告知用户图数据库能力不可用，不谎报全部正常

### Requirement: 本地/远程双模式统一架构

系统 SHALL 通过统一的 `AmitiaApiClient` / `RuntimeEndpointProvider` / `ConnectionManager` / `SessionManager` 切换本地（`http://127.0.0.1:<port>`）与远程（用户配置 HTTPS）模式，两种模式共用同一套 Repository、领域模型和 UI，不得维护两套页面。本地监听必须仅 `127.0.0.1`，禁止 `0.0.0.0`，必要时使用本地随机鉴权令牌防止其他应用调用敏感 API。

#### Scenario: 切换模式不改 UI
- **WHEN** 用户从本地模式切换到远程模式
- **THEN** 同一页面继续工作，仅 Endpoint 切换，不重建 Activity/Fragment

#### Scenario: 本地端口不暴露局域网
- **WHEN** 检查本地后端监听
- **THEN** 仅监听 `127.0.0.1`，不监听 `0.0.0.0`

### Requirement: 流式聊天协议对齐

系统 SHALL 严格按现有 Web 前端（`front/src/composables/useChatSSE.ts`）和 Go 后端（`backend/internal/chat/handler.go`）的真实 SSE/WebSocket 事件名实现，禁止自行猜测 `message_start` / `delta` / `message_end` / `tool_call` / TTS 事件 / delivery intent 等字段。如现有前后端协议不一致，以最小修改方式修复并保持 Web/Electron 兼容。

#### Scenario: 流式回复真实显示
- **WHEN** 用户发送消息
- **THEN** AI 回复通过真实 SSE/WebSocket 事件逐步渲染，非固定 JSON、非延时动画

### Requirement: 角色系统数据隔离

系统 SHALL 实现单用户多角色，切换角色时不混用消息、草稿、记忆、情绪、TTS、生成状态、未读数。所有业务数据来自 Go 后端，Android 端不得本地写死角色或用 Mock 通过最终验收。

#### Scenario: 角色切换隔离
- **WHEN** 用户从角色 A 切换到角色 B
- **THEN** 角色A的草稿、未读数、记忆、TTS 状态不被带入角色B

### Requirement: Native Capability Bridge

系统 SHALL 建立 `NativeCapabilityBridge` / `CapabilityRegistry` / `PermissionBroker` / `NativeActionRequest` / `NativeActionResult`，第一阶段提供文件选择、图片选择、相机、麦克风录音、音频播放、系统通知、剪贴板、分享、应用目录、系统主题、网络状态、电池状态、前后台状态。第一阶段不得允许 Linux 后端任意执行未经授权的 Android 系统操作。

#### Scenario: 后端受控调用 Android 能力
- **WHEN** Linux 后端需要调用 Android 能力
- **THEN** 必须通过 Bridge + PermissionBroker 校验权限，不绕过 Android 权限系统

### Requirement: 首次启动引导真实进度

系统 SHALL 在首次启动引导中展示真实进度（RootFS 安装、SurrealDB 初始化、Qdrant 初始化、Go 后端初始化、健康检查），支持中断恢复、返回修改、不重复安装、已存在配置跳过、安装失败显示真实原因、不自动跳过需用户输入步骤。禁止固定时间进度条。

#### Scenario: 引导可中断恢复
- **WHEN** 用户在 RootFS 安装中退出 App
- **THEN** 下次启动从断点继续，不重新安装已完成步骤

### Requirement: 安全与隐私

系统 SHALL 做到：本地后端仅监听 localhost；本地 API 使用随机鉴权令牌；Token 使用 Android Keystore；API Key 不写入源码；Release 日志脱敏；不记录密码/完整 Token/完整聊天隐私；文件路径校验；上传类型校验；Deep Link 校验；Bridge 请求权限校验；RootFS 更新包与二进制校验 Hash；Linux 后端不绕过 Android 权限系统；不开放任意未授权命令执行接口。

#### Scenario: Release 日志脱敏
- **WHEN** Release 构建运行
- **THEN** 日志不含完整 Token、密码、完整聊天内容

### Requirement: 运行策略与前台服务

系统 SHALL 提供三种运行策略：`AlwaysOn`（前台服务保持 Runtime 运行）、`OnDemand`（按需启动）、`RemoteOnly`（不启动本地 Linux，连远程）。默认策略根据实际资源测试决定，不盲目 AlwaysOn。前台服务必须显示符合 Android 规范的常驻通知，不使用高频轮询维持假活跃，记录系统杀进程后的恢复。

#### Scenario: 系统杀进程后不损坏数据
- **WHEN** 系统杀死 App 进程
- **THEN** 重启后数据完整，Runtime 可恢复

### Requirement: 测试与构建真实执行

系统 MUST 实际执行：`./gradlew clean`、`./gradlew test`、`./gradlew lint`、`./gradlew assembleDebug`，并构建 Go 后端 Linux ARM64、RootFS 分发包、Qdrant/SurrealDB ARM64 二进制。必须记录命令、退出码、错误、修复、最终 APK 路径、Go ARM64 二进制路径、RootFS 包路径、Qdrant/SurrealDB ARM64 路径。不得只声称理论可编译。

#### Scenario: 真机验证
- **WHEN** ARM64 Android 真机可用
- **THEN** 必须在真机验证 RootFS 解压、PRoot 运行、Qdrant、SurrealDB、Go 后端、SQLite、SSE、WebSocket、音频、图片、后台、前台服务、系统杀进程、重启恢复、低电量、网络切换、屏幕旋转、字体缩放、深色模式
- **ELSE** 明确记录真机不可用的外部阻塞

### Requirement: 文档体系完整

系统 MUST 生成 `docs/android/` 下 14 份文档：`01-current-system-audit.md`、`02-capability-migration-matrix.md`、`03-runtime-dependency-audit.md`、`04-android-architecture.md`、`05-linux-runtime-design.md`、`06-process-lifecycle.md`、`07-api-mapping.md`、`08-ui-design-system.md`、`09-build-and-run.md`、`10-testing-report.md`、`11-migration-report.md`、`12-known-limitations.md`、`13-next-stage-plan.md`、`third-party-licenses.md`。

## MODIFIED Requirements

### Requirement: Go 后端启动入口平台抽象

现有 `backend/cmd/server/main.go:killExistingServer` 调用 `cmd /c netstat` 与 `taskkill`，是 Windows 专属逻辑。修改为：通过 `RuntimePlatform` 接口暴露「检测端口占用并终止旧进程」能力，Windows 实现保留 `cmd/taskkill`，Linux/Android 实现改用 `lsof`/`fuser` + `kill`，或在内嵌模式下直接通过 PID 文件管理。保持 `backend/cmd/server/main.go` 主流程不变，配置加载、迁移、路由注册、服务启动逻辑全部复用。

### Requirement: Qdrant/SurrealDB 启动管理跨平台

现有 `backend/pkg/database/qdrant/manager.go` 与 `surrealdb/manager.go` 需支持 Linux ARM64 二进制路径与启动参数。Windows 仍用 `.exe`，Linux/Android 用 ARM64 二进制，由 `RuntimePlatform` 决定可执行文件后缀与路径。

## REMOVED Requirements

无（本阶段不删除任何现有能力，所有修改必须保持 Windows Electron、Web、远程部署兼容）。

## 第一阶段验收标准（38 项，源自 stage.md 第三十一节）

详见 `checklist.md`。核心硬指标：
1. Android 工程真实存在且 Kotlin + Compose
2. 工程编译成功，Debug APK 真实生成
3. Linux RootFS 可安装、用户空间可启动
4. Go 后端 Linux ARM64 可启动
5. SQLite/Qdrant/SurrealDB 数据持久化
6. Android 可连接本地与远程后端
7. 角色/聊天/记忆数据来自真实 Go 后端
8. 流式回复通过真实协议显示
9. 角色间数据不混淆
10. 主动消息与通知可用
11. Runtime 页面显示真实状态
12. App 重启/被杀后数据不损坏
13. 无核心 Mock、无假开关
14. 单元测试/Lint/Debug 构建实际执行
15. ARM64 真机验证（或明确记录外部阻塞）
16. 不破坏现有 Web/Electron 核心链路
17. 14 份文档全部生成
18. 最终报告如实说明未完成项

## 技术决策摘要

| 决策项 | 选择 | 理由 |
|--------|------|------|
| Android UI 栈 | Kotlin + Jetpack Compose + Hilt + Material 3 + Navigation Compose | stage.md 3.1 强制 |
| 网络层 | Retrofit + OkHttp + Kotlinx Serialization | stage.md 3.1 强制 |
| 本地缓存 | Room（UI 缓存） + DataStore（偏好） + Keystore（凭据） | stage.md 二十一 |
| Linux 路线 | PRoot / proot-rs（优先），必要时 NDK/JNI | stage.md 3.2 |
| RootFS | 精简 ARM64 Ubuntu/Debian 用户空间 | stage.md 3.2 |
| Go 后端 | `GOOS=linux GOARCH=arm64`，glebarez/sqlite 纯 Go 无 CGO | 代码扫描确认 |
| 平台抽象 | `DesktopRuntime` / `AndroidEmbeddedRuntime` / `ServerRuntime` | stage.md 9.1 |
| 本地端口 | 18899 / 19178 / 18000，仅 127.0.0.1 | config.yml |
| 运行策略 | AlwaysOn / OnDemand / RemoteOnly，默认按实测 | stage.md 二十二 |
| 流式协议 | 严格对齐 `useChatSSE.ts` + `internal/chat/handler.go` | stage.md 15 |

## 风险与缓解

| 风险 | 缓解 |
|------|------|
| Qdrant ARM64 在 PRoot 中无法运行 | 评估原生运行 / 兼容版本 / VectorStore Adapter |
| SurrealDB ARM64 兼容性 | 进入 Degraded 状态而非谎报 |
| Android 后台限制 | Foreground Service + 常驻通知 + OnDemand 策略 |
| RootFS 体积过大 | 精简 Ubuntu/Debian + 二进制 Hash 校验 |
| Go 后端 Windows 专属代码 | 平台抽象，禁止复制分叉 |
| 流式协议不一致 | 以最小修改修复，保持 Web/Electron 兼容 |
| ARM64 真机不可用 | 明确记录外部阻塞，模拟器结果不能替代真机 |
