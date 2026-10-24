package interaction

import "context"

func (c *MultiAgentCoordinator) SetDurableProgressObserver(observer func(context.Context, string, MultiAgentRecoveryRef) error) {
	c.mu.Lock()
	c.durableObserver = observer
	c.mu.Unlock()
}
