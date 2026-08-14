package v2

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"gorm.io/gorm"
)

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

func NewSessionService(db *gorm.DB) SessionService {
	return &sessionService{db: db}
}

func (s *sessionService) DB() *gorm.DB { return s.db }

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

func (s *sessionService) AcquireSession(ctx *gorm.DB, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID, caps []string, capsHash string, lastAppliedRev, lastCmdSeq, lastEvtSeq int64, contractVersion string) (*RuntimeSession, *RuntimeSession, error) {
	db := s.db
	if ctx != nil {
		db = ctx
	}

	var existing RuntimeSession
	err := db.Where(
		"user_id = ? AND device_id = ? AND runtime_id = ? AND status IN (?, ?, ?, ?)",
		userID.String(), deviceID.String(), runtimeID.String(),
		SessionStatusRegistering, SessionStatusSyncing, SessionStatusReady, SessionStatusDegraded,
	).Order("connection_generation DESC").First(&existing).Error

	var prevGen int64
	var oldSession *RuntimeSession
	if err == nil {
		prevGen = existing.ConnectionGeneration
		oldSession = &existing

		now := time.Now().Format("2006-01-02 15:04:05")
		db.Model(&RuntimeSession{}).Where("id = ?", existing.ID).Updates(map[string]interface{}{
			"status":        SessionStatusSuperseded,
			"superseded_at": now,
			"updated_at":    now,
		})
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	newSession := &RuntimeSession{
		ID:                           string(runtimeidentity.RuntimeSessionID("rtsessv2_" + uuid.NewString())),
		UserID:                       userID,
		DeviceID:                     deviceID,
		RuntimeID:                    runtimeID,
		ConnectionGeneration:         prevGen + 1,
		RuntimeContractVersion:       contractVersion,
		CapabilitiesHash:             capsHash,
		LastAppliedDesiredRevision:   lastAppliedRev,
		LastProcessedCommandSequence: lastCmdSeq,
		LastEventSequence:            lastEvtSeq,
		Status:                       string(SessionStatusSyncing),
		ConnectedAt:                  now,
		LastHeartbeatAt:              now,
		CreatedAt:                    now,
		UpdatedAt:                    now,
	}

	if len(caps) > 0 {
		capsJSON := []byte{}
		for i, c := range caps {
			if i > 0 {
				capsJSON = append(capsJSON, ',')
			}
			capsJSON = append(capsJSON, '"')
			capsJSON = append(capsJSON, c...)
			capsJSON = append(capsJSON, '"')
		}
		newSession.CapabilitiesJSON = "[" + string(capsJSON) + "]"
	}

	if err := db.Create(newSession).Error; err != nil {
		return nil, oldSession, err
	}

	if oldSession != nil {
		db.Model(&RuntimeSession{}).Where("id = ?", oldSession.ID).Update("superseded_by", newSession.ID)
	}

	return newSession, oldSession, nil
}

func (s *sessionService) UpdateLastAppliedRevision(id string, revision int64) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	return s.db.Model(&RuntimeSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_applied_desired_revision": revision,
		"updated_at":                    now,
	}).Error
}

func (s *sessionService) UpdateLastProcessedCommandSequence(id string, seq int64) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	return s.db.Model(&RuntimeSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_processed_command_sequence": seq,
		"updated_at":                      now,
	}).Error
}

func (s *sessionService) UpdateLastEventSequence(id string, seq int64) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	return s.db.Model(&RuntimeSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_event_sequence": seq,
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
	now := time.Now().Format("2006-01-02 15:04:05")
	return s.db.Model(&RuntimeSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        SessionStatusSuperseded,
		"superseded_by": supersededBy,
		"superseded_at": now,
		"updated_at":    now,
	}).Error
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

		capsJSON, _ := json.Marshal(hello.Capabilities)
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

	capsJSON, _ := json.Marshal(hello.Capabilities)
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
