package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func (r *Runtime) completePackageGenerationStep(ctx context.Context, operationID, name string, order int, stable, target PackageGenerationCurrent, result string, guard PackageWriteGuard) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := r.container.PackageRepository.PutStep(ctx, PackageOperationStep{StepID: "package-step-" + uuid.NewString(), OperationID: operationID, StepName: name, StepOrder: order, Status: "completed", AttemptCount: 1, ResultJSON: result, StartedAt: now, CompletedAt: now, StableGeneration: stable.GenerationID, TargetGeneration: target.GenerationID, CurrentPointerJSON: packageGenerationJSON(target)}, guard); err != nil {
		return err
	}
	return r.container.PackageRepository.SetOperation(ctx, operationID, "in_progress", name, "", "", false, guard)
}

func packageGenerationFromInstallation(installation domain.ExtensionInstallation) PackageGenerationCurrent {
	metadata := installation.Metadata
	if metadata == nil {
		return PackageGenerationCurrent{}
	}
	generationID, _ := metadata["generationId"].(string)
	path, _ := metadata["currentPath"].(string)
	_ = path
	treeHash, _ := metadata["installedTreeHash"].(string)
	operationID, _ := metadata["lastOperationId"].(string)
	artifactID, _ := metadata["artifactId"].(string)
	fencingToken, _ := metadata["fencingToken"].(float64)
	return PackageGenerationCurrent{ExtensionID: string(installation.ExtensionID), GenerationID: generationID, Version: installation.InstalledVersion.String(), ArtifactID: artifactID, TreeHash: treeHash, OperationID: operationID, FencingToken: int64(fencingToken), UpdatedAt: installation.UpdatedAt}
}

func packageGenerationJSON(current PackageGenerationCurrent) string {
	if current.GenerationID == "" {
		return "{}"
	}
	data, err := json.Marshal(current)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func (r *Runtime) preparePackageGeneration(ctx context.Context, operationID string, artifact PackageArtifact, sourcePath string, fencingToken int64) (PackagePreparedGeneration, PackageGenerationCurrent, error) {
	if r.container == nil || r.container.PackageGenerationStore == nil {
		return PackagePreparedGeneration{}, PackageGenerationCurrent{}, fmt.Errorf("kernel: package generation store unavailable")
	}
	stable, err := r.container.PackageGenerationStore.ReadCurrent(artifact.ExtensionID)
	if err != nil && !errors.Is(err, ErrPackageGenerationNotFound) {
		return PackagePreparedGeneration{}, PackageGenerationCurrent{}, err
	}
	if errors.Is(err, ErrPackageGenerationNotFound) {
		stable = PackageGenerationCurrent{}
	}
	prepared, err := r.container.PackageGenerationStore.PrepareGeneration(ctx, PackageGenerationPrepareRequest{
		ExtensionID:      artifact.ExtensionID,
		GenerationID:     "generation-" + uuid.NewString(),
		Version:          artifact.Version,
		ArtifactID:       artifact.ArtifactID,
		OperationID:      operationID,
		SourcePath:       sourcePath,
		FencingToken:     fencingToken,
	})
	if err != nil {
		return PackagePreparedGeneration{}, stable, err
	}
	committed, err := r.container.PackageGenerationStore.CommitGeneration(ctx, prepared)
	if err != nil {
		return PackagePreparedGeneration{}, stable, err
	}
	return committed, stable, nil
}

func (r *Runtime) switchPackageGeneration(ctx context.Context, stable PackageGenerationCurrent, target PackagePreparedGeneration) error {
	expected := stable.GenerationID
	return r.container.PackageGenerationStore.SwitchCurrent(target.Current.ExtensionID, expected, target.Current)
}

func (r *Runtime) compensatePackageGeneration(ctx context.Context, stable PackageGenerationCurrent, target PackagePreparedGeneration, switched bool) error {
	if target.Current.GenerationID == "" || r.container == nil || r.container.PackageGenerationStore == nil {
		return nil
	}
	var failures []error
	if switched {
		restored, err := r.container.PackageGenerationStore.RestoreCurrent(target.Current.ExtensionID, target.Current.GenerationID)
		if err != nil {
			failures = append(failures, fmt.Errorf("restore current pointer: %w", err))
		} else if stable.GenerationID != "" && restored.GenerationID != stable.GenerationID {
			failures = append(failures, fmt.Errorf("restored current pointer mismatch"))
		}
	}
	if len(failures) == 0 {
		if _, err := r.container.PackageGenerationStore.QuarantineGeneration(ctx, target.Current); err != nil {
			failures = append(failures, fmt.Errorf("quarantine target generation: %w", err))
		}
	}
	return errors.Join(failures...)
}

func packageInstallationMetadata(base map[string]any, current PackageGenerationCurrent, path, operationID string) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	base["generationId"] = current.GenerationID
	base["currentPath"] = path
	base["installedPath"] = path
	base["installedTreeHash"] = current.TreeHash
	base["lastOperationId"] = operationID
	base["fencingToken"] = current.FencingToken
	base["currentUpdatedAt"] = time.Now().UTC().Format(time.RFC3339Nano)
	return base
}

func (r *Runtime) rebindPackageInstallationGeneration(ctx context.Context, current PackageGenerationCurrent, path string) error {
	if current.GenerationID == "" {
		return nil
	}
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(current.ExtensionID))
	if err != nil {
		return err
	}
	installation.Metadata = packageInstallationMetadata(installation.Metadata, current, path, current.OperationID)
	installation.UpdatedAt = time.Now().UTC()
	return r.container.InstallationRepository.PutInstallation(ctx, installation)
}
