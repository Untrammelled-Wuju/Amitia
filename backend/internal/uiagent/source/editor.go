package source

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/workspace"
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
	BaseSHA256    string `json:"baseSha256,omitempty"`
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

type defaultSourceEditor struct {
	precise workspace.PreciseEditingService
}

func NewSourceEditor(precise workspace.PreciseEditingService) SourceEditor {
	return &defaultSourceEditor{precise: precise}
}

func (e *defaultSourceEditor) ApplyEdits(ctx context.Context, req SourceEditRequest) (*SourceEditResult, error) {
	if req.WorkspaceID == "" {
		return nil, ErrWorkspaceIDRequired
	}
	if e.precise == nil {
		return nil, ErrPreciseUnavailable
	}

	result := &SourceEditResult{Success: true}

	if req.Transaction {
		tx, err := e.precise.BeginTransaction(ctx, req.WorkspaceID)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrTransactionFailed, err)
		}
		for _, op := range req.Operations {
			if err := e.applyOperationTx(ctx, tx, op); err != nil {
				_ = e.precise.Rollback(ctx, tx)
				return nil, err
			}
		}
		if err := e.precise.Commit(ctx, tx); err != nil {
			_ = e.precise.Rollback(ctx, tx)
			return nil, fmt.Errorf("%w: %v", ErrTransactionFailed, err)
		}
		result.TransactionToken = tx.ID
		for path := range tx.ChangedFiles {
			result.ChangedFiles = append(result.ChangedFiles, path)
		}
		result.AppliedOperations = len(req.Operations)
		return result, nil
	}

	for _, op := range req.Operations {
		if err := e.applyOperation(ctx, req.WorkspaceID, op); err != nil {
			return nil, err
		}
		result.AppliedOperations++
		result.ChangedFiles = append(result.ChangedFiles, op.Path)
	}
	return result, nil
}

func (e *defaultSourceEditor) applyOperation(ctx context.Context, workspaceID string, op SourceEditOperation) error {
	switch {
	case op.Patch != "":
		_, err := e.precise.Patch(ctx, workspace.PatchRequest{
			WorkspaceID: workspaceID,
			FilePath:    op.Path,
			Patch:       op.Patch,
		})
		return err
	case op.OldText != "":
		replaceReq := workspace.ReplaceRequest{
			WorkspaceID: workspaceID,
			FilePath:    op.Path,
			OldText:     op.OldText,
			NewText:     op.NewText,
		}
		if op.ExpectedCount > 0 {
			replaceReq.ExpectedOccurrences = op.ExpectedCount
		}
		if op.ReplaceAll {
			replaceReq.ExpectedOccurrences = 0
		}
		_, err := e.precise.Replace(ctx, replaceReq)
		return err
	default:
		return fmt.Errorf("%w: operation on %q has neither patch nor oldText", ErrUnsupportedEdit, op.Path)
	}
}

func (e *defaultSourceEditor) applyOperationTx(ctx context.Context, tx *workspace.EditTransaction, op SourceEditOperation) error {
	if op.Patch != "" {
		_, err := e.precise.ApplyPatchTx(ctx, tx, workspace.PatchRequest{
			WorkspaceID: tx.WorkspaceID,
			FilePath:    op.Path,
			BaseSHA256:  op.BaseSHA256,
			Patch:       op.Patch,
		})
		return err
	}
	return fmt.Errorf("%w: transactional edit on %q requires patch op", ErrUnsupportedEdit, op.Path)
}
