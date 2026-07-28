package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/desktop"
	"github.com/u-ai/backend/internal/extension/kernel/desktop_update"
)

type UpdateManagerAdapter struct {
	manager     *desktop_update.UpdateManager
	desktopHost *desktop.DesktopHost
}

func NewUpdateManagerAdapter(manager *desktop_update.UpdateManager, hosts ...*desktop.DesktopHost) *UpdateManagerAdapter {
	adapter := &UpdateManagerAdapter{manager: manager}
	if len(hosts) > 0 {
		adapter.desktopHost = hosts[0]
	}
	return adapter
}

func (a *UpdateManagerAdapter) CheckForUpdates(ctx context.Context, extensionID string) ([]desktop.ExtensionUpdateMeta, error) {
	metadatas, err := a.manager.CheckForUpdatesDetailed(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	result := make([]desktop.ExtensionUpdateMeta, 0, len(metadatas))
	for _, m := range metadatas {
		result = append(result, convertMetadata(m))
	}
	return result, nil
}

func (a *UpdateManagerAdapter) DownloadUpdate(ctx context.Context, extensionID, version string) (string, error) {
	metadatas, err := a.manager.CheckForUpdatesDetailed(ctx, extensionID)
	if err != nil {
		return "", err
	}
	var targetMeta *desktop_update.ExtensionUpdateMetadata
	for i := range metadatas {
		if metadatas[i].Version == version {
			targetMeta = &metadatas[i]
			break
		}
	}
	if targetMeta == nil {
		return "", fmt.Errorf("version %s not found for extension %s", version, extensionID)
	}
	op, err := a.manager.CreateUpdateOperation(ctx, extensionID, *targetMeta)
	if err != nil {
		return "", err
	}
	if err := a.manager.DownloadUpdate(ctx, op.OperationID); err != nil {
		return op.OperationID, err
	}
	return op.OperationID, nil
}

func (a *UpdateManagerAdapter) InstallUpdate(ctx context.Context, operationID string) error {
	if err := a.manager.RunFullUpdate(ctx, operationID); err != nil {
		return err
	}
	if a.desktopHost != nil {
		a.desktopHost.BuildSnapshot(desktop.SortContext{})
	}
	return nil
}

func (a *UpdateManagerAdapter) CancelUpdate(ctx context.Context, operationID string) error {
	return a.manager.CancelUpdate(ctx, operationID)
}

func (a *UpdateManagerAdapter) RetryUpdate(ctx context.Context, operationID string) error {
	return a.manager.RetryUpdate(ctx, operationID)
}

func (a *UpdateManagerAdapter) RollbackUpdate(ctx context.Context, operationID string) error {
	return a.manager.RollbackUpdate(ctx, operationID)
}

func (a *UpdateManagerAdapter) GetOperation(ctx context.Context, operationID string) (*desktop.UpdateOperationInfo, error) {
	op, ok := a.manager.GetOperation(operationID)
	if !ok {
		return nil, fmt.Errorf("operation %s not found", operationID)
	}
	return &desktop.UpdateOperationInfo{
		OperationID: op.OperationID,
		ExtensionID: op.ExtensionID,
		Status:      string(op.Status),
		Version:     op.ToVersion,
		CreatedAt:   op.StartedAt,
		UpdatedAt:   op.StartedAt,
		Error:       op.ErrorMessage,
	}, nil
}

func (a *UpdateManagerAdapter) GetOperationSteps(ctx context.Context, operationID string) ([]desktop.UpdateOperationStepInfo, error) {
	entries := a.manager.Journal().GetEntries(operationID)
	result := make([]desktop.UpdateOperationStepInfo, 0, len(entries))
	for _, e := range entries {
		var endedAt time.Time
		if e.FinishedAt != nil {
			endedAt = *e.FinishedAt
		}
		result = append(result, desktop.UpdateOperationStepInfo{
			StepID:    e.Step,
			Name:      e.Step,
			Status:    e.Status,
			StartedAt: e.StartedAt,
			EndedAt:   endedAt,
			Error:     e.Error,
		})
	}
	return result, nil
}

func convertMetadata(m desktop_update.ExtensionUpdateMetadata) desktop.ExtensionUpdateMeta {
	return desktop.ExtensionUpdateMeta{
		ExtensionID:        m.ExtensionID,
		Version:            m.Version,
		ManifestVersion:    m.ManifestVersion,
		PackageURL:         m.PackageURL,
		PackageSHA256:      m.PackageSHA256,
		PackageSHA512:      m.PackageSHA512,
		PackageSize:        m.PackageSize,
		PublisherID:        m.PublisherID,
		PublisherKeyID:     m.PublisherKeyID,
		Signature:          m.Signature,
		MinimumHostVersion: m.MinimumHostVersion,
		MaximumHostVersion: m.MaximumHostVersion,
		SupportedPlatforms: m.SupportedPlatforms,
		SupportedArch:      m.SupportedArch,
		PublishedAt:        m.PublishedAt,
		ReleaseChannel:     m.ReleaseChannel,
	}
}
