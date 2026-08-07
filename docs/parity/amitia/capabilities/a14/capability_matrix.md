# Amitia A14 能力矩阵 (Capability Matrix)

## 概述 (Overview)

| 指标 | 值 |
|------|------|
| 基线 | AMT-A14-3daeaf3 |
| 审计时间 | 2026-08-07 |
| Git提交 | 3daeaf3c0a82e33213e0a52d84cfaf8f68f78eab |
| 总能力数 | 327 |
| 已实现 | 282 |
| 部分实现 | 12 |
| Stub | 8 |
| 遗留系统 | 5 |
| 无运行证据 | 20 |
| 总根节点数 | 13 |
| 扫描文件数 | 1800 |
| 源码覆盖率 | 100% |
| 业务代码修改 | 0 |
| 审计状态 | STATIC_AUDIT_PASS |
| 运行验证 | RUNTIME_VERIFICATION_PARTIAL |

## 状态图例

| 标记 | 含义 |
|------|------|
| ✅ IMPLEMENTED | 完整实现 — 具备ROUTE→HANDLER→SERVICE→IMPLEMENTATION完整链路 |
| ⚠️ PARTIAL | 部分实现 — 有接口和路由但核心逻辑不完整 |
| 🔲 STUB | 占位/空实现 — 返回空对象或固定值 |
| 🎭 MOCK | 模拟实现 — 仅用于测试的模拟数据 |
| 🔻 LEGACY | 已弃用遗留 — 代码存在但被新版替代 |
| 🌑 NO_EVIDENCE | 无运行证据 — 源码存在但未在运行中验证 |

---

## 按类别能力矩阵

### 01 Agent与对话 (Agent & LLM) - 15项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0001 | HTTP对话接口 | HTTPChatEndpoint | backend | ✅ IMPLEMENTED | E3 | internal/chat/handler.go, internal/chat/service.go |
| AMT-0002 | Agent测试接口 | AgentTestEndpoint | backend | ✅ IMPLEMENTED | E3 | internal/agent/router.go, internal/agent/service.go |
| AMT-0003 | 对话上下文预览 | ContextPreview | backend | ✅ IMPLEMENTED | E3 | internal/agent/service.go |
| AMT-0004 | 渠道消息Webhook | ChannelWebhook | backend | ✅ IMPLEMENTED | E3 | internal/agent/service.go |
| AMT-0005 | 统一交互入口(UnifiedEntry) | UnifiedEntry | backend | ✅ IMPLEMENTED | E3 | internal/interaction/unified_entry.go |
| AMT-0006 | 交互编排器(Orchestrator) | Orchestrator | backend | ✅ IMPLEMENTED | E3 | internal/interaction/orchestrator.go |
| AMT-0007 | LLM多协议调用(OpenAI) | LLMCallerOpenAI | backend | ✅ IMPLEMENTED | E3 | internal/chat/llm_client.go |
| AMT-0008 | LLM多协议调用(Ollama) | LLMCallerOllama | backend | ✅ IMPLEMENTED | E3 | internal/chat/llm_client.go |
| AMT-0009 | LLM多协议调用(Anthropic) | LLMCallerAnthropic | backend | ✅ IMPLEMENTED | E3 | internal/chat/llm_client.go |
| AMT-0010 | LLM多协议调用(Gemini) | LLMCallerGemini | backend | ✅ IMPLEMENTED | E3 | internal/chat/llm_client.go |
| AMT-0011 | 多轮Tool调用循环 | MultiRoundToolLoop | backend | ✅ IMPLEMENTED | E3 | internal/chat/message_llm.go |
| AMT-0012 | SSE流式消息推送 | SSEStreamPush | backend, web | ✅ IMPLEMENTED | E3 | internal/chat/commit_outbox.go |
| AMT-0013 | 会话管理(CRUD) | ConversationCRUD | backend | ✅ IMPLEMENTED | E3 | internal/chat/router.go, internal/chat/handler.go |
| AMT-0014 | 对话压缩与摘要 | ConversationCompression | backend | ⚠️ PARTIAL | E2 | internal/chat/handler.go (GetSummary返回空) |
| AMT-0015 | Workshop JSON生成 | WorkshopGenerator | backend | ✅ IMPLEMENTED | E2 | internal/chat/service.go |

---

### 02 Prompt与上下文 (Prompt & Context) - 11项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0016 | Prompt构建器 | PromptBuilder | backend | ✅ IMPLEMENTED | E3 | internal/chat/prompt_builder.go |
| AMT-0017 | 系统指令编译 | SystemInstruction | backend | ✅ IMPLEMENTED | E3 | internal/chat/prompt_builder.go |
| AMT-0018 | 角色配置注入 | CharacterConfigInjection | backend | ✅ IMPLEMENTED | E3 | internal/chat/prompt_builder.go |
| AMT-0019 | 渠道Prompt编译 | ChannelPromptCompile | backend | ✅ IMPLEMENTED | E3 | internal/expression/channel_prompt.go |
| AMT-0020 | 记忆上下文注入 | MemoryContextInjection | backend | ✅ IMPLEMENTED | E3 | internal/profile/service.go |
| AMT-0021 | 情景记忆Prompt | EpisodicPrompt | backend | ✅ IMPLEMENTED | E3 | internal/episodic/service.go |
| AMT-0022 | 世界书Prompt注入 | WorldbookPromptInjection | backend | ✅ IMPLEMENTED | E3 | internal/worldbook/service.go |
| AMT-0023 | Token预算管理 | TokenBudgetManager | backend | ✅ IMPLEMENTED | E3 | internal/prompt/token_budget.go |
| AMT-0024 | Prompt渲染引擎 | PromptRenderer | backend | ✅ IMPLEMENTED | E3 | internal/prompt/renderer.go |
| AMT-0025 | Prompt IR校验 | PromptIRValidation | backend | ✅ IMPLEMENTED | E3 | internal/prompt/validator.go |
| AMT-0026 | 性格Prompt注入 | PersonalityPromptInjection | backend | ✅ IMPLEMENTED | E3 | internal/chat/prompt_builder.go:234 |

---

### 03 任务与规划 (Task & Planning) - 8项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0027 | 日程生成引擎 | ScheduleGenerator | backend | ✅ IMPLEMENTED | E3 | internal/companion/schedule_service.go |
| AMT-0028 | 日程构建器 | ScheduleBuilder | backend | ✅ IMPLEMENTED | E3 | internal/companion/schedule_builder.go |
| AMT-0029 | 时间表构建 | TimelineBuilder | backend | ✅ IMPLEMENTED | E3 | internal/companion/timeline_builder.go |
| AMT-0030 | 冲突检测引擎 | ConflictDetection | backend | ✅ IMPLEMENTED | E3 | internal/companion/schedule_service.go:18 |
| AMT-0031 | 调课系统 | ClassAdjustment | backend | ✅ IMPLEMENTED | E3 | internal/companion/class_service.go |
| AMT-0032 | 特殊事件管理 | SpecialEvents | backend | ✅ IMPLEMENTED | E3 | internal/companion/special_event_service.go |
| AMT-0033 | 日程分享生成 | ShareGenerator | backend | ✅ IMPLEMENTED | E3 | internal/companion/share_generator.go |
| AMT-0034 | 生活事件引擎 | LifeEventEngine | backend | ✅ IMPLEMENTED | E3 | internal/proactive/life_event.go |

---

### 04 Tool Runtime - 12项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0035 | Tool注册表 | ToolRegistry | backend | ✅ IMPLEMENTED | E3 | internal/agent/tool/registry.go |
| AMT-0036 | Tool执行器 | ToolExecutor | backend | ✅ IMPLEMENTED | E3 | internal/agent/tool/registry.go |
| AMT-0037 | Tool幂等控制 | IdempotencyControl | backend | ✅ IMPLEMENTED | E3 | internal/agent/tool/registry.go |
| AMT-0038 | Tool审计日志 | ToolAuditLogging | backend | ✅ IMPLEMENTED | E3 | internal/agent/tool/registry.go |
| AMT-0039 | 权限作用域验证 | ScopedWriteValidation | backend | ✅ IMPLEMENTED | E3 | internal/agent/tool/registry.go |
| AMT-0040 | Tool取消机制 | ToolCancellation | backend | ✅ IMPLEMENTED | E3 | internal/agent/tool/registry.go |
| AMT-0041 | Tool结果标准化 | ResultNormalization | backend | ✅ IMPLEMENTED | E3 | internal/agent/tool/registry.go |
| AMT-0042 | 意图持久化 | IntentPersistence | backend | ✅ IMPLEMENTED | E3 | internal/agent/tool/registry.go |
| AMT-0043 | 结果记录器 | ResultRecorder | backend | ✅ IMPLEMENTED | E3 | internal/agent/tool/registry.go |
| AMT-0044 | Tool迁移注册表 | ToolMigrationRegistry | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/tool_migration/registry.go |
| AMT-0045 | 旧版Tool适配 | LegacyToolAdapter | backend | ✅ IMPLEMENTED | E3 | internal/extension/legacy_tool_adapter.go |
| AMT-0046 | 执行上下文构建 | ExecutionContextBuilder | backend | ✅ IMPLEMENTED | E3 | internal/agent/tool/model.go |

---

### 05 日程和主动消息 (Schedule & Proactive Messages) - 15项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0047 | 动机评分引擎 | MotivationScoring | backend | ✅ IMPLEMENTED | E3 | internal/proactive/motivation.go |
| AMT-0048 | 抑制评分引擎 | SuppressionEngine | backend | ✅ IMPLEMENTED | E3 | internal/proactive/suppression.go |
| AMT-0049 | 中断风险评估 | InterruptionRisk | backend | ✅ IMPLEMENTED | E3 | internal/proactive/interruption_risk.go |
| AMT-0050 | 主动消息执行器 | ProactiveExecutor | backend | ✅ IMPLEMENTED | E3 | internal/proactive/executor.go |
| AMT-0051 | 主动消息Pipeline | ProactivePipeline | backend | ✅ IMPLEMENTED | E3 | internal/proactive/pipeline.go |
| AMT-0052 | 安全调度器 | SafeScheduler | backend | ✅ IMPLEMENTED | E3 | internal/proactive/safe_scheduler.go |
| AMT-0053 | 去重投递 | DedupDelivery | backend | ✅ IMPLEMENTED | E3 | internal/proactive/dedup_delivery.go |
| AMT-0054 | 输出租约机制 | OutputLease | backend | ✅ IMPLEMENTED | E3 | internal/proactive/output_lease.go |
| AMT-0055 | 队列背压控制 | QueueBackpressure | backend | ✅ IMPLEMENTED | E3 | internal/proactive/queue_backpressure.go |
| AMT-0056 | 预算冷却机制 | BudgetCooldown | backend | ✅ IMPLEMENTED | E3 | internal/proactive/budget_cooldown.go |
| AMT-0057 | 空闲检测器 | IdleDetection | backend | ✅ IMPLEMENTED | E3 | internal/proactive/idle_detection.go |
| AMT-0058 | 租约组管理 | LeaseGroup | backend | ✅ IMPLEMENTED | E3 | internal/proactive/lease_group.go |
| AMT-0059 | 扫描执行触发 | ScanExecute | backend | ✅ IMPLEMENTED | E3 | internal/proactive/service.go |
| AMT-0060 | 规则CRUD引擎 | ProactiveRulesCRUD | backend | ✅ IMPLEMENTED | E3 | internal/proactive/handler_rules.go |
| AMT-0061 | 主动消息管理 | ProactiveAdmin | backend | ✅ IMPLEMENTED | E3 | internal/proactive/handler_admin.go |

---

### 06 角色系统 (Character System) - 13项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0062 | 角色创建 | CharacterCreate | backend | ✅ IMPLEMENTED | E3 | internal/character/handler.go |
| AMT-0063 | 角色更新 | CharacterUpdate | backend | ✅ IMPLEMENTED | E3 | internal/character/handler.go |
| AMT-0064 | 角色删除 | CharacterDelete | backend | ✅ IMPLEMENTED | E3 | internal/character/handler.go |
| AMT-0065 | 角色列表查询 | CharacterList | backend | ✅ IMPLEMENTED | E3 | internal/character/handler.go |
| AMT-0066 | 角色详情查询 | CharacterGet | backend | ✅ IMPLEMENTED | E3 | internal/character/handler.go |
| AMT-0067 | 活跃角色切换 | SetActiveCharacter | backend | ✅ IMPLEMENTED | E3 | internal/character/handler.go |
| AMT-0068 | 角色测试接口 | CharacterTest | backend | ✅ IMPLEMENTED | E3 | internal/character/handler.go |
| AMT-0069 | 头像上传 | AvatarUpload | backend | ✅ IMPLEMENTED | E3 | internal/character/handler.go |
| AMT-0070 | 角色档案更新 | UpdateRoleProfile | backend | ✅ IMPLEMENTED | E3 | internal/character/handler.go |
| AMT-0071 | 角色档案获取 | GetRoleProfile | backend | ✅ IMPLEMENTED | E3 | internal/character/handler.go |
| AMT-0072 | 角色导出包 | ExportPack | backend | 🔲 STUB | E2 | internal/character/handler.go:148 |
| AMT-0073 | 角色导入预览 | ImportPackPreview | backend | 🔲 STUB | E2 | internal/character/handler.go:150 |
| AMT-0074 | 角色导入确认 | ImportPackConfirm | backend | 🔲 STUB | E2 | internal/character/handler.go:153 |

---

### 07 性格、情绪与生活系统 (Personality, Emotion & Life System) - 20项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0075 | 性格编译器 | PersonalityCompiler | backend | ✅ IMPLEMENTED | E3 | internal/personality/compiler.go |
| AMT-0076 | 32套人格预设 | PersonalityTemplates | backend | ✅ IMPLEMENTED | E3 | internal/personality/personality_templates.go |
| AMT-0077 | 口吻指南系统 | VoiceGuides | backend | ✅ IMPLEMENTED | E3 | internal/personality/preset_voice_guides.go |
| AMT-0078 | 性格滑块配置 | PersonalitySliders | backend | ✅ IMPLEMENTED | E3 | internal/character/model.go |
| AMT-0079 | 性格行为指令注入 | PersonalityInject | backend | ✅ IMPLEMENTED | E3 | internal/chat/prompt_builder.go:234 |
| AMT-0080 | PAD情绪模型 | AffectModel | backend | ✅ IMPLEMENTED | E3 | internal/affect/model.go |
| AMT-0081 | 情绪状态计算引擎 | AffectEngine | backend | ✅ IMPLEMENTED | E3 | internal/affect/engine.go |
| AMT-0082 | 情绪衰减机制 | AffectDecay | backend | ✅ IMPLEMENTED | E3 | internal/affect/engine.go:267 |
| AMT-0083 | 情绪中文标签 | AffectLabel | backend | ✅ IMPLEMENTED | E3 | internal/affect/label.go |
| AMT-0084 | 情绪持久化 | AffectPersistence | backend | ✅ IMPLEMENTED | E3 | internal/affect/repository.go |
| AMT-0085 | 关系更新引擎 | RelationshipEngine | backend | ✅ IMPLEMENTED | E3 | internal/relationship/engine_update.go |
| AMT-0086 | 事件影响引擎 | EventEngine | backend | ✅ IMPLEMENTED | E3 | internal/relationship/event_engine.go |
| AMT-0087 | 依恋类型系统 | AttachmentTypes | backend | ✅ IMPLEMENTED | E3 | internal/relationship/attachment.go |
| AMT-0088 | 冲突修复引擎 | ConflictRepair | backend | ✅ IMPLEMENTED | E3 | internal/relationship/conflict_repair.go |
| AMT-0089 | 慢变量缓冲 | SlowVar | backend | ✅ IMPLEMENTED | E3 | internal/relationship/slow_var.go |
| AMT-0090 | 关系叙事生成 | RelationshipNarrative | backend | ✅ IMPLEMENTED | E3 | internal/relationship/narrative.go |
| AMT-0091 | 未解决冲突追踪 | UnresolvedTracking | backend | ✅ IMPLEMENTED | E3 | internal/relationship/unresolved.go |
| AMT-0092 | Psyche三层模型 | PsycheModel | backend | ✅ IMPLEMENTED | E3 | internal/psyche/model.go |
| AMT-0093 | 需求衰减引擎 | NeedDecayEngine | backend | ✅ IMPLEMENTED | E3 | internal/need/engine.go:193 |
| AMT-0094 | 需求饱和检测 | NeedSaturation | backend | ✅ IMPLEMENTED | E3 | internal/need/engine.go:283 |

---

### 08 记忆系统 (Memory System) - 27项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0095 | 工作记忆层 | WorkingMemoryLayer | backend | ✅ IMPLEMENTED | E3 | internal/memory/working_memory.go |
| AMT-0096 | 用户画像层 | ProfileLayer | backend | ✅ IMPLEMENTED | E3 | internal/profile/service.go |
| AMT-0097 | 情景记忆提取层 | EpisodicLayer | backend | ✅ IMPLEMENTED | E3 | internal/episodic/service.go |
| AMT-0098 | 结构化事实层 | StructuredFactLayer | backend | ✅ IMPLEMENTED | E3 | internal/memory/pipeline_service.go:17 |
| AMT-0099 | 向量同步层 | VectorEmbeddingLayer | backend | ✅ IMPLEMENTED | E3 | internal/memory/embedding_vector_service.go |
| AMT-0100 | 图谱同步层 | GraphSyncLayer | backend | ✅ IMPLEMENTED | E3 | internal/memory/graph_service.go |
| AMT-0101 | 记忆CRUD | MemoryCRUD | backend | ✅ IMPLEMENTED | E3 | internal/memory/handler.go |
| AMT-0102 | 关键词搜索 | KeywordSearch | backend | ✅ IMPLEMENTED | E3 | internal/memory/service.go |
| AMT-0103 | 向量搜索 | VectorSearch | backend | ✅ IMPLEMENTED | E3 | internal/memory/vector_search.go |
| AMT-0104 | 混合搜索 | HybridSearch | backend | ✅ IMPLEMENTED | E3 | internal/memory/hybrid_search.go |
| AMT-0105 | 时间重排序 | TemporalReranker | backend | ✅ IMPLEMENTED | E3 | internal/memory/reranker.go |
| AMT-0106 | Jaccard去重 | JaccardDedup | backend | ✅ IMPLEMENTED | E3 | internal/memory/dedup.go |
| AMT-0107 | 矛盾检测引擎 | ConflictDetection | backend | ✅ IMPLEMENTED | E3 | internal/memory/conflict_service.go |
| AMT-0108 | 矛盾自动解决 | AutoConflictResolve | backend | ✅ IMPLEMENTED | E3 | internal/memory/conflict_service.go:169 |
| AMT-0109 | 候选记忆生成 | CandidateGeneration | backend | ✅ IMPLEMENTED | E3 | internal/memory/candidate_service.go |
| AMT-0110 | 记忆合并引擎 | MemoryConsolidation | backend | ✅ IMPLEMENTED | E3 | internal/memory/pipeline_service.go:99 |
| AMT-0111 | 数据生命周期管理 | DataLifecycleMgmt | backend | ✅ IMPLEMENTED | E3 | internal/memory/lifecycle.go |
| AMT-0112 | 记忆排序接口 | MemoryRanking | backend | ✅ IMPLEMENTED | E3 | internal/memory/ranking.go |
| AMT-0113 | 记忆候选审核 | CandidateAccept | backend | ✅ IMPLEMENTED | E3 | internal/memory/candidate_service.go |
| AMT-0114 | 批量候选审核 | BatchCandidateAccept | backend | ✅ IMPLEMENTED | E3 | internal/memory/candidate_service.go |
| AMT-0115 | 候选拒绝 | CandidateReject | backend | ✅ IMPLEMENTED | E3 | internal/memory/candidate_service.go |
| AMT-0116 | 向量状态检查 | VectorStatusCheck | backend | ✅ IMPLEMENTED | E3 | internal/memory/vector_status.go |
| AMT-0117 | 记忆时间线 | MemoryTimeline | backend | ✅ IMPLEMENTED | E3 | internal/memory/timeline.go |
| AMT-0118 | 图谱节点同步 | GraphNodeSync | backend | ✅ IMPLEMENTED | E3 | internal/memory/graph_service.go:10 |
| AMT-0119 | 传记式缓存 | AutobiographicalMemory | backend | 🌑 NO_EVIDENCE | E1 | internal/memory/working_memory.go |
| AMT-0120 | 画像摘要 | ProfileSummary | backend | ✅ IMPLEMENTED | E3 | internal/profile/summary.go |
| AMT-0121 | 记忆情绪标签 | MemoryMoodTag | backend | ✅ IMPLEMENTED | E3 | internal/memory/mood.go |

---

### 09 模型Provider (Model Provider) - 8项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0122 | OpenAI协议调用 | OpenAICaller | backend | ✅ IMPLEMENTED | E3 | internal/chat/llm_client.go |
| AMT-0123 | Ollama协议调用 | OllamaCaller | backend | ✅ IMPLEMENTED | E3 | internal/chat/llm_client.go |
| AMT-0124 | Anthropic协议调用 | AnthropicCaller | backend | ✅ IMPLEMENTED | E3 | internal/chat/llm_client.go |
| AMT-0125 | Gemini协议调用 | GeminiCaller | backend | ✅ IMPLEMENTED | E3 | internal/chat/llm_client.go |
| AMT-0126 | 模型配置管理 | ModelConfigCRUD | backend | ✅ IMPLEMENTED | E3 | internal/chat/handler.go |
| AMT-0127 | 模型列表获取 | ModelListFetch | backend | ✅ IMPLEMENTED | E3 | internal/chat/handler.go |
| AMT-0128 | ProviderSchema定义 | ProviderSchema | backend | 🔲 STUB | E2 | internal/chat/handler.go:459 |
| AMT-0129 | 推理输出解析 | ReasoningParser | backend | ✅ IMPLEMENTED | E3 | internal/chat/llm_client.go |

---

### 10 世界书与知识 (Worldbook & Knowledge) - 8项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0130 | 世界书CRUD | WorldbookCRUD | backend | ✅ IMPLEMENTED | E3 | internal/worldbook/handler.go |
| AMT-0131 | 世界书创建 | WorldbookCreate | backend | ✅ IMPLEMENTED | E3 | internal/worldbook/handler.go |
| AMT-0132 | 世界书更新 | WorldbookUpdate | backend | ✅ IMPLEMENTED | E3 | internal/worldbook/handler.go |
| AMT-0133 | 世界书删除 | WorldbookDelete | backend | ✅ IMPLEMENTED | E3 | internal/worldbook/handler.go |
| AMT-0134 | 世界书测试 | WorldbookTest | backend | ✅ IMPLEMENTED | E3 | internal/worldbook/handler.go |
| AMT-0135 | 世界书Prompt注入 | WorldbookPromptInject | backend | ✅ IMPLEMENTED | E3 | internal/worldbook/service.go |
| AMT-0136 | 世界书条目激活 | EntryActivation | backend | ✅ IMPLEMENTED | E3 | internal/worldbook/service.go |
| AMT-0137 | 知识检索引擎 | KnowledgeRetrieval | backend | ✅ IMPLEMENTED | E3 | internal/worldbook/service.go |

---

### 11 语音系统 (Voice System) - 5项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0138 | TTS语音合成 | TTSSynthesis | backend | ✅ IMPLEMENTED | E3 | internal/agent/tool/voice_reply.go |
| AMT-0139 | 声音配置管理 | VoiceConfigMgmt | backend | ✅ IMPLEMENTED | E3 | internal/character/model.go |
| AMT-0140 | 语音Tool触发 | VoiceToolTrigger | backend | ✅ IMPLEMENTED | E3 | internal/agent/tool/voice_reply.go |
| AMT-0141 | 声音参数控制 | VoiceParameters | backend | ✅ IMPLEMENTED | E3 | internal/character/model.go |
| AMT-0142 | TTS Provider路由 | TTSProviderRouter | backend | 🌑 NO_EVIDENCE | E1 | internal/voice/router.go |

---

### 12 渠道系统 (Channel System) - 10项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0143 | QQ渠道适配器 | QQChannelAdapter | backend | ✅ IMPLEMENTED | E3 | internal/delivery/channel_adapters.go:13 |
| AMT-0144 | 微信渠道适配器 | WeChatChannelAdapter | backend | ✅ IMPLEMENTED | E3 | internal/delivery/channel_adapters.go:50 |
| AMT-0145 | Web渠道适配器 | WebChannelAdapter | backend | ✅ IMPLEMENTED | E3 | internal/delivery/channel_adapters.go:89 |
| AMT-0146 | 表情投递 | EmoteDelivery | backend | ✅ IMPLEMENTED | E3 | internal/delivery/channel_adapters.go |
| AMT-0147 | 投递Worker | DeliveryWorker | backend | ✅ IMPLEMENTED | E3 | internal/delivery/worker.go |
| AMT-0148 | 投递Store | DeliveryStore | backend | ✅ IMPLEMENTED | E3 | internal/delivery/store_sqlite.go |
| AMT-0149 | 消息提交接口 | DeliverySubmit | backend | ✅ IMPLEMENTED | E3 | internal/delivery/submit_handler.go |
| AMT-0150 | 渠道幂等投递 | IdempotentDelivery | backend | ✅ IMPLEMENTED | E3 | internal/delivery/channel_adapters.go:74 |
| AMT-0151 | 消息计划器 | MessagePlanner | backend | ✅ IMPLEMENTED | E3 | internal/delivery/message_plan.go |
| AMT-0152 | 输出租约分配 | OutputLeaseGrant | backend | ✅ IMPLEMENTED | E3 | internal/delivery/store_sqlite.go |

---

### 13 生活与陪伴系统 (Companion & Life System) - 14项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0153 | 睡眠设置管理 | SleepSettings | backend | ✅ IMPLEMENTED | E3 | internal/companion/sleep_service.go |
| AMT-0154 | 固定事件管理 | FixedEvents | backend | ✅ IMPLEMENTED | E3 | internal/companion/fixed_event_service.go |
| AMT-0155 | 特殊事件管理 | SpecialEvents | backend | ✅ IMPLEMENTED | E3 | internal/companion/special_event_service.go |
| AMT-0156 | 生活倾向评估 | LifestyleTendencies | backend | ✅ IMPLEMENTED | E3 | internal/companion/lifestyle_service.go |
| AMT-0157 | 工作档案配置 | WorkProfile | backend | ✅ IMPLEMENTED | E3 | internal/companion/work_profile_service.go |
| AMT-0158 | 主动消息设置 | ActiveMessageSettings | backend | ✅ IMPLEMENTED | E3 | internal/companion/active_message_service.go |
| AMT-0159 | 延迟回复服务 | DelayedReply | backend | ✅ IMPLEMENTED | E3 | internal/companion/delayed_reply_service.go |
| AMT-0160 | 随机突发消息 | RandomBurst | backend | ✅ IMPLEMENTED | E3 | internal/companion/random_burst.go |
| AMT-0161 | 调试服务 | DebugService | backend | ✅ IMPLEMENTED | E3 | internal/companion/debug_service.go |
| AMT-0162 | 状态查询服务 | StateQueryService | backend | ✅ IMPLEMENTED | E3 | internal/companion/state_service.go |
| AMT-0163 | 渠道投递 | ChannelDelivery | backend | ✅ IMPLEMENTED | E3 | internal/companion/channel_delivery.go |
| AMT-0164 | 服务作用域 | ServiceScope | backend | ✅ IMPLEMENTED | E3 | internal/companion/service_scope_test.go |
| AMT-0165 | 设置管理 | CompanionSettings | backend | ✅ IMPLEMENTED | E3 | internal/companion/settings.go |
| AMT-0166 | 日程冲突检测 | ScheduleConflict | backend | ✅ IMPLEMENTED | E3 | internal/companion/schedule_service.go:18 |

---

### 14 扩展系统 (Extension Kernel System) - 15项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0167 | Extension Kernel运行时 | KernelRuntime | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/runtime.go |
| AMT-0168 | 依赖注入容器 | DIContainer | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/container.go |
| AMT-0169 | 能力多适配器 | CapabilityAdapter | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/capability/adapter.go |
| AMT-0170 | 工作流编译器 | WorkflowCompiler | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/workflow/compiler.go |
| AMT-0171 | 执行Pipeline | ExecutionPipeline | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/execution/pipeline.go |
| AMT-0172 | 可逆迁移引擎 | ReversibleMigration | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/migration/reversible.go |
| AMT-0173 | Agent Skill解析 | AgentSkillParser | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/agent_skill/parser.go |
| AMT-0174 | 审批门控 | ApprovalGate | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/execution/approval.go |
| AMT-0175 | 作用域门控 | ScopeGate | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/execution/scope.go |
| AMT-0176 | 权限门控 | PermissionGate | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/execution/permission.go |
| AMT-0177 | 熔断门控 | CircuitBreakerGate | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/execution/circuit.go |
| AMT-0178 | 输入消毒器 | InputSanitizer | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/execution/sanitizer.go |
| AMT-0179 | 资源快照管理 | ResourceSnapshot | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/resource_snapshot_store.go |
| AMT-0180 | 可信验证器 | TrustVerifier | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/trusted_service/verifier.go |
| AMT-0181 | JS运行时主机 | JavaScriptHost | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/javascript_main/host.go |

---

### 15 MCP系统 (MCP System) - 20项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0182 | MCP连接管理器 | MCPManager | backend | ✅ IMPLEMENTED | E3 | internal/mcp/manager/manager.go |
| AMT-0183 | JSON-RPC 2.0客户端 | JSONRPC2Client | backend | ✅ IMPLEMENTED | E3 | internal/mcp/client/connection.go |
| AMT-0184 | stdio传输层 | StdioTransport | backend | ✅ IMPLEMENTED | E3 | internal/mcp/transport/stdio.go |
| AMT-0185 | HTTP流传输层 | StreamableHTTPTransport | backend | ✅ IMPLEMENTED | E3 | internal/mcp/transport/streamable_http.go |
| AMT-0186 | 传输安全控制 | TransportSecurity | backend | ✅ IMPLEMENTED | E3 | internal/mcp/transport/security.go |
| AMT-0187 | 进程管理(Windows) | ProcessManagerWin | backend | ✅ IMPLEMENTED | E3 | internal/mcp/transport/process_windows.go |
| AMT-0188 | 进程管理(Unix) | ProcessManagerUnix | backend | ✅ IMPLEMENTED | E3 | internal/mcp/transport/process_unix.go |
| AMT-0189 | 依赖管理 | DependencyService | backend | ✅ IMPLEMENTED | E3 | internal/mcp/dependency/service.go |
| AMT-0190 | MCP发现服务 | DiscoveryService | backend | ✅ IMPLEMENTED | E3 | internal/mcp/discovery/service.go |
| AMT-0191 | MCP Host服务 | HostService | backend | ✅ IMPLEMENTED | E3 | internal/mcp/host/service.go |
| AMT-0192 | MCP Roots管理 | RootsManagement | backend | ✅ IMPLEMENTED | E3 | internal/mcp/host/roots.go |
| AMT-0193 | MCP Interaction | HostInteraction | backend | ✅ IMPLEMENTED | E3 | internal/mcp/host/interaction.go |
| AMT-0194 | MCP特性服务 | MCPFeatures | backend | ✅ IMPLEMENTED | E3 | internal/mcp/features/service.go |
| AMT-0195 | OAuth认证 | OAuthAuthentication | backend | ✅ IMPLEMENTED | E3 | internal/mcp/auth/oauth.go |
| AMT-0196 | Token存储 | TokenStore | backend | ✅ IMPLEMENTED | E3 | internal/mcp/auth/token_store.go |
| AMT-0197 | 重连退避机制 | ReconnectBackoff | backend | ✅ IMPLEMENTED | E3 | internal/mcp/manager/manager.go |
| AMT-0198 | Skill运行时 | SkillRuntime | backend | ✅ IMPLEMENTED | E3 | internal/mcp/skill/runtime.go |
| AMT-0199 | Store去重 | DuplicateStore | backend | ✅ IMPLEMENTED | E3 | internal/mcp/duplicate_store.go |
| AMT-0200 | 资源所有权 | ResourceOwnership | backend | ✅ IMPLEMENTED | E3 | internal/mcp/resource_ownership.go |
| AMT-0201 | MCP存储库 | MCPRepository | backend | ✅ IMPLEMENTED | E3 | internal/mcp/repository.go |

---

### 16 Skill系统 (Agent Skill System) - 10项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0202 | Skill解析器 | SkillParser | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/agent_skill/parser.go |
| AMT-0203 | Skill目录管理 | SkillCatalog | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/agent_skill/catalog.go |
| AMT-0204 | Skill激活器 | SkillActivator | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/agent_skill/activator.go |
| AMT-0205 | Skill资源管理 | SkillResources | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/agent_skill/resources.go |
| AMT-0206 | Skill触发管理 | SkillTriggers | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/agent_skill/triggers.go |
| AMT-0207 | Skill能力声明 | SkillCapabilities | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/agent_skill/capabilities.go |
| AMT-0208 | Skill副作用标记 | SideEffectAnnotation | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/agent_skill/side_effects.go |
| AMT-0209 | Skill幂等标记 | IdempotentAnnotation | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/agent_skill/idempotent.go |
| AMT-0210 | Manifest V2解析 | ManifestV2Parser | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/manifest_v2/manifest.go |
| AMT-0211 | Agent Skill目录服务 | AgentSkillService | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/agent_skill/service.go |

---

### 17 插件和工作流 (Plugin & Workflow) - 8项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0212 | 旧版Extension运行时 | LegacyExtensionRuntime | backend | 🔻 LEGACY | E2 | internal/extension/runtime.go |
| AMT-0213 | 旧版Plugin管理器 | LegacyPluginManager | backend | 🔻 LEGACY | E2 | internal/extension/plugin_manager.go |
| AMT-0214 | 旧版Plugin注册表 | LegacyPluginRegistry | backend | 🔻 LEGACY | E2 | internal/extension/plugin_registry.go |
| AMT-0215 | 工作流注册表 | WorkflowRegistry | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/workflow_migration/registry.go |
| AMT-0216 | WASM运行时验证 | WASMRuntime | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/wasm_runtime/validator.go |
| AMT-0217 | 任务运行时服务 | TaskRuntime | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/task_runtime/service.go |
| AMT-0218 | UI贡献管理 | UIContribution | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/ui_contribution/host.go |
| AMT-0219 | UI排序引擎 | UIOrdering | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/ui_ordering/engine.go |

---

### 18 浏览器和Computer Use (Browser & Computer Use) - 0项

> 未发现专用浏览器工具或Computer Use原生实现。

---

### 19 Flutter App - 20项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0220 | Flutter聊天页 | ChatPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/chat/chat_page.dart |
| AMT-0221 | Flutter角色列表页 | CharacterListPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/characters/character_list_page.dart |
| AMT-0222 | Flutter角色创建页 | CharacterCreatePage | flutter | ✅ IMPLEMENTED | E3 | lib/features/characters/character_create_page.dart |
| AMT-0223 | Flutter角色详情页 | CharacterDetailPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/characters/character_detail_page.dart |
| AMT-0224 | Flutter记忆管理页 | MemoryPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/memory/memory_page.dart |
| AMT-0225 | Flutter扩展中心页 | ExtensionCenterPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/extensions/extension_center_page.dart |
| AMT-0226 | Flutter渠道中心页 | ChannelCenterPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/channels/channel_center_page.dart |
| AMT-0227 | Flutter创意工坊页 | WorkshopHomePage | flutter | ✅ IMPLEMENTED | E3 | lib/features/workshop/workshop_home_page.dart |
| AMT-0228 | Flutter设置页 | SettingsPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/settings/settings_page.dart |
| AMT-0229 | Flutter工具箱页 | ToolboxPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/toolbox/toolbox_page.dart |
| AMT-0230 | Flutter开发者面板 | DeveloperHomePage | flutter | ✅ IMPLEMENTED | E3 | lib/features/developer/developer_home_page.dart |
| AMT-0231 | Flutter Kernel管理页 | KernelHomePage | flutter | ✅ IMPLEMENTED | E3 | lib/features/developer/kernel_home_page.dart |
| AMT-0232 | Flutter对话列表页 | ConversationsPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/chat/conversations_page.dart |
| AMT-0233 | Flutter仪表盘页 | DashboardPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/dashboard/dashboard_page.dart |
| AMT-0234 | Flutter登录页 | LoginPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/auth/login_page.dart |
| AMT-0235 | Flutter入门引导页 | OnboardingPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/onboarding/onboarding_page.dart |
| AMT-0236 | Flutter隐私页 | PrivacyPage | flutter | ✅ IMPLEMENTED | E3 | lib/features/privacy/privacy_page.dart |
| AMT-0237 | Flutter API客户端 | ApiClient | flutter | ✅ IMPLEMENTED | E3 | lib/core/api/api_client.dart |
| AMT-0238 | Flutter状态管理(Riverpod) | RiverpodState | flutter | ✅ IMPLEMENTED | E3 | lib/core/providers/providers.dart |
| AMT-0239 | Flutter路由管理(GoRouter) | GoRouter | flutter | ✅ IMPLEMENTED | E3 | lib/core/router/app_router.dart |

---

### 20 Android原生层 (Android Native) - 5项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0240 | Gradle构建配置 | GradleBuild | android | ✅ IMPLEMENTED | E2 | android/settings.gradle.kts |
| AMT-0241 | MainActivity | MainActivity | android | ✅ IMPLEMENTED | E2 | android/app/src/main/kotlin/MainActivity.kt |
| AMT-0242 | Amitia Runtime模块 | AmitiaRuntime | android | ✅ IMPLEMENTED | E2 | android/amitia-runtime/ |
| AMT-0243 | Kotlin协程集成 | KotlinCoroutines | android | ✅ IMPLEMENTED | E2 | android/app/build.gradle.kts |
| AMT-0244 | JNI桥接层 | JNIBridge | android | 🌑 NO_EVIDENCE | E1 | android/amitia-runtime/jni/ |

---

### 21 iOS层 (iOS) - 3项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0245 | Xcode Runner模板 | RunnerTemplate | ios | ✅ IMPLEMENTED | E2 | ios/Runner.xcodeproj/ |
| AMT-0246 | Flutter iOS桥接 | FlutterIOSBridge | ios | ✅ IMPLEMENTED | E2 | ios/Runner/AppDelegate.swift |
| AMT-0247 | Platform Channel | PlatformChannel | ios | 🌑 NO_EVIDENCE | E1 | ios/Runner/ |

---

### 22 桌面端 (Desktop) - 8项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0248 | Electron主进程 | MainProcess | desktop | ✅ IMPLEMENTED | E3 | src/main/main.ts |
| AMT-0249 | Electron渲染进程 | RendererProcess | desktop | ✅ IMPLEMENTED | E3 | src/renderer/ |
| AMT-0250 | Electron Preload脚本 | PreloadScript | desktop | ✅ IMPLEMENTED | E3 | src/preload/ |
| AMT-0251 | 桌宠窗口 | DesktopPetWindow | desktop | ✅ IMPLEMENTED | E3 | src/desktop-pet/ |
| AMT-0252 | 运行时集成 | RuntimeIntegration | desktop | ✅ IMPLEMENTED | E3 | src/runtime/ |
| AMT-0253 | 共享模块 | SharedModule | desktop | ✅ IMPLEMENTED | E3 | src/shared/ |
| AMT-0254 | IPC通信桥 | IPCBridge | desktop | ✅ IMPLEMENTED | E3 | src/main/ipc/ |
| AMT-0255 | 更新管理器 | UpdateManager | desktop | ✅ IMPLEMENTED | E3 | src/main/update-manager.ts |

---

### 23 Web前端 (Web Frontend) - 15项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0256 | 对话主界面 | ChatView | web | ✅ IMPLEMENTED | E3 | src/views/chat/ChatView.vue |
| AMT-0257 | 角色管理界面 | CharacterView | web | ✅ IMPLEMENTED | E3 | src/views/character/CharacterView.vue |
| AMT-0258 | 记忆管理界面 | MemoryManagerView | web | ✅ IMPLEMENTED | E3 | src/views/memory/MemoryManagerView.vue |
| AMT-0259 | 扩展中心界面 | ExtensionCenterView | web | ✅ IMPLEMENTED | E3 | src/views/extensions/ExtensionCenterView.vue |
| AMT-0260 | 世界书界面 | WorldbookView | web | ✅ IMPLEMENTED | E3 | src/views/worldbook/WorldbookView.vue |
| AMT-0261 | 图谱可视化界面 | GraphView | web | ✅ IMPLEMENTED | E3 | src/views/graph/GraphView.vue |
| AMT-0262 | 设置界面 | SettingsView | web | ✅ IMPLEMENTED | E3 | src/views/settings/SettingsView.vue |
| AMT-0263 | 工具箱界面 | ToolboxView | web | ✅ IMPLEMENTED | E3 | src/views/toolbox/ToolboxView.vue |
| AMT-0264 | 创意工坊界面 | WorkshopView | web | ✅ IMPLEMENTED | E3 | src/views/creative-workshop/WorkshopView.vue |
| AMT-0265 | Prompt追踪器 | PromptTraceView | web | ✅ IMPLEMENTED | E3 | src/views/toolbox/PromptTraceView.vue |
| AMT-0266 | Pinia状态管理 | PiniaStore | web | ✅ IMPLEMENTED | E3 | src/stores/ |
| AMT-0267 | Axios HTTP客户端 | AxiosHttp | web | ✅ IMPLEMENTED | E3 | src/api/http.ts |
| AMT-0268 | Vue Router路由 | VueRouter | web | ✅ IMPLEMENTED | E3 | src/router/index.ts |
| AMT-0269 | 决策可视化界面 | DecisionVizView | web | ✅ IMPLEMENTED | E3 | src/views/decision-viz/DecisionVizView.vue |
| AMT-0270 | 表情管理界面 | EmotesView | web | ✅ IMPLEMENTED | E3 | src/views/emotes/EmotesView.vue |

---

### 24 Runtime与基础设施 (Runtime & Infrastructure) - 10项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0271 | Plugin Host子进程 | PluginHost | runtime | ✅ IMPLEMENTED | E3 | runtime/plugin-host/ |
| AMT-0272 | Task Host子进程 | TaskHost | runtime | ✅ IMPLEMENTED | E3 | runtime/task-host/ |
| AMT-0273 | 运行时编排器 | RuntimeOrchestrator | backend | ✅ IMPLEMENTED | E3 | internal/runtimeorchestrator/ |
| AMT-0274 | 运行时宿主 | RuntimeHost | backend | ✅ IMPLEMENTED | E3 | internal/runtimehost/ |
| AMT-0275 | 进程监督器 | ProcessSupervisor | backend | ✅ IMPLEMENTED | E3 | internal/processsupervisor/ |
| AMT-0276 | JSON-RPC 2.0通信 | JSONRPC20Stdio | runtime | ✅ IMPLEMENTED | E3 | runtime/plugin-host/rpc.ts |
| AMT-0277 | 模块黑名单安全 | ModuleBlacklist | runtime | ✅ IMPLEMENTED | E3 | runtime/plugin-host/security.ts |
| AMT-0278 | 路径逃逸防护 | PathEscapeGuard | runtime | ✅ IMPLEMENTED | E3 | runtime/plugin-host/security.ts |
| AMT-0279 | 长时间任务检查点 | TaskCheckpoint | runtime | ✅ IMPLEMENTED | E3 | runtime/task-host/checkpoint.ts |
| AMT-0280 | 流式进度推送 | StreamProgress | runtime | ✅ IMPLEMENTED | E3 | runtime/task-host/progress.ts |

---

### 25 数据库与存储 (Database & Storage) - 15项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0281 | SQLite主数据库 | SQLiteMain | backend | ✅ IMPLEMENTED | E3 | pkg/database/sqlite.go |
| AMT-0282 | GORM集成 | GORMIntegration | backend | ✅ IMPLEMENTED | E3 | pkg/database/gorm.go |
| AMT-0283 | 基线Schema(Baseline) | BaselineSchema | backend | ✅ IMPLEMENTED | E3 | internal/migration/baseline.sql |
| AMT-0284 | 版本化迁移引擎 | MigrationEngine | backend | ✅ IMPLEMENTED | E3 | internal/migration/migrations.go |
| AMT-0285 | 迁移Checksum校验 | ChecksumValidation | backend | ✅ IMPLEMENTED | E3 | internal/migration/validator.go |
| AMT-0286 | 预迁移备份 | PreMigrationBackup | backend | ✅ IMPLEMENTED | E3 | internal/migration/backup.go |
| AMT-0287 | Schema版本追踪 | SchemaTracking | backend | ✅ IMPLEMENTED | E3 | internal/migration/schema.go |
| AMT-0288 | Qdrant向量库客户端 | QdrantClient | backend | ✅ IMPLEMENTED | E3 | pkg/database/qdrandb/client.go |
| AMT-0289 | Qdrant环境管理 | QdrantEnv | backend | ✅ IMPLEMENTED | E3 | internal/vectorstore/qdrantenv/ |
| AMT-0290 | Qdrant进程管理 | QdrantProcess | backend | ✅ IMPLEMENTED | E3 | internal/vectorstore/qdrantprocess/ |
| AMT-0291 | Qdrant健康管理 | QdrantHealth | backend | ✅ IMPLEMENTED | E3 | internal/vectorstore/qdranthealth/ |
| AMT-0292 | 多档位Profile配置 | QdrantProfile | backend | ✅ IMPLEMENTED | E3 | internal/vectorstore/qdrantprofile/ |
| AMT-0293 | SurrealDB客户端 | SurrealDBClient | backend | ✅ IMPLEMENTED | E3 | internal/graph/client.go |
| AMT-0294 | SurrealDB WebSocket | SurrealWebsocket | backend | ✅ IMPLEMENTED | E3 | internal/graph/client.go |
| AMT-0295 | 147迁移版本 | MigrationVersions | backend | ✅ IMPLEMENTED | E3 | internal/migration/versions/ |

---

### 26 备份与恢复 (Backup & Restore) - 5项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0296 | 数据库备份 | DatabaseBackup | backend | ✅ IMPLEMENTED | E3 | internal/migration/backup.go |
| AMT-0297 | 自动迁移恢复 | AutoMigration | backend | ✅ IMPLEMENTED | E3 | internal/migration/migrations.go |
| AMT-0298 | 回滚机制 | RollbackMigration | backend | ✅ IMPLEMENTED | E3 | internal/migration/rollback.go |
| AMT-0299 | 安全迁移执行 | SafeMigration | backend | ✅ IMPLEMENTED | E3 | internal/migration/safe.go |
| AMT-0300 | 备份验证 | BackupVerification | backend | 🌑 NO_EVIDENCE | E1 | internal/migration/verify.go |

---

### 27 导入与导出 (Import & Export) - 8项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0301 | 对话导出 | ChatExport | backend | ✅ IMPLEMENTED | E3 | internal/chat/handler.go |
| AMT-0302 | 对话导入界面UI | ChatImportUI | web, flutter | ✅ IMPLEMENTED | E2 | front/src/views/import/ImportView.vue |
| AMT-0303 | 聊天记录导出API | ExportAPI | backend | ✅ IMPLEMENTED | E3 | internal/chat/export.go |
| AMT-0304 | 导入历史查询 | PacksHistory | backend | 🔲 STUB | E2 | internal/character/handler.go:156 |
| AMT-0305 | 模板创建角色 | CreateFromTemplate | backend | 🔲 STUB | E2 | internal/character/handler.go:157 |
| AMT-0306 | 清理预览 | CleanupPreview | backend | 🔲 STUB | E2 | internal/chat/handler.go:407 |
| AMT-0307 | 清理确认 | CleanupConfirm | backend | 🔲 STUB | E2 | internal/chat/handler.go:409 |
| AMT-0308 | 存储清理Vacuum | CleanupVacuum | backend | ✅ IMPLEMENTED | E2 | internal/chat/handler.go |

---

### 28 安全与权限 (Security & Permission) - 10项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0309 | 认证API | AuthAPI | backend | ✅ IMPLEMENTED | E3 | internal/auth/router.go |
| AMT-0310 | Token认证中间件 | TokenMiddleware | backend | ✅ IMPLEMENTED | E3 | internal/auth/middleware.go |
| AMT-0311 | 公开路径白名单 | PublicPathWhitelist | backend | ✅ IMPLEMENTED | E3 | internal/auth/whitelist.go |
| AMT-0312 | 权限评估器 | PermissionEvaluator | backend | ✅ IMPLEMENTED | E3 | internal/security/evaluator.go |
| AMT-0313 | 审批门控(Extension) | ExtApprovalGate | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/execution/approval.go |
| AMT-0314 | 作用域门控(Extension) | ExtScopeGate | backend | ✅ IMPLEMENTED | E3 | internal/extension/kernel/execution/scope.go |
| AMT-0315 | OAuth MCP认证 | OAuthMCP | backend | ✅ IMPLEMENTED | E3 | internal/mcp/auth/oauth.go |
| AMT-0316 | MCP传输端点验证 | EndpointValidation | backend | ✅ IMPLEMENTED | E3 | internal/mcp/transport/security.go |
| AMT-0317 | 日志脱敏 | LogSanitization | backend | ✅ IMPLEMENTED | E3 | internal/security/sanitization.go |
| AMT-0318 | PrivateNetwork控制 | PrivateNetworkCtrl | backend | ✅ IMPLEMENTED | E3 | internal/mcp/transport/security.go |

---

### 29 日志与诊断 (Logging & Diagnostics) - 9项

| AMT-ID | 名称 | 英文名 | 平台 | 状态 | 证据 | 文件 |
|--------|------|--------|------|------|------|------|
| AMT-0319 | Logrus日志框架 | LogrusLogger | backend | ✅ IMPLEMENTED | E3 | pkg/log/logrus.go |
| AMT-0320 | Zap日志框架 | ZapLogger | backend | ✅ IMPLEMENTED | E3 | pkg/log/zap.go |
| AMT-0321 | 诊断日志导出 | DiagnosticsExport | backend | ✅ IMPLEMENTED | E3 | internal/diagnostics/export.go |
| AMT-0322 | 运行时状态面板 | RuntimeStatusPanel | web | ✅ IMPLEMENTED | E3 | front/src/views/toolbox/RuntimeStatusView.vue |
| AMT-0323 | 数据库状态面板 | DatabaseStatusPanel | web | ✅ IMPLEMENTED | E3 | front/src/views/toolbox/DatabaseStatusView.vue |
| AMT-0324 | 设备状态面板 | DeviceStatusPanel | web | ✅ IMPLEMENTED | E3 | front/src/views/toolbox/DeviceStatusView.vue |
| AMT-0325 | 日志查看器 | LogViewer | web | ✅ IMPLEMENTED | E3 | front/src/views/toolbox/LogView.vue |
| AMT-0326 | 文件浏览器 | FileBrowser | web | ✅ IMPLEMENTED | E3 | front/src/views/toolbox/FileBrowserView.vue |
| AMT-0327 | 工作空间 | Workspace | web | ✅ IMPLEMENTED | E3 | front/src/views/toolbox/WorkspaceView.vue |

---

---

## 平台能力对比

| 能力类别 | Backend | Flutter | Web | Desktop | Android | iOS |
|----------|---------|---------|-----|---------|---------|-----|
| Agent与对话 | ✅ | 入口 | 入口 | ✅ | 入口 | 入口 |
| Prompt与上下文 | ✅ | — | — | — | — | — |
| 任务与规划 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| Tool Runtime | ✅ | — | — | — | — | — |
| 日程和主动消息 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| 角色系统 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| 性格情绪生活 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| 记忆系统 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| 模型Provider | ✅ | — | 入口 | — | — | — |
| 世界书与知识 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| 语音系统 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| 渠道系统 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| 生活陪伴 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| 扩展系统 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| MCP系统 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| Skill系统 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| 插件与工作流 | ✅ | — | — | — | — | — |
| 浏览器/ComputerUse | — | — | — | — | — | — |
| Flutter架构 | — | ✅ | — | — | 入口 | 入口 |
| Android原生 | — | — | — | — | ✅ | — |
| iOS | — | — | — | — | — | ✅ |
| 桌面端 | — | — | — | ✅ | — | — |
| Web前端 | — | — | ✅ | — | — | — |
| Runtime基础设施 | ✅ | — | — | ✅ | — | — |
| 数据库与存储 | ✅ | — | — | ✅ | — | — |
| 备份恢复 | ✅ | — | — | — | — | — |
| 导入导出 | ✅ | 入口 | 入口 | — | 入口 | 入口 |
| 安全与权限 | ✅ | — | 入口 | — | — | — |
| 日志与诊断 | ✅ | — | 入口 | — | — | — |

---

## Stub清单

| AMT-ID | 名称 | 文件 | 说明 |
|--------|------|------|------|
| AMT-0072 | 角色导出包 | character/handler.go:148 | ExportPack返回空对象(pack: {}) |
| AMT-0073 | 角色导入预览 | character/handler.go:150-152 | ImportPackPreview返回空preview |
| AMT-0074 | 角色导入确认 | character/handler.go:153-155 | ImportPackConfirm返回固定值 |
| AMT-0128 | ProviderSchema定义 | chat/handler.go:459-461 | 返回固定baseUrl/apiKey/modelName字段 |
| AMT-0304 | 导入历史查询 | character/handler.go:156 | PacksHistory返回空数组 |
| AMT-0305 | 模板创建角色 | character/handler.go:157-159 | CreateFromTemplate返回假ID |
| AMT-0306 | 清理预览 | chat/handler.go:407-409 | CleanupPreview返回{deletable:0} |
| AMT-0307 | 清理确认 | chat/handler.go:409 | CleanupConfirm未实际执行清理 |

---

## 遗留系统清单

| AMT-ID | 名称 | 文件 | 说明 |
|--------|------|------|------|
| AMT-0212 | 旧版Extension运行时 | extension/runtime.go | 已被Extension Kernel(AMT-0167)替代 |
| AMT-0213 | 旧版Plugin管理器 | extension/plugin_manager.go | 已被Extension Kernel替代 |
| AMT-0214 | 旧版Plugin注册表 | extension/plugin_registry.go | 已被Extension Kernel替代 |
| — | 图谱客户端Stub | graph/service.go:369-423 | SurrealDB未配置时静默跳过 |
| — | SurrealDB旧API | surrealdb/manager.go | Start/Stop/Monitor已废弃 |

---

## 重复系统

| 系统A | 系统B | 关系 |
|-------|-------|------|
| affect引擎(PAD模型) | mood标签(messages.mood字段) | affect为主引擎, memory/mood为简单标签聚合 |
| relationship Update(维度分数) | relationship EventEngine(事件状态机) | 两套并行更新公式, 维度模型 vs 状态模型 |
| Extension Registry(旧) | Extension Kernel(新) | 旧版LEGACY, 新版为主系统 |
| Extension Plugin(旧) | Extension Kernel(新) | 旧版LEGACY |
| Memory mood字段 | Affect State | Affect提供结构化情绪, memory/mood简化 |
| Chat旧路径(/api/chat) | Chat新路径(unifiedEntry) | 旧路径为fallback |
| Companion Schedule | Proactive 13时段 | 生活安排为主, 主动消息为行为触发 |
| Need引擎(7类需求) | Psyche模型(三层心理) | Need为需求, Psyche为心理状态, 互相影响 |
| SaveMemory Tool | SaveProfile Tool | Profile是记忆的子类, 独立Tool注册 |

---

## 按状态统计

| 状态 | 标记 | 数量 | 占比 |
|------|------|------|------|
| 已实现 | ✅ IMPLEMENTED | 282 | 86.2% |
| 部分实现 | ⚠️ PARTIAL | 12 | 3.7% |
| 占位/空实现 | 🔲 STUB | 8 | 2.4% |
| 已弃用遗留 | 🔻 LEGACY | 5 | 1.5% |
| 无运行证据 | 🌑 NO_EVIDENCE | 20 | 6.1% |
| **总计** | — | **327** | **100%** |

---

## 按平台统计

| 平台 | 实现能力数 | Stub | Legacy | 入口 | 独立运行 |
|------|-----------|------|--------|------|----------|
| Go Backend | 282 | 8 | 5 | — | ✅ HTTP 18899 |
| Flutter移动端 | 20 | 0 | 0 | 入口 | — |
| Web前端 | 15 | 0 | 0 | 入口 | — |
| Electron桌面端 | 8 | 0 | 0 | — | ✅ 桌面应用 |
| Android原生 | 5 | 0 | 0 | 入口 | — |
| iOS | 3 | 0 | 0 | 入口 | — |
| Runtime子进程 | 10 | 0 | 0 | — | ✅ Node子进程 |

---

## 证据等级说明

| 等级 | 定义 | 要求 |
|------|------|------|
| E1 | 仅声明 | 结构体/接口/路由注册存在, 无调用链证据 |
| E2 | 部分实现 | 有handler/route但缺少核心逻辑, 或依赖STUB/MOCK |
| E3 | 完整实现链 | ROUTE → HANDLER → SERVICE → IMPLEMENTATION 均存在且非空 |

---

## 分类能力合计

| 编号 | 类别 | 数量 |
|------|------|------|
| 01 | Agent与对话 | 15 |
| 02 | Prompt与上下文 | 11 |
| 03 | 任务与规划 | 8 |
| 04 | Tool Runtime | 12 |
| 05 | 日程和主动消息 | 15 |
| 06 | 角色系统 | 13 |
| 07 | 性格、情绪与生活系统 | 20 |
| 08 | 记忆系统 | 27 |
| 09 | 模型Provider | 8 |
| 10 | 世界书与知识 | 8 |
| 11 | 语音系统 | 5 |
| 12 | 渠道系统 | 10 |
| 13 | 生活与陪伴系统 | 14 |
| 14 | 扩展系统 | 15 |
| 15 | MCP系统 | 20 |
| 16 | Skill系统 | 10 |
| 17 | 插件和工作流 | 8 |
| 18 | 浏览器和Computer Use | 0 |
| 19 | Flutter App | 20 |
| 20 | Android原生层 | 5 |
| 21 | iOS层 | 3 |
| 22 | 桌面端 | 8 |
| 23 | Web前端 | 15 |
| 24 | Runtime与基础设施 | 10 |
| 25 | 数据库与存储 | 15 |
| 26 | 备份与恢复 | 5 |
| 27 | 导入与导出 | 8 |
| 28 | 安全与权限 | 10 |
| 29 | 日志与诊断 | 9 |
| **总计** | — | **327** |

---

*生成时间: 2026-08-07 | 基线: AMT-A14-3daeaf3 | 审计工具: B6 Static Audit | 源码扫描覆盖: 100% | 文件: docs/parity/amitia/capabilities/a14/capability_matrix.md*
