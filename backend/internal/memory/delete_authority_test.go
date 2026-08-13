package memory

import (
	"errors"
	"testing"
)

type memoryAuthorityRepo struct {
	items    []Memory
	updated  map[string]map[string]interface{}
	deleted  []string
	unmarked []string
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

func (r *memoryAuthorityRepo) Delete(id string) error {
	r.deleted = append(r.deleted, id)
	return nil
}

func (r *memoryAuthorityRepo) DeleteAll(characterID string) error { return nil }

func (r *memoryAuthorityRepo) Search(keyword, characterID, userID string, limit int) ([]Memory, error) {
	out := make([]Memory, 0, len(r.items))
	for _, item := range r.items {
		if !memoryMatchesRetrievalScope(item, characterID, userID) {
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

func (r *memoryAuthorityRepo) UnmarkEmbedded(id string) error {
	r.unmarked = append(r.unmarked, id)
	return nil
}

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

func (r *memoryAuthorityRepo) FindByDerivationKey(derivationKey string) (*Memory, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) ListDerivationsByOutput(outputMemoryID string) ([]MemoryDerivation, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) ListDerivationsByInput(inputMemoryID string) ([]MemoryDerivation, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) CreateDerivation(d *MemoryDerivation) error { return nil }

func (r *memoryAuthorityRepo) FindDerivedMemoryBySourceIDs(sourceIDs []string, kind string) (*Memory, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) StreamExportable(characterID string, limit, offset int) ([]Memory, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) StreamExportableByIDs(ids []string, limit, offset int) ([]Memory, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) CountExportable(characterID string) (int64, error) { return 0, nil }

func (r *memoryAuthorityRepo) CountExportableByIDs(ids []string) (int64, error) { return 0, nil }

func (r *memoryAuthorityRepo) ListEventsByMemoryIDs(memoryIDs []string) ([]MemoryEventV1, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) ListTemporalByMemoryIDs(memoryIDs []string) ([]MemoryTemporalV1, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) ListDerivationsByMemoryIDs(memoryIDs []string) ([]MemoryDerivationV1, error) {
	return nil, nil
}

func (r *memoryAuthorityRepo) IsNewID(id string) (bool, error) { return true, nil }

func (r *memoryAuthorityRepo) BatchUpsert(records []MemoryEventV1) error { return nil }

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

	got, err := svc.GetRankedMemories("char-a", "", "favorite", 10)
	if err != nil {
		t.Fatalf("GetRankedMemories error = %v", err)
	}
	if len(got) != 1 || got[0].Memory.ID != "active" {
		t.Fatalf("GetRankedMemories returned %#v, want only active", got)
	}
}

func TestSearchRequiresMatchingUserIDForUserScope(t *testing.T) {
	repo := &memoryAuthorityRepo{items: []Memory{
		{ID: "character-memory", CharacterID: "char-a", Scope: "character", Key: "favorite", Value: "tea", VerifiedStatus: "user_verified"},
		{ID: "own-user-memory", CharacterID: "user-1", Scope: "user", Key: "favorite", Value: "cake", VerifiedStatus: "user_verified"},
		{ID: "other-user-memory", CharacterID: "user-2", Scope: "user", Key: "favorite", Value: "coffee", VerifiedStatus: "user_verified"},
	}}
	svc := &service{repo: repo}

	got, err := svc.Search(&SearchMemoryRequest{Keyword: "favorite", CharacterID: "char-a", UserID: "user-1", Limit: 10})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Search returned %d items: %#v, want 2", len(got), got)
	}
	ids := map[string]bool{}
	for _, item := range got {
		ids[item.ID] = true
	}
	if !ids["character-memory"] || !ids["own-user-memory"] || ids["other-user-memory"] {
		t.Fatalf("Search returned IDs %#v, want character and matching user memory only", ids)
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
	if len(repo.unmarked) != 1 || repo.unmarked[0] != "memory-1" {
		t.Fatalf("UnmarkEmbedded calls = %#v, want memory-1", repo.unmarked)
	}
}

func TestUpdateInvalidatingStatusClearsEmbeddingMarker(t *testing.T) {
	status := "deleted"
	repo := &memoryAuthorityRepo{items: []Memory{
		{ID: "memory-1", CharacterID: "char-a", Scope: "character", Key: "favorite", Value: "tea", Importance: 10, VerifiedStatus: "user_verified"},
	}}
	svc := &service{repo: repo}

	got, err := svc.Update("memory-1", &UpdateMemoryRequest{VerifiedStatus: &status})
	if err != nil {
		t.Fatalf("Update error = %v", err)
	}
	if got.VerifiedStatus != status {
		t.Fatalf("VerifiedStatus = %q, want %q", got.VerifiedStatus, status)
	}
	if len(repo.unmarked) != 1 || repo.unmarked[0] != "memory-1" {
		t.Fatalf("UnmarkEmbedded calls = %#v, want memory-1", repo.unmarked)
	}
}

func TestDeleteClearsEmbeddingMarker(t *testing.T) {
	repo := &memoryAuthorityRepo{items: []Memory{
		{ID: "memory-1", CharacterID: "char-a", Scope: "character", Key: "favorite", Value: "tea", Importance: 10, VerifiedStatus: "user_verified"},
	}}
	svc := &service{repo: repo}

	if err := svc.Delete("memory-1"); err != nil {
		t.Fatalf("Delete error = %v", err)
	}
	if len(repo.unmarked) != 1 || repo.unmarked[0] != "memory-1" {
		t.Fatalf("UnmarkEmbedded calls = %#v, want memory-1", repo.unmarked)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "memory-1" {
		t.Fatalf("Delete calls = %#v, want memory-1", repo.deleted)
	}
}
