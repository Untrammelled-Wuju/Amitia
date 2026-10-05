package coordination

import "context"

type ProjectionLayerStatus struct {
	Kind          string `json:"kind"`
	Total         int64  `json:"total"`
	Current       int64  `json:"current"`
	Pending       int64  `json:"pending"`
	Retrying      int64  `json:"retrying"`
	NextAttemptAt int64  `json:"nextAttemptAt,omitempty"`
}

type ProjectionStatus struct {
	OwnerID string                  `json:"ownerId"`
	RoleID  string                  `json:"roleId"`
	Layers  []ProjectionLayerStatus `json:"layers"`
}

type ProjectionPort interface {
	ProjectionState(context.Context, ExecutionScope, bool) (ProjectionStatus, error)
}

func (s *OwnershipStore) ProjectionStatus(ctx context.Context, role string) (ProjectionStatus, error) {
	result := ProjectionStatus{OwnerID: s.ownerID, RoleID: role, Layers: []ProjectionLayerStatus{}}
	if role == "" || len(role) > 512 {
		return result, ErrRoleRequired
	}
	for _, kind := range []string{"vector", "graph"} {
		layer := ProjectionLayerStatus{Kind: kind}
		err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN p.projected_revision=v.revision AND p.projected_source_revision=COALESCE(m.revision,0) THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN COALESCE(p.attempts,0)>0 THEN 1 ELSE 0 END),0),COALESCE(MIN(CASE WHEN COALESCE(p.attempts,0)>0 THEN p.next_attempt_at END),0) FROM kernel_device_owned_resources v LEFT JOIN kernel_device_owned_resources m ON m.owner_id=v.owner_id AND m.kind='memory' AND m.resource_id=v.source_id AND m.role_id=v.role_id LEFT JOIN kernel_device_owned_projections p ON p.owner_id=v.owner_id AND p.kind=v.kind AND p.resource_id=v.resource_id WHERE v.owner_id=? AND v.role_id=? AND v.kind=?`, s.ownerID, role, kind).Scan(&layer.Total, &layer.Current, &layer.Retrying, &layer.NextAttemptAt)
		if err != nil {
			return result, err
		}
		layer.Pending = layer.Total - layer.Current
		result.Layers = append(result.Layers, layer)
	}
	return result, nil
}
