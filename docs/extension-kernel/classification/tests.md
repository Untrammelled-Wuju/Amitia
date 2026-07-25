# 测试分类

> 范围：所有 `backend/internal/extension/*_test.go`, `backend/internal/mcp/**/*_test.go`

---

## 一、直接保留

### TST-001: Agent Skill 解析测试
- **文件**: `agent_skill_test.go`
- **测试**: `TestAgentSkillParserValidation`, `TestAgentSkillDirectoryLimitsAndUnsafeSVG`, `TestAgentSkillParserResourcesMappingsAndOpenAI`, `TestAgentSkillParserAmitiaMCPDependenciesOverrideOpenAI`, `TestAgentSkillZIPSecurity`
- **目标分类**: 直接保留
- **判定依据**: 测试核心解析器，解析器本身保留抽取
- **保留条件**: 随解析器抽取到 Agent Skill Catalog

### TST-002: MCP Protocol 测试
- **文件**: `client/client_test.go`, `transport/stdio_test.go`, `transport/streamable_http_test.go`, `protocol/message_test.go`
- **目标分类**: 直接保留
- **判定依据**: 测试标准 MCP 协议实现
- **保留条件**: 随 MCP 协议基础设施保留

### TST-003: MCP OAuth 测试
- **文件**: `auth/oauth_test.go`, `auth/token_store_test.go`
- **目标分类**: 直接保留
- **判定依据**: 测试 OAuth 认证流程
- **保留条件**: 随 MCP OAuth 保留

### TST-004: MCP 功能测试
- **文件**: `discovery/service_test.go`, `features/service_test.go`, `host/service_test.go`, `host/interaction_test.go`, `dependency/service_test.go`, `manager/manager_test.go`
- **目标分类**: 直接保留
- **判定依据**: 测试 MCP 标准功能
- **保留条件**: 随 MCP 基础设施保留

### TST-005: Plugin 运行时测试
- **文件**: `plugin_runtime_test.go`
- **目标分类**: 改造成新内核测试
- **判定依据**: 测试 Plugin 运行时，运行时可改造

### TST-006: Runtime 测试
- **文件**: `runtime_test.go`
- **目标分类**: 改造成新内核测试
- **判定依据**: 测试 Runtime 装配
- **改造方向**: 改造为 Extension Kernel 装配测试

### TST-007: Workshop Workflow 测试
- **文件**: `workshop_workflow_test.go`
- **测试**: `TestWorkflowCompilerStaticPolicies`, `TestWorkflowCompilerRejectsTransitiveSkillCycle`, `TestNetworkTargetPolicy` 等
- **目标分类**: 直接保留
- **判定依据**: 测试 Workflow 编译器核心逻辑
- **保留条件**: 随 Workflow Engine 保留

### TST-008: Workshop 集成测试
- **文件**: `workshop_integration_test.go`
- **测试**: `TestWorkshopEndToEndInstallExecuteAndRestore` 等
- **目标分类**: 改造成新内核测试
- **判定依据**: 测试 Workshop 完整流程
- **改造方向**: 改造为 Developer Tooling 集成测试

---

## 二、改造成新内核测试

| 文件 | 当前测试 | 改造方向 |
|---|---|---|
| `scope_binding_test.go` | Scope Binding 测试 | 改造成 Scope Manager 测试 |
| `registry_clone_test.go` | Registry Clone 测试 | 改造成 Contribution Registry 测试 |
| `permission_race_test.go` | 权限并发测试 | 改造成 Permission Broker 并发测试 |
| `openapi_routes_test.go` | OpenAPI 路由覆盖测试 | 改造成新路由覆盖测试 |
| `package_manager_test.go` | Package Manager 测试 | 改造成新 Package Manager 测试 |
| `mcp/repository_test.go` | MCP Repository 测试 | 保留核心，改造表结构测试 |

---

## 三、仅用于迁移验证

| 文件 | 用途 |
|---|---|
| `mcp_client_test.go`（migration 目录） | 验证 MCP 旧表结构存在 |
| `extensions_test.go`（migration 目录） | 验证扩展旧表结构存在 |
| `extension_ecosystem_repair_test.go`（migration 目录） | 验证修复逻辑 |
| `extension_packages_test.go`（migration 目录） | 验证 Package 旧表结构 |

---

## 四、删除

| 文件 | 原因 |
|---|---|
| `skill/runtime_test.go` | 被删除的 MCP Skill Runtime 的测试 |
| （无其他删除） | 所有其他测试均可在迁移后用新测试替代 |

---

## 五、需要补充的测试

| 新测试 | 优先级 |
|---|---|
| Extension Kernel 统一装配测试 | P0 |
| Contribution Registry 注册/查询/卸载测试 | P0 |
| Tool Registry 注册/发现/过滤测试 | P0 |
| Scope Manager 作用域解析测试 | P1 |
| Permission Broker 统一权限测试 | P1 |
| Runtime Supervisor 统一生命周期测试 | P1 |
| Workflow Engine 通用 Adapter 测试 | P1 |
| Package Manager v2 安装/升级/回滚测试 | P0 |
| Secret Broker 加密/解密/迁移测试 | P0 |
| Event Bus 统一事件测试 | P2 |
| Developer Tooling 端到端测试 | P2 |
