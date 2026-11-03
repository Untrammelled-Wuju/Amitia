package notificationruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (r *Runtime) QueueAttention(ctx context.Context, notification AttentionNotification) ([]ScheduledDelivery, error) {
	if r == nil || r.repo == nil {
		return nil, errors.New("notification runtime unavailable")
	}
	notification.SpaceID = strings.TrimSpace(notification.SpaceID)
	notification.AttentionID = strings.TrimSpace(notification.AttentionID)
	notification.State = strings.TrimSpace(notification.State)
	if notification.SpaceID == "" || notification.AttentionID == "" {
		return nil, errors.New("attention notification requires space and item identity")
	}
	if notification.Title == "" {
		notification.Title = "Amitia 需要你的处理"
	}
	devices, err := r.repo.ListPushDevices(ctx, notification.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("load attention notification devices: %w", err)
	}
	results := make([]ScheduledDelivery, 0, len(devices))
	for _, endpoint := range devices {
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("attention-required:"+notification.SpaceID+":"+endpoint.DeviceID+":"+notification.AttentionID+":"+notification.State)).String()
		if !endpoint.SystemNotificationsEnabled {
			results = append(results, ScheduledDelivery{ID: id, State: "blocked", Error: "系统通知权限已关闭"})
			continue
		}
		if attentionSuppressedForPresence(endpoint) {
			results = append(results, ScheduledDelivery{ID: id, State: "suppressed"})
			continue
		}
		title, body := PreviewForMode(endpoint.PreviewMode, notification.Title, notification.Body)
		deepLink := notification.DeepLink
		if deepLink == "" && notification.ConversationID != "" {
			deepLink = fmt.Sprintf("amitia://chat/%s?attentionId=%s", notification.ConversationID, notification.AttentionID)
		}
		expiresAt := time.Now().UTC().Add(24 * time.Hour)
		if notification.ExpiresAt != nil && notification.ExpiresAt.After(time.Now().UTC()) {
			expiresAt = notification.ExpiresAt.UTC()
		}
		envelope := PushEnvelope{
			NotificationID: id,
			Type:           "attention.required",
			SpaceID:        notification.SpaceID,
			ConversationID: notification.ConversationID,
			CharacterID:    notification.CharacterID,
			RunID:          notification.AttentionID,
			Title:          title,
			Body:           body,
			DeepLink:       deepLink,
			Priority:       "high",
			TTL:            time.Until(expiresAt),
			Sound:          endpoint.SoundEnabled,
			Data: map[string]string{
				"attentionId": notification.AttentionID,
				"kind":        notification.Kind,
				"state":       notification.State,
				"requestId":   notification.RequestID,
				"approvalId":  notification.ApprovalID,
			},
		}
		encoded, err := json.Marshal(envelope)
		if err != nil {
			return nil, fmt.Errorf("encode attention notification: %w", err)
		}
		now := time.Now().UTC()
		row := &NotificationOutbox{
			ID:             id,
			NotificationID: id,
			SpaceID:        notification.SpaceID,
			DeviceID:       endpoint.DeviceID,
			Type:           envelope.Type,
			RunID:          notification.AttentionID,
			EnvelopeJSON:   string(encoded),
			Status:         "queued",
			NextAttemptAt:  now,
			ExpiresAt:      expiresAt,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := r.repo.CreateOutbox(ctx, row); err != nil {
			return nil, fmt.Errorf("persist attention notification: %w", err)
		}
		if row.Status == "queued" || row.Status == "retry" {
			r.enqueueDelivery(deliveryJob{outboxID: row.ID})
		}
		results = append(results, ScheduledDelivery{ID: row.ID, State: row.Status})
	}
	if len(results) == 0 {
		results = append(results, ScheduledDelivery{ID: notification.AttentionID, State: "blocked", Error: "no registered notification device"})
	}
	return results, nil
}

func attentionSuppressedForPresence(endpoint DeviceEndpoint) bool {
	if !endpoint.Foreground {
		return false
	}
	if endpoint.Platform == "desktop" || endpoint.Platform == "web" {
		if endpoint.PresenceUpdatedAt.IsZero() {
			return false
		}
	}
	return time.Since(endpoint.PresenceUpdatedAt) < 90*time.Second
}
