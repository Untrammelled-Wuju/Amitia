// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

func (s *service) sidecarGet(path string) (*http.Response, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	return client.Get("http://127.0.0.1:9876" + path)
}

func (s *service) sidecarPost(path string, body map[string]interface{}) (*http.Response, error) {
	jsonBody, _ := json.Marshal(body)
	client := &http.Client{Timeout: 30 * time.Second}
	return client.Post("http://127.0.0.1:9876"+path, "application/json", bytes.NewReader(jsonBody))
}

func (s *service) readSidecarResponse(resp *http.Response, err error) map[string]interface{} {
	if err != nil {
		return map[string]interface{}{"success": false, "error": err.Error(), "available": false}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	if result == nil {
		result = map[string]interface{}{"success": resp.StatusCode == 200}
	}
	result["available"] = true
	return result
}

func (s *service) qqSidecarGet(path string) (*http.Response, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	return client.Get("http://127.0.0.1:9877" + path)
}

func (s *service) qqSidecarPost(path string, body map[string]interface{}) (*http.Response, error) {
	jsonBody, _ := json.Marshal(body)
	client := &http.Client{Timeout: 30 * time.Second}
	return client.Post("http://127.0.0.1:9877"+path, "application/json", bytes.NewReader(jsonBody))
}

func (s *service) qqReadSidecarResponse(resp *http.Response, err error) map[string]interface{} {
	if err != nil {
		return map[string]interface{}{"success": false, "error": err.Error(), "available": false}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	if result == nil {
		result = map[string]interface{}{"success": resp.StatusCode == 200}
	}
	result["available"] = true
	return result
}
