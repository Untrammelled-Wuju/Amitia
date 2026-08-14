// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"context"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/embedding"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/temporal"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"gorm.io/gorm"
)

type Service interface {
	List(q MemoryListQuery) (*MemoryListResponse, error)
	Create(req *CreateMemoryRequest) (*Memory, error)
	Update(id string, req *UpdateMemoryRequest) (*Memory, error)
	Delete(id string) error
	DeleteAll(characterID string) error
	Search(req *SearchMemoryRequest) ([]Memory, error)
	VectorSearch(req *VectorSearchRequest) ([]VectorSearchResult, error)
	HybridSearch(req *VectorSearchRequest) ([]HybridSearchResult, error)
	RecordUse(id string) (*Memory, error)
	GetVectorStatus() map[string]interface{}
	GetTimeline(page, pageSize int, userID, source, memoryType, timelineType string) ([]map[string]interface{}, int64, error)
	GenerateCandidates(conversationID string) ([]MemoryCandidate, error)
	SubmitCandidate(req *SubmitCandidateRequest) (*MemoryCandidate, error)
	ListCandidates() []MemoryCandidate
	AcceptCandidate(id string) (*Memory, error)
	RejectCandidate(id string) error
	BatchAcceptCandidates(ids []string) ([]Memory, error)
	UpdateCandidate(id string, req *UpdateCandidateRequest) (*MemoryCandidate, error)
	DeleteCandidate(id string) error
	CheckConflict(req *CheckConflictRequest) (*CheckConflictResponse, error)
	ResolveConflict(req *ResolveConflictRequest) (*ResolveConflictResponse, error)
	AutoResolveConflict(key, value, characterID string, newConfidence int) (*ResolveConflictResponse, error)
	GetRankedMemories(characterID, userID, query string, limit int) ([]RankedMemory, error)
	ExtractCandidates() ([]MemoryCandidate, error)
	RebuildIndex() (map[string]interface{}, error)
	RebuildEmbeddings() (map[string]interface{}, error)
	SyncEmbedding(memID, key, value, characterID, memoryType string) bool
	SyncGraphMemory(id string) bool
	BatchVerify(ids []string, status string) error
	BatchSetImportance(ids []string, importance int) error
	RetrieveStats() (map[string]interface{}, error)

	SummarizeMemories(req *MemorySummaryRequest) (*MemorySummaryResult, error)
	RunConsolidation(req *ConsolidationRequest) (*ConsolidationResult, error)
	ListConsolidationCandidates(kind string) ([]MemoryCandidate, error)
	AcceptConsolidationCandidate(id string) (*Memory, error)
	RejectConsolidationCandidate(id string) error

	WriteAuthoritySnapshot() WriteAuthoritySnapshot
}

type WriteAuthoritySnapshot struct {
	CanonicalMutationEnabled   bool `json:"canonicalMutationEnabled"`
	LegacyRawWriterEnabled     bool `json:"legacyRawWriterEnabled"`
	LegacyHistoryWriterEnabled bool `json:"legacyHistoryWriterEnabled"`
	RawImportWriterEnabled     bool `json:"rawImportWriterEnabled"`
}

type RankedMemory struct {
	Memory            Memory  `json:"memory"`
	FinalScore        float64 `json:"finalScore"`
	VectorScore       float64 `json:"vectorScore"`
	KeywordScore      float64 `json:"keywordScore"`
	ImportanceNorm    float64 `json:"importanceNorm"`
	TemporalBoost     float64 `json:"temporalBoost"`
	ValidityPenalty   float64 `json:"validityPenalty"`
	TemporalReference string  `json:"temporalReference"`
}

type UpdateCandidateRequest struct {
	Key        *string `json:"key"`
	Value      *string `json:"value"`
	MemoryType *string `json:"memoryType"`
	Importance *int    `json:"importance"`
}

type CheckConflictRequest struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	MemoryType  string `json:"memoryType"`
	Importance  int    `json:"importance"`
	CharacterID string `json:"characterId"`
}

type CheckConflictResponse struct {
	HasConflict bool           `json:"hasConflict"`
	Conflicts   []ConflictItem `json:"conflicts"`
}

type ConflictItem struct {
	Memory Memory `json:"memory"`
	Reason string `json:"reason"`
}

type ResolveConflictRequest struct {
	Action      string `json:"action"`
	NewKey      string `json:"newKey"`
	NewValue    string `json:"newValue"`
	NewType     string `json:"newType"`
	Importance  int    `json:"importance"`
	CharacterID string `json:"characterId"`
	ConflictID  string `json:"conflictId"`
}

type ResolveConflictResponse struct {
	Resolved bool   `json:"resolved"`
	MemoryID string `json:"memoryId"`
}
type MemoryCandidate struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Value          string `json:"value"`
	MemoryType     string `json:"memoryType"`
	Importance     int    `json:"importance"`
	SourceText     string `json:"sourceText"`
	ConversationID string `json:"conversationId"`
	CharacterID    string `json:"characterId"`
	CreatedAt      string `json:"createdAt"`
}
type SubmitCandidateRequest struct {
	Key            string `json:"key"`
	Value          string `json:"value"`
	MemoryType     string `json:"memoryType"`
	Importance     int    `json:"importance"`
	SourceText     string `json:"sourceText"`
	ConversationID string `json:"conversationId"`
	CharacterID    string `json:"characterId"`
	CandidateKind  string `json:"candidateKind"`
	DerivationKey  string `json:"derivationKey"`
	Reason         string `json:"reason"`
}

type VectorSearchResult struct {
	Memory         Memory  `json:"memory"`
	Score          float32 `json:"score"`
	CollectionName string  `json:"collectionName"`
	MemoryLayer    string  `json:"memoryLayer"`
	MatchType      string  `json:"matchType,omitempty"`
}

type HybridSearchResult struct {
	Memory            Memory  `json:"memory"`
	Score             float64 `json:"score"`
	VectorScore       float64 `json:"vectorScore"`
	KeywordScore      float64 `json:"keywordScore"`
	MatchType         string  `json:"matchType"`
	CollectionName    string  `json:"collectionName"`
	MemoryLayer       string  `json:"memoryLayer"`
	TemporalBoost     float64 `json:"temporalBoost"`
	ValidityPenalty   float64 `json:"validityPenalty"`
	TemporalReference string  `json:"temporalReference"`
}

type service struct {
	repo                     Repository
	db                       *gorm.DB
	embeddingSvc             *embedding.Service
	graphSvc                 graph.Service
	dataLifecycleCoordinator *mindruntime.DataLifecycleCoordinator
	embedMu                  sync.Mutex
	embedLocks               map[string]*sync.Mutex
	temporalReranker         TemporalMemoryReranker
	temporalRepo             *temporal.SQLiteRepository
}

type TemporalMemoryReranker interface {
	RerankMemoryScores(ctx context.Context, query string, candidates []temporal.MemoryScoreCandidate) (map[string]temporal.MemoryScoreResult, error)
}

func SetTemporalMemoryReranker(memoryService Service, reranker TemporalMemoryReranker) {
	if target, ok := memoryService.(*service); ok {
		target.temporalReranker = reranker
	}
}

func SetTemporalRepo(memoryService Service, temporalRepo *temporal.SQLiteRepository) {
	if target, ok := memoryService.(*service); ok {
		target.temporalRepo = temporalRepo
	}
}

func NewService(repo Repository, ctx *app.AppContext, graphSvc ...graph.Service) Service {
	var gs graph.Service
	if len(graphSvc) > 0 {
		gs = graphSvc[0]
	}
	return &service{
		repo:         repo,
		db:           ctx.DB,
		embeddingSvc: embedding.NewService(ctx.DB),
		graphSvc:     gs,
		embedLocks:   map[string]*sync.Mutex{},
	}
}

func (s *service) SetDataLifecycleCoordinator(c *mindruntime.DataLifecycleCoordinator) {
	s.dataLifecycleCoordinator = c
}

func (s *service) WriteAuthoritySnapshot() WriteAuthoritySnapshot {
	return WriteAuthoritySnapshot{
		CanonicalMutationEnabled:   true,
		LegacyRawWriterEnabled:     false,
		LegacyHistoryWriterEnabled: false,
		RawImportWriterEnabled:     false,
	}
}

func extractJSONArray(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "["); idx >= 0 {
		s = s[idx:]
	}
	if idx := strings.LastIndex(s, "]"); idx >= 0 {
		s = s[:idx+1]
	}
	return s
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func jaccardSimilarity(a, b string) float64 {
	wordsA := make(map[string]bool)
	wordsB := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(a)) {
		wordsA[w] = true
	}
	for _, w := range strings.Fields(strings.ToLower(b)) {
		wordsB[w] = true
	}
	intersection := 0
	for w := range wordsA {
		if wordsB[w] {
			intersection++
		}
	}
	union := len(wordsA) + len(wordsB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func deleteVectorsFromCollections(ids []string, collectionNames ...string) {
	if qdrantDB.Client == nil || len(ids) == 0 {
		return
	}
	if len(collectionNames) == 0 {
		collectionNames = qdrantDB.CollectionNames()
	}
	for _, collectionName := range collectionNames {
		if collectionName == "" {
			continue
		}
		if err := qdrantDB.DeleteVectors(ids, collectionName); err != nil {
			log.Warn("删除向量失败:", collectionName, err)
		}
	}
}

func collectionNameForMemoryType(memoryType string) string {
	return qdrantDB.ResolveConfiguredCollection(collectionKeyForMemoryType(memoryType))
}

func collectionKeyForMemoryType(memoryType string) string {
	switch strings.ToLower(memoryType) {
	case "working_memory", "working", "summary", "current_summary":
		return "working_memory"
	case "profile", "user_profile", "personal_info", "hobby", "preference", "habit", "relationship", "nickname":
		return "user_profiles"
	case "episodic", "episode", "event", "moment", "scene":
		return "episodic_memories"
	default:
		return "memory_embeddings"
	}
}

func collectionKeyFromCollectionName(collectionName string) string {
	keys := []string{"memory_embeddings", "working_memory", "user_profiles", "episodic_memories"}
	for _, key := range keys {
		if qdrantDB.ResolveConfiguredCollection(key) == collectionName {
			return key
		}
	}
	return collectionName
}

func memoryLayerLabel(collectionKey string) string {
	switch collectionKey {
	case "working_memory":
		return "当前摘要"
	case "user_profiles":
		return "用户画像"
	case "episodic_memories":
		return "情景回忆"
	default:
		return "事实记忆"
	}
}

func keywordMatchScore(query, key, value string) float64 {
	keyLower := strings.ToLower(key)
	valLower := strings.ToLower(value)

	if strings.Contains(keyLower, query) || strings.Contains(valLower, query) {
		return 1.0
	}

	queryWords := strings.Fields(query)
	matchCount := 0
	for _, w := range queryWords {
		if strings.Contains(keyLower, w) || strings.Contains(valLower, w) {
			matchCount++
		}
	}
	if len(queryWords) > 0 {
		return float64(matchCount) / float64(len(queryWords))
	}
	return 0
}
