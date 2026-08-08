package domain

type ControlMode string

const (
	ControlModeObserveOnly   ControlMode = "observe_only"
	ControlModeAssist        ControlMode = "assist"
	ControlModeSharedControl ControlMode = "shared_control"
	ControlModePluginControl ControlMode = "plugin_control"
	ControlModeUserControl   ControlMode = "user_control"
	ControlModeSuspended     ControlMode = "suspended"
)
