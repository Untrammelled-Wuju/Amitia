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
	_, err := s.ValidateAndCanonicalize(ctx, n)
	return err
}

// ValidateAndCanonicalize applies the complete channel.publish trust boundary and
// returns a notification safe for downstream persistence/fanout. The plugin's
// route identity/generation/permission are taken from the authenticated
// notification envelope, not from payload data. Binary payloads are rewritten
// to the host-authoritative reference returned by the binary resolver.
func (s *ChannelNotificationSink) ValidateAndCanonicalize(ctx context.Context, n notification.Notification) (notification.Notification, error) {
	if n.Method != channelPublishMethod {
		return n, nil
	}
	if s == nil || s.router == nil {
		return notification.Notification{}, fmt.Errorf("channel notification sink: router is nil")
	}
	var payload channelPublishPayload
	if err := json.Unmarshal(n.Payload, &payload); err != nil {
		return notification.Notification{}, fmt.Errorf("channel.publish: decode payload: %w", err)
	}
	if payload.ChannelID == "" {
		return notification.Notification{}, domain.NewHostError(domain.ErrInvalidArgument, "channel.publish: channelId is required")
	}
	if s.permissions == nil {
		return notification.Notification{}, domain.NewHostError(domain.ErrPermissionDenied, "channel.publish: permission checker unavailable")
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
		return notification.Notification{}, domain.NewHostError(domain.ErrPermissionDenied, message)
	}
	if n.Generation <= 0 {
		return notification.Notification{}, domain.NewHostError(domain.ErrInvalidArgument, "channel.publish: trusted generation must be positive")
	}
	if s.generation != nil {
		currentGeneration, err := s.generation.GetCurrentGeneration(n.RuntimeID)
		if err != nil || currentGeneration <= 0 {
			return notification.Notification{}, domain.NewHostErrorWithCause(domain.ErrRuntimeUnavailable, "channel.publish: runtime generation unavailable", err)
		}
		if currentGeneration != n.Generation {
			return notification.Notification{}, domain.NewHostError(domain.ErrConflict, "channel.publish: stale runtime generation")
		}
	}
	if s.admission != nil {
		decision, release := s.admission.AcquireQueuePublish(ctx, resource.RuntimeIdentitySubject{
			PluginID: string(n.PluginID), RuntimeID: string(n.RuntimeID), ServiceID: string(n.ServiceID), Generation: n.Generation,
		})
		if !decision.Allowed {
			return notification.Notification{}, domain.NewHostError(domain.ErrResourceExhausted, "channel.publish: queue admission denied: "+string(decision.Reason))
		}
		defer release()
	}

	canonicalPayload, err := s.router.RouteCanonical(ctx, channel.IncomingChannelMessage{
		Peer: ipc.Peer{
			PluginID:   n.PluginID,
			RuntimeID:  n.RuntimeID,
			ServiceID:  n.ServiceID,
			Generation: n.Generation,
		},
		ChannelID: domain.ChannelID(payload.ChannelID),
		Payload:   payload.Payload,
		Metadata:  payload.Metadata,
	})
	if err != nil {
		return notification.Notification{}, err
	}
	payload.Payload = canonicalPayload
	canonicalEnvelope, err := json.Marshal(payload)
	if err != nil {
		return notification.Notification{}, fmt.Errorf("channel.publish: encode canonical payload: %w", err)
	}
	n.Payload = canonicalEnvelope
	return n, nil
}
