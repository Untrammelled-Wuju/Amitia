package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"strings"
	"time"
)

type AppendedMessagePart struct {
	Type          string         `json:"type"`
	Content       string         `json:"content,omitempty"`
	ExtensionType string         `json:"extensionType,omitempty"`
	MIMEType      string         `json:"mimeType,omitempty"`
	URL           string         `json:"url,omitempty"`
	FallbackURL   string         `json:"fallbackUrl,omitempty"`
	AltText       string         `json:"altText,omitempty"`
	Width         int            `json:"width,omitempty"`
	Height        int            `json:"height,omitempty"`
	IsAnimated    bool           `json:"isAnimated,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

func normalizeAppendedMessagePart(index int, part AppendedMessagePart) (AppendedMessagePart, error) {
	part.Type = strings.ToLower(strings.TrimSpace(part.Type))
	part.Content = strings.TrimSpace(part.Content)
	part.ExtensionType = strings.TrimSpace(part.ExtensionType)
	part.MIMEType = strings.TrimSpace(part.MIMEType)
	part.URL = strings.TrimSpace(part.URL)
	part.FallbackURL = strings.TrimSpace(part.FallbackURL)
	part.AltText = strings.TrimSpace(part.AltText)
	if part.Type != "text" && part.Type != "image" && part.Type != "audio" && part.Type != "video" && part.Type != "file" {
		return AppendedMessagePart{}, fmt.Errorf("part %d has invalid type %q", index, part.Type)
	}
	if part.Type == "text" && part.Content == "" {
		return AppendedMessagePart{}, fmt.Errorf("part %d text content is required", index)
	}
	if part.Type != "text" && part.URL == "" {
		return AppendedMessagePart{}, fmt.Errorf("part %d %s url is required", index, part.Type)
	}
	return part, nil
}

type AppendOwnedMessagesRequest struct {
	ConversationID   string
	Role             string
	Source           string
	Channel          string
	ReplyToMessageID *string
	Parts            []AppendedMessagePart
}
type AppendOwnedMessagesResult struct {
	MessageIDs      []string `json:"messageIds"`
	Sequences       []int64  `json:"sequences"`
	DeliveryGroupID string   `json:"deliveryGroupId"`
	LastSequence    int64    `json:"lastSequence"`
}

func (e *Engine) AppendOwnedMessages(ctx context.Context, request Request, appendRequest AppendOwnedMessagesRequest) (result AppendOwnedMessagesResult, err error) {
	if appendRequest.ConversationID == "" || len(appendRequest.ConversationID) > 512 || len(request.RequestID) > 128 || request.RequestID == "" || len(appendRequest.Parts) == 0 || len(appendRequest.Parts) > 16 || len(body(appendRequest)) > 2<<20 {
		return result, coordination.ErrPendingLimit
	}
	appendRequest.Role = strings.ToLower(strings.TrimSpace(appendRequest.Role))
	if appendRequest.Role == "" {
		appendRequest.Role = "assistant"
	}
	if appendRequest.Role != "user" && appendRequest.Role != "assistant" {
		return result, fmt.Errorf("role must be user or assistant")
	}
	texts := 0
	for i, part := range appendRequest.Parts {
		normalized, err := normalizeAppendedMessagePart(i, part)
		if err != nil {
			return result, err
		}
		appendRequest.Parts[i] = normalized
		if normalized.Type == "text" {
			texts++
		}
	}
	if appendRequest.Role == "assistant" && texts > 1 {
		return result, fmt.Errorf("assistant append accepts at most one text part")
	}
	run := func(current context.Context) error {
		scope, ok := coordination.FromContext(current)
		if !ok {
			return coordination.ErrWrongOwner
		}
		unlock := e.lock("append-conversation\x00" + scope.ResourceOwnerID + "\x00" + scope.RoleID + "\x00" + appendRequest.ConversationID)
		defer unlock()
		port, ok := e.data.(coordination.ResourcePort)
		if !ok {
			return coordination.ErrWrongOwner
		}
		identity := sha256.Sum256(body([]string{scope.ResourceOwnerID, scope.RoleID, request.RequestID}))
		prefix := "append/" + hex.EncodeToString(identity[:])
		receiptID := prefix + "/receipt"
		fingerprint := sha256.Sum256(body(appendRequest))
		hash := hex.EncodeToString(fingerprint[:])
		receipt, err := port.Resource(current, scope, "checkpoint", receiptID)
		if err != nil {
			return err
		}
		if receipt != nil {
			var saved struct {
				Hash   string                    `json:"hash"`
				Result AppendOwnedMessagesResult `json:"result"`
			}
			if receipt.Deleted || json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != hash {
				return coordination.ErrRequestConflict
			}
			result = saved.Result
			return nil
		}
		snapshot, err := e.data.Snapshot(current, scope, coordination.DataQuery{ConversationID: appendRequest.ConversationID, Limit: 128})
		if err != nil {
			return err
		}
		if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
			return err
		}
		conversation, err := port.Resource(current, scope, "conversation", appendRequest.ConversationID)
		if err != nil {
			return err
		}
		if conversation == nil || conversation.Deleted {
			return ErrConversationUnavailable
		}
		var document map[string]any
		if err := json.Unmarshal(conversation.Body, &document); err != nil {
			return err
		}
		if document["characterId"] != nil && document["characterId"] != scope.RoleID {
			return coordination.ErrWrongOwner
		}
		channel, _ := document["channel"].(string)
		if appendRequest.Channel != "" && channel != "" && appendRequest.Channel != channel {
			return coordination.ErrWrongOwner
		}
		if channel == "" {
			channel = appendRequest.Channel
		}
		if channel == "" {
			channel = "web"
		}
		peer, _ := document["peerId"].(string)
		if appendRequest.ReplyToMessageID != nil {
			quoted, err := port.Resource(current, scope, "message", *appendRequest.ReplyToMessageID)
			if err != nil {
				return err
			}
			if quoted == nil || quoted.Deleted {
				return coordination.ErrWrongOwner
			}
			var ref struct {
				ConversationID string `json:"conversationId"`
			}
			if json.Unmarshal(quoted.Body, &ref) != nil || ref.ConversationID != appendRequest.ConversationID {
				return coordination.ErrWrongOwner
			}
		}
		sequence := nextOwnedMessageSequence(snapshot, appendRequest.ConversationID)
		mutations := []coordination.Mutation{}
		outputs := []map[string]any{}
		text := ""
		now := time.Now().UTC().Format(time.RFC3339Nano)
		result.DeliveryGroupID = prefix
		for i, part := range appendRequest.Parts {
			id := fmt.Sprintf("%s/message/%02d", prefix, i)
			content := part.Content
			if content == "" {
				content = part.AltText
			}
			if content == "" {
				content = "[" + part.Type + "]"
			}
			status := "sent"
			if channel != "web" && channel != "voice" {
				status = "sending"
			}
			message := map[string]any{"id": id, "conversationId": appendRequest.ConversationID, "characterId": scope.RoleID, "ownerId": scope.ResourceOwnerID, "role": appendRequest.Role, "content": content, "msgType": part.Type, "source": appendRequest.Source, "status": status, "requestId": request.RequestID, "sequence": sequence + int64(i), "createdAt": now, "extensionType": part.ExtensionType, "mimeType": part.MIMEType, "altText": part.AltText, "originalAsset": part.URL, "fallbackAsset": part.FallbackURL, "isAnimated": part.IsAnimated, "mediaWidth": part.Width, "mediaHeight": part.Height, "metadata": part.Metadata, "deliveryGroupId": prefix, "deliverySequence": i + 1, "replyToMessageId": appendRequest.ReplyToMessageID, "executionScope": scope}
			switch part.Type {
			case "image":
				message["imageUrl"] = part.URL
			case "audio":
				message["audioUrl"] = part.URL
			case "video":
				message["videoUrl"] = part.URL
			}
			mutations = append(mutations, coordination.Mutation{Kind: "message", ID: id, RoleID: scope.RoleID, Body: body(message)})
			result.MessageIDs = append(result.MessageIDs, id)
			result.Sequences = append(result.Sequences, sequence+int64(i))
			result.LastSequence = sequence + int64(i)
			if part.Type == "text" {
				text = part.Content
			} else {
				placement := "after_text"
				if text == "" {
					placement = "before_text"
				}
				outputs = append(outputs, map[string]any{"outputId": id, "placement": placement, "part": part})
			}
		}
		document["lastMessageSequence"], document["updatedAt"] = result.LastSequence, now
		mutations = append(mutations, coordination.Mutation{Kind: "conversation", ID: conversation.ID, RoleID: scope.RoleID, ExpectedRevision: conversation.Revision, Body: body(document)}, coordination.Mutation{Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: body(map[string]any{"hash": hash, "result": result})})
		if appendRequest.Role == "assistant" && channel != "web" && channel != "voice" && channel != "device-mesh" && peer != "" {
			deliveryScope := scope
			deliveryScope.RequestID = request.RequestID
			pending := coordination.PendingChannelDelivery{Pending: true, Scope: deliveryScope, Request: coordination.ChannelDeliveryRequest{ConversationID: appendRequest.ConversationID, RequestID: request.RequestID, Channel: channel, PeerID: peer, Text: text, Outputs: body(outputs)}}
			mutations = append(mutations, coordination.Mutation{Kind: "checkpoint", ID: "channel-delivery/" + request.RequestID, RoleID: scope.RoleID, Body: body(pending)})
		}
		return e.submit(current, scope, "append", mutations)
	}
	if request.ExpectedScope != nil {
		err = e.WithExecutionScope(ctx, *request.ExpectedScope, run)
	} else {
		var current context.Context
		var finish func()
		current, _, finish, err = e.continuityAuthority(ctx, request)
		if err == nil {
			defer finish()
			err = run(current)
		}
	}
	return result, err
}
