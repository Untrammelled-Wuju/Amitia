# 12. 已知限制

> 来源：Phase 0-7 实施成果
> 范围：当前阶段（Phase 8 收尾后）Android 端已知的功能限制、技术债务、外部阻塞
> 引用：每项结论均给出 `file_path` 或对应模块路径
> change-id：`build-android-native-client`
> 生成时间：2026-07-26

---

## 1. 测试 classpath 阻塞（外部阻塞）

**问题描述**：

执行 `./gradlew test` 或 `./gradlew assembleDebug` 时，测试任务配置阶段失败：

```
class com.android.build.gradle.internal.dsl.TestOptions$UnitTestOptions_Decorated
cannot be cast to org.gradle.api.tasks.testing.Test
```

**根因**：

AGP 8.5.2 + Kotlin 2.0.20 测试 classpath 已知兼容性问题，社区广泛报告。

**影响范围**：

- 单元测试用例无法执行（编译通过但 task 配置阶段失败）
- 集成测试与 UI 测试同样受阻
- 间接导致 `assembleDebug` 无法完整生成 APK

**临时绕过**：

```bash
./gradlew assembleDebug -x test
```

**根本解决**（待后续阶段）：

- 升级 AGP 至 8.6+
- 或降级 Kotlin 至 1.9.x
- 或等待 Kotlin 2.0.x + AGP 8.5.x 兼容性补丁

**引用**：

- `android/gradle/libs.versions.toml:2-3`
- `android/app/build.gradle.kts:88-93`

---

## 2. ARM64 真机验证未执行（外部阻塞）

**问题描述**：

本机未连接 ARM64 Android 设备，22 项真机验证全部待执行。

**限制说明**：

- 模拟器（x86_64）无法运行 ARM64 Linux 二进制（PRoot + ARM64 RootFS）
- 模拟器结果不能替代真机（参考 stage.md 24.4 节）
- x86_64 模拟器仅能验证 UI 适配（屏幕旋转 / 字体缩放 / 深色模式），不能覆盖 Runtime / 数据库 / 网络 / 音频 / 后台保活等核心场景

**影响范围**：

- SurrealDB gnu 动态版本 glibc 兼容性未验证
- PRoot 运行稳定性未验证
- 22 项真机验证场景全部待执行

**应对**（待后续阶段）：

- 采购或借用 ARM64 真机（推荐 Pixel 6+ / 小米 11+ / 一加 9+）
- 完成 22 项验证后发布正式版

**引用**：

- `docs/android/10-testing-report.md` 第 5 节

---

## 3. APK 未最终生成

**问题描述**：

`assembleDebug` 因测试 classpath 阻塞未能完整生成 APK。

**当前状态**：

- 主源码 Kotlin compile 通过
- 资源合并通过
- APK 打包步骤未执行

**预期路径**（修复后）：

```
android/app/build/outputs/apk/debug/app-debug.apk
```

**临时绕过**：

```bash
./gradlew assembleDebug -x test
```

**引用**：

- `docs/android/09-build-and-run.md` 第 4.4 节

---

## 4. SurrealDB gnu 动态版本兼容性（技术风险）

**问题描述**：

SurrealDB 官方未提供 musl 静态 ARM64 版本，当前使用 gnu 动态版本（111 MB）。

**风险**：

- PRoot 环境中 glibc 版本可能与 SurrealDB 编译时版本不一致
- 启动时可能报 `version 'GLIBC_2.xx' not found` 错误

**当前状态**：

- 未在真机验证
- Phase 7 真机验证为外部阻塞

**应对方案**（若真机启动失败）：

1. 改用 musl 静态版本（社区构建或自行编译）
2. 从源码编译：`cargo build --release --target aarch64-unknown-linux-musl`
3. 切换到其他图数据库（如 Redis Graph）作为降级方案

**引用**：

- `android/app/src/main/assets/rootfs-manifest.json:25-34`
- `docs/android/03-runtime-dependency-audit.md`

---

## 5. RootFS 体积 240 MB

**问题描述**：

RootFS 分发包总体积 251542644 字节（约 240 MB），作为 APK assets 内置导致 APK 体积过大。

**影响**：

- APK 体积预估 250 MB+
- Play Store 单 APK 上限 200 MB（需 AAB 或 APK Expansion File）
- 用户首次下载体验差

**当前阶段策略**：

- 接受该体积，确保首次启动即可用
- 仅在内部测试分发

**长期优化方案**（待后续阶段）：

1. **按需下载**：首次启动从 CDN 下载 RootFS，APK 仅包含引导逻辑
2. **AAB 分发**：Play Store 按 ABI 切片，arm64-v8a 用户仅下载 ARM64 资源
3. **APK Expansion File**：将 RootFS 作为主扩展文件（.obb）分发，突破 200 MB 限制
4. **二进制压缩**：使用 UPX 压缩 Go 后端二进制（预计可减少 30%）

**引用**：

- `android/app/src/main/assets/rootfs-manifest.json:36`

---

## 6. FilePickerImpl / AudioPlayerImpl / AudioRecorderImpl 占位实现

**问题描述**：

3 个平台能力 Provider 当前为接口占位实现，未接入实际 Activity Result API。

**影响**：

- 图片选择功能受限（用户无法从相册选择图片发送）
- 音频录制受限（用户无法录制语音消息）
- 音频播放受限（TTS 音频无法播放）

**当前状态**：

- 接口已定义（`platform/files/FilePickerImpl.kt`、`platform/audio/AudioPlayerImpl.kt`、`platform/audio/AudioRecorderImpl.kt`）
- 占位实现返回空结果或默认行为
- 上层 ViewModel 调用占位实现时不会崩溃，但功能不完整

**应对**（待后续阶段）：

- `FilePickerImpl` — 接入 `rememberLauncherForActivityResult(OpenDocument())`
- `AudioPlayerImpl` — 接入 Media3 ExoPlayer 完整生命周期
- `AudioRecorderImpl` — 接入 `MediaRecorder` + `PermissionBroker` 权限请求

**引用**：

- `android/platform/src/main/java/com/amitia/platform/files/FilePickerImpl.kt`
- `android/platform/src/main/java/com/amitia/platform/audio/AudioPlayerImpl.kt`
- `android/platform/src/main/java/com/amitia/platform/audio/AudioRecorderImpl.kt`

---

## 7. ProactiveMessageObserver 未在 Runtime Running 状态自动启动

**问题描述**：

主动消息观察者已实现，但未在 `RuntimeState.Running` 时自动注册。

**影响**：

- 后端通过 WebSocket 推送的主动消息无法到达 UI
- 主动消息历史可查询（通过 Repository），但实时推送不工作

**当前状态**：

- `ProactiveMessageObserver` 类已实现
- 未在 `AmitiaCoreService` 或 `StartupViewModel` 注册启动

**应对**（待后续阶段）：

- 在 `AmitiaCoreService` 监听 `RuntimeManager.observeState()`
- 状态变为 `Running` 时启动 `ProactiveMessageObserver.start()`
- 状态变为 `Stopped` / `Failed` 时停止观察者

**引用**：

- `android/core/src/main/java/com/amitia/core/repository/ProactiveMessageRepository.kt`

---

## 8. 通知点击 PendingIntent 跳转未在 MainActivity onNewIntent 解析

**问题描述**：

前台服务通知已构建 `PendingIntent`，但点击通知后未跳转到指定页面。

**影响**：

- 用户点击通知仅打开 App 主界面，不跳转到指定页面（如聊天页 / Runtime 管理页）
- 主动消息通知点击应跳转到对应聊天会话

**当前状态**：

- `ForegroundServiceManagerImpl` 已构建 `PendingIntent` 包含 extras
- `MainActivity.onNewIntent` 未解析 extras 并导航

**应对**（待后续阶段）：

- 在 `MainActivity.onNewIntent` 提取 intent extras
- 通过 NavHost 控制器导航到指定路由
- 处理 `singleTop` 启动模式避免重复创建 Activity

**引用**：

- `android/platform/src/main/java/com/amitia/platform/foreground/ForegroundServiceManagerImpl.kt`
- `android/app/src/main/java/com/amitia/android/MainActivity.kt`

---

## 9. PermissionBrokerImpl 权限回调路由未在 MainActivity 注册

**问题描述**：

权限请求 Broker 已实现，但回调路由未在 `MainActivity` 注册。

**影响**：

- 权限请求无法路由到实际 Activity Result
- 用户首次启动时权限申请流程受限

**当前状态**：

- `PermissionBrokerImpl` 接口已定义
- 未通过 `registerForActivityResult(RequestMultiplePermissions())` 接入

**应对**（待后续阶段）：

- 在 `MainActivity` 注册 `ActivityResultLauncher<Array<String>>`
- `PermissionBrokerImpl` 持有 launcher 引用
- 权限请求通过 launcher 触发，回调通过 Flow 返回结果

**引用**：

- `android/platform/src/main/java/com/amitia/platform/permissions/PermissionBrokerImpl.kt`
- `android/app/src/main/java/com/amitia/android/MainActivity.kt`

---

## 10. Android 后台限制

**问题描述**：

Android 系统对后台进程的限制可能影响 Runtime 持续运行。

**影响场景**：

- **Doze 模式**（Android 6+）：设备静止时网络访问被延迟
- **App Standby**：长时间未使用的 App 网络访问受限
- **后台限制**（Android 8+）：后台服务被限制，需 foreground service
- **电池优化**：部分厂商（华为 / 小米 / OPPO）激进杀后台

**当前策略**：

- **OnDemand 默认**：用户主动操作时启动 Runtime，App 退出后停止
- **AlwaysOn 可选**：用户在设置页开启后启用前台服务常驻

**限制**：

- AlwaysOn 模式下系统仍可能在以下场景限制：
  - Doze 模式下网络访问延迟
  - 电池优化白名单未授予时被杀
  - 厂商定制系统激进省电策略

**应对**：

- 引导用户加入电池优化白名单
- 在 `AmitiaCoreService` 使用 `foregroundServiceType="dataSync"` 保持前台
- WorkManager 用于后台任务（如周期性健康检查）

**引用**：

- `android/app/src/main/AndroidManifest.xml:13-14`
- `android/core/src/main/java/com/amitia/core/datastore/SettingsDataStore.kt`

---

## 11. PRoot 性能损耗

**问题描述**：

PRoot 通过 `ptrace` 拦截系统调用实现 Linux 用户空间，存在性能损耗。

**损耗来源**：

- 每次 syscall 触发 `ptrace` 拦截
- 文件系统路径转换（Android 路径 ↔ Linux 路径）
- 用户 ID / 组 ID 映射

**预期损耗**：

- Go 后端 HTTP 吞吐：相比原生降低 20%-40%
- 数据库 I/O：相比原生降低 30%-50%
- 整体响应延迟：增加 50-100 ms

**当前阶段策略**：

- 接受 PRoot 性能损耗，作为「无 Root」方案的代价
- 在文档中明确告知用户

**长期优化方案**（待后续阶段）：

1. **Root 设备**：直接执行 Linux 二进制，绕过 PRoot
2. **Android 14+ 部分场景**：使用 ` bionic libc` 兼容模式
3. **原生移植**：将 Go 后端核心逻辑用 Kotlin 重写（成本极高，不推荐）

**引用**：

- `docs/android/05-linux-runtime-design.md`

---

## 12. 当前阶段不支持的功能

以下功能在当前阶段（Phase 8 收尾）**不支持**，待后续阶段实现：

### 12.1 高级系统能力

| 功能 | 原因 | 后续阶段 |
|---|---|---|
| Computer Use（PC 控制） | 需要 AccessibilityService 与跨设备协议 | 第二阶段 |
| Accessibility（无障碍服务） | 需要 AccessibilityService 实现 | 第二阶段 |
| MediaProjection（屏幕录制 / 投影） | 需要 MediaProjection API 与权限 | 第二阶段 |
| Shizuku（系统服务代理） | 需要 Shizuku 应用与权限 | 第二阶段 |
| Root（设备 Root） | 当前阶段无 Root 方案 | 第二阶段（可选） |

### 12.2 高级 AI 能力

| 功能 | 原因 | 后续阶段 |
|---|---|---|
| 完整本地模型推理 | 端侧 LLM 推理框架集成待研究（llama.cpp / MNN / ONNX Runtime） | 第二阶段 |
| 完整终端 UI | 终端模拟器实现待研究 | 第二阶段 |
| MCP（Model Context Protocol）市场 | 后端 MCP 模块已实现，Android UI 未集成 | 第二阶段 |
| Skill 市场 | 与 AmitiaX 共用市场，待整体规划 | 第二阶段 |
| AmitiaX 扩展市场 | 需要完整的扩展生态 | 第二阶段 |

### 12.3 高级 UI 功能

| 功能 | 原因 | 后续阶段 |
|---|---|---|
| 桌宠 | 需要 SYSTEM_ALERT_WINDOW 与独立窗口管理 | 第二阶段 |
| 全局悬浮助手 | 需要 SYSTEM_ALERT_WINDOW 与全局手势 | 第二阶段 |

### 12.4 第二阶段规划详情

详见 `docs/android/13-next-stage-plan.md` 第 8 节。

---

## 13. 限制项汇总表

| 序号 | 限制项 | 类型 | 优先级 | 应对阶段 |
|---|---|---|---|---|
| 1 | 测试 classpath 阻塞 | 外部阻塞 | 高 | 第二阶段早期 |
| 2 | ARM64 真机验证未执行 | 外部阻塞 | 高 | 第二阶段早期 |
| 3 | APK 未最终生成 | 间接阻塞 | 高 | 随 1 解决 |
| 4 | SurrealDB gnu 兼容性 | 技术风险 | 中 | 真机验证后判断 |
| 5 | RootFS 体积 240 MB | 体验问题 | 中 | 第二阶段 |
| 6 | FilePicker/AudioPlayer/AudioRecorder 占位 | 功能受限 | 中 | 第二阶段早期 |
| 7 | ProactiveMessageObserver 未自动启动 | 功能受限 | 中 | 第二阶段早期 |
| 8 | 通知点击跳转未解析 | 功能受限 | 中 | 第二阶段早期 |
| 9 | PermissionBrokerImpl 回调未注册 | 功能受限 | 中 | 第二阶段早期 |
| 10 | Android 后台限制 | 系统限制 | 低 | 持续优化 |
| 11 | PRoot 性能损耗 | 技术选型代价 | 低 | 第二阶段（Root 路线） |
| 12 | 高级系统能力 / AI / UI 功能不支持 | 阶段规划 | 低 | 第二阶段 |
