package packageformat

const ManifestSchemaVersion = 2

const ManifestFormatCanonical = "amitia-desktop-pet"

const (
	BindingPolicyBound    = "bound"
	BindingPolicyUnbound  = "unbound"
	BindingPolicyInferred = "legacy_inferred"
)

const (
	RenderModeSprite = "sprite"
	RenderModeCanvas = "canvas"
)

const (
	CoordinateSystemTopLeft    = "top-left"
	CoordinateSystemCentered   = "centered"
	CoordinateSystemBottomLeft = "bottom-left"
)

const (
	LoopTypeNone   = "none"
	LoopTypePing   = "ping-pong"
	LoopTypeLoop   = "loop"
	LoopTypeOnce   = "once"
	LoopTypeHold   = "hold"
)

const (
	FileRolePreview     = "preview"
	FileRoleManifest    = "manifest"
	FileRoleActionConfig = "action-config"
	FileRoleFrame        = "frame"
	FileRoleMetadata     = "metadata"
	FileRoleAsset        = "asset"
)

const (
	QualityVerdictPass    = "pass"
	QualityVerdictWarn    = "warning"
	QualityVerdictFail    = "fail"
	QualityVerdictSkipped = "skipped"
)

type ManifestAuthor struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type ManifestLicense struct {
	SPDX        string `json:"spdx"`
	NoticePath  string `json:"noticePath"`
}

type ManifestCompatibility struct {
	MinRuntimeVersion  string  `json:"minRuntimeVersion"`
	MaxRuntimeVersion  *string `json:"maxRuntimeVersion,omitempty"`
	RenderMode         string  `json:"renderMode"`
}

type ManifestBinding struct {
	Policy            string `json:"policy"`
	SourceCharacterID string `json:"sourceCharacterId"`
}

type ManifestCanvas struct {
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	CoordinateSystem string `json:"coordinateSystem"`
}

type ManifestActionEntry struct {
	Key                  string `json:"key"`
	Name                 string `json:"name"`
	Config               string `json:"config"`
	RevisionID           string `json:"revisionId"`
	QualityEvaluationID  string `json:"qualityEvaluationId"`
	QualityVerdict       string `json:"qualityVerdict"`
	LoopType             string `json:"loopType"`
	FPS                  int    `json:"fps"`
	FrameCount           int    `json:"frameCount"`
	SupportsDefaultIdle  bool   `json:"supportsDefaultIdle"`
}

type ManifestCapabilities struct {
	TransparentBackground bool `json:"transparentBackground"`
	FrameSequence         bool `json:"frameSequence"`
	PerFrameDuration      bool `json:"perFrameDuration"`
	Audio                 bool `json:"audio"`
}

type FileManifestEntry struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	Bytes     int64  `json:"bytes"`
	MediaType string `json:"mediaType"`
	Role      string `json:"role"`
	ActionKey string `json:"actionKey,omitempty"`
	FrameID   string `json:"frameId,omitempty"`
}

type ManifestIntegrity struct {
	Algorithm      string              `json:"algorithm"`
	ContentRootHash string              `json:"contentRootHash"`
	FileCount      int                 `json:"fileCount"`
	TotalBytes     int64               `json:"totalBytes"`
	Files          []FileManifestEntry `json:"files"`
}

type ManifestProvenance struct {
	SourceType        string `json:"sourceType"`
	GenerationTaskID  string `json:"generationTaskId"`
	ProcessingTaskID  string `json:"processingTaskId"`
	BuiltAt           string `json:"builtAt"`
	Builder           string `json:"builder"`
}

type Manifest struct {
	SchemaVersion  int                    `json:"schemaVersion"`
	ManifestFormat string                 `json:"manifestFormat"`
	PetID          string                 `json:"petId"`
	ReleaseID      string                 `json:"releaseId"`
	Version        string                 `json:"version"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Author         ManifestAuthor         `json:"author"`
	License        ManifestLicense        `json:"license"`
	Compatibility  ManifestCompatibility  `json:"compatibility"`
	Binding        ManifestBinding        `json:"binding"`
	Canvas         ManifestCanvas         `json:"canvas"`
	DefaultAction  string                 `json:"defaultAction"`
	Preview        string                 `json:"preview"`
	Actions        []ManifestActionEntry  `json:"actions"`
	Capabilities   ManifestCapabilities   `json:"capabilities"`
	Integrity      ManifestIntegrity      `json:"integrity"`
	Provenance     ManifestProvenance     `json:"provenance"`
}

func NewManifest() *Manifest {
	return &Manifest{
		SchemaVersion:  ManifestSchemaVersion,
		ManifestFormat: ManifestFormatCanonical,
		Binding: ManifestBinding{
			Policy: BindingPolicyUnbound,
		},
		Compatibility: ManifestCompatibility{
			RenderMode: RenderModeSprite,
		},
		Canvas: ManifestCanvas{
			CoordinateSystem: CoordinateSystemTopLeft,
		},
		Integrity: ManifestIntegrity{
			Algorithm: TreeHashAlgorithm,
		},
		Capabilities: ManifestCapabilities{
			TransparentBackground: true,
			FrameSequence:         true,
		},
	}
}
