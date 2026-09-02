package virtualdisplay

import (
	"time"
)

type VirtualDisplayRef string

func (r VirtualDisplayRef) String() string { return string(r) }
func (r VirtualDisplayRef) IsEmpty() bool  { return string(r) == "" }

type State string

const (
	StateCreating  State = "creating"
	StateReady     State = "ready"
	StatePaused    State = "paused"
	StateResizing  State = "resizing"
	StateReleasing State = "releasing"
	StateReleased  State = "released"
	StateFailed    State = "failed"
)

func (s State) IsActive() bool {
	switch s {
	case StateCreating, StateReady, StatePaused, StateResizing:
		return true
	}
	return false
}

func (s State) IsTerminal() bool {
	switch s {
	case StateReleased, StateFailed:
		return true
	}
	return false
}

type VirtualDisplayInfo struct {
	Ref             VirtualDisplayRef `json:"ref"`
	DisplayID       int               `json:"displayId"`
	Generation      uint64            `json:"generation"`
	Name            string            `json:"name"`
	Width           int               `json:"width"`
	Height          int               `json:"height"`
	DensityDPI      int               `json:"densityDpi"`
	Rotation        int               `json:"rotation"`
	RefreshRate     float64           `json:"refreshRate,omitempty"`
	SurfaceAttached bool              `json:"surfaceAttached"`
	State           string            `json:"state"`
	CreatedAt       int64             `json:"createdAt"`
}

type VirtualDisplayRecord struct {
	Ref             VirtualDisplayRef
	DisplayID       int
	Generation      uint64
	Name            string
	Width           int
	Height          int
	DensityDPI      int
	Rotation        int
	RefreshRate     float64
	SurfaceAttached bool
	State           State
	CreatedAt       time.Time
}

type DisplayTarget struct {
	DisplayID  int
	Generation uint64
	Width      int
	Height     int
	DPI        int
	Rotation   int
}

type CreateRequest struct {
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	DensityDPI  int     `json:"densityDpi"`
	RefreshRate float64 `json:"refreshRate"`
	ZOrder      int     `json:"zOrder"`
}

type CreateResult struct {
	Display                   VirtualDisplayInfo `json:"display"`
	FrameSourceReady          bool               `json:"frameSourceReady"`
	ThirdPartyLaunchSupported bool               `json:"thirdPartyLaunchSupported"`
	UITreeSupported           bool               `json:"uiTreeSupported"`
	GestureSupported          bool               `json:"gestureSupported"`
}

type GetRequest struct {
	Ref VirtualDisplayRef `json:"ref"`
}

type ResizeRequest struct {
	Ref        VirtualDisplayRef `json:"ref"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	DensityDPI int               `json:"densityDpi"`
}

type ResizeResult struct {
	Display VirtualDisplayInfo `json:"display"`
}

type ReleaseRequest struct {
	Ref VirtualDisplayRef `json:"ref"`
}

type ReleaseResult struct {
	Released  bool   `json:"released"`
	WasActive bool   `json:"wasActive"`
	State     string `json:"state"`
	Status    string `json:"status"`
}

type StatusResult struct {
	Supported                 bool                 `json:"supported"`
	FeatureSecondaryDisplays  bool                 `json:"featureSecondaryDisplays"`
	CanCreate                 bool                 `json:"canCreate"`
	Active                    bool                 `json:"active"`
	ActiveCount               int                  `json:"activeCount"`
	Display                   *VirtualDisplayInfo  `json:"display,omitempty"`
	Displays                  []VirtualDisplayInfo `json:"displays,omitempty"`
	FrameSourceSupported      bool                 `json:"frameSourceSupported"`
	UITreeSupported           bool                 `json:"uiTreeSupported"`
	GestureSupported          bool                 `json:"gestureSupported"`
	ThirdPartyLaunchSupported bool                 `json:"thirdPartyLaunchSupported"`
	State                     string               `json:"state"`
	Reason                    string               `json:"reason"`
}

type DisplayLaunchTarget struct {
	DisplayID  int
	Generation uint64
}
