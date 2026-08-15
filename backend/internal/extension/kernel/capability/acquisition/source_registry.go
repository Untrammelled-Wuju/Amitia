package acquisition

import (
	"context"
	"sync"
)

// SourceSourceRegistry 管理所有 Source 的注册与发现
type SourceRegistry struct {
	mu      sync.RWMutex
	sources map[string]Source
}

// NewSourceRegistry 创建 SourceRegistry 实例
func NewSourceRegistry() *SourceRegistry {
	return &SourceRegistry{
		sources: make(map[string]Source),
	}
}

// Register 注册一个 Source
func (r *SourceRegistry) Register(s Source) {
	if s == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[s.ID()] = s
}

// Unregister 注销指定 ID 的 Source
func (r *SourceRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sources, id)
}

// List 返回所有已注册的 Source 列表
func (r *SourceRegistry) List() []Source {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Source, 0, len(r.sources))
	for _, s := range r.sources {
		result = append(result, s)
	}
	return result
}

// SearchResultSet 聚合所有 Source 的搜索结果
type SearchResultSet struct {
	Candidates []CapabilityCandidate `json:"candidates"`
	Errors     []SourceError         `json:"errors,omitempty"`
}

// SourceError 记录单个 Source 搜索过程中的错误
type SourceError struct {
	SourceID string `json:"sourceId"`
	Error    string `json:"error"`
}

// SearchAll 并行调用所有已注册的 Source 进行搜索，并聚合结果
func (r *SourceRegistry) SearchAll(ctx context.Context, request AcquisitionRequest) SearchResultSet {
	sources := r.List()
	if len(sources) == 0 {
		return SearchResultSet{}
	}

	var (
		mu        sync.Mutex
		candidates []CapabilityCandidate
		errors     []SourceError
		wg        sync.WaitGroup
	)

	for _, src := range sources {
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			result, err := s.Search(ctx, request)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, SourceError{
					SourceID: s.ID(),
					Error:    err.Error(),
				})
				return
			}
			candidates = append(candidates, result...)
		}(src)
	}

	wg.Wait()

	return SearchResultSet{
		Candidates: candidates,
		Errors:     errors,
	}
}
