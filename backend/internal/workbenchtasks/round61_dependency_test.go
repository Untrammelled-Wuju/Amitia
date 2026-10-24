package workbenchtasks

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/interaction"
)

func TestRound61DurableAssignmentDependenciesProjection(t *testing.T) {
	record := taskRecord("space", "conv", "coord-dependencies", "running")
	refs := record.RecoveryDescriptor.MultiAgent.AssignmentRefs
	refs[1].Dependencies = []string{refs[0].AssignmentID}
	result, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv")
	if err != nil || len(result.Tasks) != 1 || len(result.Tasks[0].Assignments) != 2 || len(result.Tasks[0].Assignments[1].Dependencies) != 1 || result.Tasks[0].Assignments[1].Dependencies[0] != refs[0].AssignmentID {
		t.Fatalf("durable dependencies were not projected: %+v %v", result, err)
	}
	refs[1].Dependencies[0] = "changed-later"
	if result.Tasks[0].Assignments[1].Dependencies[0] != "worker-1" {
		t.Fatal("projection retained mutable RecoveryDescriptor dependency slice")
	}
}

func TestRound61RejectsUnsafeDependencySnapshots(t *testing.T) {
	scenarios := map[string]func([]interaction.AssignmentRecoveryRef){
		"unknown predecessor": func(refs []interaction.AssignmentRecoveryRef) { refs[1].Dependencies = []string{"missing"} },
		"self dependency":     func(refs []interaction.AssignmentRecoveryRef) { refs[0].Dependencies = []string{refs[0].AssignmentID} },
		"cyclic dependencies": func(refs []interaction.AssignmentRecoveryRef) {
			refs[0].Dependencies = []string{refs[1].AssignmentID}
			refs[1].Dependencies = []string{refs[0].AssignmentID}
		},
		"duplicate edge": func(refs []interaction.AssignmentRecoveryRef) {
			refs[1].Dependencies = []string{refs[0].AssignmentID, refs[0].AssignmentID}
		},
		"oversized edges": func(refs []interaction.AssignmentRecoveryRef) {
			refs[1].Dependencies = []string{"worker-1", "worker-1", "worker-1", "worker-1", "worker-1", "worker-1", "worker-1", "worker-1", "worker-1"}
		},
		"padded identity": func(refs []interaction.AssignmentRecoveryRef) {
			refs[1].AssignmentID = " worker-2 "
		},
	}
	for name, mutate := range scenarios {
		t.Run(name, func(t *testing.T) {
			record := taskRecord("space", "conv", "coord", "running")
			mutate(record.RecoveryDescriptor.MultiAgent.AssignmentRefs)
			if validateCheckpoint(record.RecoveryDescriptor.MultiAgent) {
				t.Fatal("unsafe DAG checkpoint was accepted")
			}
			result, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv")
			if err != nil || len(result.Tasks) != 0 {
				t.Fatalf("unsafe DAG was exposed in Workbench: %+v %v", result, err)
			}
		})
	}
}
