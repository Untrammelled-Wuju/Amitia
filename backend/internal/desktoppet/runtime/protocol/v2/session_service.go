// Package v2 provides the DesktopPet runtime protocol v2 implementation.
//
// IMPORTANT: This package contains the DesktopPet domain session projection.
// The authoritative Runtime Session is owned by deviceruntime.Service (G12).
//
// The desktop_pet_runtime_sessions table is retained as a COMPATIBILITY PROJECTION
// for DesktopPet v2 runtime protocol. Production code MUST:
//   - Use SyncFromDeviceRuntimeSession as the primary session creation path
//   - NOT call CreateSession/AcquireSession for new production flows
//   - Treat RuntimeSession rows as a read-only view of G12 session state
//
// Session IDs in this projection MUST match deviceruntime.RuntimeSession.ID exactly.
// Do NOT generate a separate DesktopPet session ID namespace.
package v2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"gorm.io/gorm"
)

// SessionService is the DesktopPet domain session projection store interface.
//
// Production primary path: SyncFromDeviceRuntimeSession (sync from G12 authoritative session).
// Deprecated compatibility paths: CreateSession, AcquireSession (retained for legacy callers).
type SessionService interface {
	CreateSession(userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID, prevGen int64) (*RuntimeSession, error)
	GetSession(id string) (*RuntimeSession, error)
	GetActiveSession(userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (*RuntimeSession, error)
	AcquireSession(ctx *gorm.DB, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID, caps []string, capsHash string, lastAppliedRev, lastCmdSeq, lastEvtSeq int64, contractVersion string) (*RuntimeSession, *RuntimeSession, error)
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

// NewSessionService creates a new DesktopPet domain session projection store.
// Production code should prefer syncing sessions from G12 deviceruntime.Service
// via SyncFromDeviceRuntimeSession, NOT via direct CreateSession/AcquireSession.
func NewSessionService(db *gorm.DB) SessionService {
	return &sessionService{db: db}
}

func (s *sessionService) DB() *gorm.DB { return s.db }

// CreateSession is a DEPRECATED compatibility path for DesktopPet v2 session creation.
// Production code must use deviceruntime.Service session creation as the authority
// and then sync the projection row via SyncFromDeviceRuntimeSession.
//
// Deprecated: Use SyncFromDeviceRuntimeSession with deviceruntime.Service instead.
func (s *sessionService) CreateSession(userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID, prevGen int64) (*RuntimeSession, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	session := &RuntimeSession{
		ID:                           string(runtimeidentity.RuntimeSessionID("rtsessv2_" + uuid.NewString())),
		UserID:                       userID,
		DeviceID:                     deviceID,
		RuntimeID:                    runtimeID,
		ConnectionGeneration:         prevGen + 1,
		Status:                       string(SessionStatusSyncing),
		LastAppliedDesiredRevision:   0,
		LastProcessedCommandSequence: 0,
		LastEventSequence:            0,
		ConnectedAt:                  now,
		LastHeartbeatAt:              now,
		CreatedAt:                    now,
		UpdatedAt:                    now,
	}
	if err := s.db.Create(session).Error; err != nil {
		return nil, err
	}
	return session, nil
}

func (s *sessionService) GetSession(id string) (*RuntimeSession, error) {
	var session RuntimeSession
	err := s.db.Where("id = ?", id).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *sessionService) GetActiveSession(userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (*RuntimeSession, error) {
	var session RuntimeSession
	err := s.db.Where(
		"user_id = ? AND device_id = ? AND runtime_id = ? AND status IN (?, ?, ?, ?)",
		userID.String(), deviceID.String(), runtimeID.String(),
		SessionStatusRegistering, SessionStatusSyncing, SessionStatusReady, SessionStatusDegraded,
	).Order("connection_generation DESC").First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// AcquireSession is a DEPRECATED compatibility path for DesktopPet v2 session re-acquisition.
// Production code must use deviceruntime.Service session management as the authority
// and then sync the projection row via SyncFromDeviceRuntimeSession.
//
// Deprecated: Use SyncFromDeviceRuntimeSession with deviceruntime.Service instead.
func (s *sessionService) AcquireSession(ctx *gorm.DB, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID, caps []string, capsHash string, lastAppliedRev, lastCmdSeq, lastEvtSeq int64, contractVersion string) (*RuntimeSession, *RuntimeSession, error) {
	base := s.db
	if ctx != nil {
		base = ctx
	}

	var newSession *RuntimeSession
	var oldSession *RuntimeSession
	err := base.Transaction(func(tx *gorm.DB) error {
		var existing RuntimeSession
		err := tx.Where(
			"user_id = ? AND device_id = ? AND runtime_id = ? AND status IN (?, ?, ?, ?)",
			userID.String(), deviceID.String(), runtimeID.String(),
			SessionStatusRegistering, SessionStatusSyncing, SessionStatusReady, SessionStatusDegraded,
		).Order("connection_generation DESC").First(&existing).Error

		var prevGen int64
		if err == nil {
			prevGen = existing.ConnectionGeneration
			copy := existing
			oldSession = &copy
			now := time.Now().UTC().Format("2006-01-02 15:04:05")
			result := tx.Model(&RuntimeSession{}).Where("id = ?", existing.ID).Updates(map[string]interface{}{
				"status":        SessionStatusSuperseded,
				"superseded_at": now,
				"updated_at":    now,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("supersede previous runtime session: expected 1 row, got %d", result.RowsAffected)
			}
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		now := time.Now().UTC().Format("2006-01-02 15:04:05")
		capsJSON, err := json.Marshal(caps)
		if err != nil {
			return fmt.Errorf("marshal runtime capabilities: %w", err)
		}
		created := &RuntimeSession{
			ID:                           string(runtimeidentity.RuntimeSessionID("rtsessv2_" + uuid.NewString())),
			UserID:                       userID,
			DeviceID:                     deviceID,
			RuntimeID:                    runtimeID,
			ConnectionGeneration:         prevGen + 1,
			RuntimeContractVersion:       contractVersion,
			CapabilitiesHash:             capsHash,
			CapabilitiesJSON:             string(capsJSON),
			LastAppliedDesiredRevision:   maxInt64(0, lastAppliedRev),
			LastProcessedCommandSequence: maxInt64(0, lastCmdSeq),
			LastEventSequence:            maxInt64(0, lastEvtSeq),
			Status:                       string(SessionStatusSyncing),
			ConnectedAt:                  now,
			LastHeartbeatAt:              now,
			CreatedAt:                    now,
			UpdatedAt:                    now,
		}
		if err := tx.Create(created).Error; err != nil {
			return err
		}
		if oldSession != nil {
			result := tx.Model(&RuntimeSession{}).Where("id = ?", oldSession.ID).Updates(map[string]interface{}{
				"superseded_by": created.ID,
				"updated_at":    now,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("link superseded runtime session: expected 1 row, got %d", result.RowsAffected)
			}
		}
		newSession = created
		return nil
	})
	if err != nil {
		return nil, oldSession, err
	}
	return newSession, oldSession, nil
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
			UserID:                       runtimeSession.UserID,
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

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
