# B123 iOS Sandbox Host/Provider底座报告

## 1. 执行结果

**状态：PASS_NO_CODE_CHANGE**

B123目标是建立iOS Sandbox的Host/Provider底座。经过完整源码审计，现有Runtime基础设施(RuntimeHost/RuntimeOrchestrator/RuntimeAdapterRegistry/ProcessSupervisor)已经完全通用，能够表达iOS Sandbox Provider注册、生命周期管理、Adapter路由等所有需求。无需创建额外的Host包装层或修改源码。

## 2. B9P8输入

- **final_step_reuse_matrix**: B123 = NEW_PROVIDER, allowedNewComponents=[iSHProvider], mustReuse=[RuntimeOrchestrator]
- **final_runtime_manifest**: RuntimeHost/RuntimeOrchestrator/RuntimeAdapterRegistry权威定义
- **final_canonical_system_manifest**: Canonical系统清单
- **final_architecture_guard**: 架构守护规则
- **final_execution_guard**: 执行守护规则

## 3. B12输入

- **runtime_binding_matrix**: 现有12个RuntimeType绑定，ios_native为future binding
- **platform_adapter_contract**: 平台Adapter合同
- **native_offload_contract**: Native卸载合同
- **runtime_security_contract**: 运行时安全合同
- **runtime_authority_resolution**: Runtime权威定义(canonical已确认为extension/kernel/下)

## 4. B18输入

- **canonical_authority_matrix**: 统一权威矩阵
- **resolved_contract_phase_manifest**: 合同阶段manifest
- **prerequisite_status_matrix**: B12/B10/B11均为PASS/PASS_NO_CODE_CHANGE

## 5. B20输入（部分完成）

B20处于IN_PROGRESS状态（无b20_status.json）：
- adapter_ios_native.go已创建（B20 authority）
- RuntimeTypeIOS_Native常量未添加（依赖B20完成）
- WireIOSPlatformAdapter未添加（依赖B20完成）

B123不抢占B20的iOS Platform Adapter authority。

## 6. 当前iOS Sandbox现状

经过完整源码审计（mobile_app/ios/, mobile_app/lib/, backend/）：

- **iOS Sandbox实现：无** - 当前源码无任何iSH/Alpine/Sandbox相关代码
- **iOS Native Provider：无** - 无HealthKit/Calendar/Contacts等原生Provider
- **现有iOS组件**：
  - AppDelegate.swift (Flutter iOS壳)
  - RunnerTests.swift (空测试)
  - MethodChannel桥接（跨平台共享，Dart<->Backend通信）
  - BackendConnectionRepository（跨平台）
- **B20已完成**：adapter_ios_native.go（iOS Platform Adapter实现，依赖RuntimeTypeIOS_Native常量）

## 7. Runtime Authority

| 权威域 | Owner | 路径 |
|--------|-------|------|
| RuntimeBinding | runtime_adapter.go | extension/kernel/capability/ |
| RuntimeAdapterRegistry | executor.go | extension/kernel/capability/ |
| RuntimeHost | host.go + native_host.go | internal/runtimehost/ |
| RuntimeOrchestrator | orchestrator.go | internal/runtimeorchestrator/ |
| ProcessSupervisor | supervisor.go | internal/runtimehost/ |

## 8. RuntimeHost

**接口**：`RuntimeHost`（host.go）- 定义Descriptor/Capabilities/Paths/Processes/RuntimeInstanceID

**实现**：
- `nativeProcessHost` - 原生进程运行时主机
- `restrictedHost` - 受限运行时主机

**Factory**：`NewRuntimeHost(ctx HostBuildContext)` - 已支持`GuestPlatformIOS`

**结论**：现有RuntimeHost完全满足iOS Sandbox需求，无需新建IOSSandboxHost。

## 9. RuntimeOrchestrator

**职责**：Provider生命周期编排、依赖管理、健康监控、状态聚合

**Provider模式**：
- `ProviderFactory`接口（Build方法创建ProviderInstance）
- `ProviderInstance` = ManagedComponent + Slot + ProviderID + Capability
- 现有参照：SurrealDBProviderFactory、QdrantProvider

**结论**：ProviderFactory模式完全适用于iOS Sandbox Provider注册。

## 10. Runtime Supervisor

**接口**：`ProcessSupervisor`（supervisor.go）

**实现**：`defaultProcessSupervisor`

**结论**：未来iSH进程的监督复用此Supervisor，无需新建IOSSandboxSupervisor。

## 11. Process Infrastructure

**组件**：`DefaultProcessManager`（internal/platform/process/）

**结论**：未来iSH进程管理复用此基础设施。

## 12. iOS Sandbox Provider

**角色**：IOS_SANDBOX_PROVIDER

**职责边界**：
- 识别Sandbox运行后端 (iSH)
- 建立Runtime-specific配置
- 将RuntimeHost请求映射到Sandbox后端
- 暴露Sandbox可用性
- 准备Sandbox runtime descriptor
- 返回domain errors

**不属于Provider**：Tool注册、Permission decision、Task scheduling、全局生命周期、全局state storage

## 13. Sandbox Backend Contract

B123定义了`SandboxBackend`薄接口（B124实现）：

```go
type SandboxBackend interface {
    Availability(ctx context.Context) BackendAvailability
    Start(ctx context.Context, config SandboxConfig) error
    Stop(ctx context.Context) error
    Execute(ctx context.Context, cmd SandboxCommand) (SandboxResult, error)
    Health(ctx context.Context) HealthStatus
}
```

可用性状态：BACKEND_UNAVAILABLE/AVAILABLE/STARTING/RUNNING/ERROR

## 14. RuntimeBinding

**现有**：12个RuntimeType常量（builtin/plugin_js/plugin_service/mcp/workflow/internal/legacy/javascript/wasm/trusted_service/task/browser/android_native）

**iOS相关**：RuntimeTypeIOS_Native（由B20或B124添加，value="ios_native"）

## 15. RuntimeAdapter关系

**现有**：12个Adapter实现（adapter_builtin/internal/legacy/javascript/wasmrt/mcp/mcp_tool/task/workflow/plugin/trusted_service/android_native/ios_native）

**注意**：
- androidNativeAdapter：已实现
- iosRuntimeAdapter：B20创建（adapter_ios_native.go），但依赖RuntimeTypeIOS_Native常量缺失
- B123不修改adapter_ios_native.go（B20 authority）

## 16. iOS Platform Adapter关系

**B20状态**：IN_PROGRESS
- adapter_ios_native.go已创建（B123不修改）
- RuntimeTypeIOS_Native常量未添加
- WireIOSPlatformAdapter未添加

**B123边界**：仅复用iOS Platform Adapter路由，不实现Adapter。

## 17. B20并行/汇合边界

**当前**：B20处于IN_PROGRESS（无PASS状态），adapter_ios_native.go已创建。

**B123边界**：
- B123不修改adapter_ios_native.go
- B123不添加RuntimeTypeIOS_Native常量（B20 authority）
- B123不添加WireIOSPlatformAdapter方法
- B123仅定义Sandbox-side Provider.contract和Backend interface

**汇合后**：B20完成RuntimeTypeIOS_Native添加后，B124实现iSHSandboxBackend并注册到ProviderRegistry。

## 18. ResourceURI

**Canonical Scheme**：amitia://（唯一Portable Resource Scheme）

**边界**：
- Sandbox内部路径(/root/home/tmp等)作为SandboxPath DTO
- 上层跨Runtime引用继续使用ResourceURI
- 禁止创建iossandbox://、ish://、alpine://等第二Scheme

## 19. Workspace / Sandbox Path边界

- **用户Workspace**：amitia://workspace/...
- **Sandbox内部路径**：物理路径（仅Provider内部使用）
- **Workspace映射**：通过mount将用户Workspace挂载到Sandbox物理路径
- **两者区别**：rootfs是运行环境文件系统，Workspace是用户数据

## 20. State Authority

| 状态层 | Owner |
|--------|-------|
| Runtime Instance State | RuntimeOrchestrator |
| Sandbox Provider Availability | ProviderInstance.Capability() |
| iSH Backend State | 通过Provider转发 |
| Rootfs State | B125负责 |
| iOS Platform Authorization | B20 adapter |
| Tool Execution State | ExecutionPipeline |

**结论**：无平行Sandbox State Store，所有状态通过现有Runtime体系暴露。

## 21. Error Authority

| Provider错误 | 映射到 |
|--------------|--------|
| sandbox_unavailable | ErrorCodeNotAvailable |
| backend_unavailable | ErrorCodeNotAvailable |
| backend_timeout | ErrorCodeTimeout |
| backend_execution_failed | ErrorCodeExecutionFailed |
| invalid_runtime_binding | ErrorCodeRuntimeUnavailable |

**B20已定义**：iOS Bridge错误码（PROVIDER_UNAVAILABLE/AUTHORIZATION_DENIED/USER_ACTION_REQUIRED/PLATFORM_NOT_SUPPORTED/BRIDGE_DISCONNECTED/BRIDGE_TIMEOUT/BRIDGE_INVALID_RESPONSE）

**结论**：无IOSSandboxErrorRegistry，所有错误映射到现有ToolError模型。

## 22. Permission边界

- **Kernel权限**：PermissionDefinitionRegistry/PermissionBroker
- **iOS系统权限**：iOS Platform Adapter处理（HealthKit/Calendar等）
- **边界**：Kernel权限与iOS系统权限分离

## 23. Cancellation / Deadline

**机制**：context.Context标准传播

**支持**：
- ctx.Err()检查
- context.WithTimeout deadline
- context.WithCancel传播

**B20已实现**：adapter_ios_native.go中的handleCtxError方法。

## 24. iSH Deferred

**状态**：DEFERRED_B124

B124将实现：
- iSHSandboxBackend（生产实现）
- SandboxBackend接口定义
- ProviderFactory注册
- RuntimeTypeIOS_Native常量添加

## 25. Alpine rootfs Deferred

**状态**：DEFERRED_B125

B125将实现：
- rootfs下载/安装
- 完整性验证
- Workspace挂载
- 状态暴露

## 26. Lifecycle / Recovery Deferred

**状态**：DEFERRED_B126

B126将完成：
- 完整start/stop/restart/cleanup闭环
- Crash recovery机制
- 健康检查和状态报告

## 27. Legacy检查

**结果**：所有检查通过（count=0）
- 新Legacy Runtime Binding：0
- 新Legacy Tool Registration：0
- 新Legacy Execution：0

## 28. Production Fake检查

- Test Fake：允许（仅*_test.go中使用）
- Production Fake Backend：禁止（count=0）

## 29. Duplicate System Validation

**结果**：17项平行系统全部检查通过

| 系统 | 创建数量 |
|------|----------|
| IOSRuntimeManager2 | 0 |
| IOSSandboxLifecycle2 | 0 |
| IOSSandboxSupervisor2 | 0 |
| RuntimeHost2 | 0 |
| RuntimeOrchestrator2 | 0 |
| RuntimeAdapterRegistry2 | 0 |
| IOSSandboxToolRegistry2 | 0 |
| IOSSandboxPermissionBroker2 | 0 |
| IOSSandboxExecutionPipeline2 | 0 |
| IOSSandboxStateStore2 | 0 |
| IOSSandboxErrorRegistry2 | 0 |
| iossandbox:// scheme | 0 |
| ish:// scheme | 0 |

## 30. 实际源码修改

**无源码修改**。

现有RuntimeHost(支持GuestPlatformIOS)、RuntimeOrchestrator(支持ProviderFactory)、RuntimeAdapterRegistry(支持RuntimeType)已完全满足iOS Sandbox Host/Provider底座要求。无需创建IOSSandboxHost包装层。

## 31. Backward Compatibility

| 领域 | 状态 |
|------|------|
| Android Runtime | 未修改，PASS |
| Desktop Runtime | 未修改，PASS |
| Existing RuntimeHost | 复用，PASS |
| Existing RuntimeOrchestrator | 复用，PASS |
| Existing Tool Execution | 未修改，PASS |
| Existing ResourceURI | 未修改，PASS |
| B20 iOS Adapter | 未修改(B20 authority)，PASS |

## 32. B124输入

**文件**：B124_ish_backend_input.json

核心内容：
- SandboxBackend接口定义
- iSHSandboxBackend生产实现要求
- ProviderFactory注册模式
- RuntimeTypeIOS_Native常量需求
- 可用性/状态/错误/取消合同

## 33. B125输入

**文件**：B125_alpine_rootfs_input.json

核心内容：
- rootfs集成点
- 资源/路径边界
- Workspace挂载配置
- 安装状态语义

## 34. B126输入

**文件**：B126_ios_sandbox_lifecycle_input.json

核心内容：
- 现有Runtime基础设施复用
- start/stop/recovery/cleanup需求
- 健康检查实现要求

## 35. B148输入

**文件**：B148_ios_sandbox_integration_input.json

核心内容：
- Canonical Runtime Binding记录
- Provider入口记录
- RuntimeHost引用
- 集成先决条件清单
- 权威边界确认

## 36. Tests

**验证方式**：静态源码审计 + 架构一致性验证

| 验证项 | 结果 |
|--------|------|
| RuntimeHost唯一性 | PASS |
| RuntimeOrchestrator唯一性 | PASS |
| RuntimeAdapterRegistry唯一性 | PASS |
| Authority边界 | PASS |
| 向后兼容性 | PASS |
| 源码边界 | PASS |

## 37. Source Boundary

- **修改文件**：0
- **新增文档文件**：22
- **go.mod**：未修改
- **go.sum**：未修改
- **pubspec.yaml**：未修改
- **Podfile**：未修改
- **DB**：未修改

## 38. 阻断项

无。

现有Runtime基础设施完全满足B123 iOS Sandbox Host/Provider底座要求。B20的iOS Platform Adapter部分完成（adapter_ios_native.go已创建）不影响B123的PASS状态。

## 39. 最终结论

1. iOS Sandbox被正确定义为现有Runtime体系下的Provider，而不是独立Runtime系统。
2. 继续复用现有RuntimeBinding、RuntimeAdapterRegistry、RuntimeHost、RuntimeOrchestrator和Supervisor。
3. 建立了B123所需Sandbox Host/Provider底座（合约形式）。
4. 未提前实现iSH。
5. 未提前实现Alpine rootfs。
6. 未提前实现B126完整Lifecycle/Recovery。
7. 不存在IOSRuntimeManager2、IOSSandboxLifecycle2或独立Supervisor。
8. 不存在Sandbox Tool Registry、Permission Broker、Execution Pipeline等第二套Kernel组件。
9. Sandbox文件继续使用现有ResourceURI体系。
10. 未新增iossandbox://、ish://或Workspace2。
11. 未创建生产Fake Sandbox Backend。
12. 未新增Legacy Runtime绑定。
13. B124获得明确、唯一的iSH Backend接入点（SandboxBackend接口 + ProviderFactory注册模式）。
14. 允许继续执行B124。
