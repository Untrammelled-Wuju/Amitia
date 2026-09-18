package host_registry

import (
	"context"
	"database/sql"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type DeviceTrustState string

const (
	DeviceTrustPending DeviceTrustState = "pending"
	DeviceTrustTrusted DeviceTrustState = "trusted"
	DeviceTrustRevoked DeviceTrustState = "revoked"
)

type DeviceRecord struct {
	SpaceID    runtimeidentity.SpaceID
	DeviceID   runtimeidentity.DeviceID
	Platform   runtimeidentity.Platform
	Label      string
	TrustState DeviceTrustState
	CreatedAt  time.Time
	TrustedAt  *time.Time
	LastSeenAt time.Time
	Revision   int64
}

func (r *Registry) EnsureDevice(ctx context.Context, record DeviceRecord) (*DeviceRecord, error) {
	existing, err := r.GetDevice(ctx, record.DeviceID)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		if existing.SpaceID != record.SpaceID {
			return nil, ErrDeviceOwnedByOther
		}
		changed := false
		if record.Label != "" && existing.Label != record.Label {
			existing.Label = record.Label
			changed = true
		}
		if existing.Platform == "" && record.Platform != "" {
			existing.Platform = record.Platform
			changed = true
		}
		now := time.Now().UTC()
		existing.LastSeenAt = now
		changed = true
		if changed {
			existing.Revision++
			if err := r.repo.SaveDevice(ctx, existing); err != nil {
				return nil, err
			}
		}
		return existing, nil
	}

	now := time.Now().UTC()
	record.CreatedAt = now
	record.LastSeenAt = now
	record.Revision = 1
	if record.TrustState == "" {
		record.TrustState = DeviceTrustPending
	}
	if err := r.repo.SaveDevice(ctx, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *Registry) GetDevice(ctx context.Context, deviceID runtimeidentity.DeviceID) (*DeviceRecord, error) {
	return r.repo.GetDevice(ctx, deviceID)
}

func (r *Registry) ListDevicesBySpace(ctx context.Context, spaceID runtimeidentity.SpaceID) ([]*DeviceRecord, error) {
	return r.repo.ListDevicesBySpace(ctx, spaceID)
}

func (r *Registry) RequireDeviceOwnedBy(ctx context.Context, spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID) error {
	dev, err := r.GetDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	if dev == nil {
		return ErrDeviceNotFound
	}
	if dev.SpaceID != spaceID {
		return ErrDeviceOwnedByOther
	}
	return nil
}

func (r *Registry) RequireTrustedDevice(ctx context.Context, spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID) error {
	dev, err := r.GetDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	if dev == nil {
		return ErrDeviceNotFound
	}
	if dev.SpaceID != spaceID {
		return ErrDeviceOwnedByOther
	}
	if dev.TrustState != DeviceTrustTrusted {
		return ErrDeviceNotTrusted
	}
	return nil
}

func (r *Registry) MarkDeviceTrusted(ctx context.Context, deviceID runtimeidentity.DeviceID) error {
	now := time.Now().UTC()
	return r.repo.UpdateDeviceTrust(ctx, deviceID, DeviceTrustTrusted, &now)
}

func (r *Registry) MarkDeviceTrustedTx(ctx context.Context, tx *sql.Tx, deviceID runtimeidentity.DeviceID) error {
	now := time.Now().UTC()
	return r.repo.UpdateDeviceTrustTx(ctx, tx, deviceID, DeviceTrustTrusted, &now)
}

func (r *Registry) MarkDeviceSeen(ctx context.Context, deviceID runtimeidentity.DeviceID) error {
	now := time.Now().UTC()
	return r.repo.UpdateDeviceLastSeen(ctx, deviceID, now)
}

func (r *Registry) RevokeDevice(ctx context.Context, deviceID runtimeidentity.DeviceID) error {
	now := time.Now().UTC()
	return r.repo.UpdateDeviceTrust(ctx, deviceID, DeviceTrustRevoked, &now)
}
