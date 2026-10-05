package coordination

import (
	"container/heap"
	"context"
	"encoding/json"
	"math"
	"sort"
	"time"
)

type semanticHit struct {
	score  float64
	vector Resource
	memory Resource
}
type semanticHeap []semanticHit

func (h semanticHeap) Len() int { return len(h) }
func (h semanticHeap) Less(i, j int) bool {
	return h[i].score < h[j].score || h[i].score == h[j].score && h[i].memory.ID > h[j].memory.ID
}
func (h semanticHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *semanticHeap) Push(v any)   { *h = append(*h, v.(semanticHit)) }
func (h *semanticHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func ValidateQueryVector(query DataQuery) error {
	if len(query.Vector) > 8192 || len(query.VectorModel) > 128 || len(query.Vector) > 0 && query.VectorModel == "" {
		return ErrPendingLimit
	}
	var norm float64
	for _, v := range query.Vector {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return ErrWrongOwner
		}
		norm += float64(v) * float64(v)
	}
	if len(query.Vector) > 0 && norm == 0 {
		return ErrWrongOwner
	}
	return nil
}

func (s *OwnershipStore) SemanticSearch(ctx context.Context, role string, query DataQuery, limit int) ([]Resource, error) {
	if err := ValidateQueryVector(query); err != nil {
		return nil, err
	}
	if len(query.Vector) == 0 {
		return nil, nil
	}
	if role == "" {
		return nil, ErrWrongOwner
	}
	if limit <= 0 || limit > 32 {
		limit = 32
	}
	rows, err := s.db.QueryContext(ctx, `SELECT v.resource_id,v.source_id,v.revision,v.body,m.revision,m.body FROM kernel_device_owned_resources v JOIN kernel_device_owned_resources m ON m.owner_id=v.owner_id AND m.kind='memory' AND m.resource_id=v.source_id AND m.role_id=v.role_id WHERE v.owner_id=? AND v.role_id=? AND v.kind='vector' AND v.deleted=0 AND m.deleted=0`, s.ownerID, role)
	if err != nil {
		return nil, err
	}
	hits := &semanticHeap{}
	heap.Init(hits)
	now := time.Now()
	for rows.Next() {
		hit := semanticHit{vector: Resource{OwnerID: s.ownerID, Kind: "vector", RoleID: role}, memory: Resource{OwnerID: s.ownerID, Kind: "memory", RoleID: role}}
		if err := rows.Scan(&hit.vector.ID, &hit.vector.SourceID, &hit.vector.Revision, &hit.vector.Body, &hit.memory.Revision, &hit.memory.Body); err != nil {
			_ = rows.Close()
			return nil, err
		}
		hit.memory.ID = hit.vector.SourceID
		if !ResourceUsable(hit.memory.Body, now) || !ResourceUsable(hit.vector.Body, now) {
			continue
		}
		var document struct {
			Content struct {
				Values []float32 `json:"values"`
				Model  string    `json:"modelFingerprint"`
			} `json:"content"`
		}
		if json.Unmarshal(hit.vector.Body, &document) != nil || document.Content.Model != query.VectorModel || len(document.Content.Values) != len(query.Vector) {
			continue
		}
		var dot, left, right float64
		valid := true
		for i, v := range document.Content.Values {
			value := float64(v)
			if math.IsNaN(value) || math.IsInf(value, 0) {
				valid = false
				break
			}
			q := float64(query.Vector[i])
			dot += value * q
			left += value * value
			right += q * q
		}
		if !valid || left == 0 || right == 0 {
			continue
		}
		hit.score = dot / math.Sqrt(left*right)
		if hit.score <= 0 {
			continue
		}
		heap.Push(hits, hit)
		if hits.Len() > limit {
			heap.Pop(hits)
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	sort.Slice(*hits, func(i, j int) bool {
		return (*hits)[i].score > (*hits)[j].score || (*hits)[i].score == (*hits)[j].score && (*hits)[i].memory.ID < (*hits)[j].memory.ID
	})
	result := make([]Resource, 0, hits.Len()*2)
	for _, hit := range *hits {
		result = append(result, hit.memory, hit.vector)
	}
	return result, ctx.Err()
}
