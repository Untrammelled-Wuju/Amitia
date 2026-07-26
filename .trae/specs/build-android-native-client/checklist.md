# Checklist — Amitia Android 原生客户端构建（第一阶段）

> 来源：`AndroidAPP/stage.md` 第三十一节验收标准（38 项）+ 第三十二节最终回复格式（20 项）+ 关键工程检查点
> 用法：每完成一项打勾。验收阶段必须逐项核实代码/文档/构建产物，不允许仅凭声明。

---

## A. 工程与编译（stage.md 验收 1-4, 32-34）

- [ ] A1: `android/` 目录真实存在 Android 原生工程（非空壳）
- [ ] A2: 使用 Kotlin + Jetpack Compose（非 Flutter/RN/WebView 套壳）
- [ ] A3: 使用 Gradle Kotlin DSL（`build.gradle.kts` + `settings.gradle.kts`）
- [ ] A4: 使用 Hilt + Material 3 + Navigation Compose + Retrofit + OkHttp + Kotlinx Serialization + Room + DataStore + Coil + Media3 + WorkManager + Foreground Service
- [ ] A5: 工程按 `core/runtime/platform/feature/native` 分层，模块职责清晰，无大量空模块
- [ ] A6: `./gradlew clean` 实际执行，退出码 0
- [ ] A7: `./gradlew test` 实际执行，退出码 0
- [ ] A8: `./gradlew lint` 实际执行，退出码 0
- [ ] A9: `./gradlew assembleDebug` 实际执行，Debug APK 真实生成，记录路径

## B. 内嵌 Linux Runtime（stage.md 验收 5-7, 21-24）

- [ ] B1: Linux RootFS 可在 Android 私有目录安装（`files/runtime/rootfs/`）
- [ ] B2: Linux 用户空间可启动（PRoot/proot-rs 路线，无 Root，不依赖 Termux）
- [ ] B3: RootFS 安装显示真实进度（非固定延时），支持校验/重试/升级
- [ ] B4: RootFS 与用户数据分离（`files/amitia-data/` 不被升级覆盖）
- [ ] B5: Runtime 状态机 10 状态完整（NotInstalled/Installing/Installed/Starting/Running/Degraded/Stopping/Stopped/Failed/Updating），每状态含阶段/进度/可读信息/错误/可重试/需用户操作
- [ ] B6: 启动顺序严格（RootFS → SurrealDB → 健康 → Qdrant → 健康 → Go 后端 → 健康 → UI 显示运行成功）
- [ ] B7: 健康检查通过端口/HTTP/进程状态，禁止固定延时猜测
- [ ] B8: 停止顺序严格（停接受新请求 → Go 后端 → Qdrant → SurrealDB → 刷新日志状态）
- [ ] B9: App 重启后可恢复本地 Runtime 状态
- [ ] B10: App 被系统结束后不损坏数据

## C. Go 后端 Linux ARM64（stage.md 验收 7, 36）

- [ ] C1: `GOOS=linux GOARCH=arm64 go build` 实际执行成功，产物路径记录
- [ ] C2: 未启用 CGO（glebarez/sqlite 纯 Go 驱动），交叉编译无平台依赖错误
- [ ] C3: Windows 专属代码（`cmd`/`taskkill`/`syscall` Windows API/绝对盘符/Electron 环境变量）已抽象为 `DesktopRuntime`/`AndroidEmbeddedRuntime`/`ServerRuntime` 平台接口
- [ ] C4: Go 后端 Linux ARM64 可在内嵌 Linux 中启动
- [ ] C5: 修改后 Windows Electron 客户端仍可启动（不破坏桌面端）
- [ ] C6: 修改后 Web 端仍可连接（不破坏 Web）
- [ ] C7: 修改后远程部署仍可运行
- [ ] C8: 未复制分叉后端，业务逻辑复用现有代码

## D. 数据库与持久化（stage.md 验收 8-10）

- [ ] D1: SQLite Linux ARM64 持久化正常，数据目录 `files/amitia-data/sqlite/`
- [ ] D2: SQLite 迁移在 Linux ARM64 正常执行
- [ ] D3: SQLite 备份/恢复/并发/文件锁/异常退出/损坏检测可用
- [ ] D4: Android Room 仅作 UI 缓存，不直接修改 Amitia 核心 SQLite 数据库
- [ ] D5: Qdrant Linux ARM64 可启动并持久化数据，端口 19178
- [ ] D6: Qdrant 重启后数据仍存在，Go 后端可正常连接
- [ ] D7: Qdrant 不可用时记录真实错误并尝试兼容/降级/Adapter，不静默忽略
- [ ] D8: SurrealDB Linux ARM64 可启动并持久化数据，端口 18000，dataPath `data/graph.db`
- [ ] D9: SurrealDB 不可用时进入 Degraded 状态，不谎报全部能力正常
- [ ] D10: 现有 Qdrant/SurrealDB 数据不被破坏

## E. 连接与双模式（stage.md 验收 11-12）

- [ ] E1: 实现 `AmitiaApiClient`/`RuntimeEndpointProvider`/`ConnectionManager`/`SessionManager`
- [ ] E2: 本地模式默认连接 `http://127.0.0.1:18899`，仅监听 127.0.0.1（不监听 0.0.0.0）
- [ ] E3: 远程模式连接用户配置地址
- [ ] E4: 本地/远程共用同一套 Repository/领域模型/UI，无两套页面
- [ ] E5: 支持 REST/SSE/WebSocket/流式 HTTP/文件上传/图片/音频/超时/重连/Token/错误映射/连接状态
- [ ] E6: 本地 API 使用随机鉴权令牌，防止其他应用调用敏感 API
- [ ] E7: 不在页面硬编码 `127.0.0.1` 或远程地址

## F. 角色与聊天（stage.md 验收 13-17）

- [ ] F1: 角色列表/当前/切换/头像/名称/身份/性格/提示词摘要/状态/创建/编辑/删除确认全部来自真实 Go 后端
- [ ] F2: 切换角色不混用消息/草稿/记忆/情绪/TTS/生成状态/未读数
- [ ] F3: 角色独立聊天记录/记忆/语音/模型配置/主动消息状态
- [ ] F4: 聊天历史/分页/文本/流式回复/状态/失败重试/复制/删除/系统消息/主动消息/日期分组/草稿/键盘适配/返回位置恢复/消息去重/页面重建不中断
- [ ] F5: 流式回复严格对齐 `useChatSSE.ts` + `internal/chat/handler.go` 真实事件名（`message_start`/`delta`/`message_end`/`tool_call` 等），未自行猜测
- [ ] F6: 用户消息可真实发送到后端
- [ ] F7: AI 回复通过真实流式协议显示，非固定 JSON/非延时动画

## G. 图片/语音/TTS/记忆（stage.md 验收 18）

- [ ] G1: 图片消息（相册/相机/预览/压缩/上传/进度/失败重试/移除/多图限制/类型校验/图片上下文）真实可用，最小权限
- [ ] G2: 语音消息（麦克风/录音/取消/时长/音量/上传/播放/暂停/进度/音频焦点/资源释放/错误恢复）真实可用
- [ ] G3: TTS 复用后端 edge-tts + 豆包，角色声音/失败回退/自动播放/音频 URL/流式结束事件，未重建冲突调度系统
- [ ] G4: 记忆系统（长期/情景/初始/世界书/时间线/搜索/图谱摘要/来源/CRUD/按角色筛选）使用真实 SurrealDB 数据，非静态图

## H. 主动消息与通知（stage.md 验收 19-20）

- [ ] H1: 前台主动消息可在 Android 显示
- [ ] H2: 后台消息可通过系统通知呈现
- [ ] H3: 点击通知进入对应角色和会话
- [ ] H4: 相同消息去重
- [ ] H5: 角色级通知设置 + 通知权限 + 通知隐私
- [ ] H6: 应用重启后未读恢复

## I. Runtime 管理与错误（stage.md 验收 21-22）

- [ ] I1: Runtime 管理页面显示真实状态（模式/RootFS 版本/Runtime 状态/Go 后端/Qdrant/SurrealDB/端口/运行时长/内存/数据占用/日志/最后错误）
- [ ] I2: Runtime 页面支持启动/停止/重启/修复/更新/导出诊断/清理/备份/恢复
- [ ] I3: 危险操作二次确认，清理 Runtime 不删用户数据
- [ ] I4: 服务启动失败时显示真实错误原因（非通用错误）
- [ ] I5: 统一错误类型 19 种（RootFS 未安装/安装失败/Runtime 启动失败/Qdrant/SurrealDB/Go 后端/端口冲突/二进制不兼容/数据目录无权限/服务超时/Token 失效/网络不可用/远程不可达/流式断开/上传失败/音频失败/迁移失败/Runtime 被杀/未知）
- [ ] I6: 错误页提供重试/诊断/日志导出，不泄敏，单点失败不崩应用

## J. 安全与隐私（stage.md 验收 30-31）

- [ ] J1: 本地后端仅监听 localhost
- [ ] J2: 本地 API 使用随机鉴权令牌
- [ ] J3: Token 使用 Android Keystore
- [ ] J4: API Key 不写入源码
- [ ] J5: Release 日志脱敏（不含完整 Token/密码/完整聊天内容）
- [ ] J6: 文件路径校验 + 上传类型校验 + Deep Link 校验 + Bridge 请求权限校验
- [ ] J7: RootFS 更新包 + 二进制 Hash 校验
- [ ] J8: Linux 后端不绕过 Android 权限系统，不开放任意未授权命令执行接口

## K. 性能与运行策略（stage.md 第二十二节）

- [ ] K1: 控制 RootFS 体积 / APK 体积 / Qdrant 内存 / SurrealDB 内存 / Go 后端内存 / 后台耗电 / 日志体积 / 数据目录体积 / 图片音频缓存 / 冷启动 / Runtime 启动时间
- [ ] K2: 实现 AlwaysOn / OnDemand / RemoteOnly 三种策略
- [ ] K3: Foreground Service 显示符合 Android 规范的常驻通知
- [ ] K4: 不使用高频轮询维持假活跃
- [ ] K5: 默认策略根据实测决定，不盲目 AlwaysOn

## L. Native Bridge（stage.md 第十一节）

- [ ] L1: 实现 `NativeCapabilityBridge`/`CapabilityRegistry`/`PermissionBroker`/`NativeActionRequest`/`NativeActionResult`
- [ ] L2: 第一阶段 Bridge 提供文件/图片/相机/麦克风/音频/通知/剪贴板/分享/应用目录/主题/网络/电池/前后台
- [ ] L3: 预留 Accessibility/MediaProjection/悬浮窗/Shizuku/Root/屏幕理解/手势/Computer Use 接口
- [ ] L4: Linux 后端调用 Android 能力必须通过 Bridge + PermissionBroker，第一阶段不得允许未授权操作

## M. 首次启动引导（stage.md 第十三节）

- [ ] M1: 引导流程 9 步（欢迎/模式选择/Runtime 安装或远程配置/环境检查/登录/模型配置/角色设置/初始记忆/完成）
- [ ] M2: 本地模式展示真实进度（RootFS/SurrealDB/Qdrant/Go 后端/健康检查），非固定时间进度条
- [ ] M3: 支持中断恢复 + 返回修改 + 不重复安装 + 已存在配置跳过 + 失败显示真实原因 + 不自动跳过需用户输入步骤

## N. UI/UX 设计（stage.md 第十二节）

- [ ] N1: 五大导航（首页/对话/角色/能力/设置）
- [ ] N2: 深色优先 + 低饱和 + 克制毛玻璃 + 清晰层级 + 少量高质量动效
- [ ] N3: 禁止大面积蓝紫渐变/默认 AI 紫/大量功能卡片/大量发光边框/大量阴影/全胶囊/首页工具宫格/复制桌面 Web 排版/无意义粒子/复杂动画掩盖加载

## O. 测试与构建真实执行（stage.md 验收 32-35）

- [ ] O1: 单元测试覆盖（Runtime 状态机/启动停止顺序/进程退出/健康检查/Endpoint 切换/Token 保存/API 错误映射/流式事件解析/消息去重/角色切换/草稿/Runtime 版本迁移/数据目录策略）
- [ ] O2: 集成测试覆盖（Android → Linux → SurrealDB → Qdrant → Go 后端 → 健康 → 角色 → 聊天 → 发消息 → 流式回复）
- [ ] O3: UI 测试覆盖 14 项（首次启动/选本地/安装 Runtime/启动服务/登录/首页/切角色/发消息/流式/记忆/Runtime 状态/停止重启/切远程/深色亮色）
- [ ] O4: 真机验证 22 项（Android 版本/CPU/RootFS 解压/PRoot/Qdrant/SurrealDB/Go 后端/SQLite/网络/SSE/WebSocket/音频/图片/后台/前台服务/系统杀进程/重启恢复/低电量/网络切换/屏幕旋转/字体缩放/深色），真机不可用时明确记录外部阻塞
- [ ] O5: 记录所有构建命令/退出码/错误/修复
- [ ] O6: 记录 APK 路径 / Go ARM64 路径 / RootFS 包路径 / Qdrant ARM64 路径 / SurrealDB ARM64 路径

## P. 文档完整（stage.md 验收 37）

- [x] P1: `docs/android/01-current-system-audit.md`
- [x] P2: `docs/android/02-capability-migration-matrix.md`
- [x] P3: `docs/android/03-runtime-dependency-audit.md`
- [ ] P4: `docs/android/04-android-architecture.md`
- [ ] P5: `docs/android/05-linux-runtime-design.md`
- [ ] P6: `docs/android/06-process-lifecycle.md`
- [ ] P7: `docs/android/07-api-mapping.md`
- [ ] P8: `docs/android/08-ui-design-system.md`
- [ ] P9: `docs/android/09-build-and-run.md`
- [ ] P10: `docs/android/10-testing-report.md`
- [ ] P11: `docs/android/11-migration-report.md`
- [ ] P12: `docs/android/12-known-limitations.md`
- [ ] P13: `docs/android/13-next-stage-plan.md`
- [ ] P14: `docs/android/third-party-licenses.md`

## Q. 不破坏现有链路（stage.md 验收 36, 第二十六节）

- [ ] Q1: Windows Electron 端可启动
- [ ] Q2: Web 端可连接
- [ ] Q3: 远程部署可运行
- [ ] Q4: 现有 SQLite 数据可迁移
- [ ] Q5: 现有 Qdrant 数据不被破坏
- [ ] Q6: 现有 SurrealDB 数据不被破坏
- [ ] Q7: API 与流式协议尽量兼容，禁止为 Android 删除桌面逻辑

## R. 真实性保证（stage.md 验收 28-29, 4.3 节）

- [ ] R1: 不存在核心 Mock 数据通过最终验收
- [ ] R2: 不存在假的功能开关
- [ ] R3: 不使用固定 JSON 冒充后端数据
- [ ] R4: 不使用延时动画冒充 AI 回复
- [ ] R5: 不用本地写死角色冒充角色接口
- [ ] R6: 不用静态页面冒充记忆系统
- [ ] R7: 后端启动失败时不显示运行成功
- [ ] R8: Qdrant/SurrealDB 未启动时不静默忽略
- [ ] R9: Android 不只打开 Web 页面

## S. 最终报告（stage.md 验收 38, 第三十二节 20 项）

- [ ] S1: Android 客户端总体完成情况
- [ ] S2: 当前采用的内嵌 Linux 技术
- [ ] S3: Go 后端 ARM64 构建情况
- [ ] S4: SQLite 运行情况
- [ ] S5: Qdrant ARM64 运行情况
- [ ] S6: SurrealDB ARM64 运行情况
- [ ] S7: 本地模式运行结果
- [ ] S8: 远程模式运行结果
- [ ] S9: 已迁移功能
- [ ] S10: Android 架构
- [ ] S11: UI 设计说明
- [ ] S12: 修改过的 Go 后端文件
- [ ] S13: 新增的 Android 文件
- [ ] S14: 所有实际执行的测试命令
- [ ] S15: 所有实际构建结果
- [ ] S16: APK 实际路径
- [ ] S17: 真机验证结果
- [ ] S18: 未完成项
- [ ] S19: 未完成原因
- [ ] S20: 当前是否达到第一阶段验收标准

## T. 项目规则合规（AGENTS.md）

- [ ] T1: 未使用 cmd/PowerShell 批量替换（必要时用 PowerShell 7）
- [ ] T2: 代码中无注释（项目规则）
- [ ] T3: 未修改编译后产物（仅在源码修改）
- [ ] T4: 端口避开 3000（使用 18899/19178/18000）
- [ ] T5: 未拉取 git
- [ ] T6: 未修改项目启动端口
- [ ] T7: 中文回复
