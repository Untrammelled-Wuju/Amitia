# Amitia A线43步 / B线154步 同步推进表 (Post-B9最终冻结版)

## 声明

本文件为B9P8冻结后的正式执行计划。

B1~B9: 历史阶段。
B9P1~B9P8: Post-B9架构收口阶段。
B10~B154: 必须遵守Post-B9 Architecture Guard。

## 执行优先级

1. B9P8 Resolved Post-B9 Manifest
2. B9P8 Final Architecture/Execution Guard
3. B9P7 Step Reuse Matrix
4. B9P4 Resolved Mapping
5. B9P6 Canonical System Resolution
6. B9P5 State/Error Ownership
7. B9P3 Capability ID Correction
8. B9P2 Corrected Parity Scope
9. 原B10~B154详细方案
10. 原同步推进表
11. B8/B9历史协议，仅用于追踪

## 批次一: B1~B9 (历史阶段 - 已完成)

| Step | 职责 | 状态 |
|------|------|------|
| B1~B9 | B9历史阶段 | COMPLETE (FROZEN) |

## 批次二: B9P1~B9P8 (Post-B9架构收口 - 已完成)

| Patch | 职责 | 状态 |
|-------|------|------|
| B9P1 | A14补证 + Post-B9源码锚点 | PASS |
| B9P2 | Parity分母净化 (506→502) | PASS |
| B9P3 | Capability ID修订 | PASS |
| B9P4 | Capability/Tool/Permission/Provider/Runtime映射 | PASS |
| B9P5 | State/Error收口 | PASS |
| B9P6 | Canonical System唯一性确认 | PASS |
| B9P7 | B10~B154复用矩阵修订 | PASS |
| B9P8 | 最终验收 + 冻结 + B10 Release Gate | PASS |

## 批次三: B10~B18 (Contract层, EXTEND+REUSE为主)

| Step | Corrected Responsibility | Construction Mode | Canonical Target | Forbidden Duplicate | Prerequisite |
|------|------------------------|-------------------|-----------------|---------------------|--------------|
| B10 | 对现有ToolDefinition/ExecutionPipeline执行Gap审计并补字段 | EXTEND+REUSE | ToolRegistry/ToolFacade/ExecutionPipeline | 禁止新建ToolRegistry/Schema/Result | B9P8 PASS |
| B11 | 给PermissionDefinitionRegistry补14个GAP | EXTEND+REUSE | PermissionBroker | 禁止新Permission系统 | B10 |
| B12 | 补Native Offload/Runtime合同 | EXTEND+ADAPTER | RuntimeAdapter/Host | 禁止Runtime2 | B11 |
| B13 | 扩展ResourceURI支持workspace/SAF/SFTP | EXTEND | ResourceURI | 禁止minis主Scheme | B12 |
| B14 | 建立Browser Runtime/Session | NEW_PROVIDER+EXTEND | Browser Provider(新建) | 禁止Browser独立Registry | B11 |
| B15 | 复用现有imageprovider/asr/tts+新建FFmpegProvider | REUSE+EXTEND+NEW | ImageProvider/Voice | 禁止Media2/Voice2/Provider2 | B11 |
| B16 | 映射现有Model/Voice/Automation到Kernel | REUSE+EXTEND | chat/model_service | 禁止Model2/Voice2 | B13 |
| B17 | Extension Manifest补强 | EXTEND | manifest_v2/ContributionRegistry | 禁止Manifest v3独立体系 | B16 |
| B18 | 验证Extension Kernel唯一性 | VALIDATION_ONLY | N/A | 禁止修改实现 | B10~B17全完成 |

## 批次四: B19~B38 (Adapter + Agent能力迁移)

| Step | Corrected Responsibility | Construction Mode | Canonical Target | Forbidden Duplicate | Prerequisite |
|------|------------------------|-------------------|-----------------|---------------------|--------------|
| B19 | Android平台能力映射到现有框架 | ADAPTER_ONLY | AndroidCapabilityAdapter | 禁止AndroidToolCenter | B18 |
| B20 | iOS平台能力映射到现有框架 | ADAPTER_ONLY | IOSCapabilityAdapter | 禁止iOS独立Kernel | B18 |
| B21 | Desktop平台能力映射到现有框架 | ADAPTER_ONLY | DesktopProcessAdapter | 禁止Desktop独立Runtime | B18 |
| B22 | 验证Adapter层一致性 | VALIDATION_ONLY | N/A | 禁止修改Adapter | B19,B20,B21 |
| B23 | Planner能力迁移到ExecutionPipeline | MIGRATION_ONLY | MindRuntime | 禁止ParityPlanner系统 | B18 |
| B24 | Observer能力纳入Observation体系 | MIGRATION_ONLY | MindRuntime | 禁止ObserverRuntime2 | B23 |
| B25 | TaskGraph能力映射到Agent任务体系 | MIGRATION_ONLY | Workflow | 禁止TaskGraphEngine2 | B23 |
| B26 | Background能力纳入后台任务体系 | MIGRATION_ONLY | TaskRuntime | 禁止BackgroundRuntime | B23 |
| B27 | ToolFacade能力统一到现有Tool体系 | MIGRATION_ONLY | ToolFacade | 禁止替代Facade实现 | B23 |
| B28 | MultiAgent能力映射到Agent通信体系 | MIGRATION_ONLY | AgentSkill | 禁止MultiAgent独立系统 | B23 |
| B29 | Context编排能力纳入Extension Kernel | MIGRATION_ONLY | ExtensionKernel | 禁止Context独立引擎 | B23 |
| B30 | ErrorHandling能力映射到现有体系 | MIGRATION_ONLY | ErrorSystem | 禁止ErrorHandling替代体系 | B23 |
| B31 | StateManagement能力纳入状态引擎 | MIGRATION_ONLY | StateManagement | 禁止StateManagement替代 | B23 |
| B32 | Agent通信协议映射到内核通道 | MIGRATION_ONLY | Workflow | 禁止通信替代协议 | B23 |
| B33 | ExecutionPipeline内容统一到现有Pipeline | MIGRATION_ONLY | ExecutionPipeline | 禁止ExecutionPipeline2 | B23 |
| B34 | DecisionEngine能力纳入决策体系 | MIGRATION_ONLY | MindRuntime | 禁止DecisionEngine2 | B23 |
| B35 | CheckpointStore能力映射到现有体系 | MIGRATION_ONLY | CheckpointStore | 禁止CheckpointStore2 | B33 |
| B36 | RecoverySystem能力纳入恢复体系 | MIGRATION_ONLY | MindRuntime | 禁止RecoverySystem替代 | B33 |
| B37 | ProgressTracker能力映射到进度体系 | MIGRATION_ONLY | TaskRuntime | 禁止ProgressTracker独立 | B33 |
| B38 | Capacity能力统一到Runtime体系 | MIGRATION_ONLY | ToolFacade | 禁止Capacity替代Runtime | B33 |

## 批次五: B39~B54 (Parity Gap Hardening)

| Step | Corrected Responsibility | Construction Mode | Canonical Target | Forbidden Duplicate | Prerequisite |
|------|------------------------|-------------------|-----------------|---------------------|--------------|
| B39 | 补强ExecutionPipeline错误传播/重试 | EXTEND | ExecutionPipeline | 禁止Pipeline2 | B33 |
| B40 | 补强PermissionBroker Scope继承/审批 | EXTEND | PermissionBroker | 禁止Broker2 | B11 |
| B41 | 补强ToolContext生命周期传播 | EXTEND | ToolFacade | 禁止ToolContext2 | B10 |
| B42 | 补强ToolResult错误码/元数据 | EXTEND | RuntimeAdapterRegistry | 禁止ToolResult2 | B12 |
| B43 | 补强AuditSystem日志/检索 | EXTEND | ToolRegistry | 禁止AuditSystem2 | B10 |
| B44 | 补强QuotaSystem计费/限流 | EXTEND | ExecutionPipeline | 禁止QuotaSystem2 | B39 |
| B45 | 补强ValidationLayer规则覆盖 | EXTEND | PermissionBroker | 禁止ValidationLayer2 | B40 |
| B46 | 补强RateLimiter多维策略 | EXTEND | ResourceURI | 禁止RateLimiter2 | B13 |
| B47 | 补强ConcurrencyControl锁粒度 | EXTEND | HookSystem | 禁止ConcurrencyControl替代 | B107 |
| B48 | 补强TimeoutManagement唤醒/级联 | EXTEND | Schedule | 禁止Timeout替代方案 | B108 |
| B49 | 补强ErrorRecovery决策链 | EXTEND | TaskRuntime | 禁止ErrorRecovery替代 | B36 |
| B50 | 补强ResourceLimit度量/告警 | EXTEND | Workflow | 禁止ResourceLimit替代 | B103 |
| B51 | 补强SandboxConfig策略/分发 | EXTEND | MindRuntime | 禁止SandboxConfig替代 | B51依赖 |
| B52 | 补强ToolExecutionHook注册 | EXTEND | PermissionBroker | 禁止Hook替代机制 | B40 |
| B53 | 补强ExecutionMonitor展示/告警 | EXTEND | ToolFacade | 禁止Monitor替代方案 | B10 |
| B54 | 补强FallbackMechanism决策链 | EXTEND | ResourceURI | 禁止Fallback替代体系 | B13 |

## 批次六: B55~B78 (Linux/Android Native/Media Provider)

| Step | Corrected Responsibility | Construction Mode | Canonical Target | Forbidden Duplicate | Prerequisite |
|------|------------------------|-------------------|-----------------|---------------------|--------------|
| B55 | Linux Shell Provider新建 | NEW_PROVIDER | LinuxShellProvider | 禁止AndroidToolCenter替代 | B12 |
| B56 | Android Process Provider新建 | NEW_PROVIDER | AndroidProcessProvider | 禁止AndroidPermissionCenter | B55 |
| B57 | Android FileSystem Provider新建 | NEW_PROVIDER | AndroidFileSystemProvider | 禁止AndroidLogCenter | B55 |
| B58 | Android Network Provider新建 | NEW_PROVIDER | AndroidNetworkProvider | 禁止AndroidResourceMonitor | B55 |
| B59 | Android Storage Provider新建 | NEW_PROVIDER | AndroidStorageProvider | 禁止AndroidStateTracker | B56 |
| B60 | Android Camera Provider新建 | NEW_PROVIDER | AndroidCameraProvider | 禁止AndroidCapability替代 | B19 |
| B61 | Android Sensor Provider新建 | NEW_PROVIDER | AndroidSensorProvider | 禁止AndroidAdapter替代 | B19 |
| B62 | Android Accessibility Provider新建 | NEW_PROVIDER | AndroidAccessibilityProvider | 禁止AndroidNativeTool替代 | B19 |
| B63 | Android Location Provider新建 | NEW_PROVIDER | AndroidLocationProvider | 禁止AndroidNativePermission | B62 |
| B64 | Android Notification Provider新建 | NEW_PROVIDER | AndroidNotificationProvider | 禁止AndroidNativeRuntime | B19 |
| B65 | Android MediaSession Provider新建 | NEW_PROVIDER | AndroidMediaSessionProvider | 禁止AndroidNativeLogger | B19 |
| B66 | Android Telephony Provider新建 | NEW_PROVIDER | AndroidTelephonyProvider | 禁止AndroidNativeResource | B67 |
| B67 | Android Permission Provider新建 | NEW_PROVIDER | AndroidPermissionProvider | 禁止AndroidNativeState | B19 |
| B68 | Android Provider全量接线 | INTEGRATION_ONLY | Android Provider Registry | 禁止AndroidNativeCapability | B55~B67 |
| B69 | Image Generation Provider新建 | NEW_PROVIDER | ImageGenerationProvider | 禁止MediaProvider替代 | B15 |
| B70 | Image Recognition Provider新建 | NEW_PROVIDER | ImageRecognitionProvider | 禁止AudioProvider替代 | B69 |
| B71 | TTS Provider扩展 | EXTEND | TTSProvider | 禁止VideoProvider替代 | B15 |
| B72 | ASR Provider扩展 | EXTEND | ASRProvider | 禁止ImageProvider替代 | B15 |
| B73 | Video Processing Provider新建 | NEW_PROVIDER | VideoProcessingProvider | 禁止CameraProvider替代 | B74 |
| B74 | Audio Processing Provider新建 | NEW_PROVIDER | AudioProcessingProvider | 禁止IOSDevice替代Runtime | B15 |
| B75 | Screen Capture Provider新建 | NEW_PROVIDER | ScreenCaptureProvider | 禁止IOSAudio替代 | B19 |
| B76 | Webcam Provider新建 | NEW_PROVIDER | WebcamProvider | 禁止IOSVideo替代 | B75 |
| B77 | Audio Stream Provider新建 | NEW_PROVIDER | AudioStreamProvider | 禁止IOSImage替代 | B76 |
| B78 | Media Provider全量接线 | INTEGRATION_ONLY | MediaProviderRegistry | 禁止IOSCamera替代 | B69~B77 |

## 批次七: B79~B92 (Browser/Search/Media/Workspace)

| Step | Corrected Responsibility | Construction Mode | Canonical Target | Forbidden Duplicate | Prerequisite |
|------|------------------------|-------------------|-----------------|---------------------|--------------|
| B79 | Browser Runtime Provider新建 | NEW_PROVIDER | BrowserRuntimeProvider | 禁止BrowserToolRegistry | B14 |
| B80 | Browser Runtime能力扩展 | EXTEND | BrowserRuntimeProvider | 禁止BrowserRuntime替代 | B79 |
| B81 | Browser Permission收敛到Broker | EXTEND | BrowserRuntimeProvider | 禁止BrowserPermission替代 | B80,B11 |
| B82 | Browser Workspace收敛 | EXTEND | BrowserRuntimeProvider | 禁止BrowserWorkspace | B80,B13 |
| B83 | Browser HistoryStore收敛 | REUSE | BrowserRuntimeProvider | 禁止HistoryStore2 | B80 |
| B84 | Search Engine Provider新建 | NEW_PROVIDER | SearchEngineProvider | 禁止SearchToolRegistry | B14 |
| B85 | Search HistoryStore收敛 | EXTEND | SearchEngineProvider | 禁止SearchHistoryStore2 | B84 |
| B86 | Search Permission收敛 | REUSE | SearchEngineProvider | 禁止SearchPermission替代 | B84,B11 |
| B87 | Media Capability注册完整性验证 | REUSE | MediaProvider | 禁止MediaCapability替代 | B78 |
| B88 | MCP Capability注册完整性验证 | REUSE | MCP | 禁止MCPCapability替代 | B110 |
| B89 | Workspace Capability注册完整性验证 | EXTEND | WorkspaceProvider | 禁止WorkspaceCapability替代 | B13 |
| B90 | Integration Capability注册完整性验证 | EXTEND | PermissionBroker | 禁止IntegrationCapability替代 | B11 |
| B91 | Validation Capability注册完整性验证 | EXTEND | ToolFacade | 禁止ValidationCapability替代 | B10 |
| B92 | Migration Capability注册完整性验证 | REUSE | ToolFacade | 禁止MigrationCapability替代 | B140 |

## 批次八: B93~B110 (MCP/Skill/Workflow/Hook/Schedule)

| Step | Corrected Responsibility | Construction Mode | Canonical Target | Forbidden Duplicate | Prerequisite |
|------|------------------------|-------------------|-----------------|---------------------|--------------|
| B93 | MCP Tool Registry同步到ToolFacade | EXTEND | MCPToolRegistry/ToolFacade | 禁止MCPManager2 | B10,B11,B12 |
| B94 | MCP Tool Registry同步机制扩展 | EXTEND | MCP | 禁止MCPToolRegistry2 | B93 |
| B95 | MCP Skill Runtime同步 | EXTEND | MCP | 禁止SkillRuntime2 | B94 |
| B96 | MCP Session/Registry扩展 | EXTEND | MCP | 禁止SkillRegistry2 | B95 |
| B97 | MCP Runtime纳入RuntimeOrchestrator | EXTEND | MCP | 禁止WorkflowEngine2 | B96 |
| B98 | MCP Permission同步到Broker | EXTEND | PermissionBroker/MCP | 禁止EventBus2 | B97,B11 |
| B99 | MCP AgentSkill同步 | EXTEND | MCP | 禁止HookSystem2 | B95,B100 |
| B100 | AgentSkill能力扩展 | EXTEND | AgentSkill | 禁止Scheduler2 | B95 |
| B101 | AgentSkill Extension集成 | EXTEND | AgentSkill | 禁止WorkflowStore替代 | B100,B17 |
| B102 | AgentSkill TaskRuntime集成 | EXTEND | AgentSkill | 禁止WorkflowAudit替代 | B100 |
| B103 | Workflow Engine能力增强 | EXTEND | WorkflowEngine | 禁止MCPConnection替代 | B97,B99,B101,B102 |
| B104 | Workflow Schedule集成 | EXTEND | Workflow | 禁止MCPToolSync替代 | B103 |
| B105 | Workflow执行走Pipeline | EXTEND | Workflow | 禁止SkillExecutor替代 | B103 |
| B106 | Workflow Hook集成 | EXTEND | Workflow | 禁止SkillOrchestrator替代 | B103 |
| B107 | Workflow TaskRuntime集成 | EXTEND | Workflow/TaskRuntime | 禁止WorkflowOrchestrator替代 | B103,B104,B105,B106 |
| B108 | Workflow Hook扩展 | EXTEND | Workflow/HookSystem | 禁止EventDispatcher替代 | B106 |
| B109 | Workflow Event扩展 | EXTEND | Workflow/EventSystem | 禁止HookExecutor替代 | B108 |
| B110 | MCP/Skill/Workflow/Hook全量接线 | INTEGRATION_ONLY | All | 禁止TaskScheduler替代 | B93~B109 |

## 批次九: B111~B122 (Model/Voice/Memory/Character)

| Step | Corrected Responsibility | Construction Mode | Canonical Target | Forbidden Duplicate | Prerequisite |
|------|------------------------|-------------------|-----------------|---------------------|--------------|
| B111 | Responses API/MNN Local Provider新建 | NEW_PROVIDER | ResponsesAPI/MNNLocalProvider | 禁止ModelConfigs_v2 | B16 |
| B112 | ModelService增强+Embedding集成 | EXTEND | ModelService/Embedding | 禁止ModelRouter2 | B111 |
| B113 | Local Model Provider新建 | NEW_PROVIDER | LocalModelProvider | 禁止VoiceRuntime2 | B16 |
| B114 | Model/Responses API集成 | EXTEND | ModelService | 禁止ASR Config2 | B111,B112,B113 |
| B115 | Voice Provider新建 | NEW_PROVIDER | VoiceProvider | 禁止TTS Config2 | B15 |
| B116 | Character能力扩展 | EXTEND | Character | 禁止MemorySystem2 | B121 |
| B117 | Character Schedule集成 | EXTEND | Character | 禁止MemoryStore2 | B116 |
| B118 | Memory Store/Graph能力扩展 | EXTEND | MemoryStore/MemoryGraph | 禁止VectorStore2 | B16 |
| B119 | Memory能力扩展 | EXTEND | Memory | 禁止MemoryGraph2 | B118 |
| B120 | Memory Persistence集成 | EXTEND | Memory | 禁止CharacterCore2 | B119 |
| B121 | Character Store/Service能力扩展 | EXTEND | CharacterStore/CharacterService | 禁止CharacterDB替代 | B120 |
| B122 | Model/Voice/Memory/Character全量接线 | INTEGRATION_ONLY | All | 禁止CharacterService替代 | B111~B121 |

## 批次十: B123~B154 (iOS Provider + Integration/Migration/Validation)

| Step | Corrected Responsibility | Construction Mode | Canonical Target | Forbidden Duplicate | Prerequisite |
|------|------------------------|-------------------|-----------------|---------------------|--------------|
| B123 | iOS Alchemy Provider新建 | NEW_PROVIDER | IOSAlchemyProvider | 禁止IOSSandboxLifecycle2 | B20 |
| B124 | iOS Alchemy能力扩展 | EXTEND | IOSAlchemyProvider | 禁止IOSRuntimeManager2 | B123 |
| B125 | iOS Tool注册到ToolRegistry | EXTEND | IOSAlchemyProvider | 禁止IOSToolRegistry2 | B124 |
| B126 | iOS Permission走PermissionBroker | EXTEND | IOSAlchemyProvider | 禁止IOSPermissionCenter2 | B124,B11 |
| B127 | HealthKit Provider新建 | NEW_PROVIDER | HealthKitProvider | 禁止IOSCapabilityRegistry2 | B123 |
| B128 | HomeKit Provider新建 | NEW_PROVIDER | HomeKitProvider | 禁止IOSResourceMonitor2 | B123 |
| B129 | CoreML Provider新建 | NEW_PROVIDER | CoreMLProvider | 禁止IOSStateTracker2 | B123 |
| B130 | CallKit Provider新建 | NEW_PROVIDER | CallKitProvider | 禁止IOSEventBus2 | B123 |
| B131 | NFC Provider新建 | NEW_PROVIDER | NFCProvider | 禁止IOSWorkflowEngine2 | B123 |
| B132 | ARKit Provider新建 | NEW_PROVIDER | ARKitProvider | 禁止IOSScheduler2 | B123 |
| B133 | PassKit Provider新建 | NEW_PROVIDER | PassKitProvider | 禁止IOSMemoryManager2 | B123 |
| B134 | CarPlay Provider新建 | NEW_PROVIDER | CarPlayProvider | 禁止IOSCharacterService2 | B123 |
| B135 | WatchConnectivity Provider新建 | NEW_PROVIDER | WatchConnectivityProvider | 禁止IOSVoiceService2 | B123 |
| B136 | Intents Provider新建 | NEW_PROVIDER | IntentsProvider | 禁止IOSModelService2 | B123 |
| B137 | iOS Provider全量接线 | REUSE | IOSAlchemyProvider | 禁止RuntimeSupervisor2 | B123~B136 |
| B138 | iOS Provider与Capability接线 | REUSE | IOSAlchemyProvider | 禁止统一Runtime替代 | B137 |
| B139 | 三端统一Adapter接入 | ADAPTER_ONLY | UnifiedCapabilityAdapter | 禁止统一Runtime替代 | B137,B68,B21 |
| B140 | Agent→ToolFacade Final Cutover | MIGRATION_ONLY | LegacyToolSystem | 禁止保留Legacy Registry | B23~B38 |
| B141 | Existing Execution Kernel Final Cutover | MIGRATION_ONLY | AlternativeExecutionChain | 禁止保留Pipeline副本 | B140 |
| B142 | Contract一致性验证 | VALIDATION_ONLY | N/A | 禁止修改实现 | B141 |
| B143 | Permission分配一致性验证 | VALIDATION_ONLY | N/A | 禁止修改实现 | B141 |
| B144 | Contract兼容性检查 | VALIDATION_ONLY | N/A | 禁止修改实现 | B141 |
| B145 | Workflow执行验证 | VALIDATION_ONLY | N/A | 禁止修改实现 | B141 |
| B146 | Extension生命周期验证 | VALIDATION_ONLY | N/A | 禁止修改实现 | B141 |
| B147 | Model/Voice/Memory/Character适配验证 | VALIDATION_ONLY | N/A | 复用现有领域 | B141 |
| B148 | Android适配验证 | VALIDATION_ONLY | N/A | 禁止修改实现 | B141,B68 |
| B149 | iOS适配验证 | VALIDATION_ONLY | N/A | 禁止修改实现 | B141,B137 |
| B150 | Desktop适配验证 | VALIDATION_ONLY | N/A | 禁止修改实现 | B141,B21 |
| B151 | 残余迁移/重复入口/Mock清理 | EXTEND+MIGRATION_ONLY | LegacyAudit | 仅清理不承担新Provider | B141 |
| B152 | 迁移State验证 | VALIDATION_ONLY | N/A | 禁止修改实现 | B151 |
| B153 | Pipeline唯一性验证 | VALIDATION_ONLY | N/A | 禁止修改实现 | B151 |
| B154 | 整体系统验收验证 | VALIDATION_ONLY | N/A | 禁止修改实现 | B153 |

## 并行执行规则

同文件写集相交 → SERIAL

三条初始合同轨道：
- Track-A (Core Contract): B10→B11→B12→B13 (串行)
- Track-B (Adapter): B19∥B20∥B21 (并行)
- Track-C (Linux Provider): B55∥B56∥B57∥B58∥B59∥B60∥B61 (并行)

B18条件: 必须等待Track-A + B14~B17完成。

## A线职责与B线分工

A线: RuntimeHost、RuntimeOrchestrator、ProcessSupervisor、Runtime Service、Flutter Bridge、Runtime install、Runtime lifecycle

B线: 复用A线实现，所有涉及Runtime的步骤均为EXTEND/REUSE模式，不新建Runtime。

Android特别Guard: B55~B78禁止自行建立Runtime生命周期。
iOS特别Guard: iOS可以有Sandbox Provider/Platform Adapter，但不得复制整套独立Kernel。
Desktop特别Guard: 现有Runtime能力必须复用，不得为了统一再写Desktop Runtime2。

## 全局 FORBIDDEN 动作清单

1. 禁止创建第二Tool Registry (ToolRegistry2)
2. 禁止创建第二Permission Broker (PermissionBroker2)
3. 禁止创建第二Execution Pipeline (ExecutionPipeline2)
4. 禁止创建第二Agent Runtime (AgentRuntime2)
5. 禁止创建第二Workflow Engine (WorkflowEngine2)
6. 禁止创建第二Memory Store (MemoryStore2)
7. 禁止创建第二Vector Store (VectorStore2)
8. 禁止创建第二Character Core (CharacterCore2)
9. 禁止创建第二MCP Manager (MCPManager2)
10. 禁止创建第二Skill Runtime (SkillRuntime2)
11. 禁止创建第二Event Bus (EventBus2)
12. 禁止创建第二Hook System (HookSystem2)
13. 禁止创建第二Scheduler (Scheduler2)
14. 禁止创建第二Model Config (ModelConfig_v2)
15. 禁止创建第二Browser Tool Registry
16. 禁止创建第二Search Tool Registry
17. 禁止iOS独立Kernel/Registry/Runtime
18. 禁止Android独立ToolCenter/PermissionCenter
19. 禁止Desktop独立RuntimeKernel

## 统计

A线: 43步 (A1~A43，含Runtime/Bridge/Platform适配)
B线: 154步 (B1~B154)
B9P: 8步 (B9P1~B9P8)

B10~B154分类统计:
- REUSE: 1
- EXTEND: 72
- ADAPTER_ONLY: 3
- NEW_PROVIDER: 51
- MIGRATION_ONLY: 2
- INTEGRATION_ONLY: 5
- VALIDATION_ONLY: 11
