package system

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/configwrite"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

const taskPolicyKey = "task_runtime_max_concurrent"

type TaskMaintenanceConfig struct {
	Source          string                 `json:"source"`
	RuntimeProfile  string                 `json:"runtimeProfile"`
	MaxTasks        int                    `json:"maxTasks"`
	Timeout         timeoutpolicy.Settings `json:"timeout"`
	Revision        int64                  `json:"revision"`
	TimeoutRevision int64                  `json:"timeoutRevision"`
	Applied         bool                   `json:"applied"`
}

type TaskMaintenanceUpdate struct {
	MaxTasks                int                    `json:"maxTasks"`
	Timeout                 timeoutpolicy.Settings `json:"timeout"`
	ExpectedRevision        int64                  `json:"expectedRevision"`
	ExpectedTimeoutRevision int64                  `json:"expectedTimeoutRevision"`
}

func (s *service) AttachTaskRuntime(runtime *task_runtime.TaskRuntimeService) error {
	if runtime == nil {
		return nil
	}
	s.taskRuntime = runtime
	value := runtime.MaxConcurrent()
	var migratedTimeout *timeoutpolicy.Settings
	err := configwrite.Transaction(s.db, func(tx *gorm.DB) error {
		store := NewSettingsStore(tx)
		raw, revision, err := store.Get(taskPolicyKey)
		if err != nil {
			return err
		}
		if revision == 0 {
			legacy, _, err := store.Get("long_running_max_tasks")
			if err != nil {
				return err
			}
			if legacy != "" {
				parsed, err := strconv.Atoi(legacy)
				if err == nil && task_runtime.ValidateMaxConcurrent(parsed) == nil {
					value = parsed
				}
			}
			if _, err := store.Upsert(taskPolicyKey, strconv.Itoa(value)); err != nil {
				return err
			}
		} else {
			value, err = strconv.Atoi(raw)
			if err != nil || task_runtime.ValidateMaxConcurrent(value) != nil {
				return errors.New("任务并发配置无效")
			}
		}
		_, timeoutRevision, err := store.Get(timeoutSettingsKey)
		if err != nil {
			return err
		}
		if timeoutRevision == 0 {
			legacy, _, err := store.Get("long_running_timeout")
			if err != nil {
				return err
			}
			minutes, parseErr := strconv.Atoi(legacy)
			settings := timeoutpolicy.Settings{Seconds: minutes * 60}
			if parseErr == nil && timeoutpolicy.Validate(settings) == nil {
				encoded, err := json.Marshal(settings)
				if err != nil {
					return err
				}
				if _, err := store.Upsert(timeoutSettingsKey, string(encoded)); err != nil {
					return err
				}
				migratedTimeout = &settings
			}
		}
		return tx.Where("key IN ?", []string{"long_running_max_tasks", "long_running_timeout"}).Delete(&AppSetting{}).Error
	})
	if err != nil {
		return err
	}
	runtime.ApplyMaxConcurrent(value)
	if migratedTimeout != nil {
		timeoutpolicy.Configure(*migratedTimeout)
	}
	return nil
}

func (s *service) taskMaintenanceConfig(ctx context.Context) (TaskMaintenanceConfig, error) {
	out := TaskMaintenanceConfig{Source: "task-runtime", RuntimeProfile: s.runtimeProfile.String()}
	if s.taskRuntime == nil {
		return out, errors.New("任务执行服务未就绪")
	}
	store := NewSettingsStore(s.db.WithContext(ctx))
	raw, revision, err := store.Get(taskPolicyKey)
	if err != nil {
		return out, err
	}
	value, err := strconv.Atoi(raw)
	if err != nil || task_runtime.ValidateMaxConcurrent(value) != nil {
		return out, errors.New("任务执行配置无效")
	}
	_, timeoutRevision, err := store.Get(timeoutSettingsKey)
	if err != nil {
		return out, err
	}
	out.MaxTasks, out.Revision, out.TimeoutRevision = s.taskRuntime.MaxConcurrent(), revision, timeoutRevision
	out.Timeout, _ = timeoutpolicy.Current()
	out.Applied = out.MaxTasks == value
	if !out.Applied {
		return out, errors.New("持久化配置与执行器状态不一致")
	}
	return out, nil
}

func (s *service) TaskMaintenanceConfig(ctx context.Context) (TaskMaintenanceConfig, error) {
	s.taskPolicyMu.Lock()
	defer s.taskPolicyMu.Unlock()
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return TaskMaintenanceConfig{}, err
	}
	return s.taskMaintenanceConfig(ctx)
}

func (s *service) UpdateTaskMaintenanceConfig(ctx context.Context, request TaskMaintenanceUpdate) (TaskMaintenanceConfig, error) {
	s.taskPolicyMu.Lock()
	defer s.taskPolicyMu.Unlock()
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return TaskMaintenanceConfig{}, err
	}
	if s.taskRuntime == nil {
		return TaskMaintenanceConfig{}, errors.New("任务执行服务未就绪")
	}
	if err := task_runtime.ValidateMaxConcurrent(request.MaxTasks); err != nil {
		return TaskMaintenanceConfig{}, err
	}
	if err := timeoutpolicy.Validate(request.Timeout); err != nil {
		return TaskMaintenanceConfig{}, err
	}
	raw, err := json.Marshal(request.Timeout)
	if err != nil {
		return TaskMaintenanceConfig{}, err
	}
	err = configwrite.TransactionAndApply(s.db.WithContext(ctx), func(tx *gorm.DB) error {
		store := NewSettingsStore(tx)
		if _, err := store.SetWithCAS(taskPolicyKey, strconv.Itoa(request.MaxTasks), request.ExpectedRevision); err != nil {
			return err
		}
		_, err := store.SetWithCAS(timeoutSettingsKey, string(raw), request.ExpectedTimeoutRevision)
		return err
	}, func() {
		s.taskRuntime.ApplyMaxConcurrent(request.MaxTasks)
		timeoutpolicy.Configure(request.Timeout)
	})
	if err != nil {
		return TaskMaintenanceConfig{}, err
	}
	return s.taskMaintenanceConfig(ctx)
}

func (s *service) TaskMaintenanceStatus(ctx context.Context) (task_runtime.MaintenanceStatus, error) {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return task_runtime.MaintenanceStatus{}, err
	}
	if s.taskRuntime == nil {
		return task_runtime.MaintenanceStatus{}, errors.New("任务执行服务未就绪")
	}
	return s.taskRuntime.MaintenanceStatus(ctx)
}

func (h *Handler) TaskMaintenanceConfig(c *gin.Context) {
	value, err := h.service.TaskMaintenanceConfig(c.Request.Context())
	if err != nil {
		util.ErrorResponse(c, http.StatusConflict, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, value)
}

func (h *Handler) UpdateTaskMaintenanceConfig(c *gin.Context) {
	var request TaskMaintenanceUpdate
	if err := c.ShouldBindJSON(&request); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "任务配置格式错误", nil)
		return
	}
	value, err := h.service.UpdateTaskMaintenanceConfig(c.Request.Context(), request)
	if err != nil {
		util.ErrorResponse(c, http.StatusConflict, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, value)
}

func (h *Handler) TaskMaintenanceStatus(c *gin.Context) {
	value, err := h.service.TaskMaintenanceStatus(c.Request.Context())
	if err != nil {
		util.ErrorResponse(c, http.StatusConflict, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, value)
}
