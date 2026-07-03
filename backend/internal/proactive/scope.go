package proactive

import (
	"strings"

	"gorm.io/gorm"
)

type proactiveCharacter struct {
	ID       string
	Name     string
	Identity string
}

func normalizeProactiveChannel(channel string) string {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return "web"
	}
	return channel
}

func primaryConversationChannel(channel string) string {
	channel = normalizeProactiveChannel(channel)
	if channel == "all" {
		return "all"
	}
	if strings.Contains(channel, "web") {
		return "web"
	}
	if strings.Contains(channel, "wechat") {
		return "wechat"
	}
	if strings.Contains(channel, "qq") {
		return "qq"
	}
	return channel
}

func resolveProactiveConversation(db *gorm.DB, conversationID, characterID, channel string, requirePeer bool) string {
	targetChannel := primaryConversationChannel(channel)
	query := db.Table("conversations").Select("id")
	if conversationID != "" {
		query = query.Where("id = ?", conversationID)
		if characterID != "" {
			query = query.Where("character_id = ?", characterID)
		}
		if targetChannel != "" {
			if targetChannel != "" && targetChannel != "all" {
				query = query.Where("channel = ?", targetChannel)
			}
		}
		if requirePeer {
			query = query.Where("peer_id != '' AND peer_id IS NOT NULL")
		}
	} else {
		if characterID == "" {
			return ""
		}
		if targetChannel == "all" {
			query = query.Where("character_id = ?", characterID)
		} else {
			query = query.Where("character_id = ? AND channel = ?", characterID, targetChannel)
		}
		if requirePeer {
			query = query.Where("peer_id != '' AND peer_id IS NOT NULL")
		}
	}
	var id string
	query.Order("updated_at DESC").Limit(1).Row().Scan(&id)
	return id
}

func resolveProactiveCharacter(db *gorm.DB, characterID, conversationID string) (proactiveCharacter, bool) {
	characterID = strings.TrimSpace(characterID)
	conversationID = strings.TrimSpace(conversationID)
	if characterID == "" && conversationID != "" {
		db.Table("conversations").Select("character_id").Where("id = ?", conversationID).Limit(1).Row().Scan(&characterID)
	}
	if characterID == "" {
		return proactiveCharacter{}, false
	}
	var ch proactiveCharacter
	err := db.Table("characters").Select("id, COALESCE(name,''), COALESCE(identity,'')").Where("id = ?", characterID).Limit(1).Row().Scan(&ch.ID, &ch.Name, &ch.Identity)
	if err != nil || ch.ID == "" {
		return proactiveCharacter{}, false
	}
	if ch.Name == "" {
		ch.Name = "AI助手"
	}
	return ch, true
}

func resolveProactiveCharacterProfile(db *gorm.DB, characterID string) (string, string, bool) {
	ch, ok := resolveProactiveCharacter(db, characterID, "")
	if !ok {
		return "", "", false
	}
	return ch.Name, ch.Identity, true
}
