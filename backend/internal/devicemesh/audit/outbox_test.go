package audit_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh"
	meshaudit "github.com/u-ai/backend/internal/devicemesh/audit"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/pairing"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/securityaudit"
)

func databases(t *testing.T) (*sql.DB, *sql.DB) {
	t.Helper()
	kernel, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	kernel.SetMaxOpenConns(1)
	main, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "main.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernel.Close(); main.Close() })
	if err := kernelsqlite.Migrate(t.Context(), kernel); err != nil {
		t.Fatal(err)
	}
	baseline, err := os.ReadFile(filepath.Join("..", "..", "migration", "baseline.sql"))
	if err != nil {
		t.Fatal(err)
	}
	ddl := regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS security_audit_events\s*\(.*?\);`).Find(baseline)
	if len(ddl) == 0 {
		t.Fatal("canonical audit schema missing")
	}
	if _, err := main.Exec(string(ddl)); err != nil {
		t.Fatal(err)
	}
	return kernel, main
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAuditOutboxTransactionRollbackRetryRestartIdempotencyAndSpaceIsolation(t *testing.T) {
	kernel, main := databases(t)
	queue := func(space string, commit bool) {
		tx, err := kernel.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := meshaudit.QueueTx(t.Context(), tx, space, "device", "device_mesh.fixture", meshaudit.Details{CoreID: "core"}); err != nil {
			t.Fatal(err)
		}
		if commit {
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		} else {
			tx.Rollback()
		}
	}
	queue("a", false)
	if count(t, kernel, "kernel_security_audit_outbox") != 0 {
		t.Fatal("rollback leaked audit")
	}
	queue("a", true)
	queue("b", true)
	if _, err := main.Exec(`CREATE TRIGGER reject_audit BEFORE INSERT ON security_audit_events BEGIN SELECT RAISE(ABORT,'offline'); END`); err != nil {
		t.Fatal(err)
	}
	bridge := meshaudit.Bridge{Kernel: kernel, Canonical: main, SpaceID: "a"}
	if err := bridge.Flush(t.Context()); err == nil {
		t.Fatal("canonical failure swallowed")
	}
	var attempts int
	if err := kernel.QueryRow(`SELECT attempts FROM kernel_security_audit_outbox WHERE space_id='a'`).Scan(&attempts); err != nil || attempts != 1 {
		t.Fatal("failure did not remain retryable")
	}
	if _, err := main.Exec(`DROP TRIGGER reject_audit`); err != nil {
		t.Fatal(err)
	}
	var kernelPath string
	if err := kernel.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&kernelPath); err != nil {
		t.Fatal(err)
	}
	if err := kernel.Close(); err != nil {
		t.Fatal(err)
	}
	kernel, err := sql.Open("sqlite", kernelPath)
	if err != nil {
		t.Fatal(err)
	}
	kernel.SetMaxOpenConns(1)
	t.Cleanup(func() { kernel.Close() })
	var originalID, originalPayload, occurred string
	if err := kernel.QueryRow(`SELECT event_id,payload_json,occurred_at FROM kernel_security_audit_outbox WHERE space_id='a'`).Scan(&originalID, &originalPayload, &occurred); err != nil {
		t.Fatal(err)
	}
	restarted := meshaudit.Bridge{Kernel: kernel, Canonical: main, SpaceID: "a"}
	if err := restarted.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if count(t, main, "security_audit_events") != 1 || count(t, kernel, "kernel_security_audit_outbox") != 1 {
		t.Fatal("canonical delivery crossed spaces or retained acknowledged duplicate")
	}
	if _, err := kernel.Exec(`INSERT INTO kernel_security_audit_outbox(event_id,space_id,payload_json,occurred_at) VALUES(?,'a',?,?)`, originalID, originalPayload, occurred); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if count(t, main, "security_audit_events") != 1 {
		t.Fatal("replay duplicated canonical audit")
	}
	restarted.SpaceID = "b"
	if err := restarted.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if count(t, kernel, "kernel_security_audit_outbox") != 0 || count(t, main, "security_audit_events") != 2 {
		t.Fatal("remaining space was not delivered")
	}
}

func TestPairingExchangeAuditRollsBackCredentialAndTrustWhenQueueFails(t *testing.T) {
	kernel, main := databases(t)
	registry := host_registry.NewRegistry(kernel)
	if _, err := registry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "core", DeviceID: "issuer", Platform: runtimeidentity.PlatformWindows, TrustState: host_registry.DeviceTrustTrusted}); err != nil {
		t.Fatal(err)
	}
	runtime, err := devicemesh.NewCloudRuntime(kernel, registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { runtime.Stop() })
	service, err := pairing.NewService(kernel, t.TempDir(), "core", registry, runtime.BootstrapSvc)
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := service.CreateOffer(t.Context(), "issuer", 0)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := service.Claim(t.Context(), pairing.ClaimRequest{OfferToken: token, DeviceID: "phone", RuntimeID: "runtime", Platform: runtimeidentity.PlatformAndroid})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Exec(`CREATE TRIGGER reject_pair_audit BEFORE INSERT ON kernel_security_audit_outbox WHEN NEW.payload_json LIKE '%device_mesh.device_paired%' BEGIN SELECT RAISE(ABORT,'audit unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := runtime.BootstrapSvc.Exchange(t.Context(), claim.RawTicket, "phone", "runtime"); err == nil {
		t.Fatal("bootstrap exchanged without durable audit")
	}
	device, err := registry.GetDevice(t.Context(), "phone")
	if err != nil || device.TrustState != host_registry.DeviceTrustPending {
		t.Fatal("failed audit made device trusted")
	}
	if count(t, kernel, "kernel_device_runtime_credentials") != 0 {
		t.Fatal("failed audit leaked credential")
	}
	if _, err := kernel.Exec(`DROP TRIGGER reject_pair_audit`); err != nil {
		t.Fatal(err)
	}
	_, raw, _, err := runtime.BootstrapSvc.Exchange(t.Context(), claim.RawTicket, "phone", "runtime")
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.StartSecurityAudit(main, "core"); err != nil {
		t.Fatal(err)
	}
	var pairs int
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := main.QueryRow(`SELECT count(*) FROM security_audit_events WHERE event_type='device_mesh.device_paired'`).Scan(&pairs); err != nil {
			t.Fatal(err)
		}
		if pairs == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pairs != 1 {
		t.Fatal("actual trust exchange was not audited once by production worker")
	}
	rows, err := main.Query(`SELECT details_json FROM security_audit_events`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var details string
		if err := rows.Scan(&details); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(details, token) || strings.Contains(details, claim.RawTicket) || strings.Contains(details, raw) {
			t.Fatal("pairing secret leaked into audit")
		}
	}
}

func TestAuthorityChangesQueueRealActorAndAuditFailureRollsBack(t *testing.T) {
	kernel, main := databases(t)
	service := coordination.NewService(kernel)
	ctx := auth.WithActor(t.Context(), &auth.ActorContext{SpaceID: runtimeidentity.SpaceID("core"), DeviceID: runtimeidentity.DeviceID("administrator"), PrincipalType: auth.PrincipalLocalUI, AuthMethod: "desktop_session", IsLocalTrusted: true})
	policy, err := service.ChangeMode(ctx, "core", "phone", 1, true, "role")
	if err != nil {
		t.Fatal(err)
	}
	policy, err = service.GrantAdministrator(ctx, "core", "phone", policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RevokeDevice(ctx, "core", "phone", func(*sql.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	providerCtx := meshaudit.WithActor(ctx, meshaudit.Actor{SpaceID: "core", DeviceID: "administrator", PrincipalType: "device_runtime", AuthMethod: "local_provider_authority", Realm: "local"})
	if _, _, err := service.BindProvider(providerCtx, "next-core"); err != nil {
		t.Fatal(err)
	}
	bridge := meshaudit.Bridge{Kernel: kernel, Canonical: main, SpaceID: "core"}
	if err := bridge.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if count(t, main, "security_audit_events") != 4 {
		t.Fatal("authority mutations lacked production audit")
	}
	var event securityaudit.AuditEvent
	if err := main.QueryRow(`SELECT details_json,principal_type,auth_method,device_id FROM security_audit_events WHERE event_type='device_mesh.administrator_changed'`).Scan(&event.DetailsJSON, &event.PrincipalType, &event.AuthMethod, &event.DeviceID); err != nil {
		t.Fatal(err)
	}
	var details meshaudit.Details
	if json.Unmarshal([]byte(event.DetailsJSON), &details) != nil || details.ActorDeviceID != "administrator" || details.TargetDeviceID != "phone" || details.Source != "device-mesh" || details.Realm != "local" || event.PrincipalType != "local_ui" || event.AuthMethod != "desktop_session" {
		t.Fatal("actor or realm was replaced with target device")
	}
	if strings.Contains(event.DetailsJSON, "role") || strings.Contains(event.DetailsJSON, "credential") {
		t.Fatal("audit recorded sensitive configuration")
	}
	before, err := service.Get(ctx, "core", "phone")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Exec(`CREATE TRIGGER reject_queue BEFORE INSERT ON kernel_security_audit_outbox BEGIN SELECT RAISE(ABORT,'audit unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeMode(ctx, "core", "phone", before.ModeRevision, false, ""); err == nil {
		t.Fatal("audit failure was ignored")
	}
	after, err := service.Get(ctx, "core", "phone")
	if err != nil || after != before {
		t.Fatal("mode committed without durable audit")
	}
	if _, err := service.GrantAdministrator(ctx, "core", "phone", before.PermissionRevision, true); err == nil {
		t.Fatal("administrator grant committed without audit")
	}
	after, err = service.Get(ctx, "core", "phone")
	if err != nil || after != before {
		t.Fatal("administrator transaction failed to roll back")
	}
	if _, _, err := service.BindProvider(providerCtx, "another-core"); err == nil {
		t.Fatal("provider switch committed without audit")
	}
	provider, _, err := service.Provider(context.Background())
	if err != nil || provider != "next-core" {
		t.Fatal("failed audit changed provider")
	}
	if _, err := kernel.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('phone','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	if err := service.RevokeDevice(ctx, "core", "phone", func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE kernel_devices SET trust_state='revoked' WHERE device_id='phone'`)
		return err
	}); err == nil {
		t.Fatal("revocation committed without audit")
	}
	var trust string
	if err := kernel.QueryRow(`SELECT trust_state FROM kernel_devices WHERE device_id='phone'`).Scan(&trust); err != nil || trust != "trusted" {
		t.Fatal("failed audit left device revoked")
	}
}

func TestAuditBridgeDoesNotAcknowledgeConflictingCanonicalEventOrForeignActor(t *testing.T) {
	kernel, main := databases(t)
	tx, err := kernel.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := meshaudit.WithActor(t.Context(), meshaudit.Actor{SpaceID: "foreign", DeviceID: "administrator", PrincipalType: "local_ui", AuthMethod: "desktop_session", Realm: "local"})
	if err := meshaudit.QueueTx(ctx, tx, "core", "phone", "device_mesh.mode_changed", meshaudit.Details{}); err == nil {
		t.Fatal("foreign actor wrote target Core audit")
	}
	tx.Rollback()
	tx, err = kernel.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := meshaudit.QueueTx(t.Context(), tx, "core", "phone", "device_mesh.mode_changed", meshaudit.Details{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := kernel.QueryRow(`SELECT event_id FROM kernel_security_audit_outbox`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := main.Exec(`INSERT INTO security_audit_events(event_id,event_type,severity,outcome,space_id,device_id,runtime_id,session_id,principal_type,auth_method,ip_address,user_agent,reason_code,details_json,occurred_at) VALUES(?,'device_mesh.mode_changed','info','success','foreign','phone','','','local_ui','desktop_session','','','','{}','now')`, id); err != nil {
		t.Fatal(err)
	}
	bridge := meshaudit.Bridge{Kernel: kernel, Canonical: main, SpaceID: "core"}
	if err := bridge.Flush(t.Context()); err == nil {
		t.Fatal("conflicting canonical event acknowledged")
	}
	if count(t, kernel, "kernel_security_audit_outbox") != 1 {
		t.Fatal("conflicting audit lost retryable queue")
	}
}
