/goal

你现在是 Amitia 项目的首席 Android 架构师、移动端工程负责人、Linux Runtime 工程师和 UI/UX 设计师。

你的任务是在当前 Amitia 仓库中，直接构建一个可编译、可安装、可运行的 Android 客户端。

本任务不是只输出设计方案、技术文档或页面原型。你必须实际修改代码、创建工程、迁移功能、构建 APK、运行测试，并持续修复问题，直到达到本目标定义的验收标准。

除非遇到仓库不可读取、必要外部文件完全无法获取、签名或密钥缺失等绝对外部阻塞，否则不要停下来询问用户，也不要等待用户逐步确认。
一、产品目标
======

为 Amitia 构建正式的 Android 原生客户端。

第一阶段的核心目标是：

1. 将 Amitia 当前已经真实实现的用户端能力迁移到 Android。

2. Android 使用原生 Kotlin 和 Jetpack Compose。

3. 将现有 Amitia Go 后端运行在 Android 应用内嵌的 Linux 用户空间中。

4. 复用现有后端业务逻辑，不重写一套 Kotlin 业务后端。

5. Android 原生层只负责 UI、Android 系统能力和 Linux Runtime 管理。

6. SQLite、Qdrant、SurrealDB 和 Amitia Go 后端均由内嵌 Linux Runtime 管理。

7. 为后续发展为类似 Operit 的本地 AI Agent、Computer Use、MCP、Skill、插件和本地工具平台预留正确架构。

8. 第一阶段优先保证当前 Amitia 功能迁移成功，不在本阶段无限扩展尚未存在的未来功能。

最终运行结构应为：
    Android Kotlin / Jetpack Compose UI
                    │
                    │ localhost HTTP / WebSocket / SSE
                    │ Unix Domain Socket / Native Bridge
                    ▼
    Android Runtime Manager
                    │
                    ▼
    内嵌 Linux 用户空间
    ├── Amitia Go Backend
    ├── SQLite 数据文件
    ├── Qdrant Linux ARM64
    ├── SurrealDB Linux ARM64
    ├── Python Runtime
    ├── Node.js Runtime
    ├── Shell
    └── 后续 MCP / Skill / AmitiaX Runtime

Android 端不得重新实现角色、记忆、聊天、提示词、模型调度等核心业务逻辑。
二、已确认的 Amitia 后端依赖
==================

不要自行假设 Amitia 使用 MySQL 或 Redis。

当前已确认：

* 核心业务数据库使用 SQLite。

* 当前没有 Redis 依赖。

* 向量数据库使用独立 Qdrant 可执行程序。

* 图数据库使用 SurrealDB。

* 核心后端使用 Go。

* 当前 Windows 环境中的 Qdrant 可能是 `.exe`。

* Android 内嵌 Linux 中必须使用 Linux ARM64 版本。

* SurrealDB 同样必须使用 Linux ARM64 版本。

* Amitia 是单用户、多角色系统。

* Android 客户端必须连接现有角色、聊天、记忆、模型、主动消息和渠道系统。

在实际执行前，必须通过扫描代码重新确认这些事实，并记录真实路径、启动参数、端口和数据目录。

如果扫描结果与以上信息存在差异，以代码真实实现为准，但必须在报告中明确说明差异。
三、核心技术决策
========

3.1 Android 技术栈
---------------

必须使用：

* Kotlin

* Jetpack Compose

* Gradle Kotlin DSL

* Material 3 基础组件

* Navigation Compose

* ViewModel

* Kotlin Coroutines

* StateFlow

* Hilt

* Retrofit

* OkHttp

* Kotlinx Serialization

* Room

* DataStore

* Android Keystore

* Coil

* Media3

* WorkManager

* Foreground Service

不得采用：

* Flutter

* React Native

* WebView 套壳

* 将现有 Web 页面直接嵌入 Android

* 将整个客户端写入单个 Activity

* 在 Compose 页面中直接请求后端

* 再写一套 Kotlin 业务后端

3.2 内嵌 Linux 技术路线
-----------------

在 Android 应用中建立无 Root 的 Linux 用户空间。

优先评估并选择适合当前项目的成熟方案，例如：

* PRoot 用户空间

* proot-rs

* libproot

* 自有 Linux Runtime 管理层

* 必要时结合 NDK/JNI

Linux RootFS 优先采用精简的 ARM64 Ubuntu 或 Debian 用户空间。

需要满足：

* 不要求 Root。

* 不依赖用户单独安装 Termux。

* Linux RootFS 由 Amitia Android 应用管理。

* 首次启动时可以自动解压和初始化。

* 后续启动不重复完整安装。

* Linux 数据与 Android 应用私有目录隔离。

* 支持 ARM64 Android 设备。

* 为未来 Python、Node.js、Shell、MCP 和 Skill 保留能力。

* 不能只做命令行演示，必须真实启动 Amitia 后端。

如果 PRoot 性能或兼容性存在问题，允许使用更合适的实现，但必须保留“应用内无 Root Linux 用户空间”的产品目标。
3.3 本地和远程双运行模式
--------------

Android 客户端需要支持两种运行模式。

### 本地模式

    Android UI
        ↓
    127.0.0.1 / Unix Socket
        ↓
    内嵌 Linux 中的 Amitia Go 后端

本地模式由 Android 应用自动管理：

* Linux RootFS

* Qdrant

* SurrealDB

* Amitia Go 后端

* 数据目录

* 启动顺序

* 健康检查

* 崩溃恢复

* 日志

* 停止和重启

### 远程模式

    Android UI
        ↓
    HTTPS / WebSocket / SSE
        ↓
    远程 Amitia 服务端

远程模式允许用户填写已有 Amitia 服务地址。

本地模式和远程模式必须共用同一套 Android Repository、领域模型和 UI，不得维护两套页面。

连接目标只能通过统一的 RuntimeEndpoint 或 ServerEndpoint 进行切换。
四、严格执行原则
========

4.1 先扫描真实代码
-----------

开始编码前必须完整扫描：

* Go 后端目录

* Web 前端目录

* Electron 目录

* SQLite 初始化和迁移代码

* Qdrant 启动逻辑

* SurrealDB 启动逻辑

* 配置文件

* 环境变量

* API 路由

* WebSocket

* SSE

* 流式聊天协议

* 文件上传

* 音频处理

* TTS

* 角色系统

* 记忆系统

* 模型系统

* 主动消息系统

* 渠道系统

* 首次启动流程

* 现有后端启动入口

* Windows 侧车或子进程管理逻辑

不得只根据 README 判断实现状态。

必须检查：
    前端页面
    → 前端请求层
    → API 路由
    → Handler
    → Service
    → Repository
    → 数据库或外部服务

只有完整链路存在，才能标记为“已真实实现”。
4.2 不得只输出方案
-----------

审计完成后必须继续创建和修改代码。

禁止在输出以下内容后停止：

* 架构建议

* 功能迁移表

* UI 原型

* 接口清单

* 待办事项

* 下一阶段建议

这些文档只是执行过程的一部分，不代表完成。
4.3 不允许伪实现
----------

禁止：

* 使用固定 JSON 冒充后端数据。

* 使用延时动画冒充 AI 回复。

* 用本地写死角色冒充角色接口。

* 用静态页面冒充记忆系统。

* 用假开关冒充功能已接入。

* 后端启动失败时仍显示运行成功。

* Qdrant 或 SurrealDB 未启动时静默忽略。

* Android 只打开 Web 页面。

* 使用 Mock 数据通过最终验收。

开发调试可以有 Mock，但正式主路径必须使用真实 Amitia 后端。
五、第一阶段范围
========

第一阶段重点是将 Amitia 当前已有能力迁移到 Android，同时完成本地 Linux 后端运行。

本阶段必须完成：

1. Android 原生工程。

2. 内嵌 Linux Runtime。

3. Amitia Go 后端 Linux ARM64 运行。

4. SQLite 数据持久化。

5. Qdrant Linux ARM64 运行。

6. SurrealDB Linux ARM64 运行。

7. Android 与本地后端通信。

8. 远程后端模式。

9. 首次启动引导。

10. 登录或现有单用户初始化流程。

11. 多角色系统。

12. 聊天系统。

13. 流式回复。

14. 图片消息。

15. 语音消息。

16. TTS。

17. 记忆浏览和管理。

18. 主动消息。

19. Android 通知。

20. 模型配置。

21. 渠道状态。

22. 设置。

23. 本地缓存。

24. 构建 APK。

25. 测试和验收。

以下未来能力只预留架构，本阶段不要求全部实现：

* 完整 Computer Use

* 无障碍自动操作

* 屏幕理解

* Root 控制

* Shizuku

* ADB 控制

* 完整本地模型推理

* 完整终端 UI

* MCP 市场

* Skill 市场

* AmitiaX 扩展市场

* 桌宠

* 全局悬浮助手

* 自动编程工作区

但本阶段的目录和接口设计不能阻碍这些功能未来加入。
六、第一阶段文档审计
==========

首先生成：
    docs/android/01-current-system-audit.md

内容必须包括：

* 当前后端入口。

* 后端启动命令。

* Go 版本。

* CGO 使用情况。

* SQLite 驱动。

* SQLite 文件路径。

* 数据库迁移方式。

* Qdrant 当前版本。

* Qdrant 当前可执行文件路径。

* Qdrant 启动参数。

* Qdrant 端口。

* Qdrant 数据目录。

* SurrealDB 当前版本。

* SurrealDB 启动参数。

* SurrealDB 端口。

* SurrealDB 数据目录。

* 后端依赖的环境变量。

* 后端依赖的文件资源。

* 后端是否使用绝对 Windows 路径。

* 后端是否调用 Windows 专属命令。

* 后端是否存在平台相关代码。

* Web 和 Electron 如何启动后端。

* 所有真实 API。

* 所有实时协议。

* 所有当前用户可用功能。

生成：
    docs/android/02-capability-migration-matrix.md

使用以下表格：

| 功能  | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口  | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞  |
| --- | ------ | ----------- | ---- | ----------- | --- | ---- | ------ | ------ | --- |

状态只能使用：

* 已真实实现

* 部分实现

* 仅 UI

* 后端缺失

* 已废弃

* 无法确认

生成：
    docs/android/03-runtime-dependency-audit.md

专门记录：

* Go 后端 Linux ARM64 兼容性。

* SQLite Linux ARM64 兼容性。

* Qdrant Linux ARM64 获取或编译方式。

* SurrealDB Linux ARM64 获取或编译方式。

* RootFS 体积。

* 后端及数据库预计内存。

* 端口冲突风险。

* Android 后台限制。

* 文件权限问题。

* `/proc`、`/dev`、网络和 DNS 需求。

* PRoot 环境兼容性。

* 进程退出行为。

* 应用升级时的数据迁移策略。

文档生成后立即继续实现。
七、Android 工程结构
==============

如果仓库中尚无 Android 工程，在根目录创建：
    android/

建议结构：
    android/
    ├── app
    ├── core/
    │   ├── common
    │   ├── model
    │   ├── network
    │   ├── database
    │   ├── datastore
    │   ├── security
    │   ├── designsystem
    │   ├── media
    │   └── logging
    ├── runtime/
    │   ├── api
    │   ├── manager
    │   ├── linux
    │   ├── process
    │   ├── bootstrap
    │   ├── health
    │   └── bridge
    ├── platform/
    │   ├── notification
    │   ├── files
    │   ├── audio
    │   ├── permissions
    │   └── foreground
    ├── feature/
    │   ├── startup
    │   ├── onboarding
    │   ├── auth
    │   ├── home
    │   ├── chat
    │   ├── character
    │   ├── memory
    │   ├── models
    │   ├── channels
    │   ├── runtime
    │   └── settings
    └── native/
        └── proot

可以根据 Gradle 构建稳定性适当合并模块，但必须保持包结构和职责清晰。

不要创建大量没有内容的空模块。
八、内嵌 Linux Runtime 实现
=====================

8.1 RootFS 管理
-------------

实现 LinuxRootfsManager：

* 检测 RootFS 是否已安装。

* 检测 RootFS 版本。

* 首次启动解压。

* 显示真实解压进度。

* 支持校验文件完整性。

* 支持安装失败重试。

* 支持升级。

* 升级时不得删除用户数据。

* RootFS 和用户数据分离。

* 可清理 Runtime，但需要二次确认。

* 不允许主线程解压大型文件。

目录建议：
    files/
    ├── runtime/
    │   ├── rootfs/
    │   ├── bin/
    │   ├── logs/
    │   ├── tmp/
    │   └── versions/
    └── amitia-data/
        ├── sqlite/
        ├── qdrant/
        ├── surrealdb/
        ├── uploads/
        ├── models/
        ├── extensions/
        └── backups/

用户数据不得放入可被 Runtime 更新覆盖的 RootFS 目录。
8.2 Linux 进程管理器
---------------

实现统一的 LinuxProcessManager。

必须支持：

* 启动命令。

* 环境变量。

* 工作目录。

* 标准输出。

* 标准错误。

* 退出码。

* 超时。

* 终止。

* 强制终止。

* 进程状态。

* 崩溃次数。

* 最后启动时间。

* 最后退出原因。

* 日志滚动。

* 重启策略。

* 防止重复启动。

* 应用退出时的资源释放。

禁止通过固定延时猜测服务是否启动成功。

必须通过端口、HTTP 健康检查或进程状态确认。
8.3 启动顺序
--------

本地 Runtime 启动顺序：
    1. 检查 RootFS
    2. 检查 Runtime 文件
    3. 检查数据目录
    4. 检查配置
    5. 启动 SurrealDB
    6. 等待 SurrealDB 健康
    7. 启动 Qdrant
    8. 等待 Qdrant 健康
    9. 启动 Amitia Go Backend
    10. 等待 Amitia Backend 健康
    11. Android Repository 建立连接
    12. 显示本地服务已运行

停止顺序：
    1. 停止接受新请求
    2. 停止 Amitia Go Backend
    3. 停止 Qdrant
    4. 停止 SurrealDB
    5. 刷新日志和状态

只有服务真实健康后，UI 才能显示运行成功。
8.4 Runtime 状态机
---------------

至少实现：
    NotInstalled
    Installing
    Installed
    Starting
    Running
    Degraded
    Stopping
    Stopped
    Failed
    Updating

每个状态必须包含：

* 当前阶段。

* 进度。

* 可读信息。

* 错误原因。

* 是否可重试。

* 是否需要用户操作。

不能只返回一个 Boolean。
九、后端移植
======

9.1 Go 后端
---------

为 Android 本地模式提供 Linux ARM64 构建。

目标：
    GOOS=linux
    GOARCH=arm64

需要审计：

* 是否启用 CGO。

* SQLite 驱动是否需要 CGO。

* 是否存在 Windows 专用路径。

* 是否调用 PowerShell、cmd.exe 或 `.exe`。

* 是否使用 Windows 服务。

* 是否使用系统托盘。

* 是否依赖 Electron 环境变量。

* 是否使用绝对盘符路径。

* 是否依赖桌面浏览器。

* 是否使用只能在 Windows 上工作的库。

如有平台相关逻辑，应抽象成：
    DesktopRuntime
    AndroidEmbeddedRuntime
    ServerRuntime

禁止复制一份完整后端形成 Android 专属分叉。

后端修改必须保持：

* Windows Electron 可用。

* Web 部署可用。

* Android Linux ARM64 可用。

9.2 SQLite
----------

继续使用现有 SQLite。

必须处理：

* Android 应用数据目录。

* Linux Runtime 路径映射。

* 数据库迁移。

* 备份。

* 恢复。

* 应用升级。

* 并发访问。

* 文件锁。

* 异常退出。

* 数据损坏检测。

Android 原生 Room 只用于 Android UI 缓存，不能直接修改 Amitia 核心 SQLite 数据库。

Amitia 核心数据只能由 Go 后端访问。
9.3 Qdrant
----------

当前 Windows `.exe` 不能放入 Android Linux Runtime。

必须：

1. 确认当前 Qdrant 版本。

2. 获取对应 Linux ARM64 构建。

3. 如果没有可直接使用的构建，评估从源代码构建 ARM64。

4. 验证能否在所选 PRoot 环境中运行。

5. 验证存储路径。

6. 验证端口。

7. 验证健康检查。

8. 验证重启后数据仍存在。

9. 验证 Go 后端可以正常连接。

10. 验证 ARM64 Android 真机运行。

不得用假进程替代 Qdrant。

如果 Qdrant 因内核、指令集或 PRoot 限制无法运行，必须：

* 记录真实错误。

* 尝试兼容构建参数。

* 尝试降级到兼容版本。

* 评估直接原生运行而非 PRoot。

* 在不破坏现有接口的前提下提供可替换 VectorStore Adapter。

不能直接删除向量记忆功能。
9.4 SurrealDB
-------------

必须为 SurrealDB 准备 Linux ARM64 版本。

验证：

* 二进制可执行。

* 当前存储引擎可用。

* 数据目录持久化。

* 端口可访问。

* 认证配置正确。

* Go 后端可连接。

* 应用重启后数据存在。

* 异常退出后可恢复。

如当前 SurrealDB 只用于特定图数据库能力，服务不可用时可以进入 Degraded 状态，但不能谎报全部能力正常。
十、Android 与后端通信
===============

建立统一：
    AmitiaApiClient
    RuntimeEndpointProvider
    ConnectionManager
    SessionManager

本地模式默认连接：
    http://127.0.0.1:<动态或固定受控端口>

远程模式连接用户配置地址。

需要支持：

* REST。

* SSE。

* WebSocket。

* 流式 HTTP。

* 文件上传。

* 图片。

* 音频。

* 超时。

* 重连。

* Token。

* 错误映射。

* 连接状态。

不得在页面中硬编码 `127.0.0.1` 或远程地址。

如果本地通信适合 Unix Domain Socket，可以用于 Runtime 管理和敏感控制；业务 API 可以继续使用 localhost。

本地监听端口必须：

* 仅监听 `127.0.0.1`。

* 不监听 `0.0.0.0`。

* 避免局域网暴露。

* 必要时使用本地随机鉴权令牌。

* 防止其他应用直接调用敏感本地 API。

十一、Android Native Bridge
========================

Android 原生系统能力不能放入 Linux 中直接操作。

建立统一：
    NativeCapabilityBridge

第一阶段至少提供：

* 文件选择。

* 图片选择。

* 相机调用。

* 麦克风录音。

* 音频播放。

* 系统通知。

* 剪贴板。

* 分享。

* 应用目录。

* 系统主题。

* 网络状态。

* 电池状态。

* 前后台状态。

后续预留：

* AccessibilityService。

* MediaProjection。

* 悬浮窗。

* Shizuku。

* Root。

* 屏幕理解。

* 应用启动。

* 手势模拟。

* Computer Use。

Linux 中的 Go 后端未来通过受控 Bridge 调用 Android 能力，但第一阶段不得允许后端任意执行未经授权的 Android 系统操作。

建立：
    CapabilityRegistry
    PermissionBroker
    NativeActionRequest
    NativeActionResult
十二、UI/UX 设计
===========

由你自主完成具体布局，但必须遵守以下产品边界。

Amitia 的 Android 端定位是：
    AI 陪伴角色
    +
    个人智能体
    +
    本地 Agent Runtime

不是普通 AI 聊天工具，也不是 Operit 换皮。
12.1 设计关键词
----------

* 克制。

* 安静。

* 沉浸。

* 有陪伴感。

* 低饱和。

* 深色优先。

* 轻微毛玻璃。

* 清晰层级。

* 移动端原生。

* 少量高质量动效。

禁止：

* 大面积蓝紫渐变。

* 默认 AI 紫色。

* 大量功能卡片。

* 大量发光边框。

* 大量阴影。

* 所有元素都做成胶囊。

* 首页做成工具宫格。

* 直接复制桌面 Web 排版。

* 使用无意义的粒子背景。

* 用复杂动画掩盖加载过程。

12.2 页面结构
---------

建议主要导航：
    首页
    对话
    角色
    能力
    设置

其中：

### 首页

突出：

* 当前角色。

* 角色状态。

* 最近对话。

* 主动消息。

* 继续对话。

* Runtime 运行状态。

* 必要异常。

不要堆叠大量设置入口。

### 对话

包括：

* 会话列表。

* 当前角色聊天。

* 图片和语音。

* 流式回复。

* 历史记录。

* 主动消息。

### 角色

包括：

* 角色列表。

* 当前角色。

* 角色详情。

* 角色创建。

* 角色编辑。

* 角色记忆。

* 角色语音。

* 角色模型。

### 能力

第一阶段显示：

* 本地 Runtime。

* 模型。

* 记忆。

* 渠道。

* 已存在的 Skill、MCP 或扩展能力。

未来用于：

* MCP。

* Skill。

* AmitiaX。

* Computer Use。

* 终端。

* 工作区。

### 设置

包括：

* 本地或远程模式。

* 本地 Runtime 管理。

* 远程服务地址。

* 主题。

* 通知。

* 语音。

* 缓存。

* 数据备份。

* 日志。

* 关于。

* 版本。

* 退出登录。

十三、首次启动引导
=========

首次启动必须按真实状态展示。

建议流程：
    1. 欢迎
    2. 选择本地模式或远程模式
    3. 本地 Runtime 安装，或远程地址配置
    4. 环境检查
    5. 登录或单用户初始化
    6. 模型配置
    7. 角色设置
    8. 初始记忆
    9. 完成

本地模式中必须展示真实进度：

* RootFS 安装。

* SurrealDB 初始化。

* Qdrant 初始化。

* Go 后端初始化。

* 健康检查。

不得用固定时间进度条。

引导必须：

* 支持中断恢复。

* 支持返回修改。

* 不重复安装。

* 已存在配置时跳过对应步骤。

* 安装失败时显示真实原因。

* 不自动跳过需要用户输入的步骤。

十四、角色系统
=======

Amitia 是单用户、多角色。

必须实现：

* 角色列表。

* 当前角色。

* 切换角色。

* 角色头像。

* 角色名称。

* 角色身份。

* 性格。

* 提示词摘要。

* 角色状态。

* 创建。

* 编辑。

* 删除确认。

* 独立聊天记录。

* 独立记忆。

* 独立语音。

* 独立模型配置。

* 独立主动消息状态。

切换角色时不得混用：

* 消息。

* 草稿。

* 记忆。

* 情绪。

* TTS。

* 生成状态。

* 未读数。

所有业务数据来自 Go 后端。
十五、聊天系统
=======

聊天为最高优先级。

必须实现：

* 会话历史。

* 分页。

* 文本消息。

* 流式回复。

* 用户消息状态。

* AI 生成状态。

* 失败重试。

* 复制。

* 删除，如果后端支持。

* 图片消息。

* 语音消息。

* 系统消息。

* 主动消息。

* 日期分组。

* 草稿保存。

* 键盘适配。

* 返回位置恢复。

* 消息去重。

* 页面重建后不中断状态。

必须从现有 Web 前端和 Go 后端确认真实流式协议。

不得自行猜测：

* SSE 事件名称。

* WebSocket 消息结构。

* `message_start`。

* `delta`。

* `message_end`。

* `error`。

* `usage`。

* `tool_call`。

* TTS 事件。

* delivery intent。

如现有前后端协议不一致，应以最小修改方式修复，并保持 Web 和 Electron 兼容。
十六、图片和语音
========

16.1 图片
-------

如后端支持，必须实现：

* 相册。

* 相机。

* 预览。

* 压缩。

* 上传。

* 进度。

* 失败重试。

* 移除。

* 多图限制。

* 文件类型校验。

* 图片上下文。

只申请 Android 当前规范要求的最小权限。
16.2 语音消息
---------

如后端支持，必须实现：

* 麦克风权限。

* 录音。

* 取消录音。

* 时长。

* 音量状态。

* 上传。

* 播放。

* 暂停。

* 播放进度。

* 音频焦点。

* 资源释放。

* 错误恢复。

16.3 TTS
--------

复用现有 Amitia TTS 后端。

检查并接入：

* Edge-TTS。

* 豆包 TTS。

* 角色声音。

* 失败回退。

* 自动播放设置。

* 音频 URL。

* 流式回复结束事件。

不得在 Android 再创建一套与服务端冲突的 TTS 调度系统。
十七、记忆系统
=======

根据现有真实能力迁移：

* 长期记忆。

* 情景记忆。

* 初始记忆。

* 世界书。

* 记忆时间线。

* 记忆搜索。

* 图谱摘要。

* 记忆来源。

* 创建。

* 编辑。

* 删除。

* 按角色筛选。

复杂图谱在 Android 首版可以用列表和关系详情呈现，不要求直接复刻桌面大图。

但必须使用真实 SurrealDB 数据，不能用静态图代替。
十八、主动消息和通知
==========

实现：

* 前台主动消息。

* 后台通知。

* 点击通知进入对应角色和会话。

* 相同消息去重。

* 角色级通知设置。

* 通知权限。

* 通知隐私。

* 应用重启后的未读恢复。

本地 Runtime 可以通过 Android Bridge 触发系统通知。

如果 Android 后台限制导致 Linux 后端无法永久运行，需要：

1. 使用 Foreground Service 管理 Runtime。

2. 显示符合 Android 规范的常驻通知。

3. 提供节能模式。

4. 提供“始终运行”和“按需启动”策略。

5. 记录系统杀进程后的恢复。

6. 不使用高频轮询维持假活跃。

十九、本地 Runtime 管理页面
==================

必须提供独立 Runtime 管理页面。

显示：

* 模式。

* RootFS 版本。

* Runtime 状态。

* Go 后端状态。

* Qdrant 状态。

* SurrealDB 状态。

* 端口。

* 运行时长。

* 内存占用，如可获取。

* 数据占用。

* 日志。

* 最后错误。

* 启动。

* 停止。

* 重启。

* 修复。

* 更新。

* 导出诊断。

* 清理 Runtime。

* 备份数据。

* 恢复数据。

危险操作必须二次确认。

“清理 Runtime”不得默认删除用户数据。
二十、数据和缓存
========

Android 使用 Room 保存 UI 缓存：

* 最近角色。

* 最近会话。

* 最近消息。

* 消息发送状态。

* 草稿。

* Runtime 状态快照。

* 主动消息。

* 待重试任务。

DataStore 保存：

* 运行模式。

* 远程地址。

* 当前角色。

* 主题。

* 通知偏好。

* 语音偏好。

* 引导状态。

* Runtime 版本。

Android Keystore 保存：

* Token。

* 本地通信密钥。

* 敏感凭据。

Room 不能成为 Amitia 核心业务数据库。

核心业务数据仍由内嵌 Go 后端和 SQLite 管理。
二十一、安全
======

必须做到：

* 本地后端仅监听 localhost。

* 本地 API 使用随机鉴权令牌或等效保护。

* Token 使用 Keystore。

* API Key 不写入源码。

* Release 日志脱敏。

* 不记录密码。

* 不记录完整 Token。

* 不记录完整聊天隐私数据。

* 文件路径校验。

* 上传类型校验。

* Deep Link 校验。

* Bridge 请求权限校验。

* Linux 后端不能绕过 Android 权限系统。

* 不开放任意未授权命令执行接口。

* RootFS 更新包需要校验。

* 二进制需要校验 Hash。

二十二、性能和资源
=========

Android 手机资源有限。

必须控制：

* RootFS 体积。

* APK 或首次下载体积。

* Qdrant 内存。

* SurrealDB 内存。

* Go 后端内存。

* 后台耗电。

* 日志体积。

* 数据目录体积。

* 图片缓存。

* 音频缓存。

* 冷启动。

* Runtime 启动时间。

实现运行策略：
    AlwaysOn
    OnDemand
    RemoteOnly

### AlwaysOn

前台服务保持本地 Runtime 运行。

### OnDemand

用户打开应用或需要主动任务时启动。

### RemoteOnly

不启动本地 Linux，连接远程服务。

默认策略应根据实际资源测试决定，不能盲目默认 AlwaysOn。
二十三、错误处理
========

统一错误类型：

* RootFS 未安装。

* RootFS 安装失败。

* Runtime 启动失败。

* Qdrant 启动失败。

* SurrealDB 启动失败。

* Go 后端启动失败。

* 端口冲突。

* 二进制不兼容。

* 数据目录无权限。

* 服务超时。

* Token 失效。

* 网络不可用。

* 远程服务不可达。

* 流式连接断开。

* 上传失败。

* 音频失败。

* 数据库迁移失败。

* Runtime 被系统杀死。

* 未知错误。

错误页面必须：

* 展示可理解原因。

* 提供重试。

* 提供诊断信息。

* 提供日志导出。

* 不泄露敏感信息。

* 不因单个子服务失败导致整个应用崩溃。

二十四、测试
======

24.1 单元测试
---------

至少覆盖：

* Runtime 状态机。

* 启动顺序。

* 停止顺序。

* 进程退出处理。

* 健康检查。

* Endpoint 切换。

* Token 保存。

* API 错误映射。

* 流式事件解析。

* 消息去重。

* 角色切换。

* 草稿。

* Runtime 版本迁移。

* 数据目录策略。

24.2 集成测试
---------

至少覆盖：
    Android
    → 启动 Linux
    → 启动 SurrealDB
    → 启动 Qdrant
    → 启动 Go 后端
    → 调用健康接口
    → 获取角色
    → 获取聊天记录
    → 发送消息
    → 接收流式回复
24.3 UI 测试
----------

覆盖：

1. 首次启动。

2. 选择本地模式。

3. 安装 Runtime。

4. 启动本地服务。

5. 登录或初始化。

6. 进入首页。

7. 切换角色。

8. 发送消息。

9. 接收流式回复。

10. 查看记忆。

11. 查看 Runtime 状态。

12. 停止和重启 Runtime。

13. 切换远程模式。

14. 深色和亮色模式。

24.4 真机验收
---------

必须优先在 ARM64 Android 真机验证：

* Android 版本。

* CPU 架构。

* RootFS 解压。

* PRoot 运行。

* Qdrant。

* SurrealDB。

* Go 后端。

* SQLite。

* 网络。

* SSE。

* WebSocket。

* 音频。

* 图片。

* 后台。

* 前台服务。

* 系统杀进程。

* 重启恢复。

* 低电量模式。

* 网络切换。

* 屏幕旋转。

* 字体缩放。

* 深色模式。

仅模拟器成功不能证明 Linux ARM64 Runtime 真正可用。
二十五、构建
======

必须实际运行：
    .\gradlew.bat clean
    .\gradlew.bat test
    .\gradlew.bat lint
    .\gradlew.bat assembleDebug

如果在 Linux 环境：
    ./gradlew clean
    ./gradlew test
    ./gradlew lint
    ./gradlew assembleDebug

同时构建：

* Amitia Go Backend Linux ARM64。

* 必要 Runtime 组件。

* RootFS 分发包或安装包。

* Android Debug APK。

不得只声称理论可编译。

必须记录：

* 命令。

* 退出码。

* 错误。

* 修复。

* 最终 APK 路径。

* Go 后端 ARM64 二进制路径。

* RootFS 包路径。

* Qdrant ARM64 路径。

* SurrealDB ARM64 路径。

二十六、后端兼容要求
==========

任何后端修改必须保持：

* Windows Electron 端可启动。

* Web 端可连接。

* 远程部署可运行。

* 现有 SQLite 数据可迁移。

* 现有 Qdrant 数据不被破坏。

* 现有 SurrealDB 数据不被破坏。

* API 尽量兼容。

* 流式协议尽量兼容。

禁止为了 Android 直接删除桌面逻辑。

需要使用平台抽象，而不是复制整个系统。
二十七、许可证和第三方代码
=============

可以研究 Operit 的产品思路、模块边界和运行方式，但：

* 不直接复制无法确认许可证边界的代码。

* 不复制品牌资源。

* 不复制 UI。

* 不制作 Operit 换皮。

* 所有第三方组件必须记录许可证。

* 新增依赖必须生成依赖许可证清单。

* RootFS、Qdrant、SurrealDB、PROot 等分发方式必须检查许可证要求。

生成：
    docs/android/third-party-licenses.md
二十八、交付文档
========

最终必须生成：
    docs/android/01-current-system-audit.md
    docs/android/02-capability-migration-matrix.md
    docs/android/03-runtime-dependency-audit.md
    docs/android/04-android-architecture.md
    docs/android/05-linux-runtime-design.md
    docs/android/06-process-lifecycle.md
    docs/android/07-api-mapping.md
    docs/android/08-ui-design-system.md
    docs/android/09-build-and-run.md
    docs/android/10-testing-report.md
    docs/android/11-migration-report.md
    docs/android/12-known-limitations.md
    docs/android/13-next-stage-plan.md
    docs/android/third-party-licenses.md

迁移报告必须列出：

* 已迁移功能。

* 部分迁移功能。

* 未迁移功能。

* 未迁移原因。

* Android 新增文件。

* Go 后端修改文件。

* Qdrant 处理方式。

* SurrealDB 处理方式。

* RootFS 处理方式。

* 测试结果。

* 构建结果。

* APK 路径。

* 真机验证结果。

* 当前阻塞。

* 是否达到第一阶段验收。

二十九、执行步骤
========

严格按以下顺序执行，但阶段之间不要等待用户确认：

1. 扫描完整仓库。

2. 定位 Go 后端启动入口。

3. 定位 SQLite。

4. 定位 Qdrant。

5. 定位 SurrealDB。

6. 定位 Web 和 Electron 的后端启动逻辑。

7. 审计 API 和流式协议。

8. 生成现状审计文档。

9. 生成迁移矩阵。

10. 评估 Linux ARM64 兼容性。

11. 确定 RootFS 和 PRoot 技术路线。

12. 创建 Android 工程。

13. 建立 Design System。

14. 建立 Runtime 状态机。

15. 实现 RootFS 安装。

16. 实现 Linux 进程管理。

17. 构建 Go 后端 Linux ARM64。

18. 准备 SurrealDB Linux ARM64。

19. 准备 Qdrant Linux ARM64。

20. 实现启动顺序和健康检查。

21. 实现本地后端连接。

22. 实现远程后端连接。

23. 实现启动和引导。

24. 实现账户和会话。

25. 实现首页。

26. 实现角色系统。

27. 实现聊天历史。

28. 实现流式回复。

29. 实现图片消息。

30. 实现语音消息。

31. 实现 TTS。

32. 实现记忆系统。

33. 实现模型配置。

34. 实现主动消息。

35. 实现 Android 通知。

36. 实现渠道状态。

37. 实现 Runtime 管理页面。

38. 实现设置。

39. 实现 Room 缓存。

40. 实现错误恢复。

41. 实现前台服务。

42. 实现日志和诊断。

43. 完成单元测试。

44. 完成集成测试。

45. 完成 UI 测试。

46. 构建 APK。

47. 真机安装。

48. 验证完整链路。

49. 修复发现的问题。

50. 更新全部文档。

51. 输出最终验收报告。

三十、禁止提前停止
=========

不得因为以下原因停止：

* 工作量较大。

* 页面较多。

* Gradle 首次构建失败。

* Go 交叉编译失败。

* Qdrant ARM64 需要额外处理。

* SurrealDB 启动失败。

* PRoot 出现兼容问题。

* Android 后台限制复杂。

* 需要阅读大量现有代码。

* 某个次要功能暂时阻塞。

遇到问题必须：

1. 定位根因。

2. 尝试修复。

3. 尝试兼容方案。

4. 记录真实错误。

5. 继续完成不受影响的工作。

6. 不得用伪实现掩盖阻塞。

三十一、第一阶段验收标准
============

只有同时满足以下条件，才能将目标标记为完成：

1. Android 原生工程真实存在。

2. 使用 Kotlin 和 Jetpack Compose。

3. Android 工程成功编译。

4. Debug APK 真实生成。

5. 内嵌 Linux RootFS 可以安装。

6. Linux 用户空间可以启动。

7. Amitia Go 后端可以在 Linux ARM64 环境启动。

8. SQLite 数据可以持久化。

9. Qdrant Linux ARM64 可以启动并持久化数据。

10. SurrealDB Linux ARM64 可以启动并持久化数据。

11. Android 可以连接本地 Amitia 后端。

12. Android 可以切换远程 Amitia 后端。

13. 角色数据来自真实 Go 后端。

14. 聊天记录来自真实 Go 后端。

15. 用户消息可以真实发送。

16. AI 回复通过真实流式协议显示。

17. 角色之间数据不会混淆。

18. 记忆来自真实后端。

19. 主动消息可以在 Android 显示。

20. 后台消息可以通过通知呈现。

21. Runtime 页面显示真实状态。

22. 服务启动失败时显示真实错误。

23. App 重启后可以恢复本地 Runtime 状态。

24. App 被系统结束后不会损坏数据。

25. 深色和亮色主题可用。

26. Android 返回手势正常。

27. 键盘不遮挡输入框。

28. 不存在核心 Mock 数据。

29. 不存在假的功能开关。

30. Token 和敏感信息安全存储。

31. Release 日志不暴露敏感数据。

32. 单元测试已实际执行。

33. Lint 已实际执行。

34. Debug 构建已实际执行。

35. APK 已在 ARM64 真机验证，或明确记录真机不可用的外部阻塞。

36. 没有破坏现有 Web 和 Electron 核心链路。

37. 全部要求文档已经生成。

38. 最终报告如实说明所有未完成内容。

三十二、最终回复格式
==========

执行完成后，最终回复必须包含：

1. Android 客户端总体完成情况。

2. 当前采用的内嵌 Linux 技术。

3. Go 后端 ARM64 构建情况。

4. SQLite 运行情况。

5. Qdrant ARM64 运行情况。

6. SurrealDB ARM64 运行情况。

7. 本地模式运行结果。

8. 远程模式运行结果。

9. 已迁移功能。

10. Android 架构。

11. UI 设计说明。

12. 修改过的 Go 后端文件。

13. 新增的 Android 文件。

14. 所有实际执行的测试命令。

15. 所有实际构建结果。

16. APK 实际路径。

17. 真机验证结果。

18. 未完成项。

19. 未完成原因。

20. 当前是否达到第一阶段验收标准。

现在立即开始扫描当前 Amitia 仓库并执行。

不要只输出计划。

不要等待用户确认。

不要重新询问已经提供的信息。

持续实施，直到完成构建、测试和验收，或者遇到确实无法由代码解决的外部阻塞。
