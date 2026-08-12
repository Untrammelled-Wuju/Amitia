package overlay

type CapabilityState struct {
	Supported          bool   `json:"supported"`
	PermissionRequired bool   `json:"permissionRequired"`
	PermissionGranted  bool   `json:"permissionGranted"`
	NativeHostReady    bool   `json:"nativeHostReady"`

	CanCreate          bool   `json:"canCreate"`
	CanUpdate          bool   `json:"canUpdate"`
	CanInteract        bool   `json:"canInteract"`

	ActiveCount        int    `json:"activeCount"`
	UserActionRequired bool   `json:"userActionRequired"`
	State              string `json:"state"`
}

type OverlayInstance struct {
	ID        string `json:"overlayId"`
	Kind      string `json:"kind"`

	Visible   bool   `json:"visible"`
	Focusable bool   `json:"focusable"`
	Touchable bool   `json:"touchable"`

	X         int    `json:"x"`
	Y         int    `json:"y"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`

	Gravity   string `json:"gravity"`
	DisplayID int    `json:"displayId"`

	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

type TextContent struct {
	Text      string  `json:"text"`
	FontScale float64 `json:"fontScale,omitempty"`
	MaxLines  int     `json:"maxLines,omitempty"`
}

type CardContent struct {
	Title    string         `json:"title,omitempty"`
	Body     string         `json:"body,omitempty"`
	ImageURI string         `json:"imageUri,omitempty"`
	Actions  []OverlayAction `json:"actions,omitempty"`
}

type OverlayAction struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	Action      string         `json:"action"`
	ToolID      string         `json:"toolId,omitempty"`
	ToolInput   map[string]any `json:"toolInput,omitempty"`
}

type CreateRequest struct {
	Kind      string         `json:"kind"`
	Content   map[string]any `json:"content"`

	X         *int            `json:"x,omitempty"`
	Y         *int            `json:"y,omitempty"`
	Width     *int            `json:"width,omitempty"`
	Height    *int            `json:"height,omitempty"`

	Gravity   string          `json:"gravity,omitempty"`

	Focusable *bool           `json:"focusable,omitempty"`
	Touchable *bool           `json:"touchable,omitempty"`

	Draggable *bool           `json:"draggable,omitempty"`

	TTLms     *int64          `json:"ttlMs,omitempty"`
}

type CreateResult struct {
	Overlay OverlayInstance `json:"overlay"`
}

type UpdateRequest struct {
	OverlayID string         `json:"overlayId"`
	Content   map[string]any `json:"content,omitempty"`

	X         *int            `json:"x,omitempty"`
	Y         *int            `json:"y,omitempty"`
	Width     *int            `json:"width,omitempty"`
	Height    *int            `json:"height,omitempty"`

	Gravity   string          `json:"gravity,omitempty"`

	Focusable *bool           `json:"focusable,omitempty"`
	Touchable *bool           `json:"touchable,omitempty"`

	Draggable *bool           `json:"draggable,omitempty"`

	TTLms     *int64          `json:"ttlMs,omitempty"`
}

type UpdateResult struct {
	Overlay OverlayInstance `json:"overlay"`
}

type ShowRequest struct {
	OverlayID string `json:"overlayId"`
}

type ShowResult struct {
	Overlay OverlayInstance `json:"overlay"`
}

type HideRequest struct {
	OverlayID string `json:"overlayId"`
}

type HideResult struct {
	Overlay OverlayInstance `json:"overlay"`
}

type CloseRequest struct {
	OverlayID string `json:"overlayId"`
}

type CloseResult struct {
	Closed bool `json:"closed"`
}

type CloseAllResult struct {
	ClosedCount int `json:"closedCount"`
}

type ListResult struct {
	Overlays []OverlayInstance `json:"overlays"`
	Count    int               `json:"count"`
}

type PermissionResult struct {
	Opened           bool `json:"opened"`
	UserActionRequired bool `json:"userActionRequired"`
	PermissionGranted bool `json:"permissionGranted"`
}

const (
	OverlayKindText  = "text"
	OverlayKindImage = "image"
	OverlayKindCard  = "card"
	OverlayKindStatus = "status"
	OverlayKindCustom = "custom"

	StateAvailable       = "available"
	StateUnsupported     = "unsupported"
	StateHostUnavailable = "host_unavailable"
	StatePermissionRequired = "permission_required"
	StateReady           = "ready"
)

const (
	ActionDismiss   = "dismiss"
	ActionEmitEvent = "emit_event"
	ActionInvokeTool = "invoke_tool"
	ActionOpenApp   = "open_app"
)

const (
	GravityTopLeft      = "top_left"
	GravityTopRight     = "top_right"
	GravityBottomLeft   = "bottom_left"
	GravityBottomRight  = "bottom_right"
	GravityCenter       = "center"
	GravityTopCenter    = "top_center"
	GravityBottomCenter = "bottom_center"
)
