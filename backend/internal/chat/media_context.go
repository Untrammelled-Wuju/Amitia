package chat

import (
	"fmt"
	"strings"

	applog "github.com/u-ai/backend/log"
)

func AnalyzeImageContext(ownerSpaceID, imageURL string) (string, string) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return "", ""
	}
	applog.Info(fmt.Sprintf("[Image] Analyzing image: %s", imageURL[:min(len(imageURL), 80)]))
	desc, errDetail := analyzeImageInternal(ownerSpaceID, imageURL)
	if desc == "" && errDetail != "" {
		applog.Warn(fmt.Sprintf("[Image] Analysis failed: %s", errDetail))
		return "[图片描述：图片解析失败：" + errDetail + "]", errDetail
	}
	if desc == "" {
		return "", ""
	}
	applog.Info(fmt.Sprintf("[Image] Analysis success, descLen=%d", len(desc)))
	return "[图片描述：" + desc + "]", ""
}

func AnalyzeVideoContext(ownerSpaceID, videoURL string) (string, string) {
	videoURL = strings.TrimSpace(videoURL)
	if videoURL == "" {
		return "", ""
	}
	applog.Info(fmt.Sprintf("[Video] Analyzing video: %s", videoURL[:min(len(videoURL), 80)]))
	desc, errDetail := analyzeVideoInternal(ownerSpaceID, videoURL)
	if desc == "" && errDetail != "" {
		applog.Warn(fmt.Sprintf("[Video] Analysis failed: %s", errDetail))
		return "[视频描述：视频解析失败：" + errDetail + "]", errDetail
	}
	if desc == "" {
		applog.Warn("[Video] Analysis returned empty without error")
		return "", ""
	}
	applog.Info(fmt.Sprintf("[Video] Analysis success, descLen=%d", len(desc)))
	return "[视频描述：" + desc + "]", ""
}
