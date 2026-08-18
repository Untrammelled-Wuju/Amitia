package source

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/workspace"
)

type EditMode string

const (
	EditModePrecise         EditMode = "precise"
	EditModePreciseWithDiff EditMode = "precise_with_diff"
	EditModeKeyword         EditMode = "keyword_replace"
	EditModePatch           EditMode = "patch"
	EditModeAST             EditMode = "ast_edit"
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
	WorkspaceID  string                `json:"workspaceId"`
	Operations   []SourceEditOperation `json:"operations"`
	Transaction  bool                  `json:"transaction"`
	AutoCommit   bool                  `json:"autoCommit"`
	ExistingTxID string                `json:"existingTxId,omitempty"`
}

type SourceEditResult struct {
	Success           bool     `json:"success"`
	AppliedOperations int      `json:"appliedOperations"`
	TransactionToken  string   `json:"transactionToken,omitempty"`
	ChangedFiles      []string `json:"changedFiles,omitempty"`
	DiffPreview       string   `json:"diffPreview,omitempty"`
}

type SourceEditor interface {
	ApplyEdits(ctx context.Context, req SourceEditRequest) (*SourceEditResult, error)
	BeginTransaction(ctx context.Context, workspaceID string) (*workspace.EditTransaction, error)
	ApplyPatchesTx(ctx context.Context, tx *workspace.EditTransaction, req SourceEditRequest) (*SourceEditResult, error)
	PreviewDiff(ctx context.Context, tx *workspace.EditTransaction) (*workspace.DiffResult, error)
	CommitTx(ctx context.Context, tx *workspace.EditTransaction) error
	RollbackTx(ctx context.Context, tx *workspace.EditTransaction) error
}

// PreviewTransactionEditor is implemented by editors whose transaction can be
// materialized for build/runtime validation before the final commit.
type PreviewTransactionEditor interface {
	SourceEditor
	MaterializePreviewTx(ctx context.Context, tx *workspace.EditTransaction) error
	FinalizePreviewTx(ctx context.Context, tx *workspace.EditTransaction) error
}

type previewTransactionService interface {
	MaterializePreview(ctx context.Context, tx *workspace.EditTransaction) error
	FinalizePreviewCommit(ctx context.Context, tx *workspace.EditTransaction) error
}

type defaultSourceEditor struct {
	precise   workspace.PreciseEditingService
	activeTxs map[string]*workspace.EditTransaction
}

func NewSourceEditor(precise workspace.PreciseEditingService) SourceEditor {
	return &defaultSourceEditor{
		precise:   precise,
		activeTxs: make(map[string]*workspace.EditTransaction),
	}
}

func (e *defaultSourceEditor) ApplyEdits(ctx context.Context, req SourceEditRequest) (*SourceEditResult, error) {
	if req.WorkspaceID == "" {
		return nil, ErrWorkspaceIDRequired
	}
	if e.precise == nil {
		return nil, ErrPreciseUnavailable
	}
	if len(req.Operations) == 0 {
		return nil, fmt.Errorf("%w: no operations to apply", ErrUnsupportedEdit)
	}

	result := &SourceEditResult{}

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
		if req.AutoCommit {
			if err := e.precise.Commit(ctx, tx); err != nil {
				_ = e.precise.Rollback(ctx, tx)
				return nil, fmt.Errorf("%w: %v", ErrTransactionFailed, err)
			}
		} else {
			e.activeTxs[tx.ID] = tx
		}
		result.TransactionToken = tx.ID
		for path := range tx.ChangedFiles {
			result.ChangedFiles = append(result.ChangedFiles, path)
		}
		result.AppliedOperations = len(req.Operations)
		result.Success = len(result.ChangedFiles) > 0
		return result, nil
	}

	for _, op := range req.Operations {
		if err := e.applyOperation(ctx, req.WorkspaceID, op); err != nil {
			return nil, err
		}
		result.AppliedOperations++
		result.ChangedFiles = append(result.ChangedFiles, op.Path)
	}
	result.Success = result.AppliedOperations > 0
	return result, nil
}

func (e *defaultSourceEditor) BeginTransaction(ctx context.Context, workspaceID string) (*workspace.EditTransaction, error) {
	if e.precise == nil {
		return nil, ErrPreciseUnavailable
	}
	tx, err := e.precise.BeginTransaction(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransactionFailed, err)
	}
	e.activeTxs[tx.ID] = tx
	return tx, nil
}

func (e *defaultSourceEditor) ApplyPatchesTx(ctx context.Context, tx *workspace.EditTransaction, req SourceEditRequest) (*SourceEditResult, error) {
	if tx == nil {
		return nil, fmt.Errorf("%w: transaction is nil", ErrTransactionFailed)
	}
	if e.precise == nil {
		return nil, ErrPreciseUnavailable
	}
	if len(req.Operations) == 0 {
		return nil, fmt.Errorf("%w: no operations to apply", ErrUnsupportedEdit)
	}

	result := &SourceEditResult{}
	for _, op := range req.Operations {
		if err := e.applyOperationTx(ctx, tx, op); err != nil {
			_ = e.precise.Rollback(ctx, tx)
			delete(e.activeTxs, tx.ID)
			return nil, err
		}
	}
	result.TransactionToken = tx.ID
	for path := range tx.ChangedFiles {
		result.ChangedFiles = append(result.ChangedFiles, path)
	}
	result.AppliedOperations = len(req.Operations)
	result.Success = len(result.ChangedFiles) > 0
	return result, nil
}

func (e *defaultSourceEditor) PreviewDiff(ctx context.Context, tx *workspace.EditTransaction) (*workspace.DiffResult, error) {
	if tx == nil {
		return nil, fmt.Errorf("%w: transaction is nil", ErrTransactionFailed)
	}
	if e.precise == nil {
		return nil, ErrPreciseUnavailable
	}
	return e.precise.PreviewDiff(ctx, tx)
}

func (e *defaultSourceEditor) CommitTx(ctx context.Context, tx *workspace.EditTransaction) error {
	if tx == nil {
		return fmt.Errorf("%w: transaction is nil", ErrTransactionFailed)
	}
	if e.precise == nil {
		return ErrPreciseUnavailable
	}
	if err := e.precise.Commit(ctx, tx); err != nil {
		_ = e.precise.Rollback(ctx, tx)
		delete(e.activeTxs, tx.ID)
		return fmt.Errorf("%w: %v", ErrTransactionFailed, err)
	}
	delete(e.activeTxs, tx.ID)
	return nil
}

func (e *defaultSourceEditor) MaterializePreviewTx(ctx context.Context, tx *workspace.EditTransaction) error {
	previewSvc, ok := e.precise.(previewTransactionService)
	if !ok {
		return fmt.Errorf("%w: precise service does not support preview transactions", ErrPreciseUnavailable)
	}
	if err := previewSvc.MaterializePreview(ctx, tx); err != nil {
		return fmt.Errorf("%w: %v", ErrTransactionFailed, err)
	}
	return nil
}

func (e *defaultSourceEditor) FinalizePreviewTx(ctx context.Context, tx *workspace.EditTransaction) error {
	previewSvc, ok := e.precise.(previewTransactionService)
	if !ok {
		return fmt.Errorf("%w: precise service does not support preview transactions", ErrPreciseUnavailable)
	}
	if err := previewSvc.FinalizePreviewCommit(ctx, tx); err != nil {
		_ = e.precise.Rollback(ctx, tx)
		delete(e.activeTxs, tx.ID)
		return fmt.Errorf("%w: %v", ErrTransactionFailed, err)
	}
	delete(e.activeTxs, tx.ID)
	return nil
}

func (e *defaultSourceEditor) RollbackTx(ctx context.Context, tx *workspace.EditTransaction) error {
	if tx == nil {
		return fmt.Errorf("%w: transaction is nil", ErrTransactionFailed)
	}
	if e.precise == nil {
		return ErrPreciseUnavailable
	}
	err := e.precise.Rollback(ctx, tx)
	delete(e.activeTxs, tx.ID)
	return err
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
