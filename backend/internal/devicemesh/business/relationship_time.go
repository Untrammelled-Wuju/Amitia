package business

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) RelationshipTime(ctx context.Context, request Request, input coordination.RelationshipTimeRequest) (json.RawMessage, error) {
	request.RequestID = uuid.NewString()
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return nil, err
	}
	defer finish()
	if len(input.Settings) > 0 && (request.ExpectedScope == nil || summaryAuthority(*request.ExpectedScope) != summaryAuthority(scope)) {
		return nil, coordination.ErrScopeExpired
	}
	port, ok := e.data.(coordination.RelationshipTimePort)
	if !ok {
		return nil, errors.New("Owner 关系时间端口未接通")
	}
	result, err := port.RelationshipTime(ctx, scope, input)
	if err != nil {
		return nil, err
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return nil, err
	}
	return result, coordination.ValidateRoleRevision(ctx, e.data, scope)
}
