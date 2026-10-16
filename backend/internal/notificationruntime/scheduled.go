package notificationruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ScheduledDelivery struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

func (r *Runtime) QueueScheduled(ctx context.Context, spaceID, deviceID, triggerID, conversationID, characterID, messageID, title, content, kind string) ([]ScheduledDelivery, error) {
	devices, err := r.repo.ListPushDevices(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	results := []ScheduledDelivery{}
	for _, endpoint := range devices {
		if endpoint.DeviceID != deviceID {
			continue
		}
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(triggerID+"/notification/"+endpoint.DeviceID+"/"+endpoint.Platform)).String()
		if !endpoint.SystemNotificationsEnabled || !endpoint.ReminderPushEnabled {
			results = append(results, ScheduledDelivery{ID: id, State: "blocked", Error: "系统通知权限或提醒通知已关闭"})
			continue
		}
		if suppressMessageForPresence(endpoint, conversationID) {
			results = append(results, ScheduledDelivery{ID: id, State: "suppressed"})
			continue
		}
		previewTitle, previewBody := PreviewForMode(endpoint.PreviewMode, title, content)
		envelope := PushEnvelope{NotificationID: id, Type: kind, SpaceID: spaceID, ConversationID: conversationID, CharacterID: characterID, MessageID: messageID, RunID: triggerID, Revision: 1, Title: previewTitle, Body: previewBody, DeepLink: fmt.Sprintf("amitia://chat/%s?message=%s", conversationID, messageID), Priority: "high", TTL: 24 * time.Hour, Sound: endpoint.SoundEnabled, Data: map[string]string{"source": "proactive", "reminderId": triggerID}}
		encoded, err := json.Marshal(envelope)
		if err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		row := &NotificationOutbox{ID: id, NotificationID: id, SpaceID: spaceID, DeviceID: endpoint.DeviceID, Type: kind, RunID: triggerID, EnvelopeJSON: string(encoded), Status: "queued", NextAttemptAt: now, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now, UpdatedAt: now}
		if err := r.repo.CreateOutbox(ctx, row); err != nil {
			return nil, err
		}
		if row.Status == "queued" || row.Status == "retry" {
			r.enqueueDelivery(deliveryJob{outboxID: row.ID})
		}
		results = append(results, ScheduledDelivery{ID: row.ID, State: row.Status})
	}
	if len(results) == 0 {
		results = append(results, ScheduledDelivery{ID: deviceID, State: "blocked", Error: "设备未注册通知通道"})
	}
	return results, nil
}

func (r *Runtime) ScheduledState(ctx context.Context, id string) (ScheduledDelivery, error) {
	var row NotificationOutbox
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return ScheduledDelivery{}, err
	}
	return ScheduledDelivery{ID: row.ID, State: row.Status, Error: row.LastError}, nil
}

func (r *Runtime) AcknowledgeScheduled(ctx context.Context, spaceID, deviceID, id, state string) error {
	if state != "displayed" && state != "clicked" && state != "display_failed" {
		return errors.New("通知回执状态无效")
	}
	var row NotificationOutbox
	if err := r.db.WithContext(ctx).Where("notification_id = ? AND space_id = ? AND device_id = ?", id, spaceID, deviceID).First(&row).Error; err != nil {
		return err
	}
	var envelope PushEnvelope
	if err := json.Unmarshal([]byte(row.EnvelopeJSON), &envelope); err != nil {
		return err
	}
	if envelope.Type != "reminder.triggered" && envelope.Type != "proactive.message" {
		return errors.New("通知不属于日程任务")
	}
	query := r.db.WithContext(ctx).Model(&NotificationOutbox{}).Where("id = ?", row.ID)
	if state != "clicked" {
		query = query.Where("status != ?", "clicked")
	}
	if state == "display_failed" {
		query = query.Where("status != ?", "displayed")
	}
	return query.Updates(map[string]any{"status": state, "updated_at": time.Now().UTC()}).Error
}
