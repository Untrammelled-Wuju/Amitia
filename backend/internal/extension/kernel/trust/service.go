package trust

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type TrustService struct {
	store             *PublisherStore
	verifier          *SignatureVerifier
	rotator           *KeyRotator
	rotationLog       *RotationLog
	revocationList    *RevocationList
	blocklist         *PackageBlocklist
	userTrust         *UserTrustStore
	policyEngine      *LifecyclePolicyEngine
	continuityChecker *UpdateContinuityChecker
	transferLog       *OwnershipTransferLog
	development       *DevelopmentTrustManager
	quarantine        *QuarantineManager
}

type TrustServiceConfig struct {
	BuiltinRoots []PublisherIdentity
}

func NewTrustService(cfg TrustServiceConfig) *TrustService {
	store := NewPublisherStore()
	for _, root := range cfg.BuiltinRoots {
		_ = store.RegisterBuiltinRoot(root.PublisherID, root)
	}
	userTrust := NewUserTrustStore()
	revocationList := NewRevocationList("local")
	blocklist := NewPackageBlocklist()
	transferLog := NewOwnershipTransferLog()
	verifier := NewSignatureVerifier(store)
	rotator := NewKeyRotator(store)
	rotationLog := NewRotationLog()
	policyEngine := NewLifecyclePolicyEngine(store, userTrust, revocationList, blocklist)
	continuityChecker := NewUpdateContinuityChecker(store, transferLog)
	development := NewDevelopmentTrustManager(store, userTrust)
	quarantine := NewQuarantineManager()

	return &TrustService{
		store:             store,
		verifier:          verifier,
		rotator:           rotator,
		rotationLog:       rotationLog,
		revocationList:    revocationList,
		blocklist:         blocklist,
		userTrust:         userTrust,
		policyEngine:      policyEngine,
		continuityChecker: continuityChecker,
		transferLog:       transferLog,
		development:       development,
		quarantine:        quarantine,
	}
}

func (s *TrustService) Store() *PublisherStore                      { return s.store }
func (s *TrustService) Verifier() *SignatureVerifier                { return s.verifier }
func (s *TrustService) Rotator() *KeyRotator                        { return s.rotator }
func (s *TrustService) RotationLog() *RotationLog                   { return s.rotationLog }
func (s *TrustService) RevocationList() *RevocationList             { return s.revocationList }
func (s *TrustService) Blocklist() *PackageBlocklist                { return s.blocklist }
func (s *TrustService) UserTrust() *UserTrustStore                  { return s.userTrust }
func (s *TrustService) PolicyEngine() *LifecyclePolicyEngine        { return s.policyEngine }
func (s *TrustService) ContinuityChecker() *UpdateContinuityChecker { return s.continuityChecker }
func (s *TrustService) TransferLog() *OwnershipTransferLog          { return s.transferLog }
func (s *TrustService) Development() *DevelopmentTrustManager       { return s.development }
func (s *TrustService) Quarantine() *QuarantineManager              { return s.quarantine }

type VerifyRequest struct {
	Document              SignatureDocument
	Payload               SignaturePayload
	ActualManifestHash    string
	ActualContentTreeHash string
	ActualPackageHash     string
}

type VerifyResponse struct {
	Signature SignatureVerificationResult
	Trust     PolicyResult
}

func (s *TrustService) VerifyAndEvaluate(ctx context.Context, req VerifyRequest, policyInput PolicyInput) VerifyResponse {
	sigResult := s.verifier.Verify(ctx, VerifyInput{
		Document:              req.Document,
		ActualPayload:         req.Payload,
		ActualManifestHash:    req.ActualManifestHash,
		ActualContentTreeHash: req.ActualContentTreeHash,
		ActualPackageHash:     req.ActualPackageHash,
	})
	policyInput.SignatureResult = sigResult
	policyInput.PublisherID = req.Document.PublisherID
	policyInput.KeyID = req.Document.KeyID
	policyInput.PackageHash = req.ActualPackageHash
	policyResult := s.policyEngine.Evaluate(ctx, policyInput)

	if policyResult.Action == LifecycleActionQuarantine {
		s.quarantine.Quarantine(QuarantineEntry{
			ExtensionID: policyInput.ExtensionID,
			Version:     policyInput.Version,
			PackageHash: req.ActualPackageHash,
			PublisherID: req.Document.PublisherID,
			Reason:      policyResult.QuarantineReason,
			Severity:    string(policyResult.TrustLevel),
		})
	}

	return VerifyResponse{
		Signature: sigResult,
		Trust:     policyResult,
	}
}

func (s *TrustService) EvaluateAutoUpdate(ctx context.Context, input PolicyInput) AutoUpdateEligibility {
	return s.policyEngine.EvaluateAutoUpdate(ctx, input)
}

func (s *TrustService) RotateKey(ctx context.Context, req RotationRequest) (RotationResult, error) {
	result := s.rotator.Rotate(ctx, req)
	if !result.Success {
		return result, fmt.Errorf("trust: rotation failed: %s", result.Reason)
	}
	s.rotationLog.Append(RotationRecord{
		PublisherID:        req.PublisherID,
		OldKeyID:           req.OldKeyID,
		NewKeyID:           req.NewKeyID,
		Reason:             req.Reason,
		ContinuityVerified: len(req.ContinuitySignature) > 0,
	})
	return result, nil
}

func (s *TrustService) RevokePackage(ctx context.Context, entry RevocationEntry) error {
	if err := s.revocationList.Add(entry); err != nil {
		return err
	}
	quarantined := s.quarantine.List()
	for _, q := range quarantined {
		if q.PackageHash == entry.PackageHash {
			updated := q
			updated.Reason = entry.Reason
			updated.Severity = string(entry.Severity)
			s.quarantine.Quarantine(updated)
		}
	}
	return nil
}

func (s *TrustService) BlockPackage(ctx context.Context, entry PackageBlockEntry) error {
	return s.blocklist.Block(entry)
}

func (s *TrustService) UnblockPackage(ctx context.Context, packageHash string) error {
	return s.blocklist.Unblock(packageHash)
}

func (s *TrustService) AuthorizeOwnershipTransfer(ctx context.Context, req OwnershipTransferRequest) (OwnershipTransferResult, error) {
	return s.continuityChecker.AuthorizeTransfer(ctx, req)
}

func (s *TrustService) RegisterDevelopmentWorkspace(ctx context.Context, req RegisterWorkspaceRequest) error {
	return s.development.Register(ctx, req)
}

func (s *TrustService) RevokeDevelopmentWorkspace(ctx context.Context, workspacePath, reason string) error {
	return s.development.Revoke(ctx, workspacePath, reason)
}

func (s *TrustService) SyncRevocations(ctx context.Context, remote *RevocationList) int {
	return s.revocationList.Merge(ctx, remote)
}

func (s *TrustService) SyncBlocklist(ctx context.Context, remote *PackageBlocklist) int {
	return s.blocklist.Merge(ctx, remote)
}

func (s *TrustService) CheckUpdateContinuity(ctx context.Context, input UpdateContinuityCheck) UpdateContinuityResult {
	return s.continuityChecker.Check(ctx, input)
}

func (s *TrustService) Snapshot() TrustSnapshot {
	return TrustSnapshot{
		Publishers:    s.store.List(context.Background()),
		Revocations:   s.revocationList.Snapshot(),
		Blocklist:     s.blocklist.Snapshot(),
		UserDecisions: s.userTrust.Snapshot(),
		Timestamp:     time.Now().UTC(),
	}
}

type TrustSnapshot struct {
	Publishers    []PublisherIdentity
	Revocations   []RevocationEntry
	Blocklist     []PackageBlockEntry
	UserDecisions []UserTrustDecision
	Timestamp     time.Time
}

func (s *TrustService) Restore(snapshot TrustSnapshot) error {
	if snapshot.Timestamp.IsZero() {
		return errors.New("trust: invalid snapshot timestamp")
	}
	for _, identity := range snapshot.Publishers {
		if identity.Source == TrustSourceBuiltin {
			_ = s.store.RegisterBuiltinRoot(identity.PublisherID, identity)
		} else if identity.Source == TrustSourceOfficialFeed {
			_ = s.store.RegisterFromOfficialFeed(identity)
		} else if identity.Source == TrustSourceUserDecision {
			_ = s.store.RegisterUserDecision(identity)
		}
	}
	s.revocationList.Restore(snapshot.Revocations)
	s.blocklist.Restore(snapshot.Blocklist)
	s.userTrust.Restore(snapshot.UserDecisions)
	return nil
}

var (
	ErrInvalidTrustSnapshot = errors.New("trust: invalid snapshot")
)
