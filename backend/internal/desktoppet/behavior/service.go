package behavior

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior/bindings"
)

type AdapterManagerTarget interface {
}

type BindingRepository interface {
	Create(ctx context.Context, binding bindings.BehaviorBinding) error
	Update(ctx context.Context, id string, updates map[string]interface{}) error
	UpdateTyped(ctx context.Context, req bindings.BindingUpdateRequest) (*bindings.BehaviorBinding, error)
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*bindings.BehaviorBinding, error)
	ListByUserCharacter(ctx context.Context, userID, characterID string) ([]bindings.BehaviorBinding, error)
	ListByEventType(ctx context.Context, userID, characterID, eventType string) ([]bindings.BehaviorBinding, error)
}

type CompileFunc func(conditionsJSON json.RawMessage) (interface{}, error)

type ValidateBindingFunc func(binding bindings.BehaviorBinding, condition interface{}) error

type ValidateActionFunc func(preferredAction string, availableActions []string) error

type ReloadEvaluatorFunc func(ctx context.Context, engine *BehaviorEngine, repo BindingRepository, userID, characterID string) error

type BehaviorService struct {
	engine          *BehaviorEngine
	repo            BindingRepository
	adapterManager  any
	compile         CompileFunc
	validateBinding ValidateBindingFunc
	validateAction  ValidateActionFunc
	reloadEvaluator ReloadEvaluatorFunc
}

func (s *BehaviorService) SetAdapterManager(am any) {
	s.adapterManager = am
}

type ServiceOption func(*BehaviorService)

func WithCompileFunc(fn CompileFunc) ServiceOption {
	return func(s *BehaviorService) { s.compile = fn }
}

func WithValidateBindingFunc(fn ValidateBindingFunc) ServiceOption {
	return func(s *BehaviorService) { s.validateBinding = fn }
}

func WithValidateActionFunc(fn ValidateActionFunc) ServiceOption {
	return func(s *BehaviorService) { s.validateAction = fn }
}

func WithReloadEvaluatorFunc(fn ReloadEvaluatorFunc) ServiceOption {
	return func(s *BehaviorService) { s.reloadEvaluator = fn }
}

func NewBehaviorService(engine *BehaviorEngine, repo BindingRepository, opts ...ServiceOption) *BehaviorService {
	svc := &BehaviorService{
		engine: engine,
		repo:   repo,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func (s *BehaviorService) Start(ctx context.Context) error {
	return s.engine.Start(ctx)
}

func (s *BehaviorService) Stop() error {
	return s.engine.Stop()
}

func (s *BehaviorService) IsRunning() bool {
	return s != nil && s.engine != nil && s.engine.IsRunning()
}

func (s *BehaviorService) SubmitEvent(ctx context.Context, event BehaviorEventEnvelope) error {
	return s.engine.SubmitEvent(ctx, event)
}

func (s *BehaviorService) GetBehaviorState(ctx context.Context, userID, characterID string) (*BehaviorContextSnapshot, error) {
	return s.engine.GetState(ctx, userID, characterID)
}

func (s *BehaviorService) GetMetrics() map[string]interface{} {
	return s.engine.GetMetrics()
}

func (s *BehaviorService) SimulateEvent(ctx context.Context, event BehaviorEventEnvelope) (*BehaviorDecision, error) {
	return s.engine.Simulate(ctx, event)
}

func (s *BehaviorService) TriggerReconcile(ctx context.Context, userID, characterID string) error {
	return s.engine.Reconcile(ctx, userID, characterID)
}

func (s *BehaviorService) SetShadowMode(enabled bool) {
	s.engine.SetShadowMode(enabled)
}

func (s *BehaviorService) SetRuntimeCommandEnabled(enabled bool) {
	s.engine.SetRuntimeCommandEnabled(enabled)
}

func (s *BehaviorService) CreateBinding(ctx context.Context, binding bindings.BehaviorBinding) error {
	if s.compile != nil && s.validateBinding != nil {
		condition, err := s.compile(binding.ConditionsJSON)
		if err != nil {
			return NewBehaviorErrorWithCause(ErrCodeBindingInvalid, "条件编译失败", err)
		}
		if err := s.validateBinding(binding, condition); err != nil {
			return err
		}
	}
	if binding.ID == "" {
		binding.ID = UUIDNew()
	}
	now := time.Now()
	binding.CreatedAt = now
	binding.UpdatedAt = now
	if binding.Version == 0 {
		binding.Version = 1
	}
	if err := s.repo.Create(ctx, binding); err != nil {
		return err
	}
	return nil
}

var serviceBindingUpdateWhitelist = map[string]bool{
	"event_type":       true,
	"conditions_json":  true,
	"semantic":         true,
	"preferred_action": true,
	"priority_offset":  true,
	"cooldown_ms":      true,
	"enabled":          true,
}

func filterServiceBindingUpdates(updates map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{})
	for k, v := range updates {
		if serviceBindingUpdateWhitelist[k] {
			filtered[k] = v
		}
	}
	return filtered
}

func (s *BehaviorService) UpdateBinding(ctx context.Context, id string, updates map[string]interface{}) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	filtered := filterServiceBindingUpdates(updates)

	if s.compile != nil && s.validateBinding != nil {
		merged := *existing
		if v, ok := filtered["event_type"].(string); ok {
			merged.EventType = v
		}
		if v, ok := filtered["conditions_json"].(string); ok {
			merged.ConditionsJSON = json.RawMessage(v)
		}
		if v, ok := filtered["semantic"].(string); ok {
			merged.Semantic = v
		}
		if v, ok := filtered["preferred_action"].(string); ok {
			merged.PreferredAction = v
		}
		if v, ok := filtered["priority_offset"]; ok {
			switch val := v.(type) {
			case float64:
				merged.PriorityOffset = int(val)
			case int:
				merged.PriorityOffset = val
			}
		}
		if v, ok := filtered["cooldown_ms"]; ok {
			switch val := v.(type) {
			case float64:
				merged.CooldownMS = int64(val)
			case int64:
				merged.CooldownMS = val
			case int:
				merged.CooldownMS = int64(val)
			}
		}
		if v, ok := filtered["enabled"].(bool); ok {
			merged.Enabled = v
		}
		condition, err := s.compile(merged.ConditionsJSON)
		if err != nil {
			return NewBehaviorErrorWithCause(ErrCodeBindingInvalid, "条件编译失败", err)
		}
		if err := s.validateBinding(merged, condition); err != nil {
			return err
		}
	}

	filtered["updated_at"] = time.Now().Format(time.RFC3339)
	return s.repo.Update(ctx, id, filtered)
}

func (s *BehaviorService) UpdateBindingTyped(ctx context.Context, req bindings.BindingUpdateRequest) (*bindings.BehaviorBinding, error) {
	existing, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if existing.UserID != req.UserID {
		return nil, NewBehaviorError("behavior_binding_not_owned", "无权操作该绑定")
	}

	if s.compile != nil && s.validateBinding != nil {
		merged := *existing
		if req.EventType != nil {
			merged.EventType = *req.EventType
		}
		if req.ConditionsJSON != nil {
			merged.ConditionsJSON = json.RawMessage(*req.ConditionsJSON)
		}
		if req.Semantic != nil {
			merged.Semantic = *req.Semantic
		}
		if req.PreferredAction != nil {
			merged.PreferredAction = *req.PreferredAction
		}
		if req.PriorityOffset != nil {
			merged.PriorityOffset = *req.PriorityOffset
		}
		if req.CooldownMS != nil {
			merged.CooldownMS = *req.CooldownMS
		}
		if req.Enabled != nil {
			merged.Enabled = *req.Enabled
		}
		condition, err := s.compile(merged.ConditionsJSON)
		if err != nil {
			return nil, NewBehaviorErrorWithCause(ErrCodeBindingInvalid, "条件编译失败", err)
		}
		if err := s.validateBinding(merged, condition); err != nil {
			return nil, err
		}
	}

	updated, err := s.repo.UpdateTyped(ctx, req)
	if err != nil {
		if err.Error() == "version conflict" {
			return nil, ErrVersionConflict
		}
		return nil, err
	}
	return updated, nil
}

func (s *BehaviorService) DeleteBinding(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *BehaviorService) ListBindings(ctx context.Context, userID, characterID string) ([]bindings.BehaviorBinding, error) {
	return s.repo.ListByUserCharacter(ctx, userID, characterID)
}

func (s *BehaviorService) GetBinding(ctx context.Context, id string) (*bindings.BehaviorBinding, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *BehaviorService) CompileAndLoadBindings(ctx context.Context, userID, characterID string) error {
	if s.reloadEvaluator != nil {
		return s.reloadEvaluator(ctx, s.engine, s.repo, userID, characterID)
	}
	return nil
}
