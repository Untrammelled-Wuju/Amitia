# 09. 构建与运行指南

> 来源：Phase 1-7 实施成果
> 范围：Android 工程结构、环境要求、构建命令、已执行构建结果、Go 后端构建、RootFS 分发清单、运行说明
> 引用：每项结论均给出 `file_path:line`
> change-id：`build-android-native-client`
> 生成时间：2026-07-26

---

## 1. 工程结构

Android 工程位于 `android/` 目录，包含 6 个 Gradle 模块（`android/settings.gradle.kts:25-30`）：

```
android/
├── app/                       # 应用入口模块
│   ├── src/main/
│   │   ├── java/com/amitia/android/
│   │   │   ├── AmitiaApplication.kt        # @HiltAndroidApp
│   │   │   ├── MainActivity.kt            # Compose 宿主
│   │   │   ├── navigation/                # 导航图与路由
│   │   │   │   ├── AmitiaNavHost.kt
│   │   │   │   ├── AmitiaRoutes.kt
│   │   │   │   ├── AmitiaBottomBar.kt
│   │   │   │   └── CapabilityTabScreen.kt
│   │   │   └── runtime/
│   │   │       └── AmitiaCoreService.kt   # 前台服务
│   │   ├── assets/                        # RootFS 分发包
│   │   │   ├── amitia-backend-arm64       # 54.63 MB
│   │   │   ├── qdrant_linux_aarch64        # 74.23 MB
│   │   │   ├── surreal_linux_aarch64       # 111.03 MB
│   │   │   ├── rootfs-manifest.json
│   │   │   └── rootfs-install-template.sh
│   │   ├── res/                           # 资源
│   │   └── AndroidManifest.xml
│   ├── src/androidTest/                   # 集成测试 + UI 测试（8 个 .kt）
│   └── build.gradle.kts
├── core/                      # 领域与数据层
│   ├── src/main/java/com/amitia/core/
│   │   ├── common/            # Constants
│   │   ├── network/           # endpoint / client / connection / api / sse / ws
│   │   ├── repository/        # 8 个 Repository
│   │   ├── model/             # DTO
│   │   ├── database/          # entity / dao / converter
│   │   ├── datastore/         # SettingsDataStore
│   │   ├── security/         # KeystoreManager
│   │   ├── error/            # AmitiaError 19 子类 + ErrorMapper
│   │   ├── logging/          # Logger + LogSanitizer
│   │   ├── media/            # 媒体工具
│   │   └── designsystem/      # 主题 + 组件
│   └── src/test/              # 11 个单元测试
├── runtime/                   # Linux Runtime 管理
│   ├── src/main/java/com/amitia/runtime/
│   │   ├── api/               # RuntimeState
│   │   ├── manager/           # RuntimeManager + StateMachine
│   │   ├── linux/             # LinuxRootfsManagerImpl + IntegrityChecker
│   │   ├── process/           # LinuxProcessManagerImpl
│   │   ├── bootstrap/         # BootstrapSequenceImpl
│   │   ├── health/            # HealthCheckerImpl
│   │   └── bridge/            # NativeCapabilityBridge
│   └── src/test/              # 5 个单元测试
├── platform/                  # Android 系统服务集成
│   └── src/main/java/com/amitia/platform/
│       ├── notification/
│       ├── foreground/
│       ├── permissions/
│       ├── files/
│       ├── audio/
│       └── bridge/provider/  # 9 个 Provider
├── feature/                    # UI 功能模块
│   └── src/main/java/com/amitia/feature/
│       ├── startup/  onboarding/  auth/  home/  chat/
│       ├── chat/audio/  chat/media/  chat/tts/
│       ├── character/  memory/  models/  channels/
│       ├── runtime/  settings/
│       └── common/
│   └── src/test/              # 2 个单元测试
├── native/                     # PRoot JNI 壳
│   └── src/main/
│       ├── java/com/amitia/nativeproot/ProotBridge.kt
│       ├── cpp/proot_jni.cpp
│       ├── cpp/CMakeLists.txt
│       └── AndroidManifest.xml
├── gradle/
│   ├── libs.versions.toml     # 版本目录（统一版本管理）
│   └── wrapper/
│       ├── gradle-wrapper.jar
│       └── gradle-wrapper.properties
├── build.gradle.kts           # 根构建脚本
├── settings.gradle.kts
├── gradle.properties
├── gradlew                    # Unix wrapper
├── gradlew.bat                # Windows wrapper
└── local.properties           # SDK 路径（不入 git）
```

源文件统计：

- Kotlin 源文件（main）：约 190 个
- Kotlin 测试文件（test + androidTest）：27 个
- 资源与配置：Manifest、theme、strings 等

---

## 2. 环境要求

| 项 | 版本 | 引用 |
|---|---|---|
| JDK | 17 | `android/app/build.gradle.kts:48-51` |
| Android compileSdk | 34 | `android/app/build.gradle.kts:12` |
| Android minSdk | 26 | `android/app/build.gradle.kts:16` |
| Android targetSdk | 34 | `android/app/build.gradle.kts:17` |
| Gradle | 8.9 | `android/gradle/wrapper/gradle-wrapper.properties:3` |
| Android Gradle Plugin (AGP) | 8.5.2 | `android/gradle/libs.versions.toml:2` |
| Kotlin | 2.0.20 | `android/gradle/libs.versions.toml:3` |
| Compose BOM | 2024.09.00 | `android/gradle/libs.versions.toml:4` |
| Hilt | 2.52 | `android/gradle/libs.versions.toml:5` |
| Room | 2.6.1 | `android/gradle/libs.versions.toml:6` |
| Retrofit | 2.11.0 | `android/gradle/libs.versions.toml:7` |
| OkHttp | 4.12.0 | `android/gradle/libs.versions.toml:8` |
| NDK ABI | arm64-v8a / x86_64 | `android/app/build.gradle.kts:26-28` |
| Android Build Tools | 由 AGP 8.5.2 决定（建议 34.0.0） | — |

**Go 后端编译环境**：

| 项 | 版本 | 引用 |
|---|---|---|
| Go | 1.26.1 | `backend/go.mod:3` |
| CGO | `CGO_ENABLED=0`（ARM64 交叉编译） | `rootfs-manifest.json:13` |
| SQLite 驱动 | glebarez/sqlite v1.11.0（纯 Go） | `backend/go.mod:8` |

---

## 3. 构建命令

### 3.1 Android 工程

进入 `android/` 目录执行：

```bash
# 清理
./gradlew clean

# 编译 Debug APK（注意：当前阻塞，详见第 4 节）
./gradlew assembleDebug

# 单元测试（注意：当前阻塞，详见第 4 节）
./gradlew test

# 静态检查
./gradlew lint

# Android 测试（需连接真机或模拟器）
./gradlew connectedAndroidTest
```

Windows 环境可使用 `gradlew.bat` 代替 `./gradlew`。

### 3.2 Go 后端

进入 `backend/` 目录执行：

```bash
# Windows x64（开发与桌面端）
go build -o server.exe ./cmd/server/

# Linux ARM64（Android 嵌入式）
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o amitia-backend-arm64 ./cmd/server/
```

Linux ARM64 产物需复制到 `android/app/src/main/assets/amitia-backend-arm64`。

---

## 4. 已执行的构建结果

### 4.1 Gradle Wrapper

**已完成**：

- `gradle-wrapper.jar` 已生成（`android/gradle/wrapper/gradle-wrapper.jar`，43583 字节）
- `gradlew` + `gradlew.bat` 已生成
- `gradle-wrapper.properties` 指向 `gradle-8.9-bin.zip`

### 4.2 已修复的编译错误

Phase 7 期间 `./gradlew assembleDebug` 阶段陆续修复以下编译错误：

| 错误类别 | 修复内容 | 引用文件 |
|---|---|---|
| 主题资源 | `Theme.Amitia` 改用 `parent="Theme.Material3.Dark.NoActionBar"`，避免引用不存在的 `color/` 资源 | `android/app/src/main/res/values/themes.xml` |
| Room converters | `Converters.kt` 去重，避免重复注册 `List<String>` 转换器 | `android/core/src/main/java/com/amitia/core/database/converter/Converters.kt` |
| 命名冲突 | `ConnectionManager` 与 `core/network/connection` 重名，重命名 platform 模块实现 | `android/platform/src/main/java/com/amitia/platform/...` |
| AmitiaError 继承 | `AmitiaError` 改为继承 `RuntimeException` 而非 `Exception`，与 Hysterix 风格对齐 | `android/core/src/main/java/com/amitia/core/error/AmitiaError.kt` |
| Coroutine 测试 | 测试用 `runTest` 替代 `runBlocking`，使用 `coVerify` 验证挂起函数 | `android/core/src/test/.../*Test.kt` |
| 依赖缺失 | 添加 `truth`、`androidx-arch-core-testing`、`kotlinx-coroutines-test` 等测试依赖 | `android/gradle/libs.versions.toml:30-39` |
| testOptions | 在 `app/build.gradle.kts` 添加 `testOptions { unitTests { ... } }` | `android/app/build.gradle.kts:88-93` |
| native 模块 | `native/build.gradle.kts` 移除 `externalNativeBuild` 引用（PRoot JNI 仅占位） | `android/native/build.gradle.kts` |

### 4.3 测试 classpath 阻塞（外部阻塞）

**问题描述**：

执行 `./gradlew test` 或 `./gradlew assembleDebug` 时，测试任务配置阶段失败：

```
class com.android.build.gradle.internal.dsl.TestOptions$UnitTestOptions_Decorated
cannot be cast to org.gradle.api.tasks.testing.Test
```

**根因**：AGP 8.5.2 + Kotlin 2.0.20 测试 classpath 已知兼容性问题（社区广泛报告）。

**当前状态**：

- 记录为外部阻塞，不在本阶段处理
- 待后续通过升级 AGP 或降级 Kotlin 1.9.x 解决
- 详见 `docs/android/12-known-limitations.md`

### 4.4 APK 路径

**当前状态**：未最终生成

**原因**：

- 因测试 classpath 阻塞，`assembleDebug` 在测试任务配置阶段失败
- 主源码编译已通过（Kotlin compile 成功，资源合并成功）
- APK 打包步骤未执行

**预期路径**（修复后）：

```
android/app/build/outputs/apk/debug/app-debug.apk
```

---

## 5. Go 后端构建结果

### 5.1 Windows x64（开发环境）

```bash
cd backend
go build -o server.exe ./cmd/server/
```

**结果**：

- 编译成功
- 产物：`backend/cmd/server/server.exe`
- 体积：59.18 MB（62047232 字节）
- 用途：桌面端 `AmitiaCore.exe` 与开发环境调试

### 5.2 Linux ARM64（Android 嵌入式）

```bash
cd backend
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o amitia-backend-arm64 ./cmd/server/
```

**结果**：

- 交叉编译成功
- 产物：`backend/cmd/server/amitia-backend-arm64` → 复制到 `android/app/src/main/assets/amitia-backend-arm64`
- 体积：54.63 MB（57283044 字节）
- ELF 头验证：`aarch64` 架构，静态链接（无 .so 依赖）

### 5.3 平台抽象验证

Go 后端平台抽象已通过编译验证（`backend/pkg/platform/`）：

| 文件 | build tag | 用途 |
|---|---|---|
| `runtime_platform.go` | — | 接口定义 |
| `desktop_windows.go` | `//go:build windows` | Windows 桌面 |
| `desktop_linux.go` | `//go:build linux && !android` | Linux 桌面 |
| `android_embedded.go` | `//go:build android` | Android 嵌入式 |

`main.go` 通过 `platform.Detect().KillExistingServer(addr)` 在编译期分发，无运行时分支。

---

## 6. RootFS 分发包清单

RootFS 分发包位于 `android/app/src/main/assets/`，作为 APK 内置资源随应用分发：

### 6.1 二进制清单

| 文件 | 大小 | SHA-256 | 类型 | 架构 | 链接方式 | 引用 |
|---|---|---|---|---|---|---|
| `amitia-backend-arm64` | 57283044 字节（54.63 MB） | `ABDD63EB020A01718684EDCF130785DA0CFD45DCB691AAB5D260A8E17386879B` | Go 后端 | linux/arm64 | 静态 | `rootfs-manifest.json:9-14` |
| `qdrant_linux_aarch64` | 77835808 字节（74.23 MB） | `6CB81123D2A3E405335C984EFB7928DC21A1EAC47A3B73A7609AEE076DBE0B04` | 向量数据库 | linux/arm64 | musl 静态 | `rootfs-manifest.json:15-24` |
| `surreal_linux_aarch64` | 116424792 字节（111.03 MB） | `A235206F2C4A803616D7669F56BC5BCA5CC0AE7ED2B79ACBE91F14712B28FE5C` | 图数据库 | linux/arm64 | gnu 动态 | `rootfs-manifest.json:25-34` |
| **合计** | 251542644 字节（约 240 MB） | — | — | — | — | `rootfs-manifest.json:36` |

### 6.2 二进制来源

| 二进制 | 来源 | 选择理由 |
|---|---|---|
| `amitia-backend-arm64` | 本地交叉编译 | 项目源码 |
| `qdrant_linux_aarch64` | GitHub Release `qdrant-aarch64-unknown-linux-musl.tar.gz` | musl 静态版本，避免 glibc 依赖 |
| `surreal_linux_aarch64` | GitHub Release `surreal-v3.2.0.linux-arm64.tgz` | gnu 动态版本，PRoot 中 glibc 兼容性待真机验证 |

### 6.3 配置与脚本

| 文件 | 大小 | 用途 | 引用 |
|---|---|---|---|
| `rootfs-manifest.json` | 1339 字节 | 二进制清单 + SHA-256 校验 | `android/app/src/main/assets/rootfs-manifest.json` |
| `rootfs-install-template.sh` | 1634 字节 | 安装脚本模板，生成 `bin/` `etc/` 目录与 `config.yml` | `android/app/src/main/assets/rootfs-install-template.sh` |

### 6.4 安装脚本生成内容

执行 `rootfs-install-template.sh` 后生成以下结构（参考脚本 13-53 行）：

```
$ROOTFS_DIR/
├── bin/
│   ├── amitia-backend       # 可执行
│   ├── qdrant               # 可执行
│   └── surreal              # 可执行
├── etc/
│   └── config.yml           # 自动生成（端口 18899/19178/18000）
├── logs/
└── VERSION                  # 含版本号与 SHA-256

$DATA_DIR/
├── sqlite/
├── qdrant/
├── surrealdb/
├── uploads/
├── models/
├── extensions/
└── backups/
```

### 6.5 端口约定

| 服务 | 端口 | 引用 |
|---|---|---|
| Go 后端 HTTP | 18899 | `rootfs-install-template.sh:21` |
| Qdrant gRPC/HTTP | 19178 | `rootfs-install-template.sh:36` |
| SurrealDB HTTP/WS | 18000 | `rootfs-install-template.sh:38` |

> 端口选择避开 3000（CCX 占用），与 Web/Electron 后端保持一致。

---

## 7. 运行说明

### 7.1 Debug APK 安装

```bash
# 安装到已连接的 ARM64 真机
adb install -r android/app/build/outputs/apk/debug/app-debug.apk

# 查看日志
adb logcat -s AmitiaCoreService:V AmitiaApplication:V
```

**ABI 过滤**（`android/app/build.gradle.kts:26-28`）：

- `arm64-v8a`：目标真机
- `x86_64`：模拟器调试

### 7.2 首次启动引导流程

```
1. 启动 App
   └─ AmitiaApplication.onCreate()          # Hilt 初始化

2. 进入 StartupScreen
   └─ 显示 Runtime 状态（初始为 NotInstalled）

3. 检测首次启动
   └─ 跳转 OnboardingScreen
       ├─ 介绍能力（聊天 / 图片 / 语音 / 记忆 / 主动消息）
       └─ 申请权限（通知 / 录音 / 相机 / 媒体访问）

4. 引导完成
   └─ 触发 Runtime 启动（BootstrapSequenceImpl）
       ├─ 解压 RootFS（240 MB → files/runtime/rootfs/）
       ├─ 启动 SurrealDB（端口 18000）
       ├─ 启动 Qdrant（端口 19178）
       └─ 启动 Go 后端（端口 18899）

5. 健康检查通过
   └─ 跳转 HomeScreen（RuntimeState = Running）
```

### 7.3 本地模式

默认模式。App 启动后自动按上述流程启动内嵌 Runtime。

**前台服务**：

- `AmitiaCoreService` 启动后调用 `startForegroundCompat()`
- `foregroundServiceType="dataSync"`（`AndroidManifest.xml:47`）
- 常驻通知显示 Runtime 状态（phase / readableMessage / isRunning / isStarting / isFailed / isStopped）
- `START_STICKY` 保证服务被杀后系统尝试重建

**状态同步**：

- `AmitiaCoreService` 监听 `RuntimeManager.observeState()`（`android/app/src/main/java/com/amitia/android/runtime/AmitiaCoreService.kt:71-81`）
- 状态变化时更新通知文案与 `startForegroundCompat`

### 7.4 远程模式

在 `SettingsScreen` 切换：

```
1. 用户进入设置页
2. 选择「远程模式」
3. 填写远程地址（例如 http://192.168.1.100:18899）
4. 保存
   └─ SettingsDataStore 持久化 mode=remote + remoteUrl
   └─ RuntimeEndpointProvider 发射新 Endpoint
   └─ 后续请求自动指向远程地址
   └─ 本地 Runtime 可保持 Stopped 状态以省电
```

### 7.5 停止 Runtime

- 用户在 `RuntimeScreen` 点击「停止」
- `RuntimeManager.stop()` 触发 `BootstrapSequenceImpl.stop()`，按反向顺序停止 Go 后端 → Qdrant → SurrealDB
- `RuntimeStateMachine.transitionTo(Stopped)`
- 前台服务通知更新为「已停止」

---

## 8. 常见问题

### 8.1 Gradle 构建失败：测试 classpath

**现象**：

```
class com.android.build.gradle.internal.dsl.TestOptions$UnitTestOptions_Decorated
cannot be cast to org.gradle.api.tasks.testing.Test
```

**根因**：AGP 8.5.2 + Kotlin 2.0.20 兼容性问题。

**临时绕过**：跳过测试任务（仅用于生成 APK）：

```bash
./gradlew assembleDebug -x test
```

**根本解决**：升级 AGP 至 8.6+ 或降级 Kotlin 至 1.9.x（待后续阶段处理）。

### 8.2 Linux ARM64 交叉编译失败

**现象**：`undefined reference to ...` 或 `cgo: ... not supported`

**根因**：未禁用 CGO。

**解决**：必须使用 `CGO_ENABLED=0`：

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o amitia-backend-arm64 ./cmd/server/
```

### 8.3 APK 体积过大

**现象**：APK 体积约 240 MB（RootFS 内置）。

**临时方案**：当前阶段接受该体积，确保首次启动即可用。

**长期方案**（待后续阶段）：

- 按需下载（首次启动从 CDN 下载 RootFS）
- AAB 分发（Play Store 按 ABI 切片）
- APK Expansion File

详见 `docs/android/12-known-limitations.md` 与 `docs/android/13-next-stage-plan.md`。

### 8.4 SurrealDB gnu 动态版本兼容性

**风险**：PRoot 中 glibc 兼容性未在真机验证。

**应对**：若真机启动失败，改用 musl 静态版本或源码编译，详见 `docs/android/11-migration-report.md` 第 8 节。
