# 会话消息队列交接文档

- 日期：2026-11-20
- 工作区：`/Users/untrammelled/Desktop/Project/U-AI`
- 分支：`develop`，相对 `origin/develop` ahead 2
- Git 状态：未提交、未推送

## 当前结论

消息队列的主体功能已经完成并通过定向测试，覆盖入队、顺序执行、拖拽或键盘排序、引导、删除、重启恢复、失败与不确定状态可见、引用和附件引用、群聊普通模式与讨论模式。

当前不能宣称“全部验收完成”：

- 完整项目已停止，交接时只剩一个 `plugin-host` 孤儿进程。
- 实际会话数据库里仍有一条失败状态的测试排队消息，需要下次启动后删除。
- `cmd/server` 整包测试存在与本功能无关的既有失败，因此只有定向测试和编译通过，不是全量绿色。
- 手机端没有构建或安装 APK；已通过 Flutter widget 测试验证长按拖拽和操作按钮，未做真机交互。

## 核心设计

### 持久化与归属

队列文档存储为 `checkpoint`，资源 ID 前缀为 `queued-chat/`。队列按以下信息隔离：

- 空间
- 发起设备
- 目标设备
- 会话
- 角色
- 资源所有者
- 角色所有者

队列状态包括：

- `queued`
- `running`
- `steering`
- `steered`
- `completed`
- `failed`
- `uncertain`
- `deleted`

### 顺序与领取

- 消息按持久化 `position` 排序，`position` 相同时按请求 ID 排序。
- 执行锁先于队列领取，只有会话空闲时当前首项才会进入执行。
- 非首项返回“尚未轮到执行”，由后台重试，不改变顺序。
- 重复 requestId 且内容不同会返回冲突；相同请求可幂等读取。

### 重启恢复

- 数据库中的队列文档会恢复 `executionScope`。
- 恢复时会把 `TargetDeviceID`、`ResourceOwnerID`、`RoleOwnerID` 回填到请求中，避免协调层错误地把设备数据归属判为中心 Core。
- 已领取但没有输入检查点时，恢复后允许重新执行。
- 已有 `running` 检查点时不再重放模型或工具，转为 `uncertain` 并展示完整错误。
- 已有 `completed` 检查点但队列结果尚未落盘时，补记队列完成，不再次调用模型。
- 群聊使用已有 `group-round` 做幂等恢复。

### 引导

引导采用持久化过渡态：

1. 校验目标 turn 仍可引导。
2. 队列文档写成 `steering`。
3. 运行时接收引导内容。
4. 队列文档写成 `steered`。

如果进程在 `steering` 后中断，恢复时转为 `uncertain`，不会再次投递，避免重复执行。完整原始错误链保存在队列文档和响应中。

### 管理请求竞态

引导、删除、排序和列表刷新不再每次重新查询会话。它们复用队列列表返回的 `executionScope` 直接调用后端，由后端再次校验归属。这样可以避免其他轮询更新版本后把合法操作误报为“旧上下文已丢弃”。

## 已完成代码

### 后端

- `backend/internal/devicemesh/business/chat_queue.go`
  - 队列内容、位置、状态、引用和附件摘要
  - 领取、排序、删除、引导、恢复
  - `steering` 过渡态和不确定状态
  - 恢复时回填持久化所有者
- `backend/internal/devicemesh/business/chat_queue_order_test.go`
  - 顺序、删除、引导、重复请求、会话隔离、设备隔离、引用、重启与回填测试
- `backend/internal/devicemesh/coordination/ownership.go`
  - 队列快照包含 `steering`
- `backend/cmd/server/mesh_chat_queue_integration_test.go`
  - 群聊普通模式、讨论模式、成员设置变化错误
- `backend/internal/devicemesh/business/engine.go`
  - 执行路径接入队列领取
- `backend/internal/devicemesh/business/group_chat.go`
  - 群聊执行路径接入队列领取
- `backend/cmd/server/mesh_chat_runtime_router.go`
  - 队列列表、入队、排序、删除、引导接口
- `backend/internal/conversationstream/manager.go`
  - 引导内容在运行时提交后接收
- `backend/go.mod`、`backend/go.sum`
  - 补齐既有 PDF 附件改动需要的精确依赖校验和

### 桌面端

- `front/src/components/QueuedMessages.vue`
  - Codex 风格紧凑队列
  - 鼠标拖拽
  - 上下移动按钮，支持键盘操作
  - 引导、删除、失败状态和完整原始错误
- `front/src/composables/useConversationQueue.ts`
  - 队列轮询、会话与角色切换保护
  - 使用已确认 `executionScope`
- `front/src/runtime/owned-chat-management.ts`
  - 新增使用既有执行范围的队列管理请求
- `front/src/views/web-chat/BuiltinWebChatView.vue`
  - 忙态判断同时使用运行状态和提交状态
  - 排队引用、上下文、附件、群聊参数
  - 停止任务入口保留
- `front/src/composables/useWebChatSend.ts`
  - 语附件排队
  - 完成态与失败态保持
- `front/src/runtime/device-owned-attachments.ts`
  - 附件先上传，再使用制品引用
- `front/src/__tests__/queuedMessages.test.ts`
- `front/src/__tests__/device-owned-attachments.test.ts`

### 手机端

- `mobile_app/lib/features/chat/presentation/widgets/queued_messages_panel.dart`
  - 独立队列面板
  - `ReorderableDelayedDragStartListener` 长按拖拽
  - 附件、引用、状态、引导、删除、原始错误
- `mobile_app/lib/features/chat/presentation/pages/chat_page.dart`
  - 引用排队、附件排队、停止与删除
- `mobile_app/lib/features/chat/runtime/conversation_runtime_controller.dart`
  - 队列轮询 generation
  - 会话和角色切换隔离
  - `executionScope` 复用
  - 已中断回合不再允许引导
- `mobile_app/lib/core/services/device_owned_chat_service.dart`
  - 使用既有范围执行队列管理请求
- `mobile_app/lib/core/services/device_owned_attachments.dart`
  - 聊天附件上传与引用校验
- `mobile_app/lib/core/artifact/artifact_model.dart`
- `mobile_app/lib/core/artifact/artifact_service.dart`
- `mobile_app/test/features/chat/conversation_runtime_controller_test.dart`
- `mobile_app/test/features/chat/widgets/queued_messages_panel_test.dart`

## 已验证

### 后端

通过：

```bash
go test ./internal/devicemesh/business -run 'TestQueued|TestSteered|TestDuplicateQueued|TestInterruptedSteering|TestRunningCheckpoint|TestCompletedTurnRecovers|TestQueuedChatRestartRestoresPersistedResourceOwner|TestQueuedChatAcceptsValidatedQuote' -count=1
go test ./cmd/server -run 'TestQueuedGroupMessageUsesGroupExecution' -count=1
go build ./cmd/server
```

竞态检测通过：

```bash
go test -race ./internal/devicemesh/business -run 'TestQueued|TestSteered|TestDuplicateQueued|TestInterruptedSteering|TestRunningCheckpoint|TestCompletedTurnRecovers' -count=1
```

业务包和协调包整包测试曾通过；`cmd/server` 整包测试曾失败于：

- `TestActualTLSAttachmentsStayAtTheirSelectedOwner`
- `TestOwnedRealtimeRealTLSRejectsReplayForeignCoreOriginAndSavesEachMode`

这两个失败属于当时工作区中并发进行的附件或实时路由改动，不是队列定向测试失败。

### 桌面端

通过：

```bash
node node_modules/vue-tsc/bin/vue-tsc.js --noEmit
node node_modules/vitest/vitest.mjs run src/__tests__/queuedMessages.test.ts src/__tests__/device-owned-attachments.test.ts src/__tests__/webChatSendCompletion.test.ts src/__tests__/ownedProjectDraftSend.test.ts
node node_modules/vite/bin/vite.js build
```

结果：

- 类型检查通过
- 21 项定向测试通过
- 生产构建通过

### 手机端

通过：

```bash
flutter test test/features/chat/conversation_runtime_controller_test.dart test/features/chat/widgets/queued_messages_panel_test.dart test/core/widgets/chat_input_send_mode_test.dart
```

结果：30 项测试通过。另有一组附件服务与手机聊天服务测试共 64 项通过。

### 实际桌面交互

已在 Electron 实际界面验证：

- 当前任务执行中发送消息进入队列
- 两条消息按顺序展示
- 使用上下移动按钮调整顺序
- 将排队消息用于引导当前任务
- 删除排队消息
- 停止当前任务入口可用
- 重启后队列从 `queued` 恢复并领取为 `running`
- 失败项显示完整原始错误，引导按钮禁用

受 CUA 坐标缩放影响，物理鼠标拖拽没有在 Electron 外层窗口完成；Vue 组件的 `dragstart/drop` 测试已通过。

## 未完成或需要继续验证

1. 当前完整服务已停止。交接时内核、前端、Qdrant、SurrealDB 进程均不在运行。
2. `AmitiaData/plugin-host/dist/index.js` 仍有一个遗留进程，启动脚本会清理。
3. 实际会话数据库中有一条失败测试记录：

   - requestId：`65af6e71-45c8-41e8-9fcb-36b62412d77c`
   - 文本：`排队消息一：统计完成后补充列出 components 目录。`
   - 状态：`failed`

   下次启动完整项目后，应通过队列界面删除。

4. 自动执行已验证到“重启后首项进入 running”，但由于实际宿主工具长时间阻塞并被停止，没有完整观察“首项结束后第二项自动完成”的真实模型链路。后端顺序和恢复测试已覆盖该行为。
5. 手机端未构建或安装 APK。长按拖拽已通过 Flutter widget 交互测试，但没有真机验证。
6. 前端热更新在包含 DSH 动态 client module 时可能触发 CSP blob 模块失效；完整重启可恢复。该问题与队列业务逻辑无直接关系，但会影响开发态连续 UI 验收。
7. 图片、视频、文件、语音的制品上传及引用有定向测试，尚未完成真实界面的多附件完整对话验收。
8. 跨设备实际双端同步尚未完成真实设备联调；后端已有会话和发起设备隔离测试。
9. 排队引用的后端校验和两端请求构造已覆盖，实际界面引用排队尚未完成最终端到端验证。

## 下次继续建议

1. 先检查工作区，不要回退其他未提交修改。
2. 使用项目内 Node 和既有完整启动脚本启动项目。
3. 启动后清理上述失败测试队列记录。
4. 重新执行本文件中的后端、前端和 Flutter 定向测试。
5. 使用两个真实客户端验证：

   - 相同数据所有者、不同发起设备
   - 不同会话
   - 一端入队后另一端刷新
   - 引导和删除的跨端可见性

6. 用一个可控的短任务完成“首项结束后自动执行第二项”的真实验收。
7. 完成手机端引用、图片、视频、文件和语音的排队真机验证。

## 构建与数据说明

- 本次没有提交或推送 Git。
- 未构建 Android Release APK。
- 为运行当前源码，开发核心 `desktop/resources/core/AmitiaCore` 和 `desktop/resources/core/macos-arm64/AmitiaCore` 已重建。
- `front/dist` 由生产构建生成，属于构建输出，不是源码。
- `backend/server` 临时编译文件已删除。
