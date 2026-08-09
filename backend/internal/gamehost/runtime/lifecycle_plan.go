package runtime

import (
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type LifecycleAction string

const (
	LifecycleActionStart LifecycleAction = "start"
	LifecycleActionStop  LifecycleAction = "stop"
)

type ServicePlanEntry struct {
	ServiceInstanceID ServiceInstanceID
	ServiceID         domain.ServiceID
	Required          bool
	Dependencies      []domain.ServiceID
}

type LifecycleStage struct {
	Index    int
	Services []ServicePlanEntry
}

type LifecyclePlan struct {
	RuntimeID domain.RuntimeInstanceID
	PluginID  domain.PluginID
	Action    LifecycleAction
	Stages    []LifecycleStage
	CreatedAt time.Time
}

func (p LifecyclePlan) Flatten() []domain.ServiceID {
	var result []domain.ServiceID
	for _, stage := range p.Stages {
		for _, svc := range stage.Services {
			result = append(result, svc.ServiceID)
		}
	}
	return result
}

func (p LifecyclePlan) StageCount() int {
	return len(p.Stages)
}

func (p LifecyclePlan) ServiceCount() int {
	count := 0
	for _, stage := range p.Stages {
		count += len(stage.Services)
	}
	return count
}

func (p LifecyclePlan) ContainsService(serviceID domain.ServiceID) bool {
	for _, stage := range p.Stages {
		for _, svc := range stage.Services {
			if svc.ServiceID == serviceID {
				return true
			}
		}
	}
	return false
}
