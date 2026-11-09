package plugincontract

import (
	"encoding/json"
	"errors"
)

const SchemaVersion = 1

var ErrInvalidManifest = errors.New("plugincontract: invalid manifest")

type Manifest struct {
	SchemaVersion    int                          `json:"schemaVersion"`
	Plugin           PluginMeta                   `json:"plugin"`
	Compatibility    Compatibility                `json:"compatibility"`
	Modules          []Module                     `json:"modules"`
	Dependencies     []PluginDependency           `json:"dependencies,omitempty"`
	Permissions      []Permission                 `json:"permissions,omitempty"`
	Resources        []Resource                   `json:"resources,omitempty"`
	SecretRefs       []SecretRef                  `json:"secretRefs,omitempty"`
	AdapterProviders []AdapterProviderDeclaration `json:"adapterProviders,omitempty"`
	Data             *DataContract                `json:"data,omitempty"`
	Integrity        Integrity                    `json:"integrity"`
	Development      Development                  `json:"development,omitempty"`
}

type LocalizedText struct {
	Default      string            `json:"default"`
	Translations map[string]string `json:"translations,omitempty"`
}

type PluginMeta struct {
	ID          string         `json:"id"`
	Name        LocalizedText  `json:"name"`
	Description LocalizedText  `json:"description,omitempty"`
	Version     string         `json:"version"`
	License     string         `json:"license,omitempty"`
	Homepage    string         `json:"homepage,omitempty"`
	Repository  string         `json:"repository,omitempty"`
	Publisher   Publisher      `json:"publisher"`
	Categories  []string       `json:"categories,omitempty"`
	Keywords    []string       `json:"keywords,omitempty"`
	Icon        string         `json:"icon,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Publisher struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	TrustLevel  string `json:"trustLevel,omitempty"`
	Contact     string `json:"contact,omitempty"`
	Website     string `json:"website,omitempty"`
}

type Compatibility struct {
	MinHostVersion string   `json:"minHostVersion"`
	MaxHostVersion string   `json:"maxHostVersion,omitempty"`
	Platforms      []string `json:"platforms,omitempty"`
	FeatureFlags   []string `json:"featureFlags,omitempty"`
}

type Module struct {
	ID                   string                   `json:"id"`
	Name                 LocalizedText            `json:"name"`
	Description          LocalizedText            `json:"description,omitempty"`
	Version              string                   `json:"version,omitempty"`
	Runtime              *RuntimeSpec             `json:"runtime,omitempty"`
	Services             ServiceDeclaration       `json:"services,omitempty"`
	Dynamic              DynamicCapability        `json:"dynamic,omitempty"`
	Contributions        []ContributionDescriptor `json:"contributions,omitempty"`
	Permissions          []Permission             `json:"permissions,omitempty"`
	Resources            []Resource               `json:"resources,omitempty"`
	Dependencies         []PluginDependency       `json:"dependencies,omitempty"`
	Placement            ServicePlacement         `json:"placement,omitempty"`
	Compatibility        *ModuleCompatibility     `json:"compatibility,omitempty"`
	Policies             *ModulePolicies          `json:"policies,omitempty"`
	DeviceRequirements   *DeviceRequirements      `json:"deviceRequirements,omitempty"`
	ProvidedCapabilities []ProvidedCapability     `json:"providedCapabilities,omitempty"`
	Provider             *ProviderMetadata        `json:"provider,omitempty"`
}

type RuntimeSpec struct {
	Adapter         string            `json:"adapter"`
	ServiceID       string            `json:"serviceId,omitempty"`
	Entry           string            `json:"entry,omitempty"`
	WorkerCount     int               `json:"workerCount,omitempty"`
	Timeout         string            `json:"timeout,omitempty"`
	MemoryBytes     int64             `json:"memoryBytes,omitempty"`
	CPUPercent      int               `json:"cpuPercent,omitempty"`
	MaxProcesses    int               `json:"maxProcesses,omitempty"`
	Permissions     []string          `json:"permissions,omitempty"`
	Capabilities    map[string]bool   `json:"capabilities,omitempty"`
	Environment     map[string]string `json:"environment,omitempty"`
	HostAccess      *HostAccess       `json:"hostAccess,omitempty"`
	NativeArtifacts []NativeArtifact  `json:"nativeArtifacts,omitempty"`
}

type HostAccess struct {
	Filesystem *FilesystemAccess `json:"filesystem,omitempty"`
	Network    *NetworkAccess    `json:"network,omitempty"`
	Process    *ProcessAccess    `json:"process,omitempty"`
}

type FilesystemAccess struct {
	Mode  string   `json:"mode"`
	Roots []string `json:"roots,omitempty"`
}

type NetworkAccess struct {
	Mode string `json:"mode"`
}

type ProcessAccess struct {
	Mode         string `json:"mode"`
	MaxProcesses int    `json:"maxProcesses,omitempty"`
}

type NativeArtifact struct {
	ID           string   `json:"id"`
	Platform     string   `json:"platform"`
	Architecture string   `json:"architecture,omitempty"`
	Path         string   `json:"path"`
	SHA256       string   `json:"sha256"`
	Executable   bool     `json:"executable"`
	Arguments    []string `json:"arguments,omitempty"`
}

type ModuleCompatibility struct {
	MinHostVersion string   `json:"minHostVersion,omitempty"`
	Platforms      []string `json:"platforms,omitempty"`
}

type ModulePolicies struct {
	Isolation        string `json:"isolation,omitempty"`
	NetworkAccess    bool   `json:"networkAccess,omitempty"`
	FileSystemAccess bool   `json:"fileSystemAccess,omitempty"`
}

type DeviceRequirements struct {
	Platforms         []string `json:"platforms,omitempty"`
	Architectures     []string `json:"architectures,omitempty"`
	MinAppVersion     string   `json:"minAppVersion,omitempty"`
	MinRuntimeVersion string   `json:"minRuntimeVersion,omitempty"`
	RequiredFeatures  []string `json:"requiredFeatures,omitempty"`
}

type ProvidedCapability struct {
	ID       string         `json:"id"`
	Version  string         `json:"version,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ProviderMetadata struct {
	ID       string            `json:"id,omitempty"`
	Priority int               `json:"priority,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
	Metadata map[string]any    `json:"metadata,omitempty"`
}

type ServiceDeclaration struct {
	Provides []ServiceOffer       `json:"provides,omitempty"`
	Requires []ServiceRequirement `json:"requires,omitempty"`
}

type ServiceOffer struct {
	ServiceID    string           `json:"serviceId"`
	Version      string           `json:"version"`
	ContractHash string           `json:"contractHash,omitempty"`
	Scopes       []ServiceScope   `json:"scopes"`
	Placement    ServicePlacement `json:"placement"`
	Priority     int              `json:"priority,omitempty"`
}

type ServiceRequirement struct {
	ServiceID    string           `json:"serviceId"`
	VersionRange string           `json:"versionRange,omitempty"`
	ContractHash string           `json:"contractHash,omitempty"`
	Optional     bool             `json:"optional,omitempty"`
	Selection    ServiceSelection `json:"selection,omitempty"`
	Reason       string           `json:"reason,omitempty"`
}

type ServiceScope string

const (
	ServiceScopeGlobal       ServiceScope = "global"
	ServiceScopeCharacter    ServiceScope = "character"
	ServiceScopeConversation ServiceScope = "conversation"
	ServiceScopeDevice       ServiceScope = "device"
	ServiceScopePlugin       ServiceScope = "plugin"
)

type ServiceSelection string

const (
	ServiceSelectionSingle    ServiceSelection = "single"
	ServiceSelectionAll       ServiceSelection = "all"
	ServiceSelectionExclusive ServiceSelection = "exclusive"
)

type ServicePlacement string

const (
	ServicePlacementCore   ServicePlacement = "core"
	ServicePlacementDevice ServicePlacement = "device"
	ServicePlacementCloud  ServicePlacement = "cloud"
	ServicePlacementHybrid ServicePlacement = "hybrid"
)

type DynamicCapability struct {
	AllowedAdapters  []string `json:"allowedAdapters,omitempty"`
	AllowedKinds     []string `json:"allowedKinds,omitempty"`
	MaxContributions int      `json:"maxContributions,omitempty"`
	MaxTools         int      `json:"maxTools,omitempty"`
	MaxRoutes        int      `json:"maxRoutes,omitempty"`
	MaxServices      int      `json:"maxServices,omitempty"`
	AllowUnregister  bool     `json:"allowUnregister,omitempty"`
}

type ContributionDescriptor struct {
	ID                  string             `json:"id"`
	Kind                string             `json:"kind"`
	Adapter             string             `json:"adapter"`
	ContractVersion     int                `json:"contractVersion"`
	Schema              json.RawMessage    `json:"schema,omitempty"`
	Name                LocalizedText      `json:"name"`
	Description         LocalizedText      `json:"description,omitempty"`
	Version             string             `json:"version,omitempty"`
	Spec                map[string]any     `json:"spec,omitempty"`
	RequiredPermissions []string           `json:"requiredPermissions,omitempty"`
	RequiredScope       []string           `json:"requiredScope,omitempty"`
	Exposure            *Exposure          `json:"exposure,omitempty"`
	RuntimeBinding      *RuntimeBinding    `json:"runtimeBinding,omitempty"`
	Dependencies        []PluginDependency `json:"dependencies,omitempty"`
	Metadata            map[string]any     `json:"metadata,omitempty"`
}

type Exposure struct {
	VisibleByDefault    bool     `json:"visibleByDefault,omitempty"`
	HiddenFromDiscovery bool     `json:"hiddenFromDiscovery,omitempty"`
	RequiredRoles       []string `json:"requiredRoles,omitempty"`
}

type RuntimeBinding struct {
	RuntimeID  string `json:"runtimeId"`
	Generation int64  `json:"generation,omitempty"`
}

type Permission struct {
	Name     string `json:"name"`
	Reason   string `json:"reason,omitempty"`
	Required bool   `json:"required,omitempty"`
	Scope    string `json:"scope,omitempty"`
}

type Resource struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Path string `json:"path"`
	Hash string `json:"hash,omitempty"`
	Size int64  `json:"size,omitempty"`
}

type SecretRef struct {
	Ref         string `json:"ref"`
	ServiceID   string `json:"serviceId,omitempty"`
	Purpose     string `json:"purpose,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type AdapterProviderDeclaration struct {
	ServiceID                string          `json:"serviceId"`
	AdapterID                string          `json:"adapterId"`
	ContractVersion          int             `json:"contractVersion"`
	SupportedKinds           []string        `json:"supportedKinds"`
	Operations               []string        `json:"operations,omitempty"`
	ContributionSchema       json.RawMessage `json:"contributionSchema"`
	RequiredHostCapabilities []string        `json:"requiredHostCapabilities,omitempty"`
	PermissionRequirements   []string        `json:"permissionRequirements,omitempty"`
	Platforms                []string        `json:"platforms,omitempty"`
}

type DataContract struct {
	SchemaVersion int      `json:"schemaVersion"`
	Scopes        []string `json:"scopes,omitempty"`
	Backup        bool     `json:"backup,omitempty"`
	Export        bool     `json:"export,omitempty"`
	Uninstall     string   `json:"uninstall,omitempty"`
}

type Integrity struct {
	Algorithm       string            `json:"algorithm"`
	ContentTreeHash string            `json:"contentTreeHash"`
	FileHashes      map[string]string `json:"fileHashes,omitempty"`
}

type Development struct {
	HotReload  bool     `json:"hotReload,omitempty"`
	SourceMaps bool     `json:"sourceMaps,omitempty"`
	TestEntry  string   `json:"testEntry,omitempty"`
	WatchPaths []string `json:"watchPaths,omitempty"`
}

type PluginDependency struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Version  string `json:"version,omitempty"`
	Optional bool   `json:"optional,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type ValidationIssue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationReport struct {
	Issues []ValidationIssue `json:"issues,omitempty"`
}

func (r *ValidationReport) Add(path string, code string, message string) {
	if r == nil {
		return
	}
	r.Issues = append(r.Issues, ValidationIssue{Path: path, Code: code, Message: message})
}

func (r ValidationReport) HasErrors() bool {
	return len(r.Issues) > 0
}
