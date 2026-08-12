package interaction

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/androidnative/adb"
)

type ADBExecutor struct {
	executor adb.InternalADBExecutor
	policy   Policy
}

func NewADBExecutor(executor adb.InternalADBExecutor, policy Policy) *ADBExecutor {
	return &ADBExecutor{
		executor: executor,
		policy:   policy,
	}
}

func (e *ADBExecutor) Tap(
	ctx context.Context,
	x int,
	y int,
) error {
	if e.executor == nil {
		return &Error{Code: INTERACTION_ADB_UNAVAILABLE, Message: "adb executor not available"}
	}

	if !e.policy.AllowADBFallback {
		return &Error{Code: INTERACTION_ADB_UNAVAILABLE, Message: "adb fallback not allowed by policy"}
	}

	result, err := e.executor.ExecuteArgs(
		ctx,
		"",
		[]string{"shell", "input", "tap", formatInt(x), formatInt(y)},
		adb.InternalADBExecuteOptions{
			Timeout: DefaultGestureTimeoutMS * time.Millisecond,
		},
	)
	if err != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "adb tap failed: " + err.Error()}
	}

	if result.ExitCode != 0 && result.ExitCodeAvailable {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "adb tap returned non-zero exit code"}
	}

	return nil
}

func (e *ADBExecutor) Swipe(
	ctx context.Context,
	startX, startY, endX, endY int,
	durationMS int,
) error {
	if e.executor == nil {
		return &Error{Code: INTERACTION_ADB_UNAVAILABLE, Message: "adb executor not available"}
	}

	if !e.policy.AllowADBFallback {
		return &Error{Code: INTERACTION_ADB_UNAVAILABLE, Message: "adb fallback not allowed by policy"}
	}

	if durationMS <= 0 {
		durationMS = DefaultSwipeDurationMS
	}

	result, err := e.executor.ExecuteArgs(
		ctx,
		"",
		[]string{"shell", "input", "swipe", formatInt(startX), formatInt(startY), formatInt(endX), formatInt(endY), formatInt(durationMS)},
		adb.InternalADBExecuteOptions{
			Timeout: DefaultGestureTimeoutMS * time.Millisecond,
		},
	)
	if err != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "adb swipe failed: " + err.Error()}
	}

	if result.ExitCode != 0 && result.ExitCodeAvailable {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "adb swipe returned non-zero exit code"}
	}

	return nil
}

func (e *ADBExecutor) InputText(
	ctx context.Context,
	text string,
) error {
	if e.executor == nil {
		return &Error{Code: INTERACTION_ADB_UNAVAILABLE, Message: "adb executor not available"}
	}

	if !e.policy.AllowADBFallback {
		return &Error{Code: INTERACTION_ADB_UNAVAILABLE, Message: "adb fallback not allowed by policy"}
	}

	if len([]rune(text)) > MaxInputTextRunes {
		return &Error{Code: INTERACTION_INVALID_REQUEST, Message: "text too large for adb input"}
	}

	result, err := e.executor.ExecuteArgs(
		ctx,
		"",
		[]string{"shell", "input", "text", text},
		adb.InternalADBExecuteOptions{
			Timeout: DefaultInputTimeoutMS * time.Millisecond,
		},
	)
	if err != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "adb input text failed: " + err.Error()}
	}

	if result.ExitCode != 0 && result.ExitCodeAvailable {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "adb input text returned non-zero exit code"}
	}

	return nil
}

func formatDuration(ms int) string {
	return fmt.Sprintf("%dms", ms)
}
