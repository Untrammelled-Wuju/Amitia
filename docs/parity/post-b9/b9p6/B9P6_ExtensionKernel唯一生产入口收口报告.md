# B9P6 - Extension Kernel唯一生产入口收口审计与重复系统Guard冻结

## 一、任务概述

- **TaskId**: B9P6
- **任务名称**: Extension Kernel唯一生产入口收口审计与重复系统Guard冻结
- **执行时间**: 2026-08-07T18:00:00 ~ 2026-08-07T18:45:00 (+08:00)
- **前置依赖**: B9P1=PASS, B9P2=PASS
- **下游消费**: B9P4 (b9p4_kernel_input.json)

## 二、前置条件验证

### 2.1 B9P1状态

| 项目 | 结果 |
|------|------|
| Status | PASS |
| AnchorId | AMT-POST-B9-a3a84ec86812 |
| SourceMutated | true (desktoppet/processing/handler.go truncation fix, cmd/server/main.go unused import removal) |
| a14验证 | 全部PASS (linuxArm64, linuxAmd64, windowsAmd64, cgo, basicStartup) |

### 2.2 B9P2状态

| 项目 | 结果 |
|------|------|
| Status | PASS |
| Baseline版本 | PARITY-2026-08-07-V2 |
| Capabilities总数 | 502 (FROZEN) |
| 净化状态 | applied |

### 2.3 Extension Kernel源码SHA256校验

| 文件 | SHA256状态 |
|------|-----------|
| backend/internal/extension/kernel/tool_facade.go | MATCH |
| backend/internal/extension/kernel/capability/registry.go | MATCH |
| backend/internal/extension/kernel/execution/pipeline.go | MATCH |
| backend/internal/extension/kernel/container_builder.go | MATCH |
| backend/internal/extension/FROZEN.md | MATCH |
| backend/cmd/server/tool_facade_wiring.go | MATCH |
| backend/internal/chat/tool_runtime.go | MATCH |
| backend/internal/extension/kernel/legacy_call_counter.go | MATCH |
| backend/internal/agent/tool/registry.go | MATCH |
| backend/internal/extension/kernel/final_gate.go | MATCH |
| backend/internal/extension/kernel/tool_facade_mcp.go | MATCH |
| backend/internal/extension/runtime.go | MATCH |
| backend/internal/extension/kernel/capability/adapter_legacy.go | MATCH |
| backend/internal/extension/kernel/permission/broker.go | MATCH |

**结论**: 14/14 Extension Kernel核心文件全部匹配B9P1 anchor。

### 2.4 Drift检测结果

| 文件 | 状态 | 影响评估 |
|------|------|---------|
| backend/cmd/server/services.go | DRIFT | 无影响 (Post-B9P1 commit d42ddf7 desktop pet P0 fixes) |

drift原因: commit d42ddf7 "完成桌宠系统P0精准修复方案(P0-01~P0-21)" 在B9P1 anchor创建后修改了services.go。Extension Kernel wiring路径（ToolFacade/ToolRegistry/ExecutionPipeline）未发生变化。

## 三、Extension Kernel核心系统清单

### 3.1 核心系统总览

| 系统ID | 路径 | SHA256验证 | 角色 |
|--------|------|-----------|------|
| ToolFacade | backend/internal/extension/kernel/tool_facade.go | MATCH | 模型Tool入口 (PreferKernel=true, FallbackOnError=false) |
| ToolRegistry | backend/internal/extension/kernel/capability/registry.go | MATCH | 全局唯一Agent Tool注册中心 |
| ExecutionPipeline | backend/internal/extension/kernel/execution/pipeline.go | MATCH | 通用Tool执行Pipeline (16 gates) |
| PermissionBroker | backend/internal/extension/kernel/permission/broker.go | MATCH | 唯一Kernel权限决策中心 |
| ContainerBuilder | backend/internal/extension/kernel/container_builder.go | MATCH | 唯一Kernel组装入口 |

### 3.2 ExecutionPipeline 16道关卡

1. InvocationValidator
2. InputValidator
3. AvailabilityGate
4. ScopeGate
5. PermissionGate (via PermissionBroker)
6. ApprovalGate
7. DepthGuard
8. RateLimiter
9. ConcurrencyCtrl
10. IdempotencyGuard
11. TimeoutCtrl
12. CancellationCtrl
13. RetryCtrl
14. ResultValidator
15. Sanitizer
16. CircuitBreaker

### 3.3 ContainerBuilder构建的完整组件列表

- PermissionDefinitionRegistry
- PermissionBroker
- ScopeManager
- RuntimeAdapterRegistry
- ToolRegistry
- ExecutionPipeline
- ToolFacade
- WorkflowRegistry
- WorkflowExecutor
- ScheduleService
- HookService
- EventService
- TaskRuntimeService
- AgentSkillCatalog
- HostAPIGateway

### 3.4 领域系统（Domain）

| 系统 | 路径 | 角色 |
|------|------|------|
| Workflow | backend/internal/extension/kernel/workflow/ | Workflow定义与执行引擎 |
| AgentSkill | backend/internal/extension/kernel/agent_skill/ | Agent Skill目录与Prompt贡献 |
| Hook | backend/internal/extension/kernel/hook/ | Hook系统 |
| Event | backend/internal/extension/kernel/event/ | 事件系统 |
| Schedule | backend/internal/extension/kernel/schedule/ | 定时调度系统 |
| TaskRuntime | backend/internal/extension/kernel/task_runtime/ | 长任务运行时 |

## 四、全局Registry扫描与分类

### 4.1 统计

| 分类 | 数量 |
|------|------|
| CANONICAL_GLOBAL | 4 |
| CANONICAL_DOMAIN | 14 |
| LEGACY_MIGRATION_SOURCE | 2 |
| LEGACY_FROZEN | 3 |
| DOMAIN_REGISTRY_VALID | 29 |
| DUPLICATE_PRODUCTION_REGISTRY | 0 |

### 4.2 唯一全局Agent Tool Registry

**确认**: `backend/internal/extension/kernel/capability/registry.go` 是唯一的CANONICAL_GLOBAL Agent Tool Registry。

- 全局实例数: 1 (由ContainerBuilder创建)
- 无其他生产环境全局Tool Registry

### 4.3 Global Tool Registry清单

| ID | 路径 | 类型 |
|----|------|------|
| TOOL_REGISTRY | backend/internal/extension/kernel/capability/registry.go | CANONICAL_GLOBAL |
| PERMISSION_DEFINITION_REGISTRY | backend/internal/extension/kernel/permission/definition_registry.go | CANONICAL_GLOBAL |
| RUNTIME_ADAPTER_REGISTRY | backend/internal/extension/kernel/capability/ | CANONICAL_GLOBAL |

### 4.4 Domain Registry清单 (14个)

| ID | 路径 | 用途 |
|----|------|------|
| CONTRIBUTION_REGISTRY | kernel/contribution/registry.go | Extension贡献注册 |
| AGENT_SKILL_CATALOG | kernel/agent_skill/catalog.go | Agent Skill定义 |
| WORKFLOW_REGISTRY | kernel/workflow/registry.go | Workflow定义 |
| EVENT_SUBSCRIPTION_REGISTRY | kernel/event/ | 事件订阅 |
| EVENT_SCHEMA_REGISTRY | kernel/event/ | 事件类型Schema |
| HOOK_REGISTRY | kernel/hook/registry.go | Hook定义 |
| HOST_COMMAND_REGISTRY | kernel/host_command_registry.go | 宿主命令 |
| HOST_REGISTRY | kernel/host_registry/registry.go | Extension宿主能力 |
| SLOT_REGISTRY | kernel/extension_slots/registry.go | Extension槽位 |
| PAGE_REGISTRY | kernel/extension_page_host/host.go | Extension页面 |
| SCHEMA_REGISTRY | kernel/schema_ui/schema.go | UI Schema |
| CHAT_EXTENSION_REGISTRY | kernel/chat_ui_extension/registry.go | Chat扩展 |

## 五、Legacy Registry清单

| ID | 路径 | 分类 | 生产使用 |
|----|------|------|---------|
| LEGACY_AGENT_TOOL_REGISTRY | backend/internal/agent/tool/registry.go | LEGACY_MIGRATION_SOURCE | 无 |
| LEGACY_EXTENSION_REGISTRY | backend/internal/extension/registry.go | LEGACY_FROZEN | 无 |
| LEGACY_PLUGIN_REGISTRY | backend/internal/extension/plugin_registry.go | LEGACY_FROZEN | 无 |
| WORKSHOP_REGISTRY | backend/internal/extension/go-workshop | LEGACY_FROZEN | 无 |

**结论**: Legacy Registry无生产读写、无生产执行、无生产注册。

## 六、Executor系统清单

### 6.1 唯一通用Tool执行器

**KERNEL_EXECUTION_PIPELINE** (`backend/internal/extension/kernel/execution/pipeline.go`) 是经确认的唯一CANONICAL_TOOL_EXECUTOR。

| 指标 | 值 |
|------|---|
| globalToolExecutionPipelineCount | 1 |
| globalToolExecutionPipelinePath | backend/internal/extension/kernel/execution/pipeline.go |

### 6.2 完整执行器清单

| ID | 路径 | 分类 | 生产执行允许 |
|----|------|------|------------|
| KERNEL_EXECUTION_PIPELINE | kernel/execution/pipeline.go | CANONICAL_TOOL_EXECUTOR | 是 |
| KERNEL_RUNTIME_DISPATCHER | kernel/execution/dispatcher.go | RUNTIME_DISPATCHER | 是 |
| WORKFLOW_EXECUTOR | kernel/workflow/executor.go | DOMAIN_EXECUTOR | 是 |
| MCP_SKILL_RUNNER | mcp/skill/runtime.go | DOMAIN_EXECUTOR | 是 |
| SCHEDULE_RUNNER | kernel/schedule/executor.go | DOMAIN_EXECUTOR | 是 |
| TASK_SUPERVISOR | kernel/task_runtime/supervisor.go | DOMAIN_EXECUTOR | 是 |
| TRUSTED_SERVICE_SUPERVISOR | kernel/trusted_service/supervisor.go | DOMAIN_EXECUTOR | 是 |
| DESKTOP_PET_RUNNER | desktoppet/runtime/service.go | DOMAIN_EXECUTOR | 是 |
| INTERACTION_RUNTIME_PIPELINE | interaction/runtime_pipeline.go | DOMAIN_EXECUTOR | 是 |
| LEGACY_EXTENSION_EXECUTOR | extension/executor.go | LEGACY_FROZEN_EXECUTOR | 否 |

### 6.3 Legacy Executor清单

| ID | 路径 | 生产允许 |
|----|------|---------|
| LEGACY_EXTENSION_EXECUTOR | backend/internal/extension/executor.go | 否 |
| WORKFLOW_COMPILER | backend/internal/extension/workflow_compiler.go | 否 |
| WORKFLOW_EXECUTOR(legacy) | backend/internal/extension/workflow_executor.go | 否 |
| AGENT_SKILL_RUNTIME | backend/internal/extension/skill_agent_runtime.go | 否 |

## 七、Permission系统清单

### 7.1 唯一Kernel权限决策中心

**KERNEL_PERMISSION_BROKER** (`backend/internal/extension/kernel/permission/broker.go`) 是经确认的唯一CANONICAL_TOOL_PERMISSION_BROKER。

| 指标 | 值 |
|------|---|
| globalToolPermissionBrokerCount | 1 |
| globalToolPermissionBrokerPath | backend/internal/extension/kernel/permission/broker.go |

### 7.2 完整Permission系统清单

| ID | 路径 | 类型 | 生产决策允许 |
|----|------|------|------------|
| KERNEL_PERMISSION_BROKER | kernel/permission/broker.go | CANONICAL_TOOL_PERMISSION_BROKER | 是 |
| KERNEL_PERMISSION_DEFINITION_REGISTRY | kernel/permission/definition_registry.go | PERMISSION_DEFINITION_REGISTRY | 是 |
| SCOPE_MANAGER | kernel/scope/ | SCOPE_MANAGER | 是 |
| PLATFORM_OS_PERMISSIONS | AndroidManifest.xml/iOS Info.plist | PLATFORM_PERMISSION_PROVIDER | 是(独立) |
| LEGACY_EXTENSION_PERMISSION | extension/permission.go | LEGACY_FROZEN | 否 |

### 7.3 KERNEL_PERMISSION_BROKER消费者

- ExecutionPipeline.PermissionGate
- Workflow StepGuard
- Schedule Service
- Host API Gateway
- Hook Service

### 7.4 决策类型

- DecisionAllow
- DecisionAllowOnce
- DecisionAllowPersistent
- DecisionAllowSession
- DecisionDeny
- DecisionRequireApproval

## 八、Runtime Adapter系统清单

### 8.1 唯一RuntimeAdapterRegistry

**确认**: `backend/internal/extension/kernel/capability/` 包含唯一的全局RuntimeAdapterRegistry。

| 指标 | 值 |
|------|---|
| globalRuntimeAdapterRegistryCount | 1 |
| globalRuntimeAdapterRegistryPath | backend/internal/extension/kernel/capability/ |
| unifiedContract | 所有Adapter实现RuntimeAdapter/RuntimeBinding接口 |

### 8.2 完整Runtime Adapter清单

| ID | 路径 | 类型 | 描述 |
|----|------|------|------|
| JAVASCRIPT_MAIN | kernel/javascript_main/ | CANONICAL_RUNTIME_ADAPTER | JavaScript执行引擎 (QuickJS/Node) |
| WASM_RUNTIME | kernel/wasm_runtime/ | CANONICAL_RUNTIME_ADAPTER | WASM字节码执行引擎 |
| TASK_RUNTIME | kernel/task_runtime/ | CANONICAL_RUNTIME_ADAPTER | 长任务运行时 |
| MCP_REMOTE | mcp/ | CANONICAL_RUNTIME_ADAPTER | MCP远程工具 |
| WORKFLOW_ADAPTER | kernel/workflow/ | CANONICAL_RUNTIME_ADAPTER | Workflow引擎 |
| PLUGIN_HOST | kernel/plugin/ | CANONICAL_RUNTIME_ADAPTER | Extension插件宿主 |
| TRUSTED_SERVICE | kernel/trusted_service/ | CANONICAL_RUNTIME_ADAPTER | 受信任服务 |
| LEGACY_ADAPTER | kernel/capability/adapter_legacy.go | TEMPORARY_MIGRATION_ADAPTER | 临时Legacy迁移适配器 |

### 8.3 Legacy Runtime清单

| ID | 路径 | 分类 | 生产绑定 |
|----|------|------|---------|
| LEGACY_EXTENSION_RUNTIME | extension/runtime.go | LEGACY_FROZEN_RUNTIME | 否 |

bridge用途: `extensionRuntime.Kernel.SetContainer(kernelContainer)` 桥接到新Kernel容器。

## 九、生产入口确认

### 9.1 唯一生产入口清单

| EntryID | 类型 | 目标域 | 使用ToolFacade | 使用ToolRegistry | 使用PermissionBroker | 使用ExecutionPipeline |
|---------|------|--------|---------------|-----------------|--------------------|--------------------|
| CHAT_AGENT | Chat/Agent | Extension Kernel ToolFacade | 是 | 是 | 是 | 是 |
| MCP_TOOL_SYNC | MCP | Extension Kernel ToolFacade | 是 | 是 | 是 | 是 |
| EXTENSION_HOST_API | Extension Host API | Kernel Host API Gateway | 否 | 是 | 是 | 是 |
| WORKFLOW_EXEC | Workflow | Kernel Workflow Engine | 否 | 是 | 是 | 是 |
| SCHEDULE_EXEC | Schedule | Kernel Schedule Service | 否 | 是 | 是 | 是 |

### 9.2 Chat生产接线验证

| 项目 | 结果 |
|------|------|
| SetToolRuntime调用次数 | 1 (services.go:335) |
| 目标 | newChatToolRuntimeAdapter(toolFacade) |
| adapter分类 | BOUNDARY_ADAPTER (只转发，不保存Registry、不独立执行Tool、不判权限、不选择Provider) |
| 第二绑定 | 未发现 |
| Legacy fallback | 禁用 (FallbackOnError=false) |

### 9.3 MCP接线验证

| 项目 | 结果 |
|------|------|
| MCP同步路径 | mcpToolFacadeSyncerAdapter -> facade.SyncMCPTools -> registry.BatchRegister |
| 直接LLM暴露 | 未发现 |
| 双注册监控 | 通过duplicate metric监控，无永久双路径 |
| 无第二MCP暴露路径 | 确认 |

### 9.4 生产Tool执行链

```
Layer1 Entry (Chat/Agent/MCP/Extension/Workflow/Schedule)
    ↓
Layer2 ToolFacade (PreferKernel=true, FallbackOnError=false)
    ↓
Layer3 ToolRegistry (GetByModelName / List)
    ↓
Layer4 ExecutionPipeline.Execute
    ↓
Layer5 16-Gate Chain
    ↓
Layer6 RuntimeDispatcher (lookup via RuntimeAdapterRegistry)
    ↓
Layer7 Runtime Adapter (JS/WASM/Task/MCP/Workflow/Plugin/TrustedService)
    ↓
Layer8 UnifiedToolResult
```

## 十、生产旁路扫描

### 10.1 扫描覆盖

- 扫描函数: Execute, Dispatch, Run, Invoke, Call, ExecuteModelTool, ExecuteTool, ExecuteSkill, ExecuteWorkflow
- 扫描目录: backend/cmd/server/, backend/internal/chat/, backend/internal/extension/, backend/internal/extension/kernel/

### 10.2 扫描结果

| 分类 | 数量 |
|------|------|
| PRODUCTION_TOOL_BYPASS | 0 |
| PRODUCTION_PERMISSION_BYPASS | 0 |
| PRODUCTION_EXECUTION_BYPASS | 0 |
| VALID_DOMAIN_DIRECT_CALL | 0 |
| TEST_ONLY | 0 |
| MIGRATION_ONLY | 0 |
| UNKNOWN | 0 |

### 10.3 具体检查项

| 检查项 | 结果 |
|--------|------|
| Chat直接执行检查 | PASS - chat.execute使用s.toolRuntime (chatToolRuntimeAdapter -> ToolFacade) |
| MCP直接执行检查 | PASS - MCP使用mcpToolFacadeSyncerAdapter -> ToolFacade |
| Workflow直接执行检查 | PASS - Workflow executor使用PermissionBroker.Evaluate |
| Schedule直接执行检查 | PASS - Schedule使用KernelToolExecutorAdapter(executionKernel) |

**结论**: 所有Chat/MCP/HostAPI/Workflow/Schedule路径均通过Canonical Kernel，无生产旁路。

## 十一、Fake/Mock Provider扫描

### 11.1 扫描结果

| ID | 路径 | 分类 | 生产中是否Fake |
|----|------|------|---------------|
| BEHAVIOR_ADAPTERS | desktoppet/behavior/adapters/ | DOMAIN_ADAPTER | 否 |
| NOOP_PUBLISHER | cmd/server/services.go:1150 | PRODUCTION_INFRASTRUCTURE | 否 |
| NOOP_PROVIDERS_FOR_TESTING | **/*_test.go | TEST_ONLY | 测试专用 |
| CONTEXT_LOADER_REGISTRY | interaction/context_loader.go | DOMAIN_REGISTRY | 否 |
| PROVIDER_REGISTRY | runtimeorchestrator/provider_registry.go | DOMAIN_REGISTRY | 否 |
| CIRCUIT_BREAKER_REGISTRY | mindruntime/circuit_breaker.go | INFRASTRUCTURE | 否 |

### 11.2 Fake总结

| 分类 | 数量 |
|------|------|
| TEST_ONLY | 1 |
| PRODUCTION_FAKE | 0 |
| PRODUCTION_FALLBACK | 0 |
| PRODUCTION_REQUIRED | 1 |

**结论**: 生产环境中无Fake Provider、无Fake Runtime。

## 十二、重复系统检测

### 12.1 检测总结

| 维度 | 重复生产数量 |
|------|------------|
| 重复生产Tool Registry | 0 |
| 重复生产Executor | 0 |
| 重复Permission系统 | 0 |
| 重复Runtime系统 | 0 |
| 永久双注册 | 0 |

### 12.2 Registry重复分析

| Registry | 创建者 | 消费者 | 判定 |
|----------|--------|--------|------|
| ToolRegistry | ContainerBuilder | ToolFacade, ExecutionKernel.ToolResolver | 唯一实例 |
| LegacyToolRegistry | agent/tool | Migration sourced only | 不同生命周期阶段，非重复 |
| MCPRegistry | mcpmanager | MCP Server连接管理 | 领域注册中心，非Tool Registry重复 |

### 12.3 Executor重复分析

- 全局ExecutionPipeline: 1个
- Workflow/Schedule/TaskExecutor: 通过RuntimeAdapter或ExecutionKernel接Kernel的Domain Executor，非重复
- MCPRunner: 通过ToolFacade同步到Kernel，非重复

### 12.4 Permission重复分析

- 唯一PermissionBroker: DefaultPermissionBroker
- ScopeManager: PermissionBroker的组成部分，非独立系统
- Legacy Permission: 冻结，无生产决策

## 十三、重复系统Guard规则（冻结）

### 13.1 规则概述

共16条BLOCKER级别规则，用于防止B10~B154期间创建第二套系统。

### 13.2 规则列表

| 规则ID | 描述 | 规范目标 |
|--------|------|---------|
| NO_SECOND_GLOBAL_TOOL_REGISTRY | 禁止创建第二套全局Agent Tool Registry | capability.ToolRegistry (NewToolRegistry只允许在ContainerBuilder中调用) |
| NO_SECOND_TOOL_EXECUTION_PIPELINE | 禁止绕过ExecutionPipeline创建独立通用Tool执行器 | ExecutionPipeline |
| NO_SECOND_PERMISSION_BROKER | 禁止创建第二套Kernel Permission决策系统 | permission.PermissionBroker |
| NO_PARALLEL_AGENT_TOOL_RUNTIME | 禁止向Chat绑定非ToolFacade的ModelToolRuntime实现 | chatToolRuntimeAdapter(ToolFacade) |
| NO_PERMANENT_LEGACY_FALLBACK | 生产环境禁止启用FallbackOnError=true | DefaultToolFacadeConfig{FallbackOnError: false} |
| NO_NEW_TOOL_IN_LEGACY_REGISTRY | 禁止向旧Registry注册任何新Tool | backend/internal/agent/tool.Registry |
| NO_NEW_PROVIDER_ON_LEGACY_RUNTIME_ADAPTER | B10以后新Provider禁止注册到LegacyRuntimeAdapter | adapter_legacy.go |
| NO_SECOND_MCP_TOOL_EXPOSURE_PATH | MCP Tool只能通过ToolFacade.SyncMCPTools注册到Kernel | mcpToolFacadeSyncerAdapter |
| NO_SECOND_SKILL_RUNTIME | 禁止在Kernel AgentSkill之外另立Skill执行系统 | AgentSkillCatalog |
| NO_SECOND_WORKFLOW_ENGINE | 禁止在Kernel Workflow之外另立Workflow执行引擎 | WorkflowExecutor |
| NO_SECOND_EVENT_BUS | 禁止在Kernel Event之外另立事件系统 | Event.Service |
| NO_SECOND_HOOK_SYSTEM | 禁止在Kernel Hook之外另立Hook系统 | Hook.Service |
| NO_SECOND_SCHEDULER | 禁止在Kernel Schedule之外另立调度系统 | Schedule.Service |
| NO_SECOND_TASK_RUNTIME | 禁止在Kernel TaskRuntime之外另立长任务系统 | TaskRuntime.Service |
| NO_HOST_API_PERMISSION_BYPASS | Host API必须经过PermissionBroker和ScopeManager | host_api.Gateway |
| PROVIDER_MUST_USE_TOOL_REGISTRY | 所有Future Provider必须注册到Canonical ToolRegistry | capability.ToolRegistry |

### 13.3 冻结范围

此冻结规则自B9P6完成时生效，持续至B154（或用户明确解除）。

任何B10~B154的改动若违反以上规则，将被Code Review拦截。

## 十四、迁移Adapter系统

### 14.1 Adapter清单

| ID | 路径 | 分类 | 退出条件 |
|----|------|------|---------|
| TOOL_MIGRATION | kernel/tool_migration/ | MIGRATION_SYSTEM | 所有builtin handler迁移为Kernel builtin ToolDefinition |
| SKILL_MIGRATION | kernel/skill_migration/ | MIGRATION_SYSTEM | 所有Legacy AgentSkill迁移到AgentSkillCatalog + 生成ToolMapping |
| MCP_MIGRATION | kernel/mcp_migration/ | MIGRATION_SYSTEM | MCP Server注册稳定通过ToolFacade.SyncMCPTools |
| WORKFLOW_MIGRATION | kernel/workflow_migration/ | MIGRATION_SYSTEM | Legacy Workflow definition全部迁移到Kernel Workflow |
| PLUGIN_MIGRATION | kernel/plugin_migration/ | MIGRATION_SYSTEM | 所有Go Plugin迁移为Kernel Extension |
| AMITIAX_MIGRATION | kernel/amitiax_migration/ | MIGRATION_SYSTEM | .amitiax v1包系统被Kernel Package取代 |
| DATA_MIGRATION | kernel/data_migration/ | MIGRATION_SYSTEM | 所有extension数据存储迁移到kernel.db |
| LEGACY_RUNTIME_ADAPTER | kernel/capability/adapter_legacy.go | TEMPORARY_MIGRATION_ADAPTER | 无ToolDefinition引用RuntimeBinding=legacy |

### 14.2 统计

- 总数: 8
- 有退出条件: 8/8 (100%)
- 允许生产fallback: 0
- 允许新注册: 0

## 十五、Legacy系统分类

### 15.1 Legacy系统总计

| 类型 | 数量 |
|------|------|
| Legacy Registry | 4 |
| Legacy Executor | 4 |
| Legacy Runtime | 1 |
| Legacy Permission | 1 |
| Legacy Counter Metrics | 17 |

### 15.2 生产Legacy执行

| 指标 | 值 |
|------|---|
| 生产Legacy Registry Reader数 | 0 |
| 生产Legacy Registry Writer数 | 0 |
| 生产Legacy Executor数 | 0 |
| 生产Legacy Fallback数 | 0 |

**结论**: 生产环境零Legacy执行。

## 十六、FinalGate / Legacy释放门

### 16.1 FinalGate指标

| 类别 | 指标数 | 具体指标 |
|------|--------|---------|
| duplicate | 3 | duplicate_mcp_tool_registrations, duplicate_contribution_registrations, duplicate_schedule_runs |
| orphan | 8 | orphan_candidate_contributions, orphan_runtime_instances, orphan_ui_sessions, orphan_sandbox_sessions, orphan_staging_directories, orphan_installed_directories, orphan_artifacts, orphan_installation_generations |
| cleanup | 5 | failed_cleanup_resources, artifact_hash_mismatch, corrupted_artifacts, installation_without_files, files_without_installation |
| recovery | 5 | incomplete_package_operations, requires_recovery_operations, ambiguous_recovery_operations, unresolved_package_operations, failed_uninstall_restores |
| legacy | 5 | legacy_package_read_calls, legacy_package_write_calls, legacy_tool_execute_calls, legacy_mcp_execute_calls, new_package_legacy_read_calls |
| audit | 2 | audit_incomplete_operations, installation_read_model_mismatches |
| lifecycle | 2 | lifecycle_requires_recovery, active_contribution_for_disabled_installation |
| security | 2 | unsigned_production_packages, untrusted_installed_packages |

总计: 31 FT指标，覆盖8个类别。

### 16.2 Extension相关B9P6 FinalGate候选

- production_tool_bypass_count → 0
- duplicate_global_tool_registry_count → 0
- duplicate_permission_broker_count → 0
- production_fake_provider_count → 0
- new_legacy_tool_registration_count → 0

### 16.3 LegacyCallCounter指标 (6个)

- toolExecuteCalls
- modelToolsCalls
- promptHookCalls
- mcpExecuteCalls
- packageWriteCalls
- duplicateMCPToolRegistrations
- duplicateContributionRegistrations

### 16.4 LegacyCallCounter原子计数器 (17个)

pluginStart, pluginDispatch, toolExecute, packageInstall, skillExecute, mcpToolRegister, scheduleTick, promptHook, workflowExecute, hostCommand, extensionResolve, hostCommandExecution, legacyModelTools, legacyToolExecute, legacyFallback, legacyPromptHook, legacyPackageOperations。

## 十七、Canonical System Resolution总结

| 系统域 | Canonical组件 | 状态 | 重复数 |
|--------|-------------|------|--------|
| Agent Tool Entry | ToolFacade | ACTIVE | 0 |
| Capability | ToolRegistry | ACTIVE | 0 |
| Execution | ExecutionPipeline | ACTIVE | 0 |
| Permission | PermissionBroker | ACTIVE | 0 |
| Runtime | RuntimeAdapterRegistry | ACTIVE | 0 |
| Assembly | ContainerBuilder | ACTIVE | 0 |
| MCP | MCPToolFacadeSyncer | ACTIVE | 0 |
| Workflow | WorkflowEngine | ACTIVE | 0 |
| Skill | AgentSkillCatalog | ACTIVE | 0 |

**结论**: 每个域都有且仅有一个Canonical生产系统，无重复生产系统。

## 十八、B140前置条件评估

### 18.1 静态前置条件

| 条件 | 状态 |
|------|------|
| toolFacadeProductionEntry | PASS |
| kernelToolRegistryCanonical | PASS |
| executionPipelineCanonical | PASS |
| permissionBrokerCanonical | PASS |
| legacyFallbackDisabled | PASS (FallbackOnError=false) |
| newLegacyRegistrations | PASS (0 detected) |
| productionBypassCount | PASS (0 detected) |
| duplicateProductionRegistryCount | PASS (0 detected) |
| productionFakeProviderCount | PASS (0 detected) |

**静态前置条件**: 全部满足。

### 18.2 运行时证明需求（后续B140验证）

| 指标 | 目标 |
|------|------|
| legacyFallbackTotal | = 0 |
| legacyToolExecuteCalls | = 0 |
| legacyModelToolsCalls | = 0 |

## 十九、B141/B146/B147前置条件评估

### 19.1 B141 (Platform Provider接入)

| 条件 | 状态 |
|------|------|
| runtimeAdapterContract | READY |
| executionPipelinePassthrough | READY |
| noProviderDirectModelExposure | READY (B9P6确认无现有违规) |
| noDuplicatePermissionBroker | READY |

### 19.2 B146 (Parity Gap总验收)

已有Canonical系统:
- MCP via ToolFacade
- Skill via AgentSkillCatalog
- Workflow via Kernel Workflow
- Hook via Kernel Hook
- Event via Kernel Event
- Schedule via Kernel Schedule

**状态**: CONFIRMED

## 二十、兼容层扫描

| 分类 | 数量 |
|------|------|
| 类型兼容(DTO) | 6 |
| 临时迁移兼容 | 1 |
| 永久双执行 | 0 |
| 永久双写 | 0 |

**结论**: 无永久兼容层，仅有临时迁移适配和DTO转换。

## 二十一、历史/并行范围验证

| 检查项 | 结果 |
|--------|------|
| B9历史文件修改 | 0 |
| B9P1文件修改 | 0 |
| B9P2文件修改 | 0 |
| 业务源码修改 | **0** |
| 并行目录违规 | 0 (无写入b9p3/或b9p5/) |

**结论**: 本次任务未修改任何业务源码，未触碰B9/B9P1/B9P2文件，未写入并行任务目录。

## 二十二、审计结论

### 22.1 唯一生产入口确认

| 维度的唯一性 | 结论 |
|-------------|------|
| Agent Tool入口 | 唯一 (ToolFacade) |
| 全局Tool注册中心 | 唯一 (ToolRegistry) |
| 通用Tool执行器 | 唯一 (ExecutionPipeline) |
| 权限决策中心 | 唯一 (PermissionBroker) |
| Kernel组装入口 | 唯一 (ContainerBuilder) |
| ModelToolRuntime (Chat) | 唯一 (chatToolRuntimeAdapter→ToolFacade) |
| MCP工具暴露 | 唯一 (ToolFacade.SyncMCPTools) |

### 22.2 扫描零发现

| 维度 | 发现数 |
|------|--------|
| 生产旁路 | 0 |
| 生产重复注册 | 0 |
| 生产Legacy执行 | 0 |
| 生产Fake Provider | 0 |
| 永久兼容层 | 0 |
| 生产权限绕过 | 0 |

### 22.3 摘要统计

| 指标 | 值 |
|------|---|
| globalAgentToolRegistryCount | 1 |
| globalToolExecutionPipelineCount | 1 |
| globalToolPermissionBrokerCount | 1 |
| globalModelToolFacadeCount | 1 |
| productionLegacyExecutors | 0 |
| productionBypassCount | 0 |
| productionFakeProviderCount | 0 |
| duplicateProductionRegistryCount | 0 |
| duplicateProductionExecutorCount | 0 |
| duplicatePermissionSystemCount | 0 |
| duplicateRuntimeSystemCount | 0 |
| permanentDualRegistrationCount | 0 |
| temporaryMigrationAdapters | 8 |
| migrationAdaptersWithoutExitCondition | 0 |

## 二十三、B9P4交付物

本任务生成了 `b9p4_kernel_input.json`，包含Extension Kernel完整Schema、Canonic

al Systems、Production Chains、Guard Rules等结构化数据，供B9P4 (生产环境唯一入口固化实施) 消费。

同时生成了 `B9P4_input_manifest.md` 提供人类可读摘要。

## 二十四、最终状态

| 项目 | 值 |
|------|---|
| TaskId | B9P6 |
| 审计结论 | PASS (with drift note) |
| 业务源码修改 | 0 |
| B9P1/B9P2历史文件修改 | 0 |
| 并行目录违规 | 0 |
| Drift | services.go (commit d42ddf7, non-kernel change) |
| 下游 | B9P4 (b9p4_kernel_input.json已就绪) |

## 二十五、输出文件清单

### 25.1 原数据JSON文件

1. input_manifest.json - 输入范围与素材清单
2. b9p6_status.json - 执行状态
3. extension_kernel_inventory.json - Extension Kernel核心系统清单
4. canonical_registry_inventory.json - Canonical Registry清单
5. canonical_executor_inventory.json - Canonical Executor清单
6. canonical_permission_inventory.json - Canonical Permission清单
7. canonical_runtime_inventory.json - Canonical Runtime清单
8. canonical_system_resolution.json - 唯一系统判定
9. legacy_registry_inventory.json - Legacy Registry清单
10. legacy_executor_inventory.json - Legacy Executor清单
11. legacy_runtime_inventory.json - Legacy Runtime清单
12. legacy_usage_classification.json - Legacy用途分类
13. legacy_reference_inventory.json - Legacy引用关系
14. legacy_counter_inventory.json - Legacy计数器
15. final_gate_inventory.json - FinalGate指标清单
16. production_entrypoints.json - 生产入口清单
17. production_tool_chain.json - 生产Tool执行链
18. production_registration_chain.json - 生产注册链
19. production_permission_chain.json - 生产权限链
20. production_runtime_chain.json - 生产运行时链
21. production_bypass_inventory.json - 生产旁路扫描结果
22. direct_execution_inventory.json - 直接执行扫描结果
23. duplicate_registry_inventory.json - 重复Registry检测
24. duplicate_permission_inventory.json - 重复Permission检测
25. duplicate_executor_inventory.json - 重复Executor检测
26. duplicate_runtime_inventory.json - 重复Runtime检测
27. duplicate_system_guard.json - 重复系统Guard冻结规则 (16条)
28. fake_mock_provider_inventory.json - Fake/Mock扫描结果
29. migration_adapter_inventory.json - 迁移Adapter清单
30. compatibility_layer_inventory.json - 兼容层清单
31. runtime_adapter_inventory.json - Runtime Adapter清单
32. chat_tool_wiring.json - Chat接线分析
33. mcp_tool_wiring.json - MCP接线分析
34. skill_wiring.json - Skill接线分析
35. workflow_wiring.json - Workflow接线分析
36. hook_event_schedule_wiring.json - Hook/Event/Schedule接线分析
37. production_entry_guard.json - 生产入口守卫
38. migration_exit_conditions.json - 迁移退出条件
39. b140_preconditions.json - B140前置条件评估
40. b9p4_kernel_input.json - B9P4消费用输入
41. B9P4_input_manifest.md - B9P4输入Manifest (Markdown)
42. verification.log - 验证日志
43. B9P6_ExtensionKernel唯一生产入口收口报告.md - 本文档

## 二十六、B9P6 PASS条件复核

| PASS条件 | 状态 |
|----------|------|
| extension-kernel是唯一生产源码入口 | 确认 |
| 不存在第二套全局Tool Registry | 确认 |
| 不存在第二套通用Tool Executor | 确认 |
| 不存在第二套Permission决策系统 | 确认 |
| 不存在生产环境Fake/Mock Provider | 确认 |
| 不存在生产环境Legacy执行 | 确认 |
| 不存在生产环境旁路 | 确认 |
| 不存在永久兼容层 | 确认 |
| 不存在永久双注册 | 确认 |
| 所有生产接入路径唯一 | 确认 |
| Guard冻结规则覆盖B10~B154 | 确认 (16条BLOCKER) |
| B9P4输入数据完整 | 确认 |
| 业务源码零修改 | 确认 |

**B9P6 最终结论: PASS**
