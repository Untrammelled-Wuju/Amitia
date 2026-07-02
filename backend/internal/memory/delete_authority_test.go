package memory

import (
	"errors"
	"testing"
)

type memoryAuthorityRepo struct {
	items   []Memory
	updated map[string]map[string]interface{}
}

func (r *memoryAuthorityRepo) List(q MemoryListQuery) ([]Memory, int64, error) {
	return append([]Memory(nil), r.items...), int64(len(r.items)), nil
}

func (r *memoryAuthorityRepo) FindByID(id string) (*Memory, error) {
	for i := range r.items {
		if r.items[i].ID == id {
			return &r.items[i], nil
		}
	}
	return nil, errors.New("not found")
}

func (r *memoryAuthorityRepo) Create(m *Memory) error {
	r.items = append(r.items, *m)
	return nil
}

func (r *memoryAuthorityRepo) Update(id string, updates map[string]interface{}) error {
	if r.updated == nil {
		r.updated = map[string]map[string]interface{}{}
	}
	r.updated[id] = updates
	for i := range r.items {
		if r.items[i].ID != id {
			continue
		}
		if status, ok := updates["verified_status"].(string); ok {
			r.items[i].VerifiedStatus = status
		}
		return nil
	}
	return errors.New("not found")
}

func (r *memoryAuthorityRepo) Delete(id string) error { return nil }

func (r *memoryAuthorityRepo) DeleteAll(characterID string) error { return nil }

func (r *memoryAuthorityRepo) Search(keyword, characterID string, limit int) ([]Memory, error) {
	out := make([]Memory, 0, len(r.items))
	for _, item := range r.items {
		if characterID != "" && item.CharacterID != characterID && item.Scope != "user" {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *memoryAuthorityRepo) SearchByKey(key, characterID string) ([]Memory, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) RecordUse(id string) error { return nil }

func (r *memoryAuthorityRepo) VectorStatus() (totalMem, embedded int64) { return 0, 0 }

func (r *memoryAuthorityRepo) MarkEmbedded(id string) error { return nil }

func (r *memoryAuthorityRepo) GetConversationMessages(conversationID string, limit int) ([]map[string]interface{}, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) GetRankedByImportance(characterID string, limit int) ([]Memory, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) ListCandidates() ([]MemoryCandidateModel, error) { return nil, nil }

func (r *memoryAuthorityRepo) CreateCandidate(c *MemoryCandidateModel) error { return nil }

func (r *memoryAuthorityRepo) UpdateCandidate(id string, updates map[string]interface{}) error {
	return nil
}

func (r *memoryAuthorityRepo) DeleteCandidate(id string) error { return nil }

func (r *memoryAuthorityRepo) GetCandidateByID(id string) (*MemoryCandidateModel, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) DeleteAllCandidates() error { return nil }

func TestMemorySearchFiltersDeletedInvalidatedAndTombstone(t *testing.T) {
	repo := &memoryAuthorityRepo{items: []Memory{
		{ID: "deleted", CharacterID: "char-a", Scope: "character", Key: "favorite", Value: "deleted", VerifiedStatus: "deleted"},
		{ID: "tombstone", CharacterID: "char-a", Scope: "character", Key: "favorite", Value: "tombstone", VerifiedStatus: "tombstone"},
		{ID: "invalidated", CharacterID: "char-a", Scope: "character", Key: "favorite", Value: "invalidated", VerifiedStatus: "invalidated"},
		{ID: "active", CharacterID: "char-a", Scope: "character", Key: "favorite", Value: "active", VerifiedStatus: "user_verified"},
	}}
	svc := &service{repo: repo}

	got, err := svc.Search(&SearchMemoryRequest{Keyword: "favorite", CharacterID: "char-a", Limit: 10})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "active" {
		t.Fatalf("Search returned %#v, want only active", got)
	}
}

func TestGetRankedMemoriesFiltersDeletedInvalidatedAndTombstone(t *testing.T) {
	repo := &memoryAuthorityRepo{items: []Memory{
		{ID: "active", CharacterID: "char-a", Scope: "character", Key: "favorite", Value: "tea", Importance: 10, VerifiedStatus: "user_verified"},
		{ID: "deleted", CharacterID: "char-a", Scope: "character", Key: "favorite", Value: "coffee", Importance: 10, VerifiedStatus: "deleted"},
		{ID: "tombstone", CharacterID: "char-a", Scope: "character", Key: "favorite", Value: "milk", Importance: 10, VerifiedStatus: "tombstone"},
		{ID: "invalidated", CharacterID: "char-a", Scope: "character", Key: "favorite", Value: "juice", Importance: 10, VerifiedStatus: "invalidated"},
	}}
	svc := &service{repo: repo}

	got, err := svc.GetRankedMemories("char-a", "favorite", 10)
	if err != nil {
		t.Fatalf("GetRankedMemories error = %v", err)
	}
	if len(got) != 1 || got[0].Memory.ID != "active" {
		t.Fatalf("GetRankedMemories returned %#v, want only active", got)
	}
}

func TestBatchVerifyInvalidatingStatusPersistsAndStaysFiltered(t *testing.T) {
	repo := &memoryAuthorityRepo{items: []Memory{
		{ID: "memory-1", CharacterID: "char-a", Scope: "character", Key: "favorite", Value: "tea", Importance: 10, VerifiedStatus: "user_verified"},
	}}
	svc := &service{repo: repo}

	if err := svc.BatchVerify([]string{"memory-1"}, "invalidated"); err != nil {
		t.Fatalf("BatchVerify error = %v", err)
	}
	got, err := svc.Search(&SearchMemoryRequest{Keyword: "favorite", CharacterID: "char-a", Limit: 10})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Search returned %#v after invalidation, want none", got)
	}
}
