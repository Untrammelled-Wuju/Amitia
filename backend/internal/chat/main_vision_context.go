package chat

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/vision"
)

func (s *service) generateLocalVision(ctx context.Context, cfg *vision.VisionConfig, images []string, prompt string, maxTokens int) (string, error) {
	model, err := s.repo.GetModelByID(cfg.ID)
	if err != nil {
		return "", err
	}
	if maxTokens > 0 {
		model.MaxTokens = maxTokens
		model.MaxOutputTokens = maxTokens
	}
	parts := []ModelContentPart{{Type: ContentTypeText, Text: prompt}}
	for _, uri := range images {
		parts = append(parts, ModelContentPart{Type: ContentTypeImage, ResourceURI: uri})
	}
	text, _, err := s.callLLMMode(ctx, model, []map[string]interface{}{{"role": "user", "parts": parts}}, false)
	return text, err
}

func UseNativeMainVision(modelID int) bool {
	cfg, err := getVisionModelConfig()
	return err == nil && cfg.FromMainModel && (modelID == 0 || modelID == cfg.ID)
}

func resolveVisionImage(ctx context.Context, spaceID, uri string) (string, error) {
	if strings.HasPrefix(uri, "amitia://artifacts/") {
		if globalArtifactResolver == nil {
			return "", fmt.Errorf("图片资源解析器未就绪")
		}
		rc, res, err := globalArtifactResolver.Open(ctx, spaceID, uri)
		if err != nil {
			return "", err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, 20*1024*1024+1))
		if err != nil {
			return "", err
		}
		if len(data) > 20*1024*1024 {
			return "", fmt.Errorf("图片超过识别大小限制")
		}
		mimeType := res.MIMEType
		if mimeType == "" {
			mimeType = "image/png"
		}
		return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
	}
	if strings.HasPrefix(uri, "/images/") {
		if filepath.Base(uri) != strings.TrimPrefix(uri, "/images/") {
			return "", fmt.Errorf("无效的图片路径")
		}
		data, err := os.ReadFile(filepath.Join(config.AppCfg.Storage.DataDir, "images", filepath.Base(uri)))
		if err != nil {
			return "", err
		}
		mimeType := mime.TypeByExtension(filepath.Ext(uri))
		if mimeType == "" {
			mimeType = "image/png"
		}
		return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
	}
	if strings.HasPrefix(uri, "data:image/") || strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return uri, nil
	}
	return "", fmt.Errorf("不支持的图片资源格式")
}

func attachMainVisionImages(ctx context.Context, messages []map[string]interface{}, history []map[string]string, spaceID, currentImage string, modelID int) error {
	if !UseNativeMainVision(modelID) {
		return nil
	}
	var parts []ModelContentPart
	for _, item := range history {
		if item["imageUrl"] == "" {
			continue
		}
		uri, err := resolveVisionImage(ctx, spaceID, item["imageUrl"])
		if err != nil {
			return fmt.Errorf("历史图片读取失败：%w", err)
		}
		parts = append(parts, ModelContentPart{Type: ContentTypeText, Text: "历史图片，对应消息：" + item["content"]}, ModelContentPart{Type: ContentTypeImage, ResourceURI: uri})
	}
	if currentImage != "" {
		uri, err := resolveVisionImage(ctx, spaceID, currentImage)
		if err != nil {
			return err
		}
		parts = append(parts, ModelContentPart{Type: ContentTypeText, Text: "当前用户上传的图片"}, ModelContentPart{Type: ContentTypeImage, ResourceURI: uri})
	}
	if len(parts) == 0 {
		return nil
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i]["role"] != "user" {
			continue
		}
		content, _ := messages[i]["content"].(string)
		parts = append(parts, ModelContentPart{Type: ContentTypeText, Text: content})
		messages[i]["parts"] = parts
		return nil
	}
	return fmt.Errorf("视觉对话缺少用户消息")
}
