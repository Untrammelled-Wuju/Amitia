# Amitia Personal WeChat Channel

独立于 `plugins/channels/wechat`（Tencent OpenClaw/iLink）的普通个人微信渠道插件。现有 OpenClaw 插件保持原样，不依赖本插件。

## 1.5.0 Windows / Linux 共用架构

- Windows x64：Amitia Native Agent 托管官方 `Weixin.exe`。优先使用版本化 Hook Driver；没有验证过的 Hook 时，UI Driver 只提供登录二维码/登录状态，消息收发 fail-closed。
- Linux x64：Amitia Native Agent 托管官方 Linux 微信。优先使用版本化 `LD_PRELOAD` Driver；没有验证过的版本 Adapter 时，在 X11/XWayland 可用 UI Driver 提供二维码/登录状态，消息收发 fail-closed。
- 两个平台共用同一个 Channel Provider、Service、UI、消息规范和 Native Driver Contract。
- 入站消息继续进入 Amitia 当前空间默认角色，不增加微信账号→角色的专属宿主逻辑。

## 公共 Native Companion 能力

本插件使用的 `runtime.nativeCompanions` 是 Extension Kernel 的公共 Trusted Service 能力，不属于微信插件。任何可信 Service 扩展都可用相同 manifest 契约声明平台原生辅助程序/库。

宿主会：

1. 只允许 manifest 精确声明的 native 资产进入包；
2. 在包校验和 Service 注册时复核 SHA-256；
3. 只向当前 OS/架构暴露匹配的 canonical path；
4. 注入 `AMITIA_NATIVE_COMPANIONS_VERSION=1` 与 `AMITIA_NATIVE_COMPANIONS`；
5. 未声明、路径越界、哈希不一致均 fail-closed。

插件不会通过改后缀、Base64 解包或运行时下载来绕过安全检查。

## 当前内置能力

| 能力 | Windows | Linux |
| --- | --- | --- |
| 官方客户端探测/启动 | 是 | 是 |
| 插件内登录画面/二维码 | UI Driver | X11/XWayland UI Driver |
| 登录状态 | UI 派生 / Hook | UI 派生 / Preload |
| 隐藏托管窗口 | Win32 | X11/XWayland best-effort |
| 文本接收 | 仅验证过的 Hook | 仅验证过的 Preload Adapter |
| 文本发送 | 仅验证过的 Hook | 仅验证过的 Preload Adapter |

登录 UI fallback 与消息 Hook 是两层能力。插件不会因为二维码可用就宣称消息链路已经可用。

## Driver 状态

Native Agent 对上层暴露统一 capability：

- `qr`
- `loginStatus`
- `selfProfile`
- `receiveText`
- `sendText`
- `attached`

Service 只有在 `receiveText && sendText` 都为真时才进入完整 `connected`；仅登录完成时进入 `connected_limited`。

## 第三方兼容层

历史 hero/aixed HTTP Driver 兼容代码仍保留用于开发迁移，但默认关闭。只有显式设置：

```text
AMITIA_WECHAT_EXTERNAL_DRIVER_COMPAT=1
```

才会探测外部服务。正式 `.amitiax` 默认只走 Amitia Native Companion。

旧版 hero `9999/message` 无鉴权回调端口默认完全不启动。仅在开发迁移时同时显式设置：

```text
AMITIA_WECHAT_EXTERNAL_DRIVER_COMPAT=1
AMITIA_WECHAT_EXTERNAL_CALLBACK_UNAUTHENTICATED=1
```

才会开启该不安全兼容入口。生产环境禁止开启第二个开关。

## Trusted Service 本地鉴权

本插件使用 Extension Kernel 的公共 Trusted Service Loopback Authentication v1。宿主为每个 `extensionId + moduleId` 派生进程级随机 Bearer Token，并同时注入插件 Service 与宿主 Channel HTTP Provider：

```text
AMITIA_SERVICE_AUTH_VERSION=1
AMITIA_SERVICE_AUTH_TOKEN=<process-scoped-token>
AMITIA_CORE_URL=http://127.0.0.1:<host-port>
```

`19878` 上所有插件 API 均要求 `Authorization: Bearer ...`。插件使用宿主注入的 `AMITIA_CORE_URL` 提交入站消息，同时携带 `X-Amitia-Extension-ID` 和 `X-Amitia-Module-ID`，宿主校验其声明的 `channel.provider` 与 `channelId`。Service 不返回 wildcard CORS，Token 不暴露给插件 UI。该鉴权机制属于公共 Extension Kernel 能力，不是微信专属实现。

渠道状态由插件 Service 维护。插件页面只在打开时和用户显式操作后读取状态，不启动定时轮询。
