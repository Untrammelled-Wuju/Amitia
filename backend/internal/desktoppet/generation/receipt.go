package generation

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var (
	ErrProviderReceiptNotFound = NewGenerationError(ErrCodeProviderReceiptNotFound, "provider receipt not found", nil)
)

type ProviderReceipt struct {
	ID                string `gorm:"column:id;primaryKey;type:text" json:"id"`
	AttemptID         string `gorm:"column:attempt_id;type:text" json:"attemptId"`
	ProviderID        string `gorm:"column:provider_id;type:text" json:"providerId"`
	Model             string `gorm:"column:model;type:text" json:"model"`
	IdempotencyKey    string `gorm:"column:idempotency_key;type:text" json:"idempotencyKey"`
	ProviderRequestID string `gorm:"column:provider_request_id;type:text" json:"providerRequestId"`
	ProviderTaskID    string `gorm:"column:provider_task_id;type:text" json:"providerTaskId"`
	SubmittedAt       string `gorm:"column:submitted_at;type:text" json:"submittedAt"`
	FirstPolledAt     string `gorm:"column:first_polled_at;type:text" json:"firstPolledAt"`
	CompletedAt       string `gorm:"column:completed_at;type:text" json:"completedAt"`
	RequestHash       string `gorm:"column:request_hash;type:text" json:"requestHash"`
	ResponseHash      string `gorm:"column:response_hash;type:text" json:"responseHash"`
	ProviderStatus    string `gorm:"column:provider_status;type:text" json:"providerStatus"`
	RawMetadataJSON   string `gorm:"column:raw_metadata_json;type:text" json:"rawMetadataJson"`
	CreatedAt         string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt         string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (ProviderReceipt) TableName() string { return "desktop_pet_generation_provider_receipts" }

type ReceiptRepository interface {
	Create(tx *gorm.DB, receipt *ProviderReceipt) error
	GetByAttemptID(attemptID string) (*ProviderReceipt, error)
	GetByIdempotencyKey(key string) (*ProviderReceipt, error)
	UpdatePolled(attemptID string, firstPolledAt string, providerStatus string) error
	UpdateCompleted(attemptID string, completedAt string, responseHash string, providerStatus string, rawMetadata string) error
}

type receiptRepository struct {
	db *gorm.DB
}

func NewReceiptRepository(db *gorm.DB) ReceiptRepository {
	return &receiptRepository{db: db}
}

func (r *receiptRepository) Create(tx *gorm.DB, receipt *ProviderReceipt) error {
	if tx == nil {
		tx = r.db
	}
	if receipt.ID == "" {
		receipt.ID = generateUUID()
	}
	if receipt.CreatedAt == "" {
		receipt.CreatedAt = nowRFC3339()
	}
	if receipt.UpdatedAt == "" {
		receipt.UpdatedAt = nowRFC3339()
	}
	if err := tx.Create(receipt).Error; err != nil {
		return NewGenerationError(ErrCodeProviderReceiptPersistFailed, fmt.Sprintf("create provider receipt: %v", err), err)
	}
	return nil
}

func (r *receiptRepository) GetByAttemptID(attemptID string) (*ProviderReceipt, error) {
	var receipt ProviderReceipt
	err := r.db.Where("attempt_id = ?", attemptID).First(&receipt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderReceiptNotFound
		}
		return nil, err
	}
	return &receipt, nil
}

func (r *receiptRepository) GetByIdempotencyKey(key string) (*ProviderReceipt, error) {
	var receipt ProviderReceipt
	err := r.db.Where("idempotency_key = ? AND idempotency_key != ''", key).First(&receipt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderReceiptNotFound
		}
		return nil, err
	}
	return &receipt, nil
}

func (r *receiptRepository) UpdatePolled(attemptID string, firstPolledAt string, providerStatus string) error {
	updates := map[string]interface{}{
		"first_polled_at": firstPolledAt,
		"provider_status": providerStatus,
		"updated_at":      nowRFC3339(),
	}
	result := r.db.Model(&ProviderReceipt{}).Where("attempt_id = ?", attemptID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrProviderReceiptNotFound
	}
	return nil
}

func (r *receiptRepository) UpdateCompleted(attemptID string, completedAt string, responseHash string, providerStatus string, rawMetadata string) error {
	updates := map[string]interface{}{
		"completed_at":      completedAt,
		"response_hash":     responseHash,
		"provider_status":   providerStatus,
		"raw_metadata_json": rawMetadata,
		"updated_at":        nowRFC3339(),
	}
	result := r.db.Model(&ProviderReceipt{}).Where("attempt_id = ?", attemptID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrProviderReceiptNotFound
	}
	return nil
}
