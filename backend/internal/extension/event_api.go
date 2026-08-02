package extension

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type EventAPI struct {
	runtime *Runtime
}

func NewEventAPI(runtime *Runtime) *EventAPI {
	return &EventAPI{runtime: runtime}
}

func (api *EventAPI) service(c *gin.Context) *event.Service {
	if api.runtime == nil || api.runtime.Kernel == nil {
		return nil
	}
	container := api.runtime.Kernel.Container()
	if container == nil {
		return nil
	}
	return container.EventService
}

func (api *EventAPI) RegisterRoutes(group *gin.RouterGroup) {
	events := group.Group("/events")
	events.GET("/types", api.listEventTypes)
	events.GET("/types/:typeId/:version", api.getEventType)
	events.POST("/types", api.registerEventType)
	events.POST("/publish", api.publishEvent)
	events.POST("/publish-tx", api.publishEventTx)
	events.GET("/subscriptions", api.listSubscriptions)
	events.GET("/subscriptions/:contributionId", api.getSubscription)
	events.POST("/subscriptions", api.registerSubscription)
	events.DELETE("/subscriptions/:contributionId", api.unregisterSubscription)
	events.GET("/deliveries", api.listDeliveries)
	events.GET("/deliveries/:deliveryId", api.getDelivery)
	events.GET("/dead-letters", api.listDeadLetters)
	events.GET("/dead-letters/:deadLetterId", api.getDeadLetter)
	events.POST("/dead-letters/:deadLetterId/replay", api.replayDeadLetter)
	events.POST("/dead-letters/:deadLetterId/discard", api.discardDeadLetter)
	events.GET("/stats", api.getStats)
	events.GET("/outbox", api.listOutbox)
	events.GET("/audit", api.listAudit)
	events.POST("/circuits/:subscriptionId/reset", api.resetCircuit)
}

func (api *EventAPI) listEventTypes(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	types, err := svc.ListEventTypes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": types, "total": len(types)})
}

func (api *EventAPI) getEventType(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	typeID := event.EventTypeID(c.Param("typeId"))
	versionStr := c.Param("version")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version"})
		return
	}
	def, err := svc.GetEventType(c.Request.Context(), typeID, version)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, def)
}

func (api *EventAPI) registerEventType(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	var def event.EventTypeDefinition
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if err := svc.RegisterEventType(c.Request.Context(), def); err != nil {
		if strings.Contains(err.Error(), "conflict") || strings.Contains(err.Error(), "duplicate") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "event type registered"})
}

func (api *EventAPI) publishEvent(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	var body struct {
		EventTypeID   string          `json:"eventTypeId"`
		Version       int             `json:"version"`
		Payload       json.RawMessage `json:"payload"`
		ProducerID    string          `json:"producerId"`
		ProducerType  string          `json:"producerType"`
		AggregateType string          `json:"aggregateType"`
		AggregateID   string          `json:"aggregateId"`
		PartitionKey  string          `json:"partitionKey"`
		OrderingKey   string          `json:"orderingKey"`
		TraceID       string          `json:"traceId"`
		OperationID   string          `json:"operationId"`
		ParentEventID string          `json:"parentEventId"`
		ParentDepth   int             `json:"parentDepth"`
		Metadata      json.RawMessage `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	opts := event.PublishOptions{
		ProducerID:    body.ProducerID,
		ProducerType:  body.ProducerType,
		AggregateType: body.AggregateType,
		AggregateID:   body.AggregateID,
		PartitionKey:  body.PartitionKey,
		OrderingKey:   body.OrderingKey,
		TraceID:       body.TraceID,
		OperationID:   body.OperationID,
		ParentEventID: body.ParentEventID,
		ParentDepth:   body.ParentDepth,
		Metadata:      body.Metadata,
	}
	result, err := svc.Publish(c.Request.Context(), event.EventTypeID(body.EventTypeID), body.Version, body.Payload, opts)
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, result)
}

func (api *EventAPI) publishEventTx(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	var body struct {
		EventTypeID   string          `json:"eventTypeId"`
		Version       int             `json:"version"`
		Payload       json.RawMessage `json:"payload"`
		ProducerID    string          `json:"producerId"`
		ProducerType  string          `json:"producerType"`
		AggregateType string          `json:"aggregateType"`
		AggregateID   string          `json:"aggregateId"`
		PartitionKey  string          `json:"partitionKey"`
		OrderingKey   string          `json:"orderingKey"`
		TraceID       string          `json:"traceId"`
		OperationID   string          `json:"operationId"`
		ParentEventID string          `json:"parentEventId"`
		ParentDepth   int             `json:"parentDepth"`
		Metadata      json.RawMessage `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	tx, err := svc.BeginTx(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer tx.Rollback()
	opts := event.PublishOptions{
		ProducerID:    body.ProducerID,
		ProducerType:  body.ProducerType,
		AggregateType: body.AggregateType,
		AggregateID:   body.AggregateID,
		PartitionKey:  body.PartitionKey,
		OrderingKey:   body.OrderingKey,
		TraceID:       body.TraceID,
		OperationID:   body.OperationID,
		ParentEventID: body.ParentEventID,
		ParentDepth:   body.ParentDepth,
		Metadata:      body.Metadata,
	}
	result, err := svc.PublishTx(c.Request.Context(), tx, event.EventTypeID(body.EventTypeID), body.Version, body.Payload, opts)
	if err != nil {
		writeEventError(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, result)
}

func (api *EventAPI) listSubscriptions(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	extensionID := c.Query("extensionId")
	eventTypeID := c.Query("eventTypeId")
	if extensionID != "" {
		subs := svc.ListSubscriptionsByExtension(c.Request.Context(), extensionID)
		c.JSON(http.StatusOK, gin.H{"items": subs, "total": len(subs)})
		return
	}
	if eventTypeID != "" {
		subs := svc.ListSubscriptionsByType(c.Request.Context(), event.EventTypeID(eventTypeID))
		c.JSON(http.StatusOK, gin.H{"items": subs, "total": len(subs)})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId or eventTypeId required"})
}

func (api *EventAPI) getSubscription(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	contributionID := c.Param("contributionId")
	sub, ok := svc.GetSubscription(c.Request.Context(), contributionID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (api *EventAPI) registerSubscription(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	var defs []event.EventSubscriptionDefinition
	if err := c.ShouldBindJSON(&defs); err != nil {
		var single event.EventSubscriptionDefinition
		if err := c.ShouldBindJSON(&single); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
			return
		}
		defs = []event.EventSubscriptionDefinition{single}
	}
	if err := svc.RegisterSubscriptions(c.Request.Context(), defs); err != nil {
		if strings.Contains(err.Error(), "conflict") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "subscription registered", "count": len(defs)})
}

func (api *EventAPI) unregisterSubscription(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	contributionID := c.Param("contributionId")
	if err := svc.UnregisterSubscription(c.Request.Context(), contributionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (api *EventAPI) listDeliveries(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	limit, offset := parsePagination(c, 100)
	filter := event.DeliveryFilter{
		ExtensionID:    c.Query("extensionId"),
		SubscriptionID: c.Query("subscriptionId"),
		EventID:        c.Query("eventId"),
	}
	if status := c.Query("status"); status != "" {
		filter.Status = event.DeliveryStatus(status)
	}
	deliveries, err := svc.ListDeliveries(c.Request.Context(), filter, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": deliveries, "total": len(deliveries)})
}

func (api *EventAPI) getDelivery(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	deliveryID := c.Param("deliveryId")
	delivery, err := svc.GetDelivery(c.Request.Context(), deliveryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, delivery)
}

func (api *EventAPI) listDeadLetters(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	limit, offset := parsePagination(c, 100)
	filter := event.DeadLetterFilter{
		ExtensionID:    c.Query("extensionId"),
		SubscriptionID: c.Query("subscriptionId"),
	}
	if reason := c.Query("reason"); reason != "" {
		filter.Reason = event.DeadLetterReason(reason)
	}
	if status := c.Query("status"); status != "" {
		filter.Status = event.DeadLetterStatus(status)
	}
	records, err := svc.ListDeadLetters(c.Request.Context(), filter, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": records, "total": len(records)})
}

func (api *EventAPI) getDeadLetter(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	deadLetterID := c.Param("deadLetterId")
	record, err := svc.GetDeadLetter(c.Request.Context(), deadLetterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, record)
}

func (api *EventAPI) replayDeadLetter(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	deadLetterID := c.Param("deadLetterId")
	var body struct {
		Strategy          string `json:"strategy"`
		NewSubscriptionID string `json:"newSubscriptionId"`
		RequestedBy       string `json:"requestedBy"`
		Reason            string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Strategy == "" {
		body.Strategy = "replay_same_subscription"
	}
	req := event.ReplayRequest{
		DeadLetterID:      deadLetterID,
		Strategy:          event.ReplayStrategy(body.Strategy),
		NewSubscriptionID: body.NewSubscriptionID,
		RequestedBy:       body.RequestedBy,
		Reason:            body.Reason,
	}
	if err := svc.ReplayDeadLetter(c.Request.Context(), req); err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "dead letter replayed"})
}

func (api *EventAPI) discardDeadLetter(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	deadLetterID := c.Param("deadLetterId")
	if err := svc.DiscardDeadLetter(c.Request.Context(), deadLetterID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (api *EventAPI) getStats(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	stats, err := svc.Stats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (api *EventAPI) listOutbox(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	limit, offset := parsePagination(c, 100)
	extensionID := c.Query("extensionId")
	statusStr := c.Query("status")
	if extensionID != "" {
		records, err := svc.ListOutboxByExtension(c.Request.Context(), extensionID, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": records, "total": len(records)})
		return
	}
	if statusStr != "" {
		records, err := svc.ListOutboxByStatus(c.Request.Context(), event.OutboxStatus(statusStr), limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": records, "total": len(records)})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId or status required"})
}

func (api *EventAPI) listAudit(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	filter := event.AuditFilter{
		EventID:     c.Query("eventId"),
		DeliveryID:  c.Query("deliveryId"),
		ExtensionID: c.Query("extensionId"),
		Action:      c.Query("action"),
	}
	entries := svc.QueryAudit(filter)
	c.JSON(http.StatusOK, gin.H{"items": entries, "total": len(entries)})
}

func (api *EventAPI) resetCircuit(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event service unavailable"})
		return
	}
	subscriptionID := c.Param("subscriptionId")
	svc.ResetCircuit(subscriptionID)
	c.JSON(http.StatusOK, gin.H{"message": "circuit reset"})
}

func parsePagination(c *gin.Context, defaultLimit int) (int, int) {
	limit := defaultLimit
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	offset := 0
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	return limit, offset
}

func writeEventError(c *gin.Context, err error) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not found"):
		c.JSON(http.StatusNotFound, gin.H{"error": msg})
	case strings.Contains(msg, "denied") || strings.Contains(msg, "permission"):
		c.JSON(http.StatusForbidden, gin.H{"error": msg})
	case strings.Contains(msg, "conflict"):
		c.JSON(http.StatusConflict, gin.H{"error": msg})
	case strings.Contains(msg, "invalid") || strings.Contains(msg, "required"):
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
	case strings.Contains(msg, "depth") || strings.Contains(msg, "loop"):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": msg})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
	}
}

var _ = context.Background
var _ = sql.ErrNoRows
var _ = fmt.Sprintf
var _ = time.Now
