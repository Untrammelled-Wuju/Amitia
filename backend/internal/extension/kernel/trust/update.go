package trust

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type UpdateContinuityCheck struct {
	ExtensionID          string
	PreviousVersion      string
	NewVersion           string
	PreviousPublisherID  string
	NewPublisherID       string
	PreviousKeyID        string
	NewKeyID             string
	NewKeyRotatedFromKey string
	PreviousPackageHash  string
	NewPackageHash       string
}

type UpdateContinuityResult struct {
	IsValid              bool
	IsOwnershipTransfer  bool
	IsKeyRotation        bool
	IsVersionRegression  bool
	Warnings             []string
	Reason               string
	RequiredActions      []string
}

type OwnershipTransferRequest struct {
	ExtensionID       string
	OldPublisherID    string
	NewPublisherID    string
	AuthorizationBy   string
	AcceptanceBy      string
	UserConfirmed     bool
	Reason            string
}

type OwnershipTransferResult struct {
	Success      bool
	Reason       string
	TransferID   string
	TransferredAt time.Time
}

type OwnershipTransferLog struct {
	mu      sync.RWMutex
	records []OwnershipTransferRecord
}

type OwnershipTransferRecord struct {
	TransferID    string    `json:"transfer_id"`
	ExtensionID   string    `json:"extension_id"`
	OldPublisher  string    `json:"old_publisher"`
	NewPublisher  string    `json:"new_publisher"`
	AuthorizationBy string `json:"authorization_by"`
	AcceptanceBy  string    `json:"acceptance_by"`
	Reason        string    `json:"reason"`
	TransferredAt time.Time `json:"transferred_at"`
}

func NewOwnershipTransferLog() *OwnershipTransferLog {
	return &OwnershipTransferLog{}
}

func (l *OwnershipTransferLog) Record(record OwnershipTransferRecord) {
	if record.TransferredAt.IsZero() {
		record.TransferredAt = time.Now().UTC()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, record)
}

func (l *OwnershipTransferLog) List(extensionID string) []OwnershipTransferRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if extensionID == "" {
		return l.records
	}
	var result []OwnershipTransferRecord
	for _, r := range l.records {
		if r.ExtensionID == extensionID {
			result = append(result, r)
		}
	}
	return result
}

func (l *OwnershipTransferLog) Last(extensionID string) *OwnershipTransferRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for i := len(l.records) - 1; i >= 0; i-- {
		if l.records[i].ExtensionID == extensionID {
			r := l.records[i]
			return &r
		}
	}
	return nil
}

type UpdateContinuityChecker struct {
	store           *PublisherStore
	transferLog     *OwnershipTransferLog
}

func NewUpdateContinuityChecker(store *PublisherStore, transferLog *OwnershipTransferLog) *UpdateContinuityChecker {
	return &UpdateContinuityChecker{
		store:       store,
		transferLog: transferLog,
	}
}

func (c *UpdateContinuityChecker) Check(ctx context.Context, input UpdateContinuityCheck) UpdateContinuityResult {
	result := UpdateContinuityResult{IsValid: true}

	if input.ExtensionID == "" {
		result.IsValid = false
		result.Reason = "extension id required"
		return result
	}

	if input.PreviousVersion == "" {
		return result
	}

	if input.NewPublisherID != input.PreviousPublisherID {
		result.IsOwnershipTransfer = true
		if transfer := c.transferLog.Last(input.ExtensionID); transfer == nil {
			result.IsValid = false
			result.Reason = "ownership transfer not authorized"
			result.RequiredActions = append(result.RequiredActions, "ownership_transfer_authorization")
			return result
		}
	}

	if input.NewKeyID != input.PreviousKeyID {
		result.IsKeyRotation = true
		if input.NewKeyRotatedFromKey == "" || input.NewKeyRotatedFromKey != input.PreviousKeyID {
			if input.NewPublisherID == input.PreviousPublisherID {
				newIdentity, err := c.store.Get(ctx, input.NewPublisherID)
				if err != nil {
					result.IsValid = false
					result.Reason = "new publisher not registered"
					return result
				}
				newKey := newIdentity.FindKey(input.NewKeyID)
				if newKey == nil || newKey.RotatedFrom != input.PreviousKeyID {
					result.IsValid = false
					result.Reason = "key rotation continuity broken"
					result.RequiredActions = append(result.RequiredActions, "key_rotation_continuity")
					return result
				}
				if newKey.ContinuitySignedBy == "" {
					result.Warnings = append(result.Warnings, "key rotation lacks continuity signature")
				}
			}
		}
	}

	if !isVersionIncreasing(input.PreviousVersion, input.NewVersion) {
		result.IsVersionRegression = true
		result.Warnings = append(result.Warnings, "version is not strictly increasing")
	}

	if input.PreviousPackageHash == input.NewPackageHash && input.PreviousVersion != input.NewVersion {
		result.Warnings = append(result.Warnings, "package hash unchanged across versions")
	}

	return result
}

func (c *UpdateContinuityChecker) AuthorizeTransfer(ctx context.Context, req OwnershipTransferRequest) (OwnershipTransferResult, error) {
	if req.ExtensionID == "" {
		return OwnershipTransferResult{}, errors.New("trust: extension id required")
	}
	if req.OldPublisherID == "" || req.NewPublisherID == "" {
		return OwnershipTransferResult{}, errors.New("trust: old and new publisher required")
	}
	if req.OldPublisherID == req.NewPublisherID {
		return OwnershipTransferResult{}, errors.New("trust: old and new publisher must differ")
	}
	if !req.UserConfirmed {
		return OwnershipTransferResult{}, errors.New("trust: user confirmation required")
	}

	oldIdentity, err := c.store.Get(ctx, req.OldPublisherID)
	if err != nil {
		return OwnershipTransferResult{}, fmt.Errorf("trust: old publisher not found: %w", err)
	}
	if oldIdentity.FindKey(req.AuthorizationBy) == nil {
		return OwnershipTransferResult{}, errors.New("trust: authorization key not found for old publisher")
	}

	newIdentity, err := c.store.Get(ctx, req.NewPublisherID)
	if err != nil {
		return OwnershipTransferResult{}, fmt.Errorf("trust: new publisher not found: %w", err)
	}
	if newIdentity.FindKey(req.AcceptanceBy) == nil {
		return OwnershipTransferResult{}, errors.New("trust: acceptance key not found for new publisher")
	}

	transferID := fmt.Sprintf("transfer-%s-%d", req.ExtensionID, time.Now().UnixNano())
	now := time.Now().UTC()
	c.transferLog.Record(OwnershipTransferRecord{
		TransferID:      transferID,
		ExtensionID:     req.ExtensionID,
		OldPublisher:    req.OldPublisherID,
		NewPublisher:    req.NewPublisherID,
		AuthorizationBy: req.AuthorizationBy,
		AcceptanceBy:    req.AcceptanceBy,
		Reason:          req.Reason,
		TransferredAt:   now,
	})

	return OwnershipTransferResult{
		Success:       true,
		TransferID:    transferID,
		TransferredAt: now,
		Reason:        "ownership transferred",
	}, nil
}

func isVersionIncreasing(old, new string) bool {
	if old == "" {
		return true
	}
	if new == "" {
		return false
	}
	if old == new {
		return false
	}
	return old < new
}
