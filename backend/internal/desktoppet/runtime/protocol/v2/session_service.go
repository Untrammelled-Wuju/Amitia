package v2

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SessionService interface {
	CreateSession(userID, deviceID, runtimeID string, prevGen int64) (*RuntimeSession, error)
	GetSession(id string) (*RuntimeSession, error)
	GetActiveSession(userID, deviceID, runtimeID string) (*RuntimeSession, error)
	AcquireSession(ctx *gorm.DB, userID, deviceID, runtimeID string, caps []string, capsHash string, lastAppliedRev, lastCmdSeq, lastEvtSeq int64, contractVersion string) (*RuntimeSession, *RuntimeSession, error)
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

func (s *sessionService) CreateSession(userID, deviceID, runtimeID string, prevGen int64) (*RuntimeSession, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	session := &RuntimeSession{
		ID:                           "rtsessv2_" + uuid.NewString(),
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

func (s *sessionService) GetActiveSession(userID, deviceID, runtimeID string) (*RuntimeSession, error) {
	var session RuntimeSession
	err := s.db.Where(
		"user_id = ? AND device_id = ? AND runtime_id = ? AND status IN (?, ?, ?, ?)",
		userID, deviceID, runtimeID,
		SessionStatusRegistering, SessionStatusSyncing, SessionStatusReady, SessionStatusDegraded,
	).Order("connection_generation DESC").First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *sessionService) AcquireSession(ctx *gorm.DB, userID, deviceID, runtimeID string, caps []string, capsHash string, lastAppliedRev, lastCmdSeq, lastEvtSeq int64, contractVersion string) (*RuntimeSession, *RuntimeSession, error) {
	db := s.db
	if ctx != nil {
		db = ctx
	}

	var existing RuntimeSession
	err := db.Where(
		"user_id = ? AND device_id = ? AND runtime_id = ? AND status IN (?, ?, ?, ?)",
		userID, deviceID, runtimeID,
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
		ID:                           "rtsessv2_" + uuid.NewString(),
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

func (s *sessionService) DeleteTerminalBefore(cutoff time.Time) (int64, error) {
	cutoffStr := cutoff.Format("2006-01-02 15:04:05")
	result := s.db.Where(
		"status IN (?, ?) AND updated_at < ?",
		SessionStatusClosed, SessionStatusSuperseded, cutoffStr,
	).Delete(&RuntimeSession{})
	return result.RowsAffected, result.Error
}
