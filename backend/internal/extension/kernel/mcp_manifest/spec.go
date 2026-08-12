package mcp_manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const MCPServerSpecVersion = 1

type MCPServerSpec struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Transport     MCPTransportSpec    `json:"transport"`
	Capabilities  MCPCapabilityPolicy `json:"capabilities,omitempty"`
	Lifecycle     MCPLifecyclePolicy  `json:"lifecycle,omitempty"`
	Security      MCPSecurityPolicy   `json:"security,omitempty"`
	Metadata      map[string]string   `json:"metadata,omitempty"`
}

type MCPTransportSpec struct {
	Type   MCPTransportType        `json:"type"`
	Stdio  *MCPStdioTransportSpec  `json:"stdio,omitempty"`
	Remote *MCPRemoteTransportSpec `json:"remote,omitempty"`
}

type MCPTransportType string

const (
	MCPTransportTypeStdio         MCPTransportType = "stdio"
	MCPTransportTypeStreamableHTTP MCPTransportType = "streamable_http"
	MCPTransportTypeSSE           MCPTransportType = "sse"
)

type MCPStdioTransportSpec struct {
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	WorkDir     string            `json:"workDir,omitempty"`
	Env         MCPEnvSpec        `json:"env,omitempty"`
	RuntimeHint string            `json:"runtimeHint,omitempty"`
}

type MCPEnvSpec struct {
	Values  map[string]string `json:"values,omitempty"`
	Secrets map[string]string `json:"secrets,omitempty"`
}

type MCPRemoteTransportSpec struct {
	URL                 string            `json:"url"`
	Headers             map[string]MCPValueRef `json:"headers,omitempty"`
	Auth                *MCPAuthSpec      `json:"auth,omitempty"`
	AllowPrivateNetwork bool              `json:"allowPrivateNetwork,omitempty"`
}

type MCPValueRef struct {
	Value      string `json:"value,omitempty"`
	SecretRef  string `json:"secretRef,omitempty"`
}

type MCPAuthSpec struct {
	Type       MCPAuthType `json:"type,omitempty"`
	SecretRef  string      `json:"secretRef,omitempty"`
	OAuth      *MCPOAuthRef `json:"oauth,omitempty"`
}

type MCPAuthType string

const (
	MCPAuthTypeNone         MCPAuthType = "none"
	MCPAuthTypeBearerToken  MCPAuthType = "bearer_token"
	MCPAuthTypeCustomHeaders MCPAuthType = "custom_headers"
	MCPAuthTypeStdioEnv     MCPAuthType = "stdio_env"
	MCPAuthTypeOAuth        MCPAuthType = "oauth"
)

type MCPOAuthRef struct {
	ProviderRef string   `json:"providerRef,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	ClientRef   string   `json:"clientRef,omitempty"`
}

type MCPCapabilityPolicy struct {
	Server MCPServerFeaturePolicy `json:"server,omitempty"`
	Client MCPClientFeaturePolicy `json:"client,omitempty"`
}

type MCPServerFeaturePolicy struct {
	Tools      MCPCapabilityState `json:"tools,omitempty"`
	Resources  MCPCapabilityState `json:"resources,omitempty"`
	Prompts    MCPCapabilityState `json:"prompts,omitempty"`
	Tasks      MCPCapabilityState `json:"tasks,omitempty"`
	Completion MCPCapabilityState `json:"completion,omitempty"`
	Logging    MCPCapabilityState `json:"logging,omitempty"`
}

type MCPClientFeaturePolicy struct {
	Roots      MCPCapabilityState `json:"roots,omitempty"`
	Sampling   MCPCapabilityState `json:"sampling,omitempty"`
	Elicitation MCPCapabilityState `json:"elicitation,omitempty"`
	Tasks      MCPCapabilityState `json:"tasks,omitempty"`
}

type MCPCapabilityState string

const (
	MCPCapabilityStateDisabled MCPCapabilityState = "disabled"
	MCPCapabilityStateOptional MCPCapabilityState = "optional"
	MCPCapabilityStateRequired MCPCapabilityState = "required"
)

type MCPLifecyclePolicy struct {
	AutoStart          bool                `json:"autoStart,omitempty"`
	RestartPolicy      MCPRestartPolicy    `json:"restartPolicy,omitempty"`
	MaxRestartAttempts int                 `json:"maxRestartAttempts,omitempty"`
	StartupTimeout     string              `json:"startupTimeout,omitempty"`
	ShutdownTimeout    string              `json:"shutdownTimeout,omitempty"`
	ReconnectPolicy    MCPReconnectPolicy  `json:"reconnectPolicy,omitempty"`
	RefreshOnReconnect bool                `json:"refreshOnReconnect,omitempty"`
}

type MCPRestartPolicy string

const (
	MCPRestartPolicyNever    MCPRestartPolicy = "never"
	MCPRestartPolicyOnFailure MCPRestartPolicy = "on_failure"
	MCPRestartPolicyAlways   MCPRestartPolicy = "always"
)

type MCPReconnectPolicy struct {
	Enabled     bool   `json:"enabled,omitempty"`
	MaxAttempts int    `json:"maxAttempts,omitempty"`
	Backoff     string `json:"backoff,omitempty"`
}

type MCPSecurityPolicy struct {
	AllowPrivateNetwork bool `json:"allowPrivateNetwork,omitempty"`
}

type CompiledMCPServerManifest struct {
	ServerID           string
	ExtensionID        string
	ModuleID           string
	ContributionID     string
	Spec               MCPServerSpec
	ConfigurationHash  string
	RequiredPermissions []string
	RequiredScope      []string
}

func ParseSpec(raw map[string]any) (MCPServerSpec, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return MCPServerSpec{}, fmt.Errorf("mcp_manifest: marshal spec: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var spec MCPServerSpec
	if err := decoder.Decode(&spec); err != nil {
		return MCPServerSpec{}, fmt.Errorf("mcp_manifest: parse spec: %w", err)
	}
	return spec, nil
}

func (s MCPServerSpec) Canonicalize() MCPServerSpec {
	if s.SchemaVersion == 0 {
		s.SchemaVersion = MCPServerSpecVersion
	}
	s.Lifecycle.normalize()
	return s
}

func (l *MCPLifecyclePolicy) normalize() {
	if l.RestartPolicy == "" {
		l.RestartPolicy = MCPRestartPolicyOnFailure
	}
	if l.ReconnectPolicy.Enabled && l.ReconnectPolicy.MaxAttempts == 0 {
		l.ReconnectPolicy.MaxAttempts = 6
	}
	if l.StartupTimeout == "" {
		l.StartupTimeout = "10s"
	}
	if l.ShutdownTimeout == "" {
		l.ShutdownTimeout = "3s"
	}
}

func (s MCPServerSpec) ComputeConfigurationHash() string {
	h := sha256.New()
	fmt.Fprintf(h, "transport.type=%s\n", s.Transport.Type)
	if s.Transport.Stdio != nil {
		fmt.Fprintf(h, "stdio.command=%s\n", s.Transport.Stdio.Command)
		for _, arg := range s.Transport.Stdio.Args {
			fmt.Fprintf(h, "stdio.arg=%s\n", arg)
		}
		fmt.Fprintf(h, "stdio.workDir=%s\n", s.Transport.Stdio.WorkDir)
		for k, v := range s.Transport.Stdio.Env.Values {
			fmt.Fprintf(h, "stdio.env.%s=%s\n", k, v)
		}
		for k, ref := range s.Transport.Stdio.Env.Secrets {
			fmt.Fprintf(h, "stdio.env.%s=secret:%s\n", k, ref)
		}
	}
	if s.Transport.Remote != nil {
		fmt.Fprintf(h, "remote.url=%s\n", s.Transport.Remote.URL)
		fmt.Fprintf(h, "remote.allowPrivateNetwork=%v\n", s.Transport.Remote.AllowPrivateNetwork)
		for k, ref := range s.Transport.Remote.Headers {
			fmt.Fprintf(h, "remote.header.%s\n", k)
			_ = ref
		}
		if s.Transport.Remote.Auth != nil {
			fmt.Fprintf(h, "remote.auth.type=%s\n", s.Transport.Remote.Auth.Type)
			fmt.Fprintf(h, "remote.auth.secretRef=%s\n", s.Transport.Remote.Auth.SecretRef)
		}
	}
	fmt.Fprintf(h, "security.allowPrivateNetwork=%v\n", s.Security.AllowPrivateNetwork)
	capBytes, _ := json.Marshal(s.Capabilities)
	h.Write(capBytes)
	lifecycleBytes, _ := json.Marshal(s.Lifecycle)
	h.Write(lifecycleBytes)
	return hex.EncodeToString(h.Sum(nil))
}
