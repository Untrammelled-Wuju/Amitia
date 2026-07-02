// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/tts"
	"gorm.io/gorm"
)

// SystemFormatInstruction is injected into every LLM call to enforce WeChat-style line splitting.
// It is NOT part of any character prompt and cannot be modified per character.
const SystemFormatInstruction = `【回复格式 - 系统固定规则】

每句话必须单独一行，用换行符分隔。
每句话尽量短，像微信连续消息一样。
能一句说完就一句，不要写长段落。
不要把多句话连成一段。
不要用句号连接多个意思。`

type Handler struct {
	service     Service
	db          *gorm.DB
	chatSvc     chat.Service
	dataLifecycle *mindruntime.DataLifecycleCoordinator
	unifiedEntry *interaction.UnifiedEntry
	versionInfo atomic.Value
}

func NewHandler(srv Service, db *gorm.DB, chatSvc chat.Service, dataLifecycle *mindruntime.DataLifecycleCoordinator, unifiedEntry *interaction.UnifiedEntry) *Handler {
	h := &Handler{service: srv, db: db, chatSvc: chatSvc, unifiedEntry: unifiedEntry, dataLifecycle: dataLifecycle}
	h.versionInfo.Store(srv.GetVersion())
	return h
}

func (h *Handler) getDBPath() string {
	var dbPath string
	h.db.Raw("PRAGMA database_list").Row().Scan(nil, nil, &dbPath)
	if dbPath == "" {
		dbPath = filepath.Join("data", "app.db")
	}
	return dbPath
}

func (h *Handler) readLogTail(path string, limit int) []map[string]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return []map[string]interface{}{}
	}
	lines := strings.Split(string(data), "\n")
	start := 0
	if len(lines) > limit {
		start = len(lines) - limit
	}
	result := []map[string]interface{}{}
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		entry := map[string]interface{}{"line": i + 1, "content": line, "timestamp": time.Now().Format("2006-01-02 15:04:05")}
		if strings.Contains(line, "error") {
			entry["level"] = "error"
		} else if strings.Contains(line, "warn") {
			entry["level"] = "warn"
		} else {
			entry["level"] = "info"
		}
		result = append(result, entry)
	}
	return result
}

func ttsSynthesizeWithTimeout(cfg *tts.TtsConfig, text string, timeout time.Duration) (*tts.SynthesizeResponse, error) {
	type result struct {
		res *tts.SynthesizeResponse
		err error
	}
	ch := make(chan result, 1)
	go func() {
		r, e := tts.Synthesize(cfg, text)
		ch <- result{r, e}
	}()
	select {
	case res := <-ch:
		return res.res, res.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("tts timeout after %v", timeout)
	}
}

func isReasoningLine(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "（推理") || strings.HasPrefix(s, "(推理")
}
