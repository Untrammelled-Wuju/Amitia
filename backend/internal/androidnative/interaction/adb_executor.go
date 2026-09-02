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

type adbStatusProvider interface {
	Status(ctx context.Context) adb.ADBStatus
}

func (e *ADBExecutor) ProbeHealth(ctx context.Context) ProviderCapabilityHealth {
	if e == nil || e.executor == nil {
		return newProviderHealth("adb", ProviderStateUnavailable, "adb executor not available", "", true)
	}
	if !e.policy.AllowADBFallback {
		return newProviderHealth("adb", ProviderStateUnavailable, "disabled by interaction policy", "", false)
	}
	probe, ok := e.executor.(adbStatusProvider)
	if !ok {
		return newProviderHealth("adb", ProviderStateSupported, "status probe not exposed by executor", "", true)
	}
	status := probe.Status(ctx)
	switch status.State {
	case adb.BackendReady:
		return newProviderHealth("adb", ProviderStateReady, "", "authorized_device", true)
	case adb.BackendUnauthorized:
		return newProviderHealth("adb", ProviderStatePermissionRequired, "adb device authorization required", "adb_authorization", true)
	case adb.BackendNoServer:
		return newProviderHealth("adb", ProviderStateStarting, "adb server is not available", "", true)
	case adb.BackendOffline, adb.BackendAmbiguous:
		return newProviderHealth("adb", ProviderStateDegraded, status.State, "", true)
	case adb.BackendNoDevice:
		return newProviderHealth("adb", ProviderStateUnavailable, "no adb device connected", "", true)
	default:
		return newProviderHealth("adb", ProviderStateUnavailable, status.State, "", true)
	}
}
