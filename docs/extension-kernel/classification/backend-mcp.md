# 后端 MCP 分类

> 范围：`backend/internal/mcp/` 全部文件

---

## 一、保留并抽取

### MCP-001: MCP Client
- **文件**: `client/connection.go`, `client/request_manager.go`, `client/client_test.go`
- **类型/函数**: `Client`, `Connection`, `RequestManager`
- **当前职责**: MCP 标准协议客户端
- **目标分类**: 保留并抽取
- **判定依据**: 标准 MCP 协议实现，不依赖 Amitia 业务模型
- **目标组件**: MCP Manager
- **抽取目标**: 独立 MCP Client 包

### MCP-002: MCP Transport
- **文件**: `transport/transport.go`, `transport/stdio.go`, `transport/streamable_http.go`, `transport/security.go`, `transport/process_windows.go`, `transport/process_unix.go`
- **类型/函数**: `Transport`, `StdioTransport`, `StreamableHTTPTransport`, 安全校验
- **当前职责**: MCP 标准传输层
- **目标分类**: 保留并抽取
- **判定依据**: 标准 MCP 传输协议实现
- **目标组件**: MCP Manager
- **抽取目标**: 独立 MCP Transport 包

### MCP-003: MCP OAuth
- **文件**: `auth/oauth.go`, `auth/token_store.go`, `auth/oauth_test.go`, `auth/token_store_test.go`
- **类型/函数**: `OAuthClient`, `TokenStore`, `PendingSession`
- **当前职责**: MCP OAuth 认证
- **目标分类**: 保留并抽取
- **判定依据**: MCP 标准认证协议，通用能力
- **目标组件**: MCP Manager（认证层）
- **抽取目标**: 独立 MCP OAuth 包

### MCP-004: MCP Protocol
- **文件**: `protocol/message.go`, `protocol/errors.go`, `protocol/version.go`, `protocol/message_test.go`
- **类型/函数**: `Message`, `JSONRPCRequest`, `JSONRPCResponse`, 错误定义, 版本协商
- **当前职责**: MCP JSON-RPC 协议实现
- **目标分类**: 保留并抽取
- **判定依据**: 标准 MCP JSON-RPC 消息格式
- **目标组件**: MCP Manager
- **抽取目标**: 独立 MCP Protocol 包

### MCP-005: MCP Discovery
- **文件**: `discovery/service.go`, `discovery/service_test.go`
- **类型/函数**: `DiscoveryService`
- **当前职责**: MCP 服务器发现与能力同步
- **目标分类**: 保留并抽取
- **判定依据**: Discovery 本质是通用能力（Tools/Resources/Prompts 同步）
- **目标组件**: MCP Manager
- **抽取目标**: 独立 MCP Discovery 服务

### MCP-006: MCP Features
- **文件**: `features/service.go`, `features/service_test.go`
- **类型/函数**: `FeatureService`
- **当前职责**: MCP 功能特性管理
- **目标分类**: 保留并抽取
- **判定依据**: Feature 检测是通用能力
- **目标组件**: MCP Manager

### MCP-007: MCP Host
- **文件**: `host/service.go`, `host/interaction.go`, `host/roots.go`
- **类型/函数**: `HostService`, `InteractionHandler`, `RootsProvider`
- **当前职责**: MCP Host 端能力（Sampling, Elicitation, Roots）
- **目标分类**: 保留并抽取
- **判定依据**: MCP Host 标准能力
- **目标组件**: MCP Manager

### MCP-008: MCP Dependency
- **文件**: `dependency/service.go`, `dependency/service_test.go`
- **类型/函数**: `DependencyService`
- **当前职责**: MCP 依赖管理
- **目标分类**: 改造后复用
- **判定依据**: 依赖逻辑正确，但绑定 Agent Skill 旧模型
- **目标组件**: Dependency Resolver
- **目标新模型**: 统一依赖声明

---

## 二、改造后复用

### MCP-101: MCP Connection Manager
- **文件**: `manager/manager.go`, `manager/manager_test.go`
- **类型/函数**: `Manager`
- **当前职责**: MCP 连接池管理
- **目标分类**: 改造后复用
- **判定依据**: 连接管理正确，但需要统一生命周期和重连策略
- **目标组件**: MCP Manager
- **目标新模型**: 统一 Connection Pool

### MCP-102: MCP Repository
- **文件**: `repository.go`, `repository_test.go`
- **类型/函数**: `Repository`, `Server`, `ServerInput`, CRUD 方法
- **当前职责**: MCP 数据持久化
- **目标分类**: 改造后复用（详见数据表分类）
- **判定依据**: 数据存储逻辑，但表结构需重构

### MCP-103: MCP Models
- **文件**: `model.go`
- **类型**: `Server`, `ServerScopeBinding`, `ServerCredential`, `ServerCapability`, `ToolDefinition`, `ResourceDefinition`, `PromptDefinition`, `DependencyLink`, `Operation`, `OAuthSession`, `Task`, `AuditLog`
- **当前职责**: MCP 数据模型
- **目标分类**: 改造后复用
- **判定依据**: 模型定义正确，但部分表结构需调整
- **目标组件**: MCP Manager 数据层

---

## 三、仅用于迁移

### MCP-201: MCP Server 旧表读取
- **文件**: `repository.go`
- **类型/函数**: `GetServer`, `ListServers`, `ListEnabledServers`, `DeleteServer`
- **当前职责**: 旧 MCP Server 表 CRUD
- **目标分类**: 仅用于迁移
- **迁移来源**: `mcp_servers` 表
- **迁移目标**: 新 MCP Server 存储
- **删除条件**: 旧 Server 数据全部迁移

### MCP-202: ScopeBinding 旧表
- **文件**: `repository.go`
- **类型/函数**: `SetScopeEnabled`, `ResolveScopeEnabled`
- **当前职责**: 旧 MCP 作用域绑定
- **目标分类**: 仅用于迁移
- **迁移来源**: `mcp_server_scope_bindings`
- **迁移目标**: 新 Scope Manager
- **删除条件**: 迁移完成

### MCP-203: Tool Enabled 旧表
- **文件**: `repository.go`
- **类型/函数**: `GetToolBySkillID`, `SetToolEnabled`
- **当前职责**: 旧 MCP Tool 启用状态
- **目标分类**: 仅用于迁移
- **迁移来源**: `mcp_tools` 表
- **迁移目标**: 新 Tool Registry
- **删除条件**: Tool 迁移完成

### MCP-204: DependencyLink 旧表
- **文件**: `repository.go`
- **类型/函数**: `UpsertDependencyLink`, `ListDependencyLinks`, `RemoveDependencyLinks`
- **当前职责**: 旧依赖关系
- **目标分类**: 仅用于迁移
- **迁移来源**: `mcp_dependency_links`
- **迁移目标**: 新 Dependency Manager
- **删除条件**: 依赖数据迁移完成

### MCP-205: OAuth Token 旧引用
- **文件**: `repository.go`
- **类型/函数**: `PutCredentialReference`, `CredentialReference`, `SaveOAuthTokenReference`, `OAuthTokenReference`
- **当前职责**: 旧凭证引用存储
- **目标分类**: 仅用于迁移
- **迁移来源**: `mcp_server_credentials`, `mcp_oauth_sessions`
- **迁移目标**: 新 Secret Broker
- **删除条件**: 凭证迁移完成

### MCP-206: Tool/Resource/Prompt Sync 旧版
- **文件**: `repository.go`
- **类型/函数**: `SyncTools`, `SyncResources`, `SyncPrompts`
- **当前职责**: 旧同步逻辑
- **目标分类**: 仅用于迁移
- **迁移来源**: MCP Server 发现缓存
- **迁移目标**: 新 MCP Manager 同步
- **删除条件**: 新 MCP Manager 就绪

---

## 四、最终删除

### MCP-301: MCP Skill Runtime（核心删除）
- **文件**: `skill/runtime.go`, `skill/runtime_test.go`
- **类型/函数**: `Runtime`, `RegisterAll`, `RegisterServer`, `build`
- **当前职责**: MCP Tool → SkillDefinition 适配器
- **目标分类**: 最终删除
- **替代组件**: MCP Manager 直接注册 Tool 到 Tool Registry
- **依赖解除顺序**: 
  1. Tool Registry 就绪
  2. MCP Manager 直接注册
  3. 删除 `mcp/skill/runtime.go`
- **删除步骤**: 本文件及其测试直接删除

### MCP-302: MCP API 直接写 Extension Registry
- **文件**: Manager 中通过 Skill Runtime 注册逻辑
- **目标分类**: 最终删除
- **替代组件**: MCP Manager → Tool Registry 直接注册

### MCP-303: 重复审计模型
- **文件**: `model.go` 中 `AuditLog`
- **目标分类**: 最终删除
- **替代组件**: 统一 Audit Store
- **数据迁移条件**: 旧审计日志迁移

### MCP-304: 独立扩展生命周期
- **文件**: Manager 中独立的 Connect/Disconnect 管理
- **目标分类**: 最终删除
- **替代组件**: Runtime Supervisor 统一管理
