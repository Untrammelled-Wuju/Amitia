# 13. 下一阶段计划

> 来源：Phase 0-7 实施成果 + Phase 8 已知限制
> 范围：第二阶段（Phase 9+）的技术规划，包括阻塞修复、真机验证、APK 生成、RootFS 优化、Provider 接入、能力扩展
> 引用：每项结论均给出 `file_path` 或对应文档
> change-id：`build-android-native-client`
> 生成时间：2026-07-26

---

## 1. 修复测试 classpath 问题（高优先级）

**目标**：解决 AGP 8.5.2 + Kotlin 2.0.20 测试 classpath 兼容性问题，使 `./gradlew test` 与 `./gradlew assembleDebug` 完整通过。

**方案选项**（按推荐度排序）：

### 1.1 升级 AGP 至 8.6+

**操作步骤**：

```toml
# android/gradle/libs.versions.toml
agp = "8.6.0"  # 或更高
```

**预期效果**：

- AGP 8.6 修复了与 Kotlin 2.0 测试 classpath 的兼容性问题
- 单元测试可正常执行
- `assembleDebug` 完整生成 APK

**风险**：

- AGP 8.6 要求 Gradle 8.7+
- 部分 AndroidX 库可能需要升级

### 1.2 降级 Kotlin 至 1.9.x

**操作步骤**：

```toml
# android/gradle/libs.versions.toml
kotlin = "1.9.24"
composeBom = "2024.06.00"  # 配套降级
```

**预期效果**：

- Kotlin 1.9.x 与 AGP 8.5.2 兼容性良好
- 测试 classpath 问题消失

**风险**：

- 失去 Kotlin 2.0 新特性（K2 编译器、Compose 编译器独立插件等）
- 部分依赖 Kotlin 2.0 API 的代码需要调整

### 1.3 等待社区修复

**操作步骤**：

- 持续关注 Kotlin 与 AGP 官方更新
- 等待兼容性补丁发布

**预期效果**：

- 不主动调整版本，等待官方修复

**风险**：

- 时间不可控
- 可能需要数周或数月

**建议**：优先采用 1.1 升级 AGP 方案。

---

## 2. ARM64 真机完整验证 22 项（高优先级）

**目标**：在 ARM64 真机上完成 22 项验证，覆盖功能、性能、稳定性。

**前置条件**：

- 测试 classpath 问题已解决
- Debug APK 已生成
- ARM64 真机已采购或借用

**验证清单**：

| 序号 | 验证项 | 验证方法 | 通过标准 |
|---|---|---|---|
| 1 | Android 版本兼容性 | minSdk 26 / targetSdk 34 | Android 8.0 ~ 14 全部启动正常 |
| 2 | CPU 架构 | `adb shell getprop ro.product.cpu.abi` | 返回 `arm64-v8a` |
| 3 | RootFS 解压 | 检查 `files/runtime/rootfs/bin/` | 包含 3 个可执行文件 |
| 4 | PRoot 运行 | `adb logcat -s proot` | 无 error 日志 |
| 5 | Qdrant 启动 | `curl http://127.0.0.1:19178/healthz` | 返回 200 |
| 6 | SurrealDB 启动 | `curl http://127.0.0.1:18000/health` | 返回 200 |
| 7 | Go 后端启动 | `curl http://127.0.0.1:18899/health` | 返回 200 |
| 8 | SQLite 持久化 | 重启 App 后查询 | 聊天历史与角色配置保留 |
| 9 | 网络连接 | ApiClient 访问 `127.0.0.1:18899` | 请求成功 |
| 10 | SSE 流式回复 | 发送消息 | 收到 `message_start` / `token` / `message_end` |
| 11 | WebSocket 主动消息 | 触发主动消息推送 | UI 实时收到 |
| 12 | 音频录制 | 录制语音 | 文件生成且可播放 |
| 13 | 音频播放 | 播放 TTS | 听到语音 |
| 14 | 图片选择 | 从相册选图 | 图片上传成功 |
| 15 | 后台保活 | App 退到后台 10 分钟 | Runtime 持续运行 |
| 16 | 前台服务常驻通知 | 检查通知栏 | 通知显示且不可滑动清除 |
| 17 | 系统杀进程恢复 | `adb shell am force-stop` 后重启 | 服务自动重建 |
| 18 | 重启数据恢复 | 杀进程后重启 App | 数据未丢失 |
| 19 | 低电量模式 | 启用 Doze 模式 | Runtime 不被限制（需电池优化白名单） |
| 20 | 网络切换 | WiFi ↔ 移动数据 | SSE 自动重连 |
| 21 | 屏幕旋转 | 横竖屏切换 | UI 状态保留 |
| 22 | 字体缩放 / 深色模式 | 系统设置变更 | UI 适配正常 |

**通过标准**：22 项全部通过即可发布 Beta 版本。

---

## 3. 生成正式 Debug APK + Release APK（高优先级）

**目标**：完整生成 Debug APK 与 Release APK，用于内测与正式发布。

**前置条件**：

- 测试 classpath 问题已解决
- ARM64 真机验证通过

**Debug APK**：

```bash
cd android
./gradlew assembleDebug
```

**输出**：

```
android/app/build/outputs/apk/debug/app-debug.apk
```

**Release APK**：

```bash
cd android
./gradlew assembleRelease
```

**输出**：

```
android/app/build/outputs/apk/release/app-release.apk
```

**Release 配置**（`android/app/build.gradle.kts:32-39`）：

- `isMinifyEnabled = true`
- `isShrinkResources = true`
- ProGuard 规则：`proguard-android-optimize.txt` + `proguard-rules.pro`
- 签名：当前使用 debug 签名（待后续配置正式签名）

**正式签名**（待后续阶段）：

- 生成正式 keystore
- 配置 `signingConfigs.release`
- 上传 keystore 到 CI/CD 系统（加密存储）

---

## 4. SurrealDB 切换 musl 静态版本（条件性）

**触发条件**：ARM64 真机验证第 6 项（SurrealDB 启动）失败，且错误为 glibc 版本不兼容。

**目标**：将 SurrealDB 替换为 musl 静态版本，消除 glibc 依赖。

**方案选项**：

### 4.1 社区构建版本

寻找第三方构建的 SurrealDB musl ARM64 版本，例如：

- `surreal-aarch64-unknown-linux-musl`
- 来自 Alpine Linux 包或社区发布

### 4.2 自行从源码编译

```bash
# 准备 Rust + 交叉编译工具链
rustup target add aarch64-unknown-linux-musl

# 克隆 SurrealDB 源码
git clone https://github.com/surrealdb/surrealdb.git
cd surrealdb

# 交叉编译
cargo build --release --target aarch64-unknown-linux-musl
```

**预期产物**：`surreal` 静态二进制，约 80-100 MB。

### 4.3 替换 RootFS 分发包

更新 `android/app/src/main/assets/surreal_linux_aarch64` 与 `rootfs-manifest.json`：

- 替换二进制文件
- 更新 SHA-256
- 更新 `rootfs-install-template.sh`（如有路径变化）

---

## 5. RootFS 体积优化（中优先级）

**目标**：将 APK 体积从 250 MB+ 降至 50 MB 以下，符合 Play Store 上架要求。

**方案选项**：

### 5.1 按需下载（推荐）

**实现思路**：

- APK 仅包含引导逻辑与小型配置
- 首次启动从 CDN 下载 RootFS（240 MB）到 `files/runtime/rootfs/`
- 下载进度显示在 OnboardingScreen

**优点**：

- APK 体积小（< 50 MB）
- 用户首次启动可看到进度
- 后续 RootFS 升级只需更新 CDN 资源

**缺点**：

- 首次启动需要网络
- 需要维护 CDN

### 5.2 AAB 分发

**实现思路**：

- 使用 Android App Bundle 按 ABI 切片
- arm64-v8a 用户仅下载 ARM64 资源
- x86_64 模拟器用户仅下载 x86_64 资源（如提供）

**优点**：

- Play Store 自动处理
- 用户下载体积减少

**缺点**：

- 仅适用于 Play Store 分发
- 需要为每个 ABI 准备完整 RootFS

### 5.3 APK Expansion File

**实现思路**：

- 将 RootFS 作为主扩展文件（`.obb`）打包
- APK 体积 < 100 MB
- 扩展文件最大 2 GB

**优点**：

- 突破 APK 单文件 200 MB 限制
- 兼容 Play Store 与第三方分发

**缺点**：

- 需处理 `.obb` 文件加载逻辑
- 用户首次启动可能需要解压

### 5.4 二进制压缩

**实现思路**：

- 使用 UPX 压缩 Go 后端二进制
- 预计可减少 30% 体积
- 不影响运行时性能（启动时解压到内存）

**优点**：

- 实现简单
- 兼容现有架构

**缺点**：

- 部分杀毒软件可能误报
- 启动时间略增

**建议**：组合使用方案 5.1（按需下载）+ 5.4（二进制压缩）。

---

## 6. FilePicker / AudioPlayer / AudioRecorder 完整接入 Activity Result（中优先级）

**目标**：将 3 个占位 Provider 接入实际 Activity Result API，实现完整功能。

### 6.1 FilePickerImpl

**实现思路**：

```kotlin
// 在 Compose 中调用
val launcher = rememberLauncherForActivityResult(
    contract = ActivityResultContracts.OpenDocument()
) { uri ->
    if (uri != null) {
        // 处理选择的文件
    }
}

// 触发选择
launcher.launch(arrayOf("image/*"))
```

**集成位置**：

- `feature/chat/media/ImagePickerButton.kt`
- 通过 Hilt 注入 `FilePicker` 接口

### 6.2 AudioPlayerImpl

**实现思路**：

```kotlin
// 使用 Media3 ExoPlayer
val player = ExoPlayer.Builder(context).build()
player.setMediaItem(MediaItem.fromUri(uri))
player.prepare()
player.play()
```

**集成位置**：

- `feature/chat/tts/TtsPlayerController.kt`
- 生命周期与 Compose `LocalLifecycleOwner` 绑定

### 6.3 AudioRecorderImpl

**实现思路**：

```kotlin
// 使用 MediaRecorder
val recorder = MediaRecorder().apply {
    setAudioSource(MediaRecorder.AudioSource.MIC)
    setOutputFormat(MediaRecorder.OutputFormat.MPEG_4)
    setAudioEncoder(MediaRecorder.AudioEncoder.AAC)
    setOutputFile(outputFile)
    prepare()
    start()
}
```

**集成位置**：

- `feature/chat/audio/AudioRecorderButton.kt`
- 通过 `PermissionBroker` 请求 `RECORD_AUDIO` 权限

---

## 7. ProactiveMessageObserver 自动启动 + 通知点击跳转（中优先级）

### 7.1 ProactiveMessageObserver 自动启动

**实现思路**：

```kotlin
// 在 AmitiaCoreService 中
runtimeManager.observeState().collectLatest { state ->
    when (state) {
        is RuntimeState.Running -> proactiveMessageObserver.start()
        is RuntimeState.Stopped, is RuntimeState.Failed -> proactiveMessageObserver.stop()
        else -> {}
    }
}
```

**集成位置**：

- `android/app/src/main/java/com/amitia/android/runtime/AmitiaCoreService.kt`

### 7.2 通知点击跳转

**实现思路**：

```kotlin
// 在 MainActivity 中
override fun onNewIntent(intent: Intent?) {
    super.onNewIntent(intent)
    val route = intent?.getStringExtra(EXTRA_ROUTE)
    if (route != null) {
        navController.navigate(route)
    }
}
```

**集成位置**：

- `android/app/src/main/java/com/amitia/android/MainActivity.kt`
- `AndroidManifest.xml` 中 MainActivity 启动模式改为 `singleTop`

---

## 8. 第二阶段规划

第二阶段（Phase 9+）扩展能力，覆盖以下方向：

### 8.1 高级系统能力

| 能力 | 实现方案 | 优先级 |
|---|---|---|
| Computer Use（PC 控制） | AccessibilityService + 跨设备协议 | 中 |
| Accessibility（无障碍服务） | AccessibilityService 实现 | 中 |
| MediaProjection（屏幕录制 / 投影） | MediaProjection API + 前台服务 | 中 |
| Shizuku（系统服务代理） | Shizuku 应用集成 + 权限申请 | 低 |
| Root（设备 Root） | Root 检测 + 直接执行二进制（绕过 PRoot） | 低 |

### 8.2 高级 AI 能力

| 能力 | 实现方案 | 优先级 |
|---|---|---|
| 本地模型推理 | llama.cpp / MNN / ONNX Runtime 集成 | 高 |
| 完整终端 UI | 终端模拟器（如 termux 组件） | 中 |
| MCP 市场 | 后端 MCP 模块 + Android UI 集成 | 中 |
| Skill 市场 | 与 AmitiaX 共用市场 | 低 |
| AmitiaX 扩展市场 | 完整扩展生态 | 低 |

### 8.3 高级 UI 功能

| 功能 | 实现方案 | 优先级 |
|---|---|---|
| 桌宠 | SYSTEM_ALERT_WINDOW + 独立窗口管理 | 中 |
| 全局悬浮助手 | SYSTEM_ALERT_WINDOW + 全局手势识别 | 中 |

### 8.4 性能优化

| 优化项 | 实现方案 | 优先级 |
|---|---|---|
| Root 设备绕过 PRoot | 检测 Root + 直接执行 Linux 二进制 | 中 |
| 端侧推理加速 | NDK + NEON 指令优化 | 中 |
| SSE 长连接保活 | WorkManager 周期性健康检查 | 低 |

### 8.5 工程化

| 项 | 实现方案 | 优先级 |
|---|---|---|
| CI/CD | GitHub Actions 自动构建 + 测试 | 高 |
| 签名自动化 | 加密 keystore + CI 注入 | 高 |
| 崩溃监控 | Sentry / Firebase Crashlytics | 中 |
| 性能监控 | Firebase Performance | 低 |

---

## 9. 时间线建议

### 9.1 第二阶段早期（1-2 周）

- 修复测试 classpath 问题
- 完整生成 Debug APK
- ARM64 真机验证 22 项
- FilePicker / AudioPlayer / AudioRecorder 接入
- ProactiveMessageObserver 自动启动
- 通知点击跳转解析

### 9.2 第二阶段中期（3-6 周）

- RootFS 体积优化（按需下载 + 二进制压缩）
- SurrealDB musl 切换（如真机验证失败）
- 本地模型推理集成
- MCP 市场集成

### 9.3 第二阶段晚期（7-12 周）

- Computer Use / Accessibility / MediaProjection
- 桌宠 / 全局悬浮助手
- CI/CD 与签名自动化
- 正式发布 v1.0

---

## 10. 风险与依赖

### 10.1 技术风险

| 风险 | 影响 | 应对 |
|---|---|---|
| ARM64 真机验证失败 | 影响 v1.0 发布 | 提前采购真机 |
| SurrealDB gnu 不兼容 | 需切换 musl | 准备源码编译方案 |
| 本地模型推理性能不达标 | 影响端侧 AI 体验 | 评估多种推理框架 |
| PRoot 性能损耗过大 | 影响用户体验 | 评估 Root 路线 |

### 10.2 外部依赖

| 依赖 | 用途 | 风险 |
|---|---|---|
| ARM64 真机 | 真机验证 | 需采购或借用 |
| CDN | RootFS 按需下载 | 需选择服务商 |
| Shizuku 应用 | 系统服务代理 | 第三方维护 |
| SurrealDB musl 版本 | 数据库兼容性 | 需自行编译 |

### 10.3 团队依赖

| 角色 | 职责 | 时间投入 |
|---|---|---|
| Android 工程师 | 功能开发与优化 | 全职 |
| Go 后端工程师 | 后端协同优化 | 兼职 |
| QA 工程师 | 真机测试与回归 | 兼职 |
| DevOps 工程师 | CI/CD 与签名 | 兼职 |

---

## 11. 验收标准

第二阶段完成时，应满足以下验收标准：

| 序号 | 标准 | 验证方法 |
|---|---|---|
| 1 | 测试 classpath 问题已解决 | `./gradlew test` 通过 |
| 2 | 22 项真机验证全部通过 | 真机测试报告 |
| 3 | Debug APK 与 Release APK 完整生成 | 构建产物检查 |
| 4 | RootFS 体积 < 100 MB | APK 体积检查 |
| 5 | FilePicker / AudioPlayer / AudioRecorder 功能完整 | 真机功能测试 |
| 6 | ProactiveMessageObserver 自动启动 | 真机推送测试 |
| 7 | 通知点击跳转 | 真机通知测试 |
| 8 | 本地模型推理可运行 | 端侧推理 demo |
| 9 | CI/CD 自动化 | GitHub Actions 配置检查 |
| 10 | 正式签名配置 | keystore 检查 |

满足全部 10 项即可发布 v1.0 正式版。
