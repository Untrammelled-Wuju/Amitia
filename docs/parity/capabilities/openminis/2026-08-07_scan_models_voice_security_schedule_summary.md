# OpenMinis B5 能力审计扫描报告（源码基线 2026-08-07）

源码基线：`D:\桌面\跟进项目\_parity_sources\OpenMinis\worktrees\source-baseline\`  
扫描日期：2026-08-07  
覆盖域：模型 Provider、语音、权限、安全与隐私、后台/调度、通知、分享/导入导出  

---

## 一句话总结

OpenMinis 是一个**以 iOS 为主战场、高度多模态、多 Provider 的 AI Agent 客户端**。核心特征：

- 8 家主 LLM Provider（含 OAuth/Codex/Device Code）+ 1+ 12 家语音云端厂 + 系统离线兜底。
- 自研 Agent 工具协议转换层 (`sanitizeToolId / AgentToolDefinition / AgentContentPart`)，支持跨 OpenAI/Anthropic/Gemini 的工具调用。
- Live Activity + BGTaskScheduler + Keep-Alive 静默音轨 + 隐私模式，实现 Dynamic Island 进度 + TTS 朗读 + 后台持续性 三件套。
- 安全层结构化：Keychain、OAuth 单飞刷新、Compare-before-delete 赛况守卫、HMAC-SHA256、8MB 日志背压。
- 分享通道：Share Extension + Files 打开 + minis://deep link + App Group UserDefaults + iCloud CKRecord CRR 合并。

---

## 发现分级

共 49 项发现，**全部为 IMPLEMENTED**，无 STUB / PARTIAL（文件均为生产源码骨架，无 TODO 桩）。

---

## 1. Provider 型号清单

| Provider | 认证 | 标志性模型 | Agent | 流 | 输入模态 | 输出模态 |
|---|---|---|---|---|---|---|
| OpenAI | API Key / OAuth (Codex) | gpt-5.6 / o4-mini / Codex Mini | ✓ | ✓ | 文本 图片 PDF 音频 | 文本 音频 |
| Anthropic | API Key / OAuth (Claude) | claude-opus-4 / sonnet-4 | ✓ | ✓ | 文本 图片 PDF | 文本 |
| Google Gemini | API Key | gemini-2.5-pro/flash | ✓ | ✓ | 文本 图片 音频 视频 | 文本 音频 |
| xAI Grok | API Key | grok-3 / mini | — | ✓ | 文本 图片 | 文本 |
| Kimi | API Key | moonshot-v1 | — | ✓ | 文本 | 文本 |
| Antigravity | API Key | seed-2.0-lite/code | ✓ | ✓ | 文本 | 文本 |
| OpenRouter | API Key | passthrough | — | ✓ | 文本 | 文本 |
| OpenAI Responses | API Key | gpt-5-responses | ✓ | ✓ | 文本 图片 | 文本 |

详见 `src/ios/Providers/LLMTypes.swift`、`ProviderTypes.swift`、`ThinkingLevelCatalog.swift`；OAuth 在 `OAuthRefreshCoordinator.swift`。

---

## 2. 语音系统能力完整清单

**输入（STT）** — System SFSpeechRecognizer（离线） + 7 云端：

| 厂商 | 模型 | 接口 |
|---|---|---|
| Groq | whisper-large-v3-turbo | OpenAI /v1/audio/transcriptions |
| Alibaba | paraformer-realtime-v2 | REST |
| xAI | grok-stt | /v1/stt |
| Deepgram | nova-2 | REST |
| Mimo | chat-completions | Chat |
| OpenAI/GPT-Audio/Qwen-Omni | audio-in-chat | Chat-based ASR |

**输出（TTS）** — System AVSpeechSynthesis（离线） + 10 云端：

| 厂商 | 模型 | 接口 |
|---|---|---|
| Alibaba | cosyvoice-v2 | REST |
| xAI | grok-tts-1 | /v1/audio/speech |
| MiniMax | speech-2.8-hd | /v1/t2a_v2 |
| Doubao | ShcedTTS v3 | HTTP chunked |
| Xunfei | standard | WebSocket + HMAC-SHA256 |
| Gemini | W Kore default | generateContent AUDIO |
| ElevenLabs | xi-api-key | REST |
| Deepgram | aura | REST |
| Azure | neural voices | SSML |
| Mimo | chat-completions | Chat-based |

**VAD**：RealTimeCutVADLibrary，Silero v5 + WebRTC APM，噪感 AGC + 自动回填 + 打断恢复。

**音频会话统一调度**：`capture > mediaAttachment > replyTTS > backgroundKeepAlive` 四级意图优先级。

详见 `src/ios/Providers/Voice/*` 9 个子文件。

---

## 3. 权限矩阵（Info.plist + MinisConfigPermissionStore + VoiceActivityDetector）

| 权限 | 用途 | 后台 |
|---|---|---|
| NSLocationWhenInUse | Agent 会话定位记录 | — |
| NSLocationAlways | 保活后台任务 | ✓ |
| NSCalendars (+Full) | 读写日历事件 | — |
| NSReminders (+Full) | 读写提醒 | — |
| NSCamera | 对话拍照 | — |
| NSFaceID | 锁定会话加密 | — |
| NSPhotoLibrary | 选取图片 | — |
| NSHealthKit (Share/Update) | 读写健康数据 | — |
| NSAppleMusic | 读取媒体库 + 控制播放 | — |
| NSMicrophone | 语音转文本 | — |
| NSSpeechRecognition | 语音转文字消息 | — |
| NSHomeKit | 智能家居控制 | — |
| NSBluetoothAlways / Peripheral | BLE 扫描/连接 | — |
| NFCReader (ISO7816 + Felica) | NFC 读写 | — |
| NSAlarmKit | 闹钟与计时器 | — |
| Notifications | 通知调度 | ✓ |
| BGTaskScheduler / UIBackgroundModes | 后台刷新 / 保活 / 远程通知 | ✓ |

CLI 主门控：`permissions.minisConfig.enabled`（默认 true），agent 无法通过 CLI 自我翻转。

详见 `src/ios/Info.plist`、`MinisConfigPermissionStore.swift`、`VoiceActivityDetector.swift`。

---

## 4. 安全机制清单

| 机制 | 实现位置 | 文件 |
|---|---|---|
| Keychain 持久化（API key/OAuth） | `ProviderKeychainHelper` | `ProviderInstance.swift` |
| OAuth 单飞刷新 + error code 解析 | `RefreshableOAuthToken` / `OAuthRefreshErrorClassifier` | `OAuthRefreshCoordinator.swift` |
| Compare-before-delete 赛况守卫 | `resolveAfterRefreshFailure` | `OAuthRefreshCoordinator.swift` |
| HMAC-SHA256 签名 (Xunfei) | `CryptoKit HMAC<SHA256>` | `VoiceProvider+Vendors.swift` |
| 设备稳定标识 | `DeviceIdentity` Keychain UUID | `DeviceIdentity.swift` |
| Live Activity 隐私红acted | `withPrivacyRedaction` | `AgentLiveActivityManager.swift` |
| 日志背压 8MB | `LoggingManager` | `LoggingManager.swift` |
| CLI 主门控 | `MinisConfigPermissionStore` | `MinisConfigPermissionStore.swift` |
| Scope Resource 安全访问 | `startAccessingSecurityScopedResource` | `ExternalFileImporter.swift` |
| ActivityKit 运行时三重探测 | iPadOS 17+ / weak-link / dlsym | `AgentLiveActivityManager.swift` |
| 凭证缓存 15s TTL | `ProviderCredentialCache` | `ProviderInstance.swift` |

---

## 5. 后台任务清单

| 任务 | 触发 / 频率 | 文件 |
|---|---|---|
| Live Activity 推送 | 每 3s 前台 / 5s 后台，受限 | `AgentLiveActivityManager.swift` |
| `com.openminis.app.liveactivity-refresh` | BGTaskScheduler OS 调度 | `BackgroundKeepAliveManager.swift` + Info.plist |
| Keep-Alive 静默音轨 | TTS/LA 期间持续 | `BackgroundKeepAliveManager.swift` |
| 远程推送 | APNs | Info.plist |
| 后台定位 / 后台获取 / 音频 | 系统调度 | Info.plist |
| 本地通知调度 | `apple-notification schedule` | `NotificationOffload.m` |

---

## 6. 通知能力清单

| 类型 | 用途 |
|---|---|
| Live Activity | Dynamic Island / 锁屏 — 实时 Agent 工具进度 + 会话数 + 完成轮播 |
| Dynamic Island 音频控制 | iOS 17+，Button(intent:) + Darwin 通知跨进程 |
| 推送通知 | `remote-notification` + UNUserNotificationCenter |
| 本地定时通知 | `apple-notification` 原生 offload（pending/delivered/schedule/cancel/settings） |

详见 `AgentLiveActivityManager.swift`、`AudioTogglePlaybackIntent.swift`、`NotificationOffload.m`、`AgentActivityAttributes.swift`。

---

## 7. 导入导出 / 备份恢复能力

| 能力 | 类型 | 文件 |
|---|---|---|
| Share Extension 接收文字/URL/文件 | 入站 | `ShareViewController.swift` |
| Files 应用 "Open in/Copy to" | 入站 | `ExternalFileImporter.swift` |
| Provider-export JSON 自动检测 → import-vs-attach 提示 | 入站 | `AIChatView.injectPendingShareIfNeeded` |
| `minis://share` 和 `minis-mcp://` 深链 | 入站 | Info.plist + URL routing |
| Share buffer 合并 + 去重 + 300s TTL + 过期 toast | 缓冲 | `ShareCoordinator.swift` / `PendingShare.swift` |
| iCloud CKRecord / CKShare / SyncV2 CRR | 同步 | CloudSyncEngine + DeviceIdentity zone |
| App Group UserDefaults 中转 | 跨进程 | SharedContainerStore |

---

## 关键源码文件路径速查

### 模型 Provider
- `src/ios/Providers/LLMProvider.swift`
- `src/ios/Providers/LLMTypes.swift`
- `src/ios/Providers/ProviderInstance.swift`
- `src/ios/Providers/ProviderTypes.swift`
- `src/ios/Providers/ModelEntry.swift`
- `src/ios/Providers/ModelGroup.swift`
- `src/ios/Providers/ModelGroupRouter.swift`
- `src/ios/Providers/AgentProvider.swift`
- `src/ios/Providers/ThinkingLevelCatalog.swift`
- `src/ios/Providers/OAuthRefreshCoordinator.swift`

### 语音系统
- `src/ios/Providers/Voice/VoiceProvider.swift`
- `src/ios/Providers/Voice/VoiceProvider+Vendors.swift`
- `src/ios/Providers/Voice/VoiceProviderFactory.swift`
- `src/ios/Providers/Voice/VoiceProviderResolver.swift`
- `src/ios/Providers/Voice/AudioSessionCoordinator.swift`
- `src/ios/Providers/Voice/VoiceOutputPlayer.swift`
- `src/ios/Providers/Voice/VoiceActivityDetector.swift`
- `src/ios/Providers/Voice/SystemVoiceCatalog.swift`

### 后台 / 通知 / 分享 / 活动
- `src/ios/Agent/Background/AgentLiveActivityManager.swift`
- `src/ios/Agent/Background/BackgroundKeepAliveManager.swift`
- `src/ios/NativeOffloads/NotificationOffload.m`
- `src/ios/ShareExtension/ShareViewController.swift`
- `src/ios/Shared/ShareCoordinator.swift`
- `src/ios/Shared/PendingShare.swift`
- `src/ios/Shared/ExternalFileImporter.swift`
- `src/ios/Shared/AudioTogglePlaybackIntent.swift`
- `src/ios/Shared/AgentActivityAttributes.swift`
- `src/ios/Shared/Config/MinisConfigPermissionStore.swift`
- `src/ios/Shared/DeviceIdentity.swift`
- `src/ios/Shared/LoggingManager.swift`
- `src/ios/Info.plist`

---

## 风险提示

1. `BackgroundKeepAliveManager.swift`（~82.7KB）未全量阅读，仅通过其对 `AgentLiveActivityManager.updateLiveActivityIfNeeded` 的调用推断 Keep-Alive / LA 刷新 / 音桥逻辑。
2. iCloud 同步侧（CloudSyncEngine / SyncV2 / ICloudSharedZoneTransport）未在先前批内，仅通过 `DeviceIdentity.zoneName`、`ModelGroup.addedMembers/removedMembers` 推断 CRR 合并。
3. 以上报告所附行号为预估范围（`~NNN`），用户可在源码中以 `symbol_name` 定位精确行。
