package system

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) ContinuityListThreads(c *gin.Context) {
	if h.continuityRepo == nil {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "持续事项服务不可用", nil)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	statuses := []continuity.ThreadStatus{}
	for _, raw := range strings.Split(c.Query("status"), ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		status := continuity.ThreadStatus(strings.TrimSpace(raw))
		if !continuity.ValidThreadStatus(status) {
			util.ErrorResponse(c, http.StatusBadRequest, "status 无效", nil)
			return
		}
		statuses = append(statuses, status)
	}
	items, err := h.continuityRepo.ListThreads(continuity.ListThreadsFilter{SpaceID: webChatSpaceID(c), CharacterID: c.Query("characterId"), Status: statuses, Query: c.Query("q"), Limit: limit, Offset: offset})
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) ContinuityCreateThread(c *gin.Context) {
	if h.continuityRepo == nil {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "持续事项服务不可用", nil)
		return
	}
	var body struct {
		CharacterID string `json:"characterId"`
		ParentID    string `json:"parentThreadId"`
		Title       string `json:"title"`
		Goal        string `json:"goal"`
		Summary     string `json:"summary"`
		NextAction  string `json:"nextAction"`
		Priority    int    `json:"priority"`
	}
	if c.ShouldBindJSON(&body) != nil || strings.TrimSpace(body.Title) == "" {
		util.ErrorResponse(c, http.StatusBadRequest, "title 必填", nil)
		return
	}
	spaceID := webChatSpaceID(c)
	parentID := strings.TrimSpace(body.ParentID)
	if parentID != "" {
		parent, err := h.continuityRepo.GetThread(parentID, spaceID)
		if err != nil || parent == nil {
			util.ErrorResponse(c, http.StatusBadRequest, "parentThreadId 不属于当前空间", nil)
			return
		}
	}
	thread := &continuity.Thread{SpaceID: spaceID, CharacterID: strings.TrimSpace(body.CharacterID), ParentThreadID: parentID, Title: strings.TrimSpace(body.Title), Goal: strings.TrimSpace(body.Goal), Summary: strings.TrimSpace(body.Summary), CurrentState: strings.TrimSpace(body.Summary), NextAction: strings.TrimSpace(body.NextAction), Priority: body.Priority, Status: continuity.ThreadStatusActive, Confidence: 1}
	if err := h.continuityRepo.CreateThread(thread); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	_, _ = h.continuityRepo.AppendEvent(&continuity.ThreadEvent{ThreadID: thread.ID, EventType: "thread.created", SourceType: "user", PayloadJSON: `{"summary":"手动创建持续事项"}`, IdempotencyKey: "thread-created|" + thread.ID, OccurredAt: time.Now().UTC()})
	util.SuccessResponse(c, thread)
}

func (h *Handler) continuityOwnedThread(c *gin.Context) *continuity.Thread {
	if h.continuityRepo == nil {
		return nil
	}
	thread, _ := h.continuityRepo.GetThread(strings.TrimSpace(c.Param("id")), webChatSpaceID(c))
	return thread
}

func (h *Handler) ContinuityGetThread(c *gin.Context) {
	thread := h.continuityOwnedThread(c)
	if thread == nil {
		util.ErrorResponse(c, http.StatusNotFound, "持续事项不存在", nil)
		return
	}
	waits, _ := h.continuityRepo.ListWaits(thread.ID, nil, 100)
	events, _ := h.continuityRepo.ListRecentEvents(thread.ID, 50)
	bindings, _ := h.continuityRepo.ListBindings(thread.ID)
	util.SuccessResponse(c, gin.H{"thread": thread, "waits": waits, "events": events, "bindings": bindings})
}

func (h *Handler) ContinuityUpdateThread(c *gin.Context) {
	thread := h.continuityOwnedThread(c)
	if thread == nil {
		util.ErrorResponse(c, http.StatusNotFound, "持续事项不存在", nil)
		return
	}
	var body struct {
		Title        *string                  `json:"title"`
		Goal         *string                  `json:"goal"`
		Status       *continuity.ThreadStatus `json:"status"`
		Summary      *string                  `json:"summary"`
		CurrentState *string                  `json:"currentState"`
		NextAction   *string                  `json:"nextAction"`
		Priority     *int                     `json:"priority"`
	}
	if c.ShouldBindJSON(&body) != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "请求体无效", nil)
		return
	}
	updates := map[string]interface{}{}
	if body.Title != nil {
		updates["title"] = strings.TrimSpace(*body.Title)
	}
	if body.Goal != nil {
		updates["goal"] = strings.TrimSpace(*body.Goal)
	}
	if body.Summary != nil {
		updates["summary"] = strings.TrimSpace(*body.Summary)
	}
	if body.CurrentState != nil {
		updates["current_state"] = strings.TrimSpace(*body.CurrentState)
	}
	if body.NextAction != nil {
		updates["next_action"] = strings.TrimSpace(*body.NextAction)
	}
	if body.Priority != nil {
		updates["priority"] = *body.Priority
	}
	terminal := false
	if body.Status != nil {
		if !continuity.ValidThreadStatus(*body.Status) {
			util.ErrorResponse(c, http.StatusBadRequest, "status 无效", nil)
			return
		}
		updates["status"] = *body.Status
		terminal = body.Status.IsTerminal()
		if terminal {
			now := time.Now().UTC()
			updates["completed_at"] = &now
			updates["next_action"] = ""
		} else {
			updates["completed_at"] = nil
		}
	}
	updated, err := h.continuityRepo.UpdateThread(thread.ID, webChatSpaceID(c), updates)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if terminal {
		_ = h.continuityRepo.CancelOpenWaits(thread.ID, "thread_terminal")
	}
	util.SuccessResponse(c, updated)
}

func (h *Handler) ContinuityListEvents(c *gin.Context) {
	thread := h.continuityOwnedThread(c)
	if thread == nil {
		util.ErrorResponse(c, http.StatusNotFound, "持续事项不存在", nil)
		return
	}
	items, err := h.continuityRepo.ListRecentEvents(thread.ID, 100)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) ContinuityListWaits(c *gin.Context) {
	thread := h.continuityOwnedThread(c)
	if thread == nil {
		util.ErrorResponse(c, http.StatusNotFound, "持续事项不存在", nil)
		return
	}
	items, err := h.continuityRepo.ListWaits(thread.ID, nil, 100)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) ContinuityCreateWait(c *gin.Context) {
	thread := h.continuityOwnedThread(c)
	if thread == nil {
		util.ErrorResponse(c, http.StatusNotFound, "持续事项不存在", nil)
		return
	}
	var body struct {
		Type        string         `json:"type"`
		Description string         `json:"description"`
		Condition   map[string]any `json:"condition"`
		ResumeHint  string         `json:"resumeHint"`
		DueAt       *time.Time     `json:"dueAt"`
		AutoResume  *bool          `json:"autoResume"`
	}
	if c.ShouldBindJSON(&body) != nil || !continuity.ValidWaitType(body.Type) {
		util.ErrorResponse(c, http.StatusBadRequest, "wait type 无效", nil)
		return
	}
	autoResume := body.Type != continuity.WaitTypeUser
	if body.AutoResume != nil {
		autoResume = *body.AutoResume
	}
	if body.Type == continuity.WaitTypeTime && body.DueAt == nil {
		util.ErrorResponse(c, http.StatusBadRequest, "time wait 必须提供 dueAt", nil)
		return
	}
	condition, _ := json.Marshal(body.Condition)
	if !validManagementWaitCondition(body.Type, body.Condition) {
		util.ErrorResponse(c, http.StatusBadRequest, "该 wait type 必须提供可精确匹配的 condition selector", nil)
		return
	}
	wait := &continuity.Wait{ThreadID: thread.ID, WaitType: body.Type, Status: continuity.WaitStatusWaiting, Description: strings.TrimSpace(body.Description), ConditionJSON: string(condition), ResumeHint: strings.TrimSpace(body.ResumeHint), DueAt: body.DueAt, AutoResume: autoResume}
	if err := h.continuityRepo.CreateWait(wait); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if thread.Status == continuity.ThreadStatusActive {
		_, _ = h.continuityRepo.UpdateThreadCAS(thread.ID, thread.Revision, map[string]interface{}{"status": continuity.ThreadStatusWaiting})
	}
	util.SuccessResponse(c, wait)
}

func (h *Handler) ContinuityResolveWait(c *gin.Context) {
	thread := h.continuityOwnedThread(c)
	if thread == nil || h.continuityCoordinator == nil {
		util.ErrorResponse(c, http.StatusNotFound, "持续事项或 Wait 服务不存在", nil)
		return
	}
	wait, err := h.continuityRepo.GetWait(strings.TrimSpace(c.Param("waitId")))
	if err != nil || wait == nil || wait.ThreadID != thread.ID {
		util.ErrorResponse(c, http.StatusNotFound, "Wait 不存在", nil)
		return
	}
	var body struct {
		Resolution map[string]any `json:"resolution"`
		Resume     *bool          `json:"resume"`
	}
	_ = c.ShouldBindJSON(&body)
	resume := wait.AutoResume
	if body.Resume != nil {
		resume = *body.Resume
	}
	updated, err := h.continuityCoordinator.ResolveWait(c.Request.Context(), wait.ID, "manual", body.Resolution, resume)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, updated)
}

func (h *Handler) ContinuityCancelWait(c *gin.Context) {
	thread := h.continuityOwnedThread(c)
	if thread == nil || h.continuityCoordinator == nil {
		util.ErrorResponse(c, http.StatusNotFound, "持续事项或 Wait 服务不存在", nil)
		return
	}
	wait, err := h.continuityRepo.GetWait(strings.TrimSpace(c.Param("waitId")))
	if err != nil || wait == nil || wait.ThreadID != thread.ID {
		util.ErrorResponse(c, http.StatusNotFound, "Wait 不存在", nil)
		return
	}
	updated, err := h.continuityCoordinator.CancelWait(c.Request.Context(), wait.ID, "manual")
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, updated)
}

func (h *Handler) ContinuitySignal(c *gin.Context) {
	if h.continuityCoordinator == nil {
		util.ErrorResponse(c, http.StatusServiceUnavailable, "持续事项 Wait 服务不可用", nil)
		return
	}
	var signal continuity.Signal
	if c.ShouldBindJSON(&signal) != nil || !continuity.ValidWaitType(signal.WaitType) {
		util.ErrorResponse(c, http.StatusBadRequest, "signal 无效", nil)
		return
	}
	// Never trust a caller supplied ownership scope.
	signal.SpaceID = webChatSpaceID(c)
	resolved, err := h.continuityCoordinator.Signal(c.Request.Context(), signal)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"resolved": resolved, "count": len(resolved)})
}

func validManagementWaitCondition(waitType string, condition map[string]any) bool {
	if waitType == continuity.WaitTypeUser || waitType == continuity.WaitTypeTime {
		return true
	}
	keys := []string{}
	switch waitType {
	case continuity.WaitTypeDevice:
		keys = []string{"deviceId"}
	case continuity.WaitTypeApproval:
		keys = []string{"approvalId", "requestId", "toolCallId"}
	case continuity.WaitTypeDependency:
		keys = []string{"executionId", "workflowRunId", "taskRunId", "operationId", "invocationId"}
	case continuity.WaitTypeExternal:
		for key, value := range condition {
			if key == "kind" || key == "autoResume" || key == "resumeHint" {
				continue
			}
			if strings.TrimSpace(toManagementString(value)) != "" {
				return true
			}
		}
		return false
	}
	for _, key := range keys {
		if strings.TrimSpace(toManagementString(condition[key])) != "" {
			return true
		}
	}
	return false
}

func toManagementString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
