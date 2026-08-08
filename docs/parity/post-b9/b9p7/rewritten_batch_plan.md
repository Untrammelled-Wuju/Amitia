# B10-B154 施工批次修订稿 (Post-B9 Architecture Guard)

## 0. 核心修订原则
1. 现有Extension Kernel系统为唯一Canonical底座
2. 只补缺口(EXTEND)、加Adapter(ADAPTER_ONLY)、加Provider(NEW_PROVIDER)
3. 禁止创建任何平行系统(FORBIDDEN_DUPLICATE)
4. 迁移步骤唯一方向: Legacy → Canonical

## 1. 批次三: B10-B18 (Contract层, EXTEND+REUSE为主)
| 步骤 | 修订后职责 | 施工模式 | Canonical Target | 禁止项 |
|------|----------|---------|-----------------|--------|
| B10 | 对现有ToolDefinition/ExecutionPipeline执行Gap审计并补字段 | EXTEND+REUSE | ToolRegistry/ToolFacade/ExecutionPipeline | 禁止新建ToolRegistry/Schema/Result |
| B11 | 给PermissionDefinitionRegistry补14个GAP | EXTEND+REUSE | PermissionBroker | 禁止新Permission系统 |
| B12 | 补Native Offload/Runtime合同 | EXTEND+ADAPTER | RuntimeAdapter/Host | 禁止Runtime2 |
| B13 | 扩展ResourceURI支持workspace/SAF/SFTP | EXTEND | ResourceURI | 禁止minis主Scheme |
| B14 | 建立Browser Runtime/Session | NEW_PROVIDER+EXTEND | Browser Provider(新建) | 禁止Browser独立Registry |
| B15 | 复用现有imageprovider/asr/tts+新建FFmpegProvider | REUSE+EXTEND+NEW | ImageProvider/Voice | 禁止Media2/Voice2/Provider2 |
| B16 | 映射现有Model/Voice/Automation到Kernel | REUSE+EXTEND | chat/model_service | 禁止Model2/Voice2 |
| B17 | Extension Manifest补强 | EXTEND | manifest_v2/ContributionRegistry | 禁止Manifest v3独立体系 |
| B18 | 验证Extension Kernel唯一性 | VALIDATION_ONLY | N/A | 禁止修改实现 |

## 2. 批次四: B19-B38 (Adapter + Agent能力迁移)
| 步骤 | 修订后职责 | 施工模式 | Canonical Target | 禁止项 |
|------|----------|---------|-----------------|--------|
| B19 | Android平台能力映射到现有框架 | ADAPTER_ONLY | AndroidCapabilityAdapter | 禁止AndroidToolCenter |
| B20 | iOS平台能力映射到现有框架 | ADAPTER_ONLY | IOSCapabilityAdapter | 禁止iOS独立Kernel |
| B21 | Desktop平台能力映射到现有框架 | ADAPTER_ONLY | DesktopProcessAdapter | 禁止Desktop独立Runtime |
| B22 | 验证Adapter层一致性 | VALIDATION_ONLY | N/A | 禁止修改Adapter |
| B23 | Planner能力迁移到ExecutionPipeline | MIGRATION_ONLY | MindRuntime | 禁止ParityPlanner系统 |
| B24 | Observer能力纳入Observation体系 | MIGRATION_ONLY | MindRuntime | 禁止ObserverRuntime2 |
| B25 | TaskGraph能力映射到Agent任务体系 | MIGRATION_ONLY | Workflow | 禁止TaskGraphEngine2 |
| B26 | Background能力纳入后台任务体系 | MIGRATION_ONLY | TaskRuntime | 禁止BackgroundRuntime |
| B27 | ToolFacade能力统一到现有Tool体系 | MIGRATION_ONLY | ToolFacade | 禁止替代Facade实现 |
| B28 | MultiAgent能力映射到Agent通信体系 | MIGRATION_ONLY | AgentSkill | 禁止MultiAgent独立系统 |
| B29 | Context编排能力纳入Extension Kernel | MIGRATION_ONLY | ExtensionKernel | 禁止Context独立引擎 |
| B30 | ErrorHandling能力映射到现有体系 | MIGRATION_ONLY | ErrorSystem | 禁止ErrorHandling替代体系 |
| B31 | StateManagement能力纳入状态引擎 | MIGRATION_ONLY | StateManagement | 禁止StateManagement替代 |
| B32 | Agent通信协议映射到内核通道 | MIGRATION_ONLY | Workflow | 禁止通信替代协议 |
| B33 | ExecutionPipeline内容统一到现有Pipeline | MIGRATION_ONLY | ExecutionPipeline | 禁止ExecutionPipeline2 |
| B34 | DecisionEngine能力纳入决策体系 | MIGRATION_ONLY | MindRuntime | 禁止DecisionEngine2 |
| B35 | CheckpointStore能力映射到现有体系 | MIGRATION_ONLY | CheckpointStore | 禁止CheckpointStore2 |
| B36 | RecoverySystem能力纳入恢复体系 | MIGRATION_ONLY | MindRuntime | 禁止RecoverySystem替代 |
| B37 | ProgressTracker能力映射到进度体系 | MIGRATION_ONLY | TaskRuntime | 禁止ProgressTracker独立 |
| B38 | Capacity能力统一到Runtime体系 | MIGRATION_ONLY | ToolFacade | 禁止Capacity替代Runtime |

## 3. 批次五: B39-B54 (Parity Gap Hardening)
| 步骤 | 修订后职责 | 施工模式 | Canonical Target | 禁止项 |
|------|----------|---------|-----------------|--------|
| B39 | 补强ExecutionPipeline错误传播/重试 | EXTEND | ExecutionPipeline | 禁止Pipeline2 |
| B40 | 补强PermissionBroker Scope继承/审批 | EXTEND | PermissionBroker | 禁止Broker2 |
| B41 | 补强ToolContext生命周期传播 | EXTEND | ToolFacade | 禁止ToolContext2 |
| B42 | 补强ToolResult错误码/元数据 | EXTEND | RuntimeAdapterRegistry | 禁止ToolResult2 |
| B43 | 补强AuditSystem日志/检索 | EXTEND | ToolRegistry | 禁止AuditSystem2 |
| B44 | 补强QuotaSystem计费/限流 | EXTEND | ExecutionPipeline | 禁止QuotaSystem2 |
| B45 | 补强ValidationLayer规则覆盖 | EXTEND | PermissionBroker | 禁止ValidationLayer2 |
| B46 | 补强RateLimiter多维策略 | EXTEND | ResourceURI | 禁止RateLimiter2 |
| B47 | 补强ConcurrencyControl锁粒度 | EXTEND | HookSystem | 禁止ConcurrencyControl替代 |
| B48 | 补强TimeoutManagement唤醒/级联 | EXTEND | Schedule | 禁止Timeout替代方案 |
| B49 | 补强ErrorRecovery决策链 | EXTEND | TaskRuntime | 禁止ErrorRecovery替代 |
| B50 | 补强ResourceLimit度量/告警 | EXTEND | Workflow | 禁止ResourceLimit替代 |
| B51 | 补强SandboxConfig策略/分发 | EXTEND | MindRuntime | 禁止SandboxConfig替代 |
| B52 | 补强ToolExecutionHook注册 | EXTEND | PermissionBroker | 禁止Hook替代机制 |
| B53 | 补强ExecutionMonitor展示/告警 | EXTEND | ToolFacade | 禁止Monitor替代方案 |
| B54 | 补强FallbackMechanism决策链 | EXTEND | ResourceURI | 禁止Fallback替代体系 |

## 4. 批次六: B55-B78 (Desktop/Android/iOS Provider)
| 步骤 | 修订后职责 | 施工模式 | Canonical Target | 禁止项 |
|------|----------|---------|-----------------|--------|
| B55 | Linux Shell Provider新建 | NEW_PROVIDER | LinuxShellProvider | 禁止AndroidToolCenter替代 |
| B56 | Android Process Provider新建 | NEW_PROVIDER | AndroidProcessProvider | 禁止AndroidPermissionCenter |
| B57 | Android FileSystem Provider新建 | NEW_PROVIDER | AndroidFileSystemProvider | 禁止AndroidLogCenter |
| B58 | Android Network Provider新建 | NEW_PROVIDER | AndroidNetworkProvider | 禁止AndroidResourceMonitor |
| B59 | Android Storage Provider新建 | NEW_PROVIDER | AndroidStorageProvider | 禁止AndroidStateTracker |
| B60 | Android Camera Provider新建 | NEW_PROVIDER | AndroidCameraProvider | 禁止AndroidCapability替代 |
| B61 | Android Sensor Provider新建 | NEW_PROVIDER | AndroidSensorProvider | 禁止AndroidAdapter替代 |
| B62 | Android Accessibility Provider新建 | NEW_PROVIDER | AndroidAccessibilityProvider | 禁止AndroidNativeTool替代 |
| B63 | Android Location Provider新建 | NEW_PROVIDER | AndroidLocationProvider | 禁止AndroidNativePermission |
| B64 | Android Notification Provider新建 | NEW_PROVIDER | AndroidNotificationProvider | 禁止AndroidNativeRuntime |
| B65 | Android MediaSession Provider新建 | NEW_PROVIDER | AndroidMediaSessionProvider | 禁止AndroidNativeLogger |
| B66 | Android Telephony Provider新建 | NEW_PROVIDER | AndroidTelephonyProvider | 禁止AndroidNativeResource |
| B67 | Android Permission Provider新建 | NEW_PROVIDER | AndroidPermissionProvider | 禁止AndroidNativeState |
| B68 | Android Provider全量接线 | INTEGRATION_ONLY | Android Provider Registry | 禁止AndroidNativeCapability |
| B69 | Image Generation Provider新建 | NEW_PROVIDER | ImageGenerationProvider | 禁止MediaProvider替代 |
| B70 | Image Recognition Provider新建 | NEW_PROVIDER | ImageRecognitionProvider | 禁止AudioProvider替代 |
| B71 | TTS Provider扩展 | EXTEND | TTSProvider | 禁止VideoProvider替代 |
| B72 | ASR Provider扩展 | EXTEND | ASRProvider | 禁止ImageProvider替代 |
| B73 | Video Processing Provider新建 | NEW_PROVIDER | VideoProcessingProvider | 禁止CameraProvider替代 |
| B74 | Audio Processing Provider新建 | NEW_PROVIDER | AudioProcessingProvider | 禁止IOSDevice替代Runtime |
| B75 | Screen Capture Provider新建 | NEW_PROVIDER | ScreenCaptureProvider | 禁止IOSAudio替代 |
| B76 | Webcam Provider新建 | NEW_PROVIDER | WebcamProvider | 禁止IOSVideo替代 |
| B77 | Audio Stream Provider新建 | NEW_PROVIDER | AudioStreamProvider | 禁止IOSImage替代 |
| B78 | Media Provider全量接线 | INTEGRATION_ONLY | MediaProviderRegistry | 禁止IOSCamera替代 |

## 5. 批次七: B79-B92 (Browser/Search/Media/Workspace)
| 步骤 | 修订后职责 | 施工模式 | Canonical Target | 禁止项 |
|------|----------|---------|-----------------|--------|
| B79 | Browser Runtime Provider新建 | NEW_PROVIDER | BrowserRuntimeProvider | 禁止BrowserToolRegistry |
| B80 | Browser Runtime能力扩展 | EXTEND | BrowserRuntimeProvider | 禁止BrowserRuntime替代 |
| B81 | Browser Permission收敛到Broker | EXTEND | BrowserRuntimeProvider | 禁止BrowserPermission替代 |
| B82 | Browser Workspace收敛 | EXTEND | BrowserRuntimeProvider | 禁止BrowserWorkspace |
| B83 | Browser HistoryStore收敛 | REUSE | BrowserRuntimeProvider | 禁止HistoryStore2 |
| B84 | Search Engine Provider新建 | NEW_PROVIDER | SearchEngineProvider | 禁止SearchToolRegistry |
| B85 | Search HistoryStore收敛 | EXTEND | SearchEngineProvider | 禁止SearchHistoryStore2 |
| B86 | Search Permission收敛 | REUSE | SearchEngineProvider | 禁止SearchPermission替代 |
| B87 | Media Capability注册完整性验证 | REUSE | MediaProvider | 禁止MediaCapability替代 |
| B88 | MCP Capability注册完整性验证 | REUSE | MCP | 禁止MCPCapability替代 |
| B89 | Workspace Capability注册完整性验证 | EXTEND | WorkspaceProvider | 禁止WorkspaceCapability替代 |
| B90 | Integration Capability注册完整性验证 | EXTEND | PermissionBroker | 禁止IntegrationCapability替代 |
| B91 | Validation Capability注册完整性验证 | EXTEND | ToolFacade | 禁止ValidationCapability替代 |
| B92 | Migration Capability注册完整性验证 | REUSE | ToolFacade | 禁止MigrationCapability替代 |

## 6. 批次八: B93-B110 (MCP/Skill/Workflow/Hook/Schedule)
| 步骤 | 修订后职责 | 施工模式 | Canonical Target | 禁止项 |
|------|----------|---------|-----------------|--------|
| B93 | MCP Tool Registry同步到ToolFacade | EXTEND | MCPToolRegistry/ToolFacade | 禁止MCPManager2 |
| B94 | MCP Tool Registry同步机制扩展 | EXTEND | MCP | 禁止MCPToolRegistry2 |
| B95 | MCP Skill Runtime同步 | EXTEND | MCP | 禁止SkillRuntime2 |
| B96 | MCP Session/Registry扩展 | EXTEND | MCP | 禁止SkillRegistry2 |
| B97 | MCP Runtime纳入RuntimeOrchestrator | EXTEND | MCP | 禁止WorkflowEngine2 |
| B98 | MCP Permission同步到Broker | EXTEND | PermissionBroker/MCP | 禁止EventBus2 |
| B99 | MCP AgentSkill同步 | EXTEND | MCP | 禁止HookSystem2 |
| B100 | AgentSkill能力扩展 | EXTEND | AgentSkill | 禁止Scheduler2 |
| B101 | AgentSkill Extension集成 | EXTEND | AgentSkill | 禁止WorkflowStore替代 |
| B102 | AgentSkill TaskRuntime集成 | EXTEND | AgentSkill | 禁止WorkflowAudit替代 |
| B103 | Workflow Engine能力增强 | EXTEND | WorkflowEngine | 禁止MCPConnection替代 |
| B104 | Workflow Schedule集成 | EXTEND | Workflow | 禁止MCPToolSync替代 |
| B105 | Workflow执行走Pipeline | EXTEND | Workflow | 禁止SkillExecutor替代 |
| B106 | Workflow Hook集成 | EXTEND | Workflow | 禁止SkillOrchestrator替代 |
| B107 | Workflow TaskRuntime集成 | EXTEND | Workflow/TaskRuntime | 禁止WorkflowOrchestrator替代 |
| B108 | Workflow Hook扩展 | EXTEND | Workflow/HookSystem | 禁止EventDispatcher替代 |
| B109 | Workflow Event扩展 | EXTEND | Workflow/EventSystem | 禁止HookExecutor替代 |
| B110 | MCP/Skill/Workflow/Hook全量接线 | INTEGRATION_ONLY | All | 禁止TaskScheduler替代 |

## 7. 批次九: B111-B122 (Model/Voice/Memory/Character)
| 步骤 | 修订后职责 | 施工模式 | Canonical Target | 禁止项 |
|------|----------|---------|-----------------|--------|
| B111 | Responses API/MNN Local Provider新建 | NEW_PROVIDER | ResponsesAPI/MNNLocalProvider | 禁止ModelConfigs_v2 |
| B112 | ModelService增强+Embedding集成 | EXTEND | ModelService/Embedding | 禁止ModelRouter2 |
| B113 | Local Model Provider新建 | NEW_PROVIDER | LocalModelProvider | 禁止VoiceRuntime2 |
| B114 | Model/Responses API集成 | EXTEND | ModelService | 禁止ASR Config2 |
| B115 | Voice Provider新建 | NEW_PROVIDER | VoiceProvider | 禁止TTS Config2 |
| B116 | Character能力扩展 | EXTEND | Character | 禁止MemorySystem2 |
| B117 | Character Schedule集成 | EXTEND | Character | 禁止MemoryStore2 |
| B118 | Memory Store/Graph能力扩展 | EXTEND | MemoryStore/MemoryGraph | 禁止VectorStore2 |
| B119 | Memory能力扩展 | EXTEND | Memory | 禁止MemoryGraph2 |
| B120 | Memory Persistence集成 | EXTEND | Memory | 禁止CharacterCore2 |
| B121 | Character Store/Service能力扩展 | EXTEND | CharacterStore/CharacterService | 禁止CharacterDB替代 |
| B122 | Model/Voice/Memory/Character全量接线 | INTEGRATION_ONLY | All | 禁止CharacterService替代 |

## 8. 批次十: B123-B154 (iOS Provider + Integration/Migration/Validation)
| 步骤 | 修订后职责 | 施工模式 | Canonical Target | 禁止项 |
|------|----------|---------|-----------------|--------|
| B123 | iOS Alchemy Provider新建 | NEW_PROVIDER | IOSAlchemyProvider | 禁止IOSSandboxLifecycle2 |
| B124 | iOS Alchemy能力扩展 | EXTEND | IOSAlchemyProvider | 禁止IOSRuntimeManager2 |
| B125 | iOS Tool注册到ToolRegistry | EXTEND | IOSAlchemyProvider | 禁止IOSToolRegistry2 |
| B126 | iOS Permission走PermissionBroker | EXTEND | IOSAlchemyProvider | 禁止IOSPermissionCenter2 |
| B127 | HealthKit Provider新建 | NEW_PROVIDER | HealthKitProvider | 禁止IOSCapabilityRegistry2 |
| B128 | HomeKit Provider新建 | NEW_PROVIDER | HomeKitProvider | 禁止IOSResourceMonitor2 |
| B129 | CoreML Provider新建 | NEW_PROVIDER | CoreMLProvider | 禁止IOSStateTracker2 |
| B130 | CallKit Provider新建 | NEW_PROVIDER | CallKitProvider | 禁止IOSAdapterRegistry2 |
| B131 | NFC Provider新建 | NEW_PROVIDER | NFCProvider | 禁止IOSEventBus2 |
| B132 | ARKit Provider新建 | NEW_PROVIDER | ARKitProvider | 禁止IOSWorkflowEngine2 |
| B133 | PassKit Provider新建 | NEW_PROVIDER | PassKitProvider | 禁止IOSScheduler2 |
| B134 | CarPlay Provider新建 | NEW_PROVIDER | CarPlayProvider | 禁止IOSMemoryManager2 |
| B135 | WatchConnectivity Provider新建 | NEW_PROVIDER | WatchConnectivityProvider | 禁止IOSCharacterService2 |
| B136 | Intents Provider新建 | NEW_PROVIDER | IntentsProvider | 禁止IOSVoiceService2 |
| B137 | iOS Provider全量接线 | REUSE | IOSAlchemyProvider | 禁止IOSModelService2 |
| B138 | iOS Provider与Capability接线 | REUSE | IOSAlchemyProvider | 禁止RuntimeSupervisor2 |
| B139 | 三端统一Adapter接入 | ADAPTER_ONLY | UnifiedCapabilityAdapter | 禁止统一Runtime替代 |
| B140 | Legacy Tool系统迁移 | MIGRATION_ONLY | LegacyToolSystem | 禁止保留Legacy Registry |
| B141 | Legacy Execution Pipeline迁移 | MIGRATION_ONLY | AlternativeExecutionChain | 禁止保留Pipeline副本 |
| B142 | Contract一致性验证 | VALIDATION_ONLY | N/A | 禁止修改实现 |
| B143 | Permission分配一致性验证 | VALIDATION_ONLY | N/A | 禁止修改实现 |
| B144 | Contract兼容性检查 | VALIDATION_ONLY | N/A | 禁止修改实现 |
| B145 | Workflow执行验证 | VALIDATION_ONLY | N/A | 禁止修改实现 |
| B146 | MultiAgent状态验证 | VALIDATION_ONLY | N/A | 禁止修改实现 |
| B147 | Extension生命周期验证 | VALIDATION_ONLY | N/A | 禁止修改实现 |
| B148 | Android适配验证 | VALIDATION_ONLY | N/A | 禁止修改实现 |
| B149 | iOS适配验证 | VALIDATION_ONLY | N/A | 禁止修改实现 |
| B150 | Desktop适配验证 | VALIDATION_ONLY | N/A | 禁止修改实现 |
| B151 | Integration验证/Integration验证 | EXTEND+MIGRATION_ONLY | LegacyAudit | 禁止修改实现 |
| B152 | 迁移State验证 | VALIDATION_ONLY | N/A | 禁止修改实现 |
| B153 | Pipeline唯一性验证 | VALIDATION_ONLY | N/A | 禁止修改实现 |
| B154 | 整体系统验收验证 | VALIDATION_ONLY | N/A | 禁止修改实现 |

## 9. 全局施工统计
- EXTEND: 72步
- NEW_PROVIDER: 51步
- REUSE: 1步
- VALIDATION_ONLY: 11步
- ADAPTER_ONLY: 3步
- INTEGRATION_ONLY: 5步
- MIGRATION_ONLY: 2步

## 10. 全局 FORBIDDEN 动作清单
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
20. 禁止保留Legacy系统注册
21. 禁止长期双写新旧系统
22. 禁止新增Legacy写入
23. 禁止创建V2替代目录
24. 禁止绕过PermissionBroker
25. 禁止绕过ExecutionPipeline
26. 禁止Provider直接暴露为LLM Tool
27. 禁止Adapter创建独立状态事实源
28. 禁止Adapter创建独立权限事实源
29. 禁止为了Parity重新分层Extension Kernel
