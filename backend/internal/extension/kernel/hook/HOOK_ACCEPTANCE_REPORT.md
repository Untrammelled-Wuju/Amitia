# Amitia 第三方 Hook 系统 - 最终验收报告

## 1. 系统概述

Amitia 第三方 Hook 系统为扩展提供受控的消息/模型/工具链路拦截能力，通过定义好的 Hook Point 让第三方扩展在宿主业务流程的关键节点注入自定义逻辑，同时通过严格的权限、作用域、熔断和深度保护确保系统安全。

## 2. 架构组件

### 2.1 核心组件（Go 后端）

| 文件 | 职责 |
|------|------|
| `point.go` | Hook 点定义、阶段（Before/Filter/Transform/Observe/After）、变更规则 |
| `registry.go` | Hook 点注册表，管理生命周期 |
| `contribution.go` | Hook 贡献定义与校验 |
| `pipeline.go` | 核心管道编排，执行阶段流水线 |
| `circuit.go` | 熔断器（Closed/Open/Half-Open 三态） |
| `depth.go` | 递归检测与深度保护 |
| `failure.go` | 失败策略解析与熔断计数 |
| `validator.go` | JSON Patch 校验（白名单、敏感字段、变更声明） |
| `ordering.go` | Hook 排序与循环检测 |
| `lifecycle.go` | 贡献生命周期管理（安装/启用/禁用/更新/卸载） |
| `read_model.go` | 只读查询模型 |
| `service.go` | 统一服务入口 |
| `host_adapters.go` | 宿主集成适配器（消息/模型/工具/工作流） |
| `persistence.go` | SQLite 持久化层 |
| `memory_store.go` | 内存存储（测试用） |
| `runtime_bridge.go` | 运行时桥接 |
| `permission_checker.go` | 权限检查器 |
| `scope_checker.go` | 作用域检查器 |
| `dependency_checker.go` | 依赖检查器 |
| `errors.go` | 错误码与错误结构 |
| `context.go` | 上下文快照与调用输入 |
| `result.go` | 结果与变更操作 |
| `points_register.go` | 默认 Hook 点注册 |
| `bridge.go` | 接口定义（RuntimeBridge/PermissionChecker/ScopeChecker/TraceRecorder/ContributionStore） |

### 2.2 API 层

| 文件 | 职责 |
|------|------|
| `extension/hook_handler.go` | Gin HTTP Handler，提供 RESTful API |
| `extension/router.go` | 路由注册 |
| `extension/kernel_api.go` | Kernel API（已存在，Hook 路由并行注册） |

### 2.3 前端

| 文件 | 职责 |
|------|------|
| `front/src/views/kernel/hook-api.ts` | API 客户端 |
| `front/src/views/kernel/HookCenterView.vue` | Hook 管理页面 |
| `front/src/router/index.ts` | 路由注册 |
| `front/src/navigation/app-nav.ts` | 导航入口 |

### 2.4 SDK

| 文件 | 职责 |
|------|------|
| `hook/sdk/hook-sdk.ts` | TypeScript SDK，供扩展开发者使用 |

### 2.5 宿主集成

| 文件 | 职责 |
|------|------|
| `chat/hook_integration.go` | Chat 服务 Hook 集成层 |
| `cmd/server/services.go` | 服务初始化时注入 HookAdapter |

## 3. API 端点

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/extensions/kernel/hooks/points` | 列出所有 Hook 点 |
| GET | `/api/extensions/kernel/hooks/contributions` | 列出所有贡献（支持 extensionId 过滤） |
| GET | `/api/extensions/kernel/hooks/contributions/:id` | 获取单个贡献详情 |
| POST | `/api/extensions/kernel/hooks/contributions/:id/enable` | 启用贡献 |
| POST | `/api/extensions/kernel/hooks/contributions/:id/disable` | 禁用贡献 |
| GET | `/api/extensions/kernel/hooks/contributions/:id/circuit` | 获取熔断器状态 |
| POST | `/api/extensions/kernel/hooks/contributions/:id/circuit/reset` | 重置熔断器 |
| GET | `/api/extensions/kernel/hooks/points/:pointId/contributions` | 按钩子点列出贡献 |

## 4. 默认 Hook 点

| Hook 点 ID | 风险等级 | 支持阶段 | 第三方 |
|------------|----------|----------|--------|
| message.before_send/1 | medium | before, filter, transform, observe, after | 是 |
| message.before_persist/1 | high | before, filter, transform, observe, after | 是 |
| message.after_persist/1 | low | before, observe, after | 是 |
| model.before_request/1 | high | before, filter, transform, observe | 是 |
| model.after_response/1 | medium | before, transform, observe, after | 是 |
| prompt.before_assemble/1 | medium | before, transform, observe | 是 |
| prompt.after_assemble/1 | low | before, observe, after | 是 |
| tool.before_execute/1 | high | before, filter, transform, observe | 是 |
| tool.after_execute/1 | medium | before, transform, observe, after | 是 |
| workflow.before_start/1 | medium | before, filter, observe | 是 |
| workflow.after_finish/1 | low | before, observe, after | 是 |
| system.shutdown/1 | critical | before, observe | 否 |

## 5. 安全机制

### 5.1 权限检查
- 每个贡献声明所需权限（PermissionRequirement）
- 运行时通过 PermissionBroker 进行权限验证
- 权限不足时返回 `hook_permission_denied` 错误

### 5.2 作用域检查
- 每个贡献声明作用域规则（ScopeRule）
- 运行时通过 ScopeManager 进行作用域验证
- 作用域不匹配时返回 `hook_scope_denied` 错误

### 5.3 熔断器
- 默认阈值：5 次连续失败触发熔断
- 三态：Closed（正常）→ Open（熔断）→ Half-Open（半开恢复）
- 恢复超时：30 秒
- 可手动重置
- 计入熔断的错误：hook_timeout、hook_runtime_error、hook_result_invalid、hook_permission_denied、circuit_open

### 5.4 深度保护
- 默认最大深度：10
- 递归检测：同一 invocation 内同一 contribution + hook point 组合触发递归错误
- 深度超限时返回 `hook_depth_exceeded` 错误

### 5.5 变更校验
- JSON Patch 操作限制：最大 32 个操作
- 路径长度限制：最大 256 字符
- 路径白名单：必须匹配 Hook 点的 AllowedMutations
- 敏感字段保护：禁止修改标记为 sensitive 的路径
- 变更声明：贡献必须声明 MutationClaims 才能修改对应路径
- 独占写入：ConflictExclusive 模式下同一路径只能被一个贡献修改
- 结果大小限制：默认 128KB

## 6. 单元测试

共 15 个测试，全部通过：

| 测试 | 覆盖范围 |
|------|----------|
| TestPipeline_NoContributions | 无贡献时管道正常通过 |
| TestPipeline_HookPointNotFound | 未知钩子点时中止 |
| TestCircuitBreaker_OpenAfterFailures | 达到阈值后熔断开启 |
| TestCircuitBreaker_ResetAfterSuccess | 成功后熔断关闭 |
| TestCircuitBreaker_ManualReset | 手动重置熔断器 |
| TestDepthGuard_RecursionDetection | 递归调用检测 |
| TestDepthGuard_MaxDepthExceeded | 最大深度超限 |
| TestMemoryStore_RegisterAndGet | 贡献注册与获取 |
| TestMemoryStore_ListByHookPoint | 按钩子点列出贡献 |
| TestMemoryStore_SetEnabled | 启用/禁用贡献 |
| TestPatchValidator_ValidPatch | 合法变更通过校验 |
| TestPatchValidator_UnauthorizedPath | 未授权路径被拒绝 |
| TestOrderHooks_ByPriority | 按优先级排序 |
| TestHookError_Error | 错误信息格式 |
| TestShouldCountCircuitFailure | 熔断计数规则 |

## 7. 宿主集成

### 7.1 Chat 服务集成
- 通过 `HookInvoker` 接口解耦
- `HookAdapter` 包装 `HostHookIntegrator`
- 在 `services.go` 初始化时自动注入
- 支持 7 个 Hook 调用点：before_send、before_persist、after_persist、model_before_request、model_after_response、tool_before_execute、tool_after_execute

### 7.2 前端管理
- Hook 管理中心页面（`/kernel/hooks`）
- 钩子点列表：支持搜索、查看风险等级/阶段/第三方权限
- 贡献列表：支持搜索、状态筛选、启用/禁用操作
- 熔断器详情：查看统计、手动重置

## 8. 验收结论

- [x] 核心 Hook 管道（Pipeline）完整实现
- [x] 熔断器（CircuitBreaker）三态机制正常
- [x] 深度保护（DepthGuard）递归检测正常
- [x] 变更校验（PatchValidator）白名单机制正常
- [x] Hook 点注册（Registry）默认 12 个点已注册
- [x] 贡献生命周期管理（Lifecycle）完整
- [x] 持久化层（SQLite）已实现
- [x] RESTful API 8 个端点已注册
- [x] 前端管理页面已实现
- [x] TypeScript SDK 已创建
- [x] 宿主业务链路集成已完成
- [x] 15 个单元测试全部通过

**状态：全部实现完成，验收通过。**
