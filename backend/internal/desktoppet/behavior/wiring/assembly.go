package wiring

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/bindings"
	"github.com/u-ai/backend/internal/desktoppet/behavior/persistence"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/runtime"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/log"

	"gorm.io/gorm"
)

type AssemblyDeps struct {
	DB              *gorm.DB
	RuntimeService  *runtime.Service
	InstallRepo     installation.Repository
	PsycheStore     psyche.PsycheStore
	DataDir         string
	ShadowMode      bool
	RuntimeCmdOn    bool
}

type AssembledBehavior struct {
	Engine  *behavior.BehaviorEngine
	Service *behavior.BehaviorService
	Repo    behavior.BehaviorStateRepository
	BindingRepo behavior.BindingRepository
	Reconciler *behavior.Reconciler
}

func AssembleBehavior(deps AssemblyDeps) (*AssembledBehavior, error) {
	if deps.DB == nil {
		return nil, fmt.Errorf("wiring/assembly: DB is required")
	}
	if deps.PsycheStore == nil {
		return nil, fmt.Errorf("wiring/assembly: PsycheStore is required")
	}

	stateRepo := persistence.NewGormBehaviorStateRepository(deps.DB)
	bindingRepo := bindings.NewRepository(deps.DB)

	var activePetPort behavior.ActivePetPort
	var runtimeActionPort behavior.RuntimeActionPort

	if deps.RuntimeService != nil && deps.InstallRepo != nil {
		activePetPort = NewActivePetAdapter(deps.InstallRepo, deps.RuntimeService.Registry(), deps.DataDir)
		runtimeActionPort = NewRuntimeActionAdapter(deps.RuntimeService)
	} else {
		activePetPort = &NoopActivePetPort{}
		runtimeActionPort = &NoopRuntimeActionPort{}
	}
	affectPort := NewAffectAdapter(deps.PsycheStore)
	activityPort := NewActivityAdapter(behavior.NewRealClock())

	clock := behavior.NewRealClock()
	fallback := behavior.DefaultFallbackGraph()
	reducer := behavior.NewReducer(clock)
	resolver := behavior.NewResolver(clock, fallback)
	arbiter := behavior.NewArbiter(clock, fallback)

	reconciler := behavior.NewReconciler(
		clock,
		reducer,
		resolver,
		arbiter,
		NewAmitiaStateSourceQuery(deps.DB),
		affectPort,
		activityPort,
		activePetPort,
		runtimeActionPort,
	)

	engineConfig := behavior.EngineConfig{
		ShadowMode:           deps.ShadowMode,
		RuntimeCommandEnabled: deps.RuntimeCmdOn,
		MailboxCapacity:      behavior.MailboxCapacity,
		MaxCASRetries:        behavior.MaxCASRetries,
	}

	engine := behavior.NewBehaviorEngine(
		engineConfig,
		clock,
		stateRepo,
		activePetPort,
		runtimeActionPort,
		reconciler,
	)

	validator := bindings.NewValidator()

	service := behavior.NewBehaviorService(
		engine,
		bindingRepo,
		behavior.WithCompileFunc(func(conditionsJSON json.RawMessage) (interface{}, error) {
			return bindings.Compile(conditionsJSON)
		}),
		behavior.WithValidateBindingFunc(func(b behavior.BehaviorBinding, condition interface{}) error {
			node, ok := condition.(bindings.ConditionNode)
			if !ok {
				return fmt.Errorf("invalid condition type")
			}
			return validator.Validate(b, node)
		}),
		behavior.WithValidateActionFunc(func(preferredAction string, availableActions []string) error {
			if preferredAction == "" {
				return nil
			}
			for _, a := range availableActions {
				if a == preferredAction {
					return nil
				}
			}
			return fmt.Errorf("preferred action %s not in available actions", preferredAction)
		}),
		behavior.WithReloadEvaluatorFunc(func(ctx context.Context, eng *behavior.BehaviorEngine, repo behavior.BindingRepository, userID, characterID string) error {
			bindingList, err := repo.ListByUserCharacter(ctx, userID, characterID)
			if err != nil {
				return err
			}
			newEvaluator := bindings.NewEvaluator()
			for _, b := range bindingList {
				if !b.Enabled {
					continue
				}
				cond, err := bindings.Compile(b.ConditionsJSON)
				if err != nil {
					log.Logger.Warnf("wiring/assembly: skip binding %s compile error: %v", b.ID, err)
					continue
				}
				newEvaluator.AddBinding(b, cond)
			}
			eng.SetBindingEvaluator(func(eventType string, origin behavior.EventOrigin, payload map[string]interface{}) []behavior.BehaviorBinding {
				evalCtx := bindings.EvalContext{
					Event:   map[string]interface{}{"eventType": eventType},
					Origin:  string(origin),
					Payload: payload,
				}
				matched := newEvaluator.Evaluate(eventType, evalCtx)
				result := make([]behavior.BehaviorBinding, 0, len(matched))
				for _, m := range matched {
					result = append(result, m.Binding)
				}
				return result
			})
			return nil
		}),
	)

	return &AssembledBehavior{
		Engine:      engine,
		Service:     service,
		Repo:        stateRepo,
		BindingRepo: bindingRepo,
		Reconciler:  reconciler,
	}, nil
}
