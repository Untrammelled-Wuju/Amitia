package permission

import (
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type EffectiveSubject struct {
	RuntimeID   string
	PluginID    string
	ServiceID   string
	ModuleID    string
	ExtensionID string
}

func (s EffectiveSubject) KernelSubject() permission.PermissionSubject {
	subj := permission.PermissionSubject{
		Type:        permission.SubjectExtension,
		ID:          s.ExtensionID,
		ExtensionID: s.ExtensionID,
	}
	if s.RuntimeID != "" {
		subj.Type = permission.SubjectRuntime
		subj.ID = s.RuntimeID
		subj.ExtensionID = s.ExtensionID
	}
	if moduleID := s.EffectiveModuleID(); moduleID != "" {
		subj.ModuleID = moduleID
	}
	return subj
}

func (s EffectiveSubject) EffectiveModuleID() string {
	return s.ModuleID
}

func (s EffectiveSubject) String() string {
	if s.ServiceID != "" {
		return fmt.Sprintf("runtime:%s/service:%s/module:%s/ext:%s", s.RuntimeID, s.ServiceID, s.EffectiveModuleID(), s.ExtensionID)
	}
	return fmt.Sprintf("runtime:%s/ext:%s", s.RuntimeID, s.ExtensionID)
}

type SubjectResolver interface {
	ResolveExtensionID(pluginID string) (string, bool)
	RuntimeExists(runtimeID string) (pluginID string, state domain.RuntimeState, err error)
	ServiceExists(runtimeID string, serviceID string) (pluginID string, moduleID string, err error)
	GetRuntimeState(runtimeID string) (domain.RuntimeState, error)
}

type GameHostSubjectMapper struct {
	resolver SubjectResolver
	clock    func() time.Time
}

func NewGameHostSubjectMapper(resolver SubjectResolver) *GameHostSubjectMapper {
	return &GameHostSubjectMapper{
		resolver: resolver,
		clock:    time.Now,
	}
}

func NewGameHostSubjectMapperWithClock(resolver SubjectResolver, clock func() time.Time) *GameHostSubjectMapper {
	return &GameHostSubjectMapper{
		resolver: resolver,
		clock:    clock,
	}
}

func (m *GameHostSubjectMapper) MapSubject(runtimeID string, pluginID string) (EffectiveSubject, error) {
	if runtimeID == "" {
		return EffectiveSubject{}, ErrInvalidSubject
	}
	if pluginID == "" {
		return EffectiveSubject{}, ErrInvalidSubject
	}

	extID, ok := m.resolver.ResolveExtensionID(pluginID)
	if !ok {
		return EffectiveSubject{}, fmt.Errorf("%w: plugin %s not found in registry", ErrInvalidSubject, pluginID)
	}

	registeredID, _, err := m.resolver.RuntimeExists(runtimeID)
	if err != nil {
		return EffectiveSubject{}, fmt.Errorf("%w: runtime %s not found: %v", ErrInvalidSubject, runtimeID, err)
	}
	if registeredID != pluginID {
		return EffectiveSubject{}, fmt.Errorf("%w: runtime %s does not belong to plugin %s", ErrInvalidSubject, runtimeID, pluginID)
	}

	return EffectiveSubject{
		RuntimeID:   runtimeID,
		PluginID:    pluginID,
		ExtensionID: extID,
	}, nil
}

func (m *GameHostSubjectMapper) MapServiceSubject(runtimeID string, pluginID string, serviceID string) (EffectiveSubject, error) {
	if serviceID == "" {
		return EffectiveSubject{}, fmt.Errorf("%w: service id required for service-scoped permission subject", ErrInvalidSubject)
	}
	base, err := m.MapSubject(runtimeID, pluginID)
	if err != nil {
		return EffectiveSubject{}, err
	}

	svcPluginID, moduleID, err := m.resolver.ServiceExists(runtimeID, serviceID)
	if err != nil {
		return EffectiveSubject{}, fmt.Errorf("%w: service %s not found in runtime %s: %v", ErrInvalidSubject, serviceID, runtimeID, err)
	}
	if svcPluginID != pluginID {
		return EffectiveSubject{}, fmt.Errorf("%w: service %s does not belong to plugin %s", ErrInvalidSubject, serviceID, pluginID)
	}

	if moduleID == "" {
		return EffectiveSubject{}, fmt.Errorf("%w: service %s has no kernel module binding", ErrInvalidSubject, serviceID)
	}
	base.ServiceID = serviceID
	base.ModuleID = moduleID
	return base, nil
}
