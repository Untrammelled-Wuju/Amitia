package system

import (
	"errors"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/pkg/util"
)

func WithModelUsage(store *mindruntime.ModelUsageStore) SystemRouterOption {
	return func(h *Handler) {
		if s, ok := h.service.(*service); ok {
			s.usageStore = store
		}
	}
}

func usageAccess(c *gin.Context) {
	actor := security.GetActor(c)
	if actor == nil || actor.SpaceID == "" || actor.DeviceID == "" {
		c.AbortWithStatus(401)
		return
	}
	if c.Query("scope") == "core" || c.Query("scope") == "" && actor.IsLocalTrusted {
		security.SharedCoreAdminOnly()(c)
		return
	}
	if c.Query("scope") != "" && c.Query("scope") != "device" {
		c.AbortWithStatusJSON(400, gin.H{"msg": "统计范围无效"})
		return
	}
	c.Next()
}

func usageFilter(c *gin.Context) (mindruntime.UsageFilter, error) {
	actor := security.GetActor(c)
	if actor == nil || actor.SpaceID == "" || actor.DeviceID == "" {
		return mindruntime.UsageFilter{}, errors.New("统计身份缺失")
	}
	filter := mindruntime.UsageFilter{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String()}
	if c.Query("scope") == "core" || c.Query("scope") == "" && actor.IsLocalTrusted {
		if !actor.HasPermission(auth.PermSystemAdmin) {
			return filter, errors.New("无权读取Core统计")
		}
		filter.DeviceID = ""
	}
	return filter, nil
}

func (h *Handler) usage(c *gin.Context, group string) {
	filter, err := usageFilter(c)
	if err != nil {
		util.ErrorResponse(c, 403, err.Error(), nil)
		return
	}
	result, err := h.service.UsageContext(c.Request.Context(), filter, group)
	if err != nil {
		util.ErrorResponse(c, 503, err.Error(), nil)
		return
	}
	if group == "overview" || group == "daily" {
		actor := security.GetActor(c)
		result["businessStatus"] = "unavailable"
		result["businessError"] = "Owner业务统计未就绪"
		var totals any
		result["messageCount"], result["conversationCount"], result["memoryCount"], result["feedbackCount"] = totals, totals, totals, totals
		if h.canonicalMemory != nil {
			view, readErr := h.canonicalMemory.BusinessUsage(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: h.memoryCoreID}, result["clearedAt"].(string))
			if readErr != nil {
				result["businessError"] = readErr.Error()
			} else {
				result["businessStatus"], result["businessOwnerId"] = "ready", view.OwnerID
				delete(result, "businessError")
				result["messageCount"], result["conversationCount"], result["memoryCount"], result["feedbackCount"] = view.Totals.Messages, view.Totals.Conversations, view.Totals.Memories, view.Totals.Feedback
				if group == "daily" {
					items := result["daily"].([]map[string]any)
					days := map[string]map[string]any{}
					for _, item := range items {
						item["messages"], item["conversations"], item["memories"], item["feedback"] = int64(0), int64(0), int64(0), int64(0)
						days[item["date"].(string)] = item
					}
					for _, day := range view.Daily {
						item := days[day.Date]
						if item == nil {
							continue
						}
						item["messages"], item["conversations"], item["memories"], item["feedback"] = day.Messages, day.Conversations, day.Memories, day.Feedback
					}
					sort.Slice(items, func(i, j int) bool { return items[i]["date"].(string) > items[j]["date"].(string) })
					if len(items) > 30 {
						items = items[:30]
					}
					result["daily"] = items
				}
			}
		}
	}
	if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
		util.ErrorResponse(c, 409, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) UsageOverview(c *gin.Context) { h.usage(c, "overview") }
func (h *Handler) UsageDaily(c *gin.Context)    { h.usage(c, "daily") }
func (h *Handler) UsageModels(c *gin.Context)   { h.usage(c, "models") }
func (h *Handler) UsageSources(c *gin.Context)  { h.usage(c, "sources") }
func (h *Handler) UsageClear(c *gin.Context) {
	filter, err := usageFilter(c)
	if err != nil {
		util.ErrorResponse(c, 403, err.Error(), nil)
		return
	}
	if c.Query("expectedCoreId") != filter.SpaceID || c.Query("expectedDeviceId") != filter.DeviceID {
		util.ErrorResponse(c, 409, "统计范围已变化，请重新加载后重置", nil)
		return
	}
	var result map[string]any
	err = coordination.CommitCurrent(c.Request.Context(), func() error {
		var err error
		result, err = h.service.ClearUsageContext(c.Request.Context(), filter)
		return err
	})
	if err != nil {
		util.ErrorResponse(c, 409, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}
