package business

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"github.com/u-ai/backend/internal/audioformat"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"unicode/utf8"
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
	total, audioCount := 0, 0
	for _, item := range items {
		if item.Kind == "audio" {
			audioCount++
		}
	}
	if audioCount > 1 {
		return errors.New("单条消息只能携带一段音频")
	}
	if audioCount == 1 && len(items) > 1 {
		for _, item := range items {
			if item.Kind != "audio" && item.Kind != "image" {
				return errors.New("语音消息只能同时携带一张图片")
			}
		}
	}
	for _, item := range items {
		if (item.Kind != "image" && item.Kind != "audio" && item.Kind != "file" && item.Kind != "video") || item.Name == "" || len(item.Name) > 256 || strings.ContainsAny(item.Name, "\x00\r\n") || len(item.Data) > base64.StdEncoding.EncodedLen(1<<20) {
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
		if item.Kind == "file" {
			if err := validateFileAttachment(data, item.MIME); err != nil {
				return err
			}
			continue
		}
		if item.Kind == "video" {
			if !validVideoAttachment(data, item.MIME) {
				return errors.New("视频内容与类型不匹配，支持 MP4、QuickTime 和 WebM")
			}
			continue
		}
		if item.Kind == "audio" {
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

func validateFileAttachment(data []byte, mime string) error {
	switch mime {
	case "text/plain", "text/markdown", "text/csv", "application/json", "application/xml", "text/xml":
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return errors.New("文本文件必须为 UTF-8，且不得包含空字节")
		}
	case "application/pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return errors.New("PDF 文件内容无效")
		}
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
			return errors.New("Word 文件内容无效")
		}
	default:
		return errors.New("当前支持 UTF-8 文本、JSON、CSV、XML、PDF 和 DOCX 文件")
	}
	return nil
}

func validVideoAttachment(data []byte, mime string) bool {
	if mime == "video/webm" {
		return bytes.HasPrefix(data, []byte{0x1a, 0x45, 0xdf, 0xa3})
	}
	if mime != "video/mp4" && mime != "video/quicktime" {
		return false
	}
	for offset := 0; offset+8 <= len(data) && offset < 4096; {
		size := int64(binary.BigEndian.Uint32(data[offset : offset+4]))
		if size < 8 || size > int64(len(data)-offset) {
			return false
		}
		if string(data[offset+4:offset+8]) == "ftyp" {
			return size >= 16
		}
		offset += int(size)
	}
	return false
}
