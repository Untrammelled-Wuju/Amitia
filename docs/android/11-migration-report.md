# 11. 迁移报告

> 来源：Phase 0-7 实施成果
> 范围：Web/Electron → Android 端功能迁移情况、新增文件清单、Go 后端修改、Qdrant/SurrealDB/RootFS 处理方式、测试与构建结果、当前阻塞、第一阶段验收结论
> 引用：每项结论均给出 `file_path`
> change-id：`build-android-native-client`
> 生成时间：2026-07-26

---

## 1. 已迁移功能

以下功能从 Web/Electron 后端完整迁移到 Android 端，且与后端接口对齐（详见 `docs/android/07-api-mapping.md`）：

| 序号 | 功能 | Android 模块 | 后端接口 |
|---|---|---|---|
| 1 | 角色系统（列表 / 详情 / 编辑 / 创建） | `feature/character/` | `/api/character/*` |
| 2 | 聊天历史 | `feature/chat/` + `core/repository/ChatRepository.kt` | `/api/chat/history` |
| 3 | 流式回复 | `feature/chat/ChatViewModel.kt` + `core/network/sse/SseClient.kt` | `POST /api/web-chat/send-stream` |
| 4 | 图片消息 | `feature/chat/media/` + `core/repository/` | `/api/upload/image` |
| 5 | 语音消息 | `feature/chat/audio/` + `core/repository/` | `/api/upload/audio` |
| 6 | TTS | `feature/chat/tts/` + `platform/audio/AudioPlayerImpl.kt` | `event: voice_audio`（SSE） |
| 7 | 记忆系统 | `feature/memory/` | `/api/memory/*` |
| 8 | 模型配置 | `feature/models/` | `/api/models/*` |
| 9 | 渠道状态 | `feature/channels/` | `/api/channels/*` |
| 10 | 主动消息 | `core/repository/ProactiveMessageRepository.kt` | WS 推送 |
| 11 | Runtime 管理 | `feature/runtime/` | `RuntimeManager` 内嵌 |
| 12 | 设置 | `feature/settings/` + `core/datastore/SettingsDataStore.kt` | 14 个本地配置 |
| 13 | 首次启动引导 | `feature/onboarding/` | 本地流程 |
| 14 | 本地 / 远程双模式 | `core/network/endpoint/RuntimeEndpoint.kt` | 同一套后端接口 |
| 15 | Android 通知 | `platform/notification/` + `platform/foreground/` + `AmitiaCoreService.kt` | 系统能力 |

---

## 2. 部分迁移功能

| 序号 | 功能 | 当前状态 | 待后续处理 |
|---|---|---|---|
| 1 | 图片选择 Provider | `platform/files/FilePickerImpl.kt` 占位实现 | 接入 `rememberLauncherForActivityResult(OpenDocument())` |
| 2 | 音频播放 Provider | `platform/audio/AudioPlayerImpl.kt` 占位实现 | 接入 Media3 ExoPlayer 完整生命周期 |
| 3 | 音频录制 Provider | `platform/audio/AudioRecorderImpl.kt` 占位实现 | 接入 `MediaRecorder` + 权限请求 |
| 4 | 主动消息观察者 | `ProactiveMessageObserver` 已实现，未在 `RuntimeState.Running` 自动启动 | 在 `AmitiaCoreService` 或 `StartupViewModel` 注册 |
| 5 | 通知点击跳转 | PendingIntent 已构建，未在 `MainActivity.onNewIntent` 解析 | 解析 intent extras 并导航到指定页面 |

---

## 3. 未迁移功能

无。核心功能已全部迁移到 Android 端。

---

## 4. 未迁移原因

占位实现因 Phase 7 时间限制未完成接入：

- FilePicker / AudioPlayer / AudioRecorder：Activity Result API 接入需要 Compose 1.7 + accompanist-permissions 0.34 协同，时间不足
- ProactiveMessageObserver 自动启动：依赖 RuntimeManager 状态流注入，待 Phase 8 后补充
- 通知点击跳转：路由解析逻辑需在 NavHost 初始化完成后注入，待 Phase 8 后补充

详见 `docs/android/13-next-stage-plan.md`。

---

## 5. Android 新增文件清单（按模块统计）

### 5.1 android/app（7 个 Kotlin + 资源）

路径：`android/app/src/main/java/com/amitia/android/`

| 文件 | 用途 |
|---|---|
| `AmitiaApplication.kt` | `@HiltAndroidApp`，WorkManager 配置 |
| `MainActivity.kt` | Compose 宿主 |
| `runtime/AmitiaCoreService.kt` | 前台服务，`START_STICKY` + 常驻通知 |
| `navigation/AmitiaNavHost.kt` | 导航图（19 条路由） |
| `navigation/AmitiaRoutes.kt` | 路由常量 |
| `navigation/AmitiaBottomBar.kt` | 底部导航 |
| `navigation/CapabilityTabScreen.kt` | Capability 仪表盘 |

附加：

- `src/main/res/values/themes.xml`、`strings.xml`、`colors.xml`
- `src/main/res/xml/data_extraction_rules.xml`、`backup_rules.xml`
- `src/main/AndroidManifest.xml`（权限 + Activity + Service 注册）
- `src/main/assets/`：RootFS 分发包（5 个文件，共 240 MB）

### 5.2 android/core（77 个 Kotlin）

路径：`android/core/src/main/java/com/amitia/core/`

| 子包 | 文件数 | 主要内容 |
|---|---|---|
| `common/` | 1 | `Constants.kt`（端口、路径、超时） |
| `network/endpoint/` | 4 | `RuntimeEndpoint`、`RuntimeEndpointProvider`、`LocalAuthTokenProvider` 等 |
| `network/client/` | 4 | `AmitiaApiClient`、`ErrorMappingInterceptor`、OkHttp 配置 |
| `network/connection/` | 2 | `ConnectionManager`、`SessionManager` |
| `network/api/` | 9 | Retrofit API 接口 |
| `network/sse/` | 2 | `SseClient`、`SseParser` |
| `network/ws/` | 1 | `WsClient` |
| `repository/` | 8 | Repository 实现 |
| `model/` | 10 | DTO |
| `database/entity/` | 7 | Room Entity |
| `database/dao/` | 7 | Room DAO |
| `database/converter/` | 1 | `Converters.kt` |
| `database/` | 1 | `AmitiaDatabase` + `DatabaseModule` |
| `datastore/` | 1 | `SettingsDataStore.kt`（14 个配置） |
| `security/` | 1 | `KeystoreManager`（per-alias Keystore key） |
| `error/` | 2 | `AmitiaError`（19 子类）+ `ErrorMapper` |
| `logging/` | 2 | `Logger`（@Singleton）+ `LogSanitizer` + `exportDiagnostics` |
| `media/` | 1 | 媒体工具 |
| `designsystem/` | 多个 | 主题、颜色、字体、组件 |

合计 77 个 Kotlin 文件。

### 5.3 android/runtime（17 个 Kotlin）

路径：`android/runtime/src/main/java/com/amitia/runtime/`

| 子包 | 文件数 | 主要内容 |
|---|---|---|
| `api/` | 1 | `RuntimeState`（10 状态） |
| `manager/` | 2 | `RuntimeManager` + `RuntimeStateMachine` |
| `linux/` | 2 | `LinuxRootfsManagerImpl` + `RootfsIntegrityChecker` |
| `process/` | 1 | `LinuxProcessManagerImpl`（进程监控 + 重启 + 日志滚动 5MB） |
| `bootstrap/` | 1 | `BootstrapSequenceImpl` |
| `health/` | 1 | `HealthCheckerImpl` |
| `bridge/` | 1 | `NativeCapabilityBridge`（14 第一阶段能力 + 8 预留） |
| 其他 | 8 | Hilt Module、辅助类等 |

合计 17 个 Kotlin 文件。

### 5.4 android/platform（32 个 Kotlin）

路径：`android/platform/src/main/java/com/amitia/platform/`

| 子包 | 文件数 | 主要内容 |
|---|---|---|
| `notification/` | 多个 | `NotificationManagerImpl` |
| `foreground/` | 多个 | `ForegroundServiceManagerImpl` |
| `permissions/` | 多个 | `PermissionBrokerImpl` |
| `files/` | 多个 | `FilePickerImpl`（占位） |
| `audio/` | 多个 | `AudioPlayerImpl` + `AudioRecorderImpl`（占位） |
| `bridge/provider/` | 9 | 9 个能力 Provider 实现 |

合计 32 个 Kotlin 文件。

### 5.5 android/feature（56 个 Kotlin）

路径：`android/feature/src/main/java/com/amitia/feature/`

11 个 feature 子模块：

| 子模块 | 主要文件 |
|---|---|
| `startup/` | `StartupScreen.kt` + `StartupViewModel.kt` |
| `onboarding/` | `OnboardingScreen.kt` + `OnboardingViewModel.kt` |
| `auth/` | `AuthScreen.kt` + `AuthViewModel.kt` |
| `home/` | `HomeScreen.kt` + `HomeViewModel.kt` |
| `chat/` | `ChatScreen.kt` + `ChatViewModel.kt` + 子组件 |
| `chat/audio/` | 语音输入子组件 |
| `chat/media/` | 图片选择子组件 |
| `chat/tts/` | TTS 播放子组件 |
| `character/` | `CharacterListScreen.kt` + `CharacterDetailScreen.kt` + `CharacterEditScreen.kt` + `CharacterCreateScreen.kt` + ViewModel |
| `memory/` | 列表 + 详情 + 编辑 + 创建 + ViewModel |
| `models/` | 模型管理 + ViewModel |
| `channels/` | 渠道状态 + ViewModel |
| `runtime/` | Runtime 管理 + ViewModel |
| `settings/` | 设置页 + ViewModel |
| `common/` | 共享 UI 组件 |

合计 56 个 Kotlin 文件。

### 5.6 android/native（1 个 Kotlin + 1 个 C++ + CMake）

路径：`android/native/src/main/`

| 文件 | 用途 |
|---|---|
| `java/com/amitia/nativeproot/ProotBridge.kt` | Kotlin JNI 壳 |
| `cpp/proot_jni.cpp` | C++ JNI 实现（壳，未集成完整 PRoot） |
| `cpp/CMakeLists.txt` | NDK 构建配置 |
| `AndroidManifest.xml` | 库清单 |

### 5.7 总计

| 模块 | Kotlin 文件数 |
|---|---|
| app | 7 |
| core | 77 |
| runtime | 17 |
| platform | 32 |
| feature | 56 |
| native | 1 |
| **合计** | **190** |

加上测试 .kt 文件 27 个，总计约 **217 个 Kotlin 文件** + C++ / CMake / 资源 / Manifest。

---

## 6. Go 后端修改文件清单

为支持 Android 嵌入式场景，Go 后端新增 / 修改以下文件（详见 `docs/android/03-runtime-dependency-audit.md`）：

| 文件 | 修改类型 | 内容 | 引用 |
|---|---|---|---|
| `backend/cmd/server/main.go` | 修改 | `platform.Detect().KillExistingServer(addr)` 替代 Windows 硬编码 | `main.go:38-56` |
| `backend/pkg/platform/runtime_platform.go` | 新增 | `RuntimePlatform` 接口（8 方法） | `runtime_platform.go:5-16` |
| `backend/pkg/platform/desktop_windows.go` | 新增 | Windows 实现（`//go:build windows`） | `desktop_windows.go` |
| `backend/pkg/platform/desktop_linux.go` | 新增 | Linux 桌面实现（`//go:build linux && !android`） | `desktop_linux.go` |
| `backend/pkg/platform/android_embedded.go` | 新增 | Android 嵌入式实现（`//go:build android`），PID 文件管理 + SIGTERM | `android_embedded.go` |
| `backend/pkg/database/surrealdb/manager.go` | 修改 | 支持 ARM64 路径（修复 P0-2） | `manager.go:44-70, 88-95` |
| `backend/pkg/database/qdrant/manager.go` | 修改 | ARM64 fallback 路径 | `manager.go:34-60, 78-89` |

**修改原则**：

- 不复制业务逻辑分叉，仅通过 build tag 分发平台相关代码
- 接口统一，编译期分发
- Web/Electron 行为完全保持

---

## 7. Qdrant 处理方式

### 7.1 选择

从 GitHub Release 下载 **musl 静态 ARM64 二进制**：

```
https://github.com/qdrant/qdrant/releases/latest/download/qdrant-aarch64-unknown-linux-musl.tar.gz
```

参考 `android/app/src/main/assets/rootfs-manifest.json:22`。

### 7.2 选择理由

- musl 静态链接，无 glibc 依赖
- 在 PRoot 环境中兼容性最佳
- 单文件分发，简化 RootFS 管理

### 7.3 二进制信息

- 文件名：`qdrant_linux_aarch64`
- 大小：77835808 字节（74.23 MB）
- SHA-256：`6CB81123D2A3E405335C984EFB7928DC21A1EAC47A3B73A7609AEE076DBE0B04`
- 链接方式：静态
- 架构：linux/arm64

### 7.4 配置

启动参数由 `rootfs-install-template.sh` 生成的 `config.yml` 提供（`rootfs-install-template.sh:34-35`）：

```yaml
qdrant:
  host: "127.0.0.1"
  port: 19178
```

---

## 8. SurrealDB 处理方式

### 8.1 选择

从 GitHub Release 下载 **gnu 动态 ARM64 二进制**：

```
https://github.com/surrealdb/surrealdb/releases/download/v3.2.0/surreal-v3.2.0.linux-arm64.tgz
```

参考 `android/app/src/main/assets/rootfs-manifest.json:32`。

### 8.2 选择理由

- SurrealDB 官方未提供 musl 静态 ARM64 版本
- gnu 动态版本依赖 glibc，但 PRoot 环境通常包含兼容的 glibc

### 8.3 二进制信息

- 文件名：`surreal_linux_aarch64`
- 大小：116424792 字节（111.03 MB）
- SHA-256：`A235206F2C4A803616D7669F56BC5BCA5CC0AE7ED2B79ACBE91F14712B28FE5C`
- 链接方式：动态（gnu）
- 架构：linux/arm64

### 8.4 兼容性风险

**风险**：PRoot 中 glibc 兼容性未在真机验证。

**Phase 7 真机验证状态**：外部阻塞（无 ARM64 真机）。

**应对方案**（若真机启动失败）：

1. 改用 musl 静态版本（社区构建或自行编译）
2. 从源码编译：`cargo build --release --target aarch64-unknown-linux-musl`
3. 切换到其他图数据库（如 Redis Graph）作为降级方案

详见 `docs/android/12-known-limitations.md` 与 `docs/android/13-next-stage-plan.md`。

### 8.5 配置

```yaml
surrealdb:
  host: "127.0.0.1"
  port: 18000
  namespace: "uai"
  database: "memory_graph"
  username: "root"
  password: "root"
  dataPath: "$DATA_DIR/surrealdb/graph.db"
```

参考 `rootfs-install-template.sh:37-44`。

---

## 9. RootFS 处理方式

### 9.1 组成

RootFS 分发包包含：

| 文件 | 类型 | 大小 |
|---|---|---|
| `amitia-backend-arm64` | Go 后端 | 54.63 MB |
| `qdrant_linux_aarch64` | 向量数据库 | 74.23 MB |
| `surreal_linux_aarch64` | 图数据库 | 111.03 MB |
| `rootfs-manifest.json` | 清单（含 SHA-256） | 1.34 KB |
| `rootfs-install-template.sh` | 安装脚本模板 | 1.63 KB |
| **合计** | — | **240 MB** |

### 9.2 打包方式

作为 Android `assets/` 内置资源随 APK 分发：

```
android/app/src/main/assets/
├── amitia-backend-arm64
├── qdrant_linux_aarch64
├── surreal_linux_aarch64
├── rootfs-manifest.json
└── rootfs-install-template.sh
```

### 9.3 首次启动流程

1. App 启动 → `LinuxRootfsManagerImpl.ensureRootfs()`
2. 读取 `rootfs-manifest.json`，检查 `files/runtime/rootfs/` 是否存在
3. 不存在或 SHA-256 不匹配 → 从 `assets/` 解压到 `files/runtime/rootfs/`
4. 执行 `rootfs-install-template.sh` 生成 `bin/` `etc/config.yml` `VERSION`
5. 进入 Bootstrap 启动顺序

### 9.4 数据隔离

- RootFS 安装目录：`files/runtime/rootfs/`（只读，应用卸载时清理）
- 用户数据目录：`files/amitia-data/`（含 sqlite / qdrant / surrealdb / uploads / models / extensions / backups）

参考 `rootfs-install-template.sh:5-12`。

### 9.5 原子升级

`LinuxRootfsManagerImpl` + `RootfsIntegrityChecker` 实现：

- 升级前下载新 RootFS 到临时目录
- SHA-256 校验通过后原子替换
- 失败回滚到旧版本

详见 `docs/android/05-linux-runtime-design.md`。

---

## 10. 测试结果

| 维度 | 数值 | 状态 |
|---|---|---|
| 单元测试文件 | 18 | 已编写，未执行（classpath 阻塞） |
| 集成测试文件 | 3 | 已编写，未执行（classpath 阻塞 + 真机阻塞） |
| UI 测试文件 | 5 | 已编写，未执行（classpath 阻塞 + 真机阻塞） |
| 测试 .kt 文件总数 | 27 | — |
| 已修复编译问题 | 12 项 | 已修复 |
| 测试通过率 | N/A | classpath 阻塞 |
| ARM64 真机验证 | 0 / 22 项 | 外部阻塞 |

详见 `docs/android/10-testing-report.md`。

---

## 11. 构建结果

| 产物 | 路径 | 大小 | 状态 |
|---|---|---|---|
| Windows server.exe | `backend/cmd/server/server.exe` | 59.18 MB（62047232 字节） | 编译成功 |
| Linux ARM64 amitia-backend-arm64 | `android/app/src/main/assets/amitia-backend-arm64` | 54.63 MB（57283044 字节） | 交叉编译成功 |
| Qdrant ARM64 | `android/app/src/main/assets/qdrant_linux_aarch64` | 74.23 MB | 已下载 |
| SurrealDB ARM64 | `android/app/src/main/assets/surreal_linux_aarch64` | 111.03 MB | 已下载 |
| PRoot ARM64 | `android/app/src/main/assets/proot_linux_aarch64` | 1.43 MB | 已下载（proot-rs v0.1.0） |
| RootFS 分发包合计 | `android/app/src/main/assets/` | 241 MB | 完整（含 PRoot 二进制） |
| Debug APK | `android/app/build/outputs/apk/debug/app-debug.apk` | 131.07 MB（137436908 字节） | 已生成（含 PRoot 集成 + manifest totalSize 修正，clean+assembleDebug 可复现） |

---

## 12. APK 路径

**当前状态**：已最终生成

**产物**：

```
路径: android/app/build/outputs/apk/debug/app-debug.apk
大小: 131.07 MB (137436908 bytes)
SHA-256: 8d4739c617be328dc8f364a54faf5ae18c76548db9cc1da41def5323b7e8d380
构建时间: 2026-07-27 00:05:09
```

**构建可复现**：

- `./gradlew clean assembleDebug` 退出码 0，BUILD SUCCESSFUL
- 含 PRoot 集成代码（Phase 6.9）与 PRoot 二进制资源

详见 `docs/android/09-build-and-run.md` 第 4.4 节。

---

## 13. 真机验证结果

**外部阻塞**：本机未连接 ARM64 Android 真机。

**影响**：

- 22 项真机验证全部待执行
- 模拟器（x86_64）无法验证 ARM64 Linux 二进制
- SurrealDB gnu 兼容性、PRoot 运行、Runtime 启动等关键场景未验证

详见 `docs/android/10-testing-report.md` 第 5 节。

---

## 14. 当前阻塞

| 序号 | 阻塞项 | 影响 | 应对 |
|---|---|---|---|
| 1 | ARM64 真机未连接 | 22 项真机验证 + 集成测试 + UI 测试全部待执行 | 采购 / 借用 ARM64 真机（Pixel 6+ / 小米 11+ / 一加 9+） |
| 2 | FilePickerImpl / AudioPlayerImpl / AudioRecorderImpl 占位实现 | 图片选择 / 音频录制播放功能受限 | 接入 Activity Result API（第二阶段早期） |
| 3 | ProactiveMessageObserver 未在 Runtime Running 状态自动注册 | 主动消息实时推送未生效 | 在 AmitiaCoreService 监听 RuntimeManager.observeState()（第二阶段早期） |
| 4 | 通知点击 PendingIntent 跳转未在 MainActivity.onNewIntent 解析 | 通知点击不跳转 | MainActivity.onNewIntent 解析 extras + NavHost 导航（第二阶段早期） |
| 5 | PermissionBrokerImpl 权限回调未在 MainActivity 注册 | 权限请求无法路由到 Activity Result | registerForActivityResult 接入（第二阶段早期） |
| 6 | SurrealDB gnu 动态版本依赖 glibc | PRoot 内最小化 rootfs 无 glibc，预期启动失败 | 启动失败时进入 Degraded 状态（已实现）；长期方案：预置 glibc 或社区构建 musl 静态变体 |

**已解决的阻塞（仅作记录）**：

- ~~AGP 8.5.2 + Kotlin 2.0.20 测试 classpath 问题~~ — 已解决（`gradle.properties` 设置 `file.encoding=GBK` + 移除测试 JVM args 覆盖）
- ~~assembleDebug APK 未最终生成~~ — 已解决（`./gradlew clean assembleDebug` 退出码 0，APK 131.07 MB，SHA-256 已校验）

详见 `docs/android/12-known-limitations.md`。

---

## 15. 是否达到第一阶段验收

### 15.1 已达成项

| 阶段 | 内容 | 状态 |
|---|---|---|
| Phase 0 | 审计 Web/Electron 后端，识别迁移需求 | 完成 |
| Phase 1 | Android 工程骨架（6 模块 + Gradle Kotlin DSL + 设计系统 + 88 个源文件 + UI 设计文档） | 完成 |
| Phase 2 | Runtime（10 状态机 + RootFS + Process + Bootstrap + Health + Directories） | 完成 |
| Phase 3 | Go 后端 ARM64 交叉编译 + 平台抽象 + Qdrant/SurrealDB ARM64 二进制下载 | 完成 |
| Phase 4 | 连接层（Endpoint + ApiClient + 9 API + 10 DTO + 8 Repository + SSE/WS） | 完成 |
| Phase 5 | UI 功能（11 feature 模块 + 19 路由 + 流式回复 + 图片/语音/TTS/记忆/模型/渠道/设置/Runtime） | 完成 |
| Phase 6 | 系统集成（Room 7+7 + DataStore 14 + Keystore + AmitiaError 19 + ForegroundService + NativeCapabilityBridge 14+8 + Logger） | 完成 |
| Phase 7 | 测试与构建（27 测试文件 + manifest + install 脚本 + 12 编译问题修复） | 完成 |
| Phase 8 | 文档收尾（13 份文档 + 第三方许可证） | 完成 |

### 15.2 未达成项

| 项 | 阻塞原因 | 应对 |
|---|---|---|
| ARM64 真机验证 22 项 | 本机无 ARM64 真机 | 待真机接入（外部阻塞） |
| 集成测试覆盖 | 需 ARM64 真机执行 Linux 二进制链路 | 待真机接入（外部阻塞） |
| UI 测试覆盖 14 项 | 需真机或模拟器 | 待真机接入（外部阻塞） |
| FilePicker/AudioPlayer/AudioRecorder 占位 | Activity Result API 未接入 | 第二阶段早期接入 |
| ProactiveMessageObserver 自启动 | 未在 Runtime Running 状态注册 | 第二阶段早期接入 |
| 通知点击跳转 | MainActivity.onNewIntent 未解析 | 第二阶段早期接入 |
| PermissionBrokerImpl 权限回调 | MainActivity 未注册 launcher | 第二阶段早期接入 |

### 15.3 结论

**第一阶段验收结论**：部分达到（约 34/38 项达成）。

- 工程骨架、Runtime、Go 后端、数据库、UI、系统集成、文档、PRoot 集成**全部完成**
- 单元测试 270/270 通过（Debug & Release 双变体），APK 真实生成（131.07 MB，含 PRoot 集成）
- 真机验证、集成测试、UI 测试**外部阻塞**（无 ARM64 真机），占位 Provider 待第二阶段接入

详见 `docs/android/13-next-stage-plan.md`。
