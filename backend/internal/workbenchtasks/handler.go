package workbenchtasks

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/workbenchhttp"
)

var validConversation = regexp.MustCompile(`^[a-zA-Z0-9_:.-]{1,160}$`)

type Handler struct {
	records RecordReader
	live    SnapshotReader
	access  ConversationAccess
	budget  BudgetReconciliationReader
}

func NewHandler(records RecordReader, observers ...SnapshotReader) *Handler {
	h := &Handler{records: records}
	if len(observers) > 0 {
		h.live = observers[0]
	}
	return h
}
func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/workbench/tasks", protectWorkbenchTaskResponse, h.list)
	group.GET("/workbench/tasks/checkpoints", protectWorkbenchTaskResponse, h.checkpoints)
	if h.budget != nil {
		group.GET("/workbench/tasks/budget-reconciliation", protectWorkbenchTaskResponse, h.budgetReconciliation)
		if _, ok := h.budget.(BudgetEvidenceReviewer); ok {
			group.POST("/workbench/tasks/budget-reconciliation/review", protectWorkbenchTaskResponse, h.budgetReconciliationReview)
		}
	}
}
func (h *Handler) WithBudgetInspection(reader BudgetReconciliationReader) *Handler {
	h.budget = reader
	return h
}

var _ BudgetReconciliationReader = (*mindruntime.ModelUsageStore)(nil)

func protectWorkbenchTaskResponse(c *gin.Context) {
	workbenchhttp.ProtectResponse(c.Writer.Header())
	c.Next()
}
func (h *Handler) list(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	var data TaskList
	h.readConversation(c, func(ctx context.Context, query QueryContext) error {
		var err error
		data, err = List(ctx, scopedRecordReader{reader: h.records, role: query.ExecutionScope.RoleID, space: query.ExecutionScope.SpaceID, canRead: query.CanReadRecord}, query.ExecutionScope.SpaceID, query.ConversationID, h.live)
		data.QueryContext = &query
		return err
	})
	if !c.Writer.Written() && data.QueryContext != nil {
		c.JSON(http.StatusOK, data)
	}
}
