package coordination

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

const (
	GroupMemberStatusActive = "active"
	GroupMemberStatusLeft   = "left"
	GroupMaxMembers         = 8
	GroupDefaultAutoReplies = 6
)

type GroupSettings struct {
	AllowCharacterMentions bool `json:"allowCharacterMentions"`
	MaxAutoReplies         int  `json:"maxAutoReplies"`
}

type GroupMember struct {
	MemberID          string `json:"memberId"`
	CharacterID       string `json:"characterId"`
	RoleOwnerID       string `json:"roleOwnerId"`
	CharacterName     string `json:"characterName"`
	CharacterAvatar   string `json:"characterAvatar,omitempty"`
	Status            string `json:"status"`
	OrderIndex        int    `json:"orderIndex"`
	JoinedSequence    int64  `json:"joinedSequence"`
	HistoryStart      int64  `json:"historyStartSequence"`
	LeftSequence      int64  `json:"leftSequence,omitempty"`
	CharacterRevision int64  `json:"characterRevision"`
}

type GroupState struct {
	ConversationID       string        `json:"conversationId"`
	ConversationRoleID   string        `json:"conversationRoleId"`
	ConversationRevision int64         `json:"conversationRevision"`
	RoleOwnerID          string        `json:"roleOwnerId"`
	Title                string        `json:"title"`
	ConversationType     string        `json:"conversationType"`
	MembershipRevision   int64         `json:"membershipRevision"`
	SettingsRevision     int64         `json:"settingsRevision"`
	LastMessageSequence  int64         `json:"lastMessageSequence"`
	Settings             GroupSettings `json:"groupSettings"`
	Members              []GroupMember `json:"members"`
}

type GroupStatePort interface {
	GroupState(context.Context, ExecutionScope, string) (GroupState, error)
}

type GroupConversationListPort interface {
	GroupConversations(context.Context, ExecutionScope) ([]Resource, map[string]string, error)
}

func GroupConversationKind(kind string) bool {
	switch kind {
	case "conversation", "message", "summary", "group-round", "group-delivery", "group-invitation":
		return true
	default:
		return false
	}
}

func GroupResourceConversationID(resource Resource) string {
	if resource.Kind == "conversation" {
		return resource.ID
	}
	var body struct {
		ConversationID string `json:"conversationId"`
	}
	if json.Unmarshal(resource.Body, &body) == nil {
		return strings.TrimSpace(body.ConversationID)
	}
	return ""
}

func (s GroupState) IsGroup() bool {
	return s.ConversationID != "" && s.ConversationType == "group"
}

func (s GroupState) ActiveMember(characterID string) (GroupMember, bool) {
	for _, member := range s.Members {
		if member.Status == GroupMemberStatusActive && member.CharacterID == characterID {
			return member, true
		}
	}
	return GroupMember{}, false
}

func (s GroupState) MemberByID(memberID string) (GroupMember, bool) {
	for _, member := range s.Members {
		if member.Status == GroupMemberStatusActive && member.MemberID == memberID {
			return member, true
		}
	}
	return GroupMember{}, false
}

func (s GroupState) Validate() error {
	if s.ConversationID == "" || s.ConversationRoleID == "" || s.RoleOwnerID == "" || s.MembershipRevision < 1 || s.SettingsRevision < 1 {
		return ErrWrongOwner
	}
	if s.ConversationType != "" && s.ConversationType != "single" && s.ConversationType != "group" {
		return ErrWrongOwner
	}
	if len(s.Members) > GroupMaxMembers+32 {
		return ErrPendingLimit
	}
	active := 0
	seenIDs := map[string]bool{}
	seenRoles := map[string]bool{}
	for _, member := range s.Members {
		if member.MemberID == "" || member.CharacterID == "" || member.RoleOwnerID != s.RoleOwnerID || member.Status != GroupMemberStatusActive && member.Status != GroupMemberStatusLeft {
			return ErrWrongOwner
		}
		if seenIDs[member.MemberID] {
			return ErrWrongOwner
		}
		seenIDs[member.MemberID] = true
		if member.Status == GroupMemberStatusActive {
			active++
			if seenRoles[member.CharacterID] {
				return ErrWrongOwner
			}
			seenRoles[member.CharacterID] = true
		}
	}
	if s.IsGroup() && active == 0 {
		return errors.New("群聊没有有效成员")
	}
	return nil
}
