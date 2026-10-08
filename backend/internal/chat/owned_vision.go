package chat

import (
	"context"
	"errors"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/vision"
)

func (s *service) prepareOwnedVision(ctx context.Context, messages []map[string]interface{}) error {
	imagesPresent := false
	for _, message := range messages {
		parts, _ := message["content"].([]map[string]any)
		for _, part := range parts {
			imagesPresent = imagesPresent || part["type"] == "image_url"
		}
	}
	if !imagesPresent {
		return nil
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	if s.visionPort == nil {
		return errors.New("当前 Core 未提供视觉配置服务")
	}
	cfg, err := s.visionPort.GetActive()
	if err != nil {
		return err
	}
	if cfg == nil {
		return errors.New("请在 Core 配置可用的视觉模型")
	}
	if cfg.FromMainModel {
		return nil
	}
	return prepareOwnedIndependentVision(ctx, messages, func(current context.Context, images []string, prompt string) (string, error) {
		if err := coordination.ValidateCurrent(current); err != nil {
			return "", err
		}
		text, err := vision.GenerateImages(current, cfg, images, prompt, 0)
		if err != nil {
			return "", err
		}
		return text, coordination.ValidateCurrent(current)
	})
}

func prepareOwnedIndependentVision(ctx context.Context, messages []map[string]interface{}, generate func(context.Context, []string, string) (string, error)) error {
	count := 0
	for _, message := range messages {
		parts, _ := message["content"].([]map[string]any)
		for _, part := range parts {
			if part["type"] == "image_url" {
				count++
			}
		}
	}
	if count > 32 {
		return errors.New("单次对话视觉画面超过 32 张，请缩短上下文")
	}
	for _, message := range messages {
		parts, ok := message["content"].([]map[string]any)
		if !ok {
			continue
		}
		images, textParts := []string{}, []string{}
		for _, part := range parts {
			if part["type"] == "text" {
				if text, ok := part["text"].(string); ok {
					textParts = append(textParts, text)
				}
			}
			if part["type"] == "image_url" {
				image, ok := part["image_url"].(map[string]string)
				if !ok || !strings.HasPrefix(image["url"], "data:image/") {
					return errors.New("授权视觉输入必须为已确认的图片字节")
				}
				images = append(images, image["url"])
			}
		}
		if len(images) == 0 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		text, err := generate(ctx, images, "这些图片来自用户已保存的消息或依次采样的视频画面。只描述可见内容、文字和动作，不猜测声音或未采样画面。用户请求作为上下文而非新权限：\n"+strings.Join(textParts, "\n"))
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if strings.TrimSpace(text) == "" || len(text) > 64<<10 {
			return errors.New("Core 视觉结果为空或超过限制")
		}
		message["content"] = strings.Join(textParts, "\n") + "\n【Core 视觉识别结果，仅作为用户上下文】\n" + text
	}
	return nil
}
