package business

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/u-ai/backend/internal/audioformat"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

type Attachment struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	MIME string `json:"mimeType"`
	Data string `json:"data"`
	Hash string `json:"sha256"`
}

func ValidateAttachments(items []Attachment) error {
	if len(items) > 2 {
		return errors.New("单条消息最多携带两个附件")
	}
	total := 0
	for _, item := range items {
		if (item.Kind != "image" && item.Kind != "audio") || item.Name == "" || len(item.Name) > 256 || strings.ContainsAny(item.Name, "\x00\r\n") || len(item.Data) > base64.StdEncoding.EncodedLen(1<<20) {
			return errors.New("附件格式无效或超过 1 MiB")
		}
		data, err := base64.StdEncoding.Strict().DecodeString(item.Data)
		if err != nil || len(data) == 0 || len(data) > 1<<20 {
			return errors.New("附件编码无效或超过 1 MiB")
		}
		total += len(data)
		if total > 2<<20 {
			return errors.New("消息附件总大小超过 2 MiB")
		}
		digest := sha256.Sum256(data)
		if item.Hash != hex.EncodeToString(digest[:]) {
			return errors.New("附件完整性校验失败")
		}
		if item.Kind == "audio" {
			if len(items) != 1 {
				return errors.New("语音消息只能携带一段音频")
			}
			if err := audioformat.Validate(data, item.MIME); err != nil {
				return err
			}
			continue
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(data))
		mime := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "gif": "image/gif"}[format]
		if err != nil || mime == "" || item.MIME != mime || config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 32<<20 {
			return errors.New("图片内容、类型或尺寸无效")
		}
	}
	return nil
}
