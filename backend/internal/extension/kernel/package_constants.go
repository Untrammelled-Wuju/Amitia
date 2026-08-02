package kernel

const (
	StepValidatePreviewSession        = "install.validate_preview_session"
	StepReverifyArtifactHash          = "install.reverify_artifact_hash"
	StepExtractToStaging              = "install.extract_to_staging"
	StepBuildCandidateDefinitions     = "install.build_candidate_definitions"
	StepCreateRollbackPoint           = "update.create_rollback_point"
	StepCommitInstalledTree           = "install.commit_installed_tree"
	StepCommitTargetGeneration        = "update.commit_target_generation"
	StepSwitchCurrentPointer          = "switch_current_pointer"
	StepInstallSwitchCurrentPointer   = "install.switch_current_pointer"
	StepUpdateSwitchCurrentPointer    = "update.switch_current_pointer"
	StepCommitKernelRepositories      = "install.commit_kernel_repositories"
	StepCommitUpdateState             = "update.commit_update_state"
	StepMarkInstallationDisabled      = "install.mark_installation_disabled"
	StepExecuteMigrations             = "update.execute_migrations"
	StepValidateAndDiff               = "update.validate_and_diff"
	StepValidateRollbackPoint         = "validate_rollback_point"
	StepCommitRollbackGeneration      = "commit_rollback_generation"
	StepRestoreRepositories           = "restore_repositories"
	StepRestoreMigrationState         = "restore_migration_state"
	StepPersistArtifactMetadata       = "persist_artifact_metadata"
	StepValidateUninstallPreflight    = "validate_uninstall_preflight"
	StepMoveToQuarantine              = "move_to_quarantine"
	StepCleanupKernelRepositories     = "cleanup_kernel_repositories"
	StepFinalizeQuarantine            = "finalize_quarantine"
	StepCompleteOperation             = "complete_operation"
	StepAcquireLease                  = "acquire_lease"
	StepLockPreviewSession            = "lock_preview_session"
	StepCheckAlreadyInstalled         = "check_already_installed"
	StepCheckInstalled                = "check_installed"
	StepValidateOwner                 = "validate_owner"
	StepValidateExtensionID           = "validate_extension_id"
	StepValidateTargetVersion         = "validate_target_version"
	StepRecheckPreflight              = "recheck_preflight"
	StepVerifyStagingTree             = "verify_staging_tree"
	StepValidateCurrentPointer        = "validate_current_pointer"
	StepPersistGenerationEvidence     = "persist_generation_evidence"
	StepConsumePreviewSession         = "consume_preview_session"
	StepBindCurrentPointer            = "bind_current_pointer"
	StepQuarantineCurrentPointer      = "quarantine_current_pointer"
	StepClearArtifactInstallationPath = "clear_artifact_installation_path"
	StepRemoveFiles                   = "remove_files"
	StepRemoveCurrentQuarantine       = "remove_current_quarantine"
	StepRecordRestoreCompletion       = "record_restore_completion"
)

const (
	StepUninstallRecoveryLoadQuarantineMetadata    = "uninstall_recovery.load_quarantine_metadata"
	StepUninstallRecoveryVerifyQuarantineMetadata  = "uninstall_recovery.verify_quarantine_metadata"
	StepUninstallRecoveryRestoreGeneration         = "uninstall_recovery.restore_generation"
	StepUninstallRecoveryRestoreCurrent            = "uninstall_recovery.restore_current"
	StepUninstallRecoveryRestoreInstallation       = "uninstall_recovery.restore_installation"
	StepUninstallRecoveryRestoreVersionState       = "uninstall_recovery.restore_version_state"
	StepUninstallRecoveryRestoreArtifactPath       = "uninstall_recovery.restore_artifact_path"
	StepUninstallRecoveryRestoreArtifactReference  = "uninstall_recovery.restore_artifact_reference"
	StepUninstallRecoveryVerifyRestoredState       = "uninstall_recovery.verify_restored_state"
	StepUninstallRecoveryReleaseQuarantineMetadata = "uninstall_recovery.release_quarantine_metadata"
	StepUninstallRecoveryFinalGate                 = "uninstall_recovery.final_gate"
	StepUninstallRecoveryFinalize                  = "uninstall_recovery.finalize"
	StepRemoveArtifact                              = "uninstall.remove_artifact"
)

var legacyStepNameMap = map[string]string{
	"commit_installed_tree":      StepCommitInstalledTree,
	"commit_target_generation":   StepCommitTargetGeneration,
	"switch_current_pointer":     StepSwitchCurrentPointer,
	"commit_kernel_repositories": StepCommitKernelRepositories,
	"commit_update_state":        StepCommitUpdateState,
	"create_rollback_point":      StepCreateRollbackPoint,
	"execute_migrations":         StepExecuteMigrations,
	"validate_and_diff":          StepValidateAndDiff,
}

func NormalizePackageStepName(name string) string {
	if normalized, ok := legacyStepNameMap[name]; ok {
		return normalized
	}
	return name
}

const (
	OpInstall   = "install"
	OpUpdate    = "update"
	OpRollback  = "rollback"
	OpUninstall = "uninstall"
)

const (
	StatusCreated          = "created"
	StatusInProgress       = "in_progress"
	StatusCompleted        = "completed"
	StatusFailed           = "failed"
	StatusCompensating     = "compensating"
	StatusRequiresRecovery = "requires_recovery"
)

const (
	ErrCodePackageInstallFailed             = "PACKAGE_INSTALL_FAILED"
	ErrCodePackageUpdateFailed              = "PACKAGE_UPDATE_FAILED"
	ErrCodePackageRollbackFailed            = "PACKAGE_ROLLBACK_FAILED"
	ErrCodePackageUninstallFailed           = "PACKAGE_UNINSTALL_FAILED"
	ErrCodePackageRecoveryRequired          = "PACKAGE_RECOVERY_REQUIRED"
	ErrCodePackageManualRecoveryRequired    = "PACKAGE_MANUAL_RECOVERY_REQUIRED"
	ErrCodePackageOperationLeaseConflict    = "PACKAGE_OPERATION_LEASE_CONFLICT"
	ErrCodePackagePreviewSessionLockFailed  = "PACKAGE_PREVIEW_SESSION_LOCK_FAILED"
	ErrCodePackagePreviewSessionStatus      = "PACKAGE_PREVIEW_SESSION_STATUS"
	ErrCodePackageAlreadyInstalled          = "PACKAGE_ALREADY_INSTALLED"
	ErrCodePackageNotInstalled              = "PACKAGE_NOT_INSTALLED"
	ErrCodePackageOwnerMismatch             = "PACKAGE_OWNER_MISMATCH"
	ErrCodePackageUpdateIDMismatch          = "PACKAGE_UPDATE_ID_MISMATCH"
	ErrCodePackageUpdateTargetUnchanged     = "PACKAGE_UPDATE_TARGET_UNCHANGED"
	ErrCodePackageUninstallPreflightFailed  = "PACKAGE_UNINSTALL_PREFLIGHT_FAILED"
	ErrCodePackageUninstallPreflightChanged = "PACKAGE_UNINSTALL_PREFLIGHT_CHANGED"
	ErrCodePackageForwardRecoveryFailed     = "PACKAGE_FORWARD_RECOVERY_FAILED"
)
