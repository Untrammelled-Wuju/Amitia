# B20 iOS Platform Adapter正式实现报告

## 1. 执行结果

**状态**: PASS

B20成功实现iOS唯一正式Platform Adapter (iosRuntimeAdapter)，接入现有RuntimeAdapterRegistry，未创建任何重复系统、未实现具体iOS业务Provider。

## 2. B9P8输入

- **b9p8_status.json**: PASS
- **final_step_reuse_matrix.json**: 读取完成，B20分类为ADAPTER_ONLY
- **final_runtime_manifest.json**: 读取完成，确认3个newAdapterRequired包含iOS
- **final_permission_manifest.json**: 读取完成

## 3. B12输入

- **b12_status.json**: PASS_NO_CODE_CHANGE
- **platform_adapter_contract.json**: 读取完成，确认iOS Adapter必须实现RuntimeAdapter接口并注册到RuntimeAdapterRegistry
- **B123_B138_runtime_input.json**: 读取完成，确认ios_native RuntimeType命名约定
- **runtime_binding_matrix.json**: 通过B17确认ios_native为futureAdapter

## 4. B18输入

- **b18_status.json**: PASS
- **platform_adapter_phase_ready.json**: 读取完成
  - iosAdapterAllowed: true
  - platformAdapterPhaseAllowed: true
- **B20_input_manifest.json**: 读取完成
  - implementationGaps确认: iosRuntimeAdapter/iOSPlatformProvider/iOSPermissionMapping需B20实现

## 5. Construction Mode

ADAPTER_ONLY - 仅实现Adapter基础设施，不实现具体业务Provider

## 6. 当前iOS工程

- **iOS Project Shell**: 存在 (mobile_app/ios/)，为标准Flutter iOS构建壳
- **AppDelegate.swift**: 仅注册GeneratedPluginRegistrant，未实现任何Runtime Adapter桥接
- **RunnerTests.swift**: 空测试模板
- **自定义Runtime服务**: 无
- **自定义Native Provider**: 无

## 7. Flutter iOS基础

- iOS平台使用与Android共享的MethodChannel `com.amitia.amitia_app/runtime`
- Dart侧`runtime_backend_connection_source.dart`跨Android/iOS共用同一通道
- 无iOS-specific Flutter Bridge或Platform Adapter存在

## 8. Existing Platform Bridge

- **MethodChannel `com.amitia.amitia_app/runtime`**: A/B线已冻结的IPC桥
- **用途**: 后端连接发现 (getBackendConnection)
- **复用策略**: iOS Platform Adapter与Android共享此通道，通过不同RuntimeType区分路由

## 9. Existing Sandbox能力

无。当前iOS项目无沙盒运行时实现，未来B123-B126实现。

## 10. Existing Native Provider

无。当前iOS项目无原生能力Provider实现，未来B127-B138实现。

## 11. iOS Adapter Gap

| Gap | Status | 处理 |
|-----|--------|------|
| iOS RuntimeAdapter接口实现 | MISSING → 已实现 | adapter_ios_native.go |
| RuntimeType IOS_Native常量 | MISSING → 已实现 | runtime_adapter.go追加 |
| WireIOSPlatformAdapter注册方法 | MISSING → 已实现 | runtime_caller_wiring.go追加 |
| iOS Bridge Request/Response DTO | MISSING → 已实现 | IOSBridgeRequest/Response定义 |
| iOS Provider接口定义 | MISSING → 已实现 | IOSProvider接口 |
| iOS Sandbox合同边界 | ALREADY_SUPPORTED | RuntimeHost/Orchestrator复用 |
| iOS系统权限映射 | ALREADY_SUPPORTED | PermissionBroker不变 |
| iOS业务Provider实现 | OUTSIDE_B20_SCOPE | Defer到B123-B138 |

## 12. RuntimeBinding

- **RuntimeType**: `RuntimeTypeIOS_Native = "ios_native"`
- **命名依据**: B17 runtime_binding_declaration_mapping明确`ios_native`为futureAdapter
- **Binding结构**: RuntimeType + RuntimeID + HandlerName + Metadata(无secret)

## 13. RuntimeAdapterRegistry接入

- **注册方式**: 通过`Container.WireIOSPlatformAdapter(provider IOSProvider)`方法
- **注册调用**: `RuntimeAdapterRegistry.Register(RuntimeTypeIOS_Native, adapter)`
- **唯一性**: 单Adapter单RuntimeType，无重复注册
- **路径**: backend/internal/extension/kernel/runtime_caller_wiring.go

## 14. iOS Platform Adapter

- **名称**: `iosRuntimeAdapter`
- **路径**: backend/internal/extension/kernel/capability/adapter_ios_native.go
- **实现接口**: `RuntimeAdapter` (Supports/Execute/Health)
- **职责限制**:
  - 验证运行时调用
  - 映射规范请求
  - 委托给iOS Provider
  - 传播context/deadline/cancellation
  - 标准化结果/错误
  - 报告可用性

## 15. iOS Sandbox Provider合同

- **未来步骤**: B123-B126
- **约束**: 必须复用RuntimeAdapter接口、RuntimeHost生命周期、RuntimeOrchestrator
- **禁止**: 不得新建IOSRuntimeManager2

## 16. iOS Native Provider合同

- **未来步骤**: B127-B138
- **Provider接口**: `IOSProvider { Execute(ctx, IOSBridgeRequest) IOSBridgeResponse; Health(ctx) HealthStatus }`
- **约束**: 实现平台行为，不得注册Tool/控制PermissionBroker/创建Execution Pipeline/保存Agent状态

## 17. Sandbox / Native执行域边界

| 域 | Provider类型 | RuntimeType | 未来步骤 |
|----|-------------|-------------|----------|
| IOS_SANDBOX | Sandbox Provider | 待B123-B126定义 | B123-B126 |
| IOS_NATIVE | Native Provider | ios_native | B127-B138 |

## 18. Bridge Request/Response

- **Request**: `IOSBridgeRequest { protocolVersion, requestId, operation, payload }`
- **Response**: `IOSBridgeResponse { protocolVersion, requestId, status, result, error }`
- **Transport**: 共享MethodChannel (不建新通道)

## 19. Context传播

- **InvocationID** → Bridge RequestID
- **Deadline** → ctx传播到Provider，返回timeout状态
- **Cancellation** → ctx.Done()检测，返回cancelled状态

## 20. Cancellation

- 通过Go context.Context传播
- Adapter使用goroutine + select模式等待ctx.Done()
- 返回`ToolResultStatusCancelled` + `ErrorCodeCancelled`

## 21. Deadline

- Kernel deadline通过ctx传播
- Adapter不得重置deadline
- 超时返回`ToolResultStatusTimedOut` + `ErrorCodeTimeout`(retryable=true)

## 22. Error Mapping

- **BRIDGE_DISCONNECTED** → `connection_lost` (retryable)
- **BRIDGE_TIMEOUT** → `timeout` (retryable)
- **BRIDGE_INVALID_RESPONSE** → `internal_error`
- **PROVIDER_UNAVAILABLE** → `not_available`
- **AUTHORIZATION_DENIED** → `permission_denied`
- **USER_ACTION_REQUIRED** → `permission_denied`
- **PLATFORM_NOT_SUPPORTED** → `not_available`

## 23. Kernel Permission边界

- **Authority**: PermissionDefinitionRegistry + PermissionBroker
- **iOS Adapter不持有Kernel Grant**
- Kernel Permission (如`ios.healthkit.read`) 独立于iOS系统Authorization状态

## 24. iOS Native Authorization边界

- **Authority**: iOS系统框架 (HealthKit/EventKit等) + 各Provider内部管理
- **Adapter不持有iOS Authorization状态**
- Provider检查iOS授权状态，返回AUTHORIZATION_DENIED或USER_ACTION_REQUIRED

## 25. ResourceURI边界

- **公共Resource ID**: `amitia://` scheme唯一标准
- **Android/iOS共享**: ResourceURI逻辑不变
- **Provider内部文件路径**: 不暴露为ResourceURI

## 26. Security-scoped Resource边界

- **file://**: Provider内部引用
- **Security-scoped URL**: Provider内部引用
- **Bookmark**: Provider persistence concern
- **B20不实现**: 仅冻结边界

## 27. State边界

| State Domain | Authority | 不双写 |
|-------------|-----------|--------|
| Tool Execution | ExecutionPipeline | ✓ |
| Adapter Availability | iosRuntimeAdapter.Health() | ✓ |
| Sandbox Runtime | RuntimeHost (复用) | ✓ |
| Provider Health | Provider.Health() | ✓ |
| Native Authorization | iOS系统/Provider | ✓ |
| Background Task | TaskRuntime/Schedule (复用) | ✓ |

## 28. Provider Health

- 支持: READY/DEGRADED/UNAVAILABLE
- 不能替代Kernel ToolState
- Health状态通过统一HealthStatus返回

## 29. Background Task边界

- **复用现有**: TaskRuntime + Schedule
- **iOS BGTaskScheduler**: 仅作为platform scheduling backend
- **不替代**: 不替代Amitia Schedule引擎

## 30. Legacy Adapter限制

- **新Provider必须**: 绑定iosRuntimeAdapter
- **禁止**: 走LegacyRuntimeAdapter

## 31. Kernel Bypass验证

- **Agent → iOS Provider**: 0 (必须走ToolFacade/ToolRegistry/ExecutionPipeline)
- **Tool → iOS Provider**: 0 (同上)
- **Permission Bypass**: 0
- **ExecutionPipeline Bypass**: 0

## 32. Duplicate System Validation

全部0：IOSToolRegistry/CapabilityRegistry/PermissionBroker/PermissionStore/ExecutionPipeline/RuntimeAdapterRegistry/RuntimeHost/RuntimeOrchestrator/RuntimeManager/StateStore/ErrorRegistry/TaskRuntime/Scheduler

## 33. 实际代码修改

### backend/internal/extension/kernel/capability/runtime_adapter.go
- Symbol: RuntimeTypeIOS_Native
- 修改: 追加常量 `RuntimeTypeIOS_Native RuntimeType = "ios_native"`
- 原因: 标识iOS执行域
- 仅Adapter/Bridge: 是
- Backward Compatible: 是

### backend/internal/extension/kernel/capability/adapter_ios_native.go [NEW]
- Symbol: iosRuntimeAdapter, IOSProvider, IOSBridgeRequest, IOSBridgeResponse, IOSError
- 新建文件，实现RuntimeAdapter接口的iOS平台支持
- 原因: B12/B18要求iOS Adapter接入
- 仅Adapter/Bridge: 是
- Backward Compatible: 是

### backend/internal/extension/kernel/runtime_caller_wiring.go
- Symbol: WireIOSPlatformAdapter
- 修改: 新增注册方法
- 原因: 提供iOS Provider到Canonical RuntimeAdapterRegistry的注册入口
- 仅Adapter/Bridge: 是
- Backward Compatible: 是

## 34. Backward Compatibility

- ✅ 现有Flutter iOS shell正常运行
- ✅ AppDelegate未修改
- ✅ Existing plugins不受影响
- ✅ Existing MethodChannel bridge继续可用
- ✅ go.mod/go.sum/database零修改

## 35. B22 Conformance输入

已生成B22_ios_adapter_input.json，提供：
- Binding/Request/Result/Availability合同定义
- Error映射验证点
- Cancellation/Deadline验证点

## 36. B123～B126 Sandbox输入

已生成B123_B126_ios_sandbox_input.json，明确：
- 必须复用RuntimeAdapter/RuntimeHost/RuntimeOrchestrator
- 不得新建IOSRuntimeManager2

## 37. B127～B138 Native Provider输入

已生成B127_B138_ios_native_provider_input.json，明确：
- iOS Adapter入口 (iosRuntimeAdapter + RuntimeTypeIOS_Native)
- Provider OS Authorization边界
- Permission/Tool注册规则

## 38. B139 Cutover输入

已生成B139_ios_cutover_input.json，记录Canonical信息供未来Cutover使用。

## 39. 测试

- **capability**: PASS (go test通过)
- **runtimehost**: PASS (cached)
- **runtimeorchestrator**: PASS (cached)
- **platform**: PASS (go build通过)
- **kernelRegression**: PASS

## 40. Source Boundary

- **Modified files**: 3 (runtime_adapter.go, adapter_ios_native.go, runtime_caller_wiring.go)
- **Unexpected files**: 0
- **go.mod**: 未修改
- **go.sum**: 未修改
- **pubspec**: 未修改
- **Podfile**: 未修改
- **DB**: 未修改

## 41. 阻断项

无。所有前置条件满足，实现完成。

## 42. 最终结论

1. **iOS Platform Adapter已正式实现**: iosRuntimeAdapter作为唯一正式iOS Platform Adapter
2. **接入现有RuntimeAdapterRegistry**: 通过WireIOSPlatformAdapter注册，未建立iOS Runtime Registry
3. **iOS Sandbox和iOS Native执行域已严格分离**: IOS_SANDBOX/IOS_NATIVE分类明确
4. **iOS Adapter只做桥接**: 未实现B123-B138具体Provider
5. **Tool/Permission/Execution/Runtime Authority仍属Canonical系统**: 全部复用现有
6. **Kernel Permission与iOS Native Authorization严格分离**: 各自独立Authority
7. **file://、security-scoped URL、bookmark保持Provider内部语义**: 统一资源仍使用ResourceURI
8. **iOS Background Task明确复用现有TaskRuntime/Schedule**: 不建立第二调度系统
9. **State/Error按Domain事实源分层**: 无Global Store
10. **未创建任何重复系统**: 全部12项duplicate system count=0
11. **无新iOS Provider绑定LegacyRuntimeAdapter**: 新Provider走iosRuntimeAdapter
12. **B123-B138已获得正式iOS Provider接入底座**: IOSProvider接口已定义
13. **B22已获得iOS Adapter Conformance输入**: B22_ios_adapter_input.json已生成
