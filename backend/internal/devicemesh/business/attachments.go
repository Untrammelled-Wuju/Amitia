package business

import (
	"errors"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type Attachment = coordination.ChatAttachmentReference

func ValidateAttachments(items []Attachment) error {
	if len(items) > 2 {
		return errors.New("单条消息最多携带两个附件")
	}
	audioCount := 0
	for _, item := range items {
		if err := item.Validate(); err != nil {
			return err
		}
		if item.Kind == "audio" {
			audioCount++
		}
	}
	if audioCount > 1 {
		return errors.New("单条消息只能携带一段音频")
	}
	if audioCount == 1 {
		for _, item := range items {
			if item.Kind != "audio" && item.Kind != "image" {
				return errors.New("语音消息只能同时携带一张图片")
			}
		}
	}
	return nil
}
