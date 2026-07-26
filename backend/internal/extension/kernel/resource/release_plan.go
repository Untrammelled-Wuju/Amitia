package resource

import "time"

type ResourceReleaseRequest struct {
	ResourceID    string         `json:"resource_id"`
	RequestedBy   ResourceOwner  `json:"requested_by"`
	DryRun        bool           `json:"dry_run"`
	UserDecisions map[string]string `json:"user_decisions,omitempty"`
}

type ResourceAction struct {
	ResourceID   string        `json:"resource_id"`
	ResourceType ResourceType  `json:"resource_type"`
	Action       string        `json:"action"`
	Reason       string        `json:"reason,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type ResourceBlocker struct {
	ResourceID       string `json:"resource_id"`
	BlockerResourceID string `json:"blocker_resource_id"`
	Reason           string `json:"reason"`
	Resolution       string `json:"resolution,omitempty"`
}

type RequiredUserDecision struct {
	ResourceID   string   `json:"resource_id"`
	Question     string   `json:"question"`
	Options      []string `json:"options"`
	DefaultOption string  `json:"default_option,omitempty"`
}

type ResourceReleasePlan struct {
	PlanID             string                  `json:"plan_id"`
	RootResourceID     string                  `json:"root_resource_id"`
	DeleteResources    []ResourceAction        `json:"delete_resources"`
	RetainResources    []ResourceAction        `json:"retain_resources"`
	TransferResources  []ResourceAction        `json:"transfer_resources"`
	Blockers           []ResourceBlocker       `json:"blockers"`
	UserDecisions      []RequiredUserDecision  `json:"user_decisions"`
	RollbackSnapshotID string                  `json:"rollback_snapshot_id,omitempty"`
	CreatedAt          time.Time               `json:"created_at"`
	Metadata           map[string]any          `json:"metadata,omitempty"`
}

func (p *ResourceReleasePlan) IsBlocked() bool {
	return len(p.Blockers) > 0
}

func (p *ResourceReleasePlan) NeedsUserInput() bool {
	return len(p.UserDecisions) > 0
}

func (p *ResourceReleasePlan) TotalActions() int {
	return len(p.DeleteResources) + len(p.RetainResources) + len(p.TransferResources)
}

type ResourceReleaseResult struct {
	PlanID          string   `json:"plan_id"`
	Success         bool     `json:"success"`
	DeletedCount    int      `json:"deleted_count"`
	RetainedCount   int      `json:"retained_count"`
	TransferredCount int     `json:"transferred_count"`
	Errors          []string `json:"errors,omitempty"`
	CompletedAt     time.Time `json:"completed_at"`
}
