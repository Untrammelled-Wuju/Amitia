# Tasks — Amitia Android 原生客户端构建（第一阶段）

> 来源：`AndroidAPP/stage.md` 第二十九节执行步骤（51 步）+ 第三十一节验收标准
> 依赖关系：Phase 0（审计）→ Phase 1（工程骨架）→ Phase 2（Runtime）→ Phase 3（后端 ARM64）→ Phase 4（连接层）→ Phase 5（UI 功能）→ Phase 6（系统集成）→ Phase 7（测试构建验收）→ Phase 8（文档收尾）
> 并行策略：同一 Phase 内无依赖的子任务可并行；不同 Phase 严格顺序。

---

## Phase 0 — 仓库审计与兼容性评估（stage.md 步骤 1-11）

- [x] Task 0.1: 扫描 Go 后端完整链路（入口/配置/SQLite/Qdrant/SurrealDB/迁移/路由/流式协议/TTS/主动消息/角色/记忆/模型/渠道），记录真实路径、端口、启动参数、数据目录
  - [x] SubTask 0.1.1: 定位 `backend/cmd/server/main.go` 入口、`config/config.yml` 端口（18899/19178/18000）、`glebarez/sqlite` 驱动、`internal/migration/` 迁移
  - [x] SubTask 0.1.2: 定位 `internal/chat/handler.go` SSE/WebSocket 事件名（`message_start`/`delta`/`message_end`/`tool_call`）
  - [x] SubTask 0.1.3: 定位 `internal/tts/`（edge-tts + 豆包）、`internal/proactive/`、`internal/character/`、`internal/memory/`、`internal/model/`、`internal/channel/`
  - [x] SubTask 0.1.4: 标记所有 Windows 专属代码（`cmd`/`taskkill`/`syscall`/绝对盘符/Electron 环境变量）

- [x] Task 0.2: 扫描 Web 前端（`front/`）与 Electron（`desktop/`），确认页面、请求层、流式协议、Electron 启动后端方式
  - [x] SubTask 0.2.1: 确认 `front/src/composables/useChatSSE.ts` 事件名与后端一致
  - [x] SubTask 0.2.2: 确认 `desktop/src/runtime/process-supervisor.ts` 如何 spawn `server.exe` + Qdrant + SurrealDB

- [x] Task 0.3: 生成 `docs/android/01-current-system-audit.md`（后端入口/启动命令/Go 版本/CGO/SQLite 驱动/路径/迁移/Qdrant/SurrealDB/环境变量/Windows 专属代码/API/实时协议/用户功能）
- [x] Task 0.4: 生成 `docs/android/02-capability-migration-matrix.md`（功能迁移矩阵表，状态只用「已真实实现/部分实现/仅 UI/后端缺失/已废弃/无法确认」）
- [x] Task 0.5: 生成 `docs/android/03-runtime-dependency-audit.md`（Go/SQLite/Qdrant/SurrealDB ARM64 兼容性、RootFS 体积、内存、端口冲突、Android 后台限制、`/proc`/`/dev`/DNS、PRoot 兼容性、进程退出、升级迁移）

## Phase 1 — Android 工程骨架（stage.md 步骤 12-13）

- [ ] Task 1.1: 在 `android/` 创建 Gradle Kotlin DSL 工程根（settings.gradle.kts / build.gradle.kts / gradle wrapper / libs.versions.toml）
- [ ] Task 1.2: 创建 `app` 模块（Application + MainActivity + AndroidManifest + 最小 Compose UI 跑通）
- [ ] Task 1.3: 创建 `core` 子模块（common/model/network/database/datastore/security/designsystem/media/logging），可按 Gradle 稳定性合并但保持包结构清晰
- [ ] Task 1.4: 创建 `runtime` 子模块（api/manager/linux/process/bootstrap/health/bridge）
- [ ] Task 1.5: 创建 `platform` 子模块（notification/files/audio/permissions/foreground）
- [ ] Task 1.6: 创建 `feature` 子模块（startup/onboarding/auth/home/chat/character/memory/models/channels/runtime/settings）
- [ ] Task 1.7: 创建 `native` 子模块（proot JNI 壳）
- [ ] Task 1.8: 建立 Design System（深色优先、低饱和、克制毛玻璃、Material 3 主题、字体、颜色 token、少量动效），生成 `docs/android/08-ui-design-system.md`

## Phase 2 — Runtime 状态机与 RootFS 管理（stage.md 步骤 14-16）

- [ ] Task 2.1: 实现 Runtime 状态机（NotInstalled/Installing/Installed/Starting/Running/Degraded/Stopping/Stopped/Failed/Updating），每状态含阶段/进度/可读信息/错误/可重试/需用户操作
- [ ] Task 2.2: 实现 `LinuxRootfsManager`（检测已安装/版本/首次解压/真实进度/完整性校验/失败重试/升级/RootFS 与用户数据分离/清理二次确认/不在主线程解压）
- [ ] Task 2.3: 实现 `LinuxProcessManager`（启动/环境变量/工作目录/stdout/stderr/退出码/超时/终止/强制终止/状态/崩溃次数/最后启动时间/最后退出原因/日志滚动/重启策略/防重复启动/应用退出释放）
- [ ] Task 2.4: 实现目录布局（`files/runtime/{rootfs,bin,logs,tmp,versions}` + `files/amitia-data/{sqlite,qdrant,surrealdb,uploads,models,extensions,backups}`），用户数据不进 RootFS 覆盖区
- [ ] Task 2.5: 实现启动顺序（RootFS → Runtime 文件 → 数据目录 → 配置 → SurrealDB → 等 SurrealDB 健康 → Qdrant → 等 Qdrant 健康 → Go 后端 → 等 Go 后端健康 → Repository 连接 → UI 显示运行）与停止顺序
- [ ] Task 2.6: 实现健康检查（端口/HTTP/进程状态，禁止固定延时），生成 `docs/android/05-linux-runtime-design.md` + `docs/android/06-process-lifecycle.md`

## Phase 3 — Go 后端 Linux ARM64 与数据库（stage.md 步骤 17-19）

- [ ] Task 3.1: 抽象 Go 后端平台层（`DesktopRuntime` / `AndroidEmbeddedRuntime` / `ServerRuntime`），隔离 `main.go:killExistingServer` 的 `cmd/taskkill` 与 `internal/mcp/transport/process_windows.go` 的 Windows 专属代码
- [ ] Task 3.2: 修改 Qdrant/SurrealDB 启动管理器支持 Linux ARM64 二进制路径与启动参数，Windows 仍用 `.exe`
- [ ] Task 3.3: 执行 `GOOS=linux GOARCH=arm64 go build -o amitia-backend-arm64`，记录命令/退出码/错误/修复
- [ ] Task 3.4: 准备 SurrealDB Linux ARM64 二进制（获取或编译），验证可执行/存储引擎/数据目录持久化/端口/认证/Go 后端可连接/重启数据存在/异常恢复
- [ ] Task 3.5: 准备 Qdrant Linux ARM64 二进制（获取或编译），验证 10 项（版本/启动参数/端口/数据目录/健康检查/重启数据/Go 后端连接/ARM64 真机/兼容降级/VectorStore Adapter 预留）
- [ ] Task 3.6: 验证 SQLite 在 Linux ARM64 持久化（glebarez/sqlite 纯 Go 无 CGO）、迁移、备份恢复、并发、文件锁、异常退出、损坏检测
- [ ] Task 3.7: 验证修改后 Windows Electron + Web 仍可启动，现有 SQLite/Qdrant/SurrealDB 数据不被破坏

## Phase 4 — Android 连接层（stage.md 步骤 20-22）

- [ ] Task 4.1: 实现 `RuntimeEndpointProvider` / `ServerEndpoint`（本地 `http://127.0.0.1:18899` + 远程用户配置地址），不硬编码地址
- [ ] Task 4.2: 实现 `AmitiaApiClient`（Retrofit + OkHttp），支持 REST/SSE/WebSocket/流式 HTTP/文件上传/图片/音频/超时/重连/Token/错误映射/连接状态
- [ ] Task 4.3: 实现 `ConnectionManager` + `SessionManager`，本地 API 随机鉴权令牌，仅监听 127.0.0.1
- [ ] Task 4.4: 实现统一 Repository 层，本地与远程共用同一套 Repository/领域模型/UI
- [ ] Task 4.5: 生成 `docs/android/07-api-mapping.md`（API 映射表 + 流式协议字段对齐）

## Phase 5 — UI 功能迁移（stage.md 步骤 23-31, 33-38）

- [x] Task 5.1: 首次启动引导（欢迎 → 模式选择 → Runtime 安装/远程配置 → 环境检查 → 登录/单用户初始化 → 模型配置 → 角色设置 → 初始记忆 → 完成），真实进度、可中断恢复、不重复安装、失败显示真实原因
  - [x] SubTask 5.1.1: OnboardingViewModel + OnboardingDataStore（DataStore 持久化步骤索引，支持中断恢复）
  - [x] SubTask 5.1.2: 9 步引导 Composable（WelcomeStep/ModeSelectionStep/RuntimeInstallStep/RemoteConfigStep/EnvCheckStep/AuthInitStep/ModelConfigStep/CharacterSetupStep/InitialMemoryStep/CompleteStep）
  - [x] SubTask 5.1.3: OnboardingScreen 整合进度条 + 上一步/下一步/跳过 + 错误显示
- [x] Task 5.2: 账户与会话（登录/Token/Keystore 存储）
  - [x] SubTask 5.2.1: AuthViewModel + AuthScreen（用户名/密码/Token 三段式登录）
  - [x] SubTask 5.2.2: SessionManager 集成 + 登录成功跳转 HOME
- [x] Task 5.3: 首页（当前角色/状态/最近对话/主动消息/继续对话/Runtime 状态/异常，不堆叠设置入口）
  - [x] SubTask 5.3.1: HomeViewModel（聚合当前角色/最近会话/主动消息/Runtime 状态）
  - [x] SubTask 5.3.2: HomeScreen（CurrentCharacterCard + RuntimeStatusCard + ConversationRow + ProactiveMessageRow）
- [x] Task 5.4: 角色系统（列表/当前/切换/头像/名称/身份/性格/提示词摘要/状态/创建/编辑/删除确认/独立聊天/独立记忆/独立语音/独立模型/独立主动消息，切换不混用消息/草稿/记忆/情绪/TTS/生成状态/未读数）
  - [x] SubTask 5.4.1: CharacterViewModel（load/switch/create/update/delete）+ CharacterDto 隔离
  - [x] SubTask 5.4.2: CharacterScreen（列表）+ CharacterDetailScreen（详情）+ CharacterEditScreen（创建/编辑）+ CharacterDeleteDialog（删除确认）
- [x] Task 5.5: 聊天历史（会话列表/分页/文本/流式回复/用户消息状态/AI 生成状态/失败重试/复制/删除/系统消息/主动消息/日期分组/草稿/键盘适配/返回位置恢复/消息去重/页面重建不中断）
  - [x] SubTask 5.5.1: ChatViewModel（loadConversation/sendMessage/retry/copy/delete）+ ChatDataStore（草稿持久化）
  - [x] SubTask 5.5.2: ChatScreen（消息列表 + 输入栏 + DateHeader 日期分组）+ MessageBubble + ChatInputBar
  - [x] SubTask 5.5.3: ChatListPlaceholder（characterId 为 null 时显示空状态，对应底部 Tab "对话"）
- [x] Task 5.6: 流式回复严格对齐 `useChatSSE.ts` + `internal/chat/handler.go` 真实事件名，不一致以最小修改修复并保持 Web/Electron 兼容
  - [x] SubTask 5.6.1: ChatViewModel SSE 事件解析（message_start/token/voice_audio/message_end）+ extractTokenText/extractField
- [x] Task 5.7: 图片消息（相册/相机/预览/压缩/上传/进度/失败重试/移除/多图限制/类型校验/图片上下文，最小权限）
  - [x] SubTask 5.7.1: ImagePickerHelper（相册/相机）+ ImageCompressor（MAX_IMAGES 限制 + 类型校验）+ ImageUploader（上传 + 进度）+ ImagePreviewDialog（预览/移除）
- [x] Task 5.8: 语音消息（麦克风权限/录音/取消/时长/音量/上传/播放/暂停/进度/音频焦点/资源释放/错误恢复）
  - [x] SubTask 5.8.1: AudioRecorderController（录音/取消/时长/音量）+ AudioPlayerController（播放/暂停/进度/音频焦点）+ VoiceRecorderButton + VoiceMessageBubble
- [x] Task 5.9: TTS（复用后端 edge-tts + 豆包，角色声音/失败回退/自动播放/音频 URL/流式结束事件，不重建调度系统）
  - [x] SubTask 5.9.1: TtsController（Media3 播放 + 失败回退 + 自动播放）+ TtsPreferences（角色声音偏好）
- [x] Task 5.10: 记忆系统（长期/情景/初始/世界书/时间线/搜索/图谱摘要/来源/CRUD/按角色筛选，首版列表+关系详情即可，必须用真实 SurrealDB 数据）
  - [x] SubTask 5.10.1: MemoryViewModel（load/search/create/update/delete/loadTimeline/loadGraph）+ MemoryUiState（typeFilter/characterFilter/timeline/graphSummary）
  - [x] SubTask 5.10.2: MemoryScreen（TabRow: 记忆列表/时间线/图谱）+ MemoryDetailScreen + MemoryEditScreen + SearchAndFilterBar + TypeFilterRow + CharacterFilterRow
- [x] Task 5.11: 模型配置、渠道状态、设置（本地/远程模式、Runtime 管理、远程地址、主题、通知、语音、缓存、备份、日志、关于、版本、退出登录）
  - [x] SubTask 5.11.1: ModelsViewModel + ModelsScreen（模型列表 + 选择）
  - [x] SubTask 5.11.2: ChannelsViewModel + ChannelsScreen（微信/QQ/Web 渠道状态 + 绑定/解绑）
  - [x] SubTask 5.11.3: SettingsViewModel + SettingsDataStore + SettingsScreen（主题/通知/TTS/语音/远程地址/缓存/备份/日志/版本/退出登录）+ RemoteConfigDialog
- [x] Task 5.12: Runtime 管理页面（模式/RootFS 版本/Runtime 状态/Go 后端/Qdrant/SurrealDB/端口/运行时长/内存/数据占用/日志/最后错误/启动/停止/重启/修复/更新/导出诊断/清理/备份/恢复，危险操作二次确认，清理不删用户数据）
  - [x] SubTask 5.12.1: RuntimeViewModel（start/stop/restart/repair/update/exportDiagnostics/cleanup/backup/restore + observeRuntimeState/observeRuntimeEvents + buildPorts/computeDataUsage/readRootfsVersion）
  - [x] SubTask 5.12.2: RuntimeScreen（HeaderCard + ServiceCard ×3 + DataUsageCard + LogsCard + ActionGrid + ActionButton）+ RuntimeActionDialog（危险操作二次确认）+ DiagnosticsExporter

## Phase 6 — 系统集成（stage.md 步骤 34, 39-42）

- [ ] Task 6.1: 主动消息 + Android 通知（前台主动消息/后台通知/点击进入对应角色会话/相同消息去重/角色级通知设置/权限/隐私/重启未读恢复）
- [ ] Task 6.2: Room 缓存层（最近角色/会话/消息/发送状态/草稿/Runtime 状态快照/主动消息/待重试任务），核心业务数据不进 Room
- [ ] Task 6.3: DataStore 偏好（运行模式/远程地址/当前角色/主题/通知/语音/引导状态/Runtime 版本）
- [ ] Task 6.4: Android Keystore 凭据（Token/本地通信密钥/敏感凭据）
- [ ] Task 6.5: 错误处理统一（RootFS 未安装/安装失败/Runtime 启动失败/Qdrant/SurrealDB/Go 后端/端口冲突/二进制不兼容/数据目录无权限/服务超时/Token 失效/网络不可用/远程不可达/流式断开/上传失败/音频失败/迁移失败/Runtime 被杀/未知），错误页可理解/重试/诊断/日志导出/不泄敏/单点失败不崩应用
- [ ] Task 6.6: Foreground Service + 运行策略（AlwaysOn/OnDemand/RemoteOnly，常驻通知，节能模式，系统杀进程恢复，不高频轮询假活跃，默认策略按实测）
- [ ] Task 6.7: Native Capability Bridge（文件/图片/相机/麦克风/音频/通知/剪贴板/分享/应用目录/主题/网络/电池/前后台，PermissionBroker 校验，预留 Accessibility/MediaProjection/悬浮窗/Shizuku/Root/屏幕理解/手势/Computer Use）
- [ ] Task 6.8: 日志与诊断（日志滚动、脱敏、导出诊断包）

## Phase 7 — 测试与构建（stage.md 步骤 43-49）

- [ ] Task 7.1: 单元测试（Runtime 状态机/启动顺序/停止顺序/进程退出/健康检查/Endpoint 切换/Token 保存/API 错误映射/流式事件解析/消息去重/角色切换/草稿/Runtime 版本迁移/数据目录策略）
- [ ] Task 7.2: 集成测试（Android → 启动 Linux → SurrealDB → Qdrant → Go 后端 → 健康接口 → 获取角色 → 获取聊天 → 发送消息 → 接收流式回复）
- [ ] Task 7.3: UI 测试（首次启动/选本地模式/安装 Runtime/启动服务/登录/进首页/切角色/发消息/收流式/看记忆/看 Runtime/停止重启/切远程/深色亮色）
- [ ] Task 7.4: 执行 `./gradlew clean` + `./gradlew test` + `./gradlew lint` + `./gradlew assembleDebug`，记录命令/退出码/错误/修复
- [ ] Task 7.5: 构建 Go 后端 ARM64 + RootFS 分发包 + Qdrant/SurrealDB ARM64，记录所有产物路径
- [ ] Task 7.6: ARM64 真机验收（Android 版本/CPU/RootFS 解压/PRoot/Qdrant/SurrealDB/Go 后端/SQLite/网络/SSE/WebSocket/音频/图片/后台/前台服务/系统杀进程/重启恢复/低电量/网络切换/屏幕旋转/字体缩放/深色），模拟器结果不替代真机；如真机不可用明确记录外部阻塞
- [ ] Task 7.7: 修复发现的问题，重新跑测试与构建直到通过

## Phase 8 — 文档收尾与最终报告（stage.md 步骤 50-51）

- [ ] Task 8.1: 生成 `docs/android/04-android-architecture.md`
- [ ] Task 8.2: 生成 `docs/android/09-build-and-run.md`（构建命令、退出码、APK 路径、Go ARM64 路径、RootFS 路径、Qdrant/SurrealDB ARM64 路径）
- [ ] Task 8.3: 生成 `docs/android/10-testing-report.md`（单元/集成/UI/真机结果）
- [ ] Task 8.4: 生成 `docs/android/11-migration-report.md`（已迁移/部分迁移/未迁移功能 + 原因 + 新增 Android 文件 + Go 后端修改文件 + Qdrant/SurrealDB/RootFS 处理方式 + 测试构建结果 + APK 路径 + 真机结果 + 阻塞 + 是否达到第一阶段验收）
- [ ] Task 8.5: 生成 `docs/android/12-known-limitations.md` + `docs/android/13-next-stage-plan.md`
- [ ] Task 8.6: 生成 `docs/android/third-party-licenses.md`（所有第三方组件 + RootFS/Qdrant/SurrealDB/PRoot 许可证）
- [ ] Task 8.7: 最终验收报告，按 stage.md 第三十二节 20 项格式回复

# Task Dependencies

- Phase 1 依赖 Phase 0 完成（需先有审计结论才能正确建立工程）
- Phase 2 依赖 Phase 1（Runtime 模块需要工程骨架）
- Phase 3 与 Phase 2 可部分并行（Go 后端 ARM64 构建不依赖 Android Runtime，但启动顺序集成依赖 Phase 2）
- Phase 4 依赖 Phase 2 + Phase 3（连接层需要 Runtime 状态 + 后端 ARM64 可用）
- Phase 5 依赖 Phase 4（UI 需要 Repository 层）
- Phase 5 内部：Task 5.1（引导）→ 5.2（账户）→ 5.3（首页）→ 5.4（角色）→ 5.5（聊天）→ 5.6（流式）→ 5.7-5.10（图片/语音/TTS/记忆）→ 5.11（模型/渠道/设置）→ 5.12（Runtime 页面）
- Phase 6 依赖 Phase 5（通知需要聊天/角色数据，Bridge 需要 Runtime）
- Phase 7 依赖 Phase 6（测试需要完整集成）
- Phase 8 依赖 Phase 7（文档需要测试与构建结果）
- Phase 0 内 Task 0.1/0.2 可并行，Task 0.3/0.4/0.5 依赖 0.1+0.2 完成后并行
- Phase 5 内 Task 5.7/5.8/5.9/5.10 可并行（互不依赖）
