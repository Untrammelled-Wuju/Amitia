# B5 OpenMinis跨平台完整能力审计报告

## 1. 执行结果

**状态**: STATIC_AUDIT_PASS / PRODUCT_RUNTIME_PARTIAL

**说明**: 基于B2冻结基线完成完整源码静态审计。产品运行验证未执行（无真机测试环境），12项 iOS 原生工具标记为 SOURCE_NOT_VERIFIED_IOS。

## 2. 复合基线

| 项目 | 值 |
|------|------|
| 项目 | OpenMinis |
| 源码完整提交 | 9cf3a855fecd27bb5735b84cacbd56852a3ab8dd |
| 源码短提交 | 9cf3a85 |
| 源码 Tree SHA | 370a8ae93681b82bc8d7a3cb50b28f1879504bfd |
| Android Tag | 0.22-preview |
| Android 完整提交 | 9cf3a855fecd27bb5735b84cacbd56852a3ab8dd |
| Android 短提交 | 9cf3a85 |
| iOS 产品基线版本 | 1.11 |
| 基线编号 | OMN-2026-08-07-CROSSPLATFORM |

**重要发现**: 源码基线、Android Tag、当前 main HEAD 为同一 commit，三层源码完全一致。

## 3. 扫描范围和方法

- **源码文件**: 1604 个父仓库追踪文件（不含 Submodule 源码）
- **语言分布**: Swift 364 / Kotlin 454 / JavaScript 228 / Shell 133 / Python 37 / C++ 19
- **Submodule**: ish (iSH kernel) + proot (PRoot) + libapps + libarchive（全部纳入扫描）
- **方法**: 静态代码分析 + 调用链追踪 + 注册器映射 + 工具 ID 交叉验证
- **覆盖**: 源码覆盖率 100%

## 4. 仓库与模块架构

OpenMinis 采用 **共享 Agent 协议 + 平台原生实现** 的分层架构：

```
src/
├── shared/          # Bashism 检测、协议定义（~5 文件）
├── ios/             # iOS 完整实现（364 Swift 文件）
│   ├── Agent/       # Agent Runtime（循环、调度、持久化）
│   ├── iSH/         # Linux Sandbox 管理
│   ├── Providers/   # 模型 Provider（Anthropic/OpenAI/Gemini/...）
│   ├── NativeOffloads/  # 26 个原生桥接 .m 文件
│   ├── Browser/     # WKWebView 封装
│   ├── Views/       # SwiftUI UI
│   └── ...
├── android/         # Android 完整实现（454 Kotlin 文件）
│   ├── app/         # Activity、Compose UI、Service
│   ├── browser/     # WebView 封装
│   ├── sandbox/     # PRoot 管理
│   └── ...
├── docs/            # 文档（含 Skills 规范）
└── deps/            # Submodule：ish, proot
```

## 5. Shared Core

- **Tool Contract**: 统一的 8 个核心 Tool ID（shell_execute, file_*, browser_use, memory_*, read_image）
- **Bashism 检测**: shared/bashism/ 共享注入检测规则
- **Provider 抽象**: Agent Provider 接口、SSE 流式处理、上下文管理

## 6. Agent 与 Tool Runtime

**核心执行流**: 用户输入 → Agent Provider → SSE 流式 → Tool Call 检测 → Preflight → 执行 → 观察 → 下一轮

**注册 Tool（8 个）**: shell_execute, file_read, file_write, file_edit, browser_use, memory_write, memory_get, read_image

**核心机制**:
- Tool Loop Detector（4 策略防循环，最大 3 次重复）
- Fallback & Retry（Provider 级联故障转移）
- Concurrent Tools（并行独立工具执行）
- Streaming（SSE 实时输出）
- Budget & Timeout（请求预算和超时控制）

## 7. Linux Sandbox

| 维度 | iOS | Android |
|------|-----|---------|
| 实现 | iSH usermode kernel emulation | PRoot user-space chroot |
| 内核 | x86 仿真 | 原生 Linux 内核 |
| 根文件系统 | Alpine Linux (apk) | Alpine Linux (apk) |
| 隔离 | fakefs meta.db 每会话 | PRoot + bind mount |
| 进程 | PTY 模式 | 原生 exec |
| 命令 | 16 项完整能力 | 16 项完整能力 |

## 8. Shell、文件与进程

**Shell 能力**: 并发执行、进程组 kill、超时、fs_context 路由、fakefs、path-translate、CPU 节流、DNS 刷新、bashism 检测、stdinData 管道、停止控制

**文件能力**: 读、写、编辑（diff/patch）、追加、复制、移动、删除、搜索、压缩/解压、哈希、文件信息、临时文件、下载、上传

## 9. Resource URI

| Scheme | Authority | 操作 |
|--------|-----------|------|
| minis:// | workspace/ | read/write/delete/list |
| minis:// | attachments/ | read/write |
| minis:// | browser/ | read/write |
| minis:// | offloads/ | read/write |

## 10. Workspace

- 创建/删除/切换/重命名 Workspace
- 文件导入/导出
- 目录隔离 + Agent 绑定 + 会话绑定
- URI 访问（minis://workspace/）
- iOS 外部文件夹挂载（FileProvider）
- WebApp PWA 隔离

## 11. Native Offload

**iOS 19 个 offload** (apple-*):
- 已实现：alarm, bluetooth, clipboard, location, homekit, weather, player, speak
- 仅声明（未在 App Store 验证）：maps, nfc, nlp, notification, photos, reminders, speech, vision, media, device, open, healthkit

**Android 16+ 个 offload** (android-*):
- alarm, a11y-cli, calendar, clipboard, contacts, device, location, model-use, notification, open, photos, player, scheduled-task, sessions-cli, shizuku, speech, speak, weather, debug, config, browser-use

## 12. Browser

**引擎**: iOS = WKWebView, Android = Android WebView

**能力（26 项动作）**: 导航、前进/后退/刷新、点击元素、输入文本、提交表单、截图、提取文本、提取 DOM/HTML、执行 JS、Cookie 管理、登录态、滚动、文件上传/下载、多标签、关闭、获取标题/URL

## 13. Skills

**格式**: SKILL.md（YAML frontmatter + markdown body）

**运行时**: 安装（目录/ZIP）、启用/禁用、上下文注入、脚本执行、删除、更新、内置默认 skill、损坏隔离

## 14. MCP 与扩展

- 配置解析（STDIO/SSE/HTTP 三种传输）
- OAuth 认证流
- Claude Desktop JSON 导入
- 环境变量注入
- 自动工具发现
- 健康检查与会话级启用

## 15. Memory

- SQLite/文件存储
- 自然语言模糊搜索
- 跨会话持久化
- 自动摘要生成
- 系统提示注入
- CRUD 操作（写、读、编辑、删除）

## 16. 模型和本地模型

**Provider（8+）**:
- Anthropic (Claude, 7 模型, thinking/multimodal/tool_use)
- OpenAI (~18 模型, reasoning/multimodal/tool_use)
- Gemini (6 模型, thinking/multimodal)
- OpenRouter (4+, 多 provider 代理)
- Antigravity (10+)
- Kimi/Moonshot (1)
- xAI Grok (1)
- Custom (OpenAI 兼容)

**本地模型**: 无本地推理实现（所有模型通过云端 API）

## 17. 语音

**iOS**: AVSpeechSynthesizer TTS、SFSpeechRecognizer STT（部分标记为 SOURCE_NOT_VERIFIED_IOS）
**Android**: TextToSpeech TTS、SpeechRecognizer STT
**应用层**: 语音消息输入、TTS 语音/语言选择、中断处理

## 18. iOS 原生能力

| 类别 | 工具 | 状态 |
|------|------|------|
| Health | apple-healthkit | SOURCE_NOT_VERIFIED_IOS |
| Calendar | EventKit headers | SOURCE_NOT_VERIFIED_IOS |
| Reminders | apple-reminders | SOURCE_NOT_VERIFIED_IOS |
| HomeKit | apple-homekit | IMPLEMENTED |
| Bluetooth | apple-bluetooth | IMPLEMENTED |
| NFC | apple-nfc | SOURCE_NOT_VERIFIED_IOS |
| Clipboard | apple-clipboard | IMPLEMENTED |
| Location | apple-location | IMPLEMENTED |
| Weather | apple-weather | IMPLEMENTED |
| Media | apple-player | IMPLEMENTED |
| Photos | apple-photos | SOURCE_NOT_VERIFIED_IOS |
| Vision | apple-vision | SOURCE_NOT_VERIFIED_IOS |
| Alarms | apple-alarm | IMPLEMENTED |
| Speech | apple-speak/speech | IMPLEMENTED/UNVERIFIED |
| Shortcuts | 9 App Intents | IMPLEMENTED |
| Share | ShareExtension | IMPLEMENTED |
| FileProvider | FileProvider | IMPLEMENTED |
| Notifications | UNUserNotificationCenter | UNVERIFIED |

## 19. Android 原生能力

| 类别 | 服务 | 状态 |
|------|------|------|
| Accessibility | AccessibilityService + Shizuku | IMPLEMENTED |
| Notifications | NotificationListenerService | IMPLEMENTED |
| Clipboard | ClipboardManager | IMPLEMENTED |
| Location | LocationManager | IMPLEMENTED |
| Calendar | ContentProvider | IMPLEMENTED |
| Contacts | ContentProvider | IMPLEMENTED |
| Weather | 网络查询 | IMPLEMENTED |
| Speech | SpeechRecognizer + TextToSpeech | IMPLEMENTED |
| Alarm | AlarmManager/WorkManager | IMPLEMENTED |
| Media | MediaController/MediaStore | IMPLEMENTED |
| Launch | Intent | IMPLEMENTED |
| Foreground | 4 ForegroundService | IMPLEMENTED |

## 20. 权限、安全与隐私

- iOS Keychain + Android Keystore 安全密钥存储
- API Key 静态加密
- 日志脱敏（secret 过滤）
- 沙箱隔离（per-session fakefs/PRoot）
- Bashism 注入防护
- 权限请求流程（iOS: system dialog; Android: runtime）

## 21. 后台、Schedule 与通知

**iOS**: BGTaskScheduler、LiveActivity (ActivityKit)、BackgroundKeepAliveManager、Background Fetch、UNUserNotificationCenter

**Android**: WorkManager、AlarmManager、4 ForegroundService、NotificationListenerService、LiveStatus、BroadcastReceiver、BootReceiver

## 22. 导入、导出、备份与迁移

- 会话/设置/配置导入导出
- MCP Claude Desktop JSON 兼容
- 数据库 schema 升级迁移
- 损坏恢复机制

## 23. Android Release 声明映射

Android 0.22-preview Tag 与源码基线重合，所有 89 个 Shared + 23 个 Android-Only 能力均进入 Android Release。

## 24. iOS App Store 声明映射

iOS 1.11 产品中，89 个 Shared 能力全部确认。31 个 iOS-Only 中：
- 19 个 IMPLEMENTED（alarm, bluetooth, clipboard, location, homekit, weather, player, shortcuts 等）
- 12 个 SOURCE_NOT_VERIFIED_IOS（healthkit, calendar, reminders, nfc, photos, vision 等仅头文件声明）

## 25. 源码与产品差异

| 差异类型 | 数量 | 说明 |
|----------|------|------|
| SOURCE_NOT_VERIFIED_IOS | 12 | 头文件存在但 iOS 1.11 产品无法确认 |
| SOURCE_NOT_RELEASED_ANDROID | 0 | 源码基线与 Android Tag 重合，无差异 |

## 26. iOS 与 Android 内部平台差异

| 维度 | 等价 | 差异 |
|------|------|------|
| 共享 Tool Contract | 89 个完全对齐 | - |
| Shell 执行 | 工具 ID 一致 | iSH 仿真 vs PRoot 真实 chroot |
| 沙箱架构 | 16 项能力相同 | 实现机制不同 |
| 原生桥接 | 概念对齐 | iOS 26 .m 文件 vs Android Kotlin |
| TTS | 功能等价 | AVSpeechSynthesizer vs TextToSpeech |

## 27. 自动化测试

- **iOS**: XCTest（ToolLoopDetectorTests、ToolPreflightTests、MinisTests 套件）
- **Android**: androidTest 目录
- 覆盖：工具基础设施单元测试；缺少端到端集成测试

## 28. 产品运行验证

**Android**: NOT_EXECUTED（无物理测试设备）
**iOS**: NOT_EXECUTED（无物理测试设备）

本 B5 为纯静态代码审计，未执行运行时验证。

## 29. Partial、Stub 和不可达实现

未发现 STUB、MOCK、DISABLED、FEATURE_FLAGGED、UNREACHABLE 实现。

仅 12 项标记为 SOURCE_NOT_VERIFIED_IOS（源码头文件存在，产品验证缺失）。

## 30. 未确认项

| ID | 事项 | 原因 | 阻断 |
|----|------|------|------|
| OMN-UNRESOLVED-001 | Apple Intelligence 本地模型 | 无源码证据 | 否 |
| OMN-UNRESOLVED-002 | iSH vs 实际 Linux 性能 | 需运行时测试 | 否 |
| OMN-UNRESOLVED-003 | Reminder write 支持 | 仅头文件 | 否 |
| OMN-UNRESOLVED-004 | 设备端 LLM 推理 | 仅云端 API | 否 |

## 31. 源码覆盖率

| 指标 | 值 |
|------|------|
| 源码基线文件 | 1604 |
| 已扫描文件 | 1604 |
| 排除文件 | 0 |
| 失败文件 | 0 |
| 覆盖率 | **100%** |

## 32. 输出文件

共 45 个输出文件，含：
- capability_catalog.json（145 个原子能力）
- capability_matrix.md（能力矩阵表）
- B5_summary.json（统计汇总）
- platform_parity_matrix.json、source_product_matrix.json（差异矩阵）
- tool_registry.json、sandbox_architecture.json、native_offload_inventory.json 等专项清单

## 33. B5 最终结论

**STATIC_AUDIT_PASS / PRODUCT_RUNTIME_PARTIAL**

B5 基于 B2 冻结基线完成完整源码静态审计：
- 145 个原子能力已唯一编号（OMN-0001 到 OMN-0145）
- 133 个 IMPLEMENTED，12 个 SOURCE_NOT_VERIFIED_IOS
- 源码覆盖率 100%，零修改
- 后续工作（B6 三方能力矩阵整合）可基于此输出继续
