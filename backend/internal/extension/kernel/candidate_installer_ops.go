package kernel

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func (i *TypedContributionInstaller) RegisterCandidateContributions(
	ctx context.Context,
	candidateID string,
	extID domain.ExtensionID,
	instanceIDs []string,
	generationID string,
	generation int64,
	contribs []domain.ContributionDefinition,
	definitionHash string,
	artifactPath string,
) error {
	if i == nil || i.container == nil {
		return fmt.Errorf("contribution-installer: container not attached")
	}
	if i.candidateNS == nil {
		return fmt.Errorf("contribution-installer: candidate namespace not configured")
	}
	if candidateID == "" {
		return fmt.Errorf("contribution-installer: candidateID is required")
	}

	for _, contrib := range contribs {
		if _, err := i.buildInstallOp(ctx, contrib, generation); err != nil {
			return fmt.Errorf("contribution-installer: validate candidate contribution %s: %w", contrib.ID, err)
		}
	}

	keys := make([]CandidateKey, 0, len(contribs))
	for _, contrib := range contribs {
		keys = append(keys, CandidateKey{
			ExtensionID:         string(contrib.ExtensionID),
			ModuleID:            string(contrib.ModuleID),
			CandidateGeneration: generation,
			ContributionID:      string(contrib.ID),
			DefinitionHash:      definitionHash,
		})
	}

	entry := &CandidateNamespaceEntry{
		CandidateID:    candidateID,
		ExtensionID:    extID,
		InstanceIDs:    instanceIDs,
		GenerationID:   generationID,
		Generation:     generation,
		Keys:           keys,
		Contribs:       contribs,
		DefinitionHash: definitionHash,
		ArtifactPath:   artifactPath,
		RegisteredAt:   time.Now().UTC(),
	}

	if err := i.candidateNS.Store(entry); err != nil {
		return fmt.Errorf("contribution-installer: store candidate %s in namespace: %w", candidateID, err)
	}

	log.Printf("[contribution-installer] registered %d candidate contributions for %s in isolated namespace (candidate=%s, generation=%d)",
		len(contribs), extID, candidateID, generation)
	return nil
}

func (i *TypedContributionInstaller) ValidateCandidateContributions(ctx context.Context, candidateID string) error {
	if i == nil || i.container == nil {
		return fmt.Errorf("contribution-installer: container not attached")
	}
	if i.candidateNS == nil {
		return fmt.Errorf("contribution-installer: candidate namespace not configured")
	}

	entry, ok := i.candidateNS.Load(candidateID)
	if !ok {
		return fmt.Errorf("contribution-installer: candidate %s not found in namespace", candidateID)
	}

	for _, contrib := range entry.Contribs {
		if _, err := i.buildInstallOp(ctx, contrib, entry.Generation); err != nil {
			return fmt.Errorf("contribution-installer: validate candidate contribution %s: %w", contrib.ID, err)
		}
	}

	if err := i.candidateNS.MarkValidated(candidateID); err != nil {
		return fmt.Errorf("contribution-installer: mark candidate %s validated: %w", candidateID, err)
	}

	log.Printf("[contribution-installer] validated %d candidate contributions for %s (candidate=%s)",
		len(entry.Contribs), entry.ExtensionID, candidateID)
	return nil
}

func (i *TypedContributionInstaller) PromoteCandidateContributions(ctx context.Context, candidateID string) ([]string, error) {
	if i == nil || i.container == nil {
		return nil, fmt.Errorf("contribution-installer: container not attached")
	}
	if i.candidateNS == nil {
		return nil, fmt.Errorf("contribution-installer: candidate namespace not configured")
	}

	entry, ok := i.candidateNS.Load(candidateID)
	if !ok {
		return nil, fmt.Errorf("contribution-installer: candidate %s not found in namespace", candidateID)
	}

	if !i.candidateNS.IsValidated(candidateID) {
		return nil, fmt.Errorf("contribution-installer: candidate %s has not been validated", candidateID)
	}

	beforeScheduleIDs, _ := i.ListScheduleIDs(ctx, entry.ExtensionID)

	if err := i.InstallContributions(ctx, entry.Contribs, entry.Generation); err != nil {
		return nil, fmt.Errorf("contribution-installer: promote install for candidate %s: %w", candidateID, err)
	}

	afterScheduleIDs, _ := i.ListScheduleIDs(ctx, entry.ExtensionID)
	beforeSet := make(map[string]bool, len(beforeScheduleIDs))
	for _, id := range beforeScheduleIDs {
		beforeSet[id] = true
	}
	newScheduleIDs := make([]string, 0)
	for _, id := range afterScheduleIDs {
		if !beforeSet[id] {
			newScheduleIDs = append(newScheduleIDs, id)
		}
	}

	log.Printf("[contribution-installer] promoted %d candidate contributions to production for %s (candidate=%s, generation=%d, newSchedules=%d) - namespace preserved for deferred cleanup",
		len(entry.Contribs), entry.ExtensionID, candidateID, entry.Generation, len(newScheduleIDs))
	return newScheduleIDs, nil
}

func (i *TypedContributionInstaller) RemoveCandidateNamespaceAfterCommit(ctx context.Context, candidateID string) error {
	if i == nil {
		return fmt.Errorf("contribution-installer: not initialized")
	}
	if i.candidateNS == nil {
		return nil
	}

	entry, ok := i.candidateNS.Load(candidateID)
	if !ok {
		return nil
	}

	i.candidateNS.Remove(candidateID)

	log.Printf("[contribution-installer] removed candidate %s from namespace after commit (ext=%s, contribs=%d)",
		candidateID, entry.ExtensionID, len(entry.Contribs))
	return nil
}

func (i *TypedContributionInstaller) DiscardCandidateNamespace(ctx context.Context, candidateID string) error {
	if i == nil {
		return fmt.Errorf("contribution-installer: not initialized")
	}
	if i.candidateNS == nil {
		return nil
	}

	entry, ok := i.candidateNS.Load(candidateID)
	if !ok {
		return nil
	}

	i.candidateNS.Remove(candidateID)

	log.Printf("[contribution-installer] discarded candidate %s from namespace (ext=%s, contribs=%d) - production untouched",
		candidateID, entry.ExtensionID, len(entry.Contribs))
	return nil
}

func (i *TypedContributionInstaller) IsCandidateRegistered(candidateID string) bool {
	if i == nil || i.candidateNS == nil {
		return false
	}
	return i.candidateNS.HasCandidate(candidateID)
}

func (i *TypedContributionInstaller) IsCandidateValidated(candidateID string) bool {
	if i == nil || i.candidateNS == nil {
		return false
	}
	return i.candidateNS.IsValidated(candidateID)
}

func (i *TypedContributionInstaller) ListNamespaceCandidates() []*CandidateNamespaceEntry {
	if i == nil || i.candidateNS == nil {
		return nil
	}
	return i.candidateNS.ListAll()
}
