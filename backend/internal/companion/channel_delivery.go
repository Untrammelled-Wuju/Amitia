// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func (s *service) sendToWechatSidecar(toUserID, content string) {
	if strings.HasPrefix(toUserID, "conv-") {
		toUserID = toUserID[5:]
	}
	body, _ := json.Marshal(map[string]string{"toUserId": toUserID, "text": content})
	req, _ := http.NewRequest("POST", "http://127.0.0.1:19876/api/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		log.Printf("[Companion] 微信发送失败: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("[Companion] 微信发送失败 HTTP %d: %s", resp.StatusCode, string(bodyBytes))
		return
	}
	log.Printf("[Companion] 微信已发送 to=%s", toUserID)
}

func (s *service) sendToQQSidecar(toUserID, content string) {
	if strings.HasPrefix(toUserID, "conv-qq-") {
		toUserID = toUserID[8:]
	} else if strings.HasPrefix(toUserID, "conv-") {
		toUserID = toUserID[5:]
	}
	body, _ := json.Marshal(map[string]string{"toUserId": toUserID, "text": content})
	req, _ := http.NewRequest("POST", "http://127.0.0.1:19877/api/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		log.Printf("[Companion] QQ发送失败: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("[Companion] QQ发送失败 HTTP %d: %s", resp.StatusCode, string(bodyBytes))
		return
	}
	log.Printf("[Companion] QQ已发送 to=%s", toUserID)
}

func (s *service) isDefaultCharacter(characterID string) bool {
	var isActive int
	s.db.Table("characters").Select("is_default").Where("id = ?", characterID).Limit(1).Row().Scan(&isActive)
	return isActive == 1
}

func (s *service) resolveConversationID(characterID, channel, existingID string) string {
	if characterID == "" {
		return ""
	}
	if channel == "" {
		channel = "web"
	}
	if existingID != "" {
		q := s.db.Table("conversations").Select("id").Where("id = ? AND character_id = ?", existingID, characterID)
		if channel != "all" {
			q = q.Where("channel = ?", channel)
		}
		var id string
		q.Limit(1).Row().Scan(&id)
		return id
	}
	channels := conversationChannels(channel)
	var id string
	s.db.Table("conversations").Select("id").
		Where("character_id = ? AND channel IN ?", characterID, channels).
		Order("updated_at DESC").
		Limit(1).Row().Scan(&id)
	return id
}

func (s *service) getWechatConvIDForChar(characterID string) string {
	var id string
	s.db.Table("conversations").Select("id").
		Where("character_id = ? AND channel = 'wechat' AND peer_id != '' AND peer_id IS NOT NULL", characterID).
		Order("updated_at DESC").
		Limit(1).Row().Scan(&id)
	return id
}

func (s *service) getQQConvIDForChar(characterID string) string {
	var id string
	s.db.Table("conversations").Select("id").
		Where("character_id = ? AND channel = 'qq' AND peer_id != '' AND peer_id IS NOT NULL", characterID).
		Order("updated_at DESC").
		Limit(1).Row().Scan(&id)
	return id
}

func conversationChannels(channel string) []string {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return []string{"web"}
	}
	if channel == "all" {
		return []string{"web", "wechat", "qq"}
	}
	fields := strings.FieldsFunc(channel, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '|'
	})
	channels := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		item := strings.TrimSpace(field)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		channels = append(channels, item)
	}
	if len(channels) == 0 {
		return []string{"web"}
	}
	return channels
}
