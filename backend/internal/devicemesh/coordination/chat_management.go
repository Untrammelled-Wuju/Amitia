package coordination

import (
	"context"
	"encoding/json"
)

type ChatManagementRequest struct {
	PrivacyDeletion     *PrivacyDeletionRequest `json:"privacyDeletion,omitempty"`
	PrivacyMessage      *PrivacyMessage         `json:"privacyMessage,omitempty"`
	Title               string                  `json:"title,omitempty"`
	ProjectID           string                  `json:"projectId,omitempty"`
	Channel             string                  `json:"channel,omitempty"`
	PeerID              string                  `json:"peerId,omitempty"`
	MessageID           string                  `json:"messageId,omitempty"`
	FeedbackID          int                     `json:"feedbackId,omitempty"`
	FeedbackType        string                  `json:"feedbackType,omitempty"`
	Reason              string                  `json:"reason,omitempty"`
	StartSequence       int64                   `json:"startSequence,omitempty"`
	EndSequence         int64                   `json:"endSequence,omitempty"`
	Limit               int                     `json:"limit,omitempty"`
	RequestID           string                  `json:"requestId,omitempty"`
	Operation           string                  `json:"operation"`
	ImportBatchID       string                  `json:"importBatchId,omitempty"`
	ConversationID      string                  `json:"conversationId,omitempty"`
	ConversationIDs     []string                `json:"conversationIds,omitempty"`
	ConversationOwnerID string                  `json:"conversationOwnerId,omitempty"`
	HistoricalRoleID    string                  `json:"historicalRoleId,omitempty"`
	Keyword             string                  `json:"keyword,omitempty"`
	Page                int                     `json:"page,omitempty"`
	PageSize            int                     `json:"pageSize,omitempty"`
	Format              string                  `json:"format,omitempty"`
	BeforeDate          string                  `json:"beforeDate,omitempty"`
	OlderThanDays       int                     `json:"olderThanDays,omitempty"`
	Channels            []string                `json:"channels,omitempty"`
	Sources             []string                `json:"sources,omitempty"`
	IncludeMemories     bool                    `json:"includeMemories"`
	PreviewID           string                  `json:"previewId,omitempty"`
	ConfirmText         string                  `json:"confirmText,omitempty"`
}

type ChatManagementPort interface {
	ManageChat(context.Context, ExecutionScope, ChatManagementRequest) (map[string]any, error)
}

func ChatManagementPayload(scope ExecutionScope, request ChatManagementRequest) ([]byte, error) {
	return json.Marshal(map[string]any{"operation": "chat-management", "scope": scope, "chatManagement": request})
}
