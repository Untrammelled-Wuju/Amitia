package main

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/interaction"
)

func (r *ownedChatIngress) bindLocalSource(ctx context.Context, spaceID string) (context.Context, func(), error) {
	if !r.EntryReady() || r.services.DeviceMesh.LocalHandler == nil || r.services.DeviceMesh.LocalDeviceID == "" || spaceID != "" && spaceID != r.coreID {
		return ctx, nil, errors.New("本地消息来源没有有效的设备绑定")
	}
	current, finish, err := r.services.DeviceMesh.LocalHandler.LocalInferenceContext(ctx)
	if err != nil {
		return ctx, nil, err
	}
	return interaction.WithEntryOwner(current, interaction.EntryOwner{SpaceID: r.coreID, CoreID: r.coreID, DeviceID: r.services.DeviceMesh.LocalDeviceID, TargetDeviceID: r.services.DeviceMesh.LocalDeviceID}), finish, nil
}

func (r *ownedChatIngress) channelOwnerResolver(ctx context.Context, extensionID, moduleID, channelID string) (context.Context, error) {
	if !r.EntryReady() || r.services.KernelContainer == nil || r.services.KernelContainer.CapabilityProviders == nil {
		return ctx, errors.New("渠道设备绑定服务未就绪")
	}
	registry := r.services.KernelContainer.CapabilityProviders
	var selected *interaction.EntryOwner
	localRuntime := false
	for _, instance := range registry.ListInstancesByCapability(capability.CapabilityID("channel.provider")) {
		if instance == nil || instance.ExtensionID != extensionID || instance.ModuleID != moduleID || !instance.IsExecutable() {
			continue
		}
		definition, ok := registry.GetByID(instance.ProviderID)
		if !ok || definition == nil {
			continue
		}
		declared, _ := definition.Metadata["channelId"].(string)
		if declared == "" {
			declared, _ = definition.Metadata["id"].(string)
		}
		if declared == "" {
			if labels, ok := definition.Metadata["labels"].(map[string]any); ok {
				declared, _ = labels["channelId"].(string)
			}
		}
		if declared != channelID {
			continue
		}
		if instance.SpaceID == "" || instance.DeviceID == "" {
			if instance.Placement == capability.ProviderPlacementCore && instance.RuntimeInstanceID != "" {
				owner := interaction.EntryOwner{SpaceID: r.coreID, CoreID: r.coreID, DeviceID: r.services.DeviceMesh.LocalDeviceID, TargetDeviceID: r.services.DeviceMesh.LocalDeviceID}
				if selected != nil && *selected != owner {
					return ctx, errors.New("渠道来源对应多个设备，必须使用设备专属接入凭据")
				}
				selected, localRuntime = &owner, true
			}
			continue
		}
		owner := interaction.EntryOwner{SpaceID: instance.SpaceID.String(), DeviceID: instance.DeviceID.String(), TargetDeviceID: instance.DeviceID.String(), CoreID: r.coreID}
		if selected != nil && *selected != owner {
			return ctx, errors.New("渠道来源对应多个设备，必须使用设备专属接入凭据")
		}
		selected = &owner
	}
	if selected != nil {
		if localRuntime {
			current, finish, err := r.bindLocalSource(ctx, r.coreID)
			if err != nil {
				return ctx, err
			}
			context.AfterFunc(current, finish)
			return current, nil
		}
		return interaction.WithEntryOwner(ctx, *selected), nil
	}
	return ctx, errors.New("渠道来源没有已认证的设备运行实例绑定")
}
