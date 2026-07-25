# Amitia 扩展系统重构第 14 步实施文档

## 第 14 步：抽取 MCP 协议基础设施

---

## 一、步骤目标

在第 13 步已经完成包安全基础设施抽取的基础上，对 Amitia 当前 MCP 系统进行协议层与产品层拆分，保留并抽取成熟的 MCP 标准协议能力，同时解除其对旧 Skill Runtime、旧 Extension Registry、独立启用状态和分散生命周期的耦合。

本步骤的目标是：

> 将 MCP 的 JSON-RPC、Client、Transport、Session、OAuth、Capability Negotiation、Tools、Resources、Prompts、Tasks、Sampling、Elicitation、Roots、Completion、通知、重连和协议错误处理抽取为独立 MCP Protocol Infrastructure。

该基础设施只负责正确实现 MCP 协议，不再负责：

- 将 MCP Tool 包装为 Skill；
- 直接向旧 Extension Registry 注册；
- 决定角色作用域；
- 决定 Tool 权限；
- 决定扩展安装；
- 管理 Agent Skill 生命周期；
- 管理 `.amitiax` 包；
- 向前端拼接最终 Tool 状态；
- 管理统一执行审计；
- 自行拥有扩展资源。

完成本步骤后，系统必须形成以下分层：

```text
MCP Product Layer
├── MCP Server Definition
├── MCP Ownership
├── MCP Scope
├── MCP Permission
├── MCP UI/API
└── MCP Dependency Integration

MCP Protocol Infrastructure
├── Client
├── JSON-RPC
├── Transport
├── Session
├── Authentication
├── Capability Negotiation
├── Tools
├── Resources
├── Prompts
├── Tasks
├── Sampling
├── Elicitation
├── Roots
├── Completion
├── Notifications
├── Reconnect
└── Protocol Diagnostics
```

---

## 二、当前需要解决的问题

当前 MCP 实现虽然可能已经具备较完整的协议能力，但与 Amitia 旧扩展系统存在以下结构问题：

1. MCP Tool 需要通过 `mcp/skill/runtime.go` 转换为 `SkillDefinition`。
2. MCP API 可能直接写入 Extension Registry。
3. MCP Server Enabled、角色绑定、Tool Enabled、Registry Scope Enabled 共同决定可用性。
4. MCP Manager 同时承担连接、恢复、状态、Tool 暴露和产品管理职责。
5. MCP Discovery 结果与统一 Tool 模型未彻底分离。
6. MCP Tool 调用可能在协议层自行处理权限、审计、超时或错误映射。
7. Agent Skill MCP Dependency 与 MCP Server 所有权混合。
8. OAuth Token、Header、Environment、Command 和 URL 的安全边界不统一。
9. stdio 子进程管理与协议 Client 强耦合。
10. Streamable HTTP、Session 和重连逻辑可能与业务状态混合。
11. MCP Tasks、Sampling、Elicitation、Roots 等能力缺少统一 Host Boundary。
12. 前端 MCP 页面可能把协议状态、扩展状态和 Tool 状态混合展示。
13. Server 删除、断开、禁用和卸载的含义不清晰。
14. 重连后可能重复注册 Tool。
15. 协议通知、列表变化和运行时缓存缺少一致性约束。

本步骤只重构协议基础设施边界，不重建完整 MCP 产品管理界面。

---

## 三、核心原则

### 1. 协议实现与产品语义分离

MCP Protocol Infrastructure 只回答：

- 如何连接；
- 如何握手；
- 如何发送请求；
- 如何接收响应；
- 如何处理通知；
- 如何协商 Capability；
- 如何调用 Tools；
- 如何读取 Resources；
- 如何获取 Prompts；
- 如何处理 Tasks；
- 如何处理 Host Callback；
- 如何恢复连接。

它不回答：

- 当前角色能否使用；
- 用户是否授权；
- Tool 是否暴露给模型；
- 该 Server 属于谁；
- 扩展卸载时是否删除；
- 是否显示在前端；
- 是否自动启用。

### 2. 协议对象不得成为业务真值

`MCPToolDescriptor` 只是协议发现结果，不代表 Tool 已启用、已注册、已授权、当前可执行或属于某个角色。

同理，`MCP Connection Ready` 只表示协议连接就绪，不代表所有 Tool 可用。

### 3. MCP Tool 不再进入 Skill 模型

目标链路：

```text
MCP tools/list
→ MCPToolDescriptor
→ MCP Tool Adapter
→ ToolDefinition
→ ToolRegistry
```

禁止：

```text
MCP tools/list
→ SkillDefinition
→ SkillRegistry
```

### 4. 所有 Tool 执行必须经过统一执行安全内核

目标链路：

```text
ToolExecutionRequest
→ ExecutionSecurityKernel
→ MCPRuntimeAdapter
→ MCP Protocol Client
→ tools/call
→ ToolResult
```

协议层不得绕过 Permission Broker、Scope Manager、Availability、统一审计、统一超时与取消、统一副作用和统一错误模型。

### 5. stdio 子进程不是 Client 本体

stdio Transport 可以使用子进程，但子进程生命周期由 Runtime Manager 或 MCP Connection Supervisor 管理；Transport 只负责读写；Client 只负责协议；Owner 来自 Resource Ownership；审计来自统一模型；权限来自 Permission Broker。

---

## 四、目标架构

建议结构：

```text
MCP Service Layer
├── MCPServerService
├── MCPConnectionService
├── MCPDiscoveryService
├── MCPToolAdapter
├── MCPDependencyService
└── MCPStatusService

MCP Protocol Infrastructure
├── protocol/
│   ├── message
│   ├── request
│   ├── response
│   ├── notification
│   ├── error
│   └── capability
├── client/
├── transport/
├── session/
├── auth/
├── features/
├── host/
├── reconnect/
└── diagnostics/
```

---

## 五、审查范围

重点审查：

```text
backend/internal/mcp/
backend/internal/mcpapi/
backend/internal/mcp/client/
backend/internal/mcp/transport/
backend/internal/mcp/auth/
backend/internal/mcp/discovery/
backend/internal/mcp/features/
backend/internal/mcp/tasks/
backend/internal/mcp/skill/
backend/internal/mcp/dependency/
```

同时审查：

```text
backend/cmd/server/services.go
backend/cmd/server/router.go
backend/internal/extension/runtime.go
backend/internal/extension/registry.go
front/src/views/mcp/
front/src/views/extensions/
electron/
```

---

## 六、协议消息模型

需要建立独立 MCP 协议消息类型。

```go
type JSONRPCRequest struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      RequestID       `json:"id"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      RequestID       `json:"id"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCNotification struct {
    JSONRPC string          `json:"jsonrpc"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}
```

要求：

- Request ID 类型统一；
- 支持字符串或数字 ID；
- 通知不得带 ID；
- 响应必须关联 Pending Request；
- 未知响应必须记录诊断；
- 非法 JSON-RPC 版本拒绝；
- 错误对象结构化；
- 大消息限制；
- 消息解码有深度限制；
- 不得将协议 Error 直接返回模型。

---

## 七、Request ID 与 Pending Request

建议建立：

```go
type RequestTracker interface {
    NextID() RequestID
    Register(id RequestID, pending PendingRequest) error
    Resolve(id RequestID, response JSONRPCResponse) bool
    Cancel(id RequestID, reason error) bool
    FailAll(reason error)
}
```

必须支持：

- 并发请求；
- 超时；
- 取消；
- 连接断开；
- 重连；
- 重复 Response；
- 未知 ID；
- Client Close；
- 服务端错误。

Pending Request 必须有 Context、Deadline、Method、Trace ID、Invocation ID、结果 Channel 和取消函数。

---

## 八、Client 生命周期

建议定义：

```go
type Client interface {
    Initialize(ctx context.Context, params InitializeParams) (InitializeResult, error)
    Request(ctx context.Context, method string, params any, result any) error
    Notify(ctx context.Context, method string, params any) error
    Close(ctx context.Context) error
    State() ClientState
    Capabilities() NegotiatedCapabilities
}
```

状态：

```text
created
connecting
initializing
ready
closing
closed
failed
```

要求：

- 未 Initialize 不得调用业务方法；
- Initialize 只能有明确状态迁移；
- 重复 Initialize 必须拒绝或幂等；
- Close 可重复调用；
- 连接断开时 Pending Request 全部结束；
- Client 不直接重连；
- Client 不直接注册 Tool；
- Client 不读取角色；
- Client 不写 Permission；
- Client 不拥有 Server Definition。

---

## 九、Initialize 与能力协商

必须实现 MCP 标准初始化链：

```text
Transport Open
→ initialize request
→ server capabilities
→ client capabilities
→ protocol version negotiation
→ initialized notification
→ ready
```

需要记录 Client 名称、Client 版本、Protocol Version、Server 名称、Server 版本、Server Capabilities、Client Capabilities、实验能力和兼容性警告。

不支持的协议版本必须明确失败并记录诊断，不得静默降级到未知行为。

---

## 十、Capability 模型

建议定义：

```go
type NegotiatedCapabilities struct {
    Tools        *ToolsCapability
    Resources    *ResourcesCapability
    Prompts      *PromptsCapability
    Logging      *LoggingCapability
    Completion   *CompletionCapability
    Tasks        *TasksCapability
    Sampling     *SamplingCapability
    Elicitation  *ElicitationCapability
    Roots        *RootsCapability
    Experimental map[string]json.RawMessage
}
```

必须区分 Server 声明、Client 声明、协商后实际可用、运行时暂时不可用和产品策略禁用。

---

## 十一、Transport 抽象

建议：

```go
type Transport interface {
    Open(ctx context.Context) error
    Send(ctx context.Context, message []byte) error
    Receive() <-chan TransportMessage
    Errors() <-chan error
    Close(ctx context.Context) error
    State() TransportState
}
```

Transport 不负责 JSON-RPC 业务解析、Capability、Tool 注册、权限、Scope、业务审计或重连策略，只负责可靠传递消息并暴露错误。

---

## 十二、stdio Transport

stdio Transport 必须支持：

- Command、Args、Environment、Working Directory；
- stdin、stdout、stderr；
- 进程启动与退出；
- 消息分帧；
- 取消和超时；
- 进程树关闭；
- 跨平台差异；
- 输出大小限制；
- stderr 日志脱敏。

### Windows

必须处理：

- 进程树终止；
- `cmd.exe` 与直接执行差异；
- 路径空格；
- 字符编码；
- 隐藏窗口；
- Job Object 或等价进程组管理；
- 退出后残留子进程。

### macOS/Linux

必须处理：

- Process Group；
- SIGTERM；
- SIGKILL；
- 工作目录；
- 权限位；
- Shell 注入防护；
- 僵尸进程回收。

禁止默认通过 Shell 拼接命令、将用户输入直接拼入 Command、在日志中打印 Secret Environment 或在 Client Close 后遗留进程。

---

## 十三、Streamable HTTP Transport

必须支持：

- HTTP Session；
- Request 与 Response；
- 协议要求的流式返回；
- Header；
- Authentication；
- 超时与取消；
- 连接关闭；
- Session ID；
- 服务端通知；
- 状态码；
- 内容类型；
- 重定向策略；
- 代理；
- TLS；
- 最大响应；
- 重试边界。

要求：

- URL 必须经过 Permission Broker 范围约束；
- Header Secret 通过 Secret Broker；
- 重定向不得扩大授权域名；
- TLS 错误不得静默忽略；
- Session 状态与 Transport 分离；
- HTTP 自动重试不得重复非幂等协议请求。

---

## 十四、Transport Factory

建议：

```go
type TransportFactory interface {
    Build(ctx context.Context, definition MCPConnectionDefinition) (Transport, error)
}
```

支持：

```text
stdio
streamable_http
```

未来可扩展，但不得在本步骤加入非标准私有 Transport。

Factory 只根据经过验证的 Connection Definition 构建 Transport。

---

## 十五、Session 模型

建议建立：

```go
type MCPSession struct {
    SessionID       string
    ServerID        string
    TransportType   string
    ProtocolVersion string
    ClientInfo      ImplementationInfo
    ServerInfo      ImplementationInfo
    Capabilities    NegotiatedCapabilities
    StartedAt       time.Time
    LastMessageAt   time.Time
    State           SessionState
}
```

Session 是运行时状态，不是 Server Definition。

应用重启后不恢复旧 Session，而是重新建立连接、生成新 Session，并将历史 Session 写入审计。

---

## 十六、认证边界

认证必须拆分为：

```text
Credential Reference
→ Auth Provider
→ Transport Request
```

不得在 MCP Server Definition 中长期保存 Secret 明文。

支持 OAuth、Bearer Token、Static Header Secret、Environment Secret 和无认证。

---

## 十七、OAuth 基础设施

建议定义：

```go
type OAuthProvider interface {
    BeginAuthorization(ctx context.Context, request OAuthAuthorizationRequest) (OAuthFlow, error)
    CompleteAuthorization(ctx context.Context, callback OAuthCallback) (CredentialReference, error)
    Refresh(ctx context.Context, credentialID string) error
    Revoke(ctx context.Context, credentialID string) error
}
```

必须支持 State、PKCE、Redirect URI、本地回调、Token Exchange、Refresh、Expiry、Revoke、并发登录、取消、错误、Secret Broker、日志脱敏和用户可见授权来源。

禁止 OAuth Token 写入普通日志、前端直接接触 Refresh Token、删除 MCP Server 时无条件删除共享 Credential、重定向到未授权地址或忽略 State 校验。

---

## 十八、Tools Feature

协议层 Tool API：

```go
type ToolsClient interface {
    ListTools(ctx context.Context, cursor string) (ListToolsResult, error)
    CallTool(ctx context.Context, name string, arguments json.RawMessage) (CallToolResult, error)
}
```

必须支持分页、空列表、Tool 列表变化通知、Input Schema、Output Schema、结构化结果、文本结果、Resource Link、错误、长任务、取消和结果大小限制。

协议层只返回 `MCPToolDescriptor` 和原始协议结果。

---

## 十九、MCP Tool Descriptor

建议：

```go
type MCPToolDescriptor struct {
    ServerID     string
    SessionID    string
    Name         string
    Description  string
    InputSchema  json.RawMessage
    OutputSchema json.RawMessage
    Annotations  map[string]any
    RevisionHash string
}
```

要求：

- Server ID 稳定；
- Session ID 仅用于诊断；
- Tool ID 由上层 Adapter 生成；
- Descriptor 不包含 Permission Grant；
- Descriptor 不包含角色 Binding；
- Descriptor 不直接进入模型；
- Revision Hash 用于变更检测。

---

## 二十、Resources Feature

至少支持：

```text
resources/list
resources/read
resources/templates/list
resources/subscribe
resources/unsubscribe
notifications/resources/list_changed
notifications/resources/updated
```

建议：

```go
type MCPResourceDescriptor struct {
    URI         string
    Name        string
    Description string
    MIMEType    string
    SizeHint    int64
    Metadata    map[string]any
}
```

要求：

- URI 校验；
- 内容大小限制；
- MIME 校验；
- 订阅生命周期；
- 通知去重；
- 缓存失效；
- 资源读取经过 Permission Broker；
- 不将全部 Resource 自动注入模型上下文。

---

## 二十一、Prompts Feature

至少支持：

```text
prompts/list
prompts/get
notifications/prompts/list_changed
```

Prompt Descriptor：

```go
type MCPPromptDescriptor struct {
    Name        string
    Description string
    Arguments   []MCPPromptArgument
}
```

要求：

- Prompt 内容属于外部不可信输入；
- 不得自动作为系统最高优先级提示词；
- 必须经过 Prompt 安全与作用域策略；
- Prompt 参数校验；
- 内容大小限制；
- 列表变化通知；
- 缓存失效。

---

## 二十二、Completion Feature

支持协议规定的 Completion。

要求：

- 明确 Capability；
- 参数校验；
- 结果数量限制；
- 超时；
- 取消；
- 不得绕过 Tool 执行链；
- Completion 只用于协议补全，不代表模型 Provider 补全。

---

## 二十三、Tasks Feature

MCP 长任务需要独立抽取。

```go
type MCPTaskDescriptor struct {
    TaskID     string
    ServerID   string
    SessionID  string
    Status     string
    CreatedAt  time.Time
    UpdatedAt  time.Time
    Metadata   map[string]any
}
```

必须支持创建、状态查询、结果获取、取消、超时、连接断开、应用重启、Session 变化、统一 Operation/Invocation 关联和重复查询限制。

MCP Task 状态不能替代统一 ExecutionStatus，应进行映射。

---

## 二十四、Sampling Host

当 MCP Server 请求 Sampling 时，必须经过 Host Boundary。

```text
MCP Server Sampling Request
→ MCP Host Adapter
→ Permission Broker
→ Scope Manager
→ Model Provider
→ Response
```

必须限制可用模型、Token、温度、上下文、角色数据、会话数据、系统 Prompt、成本、超时、调用频率和审计。

禁止 MCP Server 任意读取 Amitia 完整 Prompt 或 Secret。

---

## 二十五、Elicitation Host

MCP Elicitation 请求必须转为结构化用户交互。

必须支持请求内容、字段 Schema、用户确认、取消、超时、敏感字段、权限、审计和前端展示。

禁止在后台静默完成需要用户输入的 Elicitation、将 Password 等敏感信息回传普通日志或无 UI 时自动填充默认值并视为用户确认。

---

## 二十六、Roots Host

Roots 表示 MCP Server 可访问的宿主根资源。

必须通过 Permission Broker 和 Scope Manager 控制。

可暴露示例：

- Extension Storage Root；
- 用户明确授权目录；
- 项目目录；
- 临时工作区。

禁止默认暴露用户主目录、全部磁盘、Amitia Secret、数据库目录、其他扩展目录或聊天数据目录。

Roots 变化必须通知 Server，并写审计。

---

## 二十七、Logging Feature

MCP Server 日志必须结构化、分级、限流、脱敏并关联 Server、Session 与 Trace。

日志不能直接进入用户消息，不能伪装成 Tool Result，不能无限写入，也不能包含 Token 和 Secret。

---

## 二十八、Notifications

需要统一 Notification Router。

支持：

- Tool List Changed；
- Resource List Changed；
- Resource Updated；
- Prompt List Changed；
- Logging；
- Progress；
- Task；
- Cancel；
- 其他标准通知。

Notification Router 只负责协议分发，不直接修改 ToolRegistry。

上层服务订阅后决定重新 Discovery、失效 Cache、更新 Tool Adapter、通知前端和写审计。

---

## 二十九、Progress

协议 Progress 必须映射到统一 Runtime Event。

需要关联：

```text
trace_id
operation_id
invocation_id
request_id
progress_token
```

Progress 不得改变最终状态，必须限频、限制内容大小、脱敏，并处理未知 Invocation。

---

## 三十、取消

统一取消链：

```text
User/System Cancel
→ ExecutionSecurityKernel
→ MCPRuntimeAdapter
→ MCP Client Cancel
→ Protocol Notification/Context Cancel
→ Pending Request End
```

必须确保取消不会只修改前端状态，Context 能传播，Pending Request 能释放，Task 按协议取消，单次 Request 取消不会无条件终止整个子进程，并区分超时与取消错误。

---

## 三十一、协议错误模型

建议：

```go
type MCPProtocolError struct {
    Code      string
    Message   string
    Data      json.RawMessage
    Retryable bool
    Category  string
}
```

分类：

```text
transport
protocol
authentication
initialization
capability
request
timeout
cancelled
server
invalid_response
session
```

之后由 MCPRuntimeAdapter 映射为统一 ToolError。

不得将协议内部错误直接暴露给模型或用户。

---

## 三十二、连接监督与重连

重连不应由 Client 内部隐式完成，建议抽取：

```go
type MCPConnectionSupervisor interface {
    Connect(ctx context.Context, serverID string) error
    Disconnect(ctx context.Context, serverID string) error
    Reconnect(ctx context.Context, serverID string) error
    State(serverID string) MCPConnectionState
}
```

状态：

```text
disconnected
connecting
initializing
ready
degraded
reconnecting
failed
disabled
```

---

## 三十三、重连策略

建议：

```go
type ReconnectPolicy struct {
    Enabled      bool
    MaxAttempts  int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Jitter       float64
}
```

重连必须考虑 Server Enabled、Extension/Module Enabled、Owner 是否存在、权限是否有效、凭据是否有效、应用是否关闭、用户是否手动断开、熔断和平台支持。

用户手动断开后不得自动重连，除非明确选择。

---

## 三十四、重连后的同步

重连成功后：

```text
Initialize
→ Capability Compare
→ Discovery Refresh
→ Descriptor Diff
→ Notify MCP Service Layer
→ Tool Adapter Update
→ ToolRegistry Atomic Replace
→ Cache Invalidation
→ Audit
```

禁止直接重复 Register、旧 Tool 不清理、模型名称漂移、Tool ID 因 Session 变化、角色 Scope 被重置或 Permission Grant 被重建。

---

## 三十五、Discovery 服务边界

Discovery Service 属于产品服务层，但依赖协议基础设施。

职责：

- 调用 List Tools/Resources/Prompts；
- 分页；
- Descriptor Hash；
- 差异；
- 缓存；
- 持久快照；
- 通知上层；
- 处理部分失败。

不负责直接执行 Tool、权限授权、角色绑定、Extension 所有权或安装 Package。

---

## 三十六、MCP Tool Adapter

Adapter 负责：

```text
MCPToolDescriptor
→ ToolDefinition
```

必须映射稳定 Tool ID、模型名称、Input Schema、Output Schema、RuntimeBinding、Source、Owner、Risk、SideEffect、Availability Dependency 和 Revision Hash。

不得映射 Permission Grant、Scope Binding、当前连接 Session 或前端状态。

---

## 三十七、稳定 Tool ID

建议：

```text
mcp/<server-id>/<tool-name>
```

要求：

- Server ID 稳定；
- Tool Name 规范化；
- Session 重建不变化；
- 重连不变化；
- 角色变化不变化；
- 显示名称变化不变化；
- 名称冲突明确拒绝或稳定编码；
- Server 重新创建但属于新资源时使用新 ID。

---

## 三十八、MCP Server Definition

协议基础设施使用经过产品层验证的 Connection Definition。

```go
type MCPConnectionDefinition struct {
    ServerID        string
    TransportType   string
    Command         string
    Args            []string
    WorkingDir      string
    URL             string
    CredentialRefs  []string
    HeaderRefs      map[string]string
    EnvironmentRefs map[string]string
    TimeoutPolicy   MCPTimeoutPolicy
    ReconnectPolicy ReconnectPolicy
}
```

要求：

- Secret 只用引用；
- Command 与 Args 分离；
- URL 结构化验证；
- WorkingDir 经过 Roots/Permission；
- 不可包含角色或 Tool Enabled；
- 不可包含 Extension UI 状态。

---

## 三十九、Secret Broker 接入

所有凭据必须从 Secret Broker 获取。

协议基础设施只能在需要时请求 Credential Reference、Header Secret、Environment Secret 和 OAuth Token。

要求：

- 最小生命周期；
- 内存清理；
- 日志脱敏；
- 错误不泄露；
- Transport Close 后释放引用；
- 共享 Credential 删除需引用检查。

---

## 四十、Resource Ownership 接入

需要登记：

### MCP Server

```text
resource_type=mcp_server
```

### MCP Tool

```text
resource_type=mcp_tool
owner=MCP Server
```

### stdio Process

```text
resource_type=process
owner_resource=MCP Server
runtime_manager=MCP Connection Supervisor
```

### HTTP Connection

```text
resource_type=connection
owner_resource=MCP Server
```

### OAuth Credential

```text
resource_type=secret
owner=user/extension/shared
```

协议 Client 本身是临时运行时对象，不作为长期业务资源。

---

## 四十一、统一审计接入

必须记录：

- Connect；
- Initialize；
- Protocol Version；
- Capability；
- Discovery；
- Descriptor Diff；
- Tool Call；
- Resource Read；
- Prompt Get；
- Task；
- Sampling；
- Elicitation；
- Roots；
- Reconnect；
- Disconnect；
- Authentication Error；
- Protocol Error；
- 子进程异常；
- 残留进程清理。

审计不记录完整协议帧，调试日志可在受限模式记录脱敏摘要。

---

## 四十二、前端状态模型

前端 MCP 页面应区分：

### Definition State

- 已创建；
- 已启用；
- Owner；
- Scope；
- Transport；
- 配置。

### Connection State

- Disconnected；
- Connecting；
- Ready；
- Degraded；
- Reconnecting；
- Failed。

### Protocol State

- Protocol Version；
- Capabilities；
- Session；
- Last Error。

### Contribution State

- Tool 数量；
- Resource 数量；
- Prompt 数量；
- 是否已同步。

### Tool State

来自统一 Tool 状态，不由 MCP 页面自行拼接。

---

## 四十三、API 边界

建议拆分：

```text
/api/mcp/servers
/api/mcp/servers/:id/connection
/api/mcp/servers/:id/capabilities
/api/mcp/servers/:id/discovery
/api/mcp/servers/:id/tools
/api/mcp/servers/:id/resources
/api/mcp/servers/:id/prompts
/api/mcp/servers/:id/tasks
/api/mcp/servers/:id/diagnostics
```

Tool 启用、Scope 和权限应通过统一 Extension Kernel API 管理，而不是继续由 MCP API 直接修改 Registry。

---

## 四十四、旧 Skill Runtime 处理

重点对象：

```text
backend/internal/mcp/skill/runtime.go
```

处理原则：

- 禁止继续新增功能；
- 将 Tool Descriptor 转换逻辑迁入 MCP Tool Adapter；
- 将 Tool Call 迁入 MCPRuntimeAdapter；
- 将 Registry 写入迁入 ToolRegistry 服务层；
- 保留临时只读兼容；
- 记录调用次数；
- 后续步骤删除。

---

## 四十五、旧 Manager 处理

当前 MCP Manager 如果同时承担 Repository、Connection、Discovery、Tool 注册、重连、状态和 API，应拆分为：

```text
MCPServerService
MCPConnectionSupervisor
MCPDiscoveryService
MCPProtocolClientFactory
MCPStatusService
```

本步骤只完成协议相关拆分和调用边界，不要求一次重写所有产品 Service。

---

## 四十六、兼容层约束

迁移期间允许：

```text
旧 Manager.Connect
→ MCPConnectionSupervisor.Connect
```

允许：

```text
旧 Tool Call
→ ToolExecutionRequest
→ MCPRuntimeAdapter
```

禁止：

```text
新 MCP Protocol Client
→ 旧 Skill Runtime
```

禁止新旧 Client 并行连接同一 Server。

---

## 四十七、测试要求

必须新增以下测试。

### 1. JSON-RPC

- Request；
- Response；
- Notification；
- Error；
- 未知 ID；
- 重复 ID；
- 非法版本；
- 大消息；
- 部分消息；
- 并发。

### 2. Client

- Initialize；
- 重复 Initialize；
- Ready；
- Close；
- 断线；
- Pending Request；
- 取消；
- 超时。

### 3. stdio Transport

- 正常进程；
- 命令不存在；
- 非法输出；
- stderr；
- 进程退出；
- 父进程关闭；
- 残留进程；
- Windows；
- macOS；
- Linux。

### 4. HTTP Transport

- Session；
- Header；
- OAuth；
- TLS；
- 重定向；
- 流式响应；
- 超时；
- 断线；
- 状态码；
- 响应大小。

### 5. Capability Negotiation

- 完整；
- 部分；
- 未知；
- 不兼容版本；
- 实验能力。

### 6. Tools

- List；
- 分页；
- Call；
- 结构化结果；
- 错误；
- 通知；
- 变更。

### 7. Resources

- List；
- Read；
- Template；
- Subscribe；
- Updated；
- URI 安全；
- 大小限制。

### 8. Prompts

- List；
- Get；
- Arguments；
- 不可信内容；
- 更新通知。

### 9. Completion

- 正常；
- 超限；
- 不支持。

### 10. Tasks

- 创建；
- 查询；
- 完成；
- 失败；
- 取消；
- 断线；
- 重连；
- 应用重启。

### 11. Sampling

- Permission；
- Scope；
- Token；
- 模型限制；
- 超时；
- 拒绝。

### 12. Elicitation

- 用户确认；
- 取消；
- 超时；
- 敏感字段；
- 无 UI。

### 13. Roots

- 授权目录；
- 越权；
- 更新；
- 撤销。

### 14. Reconnect

- Backoff；
- Jitter；
- 手动断开；
- 禁用；
- 凭据过期；
- Tool Diff；
- 无重复注册。

### 15. Tool Adapter

- 稳定 ID；
- Schema；
- RuntimeBinding；
- Owner；
- Revision。

### 16. Security

- Secret 脱敏；
- Command 注入；
- URL 范围；
- Redirect；
- Root 越权；
- 大响应；
- 协议帧污染。

---

## 四十八、实施任务

### Task 1：定义 MCP 协议边界

明确 Protocol Infrastructure 与 Product Service 的职责。

### Task 2：抽取 JSON-RPC 模型

统一 Request、Response、Notification、Error 和 RequestTracker。

### Task 3：抽取 MCP Client

实现明确状态机与 Capability Negotiation。

### Task 4：抽取 Transport 接口

统一 stdio 与 Streamable HTTP。

### Task 5：重构 stdio Transport

分离进程监督与消息传输。

### Task 6：重构 HTTP Transport

统一 Session、Header、OAuth 和取消。

### Task 7：抽取 Auth Provider

接入 Secret Broker 和 OAuth。

### Task 8：抽取 Tools Feature

返回 MCPToolDescriptor，不注册 Skill。

### Task 9：抽取 Resources Feature

实现列表、读取、模板和订阅。

### Task 10：抽取 Prompts Feature

实现列表、获取和更新。

### Task 11：抽取 Completion

统一协议补全。

### Task 12：抽取 Tasks

统一长任务状态与取消。

### Task 13：抽取 Sampling Host Boundary

接入 Permission、Scope、Model Provider 和 Audit。

### Task 14：抽取 Elicitation Host Boundary

接入统一用户交互。

### Task 15：抽取 Roots Host Boundary

只暴露授权 Roots。

### Task 16：抽取 Notification Router

分发标准通知。

### Task 17：抽取 Progress 与 Cancel

映射统一 Invocation。

### Task 18：建立 MCPConnectionSupervisor

负责连接、断开、重连和健康。

### Task 19：建立 MCPDiscoveryService

支持分页、Hash、Diff 和缓存。

### Task 20：建立 MCPToolAdapter

转换为 ToolDefinition。

### Task 21：建立 MCPRuntimeAdapter

统一 Tool Call 执行入口。

### Task 22：接入 Resource Ownership

登记 Server、Tool、Process、Connection 和 Credential。

### Task 23：接入统一 Audit

替换分散 MCP Operation 日志。

### Task 24：调整 API 边界

禁止 MCP API 直接写旧 Registry。

### Task 25：迁移旧 MCP Skill Runtime

停止新增写入并统计调用。

### Task 26：完成跨平台与故障注入测试

覆盖协议、网络和进程异常。

---

## 四十九、建议目录结构

建议：

```text
backend/internal/mcp/
├── protocol/
│   ├── message.go
│   ├── request_id.go
│   ├── error.go
│   ├── capability.go
│   └── version.go
├── client/
│   ├── client.go
│   ├── state.go
│   ├── pending.go
│   └── initialize.go
├── transport/
│   ├── transport.go
│   ├── stdio.go
│   ├── http.go
│   ├── session.go
│   └── factory.go
├── auth/
│   ├── provider.go
│   ├── oauth.go
│   └── credentials.go
├── features/
│   ├── tools.go
│   ├── resources.go
│   ├── prompts.go
│   ├── completion.go
│   ├── tasks.go
│   ├── sampling.go
│   ├── elicitation.go
│   └── roots.go
├── host/
│   ├── sampling_host.go
│   ├── elicitation_host.go
│   └── roots_host.go
├── notifications/
│   ├── router.go
│   └── progress.go
├── connection/
│   ├── supervisor.go
│   ├── reconnect.go
│   └── health.go
├── discovery/
│   ├── service.go
│   ├── diff.go
│   └── cache.go
└── adapters/
    ├── tool_adapter.go
    └── runtime_adapter.go
```

目录仅为建议，不得为了目录形式重写无关代码。

---

## 五十、性能要求

建议：

- JSON-RPC Pending Request 并发安全；
- 消息读取不阻塞 Tool 执行线程；
- 大响应流式限制；
- Tool Discovery 支持分页；
- Descriptor Diff 使用 Hash；
- 重连不产生风暴；
- 通知有界队列；
- Progress 限流；
- Client Close 可及时释放；
- 一个 Server 的慢响应不阻塞其他 Server；
- stdio stderr 不阻塞 stdout；
- HTTP 连接池受控；
- 协议日志默认不保存完整帧。

---

## 五十一、风险控制

### P0：协议与安全错误

- Secret 泄露；
- Roots 越权；
- Sampling 获取完整宿主 Prompt；
- Command 注入；
- 取消不生效；
- 重定向扩大网络权限。

### P1：状态与注册错误

- 重连重复 Tool；
- 旧 Tool 不清理；
- Session 变化导致 Tool ID 变化；
- Client Ready 与 Tool 可用混淆；
- 旧 Skill Runtime 继续写入。

### P2：兼容问题

- 某 MCP Server 不符合严格协议；
- 旧 HTTP Session 行为变化；
- Tasks 映射不一致；
- Prompt 内容处理变化。

### P3：性能问题

- 大量通知；
- Discovery 全量刷新；
- 重连风暴；
- 大 Resource 占用内存；
- 日志过量。

---

## 五十二、本步骤不做的事情

本步骤明确不做：

- 不重建完整 MCP 前端；
- 不完成所有旧 MCP 数据表迁移；
- 不删除旧 MCP Skill Runtime；
- 不删除旧 Manager；
- 不实现 `.amitiax` v2 MCP Manifest；
- 不实现第三方插件 Runtime；
- 不实现 UI Contribution；
- 不实现 MCP Server 市场；
- 不新增私有 MCP 协议扩展；
- 不实现移动端；
- 不自动安装任意 MCP Server；
- 不允许协议层绕过 Permission、Scope 或 Audit。

---

## 五十三、验收产物

完成后必须提交：

### 1. MCP 协议基础设施主文档

```text
docs/extension-kernel/14-mcp-protocol-infrastructure.md
```

### 2. 独立 JSON-RPC 与 Client

包含 Request、Response、Notification、Error、Request Tracker、状态机、Initialize 和 Capability Negotiation。

### 3. Transport 抽象

至少支持 stdio 和 Streamable HTTP。

### 4. Auth 基础设施

包含 OAuth、Secret Reference、Token Refresh、Revoke 和脱敏。

### 5. Feature Clients

至少包含 Tools、Resources、Prompts、Completion、Tasks、Sampling、Elicitation 和 Roots。

### 6. Notification Router

支持列表变化、Resource Update、Progress、Logging 和 Task 通知。

### 7. MCPConnectionSupervisor

统一连接、断开、重连与健康。

### 8. MCPDiscoveryService

支持分页、Hash、Diff 和缓存。

### 9. MCPToolAdapter

MCP Tool 可转换为 ToolDefinition。

### 10. MCPRuntimeAdapter

Tool 调用经过 Execution Security Kernel。

### 11. Resource Ownership 接入

Server、Tool、Process、Connection、Credential 已登记。

### 12. Audit 接入

MCP 生命周期和协议操作可统一追踪。

### 13. 旧链路迁移报告

列出：

- 仍使用 MCP Skill Runtime 的入口；
- 仍直接写旧 Registry 的入口；
- 仍直接调用 `tools/call` 的入口；
- 仍使用旧 OAuth/Secret 的入口；
- 仍由旧 Manager 独占的职责。

### 14. 测试报告

覆盖 JSON-RPC、Client、Transport、Auth、Tools、Resources、Prompts、Tasks、Sampling、Elicitation、Roots、Reconnect、Tool Adapter、跨平台、安全和故障注入。

---

## 五十四、验收标准

本步骤通过必须满足：

1. MCP 协议层与产品层已分离。
2. Client 不再注册 Tool。
3. Transport 不再处理业务权限。
4. MCP Tool 不再新增 SkillDefinition。
5. MCP Tool 可直接转换为 ToolDefinition。
6. MCP Tool 调用经过 Execution Security Kernel。
7. stdio 与 HTTP 使用统一 Transport 接口。
8. OAuth 与 Secret 已分离。
9. Capability Negotiation 明确。
10. Tools、Resources、Prompts、Tasks、Sampling、Elicitation、Roots 已抽取。
11. Host Callback 经过 Permission、Scope 和 Audit。
12. 重连后 Tool 使用 Diff 更新，不重复注册。
13. Session 变化不影响稳定 Tool ID。
14. MCP Server、Tool、Process、Connection 和 Credential 已纳入资源所有权。
15. 前端状态能够区分 Definition、Connection、Protocol 和 Tool State。
16. 新 MCP 代码不依赖旧 Skill Runtime。
17. 旧链路有调用统计和删除计划。
18. 跨平台和故障注入测试通过。
19. 后续第 15 步可以在此基础上重构 Agent Skill Loader。

---

## 五十五、退出条件

只有满足以下条件后，才能进入第 15 步“重构 Agent Skill Loader”：

- JSON-RPC 与 MCP Client 已独立；
- stdio 与 HTTP Transport 已独立；
- Auth 与 Secret 已独立；
- Feature Clients 已抽取；
- MCPConnectionSupervisor 已落地；
- MCPDiscoveryService 已落地；
- MCPToolAdapter 已落地；
- MCPRuntimeAdapter 已落地；
- 新 Tool 不再写旧 Skill Registry；
- 重连无重复注册；
- 旧 MCP Skill Runtime 只剩迁移用途；
- 关键测试通过；
- 没有新增协议与产品层反向依赖。

---

## 五十六、执行约束

执行本步骤时必须遵守：

> MCP 协议基础设施只负责正确实现 MCP，不负责替 Amitia 决定权限、作用域、所有权、安装、启用或模型暴露。

禁止出现：

- MCP Client 直接调用 ToolRegistry；
- MCP Transport 读取角色；
- MCP Discovery 写 Permission Grant；
- MCP API 直接修改 Scope；
- MCP Server Ready 自动全局暴露 Tool；
- Sampling 读取完整 Amitia 系统 Prompt；
- Roots 默认暴露用户主目录；
- 重连后直接追加注册 Tool；
- Session ID 进入稳定 Tool ID；
- 新代码继续依赖 `mcp/skill/runtime.go`；
- 协议日志记录 Secret；
- Client 内部隐式无限重连。

本步骤完成后，Amitia 必须具备一套独立、标准、可测试、可重用、可监督、可安全接入统一 Tool 模型的 MCP 协议基础设施。
