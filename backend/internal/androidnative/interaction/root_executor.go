package interaction

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/androidnative/root"
)

type RootExecutor struct {
	executor root.InternalRootExecutor
	policy   Policy
}

func NewRootExecutor(executor root.InternalRootExecutor, policy Policy) *RootExecutor {
	return &RootExecutor{
		executor: executor,
		policy:   policy,
	}
}

func (e *RootExecutor) Tap(
	ctx context.Context,
	x int,
	y int,
) error {
	if e.executor == nil {
		return &Error{Code: INTERACTION_ROOT_UNAVAILABLE, Message: "root executor not available"}
	}

	if !e.policy.AllowRootFallback {
		return &Error{Code: INTERACTION_ROOT_UNAVAILABLE, Message: "root fallback not allowed by policy"}
	}

	req := root.ExecuteRequest{
		Executable: "input",
		Args:       []string{"tap", formatInt(x), formatInt(y)},
		TimeoutMS:  int(DefaultGestureTimeoutMS),
	}

	opts := root.InternalExecuteOptions{
		Timeout: DefaultGestureTimeoutMS,
	}

	result, err := e.executor.ExecuteRoot(ctx, req, opts)
	if err != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "root tap failed: " + err.Error()}
	}

	if result.ExitCode != 0 && result.ExitCodeAvailable {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "root tap returned non-zero exit code"}
	}

	return nil
}

func (e *RootExecutor) Swipe(
	ctx context.Context,
	startX, startY, endX, endY int,
	durationMS int,
) error {
	if e.executor == nil {
		return &Error{Code: INTERACTION_ROOT_UNAVAILABLE, Message: "root executor not available"}
	}

	if !e.policy.AllowRootFallback {
		return &Error{Code: INTERACTION_ROOT_UNAVAILABLE, Message: "root fallback not allowed by policy"}
	}

	req := root.ExecuteRequest{
		Executable: "input",
		Args:       []string{"swipe", formatInt(startX), formatInt(startY), formatInt(endX), formatInt(endY), formatInt(durationMS)},
		TimeoutMS:  int(DefaultGestureTimeoutMS),
	}

	opts := root.InternalExecuteOptions{
		Timeout: DefaultGestureTimeoutMS,
	}

	result, err := e.executor.ExecuteRoot(ctx, req, opts)
	if err != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "root swipe failed: " + err.Error()}
	}

	if result.ExitCode != 0 && result.ExitCodeAvailable {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "root swipe returned non-zero exit code"}
	}

	return nil
}

func (e *RootExecutor) InputText(
	ctx context.Context,
	text string,
) error {
	if e.executor == nil {
		return &Error{Code: INTERACTION_ROOT_UNAVAILABLE, Message: "root executor not available"}
	}

	if !e.policy.AllowRootFallback {
		return &Error{Code: INTERACTION_ROOT_UNAVAILABLE, Message: "root fallback not allowed by policy"}
	}

	if len([]rune(text)) > MaxInputTextRunes {
		return &Error{Code: INTERACTION_INVALID_REQUEST, Message: "text too large for root input"}
	}

	req := root.ExecuteRequest{
		Executable: "input",
		Args:       []string{"text", text},
		TimeoutMS:  int(DefaultInputTimeoutMS),
	}

	opts := root.InternalExecuteOptions{
		Timeout: DefaultInputTimeoutMS,
	}

	result, err := e.executor.ExecuteRoot(ctx, req, opts)
	if err != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "root input text failed: " + err.Error()}
	}

	if result.ExitCode != 0 && result.ExitCodeAvailable {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "root input text returned non-zero exit code"}
	}

	return nil
}

func formatInt(v int) string {
	return fmt.Sprintf("%d", v)
}

type rootStatusProvider interface {
	Status(ctx context.Context) (map[string]any, error)
}

func (e *RootExecutor) ProbeHealth(ctx context.Context) ProviderCapabilityHealth {
	if e == nil || e.executor == nil {
		return newProviderHealth("root", ProviderStateUnavailable, "root executor not available", "", true)
	}
	if !e.policy.AllowRootFallback {
		return newProviderHealth("root", ProviderStateUnavailable, "disabled by interaction policy", "", false)
	}
	probe, ok := e.executor.(rootStatusProvider)
	if !ok {
		return newProviderHealth("root", ProviderStateSupported, "status probe not exposed by executor", "", true)
	}
	result, err := probe.Status(ctx)
	if err != nil {
		return newProviderHealth("root", ProviderStateFailed, err.Error(), "", true)
	}
	return healthFromBridgeResponse("root", result, nil)
}
