# 后端 Extension Runtime & Registry 分类

> 范围：`backend/internal/extension/runtime.go`, `registry.go`, `executor.go`, `service.go`, `router.go`, `handler.go`, `protocol.go`, `capability.go`, `permission.go`, `repository.go`, `lifecycle_service.go`, `config_crypto.go`, `owned_resource_repository.go`, `legacy_tool_adapter.go`, `schema_validator.go`

---

## 一、保留并抽取

### EXT-RT-001: Executor 超时控制
- **文件**: `executor.go`
- **类型/函数**: `(e *Executor) Execute` 中的 `context.WithTimeout` 逻辑
- **当前职责**: Skill 执行超时控制
- **目标分类**: 保留并抽取
- **判定依据**: 通用执行安全保障，不依赖 Skill 领域模型
- **目标组件**: Runtime Supervisor
- **抽取目标**: 通用执行超时控制器，输入为 `context.Context` + timeout duration
- **需解除依赖**: `SkillDefinition.Timeout` 字段引用
- **保留测试**: `executor.go` 中超时相关逻辑

### EXT-RT-002: Executor Panic 恢复
- **文件**: `executor.go`
- **类型/函数**: `defer/recover` panic 处理逻辑
- **当前职责**: Skill handler panic 恢复
- **目标分类**: 保留并抽取
- **判定依据**: 通用运行时安全保障
- **目标组件**: Runtime Supervisor
- **抽取目标**: 通用 panic recovery wrapper
- **需解除依赖**: `ExtensionError` 类型引用（可抽象为通用 error）
- **保留测试**: panic recovery 行为测试

### EXT-RT-003: Executor 幂等性
- **文件**: `executor.go`
- **类型/函数**: `idempotencyMu`, `idempotency` map, `inFlight` map, `scopedIdempotencyKey`, `defaultIdempotencyKey`
- **当前职责**: Skill 执行幂等
- **目标分类**: 保留并抽取
- **判定依据**: 通用幂等执行模式，不依赖 Skill 领域模型
- **目标组件**: Runtime Supervisor
- **抽取目标**: 通用幂等执行器
- **需解除依赖**: `SkillDefinition.Idempotent` 字段

### EXT-RT-004: Executor 并发控制
- **文件**: `executor.go`
- **类型/函数**: `handlerSlots chan struct{}` 槽位控制
- **当前职责**: Skill handler 并发限制
- **目标分类**: 保留并抽取
- **判定依据**: 通用并发控制模式
- **目标组件**: Runtime Supervisor
- **抽取目标**: 通用并发槽位控制器

### EXT-RT-005: Executor 审计持久化
- **文件**: `executor.go`
- **类型/函数**: `CreateRun`, `SetRunStatus`, `UpdateRun` 调用
- **当前职责**: Skill 执行审计记录
- **目标分类**: 保留并抽取
- **判定依据**: 通用执行审计模式
- **目标组件**: Audit Store
- **抽取目标**: 通用执行审计接口
- **需解除依赖**: `RunView` 类型

### EXT-RT-006: Executor 副作用补偿
- **文件**: `executor.go`
- **类型/函数**: `RegisterOwnedSideEffects`, `CompensateUnownedSideEffects`
- **当前职责**: 副作用资源记录与补偿
- **目标分类**: 保留并抽取
- **判定依据**: 通用资源归属管理
- **目标组件**: Storage Broker
- **抽取目标**: 通用资源归属追踪器

### EXT-RT-007: Config Crypto
- **文件**: `config_crypto.go`
- **类型/函数**: `configCipher`, `encrypt`, `decrypt`
- **当前职责**: 扩展配置加密
- **目标分类**: 保留并抽取
- **判定依据**: 通用加密能力，不依赖扩展领域模型
- **目标组件**: Secret Broker
- **抽取目标**: 通用配置加解密器

### EXT-RT-008: Schema Validator
- **文件**: `schema_validator.go`
- **类型/函数**: `SchemaValidator`, `Validate`
- **当前职责**: JSON Schema 校验
- **目标分类**: 保留并抽取
- **判定依据**: 通用 JSON Schema 校验
- **目标组件**: Manifest Parser（通用校验器）
- **抽取目标**: 独立 Schema 校验包

### EXT-RT-009: Protocol 工具函数
- **文件**: `protocol.go`
- **类型/函数**: `normalizeJSON`, `compactSensitiveJSON`, `redactValue`, `isSensitiveKey`, `hasPlaintextSecret`, `redactJSON`, `restoreRedactedValue`
- **当前职责**: 敏感数据脱敏
- **目标分类**: 保留并抽取
- **判定依据**: 通用数据安全工具，不依赖 Skill 模型
- **目标组件**: Secret Broker
- **抽取目标**: 独立脱敏工具包

### EXT-RT-010: Protocol 版本比较
- **文件**: `protocol.go` / `registry.go`
- **类型/函数**: `semverPattern` 正则、semver 比较逻辑
- **当前职责**: 版本号校验
- **目标分类**: 保留并抽取
- **判定依据**: 通用版本比较
- **目标组件**: Package Manager（版本比较器）
- **抽取目标**: 独立 semver 包

### EXT-RT-011: Capability 定义
- **文件**: `capability.go`
- **类型/函数**: `CapabilityDefinition`, `Capability()`, `Capabilities()`
- **当前职责**: 内置能力定义
- **目标分类**: 保留并抽取
- **判定依据**: 能力定义是通用概念，但当前绑定 `SkillDefinition`
- **实际分类调整**: 改造后复用（定义本身保留，但需脱离 Skill 模型）
- **目标组件**: Contribution Registry
- **目标新模型**: 通用 `Capability` 能力定义

### EXT-RT-012: OwnedResource Repository
- **文件**: `owned_resource_repository.go`
- **类型/函数**: `ownedResourceRecord`, `RegisterOwnedSideEffects`, `CleanupOwnedResources`, `RetryOwnedResourceCleanup`
- **当前职责**: 扩展资源归属管理
- **目标分类**: 改造后复用
- **判定依据**: 资源归属是通用需求，但当前绑定 `SideEffectRecord` 和 `ExecutionScope`
- **目标组件**: Storage Broker
- **目标新模型**: 通用资源归属追踪

---

## 二、改造后复用

### EXT-RT-101: Registry 核心结构
- **文件**: `registry.go`
- **类型/函数**: `Registry`, `SkillRegistry` interface
- **当前职责**: Skill 注册与查询中枢
- **目标分类**: 改造后复用
- **判定依据**: 注册中心核心逻辑正确，但所有概念以 Skill 为中心
- **目标组件**: Contribution Registry
- **目标新模型**: `ExtensionRegistry`，注册对象从 `SkillDefinition` → `Capability`/`Contribution`
- **需要替换**: `SkillRegistry` → `ContributionRegistry`
- **需要删除**: `SkillHandler` 绑定、`SkillFilter`

### EXT-RT-102: Registry 作用域过滤
- **文件**: `registry.go`
- **类型/函数**: `GetScoped`, `Available`, `ResolveScopeEnabled`
- **当前职责**: 按角色/会话作用域过滤 Skill
- **目标分类**: 改造后复用
- **判定依据**: 作用域过滤逻辑正确，但输入输出绑定 Skill
- **目标组件**: Scope Manager
- **目标新模型**: 通用作用域过滤器

### EXT-RT-103: Runtime 装配
- **文件**: `runtime.go`
- **类型/函数**: `Runtime`, `NewRuntime`
- **当前职责**: 所有扩展子系统总装配
- **目标分类**: 改造后复用
- **判定依据**: 装配模式正确，但装配对象全是旧系统
- **目标组件**: Extension Kernel 启动装配
- **目标新模型**: `ExtensionKernel`，装配新组件

### EXT-RT-104: Runtime Close
- **文件**: `runtime.go`
- **类型/函数**: `(r *Runtime) Close`
- **当前职责**: 停止 PluginManager
- **目标分类**: 改造后复用
- **判定依据**: 关闭流程需要扩展到所有子系统
- **目标组件**: Runtime Supervisor（统一关闭）

### EXT-RT-105: Permission Evaluator
- **文件**: `permission.go`
- **类型/函数**: `DefaultPermissionEvaluator`, `EvaluateExecution`, `GrantSystemPolicy`
- **当前职责**: Skill 权限评估
- **目标分类**: 改造后复用
- **判定依据**: 权限评估模型正确，但绑定 Skill/Capability 旧概念
- **目标组件**: Permission Broker
- **目标新模型**: 通用权限评估器，输入为 `ExtensionIdentity` + `Capability`

### EXT-RT-106: ExtensionService
- **文件**: `service.go`
- **类型/函数**: `ExtensionService`, `Service` interface
- **当前职责**: 扩展服务门面
- **目标分类**: 改造后复用
- **判定依据**: 服务门面模式正确，但方法分散在 Skill/Plugin 多系统
- **目标组件**: Extension Kernel API
- **目标新模型**: 统一 `ExtensionKernelService`

### EXT-RT-107: Handler
- **文件**: `handler.go`
- **类型/函数**: `Handler`, HTTP 处理方法
- **当前职责**: Skill HTTP API
- **目标分类**: 改造后复用
- **判定依据**: HTTP 处理器模式正确，但路由全绑定 Skill
- **目标组件**: Extension Kernel HTTP API
- **目标新模型**: 统一 Extension HTTP Handler

### EXT-RT-108: Router
- **文件**: `router.go`
- **类型/函数**: `RegisterRouter`
- **当前职责**: 扩展 API 路由注册
- **目标分类**: 改造后复用
- **判定依据**: 路由注册逻辑，但路由设计为多子系统分散
- **目标组件**: Extension Kernel 路由
- **目标新模型**: 统一扩展路由

### EXT-RT-109: LifecycleService
- **文件**: `lifecycle_service.go`
- **类型/函数**: `ExtensionLifecycleService`, `extensionLifecycleService`
- **当前职责**: 扩展启停协调
- **目标分类**: 改造后复用
- **判定依据**: 生命周期管理正确，但需要统一所有子系统
- **目标组件**: Runtime Supervisor

### EXT-RT-110: Repository
- **文件**: `repository.go`
- **类型/函数**: `Repository`, 所有数据访问方法
- **当前职责**: Skill 数据持久化
- **目标分类**: 改造后复用（部分方法仅用于迁移）
- **详见**: 数据表分类文档

---

## 三、仅用于迁移

### EXT-RT-201: LegacyToolAdapter
- **文件**: `legacy_tool_adapter.go`
- **类型/函数**: `LegacyToolAdapter`, `RegisterAll`, `Adapt`
- **当前职责**: 旧工具 → SkillDefinition 适配
- **目标分类**: 仅用于迁移
- **迁移来源**: `agent/tool` 旧工具
- **迁移目标**: Extension Kernel Tool Registry
- **允许调用者**: 启动迁移脚本
- **禁止调用者**: 新扩展注册流程
- **停止写入时间**: 迁移完成后
- **删除条件**: 所有旧工具已迁移为原生 Capability

### EXT-RT-202: Repository ScopeBinding
- **文件**: `repository.go`
- **类型/函数**: `ResolveScopeEnabled`, `SetScopeEnabled`, `DeleteScopeBinding`
- **当前职责**: 旧作用域绑定读写
- **目标分类**: 仅用于迁移
- **迁移来源**: `extension_scope_bindings` 表
- **迁移目标**: 新 Scope Manager
- **删除条件**: 旧作用域数据全部迁移

### EXT-RT-203: Repository Config 读取
- **文件**: `repository.go`
- **类型/函数**: `GetEffectiveConfig`, `getStoredConfig`
- **当前职责**: 旧配置读取
- **目标分类**: 仅用于迁移
- **迁移来源**: `extension_configs` 表
- **迁移目标**: 新 Config Store

---

## 四、最终删除

### EXT-RT-301: Register AgentSkill Runtime（旧路径）
- **文件**: `runtime.go`
- **函数**: `registerAgentSkillRuntime`
- **当前职责**: Agent Skill → SkillDefinition 注册
- **目标分类**: 最终删除
- **替代组件**: Agent Skill Catalog 直接注册到 Contribution Registry
- **数据迁移条件**: 无持久化数据
- **删除步骤**: 新 AgentSkill.Runtime 完成后删除

### EXT-RT-302: Runtime ModelTools Skill 过滤
- **文件**: `runtime.go`
- **函数**: `(r *Runtime) ModelTools`
- **当前职责**: Skill 过滤为 LLM Tool
- **目标分类**: 最终删除
- **替代组件**: Tool Registry + Scope Manager 统一过滤
- **数据迁移条件**: 无持久化数据

### EXT-RT-303: Runtime pluginSnapshot
- **文件**: `runtime.go`
- **函数**: `(r *Runtime) pluginSnapshot`
- **当前职责**: 构建 Plugin Hook 执行快照
- **目标分类**: 最终删除
- **替代组件**: Extension Kernel 内置执行上下文
- **数据迁移条件**: 无持久化数据

### EXT-RT-304: Service setSkillEnabled（Workshop 耦合）
- **文件**: `service.go`
- **函数**: `setSkillEnabled` 中 Workshop CASStatus 逻辑
- **当前职责**: Skill 启用时同步 Workshop 状态
- **目标分类**: 最终删除
- **替代组件**: Workshop 订阅 Extension 生命周期事件
- **数据迁移条件**: Workshop 数据需迁移

### EXT-RT-305: Protocol SkillDefinition
- **文件**: `protocol.go`
- **类型**: `SkillDefinition`
- **当前职责**: 核心数据类型
- **目标分类**: 最终删除
- **替代组件**: 拆分后各新类型（Capability, Contribution, Resource 等）
- **数据迁移条件**: 所有 Skill 数据迁移到新表

### EXT-RT-306: Protocol Skill 专属类型
- **文件**: `protocol.go`
- **类型**: `SkillSource`, `SkillTrigger`, `RegisteredSkill`, `SkillFilter`, `SkillView`, `SkillDetailView`, `ExecuteSkillRequest`, `SkillHandler`, `SkillResult`
- **当前职责**: Skill 专属类型定义
- **目标分类**: 最终删除
- **替代组件**: 新领域类型
- **数据迁移条件**: API 迁移完成后删除

### EXT-RT-307: Service Plugin 委托方法
- **文件**: `service.go`
- **函数**: `ListPlugins`, `GetPlugin`, `EnablePlugin` 等 Plugin 方法
- **当前职责**: Service 门面中的 Plugin 委托
- **目标分类**: 最终删除
- **替代组件**: Plugin 子系统独立 API（由 Extension Kernel 统一路由）

### EXT-RT-308: Runtime BeforePrompt/AfterReply
- **文件**: `runtime.go`
- **函数**: `(r *Runtime) BeforePrompt`, `AfterReply`
- **当前职责**: Plugin Hook 调用
- **目标分类**: 最终删除
- **替代组件**: Hook Pipeline 在 Extension Kernel 中统一管理
