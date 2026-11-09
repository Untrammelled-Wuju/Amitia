package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/attention"
	"github.com/u-ai/backend/internal/notificationruntime"
	amitialog "log"
)

func startAttentionNotificationWorker(ctx context.Context, services *AppServices) {
	if services == nil || services.NotificationRuntime == nil {
		return
	}
	go func() {
		dispatch := func() {
			dispatchAttentionNotifications(ctx, services, services.LocalSpaceID, services.LocalDeviceID)
		}
		dispatch()
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				dispatch()
			}
		}
	}()
}

func dispatchAttentionNotifications(ctx context.Context, services *AppServices, spaceID, deviceID string) {
	if services == nil || services.NotificationRuntime == nil || strings.TrimSpace(spaceID) == "" {
		return
	}
	items, err := collectAttentionItems(ctx, services, spaceID, deviceID)
	if err != nil {
		amitialog.Printf("[attention-notification] collect: %v", err)
		return
	}
	for _, item := range items {
		if !item.RequiresAction || item.Kind == "source_error" {
			continue
		}
		allowed, budgetErr := attentionNotificationAllowed(ctx, services, item, time.Now().UTC())
		if budgetErr != nil {
			amitialog.Printf("[attention-notification] budget item=%s: %v", item.ID, budgetErr)
			continue
		}
		if !allowed {
			continue
		}
		body := strings.TrimSpace(item.Summary)
		if body == "" {
			body = strings.TrimSpace(item.Reason)
		}
		if body == "" {
			body = item.Title
		}
		deepLink := "amitia://attention?attentionId=" + item.ID
		if item.ConversationID != "" {
			deepLink = fmt.Sprintf("amitia://chat/%s?attentionId=%s", item.ConversationID, item.ID)
		}
		_, queueErr := services.NotificationRuntime.QueueAttention(ctx, notificationruntime.AttentionNotification{
			SpaceID:        item.SpaceID,
			AttentionID:    item.ID,
			Kind:           item.Kind,
			State:          item.Decision,
			Title:          "Amitia 需要你的处理",
			Body:           item.Title + " · " + body,
			ConversationID: item.ConversationID,
			CharacterID:    item.RoleID,
			RequestID:      item.RequestID,
			ApprovalID:     item.ApprovalID,
			DeepLink:       deepLink,
			ExpiresAt:      item.ExpiresAt,
		})
		if queueErr != nil {
			amitialog.Printf("[attention-notification] queue item=%s: %v", item.ID, queueErr)
		}
	}
}

func attentionNotificationAllowed(ctx context.Context, services *AppServices, item attention.Item, now time.Time) (bool, error) {
	if services.Attention == nil {
		return true, nil
	}
	state := strings.TrimSpace(item.Decision)
	if state == "" {
		state = "pending"
	}
	baseKey := "attention:" + item.ID + ":" + state
	seen, err := services.Attention.HasEvent(ctx, item.SpaceID, baseKey)
	if err != nil {
		return false, err
	}
	eventKind := "notification"
	idempotencyKey := baseKey
	if seen {
		eventKind = "unchanged"
		idempotencyKey = baseKey + ":unchanged:" + now.Format("2006-01-02")
	}
	payload, err := json.Marshal(map[string]any{
		"kind": item.Kind, "decision": item.Decision, "title": item.Title, "summary": item.Summary,
		"conversationId": item.ConversationID, "requestId": item.RequestID, "approvalId": item.ApprovalID,
	})
	if err != nil {
		return false, err
	}
	result, err := services.Attention.Check(ctx, attention.CheckInput{
		SpaceID: item.SpaceID, RoleID: item.RoleID, ConversationID: item.ConversationID,
		SourceKind: item.Kind, SourceID: item.ID, EventKind: eventKind,
		IdempotencyKey: idempotencyKey, PayloadJSON: string(payload),
	})
	if err != nil {
		return false, err
	}
	return result.Allowed, nil
}
