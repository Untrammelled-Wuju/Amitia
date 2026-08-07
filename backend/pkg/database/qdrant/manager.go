// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrant

import (
	"fmt"
	"net/http"
	"time"

	"github.com/u-ai/backend/log"
)

func SetQdrantShuttingDown() {}

func IsQdrantShuttingDown() bool { return false }

func WaitForQdrant(port int) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/readyz", port)
	client := http.Client{Timeout: 500 * time.Millisecond}
	for i := 0; i < 60; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			log.Info("Qdrant端口就绪", "port", port)
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	return fmt.Errorf("等待Qdrant启动超时(30s)")
}
