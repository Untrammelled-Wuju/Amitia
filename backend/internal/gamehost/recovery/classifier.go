package recovery

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type CheckpointClassifier interface {
	Classify(ctx context.Context, runtimeID domain.RuntimeInstanceID, currentRevision string) (CheckpointInfo, error)
	CanRebuild(info CheckpointInfo) bool
}

type CheckpointStoreReader interface {
	HasMetadata(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error)
	LoadMetadata(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeMetadataView, error)
	LoadCheckpoint(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeCheckpointView, error)
}

type RuntimeMetadataView struct {
	RuntimeID         domain.RuntimeInstanceID
	PluginID          domain.PluginID
	ExtensionID       string
	DescriptorRevision string
}

type RuntimeCheckpointView struct {
	RuntimeID         domain.RuntimeInstanceID
	PluginID          domain.PluginID
	RuntimeState      domain.RuntimeState
	CleanShutdown     bool
	DescriptorRevision string
}

type DefaultCheckpointClassifier struct {
	store CheckpointStoreReader
}

func NewDefaultCheckpointClassifier(store CheckpointStoreReader) *DefaultCheckpointClassifier {
	return &DefaultCheckpointClassifier{store: store}
}

func (c *DefaultCheckpointClassifier) Classify(ctx context.Context, runtimeID domain.RuntimeInstanceID, currentRevision string) (CheckpointInfo, error) {
	hasMeta, err := c.store.HasMetadata(ctx, runtimeID)
	if err != nil {
		return CheckpointInfo{
			Class:     CheckpointCorrupt,
			RuntimeID: runtimeID,
		}, fmt.Errorf("checkpoint metadata read failed: %w", err)
	}
	if !hasMeta {
		return CheckpointInfo{
			Class:     CheckpointMissing,
			RuntimeID: runtimeID,
		}, nil
	}

	metadata, err := c.store.LoadMetadata(ctx, runtimeID)
	if err != nil {
		return CheckpointInfo{
			Class:     CheckpointCorrupt,
			RuntimeID: runtimeID,
		}, fmt.Errorf("checkpoint metadata corrupt: %w", err)
	}

	if currentRevision != "" && metadata.DescriptorRevision != currentRevision {
		return CheckpointInfo{
			Class:     CheckpointStale,
			RuntimeID: metadata.RuntimeID,
			PluginID:  metadata.PluginID,
			ExtensionID: metadata.ExtensionID,
			Revision:  metadata.DescriptorRevision,
		}, nil
	}

	checkpoint, err := c.store.LoadCheckpoint(ctx, runtimeID)
	if err != nil {
		return CheckpointInfo{
			Class:     CheckpointIncompatible,
			RuntimeID: runtimeID,
		}, fmt.Errorf("checkpoint incompatible: %w", err)
	}

	return CheckpointInfo{
		Class:         CheckpointCompatible,
		RuntimeID:     checkpoint.RuntimeID,
		PluginID:      checkpoint.PluginID,
		ExtensionID:   metadata.ExtensionID,
		Revision:      checkpoint.DescriptorRevision,
		CleanShutdown: checkpoint.CleanShutdown,
		CanRebuild:    true,
	}, nil
}

func (c *DefaultCheckpointClassifier) CanRebuild(info CheckpointInfo) bool {
	return info.Class == CheckpointCompatible && info.CanRebuild
}

type FailureClassifier struct{}

func NewFailureClassifier() *FailureClassifier {
	return &FailureClassifier{}
}

func (f *FailureClassifier) Classify(event RuntimeFailureEvent) FailureClass {
	if event.ProcessCrashed {
		return FailureProcessCrash
	}
	switch event.FailureClass {
	case FailureRuntimeStartFailure:
		return FailureRuntimeStartFailure
	case FailureRuntimeRecoveryExhausted:
		return FailureRuntimeRecoveryExhausted
	case FailureUpgradeFailure:
		return FailureUpgradeFailure
	case FailurePackageRollbackRequired:
		return FailurePackageRollbackRequired
	case FailureCheckpointMissing:
		return FailureCheckpointMissing
	case FailureCheckpointStale:
		return FailureCheckpointStale
	case FailureCheckpointCorrupt:
		return FailureCheckpointCorrupt
	case FailureCheckpointIncompatible:
		return FailureCheckpointIncompatible
	default:
		return FailureHostRecoveryFailure
	}
}

func (f *FailureClassifier) DetermineLevel(class FailureClass, restartCount int, maxRestarts int) RecoveryLevel {
	switch class {
	case FailureProcessCrash:
		if restartCount >= maxRestarts {
			return RecoveryLevelRuntimeReconstruction
		}
		return RecoveryLevelProcessRestart
	case FailureRuntimeStartFailure:
		return RecoveryLevelRuntimeReconstruction
	case FailureRuntimeRecoveryExhausted:
		return RecoveryLevelQuarantine
	case FailureUpgradeFailure:
		return RecoveryLevelPackageRollback
	case FailurePackageRollbackRequired:
		return RecoveryLevelPackageRollback
	case FailureCheckpointMissing:
		return RecoveryLevelRuntimeReconstruction
	case FailureCheckpointCorrupt:
		return RecoveryLevelRuntimeReconstruction
	case FailureCheckpointStale:
		return RecoveryLevelRuntimeReconstruction
	case FailureCheckpointIncompatible:
		return RecoveryLevelQuarantine
	default:
		return RecoveryLevelQuarantine
	}
}
