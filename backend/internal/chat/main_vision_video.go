package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/vision"
)

func analyzeMainModelVideo(spaceID, uri string, cfg *vision.VisionConfig) (string, string) {
	ctx, cancel := timeoutpolicy.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	images, err := extractVisionVideoFrames(ctx, spaceID, uri)
	if err != nil {
		return "", err.Error()
	}
	text, err := vision.GenerateImages(ctx, cfg, images, "以下图片按时间顺序均匀采样自同一段视频。请描述可见的场景、人物动作、事件发展与文字。只根据采样画面判断，不要猜测未采样内容或声音。", 0)
	if err != nil {
		return "", err.Error()
	}
	return text, ""
}

func extractVisionVideoFrames(ctx context.Context, spaceID, uri string) ([]string, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("主模型视频识别需要当前运行环境提供 FFmpeg 抽帧能力")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, fmt.Errorf("主模型视频识别需要当前运行环境提供 FFprobe 视频时长检测能力")
	}
	dir, err := os.MkdirTemp("", "amitia-vision-video-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "input.video")
	file, err := os.Create(input)
	if err != nil {
		return nil, err
	}
	var reader io.ReadCloser
	switch {
	case strings.HasPrefix(uri, "amitia://artifacts/"):
		if globalArtifactResolver == nil {
			file.Close()
			return nil, fmt.Errorf("视频资源解析器未就绪")
		}
		reader, _, err = globalArtifactResolver.Open(ctx, spaceID, uri)
	case strings.HasPrefix(uri, "/videos/"):
		if filepath.Base(uri) != strings.TrimPrefix(uri, "/videos/") {
			file.Close()
			return nil, fmt.Errorf("无效的视频路径")
		}
		reader, err = os.Open(filepath.Join(config.AppCfg.Storage.DataDir, "videos", filepath.Base(uri)))
	case strings.HasPrefix(uri, "data:video/"):
		header, data, ok := strings.Cut(uri, ",")
		if !ok || !strings.HasSuffix(header, ";base64") {
			file.Close()
			return nil, fmt.Errorf("无效的视频数据")
		}
		reader = io.NopCloser(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)))
	default:
		file.Close()
		return nil, fmt.Errorf("不支持的视频资源格式")
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	size, err := io.Copy(file, io.LimitReader(reader, 256*1024*1024+1))
	reader.Close()
	file.Close()
	if err != nil {
		return nil, err
	}
	if size > 256*1024*1024 {
		return nil, fmt.Errorf("视频超过抽帧大小限制")
	}
	probe, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-protocol_whitelist", "file,pipe", "-show_entries", "format=duration", "-of", "json", input).Output()
	if err != nil {
		return nil, fmt.Errorf("视频时长检测失败：%w", err)
	}
	var metadata struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if json.Unmarshal(probe, &metadata) != nil {
		return nil, fmt.Errorf("无效的视频时长信息")
	}
	duration, err := strconv.ParseFloat(metadata.Format.Duration, 64)
	if err != nil || duration <= 0 {
		return nil, fmt.Errorf("无法检测视频时长")
	}
	filter := fmt.Sprintf("fps=%.9f,scale=1280:1280:force_original_aspect_ratio=decrease", 8/duration)
	if err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-protocol_whitelist", "file,pipe", "-i", input, "-vf", filter, "-frames:v", "8", "-q:v", "3", filepath.Join(dir, "frame-%02d.jpg")).Run(); err != nil {
		return nil, fmt.Errorf("视频抽帧失败：%w", err)
	}
	frames, err := filepath.Glob(filepath.Join(dir, "frame-*.jpg"))
	if err != nil || len(frames) == 0 {
		return nil, fmt.Errorf("视频未产生可识别画面")
	}
	images := make([]string, 0, len(frames))
	for _, frame := range frames {
		data, err := os.ReadFile(frame)
		if err != nil {
			return nil, err
		}
		images = append(images, "data:image/jpeg;base64,"+base64.StdEncoding.EncodeToString(data))
	}
	return images, nil
}
