package v2

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCommandServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:command-service-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&RuntimeCommand{}, &DeviceCommandSequence{}); err != nil {
		t.Fatalf("migrate runtime command: %v", err)
	}
	return db
}

func TestCommandProgressNeverRegressesWhenAckBeatsTransportMark(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	cmd := &RuntimeCommand{
		ID: "cmd-race", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1", CommandType: string(CommandTypePlayAction), Durability: "ephemeral",
		Status: string(CommandStatusQueued), PayloadJSON: `{}`, CreatedAt: now.Format(time.RFC3339), UpdatedAt: now.Format(time.RFC3339),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatalf("create command: %v", err)
	}
	if err := svc.MarkDispatching(cmd.ID, "runtime-1", "session-1", now); err != nil {
		t.Fatalf("dispatching: %v", err)
	}
	if err := svc.MarkRuntimeReceived(cmd.ID, "runtime-1", "session-1", now.Add(time.Millisecond)); err != nil {
		t.Fatalf("runtime received: %v", err)
	}
	// This is the dispatcher write that can race behind a very fast ACK.
	if err := svc.MarkTransportDispatched(cmd.ID, "runtime-1", now.Add(2*time.Millisecond)); err != nil {
		t.Fatalf("late transport mark: %v", err)
	}
	got, err := svc.GetCommand(cmd.ID)
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if got.Status != string(CommandStatusRuntimeReceived) {
		t.Fatalf("status regressed: got %s", got.Status)
	}
}

func TestCompletedCommandCannotBeOverwrittenByLateFailure(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	cmd := &RuntimeCommand{
		ID: "cmd-terminal", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1", CommandType: string(CommandTypeRecenterOnce), Durability: "ephemeral",
		Status: string(CommandStatusRuntimeAccepted), PayloadJSON: `{}`, CreatedAt: now.Format(time.RFC3339), UpdatedAt: now.Format(time.RFC3339),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatalf("create command: %v", err)
	}
	if err := svc.MarkCompleted(cmd.ID, "", now); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := svc.MarkFailed(cmd.ID, "LATE", "late failure", now.Add(time.Millisecond)); err == nil {
		t.Fatal("late failure must not overwrite completed terminal state")
	}
	got, err := svc.GetCommand(cmd.ID)
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if got.Status != string(CommandStatusCompleted) {
		t.Fatalf("terminal status overwritten: got %s", got.Status)
	}
}

func TestCompletedCommandCannotReenterDispatching(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	cmd := &RuntimeCommand{
		ID: "cmd-no-redispatch", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1",
		CommandType: string(CommandTypeSyncDesiredState), Durability: "durable", Status: string(CommandStatusCompleted),
		DeviceSequence: 8, PayloadJSON: `{}`, PayloadHash: ComputePayloadHash([]byte(`{}`)), PayloadSchemaVersion: 1,
		CreatedAt: now.Format("2006-01-02 15:04:05"), UpdatedAt: now.Format("2006-01-02 15:04:05"), CompletedAt: now.Format("2006-01-02 15:04:05"),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatalf("create completed command: %v", err)
	}
	if err := svc.MarkDispatching(cmd.ID, "runtime-1", "session-1", now.Add(time.Second)); err == nil {
		t.Fatal("completed command must not re-enter dispatching")
	}
	got, err := svc.GetCommand(cmd.ID)
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if got.Status != string(CommandStatusCompleted) {
		t.Fatalf("completed command changed status: %s", got.Status)
	}
}

func TestExpiredDurableDuplicateRevivesSameCommand(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	payload := SyncDesiredStatePayload{
		DesiredRevision: 12,
		DesiredHash:     "hash-12",
		InstallationID:  "inst-1",
		PetID:           "pet-1",
		ReleaseID:       "release-1",
	}
	cmd, err := svc.CreateDurableCommand("u", "d", string(CommandTypeSyncDesiredState), "desired:d:12", "desired:d", 12, payload)
	if err != nil {
		t.Fatalf("create durable: %v", err)
	}
	if err := svc.MarkExpired(cmd.ID, time.Now().UTC()); err != nil {
		t.Fatalf("expire durable: %v", err)
	}
	revived, err := svc.CreateDurableCommand("u", "d", string(CommandTypeSyncDesiredState), "desired:d:12", "desired:d", 13, payload)
	if err != nil {
		t.Fatalf("revive expired durable: %v", err)
	}
	if revived.ID != cmd.ID {
		t.Fatalf("recovery must revive the same idempotent command: got=%s want=%s", revived.ID, cmd.ID)
	}
	if revived.Status != string(CommandStatusQueued) || revived.DeviceSequence != 13 || revived.RuntimeSessionID != "" {
		t.Fatalf("revived command not dispatchable: status=%s seq=%d session=%q", revived.Status, revived.DeviceSequence, revived.RuntimeSessionID)
	}
}

func TestDurableCommandCreationRejectsNonDurableType(t *testing.T) {
	svc := NewCommandService(newCommandServiceTestDB(t))
	payload := SyncDesiredStatePayload{DesiredRevision: 1, DesiredHash: "hash-1"}
	if _, err := svc.CreateDurableCommand(
		"u", "d", string(CommandTypeRecenterOnce), "bad-durable:1", "desired:d", 1, payload,
	); err == nil {
		t.Fatal("non-durable command type must not be persisted as durable")
	}
}

func TestUnboundEphemeralCommandCreationIsRejected(t *testing.T) {
	svc := NewCommandService(newCommandServiceTestDB(t))
	if _, err := svc.CreateEphemeralCommand("u", "d", string(CommandTypeRecenterOnce), "unsafe:1", []byte(`{}`)); err == nil {
		t.Fatal("unbound ephemeral creation must fail closed")
	}
}

func TestSessionBoundEphemeralRejectsUnknownCommandType(t *testing.T) {
	svc := NewCommandService(newCommandServiceTestDB(t))
	if _, err := svc.CreateEphemeralCommandForSession(
		"u", "d", "runtime-1", "session-1", "installation-1",
		"runtime.command.typo", "unknown:1", []byte(`{}`),
	); err == nil {
		t.Fatal("unknown ephemeral runtime command type must fail closed")
	}
}

func TestAllSessionBoundEphemeralCommandsCarryAuthoritativeExpiry(t *testing.T) {
	svc := NewCommandService(newCommandServiceTestDB(t))
	types := []CommandType{
		CommandTypePlayAction,
		CommandTypeStopAction,
		CommandTypePauseAction,
		CommandTypeResumeAction,
		CommandTypeRecenterOnce,
	}
	for i, commandType := range types {
		payload := []byte(`{}`)
		if commandType == CommandTypePlayAction {
			payload = []byte(`{"runtimeId":"runtime-1","actionKey":"wave","characterId":"char-1","petInstanceId":"runtime-1","installationId":"installation-1"}`)
		}
		before := time.Now().UTC()
		cmd, err := svc.CreateEphemeralCommandForSession(
			"u", "d", "runtime-1", "session-1", "installation-1",
			string(commandType), fmt.Sprintf("ephemeral-expiry:%d", i), payload,
		)
		if err != nil {
			t.Fatalf("create %s: %v", commandType, err)
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, cmd.ExpiresAt)
		if err != nil {
			t.Fatalf("%s expiresAt is invalid: %q err=%v", commandType, cmd.ExpiresAt, err)
		}
		if !expiresAt.After(before) || expiresAt.After(before.Add(defaultEphemeralCommandTTL+2*time.Second)) {
			t.Fatalf("%s authoritative expiry out of bounds: %s", commandType, expiresAt)
		}
	}
}

func TestEphemeralCommandGetsDeviceSequence(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	cmd, err := svc.CreateEphemeralCommandForSession("u", "d", "runtime-1", "session-1", "installation-1", string(CommandTypeRecenterOnce), "recenter:1", []byte(`{}`))
	if err != nil {
		t.Fatalf("create ephemeral: %v", err)
	}
	if cmd.DeviceSequence <= 0 {
		t.Fatalf("ephemeral command must have a replay sequence: %d", cmd.DeviceSequence)
	}
}

func TestScopedDispatchDoesNotHeadOfLineBlockAcrossDevices(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC().Add(-2 * time.Second).Format("2006-01-02 15:04:05")

	for i := 0; i < 150; i++ {
		cmd := &RuntimeCommand{
			ID: "offline-" + fmt.Sprint(i), UserID: "u", DeviceID: "offline", RuntimeID: "runtime-1",
			CommandType: string(CommandTypeRecenterOnce), Durability: "ephemeral", Status: string(CommandStatusQueued),
			DeviceSequence: int64(i + 1), PayloadJSON: `{}`, PayloadHash: ComputePayloadHash([]byte(`{}`)),
			PayloadSchemaVersion: 1, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(cmd).Error; err != nil {
			t.Fatalf("create offline command %d: %v", i, err)
		}
	}
	online := &RuntimeCommand{
		ID: "online", UserID: "u", DeviceID: "online", RuntimeID: "runtime-1",
		CommandType: string(CommandTypeRecenterOnce), Durability: "ephemeral", Status: string(CommandStatusQueued),
		DeviceSequence: 151, PayloadJSON: `{}`, PayloadHash: ComputePayloadHash([]byte(`{}`)),
		PayloadSchemaVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(online).Error; err != nil {
		t.Fatalf("create online command: %v", err)
	}

	cmds, err := svc.ListCommandsToDispatchForConnection("u", "online", "runtime-1", 10)
	if err != nil {
		t.Fatalf("list scoped commands: %v", err)
	}
	if len(cmds) != 1 || cmds[0].ID != "online" {
		t.Fatalf("online device was head-of-line blocked: %#v", cmds)
	}
}

func TestDurableTransportFailureBecomesRetryableAndDispatchable(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	cmd := &RuntimeCommand{
		ID: "durable-retry", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1",
		CommandType: string(CommandTypeSyncDesiredState), Durability: "durable", Status: string(CommandStatusTransportDispatched),
		DeviceSequence: 7, PayloadJSON: `{}`, PayloadHash: ComputePayloadHash([]byte(`{}`)), PayloadSchemaVersion: 1,
		RuntimeSessionID: "old-session", CreatedAt: now.Add(-10 * time.Minute).Format("2006-01-02 15:04:05"), UpdatedAt: now.Add(-6 * time.Minute).Format("2006-01-02 15:04:05"),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatalf("create command: %v", err)
	}
	if err := svc.MarkFailedRetryable(cmd.ID, "TRANSPORT_WRITE_FAILED", "socket lost", now); err != nil {
		t.Fatalf("mark retryable: %v", err)
	}
	if err := db.Model(&RuntimeCommand{}).Where("id = ?", cmd.ID).Update("updated_at", now.Add(-2*time.Second).Format("2006-01-02 15:04:05")).Error; err != nil {
		t.Fatalf("age retryable command: %v", err)
	}
	cmds, err := svc.ListCommandsToDispatchForConnection("u", "d", "runtime-1", 10)
	if err != nil {
		t.Fatalf("list retryable: %v", err)
	}
	if len(cmds) != 1 || cmds[0].ID != cmd.ID || cmds[0].RuntimeSessionID != "" {
		t.Fatalf("durable command not retryable after transport loss: %#v", cmds)
	}
}

func TestHelloReconciliationUsesAuthoritativeDesiredStateAndRequeuesInflight(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	if err := db.Exec(`CREATE TABLE desktop_pet_runtime_desired_states (
		user_id TEXT, device_id TEXT, runtime_id TEXT, installation_id TEXT, pet_id TEXT, release_id TEXT,
		desired_enabled INTEGER, desired_visible INTEGER, desired_action_key TEXT,
		settings_snapshot_json TEXT, settings_revision INTEGER, desired_revision INTEGER, desired_hash TEXT
	)`).Error; err != nil {
		t.Fatalf("create desired table: %v", err)
	}
	if err := db.Exec(`INSERT INTO desktop_pet_runtime_desired_states
		(user_id, device_id, runtime_id, installation_id, pet_id, release_id, desired_enabled, desired_visible, desired_action_key, settings_snapshot_json, settings_revision, desired_revision, desired_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"u", "d", "runtime-1", "inst-1", "pet-1", "release-1", 1, 1, "idle", `{"scale":1}`, 4, 9, "hash-9").Error; err != nil {
		t.Fatalf("insert desired state: %v", err)
	}
	if err := db.AutoMigrate(&DeviceCommandSequence{}); err != nil {
		t.Fatalf("migrate sequence: %v", err)
	}

	payload := SyncDesiredStatePayload{DesiredRevision: 9, DesiredHash: "hash-9", InstallationID: "inst-1", PetID: "pet-1", ReleaseID: "release-1"}
	cmd, err := svc.CreateDurableCommand("u", "d", string(CommandTypeSyncDesiredState), "desired:d:9", "desired:d", 1, payload)
	if err != nil {
		t.Fatalf("create durable: %v", err)
	}
	if err := svc.MarkDispatching(cmd.ID, "runtime-1", "old-session", time.Now().UTC()); err != nil {
		t.Fatalf("mark dispatching: %v", err)
	}
	if err := svc.MarkRuntimeReceived(cmd.ID, "runtime-1", "old-session", time.Now().UTC()); err != nil {
		t.Fatalf("mark runtime received: %v", err)
	}

	revision, err := svc.ReconcileDesiredStateOnHello("u", "d", "runtime-1", 0, 2)
	if err != nil {
		t.Fatalf("reconcile hello: %v", err)
	}
	if revision != 9 {
		t.Fatalf("authoritative revision=%d want=9", revision)
	}
	got, err := svc.GetCommand(cmd.ID)
	if err != nil {
		t.Fatalf("get requeued command: %v", err)
	}
	if got.Status != string(CommandStatusQueued) || got.RuntimeSessionID != "" {
		t.Fatalf("in-flight desired command was not requeued: status=%s session=%q", got.Status, got.RuntimeSessionID)
	}
}

func TestHelloReconciliationRequeuesInflightEvenWhenClientClaimsDesiredApplied(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	if err := db.Exec(`CREATE TABLE desktop_pet_runtime_desired_states (
		user_id TEXT, device_id TEXT, runtime_id TEXT, installation_id TEXT, pet_id TEXT, release_id TEXT,
		desired_enabled INTEGER, desired_visible INTEGER, desired_action_key TEXT,
		settings_snapshot_json TEXT, settings_revision INTEGER, desired_revision INTEGER, desired_hash TEXT
	)`).Error; err != nil {
		t.Fatalf("create desired table: %v", err)
	}
	if err := db.Exec(`INSERT INTO desktop_pet_runtime_desired_states
		(user_id, device_id, runtime_id, installation_id, pet_id, release_id, desired_enabled, desired_visible, desired_action_key, settings_snapshot_json, settings_revision, desired_revision, desired_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"u", "d", "runtime-1", "inst-1", "pet-1", "release-1", 1, 1, "idle", `{}`, 1, 9, "hash-9").Error; err != nil {
		t.Fatalf("insert desired state: %v", err)
	}

	payload := SyncDesiredStatePayload{DesiredRevision: 9, DesiredHash: "hash-9", InstallationID: "inst-1", PetID: "pet-1", ReleaseID: "release-1"}
	cmd, err := svc.CreateDurableCommand("u", "d", string(CommandTypeSyncDesiredState), "desired:d:9", "desired:d", 1, payload)
	if err != nil {
		t.Fatalf("create durable: %v", err)
	}
	now := time.Now().UTC()
	if err := svc.MarkDispatching(cmd.ID, "runtime-1", "old-session", now); err != nil {
		t.Fatalf("mark dispatching: %v", err)
	}
	if err := svc.MarkRuntimeReceived(cmd.ID, "runtime-1", "old-session", now); err != nil {
		t.Fatalf("mark received: %v", err)
	}
	if err := svc.MarkRuntimeAccepted(cmd.ID, "runtime-1", "old-session", now); err != nil {
		t.Fatalf("mark accepted: %v", err)
	}

	// The client may have applied revision 9 locally while desired_applied was
	// lost before server persistence. The non-terminal command must still be
	// requeued on the new session before the revision short-circuit is allowed.
	revision, err := svc.ReconcileDesiredStateOnHello("u", "d", "runtime-1", 9, 2)
	if err != nil {
		t.Fatalf("reconcile hello: %v", err)
	}
	if revision != 9 {
		t.Fatalf("authoritative revision=%d want=9", revision)
	}
	got, err := svc.GetCommand(cmd.ID)
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if got.Status != string(CommandStatusQueued) || got.RuntimeSessionID != "" {
		t.Fatalf("in-flight command not recovered from lost terminal event: status=%s session=%q", got.Status, got.RuntimeSessionID)
	}
}

func TestExpiredOlderDesiredCommandDoesNotReviveAfterNewerRevisionExists(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	oldPayload := SyncDesiredStatePayload{
		DesiredRevision: 20,
		DesiredHash:     "hash-20",
		InstallationID:  "inst-1",
		PetID:           "pet-1",
		ReleaseID:       "release-20",
	}
	oldCmd, err := svc.CreateDurableCommand("u", "d", string(CommandTypeSyncDesiredState), "desired:d:20", "desired:d", 20, oldPayload)
	if err != nil {
		t.Fatalf("create old durable: %v", err)
	}
	if err := svc.MarkExpired(oldCmd.ID, time.Now().UTC()); err != nil {
		t.Fatalf("expire old durable: %v", err)
	}

	newPayload := SyncDesiredStatePayload{
		DesiredRevision: 21,
		DesiredHash:     "hash-21",
		InstallationID:  "inst-1",
		PetID:           "pet-1",
		ReleaseID:       "release-21",
	}
	newCmd, err := svc.CreateDurableCommand("u", "d", string(CommandTypeSyncDesiredState), "desired:d:21", "desired:d", 21, newPayload)
	if err != nil {
		t.Fatalf("create newer durable: %v", err)
	}

	stale, err := svc.CreateDurableCommand("u", "d", string(CommandTypeSyncDesiredState), "desired:d:20", "desired:d", 22, oldPayload)
	if !errors.Is(err, ErrCommandDuplication) {
		t.Fatalf("stale recovery must be deduplicated after newer revision: %v", err)
	}
	if stale == nil || stale.ID != oldCmd.ID {
		t.Fatalf("stale recovery returned wrong command: %#v", stale)
	}
	got, err := svc.GetCommand(oldCmd.ID)
	if err != nil {
		t.Fatalf("get stale command: %v", err)
	}
	if got.Status != string(CommandStatusSuperseded) || got.SupersededByCommandID != newCmd.ID || got.CompletedAt == "" {
		t.Fatalf("stale command was revived instead of superseded: status=%s supersededBy=%s completedAt=%q", got.Status, got.SupersededByCommandID, got.CompletedAt)
	}
}

func TestHelloReconciliationRetargetsQueuedDesiredCommandToCurrentRuntime(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	if err := db.Exec(`CREATE TABLE desktop_pet_runtime_desired_states (
		user_id TEXT, device_id TEXT, runtime_id TEXT, installation_id TEXT, pet_id TEXT, release_id TEXT,
		desired_enabled INTEGER, desired_visible INTEGER, desired_action_key TEXT,
		settings_snapshot_json TEXT, settings_revision INTEGER, desired_revision INTEGER, desired_hash TEXT
	)`).Error; err != nil {
		t.Fatalf("create desired table: %v", err)
	}
	if err := db.Exec(`INSERT INTO desktop_pet_runtime_desired_states
		(user_id, device_id, runtime_id, installation_id, pet_id, release_id, desired_enabled, desired_visible, desired_action_key, settings_snapshot_json, settings_revision, desired_revision, desired_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"u", "d", "runtime-old", "inst-1", "pet-1", "release-1", 1, 1, "idle", `{}`, 1, 30, "hash-30").Error; err != nil {
		t.Fatalf("insert desired state: %v", err)
	}

	payload := SyncDesiredStatePayload{DesiredRevision: 30, DesiredHash: "hash-30", InstallationID: "inst-1", PetID: "pet-1", ReleaseID: "release-1"}
	cmd, err := svc.CreateDurableCommand("u", "d", string(CommandTypeSyncDesiredState), "desired:d:30", "desired:d", 30, payload)
	if err != nil {
		t.Fatalf("create durable: %v", err)
	}
	if err := db.Model(&RuntimeCommand{}).Where("id = ?", cmd.ID).Update("runtime_id", "runtime-old").Error; err != nil {
		t.Fatalf("bind old runtime: %v", err)
	}

	revision, err := svc.ReconcileDesiredStateOnHello("u", "d", "runtime-new", 0, 3)
	if err != nil {
		t.Fatalf("reconcile hello: %v", err)
	}
	if revision != 30 {
		t.Fatalf("authoritative revision=%d want=30", revision)
	}
	got, err := svc.GetCommand(cmd.ID)
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if got.Status != string(CommandStatusQueued) || got.RuntimeID != "runtime-new" {
		t.Fatalf("queued desired command was not retargeted: status=%s runtime=%s", got.Status, got.RuntimeID)
	}
	cmds, err := svc.ListCommandsToDispatchForConnection("u", "d", "runtime-new", 10)
	if err != nil {
		t.Fatalf("list dispatchable commands: %v", err)
	}
	if len(cmds) != 1 || cmds[0].ID != cmd.ID {
		t.Fatalf("retargeted command not dispatchable on new runtime: %#v", cmds)
	}
}

func TestSessionBoundEphemeralCarriesAuthoritativeExpiry(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	payload := []byte(`{"runtimeId":"runtime-1","actionKey":"wave","characterId":"char-1","petInstanceId":"runtime-1","installationId":"inst-1"}`)
	before := time.Now().UTC()
	cmd, err := svc.CreateEphemeralCommandForSession("u", "d", "runtime-1", "session-1", "inst-1", string(CommandTypePlayAction), "play:expiry", payload)
	if err != nil {
		t.Fatalf("create session-bound ephemeral: %v", err)
	}
	if cmd.RuntimeID != "runtime-1" || cmd.RuntimeSessionID != "session-1" || cmd.InstallationID != "inst-1" {
		t.Fatalf("ephemeral route was not bound atomically: %#v", cmd)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, cmd.ExpiresAt)
	if err != nil {
		t.Fatalf("expiresAt is not RFC3339Nano: %q err=%v", cmd.ExpiresAt, err)
	}
	if !expiresAt.After(before) || expiresAt.After(before.Add(defaultEphemeralCommandTTL+2*time.Second)) {
		t.Fatalf("unexpected authoritative expiry: %s", expiresAt)
	}
}

func TestSessionBoundPlayActionCannotExtendServerTTL(t *testing.T) {
	svc := NewCommandService(newCommandServiceTestDB(t))
	requested := time.Now().UTC().Add(6 * time.Hour)
	payload := []byte(fmt.Sprintf(`{"runtimeId":"runtime-1","actionKey":"wave","characterId":"char-1","petInstanceId":"runtime-1","installationId":"inst-1","expiresAt":%q}`, requested.Format(time.RFC3339Nano)))
	before := time.Now().UTC()
	cmd, err := svc.CreateEphemeralCommandForSession("u", "d", "runtime-1", "session-1", "inst-1", string(CommandTypePlayAction), "play:ttl-cap", payload)
	if err != nil {
		t.Fatalf("create session-bound ephemeral: %v", err)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, cmd.ExpiresAt)
	if err != nil {
		t.Fatalf("parse authoritative expiry: %v", err)
	}
	if expiresAt.After(before.Add(defaultEphemeralCommandTTL + 2*time.Second)) {
		t.Fatalf("producer extended server TTL: expires=%s requested=%s", expiresAt, requested)
	}
}

func TestExpiryReconcilerAllowsShortEventDeliveryGrace(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	cmd := &RuntimeCommand{
		ID: "cmd-expiry-grace", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1", RuntimeSessionID: "session-1",
		CommandType: string(CommandTypePlayAction), Durability: "ephemeral", Status: string(CommandStatusRendererAccepted),
		ExpiresAt:   now.Add(-ephemeralExpiryReconcileGrace / 2).Format(runtimeCommandExpiryLayout),
		PayloadJSON: `{}`, CreatedAt: now.Add(-time.Minute).Format("2006-01-02 15:04:05"), UpdatedAt: now.Add(-time.Minute).Format("2006-01-02 15:04:05"),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatal(err)
	}
	expired, err := svc.ListExpiredCommands(10, 30)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range expired {
		if got.ID == cmd.ID {
			t.Fatal("renderer-accepted command inside delivery grace must not be reconciled before late start/accept events can arrive")
		}
	}
}

func TestSessionBoundPlayActionRejectsInvalidAuthoritativeExpiry(t *testing.T) {
	svc := NewCommandService(newCommandServiceTestDB(t))
	payload := []byte(`{"runtimeId":"runtime-1","actionKey":"wave","characterId":"char-1","petInstanceId":"runtime-1","installationId":"inst-1","expiresAt":"not-a-time"}`)
	if _, err := svc.CreateEphemeralCommandForSession("u", "d", "runtime-1", "session-1", "inst-1", string(CommandTypePlayAction), "play:bad-expiry", payload); err == nil {
		t.Fatal("invalid play_action expiresAt must fail closed")
	}
}

func TestRendererAcceptanceBindsPlaybackIdentity(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	cmd := &RuntimeCommand{
		ID: "cmd-renderer-bind", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1", RuntimeSessionID: "session-1",
		CommandType: string(CommandTypePlayAction), Durability: "ephemeral", Status: string(CommandStatusRuntimeAccepted),
		PayloadJSON: `{}`, CreatedAt: now.Format("2006-01-02 15:04:05"), UpdatedAt: now.Format("2006-01-02 15:04:05"),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkRendererAccepted(cmd.ID, "runtime-1", "session-1", "playback-1", now); err != nil {
		t.Fatalf("renderer accepted: %v", err)
	}
	stored, err := svc.GetCommand(cmd.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != string(CommandStatusRendererAccepted) || stored.PlaybackRequestID != "playback-1" {
		t.Fatalf("renderer acceptance did not bind playback identity: status=%s playback=%s", stored.Status, stored.PlaybackRequestID)
	}
	if err := svc.MarkPlaybackStarted(cmd.ID, "playback-wrong", now.Add(time.Millisecond)); err == nil {
		t.Fatal("mismatched playback identity must not start command")
	}
	stored, err = svc.GetCommand(cmd.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != string(CommandStatusRendererAccepted) {
		t.Fatalf("mismatched playback start mutated status: %s", stored.Status)
	}
}

func TestExpiryReconcilerIncludesPlaybackStartedEphemeral(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	started := now.Add(-defaultPlaybackLivenessCeiling - time.Minute)
	cmd := &RuntimeCommand{
		ID: "cmd-started-expired", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1", RuntimeSessionID: "session-1",
		CommandType: string(CommandTypePlayAction), Durability: "ephemeral", Status: string(CommandStatusPlaybackStarted),
		PlaybackRequestID: "playback-1", ExpiresAt: started.Add(-time.Second).Format(runtimeCommandExpiryLayout),
		PayloadJSON: `{}`, PlaybackStartedAt: started.Format(time.RFC3339Nano),
		CreatedAt: started.Format("2006-01-02 15:04:05"), UpdatedAt: started.Format("2006-01-02 15:04:05"),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatal(err)
	}
	expired, err := svc.ListExpiredCommands(10, 60)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range expired {
		if item.ID == cmd.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expired playback_started ephemeral command was not reconciled")
	}
}

func TestPlaybackStartedUsesLivenessTimeoutNotAdmissionExpiry(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	cmd := &RuntimeCommand{
		ID: "cmd-started-still-live", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1", RuntimeSessionID: "session-1",
		CommandType: string(CommandTypePlayAction), Durability: "ephemeral", Status: string(CommandStatusPlaybackStarted),
		PlaybackRequestID: "playback-1", ExpiresAt: now.Add(-time.Minute).Format(runtimeCommandExpiryLayout),
		PayloadJSON: `{}`, CreatedAt: now.Add(-2 * time.Minute).Format("2006-01-02 15:04:05"), UpdatedAt: now.Add(-2 * time.Minute).Format("2006-01-02 15:04:05"),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatal(err)
	}
	expired, err := svc.ListExpiredCommands(10, 60)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range expired {
		if item.ID == cmd.ID {
			t.Fatal("started playback must not be expired by its pre-start admission deadline")
		}
	}
}

func TestReconnectSupersedeIsScopedToOldSession(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	old := &RuntimeCommand{ID: "old", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1", RuntimeSessionID: "session-old", CommandType: string(CommandTypeRecenterOnce), Durability: "ephemeral", Status: string(CommandStatusQueued), PayloadJSON: `{}`, CreatedAt: now.Format("2006-01-02 15:04:05"), UpdatedAt: now.Format("2006-01-02 15:04:05")}
	fresh := &RuntimeCommand{ID: "fresh", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1", RuntimeSessionID: "session-new", CommandType: string(CommandTypeRecenterOnce), Durability: "ephemeral", Status: string(CommandStatusQueued), PayloadJSON: `{}`, CreatedAt: now.Format("2006-01-02 15:04:05"), UpdatedAt: now.Format("2006-01-02 15:04:05")}
	if err := db.Create(old).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(fresh).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.SupersedeEphemeralCommands("u", "d", "runtime-1", "session-old", "replaced", now); err != nil {
		t.Fatal(err)
	}
	gotOld, _ := svc.GetCommand(old.ID)
	gotFresh, _ := svc.GetCommand(fresh.ID)
	if gotOld.Status != string(CommandStatusSuperseded) {
		t.Fatalf("old session command not superseded: %s", gotOld.Status)
	}
	if gotFresh.Status != string(CommandStatusQueued) {
		t.Fatalf("new session command was incorrectly superseded: %s", gotFresh.Status)
	}
}

func TestSessionBoundPlayActionRejectsIncompleteTargetIdentity(t *testing.T) {
	svc := NewCommandService(newCommandServiceTestDB(t))
	cases := []struct {
		name    string
		payload string
	}{
		{"missing runtime", `{"actionKey":"wave","characterId":"char-1","petInstanceId":"runtime-1","installationId":"inst-1"}`},
		{"missing character", `{"runtimeId":"runtime-1","actionKey":"wave","petInstanceId":"runtime-1","installationId":"inst-1"}`},
		{"missing pet instance", `{"runtimeId":"runtime-1","actionKey":"wave","characterId":"char-1","installationId":"inst-1"}`},
		{"stale installation", `{"runtimeId":"runtime-1","actionKey":"wave","characterId":"char-1","petInstanceId":"runtime-1","installationId":"inst-old"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.CreateEphemeralCommandForSession("u", "d", "runtime-1", "session-1", "inst-1", string(CommandTypePlayAction), "play:identity:"+tc.name, []byte(tc.payload)); err == nil {
				t.Fatalf("invalid target identity must fail closed: %s", tc.payload)
			}
		})
	}
}

func TestPlaybackStartedUsesCommandMaximumInsteadOfTransportTimeout(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	started := now.Add(-10 * time.Minute)
	cmd := &RuntimeCommand{
		ID: "cmd-long-playback", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1", RuntimeSessionID: "session-1",
		CommandType: string(CommandTypePlayAction), Durability: "ephemeral", Status: string(CommandStatusPlaybackStarted),
		PayloadJSON: `{"maximumPlayMs":3600000}`, PlaybackStartedAt: started.Format(time.RFC3339Nano),
		CreatedAt: started.Format("2006-01-02 15:04:05"), UpdatedAt: started.Format("2006-01-02 15:04:05"),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatal(err)
	}
	expired, err := svc.ListExpiredCommands(10, 30)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range expired {
		if got.ID == cmd.ID {
			t.Fatal("10-minute playback with a 1-hour maximum must not be killed by the 30-second transport timeout")
		}
	}
}

func TestPlaybackStartedExpiresAfterMaximumPlusGrace(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	started := time.Now().UTC().Add(-3 * time.Minute)
	cmd := &RuntimeCommand{
		ID: "cmd-stale-playback", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1", RuntimeSessionID: "session-1",
		CommandType: string(CommandTypePlayAction), Durability: "ephemeral", Status: string(CommandStatusPlaybackStarted),
		PayloadJSON: `{"maximumPlayMs":1000}`, PlaybackStartedAt: started.Format(time.RFC3339Nano),
		CreatedAt: started.Format("2006-01-02 15:04:05"), UpdatedAt: started.Format("2006-01-02 15:04:05"),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatal(err)
	}
	expired, err := svc.ListExpiredCommands(10, 30)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, got := range expired {
		if got.ID == cmd.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("playback missing its terminal beyond maximumPlayMs + grace must be reclaimed")
	}
}

func TestDispatchingBindsRuntimeSessionBeforeTransportWrite(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	cmd := &RuntimeCommand{
		ID: "cmd-dispatch-session", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1",
		RuntimeSessionID: "old-session", CommandType: string(CommandTypePlayAction), Durability: "ephemeral",
		Status: string(CommandStatusQueued), PayloadJSON: `{}`, CreatedAt: now.Format(time.RFC3339), UpdatedAt: now.Format(time.RFC3339),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatalf("create command: %v", err)
	}
	if err := svc.MarkDispatching(cmd.ID, "runtime-1", "old-session", now); err != nil {
		t.Fatalf("mark dispatching: %v", err)
	}
	got, err := svc.GetCommand(cmd.ID)
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if got.RuntimeSessionID != "old-session" || got.Status != string(CommandStatusDispatching) {
		t.Fatalf("dispatching identity mismatch: status=%s session=%s", got.Status, got.RuntimeSessionID)
	}
	if err := svc.SupersedeEphemeralCommands("u", "d", "runtime-1", "old-session", "runtime connection replaced", now.Add(time.Second)); err != nil {
		t.Fatalf("supersede: %v", err)
	}
	got, err = svc.GetCommand(cmd.ID)
	if err != nil {
		t.Fatalf("get superseded command: %v", err)
	}
	if got.Status != string(CommandStatusSuperseded) {
		t.Fatalf("dispatching old-session ephemeral escaped reconnect fence: %s", got.Status)
	}
}
