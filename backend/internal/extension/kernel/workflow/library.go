package workflow

import (
	"encoding/json"
	"time"
)

type WorkflowRevisionState string

const (
	WorkflowRevisionDraft     WorkflowRevisionState = "draft"
	WorkflowRevisionPublished WorkflowRevisionState = "published"
	WorkflowRevisionArchived  WorkflowRevisionState = "archived"
)

func (s WorkflowRevisionState) Valid() bool {
	switch s {
	case WorkflowRevisionDraft, WorkflowRevisionPublished, WorkflowRevisionArchived:
		return true
	default:
		return false
	}
}

// WorkflowRevision is an immutable local snapshot of a user workflow.
// Revisions are intentionally local-only and scoped to the owning user.
type WorkflowRevision struct {
	RevisionID     string                `json:"revisionId"`
	WorkflowID     string                `json:"workflowId"`
	OwnerUserID    string                `json:"-"`
	RevisionNo     int64                 `json:"revisionNo"`
	Name           string                `json:"name"`
	Description    string                `json:"description"`
	Definition     WorkflowDefinition    `json:"definition"`
	DefinitionHash string                `json:"definitionHash"`
	Note           string                `json:"note,omitempty"`
	State          WorkflowRevisionState `json:"state"`
	PublishedAt    *time.Time            `json:"publishedAt,omitempty"`
	ArchivedAt     *time.Time            `json:"archivedAt,omitempty"`
	CreatedAt      time.Time             `json:"createdAt"`
}

// WorkflowRevisionSummary is the lightweight representation used by history lists.
type WorkflowRevisionSummary struct {
	RevisionID     string                `json:"revisionId"`
	WorkflowID     string                `json:"workflowId"`
	RevisionNo     int64                 `json:"revisionNo"`
	Name           string                `json:"name"`
	Description    string                `json:"description"`
	DefinitionHash string                `json:"definitionHash"`
	Note           string                `json:"note,omitempty"`
	State          WorkflowRevisionState `json:"state"`
	PublishedAt    *time.Time            `json:"publishedAt,omitempty"`
	ArchivedAt     *time.Time            `json:"archivedAt,omitempty"`
	Current        bool                  `json:"current,omitempty"`
	Installed      bool                  `json:"installed,omitempty"`
	Running        bool                  `json:"running,omitempty"`
	EffectiveState string                `json:"effectiveState,omitempty"`
	CreatedAt      time.Time             `json:"createdAt"`
}

// WorkflowTemplate stores a reusable local workflow definition. Templates are
// not registered with the runtime and therefore cannot execute on their own.
type WorkflowTemplate struct {
	TemplateID     string             `json:"templateId"`
	OwnerUserID    string             `json:"-"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Definition     WorkflowDefinition `json:"definition"`
	DefinitionHash string             `json:"definitionHash"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}

// WorkflowTemplateSummary avoids returning the full DAG when only a picker is needed.
type WorkflowTemplateSummary struct {
	TemplateID     string    `json:"templateId"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	DefinitionHash string    `json:"definitionHash"`
	NodeCount      int       `json:"nodeCount"`
	TriggerCount   int       `json:"triggerCount"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// WorkflowExportEnvelope is the stable local import/export contract.
type WorkflowExportEnvelope struct {
	Format        string             `json:"format"`
	FormatVersion int                `json:"formatVersion"`
	ExportedAt    time.Time          `json:"exportedAt"`
	Workflow      WorkflowDefinition `json:"workflow"`
}

func CloneDefinition(def WorkflowDefinition) (WorkflowDefinition, error) {
	raw, err := json.Marshal(def)
	if err != nil {
		return WorkflowDefinition{}, err
	}
	var clone WorkflowDefinition
	if err := json.Unmarshal(raw, &clone); err != nil {
		return WorkflowDefinition{}, err
	}
	return clone, nil
}
