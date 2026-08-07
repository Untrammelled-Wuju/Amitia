# OpenMinis B5 能力审计扫描报告

**扫描日期**: 2026-08-07
**扫描类型**: B5 能力审计 (完整源码扫描)
**源码范围**: iOS 364 个 Swift 文件 + Android 454 个 Kotlin 文件

---

## 一、iOS Framework 清单

### 完整 Apple Framework 调用清单

| Framework | 调用方式 | 文件位置 | 能力描述 |
|-----------|----------|----------|----------|
| **AlarmKit** | 直接调用 | AlarmOffloadBridge.swift | 闹钟/定时器调度 (iOS 26+) |
| **HomeKit** | 直接调用 | HomeKitOffload.m | 智能家居控制 |
| **CoreBluetooth** | 直接调用 | BluetoothOffload.m | BLE 扫描、连接、读写 |
| **CoreLocation** | 直接调用 | LocationOffload.m | 定位、地理编码 |
| **WeatherKit** | 直接调用 | WeatherOffloadBridge.swift | 天气数据 (当前/每小时/每日) |
| **AVFoundation** | 直接调用 | PlayerOffloadBridge.swift | 音视频播放 |
| **UIKit (UIPasteboard)** | 直接调用 | ClipboardOffload.m | 剪贴板读写 |
| **AppIntents** | 直接调用 | Agent/Intents/*.swift | Siri 快捷指令 (9个 Intent) |
| **WidgetKit** | 直接调用 | AgentWidget/ | Live Activity 小组件 |
| **Intents** | 直接调用 | AgentChatViewModel+ToolDefinitions.swift | Agent 工具定义 |
| **Speech** | 声明 | Info.plist | 语音识别 |
| **HealthKit** | 声明 | Info.plist | 健康数据读写 |
| **EventKit** | 声明 | Info.plist | 日历/提醒 |
| **Photos** | 声明 | Info.plist | 照片库 |
| **CoreNFC** | 声明 | Info.plist | NFC 读写 |
| **NaturalLanguage** | 声明 | Agent/Chat/*.swift | NLP 处理 |
| **UniformTypeIdentifiers** | 直接调用 | ShareExtension, FileProvider | 文档类型 |
| **UserNotifications** | 直接调用 | BackgroundKeepAliveManager.swift | 后台通知 |
| **BackgroundTasks** | 声明 | Info.plist | 后台任务 |
| **CoreSpotlight** | 声明 | Shared/*.swift | 搜索索引 |
| **WebKit** | 直接调用 | BrowserWebView.swift | WebView 浏览器 |
| **MapKit** | 声明 | MapsOffload.h | 地图框架 |
| **MediaPlayer** | 声明 | MediaOffload.h | 媒体播放 |

---

## 二、Android Service/Receiver/Broadcast 清单

### Services
1. **AgentForegroundService** — 前台服务，mediaPlayback 类型，长时间运行
2. **MinisNotificationListenerService** — 通知监听服务
3. **MinisAccessibilityService** — 无障碍服务 (UI自动化)
4. **ShizukuProvider** — Shizuku 特权执行 ContentProvider
5. **FileProvider** — 文件共享
6. **MinisDocumentsProvider** — 文档提供者

### Broadcast Receivers
1. **AlarmReceiver** — 闹钟触发监听 (BOOT_COMPLETED)
2. **ScheduledNotificationReceiver** — 计划通知触发
3. **ScheduledTaskAlarmReceiver** — 计划任务触发

### Activity 入口
1. **MainActivity** — 主界面 (minis:// 深度链接)
2. **OAuthRedirectActivity** — OAuth 回调捕获
3. **WebAppActivity** — PWA 快捷方式
4. **ShareReceiverActivity** — 分享接收 (SEND/SEND_MULTIPLE/VIEW)

### Package Visibility (queries)
- moe.shizuku.privileged.api (Shizuku Manager)
- frb.axeron.manager (AXManager)
- android.speech.RecognitionService
- CustomTabsService

---

## 三、注册的 Tool ID 完整列表

### iOS Native Offload Tools (iSH 命令行)

| Tool ID | 框架 | 实现状态 |
|---------|------|----------|
| apple-alarm | AlarmKit | COMPLETE |
| apple-bluetooth | CoreBluetooth | COMPLETE |
| apple-clipboard | UIKit | COMPLETE |
| apple-location | CoreLocation | COMPLETE |
| apple-homekit | HomeKit | COMPLETE |
| apple-weather | WeatherKit | COMPLETE |
| apple-player | AVFoundation | COMPLETE |
| apple-maps | MapKit | DECLARATION |
| apple-nfc | CoreNFC | DECLARATION |
| apple-nlp | NaturalLanguage | DECLARATION |
| apple-notification | UserNotifications | DECLARATION |
| apple-photos | Photos | DECLARATION |
| apple-reminders | EventKit | DECLARATION |
| apple-speech | Speech | DECLARATION |
| apple-speak | AVFoundation | DECLARATION |
| apple-vision | Vision | DECLARATION |
| apple-media | MediaPlayer | DECLARATION |
| apple-device | UIKit | DECLARATION |
| apple-open | UIKit | DECLARATION |
| apple-healthkit | HealthKit | DECLARATION |
| minis-sessions-cli | Foundation | COMPLETE |
| minis-model-use | Foundation | COMPLETE |
| minis-config | Foundation | DECLARATION |
| minis-browser-use | WebKit | DECLARATION |
| ffmpeg | FFmpeg | DECLARATION |

### Android Native Offload Tools (PRoot 命令行)

| Tool ID | 框架 | 实现状态 |
|---------|------|----------|
| android-alarm | AlarmManager | COMPLETE |
| android-a11y-cli | AccessibilityService | COMPLETE |
| android-calendar | CalendarContract | COMPLETE |
| android-clipboard | ClipboardManager | COMPLETE |
| android-contacts | ContactsContract | COMPLETE |
| android-device | PowerManager/Sensor | COMPLETE |
| android-location | LocationManager | COMPLETE |
| android-model-use | LLM Provider | COMPLETE |
| android-notification | NotificationManager | COMPLETE |
| android-open | Intent | COMPLETE |
| android-photos | MediaStore | COMPLETE |
| android-player | MediaPlayer | COMPLETE |
| android-scheduled-task | WorkManager | COMPLETE |
| android-sessions-cli | Room DB | COMPLETE |
| android-shizuku | Shizuku SDK | COMPLETE |
| android-speech | SpeechRecognizer | COMPLETE |
| android-speak | TextToSpeech | COMPLETE |
| android-weather | OkHttp | COMPLETE |
| android-debug | Debug | COMPLETE |
| android-config | SharedPreferences | COMPLETE |
| android-browser-use | WebView | COMPLETE |

### LLM Agent 工具 (双平台一致)
1. **shell_execute** — 隔离 Linux 进程执行
2. **file_read** — 文件读取
3. **file_write** — 文件写入
4. **file_edit** — 文件编辑 (精确替换)
5. **browser_use** — 浏览器控制
6. **read_image** — 图片读取
7. **memory_write** — 记忆写入
8. **memory_get** — 记忆读取

### iOS App Intents (Siri 快捷指令)
1. AskMinisIntent
2. FollowUpSessionIntent
3. GetSessionStatusIntent
4. ListSessionsIntent
5. MinisShortcutsProvider
6. OpenSessionIntent
7. QuickTaskIntent
8. RetryRunIntent
9. SendPromptIntent

---

## 四、发现的 API Endpoint 列表

### HTTP API 调用 (iOS)
- **WeatherKit**: https://weatherkit.apple.com (WeatherService.shared)
- **Anthropic**: https://api.anthropic.com
- **OpenAI**: https://api.openai.com
- **Gemini**: https://generativelanguage.googleapis.com
- **xAI**: https://api.x.ai
- **OpenRouter**: https://openrouter.ai/api
- **Models.dev**: https://models.dev/api

### HTTP API 调用 (Android)
- **Open-Meteo**: https://api.open-meteo.com/v1/forecast (天气)
- **Anthropic**: https://api.anthropic.com (OkHttp)
- **OpenAI**: https://api.openai.com (OkHttp)
- **Gemini**: https://generativelanguage.googleapis.com (OkHttp)
- **xAI**: https://api.x.ai (OkHttp)
- **OpenRouter**: https://openrouter.ai/api (OkHttp)
- **Models.dev**: https://models.dev/api (OkHttp)

### OAuth 回调端点 (双平台)
- **Claude**: http://localhost:54545/callback
- **OpenAI**: http://localhost:1455/auth/callback
- **Gemini**: http://localhost:8085/oauth2callback
- **OpenRouter**: http://localhost:3000-3002/callback

---

## 五、发现的权限列表

### iOS 权限声明 (Info.plist)
1. NSLocationWhenInUseUsageDescription
2. NSLocationAlwaysAndWhenInUseUsageDescription
3. NSCalendarsUsageDescription
4. NSCalendarsFullAccessUsageDescription
5. NSRemindersUsageDescription
6. NSRemindersFullAccessUsageDescription
7. NSCameraUsageDescription
8. NSFaceIDUsageDescription
9. NSPhotoLibraryUsageDescription
10. NSPhotoLibraryAddUsageDescription
11. NSHealthShareUsageDescription
12. NSHealthUpdateUsageDescription
13. NSAppleMusicUsageDescription
14. NSMicrophoneUsageDescription
15. NSSpeechRecognitionUsageDescription
16. NSHomeKitUsageDescription
17. NSBluetoothAlwaysUsageDescription
18. NSBluetoothPeripheralUsageDescription
19. NFCReaderUsageDescription
20. NSLocalNetworkUsageDescription
21. NSAlarmKitUsageDescription
22. NSSupportsLiveActivities
23. NSSupportsLiveActivitiesFrequentUpdates
24. UIBackgroundModes: audio, fetch, location, remote-notification
25. BGTaskSchedulerPermittedIdentifiers

### Android 权限声明 (Manifest)
1. INTERNET
2. REQUEST_INSTALL_PACKAGES
3. RECORD_AUDIO
4. FOREGROUND_SERVICE
5. FOREGROUND_SERVICE_MEDIA_PLAYBACK
6. WAKE_LOCK
7. SYSTEM_ALERT_WINDOW
8. REQUEST_IGNORE_BATTERY_OPTIMIZATIONS
9. POST_NOTIFICATIONS
10. POST_PROMOTED_NOTIFICATIONS
11. SCHEDULE_EXACT_ALARM
12. RECEIVE_BOOT_COMPLETED
13. ACCESS_NETWORK_STATE
14. READ_CALENDAR
15. WRITE_CALENDAR
16. ACCESS_FINE_LOCATION
17. ACCESS_COARSE_LOCATION
18. READ_CONTACTS
19. WRITE_CONTACTS
20. READ_MEDIA_IMAGES
21. READ_MEDIA_VISUAL_USER_SELECTED
22. ACCESS_MEDIA_LOCATION
23. READ_EXTERNAL_STORAGE (legacy)
24. WRITE_EXTERNAL_STORAGE (legacy)
25. MANAGE_EXTERNAL_STORAGE
26. CAMERA
27. com.android.alarm.permission.SET_ALARM
28. moe.shizuku.manager.permission.API_V23
29. BIND_NOTIFICATION_LISTENER_SERVICE
30. BIND_ACCESSIBILITY_SERVICE
31. INTERACT_ACROSS_USERS_FULL

---

## 六、关键能力发现

### iOS 特有能力
1. **AlarmKit** — 原生闹钟/定时器 (iOS 26+ 独占)
2. **HomeKit** — 完整智能家居 HMHomeManager 实现
3. **WeatherKit** — WeatherService.shared 完整封装
4. **AppIntents** — 9个 Siri 快捷指令
5. **WidgetKit** — Live Activity 小组件
6. **BackgroundKeepAliveManager** — 后台保活 (静音音频)
7. **FileProviderExtension** — 系统文件提供者
8. **ShareExtension** — 系统分享扩展

### Android 特有能力
1. **Shizuku** — 特权执行框架 (支持 Shizuku + AXManager)
2. **AccessibilityService** — UI 自动化 (截屏、点击、输入等)
3. **NotificationListenerService** — 全局通知读取
4. **AgentForegroundService** — 媒体播放前台服务
5. **MediaProjection** — 屏幕捕获 (通过 AccessibilityService)
6. **WorkManager** — 计划任务
7. **SYSTEM_ALERT_WINDOW** — 悬浮窗
8. **MANAGE_EXTERNAL_STORAGE** — 全文件访问
9. **POST_PROMOTED_NOTIFICATIONS** — Android 16 动态岛

### 双平台共享能力
1. **shell_execute** — Alpine Linux (iOS: iSH, Android: PRoot)
2. **browser_use** — WebView 浏览器
3. **file_read/write/edit** — 文件系统
4. **memory_write/get** — 持久化记忆
5. **OAuth** — 5个 Provider 的 OAuth 支持
6. **LLM Providers** — 7个 AI 模型提供者
7. **Tool Permission Gate** — 工具权限三态门控

### Mock/Stub/TODO 标记
- **iOS**: apple-maps, apple-nfc, apple-nlp, apple-notification, apple-photos, apple-reminders, apple-speech, apple-speak, apple-vision, apple-media, apple-device, apple-open, minis-config, minis-browser-use (仅有头文件声明，.m 文件未找到)
- **Android**: HealthManager.kt (STUB — 未实现 Health Connect SDK，仅返回字符串提示)

---

## 七、架构模式

### iOS 架构
- **SwiftUI** 主界面 + UIKit 桥接
- **App Intents** 框架处理 Siri 集成
- **Native Offload** 系统将 iSH 命令路由到原生实现 (native_offload_add_handler)
- **Combine** 响应式数据流
- **SQLite3** 原生数据库 (无 ORM)
- **URLSession/Alamofire** HTTP 客户端
- **AgentToolDefinition** 统一的工具定义模型

### Android 架构
- **Jetpack Compose** 主界面 + Activity/Fragment
- **PRoot + Alpine Linux** 替代 iSH
- **NativeOffloadHandler** 接口处理 shell 命令路由
- **OkHttp** HTTP 客户端
- **Room** 数据库 ORM
- **Coroutines** 异步处理
- **Shizuku SDK** 特权执行
- **AgentToolDefinition** 统一的工具定义模型

---

## 八、合规性/安全观察

### 双平台一致
1. **OffloadPermissionGate** — 工具权限三态门控 (BYPASS/ASK_ONCE/NOT_ALLOWED)
2. **OAuth 回调** — localhost 回调捕获 (AndroidManifest/Info.plist)
3. **API Key 保护** — ProviderConfig 加密存储
4. **容器沙箱** — 命令在隔离环境中执行
5. **证书固定** — Network Security Config

### iOS 独有
- **FileProvider** 暴露容器文件系统
- **ShareExtension** 扩展共享入口
- **Live Activity** 持续状态显示
- **Background Mode** 后台音频保活

### Android 独有
- **Shizuku** 需要单独授予特权 (API_V23)
- **AccessibilityService** 需要用户手动开启
- **NotificationListenerService** 需要系统设置授权
- **SYSTEM_ALERT_WINDOW** 需要悬浮窗权限
- **BATTERY_OPTIMIZATION** 需要忽略电池优化
- **Exact Alarm** 需要 SCHEDULE_EXACT_ALARM 权限

---

**扫描完成时间**: 2026-08-07
**完整 JSON 数据**: _scan_ios_android.json
**总计发现**: 25+ iOS Native Tools, 20+ Android Native Tools, 8 个共享 Agent Tools, 9 个 App Intents