// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/sse"
)

func TestHTTPServerBaseContextFollowsApplicationCancellation(t *testing.T) {
	appCtx, cancel := context.WithCancel(context.Background())
	srv := newHTTPServer("127.0.0.1:0", appCtx)
	baseCtx := srv.BaseContext(nil)
	cancel()
	select {
	case <-baseCtx.Done():
	default:
		t.Fatal("HTTP base context remained active after application cancellation")
	}
}

func TestSSEHandlerStopsWhenApplicationContextCancels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	appCtx, cancel := context.WithCancel(context.Background())
	engine := withRequestContextFallback(gin.New())
	engine.GET("/sse", sse.SSEHandler)
	server := httptest.NewUnstartedServer(engine)
	server.Config.BaseContext = func(net.Listener) context.Context {
		return appCtx
	}
	server.Start()
	defer server.Close()

	requestDone := make(chan error, 1)
	go func() {
		response, err := server.Client().Get(server.URL + "/sse?clientId=shutdown-test")
		if err != nil {
			requestDone <- err
			return
		}
		defer response.Body.Close()
		buffer := make([]byte, 1)
		_, err = response.Body.Read(buffer)
		requestDone <- err
	}()

	deadline := time.Now().Add(2 * time.Second)
	for !sse.Global.HasClients() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !sse.Global.HasClients() {
		t.Fatal("SSE client did not subscribe")
	}

	cancel()
	select {
	case <-requestDone:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE request remained active after application cancellation")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := server.Config.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("server shutdown returned error: %v", err)
	}
}
