package notification

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

const (
	maxMethodLength     = 1024
	maxMetadataKeyLength = 256
)

func ValidateRoute(ctx RouteContext) error {
	if ctx.PluginID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "notification: route.plugin_id must not be empty")
	}
	if ctx.RuntimeID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "notification: route.runtime_id must not be empty")
	}
	if ctx.ServiceID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "notification: route.service_id must not be empty")
	}
	if containsControlChars(string(ctx.PluginID)) {
		return domain.NewHostError(domain.ErrInvalidArgument, "notification: route.plugin_id contains control characters")
	}
	if containsControlChars(string(ctx.RuntimeID)) {
		return domain.NewHostError(domain.ErrInvalidArgument, "notification: route.runtime_id contains control characters")
	}
	if containsControlChars(string(ctx.ServiceID)) {
		return domain.NewHostError(domain.ErrInvalidArgument, "notification: route.service_id contains control characters")
	}
	return nil
}

func ValidateMethod(method string) error {
	if method == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "notification: method must not be empty")
	}
	if len(method) > maxMethodLength {
		return domain.NewHostError(domain.ErrInvalidArgument, "notification: method exceeds maximum length")
	}
	if containsControlChars(method) {
		return domain.NewHostError(domain.ErrInvalidArgument, "notification: method contains control characters")
	}
	return nil
}

func ValidateMetadata(metadata map[string]json.RawMessage) error {
	if metadata == nil {
		return nil
	}
	for k := range metadata {
		if k == "" {
			return domain.NewHostError(domain.ErrInvalidArgument, "notification: metadata key must not be empty")
		}
		if len(k) > maxMetadataKeyLength {
			return domain.NewHostError(domain.ErrInvalidArgument, "notification: metadata key exceeds maximum length")
		}
		if containsControlChars(k) {
			return domain.NewHostError(domain.ErrInvalidArgument, "notification: metadata key contains control characters")
		}
	}
	return nil
}

func containsControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 {
			return true
		}
	}
	return false
}
