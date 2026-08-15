package acquisition

import (
	"context"
	"fmt"
)

// SourceSearchService 提供基于 SourceRegistry 的统一搜索入口，管理 Source 注册与去重
type SourceSearchService struct {
	registry *SourceRegistry
}

// NewSourceSearchService 创建 SourceSearchService 实例
func NewSourceSearchService(registry *SourceRegistry) *SourceSearchService {
	return &SourceSearchService{
		registry: registry,
	}
}

// Search implements the CandidateSearcher interface, returning the
// candidates as a flat error-returning slice for use by the Planner.
func (s *SourceSearchService) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	resultSet := s.SearchResultSet(ctx, request)
	return resultSet.Candidates, nil
}

// SearchResultSet 调用所有已注册的 Source 进行搜索，并返回去重后的结果
func (s *SourceSearchService) SearchResultSet(ctx context.Context, request AcquisitionRequest) SearchResultSet {
	if s.registry == nil {
		return SearchResultSet{}
	}

	resultSet := s.registry.SearchAll(ctx, request)
	resultSet.Candidates = Deduplicate(resultSet.Candidates)
	return resultSet
}

// Deduplicate 基于 package identity + version + kind 对候选进行去重
func Deduplicate(candidates []CapabilityCandidate) []CapabilityCandidate {
	if len(candidates) == 0 {
		return nil
	}

	seen := make(map[string]bool, len(candidates))
	result := make([]CapabilityCandidate, 0, len(candidates))

	for _, c := range candidates {
		key := dedupeKey(c)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, c)
	}

	return result
}

// dedupeKey 生成去重键：基于 ID + Version + Kind
func dedupeKey(c CapabilityCandidate) string {
	return fmt.Sprintf("%s|%s|%s", c.ID, c.Version, c.Kind)
}
