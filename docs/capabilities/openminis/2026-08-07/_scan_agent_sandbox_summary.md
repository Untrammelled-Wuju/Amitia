# OpenMinis B5 能力审计扫描报告（Agent Core / Sandbox）

- 扫描日期：2026-08-07
- 扫描基线：`source-baseline`（iOS Swift / Android Kotlin / shared）
- 分析源码文件数：47
- 覆盖平台：iOS、Android、shared

---

## 1. Tool Registry 完整列表（Tool IDs）

iOS 与 Android **在 Tool ID 层面完全对齐**（均通过 `makeAgentTools()` 注册）。

| Tool ID | 描述 | 参数（节选） | 注册条件 |
|---|---|---|---|
| `shell_execute` | 执行 iSH/Alpine 隔离 shell 命令 | tool_title, command, timeout(默认900s), delay | 始终注册 |
| `file_read` | 直接读取 Linux 文件 | tool_title, path, offset, lines, direction(head/tail), max_length(15000) | 始终注册 |
| `file_write` | 写入 Linux 文件 | tool_title, path, content, append, create_dirs | 始终注册 |
| `file_edit` | 精确字符串替换编辑 | tool_title, path, old_string, new_string, replace_all | 始终注册 |
| `browser_use` | 控制 WebView 浏览器（MAX 3 tabs） | action(18种), url, selector, text, script, coordinate_x/y, cookies, viewport_*, full_page... | 始终注册 |
| `memory_write` | 写入每日记忆日志 (YYYY-MM-DD.md) | tool_title, content | memoryEnabled=true |
| `memory_get` | 模糊搜索记忆文件 | tool_title, scope(daily/all), keywords | memoryEnabled=true |
| `read_image` | 读取图片并返回视觉分析 | tool_title, path(支持 minis:// URL) | 模型支持 imageInput |

### browser_use 支持的 action 枚举（18 种）

navigate, screenshot, click, type, get_text, scroll, get_page_info, execute_js, find_elements, hover, get_readable, set_user_agent, set_viewport, get_backbone, fetch, new_tab, close_tab, list_tabs, get_cookies, set_cookies, scroll_and_collect, wait_for_dom_stable

### 关键调用链（shell_execute）

LLM tool_call → AIChatViewModel.executeTool → ISHExecutionCoordinator.execute → ISHShellExecutor.executeExecutable(fsContext) → ISH kernel fork → fakefs path-translate hook (MinisFsRouter) → 结果经 lineCallback 流式回传。整个路径并发无 PTY 共享。

---

## 2. Sandbox 架构描述

### iOS Sandbox（iSH usermode kernel emulation）

```
┌──────────────────────────────────────────────────────┐
│  iOS App进程 (PID N)                                  │
│  ┌─────────────────────────────────────────────────┐ │
│  │  ISHKernel.shared (ObjC, 启动后成为guest PID 1)   │ │
│  │  ┌───────────────────────────────────────────┐  │ │
│  │  │  Alpine Linux aarch64 rootfs              │  │ │
│  │  │  Documents/alpine-rootfs/data/            │  │ │
│  │  │  ├── var/minis/attachments (per-session)  │  │ │
│  │  │  ├── var/minis/offloads                   │  │ │
│  │  │  ├── var/minis/workspace (per-session)    │  │ │
│  │  │  ├── var/minis/skills (static)            │  │ │
│  │  │  ├── var/minis/shared (static)            │  │ │
│  │  │  ├── var/minis/mcp-servers (bind-mount)   │  │ │
│  │  │  └── usr/local/bin/apple-* (stubs)        │  │ │
│  │  └───────────────────────────────────────────┘  │ │
│  │  fakefs层: meta.db (SQLite, 路径→inode映射)     │ │
│  │  MinisFsRouter: per-session fs_context 路由钩子   │ │
│  │  bind-mount: host目录↔guest路径 (含read-only模式) │ │
│  └─────────────────────────────────────────────────┘ │
│  RootfsManager: 安装/overlay/fakefs注册              │
│  NativeOffloadUtils: JSON envelope + 工具函数         │
│  OffloadPermissionManager: 19个apple-*命令权限门控    │
└──────────────────────────────────────────────────────┘
```

**关键文件**：
- `src/ios/iSH/ISHKernel.h` — 内核 boot/exec/bindMount/pathTranslate API
- `src/ios/iSH/ISHShellExecutor.h` — 进程执行、killProcessGroup (SIGTERM→SIGKILL)
- `src/ios/Agent/ISH/ISHExecutionCoordinator.swift` — Actor 并发调度，static mount + MinisFsRouter 静态/动态双层挂载
- `src/ios/iSH/RootfsManager.swift` — Alpine aarch64 安装、PEP 668 移除、fakefs meta.db
- `src/ios/Agent/Shell/BashismDetector.swift` — busybox-ash bashism 跨平台检测

**并发模型**：commands 全并发（跨 session + 同 session），每个 fork 独立 /bin/sh + pipes + fs_context 令牌，无共享 PTY。

**安全机制**：rootfs reset 标记 didResetWhileBooted（kernel 无法 un-boot，需重启）；binDirs 启发式恢复 0755（iOS bundle 安装剥离 exec bit）；minis-mcp-cli lib 强制 0o444/0o555；外部挂载支持 read-only 模式（0o555 + EACCES 拒绝写）。

### Android Sandbox（PRoot + Termux 风格）

Android 容器由 Termux 风格的 PRoot 用户态 rootfs 提供（非 iSH 内核模拟）：

- 主体：Alpine Linux via PRoot
- 内部调用：`/bin/sh -c`，stdout/stderr *合并*（区别于 iOS 分离管道）
- 挂载抽象：`sandbox/offload/` 下 25+ OffloadHandler（Kotlin）
- 终端：`TerminalEmulator.kt` + `AnsiParser.kt` + `TerminalBuffer.kt`（原生画布渲染，非 WebView）
- Shizuku/Accessibility 集成（a11y_cli, shizuku_cli）

---

## 3. Agent Runtime 执行流程图

```
用户输入 (text + attachments[image/video/document])
    │
    ▼
ChatViewModel (iOS) / ChatViewModel (Android)
    │ makeAgentTools() 注册工具（memoryEnabled/imageInput 条件）
    │ preflightValidateToolCall → 缺失字段预检
    ├─→ 构建 AgentMessage 历史（含 compact 标记、reasoningEcho）
    │
    ▼
ProviderFactory.makeProvider(for: ModelEntry)
    ├─→ AnthropicProvider / GeminiProvider / OpenAIProvider
    │   / OpenAIAntigravityProvider / OpenRouter / xAI / Kimi
    │
    ▼
AgentProvider.streamAgentMessageClamped
    ├─→ API 流式请求 (Anthropic Messages / Gemini / OpenAI Responses)
    │
    ▼
AsyncStream<AgentStreamEvent>:
    ├─ textDelta → UI 增量渲染 (StreamingMarkdownText)
    ├─ toolCallComplete(id,name,args)
    │      │
    │      ▼
    │   ToolLoopDetector.check() — 4 策略防循环
    │      │ critical/warning → 终止/降级
    │      │
    │      ▼
    │   executeTool(toolName, args)
    │      ├─ shell_execute → ISHExecutionCoordinator.execute
    │      │     └─ offload 拦截 → OffloadPermissionManager.checkPermission
    │      │         ├─ apple-* → apple-* native handler (.m)
    │      │         └─ 标准命令 → ISHShellExecutor
    │      ├─ file_read/write/edit → iSH 文件操作
    │      ├─ browser_use → BrowserTabPool.execute (WKWebView)
    │      ├─ memory_write/get → MemoryRepository
    │      └─ read_image → 视觉模型回传
    │      │
    │      ▼
    │   toolResult → AgentContentPart.toolResult → 历史注入
    │      │
    │      ▼  (maxAgentTurns=200 控制)
    │   loop 回到 streamAgentMessageClamped
    │
    ├─ thinkingDelta → 实时思考 UI
    ├─ usage(LLMUsage) → max 模式 token 累积（非 sum）
    └─ done(stopReason: endTurn/toolUse/maxTokens/refusal)
```

**关键参数**：
- maxAgentTurns = 200
- kMaxToolResultChars = 15000
- kPerImageMaxBytes = 5MB, kMessageImageMaxBytes = 25MB, kRequestImageMaxBytes = 25MB
- kImageContextKeepCount = 20（历史最多保留20张）
- image/compactDivider 标记分层压缩

---

## 4. Native Offload 协议完整列表

### iOS：19 个 `apple-*` 命令（ObjC 实现）

| 命令 | 类别 | 显示名 | 设置页可见 | 说明 |
|---|---|---|---|---|
| `apple-healthkit` | Privacy | HealthKit | 是 | 步数、心率、睡眠等健康数据 |
| `apple-calendar` | Privacy | Calendar | 是 | 事件、日程 |
| `apple-reminders` | Privacy | Reminders | 是 | 任务、截止日期 |
| `apple-photos` | Privacy | Photos | 是 | 照片/视频/相册元数据 |
| `apple-location` | Privacy | Location | 是 | GPS 坐标与位置历史 |
| `apple-homekit` | Privacy | HomeKit | 是 | 智能家居设备/房间/场景 |
| `apple-clipboard` | Privacy | Clipboard | 是 | 剪贴板文本与图像 |
| `apple-speak` | Media | Speak | 否 | 文本朗读 |
| `apple-speech` | Speech | Speech | 否 | 语音识别 |
| `apple-player` | Media | Player | 否 | 媒体播放 |
| `apple-media` | Media | Media | 否 | 媒体控制 |
| `apple-device` | System | Device | 否 | 设备信息 |
| `apple-notification` | System | Notification | 否 | 通知管理 |
| `apple-alarm` | System | Alarm | 否 | 闹钟/计时器 |
| `apple-open` | System | Open URL | 否 | 打开 URL/应用 |
| `apple-maps` | System | Maps | 否 | 地图导航 |
| `apple-weather` | System | Weather | 否 | 天气查询 |
| `apple-nlp` | System | NLP | 否 | 自然语言处理 |
| `apple-vision` | System | Vision | 否 | 视觉分析 |

另有 7 个无 Permission 门控的 offload：`apple-nfc`, `apple-model-use`, `apple-ffmpeg`, `apple-debug`, `apple-config`, `apple-bluetooth`, `apple-browser-use` (桥接)。

**注册模式**：每个 .m 暴露 `xxx_offload_register()` C 函数，kernel 启动时统一注册。

**调用协议**：guest shell → `apple-*` 命令 → ISH 内核拦截 → 经 fakefs path-translate 或专用 handler → noff_dispatch_main_sync → Swift/ObjC 原生 API → JSON envelope 回写 stdout。

**OffloadCommandInfo.showInSettings**：仅 Privacy 显示在设置页，Media/System 为 BYPASS 不可配置。

### Android：16 个 offload 工具（Kotlin 实现）

| 工具 ID | 类别 | 默认级别 |
|---|---|---|
| `calendar`, `location`, `clipboard`, `contacts`, `photos` | Privacy | BYPASS |
| `speak`, `media_player`, `speech_recognition` | Media | BYPASS |
| `alarm`, `weather`, `notification`, `device_info` | System | BYPASS |
| `a11y_cli`, `shizuku_cli` | Integrations | **NOT_ALLOWED** |

Android 通过 `OffloadGate` 在 `NativeOffloadHandler.handle` 入口进行门控（runBlocking 桥接），通过 `android-*` 命令前缀触发。

---

## 5. Shell/Process 执行能力完整列表

| 能力 | iOS 文件 | 说明 |
|---|---|---|
| 并发进程执行 | ISHExecutionCoordinator.swift | Actor 全并发，fs_context 路由 |
| 进程组 kill | ISHShellExecutor.h : killProcessGroup | SIGTERM→SIGKILL 升级 |
| 超时控制 | ISHExecutionCoordinator (默认300s, preempt 600s) | handler 层 escalation |
| fs_context 路由 | ISHShellExecutor.executeExecutable(fsContext) | 每进程独立 mount 视图 |
| iSH 内核绑载 | ISHKernel.h : bindMountPath (readOnly 变体) | 主机↔guest 目录映射 |
| fakefs meta.db | RootfsManager.swift | SQLite 元数据、blob 编解码 stat |
| Path-translate hook | ISHKernel.h : installPathTranslateHandler | 热路径非阻塞重定向 |
| 反向 Path hook | ISHKernel.h : installPathReverseHandler | readdir 正确性 |
| Fakefs 变更通知 | ISHKernel.h : installFakefsChangeHandler | 文件写/unlink/rename 事件环 |
| CPU 节流 | ISHKernel.h : enableCPUThrottleWithDutyCycle | 后台自适应 nanosleep |
| DNS 刷新 | ISHKernel.h : refreshDns | 网络切换 /etc/resolv.conf 更新 |
| Bashism 跨兼容 | BashismDetector.swift | busybox-ash 不兼容检测 + 按需装 bash |
| Offload 拦截 | OffloadPermissionManager.extractOffloadCommand | apple-* 命令首 token 解析 |
| stdinData 管道 | ISHShellExecutor.executeExecutable(stdinData) | heredoc/pipe 输入 |
| cancel/stop | ISHExecutionCoordinator.stopCurrentCommand | 按 session/broad 停止 |
| Term 窗口尺寸 | ISHKernel.h : setTerminalSize | SIGWINCH 同步 |

---

## 6. Provider 模型列表

### 架构

- **协议层**：`LLMProvider` (sendMessage/streamMessage) → `AgentProvider` (streamAgentMessageClamped)
- **工厂层**：`LLMProviderFactory` (iOS) / `ProviderFactory` (Android)，按 ProviderInstance.providerType 分发
- **认证**：apiKey / oauth / manualToken（Android 额外支持 Azure、Codex Responses）
- **统一类型**：iOS `LLMModel` (id/displayName/provider/modalityOverride/contextWindow/maxOutputTokens/supportsReasoning/interleavedReasoningField)；Android 对齐结构

### 模型目录（~36+）

| Provider | 模型数 | 主要模型 |
|---|---|---|
| **Anthropic** | 7 | claude-fable-5, claude-opus-5/4.8/4.6, claude-sonnet-5/4.6, claude-haiku-4.5 |
| **Google Gemini** | 6 | gemini-3.1-pro-preview, gemini-3-pro/3-flash-preview, gemini-2.5-pro/flash/flash-lite |
| **OpenAI** | ~18 | gpt-5.6-sol/terra/luna, gpt-5.5/5.4/5.3/5.2/5, gpt-4.1, gpt-4o/4o-mini, o3/o3-mini/o4-mini, gpt-5.3-codex/spark, gpt-5.2/5.1-codex, gpt-5-codex(-mini), codex-mini, gpt-image-2 |
| **Antigravity** | 10+ | gemini-3.1-pro-high/low, gemini-3-pro-high/low/image, gemini-3-flash, claude-opus/thinking, claude-sonnet-4.6/4.5(-thinking) |
| **OpenRouter** | 4 | anthropic/claude-sonnet-4, google/gemini-2.5-flash, openai/gpt-4o, meta-llama/llama-4-maverick |
| **Kimi** | 1+ | Kimi 月之暗面（apiKey/oauth） |
| **xAI** | 1+ | Grok（apiKey/oauth/manual） |
| **Custom** | ∞ | 任意 OpenAI 兼容 endpoint（Azure/DeepSeek/自托管） |

### Provider 能力矩阵

| Provider | 多模态输入 |  Thinking | 认证方式 | 上下文窗口 |
|---|---|---|---|---|
| Anthropic | vision (text+image+pdf) | thinking blocks | apiKey + oauth | 1M (Haiku 200K, 输出 64K/128K) |
| Google | fullMultimodal | thought parts | apiKey + oauth | 1M |
| OpenAI | vision | Responses API encrypted | apiKey + oauth + manual | 400K (GPT-5), 200K (o3/o4) |
| Antigravity | fullMultimodal | thinking blocks | **oauth 独占** | varies |
| OpenRouter | vision | varies | apiKey + oauth | varies |
| xAI | TBD | reasoning_content | apiKey + oauth + manual | varies |
| Kimi | TBD | reasoning_content | apiKey + oauth | varies |

### Token 峰值追踪

`TokenUsage` 采用 `max` 模式累积（非 sum），对 Anthropic 类 provider 累计上报更准确。

---

## 关键发现与待关注项

| # | 发现 | 类型 |
|---|---|---|
| 1 | Android user_agent 在 browser_use 使用 `desktop_chrome`/`mobile_chrome`（iOS 用 `desktop_safari`/`mobile_safari`），参数枚举差异 | 平台差异 |
| 2 | Android shell_execute stdout/stderr *合并*输出，iOS 分离管道，可能导致模型解析差异 | 平台差异 |
| 3 | Android sandbox 采用 Termux 风格 PRoot（非 iSH），终端为原生 TerminalEmulator.kt 画布渲染 | 架构差异 |
| 4 | Offload 命名不同：iOS `apple-*`，Android `android-*`，但功能对应（calendar/location/clipboard/photos 共有） | 命名约定 |
| 5 | Android 额外存在 Shizuku/Accessibility 集成（a11y_cli/shizuku_cli），默认 NOT_ALLOWED | Android 独占 |
| 6 | 工具描述、参数、required、propertyOrdering 在 iOS 与 Android 间逐行对齐（AgentTools 注释显式引用 iOS 行号） | 一致性亮点 |
| 7 | Modality 推理采用 models.dev 富集优先 + 模式回退 + 专用语音模型精确推断三层 | 能力发现 |
| 8 | 语音模型 (ASR/TTS) 采用精确 modality（非 union），满足 voice picker 严格输入门控 | 能力发现 |

---

*本扫描结果直接基于源码阅读，所有发现的 evidence_type 均为 IMPLEMENTED（未发现 STUB/MOCK/TODO）。*
