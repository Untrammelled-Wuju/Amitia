package delivery

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type orderedPlanAdapter struct {
	delivered []string
}

func (a *orderedPlanAdapter) Name() string { return "qq" }

func (a *orderedPlanAdapter) Deliver(intent DeliveryIntent) error {
	a.delivered = append(a.delivered, intent.ContentType)
	if intent.ContentType == "emote" {
		return errors.New("emote failed")
	}
	return nil
}

func TestMessagePlanFailureDoesNotStopLaterText(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:message-plan-failure?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	store := NewSQLiteDeliveryStore(db)
	if err = store.InitSchema(); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Now().UTC()
	for index, contentType := range []string{"text", "emote", "text"} {
		intent := NewDeliveryIntent("interaction", "qq", "peer", contentType, []byte(`{}`))
		intent.ID = contentType + string(rune('1'+index))
		intent.ResponseGroupID = "response"
		intent.DeliverySequence = index + 1
		intent.CreatedAt = createdAt
		intent.MaxRetries = 1
		if contentType == "emote" {
			intent.MaxRetries = 2
		}
		if err = store.CreateIntent(intent); err != nil {
			t.Fatal(err)
		}
	}
	adapter := &orderedPlanAdapter{}
	worker := NewWorker(store, []ChannelAdapter{adapter}, WorkerConfig{BatchSize: 10, Interval: time.Second})
	worker.processBatch(context.Background())
	if !reflect.DeepEqual(adapter.delivered, []string{"text", "emote", "text"}) {
		t.Fatalf("投递未严格按计划继续执行: %#v", adapter.delivered)
	}
	var statuses []string
	if err = db.Model(&DeliveryIntentModel{}).Order("delivery_sequence ASC").Pluck("status", &statuses).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(statuses, []string{"sent", "retry", "sent"}) {
		t.Fatalf("中间表情失败不应影响后续文字: %#v", statuses)
	}
	if err = db.Model(&DeliveryIntentModel{}).Where("content_type = ?", "emote").Update("next_retry", time.Now().Add(-time.Second).UTC().Format("2006-01-02 15:04:05")).Error; err != nil {
		t.Fatal(err)
	}
	worker.processBatch(context.Background())
	statuses = nil
	if err = db.Model(&DeliveryIntentModel{}).Order("delivery_sequence ASC").Pluck("status", &statuses).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(statuses, []string{"sent", "failed", "sent"}) {
		t.Fatalf("表情达到有限重试上限后应单独失败: %#v", statuses)
	}
	if !reflect.DeepEqual(adapter.delivered, []string{"text", "emote", "text", "emote"}) {
		t.Fatalf("重试不应重复投递已成功文字: %#v", adapter.delivered)
	}
}
