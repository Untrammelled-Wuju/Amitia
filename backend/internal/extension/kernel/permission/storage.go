package permission

import "context"

type PermissionStorage interface {
	Save(ctx context.Context, grant StoredGrant) error
	GetByGrantID(ctx context.Context, grantID string) (StoredGrant, bool, error)
	List(ctx context.Context, filter PermissionGrantFilter) ([]StoredGrant, error)
	ListBySubject(ctx context.Context, subject PermissionSubject) ([]StoredGrant, error)
	MarkRevoked(ctx context.Context, grantID string) error
	Delete(ctx context.Context, grantID string) error
}

type MemoryPermissionStorage struct {
	grants map[string]StoredGrant
}

func NewMemoryPermissionStorage() *MemoryPermissionStorage {
	return &MemoryPermissionStorage{
		grants: make(map[string]StoredGrant),
	}
}

func (s *MemoryPermissionStorage) Save(ctx context.Context, grant StoredGrant) error {
	s.grants[grant.GrantID] = grant
	return nil
}

func (s *MemoryPermissionStorage) GetByGrantID(ctx context.Context, grantID string) (StoredGrant, bool, error) {
	g, ok := s.grants[grantID]
	return g, ok, nil
}

func (s *MemoryPermissionStorage) List(ctx context.Context, filter PermissionGrantFilter) ([]StoredGrant, error) {
	result := make([]StoredGrant, 0)
	for _, g := range s.grants {
		if filter.ActiveOnly && g.RevokedAt != nil {
			continue
		}
		if filter.Subject != nil {
			if string(filter.Subject.Type) != g.SubjectType || filter.Subject.ID != g.SubjectID {
				continue
			}
		}
		if filter.PermissionID != "" && filter.PermissionID != g.PermissionID {
			continue
		}
		result = append(result, g)
	}
	return result, nil
}

func (s *MemoryPermissionStorage) ListBySubject(ctx context.Context, subject PermissionSubject) ([]StoredGrant, error) {
	result := make([]StoredGrant, 0)
	for _, g := range s.grants {
		if g.SubjectType == string(subject.Type) && g.SubjectID == subject.ID {
			result = append(result, g)
		}
	}
	return result, nil
}

func (s *MemoryPermissionStorage) MarkRevoked(ctx context.Context, grantID string) error {
	if g, ok := s.grants[grantID]; ok {
		g.RevokedAt = &g.IssuedAt
		s.grants[grantID] = g
	}
	return nil
}

func (s *MemoryPermissionStorage) Delete(ctx context.Context, grantID string) error {
	delete(s.grants, grantID)
	return nil
}
