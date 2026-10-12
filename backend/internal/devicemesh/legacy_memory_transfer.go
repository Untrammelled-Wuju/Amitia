package devicemesh

import (
	"context"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (rt *Runtime) TransferLegacyMemory(ctx context.Context, scope coordination.ExecutionScope, input coordination.LegacyMemoryTransferRequest) (coordination.Acknowledgement, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return coordination.Acknowledgement{}, err
	}
	var port coordination.DataPort
	if scope.Coordinated {
		port = rt.CoreDataPort
	} else if scope.TargetDeviceID == rt.LocalDeviceID {
		port = rt.LocalDeviceDataPort
	}
	if port != nil {
		transfer, ok := port.(coordination.LegacyMemoryTransferPort)
		if !ok {
			return coordination.Acknowledgement{}, coordination.ErrWrongOwner
		}
		return transfer.TransferLegacyMemory(ctx, scope, input)
	}
	var ack coordination.Acknowledgement
	err := rt.callOwnedData(ctx, scope, "legacy-memory-transfer", coordination.DataQuery{LegacyMemoryTransfer: &input}, &ack)
	return ack, err
}
