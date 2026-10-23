package system

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/safety"
)

func (s *service) SafetyEvents(ctx context.Context, scope safety.PolicyScope, page, pageSize int, status string) ([]safety.Incident, int64, error) {
	return s.safetyRepository.ListIncidents(ctx, scope, page, pageSize, status)
}

func (s *service) DeleteSafetyEvents(ctx context.Context, scope safety.PolicyScope) (int64, error) {
	return s.safetyRepository.DeleteIncidents(ctx, scope)
}

func (s *service) HandleSafetyEvent(ctx context.Context, scope safety.PolicyScope, id string, expectedRevision int64) (safety.Incident, error) {
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil {
		return safety.Incident{}, errors.New("安全事件处理缺少可信身份")
	}
	return s.safetyRepository.HandleIncident(ctx, scope, id, expectedRevision, actor.DeviceID.String())
}
