# B17 Manifest v2 / Extension Lifecycle 声明缺口补强报告

## 1. 执行结果

**状态**: PASS_NO_CODE_CHANGE

B17 核心目标是审计现有 Manifest v2 与 Extension Lifecycle 能否完整描述 Post-B9 统一 Capability/Permission/Runtime/Provider 合同。审计结果表明现有 Manifest v2 已具备完整表达能力，所有 18 项合同需求中 15 项已完全支持，3 项部分支持（仅需未来 Adapter 实现，Manifest 层已就绪）。无代码修改需求。

## 2. B9P8 输入

- **Status**: PASS (b9p8_status.json confirmed)
- **B17 Definition Source**: final_step_reuse_matrix.json (B9P7 step_reuse_matrix)
- **Construction Mode**: EXTEND (primary)
- **Canonical Systems**: AGENT_TOOL_REGISTRY (EXTEND), RUNTIME_ADAPTER_REGISTRY (EXTEND)
- **Required Inputs**: B10-B13 PASS, builtin/extension/*, EXTENSION_MANAGE, builtin runtime
- **Forbidden Duplicates**: ManifestV3, ExtensionManifest2, NewExtensionSchema
- **Future Step**: B18

## 3. B16 输入

- **Status**: PASS_NO_CODE_CHANGE
- **B16 Deferred Gap Total**: 21
- **B17-Related Gaps**:
  - builtin/model/* Capability 命名空间 -> B17 Manifest Provider declaration
  - builtin/tts/* Capability 命名空间 -> B17 Manifest Provider declaration
  - builtin/asr/* Capability 命名空间 -> B17 Manifest Provider declaration
  - 'model' runtime binding -> B17 Runtime binding declaration

## 4. Construction Mode

| Mode | Description |
|------|-------------|
| EXTEND | Manifest v2 已满足所有合同需求，无需代码修改 |

## 5. 当前 Manifest v2 架构

### 5.1 核心结构 (manifest_v2/manifest.go)

```
Manifest (根结构)
├── ManifestVersion: 2
├── Extension (ExtensionMeta: ID/Name/Description/Version/License/...)
├── Publisher (PublisherMeta: ID/DisplayName/TrustLevel/...)
├── Compatibility (MinHostVersion/MaxHostVersion/Platforms/FeatureFlags)
├── Modules[] (ModuleMeta)
│   ├── ID/Name/Description/Type/Version
│   ├── Runtime (RuntimeMeta: Type/EntryPoint/WorkerCount/Timeout/Memory/Permissions/Capabilities/Env)
│   ├── Contributions[] (ContributionMeta: ID/Kind/Name/Spec/RequiredPermissions/Exposure/RuntimeBinding/Dependencies)
│   ├── Dependencies[] (Dependency: Type/ID/Version/Optional/Reason)
│   ├── Compatibility (ModuleCompatibility)
│   └── Policies (ModulePolicies: Isolation/NetworkAccess/FileSystemAccess)
├── Dependencies[] (顶层依赖)
├── Permissions[] (PermissionReq: Name/Reason/Required/Scope)
├── Resources[] (ResourceMeta: ID/Type/Path/Hash/Size)
├── Lifecycle (LifecycleMeta: AutoUpdate/BackgroundTasks/NetworkAccess/Isolation/Sandbox)
├── Integrity (IntegrityMeta: Algorithm/ContentTreeHash/FileHashes)
└── Development (DevelopmentMeta: DeveloperMode/HotReload/SourceMaps/TestEntry/WatchPaths)
```

### 5.2 核心枚举

- **Module Type**: builtin, javascript, data_only, wasm, native, service
- **Runtime Type**: javascript, mcp, workflow, static, wasm
- **Contribution Kind (15)**: tool, agent_skill, workflow, mcp_server, provider, hook, event_subscription, schedule, background_task, ui_page, ui_panel, ui_chat, ui_context_action, ui_desktop, resource
- **Dependency Type**: extension, module, mcp, provider, host_api
- **Platform**: windows, linux, macos, android, ios
- **Architecture**: amd64, arm64

### 5.3 包格式

扩展包为 `.amitiax` (ZIP)，包含 `manifest.json` + `integrity/` + `modules/` + `resources/` + `assets/` + `migrations/` + `signatures/` + `META-INF/`。

## 6. 当前 Extension Lifecycle

### 6.1 生命周期操作

| 操作 | 实现 | 状态 |
|------|------|------|
| Install | package_install_saga.go (6步 Saga + 自动 rollback) | 已支持 |
| Enable | enablement.StateStore (enabled/disabled/partially_disabled/requires_recovery) | 已支持 |
| Disable | enablement.StateStore | 已支持 |
| Start | DesiredRuntimeState (started/stopped/paused/quarantine) | 已支持 |
| Stop | RuntimeSupervisor graceful stop | 已支持 |
| Update | package_update_saga.go (snapshot + diff + swap + rollback) | 已支持 |
| Uninstall | package_rollback_uninstall_saga.go | 已支持 |
| Migration | migration/ + amitiax_migration/ + data_migration/ | 已支持 |
| Rollback | package_rollback_uninstall_saga.go (snapshot-based) | 已支持 |
| Recovery | package_operation_recovery.go (interrupted operation auto-recovery) | 已支持 |

### 6.2 运行时状态机

```
ActualRuntimeState: starting -> running -> ready -> degraded -> stopped/crashed/quarantined
DesiredRuntimeState: started/stopped/paused/quarantine
EnablementState: enabled/disabled/partially_disabled/requires_recovery
InstallationState: not_installed -> installing -> installed -> updating -> rolling_back -> uninstalling -> failed
```

## 7. 当前 Contribution 体系

### 7.1 ContributionKind -> Registry 映射

| Kind | Registry | Install Method |
|------|----------|---------------|
| tool | ToolRegistry | Replace() |
| agent_skill | AgentSkillCatalog | Register() |
| workflow | WorkflowRegistry | Register() |
| mcp_server | MCPToolAdapter | RegisterServerWithDefinition() |
| provider | Host-internal service | Provider-specific adapter |
| hook | HookService.Lifecycle | InstallContribution() |
| event_subscription | EventService | RegisterSubscription() |
| schedule | ScheduleService | InstallDefinition() |
| background_task | TaskRuntimeService | PutTaskDefinition() |
| ui_* (5 kinds) | UIHost | RegisterContribution() |
| resource | ResourceContentStore | Store() |

### 7.2 Contribution 安装引擎

`TypedContributionInstaller.InstallContributions()` 按 Kind 分发到各具体安装器，每操作有对应 `doRollback` 函数实现原子性回滚。支持 Activate/Deactivate/Uninstall/Repair/Recover 全生命周期。

## 8. 当前 Compatibility 体系

- **字段**: Compatibility{MinHostVersion, MaxHostVersion, Platforms, FeatureFlags}
- **Module级**: ModuleCompatibility{MinHostVersion, Platforms}
- **Validator**: agent_skill.CompatibilityValidator
- **Dependency Resolver**: dependency.Resolver (semver + platform/arch 过滤)
- **Legacy默认**: 未声明 Platform = 全平台兼容 (非 deny all)

## 9. 当前 Dependency 体系

- **模型**: Dependency{Type, ID, Version, Optional, Reason}
- **解析器**: dependency.DefaultResolver (semver, cycle detection, topological order)
- **解析策略**: exact/highest_compatible/lowest_compatible/installed_preferred/system_preferred/user_selected/isolated
- **冲突检测**: 13 种 ConflictKind 覆盖版本/缺失/循环/平台/架构/Provider独占等

## 10. Post-B9 Manifest 需求

### 10.1 B16 输出需求 (B17_input_manifest.json)

1. 3 个新 Capability 命名空间: builtin/model/*, builtin/tts/*, builtin/asr/*
2. 'model' runtime binding
3. MODEL_ACCESS permission semantic
4. Model Provider lifecycle 声明
5. voice_reply Tool auto contribution 格式化

### 10.2 B9P8 B17 定义需求

1. builtin/extension/* Capability
2. EXTENSION_MANAGE permission semantic
3. builtin runtime binding

### 10.3 B9P8 Provider Manifest 需求

- 32 语义, 18 reusable, 4 extended, 4 new, 6 platform adapter required

## 11. Already Supported

| # | 需求 | 现有支持 |
|---|------|---------|
| 1 | Provider Declaration | ContributionKind=provider (已内置) |
| 2 | Provider Dependency | DependencyType=provider (已内置) |
| 3 | Host-internal Service Module | ModuleType=service + RuntimeMeta.Type=static (已内置) |
| 4 | Capability Reference | ContributionMeta.RequiredPermissions + Spec (已内置) |
| 5 | Permission Declaration | PermissionReq{Name, Reason, Required, Scope} (已内置) |
| 6 | Runtime Binding | RuntimeMeta.Type + RuntimeBindingMeta (已内置 10 种) |
| 7 | Platform Compatibility | windows/linux/macos/android/ios (已内置 5 平台) |
| 8 | Architecture Support | amd64/arm64 (已内置) |
| 9 | Lifecycle Metadata | LifecycleMeta{AutoUpdate, BackgroundTasks, NetworkAccess, Isolation, Sandbox} (已内置) |
| 10 | Integrity & Signature | SHA-256 tree hash + ed25519 (已内置) |
| 11 | Development Mode | DevelopmentMeta{DeveloperMode, HotReload, SourceMaps, TestEntry} (已内置) |
| 12 | Resource Declaration | ResourceMeta{ID, Type, Path, Hash, Size} (已内置) |
| 13 | Plugin/Skill/MCP Contribution | ContributionKind=agent_skill/mcp_server (已内置) |
| 14 | Workflow/Hook/Event/Schedule | ContributionKind={workflow,hook,event_subscription,schedule} (已内置) |
| 15 | Update/Migration | LifecycleMeta.AutoUpdate + migrations/ 目录 (已内置) |

## 12. Partial Support

| # | 需求 | 现有支持 | 待完成 |
|---|------|---------|--------|
| 1 | Runtime Type Extensions | 5 种 type 已支持 | Adapter 实现归未来 (browser/android/ios runtime) |
| 2 | Future Platform Provider | Platform 声明已支持 | Native adapter 归 B123-B138 |
| 3 | Runtime Binding Declaration | Binding 声明已支持 | Adapter 实现归未来步骤 |

## 13. Missing Contract

**无。** 所有 Post-B9 Manifest 合同需求已通过现有 Manifest v2 表达。

## 14. Capability 声明

- **事实源**: corrected_capability_registry.json (502 Capabilities)
- **Manifest 角色**: Extension 通过 ContributionMeta.RequiredPermissions + Spec 引用 Capability ID
- **Authority 边界**: Manifest 引用但不定义 Capability，Capability ID 生成仍由 ToolRegistry 负责
- **18 条 domain=extension Capability**: corrected_capability_registry.json 已注册 (install_mcp_*, install_skill_*, hook_*, workflow_*, mcp_*, etc.)

## 15. Tool Contribution 声明

- **现有**: ContributionKind=tool + RuntimeBindingMeta 已支持 Tool 注册
- **15 种 Contribution Kind**: 全面覆盖 Tool/Skill/MCP/Provider/Workflow/Hook/Event/Schedule/UI/Resource
- **Authority 边界**: Manifest 声明 Tool Contribution，ToolRegistry 持有实际 Tool Definition

## 16. Permission 声明

- **事实源**: PermissionDefinitionRegistry + PermissionBroker
- **Manifest 角色**: Extension 声明所需 Permission (PermissionReq)，不自动 Grant
- **Fail Closed**: 引用未知 Permission -> validation error / install blocked
- **Separation**: Permission (control right) ≠ Secret (API Key) ≠ Resource (file access)

## 17. Runtime Binding 声明

- **现有 10 种 Binding**: javascript, wasm, mcp, workflow, task, static, builtin, trusted_service, plugin_service, mcp_tool
- **Model/Voice Binding**: 通过 static/runtime + service module 表达 (host_internal)
- **Manifest 角色**: 声明期望运行时，RuntimeAdapterRegistry 决定实际调度
- **Authority 边界**: Manifest ≠ Runtime State Store

## 18. Provider 声明

### Model/Voice Provider

- **Representation**: ContributionKind=provider + ModuleType=service + RuntimeMeta.Type=static
- **现有能力**: 完整，可通过 Extension 包声明新的 Model/Voice Provider
- **Config Authority**: Model Service/Voice Service 配置属于 User/System Configuration，非 Manifest 职责

### Future Platform Provider

- Browser/Search: ContributionKind=provider + RuntimeMeta.Type=javascript
- Android/iOS: ContributionKind=provider + Platform={android,ios}
- 仅需 Manifest 层声明，Adapter 实现归未来

## 19. Resource 声明

- **Manifest ResourceMeta**: 包内资产清单 + integrity hash
- **ResourceURI 系统**: 运行时资源寻址 amitia:// (B13)
- **Boundary**: Manifest 声明资源清单 + hash，ResourceURI 解析运行时路径，Permission 控制访问

## 20. Dependency 声明

- **模型**: Dependency{Type, ID, Version, Optional, Reason}
- **解析器**: dependency.DefaultResolver (semver, cycle detection, topo sort, platform/arch filter)
- **Authority**: Manifest 声明需求，Resolver 决定方式，Install Saga 执行时序

## 21. Compatibility 声明

- **Platform**: windows/linux/macos/android/ios
- **Architecture**: amd64/arm64
- **Version**: SemVer 范围 (MinHostVersion/MaxHostVersion)
- **Legacy 默认**: 未声明 = 全平台兼容

## 22. Lifecycle 声明

- **Manifest 角色**: LifecycleMeta 声明偏好 (AutoUpdate/BackgroundTasks/NetworkAccess/Isolation/Sandbox)
- **执行层次**: Lifecycle Saga (install/update/uninstall) + RuntimeSupervisor (start/stop) + Enablement (enable/disable)
- **边界**: Manifest 不持有 running/failed/degraded 等运行时状态

## 23. Model Provider Manifest

- **Status**: ALREADY_SUPPORTED
- **Representation**: ContributionKind=provider + ModuleType=service + RuntimeMeta.Type=static
- **Capability Reference**: builtin/model/text_generation etc.
- **Config**: model_configs 表 (User Configuration) ≠ Manifest
- **Credential**: Encrypted Secret ≠ Manifest

## 24. Voice Provider Manifest

- **Status**: ALREADY_SUPPORTED
- **Representation**: ContributionKind=provider + ModuleType=service
- **ASR**: 4 providers / TTS: 8 providers / Realtime: 1 provider
- **Config**: tts_configs/asr_configs 表 (User Configuration)

## 25. Automation Contribution

- **Status**: ALREADY_SUPPORTED
- **Representation**: 各自独立 ContributionKind + 各自 Canonical Registry
- **Workflow**: ContributionKind=workflow -> WorkflowRegistry
- **Hook**: ContributionKind=hook -> HookService
- **Event**: ContributionKind=event_subscription -> EventService
- **Schedule**: ContributionKind=schedule -> ScheduleService
- **Task**: ContributionKind=background_task -> TaskRuntimeService

## 26. Future Browser/Search Manifest

- **Status**: Manifest 已支持 (ContributionKind=provider + RuntimeMeta.Type=javascript)
- **待实现**: adapter_browser_runtime (仅 Adapter 实现，非 Manifest)

## 27. Android/iOS Provider Manifest

- **Status**: Manifest 已支持 (Compatibility.Platforms 包含 android/ios)
- **待实现**: adapter_android_native/adapter_ios_native (仅 Adapter 实现)

## 28. 实际代码修改

**无。** 现有 Manifest v2 与 Extension Lifecycle 已经满足 B17 最终合同。

## 29. Manifest Validator 变更

**无。** 现有 Validator 已完整覆盖:
- schema_v2 内嵌 JSON Schema
- Validate() 语义校验
- ValidateWithSchema() Schema 校验
- agent_skill.CompatibilityValidator 平台校验
- dependency.Resolver 依赖校验

## 30. Backward Compatibility

| 检查项 | 结果 |
|--------|------|
| Old Manifest Parse | PASS |
| Old Manifest Validate | PASS |
| Existing Contributions 15 Kind | PASS |
| Existing Permission Declarations | PASS |
| Existing Runtime Bindings 10 types | PASS |
| Existing Extension Package .amitiax | PASS |
| New Required Fields | 0 |

## 31. Capability/Permission/Runtime Authority 边界

### Capability Authority

- **Owner**: ToolRegistry + corrected_capability_registry.json
- **Manifest 角色**: 引用 Capability ID，不定义语义

### Permission Authority

- **Owner**: PermissionDefinitionRegistry + PermissionBroker
- **Manifest 角色**: 声明 Permission 引用，不自动 Grant

### Runtime Authority

- **Owner**: RuntimeAdapterRegistry + RuntimeHost + RuntimeOrchestrator
- **Manifest 角色**: 声明 Runtime Binding 偏好，不持有 Runtime 实例状态

### State Authority

- **Owner**: Domain-specific State Stores (enablement.StateStore, WorkflowRun, TaskRunStatus, etc.)
- **Manifest 角色**: 只有 valid/invalid 解析结果，不是 Extension 运行状态事实源

## 32. State/Lifecycle 边界

| 层次 | 职责 | 实现 |
|------|------|------|
| Manifest | 声明偏好 (AutoUpdate/Isolation/Sandbox) | LifecycleMeta |
| Lifecycle Saga | 执行 install/enable/disable/update/uninstall | *.saga.go |
| Runtime Supervisor | 跟踪实际 runtime state | runtime_supervisor/ |
| Enablement | 管理 enable/disable 状态 | enablement.StateStore |
| Health | Provider health check (runtime) | Adapter 层 |

## 33. Duplicate System Validation

| 系统 | 新增计数 |
|------|---------|
| ManifestV3 | 0 |
| Lifecycle2 | 0 |
| ContributionRegistry2 | 0 |
| ProviderRegistry2 | 0 |
| ExtensionManager2 | 0 |
| RuntimeRegistry2 | 0 |
| PermissionSystem2 | 0 |
| ProductionFakeProvider | 0 |

全部为 0。

## 34. Security Validation

| 检查项 | 结果 |
|--------|------|
| Integrity Verification (SHA-256 tree) | PASS |
| Signature Verification (ed25519) | PASS |
| Publisher Trust Levels (7 levels) | PASS |
| Key Lifecycle (active/rotated/expired/revoked) | PASS |
| Fail Closed Unknown Permission | PASS |
| Secret Separation (not in Manifest plaintext) | PASS |
| Config/Manifest Separation | PASS |

## 35. Deferred Gap

### 35.1 B105～B110

- JS/WASM 执行引擎实现 (Manifest 已就绪，Runtime Adapter 已定义)
- Workflow/Hook/Event/Schedule Tool 注册 (Manifest Contribution 已支持)

### 35.2 B111～B117

- Model/Voice Provider 实现 (Manifest 已通过 provider ContributionKind 支持)
- Wake word / Continuous listening (Manifest 层不是障碍)

### 35.3 B123～B138

- iOS Sandbox/Native Provider (Manifest Platform=ios 已声明)
- Android Native Adapter (Manifest Platform=android 已声明)

### 35.4 Other

- Future Runtime Adapter 实现 (browser/android/ios)
- Extension Marketplace 元数据
- Permission 重审批流
- Auto Update 引擎
- Provider Health 实现

## 36. B18 输入

- **Manifest Authority**: backend/internal/extension/kernel/manifest_v2/ (unique)
- **Lifecycle Authority**: Extension Kernel Saga (unique)
- **Contribution Authority**: TypedContributionInstaller (unique, 15 Kinds)
- **Runtime Declaration**: 10 existing bindings + future adapter required
- **Permission Declaration**: PermissionDefinitionRegistry (unique)
- **Capability Declaration**: corrected_capability_registry.json (unique)
- **Duplicate Guard**: 0 second systems
- **Status**: B18 可直接验收

## 37. 后续 Provider Manifest 输入

### 37.1 B105-B110 Manifest Input

- 所有 Automation Contribution 已支持 (ContributionKind={workflow,hook,event_subscription,schedule,background_task})
- JS/WASM Runtime 已定义 (RuntimeMeta.Type={javascript,wasm})

### 37.2 B111-B117 Manifest Input

- Model/Voice Provider 已支持 (ContributionKind=provider + ModuleType=service)
- 无需 Manifest 变更

### 37.3 B123-B138 Manifest Input

- Android/iOS Platform 已声明 (Compatibility.Platforms)
- Provider Contribution 已支持

## 38. 测试

| 测试类别 | 结果 |
|---------|------|
| manifest_v2 | PASS_NO_CODE_CHANGE (现有 manifest_test.go) |
| lifecycle | PASS_NO_CODE_CHANGE (现有 saga + enablement 测试) |
| contribution | PASS_NO_CODE_CHANGE (现有 contribution_installer 测试) |
| compatibility | PASS_NO_CODE_CHANGE (现有 CompatibilityValidator 测试) |
| kernel regression | PASS_NO_CODE_CHANGE (无代码修改) |
| race | NOT_REQUIRED (无共享状态修改) |
| gofmt | NO_CODE_MODIFIED |

## 39. Source Boundary

| 检查项 | 结果 |
|--------|------|
| Modified files | 0 |
| Unexpected files | 0 |
| go.mod | unchanged |
| go.sum | unchanged |
| DB | unchanged |

## 40. 阻断项

无。

## 41. 最终结论

1. **B17 仅扩展现有 Manifest v2 (实际无需修改)**: 确认。Manifest v2 已内置 provider ContributionKind、15 种 Contribution Kind、10 种 Runtime Binding、5 平台 Compatibility、完整 Lifecycle Metadata。

2. **Manifest v2 继续保持唯一 Extension 包合同**: 确认。未创建 Manifest v3 / ExtensionManifest2 / NewExtensionSchema。

3. **Extension Lifecycle 继续使用现有唯一实现**: 确认。Lifecycle Saga + Enablement + RuntimeSupervisor 三层分离，无 Lifecycle2。

4. **Contribution Registry 保持唯一**: 确认。TypedContributionInstaller 单入口，15 Kind 分发。

5. **Model/Voice/Automation Provider 声明已能被现有 Manifest 表达**: 确认。通过 ContributionKind=provider + ModuleType + RuntimeMeta + Compatibility 组合表达。

6. **Capability 仍由现有 Capability 体系负责**: 确认。Manifest 只引用 Capability ID，语义事实源在 corrected_capability_registry.json。

7. **Permission 仍由 PermissionDefinitionRegistry/PermissionBroker 负责**: 确认。Manifest 声明 Permission 需求，不自动 Grant。

8. **Runtime 仍由 RuntimeAdapter/RuntimeHost 等系统负责**: 确认。Manifest 声明 Binding 偏好，调度由 RuntimeHost 执行。

9. **State 仍由各 Domain 事实源负责**: 确认。Manifest 只有 valid/invalid 解析结果，不持有 running/failed 等状态。

10. **没有创建 Manifest v3、Lifecycle2、Provider Registry2 等第二套系统**: 确认。所有第二系统计数为 0。

11. **旧 Manifest 和现有扩展继续向后兼容**: 确认。Parse() 完整向后兼容，0 新必填字段。

12. **可以进入 B18**: 确认。B17 满足所有 PASS 条件。

---

**最终状态**: PASS_NO_CODE_CHANGE
**下一步**: B18