package package_security

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type PackageSecurityService struct {
	inspector *ArchiveInspector
	extractor *SecureExtractor
	hasher    *ContentHasher
	integrity *ManifestBindingVerifier
	signature *SignatureVerifier
	publisher *PublisherTrustService
	staging   *StagingManager
	committer *AtomicCommitter
	snapshot  *SnapshotManager
	rollback  *RollbackCoordinator
	recovery  *RecoveryJournal
	cleanup   *CleanupManager
	audit     AuditWriter
	policy    ArchivePolicy
}

func NewPackageSecurityService(policy ArchivePolicy, auditWriter AuditWriter) *PackageSecurityService {
	baseDir := "."
	recoveryJournal := NewRecoveryJournal()
	stagingMgr := NewStagingManager(baseDir + "/tmp/staging")
	snapshotMgr := NewSnapshotManager(baseDir + "/tmp/snapshots")

	return &PackageSecurityService{
		inspector: NewArchiveInspector(policy),
		extractor: NewSecureExtractor(policy),
		hasher:    NewContentHasher(),
		integrity: NewManifestBindingVerifier(),
		signature: NewSignatureVerifier(),
		publisher: NewPublisherTrustService(),
		staging:   stagingMgr,
		committer: NewAtomicCommitter(recoveryJournal),
		snapshot:  snapshotMgr,
		rollback:  NewRollbackCoordinator(snapshotMgr),
		recovery:  recoveryJournal,
		cleanup:   NewCleanupManager(stagingMgr, snapshotMgr),
		audit:     auditWriter,
		policy:    policy,
	}
}

func (s *PackageSecurityService) Inspect(ctx context.Context, raw []byte, source PackageSource) (*PackageSecurityReport, error) {
	report := &PackageSecurityReport{
		ReportID:   "sec_" + uuid.NewString(),
		SourceType: string(source.SourceType),
		Passed:     true,
		CreatedAt:  time.Now(),
	}

	report.ArchiveHash = s.hasher.HashArchive(raw)

	inspectionResult, err := s.inspector.Inspect(ctx, raw)
	if err != nil {
		report.Passed = false
		report.AddPathIssue("", err.Error(), SeverityCritical, true)
		s.auditEvent(ctx, AuditPackageReject, report, err.Error())
		return report, nil
	}

	report.EntryCount = inspectionResult.EntryCount
	report.TotalCompressed = inspectionResult.TotalCompressed
	report.TotalUncompressed = inspectionResult.TotalUncompressed
	report.CompressionRatio = inspectionResult.CompressionRatio

	for _, collision := range inspectionResult.PathCollisions {
		report.AddPathIssue(collision.PathA, collision.Reason, SeverityCritical, true)
	}

	for _, errMsg := range inspectionResult.Errors {
		report.AddPathIssue("", errMsg, SeverityCritical, true)
	}

	for _, warning := range inspectionResult.Warnings {
		report.AddWarning(warning)
	}

	if !inspectionResult.Passed {
		s.auditEvent(ctx, AuditPackageReject, report, "archive inspection failed")
		return report, nil
	}

	s.auditEvent(ctx, AuditPackageInspect, report, "inspection passed")
	return report, nil
}

func (s *PackageSecurityService) ExtractToStaging(ctx context.Context, raw []byte, report *PackageSecurityReport, purpose string) (*StagingArea, error) {
	staging, err := s.staging.Create(ctx, purpose)
	if err != nil {
		return nil, err
	}

	_, err = s.extractor.Extract(ctx, raw, staging.Path)
	if err != nil {
		s.staging.Cleanup(ctx, staging.ID)
		return nil, err
	}

	s.staging.MarkPopulated(ctx, staging.ID)

	if err := s.staging.Seal(ctx, staging.ID); err != nil {
		s.staging.Cleanup(ctx, staging.ID)
		return nil, err
	}

	return staging, nil
}

func (s *PackageSecurityService) Commit(ctx context.Context, staging *StagingArea, targetPath, packageID, version string) (*CommitResult, error) {
	if err := s.staging.Verify(ctx, staging.ID); err != nil {
		s.auditEvent(ctx, AuditHashMismatch, nil, "staging verification failed: "+staging.ID)
		return nil, err
	}

	snapshot, err := s.rollback.Prepare(ctx, RollbackPrepareRequest{
		PackageID:  packageID,
		Version:    version,
		TargetPath: targetPath,
	})
	if err == nil {
		_ = s.snapshot.Retain(ctx, snapshot.SnapshotID)
	}

	result, err := s.committer.Commit(ctx, staging, CommitRequest{
		StagingID:  staging.ID,
		TargetPath: targetPath,
		PackageID:  packageID,
		Version:    version,
	})

	if err == nil && result != nil && result.Success {
		s.auditEvent(ctx, AuditCommit, nil, "commit succeeded: "+packageID+"@"+version)
		s.staging.Cleanup(ctx, staging.ID)
	}

	return result, err
}

func (s *PackageSecurityService) Rollback(ctx context.Context, snapshotID string, targetPath string) (*RollbackResult, error) {
	result := s.rollback.Restore(ctx, snapshotID, targetPath)
	if !result.Success {
		s.auditEvent(ctx, AuditRollback, nil, "rollback failed: "+snapshotID)
		return result, ErrRollbackFailed
	}

	s.auditEvent(ctx, AuditRollback, nil, "rollback succeeded: "+snapshotID)
	return result, nil
}

func (s *PackageSecurityService) GetRecoveryJournal() *RecoveryJournal {
	return s.recovery
}

func (s *PackageSecurityService) GetCleanupManager() *CleanupManager {
	return s.cleanup
}

func (s *PackageSecurityService) GetStagingManager() *StagingManager {
	return s.staging
}

func (s *PackageSecurityService) GetSignatureVerifier() *SignatureVerifier {
	return s.signature
}

func (s *PackageSecurityService) GetPublisherTrustService() *PublisherTrustService {
	return s.publisher
}

func (s *PackageSecurityService) GetHasher() *ContentHasher {
	return s.hasher
}

func (s *PackageSecurityService) GetIntegrity() *ManifestBindingVerifier {
	return s.integrity
}

func (s *PackageSecurityService) GetArchiveInspector() *ArchiveInspector {
	return s.inspector
}

func (s *PackageSecurityService) auditEvent(ctx context.Context, eventType AuditEventType, report *PackageSecurityReport, details string) {
	event := ResourceAuditEvent{
		EventID:   uuid.NewString(),
		EventType: eventType,
		Details:   details,
		CreatedAt: time.Now(),
	}

	if report != nil {
		event.ReportID = report.ReportID
	}

	_ = s.audit.WriteAuditEvent(ctx, event)
}
