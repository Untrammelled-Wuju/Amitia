package executionjournal

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/u-ai/backend/internal/execution"
)

type RunPanicError struct {
	Value string
	Stack string
}

func (e *RunPanicError) Error() string {
	return fmt.Sprintf("执行账本：副作用执行回调 panic: %s\n%s", e.Value, e.Stack)
}

func (e *RunPanicError) Unwrap() error { return execution.ErrInvocationTransportLost }

func guardRun(run func() (json.RawMessage, error)) (result json.RawMessage, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = &RunPanicError{Value: fmt.Sprint(recovered), Stack: string(debug.Stack())}
		}
	}()
	return run()
}

func guardDispatch(dispatch func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &RunPanicError{Value: fmt.Sprint(recovered), Stack: string(debug.Stack())}
		}
	}()
	return dispatch()
}
