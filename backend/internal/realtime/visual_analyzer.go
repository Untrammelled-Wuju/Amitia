// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/vision"
)

type VisualAnalyzer interface {
	Analyze(ctx context.Context, frame VisualFrame) (string, error)
	Available() bool
}

type configuredVisualAnalyzer struct {
	visionSvc vision.Service
}

func NewConfiguredVisualAnalyzer(visionSvc vision.Service) VisualAnalyzer {
	if visionSvc == nil {
		return nil
	}
	return &configuredVisualAnalyzer{
		visionSvc: visionSvc,
	}
}

func (a *configuredVisualAnalyzer) Available() bool {
	if a == nil || a.visionSvc == nil {
		return false
	}
	cfg, err := a.visionSvc.GetActive()
	return err == nil && cfg.Ready()
}

func (a *configuredVisualAnalyzer) Analyze(ctx context.Context, frame VisualFrame) (string, error) {
	if a == nil || a.visionSvc == nil {
		return "", fmt.Errorf("visual analyzer unavailable")
	}
	cfg, err := a.visionSvc.GetActive()
	if err != nil || cfg == nil {
		return "", fmt.Errorf("active visual model unavailable")
	}
	if strings.TrimSpace(cfg.ApiKey) == "" && !cfg.IsLocal() {
		return "", fmt.Errorf("active visual model has no API key")
	}

	dataURI := "data:" + frame.MIME + ";base64," + base64.StdEncoding.EncodeToString(frame.Data)
	ctx, cancel := timeoutpolicy.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	return vision.GenerateImages(ctx, cfg, []string{dataURI}, visualPrompt(frame), 220)
}

func visualPrompt(frame VisualFrame) string {
	if frame.Source == VisualSourceScreen {
		return "你是实时屏幕视觉状态提取器。只描述当前帧中真实可见且对后续对话有帮助的信息：当前应用/页面、主要控件、可读文字、报错、选中区域和光标附近内容。不要猜测不可见内容。输出一段紧凑的中文状态，不要主动回答用户问题。"
	}
	return "你是实时摄像头视觉状态提取器。只描述当前帧中真实可见且对后续对话有帮助的信息：人物、主要物体、动作、环境、可读文字和当前关注主体。不要猜测不可见内容。输出一段紧凑的中文状态，不要主动回答用户问题。"
}
