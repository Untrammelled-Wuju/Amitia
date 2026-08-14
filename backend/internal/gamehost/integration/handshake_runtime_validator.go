package integration

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type RuntimeReader interface {
	GetRuntime(runtimeID domain.RuntimeInstanceID) (*runtime.RuntimeInstanceRef, error)
}

type RuntimeTopologyReader interface {
	GetTopologySnapshot(runtimeID domain.RuntimeInstanceID) (runtime.RuntimeTopologySnapshot, error)
}

type HandshakeRuntimeValidator struct {
	runtimes RuntimeReader
	topology RuntimeTopologyReader
}

func NewHandshakeRuntimeValidator(runtimes RuntimeReader, topology RuntimeTopologyReader) *HandshakeRuntimeValidator {
	return &HandshakeRuntimeValidator{
		runtimes: runtimes,
		topology: topology,
	}
}

func (v *HandshakeRuntimeValidator) RuntimeExists(runtimeID string) (bool, error) {
	if v.runtimes == nil {
		return false, nil
	}
	_, err := v.runtimes.GetRuntime(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (v *HandshakeRuntimeValidator) ServiceBelongsToRuntime(runtimeID, serviceID, pluginID string) error {
	if v.topology == nil {
		return nil
	}
	snap, err := v.topology.GetTopologySnapshot(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return err
	}
	for _, svc := range snap.Services {
		if string(svc.ID) == serviceID {
			return nil
		}
	}
	return &runtime.TopologyError{
		Code:      runtime.ErrNotFound,
		Message:   "service not found in runtime topology: " + serviceID,
		RuntimeID: runtimeID,
		ServiceID: serviceID,
	}
}

var _ handshake.RuntimeValidator = (*HandshakeRuntimeValidator)(nil)
