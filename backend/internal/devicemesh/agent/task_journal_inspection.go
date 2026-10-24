package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func (c *MeshClient) inspectTaskJournal(invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	if c.conf.ExecutionJournal == nil || invoke.RuntimeType != "task_recovery" || invoke.AuthorityCallID != "" || len(invoke.OwnedExecutionScope) != 0 || invoke.IdempotencyKey != "" || invoke.ProviderID != "" || invoke.WorkflowRunID != "" || invoke.WorkflowNodeID != "" || invoke.FencingToken != 0 || invoke.LogicalAttempt != 0 || len(invoke.Input) == 0 || len(invoke.Input) > 4096 {
		return nil, fmt.Errorf("设备任务账本只读查询请求无效")
	}
	var request executionjournal.TaskInspectionRequest
	if json.Unmarshal(invoke.Input, &request) != nil {
		return nil, fmt.Errorf("设备任务账本查询参数无效")
	}
	inspection, err := c.conf.ExecutionJournal.InspectTask(c.clientCtx, invoke.SpaceID.String(), invoke.DeviceID.String(), request)
	if err != nil {
		return nil, err
	}
	if c.State() != StateReady || invoke.SpaceID != c.conf.SpaceID || invoke.RuntimeSessionID != c.sessionIdentity() || invoke.ConnectionGeneration != c.sessionGeneration() {
		return nil, fmt.Errorf("设备账本查询期间会话已失效")
	}
	output, err := json.Marshal(inspection)
	if err != nil {
		return nil, err
	}
	return &protocol.RuntimeResultPayload{InvocationID: invoke.InvocationID, DeviceID: invoke.DeviceID, RuntimeID: invoke.RuntimeID, RuntimeSessionID: invoke.RuntimeSessionID, ConnectionGeneration: invoke.ConnectionGeneration, Status: "completed", Result: output, CompletedAt: time.Now().UTC()}, nil
}
