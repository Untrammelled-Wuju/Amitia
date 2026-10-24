package workbenchtasks

import "time"

type Assignment struct {
	ID             string   `json:"id"`
	Status         string   `json:"status"`
	Role           string   `json:"role"`
	Dependencies   []string `json:"dependencies,omitempty"`
	Objective      string   `json:"objective"`
	ReviewRequired bool     `json:"reviewRequired"`
}

type Task struct {
	CoordinationID      string       `json:"coordinationId"`
	ParentInteractionID string       `json:"parentInteractionId,omitempty"`
	TaskRunID           string       `json:"taskRunId,omitempty"`
	GoalID              string       `json:"goalId,omitempty"`
	GoalRevision        int64        `json:"goalRevision,omitempty"`
	ExecutionCoreID     string       `json:"executionCoreId,omitempty"`
	DeviceID            string       `json:"deviceId,omitempty"`
	DeadlineAt          *time.Time   `json:"deadlineAt,omitempty"`
	CancelRequested     bool         `json:"cancelRequested,omitempty"`
	Status              string       `json:"status"`
	Strategy            string       `json:"strategy,omitempty"`
	UpdatedAt           time.Time    `json:"updatedAt"`
	RecoveryState       string       `json:"recoveryState"`
	RecoveryReason      string       `json:"recoveryReason,omitempty"`
	AssignmentCount     int          `json:"assignmentCount"`
	CompletedCount      int          `json:"completedCount"`
	FailedCount         int          `json:"failedCount"`
	Assignments         []Assignment `json:"assignments"`
}

type TaskSummary struct {
	Total                        int `json:"total"`
	Active                       int `json:"active"`
	Paused                       int `json:"paused"`
	Succeeded                    int `json:"succeeded"`
	Failed                       int `json:"failed"`
	RuntimeObserved              int `json:"runtimeObserved"`
	CheckpointOnly               int `json:"checkpointOnly"`
	NeedsReconciliation          int `json:"needsReconciliation"`
	CodingWorkersRequiringReview int `json:"codingWorkersRequiringReview"`
}

type TaskList struct {
	QueryContext             *QueryContext `json:"queryContext,omitempty"`
	ProtocolVersion          int           `json:"protocolVersion"`
	ConversationID           string        `json:"conversationId"`
	Source                   string        `json:"source"`
	Tasks                    []Task        `json:"tasks"`
	Truncated                bool          `json:"truncated"`
	LiveObservationTruncated bool          `json:"liveObservationTruncated"`
	Summary                  TaskSummary   `json:"summary"`
}

func validateState(state string) bool {
	switch state {
	case "planning", "running", "waiting", "aggregating", "succeeded", "failed", "cancelled", "paused":
		return true
	default:
		return false
	}
}

func validateAssignment(state string) bool {
	switch state {
	case "pending", "running", "waiting", "succeeded", "failed", "cancelled", "paused":
		return true
	default:
		return false
	}
}

func trimObjective(value string) string {
	r := []rune(value)
	if len(r) > 220 {
		return string(r[:220])
	}
	return value
}
