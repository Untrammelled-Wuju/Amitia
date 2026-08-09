package runtime

type ServiceExecutionHandle struct {
	RuntimeID  string
	ServiceID  string
	InstanceID string
	PID        int
}

type ServiceExecutionResult struct {
	Handle    *ServiceExecutionHandle
	ServiceID string
	Success   bool
	Error     error
}

type StageExecutionResult struct {
	StageIndex int
	Started    []string
	Failed     map[string]error
}

func NewStageExecutionResult(stageIndex int) *StageExecutionResult {
	return &StageExecutionResult{
		StageIndex: stageIndex,
		Started:    make([]string, 0),
		Failed:     make(map[string]error),
	}
}

func (r *StageExecutionResult) AddStarted(serviceID string) {
	r.Started = append(r.Started, serviceID)
}

func (r *StageExecutionResult) AddFailure(serviceID string, err error) {
	r.Failed[serviceID] = err
}

func (r *StageExecutionResult) HasFailures() bool {
	return len(r.Failed) > 0
}

func (r *StageExecutionResult) FailedCount() int {
	return len(r.Failed)
}

type RuntimeExecutionResult struct {
	RuntimeID         string
	Success           bool
	ServiceResults    []*ServiceExecutionResult
	StageResults      []*StageExecutionResult
	Error             error
	RollbackPerformed bool
	RollbackErrors    []error
}

func NewRuntimeExecutionResult(runtimeID string) *RuntimeExecutionResult {
	return &RuntimeExecutionResult{
		RuntimeID:      runtimeID,
		ServiceResults: make([]*ServiceExecutionResult, 0),
		StageResults:   make([]*StageExecutionResult, 0),
		RollbackErrors: make([]error, 0),
	}
}

func (r *RuntimeExecutionResult) AddServiceResult(result *ServiceExecutionResult) {
	r.ServiceResults = append(r.ServiceResults, result)
}

func (r *RuntimeExecutionResult) AddStageResult(result *StageExecutionResult) {
	r.StageResults = append(r.StageResults, result)
}

func (r *RuntimeExecutionResult) FailedServices() []string {
	var failed []string
	for _, sr := range r.ServiceResults {
		if !sr.Success {
			failed = append(failed, sr.ServiceID)
		}
	}
	return failed
}

type RollbackResult struct {
	RuntimeID    string
	Stopped      []string
	Errors       []error
	StoppedCount int
}

func NewRollbackResult(runtimeID string) *RollbackResult {
	return &RollbackResult{
		RuntimeID: runtimeID,
		Stopped:   make([]string, 0),
		Errors:    make([]error, 0),
	}
}

func (r *RollbackResult) AddStopped(serviceID string) {
	r.Stopped = append(r.Stopped, serviceID)
	r.StoppedCount++
}

func (r *RollbackResult) AddError(err error) {
	r.Errors = append(r.Errors, err)
}

func (r *RollbackResult) HasErrors() bool {
	return len(r.Errors) > 0
}

type ShutdownResult struct {
	RuntimeID       string
	Stopped         []string
	FailedToStop    map[string]error
	SkippedCount    int
	CleanupErrors   []error
}

func NewShutdownResult(runtimeID string) *ShutdownResult {
	return &ShutdownResult{
		RuntimeID:     runtimeID,
		Stopped:       make([]string, 0),
		FailedToStop:  make(map[string]error),
		CleanupErrors: make([]error, 0),
	}
}

func (r *ShutdownResult) AddStopped(serviceID string) {
	r.Stopped = append(r.Stopped, serviceID)
}

func (r *ShutdownResult) AddStopFailure(serviceID string, err error) {
	r.FailedToStop[serviceID] = err
}

func (r *ShutdownResult) AddCleanupError(err error) {
	r.CleanupErrors = append(r.CleanupErrors, err)
}

func (r *ShutdownResult) HasErrors() bool {
	return len(r.FailedToStop) > 0 || len(r.CleanupErrors) > 0
}
