// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func probeServer(t *testing.T, status int, body string, delay time.Duration) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("unexpected probe path %q", r.URL.Path)
		}
		if delay > 0 {
			time.Sleep(delay)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestProbeProviderConnectionAcceptsOnlySuccessfulStatus(t *testing.T) {
	server := probeServer(t, http.StatusOK, `{"data":[{"id":"deepseek-chat"}]}`, 0)
	result := probeProviderConnection(context.Background(), server.URL, "sk-test", "openai")
	if result["success"] != true || result["status"] != "ok" {
		t.Fatalf("2xx must be the only success path: %+v", result)
	}
}

func TestProbeProviderConnectionKeepsUpstreamAuthError(t *testing.T) {
	upstream := `{"error":{"message":"Invalid API key provided","type":"invalid_request_error","code":"invalid_api_key"}}`
	server := probeServer(t, http.StatusUnauthorized, upstream, 5*time.Millisecond)
	result := probeProviderConnection(context.Background(), server.URL, "sk-bad", "openai")
	if result["success"] != false {
		t.Fatalf("401 must not be reported as success: %+v", result)
	}
	if result["status"] != "auth_failed" {
		t.Fatalf("401 must map to auth_failed: %+v", result)
	}
	if result["statusCode"] != http.StatusUnauthorized {
		t.Fatalf("statusCode lost: %+v", result)
	}
	message, _ := result["message"].(string)
	if !strings.Contains(message, "invalid_api_key") || !strings.Contains(message, "Invalid API key provided") {
		t.Fatalf("upstream body must be preserved verbatim: %q", message)
	}
	if raw, _ := result["rawResponse"].(string); raw != upstream {
		t.Fatalf("rawResponse must keep the full upstream payload: %q", raw)
	}
	if latency, _ := result["latencyMs"].(int); latency <= 0 {
		t.Fatalf("failure latency must be measured: %+v", result)
	}
}

func TestProbeProviderConnectionPreservesServerErrorBody(t *testing.T) {
	server := probeServer(t, http.StatusInternalServerError, "upstream exploded", 0)
	result := probeProviderConnection(context.Background(), server.URL, "sk-test", "openai")
	if result["success"] != false || result["status"] != "error" {
		t.Fatalf("500 must fail: %+v", result)
	}
	message, _ := result["message"].(string)
	if !strings.Contains(message, "500") || !strings.Contains(message, "upstream exploded") {
		t.Fatalf("server error body must survive: %q", message)
	}
	if truncated, _ := result["truncated"].(bool); truncated {
		t.Fatalf("short body must not be truncated: %+v", result)
	}
}

func TestProbeProviderConnectionMarksTruncatedBody(t *testing.T) {
	server := probeServer(t, http.StatusBadGateway, strings.Repeat("x", 9000), 0)
	result := probeProviderConnection(context.Background(), server.URL, "sk-test", "openai")
	if truncated, _ := result["truncated"].(bool); !truncated {
		t.Fatalf("oversized body must be flagged: %+v", result)
	}
	if length, _ := result["rawResponseLength"].(int); length != 9000 {
		t.Fatalf("rawResponseLength must report the untruncated size: %+v", result)
	}
	if raw, _ := result["rawResponse"].(string); len(raw) != 9000 {
		t.Fatalf("rawResponse must stay complete: %d", len(raw))
	}
}
