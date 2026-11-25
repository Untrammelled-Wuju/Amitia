// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/featuregate"
)

type featureRouteEntry struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
}

func featureRouteInventory(engine *gin.Engine) []featureRouteEntry {
	routes := engine.Routes()
	entries := make([]featureRouteEntry, 0, len(routes))
	for _, route := range routes {
		entries = append(entries, featureRouteEntry{Method: route.Method, Path: route.Path, Handler: route.Handler})
	}
	sort.Slice(entries, func(left, right int) bool {
		if entries[left].Path != entries[right].Path {
			return entries[left].Path < entries[right].Path
		}
		return entries[left].Method < entries[right].Method
	})
	return entries
}

func featureRouteDigest(entries []featureRouteEntry) (string, error) {
	payload, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func registerFeatureGateRoutes(apiGroup *gin.RouterGroup, engine *gin.Engine) {
	if !featuregate.Enabled(featuregate.NameRouteInventory) {
		return
	}
	apiGroup.GET("/features", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "ok",
			"data": gin.H{
				"snapshot": featuregate.Snapshot(),
				"canary":   featureCanary.snapshot(),
			},
		})
	})
	apiGroup.GET("/features/routes", func(c *gin.Context) {
		entries := featureRouteInventory(engine)
		digest, err := featureRouteDigest(entries)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "ok",
			"data": gin.H{
				"total":  len(entries),
				"digest": digest,
				"routes": entries,
			},
		})
	})
}
