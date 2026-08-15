package preview

import (
	"context"
	"time"
)

type ObservationResult struct {
	SessionID      string      `json:"sessionId"`
	PreviewToken   string      `json:"previewToken"`
	StructuralRepr interface{} `json:"structuralRepr,omitempty"`
	ScreenshotRef  string      `json:"screenshotRef,omitempty"`
	WidgetTree     interface{} `json:"widgetTree,omitempty"`
	ChangedPaths   []string    `json:"changedPaths,omitempty"`
	Raw            interface{} `json:"raw,omitempty"`
	CapturedAt     time.Time   `json:"capturedAt"`
	CanRefine      bool        `json:"canRefine"`
}

type Observer interface {
	Capture(ctx context.Context, sessionID string) (*ObservationResult, error)
}

type defaultObserver struct{}

func NewObserver() Observer {
	return &defaultObserver{}
}

func (o *defaultObserver) Capture(ctx context.Context, sessionID string) (*ObservationResult, error) {
	return &ObservationResult{
		SessionID:  sessionID,
		CapturedAt: time.Now(),
		CanRefine:  true,
	}, nil
}
