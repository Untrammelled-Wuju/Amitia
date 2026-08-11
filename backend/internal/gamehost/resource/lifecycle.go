package resource

type ResourceLifecycleHooks interface {
	OnRuntimeStop(runtimeID string)
	OnRuntimeRestart(runtimeID string)
	OnExtensionDisabled(extensionID string)
	OnExtensionUninstalled(extensionID string)
	OnHostShutdown()
}

type LifecycleCoordinator struct {
	adapter *ResourceAdmissionAdapter
	viewer  *ResourcePolicyViewer
}

func NewLifecycleCoordinator(adapter *ResourceAdmissionAdapter, viewer *ResourcePolicyViewer) *LifecycleCoordinator {
	return &LifecycleCoordinator{adapter: adapter, viewer: viewer}
}

func (c *LifecycleCoordinator) OnRuntimeStop(runtimeID string) {
	if c.adapter != nil {
		c.adapter.MarkStopping(runtimeID)
	}
}

func (c *LifecycleCoordinator) OnRuntimeRestart(runtimeID string) {
	if c.adapter != nil {
		c.adapter.ClearStopping(runtimeID)
	}
}

func (c *LifecycleCoordinator) OnExtensionDisabled(extensionID string) {
	if c.adapter != nil {
		for _, id := range c.adapter.ActiveSubjects() {
			c.adapter.MarkStopping(id)
		}
	}
}

func (c *LifecycleCoordinator) OnExtensionUninstalled(extensionID string) {
	c.OnExtensionDisabled(extensionID)
}

func (c *LifecycleCoordinator) OnHostShutdown() {
	if c.adapter != nil {
		c.adapter.Shutdown()
	}
}
