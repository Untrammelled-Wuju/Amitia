package notificationruntime

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/conversationstream"
	"gorm.io/gorm"
)

func TestPreviewForMode(t *testing.T) {
	title, body := PreviewForMode("hidden", "角色", "秘密内容")
	if title != "Amitia" || body != "你有一条新消息" {
		t.Fatalf("hidden preview leaked content: title=%q body=%q", title, body)
	}

	title, body = PreviewForMode("sender_only", "角色", "秘密内容")
	if title != "角色" || body != "发来了一条消息" {
		t.Fatalf("unexpected sender-only preview: title=%q body=%q", title, body)
	}

	title, body = PreviewForMode("full", "角色", "**你好** https://example.com")
	if title != "角色" || body != "你好" {
		t.Fatalf("full preview sanitization failed: title=%q body=%q", title, body)
	}
}

func TestRetryableProviderResult(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		{"network_error", true},
		{"request_failed", true},
		{"http_408", true},
		{"http_429", true},
		{"http_500", true},
		{"http_503", true},
		{"http_400", false},
		{"auth_failed", false},
		{"provider_unavailable", false},
	}
	for _, tc := range cases {
		if got := retryableProviderResult(ProviderResult{ErrorCode: tc.code}); got != tc.want {
			t.Fatalf("retryability for %s = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestTokenClearSet(t *testing.T) {
	set := tokenClearSet([]string{" FCM ", "apns-voip", "VIVO", "invalid", ""})
	for _, key := range []string{"fcm", "apns-voip", "vivo"} {
		if !set[key] {
			t.Fatalf("expected clear token key %q", key)
		}
	}
	if set["invalid"] {
		t.Fatal("unexpected unsupported token provider in clear set")
	}
}

func TestNormalizeAppearance(t *testing.T) {
	if got := normalizeAppearance(" DARK "); got != "dark" {
		t.Fatalf("normalize dark = %q", got)
	}
	if got := normalizeAppearance("light"); got != "light" {
		t.Fatalf("normalize light = %q", got)
	}
	if got := normalizeAppearance("system"); got != "" {
		t.Fatalf("unsupported appearance must fail closed, got %q", got)
	}
}

func TestNativeDataCapabilitiesFailClosedForOppoVivo(t *testing.T) {
	gateway := NewPushGateway(nil, nil, false)
	caps := gateway.NativeDataCapabilities()
	if caps["oppo"] || caps["vivo"] {
		t.Fatalf("OPPO/vivo cloud native-data must stay fail-closed: %#v", caps)
	}
	if _, ok := caps["mipush"]; !ok {
		t.Fatalf("native-data capability contract missing mipush: %#v", caps)
	}
}

func TestExecutionLocalizationAndPrivateFallback(t *testing.T) {
	state := ExecutionState{
		Title:   "Amitia 正在执行",
		Summary: "等待你的确认",
		Phase:   "waiting_approval",
	}
	en := localizeExecutionState(state, "en-US")
	if en.Title != "Amitia is working" || en.Summary != "Waiting for your approval" {
		t.Fatalf("unexpected English execution localization: %#v", en)
	}
	tw := localizeExecutionState(state, "zh-Hant-TW")
	if tw.Title != "Amitia 正在執行" || tw.Summary != "等待你的確認" {
		t.Fatalf("unexpected Traditional Chinese localization: %#v", tw)
	}
	private := executionStateForPreview(state, "hidden")
	if got := executionNotificationBody(private, "zh-CN"); got != "等待你的确认" {
		t.Fatalf("unexpected zh private fallback: %q", got)
	}
	if got := executionNotificationBody(private, "en-US"); got != "Waiting for your approval" {
		t.Fatalf("unexpected en private fallback: %q", got)
	}
}

func TestStableVendorNotifyID(t *testing.T) {
	envelope := PushEnvelope{
		NotificationID: "notice-1",
		MessageID:      "message-1",
		RunID:          "run-1",
	}
	first := stableVendorNotifyID(envelope)
	second := stableVendorNotifyID(envelope)
	if first != second {
		t.Fatalf("notify id is not deterministic: %d != %d", first, second)
	}
	if first < 0 {
		t.Fatalf("notify id must be non-negative: %d", first)
	}
}

func TestRepositoryDisablesOnlyInvalidProviderToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:notification-runtime-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&DeviceEndpoint{}); err != nil {
		t.Fatalf("migrate notification device: %v", err)
	}

	repo := NewRepository(db)
	ctx := context.Background()
	endpoint := &DeviceEndpoint{
		SpaceID:           "space-test",
		DeviceID:          "device-test",
		Platform:          "android",
		PreferredProvider: "hms",
		FCMToken:          "fcm-token",
		HMSToken:          "hms-token",
		MiPushToken:       "mi-token",
		OppoToken:         "oppo-token",
		VivoToken:         "vivo-token",
		HonorToken:        "honor-token",
		PushEnabled:       true,
	}
	if err := repo.UpsertDevice(ctx, endpoint); err != nil {
		t.Fatalf("upsert device: %v", err)
	}

	if err := repo.DisableToken(ctx, endpoint, "hms"); err != nil {
		t.Fatalf("disable hms token: %v", err)
	}
	stored, err := repo.GetDeviceForPlatform(ctx, "space-test", "device-test", "android")
	if err != nil {
		t.Fatalf("read device: %v", err)
	}
	if stored.HMSToken != "" {
		t.Fatalf("HMS token was not cleared: %q", stored.HMSToken)
	}
	if stored.FCMToken != "fcm-token" ||
		stored.MiPushToken != "mi-token" ||
		stored.OppoToken != "oppo-token" ||
		stored.VivoToken != "vivo-token" ||
		stored.HonorToken != "honor-token" {
		t.Fatalf("clearing HMS token modified another provider: %#v", stored)
	}
}

func TestStableVendorNotifyIDUsesBusinessIdentity(t *testing.T) {
	first := stableVendorNotifyID(PushEnvelope{
		NotificationID: "notification-1",
		RunID:          "run-42",
		MessageID:      "message-1",
	})
	second := stableVendorNotifyID(PushEnvelope{
		NotificationID: "notification-2",
		RunID:          "run-42",
		MessageID:      "message-2",
	})
	if first != second {
		t.Fatalf("same run must replace previous vendor notification: %d != %d", first, second)
	}

	conversationFirst := stableVendorNotifyID(PushEnvelope{
		NotificationID: "notification-3",
		ConversationID: "conversation-7",
		MessageID:      "message-3",
	})
	conversationSecond := stableVendorNotifyID(PushEnvelope{
		NotificationID: "notification-4",
		ConversationID: "conversation-7",
		MessageID:      "message-4",
	})
	if conversationFirst != conversationSecond {
		t.Fatalf("same conversation must share vendor notification identity: %d != %d", conversationFirst, conversationSecond)
	}
}

func TestAndroidChannelForEnvelope(t *testing.T) {
	cases := []struct {
		event string
		want  string
	}{
		{"message.completed", "amitia_messages"},
		{"run.update", "amitia_tasks"},
		{"call.incoming", "amitia_calls"},
		{"reminder.triggered", "amitia_reminders"},
		{"system.test", "amitia_system"},
	}
	for _, tc := range cases {
		if got := androidChannelForEnvelope(PushEnvelope{Type: tc.event}); got != tc.want {
			t.Fatalf("channel for %s = %q, want %q", tc.event, got, tc.want)
		}
	}
}

func TestLiveActivityRepositoryIsSpaceScoped(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repo := NewRepository(db)
	if err := repo.InitSchema(); err != nil {
		t.Fatalf("migrate notification schema: %v", err)
	}
	ctx := context.Background()
	if err := repo.UpsertLiveActivityToken(ctx, &LiveActivityToken{
		SpaceID:     "space-a",
		DeviceID:    "device-1",
		RunID:       "run-1",
		ActivityID:  "activity-a",
		UpdateToken: "token-a",
	}); err != nil {
		t.Fatalf("upsert live activity: %v", err)
	}

	if _, err := repo.GetLiveActivityToken(ctx, "space-b", "device-1", "run-1"); err == nil {
		t.Fatal("live activity token leaked across spaces")
	}
	got, err := repo.GetLiveActivityToken(ctx, "space-a", "device-1", "run-1")
	if err != nil {
		t.Fatalf("read scoped live activity: %v", err)
	}
	if got.UpdateToken != "token-a" {
		t.Fatalf("unexpected scoped update token: %q", got.UpdateToken)
	}
}

func TestRestoreActiveRunsUsesPersistedTimestampFormat(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	runtime := NewRuntime(db, nil, false)
	if err := runtime.repo.InitSchema(); err != nil {
		t.Fatalf("migrate notification schema: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE conversations (
			id TEXT PRIMARY KEY,
			space_id TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT ''
		)`).Error; err != nil {
		t.Fatalf("create conversations: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE assistant_turns (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL,
			character_id TEXT NOT NULL DEFAULT '',
			execution_id TEXT NOT NULL DEFAULT '',
			agent_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`).Error; err != nil {
		t.Fatalf("create assistant_turns: %v", err)
	}
	now := time.Now()
	created := now.Add(-30 * time.Second).Format("2006-01-02 15:04:05")
	updated := now.Format("2006-01-02 15:04:05")
	if err := db.Exec(
		"INSERT INTO conversations(id, space_id, title) VALUES (?, ?, ?)",
		"conversation-a", "space-a", "Recovered task",
	).Error; err != nil {
		t.Fatalf("insert conversation: %v", err)
	}
	if err := db.Exec(
		"INSERT INTO assistant_turns(id, conversation_id, character_id, execution_id, agent_id, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		"turn-a", "conversation-a", "character-a", "run-a", "codex", "running", created, updated,
	).Error; err != nil {
		t.Fatalf("insert assistant turn: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtime.ctx = ctx
	runtime.restoreActiveRuns(ctx)

	runtime.mu.Lock()
	restored := runtime.active["run-a"]
	runtime.mu.Unlock()
	if restored == nil {
		t.Fatal("active run was not restored")
	}
	if !restored.notified {
		t.Fatal("long-running persisted task should restore as surfaced")
	}
	if restored.spaceID != "space-a" || restored.state.ConversationID != "conversation-a" {
		t.Fatalf("restored scope mismatch: %#v", restored)
	}
	if restored.state.AgentID != "codex" {
		t.Fatalf("restored agent = %q", restored.state.AgentID)
	}
}

func TestLiveActivityTokenPreservesConversationOnTokenRefresh(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repo := NewRepository(db)
	if err := repo.InitSchema(); err != nil {
		t.Fatalf("migrate notification schema: %v", err)
	}
	ctx := context.Background()
	if err := repo.UpsertLiveActivityToken(ctx, &LiveActivityToken{
		SpaceID:        "space-a",
		DeviceID:       "device-a",
		ConversationID: "conversation-a",
		RunID:          "run-a",
		ActivityID:     "activity-a",
		UpdateToken:    "token-a",
		Revision:       9,
	}); err != nil {
		t.Fatalf("insert live activity: %v", err)
	}
	if err := repo.UpsertLiveActivityToken(ctx, &LiveActivityToken{
		SpaceID:     "space-a",
		DeviceID:    "device-a",
		RunID:       "run-a",
		ActivityID:  "activity-a",
		UpdateToken: "token-b",
	}); err != nil {
		t.Fatalf("refresh live activity token: %v", err)
	}
	got, err := repo.GetLiveActivityToken(ctx, "space-a", "device-a", "run-a")
	if err != nil {
		t.Fatalf("read live activity: %v", err)
	}
	if got.ConversationID != "conversation-a" {
		t.Fatalf("conversation identity lost on token refresh: %q", got.ConversationID)
	}
	if got.UpdateToken != "token-b" {
		t.Fatalf("update token not refreshed: %q", got.UpdateToken)
	}
	if got.Revision != 9 {
		t.Fatalf("token refresh regressed revision: %d", got.Revision)
	}
}

func TestEndedLiveActivityCannotBeRevivedByLateToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repo := NewRepository(db)
	if err := repo.InitSchema(); err != nil {
		t.Fatalf("migrate notification schema: %v", err)
	}
	ctx := context.Background()
	if err := repo.UpsertLiveActivityToken(ctx, &LiveActivityToken{
		SpaceID:        "space-a",
		DeviceID:       "device-a",
		ConversationID: "conversation-a",
		RunID:          "run-a",
		ActivityID:     "activity-a",
		UpdateToken:    "token-a",
		Revision:       4,
	}); err != nil {
		t.Fatalf("insert live activity: %v", err)
	}
	if err := repo.EndLiveActivity(ctx, "space-a", "device-a", "run-a", 5); err != nil {
		t.Fatalf("end live activity: %v", err)
	}
	if err := repo.UpsertLiveActivityToken(ctx, &LiveActivityToken{
		SpaceID:     "space-a",
		DeviceID:    "device-a",
		RunID:       "run-a",
		ActivityID:  "activity-a",
		UpdateToken: "late-token",
		Revision:    0,
	}); err != nil {
		t.Fatalf("late token should be idempotent: %v", err)
	}
	if _, err := repo.GetLiveActivityToken(ctx, "space-a", "device-a", "run-a"); err == nil {
		t.Fatal("late token callback revived terminal live activity")
	}
	var stored LiveActivityToken
	if err := db.Where(
		"space_id = ? AND device_id = ? AND run_id = ?",
		"space-a", "device-a", "run-a",
	).First(&stored).Error; err != nil {
		t.Fatalf("read terminal live activity: %v", err)
	}
	if stored.EndedAt == nil || stored.Revision != 5 || stored.UpdateToken != "token-a" {
		t.Fatalf("terminal row mutated by late token: %#v", stored)
	}
}

func TestRevokeDeviceStopsPendingNotificationState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repo := NewRepository(db)
	if err := repo.InitSchema(); err != nil {
		t.Fatalf("migrate notification schema: %v", err)
	}
	ctx := context.Background()
	endpoint := &DeviceEndpoint{
		SpaceID:     "space-a",
		DeviceID:    "device-a",
		Platform:    "ios",
		PushEnabled: true,
	}
	if err := repo.UpsertDevice(ctx, endpoint); err != nil {
		t.Fatalf("upsert device: %v", err)
	}
	if err := repo.UpsertLiveActivityToken(ctx, &LiveActivityToken{
		SpaceID:     "space-a",
		DeviceID:    "device-a",
		RunID:       "run-a",
		ActivityID:  "activity-a",
		UpdateToken: "update-a",
	}); err != nil {
		t.Fatalf("upsert live activity: %v", err)
	}
	now := time.Now().UTC()
	outbox := &NotificationOutbox{
		NotificationID: "notification-a",
		SpaceID:        "space-a",
		DeviceID:       "device-a",
		EnvelopeJSON:   "{}",
		Status:         "queued",
		NextAttemptAt:  now,
		ExpiresAt:      now.Add(time.Minute),
	}
	if err := repo.CreateOutbox(ctx, outbox); err != nil {
		t.Fatalf("create outbox: %v", err)
	}

	if err := repo.RevokeDevice(ctx, "space-a", "device-a"); err != nil {
		t.Fatalf("revoke device: %v", err)
	}
	if _, err := repo.GetDevice(ctx, "space-a", "device-a"); err == nil {
		t.Fatal("revoked device still returned as active")
	}
	var storedOutbox NotificationOutbox
	if err := db.Where("id = ?", outbox.ID).First(&storedOutbox).Error; err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if storedOutbox.Status != "failed" {
		t.Fatalf("revoked device outbox status = %q, want failed", storedOutbox.Status)
	}
	if _, err := repo.GetLiveActivityToken(ctx, "space-a", "device-a", "run-a"); err == nil {
		t.Fatal("revoked device live activity remained active")
	}
}

func TestExecutionStateCapturesTerminalUsage(t *testing.T) {
	state := ExecutionState{}
	updateExecutionState(&state, conversationstream.AgentUIEvent{
		Type: "turn.completed",
		Payload: map[string]any{
			"totalTokens": 4321,
		},
	})
	if state.TotalTokens != 4321 {
		t.Fatalf("terminal usage tokens = %d, want 4321", state.TotalTokens)
	}
	if state.Phase != "completed" || state.Progress != 1 {
		t.Fatalf("terminal execution state mismatch: %#v", state)
	}
}

func TestExecutionStateForPreviewProtectsLockScreenContent(t *testing.T) {
	original := ExecutionState{
		RunID:       "run-1",
		AgentID:     "agent-secret",
		Title:       "秘密任务",
		Summary:     "不应显示的任务详情",
		Phase:       "running",
		TotalTokens: 4321,
	}
	hidden := executionStateForPreview(original, "hidden")
	if hidden.Title != "Amitia" || hidden.Summary != "" ||
		hidden.AgentID != "" || hidden.TotalTokens != 0 {
		t.Fatalf("hidden preview leaked execution data: %#v", hidden)
	}
	senderOnly := executionStateForPreview(original, "sender_only")
	if senderOnly.Title != original.Title || senderOnly.Summary != "" {
		t.Fatalf("sender_only preview mismatch: %#v", senderOnly)
	}
	full := executionStateForPreview(original, "full")
	if full.Title != original.Title || full.Summary != original.Summary || full.AgentID != original.AgentID {
		t.Fatalf("full preview unexpectedly changed: %#v", full)
	}
}

func TestCollapseKeyKeepsDistinctMessages(t *testing.T) {
	first := collapseKey(PushEnvelope{
		Type:           "message.completed",
		ConversationID: "conversation-1",
		MessageID:      "message-1",
	})
	second := collapseKey(PushEnvelope{
		Type:           "message.completed",
		ConversationID: "conversation-1",
		MessageID:      "message-2",
	})
	if first == second {
		t.Fatalf("distinct messages in one conversation must not collapse: %q", first)
	}
	if got := collapseKey(PushEnvelope{
		Type:  "run.update",
		RunID: "run-1",
	}); got != "run:run-1" {
		t.Fatalf("run collapse key = %q", got)
	}
}

func TestRecentLiveActivityStartSurvivesRuntimeRestart(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repo := NewRepository(db)
	if err := repo.InitSchema(); err != nil {
		t.Fatalf("migrate notification schema: %v", err)
	}
	now := time.Now().UTC()
	if err := repo.CreateDelivery(context.Background(), &DeliveryLog{
		NotificationID: "start-a",
		SpaceID:        "space-a",
		DeviceID:       "device-a",
		Provider:       "apns",
		Type:           "liveactivity.start",
		Status:         "accepted",
		QueuedAt:       now,
		SentAt:         &now,
		AcceptedAt:     &now,
		ExpiresAt:      now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	recent, err := repo.HasRecentLiveActivityStart(
		context.Background(),
		"space-a",
		"device-a",
		30*time.Minute,
	)
	if err != nil {
		t.Fatalf("query recent live activity start: %v", err)
	}
	if !recent {
		t.Fatal("accepted Live Activity start was not recovered from durable delivery log")
	}
	other, err := repo.HasRecentLiveActivityStart(
		context.Background(),
		"space-a",
		"device-b",
		30*time.Minute,
	)
	if err != nil {
		t.Fatalf("query other device: %v", err)
	}
	if other {
		t.Fatal("Live Activity start throttle leaked across devices")
	}
}

func TestCreateOutboxConflictReusesPersistedIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repo := NewRepository(db)
	if err := repo.InitSchema(); err != nil {
		t.Fatalf("migrate notification schema: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	first := &NotificationOutbox{
		NotificationID: "liveactivity:start:run-1:1",
		SpaceID:        "space-a",
		DeviceID:       "device-a",
		EnvelopeJSON:   "{}",
		Status:         "retry",
		NextAttemptAt:  now.Add(time.Minute),
		ExpiresAt:      now.Add(2 * time.Minute),
	}
	if err := repo.CreateOutbox(ctx, first); err != nil {
		t.Fatalf("create first outbox: %v", err)
	}
	second := &NotificationOutbox{
		NotificationID: first.NotificationID,
		SpaceID:        first.SpaceID,
		DeviceID:       first.DeviceID,
		EnvelopeJSON:   "{}",
		Status:         "queued",
		NextAttemptAt:  now,
		ExpiresAt:      now.Add(2 * time.Minute),
	}
	if err := repo.CreateOutbox(ctx, second); err != nil {
		t.Fatalf("create duplicate outbox: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate outbox did not reuse persisted ID: %q != %q", second.ID, first.ID)
	}
	if second.Status != "retry" {
		t.Fatalf("duplicate outbox status = %q, want retry", second.Status)
	}
	if !second.NextAttemptAt.Equal(first.NextAttemptAt) {
		t.Fatalf("duplicate outbox backoff was not preserved")
	}
}
