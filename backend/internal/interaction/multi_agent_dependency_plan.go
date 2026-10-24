package interaction

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ValidateAssignmentDependencyGraph(objectives []AssignmentObjective) error {
	plan := make([]*AgentAssignment, 0, len(objectives))
	for i, objective := range objectives {
		plan = append(plan, &AgentAssignment{
			ID:           fmt.Sprintf("candidate-%d", i),
			WorkerRef:    AgentWorkerRef{WorkerID: "candidate"},
			Objective:    objective.Objective,
			Dependencies: append([]string(nil), objective.Dependencies...),
		})
	}
	return prepareAssignmentDependencies(plan)
}

func prepareAssignmentDependencies(assignments []*AgentAssignment) error {
	ids := make(map[string]struct{}, len(assignments))
	for _, a := range assignments {
		if a == nil || a.ID == "" {
			return fmt.Errorf("multi_agent: worker assignment has no durable identity")
		}
		if _, exists := ids[a.ID]; exists {
			return fmt.Errorf("multi_agent: duplicate worker assignment identity %s", a.ID)
		}
		ids[a.ID] = struct{}{}
	}
	for _, a := range assignments {
		resolved := make([]string, 0, len(a.Dependencies))
		for _, raw := range a.Dependencies {
			dependency := strings.TrimSpace(raw)
			if strings.HasPrefix(dependency, "index:") {
				n, err := strconv.Atoi(strings.TrimPrefix(dependency, "index:"))
				if err != nil || n < 0 || n >= len(assignments) {
					return fmt.Errorf("multi_agent: invalid worker dependency index %q", raw)
				}
				dependency = assignments[n].ID
			}
			resolved = append(resolved, dependency)
		}
		a.Dependencies = resolved
	}

	ref := &MultiAgentRecoveryRef{AssignmentRefs: make([]AssignmentRecoveryRef, 0, len(assignments))}
	for _, a := range assignments {
		ref.AssignmentRefs = append(ref.AssignmentRefs, AssignmentRecoveryRef{
			AssignmentID: a.ID, WorkerID: a.WorkerRef.WorkerID,
			Objective: a.Objective, Dependencies: append([]string(nil), a.Dependencies...),
		})
	}
	return validateRecoveredWorkerPlan(ref, len(assignments), 0)
}

func failBlockedAssignments(assignments []*AgentAssignment, now time.Time) bool {
	byID := make(map[string]*AgentAssignment, len(assignments))
	for _, a := range assignments {
		byID[a.ID] = a
	}
	changed := false
	for {
		advanced := false
		for _, a := range assignments {
			if a.Status != AssignmentPending {
				continue
			}
			for _, depID := range a.Dependencies {
				predecessor := byID[depID]
				if predecessor != nil && (predecessor.Status == AssignmentFailed || predecessor.Status == AssignmentCancelled) {
					a.Status = AssignmentFailed
					a.Error = "dependency_failed: prerequisite " + depID + " did not succeed"
					a.UpdatedAt = now
					advanced, changed = true, true
					break
				}
			}
		}
		if !advanced {
			return changed
		}
	}
}
