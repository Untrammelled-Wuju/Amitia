# Windows / Linux Architecture

## Shared layer

```text
Amitia UnifiedEntry / DeliveryWorker
              ↑ ↓
wechat-personal.amitiax
  ├─ UI
  ├─ Channel Provider
  ├─ PersonalWechatService
  └─ NativeCompanionManager
              ↓ stdio JSON-RPC
        Platform Native Agent
              ↓
     Platform Driver Strategy
```

`channelId = wechat_personal`。入站消息仍使用现有空间默认角色策略。

## Generic host capability

宿主只新增公共 `Native Companion v1`，不知道“微信”这一概念：

```text
runtime.nativeCompanions[]
        ↓
Package Security
        ↓ SHA256 + declared path
Canonical module installation
        ↓ current OS/arch filter
AMITIA_NATIVE_COMPANIONS_VERSION=1
AMITIA_NATIVE_COMPANIONS=[...]

插件 Service 只能从该公共描述符列表中选取 Agent/Driver Library，并通过 `driverLibraryPath` 交给 Native Agent；不得猜测包内路径或运行时下载原生库。
        ↓
Trusted Service
```

Channel 插件身份也通过通用的 `extensionId/providerId` 入站字段传递，不给 `wechat_personal` 增加宿主 switch 特例。

## Windows x64

```text
PersonalWechatService
       ↓
amitia-wechat-agent.exe
       ↓
official Weixin.exe
       ├─ verified Hook pipe → QR/login/message send+receive
       └─ Win32 UI fallback → QR/login only
```

Agent 不内置具体微信偏移。没有与当前版本匹配的 Hook 时，UI fallback 可以完成登录，但 `receiveText/sendText=false`。

## Linux x64

```text
PersonalWechatService
       ↓
amitia-wechat-agent
       ↓
official Linux WeChat
       ├─ verified LD_PRELOAD Adapter → Unix socket → full capabilities
       └─ X11/XWayland UI fallback → QR/login only
```

纯 Wayland 通常不允许任意管理/抓取其他应用窗口；这种环境下 UI fallback 会明确报告不可用。Flatpak 微信可启动，但宿主 `LD_PRELOAD` 无法直接进入其沙箱，因此消息 Hook 需要单独适配。

## Login state

两端统一：

```text
probe client
  ↓
start official client when needed
  ↓
probe verified internal driver
  ├─ available → use its capabilities
  └─ unavailable → UI login fallback
  ↓
get QR/login image
  ↓
best-effort hide managed official window
  ↓
poll login state
```

UI-derived login state会标记 `derived=true`，不能被解释成协议级认证保证。

## Fail-closed rules

- 未验证微信内部版本：不调用内部偏移。
- Companion 哈希不一致：Trusted Service 注册失败。
- 未声明 native 二进制：`.amitiax` package security 拒绝。
- 只有 QR/login、没有消息能力：`connected_limited`，不允许发送消息。
- 现有 OpenClaw/iLink 插件保持独立。
