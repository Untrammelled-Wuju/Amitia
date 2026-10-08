package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func leaseKey(run, attempt, lease, session string, generation, sequence int64) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%d", run, attempt, lease, session, generation, sequence)
}

func (c *MeshClient) awaitTaskLease(ctx context.Context, dispatch protocol.TaskDispatchPayload, sequence int64, confirmed ...func(time.Duration) error) error {
	if c.State() != StateReady || dispatch.RuntimeSessionID != c.sessionIdentity() || dispatch.ConnectionGeneration != c.sessionGeneration() {
		return fmt.Errorf("任务通道已失效")
	}
	key := leaseKey(dispatch.TaskRunID, dispatch.AttemptID, dispatch.LeaseID, dispatch.RuntimeSessionID.String(), dispatch.ConnectionGeneration, sequence)
	ch := make(chan protocol.TaskLeaseAckPayload, 1)
	c.taskLeaseMu.Lock()
	if _, exists := c.taskLeases[key]; exists {
		c.taskLeaseMu.Unlock()
		return fmt.Errorf("任务租约请求正在处理")
	}
	c.taskLeases[key] = ch
	c.taskLeaseMu.Unlock()
	defer func() { c.taskLeaseMu.Lock(); delete(c.taskLeases, key); c.taskLeaseMu.Unlock() }()
	requestedAt := time.Now()
	if sequence == 0 {
		c.sendTaskClaim(dispatch.TaskRunID, dispatch.AttemptID, dispatch.LeaseID, c.conf.Identity.RuntimeID.String(), 300000, dispatch)
	} else {
		c.sendTaskHeartbeat(dispatch.TaskRunID, dispatch.AttemptID, dispatch.LeaseID, sequence, dispatch)
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.stopCh:
		return fmt.Errorf("任务通道已停止")
	case <-timer.C:
		return fmt.Errorf("Core 尚未确认任务租约，任务已暂停")
	case ack := <-ch:
		if !ack.Accepted || ack.LeaseDurationMs < 60000 || ack.LeaseDurationMs > 300000 || c.State() != StateReady || dispatch.RuntimeSessionID != c.sessionIdentity() || dispatch.ConnectionGeneration != c.sessionGeneration() {
			return fmt.Errorf("Core 拒绝任务租约或执行会话已失效")
		}
		if len(confirmed) > 1 {
			return fmt.Errorf("任务租约确认端口无效")
		}
		if len(confirmed) == 1 {
			return confirmed[0](time.Duration(ack.LeaseDurationMs)*time.Millisecond - time.Since(requestedAt))
		}
		return nil
	}
}

func (c *MeshClient) handleTaskLeaseAck(env *protocol.Envelope) {
	var ack protocol.TaskLeaseAckPayload
	if json.Unmarshal(env.Payload, &ack) != nil || ack.RuntimeSessionID != env.RuntimeSessionID || ack.ConnectionGeneration != env.ConnectionGeneration {
		return
	}
	key := leaseKey(ack.TaskRunID, ack.AttemptID, ack.LeaseID, ack.RuntimeSessionID.String(), ack.ConnectionGeneration, ack.Sequence)
	c.taskLeaseMu.Lock()
	ch := c.taskLeases[key]
	c.taskLeaseMu.Unlock()
	if ch != nil {
		select {
		case ch <- ack:
		default:
		}
	}
}
