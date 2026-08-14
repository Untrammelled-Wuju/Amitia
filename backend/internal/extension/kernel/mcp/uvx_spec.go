// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import "time"

type MCPLauncherKind string

const (
	MCPLauncherExecutable MCPLauncherKind = "executable"
	MCPLauncherNPX        MCPLauncherKind = "npx"
	MCPLauncherUVX        MCPLauncherKind = "uvx"
)

type UVXFetchPolicy string

const (
	UVXFetchDeny  UVXFetchPolicy = "deny"
	UVXFetchAllow UVXFetchPolicy = "allow"
)

const (
	UVXMaxArgs             = 128
	UVXMaxArgBytes         = 16 * 1024
	UVXMaxArgsTotalBytes   = 64 * 1024
	UVXDefaultReadyTimeout = 120 * time.Second
	UVXMaxReadyTimeout     = 300 * time.Second
)

type PythonIndexSpec struct {
	DefaultIndex   string   `json:"defaultIndex,omitempty"`
	ExtraIndexes   []string `json:"extraIndexes,omitempty"`
	Strategy       string   `json:"strategy,omitempty"`
	CredentialRefs []string `json:"credentialRefs,omitempty"`
}

type UvxLaunchSpec struct {
	Package          string            `json:"package"`
	Command          string            `json:"command"`
	Version          string            `json:"version,omitempty"`
	Python           string            `json:"python,omitempty"`
	Extras           []string          `json:"extras,omitempty"`
	Args             []string          `json:"args,omitempty"`
	WorkingDirectory string            `json:"workingDirectory,omitempty"`
	Index            *PythonIndexSpec  `json:"index,omitempty"`
	Offline          bool              `json:"offline,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
	CredentialRef    string            `json:"credentialRef,omitempty"`
	StartTimeout     time.Duration     `json:"startTimeout,omitempty"`
}

func (s UvxLaunchSpec) StartTimeoutOrDefault() time.Duration {
	if s.StartTimeout <= 0 {
		return UVXDefaultReadyTimeout
	}
	if s.StartTimeout > UVXMaxReadyTimeout {
		return UVXMaxReadyTimeout
	}
	return s.StartTimeout
}

type PythonToolRequirement struct {
	Name        string
	Extras      []string
	VersionSpec string
}

func (r PythonToolRequirement) Canonical() string {
	result := r.Name

	if len(r.Extras) > 0 {
		result += "[" + r.Extras[0]
		for i := 1; i < len(r.Extras); i++ {
			result += "," + r.Extras[i]
		}
		result += "]"
	}

	if r.VersionSpec != "" {
		result += r.VersionSpec
	}

	return result
}
