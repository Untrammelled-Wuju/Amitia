package checkpoint

import (
	"context"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type DescriptorResolver interface {
	Resolve(pluginID domain.PluginID) (domain.PluginDescriptor, bool)
}

type CheckpointManager struct {
	store    CheckpointStore
	resolver DescriptorResolver
}

func NewCheckpointManager(store CheckpointStore, resolver DescriptorResolver) (*CheckpointManager, error) {
	if store == nil {
		return nil, &CheckpointError{Op: "new_manager", Kind: ErrInvalidSchema, ID: "", Cause: errorString("store must not be nil")}
	}
	return &CheckpointManager{
		store:    store,
		resolver: resolver,
	}, nil
}

func (m *CheckpointManager) CreateMetadata(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	pluginID domain.PluginID,
	extensionID string,
	pluginVersion string,
	descriptorRevision string,
	now time.Time,
) (RuntimeMetadata, error) {
	metadata := RuntimeMetadata{
		SchemaVersion:      MetadataSchemaVersion,
		RuntimeID:          runtimeID,
		PluginID:           pluginID,
		ExtensionID:        extensionID,
		PluginVersion:      pluginVersion,
		CreatedAt:          now,
		DescriptorRevision: descriptorRevision,
	}
	if err := m.store.SaveMetadata(ctx, metadata); err != nil {
		return RuntimeMetadata{}, err
	}
	return metadata, nil
}

func (m *CheckpointManager) SaveCreatedCheckpoint(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	pluginID domain.PluginID,
	services []domain.ServiceID,
	descriptorRevision string,
	now time.Time,
) (RuntimeCheckpoint, error) {
	serviceCheckpoints := make([]ServiceCheckpoint, 0, len(services))
	for _, svcID := range services {
		serviceCheckpoints = append(serviceCheckpoints, ServiceCheckpoint{
			ServiceID: svcID,
			State:     runtime.ServiceStateCreated,
			Required:  true,
			UpdatedAt: now,
		})
	}
	sortServiceCheckpoints(serviceCheckpoints)

	checkpoint := RuntimeCheckpoint{
		SchemaVersion:      MetadataSchemaVersion,
		RuntimeID:          runtimeID,
		PluginID:           pluginID,
		RuntimeState:       domain.RuntimeStateCreated,
		Services:           serviceCheckpoints,
		DescriptorRevision: descriptorRevision,
		CreatedAt:          now,
		UpdatedAt:          now,
		CleanShutdown:      false,
	}

	if err := m.store.SaveCheckpoint(ctx, checkpoint); err != nil {
		return RuntimeCheckpoint{}, err
	}
	return checkpoint, nil
}

func (m *CheckpointManager) SaveRunningCheckpoint(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	pluginID domain.PluginID,
	services []ServiceCheckpoint,
	descriptorRevision string,
	lastKnownGood bool,
	now time.Time,
) (RuntimeCheckpoint, error) {
	existing, err := m.store.LoadCheckpoint(ctx, runtimeID)
	existingExists := err == nil

	svcCopy := make([]ServiceCheckpoint, len(services))
	copy(svcCopy, services)
	sortServiceCheckpoints(svcCopy)

	checkpoint := RuntimeCheckpoint{
		SchemaVersion:      MetadataSchemaVersion,
		RuntimeID:          runtimeID,
		PluginID:           pluginID,
		RuntimeState:       domain.RuntimeStateRunning,
		Services:           svcCopy,
		DescriptorRevision: descriptorRevision,
		UpdatedAt:          now,
		CleanShutdown:      false,
	}

	if existingExists {
		checkpoint.CreatedAt = existing.CreatedAt
		checkpoint.LastKnownGoodAt = existing.LastKnownGoodAt
	} else {
		checkpoint.CreatedAt = now
	}

	if lastKnownGood {
		checkpoint.LastKnownGoodAt = &now
	}

	if err := m.store.SaveCheckpoint(ctx, checkpoint); err != nil {
		return RuntimeCheckpoint{}, err
	}
	return checkpoint, nil
}

func (m *CheckpointManager) SaveStoppedCheckpoint(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	pluginID domain.PluginID,
	cleanShutdown bool,
	reason string,
	now time.Time,
) (RuntimeCheckpoint, error) {
	checkpoint, err := m.store.LoadCheckpoint(ctx, runtimeID)
	if err != nil {
		checkpoint = RuntimeCheckpoint{
			RuntimeID: runtimeID,
			PluginID:  pluginID,
			CreatedAt: now,
			Services:  []ServiceCheckpoint{},
		}
	}

	checkpoint.SchemaVersion = MetadataSchemaVersion
	checkpoint.RuntimeState = domain.RuntimeStateStopped
	checkpoint.UpdatedAt = now
	checkpoint.CleanShutdown = cleanShutdown
	checkpoint.Reason = truncateReason(reason)

	for i := range checkpoint.Services {
		if !runtime.IsTerminalServiceState(checkpoint.Services[i].State) {
			checkpoint.Services[i].State = runtime.ServiceStateStopped
			checkpoint.Services[i].UpdatedAt = now
		}
	}

	if err := m.store.SaveCheckpoint(ctx, checkpoint); err != nil {
		return RuntimeCheckpoint{}, err
	}
	return checkpoint, nil
}

func (m *CheckpointManager) SaveFailedCheckpoint(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	pluginID domain.PluginID,
	reason string,
	now time.Time,
) (RuntimeCheckpoint, error) {
	checkpoint, err := m.store.LoadCheckpoint(ctx, runtimeID)
	if err != nil {
		checkpoint = RuntimeCheckpoint{
			RuntimeID: runtimeID,
			PluginID:  pluginID,
			CreatedAt: now,
			Services:  []ServiceCheckpoint{},
		}
	}

	checkpoint.SchemaVersion = MetadataSchemaVersion
	checkpoint.RuntimeState = domain.RuntimeStateFailed
	checkpoint.UpdatedAt = now
	checkpoint.CleanShutdown = false
	checkpoint.Reason = truncateReason(reason)

	if err := m.store.SaveCheckpoint(ctx, checkpoint); err != nil {
		return RuntimeCheckpoint{}, err
	}
	return checkpoint, nil
}

func (m *CheckpointManager) LoadMetadata(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeMetadata, error) {
	return m.store.LoadMetadata(ctx, runtimeID)
}

func (m *CheckpointManager) LoadCheckpoint(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeCheckpoint, error) {
	return m.store.LoadCheckpoint(ctx, runtimeID)
}

func (m *CheckpointManager) ValidateStoredCheckpoint(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
) error {
	metadata, err := m.store.LoadMetadata(ctx, runtimeID)
	if err != nil {
		return err
	}
	checkpoint, err := m.store.LoadCheckpoint(ctx, runtimeID)
	if err != nil {
		return err
	}

	var descriptor *domain.PluginDescriptor
	if m.resolver != nil {
		if d, ok := m.resolver.Resolve(metadata.PluginID); ok {
			descriptor = &d
		}
	}

	return ValidateCheckpoint(metadata, checkpoint, descriptor)
}

func sortServiceCheckpoints(services []ServiceCheckpoint) {
	sort.SliceStable(services, func(i, j int) bool {
		return services[i].ServiceID < services[j].ServiceID
	})
}

func truncateReason(reason string) string {
	if len(reason) > MaxReasonLength {
		return reason[:MaxReasonLength]
	}
	return reason
}
