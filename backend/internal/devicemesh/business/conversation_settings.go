package business

import (
	"encoding/json"
	"errors"

	"github.com/u-ai/backend/internal/agentpermission"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type conversationSettings struct {
	MessageStyle     string `json:"messageStyle,omitempty"`
	ModelConfigID    int    `json:"modelConfigId,omitempty"`
	ReasoningEffort  string `json:"reasoningEffort,omitempty"`
	ReasoningEnabled *bool  `json:"reasoningEnabled,omitempty"`
	PermissionMode   string `json:"permissionMode,omitempty"`
}

func settingsFromRequest(request Request) conversationSettings {
	return conversationSettings{ModelConfigID: request.ModelConfigID, ReasoningEffort: request.ReasoningEffort, ReasoningEnabled: request.ReasoningEnabled, PermissionMode: request.PermissionMode, MessageStyle: request.MessageStyle}
}

func validateConversationSettings(request Request) error {
	if request.MessageStyle != "" && request.MessageStyle != "flow" && request.MessageStyle != "bubble" {
		return errors.New("聊天界面风格无效")
	}
	if request.ModelConfigID < 0 {
		return errors.New("模型配置编号无效")
	}
	switch request.ReasoningEffort {
	case "", "low", "medium", "high", "xhigh":
	default:
		return errors.New("推理强度无效")
	}
	if request.PermissionMode != "" && !agentpermission.Valid(request.PermissionMode) {
		return errors.New("权限模式无效")
	}
	return nil
}

func resolveConversationSettings(request *Request, snapshot coordination.DataSnapshot) error {
	var saved conversationSettings
	for _, resource := range snapshot.Resources {
		if resource.Kind == "conversation" && resource.ID == request.ConversationID && !resource.Deleted {
			if err := json.Unmarshal(resource.Body, &saved); err != nil {
				return err
			}
			break
		}
	}
	if request.ModelConfigID == 0 {
		request.ModelConfigID = saved.ModelConfigID
	}
	if request.ReasoningEffort == "" {
		request.ReasoningEffort = saved.ReasoningEffort
	}
	if request.ReasoningEnabled == nil {
		request.ReasoningEnabled = saved.ReasoningEnabled
	}
	if request.PermissionMode == "" {
		request.PermissionMode = saved.PermissionMode
	}
	if request.PermissionMode == "" {
		request.PermissionMode = agentpermission.RequestApproval
	}
	if request.MessageStyle == "" {
		request.MessageStyle = saved.MessageStyle
	}
	return validateConversationSettings(*request)
}

func applyConversationSettings(document map[string]any, request Request) {
	document["modelConfigId"] = request.ModelConfigID
	document["reasoningEffort"] = request.ReasoningEffort
	document["reasoningEnabled"] = request.ReasoningEnabled
	document["permissionMode"] = request.PermissionMode
	document["messageStyle"] = request.MessageStyle
}

func validateSettingsChanges(changes map[string]json.RawMessage) error {
	var settings conversationSettings
	encoded, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(encoded, &settings); err != nil {
		return errors.New("会话设置格式无效")
	}
	for _, key := range []string{"modelConfigId", "reasoningEffort", "permissionMode"} {
		if value, exists := changes[key]; exists && string(value) == "null" {
			return errors.New("会话设置不能为空")
		}
	}
	return validateConversationSettings(Request{ModelConfigID: settings.ModelConfigID, ReasoningEffort: settings.ReasoningEffort, ReasoningEnabled: settings.ReasoningEnabled, PermissionMode: settings.PermissionMode, MessageStyle: settings.MessageStyle})
}
