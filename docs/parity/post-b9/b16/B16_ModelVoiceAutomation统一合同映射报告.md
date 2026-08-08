# B16 Model / Voice / Automation 统一合同映射报告

## 1. 执行结果

**状态**: PASS_NO_CODE_CHANGE

B16 是合同映射步骤，核心目标是建立现有 Model/Voice/Automation 系统与 B9P8 冻结的统一 Capability/Provider/Runtime 合同之间的映射关系。执行过程中发现现有合同体系已经满足映射要求，所有真实功能缺口均通过 deferred_gap_inventory.json 记录并分配给后续步骤（B105-B110、B111-B114、B115-B117、B17）。

无源码修改，无 go.mod/go.sum 变更，无数据库变更。

## 2. B9P8 输入

- **Status**: PASS (b9p8_status.json confirmed)
- **Resolved Manifest**: resolved_post_b9_manifest.json loaded
- **B16 Construction Mode**: REUSE (primary) + EXTEND (secondary)
- **Frozen Manifests Used**:
  - final_capability_manifest.json
  - final_tool_manifest.json
  - final_permission_manifest.json
  - final_provider_manifest.json
  - final_runtime_manifest.json
  - final_state_projection_manifest.json
  - final_error_projection_manifest.json
  - final_canonical_system_manifest.json
  - final_step_reuse_matrix.json
  - final_architecture_guard.json
  - final_execution_guard.json
- **Architecture Guard**: 20 条 NO_SECOND 规则全部满足

## 3. Construction Mode

| Mode | Description |
|------|-------------|
| REUSE | 复用现有 chat/model_service, tts, asr, workflow, task_runtime, event, hook, schedule |
| EXTEND | 合同层映射扩展 (无代码修改，仅输出映射文件) |

## 4. Model 现有架构

### 核心服务
- **Config Service**: backend/internal/chat/service.go (Service 接口)
- **Config Model**: backend/internal/chat/model.go (ModelConfig 结构体)
- **Repository**: backend/internal/chat/repository.go
- **LLM Client**: backend/internal/chat/llm_client.go (protocolForApiType 分发)
- **Error Model**: backend/internal/chat/model_error.go (TextModelCallError)

### 数据库表
| 表名 | 用途 | 激活字段 |
|------|------|---------|
| model_configs | 聊天模型主配置 | is_active |
| vision_configs | 视觉模型配置 | is_active |
| embedding_configs | 嵌入模型配置 | is_active |
| image_gen_configs | 图像生成配置 | is_active |
| model_scenario_routes | 场景路由 | - |

### Provider 体系
- **协议分发**: protocolForApiType switch -> openai/ollama/anthropic/gemini
- **Chat Provider 数量**: 19 (含 OpenAI, DeepSeek, 通义, 智谱, Ollama, Anthropic, Gemini 等)
- **Vision Provider 数量**: 6
- **Provider 注册**: 硬编码在 chat/repository.go:ListProviders，无运行时插件注册

### 现有能力
- LLM 聊天生成 (同步，非流式)
- 视觉模型理解 (豆包 ARK Responses API)
- Embedding 嵌入 (volcengine/openai)
- 模型自动检测
- 场景路由

## 5. Model Capability

从 B9P8 合同清单提取的 Model 相关 Capability：

| Capability | 状态 | 说明 |
|-----------|------|------|
| text_generation | ALREADY_SUPPORTED | llm_client.go 19 Provider |
| vision_understanding | ALREADY_SUPPORTED | vision_service.go 6 Provider |
| embedding | ALREADY_SUPPORTED | embedding.go |
| image_generation | PARTIALLY_SUPPORTED | 配置表存在，Service 未完整 |
| streaming | MISSING | stream:false 硬编码 |
| responses_api | MISSING | 仅视觉模型用 ARK 端点 |
| model_selection | ALREADY_SUPPORTED | ModelConfig.is_active |
| scenario_routing | ALREADY_SUPPORTED | model_scenario_routes |

## 6. Model Provider Mapping

| Classification | Count | 说明 |
|---------------|-------|------|
| REUSE_EXISTING_PROVIDER | 4 | chat/vision/embedding/scenario |
| EXTEND_EXISTING_PROVIDER | 1 | image_generation |
| NEW_PROVIDER_REQUIRED | 5 | streaming/responses_api/mnn/llama_cpp/local_embedding |
| NOT_APPLICABLE | 1 | scenario_routing |

## 7. Model Runtime Binding

- **现有 Binding**: host_internal (所有 LLM/WASM 调用在 host backend 内完成)
- **合同要求**: B16 requiredRuntimeBindings 含 'model'
- **Gap**: 'model' binding 未在 final_runtime_manifest 中定义
- **Resolution**: 交给 B17 (Manifest v2) 声明 'model' runtime binding

## 8. Model Permission/State/Error

### Permission
- **合同要求**: TOOL_EXECUTE + MODEL_ACCESS
- **现有**: TOOL_EXECUTE (通用 Tool 权限)
- **Gap**: MODEL_ACCESS 未在 Permission Manifest 中定义 → B11 负责

### State
- **Domain State**: ModelConfig.IsActive, LastTestStatus
- **Protocol Projection**: 无独立请求级状态跟踪 (仅诊断级)

### Error
- **Domain Owner**: backend/internal/chat/model_error.go (TextModelCallError)
- **Protocol Class**: chat_model (B9P8 final_error_projection_manifest errorDomains)

## 9. Model Deferred Gap

| Gap ID | 类型 | Future Step |
|--------|------|-------------|
| streaming | MISSING | B111 |
| responses_api | MISSING | B111 |
| mnn_local_inference | MISSING | B112 |
| llama_cpp | MISSING | B112 |
| local_embedding | MISSING | B113 |
| MODEL_ACCESS permission | MISSING | B11 |
| builtin/model/* 命名空间 | MISSING | B17 |
| model runtime binding | MISSING | B17 |

## 10. Voice 现有架构

### 顶层结构
| Domain | 目录 | 配置表 | Provider 数 |
|--------|------|--------|------------|
| ASR | backend/internal/asr/ | asr_configs | 4 |
| TTS | backend/internal/tts/ | tts_configs | 8 |
| Realtime | backend/internal/realtime/ | 无独立表 | 1 (火山引擎) |
| Interaction | backend/interaction/ | 无 | - |

### Provider 注册机制
- ASR Provider 硬编码在 asr/repository.go:ListProviders
- TTS Provider 硬编码在 tts/repository.go:ListProviders
- Dispatch: 各自 switch(apiType) 分发
- 无插件式运行时注册

## 11. ASR

- **Canonical**: backend/internal/asr/
- **Config**: AsrConfig (id, name, api_type, api_key, base_url, resource_id, is_active)
- **Provider**: volcengine, openai, azure, aliyun
- **Dispatch**: asr.go:SubmitTask switch(apiType)
- **Routes**: /api/asr/submit, /api/asr/query
- **State**: TranscriptionStatus (interim/final/cancel)
- **Errors**: ErrVoiceSessionNotFound, ErrVoiceTurnCancelled, ErrVoiceBusy

## 12. TTS

- **Canonical**: backend/internal/tts/
- **Config**: TtsConfig (含 emotion, speed, pitch, volume, voice_type 等)
- **Provider**: volcengine, openai, azure, edge, elevenlabs, minimax, aliyun, cosyvoice
- **Dispatch**: engine.go:Synthesize switch(apiType)
- **Features**: Voice Clone (V1/V3), Voice Catalog (104 presets), Disk Cache (data/tts_cache/)
- **Routes**: /api/tts/synthesize, /api/tts/clone, /api/tts/voices

## 13. Realtime

- **Canonical**: backend/internal/realtime/
- **Architecture**: WebSocket proxy (Browser ↔ Volcengine Realtime)
- **Protocol**: 火山引擎二进制帧协议 (realtime/protocol.go)
- **State**: VoiceTurnState (listening/processing/responding/idle)
- **Note**: 单 Provider (Volcengine)，非独立 Provider 体系

## 14. Voice Capability Mapping

| Capability | Status | Provider |
|-----------|--------|----------|
| tts_synthesis | ALREADY_SUPPORTED | 8 providers |
| asr_transcription | ALREADY_SUPPORTED | 4 providers |
| voice_clone | ALREADY_SUPPORTED | 火山引擎 |
| voice_reply | ALREADY_SUPPORTED | tool_migration |
| realtime_conversation | ALREADY_SUPPORTED | 火山引擎 |
| voice_preset_management | PARTIALLY_SUPPORTED | catalog 存在，未注册 Tool |
| wake_word | MISSING | - |
| continuous_voice | MISSING | - |

## 15. Voice Provider Mapping

| Domain | Classification | Count |
|--------|---------------|-------|
| ASR | REUSE_EXISTING_PROVIDER | 4 |
| TTS | REUSE_EXISTING_PROVIDER | 8 |
| TTS voice_clone | REUSE_EXISTING_PROVIDER | 1 |
| TTS voice_preset | REUSE_EXISTING_PROVIDER | 1 |
| Realtime | REUSE_EXISTING_PROVIDER | 1 |
| Wake word | NEW_PROVIDER_REQUIRED | 1 |
| Continuous listening | NEW_PROVIDER_REQUIRED | 1 |
| Android capture | PLATFORM_ADAPTER | 1 |

## 16. Voice State/Error

### State
- **Interaction**: VoiceTurnState (listening/processing/responding/idle)
- **Realtime Transcription**: TranscriptionStatus (interim/final/cancel)
- **Channel Mode**: ChannelGroup (text/voice/all)
- **Note**: 无 SYNTHESIZING/BUFFERING/PLAYING 细分状态

### Error
- **Interaction errors**: ErrVoiceSessionNotFound/ErrVoiceTurnCancelled/ErrVoiceBusy
- **Reporting**: modelerror.Event (ModelType=voice)
- **Protocol Projection**: interaction_voice (B9P8 errorDomains)

## 17. Voice Deferred Gap

| Gap ID | 类型 | Future Step |
|--------|------|-------------|
| builtin/tts/* Capability 命名空间 | MISSING | B17 |
| builtin/asr/* Capability 命名空间 | MISSING | B17 |
| wake_word | MISSING | B115 |
| continuous_voice_session | MISSING | B115 |
| voice_preset_management_tool | TOOL_REGISTRATION | B115 |
| android_audio_capture | PLATFORM_ADAPTER | B116 |

## 18. Automation 现有架构

所有 5 个 Automation 子系统均为 B9P8 final_canonical_system_manifest.json 中定义的 B9P8 Canonical System，已注册在 Extension Kernel Container 中。

| System | Canonical Path | Kernel Registration |
|--------|---------------|---------------------|
| Workflow Engine | backend/.../kernel/workflow/ | WorkflowRegistry/Executor/TriggerManager |
| Task Runtime | backend/.../kernel/task_runtime/ | TaskRuntimeService + TaskSupervisorFactory |
| Event System | backend/.../kernel/event/ | EventService |
| Hook System | backend/.../kernel/hook/ | HookPipeline |
| Schedule System | backend/.../kernel/schedule/ | ScheduleExecutor |

## 19. Workflow

- **Definition**: WorkflowDefinition (SchemaVersion, ID, ExtensionID, Nodes, Permissions, Scope)
- **Node**: WorkflowNode (ID, Type, DependsOn, TargetID, RuntimeBinding, Permissions)
- **Executor**: WorkflowExecutor (DAG 拓扑排序执行)
- **State**: WorkflowRun.RunStatus (running/succeeded/failed/cancelled/compensating/compensated)
- **Features**: Checkpoint, Compensation (Saga), Nested Workflow, Security Guard, Builtin Handlers
- **Runtime Adapter**: adapter_workflow (RuntimeTypeWorkflow)

## 20. TaskRuntime

- **Status**: TaskRunStatus (17 级严格状态机: created → queued → starting → running → checkpointing → pausing → paused → resuming → cancelling → cancelled → succeeded/failed/timed_out/recovery_required/manual_intervention)
- **Features**: ConcurrencyLimiter, CancelSignal, Checkpoint/Recovery, Progress tracking
- **Supervisor**: TaskSupervisorFactory 接入 runtime_supervisor 体系
- **Runtime Adapter**: adapter_task (RuntimeTypeTask)

## 21. Event

- **Pattern**: Outbox 可靠投递 (OutboxRepository)
- **Components**: Publisher, Dispatcher, SubscriptionRegistry, OrderingCoordinator, DeadLetter, Trace, LoopGuard, CircuitBreaker
- **State**: OutboxStatus (pending/dispatching/dispatched/failed/dead_letter/cancelled)
- **Delivery**: DeliveryStatus tracking per subscription

## 22. Hook

- **Pipeline**: Five phases (before/filter/transform/observe/after)
- **Components**: PointRegistry, ContributionStore, PermissionChecker, ScopeChecker, DependencyChecker, DepthGuard, PatchValidator
- **Hook Points**: 13 个注册点 (message.*, model.*, prompt.*, tool.*, workflow.*, extension.*)
- **Failure Policy**: Per-contribution configurable

## 23. Schedule

- **Triggers**: Cron, Interval, OneShot
- **Targets**: tool, workflow, task, runtime_handler
- **Policies**: Misfire (skip/fire_once/catch_up_limited/reschedule_from_now), Overlap (forbid/allow/replace/queue_one/skip_if_running)
- **Features**: LeaseManager (租约防并发), Quarantine (双调度器隔离), Recovery
- **Scanner**: ScheduleScanner 周期性触发

## 24. Automation Capability Mapping

| Capability | Status | Existing System |
|-----------|--------|----------------|
| workflow_definition | ALREADY_SUPPORTED | WORKFLOW_ENGINE |
| task_runtime_execution | ALREADY_SUPPORTED | TASK_RUNTIME |
| event_publish_subscribe | ALREADY_SUPPORTED | EVENT_SYSTEM |
| hook_registration | ALREADY_SUPPORTED | HOOK_SYSTEM |
| schedule_cron | ALREADY_SUPPORTED | SCHEDULE_SYSTEM |
| compensation_transaction | ALREADY_SUPPORTED | Workflow Compensation |
| circuit_breaker | ALREADY_SUPPORTED | 各子系统均有 CircuitBreaker |
| dead_letter_queue | ALREADY_SUPPORTED | Event DeadLetter |
| outbox_pattern | ALREADY_SUPPORTED | Event Outbox |
| nested_workflow | ALREADY_SUPPORTED | Workflow builtin handlers |
| execute_workflow (Agent Tool) | REQUIRED_NOT_IMPLEMENTED | Workflow 引擎完备 |
| register_task_goal (Agent Tool) | REQUIRED_NOT_IMPLEMENTED | Task 运行时完备 |
| execute_task_plan (Agent Tool) | REQUIRED_NOT_IMPLEMENTED | Task 运行时完备 |
| js_wasm_execution | ENGINE_MISSING | Adapter 存在，引擎缺失 |

## 25. Automation Permission/State/Error

### Permission
- **TOOL_EXECUTE**: B9P4 capability_tool_mapping 要求
- **Per-System**: WorkflowDefinition.Permissions, TaskRun.PermissionSnap, Hook Contribution, Schedule Contribution 各有权限声明
- **Broker**: Pipeline.PermissionChecker (hook), ScheduleExecutor.PermissionChecker

### State
- **Workflow**: WorkflowRun.RunStatus
- **Task**: TaskRunStatus (17 级)
- **Event**: OutboxStatus, DeliveryStatus
- **Hook**: Contribution Enabled flag
- **Schedule**: ScheduleRunStatus (14 级)
- **Note**: 每个 Domain 自有 State Store，无 Global AutomationStateStore

### Error
- **Workflow**: workflow/errors.go (9 errors)
- **Task**: task_runtime/errors.go (20+ TaskErrorCode)
- **Event**: event/errors.go
- **Hook**: hook/errors.go (30+ HookErrorCode)
- **Schedule**: schedule/errors.go

## 26. Automation Deferred Gap

| Gap ID | 类型 | Future Step |
|--------|------|-------------|
| execute_workflow Agent Tool | TOOL_REGISTRATION | B105 |
| register_task_goal Agent Tool | TOOL_REGISTRATION | B105 |
| execute_task_plan Agent Tool | TOOL_REGISTRATION | B105 |
| personality_trait_model Agent Tool | TOOL_REGISTRATION | B105 |
| belief_batch/belief_system Agent Tools | TOOL_REGISTRATION | B105 |
| js/wasm_execution engine | ENGINE_MISSING | B105-B110 |

## 27. Tool Exposure

| 类型 | 数量 | 说明 |
|------|------|------|
| EXISTING_AGENT_TOOL | 7 | voice_reply, schedule, get_current_time, memory/query, memory/summary, state/read_need, state/read_psyche |
| REQUIRED_AGENT_TOOL | 6 | execute_workflow, register_task_goal, execute_task_plan, manage_voice_preset, personality_trait_model, streaming_output |
| NOT_AGENT_CALLABLE | 4 | Workflow/Event internal, model config, tts/asr direct |
| EXTENSION_API | 1 | Hook System |
| SYSTEM_TRIGGER | 1 | Event System |
| USER_CALLABLE | 1 | Schedule (HTTP) |
| **错误曝光数量** | **0** | 无 Capability 被错误注册为 Tool |

## 28. Runtime Binding

| Domain | Existing Binding | Adapter |
|--------|-----------------|---------|
| Model | host_internal | adapter_internal |
| Voice | host_internal | adapter_internal |
| Workflow | workflow | adapter_workflow |
| Task | task | adapter_task |
| Event/Hook/Schedule | builtin (indirect) | ContainerBridge |

### Identified Gap
- 'model' runtime binding: B16 requiredRuntimeBindings 要求，未在 final_runtime_manifest 定义

## 29. Existing Provider 复用

### REUSE (9)
- chat Model 19 Provider
- vision 6 Provider
- embedding 2 Protocol
- tts 8 Provider
- asr 4 Provider
- realtime 1 Provider (volcengine)
- workflow engine
- task runtime
- event/hook/schedule 系统

### EXTEND (1)
- image_generation (配置表存在，Service 未完整)

### NEW REQUIRED (5)
- streaming
- responses_api
- mnn
- llama_cpp
- local_embedding

### NOT APPLICABLE (1)
- scenario_routing

## 30. 实际修改

**无源码修改。**

B16 为合同映射步骤，仅生成 docs/parity/post-b9/b16/ 目录下的 28 个 JSON 文件和本 Markdown 报告。

### 修改文件清单
- 0 个 Go 文件
- 0 个测试文件
- 0 个配置文件
- 0 个数据库迁移
- 0 个 go.mod / go.sum 变更

所有现有系统保持不变，向后 100% 兼容。

## 31. Backward Compatibility

| 检查项 | 结果 |
|--------|------|
| 现有 Model Config 不失效 | PASS |
| 现有 Provider ID 不失效 | PASS (19 chat + 5 vision 不变) |
| 现有语音 Provider 配置不失效 | PASS (8 tts + 4 asr 不变) |
| 现有 Workflow 定义不失效 | PASS |
| 现有 Schedule 不失效 | PASS |
| 现有 Task checkpoint 不失效 | PASS |
| voice_reply Tool 注册不变 | PASS |
| LLM Client 调用链路不变 | PASS |

## 32. Duplicate System Validation

| 系统 | 新增计数 |
|------|---------|
| ModelConfig2 | 0 |
| ModelRouter2 | 0 |
| ModelProviderRegistry2 | 0 |
| VoiceRuntime2 | 0 |
| VoiceProviderRegistry2 | 0 |
| WorkflowEngine2 | 0 |
| TaskRuntime2 | 0 |
| EventBus2 | 0 |
| HookSystem2 | 0 |
| Scheduler2 | 0 |
| PermissionSystem2 | 0 |
| ExecutionPipeline2 | 0 |
| StateStore2 | 0 |
| ErrorRegistry2 | 0 |

**验证结果**: PASS - 所有第二系统计数为 0

## 33. B17 输入

B17 (Manifest v2 / Lifecycle) 需补充的声明：

1. **New Capability Namespaces**: builtin/model/*, builtin/tts/*, builtin/asr/*
2. **New Runtime Binding**: 'model' binding
3. **New Permission Semantic**: MODEL_ACCESS
4. **Lifecycle Declarations**: Model Provider lifecycle (activate/deactivate/health check)
5. **Auto Contributions**: voice_reply Tool 贡献格式化

## 34. B105～B110 输入

Automation 真实功能缺口：

1. workflow.execute Agent Tool 注册
2. task.goal.register Agent Tool 注册
3. task.plan.execute Agent Tool 注册
4. character.personality.model Tool 注册
5. belief.batch / belief.system Tool 注册
6. JS/WASM 执行引擎实现

## 35. B111～B114 输入

Model 真实 Provider/功能 Gap：

1. **B111**: LLM streaming (SSE/WebSocket)
2. **B111**: OpenAI Responses API 协议
3. **B112**: MNN 本地推理 Provider
4. **B112**: llama.cpp 本地推理 Provider
5. **B113**: 本地嵌入模型 Provider
6. **B114**: Model 配置 Agent Tool (可选)

## 36. B115～B117 输入

Voice 真实功能 Gap：

1. **B115**: Wake word 唤醒词检测
2. **B115**: Continuous voice session 持续会话
3. **B115**: Voice preset management Agent Tool
4. **B116**: 新增 TTS Provider 可扩展点
5. **B116**: 新增 ASR Provider 可扩展点
6. **B116**: Android platform audio capture adapter

## 37. 测试

由于无代码修改，不新增/修改测试：

| 测试类别 | 结果 |
|---------|------|
| Model | PASS_NO_CODE_CHANGE |
| ASR | PASS_NO_CODE_CHANGE |
| TTS | PASS_NO_CODE_CHANGE |
| Realtime | PASS_NO_CODE_CHANGE |
| Workflow | PASS_NO_CODE_CHANGE |
| TaskRuntime | PASS_NO_CODE_CHANGE |
| Event | PASS_NO_CODE_CHANGE |
| Hook | PASS_NO_CODE_CHANGE |
| Schedule | PASS_NO_CODE_CHANGE |
| Kernel Regression | PASS_NO_CODE_CHANGE |
| gofmt | NO_CODE_MODIFIED |

## 38. 修改文件

无文件修改。所有现有 Model/Voice/Automation 系统保持不变。

## 39. 阻断项

无。

B9P8 PASS 确认。所有前置条件满足。
Architecture Guard 20 条规则全部满足。Execution Guard 所有禁止操作未触发。

## 40. 最终结论

1. **B16 仅复用/扩展现有系统**: 确认。未新建任何系统，所有现有 Model/Voice/Automation 系统通过本步骤建立合同映射。

2. **Model 配置和 Provider 体系保持唯一**: 确认。model_configs 表 + protocolForApiType 分发体系 + chat/repository.go:ListProviders 为唯一事实源。

3. **ASR/TTS/Realtime 保持原有 Canonical 体系**: 确认。asr.go/tts/engine.go/realtime/proxy.go 三个分发路径各自独立，无统一覆盖。

4. **Workflow/TaskRuntime/Event/Hook/Schedule 继续使用现有 Extension Kernel 实现**: 确认。所有 5 个 Canonical System 已注册在 ContainerBuilder 中。

5. **Capability/Provider/RuntimeBinding 全部完成映射**: 确认。domain_capability_gap_matrix.json 完成 15 项 Capability 映射，所有 unresolved = 0。

6. **State/Error 继续以 Domain 模型为唯一事实源**: 确认。无新增 GlobalStateStore / ErrorRegistry。

7. **没有建立任何第二套 Model/Voice/Automation 系统**: 确认。duplicate_system_validation.json 全部为 0。

8. **Model 真实能力缺口已正确交给 B111～B114**: 确认。8 个 Gap 已分配到 B111/B112/B113/B17。

9. **Voice 真实能力缺口已正确交给 B115～B117**: 确认。6 个 Gap 已分配到 B115/B116/B17。

10. **Automation 真实能力缺口已正确交给 B105～B110**: 确认。7 个 Gap 已分配到 B105/B105-B110。

11. **Manifest/Lifecycle 缺口已正确交给 B17**: 确认。B17_input_manifest.json 包含 7 项 Manifest v2 更新需求。

12. **允许进入 B17**: 确认。B16 满足所有 PASS 条件，可无缝进入 B17 执行。

---

**最终状态**: PASS_NO_CODE_CHANGE
**下一步**: B17