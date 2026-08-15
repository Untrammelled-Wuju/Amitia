package execution

const (
	MaxCapabilityAcquisitions = 5
	MaxUIRefinements         = 3
	MaxToolRetries           = 3
	MaxRemoteWaits           = 2
	MaxNestedExecutions      = 10
)

type ExecutionBudget struct {
	CapabilityAcquisitions int `json:"capabilityAcquisitions"`
	UIRefinements          int `json:"uiRefinements"`
	ToolRetries            int `json:"toolRetries"`
	RemoteWaits            int `json:"remoteWaits"`
	NestedExecutions       int `json:"nestedExecutions"`
}

func DefaultExecutionBudget() ExecutionBudget {
	return ExecutionBudget{}
}

func (b *ExecutionBudget) CanAcquireCapability() bool {
	return b.CapabilityAcquisitions < MaxCapabilityAcquisitions
}

func (b *ExecutionBudget) CanRefineUI() bool {
	return b.UIRefinements < MaxUIRefinements
}

func (b *ExecutionBudget) CanRetryTool() bool {
	return b.ToolRetries < MaxToolRetries
}

func (b *ExecutionBudget) CanWaitRemote() bool {
	return b.RemoteWaits < MaxRemoteWaits
}

func (b *ExecutionBudget) CanNestExecution() bool {
	return b.NestedExecutions < MaxNestedExecutions
}

func (b *ExecutionBudget) IncrementAcquisitions() {
	b.CapabilityAcquisitions++
}

func (b *ExecutionBudget) IncrementRefinements() {
	b.UIRefinements++
}

func (b *ExecutionBudget) IncrementToolRetries() {
	b.ToolRetries++
}

func (b *ExecutionBudget) IncrementRemoteWaits() {
	b.RemoteWaits++
}

func (b *ExecutionBudget) IncrementNestedExecutions() {
	b.NestedExecutions++
}

func (b ExecutionBudget) IsExhausted() bool {
	return b.CapabilityAcquisitions >= MaxCapabilityAcquisitions &&
		b.UIRefinements >= MaxUIRefinements &&
		b.ToolRetries >= MaxToolRetries &&
		b.RemoteWaits >= MaxRemoteWaits &&
		b.NestedExecutions >= MaxNestedExecutions
}
