package extension

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

const workflowAndroidHealthStaleAfter = 90 * time.Second

// WorkflowAndroidRuntimeHealthStatus is the local Android capability-health
// projection used by Workflow preflight and Cloud Device Mesh inspection. It
// intentionally reports readiness; it does not grant permissions or attempt to
// unlock/wake a device.
type WorkflowAndroidRuntimeHealthStatus struct {
	DeviceID                     string    `json:"deviceId,omitempty"`
	RuntimeReady                 bool      `json:"runtimeReady"`
	NativeBridgeReady            bool      `json:"nativeBridgeReady"`
	AccessibilityConfigured      bool      `json:"accessibilityConfigured"`
	AccessibilityEnabled         bool      `json:"accessibilityEnabled"`
	AccessibilityReady           bool      `json:"accessibilityReady"`
	AccessibilityGeneration      int64     `json:"accessibilityGeneration,omitempty"`
	ScreenCaptureReady           bool      `json:"screenCaptureReady"`
	MicrophoneReady              bool      `json:"microphoneReady"`
	UIAgentReady                 bool      `json:"uiAgentReady"`
	BackgroundRestricted         bool      `json:"backgroundRestricted"`
	DeviceIdleMode               bool      `json:"deviceIdleMode"`
	PowerSaveMode                bool      `json:"powerSaveMode"`
	ScreenOn                     bool      `json:"screenOn"`
	Interactive                  bool      `json:"interactive"`
	KeyguardLocked               bool      `json:"keyguardLocked"`
	InteractionState             string    `json:"interactionState"`
	LastRuntimeFailureAtMS       int64     `json:"lastRuntimeFailureAtMs,omitempty"`
	LastRuntimeFailureGeneration int64     `json:"lastRuntimeFailureGeneration,omitempty"`
	LastRuntimeFailureCode       string    `json:"lastRuntimeFailureCode,omitempty"`
	RecoveryAttempt              int       `json:"recoveryAttempt,omitempty"`
	NextRecoveryAtMS             int64     `json:"nextRecoveryAtMs,omitempty"`
	RecoveryExhausted            bool      `json:"recoveryExhausted"`
	UpdatedAt                    time.Time `json:"updatedAt"`
	Stale                        bool      `json:"stale"`
}

func (r *Runtime) SetWorkflowAndroidRuntimeHealth(status WorkflowAndroidRuntimeHealthStatus) {
	if r == nil {
		return
	}
	status.DeviceID = strings.TrimSpace(status.DeviceID)
	status.LastRuntimeFailureCode = strings.TrimSpace(status.LastRuntimeFailureCode)
	status.InteractionState = strings.ToUpper(strings.TrimSpace(status.InteractionState))
	if status.InteractionState == "" {
		status.InteractionState = "AVAILABLE"
	}
	status.UpdatedAt = time.Now().UTC()
	status.Stale = false
	r.workflowAndroidHealthMu.Lock()
	previous := r.workflowAndroidHealth
	r.workflowAndroidHealth = status
	r.workflowAndroidHealthMu.Unlock()

	metrics := workflow.DefaultWorkflowReliabilityMetrics
	if metrics != nil {
		if status.LastRuntimeFailureAtMS > 0 && status.LastRuntimeFailureAtMS > previous.LastRuntimeFailureAtMS {
			metrics.Inc(workflow.MetricRuntimeCrashTotal)
		}
		if status.RecoveryAttempt > previous.RecoveryAttempt {
			for attempt := previous.RecoveryAttempt; attempt < status.RecoveryAttempt; attempt++ {
				metrics.Inc(workflow.MetricRuntimeRecoveryTotal)
			}
		}
		if status.RecoveryExhausted && !previous.RecoveryExhausted {
			metrics.Inc(workflow.MetricRuntimeRecoveryExhaustedTotal)
		}
		if previous.AccessibilityReady && !status.AccessibilityReady {
			metrics.Inc(workflow.MetricAndroidAccessibilityDisconnect)
		}
	}
}

func (r *Runtime) WorkflowAndroidRuntimeHealth() WorkflowAndroidRuntimeHealthStatus {
	if r == nil {
		return WorkflowAndroidRuntimeHealthStatus{Stale: true}
	}
	r.workflowAndroidHealthMu.RLock()
	status := r.workflowAndroidHealth
	r.workflowAndroidHealthMu.RUnlock()
	if status.UpdatedAt.IsZero() || time.Since(status.UpdatedAt) > workflowAndroidHealthStaleAfter {
		status.Stale = true
	}
	return status
}

func (api *WorkflowAPI) updateAndroidRuntimeHealth(c *gin.Context) {
	if api == nil || api.runtime == nil || api.effectiveLocation() != workflow.WorkflowLocationLocal {
		c.JSON(http.StatusNotFound, gin.H{"error": "local Android workflow health endpoint unavailable"})
		return
	}
	if !requireWorkflowRuntimeReporter(c) {
		return
	}
	var request WorkflowAndroidRuntimeHealthStatus
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Android workflow health status"})
		return
	}
	headerDeviceID := strings.TrimSpace(c.GetHeader("X-Amitia-Device-ID"))
	if headerDeviceID != "" {
		if request.DeviceID != "" && strings.TrimSpace(request.DeviceID) != headerDeviceID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Android workflow health device id mismatch"})
			return
		}
		request.DeviceID = headerDeviceID
	}
	if len(request.DeviceID) > 200 || len(request.InteractionState) > 64 || len(strings.TrimSpace(request.LastRuntimeFailureCode)) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Android workflow health status exceeds limits"})
		return
	}
	if request.LastRuntimeFailureAtMS < 0 || request.LastRuntimeFailureGeneration < 0 || request.RecoveryAttempt < 0 || request.RecoveryAttempt > 1000 || request.NextRecoveryAtMS < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Android runtime recovery state"})
		return
	}
	switch strings.ToUpper(strings.TrimSpace(request.InteractionState)) {
	case "", "AVAILABLE", "WAITING_UNLOCK", "WAITING_SCREEN", "BLOCKED":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Android interaction state"})
		return
	}
	api.runtime.SetWorkflowAndroidRuntimeHealth(request)
	c.Status(http.StatusNoContent)
}

func (api *WorkflowAPI) getAndroidRuntimeHealth(c *gin.Context) {
	if api == nil || api.runtime == nil || api.effectiveLocation() != workflow.WorkflowLocationLocal {
		c.JSON(http.StatusNotFound, gin.H{"error": "local Android workflow health endpoint unavailable"})
		return
	}
	if !requireWorkflowRuntimeReporter(c) {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, api.runtime.WorkflowAndroidRuntimeHealth())
}

func (api *WorkflowAPI) meshAndroidRuntimeHealth(_ context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	if api == nil || api.runtime == nil {
		return nil, errors.New("Android workflow health unavailable")
	}
	return meshResult(invoke, api.runtime.WorkflowAndroidRuntimeHealth())
}
