//go:build rsi_jobbudget

package task_runtime

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	MinJobMemoryBytes = 4 << 20
	MaxJobMemoryBytes = 8 << 30
	MaxJobProcesses   = 256
	MaxJobCPUPercent  = 100
)

var (
	ErrJobBudgetInvalid     = errors.New("task_runtime: job budget invalid")
	ErrJobBudgetUnsupported = errors.New("task_runtime: job budget unsupported")
)

type JobBudget struct {
	MemoryBytes     int64
	CPUPercent      uint32
	ActiveProcesses uint32
}

type JobBudgetSupport struct {
	Memory    bool
	CPU       bool
	Processes bool
}

type JobBudgetPolicy string

const (
	JobBudgetStrict        JobBudgetPolicy = "strict"
	JobBudgetAllowDegraded JobBudgetPolicy = "allow-degraded"
)

type JobBudgetRefusal struct {
	Missing []string
}

func (e *JobBudgetRefusal) Error() string {
	return fmt.Sprintf("%v: %s", ErrJobBudgetUnsupported, strings.Join(e.Missing, ","))
}

func (e *JobBudgetRefusal) Is(target error) bool {
	return target == ErrJobBudgetUnsupported
}

func (e *JobBudgetRefusal) Unwrap() error {
	return WrapTaskError(ErrTaskScopeDenied, "job object budget unsupported", ErrJobBudgetUnsupported)
}

func (b JobBudget) validate() error {
	problems := make([]string, 0, 3)
	if b.MemoryBytes != 0 && (b.MemoryBytes < MinJobMemoryBytes || b.MemoryBytes > MaxJobMemoryBytes) {
		problems = append(problems, fmt.Sprintf("memoryBytes=%d", b.MemoryBytes))
	}
	if b.CPUPercent > MaxJobCPUPercent {
		problems = append(problems, fmt.Sprintf("cpuPercent=%d", b.CPUPercent))
	}
	if b.ActiveProcesses > MaxJobProcesses {
		problems = append(problems, fmt.Sprintf("activeProcesses=%d", b.ActiveProcesses))
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrJobBudgetInvalid, strings.Join(problems, " "))
	}
	return nil
}

func PlanJobBudget(budget JobBudget, support JobBudgetSupport, policy JobBudgetPolicy) (JobBudget, []string, error) {
	if policy == "" {
		policy = JobBudgetStrict
	}
	if policy != JobBudgetStrict && policy != JobBudgetAllowDegraded {
		return JobBudget{}, nil, fmt.Errorf("%w: policy=%q", ErrJobBudgetInvalid, policy)
	}
	if err := budget.validate(); err != nil {
		return JobBudget{}, nil, err
	}
	missing := make([]string, 0, 3)
	if budget.MemoryBytes > 0 && !support.Memory {
		missing = append(missing, "memoryBytes")
	}
	if budget.CPUPercent > 0 && !support.CPU {
		missing = append(missing, "cpuPercent")
	}
	if budget.ActiveProcesses > 0 && !support.Processes {
		missing = append(missing, "activeProcesses")
	}
	sort.Strings(missing)
	if len(missing) == 0 {
		return budget, nil, nil
	}
	if policy == JobBudgetStrict {
		return JobBudget{}, nil, &JobBudgetRefusal{Missing: missing}
	}
	applied := budget
	notes := make([]string, 0, len(missing))
	for _, dimension := range missing {
		switch dimension {
		case "memoryBytes":
			applied.MemoryBytes = 0
		case "cpuPercent":
			applied.CPUPercent = 0
		case "activeProcesses":
			applied.ActiveProcesses = 0
		}
		notes = append(notes, dimension+"=unenforced")
	}
	return applied, notes, nil
}
