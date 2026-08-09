package checkpoint

import (
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

func ValidateCheckpoint(metadata RuntimeMetadata, checkpoint RuntimeCheckpoint, descriptor *domain.PluginDescriptor) error {
	if err := ValidateMetadata(metadata, checkpoint); err != nil {
		return err
	}
	if err := CheckpointConsistency(metadata, checkpoint); err != nil {
		return err
	}
	if descriptor != nil {
		if err := DescriptorMatch(checkpoint, *descriptor); err != nil {
			return err
		}
	}
	if err := TimestampConsistency(metadata, checkpoint); err != nil {
		return err
	}
	if err := ServiceConsistency(checkpoint); err != nil {
		return err
	}
	return nil
}

func ValidateMetadata(metadata RuntimeMetadata, checkpoint RuntimeCheckpoint) error {
	if metadata.RuntimeID == "" {
		return errCorruptMetadata("validate_metadata", "", errMissingRuntimeID)
	}
	if metadata.PluginID == "" {
		return errCorruptMetadata("validate_metadata", string(metadata.RuntimeID), errMissingPluginID)
	}
	if checkpoint.RuntimeID == "" {
		return errCorrupt("validate_checkpoint", "", errMissingRuntimeID)
	}
	if checkpoint.PluginID == "" {
		return errCorrupt("validate_checkpoint", string(checkpoint.RuntimeID), errMissingPluginID)
	}
	if !domain.IsValidRuntimeState(checkpoint.RuntimeState) {
		return errInvalidState("validate_checkpoint", string(checkpoint.RuntimeID),
			errInvalidRuntimeState(checkpoint.RuntimeState))
	}
	for _, svc := range checkpoint.Services {
		if !runtime.IsValidServiceRuntimeState(svc.State) {
			return errInvalidState("validate_checkpoint", string(checkpoint.RuntimeID),
				errInvalidServiceState(svc.State))
		}
	}
	return nil
}

func CheckpointConsistency(metadata RuntimeMetadata, checkpoint RuntimeCheckpoint) error {
	if metadata.RuntimeID != checkpoint.RuntimeID {
		return errRuntimeIDMismatch("consistency", string(metadata.RuntimeID),
			errIDMismatch(metadata.RuntimeID, checkpoint.RuntimeID))
	}
	if metadata.PluginID != checkpoint.PluginID {
		return errPluginIDMismatch("consistency", string(metadata.RuntimeID),
			errIDMismatch(metadata.PluginID, checkpoint.PluginID))
	}
	return nil
}

func DescriptorMatch(checkpoint RuntimeCheckpoint, descriptor domain.PluginDescriptor) error {
	if checkpoint.DescriptorRevision == "" {
		return nil
	}
	expectedRevision := ComputeDescriptorRevision(descriptor)
	if checkpoint.DescriptorRevision != expectedRevision {
		return newError("descriptor_match", ErrStaleRevision, string(checkpoint.RuntimeID),
			errRevisionMismatch(checkpoint.DescriptorRevision, expectedRevision))
	}
	return nil
}

func TimestampConsistency(metadata RuntimeMetadata, checkpoint RuntimeCheckpoint) error {
	if checkpoint.UpdatedAt.Before(metadata.CreatedAt) {
		return newError("timestamp", ErrInvalidSchema, string(checkpoint.RuntimeID),
			errTimestampOrder("updatedAt before createdAt"))
	}
	if checkpoint.CreatedAt.After(checkpoint.UpdatedAt) {
		return newError("timestamp", ErrInvalidSchema, string(checkpoint.RuntimeID),
			errTimestampOrder("createdAt after updatedAt"))
	}
	if checkpoint.LastKnownGoodAt != nil {
		if checkpoint.LastKnownGoodAt.Before(metadata.CreatedAt) {
			return newError("timestamp", ErrInvalidSchema, string(checkpoint.RuntimeID),
				errTimestampOrder("lastKnownGoodAt before createdAt"))
		}
		if checkpoint.LastKnownGoodAt.After(checkpoint.UpdatedAt.Add(time.Second)) {
			return newError("timestamp", ErrInvalidSchema, string(checkpoint.RuntimeID),
				errTimestampOrder("lastKnownGoodAt after updatedAt"))
		}
	}
	return nil
}

func ServiceConsistency(checkpoint RuntimeCheckpoint) error {
	seen := make(map[domain.ServiceID]struct{}, len(checkpoint.Services))
	for _, svc := range checkpoint.Services {
		if svc.ServiceID == "" {
			return newError("service_consistency", ErrInvalidService, string(checkpoint.RuntimeID),
				errEmptyServiceID)
		}
		if _, exists := seen[svc.ServiceID]; exists {
			return newError("service_consistency", ErrInvalidService, string(checkpoint.RuntimeID),
				errDuplicateService(svc.ServiceID))
		}
		seen[svc.ServiceID] = struct{}{}
	}
	return nil
}

var (
	errMissingRuntimeID   = errorString("missing runtime id")
	errMissingPluginID    = errorString("missing plugin id")
	errInvalidRuntimeState = func(s domain.RuntimeState) error {
		return errorf("invalid runtime state: %s", s)
	}
	errInvalidServiceState = func(s runtime.ServiceRuntimeState) error {
		return errorf("invalid service state: %s", s)
	}
	errIDMismatch = func(a, b string) error {
		return errorf("id mismatch: %s vs %s", a, b)
	}
	errRevisionMismatch = func(a, b string) error {
		return errorf("revision mismatch: %s vs %s", a, b)
	}
	errTimestampOrder = func(detail string) error {
		return errorf("timestamp order violation: %s", detail)
	}
	errEmptyServiceID = errorString("empty service id")
	errDuplicateService = func(id domain.ServiceID) error {
		return errorf("duplicate service id: %s", id)
	}
)

type errorString string

func (e errorString) Error() string { return string(e) }

func errorf(format string, args ...any) error {
	msg := format
	if len(args) > 0 {
		msg = sprintf(format, args...)
	}
	return errorString(msg)
}

func sprintf(format string, args ...any) string {
	result := ""
	argIdx := 0
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) && argIdx < len(args) {
			switch format[i+1] {
			case 's':
				result += toString(args[argIdx])
			default:
				result += toString(args[argIdx])
			}
			i++
			argIdx++
			continue
		}
		result += string(format[i])
	}
	return result
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if s, ok := v.(domain.RuntimeInstanceID); ok {
		return string(s)
	}
	if s, ok := v.(domain.PluginID); ok {
		return string(s)
	}
	if s, ok := v.(domain.RuntimeState); ok {
		return string(s)
	}
	if s, ok := v.(domain.ServiceID); ok {
		return string(s)
	}
	if s, ok := v.(runtime.ServiceRuntimeState); ok {
		return string(s)
	}
	if s, ok := v.(errorString); ok {
		return string(s)
	}
	return ""
}
