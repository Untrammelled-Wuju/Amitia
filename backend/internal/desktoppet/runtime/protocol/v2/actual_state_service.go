package v2

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ActualStateService interface {
	Upsert(state *RuntimeActualState) error
	Get(runtimeID, installationID string) (*RuntimeActualState, error)
	ListByRuntime(runtimeID string) ([]*RuntimeActualState, error)
	ListByUser(userID string) ([]*RuntimeActualState, error)
	Delete(runtimeID, installationID string) error
	UpdateHealth(runtimeID, health string) error
	RefreshLease(reconcilerID string, now time.Time) (bool, error)
	ReleaseLease(reconcilerID string) error
	ReapExpiredLease(horizon time.Time, reconcilerID string) (bool, error)
	AppendDomainEvent(eventType, aggregateID string, payload []byte, t time.Time, idemKey *string) (bool, error)
	ClaimDomainEvent(eventID string, now time.Time, claimTTL time.Duration) (bool, error)
	MarkDomainEventSent(eventID string, now time.Time) error
	MarkDomainEventFailed(eventID string, attempt int, errMsg string) error
	FailAbandonedEvents(threshold time.Time) (int64, error)
	DB() *gorm.DB
}

type actualStateService struct {
	db *gorm.DB
}

func NewActualStateService(db *gorm.DB) ActualStateService {
	return &actualStateService{db: db}
}

func (s *actualStateService) DB() *gorm.DB { return s.db }

func (s *actualStateService) Upsert(state *RuntimeActualState) error {
	now := time.Now().Format("2006-01-02 15:04:05")

	var existing RuntimeActualState
	err := s.db.Where(
		"user_id = ? AND device_id = ? AND runtime_id = ?",
		state.UserID, state.DeviceID, state.RuntimeID,
	).First(&existing).Error

	if err == nil {
		if !existing.CanUpdate(state.ConnectionGeneration, state.LastEventSequence) {
			return nil
		}
		state.UpdatedAt = now
		return s.db.Save(state).Error
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		state.UpdatedAt = now
		return s.db.Create(state).Error
	}
	return err
}

func (s *actualStateService) Get(runtimeID, installationID string) (*RuntimeActualState, error) {
	var state RuntimeActualState
	err := s.db.Where(
		"runtime_id = ? AND installation_id = ?", runtimeID, installationID,
	).Order("updated_at DESC").First(&state).Error
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *actualStateService) ListByRuntime(runtimeID string) ([]*RuntimeActualState, error) {
	var states []*RuntimeActualState
	err := s.db.Where("runtime_id = ?", runtimeID).
		Order("updated_at DESC").Find(&states).Error
	if err != nil {
		return nil, err
	}
	return states, nil
}

func (s *actualStateService) ListByUser(userID string) ([]*RuntimeActualState, error) {
	var states []*RuntimeActualState
	err := s.db.Where("user_id = ?", userID).
		Order("updated_at DESC").Find(&states).Error
	if err != nil {
		return nil, err
	}
	return states, nil
}

func (s *actualStateService) Delete(runtimeID, installationID string) error {
	return s.db.Where(
		"runtime_id = ? AND installation_id = ?", runtimeID, installationID,
	).Delete(&RuntimeActualState{}).Error
}

func (s *actualStateService) UpdateHealth(runtimeID, health string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	return s.db.Model(&RuntimeActualState{}).Where("runtime_id = ?", runtimeID).
		Updates(map[string]interface{}{
			"health_status": health,
			"updated_at":    now,
		}).Error
}

func (s *actualStateService) RefreshLease(reconcilerID string, now time.Time) (bool, error) {
	nowStr := now.Format("2006-01-02 15:04:05")
	result := s.db.Model(&ReconcileLease{}).Where(
		"reconciler_id = ?", reconcilerID,
	).Updates(map[string]interface{}{
		"last_heartbeat_at": nowStr,
		"updated_at":        nowStr,
	})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return true, nil
	}

	lease := ReconcileLease{
		ReconcilerID:    reconcilerID,
		LastHeartbeatAt: nowStr,
		InsertedAt:      nowStr,
		UpdatedAt:       nowStr,
	}
	if err := s.db.Create(&lease).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (s *actualStateService) ReleaseLease(reconcilerID string) error {
	return s.db.Where("reconciler_id = ?", reconcilerID).Delete(&ReconcileLease{}).Error
}

func (s *actualStateService) ReapExpiredLease(horizon time.Time, reconcilerID string) (bool, error) {
	var lease ReconcileLease
	err := s.db.Where(
		"reconciler_id != ? AND last_heartbeat_at < ?",
		reconcilerID, horizon.Format("2006-01-02 15:04:05"),
	).Order("last_heartbeat_at ASC").First(&lease).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	result := s.db.Where(
		"reconciler_id = ? AND last_heartbeat_at < ?",
		lease.ReconcilerID, horizon.Format("2006-01-02 15:04:05"),
	).Delete(&ReconcileLease{})

	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return s.RefreshLease(reconcilerID, time.Now())
	}
	return true, nil
}

func (s *actualStateService) AppendDomainEvent(eventType, aggregateID string, payload []byte, t time.Time, idemKey *string) (bool, error) {
	now := t.Format("2006-01-02 15:04:05")

	if idemKey != nil && *idemKey != "" {
		var existing DomainEventOutbox
		err := s.db.Where("idempotency_key = ?", *idemKey).First(&existing).Error
		if err == nil {
			return false, nil
		}
	}

	outbox := &DomainEventOutbox{
		ID:          "dtev_" + uuid.NewString(),
		EventType:   eventType,
		AggregateID: aggregateID,
		Payload:     payload,
		Status:      OutboxStatusPending,
		Attempt:     0,
		InsertedAt:  now,
		UpdatedAt:   now,
		PublishedAt: "",
	}

	if idemKey != nil {
		outbox.IdempotencyKey = *idemKey
	}

	if err := s.db.Create(outbox).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (s *actualStateService) ClaimDomainEvent(eventID string, now time.Time, claimTTL time.Duration) (bool, error) {
	claimExpiry := now.Add(claimTTL).Format("2006-01-02 15:04:05")
	nowStr := now.Format("2006-01-02 15:04:05")

	result := s.db.Model(&DomainEventOutbox{}).Where(
		"id = ? OR (status = ? AND (claim_expires_at < ? OR claim_expires_at = ''))",
		eventID, OutboxStatusPending, nowStr,
	).Order("inserted_at ASC").Limit(1).Updates(map[string]interface{}{
		"status":           OutboxStatusClaimed,
		"claim_expires_at": claimExpiry,
		"updated_at":       nowStr,
	})

	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *actualStateService) MarkDomainEventSent(eventID string, now time.Time) error {
	publishedAt := now.Format("2006-01-02 15:04:05")
	return s.db.Model(&DomainEventOutbox{}).Where("id = ?", eventID).Updates(map[string]interface{}{
		"status":       OutboxStatusSent,
		"published_at": publishedAt,
		"updated_at":   publishedAt,
	}).Error
}

func (s *actualStateService) MarkDomainEventFailed(eventID string, attempt int, errMsg string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	return s.db.Model(&DomainEventOutbox{}).Where("id = ?", eventID).Updates(map[string]interface{}{
		"attempt":    attempt,
		"last_error": errMsg,
		"updated_at": now,
	}).Error
}

func (s *actualStateService) FailAbandonedEvents(threshold time.Time) (int64, error) {
	thresholdStr := threshold.Format("2006-01-02 15:04:05")
	now := time.Now().Format("2006-01-02 15:04:05")
	result := s.db.Model(&DomainEventOutbox{}).Where(
		"status = ? AND inserted_at < ?",
		OutboxStatusPending, thresholdStr,
	).Updates(map[string]interface{}{
		"status":     OutboxStatusFailed,
		"last_error": "expired",
		"updated_at": now,
	})
	return result.RowsAffected, result.Error
}
