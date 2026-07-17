package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type Service interface {
	ListSkills(context.Context, ExecutionScope, SkillFilter) ([]SkillView, error)
	GetSkill(context.Context, ExecutionScope, string) (SkillDetailView, error)
	EnableSkill(context.Context, ExecutionScope, string) error
	DisableSkill(context.Context, ExecutionScope, string) error
	GetSkillConfig(context.Context, ExecutionScope, string) (json.RawMessage, error)
	UpdateSkillConfig(context.Context, ExecutionScope, string, json.RawMessage) error
	ResetSkillConfig(context.Context, ExecutionScope, string) error
	GetSkillPermissions(context.Context, ExecutionScope, string) ([]PermissionGrantView, error)
	UpdateSkillPermissions(context.Context, ExecutionScope, string, []PermissionGrantInput) error
	ExecuteSkill(context.Context, ExecuteSkillRequest) (SkillResult, error)
	ListRuns(context.Context, ExecutionScope, RunFilter) (RunPage, error)
	GetRun(context.Context, ExecutionScope, string) (RunView, error)
	ListPlugins(context.Context, int, int) (PluginPage, error)
	GetPlugin(context.Context, ExecutionScope, string) (PluginDetailView, error)
	EnablePlugin(context.Context, ExecutionScope, string) error
	DisablePlugin(context.Context, ExecutionScope, string) error
	ReloadPlugin(context.Context, string) error
	GetPluginConfig(context.Context, ExecutionScope, string) (json.RawMessage, error)
	UpdatePluginConfig(context.Context, ExecutionScope, string, json.RawMessage) error
	ResetPluginConfig(context.Context, ExecutionScope, string) error
	GetPluginPermissions(context.Context, ExecutionScope, string) ([]PermissionGrantView, error)
	UpdatePluginPermissions(context.Context, ExecutionScope, string, []PermissionGrantInput) error
	GetPluginHealth(context.Context, string) (PluginHealth, error)
	ResetPluginCircuit(context.Context, string) error
	GetPluginStates(context.Context, ExecutionScope, string) ([]PluginState, error)
	GetPluginSurface(context.Context, string) (json.RawMessage, error)
	GetPluginSchedules(context.Context, ExecutionScope, string) ([]PluginScheduleDefinition, error)
	SetPluginScheduleEnabled(context.Context, ExecutionScope, string, string, bool) error
	GetPluginEvents(context.Context, ExecutionScope, string, string, int, int) (PluginEventPage, error)
	RetryPluginEvent(context.Context, ExecutionScope, string, string) error
	ExecutePluginSurfaceAction(context.Context, ExecutionScope, string, string, json.RawMessage) (SkillResult, error)
}

type ExtensionService struct {
	registry   SkillRegistry
	executor   SkillExecutor
	repository *Repository
	validator  *SchemaValidator
	plugins    *PluginManager
}

func (s *ExtensionService) AttachPluginManager(manager *PluginManager) {
	s.plugins = manager
}

func NewService(registry SkillRegistry, executor SkillExecutor, repository *Repository, validator *SchemaValidator) *ExtensionService {
	return &ExtensionService{registry: registry, executor: executor, repository: repository, validator: validator}
}

func (s *ExtensionService) ListSkills(ctx context.Context, scope ExecutionScope, filter SkillFilter) ([]SkillView, error) {
	items, err := s.registry.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	views := make([]SkillView, 0, len(items))
	for _, item := range items {
		if item.Definition.Internal {
			continue
		}
		latest, latestErr := s.repository.LatestRun(ctx, scope, item.Definition.ID)
		if latestErr != nil {
			return nil, latestErr
		}
		views = append(views, SkillView{SkillDefinition: item.Definition, LatestRun: latest})
	}
	return views, nil
}

func (s *ExtensionService) GetSkill(ctx context.Context, scope ExecutionScope, skillID string) (SkillDetailView, error) {
	item, err := s.registry.Get(ctx, skillID)
	if err != nil {
		return SkillDetailView{}, err
	}
	permissions, err := s.repository.ListGrants(ctx, skillID)
	if err != nil {
		return SkillDetailView{}, err
	}
	config, err := s.repository.GetConfig(ctx, skillID, PermissionScope{Type: ScopeGlobal}, item.Definition.DefaultConfig)
	if err != nil {
		return SkillDetailView{}, err
	}
	runs, err := s.repository.ListRuns(ctx, scope, RunFilter{SkillID: skillID, Page: 1, PageSize: 10})
	if err != nil {
		return SkillDetailView{}, err
	}
	var latest *RunView
	if len(runs.Items) > 0 {
		latest = &runs.Items[0]
	}
	versions, err := s.repository.ListVersions(ctx, skillID)
	if err != nil {
		return SkillDetailView{}, err
	}
	return SkillDetailView{SkillView: SkillView{SkillDefinition: item.Definition, LatestRun: latest}, Permissions: permissions, Config: redactJSON(config), RecentRuns: runs.Items, Versions: versions}, nil
}

func (s *ExtensionService) EnableSkill(ctx context.Context, scope ExecutionScope, skillID string) error {
	return s.setSkillEnabled(ctx, scope, skillID, true)
}

func (s *ExtensionService) DisableSkill(ctx context.Context, scope ExecutionScope, skillID string) error {
	return s.setSkillEnabled(ctx, scope, skillID, false)
}

func (s *ExtensionService) setSkillEnabled(ctx context.Context, scope ExecutionScope, skillID string, enabled bool) error {
	item, err := s.registry.Get(ctx, skillID)
	if err != nil {
		return err
	}
	changed := item.Definition.Enabled != enabled
	if changed {
		if err := s.registry.SetEnabled(ctx, skillID, enabled); err != nil {
			return err
		}
	}
	var session workshopSessionRecord
	findErr := s.repository.db.WithContext(ctx).Where("installed_skill_id = ? AND user_id = ?", skillID, scope.UserID).Order("updated_at DESC").First(&session).Error
	if errors.Is(findErr, gorm.ErrRecordNotFound) {
		return nil
	}
	if findErr != nil {
		if changed {
			_ = s.registry.SetEnabled(ctx, skillID, item.Definition.Enabled)
		}
		return findErr
	}
	current := WorkshopSessionStatus(session.Status)
	if current == WorkshopArchived || current == WorkshopError {
		return nil
	}
	target := WorkshopDisabled
	from := []WorkshopSessionStatus{WorkshopInstalled, WorkshopEnabled}
	operation := "skill.disable"
	if enabled {
		target = WorkshopEnabled
		from = []WorkshopSessionStatus{WorkshopInstalled, WorkshopDisabled}
		operation = "skill.enable"
	}
	if current == target {
		return nil
	}
	if err := NewWorkshopRepository(s.repository.db).CASStatus(ctx, scope, session.ID, session.LockVersion, from, target, operation, map[string]interface{}{}); err != nil {
		if changed {
			_ = s.registry.SetEnabled(ctx, skillID, item.Definition.Enabled)
		}
		return err
	}
	return nil
}

func (s *ExtensionService) GetSkillConfig(ctx context.Context, _ ExecutionScope, skillID string) (json.RawMessage, error) {
	item, err := s.registry.Get(ctx, skillID)
	if err != nil {
		return nil, err
	}
	config, err := s.repository.GetConfig(ctx, skillID, PermissionScope{Type: ScopeGlobal}, item.Definition.DefaultConfig)
	if err != nil {
		return nil, err
	}
	return redactJSON(config), nil
}

func (s *ExtensionService) UpdateSkillConfig(ctx context.Context, _ ExecutionScope, skillID string, config json.RawMessage) error {
	item, err := s.registry.Get(ctx, skillID)
	if err != nil {
		return err
	}
	config = normalizeJSON(config)
	stored, storedErr := s.repository.GetConfig(ctx, skillID, PermissionScope{Type: ScopeGlobal}, item.Definition.DefaultConfig)
	if storedErr != nil {
		return storedErr
	}
	var storedValue interface{}
	var incomingValue interface{}
	if json.Unmarshal(stored, &storedValue) == nil && json.Unmarshal(config, &incomingValue) == nil {
		if merged, marshalErr := json.Marshal(restoreRedactedValue(storedValue, incomingValue)); marshalErr == nil {
			config = merged
		}
	}
	if len(item.Definition.ConfigSchema) == 0 {
		if string(config) != "{}" {
			return NewExtensionError(ErrSkillInputInvalid, "Skill does not accept configuration", skillID, false, nil)
		}
	} else if err := s.validator.Validate(skillID+"-config", item.Definition.ConfigSchema, config); err != nil {
		return NewExtensionError(ErrSkillInputInvalid, "Skill configuration is invalid", err.Error(), false, err)
	}
	return s.repository.UpdateConfig(ctx, skillID, PermissionScope{Type: ScopeGlobal}, config)
}

func (s *ExtensionService) ResetSkillConfig(ctx context.Context, _ ExecutionScope, skillID string) error {
	if _, err := s.registry.Get(ctx, skillID); err != nil {
		return err
	}
	return s.repository.ResetConfig(ctx, skillID, PermissionScope{Type: ScopeGlobal})
}

func (s *ExtensionService) GetSkillPermissions(ctx context.Context, _ ExecutionScope, skillID string) ([]PermissionGrantView, error) {
	if _, err := s.registry.Get(ctx, skillID); err != nil {
		return nil, err
	}
	return s.repository.ListGrants(ctx, skillID)
}

func (s *ExtensionService) UpdateSkillPermissions(ctx context.Context, scope ExecutionScope, skillID string, grants []PermissionGrantInput) error {
	item, err := s.registry.Get(ctx, skillID)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, capability := range item.Definition.Capabilities {
		allowed[capability] = true
	}
	for _, grant := range grants {
		if !allowed[grant.Capability] {
			return NewExtensionError(ErrSkillPermissionDenied, "Unknown capability for skill", grant.Capability, false, nil)
		}
		if err := validateGrantRoleIsolation(grant, scope); err != nil {
			return err
		}
		if grant.Decision == DecisionAllowSession && grant.ScopeType != ScopeSession {
			return fmt.Errorf("allow_session requires session scope")
		}
		if grant.Decision == DecisionAllowCharacter && grant.ScopeType != ScopeCharacter {
			return fmt.Errorf("allow_character requires character scope")
		}
	}
	return s.repository.ReplaceGrants(ctx, skillID, grants)
}

func (s *ExtensionService) ExecuteSkill(ctx context.Context, request ExecuteSkillRequest) (SkillResult, error) {
	if request.Scope.Trigger == TriggerManual {
		if err := s.repository.ValidateConversationScope(ctx, request.Scope); err != nil {
			return SkillResult{}, err
		}
	}
	return s.executor.Execute(ctx, request)
}

func (s *ExtensionService) ListRuns(ctx context.Context, scope ExecutionScope, filter RunFilter) (RunPage, error) {
	return s.repository.ListRuns(ctx, scope, filter)
}

func (s *ExtensionService) GetRun(ctx context.Context, scope ExecutionScope, runID string) (RunView, error) {
	return s.repository.GetRun(ctx, scope, runID)
}
