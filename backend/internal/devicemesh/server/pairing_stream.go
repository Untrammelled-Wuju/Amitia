package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/pairing"
)

type pairingApprovalEvent struct {
	RequestID string `json:"requestId"`
	State     string `json:"state"`
	Revision  int64  `json:"revision"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

type pairingApprovalListEvent struct {
	Requests []pairing.Approval `json:"requests"`
}

func writePairingSSE(c *gin.Context, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := c.Writer.WriteString("event: " + event + "\ndata: " + string(data) + "\n\n"); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
}

func makePairingApprovalEventStreamHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.Param("requestId")
		approval, expiresAt, events, cancel, err := deps.PairingSvc.WatchApproval(c.Request.Context(), requestID)
		if err != nil {
			if err == pairing.ErrApprovalNotFound {
				c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
				return
			}
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": err.Error()})
			return
		}
		defer cancel()
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-store")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)
		event := pairingApprovalEvent{RequestID: approval.RequestID, State: approval.State, Revision: approval.Revision, ExpiresAt: expiresAt.UTC().Format(time.RFC3339Nano)}
		if err := writePairingSSE(c, "pairing_"+approval.State, event); err != nil {
			return
		}
		if approval.State != "pending" {
			return
		}
		expiry := time.Until(expiresAt)
		if expiry <= 0 {
			event.State = "expired"
			_ = writePairingSSE(c, "pairing_expired", event)
			return
		}
		timer := time.NewTimer(expiry)
		defer timer.Stop()
		for {
			select {
			case <-c.Request.Context().Done():
				return
			case <-timer.C:
				event.State = "expired"
				_ = writePairingSSE(c, "pairing_expired", event)
				return
			case updated, ok := <-events:
				if !ok {
					return
				}
				event.State = updated.State
				event.Revision = updated.Revision
				if err := writePairingSSE(c, "pairing_"+updated.State, event); err != nil {
					return
				}
				if updated.State != "pending" {
					return
				}
			}
		}
	}
}

func makeApprovalListStreamHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requirePairingAdministrator(c) {
			return
		}
		changes, cancel := deps.PairingSvc.WatchApprovalRequests()
		defer cancel()
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-store")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)
		sendSnapshot := func() error {
			requests, err := deps.PairingSvc.PendingApprovals(c.Request.Context())
			if err != nil {
				return err
			}
			return writePairingSSE(c, "pairing_approvals", pairingApprovalListEvent{Requests: requests})
		}
		if err := sendSnapshot(); err != nil {
			return
		}
		heartbeat := time.NewTicker(20 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case <-c.Request.Context().Done():
				return
			case _, ok := <-changes:
				if !ok {
					return
				}
				if err := sendSnapshot(); err != nil {
					return
				}
			case <-heartbeat.C:
				if _, err := c.Writer.WriteString(": keepalive\n\n"); err != nil {
					return
				}
				c.Writer.Flush()
			}
		}
	}
}
