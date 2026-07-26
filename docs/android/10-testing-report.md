# 10. 测试报告

> 来源：Phase 7 测试与构建成果
> 范围：单元测试清单、集成测试清单、UI 测试清单、测试执行结果、ARM64 真机验证、已修复问题清单、待后续处理
> 引用：每项结论均给出 `file_path`
> change-id：`build-android-native-client`
> 生成时间：2026-07-26（Phase 7 测试与构建完成后更新）

---

## 1. 单元测试清单

共 **18 个**单元测试文件，分布在 3 个模块：

### 1.1 core 模块（11 个）

路径前缀：`android/core/src/test/java/com/amitia/core/`

| 测试文件 | 测试对象 | 引用 |
|---|---|---|
| `network/endpoint/RuntimeEndpointProviderTest.kt` | Local / Remote 端点切换 | `core/network/endpoint/RuntimeEndpointProvider.kt` |
| `network/endpoint/RuntimeEndpointTest.kt` | Endpoint baseUrl 与 mode | `core/network/endpoint/RuntimeEndpoint.kt` |
| `network/sse/SseParserTest.kt` | SSE 行解析与事件分发 | `core/network/sse/SseParser.kt` |
| `network/client/ErrorMappingInterceptorTest.kt` | HTTP 错误码 → AmitiaError 映射 | `core/network/client/ErrorMappingInterceptor.kt` |
| `network/endpoint/LocalAuthTokenProviderTest.kt` | 本地 JWT 签发与校验 | `core/network/endpoint/LocalAuthTokenProvider.kt` |
| `repository/ChatRepositoryTest.kt` | 聊天会话与消息持久化 | `core/repository/ChatRepository.kt` |
| `error/ErrorMapperTest.kt` | 19 个 AmitiaError 子类映射 | `core/error/ErrorMapper.kt` |
| `logging/LogSanitizerTest.kt` | 敏感信息脱敏 | `core/logging/LogSanitizer.kt` |
| `datastore/SettingsDataStoreTest.kt` | 14 个配置项读写 | `core/datastore/SettingsDataStore.kt` |
| `database/RuntimeStateDaoTest.kt` | Runtime 状态 Room DAO | `core/database/dao/RuntimeStateDao.kt` |
| `feature/CharacterDataSmokeTest.kt` | 角色数据 DTO 烟雾测试 | `core/model/` |

### 1.2 runtime 模块（5 个）

路径前缀：`android/runtime/src/test/java/com/amitia/runtime/`

| 测试文件 | 测试对象 | 引用 |
|---|---|---|
| `manager/RuntimeStateMachineTest.kt` | 10 状态机转换合法性 | `runtime/manager/RuntimeStateMachine.kt` |
| `linux/LinuxRootfsManagerImplTest.kt` | ZIP 解压 + SHA-256 + 原子升级 | `runtime/linux/LinuxRootfsManagerImpl.kt` |
| `process/LinuxProcessManagerImplTest.kt` | 进程启停 + 重启策略 + 日志滚动 | `runtime/process/LinuxProcessManagerImpl.kt` |
| `bootstrap/BootstrapSequenceImplTest.kt` | 启动顺序编排（RootFS → SurrealDB → Qdrant → Go 后端） | `runtime/bootstrap/BootstrapSequenceImpl.kt` |
| `health/HealthCheckerImplTest.kt` | 端口探测 + HTTP 轮询 | `runtime/health/HealthCheckerImpl.kt` |

### 1.3 feature 模块（2 个）

路径前缀：`android/feature/src/test/java/com/amitia/feature/`

| 测试文件 | 测试对象 | 引用 |
|---|---|---|
| `character/CharacterViewModelTest.kt` | 角色列表 / 详情 / 编辑状态 | `feature/character/CharacterViewModel.kt` |
| `chat/ChatViewModelTest.kt` | 聊天消息流与 SSE 事件收集 | `feature/chat/ChatViewModel.kt` |

---

## 2. 集成测试清单

共 **3 个**集成测试，位于 `android/app/src/androidTest/java/com/amitia/android/integration/`：

| 测试文件 | 测试范围 | 引用 |
|---|---|---|
| `RuntimeBootstrapIntegrationTest.kt` | 完整 Bootstrap 启动顺序与状态转换 | `runtime/bootstrap/BootstrapSequenceImpl.kt` |
| `SseStreamingIntegrationTest.kt` | SSE 流式回复端到端（MockWebServer） | `core/network/sse/SseClient.kt` |
| `ProactiveMessageIntegrationTest.kt` | 主动消息从后端到 UI 的完整链路 | `feature/chat/` + `core/repository/ProactiveMessageRepository.kt` |

集成测试使用 Hilt 测试配置（`AmitiaTestRunner`）+ MockWebServer + Robolectric。

---

## 3. UI 测试清单

共 **5 个**UI 测试，位于 `android/app/src/androidTest/java/com/amitia/android/ui/`：

| 测试文件 | 测试场景 | 引用 |
|---|---|---|
| `OnboardingFlowUiTest.kt` | 首次启动引导流程导航 | `feature/onboarding/OnboardingScreen.kt` |
| `ChatFlowUiTest.kt` | 聊天发送消息 + 流式接收 | `feature/chat/ChatScreen.kt` |
| `MemoryScreenUiTest.kt` | 记忆列表与编辑 | `feature/memory/MemoryScreen.kt` |
| `RuntimeScreenUiTest.kt` | Runtime 管理页启停 | `feature/runtime/RuntimeScreen.kt` |
| `SettingsScreenUiTest.kt` | 设置页 14 个配置项 | `feature/settings/SettingsScreen.kt` |

UI 测试使用 Compose UI Test JUnit4 + Hilt Android Testing。

---

## 4. 测试执行结果

### 4.1 测试文件已编写完成

| 模块 | 单元测试 | 集成测试 | UI 测试 | 合计 |
|---|---|---|---|---|
| core | 11 | — | — | 11 |
| runtime | 5 | — | — | 5 |
| feature | 2 | — | — | 2 |
| app | — | 3 | 5 | 8 |
| **合计** | **18** | **3** | **5** | **26** |

> 加上 `android/app/src/androidTest/java/com/amitia/android/AmitiaTestRunner.kt`（Hilt 测试 Runner，非测试用例），共 27 个 .kt 测试文件。

### 4.2 已修复的编译错误清单

Phase 7 期间为通过编译陆续修复以下问题：

| 问题 | 修复内容 | 引用文件 |
|---|---|---|
| 主题资源不存在 | `Theme.Amitia` 改 `parent="Theme.Material3.Dark.NoActionBar"`，移除对不存在 `color/` 资源的引用 | `android/app/src/main/res/values/themes.xml` |
| Room converters 重复 | `Converters.kt` 去重，仅保留唯一的 `List<String>` 转换器 | `android/core/src/main/java/com/amitia/core/database/converter/Converters.kt` |
| ConnectionManager 命名冲突 | `platform` 模块的 `ConnectionManager` 与 `core/network/connection` 重名，重命名 platform 实现 | `android/platform/src/main/java/com/amitia/platform/...` |
| AmitiaError 继承不当 | `AmitiaError` 改为继承 `RuntimeException` 而非 `Exception` | `android/core/src/main/java/com/amitia/core/error/AmitiaError.kt` |
| Coroutine 测试 API 误用 | 单元测试统一改用 `runTest` 替代 `runBlocking`，挂起函数验证改用 `coVerify` | `android/core/src/test/.../*Test.kt`、`android/runtime/src/test/.../*Test.kt`、`android/feature/src/test/.../*Test.kt` |
| Truth 依赖缺失 | 添加 `com.google.truth:truth:1.4.4` 到 `libs.versions.toml` | `android/gradle/libs.versions.toml:39, 116` |
| testOptions 配置缺失 | 在 `app/build.gradle.kts` 添加 `testOptions { unitTests { isIncludeAndroidResources = true; isReturnDefaultValues = true } }` | `android/app/build.gradle.kts:88-93` |
| native 模块 externalNativeBuild 缺失 | `native/build.gradle.kts` 移除 `externalNativeBuild` 引用（PRoot JNI 当前仅占位） | `android/native/build.gradle.kts` |

### 4.3 测试 classpath 阻塞（已解决）

**历史问题**：执行 `./gradlew test` 在测试任务配置阶段失败：

```
class com.android.build.gradle.internal.dsl.TestOptions$UnitTestOptions_Decorated
cannot be cast to org.gradle.api.tasks.testing.Test
```

**根因**：AGP 8.5.2 + Kotlin 2.0.20 测试 classpath 已知兼容性问题 + 非 ASCII 项目路径（`桌面/跟进项目`）导致 Gradle Worker 进程无法正确读取 classpath。

**解决方案**：
1. `android/gradle.properties` 设置 `org.gradle.jvmargs=-Xmx4096m -Dfile.encoding=GBK -Dsun.jnu.encoding=GBK -XX:+UseParallelGC`（匹配 Windows 系统编码）
2. `android/build.gradle.kts` 移除 subprojects 块中对测试 JVM args 的覆盖（避免编码冲突）
3. AGP 升级至 8.7.3 解决 TestOptions 类型转换问题

**当前状态**：已解决。`./gradlew test` 全量执行成功，退出码 0。

### 4.4 实际测试执行结果（2026-07-26）

执行命令：`./gradlew test`（不带 `-x`），退出码 0，BUILD SUCCESSFUL。

| 模块 | 测试用例数 | 失败 | 忽略 | 耗时（Debug） | 结果 |
|---|---|---|---|---|---|
| `:core:testDebugUnitTest` | 112 | 0 | 0 | 25.66s | PASS |
| `:core:testReleaseUnitTest` | 112 | 0 | 0 | 27.31s | PASS |
| `:feature:testDebugUnitTest` | 31 | 0 | 0 | 6.10s | PASS |
| `:feature:testReleaseUnitTest` | 31 | 0 | 0 | 6.09s | PASS |
| `:runtime:testDebugUnitTest` | 90 | 0 | 2 | 16.05s | PASS |
| `:runtime:testReleaseUnitTest` | 90 | 0 | 2 | 21.92s | PASS |
| **Debug 合计** | **233** | **0** | **2** | — | **PASS** |
| **Release 合计** | **233** | **0** | **2** | — | **PASS** |

> runtime 模块 2 个 ignored 为 platform-specific 测试（Linux 进程管理在 Windows 测试环境跳过），非失败。

#### feature 模块 CharacterViewModelTest 详细结果

| 测试用例 | 耗时 | 结果 |
|---|---|---|
| `init_loads_characters_and_current_id` | 0.190s | PASS |
| `switchCharacter_updates_current_id_and_marks_isCurrent_flag` | 0.138s | PASS |
| `switchCharacter_does_not_mix_data_with_other_character` | 0.381s | PASS |
| `switchCharacter_sets_error_when_repository_fails` | 4.420s | PASS |
| `loadDetail_loads_character_into_detail_state` | 0.076s | PASS |
| `createCharacter_appends_to_list_and_switches_current` | 0.128s | PASS |
| `updateCharacter_replaces_existing_entry_in_list` | 0.158s | PASS |
| `deleteCharacter_removes_from_list_and_clears_pending_delete` | 0.200s | PASS |
| `confirmDelete_sets_pendingDeleteId` | 0.040s | PASS |
| `dismissDelete_clears_pendingDeleteId` | 0.034s | PASS |
| `consumeError_clears_error_state` | 0.071s | PASS |
| `listCharacters_invokes_repository_with_default_pagination` | 0.146s | PASS |

**修复说明**：`switchCharacter_updates_current_id_and_marks_isCurrent_flag` 和 `switchCharacter_does_not_mix_data_with_other_character` 两个用例在 `setUp()` 创建 ViewModel 后未显式加载角色列表，导致 `state.characters` 为空。修复方式：在 `switchCharacter` 调用前增加 `viewModel.listCharacters()` + `advanceUntilIdle()` 确保角色列表加载完成。引用文件：`android/feature/src/test/java/com/amitia/feature/character/CharacterViewModelTest.kt:79-83, 102-106`。

### 4.5 AndroidTest 阻塞

**问题**：集成测试与 UI 测试需要真机或模拟器。

**当前状态**：

- 本机未连接 ARM64 Android 真机
- AndroidTest 任务跳过

### 4.6 Lint 检查结果（2026-07-26）

执行命令：`./gradlew lint`，退出码 0，BUILD SUCCESSFUL。

| 模块 | 错误 | 警告 | 结果 |
|---|---|---|---|
| `:platform:lintDebug` | 0 | 0 | PASS |
| `:core:lintDebug` | 0 | 0 | PASS |
| `:runtime:lintDebug` | 0 | 0 | PASS |
| `:feature:lintDebug` | 0 | 0 | PASS |
| `:native:lintDebug` | 0 | 0 | PASS |
| `:app:lintDebug` | 0 | 105 | PASS |

**修复的 lint 错误（11 项）**：

| 序号 | 错误类型 | 文件 | 修复内容 |
|---|---|---|---|
| 1 | MissingPermission | `platform/src/main/AndroidManifest.xml` | 添加 `ACCESS_NETWORK_STATE` 权限声明 |
| 2 | MissingPermission | `platform/src/main/AndroidManifest.xml` | 同上（覆盖 `cm.activeNetwork` 与 `getNetworkCapabilities`） |
| 3 | WrongConstant | `platform/src/main/java/com/amitia/platform/audio/AudioPlayerImpl.kt:146` | `setContentType` 改用 `C.AUDIO_CONTENT_TYPE_MUSIC` |
| 4 | WrongConstant | `platform/src/main/java/com/amitia/platform/audio/AudioPlayerImpl.kt:147` | `setUsage` 改用 `C.USAGE_MEDIA` |
| 5 | NewApi | `app/src/main/res/values/themes.xml:7` | `windowLightNavigationBar` 添加 `tools:targetApi="27"` |
| 6 | NewApi | `app/src/main/res/values-night/themes.xml:7` | 同上 |
| 7 | NewApi | `app/src/main/res/values/themes.xml:9` | `enforceStatusBarContrast` 添加 `tools:targetApi="29"` |
| 8 | NewApi | `app/src/main/res/values-night/themes.xml:9` | 同上 |
| 9 | NewApi | `app/src/main/res/values/themes.xml:10` | `enforceNavigationBarContrast` 添加 `tools:targetApi="29"` |
| 10 | NewApi | `app/src/main/res/values-night/themes.xml:10` | 同上 |
| 11 | PermissionImpliesUnsupportedChromeOsHardware | `app/src/main/AndroidManifest.xml:9` | 添加 `<uses-feature android:name="android.hardware.camera" android:required="false" />` |

**清理的 ObsoleteSdkInt 警告（10 项）**：

minSdk=26，移除所有 `Build.VERSION.SDK_INT` 对 `O`(26) 和 `N`(24) 的不必要版本判断：

| 文件 | 行号 | 修复内容 |
|---|---|---|
| `platform/audio/AudioPlayerImpl.kt` | 248, 266 | 移除 `>= O` 判断与 `else` 死分支，移除未使用的 `import android.os.Build` |
| `platform/audio/AudioRecorderImpl.kt` | 114, 126 | 移除 `< N` 判断（始终 false） |
| `platform/foreground/ForegroundServiceManagerImpl.kt` | 47, 107, 178 | 移除 `>= O` / `< O` 判断，移除未使用的 `import android.os.Build` |
| `platform/notification/NotificationManagerImpl.kt` | 42, 66, 161 | 移除 `< O` 判断，移除未使用的 `import android.os.Build` |

**保留的信息性警告（105 项，非阻塞）**：

| 警告类型 | 数量 | 说明 |
|---|---|---|
| GradleDependency | ~60 | 依赖库有新版本（compose-bom、room、media3 等），当前版本稳定不升级 |
| AndroidGradlePluginVersion | 6 | AGP 8.7.3 有新版 9.3.1，当前版本稳定 |
| UnsafeNativeCodeLocation | 3 | Linux ARM64 二进制放在 assets 目录（PRoot 方案设计如此） |
| SimilarGradleDependency | 6 | hilt-compiler 有多个版本（Hilt 测试配置需要） |
| UnusedResources | 5 | backup_rules.xml、app_name_debug 等未引用资源 |
| KaptUsageInsteadOfKsp | 1 | Room 编译器使用 kapt（Hilt 兼容性需要） |
| OldTargetApi | 1 | targetSdk 34（有意设置） |
| SelectedPhotoAccess | 1 | READ_MEDIA_IMAGES 未处理 Android 14+ 部分访问（后续优化） |

### 4.7 Debug APK 构建结果（2026-07-26）

执行命令：`./gradlew assembleDebug`（不带 `-x`），退出码 0，BUILD SUCCESSFUL。

| 产物 | 路径 | 大小 | SHA-256 |
|---|---|---|---|
| Debug APK | `android/app/build/outputs/apk/debug/app-debug.apk` | 130.41 MB (136,744,686 bytes) | `5c4ee59ccfea92f2aa4711e0a56bc20746f5a788a025a45aee3fcd3e6df08101` |
| Go 后端 ARM64 | `android/app/src/main/assets/amitia-backend-arm64` | 54.63 MB | `abdd63eb020a01718684edcf130785da0cfd45dcb691aab5d260a8e17386879b` |
| Qdrant ARM64 | `android/app/src/main/assets/qdrant_linux_aarch64` | 74.23 MB | `6cb81123d2a3e405335c984efb7928dc21a1eac47a3b73a7609aee076dbe0b04` |
| SurrealDB ARM64 | `android/app/src/main/assets/surreal_linux_aarch64` | 111.03 MB | `a235206f2c4a803616d7669f56bc5bca5cc0ae7ed2b79acbe91f14712b28fe5c` |
| core-debug.aar | `android/core/build/outputs/aar/core-debug.aar` | — | — |
| feature-debug.aar | `android/feature/build/outputs/aar/feature-debug.aar` | — | — |
| native-debug.aar | `android/native/build/outputs/aar/native-debug.aar` | — | — |
| platform-debug.aar | `android/platform/build/outputs/aar/platform-debug.aar` | — | — |
| runtime-debug.aar | `android/runtime/build/outputs/aar/runtime-debug.aar` | — | — |

---

## 5. ARM64 真机验证（外部阻塞）

### 5.1 阻塞原因

- 本机未连接 ARM64 Android 设备
- 模拟器结果不能替代真机（参考 stage.md 24.4 节）
- x86_64 模拟器无法运行 ARM64 Linux 二进制（PRoot + ARM64 RootFS）

### 5.2 待真机验证的 22 项

| 序号 | 验证项 | 验证方法 |
|---|---|---|
| 1 | Android 版本兼容性 | minSdk 26 / targetSdk 34，覆盖 Android 8.0 ~ 14 |
| 2 | CPU 架构 | `adb shell getprop ro.product.cpu.abi` 应返回 `arm64-v8a` |
| 3 | RootFS 解压 | 首次启动后 `files/runtime/rootfs/bin/` 应包含 3 个可执行文件 |
| 4 | PRoot 运行 | 进程 `proot` 启动成功，日志无 error |
| 5 | Qdrant 启动 | `127.0.0.1:19178` 端口监听，HTTP `/healthz` 返回 200 |
| 6 | SurrealDB 启动 | `127.0.0.1:18000` 端口监听，HTTP `/health` 返回 200 |
| 7 | Go 后端启动 | `127.0.0.1:18899` 端口监听，HTTP `/health` 返回 200 |
| 8 | SQLite 持久化 | 重启 App 后聊天历史与角色配置保留 |
| 9 | 网络连接 | Local 模式下 ApiClient 可访问 `127.0.0.1:18899` |
| 10 | SSE 流式回复 | `POST /api/web-chat/send-stream` 收到 `message_start` / `token` / `message_end` |
| 11 | WebSocket | 主动消息推送通过 WS 实时到达 |
| 12 | 音频录制 | `AudioRecorderImpl` 调用 `MediaRecorder` 录制 PCM |
| 13 | 音频播放 | `AudioPlayerImpl` 通过 Media3 ExoPlayer 播放 TTS 音频 |
| 14 | 图片选择 | `FilePickerImpl` 调用 `ACTION_OPEN_DOCUMENT` 选择图片 |
| 15 | 后台保活 | App 退到后台后 Runtime 持续运行 |
| 16 | 前台服务常驻通知 | 通知栏显示 Runtime 状态，不可滑动清除 |
| 17 | 系统杀进程恢复 | `START_STICKY` 触发服务重建 |
| 18 | 重启数据恢复 | 杀进程后重启 App，数据未丢失 |
| 19 | 低电量模式 | Doze 模式下 Runtime 是否被限制 |
| 20 | 网络切换 | WiFi ↔ 移动数据切换时 SSE 自动重连 |
| 21 | 屏幕旋转 | `configChanges` 处理后 UI 状态保留 |
| 22 | 字体缩放 / 深色模式 | 系统设置变更后 UI 适配正常 |

### 5.3 当前结论

- 22 项待真机验证，全部为外部阻塞
- 模拟器（x86_64）验证可补充第 21、22 项（UI 适配），但不能覆盖 1-20 项

---

## 6. 已修复问题清单

下表汇总 Phase 7 期间修复的全部问题（含编译错误、测试 API 误用、lint 错误与测试用例逻辑修复）：

| 序号 | 问题 | 文件 | 修复内容 |
|---|---|---|---|
| 1 | 主题资源引用不存在 | `android/app/src/main/res/values/themes.xml` | `parent` 改为 `Theme.Material3.Dark.NoActionBar` |
| 2 | Room converters 重复注册 | `android/core/src/main/java/com/amitia/core/database/converter/Converters.kt` | 去重，保留唯一实现 |
| 3 | ConnectionManager 命名冲突 | `android/platform/src/main/java/com/amitia/platform/...` | 重命名 platform 模块实现 |
| 4 | AmitiaError 继承不当 | `android/core/src/main/java/com/amitia/core/error/AmitiaError.kt` | 改继承 `RuntimeException` |
| 5 | 测试用 runBlocking | `android/core/src/test/.../*Test.kt` 等 | 改用 `runTest` |
| 6 | 测试用 verify 验证挂起函数 | 同上 | 改用 `coVerify` |
| 7 | Truth 依赖缺失 | `android/gradle/libs.versions.toml` | 添加 `truth = "1.4.4"` 与 `truth` 库声明 |
| 8 | testOptions 未配置 | `android/app/build.gradle.kts` | 添加 `testOptions { unitTests { ... } }` |
| 9 | native 模块 externalNativeBuild 引用错误 | `android/native/build.gradle.kts` | 移除 `externalNativeBuild`，PRoot JNI 仅占位 |
| 10 | Retrofit kotlinx.serialization converter 依赖 | `android/app/build.gradle.kts` | 添加 `squareup-retrofit-kotlinx-serialization` |
| 11 | Hilt 测试 Runner 缺失 | `android/app/src/androidTest/java/com/amitia/android/AmitiaTestRunner.kt` | 新增自定义 TestRunner |
| 12 | Compose 测试 manifest 缺失 | `android/app/build.gradle.kts` | 添加 `debugImplementation(libs.androidx.compose.ui.test.manifest)` |
| 13 | 测试 classpath 阻塞（ClassNotFoundException） | `android/gradle.properties` + `android/build.gradle.kts` | JVM 编码改 GBK 匹配系统编码，移除 subprojects 测试 JVM args 覆盖 |
| 14 | CharacterViewModelTest 角色列表未加载 | `android/feature/src/test/java/com/amitia/feature/character/CharacterViewModelTest.kt:79-83, 102-106` | 在 `switchCharacter` 调用前增加 `viewModel.listCharacters()` + `advanceUntilIdle()` |
| 15 | Lint MissingPermission（ACCESS_NETWORK_STATE） | `android/platform/src/main/AndroidManifest.xml` | 添加 `ACCESS_NETWORK_STATE` 权限声明 |
| 16 | Lint WrongConstant（Media3 常量） | `android/platform/src/main/java/com/amitia/platform/audio/AudioPlayerImpl.kt:146-147` | `setContentType` / `setUsage` 改用 `C.AUDIO_CONTENT_TYPE_MUSIC` / `C.USAGE_MEDIA` |
| 17 | Lint NewApi（主题属性 API 27/29） | `android/app/src/main/res/values/themes.xml` + `values-night/themes.xml` | 添加 `tools:targetApi="27"` / `tools:targetApi="29"` |
| 18 | Lint PermissionImpliesUnsupportedChromeOsHardware | `android/app/src/main/AndroidManifest.xml:9` | 添加 `<uses-feature android:name="android.hardware.camera" android:required="false" />` |
| 19 | Lint ObsoleteSdkInt（10 处死代码） | `AudioPlayerImpl.kt` / `AudioRecorderImpl.kt` / `ForegroundServiceManagerImpl.kt` / `NotificationManagerImpl.kt` | 移除 minSdk=26 时不必要的 `Build.VERSION.SDK_INT` 判断与死分支，清理未使用导入 |

---

## 7. 待后续处理

### 7.1 ARM64 真机验证（高优先级，外部阻塞）

- 22 项见第 5 节
- 需采购或借用 ARM64 真机
- 模拟器（x86_64）无法验证 PRoot + ARM64 Linux 二进制

### 7.2 AndroidTest 执行（中优先级，外部阻塞）

- 集成测试 3 个 + UI 测试 5 个需要真机或模拟器
- 单元测试已全部通过（233 tests, 0 failures）
- AndroidTest 任务在无设备时跳过

### 7.3 FilePickerImpl / AudioPlayerImpl / AudioRecorderImpl 占位实现（中优先级）

- 当前为接口占位实现
- 待接入 Activity Result API：
  - `FilePickerImpl` — `rememberLauncherForActivityResult(OpenDocument())`
  - `AudioPlayerImpl` — Media3 ExoPlayer 完整接入
  - `AudioRecorderImpl` — `MediaRecorder` 完整接入

### 7.4 ProactiveMessageObserver 在 Runtime Running 状态启动（中优先级）

- 当前观察者未在 `RuntimeState.Running` 时自动启动
- 待在 `AmitiaCoreService` 或 `StartupViewModel` 注册

### 7.5 通知点击 PendingIntent 跳转解析（中优先级）

- 通知点击当前未跳转指定页面
- 待在 `MainActivity.onNewIntent` 解析 intent extras 并导航

### 7.6 PermissionBrokerImpl 权限回调路由（低优先级）

- 当前权限回调未在 `MainActivity` 注册
- 待通过 `registerForActivityResult(RequestMultiplePermissions)` 接入

### 7.7 信息性 lint 警告优化（低优先级）

- 105 项警告中 ~60 项为 GradleDependency（依赖版本升级）
- 可在后续阶段选择性升级 compose-bom、room、media3 等依赖
- UnsafeNativeCodeLocation 为 PRoot 方案设计，不修复

---

## 8. 测试统计

| 维度 | 数值 |
|---|---|
| 单元测试文件 | 18 |
| 集成测试文件 | 3 |
| UI 测试文件 | 5 |
| 测试 Runner 文件 | 1 |
| **测试 .kt 文件总数** | **27** |
| 单元测试用例数（Debug） | 233 |
| 单元测试通过数 | 233 |
| 单元测试失败数 | 0 |
| 单元测试忽略数 | 2（platform-specific） |
| **单元测试通过率** | **100%** |
| Lint 错误 | 0 |
| Lint 警告（信息性） | 105 |
| Debug APK | 已生成（130.41 MB） |
| ARM64 真机验证 | 0 / 22 项（外部阻塞） |
| 已修复问题 | 19 项 |
| 待后续处理项 | 7 类 |
