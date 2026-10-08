package notificationruntime

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
)

type RegisterDeviceRequest struct {
	DeviceID                     string   `json:"deviceId"`
	Platform                     string   `json:"platform"`
	DeviceName                   string   `json:"deviceName"`
	AppVersion                   string   `json:"appVersion"`
	OSVersion                    string   `json:"osVersion"`
	Locale                       string   `json:"locale"`
	Appearance                   string   `json:"appearance"`
	Timezone                     string   `json:"timezone"`
	PreferredProvider            string   `json:"preferredProvider"`
	NativeDataProviders          []string `json:"nativeDataProviders"`
	FCMToken                     string   `json:"fcmToken"`
	APNSToken                    string   `json:"apnsToken"`
	VoIPToken                    string   `json:"voipToken"`
	HMSToken                     string   `json:"hmsToken"`
	MiPushToken                  string   `json:"miPushToken"`
	OppoToken                    string   `json:"oppoToken"`
	VivoToken                    string   `json:"vivoToken"`
	HonorToken                   string   `json:"honorToken"`
	LiveActivityPushToStartToken string   `json:"liveActivityPushToStartToken"`
	ClearTokens                  []string `json:"clearTokens"`
	PushEnabled                  *bool    `json:"pushEnabled"`
	SystemNotificationsEnabled   *bool    `json:"systemNotificationsEnabled"`
	MessagePushEnabled           *bool    `json:"messagePushEnabled"`
	ExecutionActivityEnabled     *bool    `json:"executionActivityEnabled"`
	CallPushEnabled              *bool    `json:"callPushEnabled"`
	ReminderPushEnabled          *bool    `json:"reminderPushEnabled"`
	PreviewMode                  string   `json:"previewMode"`
	SoundEnabled                 *bool    `json:"soundEnabled"`
	LiveActivitySupported        bool     `json:"liveActivitySupported"`
	DynamicIslandSupported       bool     `json:"dynamicIslandSupported"`
	ProgressStyleSupported       bool     `json:"progressStyleSupported"`
	CommunicationSupported       bool     `json:"communicationSupported"`
}

type UpdatePreferencesRequest struct {
	PushEnabled              *bool   `json:"pushEnabled"`
	MessagePushEnabled       *bool   `json:"messagePushEnabled"`
	ExecutionActivityEnabled *bool   `json:"executionActivityEnabled"`
	CallPushEnabled          *bool   `json:"callPushEnabled"`
	ReminderPushEnabled      *bool   `json:"reminderPushEnabled"`
	PreviewMode              *string `json:"previewMode"`
	SoundEnabled             *bool   `json:"soundEnabled"`
}

type PresenceRequest struct {
	DeviceID             string `json:"deviceId"`
	Foreground           bool   `json:"foreground"`
	ActiveConversationID string `json:"activeConversationId"`
}

type LiveActivityTokenRequest struct {
	DeviceID       string `json:"deviceId"`
	ConversationID string `json:"conversationId"`
	RunID          string `json:"runId"`
	ActivityID     string `json:"activityId"`
	UpdateToken    string `json:"updateToken"`
	Revision       int64  `json:"revision"`
}

type IncomingCallRequest struct {
	CallID         string `json:"callId"`
	ConversationID string `json:"conversationId"`
	CharacterID    string `json:"characterId"`
	CallerName     string `json:"callerName"`
	CallType       string `json:"callType"`
}

type EndCallRequest struct {
	ConversationID string `json:"conversationId"`
	Reason         string `json:"reason"`
}

func RegisterRoutes(router *gin.RouterGroup, runtime *Runtime) {
	if router == nil || runtime == nil {
		return
	}
	group := router.Group("/notifications")
	group.GET("/capabilities", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": runtime.Capabilities()})
	})
	group.GET("/devices", func(c *gin.Context) {
		spaceID, _, ok := actorScope(c, "")
		if !ok {
			return
		}
		rows, err := runtime.repo.ListDevices(c.Request.Context(), spaceID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": rows})
	})
	group.POST("/devices/register", func(c *gin.Context) {
		var request RegisterDeviceRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid notification device"})
			return
		}
		spaceID, deviceID, ok := actorScope(c, request.DeviceID)
		if !ok {
			return
		}
		requestPreviewMode := strings.ToLower(strings.TrimSpace(request.PreviewMode))
		if requestPreviewMode != "" && requestPreviewMode != "full" && requestPreviewMode != "sender_only" && requestPreviewMode != "hidden" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid preview mode"})
			return
		}
		existing, _ := runtime.repo.GetDeviceForPlatform(c.Request.Context(), spaceID, deviceID, request.Platform)
		clearTokens := tokenClearSet(request.ClearTokens)
		previewMode := requestPreviewMode
		if previewMode == "" {
			if existing != nil && strings.TrimSpace(existing.PreviewMode) != "" {
				previewMode = existing.PreviewMode
			} else {
				previewMode = "full"
			}
		}
		now := time.Now().UTC()
		endpoint := &DeviceEndpoint{
			SpaceID:                      spaceID,
			DeviceID:                     deviceID,
			Platform:                     request.Platform,
			DeviceName:                   request.DeviceName,
			AppVersion:                   request.AppVersion,
			OSVersion:                    request.OSVersion,
			Locale:                       request.Locale,
			Appearance:                   normalizeAppearance(request.Appearance),
			Timezone:                     request.Timezone,
			PreferredProvider:            request.PreferredProvider,
			NativeDataProviders:          normalizeNativeDataProviders(request.NativeDataProviders),
			FCMToken:                     strings.TrimSpace(request.FCMToken),
			APNSToken:                    strings.TrimSpace(request.APNSToken),
			VoIPToken:                    strings.TrimSpace(request.VoIPToken),
			HMSToken:                     strings.TrimSpace(request.HMSToken),
			MiPushToken:                  strings.TrimSpace(request.MiPushToken),
			OppoToken:                    strings.TrimSpace(request.OppoToken),
			VivoToken:                    strings.TrimSpace(request.VivoToken),
			HonorToken:                   strings.TrimSpace(request.HonorToken),
			LiveActivityPushToStartToken: strings.TrimSpace(request.LiveActivityPushToStartToken),
			PushEnabled:                  boolDefault(request.PushEnabled, true),
			SystemNotificationsEnabled:   boolDefault(request.SystemNotificationsEnabled, true),
			MessagePushEnabled:           boolDefault(request.MessagePushEnabled, true),
			ExecutionActivityEnabled:     boolDefault(request.ExecutionActivityEnabled, true),
			CallPushEnabled:              boolDefault(request.CallPushEnabled, true),
			ReminderPushEnabled:          boolDefault(request.ReminderPushEnabled, true),
			PreviewMode:                  previewMode,
			SoundEnabled:                 boolDefault(request.SoundEnabled, true),
			LiveActivitySupported:        request.LiveActivitySupported,
			DynamicIslandSupported:       request.DynamicIslandSupported,
			ProgressStyleSupported:       request.ProgressStyleSupported,
			CommunicationSupported:       request.CommunicationSupported,
			TokenUpdatedAt:               now,
		}
		if existing != nil {
			if request.PushEnabled == nil {
				endpoint.PushEnabled = existing.PushEnabled
			}
			if request.SystemNotificationsEnabled == nil {
				endpoint.SystemNotificationsEnabled = existing.SystemNotificationsEnabled
			}
			if request.MessagePushEnabled == nil {
				endpoint.MessagePushEnabled = existing.MessagePushEnabled
			}
			if request.ExecutionActivityEnabled == nil {
				endpoint.ExecutionActivityEnabled = existing.ExecutionActivityEnabled
			}
			if request.CallPushEnabled == nil {
				endpoint.CallPushEnabled = existing.CallPushEnabled
			}
			if request.ReminderPushEnabled == nil {
				endpoint.ReminderPushEnabled = existing.ReminderPushEnabled
			}
			if request.SoundEnabled == nil {
				endpoint.SoundEnabled = existing.SoundEnabled
			}
			if strings.TrimSpace(endpoint.DeviceName) == "" {
				endpoint.DeviceName = existing.DeviceName
			}
			if strings.TrimSpace(endpoint.AppVersion) == "" {
				endpoint.AppVersion = existing.AppVersion
			}
			if strings.TrimSpace(endpoint.OSVersion) == "" {
				endpoint.OSVersion = existing.OSVersion
			}
			if strings.TrimSpace(endpoint.Locale) == "" {
				endpoint.Locale = existing.Locale
			}
			if endpoint.Appearance == "" {
				endpoint.Appearance = existing.Appearance
			}
			if strings.TrimSpace(endpoint.Timezone) == "" {
				endpoint.Timezone = existing.Timezone
			}
			if strings.TrimSpace(endpoint.PreferredProvider) == "" {
				endpoint.PreferredProvider = existing.PreferredProvider
			}
			if endpoint.FCMToken == "" && !clearTokens["fcm"] {
				endpoint.FCMToken = existing.FCMToken
			}
			if endpoint.APNSToken == "" && !clearTokens["apns"] {
				endpoint.APNSToken = existing.APNSToken
			}
			if endpoint.VoIPToken == "" && !clearTokens["apns-voip"] {
				endpoint.VoIPToken = existing.VoIPToken
			}
			if endpoint.HMSToken == "" && !clearTokens["hms"] {
				endpoint.HMSToken = existing.HMSToken
			}
			if endpoint.MiPushToken == "" && !clearTokens["mipush"] {
				endpoint.MiPushToken = existing.MiPushToken
			}
			if endpoint.OppoToken == "" && !clearTokens["oppo"] {
				endpoint.OppoToken = existing.OppoToken
			}
			if endpoint.VivoToken == "" && !clearTokens["vivo"] {
				endpoint.VivoToken = existing.VivoToken
			}
			if endpoint.HonorToken == "" && !clearTokens["honor"] {
				endpoint.HonorToken = existing.HonorToken
			}
			if endpoint.LiveActivityPushToStartToken == "" {
				endpoint.LiveActivityPushToStartToken = existing.LiveActivityPushToStartToken
			}
			if strings.TrimSpace(request.FCMToken) == "" &&
				strings.TrimSpace(request.APNSToken) == "" &&
				strings.TrimSpace(request.VoIPToken) == "" &&
				strings.TrimSpace(request.HMSToken) == "" &&
				strings.TrimSpace(request.MiPushToken) == "" &&
				strings.TrimSpace(request.OppoToken) == "" &&
				strings.TrimSpace(request.VivoToken) == "" &&
				strings.TrimSpace(request.HonorToken) == "" &&
				strings.TrimSpace(request.LiveActivityPushToStartToken) == "" &&
				len(clearTokens) == 0 {
				endpoint.TokenUpdatedAt = existing.TokenUpdatedAt
			}
		}
		if err := runtime.repo.UpsertDevice(c.Request.Context(), endpoint); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		registered, _ := runtime.repo.GetDevice(c.Request.Context(), spaceID, deviceID)
		if registered != nil {
			go runtime.CatchUpLiveActivity(*registered)
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": registered})
	})
	group.POST("/devices/:deviceId/test", func(c *gin.Context) {
		spaceID, deviceID, ok := actorScope(c, c.Param("deviceId"))
		if !ok {
			return
		}
		result, err := runtime.TestDevicePush(c.Request.Context(), spaceID, deviceID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": result})
	})
	group.PATCH("/devices/:deviceId/preferences", func(c *gin.Context) {
		spaceID, deviceID, ok := actorScope(c, c.Param("deviceId"))
		if !ok {
			return
		}
		var request UpdatePreferencesRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid notification preferences"})
			return
		}
		values := map[string]any{}
		if request.PushEnabled != nil {
			values["push_enabled"] = *request.PushEnabled
		}
		if request.MessagePushEnabled != nil {
			values["message_push_enabled"] = *request.MessagePushEnabled
		}
		if request.ExecutionActivityEnabled != nil {
			values["execution_activity_enabled"] = *request.ExecutionActivityEnabled
		}
		if request.CallPushEnabled != nil {
			values["call_push_enabled"] = *request.CallPushEnabled
		}
		if request.ReminderPushEnabled != nil {
			values["reminder_push_enabled"] = *request.ReminderPushEnabled
		}
		if request.SoundEnabled != nil {
			values["sound_enabled"] = *request.SoundEnabled
		}
		if request.PreviewMode != nil {
			mode := strings.ToLower(strings.TrimSpace(*request.PreviewMode))
			if mode != "full" && mode != "sender_only" && mode != "hidden" {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid preview mode"})
				return
			}
			values["preview_mode"] = mode
		}
		if err := runtime.repo.UpdatePreferences(c.Request.Context(), spaceID, deviceID, values); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
			return
		}
		current, err := runtime.repo.GetDevice(c.Request.Context(), spaceID, deviceID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
			return
		}
		if err := runtime.repo.CancelPendingOutboxByPreferences(
			c.Request.Context(),
			spaceID,
			deviceID,
			!current.PushEnabled,
			!current.MessagePushEnabled,
			!current.ExecutionActivityEnabled,
			!current.CallPushEnabled,
			!current.ReminderPushEnabled,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
			return
		}
		if !current.PushEnabled || !current.ExecutionActivityEnabled {
			if err := runtime.CleanupExecutionNotifications(c.Request.Context(), spaceID, deviceID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": current})
	})
	group.POST("/presence", func(c *gin.Context) {
		var request PresenceRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid presence"})
			return
		}
		spaceID, deviceID, ok := actorScope(c, request.DeviceID)
		if !ok {
			return
		}
		if err := runtime.repo.UpdatePresence(c.Request.Context(), spaceID, deviceID, request.Foreground, request.ActiveConversationID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok"})
	})
	group.POST("/live-activities/token", func(c *gin.Context) {
		var request LiveActivityTokenRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid live activity token"})
			return
		}
		spaceID, deviceID, ok := actorScope(c, request.DeviceID)
		if !ok {
			return
		}
		runID := strings.TrimSpace(request.RunID)
		if terminal, err := runtime.repo.IsRunTerminal(
			c.Request.Context(),
			spaceID,
			deviceID,
			runID,
		); err == nil && terminal {
			// A late ActivityKit token callback must never revive a run that the
			// Cloud Core has already ended. Treat it as an idempotent no-op.
			c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok"})
			return
		}
		value := &LiveActivityToken{
			SpaceID:        spaceID,
			DeviceID:       deviceID,
			ConversationID: strings.TrimSpace(request.ConversationID),
			RunID:          runID,
			ActivityID:     strings.TrimSpace(request.ActivityID),
			UpdateToken:    strings.TrimSpace(request.UpdateToken),
			Revision:       request.Revision,
		}
		if err := runtime.repo.UpsertLiveActivityToken(c.Request.Context(), value); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		if endpoint, err := runtime.repo.GetDevice(c.Request.Context(), spaceID, deviceID); err == nil && endpoint != nil {
			go runtime.CatchUpLiveActivityRun(*endpoint, value.RunID)
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok"})
	})
	group.POST("/calls/incoming", func(c *gin.Context) {
		var request IncomingCallRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid incoming call"})
			return
		}
		spaceID, _, ok := actorScope(c, "")
		if !ok {
			return
		}
		count, err := runtime.PushIncomingCall(c.Request.Context(), spaceID, IncomingCall{
			CallID:         request.CallID,
			ConversationID: request.ConversationID,
			CharacterID:    request.CharacterID,
			CallerName:     request.CallerName,
			CallType:       request.CallType,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{"queuedDevices": count}})
	})
	group.POST("/calls/:callId/answer", func(c *gin.Context) {
		var request EndCallRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid call answer"})
			return
		}
		spaceID, deviceID, ok := actorScope(c, "")
		if !ok {
			return
		}
		count, err := runtime.AnswerCall(
			c.Request.Context(),
			spaceID,
			deviceID,
			c.Param("callId"),
			request.ConversationID,
		)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "ok",
			"data": gin.H{"dismissedDevices": count},
		})
	})
	group.POST("/calls/:callId/end", func(c *gin.Context) {
		var request EndCallRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid call end"})
			return
		}
		spaceID, _, ok := actorScope(c, "")
		if !ok {
			return
		}
		count, err := runtime.EndCall(c.Request.Context(), spaceID, c.Param("callId"), request.ConversationID, request.Reason)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": gin.H{"queuedDevices": count}})
	})
	group.DELETE("/devices/:deviceId", func(c *gin.Context) {
		spaceID, deviceID, ok := actorScope(c, c.Param("deviceId"))
		if !ok {
			return
		}
		if err := runtime.repo.RevokeDevice(c.Request.Context(), spaceID, deviceID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok"})
	})
}

func actorScope(c *gin.Context, requestedDeviceID string) (string, string, bool) {
	actor, ok := auth.FromContext(c.Request.Context())
	if !ok || actor == nil || strings.TrimSpace(string(actor.SpaceID)) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "missing authenticated space"})
		return "", "", false
	}
	spaceID := strings.TrimSpace(string(actor.SpaceID))
	deviceID := strings.TrimSpace(requestedDeviceID)
	actorDeviceID := strings.TrimSpace(string(actor.DeviceID))
	if actorDeviceID != "" {
		if deviceID != "" && deviceID != actorDeviceID {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "device identity mismatch"})
			return "", "", false
		}
		deviceID = actorDeviceID
	}
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "deviceId is required"})
		return "", "", false
	}
	return spaceID, deviceID, true
}

func normalizeNativeDataProviders(values []string) string {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(values))
	for _, raw := range values {
		provider := strings.ToLower(strings.TrimSpace(raw))
		switch provider {
		case "hms", "mipush", "honor", "oppo", "vivo":
			if !seen[provider] {
				seen[provider] = true
				normalized = append(normalized, provider)
			}
		}
	}
	sort.Strings(normalized)
	return strings.Join(normalized, ",")
}

func normalizeAppearance(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "light" || value == "dark" {
		return value
	}
	return ""
}

func tokenClearSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		switch key {
		case "fcm", "apns", "apns-voip", "hms", "mipush", "oppo", "vivo", "honor":
			result[key] = true
		}
	}
	return result
}

func boolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
