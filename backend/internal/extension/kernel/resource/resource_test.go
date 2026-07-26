package resource

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestOwnerTypes(t *testing.T) {
	owners := []OwnerType{OwnerSystem, OwnerUser, OwnerExtension, OwnerModule, OwnerShared, OwnerTemporary, OwnerMigration}
	for _, ot := range owners {
		if !ot.IsValid() {
			t.Errorf("expected %q to be valid", ot)
		}
	}
	if OwnerType("invalid").IsValid() {
		t.Error("expected invalid owner type to be invalid")
	}
}

func TestResourceOwnerConstructors(t *testing.T) {
	sys := NewSystemOwner()
	if !sys.IsSystem() {
		t.Error("expected system owner")
	}

	user := NewUserOwner("u1")
	if !user.IsUser() || user.OwnerID != "u1" {
		t.Error("expected user owner")
	}

	ext := NewExtensionOwner("ext1")
	if !ext.IsExtension() || ext.ExtensionID != "ext1" {
		t.Error("expected extension owner")
	}

	mod := NewModuleOwner("ext1", "mod1")
	if !mod.IsModule() || mod.ModuleID != "mod1" {
		t.Error("expected module owner")
	}

	shared := NewSharedOwner("shr1")
	if !shared.IsShared() {
		t.Error("expected shared owner")
	}

	tmp := NewTemporaryOwner("tmp1")
	if !tmp.IsTemporary() {
		t.Error("expected temporary owner")
	}

	mig := NewMigrationOwner("mig1")
	if !mig.IsMigration() {
		t.Error("expected migration owner")
	}
}

func TestResourceOwnerEquals(t *testing.T) {
	a := NewUserOwner("u1")
	b := NewUserOwner("u1")
	c := NewUserOwner("u2")

	if !a.Equals(b) {
		t.Error("expected equal owners")
	}
	if a.Equals(c) {
		t.Error("expected different owners")
	}
}

func TestResourceStateTransitions(t *testing.T) {
	tests := []struct {
		from     ResourceState
		to       ResourceState
		expected bool
	}{
		{StatePending, StateActive, true},
		{StatePending, StateFailed, true},
		{StatePending, StateDeleted, true},
		{StatePending, StateDisabled, false},
		{StateActive, StateDisabled, true},
		{StateActive, StateSuspended, true},
		{StateActive, StateDeleting, true},
		{StateDisabled, StateActive, true},
		{StateDisabled, StateDeleting, true},
		{StateSuspended, StateActive, true},
		{StateDeleting, StateDeleted, true},
		{StateDeleting, StateRetained, true},
		{StateDeleting, StateActive, false},
		{StateDeleted, StateActive, false},
		{StateDeleted, StateOrphaned, false},
		{StateOrphaned, StateActive, true},
		{StateOrphaned, StateDeleted, true},
		{StateOrphaned, StateRetained, true},
	}

	for _, tt := range tests {
		got := IsValidStateTransition(tt.from, tt.to)
		if got != tt.expected {
			t.Errorf("transition %s->%s: expected %v, got %v", tt.from, tt.to, tt.expected, got)
		}
	}
}

func TestResourceStateIsTerminal(t *testing.T) {
	terminals := []ResourceState{StateDeleted, StateRetained, StateOrphaned}
	for _, s := range terminals {
		if !s.IsTerminal() {
			t.Errorf("expected %q to be terminal", s)
		}
	}
	nonTerminals := []ResourceState{StatePending, StateActive, StateDisabled, StateSuspended}
	for _, s := range nonTerminals {
		if s.IsTerminal() {
			t.Errorf("expected %q to NOT be terminal", s)
		}
	}
}

func TestResourceTypes(t *testing.T) {
	types := []ResourceType{
		ResourceExtensionPackage, ResourceExtensionModule, ResourceTool,
		ResourceAgentSkill, ResourceMCPServer, ResourceMCPTool,
		ResourceWorkflow, ResourceUIContribution, ResourceHook,
		ResourceBackgroundTask, ResourceSchedule, ResourceEventSubscription,
		ResourceProvider, ResourceSecret, ResourceStorageNamespace,
		ResourceFile, ResourceArtifact, ResourceCache,
		ResourceProcess, ResourceConnection, ResourceTemporaryDirectory,
		ResourceWindow, ResourceTrayAction,
	}
	for _, rt := range types {
		if !rt.IsValid() {
			t.Errorf("expected %q to be valid", rt)
		}
	}
	if ResourceType("invalid").IsValid() {
		t.Error("expected invalid resource type to be invalid")
	}
}

func TestReferenceTypes(t *testing.T) {
	types := []ReferenceType{
		RefDependsOn, RefContains, RefUses, RefGeneratedFrom,
		RefInstalledBy, RefOwnedBy, RefScopedBy, RefSecuredBy,
		RefScheduledBy, RefRenderedBy, RefRuntimeManagedBy,
	}
	for _, rt := range types {
		if !rt.IsValid() {
			t.Errorf("expected %q to be valid", rt)
		}
	}
}

func TestOwnershipEffects(t *testing.T) {
	effects := []OwnershipEffect{
		EffectNone, EffectRetainTarget, EffectBlockDelete,
		EffectCascadeDelete, EffectTransferOnDelete, EffectPromptUser,
	}
	for _, e := range effects {
		if !e.IsValid() {
			t.Errorf("expected %q to be valid", e)
		}
	}
}

func TestDeleteStrategies(t *testing.T) {
	strategies := []DeleteStrategy{
		StrategyCascade, StrategyRetain, StrategyTransfer,
		StrategyPrompt, StrategyBlock, StrategyRebuildable,
	}
	for _, s := range strategies {
		if !s.IsValid() {
			t.Errorf("expected %q to be valid", s)
		}
	}
}

func TestTransferActions(t *testing.T) {
	actions := []TransferAction{TransferAdopt, TransferClone, TransferDetach}
	for _, a := range actions {
		if !a.IsValid() {
			t.Errorf("expected %q to be valid", a)
		}
	}
	if TransferAction("invalid").IsValid() {
		t.Error("expected invalid transfer action to be invalid")
	}
}

func TestMemoryStoreRegisterAndGet(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	rec := ResourceRecord{
		ResourceID:   "res-1",
		ResourceType: ResourceTool,
		Owner:        NewExtensionOwner("ext-1"),
		State:        StatePending,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := store.SaveResource(ctx, rec)
	if err != nil {
		t.Fatalf("SaveResource failed: %v", err)
	}

	got, err := store.GetResource(ctx, "res-1")
	if err != nil {
		t.Fatalf("GetResource failed: %v", err)
	}
	if got.ResourceID != "res-1" {
		t.Errorf("expected ResourceID 'res-1', got %q", got.ResourceID)
	}
	if got.ResourceType != ResourceTool {
		t.Errorf("expected ResourceType tool, got %q", got.ResourceType)
	}
}

func TestMemoryStoreDuplicateResource(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	rec := ResourceRecord{
		ResourceID: "dup-1",
		State:      StatePending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	_ = store.SaveResource(ctx, rec)
	err := store.SaveResource(ctx, rec)
	if err != ErrResourceAlreadyExists {
		t.Errorf("expected ErrResourceAlreadyExists, got %v", err)
	}
}

func TestMemoryStoreUpdateResourceState(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	rec := ResourceRecord{
		ResourceID: "res-2",
		State:      StatePending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	_ = store.SaveResource(ctx, rec)

	err := store.UpdateResourceState(ctx, "res-2", StateActive)
	if err != nil {
		t.Fatalf("UpdateResourceState failed: %v", err)
	}

	got, _ := store.GetResource(ctx, "res-2")
	if got.State != StateActive {
		t.Errorf("expected StateActive, got %q", got.State)
	}
}

func TestMemoryStoreInvalidStateTransition(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	rec := ResourceRecord{
		ResourceID: "res-3",
		State:      StateActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	_ = store.SaveResource(ctx, rec)

	err := store.UpdateResourceState(ctx, "res-3", StatePending)
	if err != ErrInvalidStateTransition {
		t.Errorf("expected ErrInvalidStateTransition, got %v", err)
	}
}

func TestMemoryStoreDeleteResource(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	rec := ResourceRecord{
		ResourceID: "res-4",
		State:      StatePending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	_ = store.SaveResource(ctx, rec)

	err := store.DeleteResource(ctx, "res-4")
	if err != nil {
		t.Fatalf("DeleteResource failed: %v", err)
	}

	_, err = store.GetResource(ctx, "res-4")
	if err != ErrResourceNotFound {
		t.Errorf("expected ErrResourceNotFound, got %v", err)
	}
}

func TestMemoryStoreListByOwner(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.SaveResource(ctx, ResourceRecord{
		ResourceID: "r1", Owner: NewExtensionOwner("ext-a"), State: StateActive,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	_ = store.SaveResource(ctx, ResourceRecord{
		ResourceID: "r2", Owner: NewExtensionOwner("ext-a"), State: StateActive,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	_ = store.SaveResource(ctx, ResourceRecord{
		ResourceID: "r3", Owner: NewExtensionOwner("ext-b"), State: StateActive,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	list, err := store.ListResourcesByOwner(ctx, NewExtensionOwner("ext-a"))
	if err != nil {
		t.Fatalf("ListResourcesByOwner failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 resources, got %d", len(list))
	}
}

func TestMemoryStoreListByType(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.SaveResource(ctx, ResourceRecord{
		ResourceID: "rt1", ResourceType: ResourceTool, Owner: NewSystemOwner(),
		State: StateActive, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	_ = store.SaveResource(ctx, ResourceRecord{
		ResourceID: "rt2", ResourceType: ResourceMCPServer, Owner: NewSystemOwner(),
		State: StateActive, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	list, err := store.ListResourcesByType(ctx, "")
	if err != nil {
		t.Fatalf("ListResourcesByType(all) failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 resources, got %d", len(list))
	}

	list2, err := store.ListResourcesByType(ctx, ResourceTool)
	if err != nil {
		t.Fatalf("ListResourcesByType(tool) failed: %v", err)
	}
	if len(list2) != 1 {
		t.Errorf("expected 1 tool, got %d", len(list2))
	}
}

func TestMemoryStoreUpdateResource(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	rec := ResourceRecord{
		ResourceID:   "res-update",
		ResourceType: ResourceTool,
		Owner:        NewSystemOwner(),
		State:        StateActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_ = store.SaveResource(ctx, rec)

	updated := rec
	updated.Owner = NewUserOwner("u1")
	updated.State = StateSuspended

	err := store.UpdateResource(ctx, "res-update", updated)
	if err != nil {
		t.Fatalf("UpdateResource failed: %v", err)
	}

	got, _ := store.GetResource(ctx, "res-update")
	if !got.Owner.IsUser() {
		t.Errorf("expected user owner, got %q", got.Owner.OwnerType)
	}
	if got.State != StateSuspended {
		t.Errorf("expected StateSuspended, got %q", got.State)
	}
}

func TestMemoryStoreUpdateResourceNotFound(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	rec := ResourceRecord{ResourceID: "nonexistent", State: StateActive}
	err := store.UpdateResource(ctx, "nonexistent", rec)
	if err != ErrResourceNotFound {
		t.Errorf("expected ErrResourceNotFound, got %v", err)
	}
}

func TestMemoryStoreReferenceOperations(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	ref := ResourceReference{
		ReferenceID:      "ref-1",
		SourceResourceID: "res-src",
		TargetResourceID: "res-tgt",
		ReferenceType:    RefDependsOn,
		Required:         true,
		OwnershipEffect:  EffectBlockDelete,
		CreatedAt:        time.Now(),
	}

	err := store.SaveReference(ctx, ref)
	if err != nil {
		t.Fatalf("SaveReference failed: %v", err)
	}

	got, err := store.GetReference(ctx, "ref-1")
	if err != nil {
		t.Fatalf("GetReference failed: %v", err)
	}
	if got.SourceResourceID != "res-src" {
		t.Errorf("expected source 'res-src', got %q", got.SourceResourceID)
	}

	list, err := store.ListReferencesBySource(ctx, "res-src")
	if err != nil {
		t.Fatalf("ListReferencesBySource failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 reference, got %d", len(list))
	}

	allList, err := store.ListAllReferences(ctx, "res-tgt")
	if err != nil {
		t.Fatalf("ListAllReferences failed: %v", err)
	}
	if len(allList) != 1 {
		t.Errorf("expected 1 reference, got %d", len(allList))
	}

	err = store.DeleteReference(ctx, "ref-1")
	if err != nil {
		t.Fatalf("DeleteReference failed: %v", err)
	}

	_, err = store.GetReference(ctx, "ref-1")
	if err != ErrReferenceNotFound {
		t.Errorf("expected ErrReferenceNotFound, got %v", err)
	}
}

func TestMemoryStoreCircularReference(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	ref := ResourceReference{
		ReferenceID:      "ref-circ",
		SourceResourceID: "same-res",
		TargetResourceID: "same-res",
		ReferenceType:    RefUses,
		CreatedAt:        time.Now(),
	}

	err := store.SaveReference(ctx, ref)
	if err != ErrCircularReference {
		t.Errorf("expected ErrCircularReference, got %v", err)
	}
}

func TestMemoryStoreDuplicateReference(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	ref := ResourceReference{
		ReferenceID:      "ref-dup",
		SourceResourceID: "a",
		TargetResourceID: "b",
		ReferenceType:    RefUses,
		CreatedAt:        time.Now(),
	}

	_ = store.SaveReference(ctx, ref)
	err := store.SaveReference(ctx, ref)
	if err != ErrDuplicateReference {
		t.Errorf("expected ErrDuplicateReference, got %v", err)
	}
}

func TestMemoryStoreTransfer(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	record := OwnershipTransferRecord{
		TransferID: "trf-1",
		ResourceID: "res-1",
		FromOwner:  NewExtensionOwner("ext-old"),
		ToOwner:    NewUserOwner("u1"),
		Action:     TransferAdopt,
		CreatedAt:  time.Now(),
	}

	err := store.SaveTransfer(ctx, record)
	if err != nil {
		t.Fatalf("SaveTransfer failed: %v", err)
	}

	got, err := store.GetTransfer(ctx, "trf-1")
	if err != nil {
		t.Fatalf("GetTransfer failed: %v", err)
	}
	if got.ResourceID != "res-1" {
		t.Errorf("expected ResourceID 'res-1', got %q", got.ResourceID)
	}

	list, err := store.ListTransfersByResource(ctx, "res-1")
	if err != nil {
		t.Fatalf("ListTransfersByResource failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 transfer, got %d", len(list))
	}
}

func TestMemoryStoreReleasePlan(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	plan := ResourceReleasePlan{
		PlanID:         "plan-1",
		RootResourceID: "res-1",
		DeleteResources: []ResourceAction{
			{ResourceID: "res-1", ResourceType: ResourceTool, Action: "delete"},
		},
		CreatedAt: time.Now(),
	}

	err := store.SaveReleasePlan(ctx, plan)
	if err != nil {
		t.Fatalf("SaveReleasePlan failed: %v", err)
	}

	got, err := store.GetReleasePlan(ctx, "plan-1")
	if err != nil {
		t.Fatalf("GetReleasePlan failed: %v", err)
	}
	if got.RootResourceID != "res-1" {
		t.Errorf("expected RootResourceID 'res-1', got %q", got.RootResourceID)
	}
}

func TestMemoryStoreCleanupJob(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	job := CleanupJob{
		JobID:      "job-1",
		ResourceID: "res-1",
		JobType:    CleanupJobTypeStopProcess,
		Status:     CleanupJobStatusPending,
		MaxRetries: 3,
		CreatedAt:  time.Now(),
	}

	err := store.SaveCleanupJob(ctx, job)
	if err != nil {
		t.Fatalf("SaveCleanupJob failed: %v", err)
	}

	pending, err := store.ListPendingCleanupJobs(ctx)
	if err != nil {
		t.Fatalf("ListPendingCleanupJobs failed: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("expected 1 pending job, got %d", len(pending))
	}

	err = store.UpdateCleanupJobStatus(ctx, "job-1", CleanupJobStatusCompleted)
	if err != nil {
		t.Fatalf("UpdateCleanupJobStatus failed: %v", err)
	}

	got, _ := store.GetCleanupJob(ctx, "job-1")
	if got.Status != CleanupJobStatusCompleted {
		t.Errorf("expected completed status, got %q", got.Status)
	}
	if got.FinishedAt == nil {
		t.Error("expected FinishedAt to be set")
	}
}

func TestMemoryStoreOrphanReport(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	report := OrphanReport{
		ReportID:   "rpt-1",
		TotalCount: 3,
		HighRisk:   1,
		AutoClean:  2,
		CreatedAt:  time.Now(),
		Entries: []OrphanEntry{
			{
				Kind:         OrphanKindDatabaseRecord,
				Identifier:   "orphan-1",
				Description:  "test",
				Risk:         "high",
				CanAutoClean: false,
				DetectedAt:   time.Now(),
			},
		},
	}

	err := store.SaveOrphanReport(ctx, report)
	if err != nil {
		t.Fatalf("SaveOrphanReport failed: %v", err)
	}

	got, err := store.GetOrphanReport(ctx, "rpt-1")
	if err != nil {
		t.Fatalf("GetOrphanReport failed: %v", err)
	}
	if got.TotalCount != 3 {
		t.Errorf("expected TotalCount 3, got %d", got.TotalCount)
	}
	if !got.HasHighRisk() {
		t.Error("expected HasHighRisk to be true")
	}
}

func TestServiceRegisterResource(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	rec := ResourceRecord{
		ResourceType: ResourceTool,
		Owner:        NewExtensionOwner("ext-1"),
		State:        StatePending,
	}

	err := svc.Register(ctx, &rec)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if rec.ResourceID == "" {
		t.Error("expected ResourceID to be populated")
	}

	got, err := svc.GetResource(ctx, rec.ResourceID)
	if err != nil {
		t.Fatalf("GetResource failed: %v", err)
	}
	if got.State != StatePending {
		t.Errorf("expected StatePending, got %q", got.State)
	}
}

func TestServiceRegisterInvalidType(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	rec := ResourceRecord{
		ResourceType: "invalid",
		Owner:        NewSystemOwner(),
	}

	err := svc.Register(ctx, &rec)
	if err != ErrInvalidResourceType {
		t.Errorf("expected ErrInvalidResourceType, got %v", err)
	}
}

func TestServiceAddReference(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	_ = svc.Register(ctx, &ResourceRecord{
		ResourceID: "res-a", ResourceType: ResourceTool,
		Owner: NewSystemOwner(), State: StateActive,
	})
	_ = svc.Register(ctx, &ResourceRecord{
		ResourceID: "res-b", ResourceType: ResourceMCPServer,
		Owner: NewSystemOwner(), State: StateActive,
	})

	err := svc.AddReference(ctx, ResourceReference{
		SourceResourceID: "res-a",
		TargetResourceID: "res-b",
		ReferenceType:    RefDependsOn,
		Required:         true,
	})
	if err != nil {
		t.Fatalf("AddReference failed: %v", err)
	}

	refs, err := svc.ListReferences(ctx, "res-a")
	if err != nil {
		t.Fatalf("ListReferences failed: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("expected 1 reference, got %d", len(refs))
	}
}

func TestServiceTransferOwnershipAdopt(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	rec := ResourceRecord{
		ResourceID: "res-adopt", ResourceType: ResourceTool,
		Owner: NewExtensionOwner("ext-1"), State: StateActive,
	}
	_ = store.SaveResource(ctx, rec)

	err := svc.TransferOwnership(ctx, OwnershipTransferRequest{
		ResourceID: "res-adopt",
		FromOwner:  NewExtensionOwner("ext-1"),
		ToOwner:    NewUserOwner("u1"),
		Action:     TransferAdopt,
	})
	if err != nil {
		t.Fatalf("TransferOwnership failed: %v", err)
	}

	got, _ := svc.GetResource(ctx, "res-adopt")
	if !got.Owner.IsUser() {
		t.Errorf("expected user owner, got %q", got.Owner.OwnerType)
	}
}

func TestServiceTransferOwnershipClone(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	rec := ResourceRecord{
		ResourceID: "res-clone", ResourceType: ResourceWorkflow,
		Owner: NewExtensionOwner("ext-1"), State: StateActive,
	}
	_ = store.SaveResource(ctx, rec)

	err := svc.TransferOwnership(ctx, OwnershipTransferRequest{
		ResourceID: "res-clone",
		FromOwner:  NewExtensionOwner("ext-1"),
		ToOwner:    NewUserOwner("u1"),
		Action:     TransferClone,
		CloneID:    "res-clone-user",
	})
	if err != nil {
		t.Fatalf("TransferOwnership(Clone) failed: %v", err)
	}

	original, _ := svc.GetResource(ctx, "res-clone")
	if !original.Owner.IsExtension() {
		t.Error("original should still be extension-owned")
	}

	clone, _ := svc.GetResource(ctx, "res-clone-user")
	if !clone.Owner.IsUser() {
		t.Errorf("clone should be user-owned, got %q", clone.Owner.OwnerType)
	}
}

func TestServicePlanReleaseCascade(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	_ = svc.Register(ctx, &ResourceRecord{
		ResourceID: "res-cascade", ResourceType: ResourceTool,
		Owner: NewExtensionOwner("ext-1"), State: StateActive,
		DeleteStrategy: StrategyCascade,
	})

	plan, err := svc.PlanRelease(ctx, ResourceReleaseRequest{
		ResourceID:  "res-cascade",
		RequestedBy: NewExtensionOwner("ext-1"),
	})
	if err != nil {
		t.Fatalf("PlanRelease failed: %v", err)
	}

	if len(plan.DeleteResources) != 1 {
		t.Errorf("expected 1 delete action, got %d", len(plan.DeleteResources))
	}
}

func TestServicePlanReleaseBlocked(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	_ = svc.Register(ctx, &ResourceRecord{
		ResourceID: "res-blocked", ResourceType: ResourceMCPServer,
		Owner: NewExtensionOwner("ext-1"), State: StateActive,
		DeleteStrategy: StrategyBlock,
	})

	plan, err := svc.PlanRelease(ctx, ResourceReleaseRequest{
		ResourceID:  "res-blocked",
		RequestedBy: NewExtensionOwner("ext-1"),
	})
	if err != nil {
		t.Fatalf("PlanRelease failed: %v", err)
	}

	if !plan.IsBlocked() {
		t.Error("expected plan to be blocked")
	}
}

func TestServicePlanReleasePrompt(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	_ = svc.Register(ctx, &ResourceRecord{
		ResourceID: "res-prompt", ResourceType: ResourceSecret,
		Owner: NewExtensionOwner("ext-1"), State: StateActive,
		DeleteStrategy: StrategyPrompt,
	})

	plan, err := svc.PlanRelease(ctx, ResourceReleaseRequest{
		ResourceID:  "res-prompt",
		RequestedBy: NewExtensionOwner("ext-1"),
	})
	if err != nil {
		t.Fatalf("PlanRelease failed: %v", err)
	}

	if !plan.NeedsUserInput() {
		t.Error("expected plan to need user input")
	}
}

func TestServiceExecuteRelease(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	_ = svc.Register(ctx, &ResourceRecord{
		ResourceID: "res-del", ResourceType: ResourceTool,
		Owner: NewExtensionOwner("ext-1"), State: StateActive,
		DeleteStrategy: StrategyCascade,
	})

	plan, _ := svc.PlanRelease(ctx, ResourceReleaseRequest{
		ResourceID:  "res-del",
		RequestedBy: NewExtensionOwner("ext-1"),
	})

	result, err := svc.ExecuteRelease(ctx, plan)
	if err != nil {
		t.Fatalf("ExecuteRelease failed: %v", err)
	}

	if result.DeletedCount != 1 {
		t.Errorf("expected 1 deletion, got %d", result.DeletedCount)
	}

	_, err = svc.GetResource(ctx, "res-del")
	if err != ErrResourceNotFound {
		t.Errorf("expected ErrResourceNotFound, got %v", err)
	}
}

func TestServiceExecuteReleaseBlocked(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	plan := &ResourceReleasePlan{
		PlanID: "plan-blocked",
		Blockers: []ResourceBlocker{
			{ResourceID: "res-1", Reason: "in use"},
		},
	}

	_, err := svc.ExecuteRelease(ctx, plan)
	if err != ErrReleaseBlocked {
		t.Errorf("expected ErrReleaseBlocked, got %v", err)
	}
}

func TestServiceListOwned(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	_ = svc.Register(ctx, &ResourceRecord{
		ResourceType: ResourceTool, Owner: NewExtensionOwner("ext-x"), State: StateActive,
	})
	_ = svc.Register(ctx, &ResourceRecord{
		ResourceType: ResourceMCPServer, Owner: NewExtensionOwner("ext-x"), State: StateActive,
	})

	list, err := svc.ListOwned(ctx, NewExtensionOwner("ext-x"))
	if err != nil {
		t.Fatalf("ListOwned failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 resources, got %d", len(list))
	}
}

func TestServiceUpdateState(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	rec := ResourceRecord{
		ResourceID: "res-state", ResourceType: ResourceTool,
		Owner: NewSystemOwner(), State: StatePending,
	}
	_ = store.SaveResource(ctx, rec)

	err := svc.UpdateState(ctx, "res-state", StateActive)
	if err != nil {
		t.Fatalf("UpdateState failed: %v", err)
	}

	got, _ := svc.GetResource(ctx, "res-state")
	if got.State != StateActive {
		t.Errorf("expected StateActive, got %q", got.State)
	}
}

func TestServiceScanOrphans(t *testing.T) {
	store := NewMemoryStore()
	svc := NewResourceOwnershipService(store)
	ctx := context.Background()

	expired := time.Now().Add(-1 * time.Hour)
	_ = store.SaveResource(ctx, ResourceRecord{
		ResourceID: "res-expired", ResourceType: ResourceCache,
		Owner: NewTemporaryOwner("tmp"), State: StateActive,
		ExpiresAt: &expired, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	report, err := svc.ScanOrphans(ctx)
	if err != nil {
		t.Fatalf("ScanOrphans failed: %v", err)
	}

	if !report.HasOrphans() {
		t.Error("expected orphans from expired temporary resource")
	}
}

func TestReleasePlanBlockers(t *testing.T) {
	plan := ResourceReleasePlan{}
	if plan.IsBlocked() {
		t.Error("empty plan should not be blocked")
	}

	plan.Blockers = []ResourceBlocker{{ResourceID: "r1", Reason: "test"}}
	if !plan.IsBlocked() {
		t.Error("plan with blockers should be blocked")
	}
}

func TestReleasePlanUserDecisions(t *testing.T) {
	plan := ResourceReleasePlan{}
	if plan.NeedsUserInput() {
		t.Error("empty plan should not need user input")
	}

	plan.UserDecisions = []RequiredUserDecision{{ResourceID: "r1", Question: "test"}}
	if !plan.NeedsUserInput() {
		t.Error("plan with decisions should need user input")
	}
}

func TestOrphanReportHelpers(t *testing.T) {
	r := OrphanReport{}
	if r.HasHighRisk() {
		t.Error("empty report should not have high risk")
	}
	if r.HasOrphans() {
		t.Error("empty report should not have orphans")
	}

	r.TotalCount = 5
	if !r.HasOrphans() {
		t.Error("report with entries should have orphans")
	}
}

func TestConcurrentResourceAccess(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			res := ResourceRecord{
				ResourceID:   NewResourceID(),
				ResourceType: ResourceTool,
				Owner:        NewSystemOwner(),
				State:        StateActive,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			_ = store.SaveResource(ctx, res)
			_, _ = store.GetResource(ctx, res.ResourceID)
		}(i)
	}

	wg.Wait()

	list, err := store.ListResourcesByOwner(ctx, NewSystemOwner())
	if err != nil {
		t.Fatalf("ListResourcesByOwner failed: %v", err)
	}
	if len(list) != 20 {
		t.Errorf("expected 20 resources, got %d", len(list))
	}
}

func TestIDGenerators(t *testing.T) {
	if NewResourceID() == "" {
		t.Error("NewResourceID returned empty")
	}
	if NewReferenceID() == "" {
		t.Error("NewReferenceID returned empty")
	}
	if NewTransferID() == "" {
		t.Error("NewTransferID returned empty")
	}
	if NewPlanID() == "" {
		t.Error("NewPlanID returned empty")
	}
	if NewJobID() == "" {
		t.Error("NewJobID returned empty")
	}
	if NewReportID() == "" {
		t.Error("NewReportID returned empty")
	}
}
