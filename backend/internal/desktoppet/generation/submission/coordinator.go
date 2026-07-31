package submission

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/generation"
	"gorm.io/gorm"
)

type SubmitRequest struct {
	Tx                 *gorm.DB
	AttemptID          string
	TaskActionID       string
	TaskID             string
	ProviderID         string
	Model              string
	RequestPayload     interface{}
	RequestHash        string
	ActionSpecHash     string
	ProviderConfigHash string
	PromptDocumentID   string
	PromptContentHash  string
	NegativePromptHash string
}

type SubmitResult struct {
	Receipt       *generation.ProviderReceipt
	AlreadyExists bool
}

type ProviderSubmissionCoordinator struct {
	receiptRepo generation.ReceiptRepository
}

func NewProviderSubmissionCoordinator(receiptRepo generation.ReceiptRepository) *ProviderSubmissionCoordinator {
	return &ProviderSubmissionCoordinator{receiptRepo: receiptRepo}
}

func computeIdempotencyKey(taskID, taskActionID, attemptID, requestHash string) string {
	raw := fmt.Sprintf("%s:%s:%s:%s", taskID, taskActionID, attemptID, requestHash)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (c *ProviderSubmissionCoordinator) PrepareSubmission(req *SubmitRequest) (*SubmitResult, error) {
	if req.Tx == nil {
		return nil, NewSubmissionError(ErrCodeInvalidRequest, "tx is required", nil)
	}
	if req.AttemptID == "" || req.TaskID == "" || req.TaskActionID == "" {
		return nil, NewSubmissionError(ErrCodeInvalidRequest, "attemptID, taskID and taskActionID are required", nil)
	}

	idempotencyKey := computeIdempotencyKey(req.TaskID, req.TaskActionID, req.AttemptID, req.RequestHash)

	existing, err := c.receiptRepo.GetByIdempotencyKey(idempotencyKey)
	if err != nil && !errors.Is(err, generation.ErrProviderReceiptNotFound) {
		return nil, NewSubmissionError(ErrCodeReceiptCreateFailed, "check existing receipt by idempotency key", err)
	}
	if existing != nil {
		return &SubmitResult{
			Receipt:       existing,
			AlreadyExists: true,
		}, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	receipt := &generation.ProviderReceipt{
		ID:             uuid.New().String(),
		AttemptID:      req.AttemptID,
		ProviderID:     req.ProviderID,
		Model:          req.Model,
		IdempotencyKey: idempotencyKey,
		RequestHash:    req.RequestHash,
		ProviderStatus: "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := c.receiptRepo.Create(req.Tx, receipt); err != nil {
		return nil, NewSubmissionError(ErrCodeReceiptCreateFailed, "create provider receipt", err)
	}

	return &SubmitResult{
		Receipt:       receipt,
		AlreadyExists: false,
	}, nil
}

func (c *ProviderSubmissionCoordinator) RecordSubmission(tx *gorm.DB, attemptID string, providerRequestID string, providerTaskID string) error {
	if tx == nil {
		return NewSubmissionError(ErrCodeInvalidRequest, "tx is required", nil)
	}
	if attemptID == "" {
		return NewSubmissionError(ErrCodeInvalidRequest, "attemptID is required", nil)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	updates := map[string]interface{}{
		"provider_request_id": providerRequestID,
		"provider_task_id":    providerTaskID,
		"submitted_at":        now,
		"provider_status":     "submitted",
		"updated_at":          now,
	}

	result := tx.Model(&generation.ProviderReceipt{}).Where("attempt_id = ?", attemptID).Updates(updates)
	if result.Error != nil {
		return NewSubmissionError(ErrCodeReceiptUpdateFailed, "update receipt for submission", result.Error)
	}
	if result.RowsAffected == 0 {
		return NewSubmissionError(ErrCodeReceiptUpdateFailed, "receipt not found for attempt", generation.ErrProviderReceiptNotFound)
	}
	return nil
}

func (c *ProviderSubmissionCoordinator) RecordPollResult(tx *gorm.DB, attemptID string, providerStatus string, retryAfterHint *int) error {
	if tx == nil {
		return NewSubmissionError(ErrCodeInvalidRequest, "tx is required", nil)
	}
	if attemptID == "" {
		return NewSubmissionError(ErrCodeInvalidRequest, "attemptID is required", nil)
	}

	receipt, err := c.receiptRepo.GetByAttemptID(attemptID)
	if err != nil {
		return NewSubmissionError(ErrCodeReceiptUpdateFailed, "get receipt for poll update", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	updates := map[string]interface{}{
		"provider_status": providerStatus,
		"updated_at":      now,
	}

	if receipt.FirstPolledAt == "" {
		updates["first_polled_at"] = now
	}

	if retryAfterHint != nil {
		var metadata map[string]interface{}
		if receipt.RawMetadataJSON != "" {
			_ = json.Unmarshal([]byte(receipt.RawMetadataJSON), &metadata)
		}
		if metadata == nil {
			metadata = make(map[string]interface{})
		}
		metadata["retry_after_hint"] = *retryAfterHint
		data, _ := json.Marshal(metadata)
		updates["raw_metadata_json"] = string(data)
	}

	result := tx.Model(&generation.ProviderReceipt{}).Where("attempt_id = ?", attemptID).Updates(updates)
	if result.Error != nil {
		return NewSubmissionError(ErrCodeReceiptUpdateFailed, "update receipt for poll result", result.Error)
	}
	if result.RowsAffected == 0 {
		return NewSubmissionError(ErrCodeReceiptUpdateFailed, "receipt not found for attempt", generation.ErrProviderReceiptNotFound)
	}
	return nil
}

func (c *ProviderSubmissionCoordinator) RecordCompletion(tx *gorm.DB, attemptID string, providerStatus string, responseHash string, rawMetadata string) error {
	if tx == nil {
		return NewSubmissionError(ErrCodeInvalidRequest, "tx is required", nil)
	}
	if attemptID == "" {
		return NewSubmissionError(ErrCodeInvalidRequest, "attemptID is required", nil)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	updates := map[string]interface{}{
		"completed_at":      now,
		"response_hash":     responseHash,
		"provider_status":   providerStatus,
		"raw_metadata_json": rawMetadata,
		"updated_at":        now,
	}

	result := tx.Model(&generation.ProviderReceipt{}).Where("attempt_id = ?", attemptID).Updates(updates)
	if result.Error != nil {
		return NewSubmissionError(ErrCodeReceiptUpdateFailed, "update receipt for completion", result.Error)
	}
	if result.RowsAffected == 0 {
		return NewSubmissionError(ErrCodeReceiptUpdateFailed, "receipt not found for attempt", generation.ErrProviderReceiptNotFound)
	}
	return nil
}

func (c *ProviderSubmissionCoordinator) GetReceipt(attemptID string) (*generation.ProviderReceipt, error) {
	if attemptID == "" {
		return nil, NewSubmissionError(ErrCodeInvalidRequest, "attemptID is required", nil)
	}
	return c.receiptRepo.GetByAttemptID(attemptID)
}
