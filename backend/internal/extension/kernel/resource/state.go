package resource

type ResourceState string

const (
	StatePending     ResourceState = "pending"
	StateActive      ResourceState = "active"
	StateDisabled    ResourceState = "disabled"
	StateSuspended   ResourceState = "suspended"
	StateUpdating    ResourceState = "updating"
	StateRollingBack ResourceState = "rolling_back"
	StateDeleting    ResourceState = "deleting"
	StateDeleted     ResourceState = "deleted"
	StateFailed      ResourceState = "failed"
	StateOrphaned    ResourceState = "orphaned"
	StateRetained    ResourceState = "retained"
)

func (rs ResourceState) IsValid() bool {
	switch rs {
	case StatePending, StateActive, StateDisabled, StateSuspended,
		StateUpdating, StateRollingBack, StateDeleting, StateDeleted,
		StateFailed, StateOrphaned, StateRetained:
		return true
	}
	return false
}

func (rs ResourceState) IsTerminal() bool {
	return rs == StateDeleted || rs == StateRetained || rs == StateOrphaned
}

func (rs ResourceState) IsRuntime() bool {
	return rs == StateActive || rs == StateUpdating || rs == StateRollingBack
}

var validStateTransitions = map[ResourceState][]ResourceState{
	StatePending:     {StateActive, StateFailed, StateDeleted},
	StateActive:      {StateDisabled, StateSuspended, StateUpdating, StateDeleting, StateOrphaned},
	StateDisabled:    {StateActive, StateDeleting, StateOrphaned},
	StateSuspended:   {StateActive, StateDeleting, StateOrphaned},
	StateUpdating:    {StateActive, StateRollingBack, StateFailed},
	StateRollingBack: {StateActive, StateFailed},
	StateDeleting:    {StateDeleted, StateRetained, StateFailed},
	StateFailed:      {StatePending, StateActive, StateDeleting, StateOrphaned},
	StateOrphaned:    {StateActive, StateDeleted, StateRetained},
	StateRetained:    {StateOrphaned},
}

func IsValidStateTransition(from, to ResourceState) bool {
	targets, ok := validStateTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}
