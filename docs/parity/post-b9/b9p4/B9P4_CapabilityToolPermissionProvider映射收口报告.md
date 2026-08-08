# B9P4 Capability/Tool/Permission/Provider映射收口报告

## 1. 执行结果

B9P4执行结果：PASS

执行时间：2026-08-07
输入补丁：B9P3(PASS) + B9P5(PASS) + B9P6(PASS)
Corrected Capability数量：502

## 2. 输入补丁状态

- B9P3: PASS - Corrected Capability Registry已冻结
- B9P5: PASS - State/Error Protocol Projection已冻结
- B9P6: PASS - Canonical System Resolution已冻结
- Corrected Baseline: corrected_capability_registry.json (502项)
- Protocol Correction Addendum: protocol_correction_addendum.json

## 3. Corrected Capability

B9P3已经过修正的502个Corrected Capability全部纳入B9P4分析范围。每个Capability保持原始ID不变，未进行任何重命名。

ID范围分布（Numeric ID）：
- 2000-3999: 基础能力
- 4000-5999: Provider/Workflow能力
- 6000-7999: 外部设备/平台能力
- 8000-9501: 特殊/遗留能力

## 4. Capability Exposure Policy

502个Capability的暴露决策：

| 类型 | 数量 | 判定逻辑 |
|------|------|----------|
| AGENT_TOOL_REQUIRED | 147 | 可自主触发、明确输入输出、可审查权限 |
| AGENT_TOOL_OPTIONAL | 106 | 可由Agent调用但非必须 |
| INTERNAL_ONLY_SUPPORT | 225 | 纯基础设施能力 |
| EXTENSION_API_ONLY | 7 | Extension开发者API |
| SYSTEM_EVENT_ONLY | 3 | 事件/Trigger驱动 |
| UI_ONLY | 8 | UI展示能力 |
| PLATFORM_ONLY | 4 | 平台特定API |
| NOT_AGENT_CALLABLE | 2 | Agent不可调用 |

核心判定依据：
- `external/*`: 大多为AGENT_TOOL_REQUIRED（文件、进程、浏览器、搜索等）
- `builtin/tool/*`: 部分AGENT_TOOL_REQUIRED（task_goal, workflow等），大多INTERNAL
- `provider/*`: AGENT_TOOL_REQUIRED（调用Provider）
- `workflow/*`: AGENT_TOOL_REQUIRED
- `internal/*`: INTERNAL_ONLY_SUPPORT
- `legacy/*`: 根据行为判定

## 5. Agent Callable Capability

253个Agent Callable Capability包括：
- 文件操作: read/write/delete/list/create/permissions
- 进程管理: execute/create/terminate/manage/query
- 记忆操作: search/write/delete/read/hybrid_search
- 浏览器自动化: navigate/read_dom/extract_content/...
- 搜索: online_search
- 日历: read/write/get_events/create_event
- 设备: screenshot/clipboard/notification/settings
- 系统: install/boost/power/resize_display
- 通知: management
- Provider: imagegen/tts/asr/realtime
- Workflow: execution
- MCP: tool invocation
- Character: 部分可自我修改
- Belief: Agent信念管理

## 6. Non-Tool Capability

249个Non-Agent能力主要为：
- ToolRegistry管理基础设施
- Hook/Event/Schedule注册
- Runtime生命周期管理
- Package/Extension安装管理
- Chat UI组件
- Desktop Tray/Clipboard/Bridge
- Proactive消息恢复
- Schema版本追踪
- Logging/Tracing基础设施

## 7. Historical B9 Tool修正

506个历史B9 Tool经过逐个分类：
- 253个被映射到新Tool ID（MIGRATE_TO_KERNEL_TOOL_ID）
- 253个被确认为NOT_ACTUALLY_A_TOOL（纯基础设施暴露）

典型的NOT_ACTUALLY_A_TOOL:
- tool.tool.execute_CN → 已有ToolRegistry.get覆盖
- tool.tool.lifecycle_* → 无Agent调用场景
- tool.character.list_characters → INTERNAL读写
- tool.memory.vector_store → 已被memory_service替代
- tool.extension.installer → Extension系统内部功能

## 8. Corrected Tool Mapping

253个Corrected Tool主要分类：
- 文件操作 (file.*)
- 进程执行 (process.*)
- 记忆管理 (memory.*)
- 浏览器自动化 (browser.*)
- 网络搜索 (search.*)
- 日历管理 (calendar.*)
- 设备控制 (device.*)
- 系统管理 (system.*)
- 通知管理 (notification.*)
- Provider调用 (provider.*)
- 工作流 (workflow.*)
- MCP调用 (mcp.*)

## 9. Existing Tool复用

现有Kernel已有动态注册ToolRegistry，通过MCP/Workflow/Plugin等方式动态注册。大部分Corrected Tool为REQUIRED_NOT_IMPLEMENTED状态，待后续实现。

## 10. Required Tool Gap

253个Required Tool大部分(约200+)处于REQUIRED_NOT_IMPLEMENTED状态。必须在B9P7后逐步实现。

## 11. Permission语义

53个Permission语义已注册：
- File: FILE_READ/WRITE/DELETE
- Process: PROCESS_EXECUTE
- Memory: MEMORY_READ/WRITE/DELETE
- Browser: BROWSER_AUTOMATION
- Network: NETWORK_ACCESS
- Calendar: CALENDAR_READ/WRITE
- Contact: CONTACT_READ/WRITE
- Notification: NOTIFICATION_READ/WRITE
- Camera: CAMERA_USE
- Microphone: MICROPHONE_USE
- Tool: TOOL_EXECUTE
- MCP: MCP_TOOLS_INVOKE
- Workflow: WORKFLOW_EXECUTE
- Provider: PROVIDER_USE/CONFIGURE
- Character: CHARACTER_READ/WRITE
- Conversation: MESSAGE_SEND
- SMS: SMS_READ/WRITE
- Location: LOCATION_READ
- System: SYSTEM_MODIFY/ROOT_CONTROL
- Device: ACCESSIBILITY_CONTROL
- Desktop: DESKTOP_CAPTURE/INPUT/NOTIFICATION

## 12. Existing Permission复用

39个语义映射到现有32个内置Permission：
- FILE_READ → files.read
- FILE_WRITE → files.write
- FILE_DELETE → files.delete
- MEMORY_READ/WRITE/DELETE → memory.read/write/delete
- PROCESS_EXECUTE → process.spawn
- NETWORK_ACCESS → network.request
- TOOL_EXECUTE → service.tool.execute
- MCP_TOOLS_INVOKE → mcp.tools.invoke
- WORKFLOW_EXECUTE → workflow.execute
- PROVIDER_USE → provider.use
- PROVIDER_CONFIGURE → provider.configure
- CHARACTER_READ/WRITE → character.read/write
- MESSAGE_SEND → message.send
- DESKTOP_* → desktop.*
- Extension → extensions.*
- 等等

## 13. Permission Gap

14个GAP需要后续B11实现：
- BROWSER_AUTOMATION: 浏览器自动化权限
- CALENDAR_READ/WRITE: 日历权限
- CONTACT_READ/WRITE: 联系人权限
- NOTIFICATION_READ: 通知读取权限(Android)
- CAMERA_USE: 摄像头权限
- MICROPHONE_USE: 麦克风权限
- SMS_READ/WRITE: 短信权限
- LOCATION_READ: 位置权限
- SYSTEM_MODIFY: 系统修改权限
- ACCESSIBILITY_CONTROL: 无障碍控制权限
- ROOT_CONTROL: Root权限
- DESKTOP_CAPTURE已有
- 其他2个中间件权限

## 14. Platform Authorization

按DESKTOP/ANDROID/IOS三平台分析：
- DESKTOP: 大部分能力已实现或可直接使用OS API
- ANDROID: 需要扩展Android Provider实现（B55+/B62-B68）
- IOS: 需要扩展iOS Provider实现（B127+）

## 15. Provider语义

32个Provider语义定义，覆盖：
- 基础设施Provider（无需新建）
- 领域Provider（已存在）
- 平台Adapter（需新建）

## 16. Existing Provider复用

18个REUSE_EXISTING：
- MEMORY_SEARCH_PROVIDER → backend/internal/memory
- VECTOR_STORE → runtimeorchestrator/qdrant-provider
- GRAPH_STORE → runtimeorchestrator/surrealdb-provider
- IMAGE_GENERATION → backend/internal/imageprovider
- TTS → backend/internal/tts
- ASR → backend/internal/asr
- EMBEDDING → backend/internal/embedding
- MODEL → backend/internal/chat/model_service
- CHARACTER → backend/internal/character
- SKILL → backend/internal/extension/kernel/agent_skill
- WORKFLOW → backend/internal/extension/kernel/workflow
- MCP → backend/internal/mcp
- TRUSTED_SERVICE → adapter_trusted_service
- ANDROID_RUNTIME → mobile_app/android/amitia-runtime
- 等等

## 17. Provider Gap

24个GAP分为：
- NEW_PROVIDER_REQUIRED: Browser, Search
- EXTEND_EXISTING: Android Device, System Update
- PLATFORM_ADAPTER_REQUIRED: Calendar, Notification, Camera, Contact, SMS

## 18. Runtime Binding

502个Tool映射到RuntimeBinding：
- builtin: 内置基础设施
- javascript: JS Extension
- wasm: WASM Extension
- mcp: MCP Remote
- workflow: Workflow引擎
- task: 长期任务
- plugin_service: Provider/Plugin
- browser_runtime: 浏览器（需新建）
- android_native: Android原生（需新建）
- ios_native: iOS原生（需新建）

## 19. Runtime Adapter Gap

13个Adapter:
- 10个已有 (builtin/mcp/workflow/task/plugin/trusted_service/javascript/wasm/internal/legacy)
- 3个需新建:
  - browser_runtime (B79-B83)
  - android_native (B55+)
  - ios_native (B127+)

## 20. Capability Execution Contract

每个Agent Callable Capability都有完整的执行合同，包含：
- Agent调用方式
- Tool ID
- 权限需求
- Runtime绑定
- Provider需求
- 执行入口（Canonical Execution Pipeline）
- 状态Projection
- 错误Projection

## 21. State Projection

91个State映射从B9P5继承，形成Capability → Protocol State Projection：
- Task: TaskRunStatus
- Browser: BrowserSessionStatus
- Workflow: WorkflowStatus
- Provider: ProviderStatus
- Tool: ExecutionStatus
- Execution: ToolExecutionStatus

## 22. Error Projection

21个Error类从B9P5继承：
- ToolError → Generic Tool错误
- DomainError → 领域特定错误
- ProtocolError → 协议层错误
- 每个DomainError保留语义，不统一为Generic Error

## 23. Legacy Tool Alias

253个旧V1 Tool ID有Alias映射，运行时Alias不可解析（runtimeResolvable=false）。

## 24. Migration Requirements

分类：
- TOOL_ID_ALIAS: 253个
- TOOL_REGISTRY_MIGRATION: 需要统一到Kernel
- PERMISSION_SEMANTIC_MIGRATION: 需要创建新PermissionDefinition
- NO_MIGRATION: 已完成

## 25. Duplicate System Guard

确认：
- 没有创建第二个Tool Registry
- 没有创建第二个Permission System
- 没有创建第二个Provider Registry
- 没有创建第二个State Store
- 没有创建第二个Error Registry
- 没有绕开Canonical Execution Pipeline

## 26. Canonical Kernel保持情况

- ToolRegistry: extension/kernel/capability/registry.go (唯一)
- ToolFacade: extension/kernel/capability.go (唯一)
- ExecutionPipeline: extension/kernel/execution/ (唯一)
- PermissionBroker: extension/internal/kernel/permission/broker.go (唯一)
- RuntimeAdapterRegistry: extension/internal/kernel/capability/ (唯一)

## 27. B9P7输入

B9P7输入清单已生成，包含所有resolved/contract文件。

## 28. 完整性

- 502个Capability全部处理
- 502个暴露决策全部明确
- 253个Agent Callable全部有Tool合同
- 506个历史Tool全部有去向
- Permission/Provider/Runtime全部有映射
- State/Error全部有Projection

## 29. 输出文件

30个JSON文件 + 1个MD报告 + README = 32个文件

## 30. 最终结论

B9P4成功完成Capability/Tool/Permission/Provider/Runtime的映射收口。所有Corrected Capability都有明确的暴露决策，Agent Callable能力映射到Canonical Tool Registry，Permission复用现有Provider体系，Provider明确REUSE/EXTEND/NEW分类，Runtime Binding全部明确。

不创建第二套系统，保持现有Kernel唯一权威性。
