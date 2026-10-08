package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/securityaudit"
)

const Schema = `CREATE TABLE IF NOT EXISTS kernel_security_audit_outbox (
	event_id TEXT PRIMARY KEY, space_id TEXT NOT NULL, payload_json TEXT NOT NULL,
	occurred_at TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, last_error TEXT NOT NULL DEFAULT '')`

type Actor struct{ SpaceID, DeviceID, PrincipalType, AuthMethod, Realm string }
type actorKey struct{}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

type Details struct {
	Source             string `json:"source"`
	Realm              string `json:"realm"`
	ActorDeviceID      string `json:"actorDeviceId,omitempty"`
	TargetDeviceID     string `json:"targetDeviceId,omitempty"`
	PreviousCoreID     string `json:"previousCoreId,omitempty"`
	CoreID             string `json:"coreId,omitempty"`
	ProviderEpoch      int64  `json:"providerEpoch,omitempty"`
	ModeRevision       int64  `json:"modeRevision,omitempty"`
	PermissionRevision int64  `json:"permissionRevision,omitempty"`
	Coordinated        *bool  `json:"coordinated,omitempty"`
	Administrator      *bool  `json:"administrator,omitempty"`
	ApprovalID         string `json:"approvalId,omitempty"`
}

func QueueTx(ctx context.Context, tx *sql.Tx, space, device, eventType string, details Details) error {
	if tx == nil || space == "" || !strings.HasPrefix(eventType, "device_mesh.") || len(eventType) > 100 {
		return errors.New("安全审计缺少事务、空间归属或合法事件类型")
	}
	actor, ok := ctx.Value(actorKey{}).(Actor)
	if !ok {
		if authenticated, valid := auth.FromContext(ctx); valid && authenticated != nil {
			realm := "mesh"
			if authenticated.IsLocalTrusted {
				realm = "local"
			}
			actor = Actor{SpaceID: authenticated.SpaceID.String(), DeviceID: authenticated.DeviceID.String(), PrincipalType: string(authenticated.PrincipalType), AuthMethod: authenticated.AuthMethod, Realm: realm}
		} else {
			actor = Actor{SpaceID: space, DeviceID: device, PrincipalType: "system_worker", AuthMethod: "internal", Realm: "system"}
		}
	}
	if actor.SpaceID != space {
		return errors.New("安全审计操作空间不匹配")
	}
	if actor.Realm != "mesh" && actor.Realm != "local" && actor.Realm != "system" {
		return errors.New("安全审计认证域无效")
	}
	details.Source, details.Realm, details.ActorDeviceID, details.TargetDeviceID = "device-mesh", actor.Realm, actor.DeviceID, device
	encoded, err := json.Marshal(details)
	if err != nil {
		return err
	}
	event := securityaudit.AuditEvent{EventID: "ae_" + uuid.NewString(), EventType: eventType, Severity: "info", Outcome: "success", SpaceID: space, DeviceID: device, PrincipalType: actor.PrincipalType, AuthMethod: actor.AuthMethod, DetailsJSON: string(encoded), OccurredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO kernel_security_audit_outbox(event_id,space_id,payload_json,occurred_at) VALUES(?,?,?,?)`, event.EventID, space, string(payload), event.OccurredAt)
	return err
}

type Bridge struct {
	Kernel, Canonical *sql.DB
	SpaceID           string
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorKey{}).(Actor)
	return actor, ok
}
func (b *Bridge) Flush(ctx context.Context) error {
	if b.Kernel == nil || b.Canonical == nil || b.SpaceID == "" {
		return errors.New("安全审计投递依赖不完整")
	}
	rows, err := b.Kernel.QueryContext(ctx, `SELECT event_id,payload_json FROM kernel_security_audit_outbox WHERE space_id=? ORDER BY occurred_at,event_id LIMIT 128`, b.SpaceID)
	if err != nil {
		return err
	}
	type pending struct{ id, payload string }
	var values []pending
	for rows.Next() {
		var value pending
		if err := rows.Scan(&value.id, &value.payload); err != nil {
			rows.Close()
			return err
		}
		values = append(values, value)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, value := range values {
		var event securityaudit.AuditEvent
		if json.Unmarshal([]byte(value.payload), &event) != nil || event.EventID != value.id || event.SpaceID != b.SpaceID || event.EventType == "" || event.Outcome != "success" {
			return errors.New("安全审计队列归属或内容无效")
		}
		_, err := b.Canonical.ExecContext(ctx, `INSERT INTO security_audit_events(event_id,event_type,severity,outcome,space_id,device_id,runtime_id,session_id,principal_type,auth_method,ip_address,user_agent,reason_code,details_json,occurred_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(event_id) DO NOTHING`, event.EventID, event.EventType, event.Severity, event.Outcome, event.SpaceID, event.DeviceID, event.RuntimeID, event.SessionID, event.PrincipalType, event.AuthMethod, event.IPAddress, event.UserAgent, event.ReasonCode, event.DetailsJSON, event.OccurredAt)
		if err != nil {
			_, retryErr := b.Kernel.ExecContext(ctx, `UPDATE kernel_security_audit_outbox SET attempts=attempts+1,last_error=? WHERE event_id=? AND space_id=?`, "canonical_write_failed", value.id, b.SpaceID)
			return errors.Join(fmt.Errorf("安全审计投递失败: %w", err), retryErr)
		}
		var existing securityaudit.AuditEvent
		if err := b.Canonical.QueryRowContext(ctx, `SELECT event_id,event_type,severity,outcome,space_id,device_id,runtime_id,session_id,principal_type,auth_method,ip_address,user_agent,reason_code,details_json,occurred_at FROM security_audit_events WHERE event_id=?`, value.id).Scan(&existing.EventID, &existing.EventType, &existing.Severity, &existing.Outcome, &existing.SpaceID, &existing.DeviceID, &existing.RuntimeID, &existing.SessionID, &existing.PrincipalType, &existing.AuthMethod, &existing.IPAddress, &existing.UserAgent, &existing.ReasonCode, &existing.DetailsJSON, &existing.OccurredAt); err != nil {
			return err
		}
		if existing != event {
			return errors.New("安全审计事件编号存在不同归属或内容，队列未确认")
		}
		if _, err := b.Kernel.ExecContext(ctx, `DELETE FROM kernel_security_audit_outbox WHERE event_id=? AND space_id=?`, value.id, b.SpaceID); err != nil {
			return err
		}
	}
	return nil
}
