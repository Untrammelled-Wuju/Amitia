// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	appLog "github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

var dbInstance *gorm.DB

func SetDB(db *gorm.DB) { dbInstance = db }

func HandleSession(c *gin.Context) {
	appLog.Info("HandleSession ENTER")

	voiceType := strings.TrimSpace(c.Query("voiceType"))
	if voiceType == "" {
		voiceType = "zh_female_vv_jupiter_bigtts"
	}
	resourceID := strings.TrimSpace(c.Query("resourceId"))
	if resourceID == "" {
		resourceID = "volc.speech.dialog"
	}

	// New realtime clients never need provider credentials. The query/header
	// fallback only keeps legacy clients working while configuration migrates.
	legacyAccessToken := strings.TrimSpace(c.Query("apiKey"))
	if legacyAccessToken == "" {
		legacyAccessToken = strings.TrimSpace(c.GetHeader("X-Tts-Api-Key"))
	}
	legacyAppID := strings.TrimSpace(c.Query("appId"))
	if legacyAppID == "" {
		legacyAppID = strings.TrimSpace(c.GetHeader("X-Api-App-ID"))
	}

	realtimeAppID := legacyAppID
	realtimeAccessToken := legacyAccessToken
	var ttsCfg struct {
		RealtimeAppID       string `gorm:"column:realtime_app_id"`
		RealtimeAccessToken string `gorm:"column:realtime_access_token"`
		RealtimeSecretKey   string `gorm:"column:realtime_secret_key"`
	}
	if dbInstance != nil {
		_ = dbInstance.Table("tts_configs").
			Where("is_active = 1").
			Select("realtime_app_id, realtime_access_token, realtime_secret_key").
			First(&ttsCfg).Error
		if strings.TrimSpace(ttsCfg.RealtimeAppID) != "" {
			realtimeAppID = strings.TrimSpace(ttsCfg.RealtimeAppID)
		}
		if strings.TrimSpace(ttsCfg.RealtimeAccessToken) != "" {
			realtimeAccessToken = strings.TrimSpace(ttsCfg.RealtimeAccessToken)
		}
	}

	providerAppKey := strings.TrimSpace(ttsCfg.RealtimeSecretKey)
	if providerAppKey == "" {
		providerAppKey = strings.TrimSpace(os.Getenv("AMITIA_VOLCANO_APP_KEY"))
	}
	if realtimeAccessToken == "" || realtimeAppID == "" || providerAppKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    http.StatusServiceUnavailable,
			"message": "realtime provider is not fully configured on the server",
		})
		return
	}

	conversationID := strings.TrimSpace(c.Query("conversationId"))
	dialogID := strings.TrimSpace(c.Query("dialogId"))
	requestUserID := ""
	if value, exists := c.Get("realtimeUserId"); exists && value != nil {
		requestUserID = strings.TrimSpace(fmt.Sprint(value))
	} else if value, exists := c.Get("userId"); exists && value != nil {
		requestUserID = strings.TrimSpace(fmt.Sprint(value))
	}
	desktopPetCharacterID := ""
	desktopPetUserID := ""

	systemRole := ""
	botName := "AI"
	if dbInstance != nil && conversationID != "" {
		var conv struct{ CID string }
		dbInstance.Table("conversations").Where("id = ?", conversationID).Select("character_id as cid").First(&conv)
		if conv.CID != "" {
			desktopPetCharacterID = conv.CID
			var activePet struct {
				UserID string `gorm:"column:user_id"`
			}
			dbInstance.Table("desktop_pet_installations").
				Where("character_id = ? AND is_active = 1", conv.CID).
				Order("updated_at DESC").
				Select("user_id").
				First(&activePet)
			desktopPetUserID = activePet.UserID

			var ch struct{ N, SP, SS, VT, CVID, VM string }
			dbInstance.Table("characters").Where("id = ?", conv.CID).Select("name as n, character_base as sp, speaking_style as ss, voice_type as vt, custom_voice_id as cvid, voice_mode as vm").First(&ch)
			if ch.VM == "clone" && ch.CVID != "" {
				voiceType = ch.CVID
			} else if ch.VT != "" {
				voiceType = ch.VT
			}
			if ch.N != "" {
				botName = ch.N
			}
			if ch.SP != "" {
				systemRole = ch.SP
			}
			if systemRole == "" && ch.SS != "" {
				systemRole = ch.SS
			}
		}
	}

	browserConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer browserConn.Close()
	appLog.Info("browser WS upgraded")

	var browserWriteMu sync.Mutex
	writeBrowserJSON := func(value interface{}) error {
		browserWriteMu.Lock()
		defer browserWriteMu.Unlock()
		return browserConn.WriteJSON(value)
	}

	providerHeaders := http.Header{}
	providerHeaders.Set("X-Api-App-Key", providerAppKey)
	providerHeaders.Set("X-Api-Access-Key", realtimeAccessToken)
	providerHeaders.Set("X-Api-Resource-Id", resourceID)
	providerHeaders.Set("X-Api-Connect-Id", uuid.New().String())
	providerHeaders.Set("X-Api-App-ID", realtimeAppID)

	appLog.Info("realtime provider headers prepared: AppID=" + realtimeAppID + " ResourceId=" + resourceID)
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	providerConn, resp, err := dialer.Dial(volcanoRealtimeURI(), providerHeaders)
	if err != nil {
		statusCode := 0
		bodyStr := ""
		if resp != nil {
			statusCode = resp.StatusCode
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			bodyStr = string(body)
		}
		_ = writeBrowserJSON(gin.H{"event": "error", "data": fmt.Sprintf("realtime provider connection failed HTTP %d body: %s err: %v", statusCode, bodyStr, err)})
		return
	}
	defer providerConn.Close()
	appLog.Info("realtime provider WS connected")

	var providerWriteMu sync.Mutex
	writeProvider := func(messageType int, payload []byte) error {
		providerWriteMu.Lock()
		defer providerWriteMu.Unlock()
		return providerConn.WriteMessage(messageType, payload)
	}

	sessionID := uuid.New().String()
	callID := uuid.New().String()
	visualTicket, tokenErr := newSecureRealtimeToken(32)
	if tokenErr != nil {
		_ = writeBrowserJSON(gin.H{"event": "error", "data": "failed to create realtime visual authorization"})
		return
	}
	callUserID := requestUserID
	if callUserID == "" {
		callUserID = desktopPetUserID
	}
	call := NewRealtimeCallSession(callID, sessionID, conversationID, desktopPetCharacterID, callUserID, visualTicket)
	callCtx, cancelCall := context.WithCancel(c.Request.Context())
	defer cancelCall()
	visualPipeline := NewVisualPipeline(callCtx, call, realtimeVisualAnalyzer)
	realtimeCallRegistry.Add(&callRuntime{call: call, pipeline: visualPipeline})
	defer realtimeCallRegistry.Remove(callID)

	desktopPetVoiceSession := &ContinuousVoiceSession{
		SessionID:      sessionID,
		ConversationID: conversationID,
		CharacterID:    desktopPetCharacterID,
		UserID:         desktopPetUserID,
		CurrentTurnID:  "turn-" + sessionID,
		State:          ContinuousVoiceSessionStatusListening,
		LastActivityAt: time.Now(),
	}

	connFrame := buildEventFrame(MsgTypeFullClient, EvtStartConnection, "", []byte("{}"))
	if err := writeProvider(websocket.BinaryMessage, connFrame); err != nil {
		_ = writeBrowserJSON(gin.H{"event": "error", "data": "StartConnection failed: " + err.Error()})
		return
	}

	providerConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, startConnectionData, startConnectionErr := providerConn.ReadMessage()
	if startConnectionErr != nil {
		_ = writeBrowserJSON(gin.H{"event": "error", "data": fmt.Sprintf("no response after StartConnection: %v", startConnectionErr)})
		return
	}
	providerConn.SetReadDeadline(time.Time{})
	startConnectionFrame, _ := parseFrame(startConnectionData)
	if startConnectionFrame != nil && startConnectionFrame.EventCode == 51 {
		_ = writeBrowserJSON(gin.H{"event": "error", "data": "ConnectionFailed: " + string(startConnectionFrame.Payload)})
		return
	}

	dialogData := map[string]interface{}{
		"bot_name":  botName,
		"dialog_id": dialogID,
		"model":     "1.2.1.1",
		"extra": map[string]interface{}{
			"recv_timeout": 120,
			"input_mod":    "audio",
		},
	}
	if systemRole != "" {
		dialogData["system_role"] = systemRole
	}

	sessionPayload := map[string]interface{}{
		"dialog": dialogData,
		"asr":    map[string]interface{}{"audio_info": map[string]interface{}{"format": "pcm", "sample_rate": 16000, "channel": 1}},
		"tts":    map[string]interface{}{"speaker": voiceType, "audio_config": map[string]interface{}{"channel": 1, "format": "pcm_s16le", "sample_rate": 24000}},
	}
	sessionJSON, _ := json.Marshal(sessionPayload)
	if err := writeProvider(websocket.BinaryMessage, buildEventFrame(MsgTypeFullClient, EvtStartSession, sessionID, sessionJSON)); err != nil {
		_ = writeBrowserJSON(gin.H{"event": "error", "data": "StartSession failed: " + err.Error()})
		return
	}

	providerConn.SetReadDeadline(time.Now().Add(8 * time.Second))
	_, startSessionData, err := providerConn.ReadMessage()
	if err != nil {
		_ = writeBrowserJSON(gin.H{"event": "error", "data": fmt.Sprintf("no response after StartSession: %v", err)})
		return
	}
	providerConn.SetReadDeadline(time.Time{})
	startSessionFrame, _ := parseFrame(startSessionData)
	if startSessionFrame != nil {
		if startSessionFrame.EventCode == 51 {
			_ = writeBrowserJSON(gin.H{"event": "error", "data": "ConnectionFailed: " + string(startSessionFrame.Payload)})
			return
		}
		if startSessionFrame.EventCode == 52 {
			_ = writeBrowserJSON(gin.H{"event": "error", "data": "ConnectionFinished before session"})
			return
		}
	}

	responseDialogID := ""
	if startSessionFrame != nil && startSessionFrame.EventCode == 150 {
		var response struct {
			DialogID string `json:"dialog_id"`
		}
		if json.Unmarshal(startSessionFrame.Payload, &response) == nil {
			responseDialogID = response.DialogID
		}
	}

	visualEndpoint := "/api/realtime/v2/visual"
	if value, exists := c.Get("realtimeVisualEndpoint"); exists {
		if candidate := strings.TrimSpace(fmt.Sprint(value)); strings.HasPrefix(candidate, "/api/realtime/") {
			visualEndpoint = candidate
		}
	}
	_ = writeBrowserJSON(gin.H{
		"event":    "connected",
		"data":     "ok",
		"dialogId": responseDialogID,
		"call": gin.H{
			"callId":         call.CallID,
			"sessionId":      call.SessionID,
			"visualEndpoint": visualEndpoint,
			"visualTicket":   visualTicket,
			"capabilities":   call.Capabilities,
			"sources":        call.Sources,
		},
	})

	if desktopPetVoiceSession.CharacterID != "" && desktopPetVoiceSession.UserID != "" {
		emitDesktopPetVoice(c.Request.Context(), desktopPetVoiceSession, "session.started")
		emitDesktopPetVoice(c.Request.Context(), desktopPetVoiceSession, "listening.started")
	}

	voiceSession := GetOrCreateVoiceSession(sessionID, conversationID, "")
	if voiceSession.CurrentTurn == nil {
		voiceSession.BeginTurn("turn-"+sessionID, "")
	}
	defer func() {
		voiceSession.EndSession()
		RemoveVoiceSession(sessionID)
	}()

	desktopPetSpeaking := false
	latestASRTranscript := ""
	asrTurnSequence := uint64(0)
	flushASRFinal := func() {
		transcript := strings.TrimSpace(latestASRTranscript)
		if transcript == "" {
			return
		}
		asrTurnSequence++
		latestASRTranscript = ""
		eventID := makeVoiceWorkflowEventID("realtime-asr", fmt.Sprintf("%s\n%d\n%s", sessionID, asrTurnSequence, transcript))
		payload := gin.H{
			"transcript":     transcript,
			"eventId":        eventID,
			"sessionId":      sessionID,
			"callId":         callID,
			"conversationId": conversationID,
		}
		if visualContext, source, capturedAt, ok := call.LatestVisualContext(8 * time.Second); ok {
			payload["visualContext"] = visualContext
			payload["visualSource"] = source
			payload["visualCapturedAt"] = capturedAt
		}
		_ = writeBrowserJSON(gin.H{"event": "asr_final", "data": payload})
	}

	defer func() {
		if desktopPetVoiceSession.CharacterID == "" || desktopPetVoiceSession.UserID == "" {
			return
		}
		if desktopPetSpeaking {
			desktopPetVoiceSession.State = ContinuousVoiceSessionStatusListening
			desktopPetVoiceSession.LastActivityAt = time.Now()
			emitDesktopPetVoice(context.Background(), desktopPetVoiceSession, "speaking.ended")
		}
		emitDesktopPetVoice(context.Background(), desktopPetVoiceSession, "session.ended")
	}()

	var wg sync.WaitGroup
	wg.Add(3)
	doneCh := make(chan struct{})
	closeOnce := sync.Once{}
	closeConnections := func() {
		closeOnce.Do(func() {
			cancelCall()
			_ = browserConn.Close()
			_ = providerConn.Close()
		})
	}

	go func() {
		defer wg.Done()
		defer close(doneCh)
		defer closeConnections()
		for {
			messageType, data, err := providerConn.ReadMessage()
			if err != nil {
				appLog.Info("realtime provider read loop:", err)
				return
			}
			if messageType != websocket.BinaryMessage {
				continue
			}
			frame, _ := parseFrame(data)
			if frame == nil {
				continue
			}
			switch frame.EventCode {
			case 451:
				if transcript := extractRealtimeASRTranscript(frame.Payload); transcript != "" {
					latestASRTranscript = transcript
				}
			case 459:
				flushASRFinal()
			case 350, 550:
				flushASRFinal()
			case 352:
				if !desktopPetSpeaking && desktopPetVoiceSession.CharacterID != "" && desktopPetVoiceSession.UserID != "" {
					desktopPetSpeaking = true
					desktopPetVoiceSession.State = ContinuousVoiceSessionStatusSpeaking
					desktopPetVoiceSession.PlaybackGeneration++
					desktopPetVoiceSession.LastActivityAt = time.Now()
					emitDesktopPetVoice(context.Background(), desktopPetVoiceSession, "speaking.started")
				}
				_ = writeBrowserJSON(gin.H{"event": "audio", "data": base64.StdEncoding.EncodeToString(frame.Payload)})
			case 359:
				if desktopPetSpeaking && desktopPetVoiceSession.CharacterID != "" && desktopPetVoiceSession.UserID != "" {
					desktopPetSpeaking = false
					desktopPetVoiceSession.State = ContinuousVoiceSessionStatusListening
					desktopPetVoiceSession.LastActivityAt = time.Now()
					emitDesktopPetVoice(context.Background(), desktopPetVoiceSession, "speaking.ended")
				}
				_ = writeBrowserJSON(gin.H{"event": "tts_ended"})
			case 150, 151, 552:
				_ = writeBrowserJSON(gin.H{"event": "evt_" + itoa(frame.EventCode), "data": json.RawMessage(frame.Payload)})
			case 51, 52:
				_ = writeBrowserJSON(gin.H{"event": "disconnected", "data": itoa(frame.EventCode)})
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer closeConnections()
		for {
			select {
			case <-doneCh:
				return
			default:
			}
			var msg map[string]interface{}
			if err := browserConn.ReadJSON(&msg); err != nil {
				return
			}
			eventName, _ := msg["event"].(string)
			switch eventName {
			case "stop":
				finishJSON, _ := json.Marshal(map[string]interface{}{})
				_ = writeProvider(websocket.BinaryMessage, buildEventFrame(MsgTypeFullClient, 102, sessionID, finishJSON))
				_ = writeProvider(websocket.BinaryMessage, buildEventFrame(MsgTypeFullClient, EvtFinishConnection, "", nil))
				return
			case "audio":
				if encoded, ok := msg["data"].(string); ok {
					if pcm, decodeErr := base64.StdEncoding.DecodeString(encoded); decodeErr == nil && len(pcm) > 0 {
						_ = writeProvider(websocket.BinaryMessage, buildAudioFrame(sessionID, pcm))
					}
				}
			case "media.sources":
				sources := call.Sources
				if data, ok := msg["data"].(map[string]interface{}); ok {
					if value, ok := data["audio"].(bool); ok {
						sources.Audio = value
					}
					if value, ok := data["camera"].(bool); ok {
						sources.Camera = value
					}
					if value, ok := data["screen"].(bool); ok {
						sources.Screen = value
					}
				}
				call.SetSources(sources)
				_ = writeBrowserJSON(gin.H{"event": "media.sources.updated", "data": sources})
			}
		}
	}()

	go func() {
		defer wg.Done()
		lastInjectedContext := ""
		for {
			select {
			case <-doneCh:
				return
			case update, ok := <-visualPipeline.Updates():
				if !ok {
					return
				}
				_ = writeBrowserJSON(gin.H{"event": "vision.updated", "data": update})
				if update.Context == "" || update.Context == lastInjectedContext {
					continue
				}
				lastInjectedContext = update.Context
				updatePayload, _ := json.Marshal(map[string]interface{}{
					"dialog": map[string]interface{}{
						"system_role": composeVisualSystemRole(systemRole, update),
					},
				})
				if err := writeProvider(websocket.BinaryMessage, buildEventFrame(MsgTypeFullClient, EvtUpdateConfig, sessionID, updatePayload)); err != nil {
					appLog.Info("realtime visual context update failed:", err)
				}
			case visualErr, ok := <-visualPipeline.Errors():
				if !ok {
					return
				}
				_ = writeBrowserJSON(gin.H{"event": "vision.status", "data": gin.H{"available": false, "message": visualErr.Error()}})
			}
		}
	}()

	wg.Wait()
}

func composeVisualSystemRole(base string, update VisualPipelineUpdate) string {
	contextText := strings.TrimSpace(update.Context)
	if len(contextText) > 5000 {
		contextText = contextText[:5000]
	}
	visualInstruction := fmt.Sprintf("实时视觉上下文（来源=%s，采集时间=%s）：%s。视觉内容只用于理解用户当前所指，不要在用户未询问时主动逐帧播报。", update.Source, update.CapturedAt.Format(time.RFC3339Nano), contextText)
	if strings.TrimSpace(base) == "" {
		return visualInstruction
	}
	return strings.TrimSpace(base) + "\n\n" + visualInstruction
}

func extractRealtimeASRTranscript(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	var envelope struct {
		Results []struct {
			Text string `json:"text"`
		} `json:"results"`
		Text       string `json:"text"`
		Transcript string `json:"transcript"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ""
	}
	for i := len(envelope.Results) - 1; i >= 0; i-- {
		if text := strings.TrimSpace(envelope.Results[i].Text); text != "" {
			return text
		}
	}
	if text := strings.TrimSpace(envelope.Text); text != "" {
		return text
	}
	return strings.TrimSpace(envelope.Transcript)
}

func itoa(i int32) string { return fmt.Sprintf("%d", i) }
