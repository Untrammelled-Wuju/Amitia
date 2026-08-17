package integration

import (
	"context"
	"fmt"
	"io"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/contracts"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type GameHostStdioProtocolHandler struct {
	controlPlane ipc.ControlPlane
	runtimes     RuntimeReader
	topology     RuntimeTopologyReader
	plugins      contracts.PluginRegistry
}

func NewGameHostStdioProtocolHandler(
	controlPlane ipc.ControlPlane,
	runtimes RuntimeReader,
	topology RuntimeTopologyReader,
	plugins contracts.PluginRegistry,
) *GameHostStdioProtocolHandler {
	return &GameHostStdioProtocolHandler{
		controlPlane: controlPlane,
		runtimes:     runtimes,
		topology:     topology,
		plugins:      plugins,
	}
}

func (h *GameHostStdioProtocolHandler) Attach(
	ctx context.Context,
	meta trusted_service.StdioProtocolSessionMeta,
	stdin io.WriteCloser,
	stdout io.ReadCloser,
) (io.Closer, error) {
	runtimeID, serviceID, err := runtime.ParseProcessInstanceID(meta.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("gamehost stdio: invalid instance id %q: %w", meta.InstanceID, err)
	}

	rt, err := h.runtimes.GetRuntime(runtimeID)
	if err != nil {
		return nil, fmt.Errorf("gamehost stdio: runtime %q not found: %w", runtimeID, err)
	}

	if err := h.validateServiceBelongsToRuntime(runtimeID, serviceID, rt.PluginID); err != nil {
		return nil, fmt.Errorf("gamehost stdio: service validation failed: %w", err)
	}

	if err := h.validatePluginExtension(rt.PluginID, meta.ExtensionID); err != nil {
		return nil, fmt.Errorf("gamehost stdio: plugin validation failed: %w", err)
	}
	generationReader, ok := h.runtimes.(interface {
		GetCurrentGeneration(domain.RuntimeInstanceID) (int64, error)
	})
	if !ok {
		return nil, fmt.Errorf("gamehost stdio: runtime generation reader is unavailable")
	}
	generation, err := generationReader.GetCurrentGeneration(runtimeID)
	if err != nil || generation <= 0 {
		return nil, fmt.Errorf("gamehost stdio: runtime generation is unavailable")
	}

	if meta.Generation > 0 && uint64(meta.Generation) != uint64(generation) {
		return nil, fmt.Errorf("gamehost stdio: process generation %d does not match current runtime generation %d", meta.Generation, generation)
	}

	peer := ipc.Peer{
		PluginID:   rt.PluginID,
		RuntimeID:  rt.ID,
		ServiceID:  serviceID,
		Generation: generation,
	}

	transport := ipc.NewStdioTransport(ipc.StdioTransportConfig{
		Reader: stdout,
		Writer: stdin,
		Closer: &multiCloser{readCloser: stdout, writeCloser: stdin},
	})

	conn, err := h.controlPlane.Attach(ctx, peer, transport)
	if err != nil {
		return nil, fmt.Errorf("gamehost stdio: control plane attach failed: %w", err)
	}

	return &attachedConnectionCloser{
		plane: h.controlPlane,
		id:    conn.ID,
	}, nil
}

func (h *GameHostStdioProtocolHandler) validateServiceBelongsToRuntime(
	runtimeID domain.RuntimeInstanceID,
	serviceID domain.ServiceID,
	pluginID domain.PluginID,
) error {
	snapshot, err := h.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return fmt.Errorf("topology not found for runtime %q: %w", runtimeID, err)
	}

	for _, svc := range snapshot.Services {
		if string(svc.ServiceID) == string(serviceID) && string(svc.RuntimeID) == string(runtimeID) {
			if svc.PluginID != pluginID {
				return fmt.Errorf("service %q does not belong to plugin %q", serviceID, pluginID)
			}
			return nil
		}
	}
	return fmt.Errorf("service %q not found in runtime %q topology", serviceID, runtimeID)
}

func (h *GameHostStdioProtocolHandler) validatePluginExtension(
	pluginID domain.PluginID,
	extensionID string,
) error {
	descriptor, err := h.plugins.Get(context.Background(), pluginID)
	if err != nil {
		return fmt.Errorf("plugin %q not found: %w", pluginID, err)
	}
	if descriptor.ExtensionID != extensionID {
		return fmt.Errorf("plugin %q extension mismatch: expected %q, got %q", pluginID, descriptor.ExtensionID, extensionID)
	}
	return nil
}

type multiCloser struct {
	readCloser  io.ReadCloser
	writeCloser io.WriteCloser
}

func (m *multiCloser) Close() error {
	var err error
	if m.readCloser != nil {
		err = m.readCloser.Close()
	}
	if m.writeCloser != nil {
		if werr := m.writeCloser.Close(); werr != nil && err == nil {
			err = werr
		}
	}
	return err
}

type attachedConnectionCloser struct {
	plane ipc.ControlPlane
	id    ipc.ConnectionID
}

func (c *attachedConnectionCloser) Close() error {
	return c.plane.Detach(context.Background(), c.id)
}
