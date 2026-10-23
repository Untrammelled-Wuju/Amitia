# 跨端通知单轨修复与生产模拟

日期：2026-10-23。

## 原问题

项目同时保留两套通知状态与投递路径：

- 电脑端和 Web 使用 `/api/browser-notifications/*` 保存启用与订阅状态。
- 电脑端另用 localStorage 和 Electron IPC 实现“回复完成通知”和提示音。
- 正式 Notification Runtime 已有设备偏好、消息、执行、通话、提醒、隐私预览、outbox、重试、SSE 和回执能力。
- Owned 聊天使用独立 Conversation Runtime，Notification Runtime 只观察旧 DefaultManager，当前聊天不会生成正式消息通知。
- 电脑端注册正式设备时只提交系统通知开关，后端默认关闭桌面消息、执行和通话分类。

## 单轨结构

Notification Runtime 的 `notification_devices` 是通知偏好唯一事实源。所有平台都使用：

- `POST /api/notifications/devices/register`
- `GET /api/notifications/devices`
- `PATCH /api/notifications/devices/:deviceId/preferences`
- `POST /api/notifications/devices/:deviceId/test`
- `POST /api/notifications/presence`
- `POST /api/notifications/scheduled/:id/ack`

电脑端、Web、Android 和 iOS 共享同一设备分类、预览、声音和投递链。电脑端通过 `desktop-sse` 展示，Web 通过浏览器 Notification API 展示，手机继续使用 Native Bridge、厂商 Push、FCM 和 APNs。

## 功能迁移

| 原功能 | 新位置 |
| --- | --- |
| 浏览器启用与订阅 | 正式设备 `pushEnabled`、`systemNotificationsEnabled` |
| 回复完成通知 | 正式 `messagePushEnabled` 与 `message.completed` |
| 回复提示音 | 正式 `soundEnabled` |
| 执行状态通知 | 正式 `executionActivityEnabled` 与运行事件 |
| 通话和提醒通知 | 正式 `callPushEnabled`、`reminderPushEnabled` |
| 通知隐私 | 正式 `previewMode` |
| 浏览器本地测试 | 正式设备 `/test` |
| 桌面旧本地回复通知 | Owned Runtime 事件、Notification Runtime outbox 和 desktop-sse |

旧 `/api/browser-notifications/*`、旧 system 通知 Service/Handler、Electron 回复通知 IPC、前端本地回复通知 Composable、设置组件和对应测试已删除。旧 `app_settings` 通知偏好和旧 localStorage 回复偏好仅在设备首次注册时一次性读取迁移。

## 跨模式

- 本地 Core：Owned 事件直接进入 Notification Runtime，桌面通过 SSE 接收。
- 云端非统筹 Device Agent：客户端仍以真实设备身份注册，使用同一设备偏好；投递由持有通知运行时的 Core 负责。
- 云端统筹 Core：Owner、角色和会话元数据随事件携带，不再依赖旧 `conversations/messages` 表推断。
- 设备切换和 Core 切换：注册、偏好与 presence 均使用当前认证设备和当前业务 Core，迟到事件不会读取其他 Owner 的旧表记录。

## 桌面操作系统适配

- macOS：使用系统 Notification Center，通知 ID 用于替换同类事件，会话 ID 用于通知分组。
- Windows：应用启动时设置固定 `AppUserModelId`，NSIS 安装器创建开始菜单快捷方式；通知映射 ID、会话分组和分组标题，通话与运行状态使用持续通知。
- Linux：设置应用名、`amitia.desktop`、`StartupWMClass`、通知类别和 1024px 应用图标；消息使用普通优先级，执行状态使用低优先级，通话使用关键优先级。
- 三个平台注册设备时均提交真实主机名、操作系统版本、应用版本、语言、时区和主题。

## 定向验证

- `go test ./internal/notificationruntime ./internal/devicemesh/business ./internal/configwrite ./internal/system` 通过。
- `go build ./cmd/server` 通过。
- 电脑端 `tsc --noEmit` 通过。
- 电脑端 UI Host SSE 4 项测试通过。
- 电脑端公共业务客户端、UI Host SSE 与桌面跨平台通知宿主共 10 项测试通过。
- 前端 `vue-tsc --noEmit`、生产构建通过。
- 前端通知与运行投影 10 项测试通过。
- `git diff --check` 通过。
- Linux `electron-builder --dir` 和 AppImage 临时打包通过，桌面入口、应用图标、可执行名和构建配置均被 electron-builder 接受。

## 真实生产模拟

使用当前正式配置、真实本地凭据和完整 Electron/Core/Qdrant/SurrealDB 环境：

1. Electron 在 SSE 建立前自动注册 `desktop` 设备，并以注册写入响应确认保存。
2. `/api/notifications/devices` 返回正式分类偏好，消息、执行、通话、提醒、声音和完整预览均为开启状态。
3. 正式设备测试接口返回 `accepted=true`，Provider 为 `desktop-sse`。
4. 通过真实 Owned 消息接口调用已配置模型，模型返回并保存成功。
5. 桌面通知 outbox 生成 `message.completed`，Provider 为 `desktop-sse`，一次尝试后状态为 `accepted`。
6. 关闭 `messagePushEnabled` 后再次完成真实模型消息，消息通知投递数保持不变。
7. 恢复消息分类并将 presence 设置为当前会话前台可见，再次完成真实模型消息，通知投递数保持不变。
8. 测试会话、消息、摘要、工作记忆、情景记忆、图谱和图谱索引来源均按 Owner 删除入口写入删除墓碑，测试通知 outbox 与 delivery 记录按精确编号清理。
9. 从当前源码临时生成 macOS arm64 应用包并启动，内置 Core 哈希与项目 Core 一致；测试通知写入 `system.test` outbox，`desktop-sse` 接受投递，系统失败回执通过正式接口写入 `display_failed`。

复测时发现并修复了电脑端主进程公共业务客户端的会话请求头异步组合缺陷。该缺陷会让通知设备注册、回执和 SSE 请求偶发缺少桌面会话凭据；现在通知宿主使用写入响应完成持久化确认，对业务层 `401` 使会话缓存失效并重试一次，SSE 建流也会识别 JSON `401` 后重建会话。正式测试通知现在写入同一 outbox 投递账本，系统展示回执可正常闭环。

当前核心由项目脚本从本次源码重新构建：

- Core SHA-256：`f86f64f98f742a19910ae918d43e5c916feefd4972e9515626e1a2c37fe20c58`
- 冻结范围：11092 项

验证边界：macOS 已用当前源码的临时应用包完成正式设备注册、SSE 建流、测试通知投递、系统回执和 Core 哈希验证。当前 macOS 返回 `UNErrorDomain` 错误 1（`NotificationsNotAllowed`），系统拒绝展示通知，应用正确记录为 `display_failed`；需在“系统设置 → 通知 → Amitia”授予权限后才能确认真实横幅展示，当前不能声称已完成 macOS 弹窗验收。Windows 和 Linux 完成平台逻辑、配置与 Linux 包结构验证，当前没有对应操作系统的运行实例，不能声称已在真实 Windows/Linux 桌面弹窗验收。本轮未构建 Android Release APK 或 Windows/macOS 正式安装包，未发布、未上传 Git。云端和设备切换契约由同一事件元数据、设备身份和 API 覆盖，未在当前环境启动独立 Cloud Core 群集。
