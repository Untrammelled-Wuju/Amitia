package temporal

import (
	"context"
	"strings"
)

type deviceTimezoneContextKey struct{}

func ContextWithDeviceTimezone(ctx context.Context, timezone string) context.Context {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		return ctx
	}
	return context.WithValue(ctx, deviceTimezoneContextKey{}, timezone)
}

func DeviceTimezoneFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(deviceTimezoneContextKey{}).(string)
	return strings.TrimSpace(value)
}
