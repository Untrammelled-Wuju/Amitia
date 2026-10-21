package kernel

import "context"

type MCPBindingDescriptor struct {
	ExtensionID         string            `json:"extensionId"`
	ModuleID            string            `json:"moduleId"`
	ContributionID      string            `json:"contributionId"`
	PackageGeneration   int64             `json:"packageGeneration"`
	ServerName          string            `json:"serverName"`
	DisplayName         string            `json:"displayName"`
	Description         string            `json:"description"`
	Transport           string            `json:"transport"`
	Endpoint            string            `json:"endpoint"`
	Command             string            `json:"command"`
	Args                []string          `json:"args"`
	Env                 map[string]string `json:"env"`
	Headers             map[string]string `json:"headers"`
	WorkDir             string            `json:"workDir"`
	AuthType            string            `json:"authType"`
	CredentialType      string            `json:"credentialType"`
	Credential          []byte            `json:"-"`
	AllowPrivateNetwork bool              `json:"allowPrivateNetwork"`
	Capabilities        map[string]bool   `json:"capabilities"`
	RuntimePlacement    string            `json:"runtimePlacement"`
}

type MCPBindingService interface {
	PutContributionBinding(context.Context, MCPBindingDescriptor) (string, error)
	ActivateContributionBinding(context.Context, MCPBindingDescriptor) (string, error)
	DeactivateContributionBinding(context.Context, string, string) error
	RemoveContributionBinding(context.Context, string, string) error
	RemoveExtensionBindings(context.Context, string) error
}
