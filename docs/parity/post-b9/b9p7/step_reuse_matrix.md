# B9P7 Step Reuse Matrix

- **TaskId**: B9P7
- **总步骤**: 145步 (B10-B154)
- **生成时间**: 2026-08-07
- **schemaVersion**: 1

## 总览

- B10-B154总步骤: 145步
- 已分类: 145 (100%)
- 施工模式统计:
  - EXTEND: 72步
  - NEW_PROVIDER: 51步
  - REUSE: 1步
  - VALIDATION_ONLY: 11步
  - ADAPTER_ONLY: 3步
  - INTEGRATION_ONLY: 5步
  - MIGRATION_ONLY: 2步

## B10-B18 (核心合同层)

| 步骤 | 名称 | 施工模式 | 副模式 | Canonical Target | 风险 | 重复系统风险 |
|------|------|---------|--------|-----------------|------|------------|
| B10 | Tool共享合同 | EXTEND | REUSE | AGENT_TOOL_REGISTRY, TOOL_EXECUTION_PIPELINE | CRITICAL | ToolRegistry2, ToolRuntime2, NewToolSchema |
| B11 | Permission合同 | EXTEND | REUSE | TOOL_PERMISSION_BROKER, AGENT_TOOL_REGISTRY | CRITICAL | PermissionCenter2, PermissionRegistry2, PermissionBroker2 |
| B12 | Runtime合同 | EXTEND | ADAPTER_ONLY | RUNTIME_ADAPTER_REGISTRY, AGENT_TOOL_REGISTRY | CRITICAL | RuntimeAdapter2, RuntimeRegistry2, RuntimeBinding2 |
| B13 | Workspace合同 | EXTEND | - | RESOURCE_URI | CRITICAL | ResourceResolver2, WorkspaceURI2, ResourceScheme2 |
| B14 | Browser Provider | NEW_PROVIDER | EXTEND | AGENT_TOOL_REGISTRY, TOOL_EXECUTION_PIPELINE, RUNTIME_ADAPTER_REGISTRY | CRITICAL | BrowserRuntime2, BrowserEngine2, NewBrowserCore |
| B15 | Media/Device Provider | EXTEND | NEW_PROVIDER+REUSE | AGENT_TOOL_REGISTRY, TOOL_EXECUTION_PIPELINE, IMAGE_PROVIDER | HIGH | MediaProvider2, DeviceProvider2, NewMediaCore |
| B16 | Model/Voice/Automation映射 | REUSE | EXTEND | AGENT_TOOL_REGISTRY, TOOL_EXECUTION_PIPELINE | CRITICAL | ModelConfig2, ModelRouter2, VoiceRuntime2 |
| B17 | Extension Manifest补强 | EXTEND | - | AGENT_TOOL_REGISTRY, RUNTIME_ADAPTER_REGISTRY | CRITICAL | ManifestV3, ExtensionManifest2, NewExtensionSchema |
| B18 | Extension Kernel唯一性验证 | VALIDATION_ONLY | - | N/A | LOW | 所有第二系统 |

## B19-B22 (三端Adapter层)

| 步骤 | 名称 | 施工模式 | 副模式 | Canonical Target | 风险 | 重复系统风险 |
|------|------|---------|--------|-----------------|------|------------|
| B19 | Android Capability Adapter | ADAPTER_ONLY | - | RUNTIME_ADAPTER_REGISTRY, RUNTIME_HOST | CRITICAL | AndroidRuntime2, AndroidBridge2, NewAndroidKernel |
| B20 | iOS Capability Adapter | ADAPTER_ONLY | - | RUNTIME_ADAPTER_REGISTRY, RUNTIME_HOST | CRITICAL | iOSRuntime2, iOSBridge2, NewiOSKernel |
| B21 | Desktop Adapter | ADAPTER_ONLY | - | RUNTIME_ADAPTER_REGISTRY, RUNTIME_HOST | CRITICAL | DesktopRuntime2, DesktopBridge2, NewDesktopKernel |
| B22 | 三端Adapter统一验证 | VALIDATION_ONLY | - | N/A | LOW | 各平台独立Runtime |

## B23-B38 (Agent能力迁移)

| 步骤 | 名称 | 施工模式 | 副模式 | Canonical Target | 风险 | 重复系统风险 |
|------|------|---------|--------|-----------------|------|------------|
| B23 | Planner能力迁移 | EXTEND | REUSE | AGENT_TOOL_REGISTRY, TOOL_EXECUTION_PIPELINE, WORKFLOW_ENGINE | CRITICAL | Planner2, PlanEngine2, NewPlanner |
| B24 | Observer/Reflection迁移 | EXTEND | REUSE | AGENT_TOOL_REGISTRY, MIND_RUNTIME, WORKFLOW_ENGINE | CRITICAL | Observer2, Reflection2, ObserverRuntime2 |
| B25 | TaskGraph迁移 | EXTEND | - | WORKFLOW_ENGINE, AGENT_TOOL_REGISTRY | CRITICAL | WorkflowEngine2, TaskGraph2, NewTaskOrchestration |
| B26 | Background Task迁移 | EXTEND | - | AGENT_TOOL_REGISTRY, TASK_RUNTIME | CRITICAL | TaskRuntime2, BackgroundRuntime2, NewBackgroundEngine |
| B27 | Checkpoint统一 | EXTEND | - | WORKFLOW_ENGINE, TOOL_EXECUTION_PIPELINE | HIGH | CheckpointEngine2, CheckpointStore2, NewCheckpoint |
| B28 | Multi-Agent Coordinator | EXTEND | NEW_COMPONENT(受限) | AGENT_TOOL_REGISTRY, WORKFLOW_ENGINE, TOOL_EXECUTION_PIPELINE, TASK_RUNTIME | CRITICAL | MultiAgentRuntime2, Coordinator2, NewAgentOrchestration |
| B29 | Goal Tracking迁移 | EXTEND | - | WORKFLOW_ENGINE, AGENT_TOOL_REGISTRY | HIGH | GoalEngine2, GoalTracker2, NewGoalSystem |
| B30 | Arbitration迁移 | EXTEND | - | WORKFLOW_ENGINE, AGENT_TOOL_REGISTRY | HIGH | ArbitrationEngine2, Arbiter2, NewArbiter |
| B31 | Scoring迁移 | EXTEND | - | WORKFLOW_ENGINE, AGENT_TOOL_REGISTRY | HIGH | ScoringEngine2, ScoreTracker2, NewScoring |
| B32 | Error Recovery集成 | EXTEND | - | TOOL_EXECUTION_PIPELINE | HIGH | ErrorRecoveryEngine2, ErrorCenter2, NewErrorHandler |
| B33 | Plan Revision集成 | EXTEND | - | AGENT_TOOL_REGISTRY | HIGH | PlanRevisionEngine2, NewRevisionHandler |
| B34 | Tool Selection Hardening | EXTEND | - | AGENT_TOOL_REGISTRY, MODEL_TOOL_FACADE | CRITICAL | ToolSelectionEngine2, NewSelector |
| B35 | Candidate Registration Hardening | EXTEND | - | WORKFLOW_ENGINE, AGENT_TOOL_REGISTRY | HIGH | CandidateEngine2, CandidateRegistry2 |
| B36 | Task Planning Hardening | EXTEND | - | TASK_RUNTIME, WORKFLOW_ENGINE | HIGH | TaskPlanningEngine2, NewTaskPlanner |
| B37 | Decision Consistency | EXTEND | - | MODEL_TOOL_FACADE, AGENT_TOOL_REGISTRY | HIGH | DecisionEngine2, DecisionCenter2, NewDecision |
| B38 | Agent Pipeline最终硬化 | EXTEND | REUSE | TOOL_EXECUTION_PIPELINE | CRITICAL | AgentRuntime2, AgentPipeline2, NewAgentExecutor |

## B39-B54 (Parity Gap Hardening)

| 步骤 | 名称 | 施工模式 | 副模式 | Canonical Target | 风险 | 重复系统风险 |
|------|------|---------|--------|-----------------|------|------------|
| B39 | ToolRegistry Parity Gap硬化 | EXTEND | REUSE | AGENT_TOOL_REGISTRY | CRITICAL | ToolRegistry2, ToolContract2 |
| B40 | ExecutionPipeline Parity Gap硬化 | EXTEND | REUSE | TOOL_EXECUTION_PIPELINE | CRITICAL | ExecutionPipeline2, NewExecutor |
| B41 | PermissionBroker Parity Gap硬化 | EXTEND | REUSE | TOOL_PERMISSION_BROKER | CRITICAL | PermissionCenter2, PermissionRegistry2 |
| B42 | Retry Parity Gap硬化 | EXTEND | - | TOOL_EXECUTION_PIPELINE | CRITICAL | RetryEngine2, NewRetryHandler |
| B43 | Timeout Parity Gap硬化 | EXTEND | - | TOOL_EXECUTION_PIPELINE | CRITICAL | TimeoutEngine2, NewTimeoutHandler |
| B44 | Cancellation Parity Gap硬化 | EXTEND | - | TOOL_EXECUTION_PIPELINE | CRITICAL | CancellationEngine2, NewCancelHandler |
| B45 | RateLimit Parity Gap硬化 | EXTEND | - | TOOL_EXECUTION_PIPELINE | CRITICAL | RateLimitEngine2, NewLimiter |
| B46 | Concurrency Parity Gap硬化 | EXTEND | - | TOOL_EXECUTION_PIPELINE | CRITICAL | ConcurrencyEngine2, NewConcurrencyHandler |
| B47 | Circuit Breaker Parity Gap硬化 | EXTEND | - | TOOL_EXECUTION_PIPELINE | CRITICAL | CircuitBreakerEngine2, NewCircuitBreaker |
| B48 | Idempotency Parity Gap硬化 | EXTEND | - | TOOL_EXECUTION_PIPELINE, AGENT_TOOL_REGISTRY | CRITICAL | IdempotencyEngine2, NewIdempotencyHandler |
| B49 | Audit Parity Gap硬化 | EXTEND | - | TOOL_EXECUTION_PIPELINE | CRITICAL | AuditEngine2, AuditCenter2, NewAudit |
| B50 | Resource Limits Parity Gap硬化 | EXTEND | - | TOOL_EXECUTION_PIPELINE | CRITICAL | ResourceLimitEngine2, NewLimitHandler |
| B51 | Streaming Contract Gap硬化 | EXTEND | - | TOOL_EXECUTION_PIPELINE | HIGH | StreamingEngine2, NewStreamingHandler |
| B52 | Secret Binding Gap硬化 | EXTEND | - | TOOL_PERMISSION_BROKER | HIGH | SecretEngine2, NewSecretHandler |
| B53 | Resource Accounting Gap硬化 | EXTEND | - | TOOL_EXECUTION_PIPELINE | HIGH | AccountingEngine2, NewAccountingHandler |
| B54 | Cross-Provider Cancel Gap硬化 | EXTEND | - | TOOL_EXECUTION_PIPELINE, TASK_RUNTIME | HIGH | CancelEngine2, NewCrossCancel |

## B55-B78 (Desktop/Android/iOS Platform Provider)

| 步骤 | 名称 | 施工模式 | 副模式 | Canonical Target | 风险 | 重复系统风险 |
|------|------|---------|--------|-----------------|------|------------|
| B55 | Android Linux Capability Provider | NEW_PROVIDER | REUSE | RUNTIME_HOST, AGENT_TOOL_REGISTRY | HIGH | AndroidLinuxRuntime2, NewLinuxKernel |
| B56 | Terminal Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | TerminalRuntime2, TerminalEngine2 |
| B57 | Shell Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | ShellRuntime2, ShellEngine2 |
| B58 | Package Manager Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | PackageRuntime2, PackageEngine2 |
| B59 | Script Runtime Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | ScriptRuntime2, ScriptEngine2 |
| B60 | Network Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | NetworkRuntime2, NetworkEngine2 |
| B61 | Archive Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | ArchiveRuntime2, ArchiveEngine2 |
| B62 | Android Accessibility Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | CRITICAL | AndroidNativeRuntime2, AccessibilityEngine2 |
| B63 | ADB Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | HIGH | ADBRuntime2, ADBEngine2 |
| B64 | Root Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | HIGH | RootRuntime2, RootEngine2 |
| B65 | UI Tree Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | HIGH | UITreeEngine2, UITreeRuntime2 |
| B66 | Visual Click Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | VisualClickEngine2, NewVisualClick |
| B67 | Virtual Display Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | VirtualDisplayEngine2, NewVirtualDisplay |
| B68 | Multi Display Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | MultiDisplayEngine2, NewMultiDisplay |
| B69 | Screenshot Provider | NEW_PROVIDER | REUSE | IMAGE_PROVIDER, RUNTIME_HOST | MEDIUM | ScreenshotEngine2, NewScreenshot |
| B70 | Screen Frame Provider | NEW_PROVIDER | REUSE | IMAGE_PROVIDER, RUNTIME_HOST | MEDIUM | ScreenFrameEngine2, NewScreenFrame |
| B71 | Camera Provider | NEW_PROVIDER | REUSE | IMAGE_PROVIDER, RUNTIME_HOST | MEDIUM | CameraEngine2, NewCamera |
| B72 | FFmpeg Media Provider | NEW_PROVIDER | REUSE | IMAGE_PROVIDER, RUNTIME_HOST | MEDIUM | FFmpegEngine2, NewFFmpeg |
| B73 | Media Metadata Provider | NEW_PROVIDER | REUSE | IMAGE_PROVIDER, RUNTIME_HOST | MEDIUM | MediaMetadataEngine2, NewMetadata |
| B74 | Notification Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | HIGH | NotificationEngine2, NewNotification |
| B75 | Clipboard Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | ClipboardEngine2, NewClipboard |
| B76 | Share Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | ShareEngine2, NewShare |
| B77 | Overlay Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | OverlayEngine2, NewOverlay |
| B78 | External Automation Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | ExternalAutomationEngine2, NewAutoBot |

## B79-B92 (Browser/Search/Media/Workspace)

| 步骤 | 名称 | 施工模式 | 副模式 | Canonical Target | 风险 | 重复系统风险 |
|------|------|---------|--------|-----------------|------|------------|
| B79 | Browser Provider | NEW_PROVIDER | EXTEND | AGENT_TOOL_REGISTRY, TOOL_EXECUTION_PIPELINE | CRITICAL | BrowserProvider2, BrowserTool2, NewBrowser |
| B80 | Browser Runtime | NEW_PROVIDER | EXTEND | AGENT_TOOL_REGISTRY, RUNTIME_ADAPTER_REGISTRY | CRITICAL | BrowserRuntime2, NewBrowserRuntime |
| B81 | Browser Session | NEW_PROVIDER | EXTEND | AGENT_TOOL_REGISTRY | CRITICAL | BrowserSession2, NewBrowserSession |
| B82 | Tab State | NEW_PROVIDER | EXTEND | AGENT_TOOL_REGISTRY | HIGH | TabStateEngine2, NewTabSession |
| B83 | DOM Adapter | NEW_PROVIDER | EXTEND | RUNTIME_ADAPTER_REGISTRY | HIGH | DOMEngine2, NewDOMAdapter |
| B84 | Web Search Provider | NEW_PROVIDER | - | AGENT_TOOL_REGISTRY, TOOL_EXECUTION_PIPELINE | HIGH | SearchEngine2, NewWebSearch |
| B85 | Deep Search Provider | NEW_PROVIDER | - | AGENT_TOOL_REGISTRY, TOOL_EXECUTION_PIPELINE | MEDIUM | DeepSearchEngine2, NewDeepSearch |
| B86 | Academic/Map/Image Search | NEW_PROVIDER | - | AGENT_TOOL_REGISTRY, TOOL_EXECUTION_PIPELINE | MEDIUM | AcademicSearchEngine2, MapSearchEngine2, ImageSearchEngine2 |
| B87 | Image/Voice Unified Media Adapter | EXTEND | NEW_PROVIDER+REUSE | IMAGE_PROVIDER, AGENT_TOOL_REGISTRY | MEDIUM | UnifiedMediaEngine2, NewMediaAdapter |
| B88 | Unified Media Format Conversion | EXTEND | REUSE | IMAGE_PROVIDER, AGENT_TOOL_REGISTRY | MEDIUM | MediaFormatEngine2, NewMediaFormat |
| B89 | Media Metadata Extraction | EXTEND | REUSE | IMAGE_PROVIDER, AGENT_TOOL_REGISTRY | MEDIUM | MetadataEngine2, NewMetadataExtractor |
| B90 | SAF Provider | EXTEND | NEW_PROVIDER | RESOURCE_URI, AGENT_TOOL_REGISTRY | MEDIUM | SAFEngine2, NewSAFRuntime |
| B91 | SFTP Provider | EXTEND | NEW_PROVIDER | RESOURCE_URI, AGENT_TOOL_REGISTRY | MEDIUM | SFTPRuntime2, NewSFTPEngine |
| B92 | SSH/Git Workspace Adapter | EXTEND | NEW_PROVIDER | RESOURCE_URI, AGENT_TOOL_REGISTRY | MEDIUM | SSHGitRuntime2, NewWorkspaceAdapter |

## B93-B110 (MCP/Skill/Workflow/Hook/Schedule)

| 步骤 | 名称 | 施工模式 | 副模式 | Canonical Target | 风险 | 重复系统风险 |
|------|------|---------|--------|-----------------|------|------------|
| B93 | MCP stdio transport补完 | EXTEND | - | TOOL_EXECUTION_PIPELINE, AGENT_TOOL_REGISTRY, MODEL_TOOL_FACADE | CRITICAL | MCPTransport2, NewMCPManager, MCPManager2 |
| B94 | MCP HTTP/SSE transport补完 | EXTEND | - | TOOL_EXECUTION_PIPELINE, AGENT_TOOL_REGISTRY | CRITICAL | MCPHTTPMCP2, NewMCPHTTP |
| B95 | MCP npx/uvx installation补完 | EXTEND | - | TOOL_EXECUTION_PIPELINE, AGENT_TOOL_REGISTRY | CRITICAL | MCPInstallEngine2, NewMCPInstall |
| B96 | MCP health monitoring补完 | EXTEND | - | TOOL_EXECUTION_PIPELINE | HIGH | MCPHealthEngine2, NewMCPHealth |
| B97 | MCP ToolFacade sync硬化 | EXTEND | - | MODEL_TOOL_FACADE, AGENT_TOOL_REGISTRY | CRITICAL | MCPToolFacadeSync2, MCPToolSync2, NewMCPToolSync |
| B98 | Skill SKILL.md加载补完 | EXTEND | - | AGENT_SKILL_SYSTEM, AGENT_TOOL_REGISTRY | CRITICAL | SkillRuntime2, SkillLoader2, NewSkillSystem |
| B99 | Progressive Loading补完 | EXTEND | - | AGENT_SKILL_SYSTEM | HIGH | ProgressiveLoadEngine2, NewProgressiveLoad |
| B100 | Skill scripts/references补完 | EXTEND | - | AGENT_SKILL_SYSTEM | HIGH | SkillScriptEngine2, NewSkillScript |
| B101 | Cross-ecosystem Skill Import | EXTEND | - | AGENT_SKILL_SYSTEM | HIGH | SkillImportEngine2, NewSkillImport |
| B102 | Skill security sandbox补完 | EXTEND | - | AGENT_SKILL_SYSTEM, TOOL_PERMISSION_BROKER | HIGH | SkillSandboxEngine2, NewSkillSandbox |
| B103 | Workflow state补完 | EXTEND | - | WORKFLOW_ENGINE | CRITICAL | WorkflowStateEngine2, WorkflowStore2, NewWorkflowState |
| B104 | Workflow schedule补完 | EXTEND | - | WORKFLOW_ENGINE | CRITICAL | WorkflowScheduleEngine2, NewWorkflowSchedule |
| B105 | Workflow parallel branch补完 | EXTEND | - | WORKFLOW_ENGINE | HIGH | WorkflowBranchEngine2, NewWorkflowBranch |
| B106 | Hook/Event统一 | EXTEND | - | WORKFLOW_ENGINE | CRITICAL | HookEngine2, EventEngine2, NewHookSystem |
| B107 | Scheduler timezone补完 | EXTEND | - | WORKFLOW_ENGINE | HIGH | SchedulerEngine2, NewScheduler, TimezoneEngine2 |
| B108 | Task Runtime trigger补完 | EXTEND | - | TASK_RUNTIME | HIGH | TaskTriggerEngine2, NewTaskTrigger |
| B109 | JS/WASM Runtime补强 | EXTEND | - | TOOL_EXECUTION_PIPELINE | HIGH | JSWASMRuntime2, NewScriptRuntime |
| B110 | Stream/Async execution补完 | EXTEND | - | TOOL_EXECUTION_PIPELINE | HIGH | StreamExecutionEngine2, NewAsyncExecutor |

## B111-B122 (Model/Voice/Memory/Character)

| 步骤 | 名称 | 施工模式 | 副模式 | Canonical Target | 风险 | 重复系统风险 |
|------|------|---------|--------|-----------------|------|------------|
| B111 | Responses API Adapter | EXTEND | NEW_PROVIDER+REUSE | AGENT_TOOL_REGISTRY, TOOL_EXECUTION_PIPELINE | CRITICAL | ResponsesEngine2, NewResponsesAPI, ModelConfig2 |
| B112 | Missing Cloud Provider Adapter | NEW_PROVIDER | - | AGENT_TOOL_REGISTRY | MEDIUM | CloudEngine2, NewCloudRuntime, ModelRouter2 |
| B113 | MNN/llama.cpp Local Provider | NEW_PROVIDER | - | AGENT_TOOL_REGISTRY | HIGH | MNNEngine2, LlamaCppEngine2, NewLocalModel |
| B114 | Local Embedding Provider | EXTEND | REUSE | AGENT_TOOL_REGISTRY | MEDIUM | EmbeddingEngine2, NewEmbeddingRuntime |
| B115 | Continuous Voice Provider | EXTEND | REUSE | AGENT_TOOL_REGISTRY | HIGH | ContinuousVoiceEngine2, VoiceRuntime2 |
| B116 | Wake Word Provider | EXTEND | REUSE | AGENT_TOOL_REGISTRY | HIGH | WakeWordEngine2, NewWakeWord |
| B117 | Missing STT/TTS Provider | EXTEND | REUSE | AGENT_TOOL_REGISTRY | MEDIUM | STTTSPEngine2, NewVoiceRuntime |
| B118 | Memory Category/Tag补完 | EXTEND | REUSE | AGENT_TOOL_REGISTRY | CRITICAL | MemorySystem2, MemoryCategory2, NewMemory |
| B119 | Temporal Query补完 | EXTEND | REUSE | N/A | HIGH | TemporalEngine2, NewTemporalQuery |
| B120 | Memory Summarization/ImportExport | EXTEND | REUSE | N/A | HIGH | MemorySummaryEngine2, NewMemoryIO |
| B121 | Character Card V2升级 | EXTEND | REUSE | AGENT_TOOL_REGISTRY | CRITICAL | CharacterCore2, CharacterDB2, NewCharacter |
| B122 | Character Backup/Migration | EXTEND | REUSE | N/A | HIGH | CharacterBackupEngine2, NewCharacterBackup |

## B123-B138 (iOS Sandbox/Native Provider)

| 步骤 | 名称 | 施工模式 | 副模式 | Canonical Target | 风险 | 重复系统风险 |
|------|------|---------|--------|-----------------|------|------------|
| B123 | iOS Sandbox Provider(iSH) | NEW_PROVIDER | ADAPTER_ONLY | RUNTIME_HOST, RUNTIME_ADAPTER_REGISTRY, AGENT_TOOL_REGISTRY | HIGH | iSHRuntime2, NewiOSKernel |
| B124 | iOS Alpine rootfs Provider | NEW_PROVIDER | ADAPTER_ONLY | RUNTIME_HOST, AGENT_TOOL_REGISTRY | HIGH | AlpineRootfsRuntime2, NewAlpineRuntime |
| B125 | iOS Sandbox File Provider | NEW_PROVIDER | ADAPTER_ONLY | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | SandboxFileRuntime2, NewSandboxFile |
| B126 | iOS Sandbox Network Proxy | NEW_PROVIDER | ADAPTER_ONLY | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | SandboxNetworkRuntime2, NewSandboxProxy |
| B127 | iOS Health Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | HIGH | HealthEngine2, NewHealthRuntime |
| B128 | iOS Calendar Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | CalendarEngine2, NewCalendarRuntime |
| B129 | iOS Reminders Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | RemindersEngine2, NewRemindersRuntime |
| B130 | iOS Contacts Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | ContactsEngine2, NewContactsRuntime |
| B131 | iOS HomeKit Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | HomeKitEngine2, NewHomeKitRuntime |
| B132 | iOS Bluetooth Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | BluetoothEngine2, NewBluetoothRuntime |
| B133 | iOS Clipboard/Media Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | iOSClipboardRuntime2, iOSMediaRuntime2 |
| B134 | iOS Alarms Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | LOW | AlarmsEngine2, NewAlarmsRuntime |
| B135 | iOS Share Sheet Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | ShareSheetEngine2, NewShareSheet |
| B136 | iOS Shortcuts Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | ShortcutsEngine2, NewShortcuts |
| B137 | iOS Background Task Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | BackgroundTaskEngine2, NewBackgroundRuntime |
| B138 | iOS File Provider | NEW_PROVIDER | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | MEDIUM | iOSFileRuntime2, NewFileProvider |

## B139-B154 (Integration/Migration/Validation)

| 步骤 | 名称 | 施工模式 | 副模式 | Canonical Target | 风险 | 重复系统风险 |
|------|------|---------|--------|-----------------|------|------------|
| B139 | Android/iOS/Desktop统一Runtime接入 | INTEGRATION_ONLY | - | RUNTIME_HOST, AGENT_TOOL_REGISTRY | CRITICAL | RuntimeOrchestrator2, RuntimeHost2, NewRuntime |
| B140 | AgentToolFacade最终Cutover | MIGRATION_ONLY | INTEGRATION_ONLY | MODEL_TOOL_FACADE, AGENT_TOOL_REGISTRY | CRITICAL | AgentRuntime2, LegacyAgentTool2, NewAgentTool |
| B141 | ExecutionPipeline唯一生产链 | MIGRATION_ONLY | INTEGRATION_ONLY | TOOL_EXECUTION_PIPELINE, AGENT_TOOL_REGISTRY | CRITICAL | ExecutionPipeline2, LegacyExecutor2, NewExecutor |
| B142 | 全链路生产验证 | VALIDATION_ONLY | - | N/A | LOW | - |
| B143 | 容错/恢复力验证 | VALIDATION_ONLY | - | N/A | LOW | - |
| B144 | B55-78 Platform Provider接入验收 | INTEGRATION_ONLY | - | N/A | LOW | - |
| B145 | Browser/Search/Media/Workspace验收 | INTEGRATION_ONLY | - | N/A | LOW | - |
| B146 | MCP/Skill/Workflow总验收 | VALIDATION_ONLY | - | N/A | LOW | - |
| B147 | Model/Voice/Memory/Character增量验收 | INTEGRATION_ONLY | VALIDATION_ONLY | N/A | LOW | - |
| B148 | iOS Provider接入验收 | INTEGRATION_ONLY | - | N/A | LOW | - |
| B149 | Operit Parity最终验收 | VALIDATION_ONLY | - | N/A | LOW | - |
| B150 | OpenMinis Parity最终验收 | VALIDATION_ONLY | - | N/A | LOW | - |
| B151 | 零Legacy/Mock/TODO扫描 | VALIDATION_ONLY | MIGRATION_ONLY | N/A | CRITICAL | Legacy残留, Mock残留, TODO残留 |
| B152 | Android真机全量验收 | VALIDATION_ONLY | - | N/A | LOW | - |
| B153 | iOS真机全量验收 | VALIDATION_ONLY | - | N/A | LOW | - |
| B154 | 最终冻结验收(100% Parity) | VALIDATION_ONLY | - | N/A | LOW | - |

---

## 第二套系统禁止清单 (GLOBAL FORBID)

| 禁止类别 | 典型示例 |
|---------|---------|
| Tool Registry重复 | ToolRegistry2 / CapabilityRegistry2 / ToolFacade2 |
| Execution Kernel重复 | ExecutionPipeline2 / ExecutionKernel2 |
| Permission Broker重复 | PermissionCenter2 / PermissionRegistry2 / PermissionBroker2 |
| Agent Runtime重复 | AgentRuntime2 / AgentExecutionEngine2 |
| Task Runtime重复 | TaskRuntime2 / TaskExecutionEngine2 |
| Workflow Engine重复 | WorkflowEngine2 / WorkflowOrchestrator2 |
| MCP Tool Exposure重复 | MCPManager2 / MCPToolManager2 |
| Skill Runtime重复 | SkillRuntime2 / SkillCatalog2 |
| Memory System重复 | MemorySystem2 / VectorStore2 / MemoryGraph2 |
| Model Config重复 | ModelConfig2 / ModelRouter2 / ModelService2 |
| Voice Runtime重复 | VoiceRuntime2 / ASRConfig2 / TTSConfig2 |
| Character Core重复 | CharacterCore2 / CharacterDB2 |
| Workspace重复 | WorkspaceURI2 / ResourceResolver2 / ResourceURI2 |
| Runtime Host重复 | RuntimeOrchestrator2 / RuntimeHost2 |
| State Store重复 | GlobalRuntimeState2 / StateStore2 |
| Error Registry重复 | ErrorCenter2 / ErrorRegistry2 |

---

## Canonical Systems 路径索引

| SystemId | 主要路径 | 可用施工模式 |
|---------|---------|------------|
| MODEL_TOOL_FACADE | backend/internal/extension/kernel/tool_facade.go | REUSE/EXTEND |
| AGENT_TOOL_REGISTRY | backend/internal/extension/kernel/capability/registry.go | REUSE/EXTEND |
| TOOL_EXECUTION_PIPELINE | backend/internal/extension/kernel/execution/pipeline.go | EXTEND |
| TOOL_PERMISSION_BROKER | backend/internal/extension/kernel/permission/broker.go | EXTEND |
| RUNTIME_ADAPTER_REGISTRY | backend/internal/extension/kernel/capability/ | EXTEND/ADAPTER_ONLY |
| WORKFLOW_ENGINE | backend/internal/extension/kernel/workflow/ | EXTEND |
| AGENT_SKILL_SYSTEM | backend/internal/extension/kernel/agent_skill/ | EXTEND |
| IMAGE_PROVIDER | backend/internal/imageprovider/ | REUSE/EXTEND |
| RESOURCE_URI | backend/pkg/resourceuri/ | EXTEND |
| RUNTIME_HOST | backend/internal/runtimeorchestrator/ | EXTEND/REUSE |
| MIND_RUNTIME | backend/internal/mindruntime/ | EXTEND |
| TASK_RUNTIME | backend/internal/extension/kernel/task_runtime/ | EXTEND |
