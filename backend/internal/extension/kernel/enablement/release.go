//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"fmt"
	"io"
)

var ErrPluginBusy = errors.New("plugin lifecycle: active requests in flight")

func (i *Installer) beginCall(pluginID string) {
	i.mu.Lock()
	i.activeCalls[pluginID]++
	i.mu.Unlock()
}

func (i *Installer) endCall(pluginID string) {
	i.mu.Lock()
	i.activeCalls[pluginID]--
	if i.activeCalls[pluginID] <= 0 {
		delete(i.activeCalls, pluginID)
	}
	i.mu.Unlock()
}

func (i *Installer) Deactivate(ctx context.Context, pluginID string, epoch uint64) error {
	if i == nil {
		return fmt.Errorf("%w: installer is nil", ErrInstallerState)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.Lock()
	machine, err := i.machineFor(pluginID)
	if err != nil {
		i.mu.Unlock()
		return err
	}
	if active := i.activeCalls[pluginID]; active > 0 {
		i.mu.Unlock()
		return fmt.Errorf("%w: %s has %d active requests", ErrPluginBusy, pluginID, active)
	}
	state := machine.State()
	if state != LifecycleActivated && state != LifecycleWarm {
		i.mu.Unlock()
		return fmt.Errorf("%w: %s is %s", ErrInstallerState, pluginID, state)
	}
	if state == LifecycleWarm {
		if _, err := machine.Apply(ctx, EventDeactivate, epoch); err != nil {
			i.mu.Unlock()
			return err
		}
		epoch++
	}
	if _, err := machine.Apply(ctx, EventDeactivate, epoch); err != nil {
		i.mu.Unlock()
		return err
	}
	handle := i.handles[pluginID]
	delete(i.handles, pluginID)
	if i.results != nil {
		i.results = map[string]ExecuteResult{}
	}
	i.mu.Unlock()
	if closer, ok := handle.(io.Closer); ok {
		_ = closer.Close()
	}
	return nil
}

func (i *Installer) Suspend(ctx context.Context, pluginID string, epoch uint64) error {
	if i == nil {
		return fmt.Errorf("%w: installer is nil", ErrInstallerState)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.Lock()
	machine, err := i.machineFor(pluginID)
	if err != nil {
		i.mu.Unlock()
		return err
	}
	if i.activeCalls[pluginID] > 0 {
		i.mu.Unlock()
		return fmt.Errorf("%w: %s has active requests", ErrPluginBusy, pluginID)
	}
	if _, err := machine.Apply(ctx, EventSuspend, epoch); err != nil {
		i.mu.Unlock()
		return err
	}
	handle := i.handles[pluginID]
	delete(i.handles, pluginID)
	i.mu.Unlock()
	if closer, ok := handle.(io.Closer); ok {
		_ = closer.Close()
	}
	return nil
}

func (i *Installer) Resume(ctx context.Context, pluginID string, epoch uint64) error {
	if i == nil {
		return fmt.Errorf("%w: installer is nil", ErrInstallerState)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.Lock()
	machine, err := i.machineFor(pluginID)
	if err != nil {
		i.mu.Unlock()
		return err
	}
	if machine.State() != LifecycleSuspended {
		i.mu.Unlock()
		return fmt.Errorf("%w: %s is %s, resume needs suspended", ErrInstallerState, pluginID, machine.State())
	}
	metadata, ok := i.index.entry(pluginID)
	if !ok {
		i.mu.Unlock()
		return fmt.Errorf("%w: %s metadata missing", ErrInstallerState, pluginID)
	}
	if i.spawner == nil {
		i.mu.Unlock()
		return fmt.Errorf("%w: no runtime spawner", ErrInstallerState)
	}
	if _, err := machine.Apply(ctx, EventResume, epoch); err != nil {
		i.mu.Unlock()
		return err
	}
	i.mu.Unlock()
	handle, spawnErr := i.spawner.Spawn(ctx, metadata)
	if spawnErr != nil {
		i.mu.Lock()
		if machine.State() == LifecycleActivated {
			_, _ = machine.Apply(context.Background(), EventSuspend, machine.Epoch()+1)
		}
		i.mu.Unlock()
		return fmt.Errorf("%w: %w", ErrExecuteFailed, spawnErr)
	}
	i.mu.Lock()
	i.handles[pluginID] = handle
	i.spawns++
	i.mu.Unlock()
	return nil
}
