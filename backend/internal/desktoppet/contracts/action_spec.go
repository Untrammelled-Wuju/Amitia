// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package contracts

type PlaybackMode string

const (
	PlaybackLoop     PlaybackMode = "loop"
	PlaybackOnce     PlaybackMode = "once"
	PlaybackHold     PlaybackMode = "hold"
	PlaybackPingPong PlaybackMode = "ping_pong"
)

func IsValidPlaybackMode(m string) bool {
	switch PlaybackMode(m) {
	case PlaybackLoop, PlaybackOnce, PlaybackHold, PlaybackPingPong:
		return true
	default:
		return false
	}
}

type ReturnPolicy string

const (
	ReturnNone            ReturnPolicy = "none"
	ReturnDefaultIdle     ReturnPolicy = "default_idle"
	ReturnPrevious        ReturnPolicy = "previous"
	ReturnCurrentActivity ReturnPolicy = "current_activity"
	ReturnSpecific        ReturnPolicy = "specific"
)

func IsValidReturnPolicy(p string) bool {
	switch ReturnPolicy(p) {
	case ReturnNone, ReturnDefaultIdle, ReturnPrevious, ReturnCurrentActivity, ReturnSpecific:
		return true
	default:
		return false
	}
}

type QueuePolicy string

const (
	QueueReplace       QueuePolicy = "replace"
	QueueEnqueue       QueuePolicy = "enqueue"
	QueueCoalesce      QueuePolicy = "coalesce"
	QueueDropIfRunning QueuePolicy = "drop_if_running"
)

func IsValidQueuePolicy(p string) bool {
	switch QueuePolicy(p) {
	case QueueReplace, QueueEnqueue, QueueCoalesce, QueueDropIfRunning:
		return true
	default:
		return false
	}
}

type ActionSource string

const (
	ActionSourceBuiltin ActionSource = "builtin"
	ActionSourceUser    ActionSource = "user"
	ActionSourcePlugin  ActionSource = "plugin"
)

type AnchorProfile string

const (
	AnchorFeetCenter     AnchorProfile = "feet_center"
	AnchorCenter         AnchorProfile = "center"
	AnchorEdgeContact    AnchorProfile = "edge_contact"
	AnchorSurfaceContact AnchorProfile = "surface_contact"
)

func IsValidAnchorProfile(a string) bool {
	switch AnchorProfile(a) {
	case AnchorFeetCenter, AnchorCenter, AnchorEdgeContact, AnchorSurfaceContact:
		return true
	default:
		return false
	}
}

type FramePhase struct {
	Index       int    `json:"index"`
	Description string `json:"description"`
}

type ActionIdentity struct {
	Key                 string       `json:"key"`
	Name                string       `json:"name"`
	Description         string       `json:"description"`
	CategoryKey         string       `json:"categoryKey"`
	CategoryName        string       `json:"categoryName"`
	Source              ActionSource `json:"source"`
	DefinitionVersion   int          `json:"definitionVersion"`
	Enabled             bool         `json:"enabled"`
	Recommended         bool         `json:"recommended"`
	SupportsDefaultIdle bool         `json:"supportsDefaultIdle"`
	CategorySortOrder   int          `json:"categorySortOrder"`
	ActionSortOrder     int          `json:"actionSortOrder"`
	Tags                []string     `json:"tags"`
}

type ActionGenerationSpec struct {
	Version                int          `json:"version"`
	Strategy               string       `json:"strategy"`
	FrameCount             int          `json:"frameCount"`
	FramePhases            []FramePhase `json:"framePhases"`
	MotionDescription      string       `json:"motionDescription"`
	CameraConstraint       string       `json:"cameraConstraint"`
	PoseConstraint         string       `json:"poseConstraint"`
	ContinuityConstraint   string       `json:"continuityConstraint"`
	PromptFragment         string       `json:"promptFragment"`
	NegativePromptFragment string       `json:"negativePromptFragment"`
}

type ActionPlaybackSpec struct {
	Mode             PlaybackMode `json:"mode"`
	DefaultFPS       int          `json:"defaultFps"`
	ReturnPolicy     ReturnPolicy `json:"returnPolicy"`
	ReturnActionKey  string       `json:"returnActionKey"`
	Interruptible    bool         `json:"interruptible"`
	InterruptAfterMS int          `json:"interruptAfterMs"`
	MinimumPlayMS    int          `json:"minimumPlayMs"`
	MaximumPlayMS    int          `json:"maximumPlayMs"`
	Priority         int          `json:"priority"`
	CooldownMS       int          `json:"cooldownMs"`
	MutexGroup       string       `json:"mutexGroup"`
	QueuePolicy      QueuePolicy  `json:"queuePolicy"`
	DedupWindowMS    int          `json:"dedupWindowMs"`
}

type ActionProcessingHints struct {
	AnchorProfile     AnchorProfile `json:"anchorProfile"`
	CheckLoopSeam     bool          `json:"checkLoopSeam"`
	PreserveLastFrame bool          `json:"preserveLastFrame"`
}

type ActionSpec struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Identity      ActionIdentity        `json:"identity"`
	Generation    ActionGenerationSpec  `json:"generation"`
	Playback      ActionPlaybackSpec    `json:"playback"`
	Processing    ActionProcessingHints `json:"processing"`
}

type ActionSpecSnapshot struct {
	Spec     ActionSpec `json:"spec"`
	JSON     string     `json:"json"`
	SHA256   string     `json:"sha256"`
	FrozenAt string     `json:"frozenAt"`
}

const (
	ActionSpecSchemaVersion = 1
	CatalogVersion          = 2
)

const (
	GenerationStrategySequential = "sequential_frames"
	GenerationTypeSequential     = GenerationStrategySequential
)

type CategoryDef struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
}

var BuiltinCategories = []CategoryDef{
	{Key: "idle", Name: "待机动作", SortOrder: 10},
	{Key: "movement", Name: "移动动作", SortOrder: 20},
	{Key: "interaction", Name: "基础互动", SortOrder: 30},
	{Key: "emotion", Name: "情绪动作", SortOrder: 40},
	{Key: "life", Name: "生活动作", SortOrder: 50},
	{Key: "desktop", Name: "桌面交互", SortOrder: 60},
	{Key: "dialogue", Name: "对话反馈", SortOrder: 70},
}

func CategoryByKey(key string) (CategoryDef, bool) {
	for _, c := range BuiltinCategories {
		if c.Key == key {
			return c, true
		}
	}
	return CategoryDef{}, false
}

type ActionPreset struct {
	Key           string     `json:"key"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Version       int        `json:"version"`
	ActionKeys    []string   `json:"actionKeys"`
	RequiredAnyOf [][]string `json:"requiredAnyOf"`
}

func CompatibleLoopType(mode PlaybackMode) string {
	switch mode {
	case PlaybackLoop, PlaybackPingPong:
		return "loop"
	default:
		return "once"
	}
}

func CompatibleReturnAction(policy ReturnPolicy, returnKey string) string {
	switch policy {
	case ReturnSpecific:
		return returnKey
	case ReturnDefaultIdle:
		return "idle_normal"
	default:
		return ""
	}
}
