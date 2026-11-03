package agentpermission

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func policyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&SourcePolicy{}, &DecisionRecord{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestEvaluateUsesMostSpecificPolicyAndPermissionCeiling(t *testing.T) {
	db := policyTestDB(t)
	service := NewService(db)
	now := time.Now().UTC()
	policies := []SourcePolicy{
		{ID: "channel", SpaceID: "space", ScopeType: "channel", ChannelID: "qq", ExplicitMode: ModeAllow, InferredMode: ModeObserve, TriggerMode: "all", PermissionCeiling: RequestApproval, Enabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now},
		{ID: "sender", SpaceID: "space", ScopeType: "sender", ChannelID: "qq", ConversationID: "group", SenderID: "owner", ExplicitMode: ModeAllow, InferredMode: ModeAllow, TriggerMode: "all", PermissionCeiling: FullAccess, Enabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now.Add(time.Second)},
	}
	if err := db.Create(&policies).Error; err != nil {
		t.Fatal(err)
	}
	decision, err := service.Evaluate(context.Background(), AdmissionInput{SpaceID: "space", RequestID: "request", ChannelID: "qq", ConversationID: "group", SenderID: "owner", TriggerMode: "all", Explicit: false, RequestedPermission: FullAccess})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed || decision.Policy == nil || decision.Policy.ID != "sender" || decision.PermissionMode != FullAccess || decision.Strength != "inferred" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	var record DecisionRecord
	if err := db.Where("request_id = ?", "request").First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.PolicyID != "sender" || !record.Allowed || record.Observed {
		t.Fatalf("unexpected decision record: %+v", record)
	}
}

func TestEvaluateObserveModeAndPermissionDowngrade(t *testing.T) {
	db := policyTestDB(t)
	service := NewService(db)
	now := time.Now().UTC()
	if err := db.Create(&SourcePolicy{ID: "observed", SpaceID: "space", ScopeType: "channel", ChannelID: "qq", ExplicitMode: ModeAllow, InferredMode: ModeObserve, TriggerMode: "all", PermissionCeiling: RequestApproval, Enabled: true, Revision: 3, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	decision, err := service.Evaluate(context.Background(), AdmissionInput{SpaceID: "space", RequestID: "request", ChannelID: "qq", TriggerMode: "all", Explicit: false, RequestedPermission: FullAccess})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed || !decision.Observed || decision.Record.PolicyRevision != 3 {
		t.Fatalf("unexpected observe decision: %+v", decision)
	}
	if decision.PermissionMode != RequestApproval {
		t.Fatalf("permission=%q want %q", decision.PermissionMode, RequestApproval)
	}
}

func TestEnsureCompatiblePoliciesCreatesStableChannelDefaults(t *testing.T) {
	db := policyTestDB(t)
	service := NewService(db)
	if err := service.EnsureCompatiblePolicies(context.Background(), "space", []string{"qq", "telegram", "qq"}); err != nil {
		t.Fatal(err)
	}
	if err := service.EnsureCompatiblePolicies(context.Background(), "space", []string{"qq"}); err != nil {
		t.Fatal(err)
	}
	policies, err := service.List(context.Background(), "space")
	if err != nil {
		t.Fatal(err)
	}
	if len(policies) != 2 {
		t.Fatalf("policies=%+v", policies)
	}
	for _, policy := range policies {
		if policy.ScopeType != "channel" || policy.ExplicitMode != ModeAllow || policy.InferredMode != ModeObserve || policy.TriggerMode != "direct_or_mention" || policy.PermissionCeiling != RequestApproval {
			t.Fatalf("unexpected default policy: %+v", policy)
		}
	}
}
