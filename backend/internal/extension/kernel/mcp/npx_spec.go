// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import "time"

type NPXFetchPolicy string

const (
	NPXFetchDeny  NPXFetchPolicy = "deny"
	NPXFetchAllow NPXFetchPolicy = "allow"
)

type MCPNPXSpec struct {
	Package string `json:"package"`

	Binary string `json:"binary,omitempty"`

	Args []string `json:"args,omitempty"`

	FetchPolicy string `json:"fetchPolicy,omitempty"`

	AllowFloatingVersion bool `json:"allowFloatingVersion,omitempty"`

	WorkDir string `json:"workDir,omitempty"`

	Environment map[string]string `json:"environment,omitempty"`

	CredentialRef string `json:"credentialRef,omitempty"`

	StartTimeout time.Duration `json:"startTimeout,omitempty"`
}

func (s MCPNPXSpec) FetchPolicyOrDefault() NPXFetchPolicy {
	switch NPXFetchPolicy(s.FetchPolicy) {
	case NPXFetchAllow:
		return NPXFetchAllow
	default:
		return NPXFetchDeny
	}
}

func (s MCPNPXSpec) StartTimeoutOrDefault() time.Duration {
	if s.StartTimeout <= 0 {
		return 60 * time.Second
	}
	if s.StartTimeout > 180*time.Second {
		return 180 * time.Second
	}
	return s.StartTimeout
}
