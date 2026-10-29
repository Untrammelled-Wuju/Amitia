package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

const (
	GroupRoundQueued              = "queued"
	GroupRoundRunning             = "running"
	GroupRoundCompleted           = "completed"
	GroupRoundCompletedWithErrors = "completed_with_errors"
	GroupRoundCancelled           = "cancelled"
	GroupDeliveryQueued           = "queued"
	GroupDeliveryStarting         = "starting"
	GroupDeliveryRunning          = "running"
	GroupDeliveryCompleted        = "completed"
	GroupDeliveryFailed           = "failed"
	GroupDeliveryCancelled        = "cancelled"
	GroupInvitationAccepted       = "accepted"
	GroupInviteReplyTool          = "conversation.invite_reply"
	GroupModeStandard             = "standard"
	GroupModeDiscussion           = "discussion"
)

var (
	ErrGroupMemberChanged   = errors.New("群成员已变化，请刷新后重试")
	ErrGroupSettingsChanged = errors.New("群设置已变化，请刷新后重试")
	ErrGroupTargetInvalid   = errors.New("目标角色不在当前群聊中或不可执行")
	ErrGroupBudgetExhausted = errors.New("本轮角色接话已达到上限，可手动继续")
	errDiscussionHalted     = errors.New("多角色讨论已停止")
)

type GroupMessageOptions struct {
	ExpectedMembershipRevision *int64 `json:"expectedMembershipRevision,omitempty"`
	ExpectedSettingsRevision   *int64 `json:"expectedSettingsRevision,omitempty"`
	ReplyToMessageID           string `json:"replyToMessageId,omitempty"`
	Mode                       string `json:"mode,omitempty"`
}

type GroupMention struct {
	MemberID    string `json:"memberId"`
	CharacterID string `json:"characterId"`
	RoleOwnerID string `json:"roleOwnerId,omitempty"`
}

type PreparedGroupReply struct {
	UserMessageID       string `json:"userMessageId"`
	UserMessageRevision int64  `json:"userMessageRevision"`
	UserSequence        int64  `json:"userSequence"`
	GroupRoundID        string `json:"groupRoundId"`
	DeliveryID          string `json:"deliveryId"`
	SourceMessageID     string `json:"sourceMessageId"`
	DeliveryIndex       int    `json:"deliveryIndex"`
	ContextSequence     int64  `json:"contextSequence,omitempty"`
}

type GroupReplyContext struct {
	Round        GroupRound
	Delivery     GroupDelivery
	State        coordination.GroupState
	SourceMember coordination.GroupMember
	Mode         string
}

type GroupDelivery struct {
	DeliveryID        string `json:"deliveryId"`
	GroupRoundID      string `json:"groupRoundId"`
	ChildRequestID    string `json:"childRequestId"`
	TurnID            string `json:"turnId,omitempty"`
	ExecutionID       string `json:"executionId,omitempty"`
	MemberID          string `json:"memberId"`
	CharacterID       string `json:"characterId"`
	RoleOwnerID       string `json:"roleOwnerId"`
	CharacterName     string `json:"characterName"`
	CharacterRevision int64  `json:"characterRevision"`
	OrderIndex        int    `json:"orderIndex"`
	TurnIndex         int    `json:"turnIndex,omitempty"`
	Source            string `json:"source"`
	Status            string `json:"status"`
	Error             string `json:"error,omitempty"`
	MessageID         string `json:"messageId,omitempty"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type GroupRound struct {
	RoundID            string            `json:"roundId"`
	ConversationID     string            `json:"conversationId"`
	RootRequestID      string            `json:"rootRequestId"`
	RequestFingerprint string            `json:"requestFingerprint,omitempty"`
	TriggerMessageID   string            `json:"triggerMessageId"`
	TriggerMessage     string            `json:"triggerMessage"`
	TriggerSequence    int64             `json:"triggerSequence"`
	Status             string            `json:"status"`
	MembershipRevision int64             `json:"membershipRevision"`
	SettingsRevision   int64             `json:"settingsRevision"`
	AutoReplyCount     int               `json:"autoReplyCount"`
	Mode               string            `json:"mode,omitempty"`
	CreatedAt          string            `json:"createdAt"`
	UpdatedAt          string            `json:"updatedAt"`
	Deliveries         []GroupDelivery   `json:"deliveries"`
	Invitations        []GroupInvitation `json:"invitations,omitempty"`
}

type GroupInvitation struct {
	InvitationID        string `json:"invitationId"`
	ConversationID      string `json:"conversationId"`
	RoundID             string `json:"roundId"`
	SourceMessageID     string `json:"sourceMessageId"`
	SourceDeliveryID    string `json:"sourceDeliveryId"`
	SourceCharacterID   string `json:"sourceCharacterId"`
	TargetMemberID      string `json:"targetMemberId"`
	TargetCharacterID   string `json:"targetCharacterId"`
	TargetCharacterName string `json:"targetCharacterName"`
	Status              string `json:"status"`
	CreatedAt           string `json:"createdAt"`
}

type GroupInviteProposal struct {
	InvitationID        string `json:"invitationId"`
	TargetMemberID      string `json:"targetMemberId"`
	TargetCharacterID   string `json:"targetCharacterId"`
	TargetCharacterName string `json:"targetCharacterName"`
}

type GroupResponse struct {
	ConversationID string     `json:"conversationId"`
	RequestID      string     `json:"requestId"`
	GroupRoundID   string     `json:"groupRoundId"`
	Status         string     `json:"status"`
	UserMessageID  string     `json:"userMessageId"`
	Deliveries     []Response `json:"deliveries"`
	Saved          bool       `json:"saved"`
}

type GroupInviteInput struct {
	CharacterID  string `json:"characterId"`
	ShareHistory bool   `json:"shareHistory"`
}

type GroupMembersInput struct {
	RequestID                  string             `json:"requestId"`
	CharacterID                string             `json:"characterId"`
	TargetDeviceID             string             `json:"targetDeviceId,omitempty"`
	ExpectedMembershipRevision *int64             `json:"expectedMembershipRevision"`
	Members                    []GroupInviteInput `json:"members" binding:"required"`
}

type CreateGroupConversationInput struct {
	RequestID      string             `json:"requestId"`
	CharacterID    string             `json:"characterId"`
	TargetDeviceID string             `json:"targetDeviceId,omitempty"`
	Members        []GroupInviteInput `json:"members" binding:"required"`
}

type GroupMemberRemoveInput struct {
	RequestID                  string `json:"requestId"`
	CharacterID                string `json:"characterId"`
	TargetDeviceID             string `json:"targetDeviceId,omitempty"`
	MemberID                   string `json:"memberId"`
	ExpectedMembershipRevision *int64 `json:"expectedMembershipRevision"`
}

type GroupSettingsInput struct {
	RequestID                string   `json:"requestId"`
	CharacterID              string   `json:"characterId"`
	TargetDeviceID           string   `json:"targetDeviceId,omitempty"`
	ExpectedSettingsRevision *int64   `json:"expectedSettingsRevision"`
	Name                     *string  `json:"name,omitempty"`
	AllowCharacterMentions   *bool    `json:"allowCharacterMentions,omitempty"`
	MaxAutoReplies           *int     `json:"maxAutoReplies,omitempty"`
	MemberOrder              []string `json:"memberOrder,omitempty"`
}

type GroupReplyRequestInput struct {
	RequestID       string         `json:"requestId"`
	CharacterID     string         `json:"characterId"`
	TargetDeviceID  string         `json:"targetDeviceId,omitempty"`
	SourceMessageID string         `json:"sourceMessageId"`
	Members         []GroupMention `json:"members" binding:"required"`
}

type GroupDeliveryRetryInput struct {
	RequestID      string `json:"requestId"`
	CharacterID    string `json:"characterId"`
	TargetDeviceID string `json:"targetDeviceId,omitempty"`
	DeliveryID     string `json:"deliveryId"`
}

type GroupMutationResponse struct {
	Group           coordination.GroupState      `json:"group"`
	Acknowledgement coordination.Acknowledgement `json:"acknowledgement"`
	Saved           bool                         `json:"saved"`
}

func (e *Engine) CreateGroupConversation(ctx context.Context, request Request, input CreateGroupConversationInput) (GroupMutationResponse, error) {
	request.RequestID = strings.TrimSpace(input.RequestID)
	request.RoleID = input.CharacterID
	request.TargetDeviceID = input.TargetDeviceID
	if request.RequestID == "" || len(input.Members) == 0 {
		return GroupMutationResponse{}, errors.New("创建群聊参数无效")
	}
	if request.TargetDeviceID == "" {
		request.TargetDeviceID = request.DeviceID
	}
	ctx, scope, finish, currentRole, _, err := e.beginGroupScope(ctx, request)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	defer finish()
	catalogPort, ok := e.data.(coordination.RoleCatalogPort)
	if !ok {
		return GroupMutationResponse{}, errors.New("角色目录不支持群成员")
	}
	catalog, err := catalogPort.ListOwnedRoleCatalog(ctx, scope, true)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	roleByID := make(map[string]coordination.Role, len(catalog))
	for _, role := range catalog {
		roleByID[role.ID] = role
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	members := []coordination.GroupMember{{MemberID: "member-" + uuid.NewString(), CharacterID: currentRole.ID, RoleOwnerID: scope.RoleOwnerID, CharacterName: currentRole.Name, CharacterAvatar: currentRole.Avatar, CharacterRevision: currentRole.Revision, Status: coordination.GroupMemberStatusActive, OrderIndex: 0, JoinedSequence: 0, HistoryStart: 0}}
	activeRoles := map[string]bool{currentRole.ID: true}
	for _, invited := range input.Members {
		id := strings.TrimSpace(invited.CharacterID)
		if activeRoles[id] {
			continue
		}
		role, found := roleByID[id]
		if !found || role.Disabled {
			return GroupMutationResponse{}, ErrGroupTargetInvalid
		}
		if len(activeRoles) >= coordination.GroupMaxMembers {
			return GroupMutationResponse{}, errors.New("群聊最多支持 8 个角色")
		}
		members = append(members, coordination.GroupMember{MemberID: "member-" + uuid.NewString(), CharacterID: role.ID, RoleOwnerID: scope.RoleOwnerID, CharacterName: role.Name, CharacterAvatar: role.Avatar, CharacterRevision: role.Revision, Status: coordination.GroupMemberStatusActive, OrderIndex: len(activeRoles), JoinedSequence: 0, HistoryStart: 0})
		activeRoles[role.ID] = true
	}
	if len(activeRoles) < 2 {
		return GroupMutationResponse{}, errors.New("至少需要邀请一个有效角色")
	}
	conversationID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(scope.SpaceID+"\x00"+scope.InitiatorDeviceID+"\x00"+request.RequestID)).String()
	state := coordination.GroupState{ConversationID: conversationID, ConversationRoleID: currentRole.ID, RoleOwnerID: scope.RoleOwnerID, Title: "新群聊", ConversationType: "group", MembershipRevision: 1, SettingsRevision: 1, Settings: coordination.GroupSettings{AllowCharacterMentions: false, MaxAutoReplies: coordination.GroupDefaultAutoReplies}, Members: members}
	conversation := map[string]any{"id": conversationID, "conversationId": conversationID, "title": state.Title, "characterId": currentRole.ID, "roleOwnerId": scope.RoleOwnerID, "ownerId": scope.ResourceOwnerID, "coreId": scope.CoreID, "channel": "web", "source": "manual", "conversationType": "group", "membershipRevision": 1, "settingsRevision": 1, "members": groupMembersDocument(members), "groupSettings": state.Settings, "lastMessageSequence": 0, "createdAt": now, "updatedAt": now}
	encoded, err := json.Marshal(conversation)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	fingerprintData := map[string]any{"conversationId": conversationID, "characterId": currentRole.ID, "members": input.Members, "operation": "create-group"}
	digest := sha256.Sum256(body(fingerprintData))
	location, firstRequest, err := e.coordination.RegisterRequest(ctx, scope, hex.EncodeToString(digest[:]), conversationID)
	if err != nil {
		if errors.Is(err, coordination.ErrScopeExpired) && location.State == "started" {
			return GroupMutationResponse{}, ErrUncertainExecution
		}
		return GroupMutationResponse{}, err
	}
	if !firstRequest {
		return GroupMutationResponse{}, ErrUncertainExecution
	}
	ack, err := e.commit(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "conversation", ID: conversationID, RoleID: currentRole.ID, Body: encoded}}})
	if err != nil {
		return GroupMutationResponse{}, err
	}
	if err := e.coordination.CompleteRequest(context.WithoutCancel(ctx), scope); err != nil {
		return GroupMutationResponse{}, err
	}
	state.ConversationRevision = ack.Versions["conversation/"+conversationID]
	return GroupMutationResponse{Group: state, Acknowledgement: ack, Saved: true}, nil
}

func (e *Engine) AcceptGroupInvitations(ctx context.Context, inference Inference, generation Generation, sourceMessageID string, proposals []GroupInviteProposal) error {
	if inference.Group == nil || !inference.Group.State.IsGroup() || !inference.Group.State.Settings.AllowCharacterMentions {
		if len(proposals) == 0 {
			return nil
		}
		return errors.New("当前会话未允许角色互相邀请")
	}
	proposals = append(proposals, resolveGroupTextInvitations(inference.Group.State, generation.Text, inference.Scope.RoleID)...)
	if len(proposals) > 3 {
		proposals = proposals[:3]
	}
	if len(proposals) == 0 {
		return nil
	}
	if sourceMessageID == "" || inference.Scope.ConversationRoleID == "" {
		return coordination.ErrWrongOwner
	}
	unlock := e.lock("group-invitations\x00" + inference.Scope.SpaceID + "\x00" + inference.Group.Delivery.GroupRoundID)
	defer unlock()
	scope := inference.Scope
	if port, ok := e.data.(coordination.GroupStatePort); ok {
		current, stateErr := port.GroupState(ctx, scope, inference.ConversationID)
		if stateErr != nil {
			return stateErr
		}
		if !current.IsGroup() || !current.Settings.AllowCharacterMentions {
			return errors.New("当前会话未允许角色互相邀请")
		}
		inference.Group.State = current
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return errors.New("数据来源不支持角色邀请")
	}
	roundResource, err := port.Resource(ctx, scope, "group-round", inference.Group.Delivery.GroupRoundID)
	if err != nil {
		return err
	}
	if roundResource == nil {
		return coordination.ErrResourceVersion
	}
	var round GroupRound
	if json.Unmarshal(roundResource.Body, &round) != nil || round.ConversationID != inference.ConversationID || round.RoundID != inference.Group.Delivery.GroupRoundID {
		return coordination.ErrRequestConflict
	}
	if round.Mode == GroupModeDiscussion {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	mutations := make([]coordination.Mutation, 0)
	seen := make(map[string]bool)
	deliveryMembers := make(map[string]bool, len(round.Deliveries))
	for _, delivery := range round.Deliveries {
		deliveryMembers[delivery.MemberID] = true
	}
	for _, invitation := range round.Invitations {
		seen[invitation.SourceMessageID+"\x00"+invitation.TargetMemberID] = true
	}
	for _, proposal := range proposals {
		member, active := inference.Group.State.MemberByID(proposal.TargetMemberID)
		if !active || member.CharacterID != proposal.TargetCharacterID || member.CharacterID == inference.Scope.RoleID {
			continue
		}
		key := sourceMessageID + "\x00" + member.MemberID
		if seen[key] {
			continue
		}
		seen[key] = true
		if deliveryMembers[member.MemberID] {
			continue
		}
		if round.AutoReplyCount >= inference.Group.State.Settings.MaxAutoReplies {
			round.Invitations = append(round.Invitations, GroupInvitation{InvitationID: uuid.NewString(), ConversationID: round.ConversationID, RoundID: round.RoundID, SourceMessageID: sourceMessageID, SourceDeliveryID: inference.Group.Delivery.DeliveryID, SourceCharacterID: inference.Scope.RoleID, TargetMemberID: member.MemberID, TargetCharacterID: member.CharacterID, TargetCharacterName: member.CharacterName, Status: "rejected", CreatedAt: now})
			continue
		}
		round.AutoReplyCount++
		invitation := GroupInvitation{InvitationID: uuid.NewString(), ConversationID: round.ConversationID, RoundID: round.RoundID, SourceMessageID: sourceMessageID, SourceDeliveryID: inference.Group.Delivery.DeliveryID, SourceCharacterID: inference.Scope.RoleID, TargetMemberID: member.MemberID, TargetCharacterID: member.CharacterID, TargetCharacterName: member.CharacterName, Status: GroupInvitationAccepted, CreatedAt: now}
		round.Invitations = append(round.Invitations, invitation)
		delivery := GroupDelivery{DeliveryID: uuid.NewString(), GroupRoundID: round.RoundID, ChildRequestID: round.RootRequestID + "/reply/" + uuid.NewString(), MemberID: member.MemberID, CharacterID: member.CharacterID, RoleOwnerID: member.RoleOwnerID, CharacterName: member.CharacterName, CharacterRevision: member.CharacterRevision, OrderIndex: member.OrderIndex, Source: "character", Status: GroupDeliveryQueued, CreatedAt: now, UpdatedAt: now}
		round.Deliveries = append(round.Deliveries, delivery)
		deliveryMembers[member.MemberID] = true
		encodedInvitation, err := json.Marshal(invitation)
		if err != nil {
			return err
		}
		encodedDelivery, err := json.Marshal(delivery)
		if err != nil {
			return err
		}
		mutations = append(mutations, coordination.Mutation{Kind: "group-invitation", ID: invitation.InvitationID, RoleID: coordination.StorageRole(scope, "group-invitation"), Body: encodedInvitation}, coordination.Mutation{Kind: "group-delivery", ID: delivery.DeliveryID, RoleID: coordination.StorageRole(scope, "group-delivery"), Body: encodedDelivery})
	}
	round.UpdatedAt = now
	encodedRound, err := json.Marshal(round)
	if err != nil {
		return err
	}
	mutations = append(mutations, coordination.Mutation{Kind: "group-round", ID: round.RoundID, RoleID: coordination.StorageRole(scope, "group-round"), ExpectedRevision: roundResource.Revision, Body: encodedRound})
	copyScope := scope
	copyScope.RequestID = scope.RequestID + "|group-invitations"
	_, err = e.commit(ctx, coordination.Commit{Scope: copyScope, Mutations: mutations})
	return err
}

func groupRoundID(spaceID, deviceID, requestID string) string {
	digest := sha256.Sum256([]byte(spaceID + "\x00" + deviceID + "\x00" + requestID))
	return "round_" + hex.EncodeToString(digest[:])
}

func groupRequestFingerprint(request Request) string {
	data := map[string]any{"conversationId": request.ConversationID, "message": request.Message, "characterId": request.RoleID, "mentions": request.Mentions, "group": request.Group, "attachments": request.Attachments}
	digest := sha256.Sum256(body(data))
	return hex.EncodeToString(digest[:])
}

func normalizeGroupMention(member coordination.GroupMember) GroupMention {
	return GroupMention{MemberID: member.MemberID, CharacterID: member.CharacterID, RoleOwnerID: member.RoleOwnerID}
}

func resolveRequestedGroupMentions(state coordination.GroupState, mentions []GroupMention) ([]GroupMention, error) {
	if len(mentions) > 0 {
		return mentions, nil
	}
	member, ok := state.ActiveMember(state.ConversationRoleID)
	if !ok {
		return nil, ErrGroupTargetInvalid
	}
	return []GroupMention{normalizeGroupMention(member)}, nil
}

func resolveDiscussionMembers(state coordination.GroupState) ([]coordination.GroupMember, error) {
	active := make([]coordination.GroupMember, 0, len(state.Members))
	for _, member := range state.Members {
		if member.Status == coordination.GroupMemberStatusActive {
			active = append(active, member)
		}
	}
	sort.SliceStable(active, func(left, right int) bool {
		return active[left].OrderIndex < active[right].OrderIndex
	})
	if len(active) < 2 {
		return nil, errors.New("多角色讨论至少需要两个有效成员")
	}
	start := 0
	for index, member := range active {
		if member.CharacterID == state.ConversationRoleID {
			start = index
			break
		}
	}
	turns := state.Settings.MaxAutoReplies
	if turns < 2 {
		return nil, errors.New("多角色讨论至少需要两次有效发言")
	}
	result := make([]coordination.GroupMember, 0, turns)
	for turn := 0; turn < turns; turn++ {
		result = append(result, active[(start+turn)%len(active)])
	}
	return result, nil
}

func groupMentionBoundary(value rune) bool {
	return unicode.IsSpace(value) || strings.ContainsRune("，。！？、,.!?;；:：", value)
}

func resolveGroupTextInvitations(state coordination.GroupState, text, sourceCharacterID string) []GroupInviteProposal {
	members := make([]coordination.GroupMember, 0, len(state.Members))
	for _, member := range state.Members {
		if member.Status == coordination.GroupMemberStatusActive && strings.TrimSpace(member.CharacterName) != "" && member.CharacterID != sourceCharacterID {
			members = append(members, member)
		}
	}
	sort.SliceStable(members, func(left, right int) bool {
		leftLength := len([]rune(strings.TrimSpace(members[left].CharacterName)))
		rightLength := len([]rune(strings.TrimSpace(members[right].CharacterName)))
		if leftLength != rightLength {
			return leftLength > rightLength
		}
		return members[left].OrderIndex < members[right].OrderIndex
	})
	runes := []rune(text)
	seen := make(map[string]bool)
	result := make([]GroupInviteProposal, 0, 3)
	for index := 0; index < len(runes); index++ {
		if runes[index] != '@' {
			continue
		}
		for _, member := range members {
			name := []rune(strings.TrimSpace(member.CharacterName))
			end := index + 1 + len(name)
			if len(name) == 0 || end > len(runes) || string(runes[index+1:end]) != string(name) {
				continue
			}
			if end < len(runes) && !groupMentionBoundary(runes[end]) {
				continue
			}
			if !seen[member.MemberID] {
				seen[member.MemberID] = true
				result = append(result, GroupInviteProposal{TargetMemberID: member.MemberID, TargetCharacterID: member.CharacterID, TargetCharacterName: member.CharacterName})
			}
			index = end - 1
			break
		}
		if len(result) == 3 {
			break
		}
	}
	return result
}

func resolveGroupTargets(state coordination.GroupState, mentions []GroupMention, all bool) ([]coordination.GroupMember, error) {
	if all {
		members := append([]coordination.GroupMember(nil), state.Members...)
		sort.SliceStable(members, func(i, j int) bool { return members[i].OrderIndex < members[j].OrderIndex })
		result := make([]coordination.GroupMember, 0, len(members))
		for _, member := range members {
			if member.Status == coordination.GroupMemberStatusActive {
				result = append(result, member)
			}
		}
		return result, nil
	}
	result := make([]coordination.GroupMember, 0, len(mentions))
	seen := make(map[string]bool)
	for _, mention := range mentions {
		var member coordination.GroupMember
		var ok bool
		if strings.TrimSpace(mention.MemberID) != "" {
			member, ok = state.MemberByID(strings.TrimSpace(mention.MemberID))
		} else if strings.TrimSpace(mention.CharacterID) != "" {
			member, ok = state.ActiveMember(strings.TrimSpace(mention.CharacterID))
		}
		if !ok || member.RoleOwnerID != state.RoleOwnerID || mention.CharacterID != "" && mention.CharacterID != member.CharacterID {
			return nil, ErrGroupTargetInvalid
		}
		if seen[member.MemberID] {
			continue
		}
		seen[member.MemberID] = true
		result = append(result, member)
	}
	return result, nil
}

func (e *Engine) beginGroupScope(ctx context.Context, request Request) (context.Context, coordination.ExecutionScope, func(), coordination.Role, coordination.GroupState, error) {
	if request.TargetDeviceID == "" {
		request.TargetDeviceID = request.DeviceID
	}
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, request.RoleID, request.RequestID)
	if err != nil {
		return ctx, scope, finish, coordination.Role{}, coordination.GroupState{}, err
	}
	fail := func(err error) (context.Context, coordination.ExecutionScope, func(), coordination.Role, coordination.GroupState, error) {
		finish()
		return ctx, scope, func() {}, coordination.Role{}, coordination.GroupState{}, err
	}
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return fail(err)
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return fail(err)
	}
	role, err := coordination.ResolveRole(request.RoleID, roles)
	if err != nil {
		return fail(err)
	}
	scope.RoleID, scope.RoleRevision = role.ID, role.Revision
	if err := coordination.ResolveMemoryScope(&scope, request.MemoryView); err != nil {
		return fail(err)
	}
	var state coordination.GroupState
	if request.ConversationID != "" {
		port, ok := e.data.(coordination.GroupStatePort)
		if !ok {
			return fail(errors.New("数据来源不支持群聊"))
		}
		state, err = port.GroupState(ctx, scope, request.ConversationID)
		if err != nil {
			return fail(err)
		}
		if state.IsGroup() {
			if _, active := state.ActiveMember(scope.RoleID); !active {
				return fail(coordination.ErrWrongOwner)
			}
			scope.ConversationRoleID = state.ConversationRoleID
		}
	}
	if request.ExpectedScope != nil {
		expected := *request.ExpectedScope
		expected.RequestID, expected.TurnID, expected.ExecutionID = scope.RequestID, scope.TurnID, scope.ExecutionID
		if expected != scope {
			return fail(coordination.ErrScopeExpired)
		}
	}
	scope.TurnID, scope.ExecutionID = uuid.NewString(), uuid.NewString()
	ctx = coordination.WithScope(ctx, scope)
	return ctx, scope, finish, role, state, nil
}

func groupConversationDocument(resource *coordination.Resource) (map[string]any, error) {
	if resource == nil || resource.Deleted {
		return nil, errors.New("会话不存在")
	}
	document := map[string]any{}
	if err := json.Unmarshal(resource.Body, &document); err != nil {
		return nil, coordination.ErrWrongOwner
	}
	return document, nil
}

func groupMembersDocument(members []coordination.GroupMember) []map[string]any {
	result := make([]map[string]any, 0, len(members))
	for _, member := range members {
		result = append(result, map[string]any{"memberId": member.MemberID, "characterId": member.CharacterID, "roleOwnerId": member.RoleOwnerID, "characterName": member.CharacterName, "characterAvatar": member.CharacterAvatar, "status": member.Status, "orderIndex": member.OrderIndex, "joinedSequence": member.JoinedSequence, "historyStartSequence": member.HistoryStart, "leftSequence": member.LeftSequence, "characterRevision": member.CharacterRevision})
	}
	return result
}

func membersFromGroupState(state coordination.GroupState) []coordination.GroupMember {
	return append([]coordination.GroupMember(nil), state.Members...)
}

func (e *Engine) commitGroupConversation(ctx context.Context, scope coordination.ExecutionScope, requestID, kind string, state coordination.GroupState, document map[string]any, deps ...coordination.ResourceVersion) (coordination.Acknowledgement, error) {
	document["conversationType"] = "group"
	document["id"] = state.ConversationID
	document["conversationId"] = state.ConversationID
	document["roleOwnerId"] = state.RoleOwnerID
	document["membershipRevision"] = state.MembershipRevision
	document["settingsRevision"] = state.SettingsRevision
	document["numberOfMembers"] = len(state.Members)
	document["members"] = groupMembersDocument(state.Members)
	document["groupSettings"] = state.Settings
	document["updatedAt"] = time.Now().UTC().Format(time.RFC3339Nano)
	body, err := json.Marshal(document)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	return e.commit(ctx, coordination.Commit{Scope: scope, Dependencies: deps, Mutations: []coordination.Mutation{{Kind: "conversation", ID: state.ConversationID, RoleID: coordination.StorageRole(scope, "conversation"), ExpectedRevision: state.ConversationRevision, Body: body}}})
}

func (e *Engine) InviteGroupMembers(ctx context.Context, request Request, input GroupMembersInput) (GroupMutationResponse, error) {
	if request.RequestID == "" {
		request.RequestID = strings.TrimSpace(input.RequestID)
	}
	request.ConversationID = strings.TrimSpace(request.ConversationID)
	request.RoleID = input.CharacterID
	request.TargetDeviceID = input.TargetDeviceID
	if request.ConversationID == "" || request.RequestID == "" || len(input.Members) == 0 {
		return GroupMutationResponse{}, errors.New("群成员邀请参数无效")
	}
	ctx, scope, finish, currentRole, state, err := e.beginGroupScope(ctx, request)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	defer finish()
	if state.ConversationID == "" {
		return GroupMutationResponse{}, errors.New("请先发送一条消息建立会话")
	}
	if state.IsGroup() && input.ExpectedMembershipRevision != nil && state.MembershipRevision != *input.ExpectedMembershipRevision {
		return GroupMutationResponse{}, ErrGroupMemberChanged
	}
	if !state.IsGroup() && currentRole.ID != state.ConversationRoleID {
		return GroupMutationResponse{}, coordination.ErrWrongOwner
	}
	catalogPort, ok := e.data.(coordination.RoleCatalogPort)
	if !ok {
		return GroupMutationResponse{}, errors.New("角色目录不支持群成员")
	}
	catalog, err := catalogPort.ListOwnedRoleCatalog(ctx, scope, true)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	roleByID := make(map[string]coordination.Role, len(catalog))
	for _, role := range catalog {
		roleByID[role.ID] = role
	}
	members := membersFromGroupState(state)
	if !state.IsGroup() {
		members = []coordination.GroupMember{{MemberID: "member-" + uuid.NewString(), CharacterID: currentRole.ID, RoleOwnerID: scope.RoleOwnerID, CharacterName: currentRole.Name, CharacterAvatar: currentRole.Avatar, CharacterRevision: currentRole.Revision, Status: coordination.GroupMemberStatusActive, OrderIndex: 0, JoinedSequence: 1, HistoryStart: 0}}
		state.ConversationType = "group"
		state.Settings = coordination.GroupSettings{AllowCharacterMentions: false, MaxAutoReplies: coordination.GroupDefaultAutoReplies}
		state.MembershipRevision = 1
		state.SettingsRevision = 1
	}
	activeCount := 0
	nextOrder := 0
	activeRoles := make(map[string]bool)
	for _, member := range members {
		if member.Status == coordination.GroupMemberStatusActive {
			activeCount++
			activeRoles[member.CharacterID] = true
		}
		if member.OrderIndex >= nextOrder {
			nextOrder = member.OrderIndex + 1
		}
	}
	added := false
	for _, inputMember := range input.Members {
		characterID := strings.TrimSpace(inputMember.CharacterID)
		if characterID == "" {
			return GroupMutationResponse{}, ErrGroupTargetInvalid
		}
		if activeRoles[characterID] {
			continue
		}
		role, found := roleByID[characterID]
		if !found || role.Disabled {
			return GroupMutationResponse{}, ErrGroupTargetInvalid
		}
		if activeCount >= coordination.GroupMaxMembers {
			return GroupMutationResponse{}, errors.New("群聊最多支持 8 个角色")
		}
		historyStart := int64(0)
		if !inputMember.ShareHistory {
			historyStart = state.LastMessageSequence + 1
		}
		members = append(members, coordination.GroupMember{MemberID: "member-" + uuid.NewString(), CharacterID: role.ID, RoleOwnerID: scope.RoleOwnerID, CharacterName: role.Name, CharacterAvatar: role.Avatar, CharacterRevision: role.Revision, Status: coordination.GroupMemberStatusActive, OrderIndex: nextOrder, JoinedSequence: state.LastMessageSequence + 1, HistoryStart: historyStart})
		activeCount++
		nextOrder++
		activeRoles[role.ID] = true
		added = true
	}
	if len(activeRoles) < 2 {
		return GroupMutationResponse{}, errors.New("至少需要邀请一个有效角色")
	}
	if !added {
		return GroupMutationResponse{Group: state, Saved: true}, nil
	}
	resource, err := e.data.(coordination.ResourcePort).Resource(ctx, scope, "conversation", state.ConversationID)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	document, err := groupConversationDocument(resource)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	state.Members = members
	state.MembershipRevision++
	ack, err := e.commitGroupConversation(ctx, scope, request.RequestID, "group-member-invite", state, document)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	state.ConversationRevision = ack.Versions["conversation/"+state.ConversationID]
	return GroupMutationResponse{Group: state, Acknowledgement: ack, Saved: true}, nil
}

func (e *Engine) RemoveGroupMember(ctx context.Context, request Request, input GroupMemberRemoveInput) (GroupMutationResponse, error) {
	request.RequestID = strings.TrimSpace(input.RequestID)
	request.RoleID = input.CharacterID
	request.TargetDeviceID = input.TargetDeviceID
	ctx, scope, finish, _, state, err := e.beginGroupScope(ctx, request)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	defer finish()
	if !state.IsGroup() {
		return GroupMutationResponse{}, errors.New("当前会话不是群聊")
	}
	if input.ExpectedMembershipRevision != nil && state.MembershipRevision != *input.ExpectedMembershipRevision {
		return GroupMutationResponse{}, ErrGroupMemberChanged
	}
	memberID := strings.TrimSpace(input.MemberID)
	if memberID == "" {
		return GroupMutationResponse{}, ErrGroupTargetInvalid
	}
	return e.removeGroupMember(ctx, scope, state, input.ExpectedMembershipRevision, memberID)
}

func (e *Engine) removeGroupMember(ctx context.Context, scope coordination.ExecutionScope, state coordination.GroupState, expected *int64, memberID string) (GroupMutationResponse, error) {
	if expected != nil && state.MembershipRevision != *expected {
		return GroupMutationResponse{}, ErrGroupMemberChanged
	}
	active := 0
	for _, member := range state.Members {
		if member.Status == coordination.GroupMemberStatusActive {
			active++
		}
	}
	if active <= 1 {
		return GroupMutationResponse{}, errors.New("群聊至少需要保留一个有效成员")
	}
	found := false
	for index := range state.Members {
		if state.Members[index].MemberID != memberID {
			continue
		}
		if state.Members[index].Status != coordination.GroupMemberStatusActive {
			break
		}
		state.Members[index].Status = coordination.GroupMemberStatusLeft
		state.Members[index].LeftSequence = state.LastMessageSequence + 1
		found = true
		break
	}
	if !found {
		return GroupMutationResponse{}, ErrGroupTargetInvalid
	}
	state.MembershipRevision++
	resource, err := e.data.(coordination.ResourcePort).Resource(ctx, scope, "conversation", state.ConversationID)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	document, err := groupConversationDocument(resource)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	ack, err := e.commitGroupConversation(ctx, scope, scope.RequestID, "group-member-remove", state, document)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	state.ConversationRevision = ack.Versions["conversation/"+state.ConversationID]
	return GroupMutationResponse{Group: state, Acknowledgement: ack, Saved: true}, nil
}

func (e *Engine) UpdateGroupSettings(ctx context.Context, request Request, input GroupSettingsInput) (GroupMutationResponse, error) {
	request.RequestID = strings.TrimSpace(input.RequestID)
	request.RoleID = input.CharacterID
	request.TargetDeviceID = input.TargetDeviceID
	ctx, scope, finish, _, state, err := e.beginGroupScope(ctx, request)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	defer finish()
	if !state.IsGroup() {
		return GroupMutationResponse{}, errors.New("当前会话不是群聊")
	}
	if input.ExpectedSettingsRevision != nil && state.SettingsRevision != *input.ExpectedSettingsRevision {
		return GroupMutationResponse{}, ErrGroupSettingsChanged
	}
	resource, err := e.data.(coordination.ResourcePort).Resource(ctx, scope, "conversation", state.ConversationID)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	document, err := groupConversationDocument(resource)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	changed := false
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" || utf8.RuneCountInString(name) > 128 {
			return GroupMutationResponse{}, errors.New("群名称无效")
		}
		if name != state.Title {
			document["title"] = name
			state.Title = name
			changed = true
		}
	}
	if input.AllowCharacterMentions != nil {
		if state.Settings.AllowCharacterMentions != *input.AllowCharacterMentions {
			state.Settings.AllowCharacterMentions = *input.AllowCharacterMentions
			changed = true
		}
	}
	if input.MaxAutoReplies != nil {
		if *input.MaxAutoReplies < 0 || *input.MaxAutoReplies > coordination.GroupDefaultAutoReplies {
			return GroupMutationResponse{}, errors.New("自动接话上限无效")
		}
		if state.Settings.MaxAutoReplies != *input.MaxAutoReplies {
			state.Settings.MaxAutoReplies = *input.MaxAutoReplies
			changed = true
		}
	}
	if len(input.MemberOrder) > 0 {
		order := make(map[string]int, len(input.MemberOrder))
		activeMembers := make(map[string]bool)
		for _, member := range state.Members {
			if member.Status == coordination.GroupMemberStatusActive {
				activeMembers[member.MemberID] = true
			}
		}
		for index, memberID := range input.MemberOrder {
			memberID = strings.TrimSpace(memberID)
			if memberID == "" || !activeMembers[memberID] {
				return GroupMutationResponse{}, ErrGroupTargetInvalid
			}
			if _, duplicate := order[memberID]; duplicate {
				return GroupMutationResponse{}, ErrGroupTargetInvalid
			}
			order[memberID] = index
		}
		if len(order) != len(activeMembers) {
			return GroupMutationResponse{}, errors.New("成员排序必须包含全部有效成员")
		}
		orderChanged := false
		for _, member := range state.Members {
			if member.Status == coordination.GroupMemberStatusActive && member.OrderIndex != order[member.MemberID] {
				orderChanged = true
				break
			}
		}
		sort.SliceStable(state.Members, func(i, j int) bool {
			left, lok := order[state.Members[i].MemberID]
			right, rok := order[state.Members[j].MemberID]
			if lok != rok {
				return lok
			}
			if lok && left != right {
				return left < right
			}
			return state.Members[i].OrderIndex < state.Members[j].OrderIndex
		})
		for index := range state.Members {
			if state.Members[index].Status == coordination.GroupMemberStatusActive {
				state.Members[index].OrderIndex = order[state.Members[index].MemberID]
			}
		}
		if orderChanged {
			changed = true
		}
	}
	if !changed {
		return GroupMutationResponse{Group: state, Saved: true}, nil
	}
	state.SettingsRevision++
	ack, err := e.commitGroupConversation(ctx, scope, scope.RequestID, "group-settings", state, document)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	state.ConversationRevision = ack.Versions["conversation/"+state.ConversationID]
	return GroupMutationResponse{Group: state, Acknowledgement: ack, Saved: true}, nil
}

func (e *Engine) RunGroupEvents(ctx context.Context, request Request, emit func(Event) error) (GroupResponse, error) {
	request.Message = strings.TrimSpace(request.Message)
	request.RequestID = strings.TrimSpace(request.RequestID)
	if request.ConversationID == "" || request.RequestID == "" || request.Group == nil {
		return GroupResponse{}, errors.New("群聊请求缺少会话、请求编号或能力声明")
	}
	if len(request.RequestID) > 128 || request.Message == "" || len(request.Message) > 128<<10 {
		return GroupResponse{}, errors.New("请求编号和消息内容无效")
	}
	if request.TargetDeviceID == "" {
		request.TargetDeviceID = request.DeviceID
	}
	unlock := e.lock(request.SpaceID + "\x00" + request.TargetDeviceID + "\x00" + request.ConversationID)
	defer unlock()
	ctx, scope, finish, currentRole, state, err := e.beginGroupScope(ctx, request)
	if err != nil {
		return GroupResponse{}, err
	}
	defer finish()
	ctx, cancel := context.WithCancelCause(ctx)
	activeKey := scope.SpaceID + "\x00" + scope.InitiatorDeviceID + "\x00" + scope.RequestID
	e.mu.Lock()
	e.active[activeKey] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.active, activeKey)
		e.mu.Unlock()
		cancel(nil)
	}()
	if !state.IsGroup() {
		return GroupResponse{}, errors.New("当前会话不是群聊")
	}
	fingerprint := groupRequestFingerprint(request)
	roundID := groupRoundID(scope.SpaceID, scope.InitiatorDeviceID, request.RequestID)
	roundResource, err := e.data.(coordination.ResourcePort).Resource(ctx, scope, "group-round", roundID)
	if err != nil {
		return GroupResponse{}, err
	}
	if roundResource != nil {
		var existing GroupRound
		if json.Unmarshal(roundResource.Body, &existing) != nil || existing.RootRequestID != request.RequestID {
			return GroupResponse{}, coordination.ErrRequestConflict
		}
		if existing.RequestFingerprint != "" && existing.RequestFingerprint != fingerprint {
			return GroupResponse{}, coordination.ErrRequestConflict
		}
		return e.executeGroupRound(ctx, scope, state, existing, emit)
	}
	if request.Group.ExpectedMembershipRevision != nil && state.MembershipRevision != *request.Group.ExpectedMembershipRevision {
		return GroupResponse{}, ErrGroupMemberChanged
	}
	if request.Group.ExpectedSettingsRevision != nil && state.SettingsRevision != *request.Group.ExpectedSettingsRevision {
		return GroupResponse{}, ErrGroupSettingsChanged
	}
	mode := strings.TrimSpace(request.Group.Mode)
	if mode == "" {
		mode = GroupModeStandard
	}
	if mode != GroupModeStandard && mode != GroupModeDiscussion {
		return GroupResponse{}, errors.New("群聊模式无效")
	}
	var targets []coordination.GroupMember
	if mode == GroupModeDiscussion {
		if !state.Settings.AllowCharacterMentions {
			return GroupResponse{}, errors.New("当前会话未允许角色互相讨论")
		}
		targets, err = resolveDiscussionMembers(state)
		if err != nil {
			return GroupResponse{}, err
		}
	} else {
		mentions, mentionErr := resolveRequestedGroupMentions(state, request.Mentions)
		if mentionErr != nil {
			return GroupResponse{}, mentionErr
		}
		targets, err = resolveGroupTargets(state, mentions, false)
		if err != nil {
			return GroupResponse{}, err
		}
	}
	conversationResource, err := e.data.(coordination.ResourcePort).Resource(ctx, scope, "conversation", state.ConversationID)
	if err != nil {
		return GroupResponse{}, err
	}
	conversation, err := groupConversationDocument(conversationResource)
	if err != nil {
		return GroupResponse{}, err
	}
	query := coordination.DataQuery{RequestID: request.RequestID, ConversationID: state.ConversationID, Limit: 128, GroupState: &state}
	if member, ok := state.ActiveMember(scope.RoleID); ok {
		query.GroupVisibilityStart = member.HistoryStart
	}
	snapshot, err := e.data.Snapshot(ctx, scope, query)
	if err != nil {
		return GroupResponse{}, err
	}
	if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
		return GroupResponse{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	userSequence := nextOwnedMessageSequence(snapshot, state.ConversationID)
	userMessageID := request.RequestID + "/user"
	userBody, err := json.Marshal(map[string]any{"id": userMessageID, "conversationId": state.ConversationID, "characterId": currentRole.ID, "role": "user", "senderType": "user", "content": request.Message, "requestId": request.RequestID, "createdAt": now, "sequence": userSequence, "attachments": request.Attachments, "mentions": request.Mentions, "executionScope": scope})
	if err != nil {
		return GroupResponse{}, err
	}
	deliveries := make([]GroupDelivery, 0, len(targets))
	for turnIndex, member := range targets {
		deliveryID := uuid.NewString()
		source := "user"
		if mode == GroupModeDiscussion {
			source = "discussion"
		}
		deliveries = append(deliveries, GroupDelivery{DeliveryID: deliveryID, GroupRoundID: roundID, ChildRequestID: request.RequestID + "/reply/" + deliveryID, MemberID: member.MemberID, CharacterID: member.CharacterID, RoleOwnerID: member.RoleOwnerID, CharacterName: member.CharacterName, CharacterRevision: member.CharacterRevision, OrderIndex: member.OrderIndex, TurnIndex: turnIndex, Source: source, Status: GroupDeliveryQueued, CreatedAt: now, UpdatedAt: now})
	}
	conversation["updatedAt"] = now
	conversation["lastMessageSequence"] = userSequence
	conversation["characterId"] = state.ConversationRoleID
	round := GroupRound{RoundID: roundID, ConversationID: state.ConversationID, RootRequestID: request.RequestID, RequestFingerprint: fingerprint, TriggerMessageID: userMessageID, TriggerMessage: request.Message, TriggerSequence: userSequence, Status: GroupRoundQueued, MembershipRevision: state.MembershipRevision, SettingsRevision: state.SettingsRevision, Mode: mode, CreatedAt: now, UpdatedAt: now, Deliveries: deliveries}
	roundBody, err := json.Marshal(round)
	if err != nil {
		return GroupResponse{}, err
	}
	mutations := []coordination.Mutation{{Kind: "conversation", ID: state.ConversationID, RoleID: coordination.StorageRole(scope, "conversation"), ExpectedRevision: conversationResource.Revision, Body: body(conversation)}, {Kind: "message", ID: userMessageID, RoleID: coordination.StorageRole(scope, "message"), Body: userBody}, {Kind: "group-round", ID: roundID, RoleID: coordination.StorageRole(scope, "group-round"), Body: roundBody}}
	for _, delivery := range deliveries {
		encoded, marshalErr := json.Marshal(delivery)
		if marshalErr != nil {
			return GroupResponse{}, marshalErr
		}
		mutations = append(mutations, coordination.Mutation{Kind: "group-delivery", ID: delivery.DeliveryID, RoleID: coordination.StorageRole(scope, "group-delivery"), Body: encoded})
	}
	location, firstRequest, err := e.coordination.RegisterRequest(ctx, scope, fingerprint, request.ConversationID)
	if err != nil {
		if errors.Is(err, coordination.ErrScopeExpired) && location.State == "started" {
			return GroupResponse{}, ErrUncertainExecution
		}
		return GroupResponse{}, err
	}
	if !firstRequest {
		return GroupResponse{}, ErrUncertainExecution
	}
	if _, err := e.commit(ctx, coordination.Commit{Scope: scope, Mutations: mutations}); err != nil {
		return GroupResponse{}, fmt.Errorf("群消息尚未确认保存: %w", err)
	}
	return e.executeGroupRound(ctx, scope, state, round, emit)
}

func (e *Engine) CreateGroupReplyRequest(ctx context.Context, request Request, input GroupReplyRequestInput) (GroupResponse, error) {
	request.RequestID = input.RequestID
	request.RoleID = input.CharacterID
	request.TargetDeviceID = input.TargetDeviceID
	request.RequestID = strings.TrimSpace(request.RequestID)
	if request.RequestID == "" || len(request.RequestID) > 128 || request.ConversationID == "" || strings.TrimSpace(input.SourceMessageID) == "" || len(input.Members) == 0 {
		return GroupResponse{}, errors.New("点名回复参数无效")
	}
	if request.TargetDeviceID == "" {
		request.TargetDeviceID = request.DeviceID
	}
	unlock := e.lock(request.SpaceID + "\x00" + request.TargetDeviceID + "\x00" + request.ConversationID)
	defer unlock()
	ctx, scope, finish, _, state, err := e.beginGroupScope(ctx, request)
	if err != nil {
		return GroupResponse{}, err
	}
	defer finish()
	if !state.IsGroup() {
		return GroupResponse{}, errors.New("当前会话不是群聊")
	}
	fingerprintData := map[string]any{"conversationId": state.ConversationID, "sourceMessageId": input.SourceMessageID, "mentions": input.Members, "characterId": request.RoleID, "operation": "reply-request"}
	digest := sha256.Sum256(body(fingerprintData))
	fingerprint := hex.EncodeToString(digest[:])
	roundID := groupRoundID(scope.SpaceID, scope.InitiatorDeviceID, request.RequestID)
	roundResource, err := e.data.(coordination.ResourcePort).Resource(ctx, scope, "group-round", roundID)
	if err != nil {
		return GroupResponse{}, err
	}
	if roundResource != nil {
		var existing GroupRound
		if json.Unmarshal(roundResource.Body, &existing) != nil || existing.RootRequestID != request.RequestID {
			return GroupResponse{}, coordination.ErrRequestConflict
		}
		if existing.RequestFingerprint != "" && existing.RequestFingerprint != fingerprint {
			return GroupResponse{}, coordination.ErrRequestConflict
		}
		return e.executeGroupRound(ctx, scope, state, existing, nil)
	}
	source, err := e.data.(coordination.ResourcePort).Resource(ctx, scope, "message", input.SourceMessageID)
	if err != nil || source == nil || source.Deleted {
		return GroupResponse{}, ErrGroupTargetInvalid
	}
	var userMessage struct {
		ConversationID string `json:"conversationId"`
		Role           string `json:"role"`
		SenderType     string `json:"senderType"`
		Content        string `json:"content"`
		Sequence       int64  `json:"sequence"`
	}
	if json.Unmarshal(source.Body, &userMessage) != nil || userMessage.ConversationID != state.ConversationID || userMessage.Role != "user" || strings.TrimSpace(userMessage.Content) == "" {
		return GroupResponse{}, ErrGroupTargetInvalid
	}
	targets, err := resolveGroupTargets(state, input.Members, false)
	if err != nil || len(targets) == 0 {
		return GroupResponse{}, ErrGroupTargetInvalid
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	round := GroupRound{RoundID: roundID, ConversationID: state.ConversationID, RootRequestID: request.RequestID, RequestFingerprint: fingerprint, TriggerMessageID: input.SourceMessageID, TriggerMessage: userMessage.Content, TriggerSequence: userMessage.Sequence, Status: GroupRoundQueued, MembershipRevision: state.MembershipRevision, SettingsRevision: state.SettingsRevision, CreatedAt: now, UpdatedAt: now}
	mutations := make([]coordination.Mutation, 0, len(targets)+1)
	for _, member := range targets {
		delivery := GroupDelivery{DeliveryID: uuid.NewString(), GroupRoundID: roundID, ChildRequestID: request.RequestID + "/reply/" + uuid.NewString(), MemberID: member.MemberID, CharacterID: member.CharacterID, RoleOwnerID: member.RoleOwnerID, CharacterName: member.CharacterName, CharacterRevision: member.CharacterRevision, OrderIndex: member.OrderIndex, Source: "user", Status: GroupDeliveryQueued, CreatedAt: now, UpdatedAt: now}
		round.Deliveries = append(round.Deliveries, delivery)
		encoded, marshalErr := json.Marshal(delivery)
		if marshalErr != nil {
			return GroupResponse{}, marshalErr
		}
		mutations = append(mutations, coordination.Mutation{Kind: "group-delivery", ID: delivery.DeliveryID, RoleID: coordination.StorageRole(scope, "group-delivery"), Body: encoded})
	}
	encodedRound, err := json.Marshal(round)
	if err != nil {
		return GroupResponse{}, err
	}
	mutations = append(mutations, coordination.Mutation{Kind: "group-round", ID: roundID, RoleID: coordination.StorageRole(scope, "group-round"), Body: encodedRound})
	location, firstRequest, err := e.coordination.RegisterRequest(ctx, scope, fingerprint, state.ConversationID)
	if err != nil {
		if errors.Is(err, coordination.ErrScopeExpired) && location.State == "started" {
			return GroupResponse{}, ErrUncertainExecution
		}
		return GroupResponse{}, err
	}
	if !firstRequest {
		return GroupResponse{}, ErrUncertainExecution
	}
	if _, err := e.commit(ctx, coordination.Commit{Scope: scope, Mutations: mutations}); err != nil {
		return GroupResponse{}, err
	}
	return e.executeGroupRound(ctx, scope, state, round, nil)
}

func (e *Engine) RetryGroupDelivery(ctx context.Context, request Request, roundID string, input GroupDeliveryRetryInput) (GroupResponse, error) {
	request.RequestID = input.RequestID
	request.RoleID = input.CharacterID
	request.TargetDeviceID = input.TargetDeviceID
	request.RequestID = strings.TrimSpace(request.RequestID)
	if request.RequestID == "" || len(request.RequestID) > 128 || request.ConversationID == "" || strings.TrimSpace(roundID) == "" || strings.TrimSpace(input.DeliveryID) == "" {
		return GroupResponse{}, errors.New("重试参数无效")
	}
	if request.TargetDeviceID == "" {
		request.TargetDeviceID = request.DeviceID
	}
	unlock := e.lock(request.SpaceID + "\x00" + request.TargetDeviceID + "\x00" + request.ConversationID)
	defer unlock()
	ctx, scope, finish, _, state, err := e.beginGroupScope(ctx, request)
	if err != nil {
		return GroupResponse{}, err
	}
	defer finish()
	if !state.IsGroup() {
		return GroupResponse{}, errors.New("当前会话不是群聊")
	}
	port := e.data.(coordination.ResourcePort)
	retryRoundID := groupRoundID(scope.SpaceID, scope.InitiatorDeviceID, request.RequestID)
	fingerprintData := map[string]any{"conversationId": state.ConversationID, "roundId": roundID, "deliveryId": input.DeliveryID, "characterId": request.RoleID, "operation": "retry-group-delivery"}
	digest := sha256.Sum256(body(fingerprintData))
	fingerprint := hex.EncodeToString(digest[:])
	location, firstRequest, err := e.coordination.RegisterRequest(ctx, scope, fingerprint, state.ConversationID)
	if err != nil {
		if errors.Is(err, coordination.ErrScopeExpired) && location.State == "started" {
			return GroupResponse{}, ErrUncertainExecution
		}
		return GroupResponse{}, err
	}
	retryResource, err := port.Resource(ctx, scope, "group-round", retryRoundID)
	if err != nil {
		return GroupResponse{}, err
	}
	if retryResource != nil {
		var existing GroupRound
		if json.Unmarshal(retryResource.Body, &existing) != nil ||
			existing.RoundID != retryRoundID ||
			existing.ConversationID != state.ConversationID ||
			existing.RootRequestID != request.RequestID ||
			len(existing.Deliveries) != 1 ||
			existing.Deliveries[0].Source != "retry:"+input.DeliveryID ||
			existing.RequestFingerprint != "" && existing.RequestFingerprint != fingerprint {
			return GroupResponse{}, coordination.ErrRequestConflict
		}
		return e.executeGroupRound(ctx, scope, state, existing, nil)
	}
	if !firstRequest {
		return GroupResponse{}, ErrUncertainExecution
	}
	originalResource, err := port.Resource(ctx, scope, "group-round", roundID)
	if err != nil || originalResource == nil || originalResource.Deleted {
		return GroupResponse{}, coordination.ErrResourceVersion
	}
	var original GroupRound
	if json.Unmarshal(originalResource.Body, &original) != nil || original.ConversationID != state.ConversationID {
		return GroupResponse{}, coordination.ErrRequestConflict
	}
	var failed *GroupDelivery
	for index := range original.Deliveries {
		if original.Deliveries[index].DeliveryID == input.DeliveryID && original.Deliveries[index].Status == GroupDeliveryFailed {
			failed = &original.Deliveries[index]
			break
		}
	}
	if failed == nil {
		return GroupResponse{}, errors.New("只能重试已明确失败的群成员任务")
	}
	member, active := state.ActiveMember(failed.CharacterID)
	if !active || member.MemberID != failed.MemberID {
		return GroupResponse{}, ErrGroupTargetInvalid
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	delivery := GroupDelivery{DeliveryID: uuid.NewString(), GroupRoundID: retryRoundID, ChildRequestID: request.RequestID + "/reply/" + uuid.NewString(), MemberID: member.MemberID, CharacterID: member.CharacterID, RoleOwnerID: member.RoleOwnerID, CharacterName: member.CharacterName, CharacterRevision: member.CharacterRevision, OrderIndex: member.OrderIndex, Source: "retry:" + failed.DeliveryID, Status: GroupDeliveryQueued, CreatedAt: now, UpdatedAt: now}
	round := GroupRound{RoundID: retryRoundID, ConversationID: state.ConversationID, RootRequestID: request.RequestID, RequestFingerprint: fingerprint, TriggerMessageID: original.TriggerMessageID, TriggerMessage: original.TriggerMessage, TriggerSequence: original.TriggerSequence, Status: GroupRoundQueued, MembershipRevision: state.MembershipRevision, SettingsRevision: state.SettingsRevision, CreatedAt: now, UpdatedAt: now, Deliveries: []GroupDelivery{delivery}}
	encodedDelivery, err := json.Marshal(delivery)
	if err != nil {
		return GroupResponse{}, err
	}
	encodedRound, err := json.Marshal(round)
	if err != nil {
		return GroupResponse{}, err
	}
	if _, err := e.commit(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "group-delivery", ID: delivery.DeliveryID, RoleID: coordination.StorageRole(scope, "group-delivery"), Body: encodedDelivery}, {Kind: "group-round", ID: retryRoundID, RoleID: coordination.StorageRole(scope, "group-round"), Body: encodedRound}}}); err != nil {
		return GroupResponse{}, err
	}
	return e.executeGroupRound(ctx, scope, state, round, nil)
}

func groupDeliveryEvent(event Event, delivery GroupDelivery, index int, roundID string) Event {
	event.GroupRoundID, event.DeliveryID, event.DeliveryIndex = roundID, delivery.DeliveryID, index
	event.CharacterID, event.CharacterName, event.RoleOwnerID = delivery.CharacterID, delivery.CharacterName, delivery.RoleOwnerID
	event.CharacterAvatar = ""
	return event
}

func (e *Engine) executeGroupRound(ctx context.Context, scope coordination.ExecutionScope, state coordination.GroupState, round GroupRound, emit func(Event) error) (GroupResponse, error) {
	result := GroupResponse{ConversationID: round.ConversationID, RequestID: round.RootRequestID, GroupRoundID: round.RoundID, UserMessageID: round.TriggerMessageID, Deliveries: []Response{}, Saved: true}
	if round.Status == GroupRoundCompleted || round.Status == GroupRoundCompletedWithErrors || round.Status == GroupRoundCancelled {
		terminal := true
		for _, delivery := range round.Deliveries {
			if delivery.Status != GroupDeliveryCompleted && delivery.Status != GroupDeliveryFailed && delivery.Status != GroupDeliveryCancelled {
				terminal = false
				break
			}
		}
		if terminal {
			if err := e.coordination.CompleteRequest(context.WithoutCancel(ctx), scope); err != nil {
				return result, err
			}
			result.Status = round.Status
			return result, nil
		}
	}
	round.Status = GroupRoundRunning
	failed := false
	cancelled := false
	var emitMu sync.Mutex
	var safeEmit func(Event) error
	if emit != nil {
		safeEmit = func(event Event) error {
			emitMu.Lock()
			defer emitMu.Unlock()
			return emit(event)
		}
	}
	type deliveryResult struct {
		index          int
		delivery       GroupDelivery
		response       Response
		err            error
		bookkeepingErr error
	}
	processed := make(map[string]bool)
	for {
		batch := make([]int, 0, len(round.Deliveries))
		if round.Mode == GroupModeDiscussion {
			nextIndex := -1
			for index, delivery := range round.Deliveries {
				if processed[delivery.DeliveryID] || delivery.Status == GroupDeliveryCompleted || delivery.Status == GroupDeliveryCancelled {
					continue
				}
				if nextIndex < 0 || delivery.TurnIndex < round.Deliveries[nextIndex].TurnIndex {
					nextIndex = index
				}
			}
			if nextIndex >= 0 {
				processed[round.Deliveries[nextIndex].DeliveryID] = true
				batch = append(batch, nextIndex)
			}
		} else {
			for index, delivery := range round.Deliveries {
				if !processed[delivery.DeliveryID] && delivery.Status != GroupDeliveryCompleted && delivery.Status != GroupDeliveryCancelled {
					processed[delivery.DeliveryID] = true
					batch = append(batch, index)
				}
			}
		}
		if len(batch) == 0 {
			break
		}
		results := make(chan deliveryResult, len(batch))
		for _, index := range batch {
			delivery := round.Deliveries[index]
			currentRound := round
			currentState := state
			go func(index int, delivery GroupDelivery) {
				item := deliveryResult{index: index, delivery: delivery}
				defer func() { results <- item }()
				finish := func(status string, err error) {
					item.delivery.Status = status
					if err != nil {
						item.delivery.Error = err.Error()
					}
					item.delivery.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
					item.bookkeepingErr = e.updateGroupDelivery(ctx, scope, currentRound, item.delivery)
					if status == GroupDeliveryFailed && err != nil && safeEmit != nil {
						_ = safeEmit(Event{Type: "group.delivery.failed", ConversationID: currentRound.ConversationID, Scope: scope, GroupRoundID: currentRound.RoundID, DeliveryID: delivery.DeliveryID, DeliveryIndex: index, CharacterID: delivery.CharacterID, CharacterName: delivery.CharacterName, RoleOwnerID: delivery.RoleOwnerID, Text: DescribeRuntimeError(err).UserMessage})
					}
				}
				if cause := context.Cause(ctx); cause != nil {
					item.err = cause
					finish(GroupDeliveryCancelled, cause)
					return
				}
				if delivery.Source == "character" || delivery.Source == "discussion" {
					if port, ok := e.data.(coordination.GroupStatePort); ok {
						current, err := port.GroupState(ctx, scope, currentRound.ConversationID)
						if err != nil {
							item.err = err
							finish(GroupDeliveryFailed, err)
							return
						}
						if !current.Settings.AllowCharacterMentions || current.SettingsRevision != currentState.SettingsRevision {
							if delivery.Source == "discussion" {
								item.err = errors.New("群设置已变化，多角色讨论已停止")
							} else {
								item.err = errors.New("角色互相 @ 已关闭")
							}
							finish(GroupDeliveryCancelled, item.err)
							return
						}
						if delivery.Source == "discussion" && current.MembershipRevision != currentRound.MembershipRevision {
							item.err = errors.New("群成员已变化，多角色讨论已停止")
							finish(GroupDeliveryCancelled, item.err)
							return
						}
						currentState = current
					}
				}
				member, active := currentState.ActiveMember(delivery.CharacterID)
				if !active || member.MemberID != delivery.MemberID {
					item.err = ErrGroupTargetInvalid
					finish(GroupDeliveryFailed, item.err)
					return
				}
				item.delivery.Status = GroupDeliveryStarting
				item.delivery.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
				if err := e.updateGroupDelivery(ctx, scope, currentRound, item.delivery); err != nil {
					item.bookkeepingErr = err
					return
				}
				if safeEmit != nil {
					if err := safeEmit(Event{Type: "group.delivery.started", ConversationID: currentRound.ConversationID, Scope: scope, GroupRoundID: currentRound.RoundID, DeliveryID: delivery.DeliveryID, ChildRequestID: delivery.ChildRequestID, DeliveryIndex: index, CharacterID: delivery.CharacterID, CharacterName: delivery.CharacterName, RoleOwnerID: delivery.RoleOwnerID}); err != nil {
						item.err = err
						finish(GroupDeliveryFailed, err)
						return
					}
				}
				item.delivery.Status = GroupDeliveryRunning
				if err := e.updateGroupDelivery(ctx, scope, currentRound, item.delivery); err != nil {
					item.bookkeepingErr = err
					return
				}
				message, sourceMessageID := currentRound.TriggerMessage, currentRound.TriggerMessageID
				if delivery.Source == "character" {
					for _, invitation := range currentRound.Invitations {
						if invitation.TargetMemberID != delivery.MemberID || invitation.Status != GroupInvitationAccepted {
							continue
						}
						resource, err := e.data.(coordination.ResourcePort).Resource(ctx, scope, "message", invitation.SourceMessageID)
						if err != nil || resource == nil {
							if err == nil {
								err = errors.New("接话来源消息不存在")
							}
							item.err = err
							finish(GroupDeliveryFailed, err)
							return
						}
						var source struct {
							Content string `json:"content"`
						}
						if err := json.Unmarshal(resource.Body, &source); err != nil {
							item.err = err
							finish(GroupDeliveryFailed, err)
							return
						}
						message, sourceMessageID = source.Content, invitation.SourceMessageID
						break
					}
				}
				if delivery.Source == "discussion" && delivery.TurnIndex > 0 {
					var previous *GroupDelivery
					for previousIndex := range currentRound.Deliveries {
						candidate := &currentRound.Deliveries[previousIndex]
						if candidate.TurnIndex == delivery.TurnIndex-1 {
							previous = candidate
							break
						}
					}
					if previous == nil || previous.Status != GroupDeliveryCompleted || previous.MessageID == "" {
						item.err = errors.New("讨论上一位角色发言不存在")
						finish(GroupDeliveryFailed, item.err)
						return
					}
					resource, err := e.data.(coordination.ResourcePort).Resource(ctx, scope, "message", previous.MessageID)
					if err != nil || resource == nil {
						if err == nil {
							err = errors.New("讨论上一位角色发言不存在")
						}
						item.err = err
						finish(GroupDeliveryFailed, err)
						return
					}
					var source struct {
						Content string `json:"content"`
					}
					if err := json.Unmarshal(resource.Body, &source); err != nil {
						item.err = err
						finish(GroupDeliveryFailed, err)
						return
					}
					message, sourceMessageID = source.Content, previous.MessageID
				}
				request := Request{SpaceID: scope.SpaceID, DeviceID: scope.InitiatorDeviceID, TargetDeviceID: scope.TargetDeviceID, CoreID: scope.CoreID, RoleID: delivery.CharacterID, ConversationID: currentRound.ConversationID, RequestID: delivery.ChildRequestID, Message: message, Group: &GroupMessageOptions{Mode: currentRound.Mode}, PreparedReply: &PreparedGroupReply{UserMessageID: currentRound.TriggerMessageID, UserMessageRevision: 1, UserSequence: currentRound.TriggerSequence, GroupRoundID: currentRound.RoundID, DeliveryID: delivery.DeliveryID, SourceMessageID: sourceMessageID, DeliveryIndex: index}}
				if delivery.Source == "user" {
					request.PreparedReply.ContextSequence = currentRound.TriggerSequence
				}
				var childEmit func(Event) error
				if safeEmit != nil {
					childEmit = func(event Event) error {
						return safeEmit(groupDeliveryEvent(event, delivery, index, currentRound.RoundID))
					}
				}
				response, runErr := e.runEventsLocked(ctx, request, childEmit)
				item.response = response
				item.delivery.TurnID, item.delivery.ExecutionID = response.TurnID, response.ExecutionID
				if runErr != nil && (context.Cause(ctx) != nil || response.Interrupted || errors.Is(runErr, ErrInterrupted) || errors.Is(runErr, context.Canceled) || strings.TrimSpace(runErr.Error()) == ErrInterrupted.Error()) {
					item.err = context.Cause(ctx)
					if item.err == nil {
						item.err = ErrInterrupted
					}
					finish(GroupDeliveryCancelled, item.err)
					return
				}
				if runErr != nil {
					item.err = runErr
					finish(GroupDeliveryFailed, runErr)
					return
				}
				item.delivery.MessageID = response.RequestID + "/assistant"
				item.response.GroupRoundID, item.response.DeliveryID, item.response.CharacterName, item.response.CharacterAvatar = currentRound.RoundID, delivery.DeliveryID, delivery.CharacterName, ""
				finish(GroupDeliveryCompleted, nil)
				if item.bookkeepingErr == nil && safeEmit != nil {
					_ = safeEmit(Event{Type: "group.delivery.completed", ConversationID: currentRound.ConversationID, Scope: response.Scope, GroupRoundID: currentRound.RoundID, DeliveryID: delivery.DeliveryID, DeliveryIndex: index, CharacterID: delivery.CharacterID, CharacterName: delivery.CharacterName, RoleOwnerID: delivery.RoleOwnerID})
				}
			}(index, delivery)
		}
		completed := make([]deliveryResult, len(batch))
		for range batch {
			item := <-results
			for i, index := range batch {
				if index == item.index {
					completed[i] = item
					break
				}
			}
		}
		haltDiscussion := false
		haltReason := errDiscussionHalted
		for _, item := range completed {
			if item.bookkeepingErr != nil {
				return result, item.bookkeepingErr
			}
			round.Deliveries[item.index] = item.delivery
			if item.delivery.Status == GroupDeliveryCancelled {
				if errors.Is(item.err, errDiscussionHalted) {
					failed = true
				} else {
					cancelled = true
					haltReason = item.err
				}
				if round.Mode == GroupModeDiscussion {
					haltDiscussion = true
				}
			}
			if item.delivery.Status == GroupDeliveryFailed {
				failed = true
				if round.Mode == GroupModeDiscussion {
					haltDiscussion = true
				}
			}
			if item.delivery.Status == GroupDeliveryCompleted {
				result.Deliveries = append(result.Deliveries, item.response)
			}
		}
		if latestResource, latestErr := e.data.(coordination.ResourcePort).Resource(ctx, scope, "group-round", round.RoundID); latestErr == nil && latestResource != nil {
			var latest GroupRound
			if json.Unmarshal(latestResource.Body, &latest) == nil && latest.RoundID == round.RoundID {
				current := make(map[string]GroupDelivery, len(round.Deliveries))
				for _, item := range round.Deliveries {
					current[item.DeliveryID] = item
				}
				for latestIndex := range latest.Deliveries {
					if item, ok := current[latest.Deliveries[latestIndex].DeliveryID]; ok {
						latest.Deliveries[latestIndex] = item
					}
				}
				round = latest
			}
		}
		if haltDiscussion {
			for index := range round.Deliveries {
				delivery := &round.Deliveries[index]
				if delivery.Status == GroupDeliveryCompleted || delivery.Status == GroupDeliveryFailed || delivery.Status == GroupDeliveryCancelled {
					continue
				}
				delivery.Status = GroupDeliveryCancelled
				delivery.Error = haltReason.Error()
				delivery.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
				if err := e.updateGroupDelivery(ctx, scope, round, *delivery); err != nil {
					return result, err
				}
			}
			break
		}
	}
	if cancelled {
		round.Status = GroupRoundCancelled
	} else if failed {
		round.Status = GroupRoundCompletedWithErrors
	} else {
		round.Status = GroupRoundCompleted
	}
	round.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := e.updateGroupRound(ctx, scope, round); err != nil {
		return result, err
	}
	if err := e.coordination.CompleteRequest(context.WithoutCancel(ctx), scope); err != nil {
		return result, err
	}
	if round.Status != GroupRoundCancelled && len(result.Deliveries) > 0 {
		_ = e.finishOwnedGroupTitle(ctx, scope, state, round.TriggerMessage, result.Deliveries)
	}
	result.Status = round.Status
	if emit != nil {
		_ = emit(Event{Type: "group.round.completed", ConversationID: round.ConversationID, Scope: scope, GroupRoundID: round.RoundID})
	}
	if cause := context.Cause(ctx); cause != nil {
		return result, cause
	}
	return result, nil
}

func (e *Engine) updateGroupDelivery(ctx context.Context, scope coordination.ExecutionScope, round GroupRound, delivery GroupDelivery) error {
	encoded, err := json.Marshal(delivery)
	if err != nil {
		return err
	}
	bookkeeping := context.WithoutCancel(ctx)
	resource, err := e.data.(coordination.ResourcePort).Resource(bookkeeping, scope, "group-delivery", delivery.DeliveryID)
	if err != nil {
		return err
	}
	if resource == nil {
		return coordination.ErrResourceVersion
	}
	copyScope := scope
	copyScope.RequestID = scope.RequestID + "|delivery|" + delivery.DeliveryID + "|" + delivery.Status
	_, err = e.commit(bookkeeping, coordination.Commit{Scope: copyScope, Mutations: []coordination.Mutation{{Kind: "group-delivery", ID: delivery.DeliveryID, RoleID: coordination.StorageRole(scope, "group-delivery"), ExpectedRevision: resource.Revision, Body: encoded}}})
	return err
}

func (e *Engine) updateGroupRound(ctx context.Context, scope coordination.ExecutionScope, round GroupRound) error {
	encoded, err := json.Marshal(round)
	if err != nil {
		return err
	}
	bookkeeping := context.WithoutCancel(ctx)
	resource, err := e.data.(coordination.ResourcePort).Resource(bookkeeping, scope, "group-round", round.RoundID)
	if err != nil {
		return err
	}
	if resource == nil {
		return coordination.ErrResourceVersion
	}
	copyScope := scope
	copyScope.RequestID = scope.RequestID + "|round|" + round.Status
	_, err = e.commit(bookkeeping, coordination.Commit{Scope: copyScope, Mutations: []coordination.Mutation{{Kind: "group-round", ID: round.RoundID, RoleID: coordination.StorageRole(scope, "group-round"), ExpectedRevision: resource.Revision, Body: encoded}}})
	return err
}
