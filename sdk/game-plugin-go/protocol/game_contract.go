package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/netip"
	"path"
	"sort"
	"strings"
	"time"
)

// PluginSession/PluginOperation are transport-neutral helper payloads. GameHost
// does not interpret their payloads or assign game-specific semantics to them.
type PluginSessionStatus string

const (
	PluginSessionCreated    PluginSessionStatus = "created"
	PluginSessionConnecting PluginSessionStatus = "connecting"
	PluginSessionReady      PluginSessionStatus = "ready"
	PluginSessionPaused     PluginSessionStatus = "paused"
	PluginSessionClosed     PluginSessionStatus = "closed"
	PluginSessionFailed     PluginSessionStatus = "failed"
)

type PluginSessionOpenRequest struct {
	Context  map[string]any  `json:"context,omitempty"`
	Metadata map[string]any  `json:"metadata,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

type PluginSession struct {
	ID        string              `json:"id"`
	Status    PluginSessionStatus `json:"status"`
	StartedAt time.Time           `json:"startedAt,omitempty"`
	UpdatedAt time.Time           `json:"updatedAt,omitempty"`
	Metadata  map[string]any      `json:"metadata,omitempty"`
	Payload   json.RawMessage     `json:"payload,omitempty"`
}

type PluginEvent struct {
	ID         string          `json:"id"`
	SessionID  string          `json:"sessionId,omitempty"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurredAt"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type PluginOperation struct {
	ID             string          `json:"id"`
	SessionID      string          `json:"sessionId,omitempty"`
	Type           string          `json:"type"`
	Parameters     json.RawMessage `json:"parameters,omitempty"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
	DeadlineMS     int64           `json:"deadlineMs,omitempty"`
}

type PluginOperationStatus string

const (
	PluginOperationSucceeded PluginOperationStatus = "succeeded"
	PluginOperationFailed    PluginOperationStatus = "failed"
	PluginOperationCancelled PluginOperationStatus = "cancelled"
	PluginOperationRejected  PluginOperationStatus = "rejected"
)

type PluginOperationResult struct {
	OperationID  string                `json:"operationId"`
	SessionID    string                `json:"sessionId,omitempty"`
	Status       PluginOperationStatus `json:"status"`
	Output       json.RawMessage       `json:"output,omitempty"`
	ErrorCode    string                `json:"errorCode,omitempty"`
	ErrorMessage string                `json:"errorMessage,omitempty"`
	Retryable    bool                  `json:"retryable,omitempty"`
}

// HostFeature is a GameHost transport/runtime feature. It is intentionally
// separate from AI/tool capabilities, extension permissions and runtime-engine
// capabilities.
type HostFeature string

const (
	HostFeatureRealtimeControl HostFeature = "realtime_control"
	HostFeatureStateStreaming  HostFeature = "state_streaming"
	HostFeatureEventStreaming  HostFeature = "event_streaming"
	HostFeatureCustomRPC       HostFeature = "custom_rpc"
	HostFeatureHostAPI         HostFeature = "host_api"
	HostFeatureSharedControl   HostFeature = "shared_control"
	HostFeatureMultiService    HostFeature = "multi_service"
	HostFeatureBinaryStreaming HostFeature = "binary_streaming"
)

var knownHostFeatures = map[HostFeature]struct{}{
	HostFeatureRealtimeControl: {},
	HostFeatureStateStreaming:  {},
	HostFeatureEventStreaming:  {},
	HostFeatureCustomRPC:       {},
	HostFeatureHostAPI:         {},
	HostFeatureSharedControl:   {},
	HostFeatureMultiService:    {},
	HostFeatureBinaryStreaming: {},
}

func IsKnownHostFeature(feature HostFeature) bool {
	_, ok := knownHostFeatures[feature]
	return ok
}

const (
	PluginArtifactTypeFile      = "file"
	PluginArtifactTypeDirectory = "directory"
	PluginArtifactTypeZIP       = "zip"
)

type PluginArtifact struct {
	ID                    string   `json:"id"`
	Type                  string   `json:"type"`
	Platforms             []string `json:"platforms,omitempty"`
	Architectures         []string `json:"architectures,omitempty"`
	CompatibilityVersions []string `json:"compatibilityVersions,omitempty"`
	Source                string   `json:"source"`
	Target                string   `json:"target"`
	Required              bool     `json:"required,omitempty"`
	SHA256                string   `json:"sha256,omitempty"`
}

// PluginNetworkPolicy defines the protocol-v1 network intent. Restricted
// mode never grants the plugin process ambient network access: the child stays
// network-isolated and may only use the host.network.request Host API, which
// enforces this allowlist again at DNS, redirect, address and port boundaries.
type PluginNetworkPolicy struct {
	Mode           string   `json:"mode,omitempty"`
	AllowedDomains []string `json:"allowedDomains,omitempty"`
	AllowedIPs     []string `json:"allowedIPs,omitempty"`
	AllowedPorts   []int    `json:"allowedPorts,omitempty"`
}

type PluginServiceSpec struct {
	ID        string            `json:"id"`
	ModuleID  string            `json:"moduleId"`
	Name      string            `json:"name,omitempty"`
	Kind      string            `json:"kind,omitempty"`
	Required  bool              `json:"required,omitempty"`
	DependsOn []string          `json:"dependsOn,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type PluginChannelSpec struct {
	ID            string            `json:"id"`
	ServiceID     string            `json:"serviceId,omitempty"`
	Kind          string            `json:"kind"`
	SchemaID      string            `json:"schemaId,omitempty"`
	Direction     ChannelDirection  `json:"direction,omitempty"`
	FrequencyHint *FrequencyHint    `json:"frequencyHint,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type PluginControlEffectSinkSpec struct {
	ID          string `json:"id"`
	ServiceID   string `json:"serviceId"`
	Description string `json:"description,omitempty"`
}

// PluginHostSpec contains only host-owned execution and transport information.
// Concrete integration identity, versions, worlds, entities, actions and observations
// belong to the plugin itself and may be
// carried only in opaque plugin metadata/tool definitions.
type PluginHostSpec struct {
	ProtocolVersion    string                        `json:"protocolVersion"`
	RuntimeModuleID    string                        `json:"runtimeModuleId,omitempty"`
	HostFeatures       []HostFeature                 `json:"hostFeatures,omitempty"`
	Services           []PluginServiceSpec           `json:"services,omitempty"`
	Channels           []PluginChannelSpec           `json:"channels,omitempty"`
	ControlEffectSinks []PluginControlEffectSinkSpec `json:"controlEffectSinks,omitempty"`
	Artifacts          []PluginArtifact              `json:"artifacts,omitempty"`
	Network            *PluginNetworkPolicy          `json:"network"`
	Metadata           map[string]any                `json:"metadata,omitempty"`
}

func ParsePluginHostSpec(spec map[string]any) (PluginHostSpec, error) {
	if spec == nil {
		return PluginHostSpec{}, fmt.Errorf("game plugin spec is required")
	}
	data, err := json.Marshal(spec)
	if err != nil {
		return PluginHostSpec{}, fmt.Errorf("marshal game plugin spec: %w", err)
	}
	var parsed PluginHostSpec
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return PluginHostSpec{}, fmt.Errorf("decode game plugin spec: %w", err)
	}
	parsed.ProtocolVersion = strings.TrimSpace(parsed.ProtocolVersion)
	parsed.RuntimeModuleID = strings.TrimSpace(parsed.RuntimeModuleID)
	return parsed, nil
}

func (s PluginHostSpec) Validate() error {
	if s.ProtocolVersion == "" {
		return fmt.Errorf("protocolVersion is required")
	}
	if s.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported protocolVersion %q", s.ProtocolVersion)
	}
	if s.RuntimeModuleID == "" && len(s.Services) == 0 {
		return fmt.Errorf("runtimeModuleId or services is required")
	}

	seenFeatures := make(map[HostFeature]struct{}, len(s.HostFeatures))
	for _, raw := range s.HostFeatures {
		feature := HostFeature(strings.TrimSpace(string(raw)))
		if !HostFeatureSupportedByCurrentProtocol(feature) {
			return fmt.Errorf("unsupported host feature %q for %s", raw, ProtocolVersion)
		}
		if _, exists := seenFeatures[feature]; exists {
			return fmt.Errorf("duplicate host feature %q", feature)
		}
		seenFeatures[feature] = struct{}{}
	}
	if len(s.Services) > 1 {
		if _, ok := seenFeatures[HostFeatureMultiService]; !ok {
			return fmt.Errorf("multiple services require hostFeatures to include %q", HostFeatureMultiService)
		}
	}

	seenServices := make(map[string]struct{}, len(s.Services))
	seenProcessModules := make(map[string]struct{}, len(s.Services))
	for i, service := range s.Services {
		id := strings.TrimSpace(service.ID)
		moduleID := strings.TrimSpace(service.ModuleID)
		if id == "" || moduleID == "" {
			return fmt.Errorf("services[%d] id and moduleId are required", i)
		}
		if err := ValidateServiceID(ServiceID(id)); err != nil {
			return fmt.Errorf("services[%d] id: %w", i, err)
		}
		if err := ValidateServiceName(service.Name); err != nil {
			return fmt.Errorf("services[%d] name: %w", i, err)
		}
		if _, exists := seenServices[id]; exists {
			return fmt.Errorf("duplicate service id %q", id)
		}
		seenServices[id] = struct{}{}
		kind := strings.TrimSpace(service.Kind)
		if kind != "" && kind != "process" {
			return fmt.Errorf("services[%d] unsupported kind %q; protocol v1 services are process-backed", i, kind)
		}
		if _, exists := seenProcessModules[moduleID]; exists {
			return fmt.Errorf("services[%d] reuses runtime module %q; each process service requires a distinct runtime module", i, moduleID)
		}
		seenProcessModules[moduleID] = struct{}{}
	}
	if len(s.Services) > 0 && s.RuntimeModuleID != "" {
		if _, ok := seenProcessModules[s.RuntimeModuleID]; !ok {
			return fmt.Errorf("runtimeModuleId %q must reference one of services[].moduleId when services are declared", s.RuntimeModuleID)
		}
	}
	dependencyGraph := make(map[string][]string, len(s.Services))
	for i, service := range s.Services {
		seenDeps := make(map[string]struct{}, len(service.DependsOn))
		serviceID := strings.TrimSpace(service.ID)
		for _, dep := range service.DependsOn {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				return fmt.Errorf("services[%d] contains empty dependsOn id", i)
			}
			if err := ValidateServiceID(ServiceID(dep)); err != nil {
				return fmt.Errorf("services[%d] dependsOn id: %w", i, err)
			}
			if dep == strings.TrimSpace(service.ID) {
				return fmt.Errorf("service %q cannot depend on itself", service.ID)
			}
			if _, ok := seenServices[dep]; !ok {
				return fmt.Errorf("service %q depends on unknown service %q", service.ID, dep)
			}
			if _, duplicate := seenDeps[dep]; duplicate {
				return fmt.Errorf("service %q contains duplicate dependency %q", service.ID, dep)
			}
			seenDeps[dep] = struct{}{}
			dependencyGraph[serviceID] = append(dependencyGraph[serviceID], dep)
		}
	}
	if cycle := findServiceDependencyCycle(dependencyGraph); len(cycle) > 0 {
		return fmt.Errorf("service dependency cycle detected: %s", strings.Join(cycle, " -> "))
	}

	seenChannels := make(map[string]struct{}, len(s.Channels))
	hasBinaryChannel := false
	for i, channel := range s.Channels {
		id := strings.TrimSpace(channel.ID)
		serviceID := strings.TrimSpace(channel.ServiceID)
		kind := strings.TrimSpace(channel.Kind)
		if id == "" || kind == "" {
			return fmt.Errorf("channels[%d] id and kind are required", i)
		}
		if err := ValidateChannelID(ChannelID(id)); err != nil {
			return fmt.Errorf("channels[%d] id: %w", i, err)
		}
		if err := ValidateChannelSchemaID(strings.TrimSpace(channel.SchemaID)); err != nil {
			return fmt.Errorf("channels[%d] schemaId: %w", i, err)
		}
		if err := ValidateChannelDirection(channel.Direction); err != nil {
			return fmt.Errorf("channels[%d] direction: %w", i, err)
		}
		if channel.FrequencyHint != nil {
			if err := ValidateFrequencyHint(*channel.FrequencyHint); err != nil {
				return fmt.Errorf("channels[%d] frequencyHint: %w", i, err)
			}
		}
		if len(s.Services) > 1 && serviceID == "" {
			return fmt.Errorf("channels[%d] serviceId is required when multiple services are declared", i)
		}
		if serviceID != "" {
			if err := ValidateServiceID(ServiceID(serviceID)); err != nil {
				return fmt.Errorf("channels[%d] serviceId: %w", i, err)
			}
		}
		if serviceID != "" && len(seenServices) > 0 {
			if _, ok := seenServices[serviceID]; !ok {
				return fmt.Errorf("channel %q references unknown service %q", id, serviceID)
			}
		}
		switch kind {
		case "state":
			if _, ok := seenFeatures[HostFeatureStateStreaming]; !ok {
				return fmt.Errorf("state channel %q requires hostFeatures to include %q", id, HostFeatureStateStreaming)
			}
		case "event", "log", "metric", "custom":
			if _, ok := seenFeatures[HostFeatureEventStreaming]; !ok {
				return fmt.Errorf("%s channel %q requires hostFeatures to include %q", kind, id, HostFeatureEventStreaming)
			}
		case "binary":
			hasBinaryChannel = true
		default:
			return fmt.Errorf("channels[%d] unsupported kind %q", i, kind)
		}
		channelScope := serviceID
		if channelScope == "" && len(s.Services) == 1 {
			channelScope = strings.TrimSpace(s.Services[0].ID)
		}
		channelKey := channelScope + "\x00" + id
		if _, exists := seenChannels[channelKey]; exists {
			return fmt.Errorf("duplicate channel id %q within service %q", id, channelScope)
		}
		seenChannels[channelKey] = struct{}{}
	}

	if len(s.ControlEffectSinks) > 0 {
		if _, ok := seenFeatures[HostFeatureRealtimeControl]; !ok {
			return fmt.Errorf("controlEffectSinks require hostFeatures to include %q", HostFeatureRealtimeControl)
		}
	}

	if hasBinaryChannel {
		if _, ok := seenFeatures[HostFeatureBinaryStreaming]; !ok {
			return fmt.Errorf("binary channel requires hostFeatures to include %q", HostFeatureBinaryStreaming)
		}
	}

	seenSinks := make(map[string]struct{}, len(s.ControlEffectSinks))
	for i, sink := range s.ControlEffectSinks {
		id := strings.TrimSpace(sink.ID)
		serviceID := strings.TrimSpace(sink.ServiceID)
		if id == "" || serviceID == "" {
			return fmt.Errorf("controlEffectSinks[%d] id and serviceId are required", i)
		}
		if err := ValidateServiceID(ServiceID(serviceID)); err != nil {
			return fmt.Errorf("controlEffectSinks[%d] serviceId: %w", i, err)
		}
		if _, exists := seenSinks[id]; exists {
			return fmt.Errorf("duplicate control effect sink id %q", id)
		}
		seenSinks[id] = struct{}{}
		if len(seenServices) > 0 {
			if _, ok := seenServices[serviceID]; !ok {
				return fmt.Errorf("control effect sink %q references unknown service %q", id, serviceID)
			}
		}
	}

	if s.Network == nil {
		return fmt.Errorf("network policy is required; choose none, loopback, restricted, or unrestricted explicitly")
	}
	mode := strings.ToLower(strings.TrimSpace(s.Network.Mode))
	if mode == "" {
		return fmt.Errorf("network.mode is required")
	}
	switch mode {
	case "none", "loopback", "unrestricted":
		if len(s.Network.AllowedDomains) != 0 || len(s.Network.AllowedIPs) != 0 || len(s.Network.AllowedPorts) != 0 {
			return fmt.Errorf("network allowlists are only valid for restricted mode")
		}
	case "restricted":
		if err := validateRestrictedNetworkAllowlist(s.Network.AllowedDomains, s.Network.AllowedIPs, s.Network.AllowedPorts); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported network mode %q", s.Network.Mode)
	}

	seenArtifacts := make(map[string]struct{}, len(s.Artifacts))
	for _, artifact := range s.Artifacts {
		id := strings.TrimSpace(artifact.ID)
		target := strings.TrimSpace(artifact.Target)
		artifactType := strings.ToLower(strings.TrimSpace(artifact.Type))
		if id == "" || artifactType == "" || strings.TrimSpace(artifact.Source) == "" || target == "" {
			return fmt.Errorf("artifact id, type, source and target are required")
		}
		switch artifactType {
		case PluginArtifactTypeFile, PluginArtifactTypeDirectory, PluginArtifactTypeZIP:
		default:
			return fmt.Errorf("artifact %q has unsupported type %q; protocol v1 supports file, directory and zip", id, artifact.Type)
		}
		if _, exists := seenArtifacts[id]; exists {
			return fmt.Errorf("duplicate artifact id %q", id)
		}
		seenArtifacts[id] = struct{}{}
		if !safePackageRelativePath(artifact.Source) {
			return fmt.Errorf("artifact %q source must be a safe package-relative path", id)
		}
		if !safePackageRelativePath(target) {
			return fmt.Errorf("artifact %q target must be a safe target-relative path", id)
		}
		for j, constraint := range artifact.CompatibilityVersions {
			if err := ValidateCompatibilityConstraint(constraint); err != nil {
				return fmt.Errorf("artifact %q compatibilityVersions[%d]: %w", id, j, err)
			}
		}
	}
	return nil
}

func validateRestrictedNetworkAllowlist(domains, ips []string, ports []int) error {
	if len(domains) == 0 && len(ips) == 0 {
		return fmt.Errorf("restricted mode requires at least one network.allowedDomains or network.allowedIPs entry")
	}
	if len(ports) == 0 {
		return fmt.Errorf("network.allowedPorts is required for restricted mode")
	}
	seenDomains := make(map[string]struct{}, len(domains))
	for _, raw := range domains {
		domain := strings.ToLower(strings.TrimSpace(raw))
		if domain == "" || strings.ContainsAny(domain, "/:@?#\\") || strings.HasSuffix(domain, ".") {
			return fmt.Errorf("invalid restricted network domain %q", raw)
		}
		if strings.HasPrefix(domain, "*.") {
			domain = strings.TrimPrefix(domain, "*.")
		}
		if domain == "" || strings.Contains(domain, "*") || !validDNSName(domain) {
			return fmt.Errorf("invalid restricted network domain %q", raw)
		}
		key := strings.ToLower(strings.TrimSpace(raw))
		if _, exists := seenDomains[key]; exists {
			return fmt.Errorf("duplicate restricted network domain %q", raw)
		}
		seenDomains[key] = struct{}{}
	}
	seenIPs := make(map[netip.Addr]struct{}, len(ips))
	for _, raw := range ips {
		value := strings.TrimSpace(raw)
		addr, err := netip.ParseAddr(value)
		if err != nil || addr.Zone() != "" {
			return fmt.Errorf("invalid restricted network IP %q", raw)
		}
		addr = addr.Unmap()
		if !addr.IsValid() || addr.IsUnspecified() || addr.IsMulticast() {
			return fmt.Errorf("invalid restricted network IP %q", raw)
		}
		if _, exists := seenIPs[addr]; exists {
			return fmt.Errorf("duplicate restricted network IP %q", raw)
		}
		seenIPs[addr] = struct{}{}
	}
	seenPorts := make(map[int]struct{}, len(ports))
	for _, port := range ports {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid restricted network port %d", port)
		}
		if _, exists := seenPorts[port]; exists {
			return fmt.Errorf("duplicate restricted network port %d", port)
		}
		seenPorts[port] = struct{}{}
	}
	return nil
}

func validDNSName(value string) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	labels := strings.Split(value, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
				return false
			}
		}
	}
	return true
}

func findServiceDependencyCycle(graph map[string][]string) []string {
	const (
		unvisited = iota
		visiting
		visited
	)
	state := make(map[string]int, len(graph))
	stack := make([]string, 0, len(graph))
	stackIndex := make(map[string]int, len(graph))
	keys := make([]string, 0, len(graph))
	for id := range graph {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	var visit func(string) []string
	visit = func(id string) []string {
		switch state[id] {
		case visited:
			return nil
		case visiting:
			start := stackIndex[id]
			cycle := append([]string(nil), stack[start:]...)
			return append(cycle, id)
		}
		state[id] = visiting
		stackIndex[id] = len(stack)
		stack = append(stack, id)
		deps := append([]string(nil), graph[id]...)
		sort.Strings(deps)
		for _, dep := range deps {
			if cycle := visit(dep); len(cycle) > 0 {
				return cycle
			}
		}
		stack = stack[:len(stack)-1]
		delete(stackIndex, id)
		state[id] = visited
		return nil
	}
	for _, id := range keys {
		if cycle := visit(id); len(cycle) > 0 {
			return cycle
		}
	}
	return nil
}

func safePackageRelativePath(raw string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") {
		return false
	}
	if strings.Contains(normalized, ":") {
		return false
	}
	clean := path.Clean(normalized)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}
