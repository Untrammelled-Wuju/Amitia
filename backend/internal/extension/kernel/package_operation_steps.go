package kernel

const (
	RecoveryStepVerifySideEffects       = "recovery.verify_side_effects"
	RecoveryStepEnsureVersionRecord     = "recovery.ensure_version_record"
	RecoveryStepEnsureArtifactMetadata  = "recovery.ensure_artifact_metadata"
	RecoveryStepEnsureInstallationRef   = "recovery.ensure_installation_reference"
	RecoveryStepConsumePreview          = "recovery.consume_preview"
	RecoveryStepReleasePreviewReference = "recovery.release_preview_reference"
	RecoveryStepReleaseOperationRef     = "recovery.release_operation_reference"
	RecoveryStepRunFinalGate            = "recovery.run_final_gate"
	RecoveryStepFinalizeOperation       = "recovery.finalize_operation"
)

const (
	recoveryStepOrderVerifySideEffects       = 800
	recoveryStepOrderEnsureVersionRecord     = 801
	recoveryStepOrderEnsureArtifactMetadata  = 802
	recoveryStepOrderEnsureInstallationRef   = 803
	recoveryStepOrderConsumePreview          = 804
	recoveryStepOrderReleasePreviewReference = 805
	recoveryStepOrderReleaseOperationRef     = 806
	recoveryStepOrderRunFinalGate            = 807
	recoveryStepOrderFinalizeOperation       = 808
)

type RecoveryRepairPlan struct {
	NeedVersionRecord             bool
	NeedArtifactInstalledPath     bool
	NeedInstallationReference     bool
	NeedPreviewConsume            bool
	NeedPreviewReferenceRelease   bool
	NeedOperationReferenceRelease bool
	NeedFinalGate                 bool
	NeedRollbackPointVerify       bool
	NeedMigrationJournalVerify    bool
	NeedOldVersionRetained        bool
	NeedUninstallVersionRemoved   bool
}

type RecoveryIdentityEvidence struct {
	ExtensionID       string
	Version           string
	ArtifactID        string
	GenerationID      string
	ManifestHash      string
	ContentTreeHash   string
	ArchiveHash       string
	OperationID       string
	InstalledPath     string
	InstalledTreeHash string
}

func (e RecoveryIdentityEvidence) Proven() bool {
	return e.ExtensionID != "" && e.Version != "" && e.ArtifactID != "" &&
		e.GenerationID != "" && e.ManifestHash != "" && e.ContentTreeHash != "" &&
		e.OperationID != ""
}
