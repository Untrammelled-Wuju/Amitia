package extension

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

func (api *WorkflowAPI) lockWorkflowMutation(userID, workflowID string) func() {
	if api == nil || api.runtime == nil {
		return func() {}
	}
	key := strings.TrimSpace(userID) + "\x00" + strings.TrimSpace(workflowID)
	value, _ := api.runtime.workflowMutationLocks.LoadOrStore(key, &sync.Mutex{})
	mu, ok := value.(*sync.Mutex)
	if !ok || mu == nil {
		return func() {}
	}
	mu.Lock()
	return mu.Unlock
}

type workflowAPIResponse struct {
	workflow.WorkflowDefinition
	Installation workflow.WorkflowInstallation `json:"installation"`
}

func (api *WorkflowAPI) effectiveLocation() workflow.WorkflowLocation {
	if api.location.Valid() {
		return api.location
	}
	if api != nil && api.runtime != nil && api.runtime.Kernel != nil && api.runtime.Kernel.Container() != nil {
		if api.runtime.Kernel.Container().RuntimeProfile == runtimeprofile.ProfileCloudCore {
			return workflow.WorkflowLocationCloud
		}
	}
	return workflow.WorkflowLocationLocal
}

func (api *WorkflowAPI) installationFor(ctx context.Context, def workflow.WorkflowDefinition, userID string) (*workflow.WorkflowInstallation, error) {
	if api == nil || api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil || api.runtime.Kernel.Container().WorkflowInstallationRepo == nil {
		return nil, errors.New("workflow installation repository unavailable")
	}
	repo := api.runtime.Kernel.Container().WorkflowInstallationRepo
	location := api.effectiveLocation()
	inst, err := repo.Get(ctx, userID, def.ID, location, "")
	if err == nil {
		return inst, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	// Compatibility for databases created before WorkflowInstallation existed.
	return repo.EnsureLegacy(ctx, def, userID, location)
}

func applyInstallation(def workflow.WorkflowDefinition, inst workflow.WorkflowInstallation) workflow.WorkflowDefinition {
	def.Enabled = inst.Enabled
	def.Triggers = inst.Triggers
	def.CallableByAgent = inst.CallableByAgent
	def.AgentTool = inst.AgentTool
	return def
}

func (api *WorkflowAPI) emitWorkflowInstallationEvent(ctx context.Context, typeID string, inst *workflow.WorkflowInstallation) {
	if api == nil || api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil || inst == nil {
		return
	}
	service := api.runtime.Kernel.Container().EventService
	if service == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"installationId": inst.InstallationID,
		"workflowId":     inst.WorkflowID,
		"location":       inst.Location,
		"hostDeviceId":   inst.HostDeviceID,
		"revision":       inst.Revision,
		"enabled":        inst.Enabled,
		"updatedAt":      inst.UpdatedAt,
	})
	if err != nil {
		return
	}
	revision := inst.Revision
	_, _ = service.Publish(ctx, event.EventTypeID(typeID), 1, payload, event.PublishOptions{
		ProducerID:       "host",
		ProducerType:     event.EventProducerTypeSystem,
		Domain:           event.EventDomainSync,
		AggregateType:    "workflow_installation",
		AggregateID:      inst.InstallationID,
		AggregateVersion: &revision,
		PartitionKey:     inst.OwnerUserID,
		OrderingKey:      inst.OwnerUserID,
	})
}

func workflowResponse(def workflow.WorkflowDefinition, inst *workflow.WorkflowInstallation) workflowAPIResponse {
	if inst == nil {
		return workflowAPIResponse{WorkflowDefinition: def}
	}
	return workflowAPIResponse{WorkflowDefinition: applyInstallation(def, *inst), Installation: *inst}
}

func (api *WorkflowAPI) expectedRevision(c *gin.Context, current int64) (int64, error) {
	value := strings.TrimSpace(c.Query("expectedRevision"))
	if value == "" {
		value = strings.TrimSpace(c.GetHeader("If-Match"))
		value = strings.Trim(value, `"`)
	}
	if value == "" {
		// Legacy clients did not know about installation revisions. They may keep
		// working, while updated clients always submit expectedRevision.
		return current, nil
	}
	revision, err := strconv.ParseInt(value, 10, 64)
	if err != nil || revision <= 0 {
		return 0, errors.New("expectedRevision must be a positive integer")
	}
	return revision, nil
}

func (api *WorkflowAPI) updateInstallationCAS(ctx context.Context, def workflow.WorkflowDefinition, userID string, current *workflow.WorkflowInstallation, expectedRevision int64) (*workflow.WorkflowInstallation, error) {
	if current == nil {
		return nil, errors.New("workflow installation is required")
	}
	next := *current
	next.Enabled = def.Enabled
	next.Triggers = def.Triggers
	next.CallableByAgent = def.CallableByAgent
	next.AgentTool = def.AgentTool
	updated, err := api.runtime.Kernel.Container().WorkflowInstallationRepo.UpdateCAS(ctx, next, expectedRevision)
	if err != nil {
		return nil, err
	}
	api.emitWorkflowInstallationEvent(ctx, "workflow.installation.updated", updated)
	return updated, nil
}

func writeWorkflowRevisionConflict(c *gin.Context) {
	c.JSON(http.StatusConflict, gin.H{
		"error": "WORKFLOW_REVISION_CONFLICT",
		"code":  "WORKFLOW_REVISION_CONFLICT",
	})
}

func isWorkflowRevisionConflict(err error) bool {
	return errors.Is(err, sqlite.ErrWorkflowRevisionConflict)
}

func requireWorkflowRevision(expected, current int64) error {
	if expected <= 0 || current <= 0 || expected != current {
		return sqlite.ErrWorkflowRevisionConflict
	}
	return nil
}

func (api *WorkflowAPI) validateExecutionTargets(def workflow.WorkflowDefinition) error {
	location := api.effectiveLocation()
	for _, node := range def.Nodes {
		if err := node.ExecutionTarget.Validate(); err != nil {
			return errors.New("workflow node " + node.ID + ": " + err.Error())
		}
		placement := node.ExecutionTarget.Placement
		if placement == "" {
			continue
		}
		if node.Type == "nested_workflow" && placement == workflow.WorkflowExecutionAuto {
			return errors.New("nested workflow node " + node.ID + " requires an explicit local/cloud/device target")
		}
		switch location {
		case workflow.WorkflowLocationLocal:
			if placement != workflow.WorkflowExecutionLocal {
				return errors.New("local workflow node " + node.ID + " must execute on the current device")
			}
		case workflow.WorkflowLocationCloud:
			if placement == workflow.WorkflowExecutionLocal {
				return errors.New("cloud workflow node " + node.ID + " cannot use local placement; use cloud, device, or auto")
			}
		}
	}
	return nil
}
