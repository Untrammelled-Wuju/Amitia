package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/character/card"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedCharacterPreview struct{ services *AppServices }

func (p ownedCharacterPreview) TestChat(ctx context.Context, characterID, message string) (string, error) {
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil || actor.DeviceID == "" || p.services == nil || p.services.OwnedBusiness == nil {
		return "", errors.New("角色预览缺少有效设备身份")
	}
	request := business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), TargetDeviceID: actor.DeviceID.String(), RoleID: characterID, RequestID: uuid.NewString(), Message: message, CoreID: p.services.OwnedIngress.coreID}
	response, err := p.services.OwnedBusiness.PreviewReply(ctx, request)
	return response.Text, err
}

func (p ownedCharacterPreview) ImportCardWorldbook(ctx context.Context, importID, sourceHash, spaceID, characterID string, entries []card.CharacterBookEntry) (int, error) {
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil || actor.DeviceID == "" || p.services == nil || p.services.OwnedBusiness == nil || p.services.OwnedIngress == nil || actor.SpaceID.String() != spaceID || characterID == "" {
		return 0, errors.New("角色卡世界书导入缺少有效设备身份")
	}
	prepared := make([]business.WorldbookRule, 0, len(entries))
	indexes := make([]int, 0, len(entries))
	for index, entry := range entries {
		rule, valid := cardBookWorldbookRule(entry, sourceHash, index)
		if !valid {
			continue
		}
		prepared = append(prepared, rule)
		indexes = append(indexes, index)
	}
	if len(prepared) == 0 {
		return 0, nil
	}
	request := business.Request{
		SpaceID:        actor.SpaceID.String(),
		DeviceID:       actor.DeviceID.String(),
		TargetDeviceID: actor.DeviceID.String(),
		CoreID:         p.services.OwnedIngress.coreID,
		RoleID:         characterID,
		RequestID:      uuid.NewString(),
	}
	view, err := p.services.OwnedBusiness.Query(ctx, request, coordination.DataQuery{ResourceKind: "worldbook", Management: true, Limit: 128})
	if err != nil {
		return 0, err
	}
	expectedScope := view.Scope
	imported := 0
	for index, rule := range prepared {
		requestID := cardWorldbookRequestID(importID, indexes[index])
		result, err := p.services.OwnedBusiness.ManageWorldbook(ctx, request, business.WorldbookMutation{
			ExpectedScope:  &expectedScope,
			RequestID:      requestID,
			RoleID:         characterID,
			TargetDeviceID: actor.DeviceID.String(),
			Action:         "create",
			Rule:           rule,
		})
		if err != nil {
			return imported, err
		}
		if !result.Saved || result.Rule == nil || result.Scope.ResourceOwnerID != expectedScope.ResourceOwnerID || result.Acknowledgement.OwnerID != expectedScope.ResourceOwnerID || result.Acknowledgement.RequestID != requestID {
			return imported, errors.New("角色卡世界书未获得数据所有者保存确认")
		}
		imported++
	}
	return imported, nil
}

func (p ownedCharacterPreview) ExportCardWorldbook(ctx context.Context, spaceID, characterID string) (*card.CharacterBook, error) {
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil || actor.DeviceID == "" || p.services == nil || p.services.OwnedBusiness == nil || p.services.OwnedIngress == nil || actor.SpaceID.String() != spaceID || characterID == "" {
		return nil, errors.New("角色卡世界书导出缺少有效设备身份")
	}
	request := business.Request{
		SpaceID:        actor.SpaceID.String(),
		DeviceID:       actor.DeviceID.String(),
		TargetDeviceID: actor.DeviceID.String(),
		CoreID:         p.services.OwnedIngress.coreID,
		RoleID:         characterID,
		RequestID:      uuid.NewString(),
	}
	view, err := p.services.OwnedBusiness.Query(ctx, request, coordination.DataQuery{ResourceKind: "worldbook", Management: true, Limit: 128})
	if err != nil {
		return nil, err
	}
	book := &card.CharacterBook{Entries: make([]card.CharacterBookEntry, 0)}
	for _, resource := range view.Snapshot.Resources {
		if resource.Kind != "worldbook" || resource.Deleted || resource.RoleID != characterID {
			continue
		}
		var rule business.WorldbookRule
		if json.Unmarshal(resource.Body, &rule) != nil {
			return nil, errors.New("角色世界书格式无效")
		}
		book.Entries = append(book.Entries, cardBookEntry(rule))
	}
	return book, nil
}

func cardBookWorldbookRule(entry card.CharacterBookEntry, sourceHash string, index int) (business.WorldbookRule, bool) {
	content := strings.TrimSpace(entry.Content)
	if content == "" || !json.Valid(entry.ID) && len(entry.ID) > 0 {
		return business.WorldbookRule{}, false
	}
	keys := cleanCardBookKeys(entry.Keys)
	secondary := cleanCardBookKeys(entry.SecondaryKeys)
	constant := entry.Constant != nil && *entry.Constant
	matchType, pattern := "keyword", strings.Join(keys, ",")
	enabled := entry.Enabled
	if constant {
		matchType, pattern = "regex", "(?s:.*)"
	} else if len(keys) == 0 {
		matchType, pattern, enabled = "regex", "a^", false
	}
	priority := entry.Priority
	if priority == 0 {
		priority = entry.InsertionOrder
	}
	if priority < -1000 {
		priority = -1000
	}
	if priority > 1000 {
		priority = 1000
	}
	return business.WorldbookRule{
		Scope: "role", CharacterID: "",
		MatchType: matchType, MatchPattern: pattern, MatchScope: "full_context",
		InjectContent: content, Priority: priority, Enabled: enabled,
		CharacterBookKeys: keys, SecondaryKeys: secondary,
		CaseSensitive: entry.CaseSensitive, Selective: entry.Selective,
		Constant: entry.Constant, InsertionOrder: entry.InsertionOrder,
		Position: entry.Position, Extensions: entry.Extensions,
		CharacterBookEntryName: entry.Name, CharacterBookEntryID: append(json.RawMessage(nil), entry.ID...),
		SourceCardHash: sourceHash, SourceEntryIndex: index,
	}, true
}

func cardBookEntry(rule business.WorldbookRule) card.CharacterBookEntry {
	keys := cleanCardBookKeys(rule.CharacterBookKeys)
	if len(keys) == 0 {
		switch rule.MatchType {
		case "keyword":
			keys = cleanCardBookKeys(strings.Split(rule.MatchPattern, ","))
		case "exact":
			keys = []string{rule.MatchPattern}
		}
	}
	return card.CharacterBookEntry{
		Name: rule.CharacterBookEntryName, ID: append(json.RawMessage(nil), rule.CharacterBookEntryID...),
		Keys: keys, SecondaryKeys: cleanCardBookKeys(rule.SecondaryKeys), Content: rule.InjectContent,
		Enabled: rule.Enabled, InsertionOrder: rule.InsertionOrder,
		CaseSensitive: rule.CaseSensitive, Selective: rule.Selective,
		Constant: rule.Constant, Position: rule.Position,
		Extensions: rule.Extensions, Priority: rule.Priority,
	}
}

func cleanCardBookKeys(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func cardWorldbookRequestID(importID string, index int) string {
	digest := sha256.Sum256([]byte(importID + "\x00" + strconv.Itoa(index)))
	return "card-" + hex.EncodeToString(digest[:24])
}
