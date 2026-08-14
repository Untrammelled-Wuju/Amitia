package task_runtime

type TaskProviderBindingValidator interface {
	ValidateTaskProviderBinding(
		ctx interface{ Deadline() (interface{}, bool) },
		run *TaskRun,
		target TaskExecutionTarget,
	) error
}
