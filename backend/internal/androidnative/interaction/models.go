package interaction

import (
	"time"

	"github.com/u-ai/backend/internal/androidnative/uitree"
)

type InteractionTarget struct {
	SnapshotID string `json:"snapshotId,omitempty"`
	DisplayID  int    `json:"displayId,omitempty"`
	NodeID     string `json:"nodeId,omitempty"`

	X *int `json:"x,omitempty"`
	Y *int `json:"y,omitempty"`

	Text string `json:"text,omitempty"`

	ResourceID string `json:"resourceId,omitempty"`
	Role       string `json:"role,omitempty"`

	Description string `json:"description,omitempty"`
}

type InteractionResult struct {
	Success bool `json:"success"`

	Operation string `json:"operation"`

	Strategy string `json:"strategy"`

	SnapshotID string `json:"snapshotId,omitempty"`
	NodeID     string `json:"nodeId,omitempty"`

	X *int `json:"x,omitempty"`
	Y *int `json:"y,omitempty"`

	DisplayID int `json:"displayId,omitempty"`

	Verified     bool   `json:"verified"`
	Verification string `json:"verification,omitempty"`

	DurationMS int64 `json:"durationMs"`

	Warning string `json:"warning,omitempty"`

	// BaselineScreenStateToken is internal verification state captured before a
	// visual action. It is intentionally excluded from API responses.
	BaselineScreenStateToken string `json:"-"`
}

type InteractionPlan struct {
	Operation string

	Strategy string

	SnapshotID string
	NodeID     string

	DisplayID int

	X int
	Y int

	Text string

	Direction string

	Verify bool
}

type VisualCandidate struct {
	Source string `json:"source"`

	DisplayID            int    `json:"displayId,omitempty"`
	ScreenshotGeneration int64  `json:"screenshotGeneration,omitempty"`
	ScreenStateToken     string `json:"screenStateToken,omitempty"`

	Text        string `json:"text,omitempty"`
	Description string `json:"description,omitempty"`

	Bounds uitree.Rect `json:"bounds"`

	CenterX int `json:"centerX"`
	CenterY int `json:"centerY"`

	Confidence float64 `json:"confidence,omitempty"`

	OCRLineID string `json:"ocrLineId,omitempty"`
}

type CapabilityState struct {
	Available bool `json:"available"`

	AccessibilityAction  bool `json:"accessibilityAction"`
	AccessibilityGesture bool `json:"accessibilityGesture"`

	CoordinateTap bool `json:"coordinateTap"`
	Shizuku       bool `json:"shizuku"`
	TextInput     bool `json:"textInput"`
	Scroll        bool `json:"scroll"`

	VisualLocate             bool `json:"visualLocate"`
	OCRAvailable             bool `json:"ocrAvailable"`
	ImageUnderstandAvailable bool `json:"imageUnderstandAvailable"`

	RootFallback bool `json:"rootFallback"`
	ADBFallback  bool `json:"adbFallback"`

	State       string              `json:"state"`
	HealthState ProviderHealthState `json:"healthState"`
	Reason      string              `json:"reason,omitempty"`

	Providers map[string]ProviderCapabilityHealth `json:"providers,omitempty"`
}

type ClickRequest struct {
	Target InteractionTarget `json:"target"`

	AllowCoordinateFallback bool `json:"allowCoordinateFallback,omitempty"`
	AllowShizukuFallback    bool `json:"allowShizukuFallback,omitempty"`
	AllowVisualFallback     bool `json:"allowVisualFallback,omitempty"`
	AllowRootFallback       bool `json:"allowRootFallback,omitempty"`
	AllowADBFallback        bool `json:"allowAdbFallback,omitempty"`

	Verify bool `json:"verify,omitempty"`
}

type LongClickRequest struct {
	Target InteractionTarget `json:"target"`

	DurationMS int `json:"durationMs,omitempty"`

	AllowCoordinateFallback bool `json:"allowCoordinateFallback,omitempty"`
	AllowShizukuFallback    bool `json:"allowShizukuFallback,omitempty"`
	AllowVisualFallback     bool `json:"allowVisualFallback,omitempty"`
	AllowRootFallback       bool `json:"allowRootFallback,omitempty"`
	AllowADBFallback        bool `json:"allowAdbFallback,omitempty"`

	Verify bool `json:"verify,omitempty"`
}

type InputTextRequest struct {
	Target InteractionTarget `json:"target"`

	Text string `json:"text"`

	AllowShizukuFallback bool `json:"allowShizukuFallback,omitempty"`
	AllowRootFallback    bool `json:"allowRootFallback,omitempty"`
	AllowADBFallback     bool `json:"allowAdbFallback,omitempty"`

	Verify bool `json:"verify,omitempty"`
}

type ClearTextRequest struct {
	Target InteractionTarget `json:"target"`

	Verify bool `json:"verify,omitempty"`
}

type ScrollRequest struct {
	Target InteractionTarget `json:"target"`

	Direction string `json:"direction"`

	Amount string `json:"amount,omitempty"`

	AllowCoordinateFallback bool `json:"allowCoordinateFallback,omitempty"`
	AllowShizukuFallback    bool `json:"allowShizukuFallback,omitempty"`
	AllowRootFallback       bool `json:"allowRootFallback,omitempty"`
	AllowADBFallback        bool `json:"allowAdbFallback,omitempty"`

	Verify bool `json:"verify,omitempty"`
}

type SwipeRequest struct {
	DisplayID int `json:"displayId,omitempty"`

	StartX int `json:"startX"`
	StartY int `json:"startY"`

	EndX int `json:"endX"`
	EndY int `json:"endY"`

	DurationMS int `json:"durationMs,omitempty"`

	AllowCoordinateFallback bool `json:"allowCoordinateFallback,omitempty"`
	AllowShizukuFallback    bool `json:"allowShizukuFallback,omitempty"`
	AllowRootFallback       bool `json:"allowRootFallback,omitempty"`
	AllowADBFallback        bool `json:"allowAdbFallback,omitempty"`
}

type VisualLocateRequest struct {
	DisplayID int `json:"displayId,omitempty"`

	Description string `json:"description,omitempty"`

	Text string `json:"text,omitempty"`

	Role string `json:"role,omitempty"`

	ExpectedPackage string `json:"expectedPackage,omitempty"`

	OCRFirst bool `json:"ocrFirst,omitempty"`

	TextMatchMode string `json:"textMatchMode,omitempty"`
}

type VisualClickRequest struct {
	DisplayID   int    `json:"displayId,omitempty"`
	Description string `json:"description,omitempty"`

	Text string `json:"text,omitempty"`

	Role string `json:"role,omitempty"`

	ExpectedPackage string `json:"expectedPackage,omitempty"`

	OCRFirst bool `json:"ocrFirst,omitempty"`

	TextMatchMode string `json:"textMatchMode,omitempty"`

	Verify bool `json:"verify,omitempty"`
}

type NodeActionRequest struct {
	NativeRef string         `json:"nativeRef"`
	Action    string         `json:"action"`
	Args      map[string]any `json:"args,omitempty"`
}

type GlobalActionRequest struct {
	Action string `json:"action"`
}

type GesturePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type GestureStroke struct {
	Points       []GesturePoint `json:"points"`
	StartTimeMS  int64          `json:"startTimeMs,omitempty"`
	DurationMS   int64          `json:"durationMs,omitempty"`
	WillContinue bool           `json:"willContinue,omitempty"`
}

type GestureRequest struct {
	DisplayID int             `json:"displayId,omitempty"`
	Strokes   []GestureStroke `json:"strokes"`
}

type ScreenshotRequest struct {
	DisplayID int `json:"displayId,omitempty"`
}

func (r *ClickRequest) applyPolicyDefaults(policy Policy) {
	if r.AllowCoordinateFallback || r.AllowShizukuFallback || r.AllowVisualFallback || r.AllowRootFallback || r.AllowADBFallback {
		return
	}
	r.AllowCoordinateFallback = policy.AllowCoordinateFallback
	r.AllowShizukuFallback = policy.AllowShizukuFallback
	r.AllowVisualFallback = policy.AllowVisualFallback
	r.AllowRootFallback = policy.AllowRootFallback
	r.AllowADBFallback = policy.AllowADBFallback
}

func (r *LongClickRequest) applyPolicyDefaults(policy Policy) {
	if r.AllowCoordinateFallback || r.AllowShizukuFallback || r.AllowVisualFallback || r.AllowRootFallback || r.AllowADBFallback {
		return
	}
	r.AllowCoordinateFallback = policy.AllowCoordinateFallback
	r.AllowShizukuFallback = policy.AllowShizukuFallback
	r.AllowVisualFallback = policy.AllowVisualFallback
	r.AllowRootFallback = policy.AllowRootFallback
	r.AllowADBFallback = policy.AllowADBFallback
}

func (r *InputTextRequest) applyPolicyDefaults(policy Policy) {
	if r.AllowShizukuFallback || r.AllowRootFallback || r.AllowADBFallback {
		return
	}
	r.AllowShizukuFallback = policy.AllowShizukuFallback
	r.AllowRootFallback = policy.AllowRootFallback
	r.AllowADBFallback = policy.AllowADBFallback
}

func (r *ScrollRequest) applyPolicyDefaults(policy Policy) {
	if r.AllowCoordinateFallback || r.AllowShizukuFallback || r.AllowRootFallback || r.AllowADBFallback {
		return
	}
	r.AllowCoordinateFallback = policy.AllowCoordinateFallback
	r.AllowShizukuFallback = policy.AllowShizukuFallback
	r.AllowRootFallback = policy.AllowRootFallback
	r.AllowADBFallback = policy.AllowADBFallback
}

func (r *SwipeRequest) applyPolicyDefaults(policy Policy) {
	if r.AllowCoordinateFallback || r.AllowShizukuFallback || r.AllowRootFallback || r.AllowADBFallback {
		return
	}
	r.AllowCoordinateFallback = policy.AllowCoordinateFallback
	r.AllowShizukuFallback = policy.AllowShizukuFallback
	r.AllowRootFallback = policy.AllowRootFallback
	r.AllowADBFallback = policy.AllowADBFallback
}

type InteractionContext struct {
	ExpectedPackage  string
	ExpectedWindowID string
	Timestamp        time.Time
}

type VerificationResult struct {
	Verified bool `json:"verified"`

	Method string `json:"method,omitempty"`

	Changed bool `json:"changed,omitempty"`

	Reason string `json:"reason,omitempty"`
}

func (t *InteractionTarget) EffectiveTargetType() string {
	if t.NodeID != "" {
		return TargetNode
	}
	if t.X != nil && t.Y != nil {
		return TargetCoordinate
	}
	return TargetVisual
}

func (t *InteractionTarget) HasNode() bool {
	return t.NodeID != "" && t.SnapshotID != ""
}

func (t *InteractionTarget) HasCoordinate() bool {
	return t.X != nil && t.Y != nil
}

func (t *InteractionTarget) HasVisualDescription() bool {
	return t.Description != "" || t.Text != ""
}
