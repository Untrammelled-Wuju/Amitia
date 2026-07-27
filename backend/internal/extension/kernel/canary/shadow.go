package canary

import (
	"context"
	"fmt"
)

type ShadowManager struct{}

func NewShadowManager() *ShadowManager {
	return &ShadowManager{}
}

func (m *ShadowManager) CanShadow(ctx context.Context, sideEffectClass string) bool {
	return sideEffectClass == "none" || sideEffectClass == "read_only"
}

func (m *ShadowManager) ValidateShadowConstraint(ctx context.Context, action string, constraint ShadowConstraint) error {
	for _, forbidden := range constraint.ForbiddenActions {
		if action == forbidden {
			return fmt.Errorf("canary: action %s is forbidden in shadow mode", action)
		}
	}
	return nil
}
