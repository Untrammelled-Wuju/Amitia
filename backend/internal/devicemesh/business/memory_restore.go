package business

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
)

func (e *Engine) RestoreMemoryArchive(ctx context.Context, request Request, input dataportability.ImportRequest, archive MemoryArchive) error {
	if input.OperationID == "" || archive.Format != "amitia-owned-memory-v1" {
		return errors.New("记忆恢复请求无效")
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return err
	}
	defer finish()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return coordination.ErrWrongOwner
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return err
	}
	remapRole := func(id string) string {
		if id == coordination.GlobalMemoryRole {
			return id
		}
		if input.IdentityMap != nil {
			if target, ok := input.IdentityMap.GetCharacter(id); ok {
				return target
			}
		}
		if input.DefaultCharacterID != "" {
			return input.DefaultCharacterID
		}
		return id
	}
	identities := map[string]string{}
	groups := map[string][]MemoryConsumerEntry{}
	for _, entry := range archive.Entries {
		actorID := remapRole(entry.Scope.RoleID)
		if actorID == "" {
			actorID = scope.RoleID
		}
		actor, err := coordination.ResolveRole(actorID, roles)
		if err != nil {
			return err
		}
		partition := remapRole(entry.RoleID)
		if entry.Kind == "checkpoint" || entry.Kind == "working" || entry.Kind == "summary" {
			partition = actor.ID
		}
		current := scope
		current.RoleID, current.RoleRevision, current.MemoryRoleID, current.MemoryViewAll = actor.ID, actor.Revision, partition, false
		entry.Scope = current
		id := entry.ID
		if input.CharacterPolicy == dataportability.CollisionDuplicate || entry.RoleID != partition {
			id = memoryManagementHash([]string{"memory-restore", input.OperationID, partition, entry.Kind, entry.ID})[:32]
			if entry.Kind == "checkpoint" {
				id = "memory-candidate/restore/" + id
			}
			if entry.Kind == "graph" && strings.HasPrefix(entry.ID, "memory-relation/") {
				id = "memory-relation/" + id
			}
		}
		identities[entry.Kind+"/"+entry.ID] = id
		if entry.Kind == "memory" && input.IdentityMap != nil {
			input.IdentityMap.AddMemory(entry.ID, id)
		}
		entry.RoleID = coordination.StorageRole(current, entry.Kind)
		groups[actor.ID+"\x00"+partition] = append(groups[actor.ID+"\x00"+partition], entry)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entries := groups[key]
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].Kind == "memory" && entries[j].Kind != "memory" })
		current := entries[0].Scope
		for offset := 0; offset < len(entries); offset += 8 {
			end := min(offset+8, len(entries))
			current.RequestID = "memory-restore/" + memoryManagementHash([]interface{}{input.OperationID, key, offset})[:48]
			active := coordination.WithScope(ctx, current)
			originals := []coordination.Resource{}
			for _, entry := range entries[offset:end] {
				originals = append(originals, entry.Resource)
			}
			fingerprint := memoryManagementHash([]interface{}{summaryAuthority(current), originals, input.OperationID})
			receiptID := "memory-restore/" + current.RequestID
			if receipt, err := port.Resource(active, current, "checkpoint", receiptID); err != nil {
				return err
			} else if receipt != nil {
				var saved struct {
					Hash string `json:"hash"`
				}
				if json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != fingerprint {
					return coordination.ErrRequestConflict
				}
				continue
			}
			mutations := []coordination.Mutation{}
			for _, entry := range entries[offset:end] {
				id := identities[entry.Kind+"/"+entry.ID]
				previous, err := port.Resource(active, current, entry.Kind, id)
				if err != nil {
					return err
				}
				if previous != nil && input.CharacterPolicy == dataportability.CollisionSkip {
					continue
				}
				var document map[string]json.RawMessage
				if json.Unmarshal(entry.Body, &document) != nil || document == nil {
					return errors.New("记忆备份正文无效")
				}
				document["executionScope"] = body(current)
				if entry.Kind == "graph" {
					var content map[string]json.RawMessage
					if json.Unmarshal(document["content"], &content) != nil || content == nil {
						return errors.New("记忆备份关联正文无效")
					}
					if MemoryText(content, "targetKind") == "memory" {
						target := identities["memory/"+strings.TrimPrefix(MemoryText(content, "target"), "memory:")]
						if target == "" {
							return errors.New("记忆备份缺少关联目标")
						}
						content["target"] = body(target)
						document["content"] = body(content)
					}
				}
				if conversation := MemoryText(document, "conversationId"); conversation != "" && input.IdentityMap != nil {
					document["conversationId"] = body(input.IdentityMap.RemapConversationRef(conversation))
				}
				if entry.Kind == "checkpoint" {
					var candidate OwnedMemoryCandidate
					if json.Unmarshal(entry.Body, &candidate) != nil || candidate.Type != "owned-memory-candidate" {
						return errors.New("备份候选记忆无效")
					}
					candidate.ID, candidate.ExecutionScope = id, current
					if input.IdentityMap != nil {
						candidate.ConversationID = input.IdentityMap.RemapConversationRef(candidate.ConversationID)
					}
					if candidate.State == "pending" {
						provenanceRequest := request
						provenanceRequest.RoleID, provenanceRequest.ConversationID = current.RoleID, candidate.ConversationID
						messages, dependencies, err := e.summaryHistory(active, provenanceRequest, current)
						if err != nil {
							return err
						}
						if len(messages) == 0 {
							return errors.New("无法恢复候选记忆的对话来源")
						}
						candidate.Dependencies, candidate.ProvenanceHash = dependencies, memoryManagementHash(messages)
						candidate.ConversationOrigin, candidate.HistoricalRoleID = nil, ""
					}
					document = nil
					_ = json.Unmarshal(body(candidate), &document)
				}
				source := entry.SourceID
				if source != "" {
					source = identities["memory/"+source]
					if source == "" {
						return errors.New("记忆备份缺少派生记录来源")
					}
				}
				revision := int64(0)
				if previous != nil {
					revision = previous.Revision
				}
				mutations = append(mutations, coordination.Mutation{Kind: entry.Kind, ID: id, RoleID: entry.RoleID, SourceID: source, ExpectedRevision: revision, Body: body(document)})
			}
			mutations = append(mutations, coordination.Mutation{Kind: "checkpoint", ID: receiptID, RoleID: current.RoleID, Body: body(map[string]string{"hash": fingerprint})})
			if _, err := e.commit(active, coordination.Commit{Scope: current, Mutations: mutations}); err != nil {
				return err
			}
		}
	}
	return e.coordination.Validate(ctx, scope)
}
