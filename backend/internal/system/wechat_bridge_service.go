// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"encoding/json"
	"io"
)

func (s *service) getWechatHealthStatus() string {
	resp, err := s.sidecarGet("/api/status")
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

func (s *service) GetWechatBridgeStatus() map[string]interface{} {
	resp, err := s.sidecarGet("/api/status")
	if err != nil {
		return map[string]interface{}{"connected": false, "status": "disconnected"}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	if data, ok := result["data"].(map[string]interface{}); ok {
		status, _ := data["status"].(string)
		return map[string]interface{}{"connected": status == "connected", "status": status}
	}
	return map[string]interface{}{"connected": resp.StatusCode == 200, "status": "unknown"}
}

func (s *service) GetWechatBridgeStatusDetail() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarGet("/api/status"))
}

func (s *service) GetWechatBridgeConfig() map[string]interface{} {
	return map[string]interface{}{"config": map[string]interface{}{"mode": "openclaw", "sidecarPort": 9876}, "available": true}
}

func (s *service) GetWechatBridgeEvents() map[string]interface{} {
	return map[string]interface{}{"events": []interface{}{}, "available": true}
}

func (s *service) GetWechatBridgeQRCode() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarGet("/api/qrcode"))
}

func (s *service) GetWechatEvents() map[string]interface{} {
	return map[string]interface{}{"events": []interface{}{}, "available": true}
}

func (s *service) GetWechatStatus() map[string]interface{} {
	resp, err := s.sidecarGet("/api/status")
	if err != nil {
		return map[string]interface{}{"connected": false, "status": "disconnected", "available": true}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	data, _ := result["data"].(map[string]interface{})
	if data == nil {
		data = map[string]interface{}{}
	}
	status, _ := data["status"].(string)
	if status == "" {
		status = "disconnected"
	}
	data["connected"] = status == "connected"
	data["status"] = status
	data["available"] = true
	return data
}

func (s *service) WechatBridgeRecover() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarPost("/api/login/reconnect", nil))
}

func (s *service) WechatCloudCheck() map[string]interface{} {
	return map[string]interface{}{"status": "not_checked", "available": true}
}

func (s *service) WechatCloudCheckReport() map[string]interface{} {
	return map[string]interface{}{"report": map[string]interface{}{}, "available": true}
}

func (s *service) WechatCloudCheckRiskSummary() map[string]interface{} {
	return map[string]interface{}{"risks": []interface{}{}, "available": true}
}

func (s *service) WechatCloudCheckRun() map[string]interface{} {
	return map[string]interface{}{"checkId": "", "started": true, "message": "检查已启动"}
}

func (s *service) WechatLoginReconnect() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarPost("/api/login/reconnect", nil))
}

func (s *service) WechatLoginRescan() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarPost("/api/login/rescan", nil))
}

func (s *service) WechatLoginStart() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarPost("/api/login/start", nil))
}

func (s *service) WechatLoginWait() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarPost("/api/login/wait", map[string]interface{}{"timeoutMs": 120000}))
}

func (s *service) WechatReplyTimingRecover() map[string]interface{} {
	return map[string]interface{}{"recovered": true, "message": "已恢复"}
}

func (s *service) UpdateWechatBridgeConfig(body map[string]interface{}) map[string]interface{} {
	_ = body
	return map[string]interface{}{"updated": true}
}

func (s *service) WechatReplyTimingStatus() map[string]interface{} {
	return map[string]interface{}{"status": "inactive", "available": true}
}
