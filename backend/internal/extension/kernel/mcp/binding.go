// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import "time"

type MCPBindingScope string

const (
	MCPScopeUser     MCPBindingScope = "user"
	MCPScopeExtension MCPBindingScope = "extension"
	MCPScopeBuiltin  MCPBindingScope = "builtin"
)

type ExtensionOwnerRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type MCPTransportSpec struct {
	Kind           string            `json:"kind"`
	Endpoint       string            `json:"endpoint,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	CredentialRef  string            `json:"credentialRef,omitempty"`
	StartTimeout   time.Duration     `json:"startTimeout,omitempty"`
}

type MCPLauncherSpec struct {
	Kind        string            `json:"kind"`
	Package     string            `json:"package,omitempty"`
	Command     string            `json:"command,omitempty"`
	Version     string            `json:"version,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	WorkDir     string            `json:"workDir,omitempty"`
	CredentialRef string          `json:"credentialRef,omitempty"`
}

type MCPBinding struct {
	ID          string            `json:"id"`
	Owner       ExtensionOwnerRef `json:"owner"`
	Transport   MCPTransportSpec `json:"transport"`
	Launcher    *MCPLauncherSpec  `json:"launcher,omitempty"`
	Enabled     bool              `json:"enabled"`
	Scope       MCPBindingScope   `json:"scope"`
	Permissions []string          `json:"permissions,omitempty"`
}

func (b MCPBinding) IsRemote() bool {
	return b.Transport.Kind == "streamable_http" || b.Transport.Kind == "remote"
}

func (b MCPBinding) IsLocal() bool {
	return !b.IsRemote()
}

func (b MCPBinding) RequiresLocalProcess() bool {
	if b.Launcher == nil {
		return false
	}
	switch MCPLauncherKind(b.Launcher.Kind) {
	case MCPLauncherExecutable, MCPLauncherNPX, MCPLauncherUVX:
		return true
	}
	return false
}
