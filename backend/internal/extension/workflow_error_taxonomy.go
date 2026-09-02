package extension

import (
	"regexp"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type WorkflowErrorCategory string

const (
	WorkflowErrorConfiguration      WorkflowErrorCategory = "CONFIGURATION"
	WorkflowErrorPermission         WorkflowErrorCategory = "PERMISSION"
	WorkflowErrorDeviceOffline      WorkflowErrorCategory = "DEVICE_OFFLINE"
	WorkflowErrorDeviceRestricted   WorkflowErrorCategory = "DEVICE_RESTRICTED"
	WorkflowErrorRuntime            WorkflowErrorCategory = "RUNTIME"
	WorkflowErrorTool               WorkflowErrorCategory = "TOOL"
	WorkflowErrorTimeout            WorkflowErrorCategory = "TIMEOUT"
	WorkflowErrorCondition          WorkflowErrorCategory = "CONDITION"
	WorkflowErrorUIElementNotFound  WorkflowErrorCategory = "UI_ELEMENT_NOT_FOUND"
	WorkflowErrorUIActionNoEffect   WorkflowErrorCategory = "UI_ACTION_NO_EFFECT"
	WorkflowErrorVoice              WorkflowErrorCategory = "VOICE"
	WorkflowErrorNetwork            WorkflowErrorCategory = "NETWORK"
	WorkflowErrorUserActionRequired WorkflowErrorCategory = "USER_ACTION_REQUIRED"
	WorkflowErrorInternal           WorkflowErrorCategory = "INTERNAL"
)

type WorkflowErrorDiagnostic struct {
	Code              string                `json:"code"`
	Category          WorkflowErrorCategory `json:"category"`
	Message           string                `json:"message"`
	Recoverable       bool                  `json:"recoverable"`
	RecommendedAction string                `json:"recommendedAction,omitempty"`
	NodeID            string                `json:"nodeId,omitempty"`
	DeviceID          string                `json:"deviceId,omitempty"`
}

var workflowErrorCodePattern = regexp.MustCompile(`\b[A-Z][A-Z0-9_]{2,}\b`)

func classifyWorkflowRunError(run *workflow.WorkflowRun, steps []workflow.StepRun) *WorkflowErrorDiagnostic {
	if run == nil {
		return nil
	}
	message := strings.TrimSpace(run.Error)
	nodeID, deviceID := "", ""
	for i := len(steps) - 1; i >= 0; i-- {
		if strings.TrimSpace(steps[i].Error) == "" {
			continue
		}
		message = strings.TrimSpace(steps[i].Error)
		nodeID = strings.TrimSpace(steps[i].NodeID)
		deviceID = strings.TrimSpace(steps[i].DeviceID)
		break
	}
	if message == "" && run.Status == workflow.RunStatusWaitingDevice {
		message = strings.TrimSpace(run.PauseReason)
		if message == "" {
			message = "target device is offline or missing a required capability"
		}
	}
	if message == "" {
		return nil
	}
	lower := strings.ToLower(message)
	category := WorkflowErrorInternal
	recoverable := false
	action := "查看运行 Timeline 和 Trace，确认失败节点后重试"

	switch {
	case run.Status == workflow.RunStatusWaitingDevice || containsAny(lower, "device offline", "device unavailable", "waiting_device", "capability_device_offline"):
		category, recoverable, action = WorkflowErrorDeviceOffline, true, "等待设备重新上线，或选择具备所需 Capability 的其他设备"
	case containsAny(lower, "background restricted", "doze", "battery saver", "device restricted", "screen locked", "keyguard", "device_waiting_unlock", "device_waiting_screen", "device_background_restricted"):
		category, recoverable, action = WorkflowErrorDeviceRestricted, true, "解除设备后台/锁屏限制后继续原运行"
	case containsAny(lower, "permission denied", "permission required", "accessibility", "not authorized", "forbidden"):
		category, recoverable, action = WorkflowErrorPermission, true, "在目标设备确认并授予所需权限，然后重新预检"
	case containsAny(lower, "user_action_required", "requires_user_action", "manual_intervention_required"):
		category, recoverable, action = WorkflowErrorUserActionRequired, true, "按提示完成用户确认或修复动作后继续"
	case containsAny(lower, "ui_element_not_found", "element not found", "target not found", "node not found"):
		category, recoverable, action = WorkflowErrorUIElementNotFound, true, "重新观察当前页面，必要时启用视觉定位后重试"
	case containsAny(lower, "ui_action_no_effect", "action_no_effect", "no effect"):
		category, recoverable, action = WorkflowErrorUIActionNoEffect, true, "重新观察并改用语义重匹配、坐标或视觉 fallback"
	case containsAny(lower, "timeout", "deadline exceeded", "timed out"):
		category, recoverable, action = WorkflowErrorTimeout, true, "检查目标服务/设备状态，并按节点 Retry Policy 重试"
	case containsAny(lower, "condition", "postcondition", "when expression", "expression"):
		category, recoverable, action = WorkflowErrorCondition, false, "检查 When/Postcondition 表达式与上游输出"
	case containsAny(lower, "wake", "kws", "microphone", "asr", "voice"):
		category, recoverable, action = WorkflowErrorVoice, true, "检查麦克风权限、Wake 模型状态和语音引擎"
	case containsAny(lower, "network", "connection", "socket", "transport unavailable", "dns", "websocket"):
		category, recoverable, action = WorkflowErrorNetwork, true, "检查网络和设备会话，连接恢复后继续或重试"
	case containsAny(lower, "runtime", "process exited", "child exited", "start_failed", "backend health"):
		category, recoverable, action = WorkflowErrorRuntime, true, "查看 Runtime 诊断；若自动恢复已熔断，先验证或重新安装 Runtime"
	case containsAny(lower, "tool", "handler", "provider"):
		category, recoverable, action = WorkflowErrorTool, true, "检查 Tool 参数、Provider Capability 和节点 Trace"
	case containsAny(lower, "invalid", "missing", "required", "schema", "configuration", "config"):
		category, recoverable, action = WorkflowErrorConfiguration, false, "修正工作流配置后重新执行预检"
	}

	code := "WORKFLOW_ERROR"
	if match := workflowErrorCodePattern.FindString(strings.ToUpper(message)); match != "" {
		code = match
	} else {
		code = "WORKFLOW_" + string(category)
	}
	return &WorkflowErrorDiagnostic{
		Code:              code,
		Category:          category,
		Message:           message,
		Recoverable:       recoverable,
		RecommendedAction: action,
		NodeID:            nodeID,
		DeviceID:          deviceID,
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
