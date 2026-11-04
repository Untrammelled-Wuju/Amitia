// Package v1 provides the DesktopPet runtime protocol v1 implementation.
//
// DesktopPet session rows are a read-only projection of deviceruntime.Service.
// Session IDs must match deviceruntime.RuntimeSession.ID exactly.
package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"gorm.io/gorm"
)

// SessionService is the DesktopPet domain session projection store interface.
//
// Production primary path: SyncFromDeviceRuntimeSession (sync from G12 authoritative session).
type SessionService interface {
	GetSession(id string) (*RuntimeSession, error)
	GetActiveSession(spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (*RuntimeSession, error)
	SyncFromDeviceRuntimeSession(ctx context.Context, runtimeSession deviceruntime.RuntimeSession, hello HelloPayload) (*RuntimeSession, error)
	UpdateLastAppliedRevision(id string, revision int64) error
	UpdateLastProcessedCommandSequence(id string, seq int64) error
	UpdateLastEventSequence(id string, seq int64) error
	UpdateLastHeartbeat(id string) error
	SupersedeSession(id, supersededBy string) error
	DeleteTerminalBefore(cutoff time.Time) (int64, error)
	DB() *gorm.DB
}

type sessionService struct {
	db *gorm.DB
}

func NewSessionService(db *gorm.DB) SessionService {
	return &sessionService{db: db}
}

func (s *sessionService) DB() *gorm.DB { return s.db }

func (s *sessionService) GetSession(id string) (*RuntimeSession, error) {
	var session RuntimeSession
	err := s.db.Where("id = ?", id).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *sessionService) GetActiveSession(spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (*RuntimeSession, error) {
	var session RuntimeSession
	err := s.db.Where(
		"space_id = ? AND device_id = ? AND runtime_id = ? AND status IN (?, ?, ?, ?)",
		spaceID.String(), deviceID.String(), runtimeID.String(),
		SessionStatusRegistering, SessionStatusSyncing, SessionStatusReady, SessionStatusDegraded,
	).Order("connection_generation DESC").First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *sessionService) UpdateLastAppliedRevision(id string, revision int64) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("runtime session id required")
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	return s.db.Model(&RuntimeSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_applied_desired_revision": gorm.Expr("CASE WHEN last_applied_desired_revision < ? THEN ? ELSE last_applied_desired_revision END", revision, revision),
		"updated_at":                    now,
	}).Error
}

func (s *sessionService) UpdateLastProcessedCommandSequence(id string, seq int64) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("runtime session id required")
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	return s.db.Model(&RuntimeSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_processed_command_sequence": gorm.Expr("CASE WHEN last_processed_command_sequence < ? THEN ? ELSE last_processed_command_sequence END", seq, seq),
		"updated_at":                      now,
	}).Error
}

func (s *sessionService) UpdateLastEventSequence(id string, seq int64) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("runtime session id required")
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	return s.db.Model(&RuntimeSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_event_sequence": gorm.Expr("CASE WHEN last_event_sequence < ? THEN ? ELSE last_event_sequence END", seq, seq),
		"updated_at":          now,
	}).Error
}

func (s *sessionService) UpdateLastHeartbeat(id string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	return s.db.Model(&RuntimeSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_heartbeat_at": now,
		"updated_at":        now,
	}).Error
}

func (s *sessionService) SupersedeSession(id, supersededBy string) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	result := s.db.Model(&RuntimeSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        SessionStatusSuperseded,
		"superseded_by": supersededBy,
		"superseded_at": now,
		"updated_at":    now,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("supersede runtime session %s: expected 1 row, got %d", id, result.RowsAffected)
	}
	return nil
}

func (s *sessionService) SyncFromDeviceRuntimeSession(ctx context.Context, runtimeSession deviceruntime.RuntimeSession, hello HelloPayload) (*RuntimeSession, error) {
	var existing RuntimeSession
	err := s.db.Where("id = ?", runtimeSession.ID.String()).First(&existing).Error

	now := time.Now().Format("2006-01-02 15:04:05")

	if err == gorm.ErrRecordNotFound {
		session := &RuntimeSession{
			ID:                           runtimeSession.ID.String(),
			SpaceID:                      runtimeSession.SpaceID,
			DeviceID:                     runtimeSession.DeviceID,
			RuntimeID:                    runtimeSession.RuntimeID,
			ConnectionGeneration:         runtimeSession.ConnectionGeneration,
			RuntimeVersion:               runtimeSession.RuntimeVersion,
			RuntimeContractVersion:       runtimeSession.RuntimeContractVersion,
			CapabilitiesHash:             runtimeSession.CapabilitiesHash,
			LastAppliedDesiredRevision:   hello.LastAppliedDesiredRevision,
			LastProcessedCommandSequence: hello.LastProcessedCommandSequence,
			LastEventSequence:            hello.LastEventSequence,
			Status:                       string(runtimeSession.Status),
			ConnectedAt:                  now,
			LastHeartbeatAt:              now,
			CreatedAt:                    now,
			UpdatedAt:                    now,
		}

		capsJSON, marshalErr := json.Marshal(hello.Capabilities)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal runtime capabilities: %w", marshalErr)
		}
		if len(capsJSON) > 0 {
			session.CapabilitiesJSON = string(capsJSON)
		}

		if createErr := s.db.Create(session).Error; createErr != nil {
			return nil, createErr
		}
		return session, nil
	}

	if err != nil {
		return nil, err
	}

	existing.ConnectionGeneration = runtimeSession.ConnectionGeneration
	existing.RuntimeVersion = runtimeSession.RuntimeVersion
	existing.RuntimeContractVersion = runtimeSession.RuntimeContractVersion
	existing.CapabilitiesHash = runtimeSession.CapabilitiesHash
	existing.Status = string(runtimeSession.Status)
	existing.UpdatedAt = now

	capsJSON, marshalErr := json.Marshal(hello.Capabilities)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal runtime capabilities: %w", marshalErr)
	}
	if len(capsJSON) > 0 {
		existing.CapabilitiesJSON = string(capsJSON)
	}

	if updateErr := s.db.Model(&RuntimeSession{}).Where("id = ?", runtimeSession.ID.String()).Updates(map[string]interface{}{
		"connection_generation":           existing.ConnectionGeneration,
		"runtime_version":                 existing.RuntimeVersion,
		"runtime_contract_version":        existing.RuntimeContractVersion,
		"capabilities_hash":               existing.CapabilitiesHash,
		"capabilities_json":               existing.CapabilitiesJSON,
		"status":                          existing.Status,
		"last_applied_desired_revision":   hello.LastAppliedDesiredRevision,
		"last_processed_command_sequence": hello.LastProcessedCommandSequence,
		"last_event_sequence":             hello.LastEventSequence,
		"updated_at":                      existing.UpdatedAt,
	}).Error; updateErr != nil {
		return nil, updateErr
	}

	return &existing, nil
}

func (s *sessionService) DeleteTerminalBefore(cutoff time.Time) (int64, error) {
	cutoffStr := cutoff.Format("2006-01-02 15:04:05")
	result := s.db.Where(
		"status IN (?, ?) AND updated_at < ?",
		SessionStatusClosed, SessionStatusSuperseded, cutoffStr,
	).Delete(&RuntimeSession{})
	return result.RowsAffected, result.Error
}
