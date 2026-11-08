package coordination

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/capabilities"
)

const CapabilityExposureSchema = `CREATE TABLE IF NOT EXISTS kernel_device_capability_exposure (
	space_id TEXT NOT NULL, device_id TEXT NOT NULL, capability TEXT NOT NULL,
	PRIMARY KEY(space_id,device_id,capability))`

const CapabilityExposureStateSchema = `CREATE TABLE IF NOT EXISTS kernel_device_capability_exposure_state (
	space_id TEXT NOT NULL, device_id TEXT NOT NULL, revision INTEGER NOT NULL,
	updated_at TEXT NOT NULL, PRIMARY KEY(space_id,device_id))`

type CapabilityCatalog struct {
	DeviceID     string            `json:"deviceId"`
	Revision     int64             `json:"revision"`
	Capabilities []string          `json:"capabilities"`
	Grants       []CapabilityGrant `json:"grants"`
}

type CapabilityCatalogEvent struct {
	DeviceID     string   `json:"deviceId"`
	Revision     int64    `json:"revision"`
	Capabilities []string `json:"capabilities"`
}

type capabilityEventHub struct {
	mu        sync.Mutex
	observers map[chan CapabilityCatalogEvent]struct{}
}

func (h *capabilityEventHub) subscribe() (<-chan CapabilityCatalogEvent, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.observers == nil {
		h.observers = make(map[chan CapabilityCatalogEvent]struct{})
	}
	events := make(chan CapabilityCatalogEvent, 8)
	h.observers[events] = struct{}{}
	return events, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.observers[events]; ok {
			delete(h.observers, events)
			close(events)
		}
	}
}

func (h *capabilityEventHub) publish(event CapabilityCatalogEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for observer := range h.observers {
		select {
		case observer <- event:
		default:
		}
	}
}

func (s *Service) WatchCapabilityCatalogs() (<-chan CapabilityCatalogEvent, func()) {
	return s.capabilityEvents.subscribe()
}

func (s *Service) SetCapabilityExposure(ctx context.Context, space, device string, revision int64, exposed []string) (CapabilityCatalog, error) {
	if space == "" || device == "" || revision < 1 {
		return CapabilityCatalog{}, ErrCapabilityGrant
	}
	normalized, err := capabilities.Normalize(exposed)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validateRequestAuthorities(ctx); err != nil {
		return CapabilityCatalog{}, err
	}
	policy, err := s.Get(ctx, space, device)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	var currentRevision int64
	err = s.db.QueryRowContext(ctx, `SELECT revision FROM kernel_device_capability_exposure_state WHERE space_id=? AND device_id=?`, space, device).Scan(&currentRevision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return CapabilityCatalog{}, err
	}
	if revision < currentRevision {
		return CapabilityCatalog{}, ErrRevision
	}
	if revision == currentRevision {
		catalog, err := s.CapabilityCatalog(ctx, space, device)
		if err != nil {
			return CapabilityCatalog{}, err
		}
		if slicesEqual(catalog.Capabilities, normalized) {
			return catalog, nil
		}
		return CapabilityCatalog{}, ErrRevision
	}
	stale, err := s.staleCapabilityGrants(ctx, space, device, normalized)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	if len(stale) > 0 {
		if err := s.fenceAuthorityLocked(ctx, space, device, policy.PermissionRevision, false); err != nil {
			return CapabilityCatalog{}, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	defer tx.Rollback()
	if err := ValidateRequestAuthorityTx(ctx, tx); err != nil {
		return CapabilityCatalog{}, err
	}
	var trust string
	if err := tx.QueryRowContext(ctx, `SELECT trust_state FROM kernel_devices WHERE space_id=? AND device_id=?`, space, device).Scan(&trust); err != nil {
		return CapabilityCatalog{}, err
	}
	if trust != "trusted" {
		return CapabilityCatalog{}, ErrCapabilityGrant
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM kernel_device_capability_exposure WHERE space_id=? AND device_id=?`, space, device); err != nil {
		return CapabilityCatalog{}, err
	}
	for _, capability := range normalized {
		if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_capability_exposure(space_id,device_id,capability) VALUES(?,?,?)`, space, device, capability); err != nil {
			return CapabilityCatalog{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_capability_exposure_state(space_id,device_id,revision,updated_at) VALUES(?,?,?,?) ON CONFLICT(space_id,device_id) DO UPDATE SET revision=excluded.revision,updated_at=excluded.updated_at`, space, device, revision, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return CapabilityCatalog{}, err
	}
	for _, capability := range stale {
		if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_capability_grants SET allowed=0,revision=revision+1 WHERE space_id=? AND target_id=? AND capability=?`, space, device, capability); err != nil {
			return CapabilityCatalog{}, err
		}
	}
	if len(stale) > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_coordination(space_id,device_id) VALUES(?,?) ON CONFLICT DO NOTHING`, space, device); err != nil {
			return CapabilityCatalog{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_coordination SET permission_revision=permission_revision+1 WHERE space_id=? AND device_id=?`, space, device); err != nil {
			return CapabilityCatalog{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return CapabilityCatalog{}, err
	}
	if len(stale) > 0 {
		s.cancelLocked(space, device)
	}
	catalog, err := s.CapabilityCatalog(ctx, space, device)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	s.capabilityEvents.publish(CapabilityCatalogEvent{DeviceID: device, Revision: catalog.Revision, Capabilities: catalog.Capabilities})
	return catalog, nil
}

func (s *Service) staleCapabilityGrants(ctx context.Context, space, device string, exposed []string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT capability FROM kernel_device_capability_grants WHERE space_id=? AND target_id=? AND allowed=1`, space, device)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	enabled := make(map[string]struct{}, len(exposed))
	for _, capability := range exposed {
		enabled[capability] = struct{}{}
	}
	stale := make([]string, 0)
	for rows.Next() {
		var capability string
		if err := rows.Scan(&capability); err != nil {
			return nil, err
		}
		if _, ok := enabled[capability]; !ok {
			stale = append(stale, capability)
		}
	}
	return stale, rows.Err()
}

func (s *Service) CapabilityEnabled(ctx context.Context, space, device, capability string) (bool, error) {
	if !capabilities.Valid(capability) {
		return false, nil
	}
	executor := currentQueryExecutor(ctx, s.db)
	var revision int64
	err := executor.QueryRowContext(ctx, `SELECT revision FROM kernel_device_capability_exposure_state WHERE space_id=? AND device_id=?`, space, device).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	var count int
	err = executor.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_capability_exposure WHERE space_id=? AND device_id=? AND capability=?`, space, device, capability).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 1, nil
}

func (s *Service) RequireCapabilityExposure(ctx context.Context, space, target, capability string) error {
	enabled, err := s.CapabilityEnabled(ctx, space, target, capability)
	if err != nil {
		return err
	}
	if !enabled {
		return fmt.Errorf("%w: %s 已由目标设备关闭", ErrCapabilityGrant, capability)
	}
	return nil
}

func (s *Service) CapabilityCatalog(ctx context.Context, space, device string) (CapabilityCatalog, error) {
	catalog, err := capabilityCatalogTx(ctx, currentQueryExecutor(ctx, s.db), space, device)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	grants, err := s.CapabilityGrants(ctx, space, device)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	catalog.Grants = grants
	return catalog, nil
}

type capabilityQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func capabilityCatalogTx(ctx context.Context, executor capabilityQueryer, space, device string) (CapabilityCatalog, error) {
	catalog := CapabilityCatalog{DeviceID: device, Capabilities: make([]string, 0), Grants: make([]CapabilityGrant, 0)}
	err := executor.QueryRowContext(ctx, `SELECT revision FROM kernel_device_capability_exposure_state WHERE space_id=? AND device_id=?`, space, device).Scan(&catalog.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		catalog.Capabilities = capabilities.IDs()
		return catalog, nil
	}
	if err != nil {
		return CapabilityCatalog{}, err
	}
	rows, err := executor.QueryContext(ctx, `SELECT capability FROM kernel_device_capability_exposure WHERE space_id=? AND device_id=? ORDER BY capability`, space, device)
	if err != nil {
		return CapabilityCatalog{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var capability string
		if err := rows.Scan(&capability); err != nil {
			return CapabilityCatalog{}, err
		}
		catalog.Capabilities = append(catalog.Capabilities, capability)
	}
	if err := rows.Err(); err != nil {
		return CapabilityCatalog{}, err
	}
	return catalog, nil
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
