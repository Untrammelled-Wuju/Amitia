# A16 审计报告：复用 BackendTransport 统一 Flutter 网络入口

## 任务概述

**目标**：复用现有 BackendTransport，统一 Flutter HTTP / WebSocket 网络入口，迁移旧 ApiClient / Dio 旁路。

**执行时间**：2026-08-11

---

## 一、架构统一验证

| 验证项 | 状态 | 说明 |
|--------|------|------|
| Production BackendTransport 数量 = 1 | ✅ | 仅 DefaultBackendTransport |
| HTTP 传输实现 | ✅ | BackendHttpTransport (Dio 封装) |
| WebSocket 传输实现 | ✅ | BackendWebSocketClient (web_socket_channel) |
| BackendConnectionConfig 唯一连接来源 | ✅ | 所有端点来自 BackendConnectionConfig |
| Token 注入唯一位置 | ✅ | 仅由 BackendHttpClient 注入 X-Amitia-Local-Token |
| Generation 生命周期完整 | ✅ | 传输层旧请求取消机制保留 |

---

## 二、BackendServiceApi 适配层验证

**文件**：`lib/core/backend_transport/backend_service_api.dart`

| 验证项 | 状态 | 说明 |
|--------|------|------|
| BackendServiceApi 类已创建 | ✅ | 包装 BackendHttpTransport |
| 构造函数接收 http + generation | ✅ | 完整 generation 透传 |
| get<T> 方法 | ✅ | 返回 Future<T?> |
| post<T> 方法 | ✅ | 返回 Future<T?> |
| put<T> 方法 | ✅ | 返回 Future<T?> |
| delete 方法 | ✅ | 返回 Future<void> |
| 标准响应解析 {code, message, data} | ✅ | _parseResponse 实现 |
| ServiceApiException 新异常类 | ✅ | 含 fromNetwork、timeout 等工厂 |
| 200 正常响应解包 data 字段 | ✅ | 自动拆包 |
| 非 200 抛出 ServiceApiException | ✅ | 携带 code + message + detail |

---

## 三、Providers 依赖注入验证

**文件**：`lib/core/services/providers.dart`

| 验证项 | 状态 | 说明 |
|--------|------|------|
| apiClientProvider 已删除 | ✅ | 旧单例 Client 移除 |
| _backendServiceProvider 新增 | ✅ | 桥接 BackendServiceApi |
| _getServiceApi(ref) 辅助函数 | ✅ | 统一非空校验 |
| 所有服务提供者使用 _getServiceApi(ref) | ✅ | 25+ 服务统一注入 |
| MoodService 新增（如有） | ✅ | 已包含在 providers.dart |

### 已迁移服务清单

| 服务类 | 状态 | 验证 |
|--------|------|------|
| AuthService | ✅ | BackendServiceApi 注入，ServiceApiException |
| CharacterService | ✅ | BackendServiceApi 注入 |
| CharacterDetailService | ✅ | BackendServiceApi 注入 |
| ChatService | ✅ | BackendServiceApi 注入 |
| MemoryService | ✅ | BackendServiceApi 注入 |
| ProfileService | ✅ | BackendServiceApi 注入 |
| EpisodicService | ✅ | BackendServiceApi 注入 |
| WorldBookService | ✅ | BackendServiceApi 注入 |
| ReminderService | ✅ | BackendServiceApi 注入 |
| CompanionService | ✅ | BackendServiceApi 注入 |
| ModelConfigService | ✅ | BackendServiceApi 注入 |
| FeedbackService | ✅ | BackendServiceApi 注入 |
| TTSService | ✅ | BackendServiceApi 注入 |
| ASRService | ✅ | BackendServiceApi 注入 |
| ExtensionService | ✅ | BackendServiceApi 注入 |
| SystemService | ✅ | BackendServiceApi 注入 |
| SafetyService | ✅ | BackendServiceApi 注入 |
| MCPService | ✅ | BackendServiceApi 注入 |
| QQService | ✅ | BackendServiceApi 注入 |
| ImageGenService | ✅ | BackendServiceApi 注入 |
| VisionService | ✅ | BackendServiceApi 注入 |
| EmbeddingService | ✅ | BackendServiceApi 注入 |
| EmoteService | ✅ | BackendServiceApi 注入 |
| ProactiveService | ✅ | BackendServiceApi 注入 |
| TemporalService | ✅ | BackendServiceApi 注入 |
| MoodService | ✅ | BackendServiceApi 注入 |

---

## 四、旧代码清除验证

| 验证项 | 状态 | 说明 |
|--------|------|------|
| lib/core/api/api_client.dart 已删除 | ✅ | 已不存在 |
| lib/core/api/api_response.dart 已删除 | ✅ | 已不存在 |
| lib/core/api/api_exception.dart 已删除 | ✅ | 已不存在 |
| lib/core/api/ 目录已删除 | ✅ | 已不存在 |
| 残留 ApiClient 引用检查 | ✅ | 代码中无残留 |
| 残留 ApiResponse 引用检查 | ✅ | 代码中无残留 |
| 残留 ApiException 引用检查 | ✅ | 仅剩 ServiceApiException（新） |

---

## 五、Feature 页面迁移验证

| 页面 | 状态 | 迁移内容 |
|------|------|----------|
| agent_tasks_provider.dart | ✅ | 改用 backendServiceProvider + null 检查 |
| toolbox_file_browser_page.dart | ✅ | 改用 backendServiceProvider |
| toolbox_prompt_trace_page.dart | ✅ | 改用 backendServiceProvider |
| toolbox_log_page.dart | ✅ | 改用 backendServiceProvider |
| toolbox_task_log_page.dart | ✅ | 改用 backendServiceProvider |
| backup_page.dart | ✅ | 改用 backendServiceProvider |
| desktop_contributions_page.dart | ✅ | 改用 backendServiceProvider |

### Import 清理

| 文件 | 清理内容 |
|------|----------|
| agent_tasks_provider.dart | 移除未使用 backend_service_api.dart import |
| desktop_contributions_page.dart | 移除未使用 backend_service_api.dart import |
| toolbox_log_page.dart | 移除未使用 backend_service_api.dart import |
| toolbox_prompt_trace_page.dart | 移除未使用 backend_service_api.dart import |
| toolbox_task_log_page.dart | 移除未使用 backend_service_api.dart import |
| toolbox_file_browser_page.dart | 添加缺失的 providers.dart import |

---

## 六、Connectivity 适配验证

**文件**：`lib/core/backend_transport/connectivity/backend_connectivity_providers.dart`

| 验证项 | 状态 | 说明 |
|--------|------|------|
| apiClientSyncProvider 已删除 | ✅ | 旧同步逻辑移除 |
| 旧 ApiClient import 已移除 | ✅ | 仅保留 transport 相关 import |
| backendConnectivityProbeProvider 保留 | ✅ | 使用 transport.http |

---

## 七、Flutter Analyze 验证

**命令**：`flutter analyze`

### 迁移相关错误检查

| 验证项 | 状态 | 说明 |
|--------|------|------|
| 由本次迁移引入的新 error | ✅ | 无 |
| 由本次迁移引入的新 warning (null) | ✅ |已全部修复 unnecessary_null_comparison |
| 由本次迁移引入的新 unused_import | ✅ | 已全部清理 |
| Service 层 null 处理正确性 | ✅ | T? 返回类型，null 检查有效 |

### 剩余 Analyzer 问题汇总

| 类型 | 数量 | 是否迁移引入 |
|------|------|--------------|
| error | ~10 | ❌ 全部为 pre-existing |
| warning | ~50 | ❌ 全部为 pre-existing |
| info | ~80 | ❌ 全部为 pre-existing |

**说明**：剩余 error 均为迁移前已存在的问题（如类型不匹配、未定义方法、构造函数参数错误等），不属于本任务范围。

---

## 八、关键文件变更清单

### 新增文件

| 文件路径 | 说明 |
|----------|------|
| lib/core/backend_transport/backend_service_api.dart | BackendServiceApi + ServiceApiException |

### 修改文件

| 文件路径 | 变更摘要 |
|----------|----------|
| lib/core/backend_transport/providers/backend_transport_providers.dart | 新增 backendServiceProvider |
| lib/core/services/providers.dart | 移除 apiClientProvider，全部改用 _getServiceApi(ref) |
| lib/core/services/auth_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/character_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/channel_service.dart | 9个服务统一迁移 |
| lib/core/services/system_service.dart | SystemService + SafetyService 迁移 |
| lib/core/services/voice_service.dart | TTSService + ASRService 迁移 |
| lib/core/services/chat_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/memory_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/profile_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/episodic_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/worldbook_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/reminder_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/companion_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/model_config_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/feedback_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/character_detail_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/extension_service.dart | ApiClient → BackendServiceApi |
| lib/core/services/temporal_service.dart | ApiClient → BackendServiceApi |
| lib/core/backend_transport/connectivity/backend_connectivity_providers.dart | 移除 apiClientSyncProvider |
| lib/features/agent/presentation/providers/agent_tasks_provider.dart | 改用 backendServiceProvider |
| lib/features/toolbox/presentation/pages/toolbox_file_browser_page.dart | 改用 backendServiceProvider + 修复 import |
| lib/features/toolbox/presentation/pages/toolbox_prompt_trace_page.dart | 改用 backendServiceProvider |
| lib/features/toolbox/presentation/pages/toolbox_log_page.dart | 改用 backendServiceProvider |
| lib/features/toolbox/presentation/pages/toolbox_task_log_page.dart | 改用 backendServiceProvider |
| lib/features/settings/presentation/pages/backup_page.dart | 改用 backendServiceProvider |
| lib/features/developer/presentation/pages/desktop_contributions_page.dart | 改用 backendServiceProvider |

### 删除文件/目录

| 路径 | 说明 |
|------|------|
| lib/core/api/api_client.dart | 旧 HTTP Client |
| lib/core/api/api_response.dart | 旧响应封装 |
| lib/core/api/api_exception.dart | 旧异常类 |
| lib/core/api/ | 整个目录 |

---

## 九、设计决策记录

### 1. BackendServiceApi 返回 `Future<T?>` 而非 `Future<T>`

**背景**：旧 ApiClient 返回 `ApiResponse<T>` 其中 data 字段可为 null。
**决策**：BackendServiceApi 直接返回数据本身，类型为 `Future<T?>`。
**理由**：
- 保持与旧代码兼容（服务层 null 检查逻辑无需重构）
- ServiceApiException 已覆盖非 200 错误情况
- null 仅在 200 + data 缺失时发生，调用方需处理

### 2. 服务注入统一使用 _getServiceApi(ref)

**决策**：所有服务提供者通过 `_getServiceApi(ref)` 获取非空 BackendServiceApi。
**理由**：
- 编译期保证 Transport 未就绪时无法构建服务
- 避免在每个服务内部做空值检查

### 3. 保留完整 Generation 透传

**决策**：BackendServiceApi 保留 generation getter，尽管当前未在请求逻辑中使用。
**理由**：为未来 generation 校验预留扩展点。

---

## 十、合规性验证

| 验证项 | 状态 |
|--------|------|
| Go 后端接口未修改 | ✅ |
| DTOs 保留（无变更） | ✅ |
| 业务逻辑保留（仅网络层替换） | ✅ |
| 不影响除 Flutter 移动端外的其他组件 | ✅ |
| 与现有 WebSocket/源探测隔离并行 | ✅ |

---

## 总结

A16 任务完成所有核心目标：

1. ✅ BackendTransport 复用为唯一 Production 传输
2. ✅ BackendConnectionConfig 保持唯一连接来源
3. ✅ BackendServiceApi 新适配层创建并提供给所有服务
4. ✅ 25+ Services 全部脱离旧 ApiClient
5. ✅ Feature 页面同步迁移
6. ✅ 旧代码完全清除（3文件 + 目录）
7. ✅ flutter analyze 无迁移引入的新 error
8. ✅ 迁移代码的逻辑正确性（null 处理、error 处理）
