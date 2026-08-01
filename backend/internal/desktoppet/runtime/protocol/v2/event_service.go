package v2

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	EventTypeRenderPointer   = "render.pointer"
	EventTypeRenderDrag      = "render.drag"
	EventTypeRenderState     = "render.state"
	EventTypeCommandAck      = "command.ack"
	EventTypePlaybackStarted = "playback.started"
	EventTypePlaybackEnded   = "playback.ended"
	EventTypePlaybackFailed  = "playback.failed"
	EventTypeHealth          = "render.health"
)

type EventService interface {
	Append(eventType string, payload []byte, sessionID string, seq int64, source TriggerSource, commandID *string) (*EventRecord, error)
	GetByID(eventID string) (*EventRecord, error)
	GetSession(sessionID string, fromSeq int64, limit int) ([]*EventRecord, error)
	GetPendingDeliver(sessionID string, fromSeq int64, limit int) ([]*EventRecord, error)
	GetLatestEventSeq(sessionID string) (int64, error)
	MarkDelivered(eventIDs []string, t time.Time) error
	DB() *gorm.DB
}

type eventService struct {
	db *gorm.DB
}

func NewEventService(db *gorm.DB) EventService {
	return &eventService{db: db}
}

func (s *eventService) DB() *gorm.DB { return s.db }

func (s *eventService) Append(eventType string, payload []byte, sessionID string, seq int64, source TriggerSource, commandID *string) (*EventRecord, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	payloadHash := ComputePayloadHash(payload)

	event := &EventRecord{
		ID:               "rtevtv2_" + uuid.NewString(),
		EventType:        eventType,
		Payload:          payload,
		PayloadHash:      payloadHash,
		Source:           string(source),
		RuntimeSessionID: sessionID,
		Sequence:         seq,
		OccurredAt:       now,
		Delivered:        0,
		InsertedAt:       now,
	}

	if commandID != nil && *commandID != "" {
		event.CommandID = *commandID
	}

	if err := s.db.Create(event).Error; err != nil {
		return nil, err
	}
	return event, nil
}

func (s *eventService) GetByID(eventID string) (*EventRecord, error) {
	var event EventRecord
	err := s.db.Where("id = ?", eventID).First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *eventService) GetSession(sessionID string, fromSeq int64, limit int) ([]*EventRecord, error) {
	var events []*EventRecord
	err := s.db.Where(
		"runtime_session_id = ? AND sequence >= ?",
		sessionID, fromSeq,
	).Order("sequence ASC").Limit(limit).Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (s *eventService) GetPendingDeliver(sessionID string, fromSeq int64, limit int) ([]*EventRecord, error) {
	var events []*EventRecord
	err := s.db.Where(
		"runtime_session_id = ? AND sequence >= ? AND delivered = 0",
		sessionID, fromSeq,
	).Order("sequence ASC").Limit(limit).Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (s *eventService) GetLatestEventSeq(sessionID string) (int64, error) {
	var event EventRecord
	err := s.db.Where("runtime_session_id = ?", sessionID).Order("sequence DESC").First(&event).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return event.Sequence, nil
}

func (s *eventService) MarkDelivered(eventIDs []string, t time.Time) error {
	if len(eventIDs) == 0 {
		return nil
	}
	deliveredAt := t.Format("2006-01-02 15:04:05")
	return s.db.Model(&EventRecord{}).Where("id IN ?", eventIDs).Updates(map[string]interface{}{
		"delivered":     1,
		"delivered_at":  deliveredAt,
	}).Error
}
