---
title: 通用附件与 Agent 自主处理交接
date: 2026-11-20
branch: develop
status: 开发中，未完成端到端验收
---

# 通用附件与 Agent 自主处理交接

## 结论

附件上传、消息引用、跨设备分块读取、Agent 按需处理、Web/Electron 和移动端入口已完成主要源码改造，但不能视为整项功能已完成。

当前明确未通过的是后端 `cmd/server` 聚合测试，至少存在两个问题：

1. `TestMeshChatAttachmentStorageValidatesReferenceAndChunks` 的测试范围缺少 `RequestID`，该测试范围已修正，但修正后尚未重新执行该测试。
2. `TestOwnedRealtimeRealTLSRejectsReplayForeignCoreOriginAndSavesEachMode` 在测试夹具中直接使用了未初始化的 `services.Artifact`，触发空指针。生产服务装配路径会初始化 `Artifact`，但三方设备测试夹具仍需补齐同一装配。

完整项目当前没有运行。最近一次运行的核心来自旧进程，健康接口曾正常返回；当前 `AmitiaData/run/endpoint.json` 指向端口 `50791`，进程已不存在，仅残留一个孤立 `plugin-host`。不能假定现有端口、Runtime 或插件宿主仍可用。

本轮没有提交、没有推送、没有上传、没有构建 APK、没有修改管理端、没有修改编译后产物。

## 工作区状态

- 分支：`develop`，相对 `origin/develop` ahead 2。
- 工作区有大量既有未提交修改和未跟踪文件，必须在原状态上继续，禁止 reset、checkout 覆盖或清理工作区。
- 本轮涉及的源码统计约为 121 个文件、4826 行新增、2002 行删除，其中包含此前已经存在的附件基础改造。
- 当前没有可安全视为本轮专属的提交边界。后续提交前必须逐项确认归属。

## 已完成能力

### 附件协议与上传

当前附件引用包含：

- `artifactId`
- `ownerDeviceId`
- `storageDeviceId`
- `kind`
- `name`
- `mimeType`
- `sha256`
- `sizeBytes`

`/api/artifacts/v1/chat-uploads` 使用请求体直接流式写入磁盘，不再将文件整体转换为 Base64，也不设置产品级固定大小上限。

Web/Electron 和移动端都按原始二进制流上传，消息中只保存附件引用。

### 消息保存与权限校验

普通聊天、群聊、排队消息、消息追加和重试路径均调用附件校验。

校验内容覆盖：

- 实际上传设备
- 实际存储设备
- 文件元数据
- SHA-256
- 文件大小
- 消息、会话、角色和数据归属

跨设备读取按 `storageDeviceId` 分块读取，每块上限为 512 KiB，并在读取时再次校验摘要、大小、当前角色和协调权限。

### 两阶段引用与崩溃恢复

本轮新增了两阶段附件引用：

1. 消息提交前写入 `message_attachment_pending`。
2. 消息提交成功后原子删除 pending 并写入 `message_attachment` 与 `chat_attachment_binding`。
3. 进程在“引用已写入、消息未提交”或“消息已提交、引用未确认”之间退出时，由后台恢复器查询真实消息并决定提交或释放引用。
4. 消息删除会同时清理 pending、正式消息引用和绑定记录。

恢复器每两分钟执行一次，启动时立即执行一次。远端设备离线、协调权限变化或消息暂时无法读取时，恢复器保留记录并输出完整原始错误，稍后重试。

核心文件：

- `backend/internal/artifact/chat_attachment_reference.go`
- `backend/internal/devicemesh/coordination/attachment.go`
- `backend/internal/devicemesh/business/chat_attachment_validation.go`
- `backend/cmd/server/chat_attachment_recovery.go`
- `backend/cmd/server/mesh_chat_attachment_port.go`

### Agent 工具

Agent 获得以下工具：

- `chat_attachment_read`
- `chat_attachment_extract`

模型上下文仅接收附件元数据和消息标识，不自动接收文件全文、图片字节或 Base64。

`chat_attachment_read` 按有限字节范围读取，文本合法时返回 UTF-8，否则返回二进制分块。

`chat_attachment_extract` 会流式物化附件到临时目录，校验大小和 SHA-256 后按类型处理：

- PDF：进程内文本抽取
- DOCX：ZIP/XML 正文与表格文本
- XLSX：共享字符串、工作表、单元格
- PPTX：幻灯片文本
- 图片：现有视觉模型
- 扫描 PDF：优先提取页面内嵌图片并交给视觉模型，其次尝试外部 `pdftoppm`
- 未知二进制：返回需使用已授权通用工具的结果

文档处理失败会作为工具结果返回给 Agent，不会在模型接手前直接中断整轮任务。

### Web 与 Electron

- 文件直接以 `Blob` 流式上传。
- 不再使用固定 MIME 白名单和 1 MiB 上限。
- 流式计算 SHA-256。
- 图片、视频使用 Object URL 预览。
- 历史附件按已保存消息生成授权读取地址。
- 聊天附件增加短期签名媒体票据，图片、音频、视频和 Range 请求可直接流式访问，不再由渲染进程整体下载为 Blob。

核心文件：

- `front/src/runtime/device-owned-attachments.ts`
- `front/src/runtime/sha256-stream.ts`
- `front/src/conversation/rendering/media.ts`
- `front/src/composables/useWebChatSend.ts`
- `front/src/composables/useMediaUpload.ts`
- `front/src/views/web-chat/BuiltinWebChatView.vue`

### Android 与 iOS

- 普通文件、图片、视频和音频选择均接入 `chat-uploads` 流式上传。
- 历史附件引用和授权读取地址已接入消息恢复与渲染。
- 录音改为逐块写入临时 WAV 文件，不再使用无限增长的 `BytesBuilder`，并移除 1 MiB 录音长度限制。
- Provider 事件优先使用 `resourceUri`、`filePath`、`localPath` 或 `fileUri` 进行流式上传。
- 聊天附件预览改为短期媒体票据，不再先整文件下载到内存。
- 移动端远端下载改为打开受签名保护的下载 URL，由系统浏览器或下载器流式处理。

仍需注意：Provider 协议中的 Base64 输入分支仍然保留，作为 UI Runtime 只提供 Base64 时的兼容入口；最终保存仍是附件制品，不会把 Base64 放入消息或模型上下文。若要严格执行“当前版本不保留任何 Base64 输入”，需要同步修改 Provider 协议和所有调用方。

核心文件：

- `mobile_app/lib/core/artifact/artifact_service.dart`
- `mobile_app/lib/core/services/device_owned_attachments.dart`
- `mobile_app/lib/features/chat/presentation/pages/chat_page.dart`
- `mobile_app/lib/features/chat/runtime/conversation_runtime_controller.dart`
- `mobile_app/lib/features/chat/presentation/pages/realtime_call_page.dart`

### 备份与恢复

附件备份贡献器已接入数据可携带性服务，备份包含 blob 和附件归属引用。

核心文件：

- `backend/internal/artifact/backup_contributor.go`
- `backend/cmd/server/services_dataportability.go`

## 本轮新增依赖

`backend/go.mod` 新增：

- `github.com/ledongthuc/pdf`
- `github.com/pdfcpu/pdfcpu v0.16.4`

`go mod tidy` 已执行，但依赖整理同时升级了若干间接依赖和 `golang.org/x/*` 版本，并把 `modernc.org/sqlite` 调整为直接依赖。合并前需要单独审查这次依赖变化，确认不是无关升级。

本机没有系统 `pdftotext`，系统路径也没有稳定的 `pdftoppm`。当前 PDF 文本处理已改为进程内实现，不再依赖 `pdftotext`；扫描 PDF 首先走 PDF 内嵌图片和视觉模型，外部 `pdftoppm` 只作为后备。

## 验证结果

### 已通过

- `go build -o /tmp/u-ai-server-dev ./cmd/server`
- `go test ./internal/artifact ./internal/devicemesh/coordination ./internal/devicemesh/business ./internal/chat`
- `front`: `vue-tsc --noEmit`
- `mobile_app`: `dart analyze lib` 无 error，存在约 922 条既有 warning/info

此前还通过过以下定向验证，但它们不是本轮最新改动后的最终结果：

- 后端附件、TLS 设备通道、实时媒体、渠道媒体、备份恢复测试
- 前端 23 项附件定向测试
- 移动端 66 项附件与聊天定向测试

### 未通过

`go test ./cmd/server` 当前失败：

1. `TestMeshChatAttachmentStorageValidatesReferenceAndChunks`
   - 原因：测试范围缺少 `RequestID`。
   - 状态：测试源码已补充 `RequestID`，尚未重新执行。

2. `TestOwnedRealtimeRealTLSRejectsReplayForeignCoreOriginAndSavesEachMode`
   - 原因：`newThreeCoreFixture` 未初始化 `services.Artifact`，测试在第 187 行直接访问空指针。
   - 状态：尚未修复测试夹具。

### 尚未执行

- 票据接口新增后的前端测试。
- 流式录音、媒体票据和下载改动后的移动端定向测试。
- 两阶段引用恢复器和媒体票据的专门单元测试。
- 真实设备上传、历史重读和跨设备读取。
- 权限撤销、上传中断、进程崩溃窗口和大文件 RSS 峰值验证。
- PDF、DOCX、XLSX、PPTX、扫描 PDF、图片和未知二进制的真实端到端处理。
- 完整项目重启及健康接口复核。

## 已知风险

1. `cmd/server` 测试夹具缺少附件运行时，必须补齐后再执行聚合测试。
2. 恢复器依赖消息数据所有者和角色当前可访问。权限撤销期间会保留待对账记录，安全但可能暂时阻塞附件删除。
3. PDFCPU 内嵌图片抽取可能受复杂 PDF、加密 PDF、异常颜色空间或不支持图片格式影响；失败会保留完整错误并返回给 Agent。
4. DOCX/XLSX/PPTX 当前提取的是文本和表格值，不承诺复杂排版、批注、嵌入对象和图表视觉还原。
5. 移动端下载依赖系统浏览器或下载器处理 HTTP 响应，真机文件名、取消和错误展示尚未验证。
6. 媒体票据使用进程内 HMAC 密钥，核心重启后旧票据立即失效；客户端需重新申请。
7. Provider Base64 分支仍存在，需要产品层决定是否彻底删除该输入协议。
8. 当前工作区存在大量其他未提交变更，后续测试失败可能来自无关模块。

## 下一步

1. 在 `backend/cmd/server/mesh_three_core_business_test.go` 的 `newThreeCoreFixture` 中按生产路径构建并注入 `ArtifactRuntime`，设置正确的存储设备身份。
2. 重跑：

   ```bash
   cd backend
   go test ./cmd/server -run 'TestMeshChatAttachment|TestOwnedRealtimeRealTLS'
   ```

3. 增加两阶段引用恢复测试：
   - pending 后消息存在，自动提交
   - pending 后消息不存在，自动释放
   - 消息已删除，自动移除 confirmed 引用
   - 摘要或大小不一致，保留记录并返回完整原始错误
4. 增加媒体票据测试：
   - 登录后签发
   - 短链过期
   - 篡改 ticket
   - Range 返回 206
   - 权限撤销后读取失败且保留原始错误
5. 更新前端 `conversationMedia.test.ts`，覆盖 `/content` 转 `/ticket` 和签名 URL 流式预览。
6. 重跑前端和移动端定向测试，再启动完整项目执行健康检查。
7. 进行真实端到端验证：
   - 任意二进制
   - 大于 1 MiB 文件
   - PDF、DOCX、XLSX、扫描 PDF
   - 上传中断与恢复
   - 历史消息重读
   - 跨设备分块读取
   - 跨会话、角色和设备拒绝
   - 权限撤销
   - 大文件内存峰值
8. 后端改动完成后按项目规则重启完整服务，检查前端、Core、Qdrant、SurrealDB、插件宿主、`/api/health`、`/readyz` 和实际附件链路。

## 禁止事项

- 禁止 reset、checkout 或覆盖现有工作区。
- 禁止管理端改动。
- 禁止 Git 拉取、提交或推送，除非用户明确要求。
- 禁止修改编译后产物。
- 禁止直接使用旧 Runtime Package 构建手机端。
- 未明确要求时，不要额外构建 APK。
- 前端热更新修改不得导致完整项目重启。
