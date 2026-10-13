package coordination

import "context"

func (s *OwnershipStore) MemoryDeletionComplete(ctx context.Context, role, id string) (bool, error) {
	row, err := s.Get(ctx, "memory", id)
	if err != nil {
		return false, err
	}
	if row == nil || row.RoleID != role || !row.Deleted {
		return false, ErrResourceVersion
	}
	var pending int64
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_owned_resources v LEFT JOIN kernel_device_owned_projections p ON p.owner_id=v.owner_id AND p.kind=v.kind AND p.resource_id=v.resource_id WHERE v.owner_id=? AND v.role_id=? AND v.source_id=? AND v.kind IN ('vector','graph') AND (COALESCE(p.projected_revision,0)<>v.revision OR COALESCE(p.projected_source_revision,0)<>?)`, s.ownerID, role, id, row.Revision).Scan(&pending)
	return pending == 0, err
}
