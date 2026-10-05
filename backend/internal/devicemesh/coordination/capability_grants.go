package coordination

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

const CapabilityGrantsSchema = `CREATE TABLE IF NOT EXISTS kernel_device_capability_grants (
	space_id TEXT NOT NULL, caller_id TEXT NOT NULL, target_id TEXT NOT NULL, capability TEXT NOT NULL,
	allowed INTEGER NOT NULL DEFAULT 0, revision INTEGER NOT NULL DEFAULT 1,
	target_epoch INTEGER NOT NULL, caller_epoch INTEGER NOT NULL,
	PRIMARY KEY(space_id,caller_id,target_id,capability))`

var ErrCapabilityGrant = errors.New("目标设备未授权此调用能力")

type CapabilityGrant struct {
	CallerID    string `json:"callerId"`
	TargetID    string `json:"targetId"`
	Capability  string `json:"capability"`
	Allowed     bool   `json:"allowed"`
	Revision    int64  `json:"revision"`
	TargetEpoch int64  `json:"targetEpoch"`
	CallerEpoch int64  `json:"callerEpoch"`
}

func validCapability(name string) bool {
	return name != "" && len(name) <= 128 && !strings.ContainsAny(name, "* \t\r\n") && !strings.HasPrefix(name, "coordination.")
}

func (s *Service) SetCapabilityGrant(ctx context.Context, space, caller, target, capability string, expected int64, allowed bool) (CapabilityGrant, error) {
	if caller == "" || target == "" || caller == target || !validCapability(capability) || expected < 0 {
		return CapabilityGrant{}, ErrCapabilityGrant
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	callerPolicy, err := s.Get(ctx, space, caller)
	if err != nil {
		return CapabilityGrant{}, err
	}
	targetPolicy, err := s.Get(ctx, space, target)
	if err != nil {
		return CapabilityGrant{}, err
	}
	var beforeRevision int64
	err = s.db.QueryRowContext(ctx, `SELECT revision FROM kernel_device_capability_grants WHERE space_id=? AND caller_id=? AND target_id=? AND capability=?`, space, caller, target, capability).Scan(&beforeRevision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return CapabilityGrant{}, err
	}
	if beforeRevision != expected {
		return CapabilityGrant{}, ErrRevision
	}
	for _, device := range []string{caller, target} {
		var trust string
		if err := s.db.QueryRowContext(ctx, `SELECT trust_state FROM kernel_devices WHERE space_id=? AND device_id=?`, space, device).Scan(&trust); err != nil || trust != "trusted" {
			return CapabilityGrant{}, ErrCapabilityGrant
		}
	}
	if err := s.fenceAuthorityLocked(ctx, space, target, targetPolicy.PermissionRevision); err != nil {
		return CapabilityGrant{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CapabilityGrant{}, err
	}
	defer tx.Rollback()
	for _, device := range []string{caller, target} {
		var trust string
		if err := tx.QueryRowContext(ctx, `SELECT trust_state FROM kernel_devices WHERE space_id=? AND device_id=?`, space, device).Scan(&trust); err != nil || trust != "trusted" {
			return CapabilityGrant{}, ErrCapabilityGrant
		}
	}
	var revision int64
	err = tx.QueryRowContext(ctx, `SELECT revision FROM kernel_device_capability_grants WHERE space_id=? AND caller_id=? AND target_id=? AND capability=?`, space, caller, target, capability).Scan(&revision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return CapabilityGrant{}, err
	}
	if revision != expected {
		return CapabilityGrant{}, ErrRevision
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_capability_grants(space_id,caller_id,target_id,capability,allowed,revision,target_epoch,caller_epoch) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(space_id,caller_id,target_id,capability) DO UPDATE SET allowed=excluded.allowed,revision=excluded.revision,target_epoch=excluded.target_epoch,caller_epoch=excluded.caller_epoch`, space, caller, target, capability, allowed, revision+1, targetPolicy.ProviderEpoch, callerPolicy.ProviderEpoch); err != nil {
		return CapabilityGrant{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_coordination(space_id,device_id) VALUES(?,?) ON CONFLICT DO NOTHING`, space, target); err != nil {
		return CapabilityGrant{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_coordination SET permission_revision=permission_revision+1 WHERE space_id=? AND device_id=?`, space, target); err != nil {
		return CapabilityGrant{}, err
	}
	if err := tx.Commit(); err != nil {
		return CapabilityGrant{}, err
	}
	s.cancelLocked(space, target)
	return CapabilityGrant{CallerID: caller, TargetID: target, Capability: capability, Allowed: allowed, Revision: revision + 1, TargetEpoch: targetPolicy.ProviderEpoch, CallerEpoch: callerPolicy.ProviderEpoch}, nil
}

func (s *Service) RequireCapability(ctx context.Context, space, caller, target, capability string) error {
	if !validCapability(capability) {
		return ErrCapabilityGrant
	}
	if caller == target {
		return nil
	}
	var allowed bool
	var callerEpoch, targetEpoch int64
	err := s.db.QueryRowContext(ctx, `SELECT allowed,caller_epoch,target_epoch FROM kernel_device_capability_grants WHERE space_id=? AND caller_id=? AND target_id=? AND capability=?`, space, caller, target, capability).Scan(&allowed, &callerEpoch, &targetEpoch)
	if errors.Is(err, sql.ErrNoRows) || !allowed {
		return ErrCapabilityGrant
	}
	if err != nil {
		return err
	}
	callerPolicy, err := s.Get(ctx, space, caller)
	if err != nil {
		return err
	}
	targetPolicy, err := s.Get(ctx, space, target)
	if err != nil {
		return err
	}
	if callerEpoch != callerPolicy.ProviderEpoch || targetEpoch != targetPolicy.ProviderEpoch {
		return ErrCapabilityGrant
	}
	return nil
}

func (s *Service) CapabilityGrants(ctx context.Context, space, target string) ([]CapabilityGrant, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT caller_id,target_id,capability,allowed,revision,target_epoch,caller_epoch FROM kernel_device_capability_grants WHERE space_id=? AND target_id=? ORDER BY caller_id,capability LIMIT 512`, space, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := make([]CapabilityGrant, 0)
	for rows.Next() {
		var grant CapabilityGrant
		if err := rows.Scan(&grant.CallerID, &grant.TargetID, &grant.Capability, &grant.Allowed, &grant.Revision, &grant.TargetEpoch, &grant.CallerEpoch); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}
