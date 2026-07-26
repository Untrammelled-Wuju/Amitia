# MCP Protocol Infrastructure (Step 14)

## Architecture Overview

The MCP protocol infrastructure has been extracted into a clean layered architecture separating protocol concerns from product concerns.

### Protocol Infrastructure Layer

```
backend/internal/mcp/
├── protocol/          # JSON-RPC 2.0 message types, errors, version negotiation
├── client/            # MCP Connection with state machine, request tracking
├── transport/         # Transport abstraction (stdio, Streamable HTTP)
├── auth/              # OAuth provider, token store, credential management
├── features/          # Tools, Resources, Prompts, Completion feature clients
├── host/              # Sampling, Elicitation, Roots host boundaries
├── discovery/         # Tool/Resource/Prompt discovery with pagination and hashing
├── manager/           # Connection supervision, reconnect, factory
├── dependency/        # MCP dependency management for agent skills
└── skill/             # Legacy skill runtime (frozen, migration only)
```

### Extension Kernel Integration

```
backend/internal/extension/kernel/capability/
├── adapter_mcp.go          # MCPRuntimeAdapter - executes MCP tools through security kernel
├── adapter_mcp_tool.go     # MCPToolAdapter - converts MCPToolDescriptor to ToolDefinition
├── executor.go             # DefaultToolExecutor with Availability check → RuntimeAdapter
└── runtime_adapter.go      # RuntimeAdapter interface (MCP implements this)
```

### Resource Ownership Integration

```
backend/internal/mcp/
└── resource_ownership.go   # MCPResourceIntegration - registers Server, Tool, Process, Connection
```

## Layer Separation

### Protocol Layer Responsibilities
- JSON-RPC message encoding/decoding
- Client state machine (disconnected → connecting → initializing → ready)
- Capability negotiation
- Transport management (stdio process, HTTP session)
- OAuth flow and token management
- Feature discovery (tools/list, resources/list, prompts/list)
- Notification routing
- Host callback boundaries (sampling, elicitation, roots)
- Reconnection with backoff/jitter strategy
- Progress and cancel propagation

### Protocol Layer Does NOT
- Register tools into any registry
- Determine tool permissions or scope
- Decide role bindings
- Manage extension lifecycle
- Handle UI state
- Write audit logs directly

### Product Layer (MCP Manager → Extension Kernel)
- Server definitions and configuration
- Connection supervision delegation
- Discovery result aggregation
- Tool adapter bridge (MCPToolDescriptor → ToolDefinition)
- Runtime adapter bridge (ToolExecutionRequest → MCP tools/call)
- Resource ownership registration
- Audit event emission

## Key Components

### MCPToolAdapter
Located at `kernel/capability/adapter_mcp_tool.go`

Converts MCP-discovered tool information into kernel `ToolDefinition`:
- Stable capability ID: `mcp/<server-id>/<tool-name>`
- Model name derived from capability ID
- Runtime binding: `RuntimeTypeMCP` with server ID and tool name
- Risk classification from MCP annotations (readOnly, destructive, openWorld)
- Side effect classification
- Revision hashing for change detection
- Batch operations with Diff support for atomic sync

### MCPRuntimeAdapter
Located at `kernel/capability/adapter_mcp.go`

Execution path:
```
ToolExecutionRequest → DefaultToolExecutor
  → AvailabilityEvaluator.Evaluate (permission check)
  → RuntimeAdapterRegistry.Resolve (MCP matches RuntimeTypeMCP)
  → MCPRuntimeAdapter.Execute
    → MCPCallFunc (delegates to MCP client tools/call)
    → UnifiedToolResult
```

### MCPResourceIntegration
Located at `mcp/resource_ownership.go`

Registers MCP lifecycle resources:
- `mcp_server` - registered on connect
- `mcp_tool` - registered on discovery with `contains` reference to server
- `process` - registered on stdio start with `owned_by` reference to server
- `connection` - registered on HTTP session establish

## Test Coverage

All 11 test packages pass:

| Package | Status |
|---------|--------|
| `mcp` (root) | ✅ |
| `mcp/auth` | ✅ |
| `mcp/client` | ✅ |
| `mcp/dependency` | ✅ |
| `mcp/discovery` | ✅ |
| `mcp/features` | ✅ |
| `mcp/host` | ✅ |
| `mcp/manager` | ✅ |
| `mcp/protocol` | ✅ |
| `mcp/skill` | ✅ |
| `mcp/transport` | ✅ |

Capability package tests also pass:
- `kernel/capability` ✅ (includes MCPRuntimeAdapter, MCPToolAdapter)

## Migration Status

### Clean Protocol Layer
All protocol components are independent:
- No MCP Client writes to old Registry
- No Transport reads roles or permissions
- No Discovery writes Permission Grants
- OAuth uses Secret Broker references
- Capability negotiation is explicit

### Legacy Bridges (frozen, migration only)
- `mcp/skill/runtime.go` - wraps MCP tools as SkillDefinition (marked Deprecated, used only for backward compatibility)
- `mcp/model.go` - old data models (Server, ToolDefinition, etc.) retained for existing storage
- `mcp/repository.go` - old repository retained for existing API surfaces

### Migration Path
1. Discovery continues producing descriptors with stable hashing
2. MCPToolAdapter converts to kernel ToolDefinition for new consumers
3. Old skill runtime remains for existing consumers until Step 67
4. Old API endpoints remain until Extension Kernel becomes sole entry point

## Exit Conditions

- [x] JSON-RPC and MCP Client independent
- [x] stdio and HTTP Transport independent
- [x] Auth and Secret independent
- [x] Feature Clients extracted (Tools, Resources, Prompts)
- [x] MCPConnectionSupervisor in place
- [x] MCPDiscoveryService in place (with pagination, hashing, diff)
- [x] MCPToolAdapter in place (MCPToolDescriptor → ToolDefinition)
- [x] MCPRuntimeAdapter in place (via Execution Security Kernel)
- [x] New Tools don't write to old Skill Registry
- [x] Reconnect uses hash-based diff (no duplicate registration)
- [x] Old MCP Skill Runtime frozen (migration only)
- [x] All tests pass
- [x] No new reverse dependencies between protocol and product layers
