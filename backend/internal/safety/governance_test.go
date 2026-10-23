package safety

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

type safetyBackupMemory map[string][]byte

func (m safetyBackupMemory) CreateComponent(string, string, dataportability.ComponentKind) (io.WriteCloser, error) {
	return nil, errors.New("component streaming is not used")
}

func (m safetyBackupMemory) WriteJSON(id string, value interface{}) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m[id] = raw
	return nil
}

func (m safetyBackupMemory) ReadComponent(id string) (io.ReadCloser, error) {
	raw, ok := m[id]
	if !ok {
		return nil, errors.New("component not found")
	}
	return io.NopCloser(strings.NewReader(string(raw))), nil
}

func (m safetyBackupMemory) ReadJSON(id string, value interface{}) error {
	raw, ok := m[id]
	if !ok {
		return errors.New("component not found")
	}
	return json.Unmarshal(raw, value)
}

func (m safetyBackupMemory) ListComponents() []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return ids
}

func governanceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "safety.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE app_settings (key TEXT PRIMARY KEY,value TEXT NOT NULL DEFAULT '',revision INTEGER NOT NULL DEFAULT 1,updated_at TEXT NOT NULL DEFAULT '',deleted_at DATETIME)`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY,conversation_id TEXT NOT NULL,character_id TEXT NOT NULL DEFAULT '',role TEXT NOT NULL,content TEXT NOT NULL,tokens INTEGER DEFAULT 0)`,
		`CREATE TABLE safety_events (id TEXT PRIMARY KEY,conversation_id TEXT DEFAULT '',event_type TEXT DEFAULT '',description TEXT DEFAULT '',direction TEXT DEFAULT '',handled INTEGER DEFAULT 0,created_at TEXT DEFAULT '')`,
		`CREATE TABLE safety_policies (space_id TEXT NOT NULL,owner_id TEXT NOT NULL,core_id TEXT NOT NULL,revision INTEGER NOT NULL DEFAULT 1,config_json TEXT NOT NULL DEFAULT '{}',bdi_json TEXT NOT NULL DEFAULT '{}',created_at TEXT NOT NULL DEFAULT '',updated_at TEXT NOT NULL DEFAULT '',PRIMARY KEY(space_id,owner_id))`,
		`CREATE TABLE safety_incidents (id TEXT PRIMARY KEY,space_id TEXT NOT NULL,owner_id TEXT NOT NULL,core_id TEXT NOT NULL,role_id TEXT NOT NULL DEFAULT '',conversation_id TEXT NOT NULL DEFAULT '',message_id TEXT NOT NULL DEFAULT '',feedback_id INTEGER,request_id TEXT NOT NULL DEFAULT '',event_type TEXT NOT NULL,severity TEXT NOT NULL DEFAULT 'medium',description TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT 'open',policy_revision INTEGER NOT NULL DEFAULT 0,revision INTEGER NOT NULL DEFAULT 1,details_json TEXT NOT NULL DEFAULT '{}',legacy_event_id TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL DEFAULT '',updated_at TEXT NOT NULL DEFAULT '',resolved_at TEXT NOT NULL DEFAULT '',resolved_by TEXT NOT NULL DEFAULT '')`,
		`CREATE UNIQUE INDEX uq_safety_incidents_feedback ON safety_incidents(feedback_id) WHERE feedback_id IS NOT NULL`,
		`CREATE UNIQUE INDEX uq_safety_incidents_legacy ON safety_incidents(legacy_event_id) WHERE legacy_event_id <> ''`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func governanceActorContext(spaceID, deviceID string) context.Context {
	return auth.WithActor(context.Background(), &auth.ActorContext{
		PrincipalType: auth.PrincipalTrustedDevice,
		SpaceID:       runtimeidentity.SpaceID(spaceID),
		DeviceID:      runtimeidentity.DeviceID(deviceID),
	})
}

func TestSafetyPolicyAndLegacyIncidentMigration(t *testing.T) {
	db := governanceTestDB(t)
	if err := db.Exec(`INSERT INTO app_settings(key,value) VALUES('safety_config',?),('safety_bdi_config',?)`, `{"preventEmotionalBlackmail":false,"violationAction":"audit_only","auditLogRetentionDays":90}`, `{"copingStrategy":{"selected":"reframe","alternatives":[],"selectionReason":"test"}}`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO safety_events(id,conversation_id,event_type,description,handled,created_at) VALUES('legacy-event','chat','user_reported_unsafe','legacy report',0,'2026-10-23T00:00:00Z')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO messages(id,conversation_id,character_id,role,content) VALUES('message','chat','role','assistant','content')`).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	if err := repository.ImportLegacy(t.Context(), []PolicyScope{{SpaceID: "space", OwnerID: "device", CoreID: "space"}, {SpaceID: "space", OwnerID: "space", CoreID: "space"}}); err != nil {
		t.Fatal(err)
	}
	if err := repository.MigrateLegacyEvents(t.Context(), PolicyScope{SpaceID: "space", OwnerID: "device", CoreID: "space"}); err != nil {
		t.Fatal(err)
	}
	policy, err := repository.RuntimePolicy(t.Context(), "space", "device")
	if err != nil {
		t.Fatal(err)
	}
	if policy.Revision != 1 || policy.Config.PreventEmotionalBlackmail || policy.Config.ViolationAction != "audit_only" || policy.Config.AuditLogRetentionDays != 90 || policy.BDI.CopingStrategy == nil || policy.BDI.CopingStrategy.Selected != "reframe" {
		t.Fatalf("legacy policy was not migrated: %+v", policy)
	}
	var legacySettings, incidents int64
	if err := db.Table("app_settings").Where("key IN ?", []string{"safety_config", "safety_bdi_config"}).Count(&legacySettings).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("safety_incidents").Count(&incidents).Error; err != nil {
		t.Fatal(err)
	}
	var migrated Incident
	if err := db.Table("safety_incidents").Where("legacy_event_id='legacy-event'").Take(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasTable("safety_events") {
		t.Fatal("legacy safety events table was not removed")
	}
	if legacySettings != 0 || incidents != 1 || migrated.OwnerID != "device" || migrated.RoleID != "role" || migrated.ConversationID != "chat" || migrated.Status != "open" {
		t.Fatalf("legacy migration result settings=%d incidents=%d row=%+v", legacySettings, incidents, migrated)
	}
}

func TestSafetyIncidentMigrationWithoutLegacyTableIsIdempotent(t *testing.T) {
	db := governanceTestDB(t)
	if err := db.Migrator().DropTable("safety_events"); err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	scope := PolicyScope{SpaceID: "space", OwnerID: "device", CoreID: "space"}
	if err := repository.MigrateLegacyEvents(t.Context(), scope); err != nil {
		t.Fatal(err)
	}
	if err := repository.MigrateLegacyEvents(t.Context(), scope); err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasTable("safety_events") {
		t.Fatal("legacy safety events table was recreated")
	}
}

func TestSafetyPolicyScopeResolvesAcrossRuntimeModes(t *testing.T) {
	ctx := governanceActorContext("space", "phone")
	tests := []struct {
		name       string
		context    context.Context
		wantOwner  string
		wantCoreID string
	}{
		{name: "local or uncoordinated device agent", context: ctx, wantOwner: "phone", wantCoreID: "space"},
		{name: "cloud core", context: coordination.WithScope(ctx, coordination.ExecutionScope{SpaceID: "space", CoreID: "space", ResourceOwnerID: "space"}), wantOwner: "space", wantCoreID: "space"},
		{name: "coordinated device agent", context: coordination.WithScope(ctx, coordination.ExecutionScope{SpaceID: "space", CoreID: "space", ResourceOwnerID: "phone"}), wantOwner: "phone", wantCoreID: "space"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scope, err := ScopeFromContextWithCoordinator(tc.context, nil)
			if err != nil {
				t.Fatal(err)
			}
			if scope.SpaceID != "space" || scope.OwnerID != tc.wantOwner || scope.CoreID != tc.wantCoreID {
				t.Fatalf("scope mismatch: %+v", scope)
			}
		})
	}
}

func TestSafetyGovernanceBackupRoundTripAndCollision(t *testing.T) {
	db := governanceTestDB(t)
	ctx := governanceActorContext("space", "device")
	scope := PolicyScope{SpaceID: "space", OwnerID: "device", CoreID: "space"}
	repository := NewRepository(db)
	config := DefaultPolicy().Config
	config.PreventEmotionalBlackmail = false
	config.ViolationAction = "audit_only"
	config.ExpectedRevision = 0
	if _, err := repository.SaveConfig(ctx, scope, config); err != nil {
		t.Fatal(err)
	}
	if err := repository.RecordIncident(ctx, Incident{ID: "roundtrip", SpaceID: "space", OwnerID: "device", CoreID: "space", RoleID: "role", ConversationID: "conversation", RequestID: "request", EventType: "policy.violation", Severity: "high", Description: "roundtrip", Status: "open", Revision: 1, DetailsJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	contributor := NewBackupContributor(db)
	backup := safetyBackupMemory{}
	if err := contributor.Export(ctx, dataportability.BackupRequest{Scope: dataportability.ScopeSettings}, backup); err != nil {
		t.Fatal(err)
	}
	archive, err := readSafetyArchive(backup)
	if err != nil {
		t.Fatal(err)
	}
	if archive.Policy.Config.ViolationAction != "audit_only" || len(archive.Incidents) != 1 {
		t.Fatalf("export mismatch: %+v", archive)
	}
	config.PreventEmotionalBlackmail = true
	config.ViolationAction = "block"
	config.ExpectedRevision = archive.Policy.Revision
	if _, err := repository.SaveConfig(ctx, scope, config); err != nil {
		t.Fatal(err)
	}
	if err := contributor.Import(ctx, dataportability.ImportRequest{CollisionPolicy: dataportability.CollisionReplace}, backup); err != nil {
		t.Fatal(err)
	}
	restored, err := repository.Policy(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Config.ViolationAction != "audit_only" || restored.Config.PreventEmotionalBlackmail {
		t.Fatalf("policy was not restored: %+v", restored)
	}
	var count int64
	if err := db.Table("safety_incidents").Where("id='roundtrip' AND owner_id='device' AND core_id='space'").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("replace collision count=%d err=%v", count, err)
	}
	if err := contributor.Import(ctx, dataportability.ImportRequest{CollisionPolicy: dataportability.CollisionDuplicate}, backup); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("safety_incidents").Where("owner_id='device' AND core_id='space'").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("duplicate collision count=%d err=%v", count, err)
	}
}

func TestGovernorAppliesConfiguredOutputPolicies(t *testing.T) {
	policy := DefaultPolicy()
	policy.Config.PreventEmotionalBlackmail = true
	policy.Config.ViolationAction = "block"
	governor := NewPolicyGovernor(policy)
	result := governor.CheckPostGen(PostGenInput{GeneratedText: "你欠我的，所以必须留下。"})
	if !result.Audited || result.Allowed || !result.Replacement || result.Cleaned == "你欠我的，所以必须留下。" || !strings.Contains(strings.Join(result.Reasons, ","), "emotional_blackmail") {
		t.Fatalf("blocked policy did not replace output: %+v", result)
	}
	policy.Config.ViolationAction = "audit_only"
	result = NewPolicyGovernor(policy).CheckPostGen(PostGenInput{GeneratedText: "你欠我的，所以必须留下。"})
	if !result.Audited || !result.Allowed || result.Cleaned != "你欠我的，所以必须留下。" || result.Replacement {
		t.Fatalf("audit-only policy incorrectly replaced output: %+v", result)
	}
	policy.Config.PreventEmotionalBlackmail = false
	result = NewPolicyGovernor(policy).CheckPostGen(PostGenInput{GeneratedText: "你欠我的，所以必须留下。"})
	if result.Audited || !result.Allowed {
		t.Fatalf("disabled policy still matched: %+v", result)
	}
}

func TestGovernorBlocksConfiguredRestrictedInput(t *testing.T) {
	policy := DefaultPolicy()
	policy.Config.RestrictAdultContent = true
	result := NewPolicyGovernor(policy).CheckPreGen(PreGenInput{CharacterID: "role", Text: "请生成色情内容"})
	if result.Allowed || !strings.Contains(strings.Join(result.Reasons, ","), "adult_content") {
		t.Fatalf("restricted input was not blocked: %+v", result)
	}
	policy.Config.RestrictAdultContent = false
	result = NewPolicyGovernor(policy).CheckPreGen(PreGenInput{CharacterID: "role", Text: "请生成色情内容"})
	if !result.Allowed {
		t.Fatalf("disabled restricted input policy still blocked: %+v", result)
	}
}

func TestPolicyLoadDoesNotRequireLegacySettings(t *testing.T) {
	db := governanceTestDB(t)
	policy, err := NewRepository(db).RuntimePolicy(context.Background(), "space", "device")
	if err != nil {
		t.Fatal(err)
	}
	if policy.Revision != 1 || !policy.Config.PreventEmotionalBlackmail || policy.Config.ViolationAction != "block" {
		t.Fatalf("default policy mismatch: %+v", policy)
	}
}

func TestSafetyIncidentFeedbackIsIdempotent(t *testing.T) {
	db := governanceTestDB(t)
	repository := NewRepository(db)
	feedbackID := int64(42)
	incident := Incident{ID: "incident-one", SpaceID: "space", OwnerID: "owner", CoreID: "core", RoleID: "role", ConversationID: "conversation", MessageID: "message", FeedbackID: &feedbackID, RequestID: "request", EventType: "user_reported_unsafe", Severity: "high", Description: "report", Status: "open", Revision: 1, DetailsJSON: "{}"}
	if err := repository.RecordIncident(t.Context(), incident); err != nil {
		t.Fatal(err)
	}
	incident.ID = "incident-two"
	if err := repository.RecordIncident(t.Context(), incident); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Table("safety_incidents").Where("feedback_id=?", feedbackID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("feedback incident duplicated: %d %v", count, err)
	}
}
