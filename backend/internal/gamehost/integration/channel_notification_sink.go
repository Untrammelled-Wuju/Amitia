package integration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/gamehost/channel"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/notification"
	ghpermission "github.com/u-ai/backend/internal/gamehost/permission"
	"github.com/u-ai/backend/internal/gamehost/resource"
)

const channelPublishMethod = "channel.publish"

type ChannelNotificationSink struct {
	router      *channel.Router
	admission   *resource.ResourceAdmissionAdapter
	generation  RuntimeGenerationReader
	permissions ChannelPermissionChecker
}

type RuntimeGenerationReader interface {
	GetCurrentGeneration(runtimeID domain.RuntimeInstanceID) (int64, error)
}

type ChannelPermissionChecker interface {
	CheckServicePermission(ctx context.Context, runtimeID, pluginID, serviceID, permID string) ghpermission.DecisionResult
}

func NewChannelNotificationSink(router *channel.Router) *ChannelNotificationSink {
	return &ChannelNotificationSink{router: router}
}

func (s *ChannelNotificationSink) SetResourceAdmission(admission *resource.ResourceAdmissionAdapter, generation RuntimeGenerationReader) {
	if s == nil {
		return
	}
	s.admission = admission
	s.generation = generation
}

func (s *ChannelNotificationSink) SetPermissionChecker(checker ChannelPermissionChecker) {
	if s == nil {
		return
	}
	s.permissions = checker
}

type channelPublishPayload struct {
	ChannelID string                     `json:"channelId"`
	Payload   json.RawMessage            `json:"payload"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

func (s *ChannelNotificationSink) Publish(ctx context.Context, n notification.Notification) error {
	if n.Method != channelPublishMethod {
		return nil
	}
	if s == nil || s.router == nil {
		return fmt.Errorf("channel notification sink: router is nil")
	}
	var payload channelPublishPayload
	if err := json.Unmarshal(n.Payload, &payload); err != nil {
		return fmt.Errorf("channel.publish: decode payload: %w", err)
	}
	if payload.ChannelID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "channel.publish: channelId is required")
	}
	if s.permissions == nil {
		return domain.NewHostError(domain.ErrPermissionDenied, "channel.publish: permission checker unavailable")
	}
	permissionResult := s.permissions.CheckServicePermission(
		ctx,
		string(n.RuntimeID),
		string(n.PluginID),
		string(n.ServiceID),
		permission.PermissionGameHostChannelUse,
	)
	if !permissionResult.Allowed() {
		message := "channel.publish: permission denied"
		if permissionResult.Decision == ghpermission.DecisionRequireApproval {
			message = "channel.publish: approval required"
		}
		return domain.NewHostError(domain.ErrPermissionDenied, message)
	}
	if s.admission != nil {
		if s.generation == nil {
			return domain.NewHostError(domain.ErrInternal, "channel.publish: runtime generation reader unavailable")
		}
		generation, err := s.generation.GetCurrentGeneration(n.RuntimeID)
		if err != nil || generation <= 0 {
			return domain.NewHostErrorWithCause(domain.ErrRuntimeUnavailable, "channel.publish: runtime generation unavailable", err)
		}
		decision, release := s.admission.AcquireQueuePublish(ctx, resource.RuntimeIdentitySubject{
			PluginID: string(n.PluginID), RuntimeID: string(n.RuntimeID), ServiceID: string(n.ServiceID), Generation: generation,
		})
		if !decision.Allowed {
			return domain.NewHostError(domain.ErrResourceExhausted, "channel.publish: queue admission denied: "+string(decision.Reason))
		}
		defer release()
	}
	return s.router.Route(ctx, channel.IncomingChannelMessage{
		Peer: ipc.Peer{
			PluginID:  n.PluginID,
			RuntimeID: n.RuntimeID,
			ServiceID: n.ServiceID,
		},
		ChannelID: domain.ChannelID(payload.ChannelID),
		Payload:   payload.Payload,
		Metadata:  payload.Metadata,
	})
}
