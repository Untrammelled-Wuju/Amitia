// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"encoding/json"
	"io"
	"time"
)

func (s *service) getQQHealthStatus() string {
	resp, err := s.qqSidecarGet("/api/status")
	if err != nil {
		return "disconnected"
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	if data, ok := result["data"].(map[string]interface{}); ok {
		if status, ok := data["status"].(string); ok {
			return status
		}
	}
	return "disconnected"
}

func (s *service) isQQSidecarRunning() bool {
	resp, err := s.qqSidecarGet("/api/status")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	success, _ := result["success"].(bool)
	return success
}

func (s *service) GetQQBridgeStatus() map[string]interface{} {
	resp, err := s.qqSidecarGet("/api/status")
	if err != nil {
		return map[string]interface{}{"connected": false, "status": "disconnected", "running": false}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	running := result["success"] == true
	if data, ok := result["data"].(map[string]interface{}); ok {
		status, _ := data["status"].(string)
		return map[string]interface{}{"connected": status == "connected", "status": status, "running": running}
	}
	return map[string]interface{}{"connected": resp.StatusCode == 200, "status": "unknown", "running": running}
}

func (s *service) GetQQBridgeStatusDetail() map[string]interface{} {
	return s.qqReadSidecarResponse(s.qqSidecarGet("/api/status"))
}

func (s *service) GetQQBridgeConfig() map[string]interface{} {
	return map[string]interface{}{"config": map[string]interface{}{"mode": "qqbot", "sidecarPort": 9877}, "available": true}
}

func (s *service) GetQQBridgeEvents() map[string]interface{} {
	return map[string]interface{}{"events": []interface{}{}, "available": true}
}

func (s *service) QQBridgeRecover() map[string]interface{} {
	return s.qqReadSidecarResponse(s.qqSidecarPost("/api/login/reconnect", nil))
}

func (s *service) MaintenanceRestartQQBridge() map[string]interface{} {
	result := s.qqReadSidecarResponse(s.qqSidecarPost("/api/login/reconnect", nil))
	return map[string]interface{}{"restarted": true, "restartedAt": time.Now().Format(time.DateTime), "bridgeResult": result}
}
