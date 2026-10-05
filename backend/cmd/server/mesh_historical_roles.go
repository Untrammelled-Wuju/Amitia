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
	rows, err := p.services.KernelContainer.DeviceRegistry.Database().QueryContext(ctx, `SELECT DISTINCT role_id FROM kernel_device_owned_resources WHERE owner_id=? AND deleted=0 AND kind IN ('memory','working','profile','episodic','fact','vector','graph','summary') LIMIT 4097`, p.ownerID)
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
	var legacy []struct {
		ID   string
		Name string
	}
	err = p.services.DB.WithContext(ctx).Raw(`SELECT DISTINCT c.id,c.name FROM characters c WHERE (c.space_id=? OR c.space_id='' OR c.space_id='default') AND (EXISTS(SELECT 1 FROM memories m WHERE m.character_id=c.id AND (m.space_id=? OR m.space_id='' OR m.space_id='default')) OR EXISTS(SELECT 1 FROM user_profiles u WHERE u.character_id=c.id AND (u.space_id=? OR u.space_id='' OR u.space_id='default')) OR EXISTS(SELECT 1 FROM episodic_memories e WHERE e.character_id=c.id AND (e.space_id=? OR e.space_id='' OR e.space_id='default'))) LIMIT 4097`, p.legacySpaceID, p.legacySpaceID, p.legacySpaceID, p.legacySpaceID).Scan(&legacy).Error
	if err != nil {
		return nil, err
	}
	for _, row := range legacy {
		roles[row.ID] = row.Name
	}
	var orphanIDs []string
	err = p.services.DB.WithContext(ctx).Raw(`SELECT character_id FROM memories WHERE character_id<>'' AND (space_id=? OR space_id='' OR space_id='default') UNION SELECT character_id FROM user_profiles WHERE character_id<>'' AND (space_id=? OR space_id='' OR space_id='default') UNION SELECT character_id FROM episodic_memories WHERE character_id<>'' AND (space_id=? OR space_id='' OR space_id='default') LIMIT 4097`, p.legacySpaceID, p.legacySpaceID, p.legacySpaceID).Scan(&orphanIDs).Error
	if err != nil {
		return nil, err
	}
	for _, id := range orphanIDs {
		if _, exists := roles[id]; !exists {
			roles[id] = id
		}
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
