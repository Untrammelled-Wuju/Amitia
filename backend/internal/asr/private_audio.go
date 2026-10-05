package asr

import (
	"context"
	"errors"
	"github.com/u-ai/backend/internal/audioformat"
	"io"
	"strings"
)

const maxPrivateAudioBytes = 2 << 20
const maxPrivateASRResponseBytes = 64 << 10

func readPrivateASRResponse(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxPrivateASRResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPrivateASRResponseBytes {
		return nil, errors.New("语音识别响应超过大小限制")
	}
	return data, nil
}

func RecognizePrivateAudio(ctx context.Context, cfg *AsrConfig, data []byte, mimeType, language string) (string, error) {
	if err := context.Cause(ctx); err != nil {
		return "", err
	}
	if cfg == nil || strings.TrimSpace(cfg.ApiKey) == "" || !SupportsSegmentPCM(cfg) {
		return "", errors.New("当前 Core 未配置支持私有音频识别的服务")
	}
	if len(data) == 0 || len(data) > maxPrivateAudioBytes {
		return "", errors.New("私有音频为空或超过 2 MiB")
	}
	filename := "audio.wav"
	if err := audioformat.Validate(data, mimeType); err != nil {
		return "", err
	}
	switch mimeType {
	case "audio/wav":
	case "audio/webm":
		if strings.ToLower(strings.TrimSpace(cfg.ApiType)) != "openai" {
			return "", errors.New("当前 Core 语音服务不支持 WebM 音频")
		}
		filename = "audio.webm"
	default:
		return "", errors.New("私有音频类型不受支持")
	}
	var text string
	var err error
	if strings.ToLower(strings.TrimSpace(cfg.ApiType)) == "openai" {
		text, err = recognizeOpenAI(ctx, cfg, data, filename, strings.TrimSpace(language))
	} else {
		text, err = recognizeAzure(ctx, cfg, data, strings.TrimSpace(language))
	}
	if cancelled := context.Cause(ctx); cancelled != nil {
		return "", cancelled
	}
	if err != nil {
		return "", err
	}
	if text == "" {
		return "", errors.New("语音识别未返回文字")
	}
	return text, nil
}
