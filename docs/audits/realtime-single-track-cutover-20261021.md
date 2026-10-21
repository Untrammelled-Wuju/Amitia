# 实时通话单轨接入与旧系统清理

日期：2026-10-21。

## 结论

实时语音此前同时存在旧 Cascade/Voice Session 系统和 Device Mesh Owner 实时会话。电脑端按 Owner 可用性回退到旧独立通话窗口，手机端按本地/Cloud 模式切换协议，旧实时语音页面和两端 Voice Sessions 管理面板仍直接访问旧接口。

现已统一为 Device Mesh Owner 实时会话：

- 唯一票据入口：`POST /api/device-mesh/v1/business/realtime/tickets`
- 唯一 WebSocket：`GET /api/device-mesh/v1/business/realtime/session`
- 电脑端和手机端不再按部署模式或 Owner 可用性回退旧协议
- 旧 `/api/realtime/v2/*`、`/api/voice/sessions/*`、`/api/voice/status` 已删除
- 旧独立通话窗口、旧实时语音页面和两端旧 Voice Sessions 管理面板已删除

## 迁移到新系统的能力

- 显式音色与资源选择：票据和 Owner 语音合成均接收 `voiceType`、`resourceId`
- 流式 ASR：新会话发送 `asr_final`
- 流式回复：新会话发送 `assistant_text`
- 视觉确认：新会话接受视觉帧后发送 `vision_status`
- 播放进度：客户端上报 `playback_progress`、`tts_playback_ended`
- 唤醒：新会话支持 `wake_arm`、`wake_disarm`、`wake_detected`
- 打断、单轮大小和时长限制、Owner 权限变化中断、幂等保存确认
- 来电邀请、接听票据、Owner 确认保存和通话后会话刷新

唤醒检测复用现有 Workflow Wake 检测器注册表，软件、ASR 短语和本地 KWS 后端通过统一接口接入新实时会话。

## 删除的旧代码

- `backend/internal/realtime` 整个旧实时实现
- 旧 Cascade 情绪适配器和旧桌面宠物语音生命周期桥
- 旧聊天实时语音流、实时情绪和桌面宠物通话观察器
- 旧 `RealtimeVoiceView` 和 `RealtimeCallWindowView`
- 桌面端独立通话窗口 IPC、主进程窗口管理和预加载 API
- 两端旧 Voice Sessions 列表、启动、打断、唤醒和停止操作
- Android/iOS 旧 PCM 播放通道实现

## 定向验证

- 新实时票据、真实 TLS WebSocket、ASR、视觉、Owner 保存和音频确认测试通过
- 旧 `/api/realtime/v2/tickets` 与 `/api/voice/status` 契约测试返回 404
- 新票据音色、资源、对话参数和语音合成配置传递测试通过
- 前端类型检查通过
- 前端实时会话与邀请测试 11 项通过
- 前端生产构建通过
- 桌面 TypeScript 检查通过
- 桌面主进程、预加载和渲染器生产构建通过
- 手机端相关文件静态分析无错误
- `go build ./cmd/server` 通过

## 真实运行验证

- 从当前源码重新构建桌面核心
- 完整重启 Electron、前端、Core、Qdrant、SurrealDB 和运行时宿主
- `/readyz` 返回 `ready`
- 前端返回 HTTP 200
- 旧 `/api/realtime/v2/tickets` 返回 HTTP 404
- 旧 `/api/voice/status` 返回 HTTP 404
- 新实时票据缺少 Owner 范围时返回 HTTP 400
- 当前运行核心与独立后端构建 SHA-256 均为 `8b0efdc36db1e650cd6351753b78616c05d52a29fe84638d7eea6bc30f6a7118`

## 验证边界

- 当前真实库没有可用角色，完整语音轮次由真实 TLS 隔离测试覆盖，未向正式数据创建临时角色。
- 桌面全量 Vitest 仍存在 75 个既有桌面宠物状态机、Electron 窗口 mock 和包校验失败，与实时改动无关。
- Flutter 测试仍被缺失的 `rustup` 阻塞；手机端完成静态分析和源码契约测试。
- 本轮未构建 Android Release APK，未执行真机安装。
- 本轮未提交或推送 Git。
