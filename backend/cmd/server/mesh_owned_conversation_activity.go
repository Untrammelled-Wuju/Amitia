package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func (p *meshLocalDataPort) enrichOwnedConversationActivity(ctx context.Context, bodies []json.RawMessage) (result []json.RawMessage, resultErr error) {
	if len(bodies) == 0 {
		return bodies, nil
	}
	documents := make([]map[string]any, len(bodies))
	ids := make([]string, 0, len(bodies))
	for i, raw := range bodies {
		if err := json.Unmarshal(raw, &documents[i]); err != nil {
			return nil, err
		}
		id, _ := documents[i]["id"].(string)
		if id != "" {
			ids = append(ids, id)
		}
	}
	encodedIDs, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	rows, err := p.services.KernelContainer.DeviceRegistry.Database().QueryContext(ctx, `
		SELECT kind, body, updated_at
		FROM kernel_device_owned_resources
		WHERE owner_id = ? AND deleted = 0
		  AND json_extract(CAST(body AS TEXT), '$.conversationId') IN (SELECT value FROM json_each(?))
		  AND (
		    (kind = 'checkpoint' AND substr(resource_id, 1, 5) = 'turn/'
		      AND json_extract(CAST(body AS TEXT), '$.status') IN ('completed', 'failed', 'interrupted'))
		    OR (kind = 'message' AND json_extract(CAST(body AS TEXT), '$.role') = 'assistant'
		      AND json_extract(CAST(body AS TEXT), '$.source') IN ('reminder', 'proactive'))
		  )`, p.ownerID, string(encodedIDs))
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()
	type activity struct {
		sequences   []int64
		preview     string
		previewTime time.Time
		source      string
	}
	activities := make(map[string]*activity)
	for rows.Next() {
		var kind, updated string
		var raw []byte
		if err := rows.Scan(&kind, &raw, &updated); err != nil {
			return nil, err
		}
		var record struct {
			ConversationID string `json:"conversationId"`
			CreatedAt      string `json:"createdAt"`
			Content        string `json:"content"`
			Source         string `json:"source"`
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, err
		}
		timestamp := updated
		if kind == "message" {
			timestamp = record.CreatedAt
		}
		when, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			return nil, fmt.Errorf("解析会话活动时间 %q: %w", timestamp, err)
		}
		value := activities[record.ConversationID]
		if value == nil {
			value = &activity{}
			activities[record.ConversationID] = value
		}
		value.sequences = append(value.sequences, when.UnixMilli())
		if kind == "message" && record.Content != "" && when.After(value.previewTime) {
			value.preview, value.previewTime, value.source = record.Content, when, record.Source
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result = make([]json.RawMessage, len(documents))
	for i, document := range documents {
		id, _ := document["id"].(string)
		if value := activities[id]; value != nil {
			read, _ := document["lastReadTurnSequence"].(float64)
			var last int64
			unread := 0
			for _, sequence := range value.sequences {
				if sequence > last {
					last = sequence
				}
				if sequence > int64(read) {
					unread++
				}
			}
			document["lastTerminalTurnSequence"] = last
			document["unreadCompletedCount"] = unread
			document["hasUnread"] = unread > 0
			if source, _ := document["source"].(string); source == "" && value.source != "" {
				document["source"] = value.source
			}
			if preview, _ := document["lastMessagePreview"].(string); preview == "" && value.preview != "" {
				document["lastMessagePreview"] = value.preview
				document["lastMessage"] = value.preview
				document["lastMessageRole"] = "assistant"
				document["lastMessageType"] = "text"
			}
		}
		result[i], err = json.Marshal(document)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
