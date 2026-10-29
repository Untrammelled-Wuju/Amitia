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

- APK：`artifacts/android-plugin-actions-20261029/amitia-26.2.0-beta.18-release-arm64-v8a.apk`，SHA-256 `6b4846c1f6b3898a136c911970608dbe89df654ecb2ea5754a0b22401575b106`，后端源码提交 `fc14be109c00850b06de4eb78e37c1739193d677`。
- Runtime：`artifacts/android-plugin-actions-20261029/amitia-runtime-1.0.0-android-arm64.zip`，SHA-256 `47a0b9ce7c570f9312903baccb60fd8ae6de2caf20f72585aa976a324083e774`。
- Linux ARM64 后端：`artifacts/android-plugin-actions-20261029/backend/amitia-server`，SHA-256 `04c70e7e8195d2d1d6fa45e6f94f8008dbcd0ec0b73450e62e32b9845bff591c`。
- QQ 个人号插件：`plugin/com.amitia.channel-qq-personal-0.2.83.amitiax`，SHA-256 `dbf23a65f234baca0bf9e2d50b780773a204d87b2b2d0261e4758d5536a69a53`，treeHash `d9ce75ceac0d48dabe83ea3412e424be14a1fc63a0369a53e305ffa48c93c049`。
- 模拟器内 APK 哈希与本地制品一致；`versionCode=24`、`versionName=26.2.0-beta.18`，不存在 Debug 包。
- 页面状态请求实测间隔 1468–1524 ms，共 38 个采样，自动刷新正常。
- Xvfb、fluxbox、x11vnc、websockify、Linux QQ 和 SnowLuma 均已实际启动；插件健康接口、SnowLuma WebUI 和 noVNC 返回 HTTP 200。
- x11vnc 使用 `-passwd` 启动，密码由每次桌面启动重新生成并在插件页面展示；原生 VNC 协议认证、noVNC 密码输入和 QQ 主窗口显示均已实测通过。
- VNC 画面确认 QQ 登录账号为“大肥鱼（不肥版）”。SnowLuma 完成 Hook 连接后识别 UIN `3366726108`，OneBot 客户端数为 1，插件健康接口返回 `qqOnline=true`。
- QQ 插件运行测试 77 项通过；手机端 Schema 渲染回归测试 3 项通过；此前同任务内手机端相关测试 84 项通过。
- 冷启动后插件健康接口返回 `qqRuntimeInstalled=true`、`snowlumaInstalled=true`、`snowlumaRunning=true`、`hookReady=true`、`qqOnline=true`，Hook 进程 PID 与当前 QQ 主进程一致。

## 运行环境修复

- PRoot 不实现 `setgid/getuid` 和 `setuid/getuid` 的等价降权调用，Xvfb 的子进程初始化因此返回 `ENOSYS`。新增仅随插件发布的 ARM64 兼容层，只在目标身份与当前身份一致时把该无操作调用视为成功，未修改 Xvfb 二进制，也未提升整个后端为 fake root。
- x11vnc 在 Android 环境中不可用 System V 共享内存，启动参数增加 `-noshm`。
- noVNC 工作区 Tab 地址修正为 `/vnc.html`，不再打开 websockify 根目录列表。
- 移动端安装进度使用页面生命周期 1.5 秒刷新；依赖、QQ 下载、解压和 SnowLuma 启动阶段均显示实时状态。
- PRoot 占用 ptrace 时 SnowLuma 自身注入会返回 `COMPONENT_LOAD_FAILED`。手机端改为在 QQ 主进程启动时通过受控 wrapper 预加载 Hook，并等待控制 socket 建立。
- Hook 采用两阶段启动：先启动 QQ 桌面和 SnowLuma，确认 SnowLuma 可访问后再重启桌面并注入，避免 SnowLuma 未就绪时导致 QQ `SIGTRAP`。
- QQ wrapper 保留原二进制为 `.amitia-real`，普通启动通过禁用标记直接透传，Hook 阶段再注入环境；loader 启动后清理 `LD_PRELOAD`，避免 GPU、renderer 和 utility 子进程继承后失败。
- QQ 重新登录执行 `.amitia-real --relaunch` 时，loader 会拦截主进程的 `execv/execvp/execvpe/posix_spawn/posix_spawnp` 并重新进入 wrapper，使新主进程继续自动加载 Hook。
- SnowLuma 页面再次点击“加载”时会检测到已有控制管道，只执行重连，不再调用 ptrace 注入。
- Android 拒绝 SnowLuma 读取 `/proc/locks` 和 `/proc/<pid>/environ`，导致已登录账号无法进入 readiness。兼容层仅针对 QQ 主进程实际持有的数据库生成只读锁表，并提供限定 HOME 的进程环境视图。
- QQ Linux 3.2.32 的 native Hook 在收到真实 UIN 身份提示后仍未回传登录状态帧。兼容层只在 SnowLuma 已发送真实 UIN 提示后补齐 logged-in 帧，使 OneBot 建立连接；扫码页或未持有 QQ 数据库时不会触发。
- 手机端修复 `button_group` 子按钮未解析 bindings 的通用缺陷；QQ 与 SnowLuma 均安装后，按钮显示“重新安装 QQ 与 SnowLuma”，并在运行状态中提示 Hook 已预加载。
- 手机运行状态在桌面和 SnowLuma 启动完成后原子写回 `waiting_login`，清除之前失败运行遗留的过期错误。

## 外部确认项

QQ 与 OneBot 已完成本地联通。SnowLuma 2.2.0 对 Linux QQ `3.2.32-52194` 的 readiness 缺陷仍属于上游问题，对应 issue #395、#518 和 #537；当前通过上述 Android 环境兼容层规避。实际 QQ 消息收发仍需在用户触发真实会话后验证，不能仅以连接数推断消息链路全部通过。

截图位于 `artifacts/android-channel-final-20261029/` 和 `artifacts/android-plugin-actions-20261029/`：

- `模拟器-QQ插件页面-最终.png`
- `模拟器-消息统计-当前渠道.png`
- `模拟器-SnowLuma-WebUI.png`
- `模拟器-noVNC.png`
- `模拟器-QQ已登录-VNC.png`
- `模拟器-noVNC-0.2.66-密码认证.png`
- `模拟器-QQ-0.2.66-已登录.png`
- `模拟器-QQ插件-0.2.66-状态.png`
- `插件页面-重新安装与Hook状态.png`
- `noVNC-密码认证.png`
- `QQ-已登录.png`
- `插件页面-OneBot已连接.png`
