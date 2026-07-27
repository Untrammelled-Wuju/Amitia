package permission

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
)

type UIPermissionValidator struct{}

func NewUIPermissionValidator() *UIPermissionValidator {
	return &UIPermissionValidator{}
}

func (v *UIPermissionValidator) ValidateAccess(ctx context.Context, def *extension_page_host.ExtensionPageDefinition, scopeSnapshot string) ([]string, error) {
	_ = ctx
	if def == nil || def.PageSpec == nil {
		return []string{}, nil
	}
	if scopeSnapshot == "" {
		missing := make([]string, 0, len(def.PageSpec.Permissions))
		missing = append(missing, def.PageSpec.Permissions...)
		return missing, nil
	}
	return []string{}, nil
}

func (v *UIPermissionValidator) ValidateParams(ctx context.Context, def *extension_page_host.ExtensionPageDefinition, params map[string]string) error {
	_ = ctx
	if def == nil || def.PageSpec == nil {
		return nil
	}
	for _, p := range def.PageSpec.Parameters {
		if p.Required {
			if params == nil {
				return fmt.Errorf("missing required parameter: %s", p.Name)
			}
			if _, ok := params[p.Name]; !ok {
				return fmt.Errorf("missing required parameter: %s", p.Name)
			}
		}
	}
	return nil
}
