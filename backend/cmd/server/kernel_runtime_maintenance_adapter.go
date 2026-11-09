package main

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/system"
)

type kernelRuntimeMaintenanceAdapter struct {
	service system.Service
}

func (a kernelRuntimeMaintenanceAdapter) Overview(ctx context.Context) (any, error) {
	status, err := a.service.TaskMaintenanceStatus(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(status)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	modules := mindruntime.RunAllModuleChecks()
	allHealthy := true
	for _, module := range modules {
		if !module.Healthy {
			allHealthy = false
			break
		}
	}
	payload["modulesHealthy"] = allHealthy
	payload["modules"] = modules
	payload["healthHistory"] = a.service.GetRuntimeHealthHistory()
	return payload, nil
}

func (a kernelRuntimeMaintenanceAdapter) Status(ctx context.Context) (any, error) {
	return a.service.TaskMaintenanceStatus(ctx)
}

func (a kernelRuntimeMaintenanceAdapter) Config(ctx context.Context) (any, error) {
	return a.service.TaskMaintenanceConfig(ctx)
}

func (a kernelRuntimeMaintenanceAdapter) UpdateConfig(ctx context.Context, input json.RawMessage) (any, error) {
	var request system.TaskMaintenanceUpdate
	if err := json.Unmarshal(input, &request); err != nil {
		return nil, err
	}
	return a.service.UpdateTaskMaintenanceConfig(ctx, request)
}

func (a kernelRuntimeMaintenanceAdapter) CleanupTemp(context.Context) (any, error) {
	return a.service.CleanupTemp(), nil
}

func (a kernelRuntimeMaintenanceAdapter) RotateLogs(context.Context) (any, error) {
	return a.service.RotateLogs(), nil
}

func (a kernelRuntimeMaintenanceAdapter) CheckDBIntegrity(context.Context) (any, error) {
	return a.service.CheckDBIntegrity(), nil
}
