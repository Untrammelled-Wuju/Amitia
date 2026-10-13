package main

import (
	"context"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"sort"
)

func (p *meshLocalDataPort) HistoricalRoles(ctx context.Context, scope coordination.ExecutionScope) ([]coordination.HistoricalRole, error) {
	if !scope.Coordinated || scope.TargetDeviceID != p.ownerID || scope.ResourceOwnerID != scope.CoreID || scope.RoleOwnerID != scope.CoreID {
		return nil, coordination.ErrWrongOwner
	}
	roles := map[string]string{}
	rows, err := p.services.KernelContainer.DeviceRegistry.Database().QueryContext(ctx, `SELECT DISTINCT role_id FROM kernel_device_owned_resources WHERE owner_id=? AND deleted=0 AND kind IN ('memory','working','profile','episodic','fact','vector','graph','summary','project','conversation','message','continuity','tool-result') LIMIT 4097`, p.ownerID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if id != "" {
			roles[id] = ""
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if len(roles) > 4096 {
		return nil, coordination.ErrPendingLimit
	}
	result := make([]coordination.HistoricalRole, 0, len(roles))
	for id, name := range roles {
		if len(id) > 512 || len(name) > 1024 {
			return nil, coordination.ErrPendingLimit
		}
		if name == "" {
			name = id
		}
		result = append(result, coordination.HistoricalRole{ID: id, Name: name})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}
