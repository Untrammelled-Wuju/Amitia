package domain

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ExtensionID string
type ModuleID string
type ContributionID string
type RuntimeID string

type LocalizedText struct {
	Default      string            `json:"default"`
	Translations map[string]string `json:"translations,omitempty"`
}

func (l LocalizedText) Get(lang string) string {
	if lang == "" {
		return l.Default
	}
	if t, ok := l.Translations[lang]; ok {
		return t
	}
	return l.Default
}

type SemanticVersion struct {
	Major      int    `json:"major"`
	Minor      int    `json:"minor"`
	Patch      int    `json:"patch"`
	PreRelease string `json:"preRelease,omitempty"`
	Build      string `json:"build,omitempty"`
}

var (
	ErrInvalidExtensionID = errors.New("domain: invalid extension id")
	ErrInvalidVersion     = errors.New("domain: invalid version")
	ErrInvalidModuleID    = errors.New("domain: invalid module id")
)

var extensionIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*(\.[a-z0-9-]+)+/[a-z0-9][a-z0-9-]*$`)

func ValidateExtensionID(id ExtensionID) error {
	if id == "" {
		return fmt.Errorf("%w: empty", ErrInvalidExtensionID)
	}
	if !extensionIDPattern.MatchString(string(id)) {
		return fmt.Errorf("%w: %s must match <reverse-domain>/<name>", ErrInvalidExtensionID, id)
	}
	return nil
}

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-.]+))?(?:\+([0-9A-Za-z-.]+))?$`)

func ParseVersion(s string) (SemanticVersion, error) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return SemanticVersion{}, fmt.Errorf("%w: %s", ErrInvalidVersion, s)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return SemanticVersion{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: m[4],
		Build:      m[5],
	}, nil
}

func (v SemanticVersion) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.PreRelease != "" {
		s += "-" + v.PreRelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

func (v SemanticVersion) Compare(other SemanticVersion) int {
	if v.Major != other.Major {
		return cmpInt(v.Major, other.Major)
	}
	if v.Minor != other.Minor {
		return cmpInt(v.Minor, other.Minor)
	}
	if v.Patch != other.Patch {
		return cmpInt(v.Patch, other.Patch)
	}
	if v.PreRelease == "" && other.PreRelease != "" {
		return 1
	}
	if v.PreRelease != "" && other.PreRelease == "" {
		return -1
	}
	return strings.Compare(v.PreRelease, other.PreRelease)
}

func (v SemanticVersion) IsCompatibleWith(other SemanticVersion) bool {
	if v.Major != other.Major {
		return false
	}
	if v.Minor > other.Minor {
		return false
	}
	return true
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

type InstallationState string

const (
	InstallationStateNotInstalled    InstallationState = "not_installed"
	InstallationStateInstalling      InstallationState = "installing"
	InstallationStateInstalled       InstallationState = "installed"
	InstallationStateUpdating        InstallationState = "updating"
	InstallationStateRollingBack     InstallationState = "rolling_back"
	InstallationStateUninstalling    InstallationState = "uninstalling"
	InstallationStateFailed          InstallationState = "failed"
	InstallationStateUninstallFailed InstallationState = "uninstall_failed"
)

type EnablementState string

const (
	EnablementEnabled           EnablementState = "enabled"
	EnablementDisabled          EnablementState = "disabled"
	EnablementPartiallyDisabled EnablementState = "partially_disabled"
	EnablementRequiresRecovery  EnablementState = "requires_recovery"
)

type PublisherReference struct {
	PublisherID string `json:"publisherId"`
	DisplayName string `json:"displayName,omitempty"`
	TrustLevel  string `json:"trustLevel,omitempty"`
}

type SignatureReference struct {
	Algorithm  string     `json:"algorithm,omitempty"`
	KeyID      string     `json:"keyId,omitempty"`
	SignedAt   *time.Time `json:"signedAt,omitempty"`
	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
	Status     string     `json:"status,omitempty"`
}

type PackageReference struct {
	PackageID       string             `json:"packageId"`
	ManifestVersion int                `json:"manifestVersion"`
	ArchiveHash     string             `json:"archiveHash,omitempty"`
	ContentTreeHash string             `json:"contentTreeHash,omitempty"`
	ArtifactID      string             `json:"artifactId,omitempty"`
	Signature       SignatureReference `json:"signature,omitempty"`
}

type ExtensionCompatibility struct {
	MinHostVersion string   `json:"minHostVersion,omitempty"`
	MaxHostVersion string   `json:"maxHostVersion,omitempty"`
	Platforms      []string `json:"platforms,omitempty"`
	FeatureFlags   []string `json:"featureFlags,omitempty"`
}

type ExtensionIntegrity struct {
	Algorithm       string            `json:"algorithm,omitempty"`
	ContentTreeHash string            `json:"contentTreeHash,omitempty"`
	FileHashes      map[string]string `json:"fileHashes,omitempty"`
}

type ExtensionPolicies struct {
	AutoUpdate      bool   `json:"autoUpdate,omitempty"`
	BackgroundTasks bool   `json:"backgroundTasks,omitempty"`
	NetworkAccess   bool   `json:"networkAccess,omitempty"`
	Isolation       string `json:"isolation,omitempty"`
	Sandbox         bool   `json:"sandbox,omitempty"`
}

type RollbackPointReference struct {
	SnapshotID string          `json:"snapshotId"`
	Version    SemanticVersion `json:"version"`
	CreatedAt  time.Time       `json:"createdAt"`
	Reason     string          `json:"reason,omitempty"`
}

type ExtensionDefinition struct {
	ID              ExtensionID     `json:"id"`
	Name            LocalizedText   `json:"name"`
	Description     LocalizedText   `json:"description"`
	Version         SemanticVersion `json:"version"`
	ManifestVersion int             `json:"manifestVersion"`

	Publisher PublisherReference `json:"publisher"`
	Package   PackageReference   `json:"package"`

	Modules       []ModuleDefinition     `json:"modules"`
	Dependencies  []DependencyDefinition `json:"dependencies,omitempty"`
	Compatibility ExtensionCompatibility `json:"compatibility"`
	Integrity     ExtensionIntegrity     `json:"integrity"`
	Policies      ExtensionPolicies      `json:"policies"`

	Metadata map[string]any `json:"metadata,omitempty"`
}

type ExtensionPackage struct {
	PackageID       string             `json:"packageId"`
	ExtensionID     ExtensionID        `json:"extensionId"`
	Version         SemanticVersion    `json:"version"`
	ArtifactID      string             `json:"artifactId"`
	ManifestVersion int                `json:"manifestVersion"`
	ArchiveHash     string             `json:"archiveHash"`
	ContentTreeHash string             `json:"contentTreeHash"`
	Signature       SignatureReference `json:"signature"`
	Publisher       PublisherReference `json:"publisher"`
	CreatedAt       time.Time          `json:"createdAt"`
}

type ExtensionInstallation struct {
	InstallationID   string          `json:"installationId"`
	ExtensionID      ExtensionID     `json:"extensionId"`
	InstalledVersion SemanticVersion `json:"installedVersion"`
	PackageID        string          `json:"packageId"`

	InstallationState InstallationState `json:"installationState"`
	EnablementState   EnablementState   `json:"enablementState"`

	InstalledAt time.Time `json:"installedAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Generation  int64     `json:"generation"`

	ActiveSnapshotID string                   `json:"activeSnapshotId,omitempty"`
	RollbackPoints   []RollbackPointReference `json:"rollbackPoints,omitempty"`
	Metadata         map[string]any           `json:"metadata,omitempty"`
}

func (e *ExtensionDefinition) Validate() error {
	if err := ValidateExtensionID(e.ID); err != nil {
		return err
	}
	if e.Version.Major == 0 && e.Version.Minor == 0 && e.Version.Patch == 0 {
		return fmt.Errorf("domain: extension version required")
	}
	if e.Name.Default == "" {
		return fmt.Errorf("domain: extension name required")
	}
	if e.ManifestVersion == 0 {
		return fmt.Errorf("domain: manifest version required")
	}
	if len(e.Modules) == 0 {
		return fmt.Errorf("domain: at least one module required")
	}
	moduleIDs := make(map[ModuleID]struct{})
	for i, m := range e.Modules {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("domain: module[%d] invalid: %w", i, err)
		}
		if m.ExtensionID != "" && m.ExtensionID != e.ID {
			return fmt.Errorf("domain: module %s has wrong extension id", m.ID)
		}
		if _, exists := moduleIDs[m.ID]; exists {
			return fmt.Errorf("domain: duplicate module id %s", m.ID)
		}
		moduleIDs[m.ID] = struct{}{}
	}
	return nil
}

func (e *ExtensionDefinition) FindModule(id ModuleID) (*ModuleDefinition, bool) {
	for i := range e.Modules {
		if e.Modules[i].ID == id {
			return &e.Modules[i], true
		}
	}
	return nil, false
}

func (e *ExtensionDefinition) AllContributions() []ContributionDefinition {
	var out []ContributionDefinition
	for _, m := range e.Modules {
		out = append(out, m.Contributions...)
	}
	return out
}

type ModuleType string

const (
	ModuleTypeBuiltin    ModuleType = "builtin"
	ModuleTypeNative     ModuleType = "native"
	ModuleTypeJavaScript ModuleType = "javascript"
	ModuleTypeWASM       ModuleType = "wasm"
	ModuleTypeService    ModuleType = "service"
	ModuleTypeDataOnly   ModuleType = "data_only"
)

type ModuleCompatibility struct {
	MinHostVersion string   `json:"minHostVersion,omitempty"`
	Platforms      []string `json:"platforms,omitempty"`
}

type ModulePolicies struct {
	Isolation        string `json:"isolation,omitempty"`
	NetworkAccess    bool   `json:"networkAccess,omitempty"`
	FileSystemAccess bool   `json:"fileSystemAccess,omitempty"`
}

type ModuleDefinition struct {
	ID          ModuleID      `json:"id"`
	ExtensionID ExtensionID   `json:"extensionId"`
	Name        LocalizedText `json:"name"`
	Description LocalizedText `json:"description"`
	Type        ModuleType    `json:"type"`
	Version     string        `json:"version,omitempty"`

	Runtime       *RuntimeDefinition       `json:"runtime,omitempty"`
	Contributions []ContributionDefinition `json:"contributions"`
	Dependencies  []DependencyDefinition   `json:"dependencies,omitempty"`
	Compatibility ModuleCompatibility      `json:"compatibility,omitempty"`
	Policies      ModulePolicies           `json:"policies,omitempty"`
	Metadata      map[string]any           `json:"metadata,omitempty"`
}

func (m *ModuleDefinition) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("%w: empty module id", ErrInvalidModuleID)
	}
	if m.ExtensionID == "" {
		return fmt.Errorf("domain: module %s missing extension id", m.ID)
	}
	if m.Type == "" {
		return fmt.Errorf("domain: module %s missing type", m.ID)
	}
	for i, c := range m.Contributions {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("domain: module %s contribution[%d] invalid: %w", m.ID, i, err)
		}
	}
	return nil
}

type ContributionKind string

const (
	ContributionKindTool              ContributionKind = "tool"
	ContributionKindAgentSkill        ContributionKind = "agent_skill"
	ContributionKindWorkflow          ContributionKind = "workflow"
	ContributionKindMCPServer         ContributionKind = "mcp_server"
	ContributionKindHook              ContributionKind = "hook"
	ContributionKindEventSubscription ContributionKind = "event_subscription"
	ContributionKindSchedule          ContributionKind = "schedule"
	ContributionKindUIPage            ContributionKind = "ui_page"
	ContributionKindUIPanel           ContributionKind = "ui_panel"
	ContributionKindUIChat            ContributionKind = "ui_chat"
	ContributionKindUIContextAction   ContributionKind = "ui_context_action"
	ContributionKindUIDesktop         ContributionKind = "ui_desktop"
	ContributionKindBackgroundService ContributionKind = "background_service"
	ContributionKindProvider          ContributionKind = "provider"
)

type ContributionDefinition struct {
	ID          ContributionID   `json:"id"`
	ModuleID    ModuleID         `json:"moduleId"`
	ExtensionID ExtensionID      `json:"extensionId"`
	Kind        ContributionKind `json:"kind"`
	Name        LocalizedText    `json:"name"`
	Description LocalizedText    `json:"description,omitempty"`
	Version     string           `json:"version,omitempty"`

	Definition          map[string]any         `json:"definition"`
	RequiredPermissions []string               `json:"requiredPermissions,omitempty"`
	RequiredScope       []string               `json:"requiredScope,omitempty"`
	Dependencies        []DependencyDefinition `json:"dependencies,omitempty"`
	RuntimeBinding      *RuntimeBinding        `json:"runtimeBinding,omitempty"`
	Exposure            Exposure               `json:"exposure,omitempty"`

	Metadata map[string]any `json:"metadata,omitempty"`
}

func (c *ContributionDefinition) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("domain: contribution id required")
	}
	if c.Kind == "" {
		return fmt.Errorf("domain: contribution %s missing kind", c.ID)
	}
	if c.ModuleID == "" {
		return fmt.Errorf("domain: contribution %s missing module id", c.ID)
	}
	return nil
}

type Exposure struct {
	VisibleByDefault    bool     `json:"visibleByDefault,omitempty"`
	HiddenFromDiscovery bool     `json:"hiddenFromDiscovery,omitempty"`
	RequiredRoles       []string `json:"requiredRoles,omitempty"`
}

type RuntimeType string

const (
	RuntimeTypeBuiltin    RuntimeType = "builtin"
	RuntimeTypeGo         RuntimeType = "go"
	RuntimeTypeJavaScript RuntimeType = "javascript"
	RuntimeTypeWASM       RuntimeType = "wasm"
	RuntimeTypeService    RuntimeType = "service"
	RuntimeTypeMCP        RuntimeType = "mcp"
	RuntimeTypeTask       RuntimeType = "task"
)

type RuntimeDefinition struct {
	Type         RuntimeType       `json:"type"`
	EntryPoint   string            `json:"entryPoint,omitempty"`
	WorkerCount  int               `json:"workerCount,omitempty"`
	Timeout      time.Duration     `json:"timeout,omitempty"`
	Memory       int64             `json:"memory,omitempty"`
	Permissions  []string          `json:"permissions,omitempty"`
	Capabilities map[string]bool   `json:"capabilities,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
}

type RuntimeBinding struct {
	RuntimeID   RuntimeID   `json:"runtimeId"`
	RuntimeType RuntimeType `json:"runtimeType"`
	Generation  int64       `json:"generation"`
	InstanceID  string      `json:"instanceId,omitempty"`
}

type RuntimeInstance struct {
	InstanceID  string      `json:"instanceId"`
	RuntimeID   RuntimeID   `json:"runtimeId"`
	ModuleID    ModuleID    `json:"moduleId"`
	ExtensionID ExtensionID `json:"extensionId"`
	RuntimeType RuntimeType `json:"runtimeType"`
	Generation  int64       `json:"generation"`

	DesiredState string `json:"desiredState"`
	ActualState  string `json:"actualState"`
	Health       string `json:"health"`
	Circuit      string `json:"circuit"`

	StartedAt *time.Time     `json:"startedAt,omitempty"`
	StoppedAt *time.Time     `json:"stoppedAt,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type DependencyType string

const (
	DependencyTypeExtension DependencyType = "extension"
	DependencyTypeModule    DependencyType = "module"
	DependencyTypeMCP       DependencyType = "mcp"
	DependencyTypeProvider  DependencyType = "provider"
	DependencyTypeHostAPI   DependencyType = "host_api"
)

type DependencyDefinition struct {
	Type     DependencyType `json:"type"`
	ID       string         `json:"id"`
	Version  string         `json:"version,omitempty"`
	Optional bool           `json:"optional,omitempty"`
	Reason   string         `json:"reason,omitempty"`
}

type DependencyNode struct {
	ExtensionID  ExtensionID
	ModuleID     ModuleID
	Dependencies []DependencyDefinition
}

type DependencyGraph struct {
	nodes map[string]DependencyNode
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{nodes: make(map[string]DependencyNode)}
}

func nodeKey(extID ExtensionID, modID ModuleID) string {
	return string(extID) + "/" + string(modID)
}

func (g *DependencyGraph) Add(node DependencyNode) {
	key := nodeKey(node.ExtensionID, node.ModuleID)
	g.nodes[key] = node
}

func (g *DependencyGraph) DetectCycle() []string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	var path []string
	var cycle []string

	findDependents := func(dep DependencyDefinition) []string {
		var out []string
		switch dep.Type {
		case DependencyTypeExtension:
			prefix := string(dep.ID) + "/"
			for k := range g.nodes {
				if k == string(dep.ID) || strings.HasPrefix(k, prefix) {
					out = append(out, k)
				}
			}
		case DependencyTypeModule:
			if _, ok := g.nodes[dep.ID]; ok {
				out = append(out, dep.ID)
			} else {
				parts := strings.SplitN(dep.ID, "/", 2)
				if len(parts) == 2 {
					fullKey := parts[0] + "/" + parts[1]
					if _, ok := g.nodes[fullKey]; ok {
						out = append(out, fullKey)
					}
				}
			}
		}
		return out
	}

	var visit func(key string) bool
	visit = func(key string) bool {
		color[key] = gray
		path = append(path, key)
		for _, dep := range g.nodes[key].Dependencies {
			for _, depKey := range findDependents(dep) {
				if color[depKey] == gray {
					for i, p := range path {
						if p == depKey {
							cycle = append([]string{}, path[i:]...)
							cycle = append(cycle, depKey)
							return true
						}
					}
				}
				if color[depKey] == white {
					if visit(depKey) {
						return true
					}
				}
			}
		}
		color[key] = black
		path = path[:len(path)-1]
		return false
	}

	keys := make([]string, 0, len(g.nodes))
	for k := range g.nodes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if color[k] == white {
			if visit(k) {
				return cycle
			}
		}
	}
	return nil
}

func (g *DependencyGraph) TopologicalSort() ([]string, error) {
	if cycle := g.DetectCycle(); cycle != nil {
		return nil, fmt.Errorf("domain: dependency cycle: %v", cycle)
	}
	inDegree := make(map[string]int)
	for k, n := range g.nodes {
		inDegree[k] = 0
		_ = n
	}
	for _, n := range g.nodes {
		for _, dep := range n.Dependencies {
			if dep.Type != DependencyTypeExtension && dep.Type != DependencyTypeModule {
				continue
			}
			depKey := dep.ID
			if _, ok := g.nodes[depKey]; ok {
				_ = depKey
			}
		}
	}
	for k, n := range g.nodes {
		count := 0
		for _, dep := range n.Dependencies {
			if dep.Type != DependencyTypeExtension && dep.Type != DependencyTypeModule {
				continue
			}
			if _, ok := g.nodes[dep.ID]; ok {
				count++
			}
		}
		inDegree[k] = count
	}
	var queue []string
	for k, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, k)
		}
	}
	sort.Strings(queue)
	var result []string
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		result = append(result, k)
		var next []string
		for other, n := range g.nodes {
			for _, dep := range n.Dependencies {
				if dep.ID == k {
					inDegree[other]--
					if inDegree[other] == 0 {
						next = append(next, other)
					}
				}
			}
		}
		sort.Strings(next)
		queue = append(queue, next...)
	}
	return result, nil
}

type Artifact struct {
	ArtifactID   string          `json:"artifactId"`
	ExtensionID  ExtensionID     `json:"extensionId"`
	Version      SemanticVersion `json:"version"`
	Path         string          `json:"path"`
	Size         int64           `json:"size"`
	Hash         string          `json:"hash"`
	ManifestHash string          `json:"manifestHash,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	Metadata     map[string]any  `json:"metadata,omitempty"`
}

type ResourceOwnership struct {
	ResourceID   string         `json:"resourceId"`
	OwnerType    string         `json:"ownerType"`
	OwnerID      string         `json:"ownerId"`
	ResourceType string         `json:"resourceType"`
	Reference    string         `json:"reference"`
	AcquiredAt   time.Time      `json:"acquiredAt"`
	ExpiresAt    *time.Time     `json:"expiresAt,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type DefinitionRepository interface {
	PutExtension(ctx context.Context, def ExtensionDefinition) error
	GetExtension(ctx context.Context, id ExtensionID, version SemanticVersion) (ExtensionDefinition, error)
	ListExtensions(ctx context.Context) ([]ExtensionDefinition, error)
	DeleteExtension(ctx context.Context, id ExtensionID, version SemanticVersion) error
}

type InstallationRepository interface {
	PutInstallation(ctx context.Context, inst ExtensionInstallation) error
	GetInstallation(ctx context.Context, id ExtensionID) (ExtensionInstallation, error)
	ListInstallations(ctx context.Context) ([]ExtensionInstallation, error)
	DeleteInstallation(ctx context.Context, id ExtensionID) error
}

type PackageRepository interface {
	PutPackage(ctx context.Context, pkg ExtensionPackage) error
	GetPackage(ctx context.Context, packageID string) (ExtensionPackage, error)
	ListPackages(ctx context.Context, extensionID ExtensionID) ([]ExtensionPackage, error)
}

type RuntimeRepository interface {
	PutInstance(ctx context.Context, instance RuntimeInstance) error
	GetInstance(ctx context.Context, instanceID string) (RuntimeInstance, error)
	ListInstances(ctx context.Context, extensionID ExtensionID) ([]RuntimeInstance, error)
	DeleteInstance(ctx context.Context, instanceID string) error
}
