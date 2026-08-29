package wiring

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/bindings"
	"github.com/u-ai/backend/internal/desktoppet/behavior/persistence"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/log"

	"gorm.io/gorm"
)

type AssemblyDeps struct {
	DB                *gorm.DB
	ActivePetPort     behavior.ActivePetPort
	RuntimeActionPort behavior.RuntimeActionPort
	InstallRepo       installation.Repository
	PsycheStore       psyche.PsycheStore
	DataDir           string
	ShadowMode        bool
	RuntimeCmdOn      bool
}

type AssembledBehavior struct {
	Engine      *behavior.BehaviorEngine
	Service     *behavior.BehaviorService
	Repo        behavior.BehaviorStateRepository
	BindingRepo behavior.BindingRepository
	Reconciler  *behavior.Reconciler
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

	if deps.ActivePetPort != nil {
		activePetPort = deps.ActivePetPort
	} else {
		activePetPort = &NoopActivePetPort{}
	}

	if deps.RuntimeActionPort != nil {
		runtimeActionPort = deps.RuntimeActionPort
	} else {
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
		stateRepo,
	)

	engineConfig := behavior.EngineConfig{
		ShadowMode:            deps.ShadowMode,
		RuntimeCommandEnabled: deps.RuntimeCmdOn,
		MailboxCapacity:       behavior.MailboxCapacity,
		MaxCASRetries:         behavior.MaxCASRetries,
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
	bindingEvaluator := bindings.NewEvaluator()
	engine.SetBindingEvaluator(func(scope bindings.EvaluatorScope, eventType string, origin behavior.EventOrigin, payload map[string]interface{}) []interface{} {
		evalCtx := bindings.EvalContext{
			Event:   map[string]interface{}{"eventType": eventType},
			Origin:  string(origin),
			Payload: payload,
		}
		matched := bindingEvaluator.Evaluate(scope, eventType, evalCtx)
		result := make([]interface{}, 0, len(matched))
		for _, match := range matched {
			result = append(result, match.Binding)
		}
		return result
	})

	service := behavior.NewBehaviorService(
		engine,
		bindingRepo,
		behavior.WithCompileFunc(func(conditionsJSON json.RawMessage) (interface{}, error) {
			return bindings.Compile(conditionsJSON)
		}),
		behavior.WithValidateBindingFunc(func(b bindings.BehaviorBinding, condition interface{}) error {
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
		behavior.WithResetEvaluatorFunc(bindingEvaluator.Clear),
		behavior.WithReloadEvaluatorFunc(func(ctx context.Context, eng *behavior.BehaviorEngine, repo behavior.BindingRepository, userID, characterID string) error {
			bindingList, err := repo.ListByScope(ctx, userID, characterID)
			if err != nil {
				// Persistence is authoritative. If a post-mutation reload cannot
				// read it, fail closed instead of continuing to execute a stale
				// in-memory binding that the user may have disabled or deleted.
				bindingEvaluator.ReplaceCharacterScopes(userID, characterID, nil)
				return err
			}
			replacements := make(map[bindings.EvaluatorScope][]bindings.CompiledBinding)
			for _, b := range bindingList {
				if !b.Enabled {
					continue
				}
				cond, err := bindings.Compile(b.ConditionsJSON)
				if err != nil {
					log.Logger.Warnf("wiring/assembly: skip binding %s compile error: %v", b.ID, err)
					continue
				}
				if err := validator.Validate(b, cond); err != nil {
					log.Logger.Warnf("wiring/assembly: skip binding %s validation error: %v", b.ID, err)
					continue
				}
				scope := bindings.EvaluatorScope{
					UserID:         b.UserID,
					CharacterID:    b.CharacterID,
					InstallationID: b.InstallationID,
				}
				replacements[scope] = append(replacements[scope], bindings.CompiledBinding{
					Binding:    b,
					Condition:  cond,
					CompiledAt: time.Now(),
				})
			}
			bindingEvaluator.ReplaceCharacterScopes(userID, characterID, replacements)
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
