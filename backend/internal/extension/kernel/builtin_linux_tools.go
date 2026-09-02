//go:build linux && !android

package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/androidlinux/terminal"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func amitiaLinuxAvailable(provider interface{}) bool {
	_, ok := provider.(terminal.AndroidLinuxProvider)
	return ok
}

func amitiaLinuxCall(ctx context.Context, provider interface{}, invocation capability.ToolInvocationContext, operation string, payload map[string]any) (json.RawMessage, error) {
	linuxProvider, ok := provider.(terminal.AndroidLinuxProvider)
	if !ok || linuxProvider == nil {
		return nil, errors.New("Android Linux terminal provider unavailable")
	}
	requestID := invocation.InvocationID
	if requestID == "" {
		requestID = fmt.Sprintf("amitia-linux-%d", time.Now().UnixNano())
	}
	payload = normalizeJSONLikeMap(payload)
	payload["userId"] = invocation.UserID
	payload["characterId"] = invocation.CharacterID
	payload["conversationId"] = invocation.ConversationID
	resp := linuxProvider.Execute(ctx, terminal.AndroidLinuxRequest{
		RequestID: requestID + ":" + strings.ReplaceAll(operation, ".", "-"),
		Operation: operation,
		Payload:   payload,
	})
	if !strings.EqualFold(resp.Status, "success") && !strings.EqualFold(resp.Status, "ok") {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s failed [%s]: %s", operation, resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("%s failed with status %q", operation, resp.Status)
	}
	if resp.Result == nil {
		resp.Result = map[string]any{}
	}
	data, err := json.Marshal(resp.Result)
	return json.RawMessage(data), err
}
