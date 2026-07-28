package kernel

import (
	"context"
)

type ExtensionKernelReadiness struct {
	TaskRuntimeReady      bool
	EventServiceReady     bool
	ScheduleServiceReady  bool
	RuntimeSupervisorReady bool
}

func (r ExtensionKernelReadiness) Ready() bool {
	return r.TaskRuntimeReady && r.EventServiceReady && r.ScheduleServiceReady && r.RuntimeSupervisorReady
}

func (r ExtensionKernelReadiness) FailedComponents() []string {
	var failed []string
	if !r.TaskRuntimeReady {
		failed = append(failed, "task_runtime_ready")
	}
	if !r.EventServiceReady {
		failed = append(failed, "event_service_ready")
	}
	if !r.ScheduleServiceReady {
		failed = append(failed, "schedule_service_ready")
	}
	if !r.RuntimeSupervisorReady {
		failed = append(failed, "runtime_supervisor_ready")
	}
	return failed
}

func (c *Container) CheckExtensionKernelReadiness(_ context.Context) ExtensionKernelReadiness {
	r := ExtensionKernelReadiness{}
	if c == nil {
		return r
	}
	r.TaskRuntimeReady = c.TaskRuntimeService != nil
	if c.EventService != nil {
		r.EventServiceReady = true
	}
	if c.ScheduleService != nil {
		r.ScheduleServiceReady = c.ScheduleService.IsRunning()
	}
	r.RuntimeSupervisorReady = c.RuntimeSupervisor != nil
	return r
}
