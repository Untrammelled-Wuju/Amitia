package main

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
)

type ownedCharacterPreview struct{ services *AppServices }

func (p ownedCharacterPreview) TestChat(ctx context.Context, characterID, message string) (string, error) {
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil || actor.DeviceID == "" || p.services == nil || p.services.OwnedBusiness == nil {
		return "", errors.New("角色预览缺少有效设备身份")
	}
	request := business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), TargetDeviceID: actor.DeviceID.String(), RoleID: characterID, RequestID: uuid.NewString(), Message: message, CoreID: p.services.OwnedIngress.coreID}
	response, err := p.services.OwnedBusiness.PreviewReply(ctx, request)
	return response.Text, err
}
