package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

const workflowDeviceSyncInterval = 5 * time.Second

// StartWorkflowDeviceSync starts one deduplicated durable outbox drain/reconcile
// loop for an online device. The loop exits after the device stays unreachable;
// the next Device Mesh ready event starts it again.
func (r *Runtime) StartWorkflowDeviceSync(userID, deviceID string) {
	if r == nil || r.WorkflowDeviceControl == nil || r.Kernel == nil || r.Kernel.Container() == nil || r.Kernel.Container().WorkflowDefRepo == nil {
		return
	}
	userID = strings.TrimSpace(userID)
	deviceID = strings.TrimSpace(deviceID)
	if userID == "" || deviceID == "" {
		return
	}
	key := userID + "\x00" + deviceID
	if _, loaded := r.workflowDeviceSyncLoops.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	go func() {
		defer r.workflowDeviceSyncLoops.Delete(key)
		failures := 0
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			err := r.SyncWorkflowDeviceOnce(ctx, userID, deviceID)
			cancel()
			if err != nil {
				failures++
				if failures >= 3 {
					return
				}
			} else {
				failures = 0
			}
			time.Sleep(workflowDeviceSyncInterval)
		}
	}()
}

// SyncWorkflowDeviceOnce drains durable local workflow mutations into Cloud's
// idempotent inbox and then reconciles the latest canonical revisions back to
// this device. Catalog reconciliation is not used as the normal sync path.
func (r *Runtime) SyncWorkflowDeviceOnce(ctx context.Context, userID, deviceID string) error {
	if r == nil || r.WorkflowDeviceControl == nil || r.Kernel == nil || r.Kernel.Container() == nil || r.Kernel.Container().WorkflowDefRepo == nil {
		return errors.New("workflow device sync unavailable")
	}
	userID = strings.TrimSpace(userID)
	deviceID = strings.TrimSpace(deviceID)
	if userID == "" || deviceID == "" {
		return errors.New("workflow device sync requires user and device")
	}
	repo := r.Kernel.Container().WorkflowDefRepo
	raw, err := r.WorkflowDeviceControl.Invoke(ctx, userID, deviceID, WorkflowMeshSyncOutbox, json.RawMessage(`{"limit":100}`))
	if err != nil {
		return err
	}
	var envelope struct {
		Items []sqlite.WorkflowSyncOutboxEvent `json:"items"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return fmt.Errorf("decode workflow sync outbox: %w", err)
		}
	}
	acked := make([]string, 0, len(envelope.Items))
	for _, item := range envelope.Items {
		if strings.TrimSpace(item.OwnerUserID) != userID {
			continue
		}
		result, applyErr := repo.ApplyWorkflowSyncEvent(ctx, deviceID, item)
		if applyErr != nil {
			return applyErr
		}
		// Conflict/DIVERGED are durable Cloud inbox outcomes. ACK means Cloud
		// received the mutation; it does not mean the conflicting definition was
		// accepted into the canonical branch.
		if result.Status != "" {
			acked = append(acked, item.EventID)
		}
	}
	if len(acked) > 0 {
		ackRaw, _ := json.Marshal(map[string]any{"eventIds": acked})
		if _, err := r.WorkflowDeviceControl.Invoke(ctx, userID, deviceID, WorkflowMeshSyncAck, ackRaw); err != nil {
			return err
		}
	}
	return r.reconcileWorkflowCanonicalToDevice(ctx, userID, deviceID)
}

func (r *Runtime) reconcileWorkflowCanonicalToDevice(ctx context.Context, userID, deviceID string) error {
	repo := r.Kernel.Container().WorkflowDefRepo
	stateRaw, err := r.WorkflowDeviceControl.Invoke(ctx, userID, deviceID, WorkflowMeshSyncState, json.RawMessage(`{}`))
	if err != nil {
		return err
	}
	var stateEnvelope struct {
		Items []sqlite.WorkflowSyncState `json:"items"`
	}
	if err := json.Unmarshal(stateRaw, &stateEnvelope); err != nil {
		return fmt.Errorf("decode workflow sync state: %w", err)
	}
	states := make(map[string]sqlite.WorkflowSyncState, len(stateEnvelope.Items))
	for _, state := range stateEnvelope.Items {
		states[state.WorkflowID] = state
	}
	canonical, err := repo.ListWorkflowSyncCanonical(ctx, userID)
	if err != nil {
		return err
	}
	for _, item := range canonical {
		state := states[item.WorkflowID]
		if state.Revision > item.Revision {
			// The device has local mutations not represented by this canonical
			// snapshot yet. Never overwrite them; the next outbox drain decides.
			continue
		}
		if state.Revision == item.Revision {
			if state.DefinitionHash == item.DefinitionHash || (state.Deleted && item.EventType == sqlite.WorkflowSyncDelete) {
				continue
			}
			// Same revision with a different hash is an explicit divergence.
			continue
		}
		if item.EventType == sqlite.WorkflowSyncDelete {
			payload, _ := json.Marshal(map[string]any{"workflowId": item.WorkflowID, "syncRevision": item.Revision})
			if _, err := r.WorkflowDeviceControl.Invoke(ctx, userID, deviceID, WorkflowMeshDelete, payload); err != nil {
				return err
			}
			continue
		}
		var def workflow.WorkflowDefinition
		if err := json.Unmarshal(item.Payload, &def); err != nil {
			return fmt.Errorf("decode canonical workflow %s: %w", item.WorkflowID, err)
		}
		if strings.TrimSpace(def.ID) == "" {
			def.ID = item.WorkflowID
		}
		expected := int64(0)
		getPayload, _ := json.Marshal(map[string]any{"workflowId": item.WorkflowID})
		if currentRaw, getErr := r.WorkflowDeviceControl.Invoke(ctx, userID, deviceID, WorkflowMeshGet, getPayload); getErr == nil {
			var current workflowAPIResponse
			if json.Unmarshal(currentRaw, &current) == nil {
				expected = current.Installation.Revision
			}
		}
		payload, _ := json.Marshal(map[string]any{
			"definition":       def,
			"expectedRevision": expected,
			"syncRevision":     item.Revision,
		})
		if _, err := r.WorkflowDeviceControl.Invoke(ctx, userID, deviceID, WorkflowMeshUpsert, payload); err != nil {
			if strings.Contains(err.Error(), "WORKFLOW_REVISION_CONFLICT") {
				continue
			}
			return err
		}
	}
	return nil
}
