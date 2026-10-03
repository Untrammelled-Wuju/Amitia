package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type workflowDeviceControlPlane struct {
	mesh *devicemesh.Runtime
}

func newWorkflowDeviceControlPlane(mesh *devicemesh.Runtime) extension.WorkflowDeviceControlPlane {
	if mesh == nil {
		return nil
	}
	return &workflowDeviceControlPlane{mesh: mesh}
}

func (p *workflowDeviceControlPlane) ListDevices(ctx context.Context, spaceID string) ([]extension.WorkflowDeviceDescriptor, error) {
	if p == nil || p.mesh == nil || p.mesh.Hub == nil {
		return nil, errors.New("device mesh unavailable")
	}
	uid := runtimeidentity.SpaceID(strings.TrimSpace(spaceID))
	if uid == "" {
		return nil, errors.New("user id is required")
	}
	connections := p.mesh.Hub.ListBySpace(uid)
	online := make(map[string]extension.WorkflowDeviceDescriptor, len(connections))
	for _, conn := range connections {
		if conn != nil {
			online[conn.DeviceID.String()] = extension.WorkflowDeviceDescriptor{DeviceID: conn.DeviceID.String(), RuntimeID: conn.RuntimeID.String(), Online: true, LastSeenAt: conn.LastPongAt}
		}
	}

	// Device registry is the account-owned catalog and therefore includes
	// offline devices. The hub only supplies current online/runtime state.
	if p.mesh.DeviceReg == nil {
		items := make([]extension.WorkflowDeviceDescriptor, 0, len(connections))
		for _, conn := range connections {
			if conn == nil {
				continue
			}
			items = append(items, extension.WorkflowDeviceDescriptor{DeviceID: conn.DeviceID.String(), RuntimeID: conn.RuntimeID.String(), Online: true, LastSeenAt: conn.LastPongAt})
		}
		return items, nil
	}
	records, err := p.mesh.DeviceReg.ListDevicesBySpace(ctx, uid)
	if err != nil {
		return nil, err
	}
	items := make([]extension.WorkflowDeviceDescriptor, 0, len(records))
	for _, record := range records {
		if record == nil || record.TrustState == host_registry.DeviceTrustRevoked {
			continue
		}
		descriptor := extension.WorkflowDeviceDescriptor{
			DeviceID:   record.DeviceID.String(),
			Label:      record.Label,
			Platform:   record.Platform.String(),
			LastSeenAt: record.LastSeenAt,
		}
		if conn, ok := online[record.DeviceID.String()]; ok {
			descriptor.Online = true
			descriptor.RuntimeID = conn.RuntimeID
			if conn.LastSeenAt.After(descriptor.LastSeenAt) {
				descriptor.LastSeenAt = conn.LastSeenAt
			}
		}
		items = append(items, descriptor)
	}
	return items, nil
}

func (p *workflowDeviceControlPlane) Invoke(ctx context.Context, spaceID, deviceID, operation string, input json.RawMessage) (json.RawMessage, error) {
	if p == nil || p.mesh == nil {
		return nil, errors.New("device mesh unavailable")
	}
	spaceID = strings.TrimSpace(spaceID)
	deviceID = strings.TrimSpace(deviceID)
	operation = strings.TrimSpace(operation)
	if spaceID == "" || deviceID == "" || operation == "" {
		return nil, errors.New("spaceId, deviceId, and workflow operation are required")
	}
	if p.mesh.DeviceReg == nil {
		return nil, errors.New("device registry unavailable")
	}
	if err := p.mesh.DeviceReg.RequireDeviceOwnedBy(ctx, runtimeidentity.SpaceID(spaceID), runtimeidentity.DeviceID(deviceID)); err != nil {
		return nil, err
	}
	result, err := p.mesh.InvokeDeviceHandlerWithRuntimeType(
		ctx,
		runtimeidentity.SpaceID(spaceID),
		runtimeidentity.DeviceID(deviceID),
		capability.RuntimeTypeWorkflow,
		operation,
		input,
		45*time.Second,
	)
	if err != nil {
		return nil, err
	}
	if len(result.Structured) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return result.Structured, nil
}
