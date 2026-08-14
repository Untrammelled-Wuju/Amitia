package skill

import "time"

type SkillSourceDescriptor struct {
	ArtifactID   string `json:"artifactID"`
	RootURI      string `json:"rootURI"`
	SkillFileURI string `json:"skillFileURI"`
	Source       string `json:"source"`
}

type SkillDiagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Field    string `json:"field,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
}

type ParsedSkill struct {
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	License          string                 `json:"license,omitempty"`
	Compatibility    string                 `json:"compatibility,omitempty"`
	Metadata         map[string]string      `json:"metadata,omitempty"`
	AllowedTools     []string               `json:"allowedTools,omitempty"`
	Body             string                 `json:"body"`
	RawFrontmatter   map[string]interface{} `json:"rawFrontmatter,omitempty"`
	ExtraFrontmatter map[string]interface{} `json:"extraFrontmatter,omitempty"`
	Diagnostics      []SkillDiagnostic      `json:"diagnostics,omitempty"`
	ContentHash      string                 `json:"contentHash"`
	Source           SkillSourceDescriptor  `json:"source"`
}

type SkillRoot struct {
	RootURI string
	Source  string
}

type ParsePolicy struct {
	MaxFileBytes int64

	MaxFrontmatterBytes int64

	MaxYAMLDepth int
	MaxYAMLNodes int

	MaxMetadataEntries int
	MaxMetadataBytes   int64

	MaxExtraFields int
	MaxExtraBytes  int64

	CollectResourceIndex bool
}

type ResourceItem struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	SizeBytes int64  `json:"sizeBytes"`
}

type SkillParsePreview struct {
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	License             string            `json:"license"`
	Compatibility       string            `json:"compatibility"`
	Metadata            map[string]string `json:"metadata"`
	AllowedTools        []string          `json:"allowedTools"`
	BodyBytes           int               `json:"bodyBytes"`
	BodyLines           int               `json:"bodyLines"`
	ScriptsPresent      bool              `json:"scriptsPresent"`
	ResourceCounts      map[string]int    `json:"resourceCounts"`
	CompatibilityStatus string            `json:"compatibilityStatus"`
	Diagnostics         []SkillDiagnostic `json:"diagnostics"`
	ContentHash         string            `json:"contentHash"`
}

type SkillCatalogEntry struct {
	ExtensionID         string `json:"extensionID"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	CompatibilityStatus string `json:"compatibilityStatus"`
	Enabled             bool   `json:"enabled"`
}

const SkillParserVersion = 1

const (
	SkillCompatStatusCompatible = "compatible"
	SkillCompatStatusDegraded   = "degraded"
	SkillCompatStatusBlocked    = "blocked"
)

const (
	DiagnosticSeverityInfo    = "info"
	DiagnosticSeverityWarning = "warning"
	DiagnosticSeverityError   = "error"
)

const (
	DiagnosticCodeDeprecated    = "skill.meta.deprecated"
	DiagnosticCodeParserVersion = "skill.parser.version"
)

const (
	ResourceKindScript    = "script"
	ResourceKindReference = "reference"
	ResourceKindAsset     = "asset"
	ResourceKindOther     = "other"
)

var DefaultParsePolicy = ParsePolicy{
	MaxFileBytes:         1 << 20,
	MaxFrontmatterBytes:  64 << 10,
	MaxYAMLDepth:         16,
	MaxYAMLNodes:         4096,
	MaxMetadataEntries:   64,
	MaxMetadataBytes:     32 << 10,
	MaxExtraFields:       128,
	MaxExtraBytes:        64 << 10,
	CollectResourceIndex: true,
}

var StrictParsePolicy = ParsePolicy{
	MaxFileBytes:         1 << 20,
	MaxFrontmatterBytes:  64 << 10,
	MaxYAMLDepth:         16,
	MaxYAMLNodes:         4096,
	MaxMetadataEntries:   64,
	MaxMetadataBytes:     32 << 10,
	MaxExtraFields:       128,
	MaxExtraBytes:        64 << 10,
	CollectResourceIndex: false,
}

type parseResult struct {
	frontmatter       []byte
	body              []byte
	rawYAML           map[string]interface{}
	frontmatterOffset int
}

type parseContext struct {
	policy    ParsePolicy
	diags     []SkillDiagnostic
	nodeCount int
	maxDepth  int
	directory string
	startTime time.Time
}
