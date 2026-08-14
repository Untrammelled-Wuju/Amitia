package migration

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

type testAuthorityProvider struct {
	toolFacade         interface{}
	permissionBroker   interface{}
	eventService       interface{}
	scheduleService    interface{}
	taskRuntimeService interface{}
	hookService        interface{}
}

func (p *testAuthorityProvider) ToolFacade() interface{}         { return p.toolFacade }
func (p *testAuthorityProvider) PermissionBroker() interface{}   { return p.permissionBroker }
func (p *testAuthorityProvider) EventService() interface{}       { return p.eventService }
func (p *testAuthorityProvider) ScheduleService() interface{}    { return p.scheduleService }
func (p *testAuthorityProvider) TaskRuntimeService() interface{} { return p.taskRuntimeService }
func (p *testAuthorityProvider) HookService() interface{}        { return p.hookService }

func TestCutoverPlan_Preflight(t *testing.T) {
	plan := NewCutoverPlan(&gorm.DB{}, &testAuthorityProvider{
		toolFacade:         struct{}{},
		permissionBroker:   struct{}{},
		eventService:       struct{}{},
		scheduleService:    struct{}{},
		taskRuntimeService: struct{}{},
		hookService:        struct{}{},
	})
	if err := plan.Preflight(context.Background()); err != nil {
		t.Fatalf("expected preflight to pass, got: %v", err)
	}
}

func TestCutoverPlan_Preflight_MissingAuthorities(t *testing.T) {
	plan := NewCutoverPlan(&gorm.DB{}, &testAuthorityProvider{})
	err := plan.Preflight(context.Background())
	if err == nil {
		t.Fatal("expected preflight to fail with missing authorities")
	}
}

func TestCutoverPlan_VerifyCanonicalAuthorities(t *testing.T) {
	plan := NewCutoverPlan(&gorm.DB{}, &testAuthorityProvider{
		toolFacade:         struct{}{},
		permissionBroker:   struct{}{},
		eventService:       struct{}{},
		scheduleService:    struct{}{},
		taskRuntimeService: struct{}{},
		hookService:        struct{}{},
	})
	failures := plan.VerifyCanonicalAuthorities()
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got: %v", failures)
	}
}

func TestCutoverPlan_VerifyCanonicalAuthorities_Missing(t *testing.T) {
	plan := NewCutoverPlan(&gorm.DB{}, &testAuthorityProvider{})
	failures := plan.VerifyCanonicalAuthorities()
	if len(failures) == 0 {
		t.Fatal("expected failures with missing authorities, got none")
	}
}

func TestCutoverPhaseCount(t *testing.T) {
	phases := ValidCutoverPhases()
	if len(phases) != 10 {
		t.Fatalf("expected 10 cutover phases, got %d", len(phases))
	}
}
