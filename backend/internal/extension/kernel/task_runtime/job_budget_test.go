//go:build rsi_jobbudget

package task_runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestJobBudgetPlanValid(t *testing.T) {
	budget, notes, err := PlanJobBudget(JobBudget{MemoryBytes: 512 << 20, CPUPercent: 50, ActiveProcesses: 8}, JobBudgetSupport{Memory: true, CPU: true, Processes: true}, JobBudgetStrict)
	if err != nil || len(notes) != 0 {
		t.Fatalf("budget=%+v notes=%v err=%v", budget, notes, err)
	}
	if budget.MemoryBytes != 512<<20 || budget.CPUPercent != 50 || budget.ActiveProcesses != 8 {
		t.Fatalf("budget=%+v", budget)
	}
	fmt.Printf("m07-s11 plan memory=536870912 cpu=50 processes=8\n")
}

func TestJobBudgetZero(t *testing.T) {
	budget, notes, err := PlanJobBudget(JobBudget{}, JobBudgetSupport{}, JobBudgetStrict)
	if err != nil || len(notes) != 0 || budget != (JobBudget{}) {
		t.Fatalf("budget=%+v notes=%v err=%v", budget, notes, err)
	}
	fmt.Printf("m07-s11 plan zero=true\n")
}

func TestJobBudgetInvalidPolicy(t *testing.T) {
	if _, _, err := PlanJobBudget(JobBudget{}, JobBudgetSupport{}, JobBudgetPolicy("yolo")); !errors.Is(err, ErrJobBudgetInvalid) {
		t.Fatalf("policy err=%v", err)
	}
	fmt.Printf("m07-s11 policy rejected=true\n")
}

func TestJobBudgetInvalidValues(t *testing.T) {
	_, _, err := PlanJobBudget(JobBudget{MemoryBytes: MinJobMemoryBytes - 1, CPUPercent: MaxJobCPUPercent + 1, ActiveProcesses: MaxJobProcesses + 1}, JobBudgetSupport{}, JobBudgetAllowDegraded)
	if !errors.Is(err, ErrJobBudgetInvalid) {
		t.Fatalf("values err=%v", err)
	}
	if strings.Count(err.Error(), "=") < 3 {
		t.Fatalf("err=%v", err)
	}
	fmt.Printf("m07-s11 reject dimensions=3\n")
}

func TestJobBudgetStrictRefusesUnsupportedCPU(t *testing.T) {
	_, _, err := PlanJobBudget(JobBudget{CPUPercent: 50}, JobBudgetSupport{Memory: true, Processes: true}, JobBudgetStrict)
	if !errors.Is(err, ErrJobBudgetUnsupported) {
		t.Fatalf("strict err=%v", err)
	}
	if !IsTaskErrorCode(err, ErrTaskScopeDenied) || HTTPStatusForErrorCode(ErrTaskScopeDenied) != 403 {
		t.Fatalf("task code err=%v", err)
	}
	var refusal *JobBudgetRefusal
	if !errors.As(err, &refusal) || len(refusal.Missing) != 1 || refusal.Missing[0] != "cpuPercent" {
		t.Fatalf("refusal=%+v", refusal)
	}
	fmt.Printf("m07-s11 strict refused=true dimension=cpuPercent http=403\n")
}

func TestJobBudgetDegradedZeroesUnsupportedCPU(t *testing.T) {
	budget, notes, err := PlanJobBudget(JobBudget{CPUPercent: 50, ActiveProcesses: 4}, JobBudgetSupport{Memory: true, Processes: true}, JobBudgetAllowDegraded)
	if err != nil {
		t.Fatalf("degraded: %v", err)
	}
	if budget.CPUPercent != 0 || budget.ActiveProcesses != 4 || len(notes) != 1 || notes[0] != "cpuPercent=unenforced" {
		t.Fatalf("budget=%+v notes=%v", budget, notes)
	}
	fmt.Printf("m07-s11 degraded cpu=0 processes=4\n")
}

func TestJobBudgetStrictSupported(t *testing.T) {
	budget, notes, err := PlanJobBudget(JobBudget{MemoryBytes: 64 << 20}, JobBudgetSupport{Memory: true}, JobBudgetStrict)
	if err != nil || len(notes) != 0 || budget.MemoryBytes != 64<<20 {
		t.Fatalf("budget=%+v notes=%v err=%v", budget, notes, err)
	}
	fmt.Printf("m07-s11 strict supported=true\n")
}

func TestJobBudgetStrictReportsAllMissing(t *testing.T) {
	_, _, err := PlanJobBudget(JobBudget{MemoryBytes: 64 << 20, CPUPercent: 25}, JobBudgetSupport{}, JobBudgetStrict)
	var refusal *JobBudgetRefusal
	if !errors.As(err, &refusal) || len(refusal.Missing) != 2 || refusal.Missing[0] != "cpuPercent" || refusal.Missing[1] != "memoryBytes" {
		t.Fatalf("refusal=%+v", refusal)
	}
	fmt.Printf("m07-s11 strict missing=2 sorted=true\n")
}

func TestJobBudgetDegradedReportsAllMissing(t *testing.T) {
	budget, notes, err := PlanJobBudget(JobBudget{MemoryBytes: 64 << 20, CPUPercent: 25}, JobBudgetSupport{}, JobBudgetAllowDegraded)
	if err != nil || len(notes) != 2 || budget.MemoryBytes != 0 || budget.CPUPercent != 0 {
		t.Fatalf("budget=%+v notes=%v err=%v", budget, notes, err)
	}
	fmt.Printf("m07-s11 degraded notes=2\n")
}

func TestJobBudgetEmptyPolicyIsStrict(t *testing.T) {
	if _, _, err := PlanJobBudget(JobBudget{CPUPercent: 25}, JobBudgetSupport{}, ""); !errors.Is(err, ErrJobBudgetUnsupported) {
		t.Fatalf("default policy err=%v", err)
	}
	fmt.Printf("m07-s11 default strict=true\n")
}

func TestJobBudgetBoundariesAccepted(t *testing.T) {
	budget, notes, err := PlanJobBudget(JobBudget{MemoryBytes: MaxJobMemoryBytes, CPUPercent: MaxJobCPUPercent, ActiveProcesses: MaxJobProcesses}, JobBudgetSupport{Memory: true, CPU: true, Processes: true}, JobBudgetStrict)
	if err != nil || len(notes) != 0 || budget.ActiveProcesses != MaxJobProcesses {
		t.Fatalf("budget=%+v notes=%v err=%v", budget, notes, err)
	}
	fmt.Printf("m07-s11 boundaries accepted=true\n")
}

func TestJobBudgetZeroUnsupportedOK(t *testing.T) {
	budget, notes, err := PlanJobBudget(JobBudget{}, JobBudgetSupport{}, JobBudgetStrict)
	if err != nil || len(notes) != 0 || budget != (JobBudget{}) {
		t.Fatalf("budget=%+v notes=%v err=%v", budget, notes, err)
	}
	fmt.Printf("m07-s11 zero_unsupported ok=true\n")
}
