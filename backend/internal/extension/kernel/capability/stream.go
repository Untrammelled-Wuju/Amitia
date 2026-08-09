package capability

import "context"

type ToolStreamEventType string

const (
	ToolStreamEventContent  ToolStreamEventType = "content"
	ToolStreamEventProgress ToolStreamEventType = "progress"
	ToolStreamEventTerminal ToolStreamEventType = "terminal"
)

func (t ToolStreamEventType) Valid() bool {
	switch t {
	case ToolStreamEventContent, ToolStreamEventProgress, ToolStreamEventTerminal:
		return true
	default:
		return false
	}
}

type ToolStreamProgress struct {
	Fraction      float64 `json:"fraction,omitempty"`
	Indeterminate bool    `json:"indeterminate,omitempty"`
	Message       string  `json:"message,omitempty"`
}

type ToolStreamEmission struct {
	Type     ToolStreamEventType  `json:"type"`
	Content  *ToolContent         `json:"content,omitempty"`
	Progress *ToolStreamProgress  `json:"progress,omitempty"`
	Metadata map[string]any       `json:"metadata,omitempty"`
}

type ToolStreamEvent struct {
	InvocationID string               `json:"invocationId"`
	Sequence     uint64               `json:"sequence"`
	Type         ToolStreamEventType  `json:"type"`
	Content      *ToolContent         `json:"content,omitempty"`
	Progress     *ToolStreamProgress  `json:"progress,omitempty"`
	Result       *UnifiedToolResult   `json:"result,omitempty"`
	Metadata     map[string]any       `json:"metadata,omitempty"`
}

type ToolStreamEmitter interface {
	Emit(ctx context.Context, emission ToolStreamEmission) error
}

type ToolStreamSink interface {
	Emit(ctx context.Context, event ToolStreamEvent) error
}
