package source

import (
	"context"
)

type EditMode string

const (
	EditModePrecise       EditMode = "precise"
	EditModePreciseWithDiff EditMode = "precise_with_diff"
	EditModeKeyword       EditMode = "keyword_replace"
	EditModePatch         EditMode = "patch"
	EditModeAST           EditMode = "ast_edit"
)

type SourceEditOperation struct {
	Path          string `json:"path"`
	OldText       string `json:"oldText,omitempty"`
	NewText       string `json:"newText,omitempty"`
	Patch         string `json:"patch,omitempty"`
	SearchMode    string `json:"searchMode"`
	ReplaceAll    bool   `json:"replaceAll"`
	ExpectedCount int    `json:"expectedCount"`
}

type SourceEditRequest struct {
	WorkspaceID string                `json:"workspaceId"`
	Operations  []SourceEditOperation `json:"operations"`
	Transaction bool                  `json:"transaction"`
}

type SourceEditResult struct {
	Success           bool     `json:"success"`
	AppliedOperations int      `json:"appliedOperations"`
	TransactionToken  string   `json:"transactionToken,omitempty"`
	ChangedFiles      []string `json:"changedFiles,omitempty"`
}

type SourceEditor interface {
	ApplyEdits(ctx context.Context, req SourceEditRequest) (*SourceEditResult, error)
}

type defaultSourceEditor struct{}

func NewSourceEditor() SourceEditor {
	return &defaultSourceEditor{}
}

func (e *defaultSourceEditor) ApplyEdits(ctx context.Context, req SourceEditRequest) (*SourceEditResult, error) {
	if req.WorkspaceID == "" {
		return nil, ErrWorkspaceIDRequired
	}
	return &SourceEditResult{Success: true}, nil
}
