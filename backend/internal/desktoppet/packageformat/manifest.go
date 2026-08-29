package packageformat

const ManifestSchemaVersion = 2

const ActionConfigSchemaVersion = 2

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
	LoopTypeNone     = "none"
	LoopTypePingPong = "ping_pong"
	LoopTypeLoop     = "loop"
	LoopTypeOnce     = "once"
	LoopTypeHold     = "hold"
)

const (
	PlaybackModeLoop     = "loop"
	PlaybackModeOnce     = "once"
	PlaybackModeHold     = "hold"
	PlaybackModePingPong = "ping_pong"
)

const (
	ReturnToDefault         = "default"
	ReturnToPrevious        = "previous"
	ReturnToCurrentActivity = "current_activity"
	ReturnToNone            = "none"
	ReturnToAction          = "action"
)

const (
	IntegrityAlgorithmV2       = "amitia-package-sha256-v2"
	IntegrityAlgorithmV1Legacy = "amitia-tree-sha256-v1"
)

const (
	ManifestPseudoEntryPath = "@manifest"
)

var validPlaybackModes = map[string]bool{
	PlaybackModeLoop:     true,
	PlaybackModeOnce:     true,
	PlaybackModeHold:     true,
	PlaybackModePingPong: true,
}

func NormalizePlaybackMode(mode string) string {
	if mode == "ping-pong" || mode == "pingpong" {
		return PlaybackModePingPong
	}
	return mode
}

func IsValidPlaybackMode(mode string) bool {
	return validPlaybackModes[mode]
}

func IsLegacyLoopType(mode string) bool {
	return mode == LoopTypeNone || mode == LoopTypeLoop || mode == LoopTypeOnce || mode == LoopTypeHold || mode == LoopTypePingPong
}

func MapLegacyLoopType(loopType string) string {
	switch loopType {
	case LoopTypeLoop, LoopTypeOnce, LoopTypeHold, LoopTypePingPong:
		return loopType
	case LoopTypeNone:
		return PlaybackModeOnce
	default:
		return PlaybackModeLoop
	}
}

const (
	FileRolePreview      = "preview"
	FileRoleManifest     = "manifest"
	FileRoleActionConfig = "action-config"
	FileRoleFrame        = "frame"
	FileRoleMetadata     = "metadata"
	FileRoleAsset        = "asset"
)

const (
	QualityVerdictAccepted            = "accepted"
	QualityVerdictAcceptedWithWarning = "accepted_with_warning"
	QualityVerdictNeedsReview         = "needs_review"
	QualityVerdictRejected            = "rejected"
)

const (
	QualityVerdictPass    = "pass"
	QualityVerdictWarn    = "warning"
	QualityVerdictFail    = "fail"
	QualityVerdictSkipped = "skipped"
)

func MapLegacyQualityVerdict(verdict string) string {
	switch verdict {
	case QualityVerdictPass:
		return QualityVerdictAccepted
	case QualityVerdictWarn:
		return QualityVerdictAcceptedWithWarning
	case QualityVerdictFail:
		return QualityVerdictRejected
	case QualityVerdictSkipped:
		return QualityVerdictNeedsReview
	default:
		return verdict
	}
}

type ManifestAuthor struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type ManifestLicense struct {
	SPDX       string `json:"spdx"`
	NoticePath string `json:"noticePath"`
}

type ManifestCompatibility struct {
	MinRuntimeVersion string  `json:"minRuntimeVersion"`
	MaxRuntimeVersion *string `json:"maxRuntimeVersion,omitempty"`
	RenderMode        string  `json:"renderMode"`
}

type ManifestBinding struct {
	Policy            string `json:"policy"`
	SourceCharacterID string `json:"sourceCharacterId"`
}

type ManifestCanvas struct {
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	CoordinateSystem string `json:"coordinateSystem"`
}

type ManifestActionEntry struct {
	Key                    string `json:"key"`
	Name                   string `json:"name"`
	Config                 string `json:"config"`
	RevisionID             string `json:"revisionId"`
	QualityEvaluationID    string `json:"qualityEvaluationId"`
	QualityVerdict         string `json:"qualityVerdict"`
	PlaybackMode           string `json:"playbackMode"`
	FPS                    int    `json:"fps"`
	FrameCount             int    `json:"frameCount"`
	SupportsDefaultIdle    bool   `json:"supportsDefaultIdle"`
	IsStableStateCandidate bool   `json:"isStableStateCandidate"`
	IsTransitionOnly       bool   `json:"isTransitionOnly"`
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
	Algorithm       string              `json:"algorithm"`
	ManifestHash    string              `json:"manifestHash"`
	ContentRootHash string              `json:"contentRootHash"`
	FileCount       int                 `json:"fileCount"`
	TotalBytes      int64               `json:"totalBytes"`
	Files           []FileManifestEntry `json:"files"`
}

type ManifestProvenance struct {
	SourceType       string `json:"sourceType"`
	GenerationTaskID string `json:"generationTaskId"`
	ProcessingTaskID string `json:"processingTaskId"`
	BuiltAt          string `json:"builtAt"`
	Builder          string `json:"builder"`
}

type Manifest struct {
	SchemaVersion  int                   `json:"schemaVersion"`
	ManifestFormat string                `json:"manifestFormat"`
	PetID          string                `json:"petId"`
	ReleaseID      string                `json:"releaseId"`
	Version        string                `json:"version"`
	Name           string                `json:"name"`
	Description    string                `json:"description"`
	Author         ManifestAuthor        `json:"author"`
	License        ManifestLicense       `json:"license"`
	Compatibility  ManifestCompatibility `json:"compatibility"`
	Binding        ManifestBinding       `json:"binding"`
	Canvas         ManifestCanvas        `json:"canvas"`
	DefaultAction  string                `json:"defaultAction"`
	Preview        string                `json:"preview"`
	Actions        []ManifestActionEntry `json:"actions"`
	Capabilities   ManifestCapabilities  `json:"capabilities"`
	Integrity      ManifestIntegrity     `json:"integrity"`
	Provenance     ManifestProvenance    `json:"provenance"`
}

func NewManifest() *Manifest {
	return &Manifest{
		SchemaVersion:  ManifestSchemaVersion,
		ManifestFormat: ManifestFormatCanonical,
		Binding: ManifestBinding{
			Policy: BindingPolicyUnbound,
		},
		Compatibility: ManifestCompatibility{
			// 0.0.0 is the canonical "no minimum runtime" floor for package-v2.
			// Release builders overwrite this with the concrete runtime contract.
			MinRuntimeVersion: "0.0.0",
			RenderMode:        RenderModeSprite,
		},
		Canvas: ManifestCanvas{
			CoordinateSystem: CoordinateSystemTopLeft,
		},
		Integrity: ManifestIntegrity{
			Algorithm: IntegrityAlgorithmV2,
		},
		Capabilities: ManifestCapabilities{
			TransparentBackground: true,
			FrameSequence:         true,
		},
		Provenance: ManifestProvenance{
			SourceType: "generated",
			Builder:    "amitia-packageformat-v2",
		},
	}
}
