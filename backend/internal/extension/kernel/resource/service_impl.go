package resource

import (
	"context"
	"time"
)

type DefaultResourceOwnershipService struct {
	store StorageBackend
}

var _ ResourceOwnershipService = (*DefaultResourceOwnershipService)(nil)

func NewResourceOwnershipService(store StorageBackend) *DefaultResourceOwnershipService {
	return &DefaultResourceOwnershipService{store: store}
}

func (s *DefaultResourceOwnershipService) Register(ctx context.Context, resource *ResourceRecord) error {
	if !resource.ResourceType.IsValid() {
		return ErrInvalidResourceType
	}
	if !resource.Owner.OwnerType.IsValid() {
		return ErrInvalidOwner
	}
	if !resource.State.IsValid() {
		resource.State = StatePending
	}

	if resource.ResourceID == "" {
		resource.ResourceID = NewResourceID()
	}

	now := time.Now()
	if resource.CreatedAt.IsZero() {
		resource.CreatedAt = now
	}
	resource.UpdatedAt = now

	return s.store.SaveResource(ctx, *resource)
}

func (s *DefaultResourceOwnershipService) AddReference(ctx context.Context, ref ResourceReference) error {
	if !ref.ReferenceType.IsValid() {
		return ErrInvalidResourceType
	}

	if _, err := s.store.GetResource(ctx, ref.SourceResourceID); err != nil {
		return err
	}
	if _, err := s.store.GetResource(ctx, ref.TargetResourceID); err != nil {
		return err
	}

	if ref.ReferenceID == "" {
		ref.ReferenceID = NewReferenceID()
	}
	if ref.CreatedAt.IsZero() {
		ref.CreatedAt = time.Now()
	}

	return s.store.SaveReference(ctx, ref)
}

func (s *DefaultResourceOwnershipService) RemoveReference(ctx context.Context, referenceID string) error {
	return s.store.DeleteReference(ctx, referenceID)
}

func (s *DefaultResourceOwnershipService) TransferOwnership(ctx context.Context, request OwnershipTransferRequest) error {
	if !request.Action.IsValid() {
		return ErrTransferNotAllowed
	}

	resource, err := s.store.GetResource(ctx, request.ResourceID)
	if err != nil {
		return err
	}

	if !resource.Owner.Equals(request.FromOwner) {
		return ErrTransferNotAllowed
	}

	if resource.State.IsTerminal() {
		return ErrTransferNotAllowed
	}

	if request.Action == TransferClone {
		cloned := *resource
		if request.CloneID != "" {
			cloned.ResourceID = request.CloneID
		} else {
			cloned.ResourceID = NewResourceID()
		}
		cloned.Owner = request.ToOwner
		cloned.State = StateActive
		cloned.UpdatedAt = time.Now()
		return s.store.SaveResource(ctx, cloned)
	}

	if request.Action == TransferAdopt {
		resource.Owner = request.ToOwner
		resource.UpdatedAt = time.Now()
		return s.store.UpdateResource(ctx, resource.ResourceID, *resource)
	}

	if request.Action == TransferDetach {
		resource.DeleteStrategy = StrategyRetain
		resource.UpdatedAt = time.Now()
		_ = s.store.UpdateResourceState(ctx, resource.ResourceID, StateOrphaned)
		return nil
	}

	return ErrTransferNotAllowed
}

func (s *DefaultResourceOwnershipService) PlanRelease(ctx context.Context, request ResourceReleaseRequest) (*ResourceReleasePlan, error) {
	resource, err := s.store.GetResource(ctx, request.ResourceID)
	if err != nil {
		return nil, err
	}

	if !resource.Owner.Equals(request.RequestedBy) && !request.RequestedBy.IsSystem() {
		return nil, ErrTransferNotAllowed
	}

	plan := &ResourceReleasePlan{
		PlanID:         NewPlanID(),
		RootResourceID: request.ResourceID,
		CreatedAt:      time.Now(),
	}

	s.buildReleaseActions(ctx, resource, plan)

	if request.DryRun {
		return plan, nil
	}

	return plan, nil
}

func (s *DefaultResourceOwnershipService) buildReleaseActions(ctx context.Context, resource *ResourceRecord, plan *ResourceReleasePlan) {
	if resource.State.IsTerminal() {
		return
	}

	strategy := resource.DeleteStrategy
	if strategy == "" {
		strategy = StrategyRetain
	}

	switch strategy {
	case StrategyCascade:
		plan.DeleteResources = append(plan.DeleteResources, ResourceAction{
			ResourceID:   resource.ResourceID,
			ResourceType: resource.ResourceType,
			Action:       "delete",
			Reason:       "cascade_delete",
		})

		refs, _ := s.store.ListReferencesBySource(ctx, resource.ResourceID)
		for _, ref := range refs {
			if ref.OwnershipEffect == EffectCascadeDelete {
				if targetRes, err := s.store.GetResource(ctx, ref.TargetResourceID); err == nil {
					s.buildReleaseActions(ctx, targetRes, plan)
				}
			}
		}
	case StrategyRetain:
		plan.RetainResources = append(plan.RetainResources, ResourceAction{
			ResourceID:   resource.ResourceID,
			ResourceType: resource.ResourceType,
			Action:       "retain",
			Reason:       "strategy_retain",
		})
	case StrategyTransfer:
		plan.TransferResources = append(plan.TransferResources, ResourceAction{
			ResourceID:   resource.ResourceID,
			ResourceType: resource.ResourceType,
			Action:       "transfer",
			Reason:       "strategy_transfer",
		})
	case StrategyPrompt:
		plan.UserDecisions = append(plan.UserDecisions, RequiredUserDecision{
			ResourceID:    resource.ResourceID,
			Question:      "How should this resource be handled?",
			Options:       []string{"delete", "retain", "transfer"},
			DefaultOption: "retain",
		})
	case StrategyBlock:
		plan.Blockers = append(plan.Blockers, ResourceBlocker{
			ResourceID: resource.ResourceID,
			Reason:     "strategy_block",
		})
	case StrategyRebuildable:
		plan.DeleteResources = append(plan.DeleteResources, ResourceAction{
			ResourceID:   resource.ResourceID,
			ResourceType: resource.ResourceType,
			Action:       "delete",
			Reason:       "strategy_rebuildable",
		})
	}
}

func (s *DefaultResourceOwnershipService) ExecuteRelease(ctx context.Context, plan *ResourceReleasePlan) (*ResourceReleaseResult, error) {
	result := &ResourceReleaseResult{
		PlanID:  plan.PlanID,
		Success: true,
	}

	if plan.IsBlocked() {
		result.Success = false
		return result, ErrReleaseBlocked
	}

	for _, action := range plan.DeleteResources {
		if err := s.store.DeleteResource(ctx, action.ResourceID); err != nil {
			result.Errors = append(result.Errors, err.Error())
			result.Success = false
			continue
		}
		result.DeletedCount++
	}

	for _ = range plan.TransferResources {
		result.TransferredCount++
	}

	result.RetainedCount = len(plan.RetainResources)
	result.CompletedAt = time.Now()

	return result, nil
}

func (s *DefaultResourceOwnershipService) ListOwned(ctx context.Context, owner ResourceOwner) ([]ResourceRecord, error) {
	return s.store.ListResourcesByOwner(ctx, owner)
}

func (s *DefaultResourceOwnershipService) ListReferences(ctx context.Context, resourceID string) ([]ResourceReference, error) {
	return s.store.ListAllReferences(ctx, resourceID)
}

func (s *DefaultResourceOwnershipService) GetResource(ctx context.Context, resourceID string) (*ResourceRecord, error) {
	return s.store.GetResource(ctx, resourceID)
}

func (s *DefaultResourceOwnershipService) UpdateState(ctx context.Context, resourceID string, state ResourceState) error {
	return s.store.UpdateResourceState(ctx, resourceID, state)
}

func (s *DefaultResourceOwnershipService) ScanOrphans(ctx context.Context) (*OrphanReport, error) {
	report := &OrphanReport{
		ReportID:  NewReportID(),
		CreatedAt: time.Now(),
	}

	allRefs := s.collectAllRefs(ctx)

	resources, err := s.store.ListResourcesByType(ctx, "")
	if err != nil {
		return nil, err
	}

	for _, res := range resources {
		if res.Owner.IsTemporary() && res.IsExpired() {
			report.Entries = append(report.Entries, OrphanEntry{
				Kind:         OrphanKindDatabaseRecord,
				ResourceID:   res.ResourceID,
				ResourceType: res.ResourceType,
				Identifier:   res.ResourceID,
				Description:  "expired temporary resource",
				Risk:         "low",
				CanAutoClean: true,
				DetectedAt:   time.Now(),
			})
		}
	}

	for _, ref := range allRefs {
		if _, err := s.store.GetResource(ctx, ref.TargetResourceID); err != nil {
			report.Entries = append(report.Entries, OrphanEntry{
				Kind:         OrphanKindReference,
				ResourceID:   ref.SourceResourceID,
				Identifier:   ref.ReferenceID,
				Description:  "dangling reference to " + ref.TargetResourceID,
				Risk:         "medium",
				CanAutoClean: false,
				DetectedAt:   time.Now(),
			})
		}
	}

	report.TotalCount = len(report.Entries)
	for _, entry := range report.Entries {
		if entry.Risk == "high" {
			report.HighRisk++
		}
		if entry.CanAutoClean {
			report.AutoClean++
		}
	}

	return report, nil
}

func (s *DefaultResourceOwnershipService) collectAllRefs(ctx context.Context) []ResourceReference {
	return nil
}
