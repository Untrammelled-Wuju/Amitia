package continuity

import "time"

type WorkflowLifecycleSignal struct {
	Type           string
	WorkflowID     string
	ExecutionID    string
	InstallationID string
	SpaceID        string
	CharacterID    string
	ConversationID string
	OperationID    string
	InvocationID   string
	DeviceID       string
	Status         string
	Error          string
	Timestamp      time.Time
}

type TaskLifecycleSignal struct {
	Type         string
	TaskRunID    string
	SpaceID      string
	OperationID  string
	InvocationID string
	DeviceID     string
	Status       string
	Reason       string
	ErrorCode    string
	Timestamp    time.Time
}

type ApprovalLifecycleSignal struct {
	Resolved       bool
	Approved       bool
	ApprovalID     string
	SpaceID        string
	ConversationID string
	RequestID      string
	ToolCallID     string
	Reason         string
	Timestamp      time.Time
}
