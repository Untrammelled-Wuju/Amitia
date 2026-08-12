package notification

type NotificationCapabilityState struct {
	Supported              bool   `json:"supported"`
	ListenerDeclared       bool   `json:"listenerDeclared"`
	ListenerGranted        bool   `json:"listenerGranted"`
	ListenerConnected      bool   `json:"listenerConnected"`
	PostPermissionRequired bool   `json:"postPermissionRequired"`
	PostPermissionGranted  bool   `json:"postPermissionGranted"`
	NotificationsEnabled   bool   `json:"notificationsEnabled"`
	CanRead                bool   `json:"canRead"`
	CanDismiss             bool   `json:"canDismiss"`
	CanPost                bool   `json:"canPost"`
	UserActionRequired     bool   `json:"userActionRequired"`
	State                  string `json:"state"`
}

type NotificationActionProjection struct {
	ActionRef      string `json:"actionRef"`
	Title          string `json:"title"`
	HasRemoteInput bool   `json:"hasRemoteInput"`
}

type NotificationProjection struct {
	NotificationID  string                         `json:"notificationRef"`
	PackageName     string                         `json:"packageName"`
	AppLabel        string                         `json:"appLabel"`
	PostedAt        int64                          `json:"postedAt"`
	Title           string                         `json:"title"`
	Text            string                         `json:"text"`
	SubText         string                         `json:"subText"`
	Category        string                         `json:"category"`
	Ongoing         bool                           `json:"ongoing"`
	Clearable       bool                           `json:"clearable"`
	GroupKey        string                         `json:"groupKey"`
	ChannelID       string                         `json:"channelId"`
	Importance      int                            `json:"importance"`
	HasContentAction bool                          `json:"hasContentAction"`
	Actions         []NotificationActionProjection `json:"actions,omitempty"`
	Generation      uint64                         `json:"generation"`
}

type ListInput struct {
	Limit          int    `json:"limit"`
	PackageName    string `json:"packageName,omitempty"`
	IncludeOngoing *bool  `json:"includeOngoing,omitempty"`
	IncludeOwn     *bool  `json:"includeOwn,omitempty"`
}

type GetInput struct {
	NotificationRef string `json:"notificationRef"`
}

type PostInput struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Channel string `json:"channel,omitempty"`
	Silent  *bool  `json:"silent,omitempty"`
}

type CancelOwnInput struct {
	NotificationRef string `json:"notificationRef"`
}

type DismissInput struct {
	NotificationRef string `json:"notificationRef"`
}

type OpenInput struct {
	NotificationRef string `json:"notificationRef"`
}

type InvokeActionInput struct {
	NotificationRef string `json:"notificationRef"`
	ActionRef       string `json:"actionRef"`
}

type StatusResult struct {
	Supported              bool   `json:"supported"`
	ListenerDeclared       bool   `json:"listenerDeclared"`
	ListenerGranted        bool   `json:"listenerGranted"`
	ListenerConnected      bool   `json:"listenerConnected"`
	PostPermissionRequired bool   `json:"postPermissionRequired"`
	PostPermissionGranted  bool   `json:"postPermissionGranted"`
	NotificationsEnabled   bool   `json:"notificationsEnabled"`
	CanRead                bool   `json:"canRead"`
	CanDismiss             bool   `json:"canDismiss"`
	CanPost                bool   `json:"canPost"`
	UserActionRequired     bool   `json:"userActionRequired"`
	State                  string `json:"state"`
}

type ListResult struct {
	Notifications []NotificationProjection `json:"notifications"`
	Count         int                      `json:"count"`
	FilteredCount int                      `json:"filteredCount"`
}

type GetResult struct {
	Notification *NotificationProjection `json:"notification"`
}

type PostResult struct {
	NotificationRef string `json:"notificationRef"`
	Posted          bool   `json:"posted"`
}

type CancelOwnResult struct {
	Cancelled bool `json:"cancelled"`
}

type DismissResult struct {
	Requested bool `json:"requested"`
	Dismissed *bool `json:"dismissed,omitempty"`
}

type OpenResult struct {
	Invoked bool `json:"invoked"`
}

type InvokeActionResult struct {
	Invoked bool `json:"invoked"`
}

const (
	DefaultListLimit = 50
	MaxListLimit     = 100
	TitleMaxChars    = 512
	BodyMaxChars     = 4096
	SubTextMaxChars  = 1024
	PostTitleMax     = 256
	PostBodyMax      = 4096
)

const (
	StateUnsupported                  = "unsupported"
	StateListenerNotGranted           = "listener_not_granted"
	StateListenerGrantedNotConnected  = "listener_granted_not_connected"
	StateListenerConnected            = "listener_connected"
	StatePostPermissionDenied         = "post_permission_denied"
)

const (
	OperationStatus       = "notification.status"
	OperationList         = "notification.list"
	OperationGet          = "notification.get"
	OperationPost         = "notification.post"
	OperationCancelOwn    = "notification.cancel_own"
	OperationDismiss      = "notification.dismiss"
	OperationOpen         = "notification.open"
	OperationInvokeAction = "notification.invoke_action"
)

const (
	ToolIDStatus       = "android.notification.status"
	ToolIDList         = "android.notification.list"
	ToolIDGet          = "android.notification.get"
	ToolIDPost         = "android.notification.post"
	ToolIDCancelOwn    = "android.notification.cancel_own"
	ToolIDDismiss      = "android.notification.dismiss"
	ToolIDOpen         = "android.notification.open"
	ToolIDInvokeAction = "android.notification.invoke_action"
)

const (
	ChannelAgentID   = "amitia_agent"
	ChannelAgentName = "Amitia"
	ChannelTaskID    = "amitia_task"
	ChannelTaskName  = "Amitia Task"
)

const (
	NotificationRefPrefix = "ntf_"
	OwnNotificationPrefix = "own_"
	ActionRefPrefix       = "act_"
)
