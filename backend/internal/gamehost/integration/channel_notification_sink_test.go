package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/channel"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/notification"
	ghpermission "github.com/u-ai/backend/internal/gamehost/permission"
)

type allowChannelPermissionChecker struct{}

func (allowChannelPermissionChecker) CheckServicePermission(context.Context, string, string, string, string) ghpermission.DecisionResult {
	return ghpermission.DecisionResult{Decision: ghpermission.DecisionAllowed}
}

type staticRuntimeGeneration struct {
	generation int64
	err        error
}

func (s staticRuntimeGeneration) GetCurrentGeneration(domain.RuntimeInstanceID) (int64, error) {
	return s.generation, s.err
}

func TestChannelNotificationSinkRejectsStaleTrustedGeneration(t *testing.T) {
	sink := NewChannelNotificationSink(channel.NewRouter(channel.RouterConfig{}))
	sink.SetPermissionChecker(allowChannelPermissionChecker{})
	sink.SetResourceAdmission(nil, staticRuntimeGeneration{generation: 2})

	err := sink.Publish(context.Background(), notification.Notification{
		PluginID:   "plugin-1",
		RuntimeID:  "runtime-1",
		ServiceID:  "service-1",
		Generation: 1,
		Method:     channelPublishMethod,
		Payload:    json.RawMessage(`{"channelId":"events","payload":{}}`),
	})
	if err == nil || !domain.IsHostError(err, domain.ErrConflict) {
		t.Fatalf("expected stale generation conflict, got %v", err)
	}
}

func TestChannelNotificationSinkRejectsMissingTrustedGeneration(t *testing.T) {
	sink := NewChannelNotificationSink(channel.NewRouter(channel.RouterConfig{}))
	sink.SetPermissionChecker(allowChannelPermissionChecker{})

	err := sink.Publish(context.Background(), notification.Notification{
		PluginID:  "plugin-1",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
		Method:    channelPublishMethod,
		Payload:   json.RawMessage(`{"channelId":"events","payload":{}}`),
	})
	if err == nil || !domain.IsHostError(err, domain.ErrInvalidArgument) {
		t.Fatalf("expected invalid trusted generation, got %v", err)
	}
}
