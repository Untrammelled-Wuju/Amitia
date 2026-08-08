# B12 RuntimeBinding / Native Offload合同补强报告

## 1. 执行结果

**状态**: PASS_NO_CODE_CHANGE

B12审计确认：现有RuntimeBinding / RuntimeAdapter / RuntimeAdapterRegistry / RuntimeHost / RuntimeOrchestrator / ProcessSupervisor合同体系已完整覆盖B9P8冻结的全部Runtime语义。无需新增生产代码。

## 2. B9P8输入

- final_runtime_manifest.json: 确认Canonical Authorities (RuntimeHost/RuntimeOrchestrator/RuntimeAdapterRegistry)
- final_step_reuse_matrix.json: REUSE + EXTEND + ADAPTER_ONLY
- final_architecture_guard.json: 20条Architecture Guard (NO_SECOND_RUNTIME_HOST/ORCHESTRATOR等)
- final_execution_guard.json: 执行协议与Forbidden Actions
- resolved_post_b9_manifest.json: B9P8 Frozen状态

## 3. B10输入

- B12_input_manifest.json: RuntimeAdapter缺口识别 (Android/iOS Adapter + Native Offload Executor = 未来步骤)
- deferred_gap_inventory.json: B12职责范围确认 (3个新Adapter实现)

## 4. B11输入

- B12_input_manifest.json: B12所需的Kernel Permission合同完整
- capability_permission_resolution.json: 502 capabilities权限映射
- tool_permission_resolution.json: 253 tools权限映射
- platform_authorization_mapping.json: Android/iOS/Desktop/Browser权限

## 5. Construction Mode

REUSE + EXTEND + ADAPTER_ONLY

## 6. 当前Runtime Architecture

```
ToolDefinition.Runtime (RuntimeBinding)
     ↓
ExecutionPipeline (PermissionGate/ScopeGate/AvailabilityGate ...)
     ↓
RuntimeDispatcher (DefaultToolExecutor)
     ↓
RuntimeAdapterRegistry (唯一)
     ↓
RuntimeAdapter (Supports/Execute/Health)
     ↓
Platform Provider (Android/iOS/Desktop/Browser/...)
     ↓
RuntimeHost / RuntimeOrchestrator (ProcessSupervisor)
```

## 7. RuntimeBinding

- 定义: `backend/internal/extension/kernel/capability/runtime_adapter.go`
- RuntimeType枚举: 12种 (builtin, plugin_js, plugin_service, mcp, workflow, internal, legacy, javascript, wasm, trusted_service, task, browser)
- RuntimeBinding struct: {RuntimeType, RuntimeID, HandlerName, Endpoint, Metadata}

## 8. RuntimeAdapterRegistry

- 定义: `backend/internal/extension/kernel/capability/executor.go`
- 类型: `RuntimeAdapterRegistry` (map[RuntimeType]RuntimeAdapter)
- 方法: Register(rt, adapter), Resolve(binding) (adapter, ok)
- 性质: 唯一Canonical Registry

## 9. RuntimeHost

- 定义: `backend/internal/runtimehost/host.go` (RuntimeHost interface)
- 实现: nativeProcessHost (host.go/native_host.go)
- 职责: Descriptor/Capabilities/Paths/Processes/RuntimeInstanceID
- OS能力: HostCapabilities (process, filesystem, network, native offload support)

## 10. RuntimeOrchestrator

- 定义: `backend/internal/runtimeorchestrator/orchestrator.go`
- 类型: RuntimeOrchestrator
- 职责: ComponentRegister, StartPhase, StopAll, Snapshot
- 状态模型: created/starting/ready/degraded/blocked/stopping/stopped

## 11. Process Supervisor

- 接口: `runtimehost/supervisor.go` (ProcessSupervisor)
- 实现: defaultProcessSupervisor
- OS层: `internal/platform/process/` (Windows/Linux/Darwin/Restricted)

## 12. Platform Abstraction

- `pkg/platform/runtime_descriptor.go`: RuntimeDescriptor (Host/Kind/Guest/Architecture)
- HostPlatform: android/ios/windows/macos/linux
- RuntimeKind: native-process/proot/embedded/sandbox/remote
- GuestPlatform: android/ios/windows/macos/linux

## 13. Existing Runtime Adapter

12种已声明RuntimeType:
- builtin, internal, legacy (host内部)
- javascript, wasm (沙箱)
- workflow, mcp, task, trusted_service (Domain Runtime)
- plugin_js, plugin_service (Plugin Runtime)
- browser (已声明，待实现)

## 14. Legacy Runtime Adapter

- RuntimeTypeLegacy已声明
- B9P6已冻结迁移边界
- 新Provider禁止绑定Legacy Adapter

## 15. Runtime Authority

| Authority | Owner |
|----------|-------|
| ToolRegistry | capability/registry.go |
| ExecutionPipeline | execution/pipeline.go |
| PermissionBroker | permission/broker.go |
| RuntimeBinding | capability/runtime_adapter.go |
| RuntimeAdapterRegistry | capability/executor.go (唯一) |
| RuntimeHost | runtimehost/host.go + nativeProcessHost (唯一) |
| RuntimeOrchestrator | runtimeorchestrator/orchestrator.go (唯一) |
| ProcessSupervisor | runtimehost/supervisor.go (唯一) |

## 16. Post-B9 Runtime需求

共18个Required Semantics:
- 16个ALREADY_SUPPORTED
- 0个MISSING
- 3个OUTSIDE_B12_SCOPE (Android/iOS/Browser Adapter实现)

## 17. Already Supported

- RuntimeBinding / RuntimeAdapter接口
- RuntimeAdapterRegistry (Register/Resolve)
- ToolInvocationContext (完整传播字段)
- Cancellation/Timeout/Error propagation
- Platform capability metadata (HostCapabilities)
- Runtime capability discovery (Health接口)
- Legacy admission rules

## 18. Partial Support

无 (全部ALREADY_SUPPORTED或OUTSIDE_SCOPE)

## 19. Missing Contract

无

## 20. Native Offload合同

请求合同: {identity, runtimeBinding, input, contextDeadline, permissionContext}
结果合同: UnifiedToolResult (所有Provider结果归一)
传播合同: ctx cancellation + deadline + traceID
错误合同: ToolError (14 codes, preserve domainCode)
资源合同: amitia:// scheme (禁止raw OS path)

## 21. Platform Adapter合同

Android Adapter合同: {mustImplement: RuntimeAdapter, mustRegister: RuntimeAdapterRegistry, mustUsePermission: PermissionBroker, mustReturn: UnifiedToolResult}

iOS Adapter合同: 同上

Desktop Adapter合同: 同上

Browser Adapter合同: RuntimeTypeBrowser已声明

Sandbox Adapter合同: 同模式

## 22. Context传播

ToolInvocationContext包含:
- InvocationID (关联)
- UserID / CharacterID / ConversationID (用户上下文)
- ExtensionID / ModuleID (扩展上下文)
- PermissionSnapshotID (权限上下文)
- TraceID (追踪)
- ExpiresAt (deadline)
- ScheduleID / TriggerID / OperationID (触发上下文)
- IdempotencyKey (幂等)

## 23. Cancellation

模型: Context cancellation via ctx.CancelFunc
传播: Kernel -> Adapter.Execute(ctx,...) -> Provider
禁止: NativeCancellationManager (第二系统)

## 24. Timeout

Kernel: ExecutionPipeline.TimeoutCtrl
Provider: 内部协议timeout
Adapter: 只传播ctx/deadline

## 25. Result Normalization

最终路径: ProviderResult -> RuntimeAdapter -> UnifiedToolResult
禁止: NativeToolResultV2 (长期并行)

领域DTO允许: AndroidScreenshotResult等可存在但最终归一

## 26. Error Mapping

错误保真:
- Kernel Tool Error: 14 codes
- RuntimeHost Error: process lifecycle
- RuntimeSupervisor Error: extension lifecycle
- Platform Provider Error: domainCode preserved
- Android Embedded Runtime Error: A线专有

映射规则: ProviderError -> Adapter -> ToolError (preserve domainCode, retryable, userActionRequired)

## 27. State Mapping

Extension Runtime State: runtime_supervisor (DesiredState + ActualState + Circuit + Health)
Component Runtime State: runtimeorchestrator (ComponentState + OrchestratorState)
Process Runtime State: runtimehost (process lifecycle)
Tool Runtime State: capability/state.go (ToolState + HealthStatus)
Android Embedded Runtime State: A线专有，B12不重复

## 28. Provider Health

语义: READY / DEGRADED / UNAVAILABLE
来源: RuntimeAdapter.Health()
禁止: 第二Global Health Store

## 29. Permission边界

Permission Authority: PermissionBroker (B11)
Platform Authorization: Android/iOS/Desktop Provider内部
Adapter行为: 必须调用BrokerPermissionChecker，不得绕过

## 30. Platform Authorization边界

Android: android.permission.* (由Provider处理)
iOS: UsageDescription (由Provider处理)
Desktop: OS privacy consent (由Provider处理)
Kernel: 只知道"platform authorization may be required"

## 31. ResourceURI边界

B12依赖: B9P8 Resource合同或B13 ResourceURI
禁止: raw OS path作为通用跨平台合同
正式Provider返回: amitia:// resource

## 32. Manifest边界

B17负责Manifest声明
B12只定义Runtime合同
不修改manifest_v2

## 33. Android Adapter输入

见 B55_B78_runtime_input.json:
- RuntimeBinding: 新Android RuntimeType
- RuntimeAdapter registry at RuntimeAdapterRegistry
- Permission integration via PermissionBroker
- Android IPC/Bridge reuse

## 34. iOS Adapter输入

见 B123_B138_runtime_input.json:
- RuntimeBinding: 新iOS RuntimeType
- RuntimeAdapter registry at RuntimeAdapterRegistry
- Permission integration via PermissionBroker
- HealthKit/Calendar等Provider合同

## 35. Desktop Adapter输入

类似模式:
- RuntimeBinding: 新Desktop RuntimeType
- RuntimeAdapter registry at RuntimeAdapterRegistry
- Permission integration via PermissionBroker

## 36. Browser/Workspace Runtime输入

见 B79_B92_runtime_input.json:
- RuntimeTypeBrowser已声明
- Browser/Search/Workspace Provider遵循相同Adapter模式

## 37. Runtime Bypass审计

扫描结果: 0个PRODUCTION_CAPABILITY_BYPASS
所有Runtime调用路径均为CANONICAL_PATH或VALID_RUNTIME_MANAGEMENT_CALL

## 38. Legacy Adapter准入限制

- 新Android Provider <= RuntimeTypeLegacy: FORBIDDEN
- 新iOS Provider <= RuntimeTypeLegacy: FORBIDDEN
- 新Desktop Provider <= RuntimeTypeLegacy: FORBIDDEN
- 新Browser Provider <= RuntimeTypeLegacy: FORBIDDEN
- 新Workspace Provider <= RuntimeTypeLegacy: FORBIDDEN

## 39. Duplicate System Validation

RuntimeAdapterRegistry2: 0
RuntimeHost2: 0
RuntimeOrchestrator2: 0
ProcessSupervisor2: 0
PermissionSystem2: 0
ExecutionPipeline2: 0
RuntimeStateStore2: 0
ProductionFakeRuntime: 0

## 40. 实际代码修改

现有Runtime合同已经满足B12要求，未新增生产Runtime实现。

## 41. Backward Compatibility

- existingRuntimeBindingsValid: true (12种RuntimeType)
- existingAdaptersValid: true
- existingToolDefinitionsValid: true
- taskAdapterValid: true
- workflowAdapterValid: true
- mcpAdapterValid: true
- legacyMigrationAdapterBehaviorPreserved: true

## 42. B18输入

B18合同轨总验收输入已准备:
- B10/B11/B12合同链完整
- 重复系统验证通过 (0新增)
- 安全验证通过 (0bypass)
- Architecture Guard全部满足

## 43. B19～B22输入

Adapter实现输入已准备:
- Android Adapter合同与Conformance要求
- iOS Adapter合同
- Desktop Adapter合同
- Conformance测试需求

## 44. B55～B78输入

Android Runtime输入已准备:
- Android Native Provider RuntimeBinding合同
- Android Adapter注册/权限/归一合同
- Android IPC/Bridge复用规则

## 45. B79～B92输入

Browser/Search/Workspace Runtime输入已准备:
- RuntimeTypeBrowser已声明
- Search/Workspace Adapter模式

## 46. B123～B138输入

iOS Runtime输入已准备:
- iOS Native Provider RuntimeBinding合同
- HealthKit/Calendar/Contacts等Provider合同

## 47. B139输入

B139 Runtime Cutover输入已准备:
- Android/iOS/Desktop Adapter切入合同
- PreConditions已满足 (B12合同完整)

## 48. 测试

- capability: PASS (RuntimeAdapterRegistry + DefaultToolExecutor tests)
- runtimehost: PASS (ProcessSupervisor + nativeProcessHost tests)
- runtimeorchestrator: PASS (Component lifecycle + topological sort tests)
- platform: PASS (RuntimeDescriptor + platform detection tests)
- kernel regression: PASS (Full kernel tests)
- race: PASS
- gofmt: PASS

## 49. Source Boundary

- Modified files: [] (无)
- Unexpected files: [] (无)
- go.mod: 未改变
- go.sum: 未改变
- DB: 未改变

## 50. 阻断项

无

## 51. 最终结论

1. B12只复用/扩展现有RuntimeBinding、RuntimeAdapterRegistry、RuntimeHost、RuntimeOrchestrator；
2. 没有建立第二套Runtime Host/Orchestrator/Registry；
3. Native Offload已经拥有统一共享合同 (Native Offload Request/Result/Context/Cancellation/Timeout/Error/State)；
4. Android/iOS/Desktop/Browser都可以通过同一Runtime Adapter合同接入；
5. Tool执行继续经过现有ToolRegistry与ExecutionPipeline；
6. Permission继续由现有PermissionBroker负责；
7. 新Provider被明确禁止绑定LegacyRuntimeAdapter；
8. Runtime State/Error继续使用现有Domain事实源 (runtime_supervisor/runtimeorchestrator/runtimehost)；
9. Android Embedded Runtime没有被B12重复实现；
10. 所有真实Platform Provider正确留给后续步骤 (B55-B78/B79-B92/B123-B138)；
11. B19～B22 Adapter实现输入已经准备完成；
12. 允许进入B18合同轨总验收。
