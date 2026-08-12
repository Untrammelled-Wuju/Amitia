package uitree

import "context"

type SourceType string

const (
	SourceTypeAccessibility SourceType = "accessibility"
	SourceTypeRoot          SourceType = "root"
	SourceTypeADB           SourceType = "adb"
)

type WindowType string

const (
	WindowTypeApplication      WindowType = "application"
	WindowTypeSystem           WindowType = "system"
	WindowTypeInputMethod      WindowType = "input_method"
	WindowTypeAccessibilityOverlay WindowType = "accessibility_overlay"
	WindowTypeSplitScreen      WindowType = "split_screen"
	WindowTypePictureInPicture WindowType = "picture_in_picture"
	WindowTypeUnknown          WindowType = "unknown"
)

type Rect struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
}

func (r Rect) Width() int {
	return r.Right - r.Left
}

func (r Rect) Height() int {
	return r.Bottom - r.Top
}

func (r Rect) CenterX() int {
	return (r.Left + r.Right) / 2
}

func (r Rect) CenterY() int {
	return (r.Top + r.Bottom) / 2
}

type UIWindow struct {
	WindowID    string     `json:"windowId"`
	Type        WindowType `json:"type"`
	PackageName string     `json:"packageName,omitempty"`
	Title       string     `json:"title,omitempty"`
	Active      bool       `json:"active"`
	Focused     bool       `json:"focused"`
	DisplayID   int        `json:"displayId"`
	Bounds      Rect       `json:"bounds"`
	RootNodeID  string     `json:"rootNodeId,omitempty"`
}

type UINode struct {
	NodeID             string   `json:"nodeId"`
	ParentID           string   `json:"parentId,omitempty"`
	WindowID           string   `json:"windowId"`
	ChildIDs           []string `json:"childIds,omitempty"`
	ClassName          string   `json:"className,omitempty"`
	PackageName        string   `json:"packageName,omitempty"`
	Text               string   `json:"text,omitempty"`
	ContentDescription string   `json:"contentDescription,omitempty"`
	ResourceID         string   `json:"resourceId,omitempty"`
	Role               string   `json:"role,omitempty"`
	Bounds             Rect     `json:"bounds"`
	VisibleToUser      bool     `json:"visibleToUser"`
	Enabled            bool     `json:"enabled"`
	Focusable          bool     `json:"focusable"`
	Focused            bool     `json:"focused"`
	Selected           bool     `json:"selected"`
	Checked            bool     `json:"checked"`
	Checkable          bool     `json:"checkable"`
	Clickable          bool     `json:"clickable"`
	LongClickable      bool     `json:"longClickable"`
	Scrollable         bool     `json:"scrollable"`
	Editable           bool     `json:"editable"`
	Password           bool     `json:"password"`
	Actions            []string `json:"actions,omitempty"`
	Depth              int      `json:"depth"`
	SourceRef          string   `json:"-"`
}

type UITreeCapabilityState struct {
	Available           bool   `json:"available"`
	Source              string `json:"source"`
	MultiWindow         bool   `json:"multiWindow"`
	StableNodeReference bool   `json:"stableNodeReference"`
	CanReadText         bool   `json:"canReadText"`
	CanReadActions      bool   `json:"canReadActions"`
	SupportsFind        bool   `json:"supportsFind"`
	Degraded            bool   `json:"degraded"`
	Reason              string `json:"reason,omitempty"`
}

type UITreeSnapshot struct {
	SnapshotID      string                `json:"snapshotId"`
	Generation      int64                 `json:"generation"`
	Source          string                `json:"source"`
	CapturedAt      int64                 `json:"capturedAt"`
	ActiveWindowID  string                `json:"activeWindowId,omitempty"`
	Windows         []UIWindow            `json:"windows"`
	Nodes           []UINode              `json:"nodes"`
	NodeCount       int                   `json:"nodeCount"`
	Truncated       bool                  `json:"truncated"`
	Capability      UITreeCapabilityState `json:"capability"`
}

type SnapshotRequest struct {
	Source             string `json:"source,omitempty"`
	IncludeAllWindows  bool   `json:"includeAllWindows,omitempty"`
	MaxDepth           *int   `json:"maxDepth,omitempty"`
	IncludeInvisible   bool   `json:"includeInvisible,omitempty"`
	ExcludeOwnPackage  bool   `json:"excludeOwnPackage,omitempty"`
	AllowRootFallback  bool   `json:"allowRootFallback,omitempty"`
}

type FindRequest struct {
	SnapshotID  string `json:"snapshotId,omitempty"`
	Text        string `json:"text,omitempty"`
	ResourceID  string `json:"resourceId,omitempty"`
	ClassName   string `json:"className,omitempty"`
	Role        string `json:"role,omitempty"`
	Clickable   *bool  `json:"clickable,omitempty"`
	Editable    *bool  `json:"editable,omitempty"`
	Scrollable  *bool  `json:"scrollable,omitempty"`
	Visible     *bool  `json:"visible,omitempty"`
	MatchMode   string `json:"matchMode,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type GetRequest struct {
	SnapshotID string `json:"snapshotId"`
	NodeID     string `json:"nodeId"`
}

type ResolvedUINode struct {
	SnapshotID string
	Generation int64
	Node       UINode
	Source     SourceType
	NativeRef  string
}

type NodeResolver interface {
	ResolveNode(
		ctx context.Context,
		snapshotID string,
		nodeID string,
	) (ResolvedUINode, error)
}

type SnapshotResolver interface {
	Latest(ctx context.Context) (UITreeSnapshot, error)
	GetSnapshot(ctx context.Context, snapshotID string) (UITreeSnapshot, error)
}

type FindResult struct {
	SnapshotID string   `json:"snapshotId"`
	NodeIDs    []string `json:"nodeIds"`
	Count      int      `json:"count"`
}

type GetResult struct {
	SnapshotID string `json:"snapshotId"`
	Generation int64  `json:"generation"`
	Source     string `json:"source"`
	Node       UINode `json:"node"`
}

type StatusResult struct {
	Available          bool     `json:"available"`
	PreferredSource    string   `json:"preferredSource"`
	AvailableSources   []string `json:"availableSources"`
	AccessibilityReady bool     `json:"accessibilityConnected"`
	RootAvailable      bool     `json:"rootAvailable"`
	ADBAvailable       bool     `json:"adbAvailable"`
}

type ActionResult struct {
	Success   bool   `json:"success"`
	NodeID    string `json:"nodeId,omitempty"`
	Message   string `json:"message,omitempty"`
}
