# B19 Android Platform Adapter正式实现报告

## 1. 执行结果

**状态：PASS**

B19目标已达成：实现 Android Platform Adapter唯一正式实现，接入现有 RuntimeAdapterRegistry，复用A线 Android Runtime/Bridge基础设施，严格遵循B12冻结合同。

## 2. B9P8输入

- **状态**: PASS
- **来源**: docs/parity/post-b9/b9p8/b9p8_status.json
- **manifest**: AMITIA-POST-B9-RESOLVED-V1
- **release gate**: POST-B9-B10-RELEASE-GATE
- **frozen core**: final_step_reuse_matrix.json已冻结B19施工定义

## 3. B12输入

- **状态**: PASS_NO_CODE_CHANGE
- **来源**: docs/parity/post-b9/b12/b12_status.json
- **合同**: platform_adapter_contract.json定义androidAdapter合同
- **RuntimeType**: RuntimeTypeBindingMatrix中android_native为future binding
- **admission rules**: runtime_adapter_admission_rules.json 12条规则全部满足
- **结论**: B12合同完整，无需修订；B19实现符合合同要求

## 4. B18输入

- **状态**: PASS
- **来源**: docs/parity/post-b9/b18/b18_status.json
- **输入清单**: B18/B19_input_manifest.json
- **platformAdapterPhaseAllowed**: true
- **androidAdapterAllowed**: true
- **Canonical Authority Matrix**: 所有Authority一致

## 5. Construction Mode

**ADAPTER_ONLY**

依据：B9P8 final_step_reuse_matrix.json中B19 primaryMode=ADAPTER_ONLY

允许新组件：AndroidCapabilityAdapter(新)

## 6. 当前Android架构

当前项目Android端(A线)已存在：
- AndroidRuntimeModule：Runtime封装根
- RuntimeController：生命周期管理(install/verify/start/stop/repair)
- ProotComponent/Session：嵌入式Linux Runtime
- MethodChannel 'com.amitia.amitia_app/runtime'：Flutter/Native IPC桥
- RuntimeStateMachine：15种状态，36种转换规则

## 7. Android Runtime Service

- **组件**: AndroidRuntimeModule / DefaultRuntimeModule / RuntimeController
- **路径**: mobile_app/android/amitia-runtime/src/main/kotlin/com/amitia/amitia_app/runtime/
- **分类**: CANONICAL_ANDROID_RUNTIME_INFRASTRUCTURE
- **B19处理**: 复用不修改

## 8. Embedded Linux Runtime

- **组件**: ProotComponent / ProotSession / ProotProcessLauncher
- **路径**: mobile_app/android/amitia-runtime/.../proot/
- **分类**: ANDROID_EMBEDDED_LINUX_RUNTIME
- **B19处理**: 复用不修改。B55-B61 Linux能力通过RuntimeHost而非Android Adapter

## 9. Flutter/Native Bridge

- **组件**: MethodChannel 'com.amitia.amitia_app/runtime' + RuntimeBridge + RuntimeBackendConnectionSource
- **现有消息**: runtime.getBackendConnection
- **分类**: EXISTING_PLATFORM_BRIDGE
- **B19处理**: 复用。AndroidBridgeRequest/Response DTO扩展协议定义，不改现有通道

## 10. RuntimeState

**不允许修改。**

现有15种状态完整：UNKNOWN, NOT_INSTALLED, INSTALLING, INSTALLED, VERIFYING, STARTING, READY, DEGRADED, STOPPING, STOPPED, REPAIRING, CORRUPTED, FAILED

B19禁止新增 ADAPTER_READY / PROVIDER_READY 等状态到此状态机。

## 11. Android Adapter Gap

audit发现14个gap项：
- ALREADY_SUPPORTED: 3项(Permission Broker, Embedded Linux Runtime, RuntimeState未改)
- PARTIALLY_SUPPORTED: 1项(Bridge传输层)
- MISSING: 10项(全部由B19实现补齐)

详见 android_adapter_gap_matrix.json

## 12. RuntimeBinding

- **android_native RuntimeType**: 新增 RuntimeTypeAndroid_Native = "android_native"
- **绑定路径**: RuntimeBinding{RuntimeType: RuntimeTypeAndroid_Native, HandlerName: provider_operation}
- **区分**: android_native(Android Native) vs builtin/internal(Embedded Linux) — 两种执行域严格分离

## 13. RuntimeAdapterRegistry接入

- **注册**: RuntimeAdapterRegistry.Register(RuntimeTypeAndroid_Native, adapter)
- **wiring方法**: Container.WireAndroidPlatformAdapter(provider)
- **唯一性**: 仅单次注册，重复注册覆盖
- **路径**: backend/internal/extension/kernel/runtime_caller_wiring.go

## 14. Android Platform Adapter

**实现位置**: backend/internal/extension/kernel/capability/adapter_android_native.go

**实现内容**:
- androidRuntimeAdapter 结构体实现 RuntimeAdapter 接口
- Supports(binding): 识别 RuntimeTypeAndroid_Native
- Execute(ctx, binding, invocation, input): Provider-neutral分派 + ctx cancellation/deadline传播
- Health(ctx, binding): 代理Provider健康状态
- 错误映射：Android Domain Error → ToolError(保留domainCode)

## 15. Native Provider合同

**AndroidProvider接口**:
```go
type AndroidProvider interface {
    Execute(ctx context.Context, request AndroidBridgeRequest) AndroidBridgeResponse
    Health(ctx context.Context) HealthStatus
}
```

Provider责任边界：
- 仅实现Android domain行为
- 不注册全局Tool
- 不拥有Permission authority
- 不拥有Execution pipeline
- 不拥有RuntimeHost
- 不拥有全局state

## 16. Embedded Linux / Native边界

**严格分开**:
- Embedded Linux能力(B55-B61 shell/process/filesystem)：RuntimeHost + ProotComponent执行
- Android Native能力(B62-B77 accessibility/clipboard/notification等)：Android Adapter → Bridge → Provider执行

B19不重走Linux能力为Native Adapter；未来B55-B61继续复用RuntimeHost。

## 17. Request/Response

**AndroidBridgeRequest**:
- protocolVersion: 1
- requestId (对应ToolInvocationContext.InvocationID)
- operation (对应binding.HandlerName)
- payload (对应input)

**AndroidBridgeResponse**:
- protocolVersion
- requestId
- status (success/failed/cancelled/timeout)
- result (Android domain DTO)
- error (code, message, domainCode)

transport-only，不含业务逻辑。

## 18. Context传播

Adapter.Execute完整传播：
- ctx cancellation: select ctx.Done()分支
- ctx deadline: context.DeadlineExceeded检测
- invocation.InvocationID: → AndroidBridgeRequest.requestId
- binding.HandlerName: → AndroidBridgeRequest.operation

## 19. Cancellation

```go
select {
case <-ctx.Done():
    return cancelled/timed_out result
case response := <-done:
    return normalized result
}
```

ctx取消或超时立即响应，不阻塞Provider goroutine。

## 20. Timeout

ctx deadline检测：
- context.DeadlineExceeded → ToolResultStatusTimedOut + ErrorCodeTimeout + Retryable=true
- 默认超时由Kernel ExecutionPipeline控制

## 21. Error Mapping

**Bridge Error**:
- BRIDGE_DISCONNECTED → ErrorCodeConnectionLost (Retryable)
- BRIDGE_TIMEOUT → ErrorCodeTimeout (Retryable)
- BRIDGE_INVALID_RESPONSE → ErrorCodeInternalError

**Provider Error**:
- PROVIDER_UNAVAILABLE → ErrorCodeNotAvailable
- AUTHORIZATION_DENIED → ErrorCodePermissionDenied (含domainCode=android.os.authorization_denied)
- USER_ACTION_REQUIRED → ErrorCodePermissionDenied (含domainCode=android.os.user_action_required)
- PLATFORM_NOT_SUPPORTED → ErrorCodeNotAvailable
- OPERATION_FAILED → ErrorCodeExecutionFailed

所有映射保留原始domainCode于ToolError.Details中。

## 22. Permission边界

两层严格分离：
1. **Kernel Permission**: PermissionBroker判定Extension是否有权调用Android Provider → service.android.*权限
2. **OS Authorization**: Android Provider内部判定OS级权限(android.permission.*) → 返回AUTHORIZED/DENIED/USER_ACTION_REQUIRED

Adapter不存储任何Permission状态，不调用Broker，不请求OS权限。

## 23. Android OS Authorization边界

Adapter不调用requestPermissions()/ActivityResultLauncher。

Android Provider返回OS授权状态经Bridge Response传播 → Adapter映射为ToolError → UnifiedToolResult。

## 24. ResourceURI边界

统一资源引用：
- Kernel公共合同：amitia:// (B13)
- Android Provider内部：content:// / file://(不跨边界)
- Embedded Linux内部：proot物理路径(不跨边界)

Provider返回前必须将内部资源封装为amitia://。

## 25. Android content://边界

content://仅限Android Provider内部使用(SAF/MediaStore)。不成为Kernel公共合同。

## 26. State边界

五种状态严格分离：
1. Android Embedded Runtime State (RuntimeStateMachine — 不修改)
2. Adapter Availability (动态派生，不持久化)
3. Provider Health (Provider Domain状态)
4. OS Authorization (Provider内部)
5. Tool Execution State (ExecutionPipeline)

## 27. Provider Health

Android Adapter可代理Health()调用：
- 无Provider → HealthUnhealthy
- Provider注册 → HealthReady
- Provider报告unhealthy → 透传

## 28. Legacy Adapter限制

Android Adapter绑定RuntimeTypeAndroid_Native，不绑定RuntimeTypeLegacy。ADM-002规则严格执行。

## 29. Kernel Bypass验证

- Agent direct Provider调用: 0
- Tool direct Provider调用: 0
- Permission bypass: 0
- ExecutionPipeline bypass: 0

所有调用必须经过：Tool → ToolRegistry → ExecutionPipeline → PermissionBroker → RuntimeAdapter → AndroidProvider

## 30. Duplicate System Validation

所有平行系统计数 = 0：
AndroidToolRegistry2=0, AndroidPermissionBroker2=0, AndroidExecutionPipeline2=0, RuntimeAdapterRegistry2=0, AndroidRuntimeHost2=0, AndroidRuntimeOrchestrator2=0, AndroidRuntimeManager2=0, AndroidStateStore2=0, AndroidErrorRegistry2=0

## 31. 实际代码修改

### backend/internal/extension/kernel/capability/runtime_adapter.go
- Symbol: RuntimeTypeAndroid_Native
- 修改: 追加常量RuntimeType = "android_native"
- 原因: B12合同要求android_native RuntimeType
- 仅Adapter/Bridge: 是
- Backward Compatible: 是

### backend/internal/extension/kernel/capability/adapter_android_native.go (新文件)
- Symbol: androidRuntimeAdapter / AndroidProvider / NewAndroidRuntimeAdapter 等
- 修改: 新建文件实现完整Android Adapter
- 原因: B19核心目标 - Platform Adapter实现
- 仅Adapter/Bridge: 是
- Backward Compatible: 是

### backend/internal/extension/kernel/runtime_caller_wiring.go
- Symbol: WireAndroidPlatformAdapter
- 修改: 追加12行wiring方法
- 原因: 注册入口
- 仅Adapter/Bridge: 是
- Backward Compatible: 是

## 32. Backward Compatibility

- A线Android Runtime Service: 未修改
- Embedded Linux Runtime: 未修改
- Flutter/Native Bridge: 未修改
- RuntimeState: 未修改
- Start/Stop流程: 未修改
- productionFakeProvider: 0

## 33. B22 Conformance输入

已生成 B22_android_adapter_input.json，确认Adapter满足：
- binding resolution正确
- context cancellation/deadline传播
- error normalization(保留domainCode)
- provider availability正确返回
- 唯一注册

## 34. B55～B78 Provider输入

已生成 B55_B78_android_provider_input.json，分类：
- **Linux Provider**(B55-B61): 走RuntimeHost，不经过Android Adapter
- **Native Provider**(B62-B77): 走Android Adapter，复用MethodChannel

## 35. B139 Cutover输入

已生成 B139_android_cutover_input.json，仅记录不执行Cutover。

## 36. 测试

- go test ./internal/extension/kernel/capability/: PASS
- 全部现有Kernel测试无回归
- race detector: 无竞态(cancellation select保护)

## 37. Source Boundary

- Modified files: 3 (全部在Allowed列表)
- Unexpected files: 0
- go.mod: 未修改
- go.sum: 未修改
- DB: 未修改

## 38. 阻断项

无。所有Gate条件满足。

## 39. 最终结论

1. Android Platform Adapter已正式实现 ✓
2. 接入现有RuntimeAdapterRegistry而非创建新Registry ✓
3. 复用A线Android Runtime Service、Embedded Linux Runtime和Bridge ✓
4. Android Linux与Android Native执行域明确分离 ✓
5. Android Adapter仅负责桥接而没有实现具体Provider业务 ✓
6. Tool/Permission/Execution Authority继续属于现有Extension Kernel ✓
7. Android OS Authorization与Kernel Permission保持分离 ✓
8. ResourceURI继续使用amitia://统一合同 ✓
9. Runtime State、Provider Health、OS Authorization、Tool Execution State保持不同事实源 ✓
10. 没有建立Android Tool Registry、Permission Center、Execution Pipeline或RuntimeManager2 ✓
11. 没有任何新Provider绑定LegacyRuntimeAdapter ✓
12. B55～B78已获得正式Android Provider接入底座 ✓
13. B22已获得Android Adapter Conformance输入 ✓
