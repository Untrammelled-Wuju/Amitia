# Android 26.2.0-beta.18 本地验收更新说明

## 更新内容

- 手机端渠道工作区支持插件动态页面和标签；渠道入口随已安装并启用的插件显示。
- 单会话渠道直接显示宿主消息，多会话渠道支持会话列表和只读消息详情。
- QQ 个人号手机页面采用紧凑布局，安装阶段与详细状态自动刷新；下载期间显示实际 MB 与百分比，并校验复用缓存包。
- 补齐后端 Schema 自动刷新间隔契约，确保手机收到的刷新配置在编译和传输后仍完整保留。
- QQ 个人号统一插件包提供桌面端、手机端、SnowLuma WebUI 和 noVNC 页面。
- 改善设备归属群聊、会话标题、消息读取和插件服务退出后的运行时清理。

## 适用范围

Android ARM64，本次只做本地构建和模拟器验收。版本保持 26.2.0-beta.18；不上传服务器或 Git。

## 最终本地验收

- APK：`artifacts/android-channel-final-20261029/amitia-26.2.0-beta.18-release-arm64-v8a.apk`，SHA-256 `1882131ce4dfa29cfb01e6f90ec376589dd0cc6983226988eb5e2a96eb9accc1`，源码基线 `d4bbc438d9635320dc2be4b74965337e626de539`。
- Runtime：`artifacts/android-channel-final-20261029/amitia-runtime-1.0.0-android-arm64.zip`，SHA-256 `93479e2f9518526c055e174269c37d565743c6d9cc2a00ec109ec808018e0ada`。
- QQ 个人号插件：`plugin/com.amitia.channel-qq-personal-0.2.66.amitiax`，SHA-256 `0834f5c4f39beda210e1dd35b1e932ca98d745edd7bd47d8a9252fa50a5f4da6`，treeHash `06647238fe7f7f0f5c90679b9efb13dd5e5aa2bd40bb40d442ee17052d0c1af7`。
- 模拟器内 APK 哈希与本地制品一致；`versionCode=24`、`versionName=26.2.0-beta.18`，不存在 Debug 包。
- 页面状态请求实测间隔 1468–1524 ms，共 38 个采样，自动刷新正常。
- Xvfb、fluxbox、x11vnc、websockify、Linux QQ 和 SnowLuma 均已实际启动；插件健康接口、SnowLuma WebUI 和 noVNC 返回 HTTP 200。
- x11vnc 使用 `-passwd` 启动，密码由每次桌面启动重新生成并在插件页面展示；原生 VNC 协议认证、noVNC 密码输入和 QQ 主窗口显示均已实测通过。
- VNC 画面确认 QQ 登录账号为“大肥鱼（不肥版）”。插件健康接口仍返回 `qqOnline=false`，未将 OneBot 在线标记为通过。
- QQ 插件运行测试 73 项通过；此前同任务内手机端相关测试 84 项通过。

## 运行环境修复

- PRoot 不实现 `setgid/getuid` 和 `setuid/getuid` 的等价降权调用，Xvfb 的子进程初始化因此返回 `ENOSYS`。新增仅随插件发布的 ARM64 兼容层，只在目标身份与当前身份一致时把该无操作调用视为成功，未修改 Xvfb 二进制，也未提升整个后端为 fake root。
- x11vnc 在 Android 环境中不可用 System V 共享内存，启动参数增加 `-noshm`。
- noVNC 工作区 Tab 地址修正为 `/vnc.html`，不再打开 websockify 根目录列表。
- 移动端安装进度使用页面生命周期 1.5 秒刷新；依赖、QQ 下载、解压和 SnowLuma 启动阶段均显示实时状态。
- PRoot 占用 ptrace 时 SnowLuma 自身注入会返回 `COMPONENT_LOAD_FAILED`。手机端改为在 QQ 主进程启动时通过受限 loader 调用动态 Hook，并等待控制 socket 建立。
- Hook 采用两阶段启动：先启动 QQ 桌面和 SnowLuma，确认 SnowLuma 可访问后再重启桌面并注入，避免 SnowLuma 未就绪时导致 QQ `SIGTRAP`。
- loader 使用插件父 PID 白名单和临时环境变量，只在 QQ 主进程初始化，启动后清理注入环境，避免 GPU、renderer 和 utility 子进程继承后启动失败。
- 手机运行状态在桌面和 SnowLuma 启动完成后原子写回 `waiting_login`，清除之前失败运行遗留的过期错误。

## 外部确认项

QQ 已登录且桌面画面正常，但 SnowLuma 2.2.0 对当前 Linux QQ `3.2.32-52194` 存在“Hook 已连接、登录身份未进入 readiness”的上游兼容问题，对应 SnowLuma issue #395、#518 和 #537。插件仍保持 `qqOnline=false`，不能宣称 OneBot 已接通。noVNC 密码问题已排除，当前运行实例的凭据可在 QQ 个人号插件页面的“noVNC 密码”行读取；完整桌面重启后密码会轮换。

截图位于 `artifacts/android-channel-final-20261029/`：

- `模拟器-QQ插件页面-最终.png`
- `模拟器-消息统计-当前渠道.png`
- `模拟器-SnowLuma-WebUI.png`
- `模拟器-noVNC.png`
- `模拟器-QQ已登录-VNC.png`
- `模拟器-noVNC-0.2.66-密码认证.png`
- `模拟器-QQ-0.2.66-已登录.png`
- `模拟器-QQ插件-0.2.66-状态.png`
