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
- QQ 个人号插件：`plugin/com.amitia.channel-qq-personal-0.2.57.amitiax`，SHA-256 `0bbffb25edfe9171ab410f3be5c32dc0509adbdf7e9bc6adc7fc4adcc9b9e8b7`。
- 模拟器内 APK 哈希与本地制品一致；`versionCode=24`、`versionName=26.2.0-beta.18`，不存在 Debug 包。
- 页面状态请求实测间隔 1468–1524 ms，共 38 个采样，自动刷新正常。
- Xvfb、fluxbox、x11vnc、websockify、Linux QQ 和 SnowLuma 均已实际启动；插件健康接口、SnowLuma WebUI 和 noVNC 返回 HTTP 200，x11vnc 5900 可连接并使用页面生成密码认证。
- 用户已通过 noVNC 扫码登录 QQ，VNC 画面可读取已登录的 QQ 主窗口。
- QQ 插件运行测试 70 项、手机端相关测试 84 项通过。

## 运行环境修复

- PRoot 不实现 `setgid/getuid` 和 `setuid/getuid` 的等价降权调用，Xvfb 的子进程初始化因此返回 `ENOSYS`。新增仅随插件发布的 ARM64 兼容层，只在目标身份与当前身份一致时把该无操作调用视为成功，未修改 Xvfb 二进制，也未提升整个后端为 fake root。
- x11vnc 在 Android 环境中不可用 System V 共享内存，启动参数增加 `-noshm`。
- noVNC 工作区 Tab 地址修正为 `/vnc.html`，不再打开 websockify 根目录列表。
- 移动端安装进度使用页面生命周期 1.5 秒刷新；依赖、QQ 下载、解压和 SnowLuma 启动阶段均显示实时状态。

## 外部确认项

QQ 已完成扫码登录，但 SnowLuma 的 `/api/qq-list` 仍返回 `consentRequired=true`，要求用户本人阅读并接受 SnowLuma EULA 与隐私说明。未代替用户接受该法律协议，因此 QQ 进程尚未载入 SnowLuma，插件 `qqOnline` 仍为 false。用户在 SnowLuma WebUI 接受条款并重新检测 QQ 进程后，才能完成真实 OneBot 收发验证。

截图位于 `artifacts/android-channel-final-20261029/`：

- `模拟器-QQ插件页面-最终.png`
- `模拟器-消息统计-当前渠道.png`
- `模拟器-SnowLuma-WebUI.png`
- `模拟器-noVNC.png`
- `模拟器-QQ已登录-VNC.png`
