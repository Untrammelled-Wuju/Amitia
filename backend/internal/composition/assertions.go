package composition

import (
	"fmt"
)

// AssertComplete 验证 Composition Root 所有必需权威已初始化。
// 在 Composition Root 构建完成后调用，确保没有权威为 nil。
func (r *Root) AssertComplete() error {
	if r == nil {
		return fmt.Errorf("composition root is nil")
	}
	if r.Tools == nil {
		return fmt.Errorf("composition root: ToolRegistry is nil")
	}
	if r.Providers == nil {
		return fmt.Errorf("composition root: ProviderRegistry is nil")
	}
	if r.Hosts == nil {
		return fmt.Errorf("composition root: HostRegistry is nil")
	}
	if r.Outbox == nil {
		return fmt.Errorf("composition root: Outbox is nil")
	}
	if !r.Profile.IsValid() {
		return fmt.Errorf("composition root: invalid profile %q", string(r.Profile))
	}
	return nil
}
