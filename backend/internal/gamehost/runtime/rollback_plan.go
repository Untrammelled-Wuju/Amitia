package runtime

import (
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type StartupProgress struct {
	RuntimeID            domain.RuntimeInstanceID
	StartedThisOperation []domain.ServiceID
}

func (p *LifecyclePlanner) BuildRollbackPlan(
	progress StartupProgress,
	topology RuntimeTopologySnapshot,
	graph DependencyGraphSnapshot,
) (LifecyclePlan, error) {
	if progress.RuntimeID != topology.RuntimeID {
		return LifecyclePlan{}, NewLifecyclePlanError(ErrLifecycleInvalidProgress,
			"progress runtime id mismatch with topology")
	}

	if progress.RuntimeID != graph.RuntimeID {
		return LifecyclePlan{}, NewLifecyclePlanError(ErrLifecycleInvalidProgress,
			"progress runtime id mismatch with graph")
	}

	startedSet := make(map[domain.ServiceID]struct{})
	for _, id := range progress.StartedThisOperation {
		startedSet[id] = struct{}{}
	}

	if len(startedSet) == 0 {
		return LifecyclePlan{
			RuntimeID: progress.RuntimeID,
			PluginID:  topology.PluginID,
			Action:    LifecycleActionStop,
			Stages:    []LifecycleStage{},
			CreatedAt: time.Now(),
		}, nil
	}

	for id := range startedSet {
		found := false
		for _, svc := range topology.Services {
			if svc.ServiceID == id {
				found = true
				break
			}
		}
		if !found {
			return LifecyclePlan{}, NewLifecyclePlanErrorWithCause(ErrLifecycleInvalidProgress,
				"started service not found in topology",
				NewLifecyclePlanError(ErrLifecycleInvalidProgress, string(id)))
		}
	}

	startupPlan, err := p.BuildStartupPlan(topology, graph)
	if err != nil {
		return LifecyclePlan{}, err
	}

	rollbackStages := make([]LifecycleStage, len(startupPlan.Stages))
	for i, stage := range startupPlan.Stages {
		rollbackIdx := len(startupPlan.Stages) - 1 - i
		rollbackStages[rollbackIdx] = LifecycleStage{
			Index:    rollbackIdx,
			Services: []ServicePlanEntry{},
		}
		for _, svc := range stage.Services {
			if _, started := startedSet[svc.ServiceID]; started {
				rollbackStages[rollbackIdx].Services = append(rollbackStages[rollbackIdx].Services, svc)
			}
		}
	}

	nonEmptyStages := make([]LifecycleStage, 0)
	for _, stage := range rollbackStages {
		if len(stage.Services) > 0 {
			stage.Index = len(nonEmptyStages)
			nonEmptyStages = append(nonEmptyStages, stage)
		}
	}

	return LifecyclePlan{
		RuntimeID: progress.RuntimeID,
		PluginID:  topology.PluginID,
		Action:    LifecycleActionStop,
		Stages:    nonEmptyStages,
		CreatedAt: time.Now(),
	}, nil
}
