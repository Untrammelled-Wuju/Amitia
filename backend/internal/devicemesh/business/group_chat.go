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
	"time"

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
)

var (
	ErrGroupMemberChanged   = errors.New("群成员已变化，请刷新后重试")
	ErrGroupTargetInvalid   = errors.New("目标角色不在当前群聊中或不可执行")
	ErrGroupBudgetExhausted = errors.New("本轮角色接话已达到上限，可手动继续")
)

type GroupMessageOptions struct {
	ExpectedMembershipRevision *int64 `json:"expectedMembershipRevision,omitempty"`
	ExpectedSettingsRevision   *int64 `json:"expectedSettingsRevision,omitempty"`
	ReplyToMessageID           string `json:"replyToMessageId,omitempty"`
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
}

type GroupReplyContext struct {
	Round        GroupRound
	Delivery     GroupDelivery
	State        coordination.GroupState
	SourceMember coordination.GroupMember
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
	CharacterAvatar   string `json:"characterAvatar,omitempty"`
	CharacterRevision int64  `json:"characterRevision"`
	OrderIndex        int    `json:"orderIndex"`
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
	TriggerMessageID   string            `json:"triggerMessageId"`
	TriggerMessage     string            `json:"triggerMessage"`
	TriggerSequence    int64             `json:"triggerSequence"`
	Status             string            `json:"status"`
	MembershipRevision int64             `json:"membershipRevision"`
	SettingsRevision   int64             `json:"settingsRevision"`
	AutoReplyCount     int               `json:"autoReplyCount"`
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
	request.RequestID = input.RequestID
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

func (e *Engine) AcceptGroupInvitations(ctx context.Context, inference Inference, sourceMessageID string, proposals []GroupInviteProposal) error {
	if len(proposals) == 0 {
		return nil
	}
	if inference.Group == nil || !inference.Group.State.IsGroup() || !inference.Group.State.Settings.AllowCharacterMentions {
		return errors.New("当前会话未允许角色互相邀请")
	}
	if sourceMessageID == "" || inference.Scope.ConversationRoleID == "" {
		return coordination.ErrWrongOwner
	}
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
	now := time.Now().UTC().Format(time.RFC3339Nano)
	mutations := make([]coordination.Mutation, 0)
	seen := make(map[string]bool)
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
		if round.AutoReplyCount >= inference.Group.State.Settings.MaxAutoReplies {
			round.Invitations = append(round.Invitations, GroupInvitation{InvitationID: uuid.NewString(), ConversationID: round.ConversationID, RoundID: round.RoundID, SourceMessageID: sourceMessageID, SourceDeliveryID: inference.Group.Delivery.DeliveryID, SourceCharacterID: inference.Scope.RoleID, TargetMemberID: member.MemberID, TargetCharacterID: member.CharacterID, TargetCharacterName: member.CharacterName, Status: "rejected", CreatedAt: now})
			continue
		}
		round.AutoReplyCount++
		invitation := GroupInvitation{InvitationID: uuid.NewString(), ConversationID: round.ConversationID, RoundID: round.RoundID, SourceMessageID: sourceMessageID, SourceDeliveryID: inference.Group.Delivery.DeliveryID, SourceCharacterID: inference.Scope.RoleID, TargetMemberID: member.MemberID, TargetCharacterID: member.CharacterID, TargetCharacterName: member.CharacterName, Status: GroupInvitationAccepted, CreatedAt: now}
		round.Invitations = append(round.Invitations, invitation)
		delivery := GroupDelivery{DeliveryID: uuid.NewString(), GroupRoundID: round.RoundID, ChildRequestID: round.RootRequestID + "/reply/" + uuid.NewString(), MemberID: member.MemberID, CharacterID: member.CharacterID, RoleOwnerID: member.RoleOwnerID, CharacterName: member.CharacterName, CharacterAvatar: member.CharacterAvatar, CharacterRevision: member.CharacterRevision, OrderIndex: member.OrderIndex, Source: "character", Status: GroupDeliveryQueued, CreatedAt: now, UpdatedAt: now}
		round.Deliveries = append(round.Deliveries, delivery)
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
	return "round/" + hex.EncodeToString(digest[:])
}

func normalizeGroupMention(member coordination.GroupMember) GroupMention {
	return GroupMention{MemberID: member.MemberID, CharacterID: member.CharacterID, RoleOwnerID: member.RoleOwnerID}
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
		request.RequestID = input.RequestID
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
	activeRoles := make(map[string]bool)
	for _, member := range members {
		if member.Status == coordination.GroupMemberStatusActive {
			activeCount++
			activeRoles[member.CharacterID] = true
		}
	}
	for _, inputMember := range input.Members {
		role, found := roleByID[strings.TrimSpace(inputMember.CharacterID)]
		if !found || role.Disabled {
			return GroupMutationResponse{}, ErrGroupTargetInvalid
		}
		if activeRoles[role.ID] {
			continue
		}
		if activeCount >= coordination.GroupMaxMembers {
			return GroupMutationResponse{}, errors.New("群聊最多支持 8 个角色")
		}
		historyStart := int64(0)
		if !inputMember.ShareHistory {
			historyStart = state.LastMessageSequence + 1
		}
		members = append(members, coordination.GroupMember{MemberID: "member-" + uuid.NewString(), CharacterID: role.ID, RoleOwnerID: scope.RoleOwnerID, CharacterName: role.Name, CharacterAvatar: role.Avatar, CharacterRevision: role.Revision, Status: coordination.GroupMemberStatusActive, OrderIndex: activeCount, JoinedSequence: state.LastMessageSequence + 1, HistoryStart: historyStart})
		activeCount++
		activeRoles[role.ID] = true
	}
	if len(activeRoles) < 2 {
		return GroupMutationResponse{}, errors.New("至少需要邀请一个有效角色")
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
	request.RequestID = input.RequestID
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
	request.RequestID = input.RequestID
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
		return GroupMutationResponse{}, coordination.ErrResourceVersion
	}
	resource, err := e.data.(coordination.ResourcePort).Resource(ctx, scope, "conversation", state.ConversationID)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	document, err := groupConversationDocument(resource)
	if err != nil {
		return GroupMutationResponse{}, err
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" || len(name) > 128 {
			return GroupMutationResponse{}, errors.New("群名称无效")
		}
		document["title"] = name
		state.Title = name
	}
	if input.AllowCharacterMentions != nil {
		state.Settings.AllowCharacterMentions = *input.AllowCharacterMentions
	}
	if input.MaxAutoReplies != nil {
		if *input.MaxAutoReplies < 0 || *input.MaxAutoReplies > coordination.GroupDefaultAutoReplies {
			return GroupMutationResponse{}, errors.New("自动接话上限无效")
		}
		state.Settings.MaxAutoReplies = *input.MaxAutoReplies
	}
	if len(input.MemberOrder) > 0 {
		order := make(map[string]int, len(input.MemberOrder))
		for index, memberID := range input.MemberOrder {
			order[memberID] = index
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
			state.Members[index].OrderIndex = index
		}
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
	if request.ConversationID == "" || request.RequestID == "" || request.Group == nil {
		return GroupResponse{}, errors.New("群聊请求缺少会话、请求编号或能力声明")
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
	if request.Group.ExpectedMembershipRevision != nil && state.MembershipRevision != *request.Group.ExpectedMembershipRevision {
		return GroupResponse{}, ErrGroupMemberChanged
	}
	if request.Group.ExpectedSettingsRevision != nil && state.SettingsRevision != *request.Group.ExpectedSettingsRevision {
		return GroupResponse{}, coordination.ErrResourceVersion
	}
	targets, err := resolveGroupTargets(state, request.Mentions, false)
	if err != nil {
		return GroupResponse{}, err
	}
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
		return e.executeGroupRound(ctx, scope, state, existing, emit)
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
	for _, member := range targets {
		deliveryID := uuid.NewString()
		deliveries = append(deliveries, GroupDelivery{DeliveryID: deliveryID, GroupRoundID: roundID, ChildRequestID: request.RequestID + "/reply/" + deliveryID, MemberID: member.MemberID, CharacterID: member.CharacterID, RoleOwnerID: member.RoleOwnerID, CharacterName: member.CharacterName, CharacterAvatar: member.CharacterAvatar, CharacterRevision: member.CharacterRevision, OrderIndex: member.OrderIndex, Source: "user", Status: GroupDeliveryQueued, CreatedAt: now, UpdatedAt: now})
	}
	conversation["updatedAt"] = now
	conversation["lastMessageSequence"] = userSequence
	conversation["characterId"] = state.ConversationRoleID
	round := GroupRound{RoundID: roundID, ConversationID: state.ConversationID, RootRequestID: request.RequestID, TriggerMessageID: userMessageID, TriggerMessage: request.Message, TriggerSequence: userSequence, Status: GroupRoundQueued, MembershipRevision: state.MembershipRevision, SettingsRevision: state.SettingsRevision, CreatedAt: now, UpdatedAt: now, Deliveries: deliveries}
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
	fingerprintData := map[string]any{"conversationId": request.ConversationID, "message": request.Message, "characterId": scope.RoleID, "mentions": request.Mentions, "group": request.Group, "attachments": request.Attachments}
	digest := sha256.Sum256(body(fingerprintData))
	location, firstRequest, err := e.coordination.RegisterRequest(ctx, scope, hex.EncodeToString(digest[:]), request.ConversationID)
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
	if request.RequestID == "" || request.ConversationID == "" || strings.TrimSpace(input.SourceMessageID) == "" || len(input.Members) == 0 {
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
	roundID := groupRoundID(scope.SpaceID, scope.InitiatorDeviceID, request.RequestID)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	round := GroupRound{RoundID: roundID, ConversationID: state.ConversationID, RootRequestID: request.RequestID, TriggerMessageID: input.SourceMessageID, TriggerMessage: userMessage.Content, TriggerSequence: userMessage.Sequence, Status: GroupRoundQueued, MembershipRevision: state.MembershipRevision, SettingsRevision: state.SettingsRevision, CreatedAt: now, UpdatedAt: now}
	mutations := make([]coordination.Mutation, 0, len(targets)+1)
	for _, member := range targets {
		delivery := GroupDelivery{DeliveryID: uuid.NewString(), GroupRoundID: roundID, ChildRequestID: request.RequestID + "/reply/" + uuid.NewString(), MemberID: member.MemberID, CharacterID: member.CharacterID, RoleOwnerID: member.RoleOwnerID, CharacterName: member.CharacterName, CharacterAvatar: member.CharacterAvatar, CharacterRevision: member.CharacterRevision, OrderIndex: member.OrderIndex, Source: "user", Status: GroupDeliveryQueued, CreatedAt: now, UpdatedAt: now}
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
	fingerprintData := map[string]any{"conversationId": state.ConversationID, "sourceMessageId": input.SourceMessageID, "mentions": input.Members}
	digest := sha256.Sum256(body(fingerprintData))
	location, firstRequest, err := e.coordination.RegisterRequest(ctx, scope, hex.EncodeToString(digest[:]), state.ConversationID)
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
	if request.RequestID == "" || request.ConversationID == "" || strings.TrimSpace(roundID) == "" || strings.TrimSpace(input.DeliveryID) == "" {
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
	retryRoundID := groupRoundID(scope.SpaceID, scope.InitiatorDeviceID, request.RequestID)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	delivery := GroupDelivery{DeliveryID: uuid.NewString(), GroupRoundID: retryRoundID, ChildRequestID: request.RequestID + "/reply/" + uuid.NewString(), MemberID: member.MemberID, CharacterID: member.CharacterID, RoleOwnerID: member.RoleOwnerID, CharacterName: member.CharacterName, CharacterAvatar: member.CharacterAvatar, CharacterRevision: member.CharacterRevision, OrderIndex: member.OrderIndex, Source: "retry:" + failed.DeliveryID, Status: GroupDeliveryQueued, CreatedAt: now, UpdatedAt: now}
	round := GroupRound{RoundID: retryRoundID, ConversationID: state.ConversationID, RootRequestID: request.RequestID, TriggerMessageID: original.TriggerMessageID, TriggerMessage: original.TriggerMessage, TriggerSequence: original.TriggerSequence, Status: GroupRoundQueued, MembershipRevision: state.MembershipRevision, SettingsRevision: state.SettingsRevision, CreatedAt: now, UpdatedAt: now, Deliveries: []GroupDelivery{delivery}}
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

func (e *Engine) executeGroupRound(ctx context.Context, scope coordination.ExecutionScope, state coordination.GroupState, round GroupRound, emit func(Event) error) (GroupResponse, error) {
	result := GroupResponse{ConversationID: round.ConversationID, RequestID: round.RootRequestID, GroupRoundID: round.RoundID, UserMessageID: round.TriggerMessageID, Saved: true}
	round.Status = GroupRoundRunning
	failed := false
	for index := range round.Deliveries {
		delivery := &round.Deliveries[index]
		if delivery.Status == GroupDeliveryCompleted || delivery.Status == GroupDeliveryCancelled {
			continue
		}
		if cause := context.Cause(ctx); cause != nil {
			delivery.Status, delivery.Error = GroupDeliveryCancelled, cause.Error()
			delivery.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := e.updateGroupDelivery(ctx, scope, round, *delivery); err != nil {
				return result, err
			}
			failed = true
			continue
		}
		if delivery.Source == "character" {
			if port, ok := e.data.(coordination.GroupStatePort); ok {
				currentGroup, groupErr := port.GroupState(ctx, scope, round.ConversationID)
				if groupErr == nil && (!currentGroup.Settings.AllowCharacterMentions || currentGroup.SettingsRevision != state.SettingsRevision) {
					delivery.Status, delivery.Error = GroupDeliveryCancelled, "角色互相 @ 已关闭"
					delivery.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
					if err := e.updateGroupDelivery(ctx, scope, round, *delivery); err != nil {
						return result, err
					}
					failed = true
					continue
				}
				if groupErr == nil {
					state = currentGroup
				}
			}
		}
		delivery.Status = GroupDeliveryStarting
		delivery.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := e.updateGroupDelivery(ctx, scope, round, *delivery); err != nil {
			return result, err
		}
		if emit != nil {
			if err := emit(Event{Type: "group.delivery.started", GroupRoundID: round.RoundID, DeliveryID: delivery.DeliveryID, DeliveryIndex: index, CharacterID: delivery.CharacterID, CharacterName: delivery.CharacterName, CharacterAvatar: delivery.CharacterAvatar, RoleOwnerID: delivery.RoleOwnerID}); err != nil {
				failed = true
				delivery.Status, delivery.Error = GroupDeliveryFailed, err.Error()
				continue
			}
		}
		member, active := state.ActiveMember(delivery.CharacterID)
		if !active || member.MemberID != delivery.MemberID {
			failed = true
			delivery.Status, delivery.Error = GroupDeliveryFailed, ErrGroupTargetInvalid.Error()
			if err := e.updateGroupDelivery(ctx, scope, round, *delivery); err != nil {
				return result, err
			}
			continue
		}
		delivery.Status = GroupDeliveryRunning
		if err := e.updateGroupDelivery(ctx, scope, round, *delivery); err != nil {
			return result, err
		}
		request := Request{SpaceID: scope.SpaceID, DeviceID: scope.InitiatorDeviceID, TargetDeviceID: scope.TargetDeviceID, CoreID: scope.CoreID, RoleID: delivery.CharacterID, ConversationID: round.ConversationID, RequestID: delivery.ChildRequestID, Message: round.TriggerMessage, Group: &GroupMessageOptions{}, PreparedReply: &PreparedGroupReply{UserMessageID: round.TriggerMessageID, UserMessageRevision: 1, UserSequence: round.TriggerSequence, GroupRoundID: round.RoundID, DeliveryID: delivery.DeliveryID, SourceMessageID: round.TriggerMessageID, DeliveryIndex: index}}
		var childEmit func(Event) error
		if emit != nil {
			childEmit = func(event Event) error {
				event.GroupRoundID, event.DeliveryID, event.DeliveryIndex = round.RoundID, delivery.DeliveryID, index
				event.CharacterID, event.CharacterName, event.CharacterAvatar, event.RoleOwnerID = delivery.CharacterID, delivery.CharacterName, delivery.CharacterAvatar, delivery.RoleOwnerID
				return emit(event)
			}
		}
		response, runErr := e.runEventsLocked(ctx, request, childEmit)
		if runErr != nil {
			failed = true
			delivery.Status, delivery.Error = GroupDeliveryFailed, runErr.Error()
			delivery.TurnID, delivery.ExecutionID = response.TurnID, response.ExecutionID
		} else {
			delivery.Status, delivery.MessageID = GroupDeliveryCompleted, response.RequestID+"/assistant"
			delivery.TurnID, delivery.ExecutionID = response.TurnID, response.ExecutionID
			response.GroupRoundID, response.DeliveryID, response.CharacterName, response.CharacterAvatar = round.RoundID, delivery.DeliveryID, delivery.CharacterName, delivery.CharacterAvatar
			result.Deliveries = append(result.Deliveries, response)
		}
		delivery.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := e.updateGroupDelivery(ctx, scope, round, *delivery); err != nil {
			return result, err
		}
		if latestResource, latestErr := e.data.(coordination.ResourcePort).Resource(ctx, scope, "group-round", round.RoundID); latestErr == nil && latestResource != nil {
			var latest GroupRound
			if json.Unmarshal(latestResource.Body, &latest) == nil && latest.RoundID == round.RoundID {
				round = latest
			}
		}
	}
	if failed {
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
	result.Status = round.Status
	if emit != nil {
		_ = emit(Event{Type: "group.round.completed", GroupRoundID: round.RoundID})
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
