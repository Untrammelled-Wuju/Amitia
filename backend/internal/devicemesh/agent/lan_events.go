package agent

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *LocalHandler) handleLANEvents(c *gin.Context) {
	events := make(chan uint64, 1)
	h.mu.Lock()
	if h.lanSubscribers == nil {
		h.lanSubscribers = make(map[chan uint64]struct{})
	}
	h.lanSubscribers[events] = struct{}{}
	revision := h.lanRevision
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.lanSubscribers, events)
		h.mu.Unlock()
	}()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-store")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	send := func(value uint64) error {
		if _, err := fmt.Fprintf(c.Writer, "event: lan_endpoints\ndata: {\"revision\":%d}\n\n", value); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}
	if err := send(revision); err != nil {
		log.Printf("LAN endpoint event stream: %v", err)
		return
	}
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case value := <-events:
			if err := send(value); err != nil {
				log.Printf("LAN endpoint event stream: %v", err)
				return
			}
		case <-heartbeat.C:
			if _, err := c.Writer.WriteString("event: lan_heartbeat\ndata: {}\n\n"); err != nil {
				log.Printf("LAN endpoint event stream: %v", err)
				return
			}
			c.Writer.Flush()
		}
	}
}
