package shell

import "fmt"

type Error struct {
	code    string
	message string
}

func (e *Error) Error() string {
	return e.code + ": " + e.message
}

func (e *Error) Code() string {
	return e.code
}

func (e *Error) Message() string {
	return e.message
}

const (
	ErrCodeShellNotAvailable   = "shell.not_available"
	ErrCodeInvalidMode         = "shell.invalid_mode"
	ErrCodeCommandRequired     = "shell.command_required"
	ErrCodeExecutableRequired  = "shell.executable_required"
	ErrCodeInvalidExecutable   = "shell.invalid_executable"
	ErrCodeEnvironmentDenied   = "shell.environment_denied"
	ErrCodeTooManyEnvEntries   = "shell.too_many_env_entries"
	ErrCodeEnvDataTooLarge     = "shell.env_data_too_large"
	ErrCodeStdinTooLarge       = "shell.stdin_too_large"
	ErrCodeTimeoutExceeded     = "shell.timeout_exceeded"
	ErrCodeWorkingDirInvalid   = "shell.working_dir_invalid"
	ErrCodeExecutionFailed     = "shell.execution_failed"
	ErrCodeProcessStartFailed  = "shell.process_start_failed"
	ErrCodeCancelled           = "shell.cancelled"
	ErrCodeOutputLimitExceeded = "shell.output_limit_exceeded"
)

func ErrShellNotAvailable(reason string) *Error {
	return &Error{code: ErrCodeShellNotAvailable, message: "shell not available: " + reason}
}

func ErrInvalidMode(mode string) *Error {
	return &Error{code: ErrCodeInvalidMode, message: "invalid shell mode: " + mode}
}

func ErrCommandRequired() *Error {
	return &Error{code: ErrCodeCommandRequired, message: "command is required for shell mode"}
}

func ErrExecutableRequired() *Error {
	return &Error{code: ErrCodeExecutableRequired, message: "executable is required for argv mode"}
}

func ErrInvalidExecutable(name string) *Error {
	return &Error{code: ErrCodeInvalidExecutable, message: "invalid executable name: " + name}
}

func ErrEnvironmentDenied(key string) *Error {
	return &Error{code: ErrCodeEnvironmentDenied, message: "environment variable not allowed: " + key}
}

func ErrTooManyEnvEntries(count, max int) *Error {
	return &Error{code: ErrCodeTooManyEnvEntries, message: fmt.Sprintf("too many environment entries: %d > %d", count, max)}
}

func EnvDataTooLarge(size, max int64) *Error {
	return &Error{code: ErrCodeEnvDataTooLarge, message: fmt.Sprintf("environment data too large: %d > %d", size, max)}
}

func ErrStdinTooLarge(size, max int64) *Error {
	return &Error{code: ErrCodeStdinTooLarge, message: fmt.Sprintf("stdin too large: %d > %d", size, max)}
}

func ErrTimeoutExceeded(timeout int64) *Error {
	return &Error{code: ErrCodeTimeoutExceeded, message: fmt.Sprintf("timeout exceeded: %dms", timeout)}
}

func ErrWorkingDirInvalid(path string, reason string) *Error {
	return &Error{code: ErrCodeWorkingDirInvalid, message: fmt.Sprintf("invalid working directory %s: %s", path, reason)}
}

func ErrExecutionFailed(reason string) *Error {
	return &Error{code: ErrCodeExecutionFailed, message: "execution failed: " + reason}
}

func ErrProcessStartFailed(reason string) *Error {
	return &Error{code: ErrCodeProcessStartFailed, message: "process start failed: " + reason}
}

func ErrCancelled() *Error {
	return &Error{code: ErrCodeCancelled, message: "shell execution cancelled"}
}

func ErrOutputLimitExceeded(stream string, limit int64) *Error {
	return &Error{code: ErrCodeOutputLimitExceeded, message: fmt.Sprintf("%s output exceeded limit: %d bytes", stream, limit)}
}
