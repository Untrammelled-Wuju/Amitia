# 04. Android 端架构总览

> 来源：Phase 0-7 实施成果
> 范围：Amitia Android 端整体架构、模块依赖、分层职责、关键技术决策、数据流、Runtime 流程、双模式架构、与现有 Web/Electron 后端的关系
> 引用：每项结论均给出 `file_path:line` 或对应模块路径
> change-id：`build-android-native-client`
> 生成时间：2026-07-26

---

## 1. 整体架构图（文字描述）

Amitia Android 端采用「外层 Android 原生 UI + 内层 Linux 用户空间」的混合架构，自上而下分为五层：

```
┌─────────────────────────────────────────────────────────────────────┐
│  Android 原生 UI 层（Kotlin + Jetpack Compose）                      │
│  ├─ 11 个 feature 模块（startup/onboarding/auth/home/chat/...）         │
│  ├─ Navigation Compose 路由图（19 条路由）                              │
│  └─ Material 3 深色优先设计系统（#0F1115 背景 + #8A9BB0 灰蓝主色）        │
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼ HTTP / WebSocket / SSE
┌─────────────────────────────────────────────────────────────────────┐
│  Android Runtime Manager 层（runtime 模块）                          │
│  ├─ BootstrapSequenceImpl（RootFS → SurrealDB → Qdrant → Go 后端）     │
│  ├─ LinuxRootfsManagerImpl（ZIP 解压 + SHA-256 manifest + 原子升级）    │
│  ├─ LinuxProcessManagerImpl（进程监控 + 重启策略 + 日志滚动 5MB）       │
│  ├─ HealthCheckerImpl（端口 + HTTP 轮询，无固定延时）                  │
│  └─ RuntimeStateMachine（10 状态机：NotInstalled → Running）            │
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼ 启动并管理
┌─────────────────────────────────────────────────────────────────────┐
│  内嵌 Linux 用户空间（PRoot / 未来转向原生）                          │
│  ├─ Amitia Go Backend（amitia-backend-arm64，54.63 MB，纯 Go 无 CGO）  │
│  ├─ SQLite（glebarez/sqlite 纯 Go 实现，无 .so 依赖）                 │
│  ├─ Qdrant（musl 静态 ARM64 二进制，74.23 MB）                       │
│  ├─ SurrealDB（gnu 动态 ARM64 二进制，111.03 MB）                     │
│  └─ 未来扩展：Python / Node / MCP / Skill（占位）                      │
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼ 进程间通信
┌─────────────────────────────────────────────────────────────────────┐
│  平台抽象层（platform 模块 + backend/pkg/platform）                   │
│  ├─ Android 系统服务（前台服务 / 通知 / 权限 / 文件 / 音频）            │
│  ├─ NativeCapabilityBridge（14 个第一阶段能力 + 8 预留）                │
│  └─ Go 后端平台抽象：Detect().KillExistingServer(addr)                │
└─────────────────────────────────────────────────────────────────────┘
```

**关键设计原则**：

1. **UI 与 Runtime 完全解耦**：UI 通过 `RuntimeEndpoint` 抽象访问后端，本地模式与远程模式使用同一套 Repository 与 ViewModel，UI 层不感知后端在哪运行。
2. **复用 Go 后端而非分叉**：Android 端直接复用 `backend/cmd/server` 编译产物，仅通过 build tag 增加平台分支，不复制业务逻辑。
3. **Android 系统集成独立模块化**：前台服务、通知、权限、文件、音频等系统能力全部封装在 `platform` 模块，UI 与 Runtime 模块只通过接口调用。

---

## 2. 模块依赖图

Android 工程位于 `android/` 目录，包含 6 个 Gradle 模块，依赖关系如下（参考 `android/settings.gradle.kts:25-30` 与各模块 `build.gradle.kts`）：

```
                          ┌──────────┐
                          │   :app   │  (application)
                          └────┬─────┘
            ┌──────────┬────────┼────────┬──────────┐
            ▼          ▼        ▼        ▼          ▼
        ┌──────┐  ┌────────┐ ┌──────┐ ┌────────┐ ┌───────┐
        │:core │  │:feature│ │:runtime│ │:platform│ │:native │
        └──┬───┘  └───┬────┘ └──┬───┘ └───┬────┘ └───┬───┘
            │          │         │         │          │
            └──────────┴─────────┴─────────┘          │
                       │                              │
                       └─────── 依赖 ────────────────┘
                                                   (JNI 壳，独立编译)
```

**模块清单**（`android/settings.gradle.kts:25-30`）：

- `:app` — 应用入口，依赖全部 5 个库模块
- `:core` — 领域层与数据层基础（网络 / 数据库 / DataStore / 错误 / 日志 / 设计系统）
- `:runtime` — Linux 用户空间 Runtime 管理（Bootstrap / 进程 / 健康 / RootFS）
- `:platform` — Android 系统服务集成（通知 / 前台服务 / 权限 / 文件 / 音频）
- `:feature` — 11 个功能模块的 UI 与 ViewModel
- `:native` — PRoot JNI 壳（`ProotBridge.kt` + `proot_jni.cpp`），独立编译，仅 `:app` 引用

**依赖方向约束**：

- `:app` 依赖所有库模块
- `:feature` 依赖 `:core` 与 `:runtime`（用于 Runtime 状态观察）
- `:runtime` 依赖 `:core`（Logger、Constants、错误模型）
- `:platform` 依赖 `:core`（Logger、错误模型）
- `:native` 不依赖任何 Kotlin 模块，仅通过 `BuildConfig` 暴露开关
- 任何模块**不得反向依赖** `:app`

---

## 3. 分层职责

### 3.1 UI 层（feature 模块）

**职责**：用户交互、Compose 界面渲染、状态持有

**关键文件**（`android/feature/src/main/java/com/amitia/feature/`）：

- `startup/` — 启动加载页（显示 Runtime 启动进度）
- `onboarding/` — 首次启动引导（介绍能力 + 申请权限）
- `auth/` — 本地认证（KeystoreManager 签发 JWT）
- `home/` — 主页 + Capability 仪表盘
- `chat/` — 聊天主界面 + `chat/audio/`（语音输入）+ `chat/media/`（图片选择）+ `chat/tts/`（TTS 播放）
- `character/` — 角色列表 / 详情 / 编辑 / 创建
- `memory/` — 记忆列表 / 详情 / 编辑 / 创建
- `models/` — 模型配置管理
- `channels/` — 渠道状态查看
- `runtime/` — Runtime 管理页（启停 / 状态 / 日志）
- `settings/` — 设置页（14 个配置项）

每个 feature 子模块标准结构：`XxxViewModel.kt` + `XxxScreen.kt` + 子组件 `*.kt`。

### 3.2 领域层（core/repository）

**职责**：业务规则、数据聚合、与具体数据源解耦

**关键文件**（`android/core/src/main/java/com/amitia/core/repository/`）：

- `ChatRepository` — 聊天会话与消息持久化
- `CharacterRepository` — 角色配置读写
- `MemoryRepository` — 长期记忆管理
- `ModelRepository` — 模型配置
- `ChannelRepository` — 渠道状态
- `ProactiveMessageRepository` — 主动消息历史
- `RuntimeRepository` — Runtime 状态查询
- `AuthRepository` — 本地认证 token 签发

Repository 通过 Hilt 注入的 Retrofit API 接口与 DTO 完成网络调用，调用方不感知是 Local 还是 Remote Endpoint。

### 3.3 数据层（core/network + core/database）

**职责**：网络通信、本地数据库、配置持久化

**关键文件**（`android/core/src/main/java/com/amitia/core/network/`）：

- `endpoint/RuntimeEndpoint.kt` — Local / Remote 端点抽象
- `endpoint/RuntimeEndpointProvider.kt` — 根据 DataStore 切换 Endpoint
- `client/AmitiaApiClient.kt` — OkHttp + Retrofit 客户端
- `client/ErrorMappingInterceptor.kt` — 错误码映射
- `api/*.kt` — 9 个 Retrofit API 接口
- `sse/SseClient.kt` + `SseParser.kt` — SSE 流式响应解析
- `ws/WsClient.kt` — WebSocket 客户端
- `connection/ConnectionManager.kt` — 连接状态管理
- `connection/SessionManager.kt` — 会话 token 管理

**数据库**（`android/core/src/main/java/com/amitia/core/database/`）：

- 7 个 Entity + 7 个 DAO + `AmitiaDatabase`
- `converter/Converters.kt` — Room 类型转换器
- `DatabaseModule.kt` — Hilt 注入配置

**DataStore**（`android/core/src/main/java/com/amitia/core/datastore/`）：

- `SettingsDataStore.kt` — 14 个配置项（主题 / 字体 / 模型 / 渠道 / Runtime 策略等）

### 3.4 Runtime 层（runtime 模块）

**职责**：管理内嵌 Linux 用户空间进程的生命周期

**关键文件**（`android/runtime/src/main/java/com/amitia/runtime/`）：

- `api/RuntimeState.kt` — 10 状态枚举与状态快照
- `manager/RuntimeManager.kt` + `RuntimeStateMachine.kt` — 状态机
- `linux/LinuxRootfsManagerImpl.kt` — RootFS ZIP 解压、SHA-256 校验、原子升级
- `process/LinuxProcessManagerImpl.kt` — 进程启动、监控、重启策略、日志滚动 5MB
- `bootstrap/BootstrapSequenceImpl.kt` — 启动顺序编排：RootFS → SurrealDB → Qdrant → Go 后端；停止顺序反向
- `health/HealthCheckerImpl.kt` — 端口探测 + HTTP 轮询，无固定延时
- `bridge/NativeCapabilityBridge.kt` — 14 个第一阶段能力 + 8 预留

详细设计见 `docs/android/05-linux-runtime-design.md` 与 `docs/android/06-process-lifecycle.md`。

### 3.5 平台层（platform 模块）

**职责**：封装 Android 系统能力，对 Runtime 与 UI 层提供接口

**关键文件**（`android/platform/src/main/java/com/amitia/platform/`）：

- `notification/NotificationManagerImpl.kt` — 系统通知
- `foreground/ForegroundServiceManagerImpl.kt` — 前台服务常驻通知
- `permissions/PermissionBrokerImpl.kt` — 权限请求路由
- `files/FilePickerImpl.kt` — 文件选择（占位，待 Activity Result 接入）
- `audio/AudioPlayerImpl.kt` + `AudioRecorderImpl.kt` — 音频播放录制（占位）
- `bridge/provider/*.kt` — 9 个能力 Provider 实现

**前台服务**（`android/app/src/main/java/com/amitia/android/runtime/AmitiaCoreService.kt`）：

- 继承 `Service`，`START_STICKY` + 常驻通知
- `foregroundServiceType="dataSync"`（`AndroidManifest.xml:47`）
- 监听 `RuntimeManager.observeState()` 并更新通知文案

### 3.6 Native 层（native 模块）

**职责**：PRoot JNI 壳，为 Linux 用户空间提供系统调用拦截能力

**关键文件**（`android/native/src/main/`）：

- `java/com/amitia/nativeproot/ProotBridge.kt` — Kotlin JNI 接口声明
- `cpp/proot_jni.cpp` — C++ JNI 实现（壳，未集成完整 PRoot）
- `cpp/CMakeLists.txt` — NDK 构建配置
- `AndroidManifest.xml` — 库清单

当前为占位实现，待后续接入 proot-rs 或预编译 .so。

---

## 4. 关键技术决策表

| 维度 | 决策 | 理由 | 引用 |
|---|---|---|---|
| Android UI 栈 | Kotlin 2.0.20 + Jetpack Compose BOM 2024.09.00 + Material 3 | 现代 Android 推荐栈；声明式 UI 适合状态驱动；Material 3 支持深色优先 | `android/gradle/libs.versions.toml:3-5` |
| 依赖注入 | Hilt 2.52 + KAPT | 官方推荐；与 ViewModel / WorkManager 集成成熟 | `libs.versions.toml:5` |
| 网络层 | OkHttp 4.12 + Retrofit 2.11 + kotlinx.serialization | 主流稳定栈；拦截器链清晰；与 SSE / WS 兼容 | `libs.versions.toml:7-9` |
| Linux 路线 | 内嵌 Linux 用户空间（PRoot + 静态二进制） | 复用 Web/Electron 已验证的 Go 后端 + 数据库二进制；规避重写业务逻辑 | `docs/android/05-linux-runtime-design.md` |
| Go 后端 | 复用 `backend/cmd/server`，build tag 分发平台分支 | 避免分叉；Windows / Linux 桌面 / Android 共用一份业务代码 | `backend/pkg/platform/*.go` |
| 平台抽象 | `platform.Detect().KillExistingServer(addr)` 接口 | 替代 main.go 硬编码 Windows 逻辑；编译期分发 desktop_windows.go / desktop_linux.go / android_embedded.go | `backend/pkg/platform/runtime_platform.go:5-16` |
| 端口 | 18899（后端）/ 19178（Qdrant）/ 18000（SurrealDB） | 沿用 Web/Electron 端口；避开 3000（CCX 占用） | `rootfs-install-template.sh:21-39` |
| 运行策略 | OnDemand 默认 + AlwaysOn 可选 | 平衡电池 / 性能；遵守 AGENTS.md 不乱改端口要求 | `core/datastore/SettingsDataStore.kt` |
| 流式协议 | SSE `POST /api/web-chat/send-stream`，事件 `message_start` / `token` / `voice_audio` / `message_end` | 与 Web/Electron 后端兼容；OkHttp 逐行解析 | `docs/android/07-api-mapping.md` |
| 数据库 | Room 2.6.1 + 7 Entity + 7 DAO | 官方推荐；类型安全；与 Flow 集成 | `libs.versions.toml:6` |
| SQLite 驱动 | glebarez/sqlite（纯 Go） | 无 CGO 依赖；可静态交叉编译到 ARM64 | `backend/go.mod:8` |
| RootFS 分发 | assets 内置 3 个二进制（共 240 MB） + manifest + install 脚本 | 首次启动即可用；SHA-256 校验完整性 | `android/app/src/main/assets/rootfs-manifest.json` |

---

## 5. 数据流

以「用户在聊天页发送一条消息」为例：

```
用户输入「你好」并点发送
        │
        ▼
ChatViewModel.sendMessage(text)               # feature/chat/ChatViewModel.kt
        │
        ▼
ChatRepository.sendStreamMessage(req)         # core/repository/ChatRepository.kt
        │
        ▼
ChatApi.sendStream(body) : Flow<SseEvent>     # core/network/api/ChatApi.kt
        │
        ▼
SseClient.openConnection(url, body)            # core/network/sse/SseClient.kt
        │
        ▼
OkHttp.newCall(...)                            # OkHttp 拦截器链：Auth → Logging → ErrorMapping
        │
        ▼
RuntimeEndpointProvider.current()             # core/network/endpoint/RuntimeEndpointProvider.kt
        │  ┌─ Local 模式：http://127.0.0.1:18899
        │  └─ Remote 模式：http://用户填写地址
        ▼
HTTP POST /api/web-chat/send-stream           # 内嵌 Go 后端 或 远程服务器
        │
        ▼
后端 SSE 流式返回：
  event: message_start
  event: token          data: {"text":"你"}
  event: token          data: {"text":"好"}
  event: message_end
        │
        ▼
SseParser.parse(line) → SseEvent               # core/network/sse/SseParser.kt
        │
        ▼
ChatViewModel 收集 Flow 并更新 UI State
        │
        ▼
ChatScreen 重组渲染 token
```

**关键点**：

- ViewModel 只持有 StateFlow，不感知 Endpoint 来源
- Repository 通过 Hilt 注入，单例
- ApiClient 拦截器顺序：`Auth`（注入 JWT）→ `Logging`（调试日志）→ `ErrorMapping`（HTTP 状态码 → AmitiaError 子类）
- SSE 流严格按事件名 `message_start` / `token` / `voice_audio` / `message_end` 分发

---

## 6. Runtime 启动流程

App 启动到 UI 显示「Running」的全过程：

```
AmitiaApplication.onCreate()                   # app/AmitiaApplication.kt:21
        │
        ▼
MainActivity.onCreate()                        # app/MainActivity.kt
        │
        ▼
AmitiaNavHost 首页 = StartupScreen              # app/navigation/AmitiaNavHost.kt
        │
        ▼
StartupViewModel.observeRuntimeState()         # feature/startup/StartupViewModel.kt
        │
        ▼
RuntimeManager.start() 触发 BootstrapSequence   # runtime/manager/RuntimeManager.kt
        │
        ▼
BootstrapSequenceImpl.execute()：                # runtime/bootstrap/BootstrapSequenceImpl.kt
        │
        ├─ 1. LinuxRootfsManagerImpl.ensureRootfs()
        │      └─ 检查 rootfs-manifest.json SHA-256
        │      └─ 不匹配则解压 assets → files/runtime/rootfs/
        │
        ├─ 2. 启动 SurrealDB（端口 18000）
        │      └─ LinuxProcessManagerImpl.start("surreal", args)
        │      └─ HealthCheckerImpl.waitUntilReady("127.0.0.1:18000")
        │
        ├─ 3. 启动 Qdrant（端口 19178）
        │      └─ LinuxProcessManagerImpl.start("qdrant", args)
        │      └─ HealthCheckerImpl.waitUntilReady("127.0.0.1:19178")
        │
        └─ 4. 启动 Go 后端（端口 18899）
               └─ LinuxProcessManagerImpl.start("amitia-backend", args)
               └─ platform.Detect().KillExistingServer("127.0.0.1:18899")
               └─ HealthCheckerImpl.waitUntilReady("http://127.0.0.1:18899/health")
        │
        ▼
RuntimeStateMachine.transitionTo(Running)      # runtime/manager/RuntimeStateMachine.kt
        │
        ▼
StartupViewModel 收到 Running 状态
        │
        ▼
导航到 HomeScreen                               # AmitiaRoutes.HOME
```

**停止顺序**（反向）：

```
RuntimeManager.stop()
  ├─ 停止 Go 后端（SIGTERM）
  ├─ 停止 Qdrant
  ├─ 停止 SurrealDB
  └─ RuntimeStateMachine.transitionTo(Stopped)
```

**状态机 10 状态**（详见 `docs/android/06-process-lifecycle.md`）：

```
NotInstalled → Installing → Installed → Starting → Running
                                                       │
                                                       ▼
                                                  Stopping → Stopped
                                                       │
                                                  Degraded / Failed / Updating
```

---

## 7. 双模式架构（Local / Remote）

### 7.1 设计目标

用户可在不重建 UI 的前提下，在「本地内嵌 Runtime」与「远程服务器」之间切换。所有 ViewModel、Repository、ApiClient 代码无需感知模式差异。

### 7.2 实现

**RuntimeEndpoint 接口**（`android/core/src/main/java/com/amitia/core/network/endpoint/RuntimeEndpoint.kt`）：

- `LocalEndpoint` — `http://127.0.0.1:18899`，由内嵌 Runtime 提供
- `RemoteEndpoint` — 用户填写的远程地址

**RuntimeEndpointProvider**（`android/core/src/main/java/com/amitia/core/network/endpoint/RuntimeEndpointProvider.kt`）：

- 读取 `SettingsDataStore` 中的 `mode`（local / remote）
- 暴露 `current: RuntimeEndpoint` 与 `flow: Flow<RuntimeEndpoint>`
- 切换模式时自动重连

**ApiClient**：

- OkHttp `AuthInterceptor` 在每次请求时调用 `endpointProvider.current().baseUrl()`
- 切换模式后下一次请求自动指向新地址，无需重建 ApiClient

**UI 切换**：

- `feature/settings/SettingsScreen.kt` 提供切换开关
- 切换后 `SettingsDataStore` 持久化新值
- `RuntimeEndpointProvider` 收到 DataStore 变更，发射新 Endpoint
- 所有 Repository 在下一次调用时自动应用新地址

### 7.3 注意事项

- Local 模式下 Runtime 必须处于 `Running` 状态才能连接
- Remote 模式下 Runtime 可保持 `Stopped`，UI 不依赖本地进程
- 模式切换不触发 UI 重建，仅触发 Repository 重新连接

---

## 8. 与现有 Web/Electron 后端的关系

### 8.1 复用而非分叉

Android 端**直接复用** `backend/cmd/server` 编译产物，不维护业务逻辑分叉：

| 模块 | Web/Electron | Android | 复用方式 |
|---|---|---|---|
| 业务逻辑 | `backend/cmd/server/main.go` + `services.go` 等 | 同一份代码 | 交叉编译 |
| 数据库 | GORM + glebarez/sqlite | 同一份代码 | 编译产物复用 |
| Qdrant 客户端 | `qdrant/go-client` | 同一份代码 | 编译产物复用 |
| SurrealDB 客户端 | `surrealdb.go` | 同一份代码 | 编译产物复用 |
| SSE / WebSocket | Gin + gorilla/websocket | 同一份代码 | 编译产物复用 |
| 平台相关代码 | `desktop_windows.go` / `desktop_linux.go` | `android_embedded.go`（build tag） | 接口统一，实现分发 |

### 8.2 平台抽象

Go 后端通过 `RuntimePlatform` 接口隔离平台差异（`backend/pkg/platform/runtime_platform.go:5-16`）：

```go
type RuntimePlatform interface {
    Name() string
    KillExistingServer(addr string) error
    ExecutableSuffix() string
    DefaultDataDir() string
    IsWindows() bool
    IsLinux() bool
    IsAndroidEmbedded() bool
    WritePidFile(dataDir string) error
    ReadPidFile(dataDir string) (int, error)
    RemovePidFile(dataDir string) error
}
```

- `desktop_windows.go`（`//go:build windows`）— Windows 桌面
- `desktop_linux.go`（`//go:build linux && !android`）— Linux 桌面
- `android_embedded.go`（`//go:build android`）— Android 嵌入式

`main.go` 通过 `platform.Detect().KillExistingServer(addr)` 替代原 Windows 硬编码逻辑，编译期自动分发。

### 8.3 数据隔离

Android 端用户数据与 Web/Electron 完全隔离：

- RootFS 安装目录：`files/runtime/rootfs/`
- 用户数据目录：`files/amitia-data/`（含 sqlite / qdrant / surrealdb / uploads / models / extensions / backups）
- 配置通过 `AMITIA_DATA_DIR` 环境变量传入 Go 后端

参考 `android/app/src/main/assets/rootfs-install-template.sh:5-12`。

### 8.4 API 兼容性

Android 端调用的所有 HTTP / SSE / WebSocket 接口与 Web/Electron 完全一致：

- 同一套路由（`/api/web-chat/send-stream` 等）
- 同一套 SSE 事件名（`message_start` / `token` / `voice_audio` / `message_end`）
- 同一套 DTO 结构（kotlinx.serialization 与 Go JSON 标签对齐）

详见 `docs/android/07-api-mapping.md`。

---

## 9. 后续演进方向

当前阶段（Phase 0-7）已完成第一阶段架构落地，后续演进方向包括：

1. **PRoot 替换为原生执行**：在 Root 设备或 Android 14+ 部分场景下直接执行 Linux 二进制，绕过 PRoot 性能损耗
2. **本地模型推理**：接入 llama.cpp / MNN / ONNX Runtime 在端侧运行小模型
3. **Computer Use / Accessibility / MediaProjection**：第二阶段系统能力扩展
4. **MCP / Skill / AmitiaX 市场**：扩展生态
5. **桌宠 / 全局悬浮助手**：UI 层增强

详见 `docs/android/13-next-stage-plan.md`。
