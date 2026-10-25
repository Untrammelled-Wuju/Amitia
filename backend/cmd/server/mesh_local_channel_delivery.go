package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

type channelDeliverySourceCallKey struct{}

func (p *meshLocalDataPort) SubmitChannelDelivery(ctx context.Context, scope coordination.ExecutionScope, request coordination.ChannelDeliveryRequest) error {
	if p.ownerID != scope.TargetDeviceID || scope.RequestID == "" || request.RequestID != scope.RequestID || request.ConversationID == "" || request.PeerID == "" || request.Channel == "" || strings.EqualFold(request.Channel, "web") || strings.EqualFold(request.Channel, "voice") {
		return coordination.ErrWrongOwner
	}
	if p.services.DeliveryStore == nil {
		return errors.New("设备渠道交付服务未就绪")
	}
	if len(request.Text)+len(request.Outputs) > 2<<20 {
		return coordination.ErrPendingLimit
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	ctx = coordination.WithoutAdditionalGuard(ctx)
	response := &interaction.ProcessResponse{RequestID: request.RequestID, Reply: request.Text}
	if err := ownedIngressResponseOutputs(response, request.Outputs); err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(strings.Join([]string{scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, scope.RoleID, request.RequestID}, "\x00")))
	group := "owned-" + hex.EncodeToString(digest[:])
	submit := func() error {
		return coordination.CommitCurrent(ctx, func() error {
			if callID, ok := ctx.Value(channelDeliverySourceCallKey{}).(string); ok {
				db := p.services.KernelContainer.DeviceRegistry.Database()
				if err := coordination.ValidateSourceCall(ctx, db, scope.SpaceID, callID); err != nil {
					return err
				}
				if err := coordination.ValidateSourceAuthority(ctx, db, scope); err != nil {
					return err
				}
			}
			for _, item := range response.MessagePlan.Items {
				payload, err := json.Marshal(map[string]any{"spaceId": scope.SpaceID, "deviceId": scope.TargetDeviceID, "characterId": scope.RoleID, "conversationId": request.ConversationID, "messageId": item.MessageID, "deliveryGroupId": group, "deliverySequence": item.Sequence, "content": item.Content, "mimeType": item.MIMEType, "altText": item.AltText, "originalPath": item.OriginalAssetReference, "fallbackPath": item.FallbackAssetReference, "isAnimated": item.IsAnimated})
				if err != nil {
					return err
				}
				intent := delivery.NewDeliveryIntent(group, request.Channel, request.PeerID, item.Type, payload)
				intent.ID = delivery.GenerateDeliveryID(group, request.Channel, request.PeerID, item.MessageID)
				intent.DeliveryGroupID, intent.DeliverySequence = group, item.Sequence
				if _, err := p.services.DeliveryStore.SubmitIntent(intent); err != nil {
					return err
				}
			}
			return nil
		})
	}
	if scope.Coordinated {
		if scope.RoleOwnerID != scope.CoreID || scope.ResourceOwnerID != scope.CoreID {
			return coordination.ErrWrongOwner
		}
		return submit()
	}
	return p.WithSourceRole(ctx, scope, submit)
}

func (p *meshLocalDataPort) PendingChannelDeliveries(ctx context.Context, cursor string) ([]coordination.PendingChannelDelivery, error) {
	rows, err := p.services.KernelContainer.DeviceRegistry.Database().QueryContext(ctx, `SELECT resource_id,revision,body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='checkpoint' AND deleted=0 AND resource_id LIKE 'channel-delivery/%' AND json_extract(body,'$.pending')=1 AND resource_id>? ORDER BY resource_id LIMIT 64`, p.ownerID, cursor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []coordination.PendingChannelDelivery{}
	for rows.Next() {
		var item coordination.PendingChannelDelivery
		var raw []byte
		if err := rows.Scan(&item.ID, &item.Revision, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		item.Cursor = item.ID
		if item.Scope.ResourceOwnerID != p.ownerID || !strings.HasPrefix(item.ID, "channel-delivery/") || item.Request.RequestID != item.Scope.RequestID {
			return nil, coordination.ErrWrongOwner
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (p *meshLocalDataPort) CompleteChannelDelivery(ctx context.Context, pending coordination.PendingChannelDelivery) error {
	if !strings.HasPrefix(pending.ID, "channel-delivery/") || pending.Scope.ResourceOwnerID != p.ownerID || pending.Revision <= 0 {
		return coordination.ErrWrongOwner
	}
	pending.Pending = false
	body, err := json.Marshal(pending)
	if err != nil {
		return err
	}
	scope := pending.Scope
	scope.RequestID += "|channel-delivered"
	_, err = p.Commit(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: pending.ID, RoleID: scope.RoleID, ExpectedRevision: pending.Revision, Body: body}}})
	return err
}
