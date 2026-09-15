// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"fmt"
	"net/http"
	"strings"
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

	conversationID := strings.TrimSpace(c.Query("conversationId"))
	dialogID := strings.TrimSpace(c.Query("dialogId"))
	requestSpaceID := ""
	if value, exists := c.Get("realtimeUserId"); exists && value != nil {
		requestSpaceID = strings.TrimSpace(fmt.Sprint(value))
	} else if value, exists := c.Get("spaceId"); exists && value != nil {
		requestSpaceID = strings.TrimSpace(fmt.Sprint(value))
	}
	requestSpaceID = realtimeEffectiveSpaceID(requestSpaceID)
	if requestSpaceID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "authenticated user is required"})
		return
	}
	ownedCharacterID, ownerErr := requireRealtimeConversationOwner(conversationID, requestSpaceID)
	if ownerErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "conversation not found"})
		return
	}
	desktopPetCharacterID := ownedCharacterID
	desktopPetSpaceID := requestSpaceID

	characterName := "AI"
	characterBase := ""
	speakingStyle := ""
	if dbInstance != nil && desktopPetCharacterID != "" {
		var ch struct{ N, SP, SS, VT, CVID, VM string }
		dbInstance.Table("characters").Where("id = ? AND space_id = ?", desktopPetCharacterID, requestSpaceID).Select("name as n, character_base as sp, speaking_style as ss, voice_type as vt, custom_voice_id as cvid, voice_mode as vm").First(&ch)
		if ch.VM == "clone" && ch.CVID != "" {
			voiceType = ch.CVID
		} else if ch.VT != "" {
			voiceType = ch.VT
		}
		if ch.N != "" {
			characterName = ch.N
		}
		characterBase = ch.SP
		speakingStyle = ch.SS
	}

	sessionID := uuid.New().String()
	callID := uuid.New().String()
	visualTicket, tokenErr := newSecureRealtimeToken(32)
	if tokenErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "failed to create realtime visual authorization"})
		return
	}
	callSpaceID := requestSpaceID
	if callSpaceID == "" {
		callSpaceID = desktopPetSpaceID
	}
	call := NewRealtimeCallSession(callID, sessionID, conversationID, desktopPetCharacterID, callSpaceID, visualTicket)

	visualEndpoint := "/api/realtime/v2/visual"
	if value, exists := c.Get("realtimeVisualEndpoint"); exists {
		if candidate := strings.TrimSpace(fmt.Sprint(value)); strings.HasPrefix(candidate, "/api/realtime/") {
			visualEndpoint = candidate
		}
	}

	desktopPetVoiceSession := &ContinuousVoiceSession{
		SessionID:      sessionID,
		ConversationID: conversationID,
		CharacterID:    desktopPetCharacterID,
		SpaceID:        desktopPetSpaceID,
		CurrentTurnID:  "turn-" + sessionID,
		State:          ContinuousVoiceSessionStatusListening,
		LastActivityAt: time.Now(),
	}

	serveCascadeCall(c, cascadeCallParams{
		CallID:          callID,
		SessionID:       sessionID,
		SpaceID:         requestSpaceID,
		CharacterID:     desktopPetCharacterID,
		ConversationID:  conversationID,
		CharacterName:   characterName,
		CharacterBase:   characterBase,
		SpeakingStyle:   speakingStyle,
		VoiceType:       voiceType,
		Language:        "zh-CN",
		VisualEndpoint:  visualEndpoint,
		VisualTicket:    visualTicket,
		Instruction:     speakingStyle,
		DesktopPetPhase: true,
		Call:            call,
		DesktopPet:      desktopPetVoiceSession,
		DialogID:        dialogID,
	})
}
