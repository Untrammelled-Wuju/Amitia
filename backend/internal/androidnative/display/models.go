package display

import (
	"time"

	"github.com/u-ai/backend/internal/androidnative/virtualdisplay"
)

type DisplayType string

const (
	DisplayTypeDefault     DisplayType = "default"
	DisplayTypeBuiltIn     DisplayType = "built_in"
	DisplayTypeExternal    DisplayType = "external"
	DisplayTypeWireless    DisplayType = "wireless"
	DisplayTypePresentation DisplayType = "presentation"
	DisplayTypeVirtualAmitia DisplayType = "virtual_amitia"
	DisplayTypeVirtualExternal DisplayType = "virtual_external"
	DisplayTypeUnknown     DisplayType = "unknown"
)

type DisplayState string

const (
	DisplayStateOn      DisplayState = "on"
	DisplayStateOff     DisplayState = "off"
	DisplayStateDoze    DisplayState = "doze"
	DisplayStateUnknown DisplayState = "unknown"
)

type DisplayFlags struct {
	Presentation            bool `json:"presentation"`
	Private                 bool `json:"private"`
	Secure                  bool `json:"secure"`
	Round                   bool `json:"round"`
	SupportsProtectedBuffers bool `json:"supportsProtectedBuffers"`
}

type DisplayCoordinateSpace struct {
	DisplayID int `json:"displayId"`
	Width     int `json:"width"`
	Height    int `json:"height"`
}

type DisplayTopologyPosition struct {
	Available       bool    `json:"available"`
	ParentDisplayID *int    `json:"parentDisplayId,omitempty"`
	OffsetX         float64 `json:"offsetX,omitempty"`
	OffsetY         float64 `json:"offsetY,omitempty"`
}

type DisplayInfo struct {
	DisplayID         int                       `json:"displayId"`
	Ref               string                    `json:"ref"`
	Generation        uint64                    `json:"generation"`
	Type              string                    `json:"type"`
	Name              string                    `json:"name"`
	IsDefault         bool                      `json:"isDefault"`
	IsValid           bool                      `json:"isValid"`
	State             string                    `json:"state"`
	Width             int                       `json:"width"`
	Height            int                       `json:"height"`
	DensityDPI        int                       `json:"densityDpi"`
	Rotation          int                       `json:"rotation"`
	RefreshRate       float64                   `json:"refreshRate,omitempty"`
	Flags             DisplayFlags              `json:"flags"`
	Presentation      bool                      `json:"presentation"`
	Private           bool                      `json:"private"`
	Secure            bool                      `json:"secure"`
	ManagedByAmitia   bool                      `json:"managedByAmitia"`
	VirtualRef        *virtualdisplay.VirtualDisplayRef `json:"virtualRef,omitempty"`
	UITreeSupported   bool                      `json:"uiTreeSupported"`
	GestureSupported  bool                      `json:"gestureSupported"`
	ScreenshotSupported bool                    `json:"screenshotSupported"`
	ScreenFrameSupported bool                   `json:"screenFrameSupported"`
	ActivityLaunchSupported bool                `json:"activityLaunchSupported"`
	CoordinateSpace   DisplayCoordinateSpace    `json:"coordinateSpace"`
	Topology          *DisplayTopologyPosition  `json:"topology,omitempty"`
}

type MultiDisplayStatus struct {
	Supported                    bool   `json:"supported"`
	DisplayCount                 int    `json:"displayCount"`
	DefaultDisplayID             int    `json:"defaultDisplayId"`
	SecondaryDisplaySupported     bool   `json:"secondaryDisplaySupported"`
	PresentationDisplayCount     int    `json:"presentationDisplayCount"`
	ManagedVirtualDisplayCount   int    `json:"managedVirtualDisplayCount"`
	UITreeMultiDisplaySupported  bool   `json:"uiTreeMultiDisplaySupported"`
	GestureMultiDisplaySupported bool   `json:"gestureMultiDisplaySupported"`
	ScreenshotMultiDisplaySupported bool `json:"screenshotMultiDisplaySupported"`
	ScreenFrameMultiDisplaySupported bool `json:"screenFrameMultiDisplaySupported"`
	TopologySupported            bool   `json:"topologySupported"`
	Generation                   uint64 `json:"generation"`
	State                        string `json:"state"`
	Reason                       string `json:"reason,omitempty"`
}

type DisplaySelectionPolicy struct {
	PreferExplicit       bool `json:"preferExplicit"`
	AllowDefaultFallback bool `json:"allowDefaultFallback"`
	RejectAmbiguous      bool `json:"rejectAmbiguous"`
}

type DisplayRecord struct {
	Info               DisplayInfo
	IdentityGeneration uint64
	FirstSeenAt        time.Time
	LastSeenAt         time.Time
	ManagedVirtualRef  *virtualdisplay.VirtualDisplayRef
}

type DisplaySnapshot struct {
	Generation       uint64        `json:"generation"`
	DefaultDisplayID int           `json:"defaultDisplayId"`
	Displays         []DisplayInfo `json:"displays"`
	CapturedAt       int64         `json:"capturedAt"`
}

type DisplayEvent_Type string

const (
	EventTypeAdded   DisplayEvent_Type = "added"
	EventTypeRemoved DisplayEvent_Type = "removed"
	EventTypeChanged DisplayEvent_Type = "changed"
)

type DisplayEvent struct {
	Type           string      `json:"type"`
	DisplayID      int         `json:"displayId"`
	Generation     uint64      `json:"generation"`
	Snapshot       DisplayInfo `json:"snapshot"`
	ChangedFields  []string    `json:"changedFields,omitempty"`
	ObservedAt     int64       `json:"observedAt"`
}

type DisplayListRequest struct {
	IncludeDefault     bool   `json:"includeDefault"`
	IncludeSecondary   bool   `json:"includeSecondary"`
	Type               string `json:"type"`
	PresentationOnly   bool   `json:"presentationOnly"`
	ManagedOnly        bool   `json:"managedOnly"`
	InteractiveOnly    bool   `json:"interactiveOnly"`
}

type DisplayGetRequest struct {
	DisplayID int    `json:"displayId"`
	Ref       string `json:"ref"`
}

type DisplayResolveRequest struct {
	DisplayID        int    `json:"displayId"`
	Ref              string `json:"ref"`
	VirtualRef       string `json:"virtualRef"`
	PreferredDisplay string `json:"preferredDisplay"`
}

type DisplayResolveResult struct {
	Target        DisplayTarget `json:"target"`
	FromCache     bool          `json:"fromCache"`
	Generation    uint64        `json:"generation"`
	Found         bool          `json:"found"`
}

type DisplayTarget struct {
	DisplayID              int                              `json:"displayId"`
	Ref                    string                           `json:"ref"`
	Generation             uint64                           `json:"generation"`
	Type                   string                           `json:"type"`
	ManagedVirtualRef      *virtualdisplay.VirtualDisplayRef `json:"virtualRef,omitempty"`
	Width                  int                              `json:"width"`
	Height                 int                              `json:"height"`
	DensityDPI             int                              `json:"densityDpi"`
	Rotation               int                              `json:"rotation"`
	State                  string                           `json:"state"`
	CoordinateSpace        DisplayCoordinateSpace           `json:"coordinateSpace"`
	UITreeSupported        bool                             `json:"uiTreeSupported"`
	GestureSupported       bool                             `json:"gestureSupported"`
	ScreenshotSupported    bool                             `json:"screenshotSupported"`
	ScreenFrameSupported   bool                             `json:"screenFrameSupported"`
	ActivityLaunchSupported bool                            `json:"activityLaunchSupported"`
}

type DisplayStatusRequest struct {
	IncludeTopology bool `json:"includeTopology"`
}
