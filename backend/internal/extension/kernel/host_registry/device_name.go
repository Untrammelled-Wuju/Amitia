package host_registry

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

const MaxDeviceNameRunes = 64

var (
	ErrDeviceNameEmpty      = errors.New("host_registry: device name is empty")
	ErrDeviceNameNotPending = errors.New("host_registry: device name change is not pending")
)

func NormalizeDeviceName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(trimmed))
	lastSpace := false
	count := 0
	for _, r := range trimmed {
		if unicode.IsControl(r) || r == '\u200b' || r == '\ufeff' {
			continue
		}
		if unicode.IsSpace(r) {
			if lastSpace || count == 0 {
				continue
			}
			builder.WriteRune(' ')
			lastSpace = true
			count++
			if count >= MaxDeviceNameRunes {
				break
			}
			continue
		}
		builder.WriteRune(r)
		lastSpace = false
		count++
		if count >= MaxDeviceNameRunes {
			break
		}
	}
	return strings.TrimSpace(builder.String())
}

func ApplyReportedName(record *DeviceRecord, raw string) bool {
	if record == nil {
		return false
	}
	name := NormalizeDeviceName(raw)
	if name == "" {
		return false
	}
	changed := false
	if record.ReportedName != name {
		record.ReportedName = name
		changed = true
	}
	if record.Label == "" {
		record.Label = name
		record.NameRevision = 1
		if record.PendingName != "" {
			record.PendingName = ""
		}
		return true
	}
	if record.Label == name {
		if record.PendingName != "" {
			record.PendingName = ""
			return true
		}
		return changed
	}
	if record.PendingName != name {
		record.PendingName = name
		return true
	}
	return changed
}

func (r *Registry) ReportDeviceName(ctx context.Context, deviceID runtimeidentity.DeviceID, raw string) (*DeviceRecord, error) {
	name := NormalizeDeviceName(raw)
	if name == "" {
		return nil, ErrDeviceNameEmpty
	}
	record, err := r.GetDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrDeviceNotFound
	}
	if record.Label == name && record.ReportedName == name && record.PendingName == "" {
		return record, nil
	}
	if !ApplyReportedName(record, name) {
		return record, nil
	}
	record.Revision++
	if err := r.repo.SaveDevice(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (r *Registry) ConfirmDeviceName(ctx context.Context, spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID) (*DeviceRecord, error) {
	record, err := r.GetDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrDeviceNotFound
	}
	if record.SpaceID != spaceID {
		return nil, ErrDeviceOwnedByOther
	}
	if record.PendingName == "" {
		return nil, ErrDeviceNameNotPending
	}
	record.Label = record.PendingName
	record.PendingName = ""
	record.NameRevision++
	record.Revision++
	if err := r.repo.SaveDevice(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}
