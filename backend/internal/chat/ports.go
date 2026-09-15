// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"

	"github.com/u-ai/backend/internal/memory"
	visioncfg "github.com/u-ai/backend/internal/vision"
)

type MemoryPort interface {
	GenerateCandidates(conversationID string) ([]memory.MemoryCandidate, error)
	AcceptCandidate(id string) (*memory.Memory, error)
	HybridSearch(req *memory.VectorSearchRequest) ([]memory.HybridSearchResult, error)
	RecordUse(id string) (*memory.Memory, error)
	memory.PipelineLayer
}

type ProfilePort interface {
	ExtractFromConversation(spaceID, convID string, messages []map[string]string, characterID ...string) error
	ToSystemPrompt(spaceID string, characterID ...string) string
	memory.PipelineLayer
}

type EpisodicPort interface {
	ExtractFromConversation(spaceID, convID string, messages []map[string]string, characterID ...string) error
	ToSystemPrompt(spaceID string, characterID ...string) string
	memory.PipelineLayer
}

type WorldBookPort interface {
	ToSystemPromptForSpace(spaceID, characterID, userMessage, assistantReply string) string
}

type VisionPort interface {
	GetActive() (*visioncfg.VisionConfig, error)
}

type memoryPipelineLayerAdapter struct {
	GenerateCandidatesFunc func(conversationID string) ([]memory.MemoryCandidate, error)
	AcceptCandidateFunc    func(id string) (*memory.Memory, error)
	HybridSearchFunc       func(req *memory.VectorSearchRequest) ([]memory.HybridSearchResult, error)
	RecordUseFunc          func(id string) (*memory.Memory, error)
	NameFunc               func() string
	ProcessFunc            func(ctx context.Context, convID string, messages []map[string]string, newReply string) error
}

func (a memoryPipelineLayerAdapter) GenerateCandidates(conversationID string) ([]memory.MemoryCandidate, error) {
	return a.GenerateCandidatesFunc(conversationID)
}

func (a memoryPipelineLayerAdapter) AcceptCandidate(id string) (*memory.Memory, error) {
	return a.AcceptCandidateFunc(id)
}

func (a memoryPipelineLayerAdapter) HybridSearch(req *memory.VectorSearchRequest) ([]memory.HybridSearchResult, error) {
	return a.HybridSearchFunc(req)
}

func (a memoryPipelineLayerAdapter) RecordUse(id string) (*memory.Memory, error) {
	return a.RecordUseFunc(id)
}

func (a memoryPipelineLayerAdapter) Name() string {
	return a.NameFunc()
}

func (a memoryPipelineLayerAdapter) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return a.ProcessFunc(ctx, convID, messages, newReply)
}

type profilePipelineLayerAdapter struct {
	ExtractFromConversationFunc func(spaceID, convID string, messages []map[string]string, characterID ...string) error
	ToSystemPromptFunc          func(spaceID string, characterID ...string) string
	NameFunc                    func() string
	ProcessFunc                 func(ctx context.Context, convID string, messages []map[string]string, newReply string) error
}

func (a profilePipelineLayerAdapter) ExtractFromConversation(spaceID, convID string, messages []map[string]string, characterID ...string) error {
	return a.ExtractFromConversationFunc(spaceID, convID, messages, characterID...)
}

func (a profilePipelineLayerAdapter) ToSystemPrompt(spaceID string, characterID ...string) string {
	return a.ToSystemPromptFunc(spaceID, characterID...)
}

func (a profilePipelineLayerAdapter) Name() string {
	return a.NameFunc()
}

func (a profilePipelineLayerAdapter) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return a.ProcessFunc(ctx, convID, messages, newReply)
}

type episodicPipelineLayerAdapter struct {
	ExtractFromConversationFunc func(spaceID, convID string, messages []map[string]string, characterID ...string) error
	ToSystemPromptFunc          func(spaceID string, characterID ...string) string
	NameFunc                    func() string
	ProcessFunc                 func(ctx context.Context, convID string, messages []map[string]string, newReply string) error
}

func (a episodicPipelineLayerAdapter) ExtractFromConversation(spaceID, convID string, messages []map[string]string, characterID ...string) error {
	return a.ExtractFromConversationFunc(spaceID, convID, messages, characterID...)
}

func (a episodicPipelineLayerAdapter) ToSystemPrompt(spaceID string, characterID ...string) string {
	return a.ToSystemPromptFunc(spaceID, characterID...)
}

func (a episodicPipelineLayerAdapter) Name() string {
	return a.NameFunc()
}

func (a episodicPipelineLayerAdapter) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return a.ProcessFunc(ctx, convID, messages, newReply)
}

type worldBookPortAdapter struct {
	ToSystemPromptForSpaceFunc func(spaceID, characterID, userMessage, assistantReply string) string
}

func (a worldBookPortAdapter) ToSystemPromptForSpace(spaceID, characterID, userMessage, assistantReply string) string {
	return a.ToSystemPromptForSpaceFunc(spaceID, characterID, userMessage, assistantReply)
}

type visionPortAdapter struct {
	GetActiveFunc func() (*visioncfg.VisionConfig, error)
}

func (a visionPortAdapter) GetActive() (*visioncfg.VisionConfig, error) {
	return a.GetActiveFunc()
}
