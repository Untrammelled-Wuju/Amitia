package composition

import (
	"fmt"
)

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
