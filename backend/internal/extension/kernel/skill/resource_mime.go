// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package skill

import (
	"bytes"
	"encoding/json"
	"strings"
)

var extToMime = map[string]string{
	".md":       ResourceMimeTextMarkdown,
	".markdown": ResourceMimeTextMarkdown,
	".txt":      ResourceMimeTextPlain,
	".text":     ResourceMimeTextPlain,
	".csv":      ResourceMimeTextCSV,
	".tsv":      "text/tab-separated-values",
	".html":     ResourceMimeTextHTML,
	".htm":      ResourceMimeTextHTML,
	".json":     ResourceMimeApplicationJSON,
	".yaml":     ResourceMimeApplicationYAML,
	".yml":      ResourceMimeApplicationYAML,
	".xml":      ResourceMimeApplicationXML,
	".toml":     ResourceMimeApplicationTOML,
	".ndjson":   ResourceMimeApplicationJSON,
	".log":      ResourceMimeTextPlain,
	".ini":      ResourceMimeTextPlain,
	".cfg":      ResourceMimeTextPlain,
	".conf":     ResourceMimeTextPlain,
	".env":      ResourceMimeTextPlain,
	".png":      ResourceMimeImagePNG,
	".jpg":      ResourceMimeImageJPEG,
	".jpeg":     ResourceMimeImageJPEG,
	".gif":      ResourceMimeImageGIF,
	".webp":     ResourceMimeImageWebP,
	".pdf":      ResourceMimeApplicationPDF,
	".docx":     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx":     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx":     "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

func DetectMIMEFromExtension(ext string) (string, bool) {
	ext = strings.ToLower(ext)
	mime, ok := extToMime[ext]
	return mime, ok
}

func IsTextMIME(mime string) bool {
	if mime == "" {
		return false
	}
	if strings.HasPrefix(mime, "text/") {
		return true
	}
	if strings.HasPrefix(mime, "application/json") ||
		strings.HasPrefix(mime, "application/yaml") ||
		strings.HasPrefix(mime, "application/xml") ||
		strings.HasPrefix(mime, "application/toml") ||
		strings.HasPrefix(mime, "application/javascript") ||
		strings.HasPrefix(mime, "application/xhtml") {
		return true
	}
	return false
}

func IsBinaryMIME(mime string) bool {
	return !IsTextMIME(mime)
}

func SniffTextBytes(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if !utf8Valid(data) {
		return false
	}
	for _, b := range data {
		if b == 0x00 {
			return false
		}
	}
	return true
}

func utf8Valid(data []byte) bool {
	for len(data) > 0 {
		r, size := decodeRune(data)
		if r == 0xFFFD && size <= 1 {
			return false
		}
		data = data[size:]
	}
	return true
}

func decodeRune(data []byte) (rune, int) {
	if len(data) == 0 {
		return 0, 0
	}
	b0 := data[0]
	if b0 < 0x80 {
		return rune(b0), 1
	}
	if b0 < 0xC0 {
		return 0xFFFD, 1
	}
	if b0 < 0xE0 {
		if len(data) < 2 {
			return 0xFFFD, 1
		}
		return rune(b0&0x1F)<<6 | rune(data[1]&0x3F), 2
	}
	if b0 < 0xF0 {
		if len(data) < 3 {
			return 0xFFFD, 1
		}
		return rune(b0&0x0F)<<12 | rune(data[1]&0x3F)<<6 | rune(data[2]&0x3F), 3
	}
	if b0 < 0xF8 {
		if len(data) < 4 {
			return 0xFFFD, 1
		}
		return rune(b0&0x07)<<18 | rune(data[1]&0x3F)<<12 | rune(data[2]&0x3F)<<6 | rune(data[3]&0x3F), 4
	}
	return 0xFFFD, 1
}

func StripBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}

func IsValidUTF8Text(data []byte) bool {
	data = StripBOM(data)
	return utf8Valid(data)
}

func LooksLikeJSON(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return false
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		var v any
		return json.Unmarshal(trimmed, &v) == nil
	}
	return false
}

func MIMEFromSniff(data []byte, declaredMime string) string {
	if IsTextMIME(declaredMime) && SniffTextBytes(data) {
		return declaredMime
	}
	if SniffTextBytes(data) {
		return ResourceMimeTextPlain
	}
	return "application/octet-stream"
}

func MIMEFamily(mime string) string {
	if strings.HasPrefix(mime, "text/") {
		return "text"
	}
	if strings.HasPrefix(mime, "image/") {
		return "image"
	}
	if strings.HasPrefix(mime, "application/") {
		if IsTextMIME(mime) {
			return "text"
		}
		return "binary"
	}
	return "other"
}
