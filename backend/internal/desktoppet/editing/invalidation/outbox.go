package invalidation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing/baseline"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type ActionRevisionEventOutbox interface {
	PublishCreated(ctx context.Context, event baseline.ActionRevisionEvent) error
	PublishActivated(ctx context.Context, event baseline.ActionRevisionEvent) error
	PublishDeactivated(ctx context.Context, event baseline.ActionRevisionEvent) error
	PublishSuperseded(ctx context.Context, event baseline.ActionRevisionEvent) error
}

type ActionRevisionEventRecord struct {
	ID          string `gorm:"column:id;primaryKey" json:"id"`
	EventType   string `gorm:"column:event_type" json:"eventType"`
	AggregateID string `gorm:"column:aggregate_id" json:"aggregateId"`
	Sequence    int64  `gorm:"column:sequence" json:"sequence"`
	PayloadJSON string `gorm:"column:payload_json" json:"payloadJson"`
	CreatedAt   string `gorm:"column:created_at" json:"createdAt"`
}

func (ActionRevisionEventRecord) TableName() string {
	return "desktop_pet_action_revision_events"
}

type LogEventOutbox struct {
	db *gorm.DB
}

func NewLogEventOutbox(db *gorm.DB) ActionRevisionEventOutbox {
	return &LogEventOutbox{db: db}
}

func (o *LogEventOutbox) PublishCreated(ctx context.Context, event baseline.ActionRevisionEvent) error {
	return o.publish(baseline.EventActionRevisionCreated, event)
}

func (o *LogEventOutbox) PublishActivated(ctx context.Context, event baseline.ActionRevisionEvent) error {
	return o.publish(baseline.EventActionRevisionActivated, event)
}

func (o *LogEventOutbox) PublishDeactivated(ctx context.Context, event baseline.ActionRevisionEvent) error {
	return o.publish(baseline.EventActionRevisionDeactivated, event)
}

func (o *LogEventOutbox) PublishSuperseded(ctx context.Context, event baseline.ActionRevisionEvent) error {
	return o.publish(baseline.EventActionRevisionSuperseded, event)
}

func (o *LogEventOutbox) publish(eventType string, event baseline.ActionRevisionEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		log.Logger.Errorf("序列化事件失败: eventType=%s err=%v", eventType, err)
		return err
	}
	aggregateID := fmt.Sprintf("%s:%s", event.CharacterID, event.ActionKey)
	log.Logger.Infof("action_revision_event: type=%s aggregateId=%s revisionId=%s bindingRevision=%d payload=%s",
		eventType, aggregateID, event.ActionRevisionID, event.BindingRevision, string(payload))

	if o.db != nil {
		record := ActionRevisionEventRecord{
			ID:          fmt.Sprintf("evtrec_%d", time.Now().UnixNano()),
			EventType:   eventType,
			AggregateID: aggregateID,
			Sequence:    event.BindingRevision,
			PayloadJSON: string(payload),
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		if createErr := o.db.Create(&record).Error; createErr != nil {
			log.Logger.Errorf("写入事件outbox表失败: eventType=%s err=%v", eventType, createErr)
		}
	}
	return nil
}
