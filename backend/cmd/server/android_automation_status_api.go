package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type androidAutomationProbeResult struct {
	Status string         `json:"status"`
	Result map[string]any `json:"result,omitempty"`
	Error  map[string]any `json:"error,omitempty"`
}

func registerAndroidAutomationStatusRoute(group *gin.RouterGroup, services *AppServices) {
	if group == nil {
		return
	}
	group.GET("/android-automation/status", func(c *gin.Context) {
		container := services.KernelContainer
		if container == nil || container.AndroidNativeProvider == nil {
			c.JSON(200, gin.H{
				"code": 200,
				"msg":  "ok",
				"data": gin.H{
					"available":      false,
					"providerHealth": "unavailable",
					"reason":         "android native provider is not configured in this runtime",
					"probes":         gin.H{},
				},
			})
			return
		}

		provider := container.AndroidNativeProvider
		probeCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		operations := []string{
			"accessibility.status",
			"interaction.status",
			"shizuku.status",
			"root.status",
			"adb.status",
			"notification.status",
			"system.overlay.status",
			"virtual_display.status",
		}
		results := make(map[string]androidAutomationProbeResult, len(operations))
		var mu sync.Mutex
		var wg sync.WaitGroup
		for i, operation := range operations {
			i, operation := i, operation
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp := provider.Execute(probeCtx, capability.AndroidBridgeRequest{
					ProtocolVersion: 1,
					RequestID:       fmt.Sprintf("automation-status-%d-%d", time.Now().UnixNano(), i),
					Operation:       operation,
					Payload:         map[string]any{},
				})
				item := androidAutomationProbeResult{Status: resp.Status, Result: resp.Result}
				if resp.Error != nil {
					item.Error = map[string]any{
						"code":       resp.Error.Code,
						"message":    resp.Error.Message,
						"domainCode": resp.Error.DomainCode,
					}
				}
				mu.Lock()
				results[operation] = item
				mu.Unlock()
			}()
		}
		wg.Wait()

		health := provider.Health(probeCtx)
		interactionProbe := results["interaction.status"]
		available, _ := interactionProbe.Result["available"].(bool)
		c.JSON(200, gin.H{
			"code": 200,
			"msg":  "ok",
			"data": gin.H{
				"available":      available,
				"providerHealth": string(health),
				"probedAt":       time.Now().UTC(),
				"probes":         results,
			},
		})
	})
}
