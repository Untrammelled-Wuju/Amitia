// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	apiKey := c.Query("apiKey")
	if apiKey == "" {
		apiKey = c.GetHeader("X-Tts-Api-Key")
	}
	if apiKey == "" {
		c.JSON(400, gin.H{"code": 400, "message": "API Key required"})
		return
	}
	voiceType := c.Query("voiceType")
	if voiceType == "" {
		voiceType = "zh_female_vv_jupiter_bigtts"
	}
	resourceId := "volc.speech.dialog"
	appId := c.Query("appId")
	if appId == "" {
		appId = c.GetHeader("X-Api-App-ID")
	}
	realtimeAppId := appId
	realtimeAccessToken := apiKey
	if dbInstance != nil {
		var ttsCfg struct {
			RealtimeAppId       string `gorm:"column:realtime_app_id"`
			RealtimeAccessToken string `gorm:"column:realtime_access_token"`
			RealtimeSecretKey   string `gorm:"column:realtime_secret_key"`
		}
		dbInstance.Table("tts_configs").Where("is_active = 1").Select("realtime_app_id, realtime_access_token, realtime_secret_key").First(&ttsCfg)
		if ttsCfg.RealtimeAppId != "" {
			realtimeAppId = ttsCfg.RealtimeAppId
		}
		if ttsCfg.RealtimeAccessToken != "" {
			realtimeAccessToken = ttsCfg.RealtimeAccessToken
		}

	}

	conversationId := c.Query("conversationId")
	dialogId := c.Query("dialogId")

	systemRole := ""
	botName := "AI"
	if dbInstance != nil && conversationId != "" {
		var conv struct{ CID string }
		dbInstance.Table("conversations").Where("id = ?", conversationId).Select("character_id as cid").First(&conv)
		if conv.CID != "" {
			var ch struct{ N, SP, SS, VT, CVID, VM string }
			dbInstance.Table("characters").Where("id = ?", conv.CID).Select("name as n, system_prompt as sp, speaking_style as ss, voice_type as vt, custom_voice_id as cvid, voice_mode as vm").First(&ch)
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

	volcanoHeaders := http.Header{}
	volcanoHeaders.Set("X-Api-App-Key", "PlgvMymc7f3tQnJ6")
	volcanoHeaders.Set("X-Api-Access-Key", realtimeAccessToken)
	volcanoHeaders.Set("X-Api-Resource-Id", resourceId)
	volcanoHeaders.Set("X-Api-Connect-Id", uuid.New().String())
	volcanoHeaders.Set("X-Api-App-ID", realtimeAppId)

	appLog.Info("volc headers: AppID=" + realtimeAppId + " AccessKey=" + realtimeAccessToken + " ResourceId=" + resourceId)
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	volcanoConn, resp, err := dialer.Dial(volcanoRealtimeUri, volcanoHeaders)
	if err != nil {
		sc := 0
		if resp != nil {
			sc = resp.StatusCode
		}
		bodyStr := ""
		if resp != nil {
			sc = resp.StatusCode
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			bodyStr = string(body)
		}
		browserConn.WriteJSON(gin.H{"event": "error", "data": fmt.Sprintf("volc dial failed HTTP %d body: %s err: %v", sc, bodyStr, err)})
		return
	}
	appLog.Info("volc WS connected")
	appLog.Info("volc dial success, sending StartConnection...")

	sessID := uuid.New().String()

	connFrame := buildEventFrame(MsgTypeFullClient, EvtStartConnection, "", []byte("{}"))
	if err := volcanoConn.WriteMessage(websocket.BinaryMessage, connFrame); err != nil {
		browserConn.WriteJSON(gin.H{"event": "error", "data": "StartConnection failed: " + err.Error()})
		return
	}
	appLog.Info("StartConnection sent")

	volcanoConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, scData, scErr := volcanoConn.ReadMessage()
	if scErr != nil {
		appLog.Info("volc read after StartConnection:", scErr)
		browserConn.WriteJSON(gin.H{"event": "error", "data": fmt.Sprintf("no response after StartConnection: %v", scErr)})
		return
	}
	volcanoConn.SetReadDeadline(time.Time{})
	scFrame, _ := parseFrame(scData)
	if scFrame != nil {
		appLog.Info("volc StartConnection resp evt:", scFrame.EventCode, "payload:", string(scFrame.Payload))
		if scFrame.EventCode == 51 {
			browserConn.WriteJSON(gin.H{"event": "error", "data": "ConnectionFailed: " + string(scFrame.Payload)})
			return
		}
	}

	dialogData := map[string]interface{}{"bot_name": botName, "dialog_id": dialogId, "extra": nil}
	dialogData["model"] = "1.2.1.1"
	dialogData["extra"] = map[string]interface{}{"recv_timeout": 120, "input_mod": "audio"}
	if systemRole != "" {
		dialogData["system_role"] = systemRole
	}

	sessPayload := map[string]interface{}{
		"dialog": dialogData,
		"asr":    map[string]interface{}{"audio_info": map[string]interface{}{"format": "pcm", "sample_rate": 16000, "channel": 1}},
		"tts":    map[string]interface{}{"speaker": voiceType, "audio_config": map[string]interface{}{"channel": 1, "format": "pcm_s16le", "sample_rate": 24000}},
	}
	sessJSON, _ := json.Marshal(sessPayload)
	sessFrame := buildEventFrame(MsgTypeFullClient, EvtStartSession, sessID, sessJSON)
	if err := volcanoConn.WriteMessage(websocket.BinaryMessage, sessFrame); err != nil {
		browserConn.WriteJSON(gin.H{"event": "error", "data": "StartSession failed: " + err.Error()})
		return
	}
	appLog.Info("StartSession sent")
	appLog.Info("StartSession payload: " + string(sessJSON))

	volcanoConn.SetReadDeadline(time.Now().Add(8 * time.Second))
	_, respData, err := volcanoConn.ReadMessage()
	if err != nil {
		appLog.Info("volc read after StartSession:", err)
		browserConn.WriteJSON(gin.H{"event": "error", "data": fmt.Sprintf("no response after StartSession: %v", err)})
		return
	}
	volcanoConn.SetReadDeadline(time.Time{})
	respFrame, _ := parseFrame(respData)
	if respFrame != nil {
		appLog.Info("volc init resp evt:", respFrame.EventCode, "payload:", string(respFrame.Payload))
		if respFrame.EventCode == 51 {
			browserConn.WriteJSON(gin.H{"event": "error", "data": "ConnectionFailed: " + string(respFrame.Payload)})
			return
		}
		if respFrame.EventCode == 52 {
			browserConn.WriteJSON(gin.H{"event": "error", "data": "ConnectionFinished before session"})
			return
		}
	}

	var respDialogId string
	if respFrame != nil && respFrame.EventCode == 150 {
		var ssResp struct {
			DialogID string `json:"dialog_id"`
		}
		if json.Unmarshal(respFrame.Payload, &ssResp) == nil && ssResp.DialogID != "" {
			respDialogId = ssResp.DialogID
		}
	}
	browserConn.WriteJSON(gin.H{"event": "connected", "data": "ok", "dialogId": respDialogId})

	var wg sync.WaitGroup
	wg.Add(2)
	doneCh := make(chan struct{})

	go func() {
		defer wg.Done()
		defer close(doneCh)
		for {
			msgType, data, err := volcanoConn.ReadMessage()
			if err != nil {
				appLog.Info("volc read loop:", err)
				return
			}
			if msgType != websocket.BinaryMessage {
				continue
			}
			frame, _ := parseFrame(data)
			if frame == nil {
				appLog.Info("volc nil frame len:", len(data))
				continue
			}
			appLog.Info("volc evt:", frame.EventCode)
			switch frame.EventCode {
			case 352:
				browserConn.WriteJSON(gin.H{"event": "audio", "data": base64.StdEncoding.EncodeToString(frame.Payload)})
			case 359:
				browserConn.WriteJSON(gin.H{"event": "tts_ended"})
			case 150, 151, 552:
				browserConn.WriteJSON(gin.H{"event": "evt_" + itoa(frame.EventCode), "data": json.RawMessage(frame.Payload)})
			case 51, 52:
				browserConn.WriteJSON(gin.H{"event": "disconnected", "data": itoa(frame.EventCode)})
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
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
			evt, _ := msg["event"].(string)
			switch evt {
			case "stop":
				fin, _ := json.Marshal(map[string]interface{}{})
				volcanoConn.WriteMessage(websocket.BinaryMessage, buildEventFrame(MsgTypeFullClient, 102, sessID, fin))
				volcanoConn.WriteMessage(websocket.BinaryMessage, buildEventFrame(MsgTypeFullClient, EvtFinishConnection, "", nil))
				return
			case "audio":
				if d, ok := msg["data"].(string); ok {
					if b, err := base64.StdEncoding.DecodeString(d); err == nil && len(b) > 0 {
						volcanoConn.WriteMessage(websocket.BinaryMessage, buildAudioFrame(sessID, b))
					}
				}
			}
		}
	}()

	wg.Wait()
}

func itoa(i int32) string { return fmt.Sprintf("%d", i) }
