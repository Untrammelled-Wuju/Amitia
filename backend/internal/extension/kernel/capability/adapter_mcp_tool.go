package capability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

var mcpIdentifierPattern = regexp.MustCompile(`[^a-z0-9_]+`)

type MCPToolDescriptor struct {
	ServerID     string
	ServerName   string
	Name         string
	Title        string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
	Annotations  map[string]any
	RevisionHash string
	ExtensionID  string
	ModuleID     string
}

type MCPResourceDescriptor struct {
	ServerID    string
	URI         string
	Name        string
	Title       string
	Description string
	MIMEType    string
	SizeHint    int64
	Metadata    map[string]any
}

type MCPPromptDescriptor struct {
	ServerID    string
	Name        string
	Title       string
	Description string
	Arguments   json.RawMessage
}

type MCPToolAdapter struct{}

func NewMCPToolAdapter() *MCPToolAdapter {
	return &MCPToolAdapter{}
}

func (a *MCPToolAdapter) AdaptTool(descriptor MCPToolDescriptor) ToolDefinition {
	capabilityID := BuildMCPCapabilityID(descriptor.ServerID, descriptor.Name)
	modelName := ModelNameFromCapabilityID(capabilityID)

	source := CapabilitySourceMCP
	risk := classifyMCPRisk(descriptor.Annotations)
	sideEffect := classifyMCPSideEffect(descriptor.Annotations)
	hasSideEffects := sideEffect != SideEffectNone
	idempotent := false
	if hints := descriptor.Annotations; hints != nil {
		if v, ok := hints["idempotentHint"].(bool); ok {
			idempotent = v
		}
	}

	metadata := map[string]any{
		"mcpServerId":   descriptor.ServerID,
		"mcpServerName": descriptor.ServerName,
		"mcpToolName":   descriptor.Name,
		"revisionHash":  descriptor.RevisionHash,
	}
	if descriptor.Annotations != nil {
		metadata["annotations"] = descriptor.Annotations
	}

	computedHash := computeMCPToolHash(descriptor)

	return ToolDefinition{
		ID:          string(capabilityID),
		ModelName:   modelName,
		Source:      CapabilitySourceToToolSource(source),
		Name:        descriptor.Title,
		Description: descriptor.Description,
		Version:     computedHash,
		InputSchema: descriptor.InputSchema,
		OutputSchema: func() json.RawMessage {
			if len(descriptor.OutputSchema) > 0 {
				return descriptor.OutputSchema
			}
			return json.RawMessage("{}")
		}(),
		RiskLevel:      risk,
		SideEffect:     sideEffect,
		HasSideEffects: hasSideEffects,
		Idempotent:     idempotent,
		Retryable:      idempotent,
		TimeoutMS:      30000,
		Enabled:        false,
		Compatible:     true,
		Internal:       false,
		Scope: ScopeRule{
			Type: "",
			ID:   "",
		},
		Metadata: metadata,
		ToolVersion: ToolVersion{
			SchemaVersion: 1,
			Revision:      computedHash,
		},
		State: ToolState{
			Installed:         true,
			ModuleEnabled:     false,
			CapabilityEnabled: false,
			ScopeAllowed:      true,
			PermissionGranted: true,
			RuntimeReady:      true,
			DependencyReady:   true,
			Health:            HealthUnknown,
		},
		ModelExposure: ModelExposureRule{
			ExposedByDefault:   false,
			RequiresActivation: true,
			MaxPromptTokens:    2000,
			Priority:           10,
		},
		ExecutionPolicy: ToolExecutionPolicy{
			Timeout:        30000000000,
			MaxConcurrency: 10,
			RetryPolicy: RetryPolicy{
				MaxRetries:  0,
				BackoffBase: 1000000000,
			},
			Idempotent:       idempotent,
			ApprovalRequired: false,
			AllowBackground:  false,
			MaxDepth:         1,
		},
		ResultPolicy: ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 1048576,
		},
		Runtime: RuntimeBinding{
			RuntimeType: RuntimeTypeMCP,
			RuntimeID:   descriptor.ServerID,
			HandlerName: descriptor.Name,
			Metadata: map[string]any{
				"mcpServerId":   descriptor.ServerID,
				"mcpServerName": descriptor.ServerName,
			},
		},
		ExtensionID: descriptor.ExtensionID,
		ModuleID:    descriptor.ModuleID,
	}
}

func BuildMCPCapabilityID(serverID, toolName string) CapabilityID {
	cap := "mcp/" + normalizeMCPSegment(serverID) + "/" + normalizeMCPSegment(toolName)
	return CapabilityID(cap)
}

func BuildMCPUniqueKey(serverID, toolName, extensionID, moduleID string) string {
	return serverID + "|" + toolName + "|" + extensionID + "|" + moduleID
}

func normalizeMCPSegment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = mcpIdentifierPattern.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	value = strings.ReplaceAll(value, "__", "_")
	if value == "" {
		return "unnamed"
	}
	return value
}

func classifyMCPRisk(annotations map[string]any) RiskLevel {
	if annotations == nil {
		return RiskMedium
	}
	readOnly, _ := annotations["readOnlyHint"].(bool)
	destructive, _ := annotations["destructiveHint"].(bool)
	openWorld, _ := annotations["openWorldHint"].(bool)

	if destructive || openWorld {
		return RiskHigh
	}
	if readOnly {
		return RiskLow
	}
	return RiskMedium
}

func classifyMCPSideEffect(annotations map[string]any) SideEffectLevel {
	if annotations == nil {
		return SideEffectWrite
	}
	readOnly, _ := annotations["readOnlyHint"].(bool)
	destructive, _ := annotations["destructiveHint"].(bool)

	if destructive {
		return SideEffectDestructive
	}
	if readOnly {
		return SideEffectNone
	}
	return SideEffectWrite
}

func computeMCPToolHash(descriptor MCPToolDescriptor) string {
	payload := map[string]any{
		"serverId":    descriptor.ServerID,
		"name":        descriptor.Name,
		"description": descriptor.Description,
	}
	if len(descriptor.InputSchema) > 0 {
		payload["inputSchema"] = json.RawMessage(descriptor.InputSchema)
	}
	if len(descriptor.OutputSchema) > 0 {
		payload["outputSchema"] = json.RawMessage(descriptor.OutputSchema)
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:16])
}

type MCPToolAdapterBatch struct {
	adapter *MCPToolAdapter
	results []ToolDefinition
}

func (a *MCPToolAdapter) NewBatch() *MCPToolAdapterBatch {
	return &MCPToolAdapterBatch{adapter: a, results: make([]ToolDefinition, 0)}
}

func (b *MCPToolAdapterBatch) Add(descriptor MCPToolDescriptor) {
	b.results = append(b.results, b.adapter.AdaptTool(descriptor))
}

func (b *MCPToolAdapterBatch) Tools() []ToolDefinition {
	return b.results
}

func (b *MCPToolAdapterBatch) Diff(previous map[string]string) (added, updated, removed []ToolDefinition) {
	current := make(map[string]string, len(b.results))
	for _, tool := range b.results {
		current[tool.ID] = tool.Version
	}

	for _, tool := range b.results {
		prevHash, existed := previous[tool.ID]
		if !existed {
			added = append(added, tool)
		} else if prevHash != tool.Version {
			updated = append(updated, tool)
		}
	}

	for prevID := range previous {
		if _, stillExists := current[prevID]; !stillExists {
			removed = append(removed, ToolDefinition{
				ID:      prevID,
				Enabled: false,
				State: ToolState{
					Installed: false,
				},
			})
		}
	}

	return added, updated, removed
}

var globalMCPToolAdapter *MCPToolAdapter
var globalMCPToolAdapterMu sync.Mutex

func GetGlobalMCPToolAdapter() *MCPToolAdapter {
	globalMCPToolAdapterMu.Lock()
	defer globalMCPToolAdapterMu.Unlock()
	if globalMCPToolAdapter == nil {
		globalMCPToolAdapter = NewMCPToolAdapter()
	}
	return globalMCPToolAdapter
}

type mcpServerRegistry struct {
	mu      sync.Mutex
	servers map[string]mcpServerEntry
}

type mcpServerEntry struct {
	Definition  json.RawMessage
	ExtensionID string
	Tools       []MCPToolDescriptor
}

var globalMCPServers = &mcpServerRegistry{
	servers: make(map[string]mcpServerEntry),
}

func (a *MCPToolAdapter) RegisterServerWithDefinition(ctx context.Context, serverID string, defData json.RawMessage, extensionID string) error {
	globalMCPServers.mu.Lock()
	defer globalMCPServers.mu.Unlock()
	globalMCPServers.servers[serverID] = mcpServerEntry{
		Definition:  defData,
		ExtensionID: extensionID,
	}
	return nil
}

func (a *MCPToolAdapter) UnregisterServer(ctx context.Context, serverID string) error {
	globalMCPServers.mu.Lock()
	defer globalMCPServers.mu.Unlock()
	delete(globalMCPServers.servers, serverID)
	return nil
}
