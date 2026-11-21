package chat

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var ErrUnsupportedAttachmentFormat = errors.New("unsupported attachment format")

type AttachmentFormat string

const (
	AttachmentFormatText         AttachmentFormat = "text"
	AttachmentFormatPDF          AttachmentFormat = "pdf"
	AttachmentFormatWord         AttachmentFormat = "docx"
	AttachmentFormatSpreadsheet  AttachmentFormat = "xlsx"
	AttachmentFormatPresentation AttachmentFormat = "pptx"
	AttachmentFormatLegacyOffice AttachmentFormat = "legacy-office"
	AttachmentFormatCSV          AttachmentFormat = "csv"
	AttachmentFormatArchive      AttachmentFormat = "archive"
	AttachmentFormatImage        AttachmentFormat = "image"
	AttachmentFormatBinary       AttachmentFormat = "binary"
)

var attachmentFormatByMIMEType = map[string]AttachmentFormat{
	"application/pdf": AttachmentFormatPDF,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   AttachmentFormatWord,
	"application/vnd.ms-word.document.macroenabled.12":                          AttachmentFormatWord,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         AttachmentFormatSpreadsheet,
	"application/vnd.ms-excel.sheet.macroenabled.12":                            AttachmentFormatSpreadsheet,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": AttachmentFormatPresentation,
	"application/vnd.ms-powerpoint.presentation.macroenabled.12":                AttachmentFormatPresentation,
	"application/vnd.openxmlformats-officedocument.presentationml.slideshow":    AttachmentFormatPresentation,
	"application/vnd.ms-powerpoint.slideshow.macroenabled.12":                   AttachmentFormatPresentation,
	"application/msword":            AttachmentFormatLegacyOffice,
	"application/vnd.ms-excel":      AttachmentFormatLegacyOffice,
	"application/vnd.ms-powerpoint": AttachmentFormatLegacyOffice,
	"text/csv":                      AttachmentFormatCSV,
	"text/tab-separated-values":     AttachmentFormatCSV,
}

var attachmentFormatByExtension = map[string]AttachmentFormat{
	".pdf":  AttachmentFormatPDF,
	".docx": AttachmentFormatWord,
	".docm": AttachmentFormatWord,
	".xlsx": AttachmentFormatSpreadsheet,
	".xlsm": AttachmentFormatSpreadsheet,
	".pptx": AttachmentFormatPresentation,
	".pptm": AttachmentFormatPresentation,
	".ppsx": AttachmentFormatPresentation,
	".doc":  AttachmentFormatLegacyOffice,
	".xls":  AttachmentFormatLegacyOffice,
	".ppt":  AttachmentFormatLegacyOffice,
	".csv":  AttachmentFormatCSV,
	".tsv":  AttachmentFormatCSV,
	".zip":  AttachmentFormatArchive,
	".gz":   AttachmentFormatArchive,
	".tar":  AttachmentFormatArchive,
	".7z":   AttachmentFormatArchive,
	".rar":  AttachmentFormatArchive,
}

var attachmentTextExtensions = map[string]struct{}{
	".txt": {}, ".md": {}, ".markdown": {}, ".log": {}, ".json": {}, ".jsonl": {}, ".xml": {},
	".yaml": {}, ".yml": {}, ".toml": {}, ".ini": {}, ".conf": {}, ".cfg": {}, ".env": {},
	".html": {}, ".htm": {}, ".css": {}, ".scss": {}, ".less": {},
	".js": {}, ".mjs": {}, ".cjs": {}, ".ts": {}, ".tsx": {}, ".jsx": {}, ".vue": {}, ".svelte": {},
	".go": {}, ".py": {}, ".rb": {}, ".java": {}, ".kt": {}, ".swift": {}, ".rs": {}, ".c": {}, ".h": {},
	".cpp": {}, ".hpp": {}, ".cs": {}, ".php": {}, ".sh": {}, ".zsh": {}, ".fish": {}, ".ps1": {},
	".sql": {}, ".graphql": {}, ".proto": {}, ".dart": {}, ".lua": {}, ".pl": {}, ".r": {},
	".srt": {}, ".vtt": {}, ".ass": {}, ".tex": {}, ".rst": {}, ".diff": {}, ".patch": {},
}

func ResolveAttachmentFormat(filePath, mimeType string) (AttachmentFormat, error) {
	lowerMIME := strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	extension := strings.ToLower(filepath.Ext(filePath))
	if format, ok := attachmentFormatByMIMEType[lowerMIME]; ok {
		return format, nil
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	head := make([]byte, 16)
	count, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", fmt.Errorf("read attachment signature: %w", err)
	}
	head = head[:count]
	switch {
	case bytes.HasPrefix(head, []byte("%PDF-")):
		return AttachmentFormatPDF, nil
	case bytes.HasPrefix(head, []byte("PK\x03\x04")), bytes.HasPrefix(head, []byte("PK\x05\x06")):
		return resolveZipAttachmentFormat(filePath)
	case bytes.HasPrefix(head, []byte{0xD0, 0xCF, 0x11, 0xE0}):
		return AttachmentFormatLegacyOffice, nil
	case imageFormatOf(head) != "":
		return AttachmentFormatImage, nil
	}
	if format, ok := attachmentFormatByExtension[extension]; ok {
		return format, nil
	}
	if _, ok := attachmentTextExtensions[extension]; ok {
		return AttachmentFormatText, nil
	}
	if len(head) > 0 && utf8.Valid(head) && bytes.IndexByte(head, 0) < 0 {
		return AttachmentFormatText, nil
	}
	return AttachmentFormatBinary, nil
}

func resolveZipAttachmentFormat(filePath string) (AttachmentFormat, error) {
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return AttachmentFormatArchive, nil
	}
	defer archive.Close()
	if len(archive.File) > 4096 {
		return AttachmentFormatArchive, nil
	}
	var word, spreadsheet, presentation bool
	for _, entry := range archive.File {
		switch {
		case entry.Name == "word/document.xml":
			word = true
		case entry.Name == "xl/workbook.xml":
			spreadsheet = true
		case entry.Name == "ppt/presentation.xml":
			presentation = true
		}
	}
	switch {
	case presentation:
		return AttachmentFormatPresentation, nil
	case word:
		return AttachmentFormatWord, nil
	case spreadsheet:
		return AttachmentFormatSpreadsheet, nil
	default:
		return AttachmentFormatArchive, nil
	}
}

func imageFormatOf(head []byte) string {
	switch {
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF}):
		return "jpeg"
	case bytes.HasPrefix(head, []byte("GIF87a")), bytes.HasPrefix(head, []byte("GIF89a")):
		return "gif"
	case bytes.HasPrefix(head, []byte("BM")):
		return "bmp"
	case len(head) >= 12 && bytes.HasPrefix(head, []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")):
		return "webp"
	case bytes.HasPrefix(head, []byte{0x49, 0x49, 0x2A, 0x00}), bytes.HasPrefix(head, []byte{0x4D, 0x4D, 0x00, 0x2A}):
		return "tiff"
	case len(head) >= 12 && bytes.Equal(head[4:8], []byte("ftyp")) && (bytes.Equal(head[8:12], []byte("heic")) || bytes.Equal(head[8:12], []byte("heif")) || bytes.Equal(head[8:12], []byte("avif"))):
		return "heif"
	}
	return ""
}

func attachmentFormatLabel(format AttachmentFormat) string {
	switch format {
	case AttachmentFormatText:
		return "纯文本"
	case AttachmentFormatCSV:
		return "CSV 表格"
	case AttachmentFormatPDF:
		return "PDF"
	case AttachmentFormatWord:
		return "DOCX"
	case AttachmentFormatSpreadsheet:
		return "XLSX"
	case AttachmentFormatPresentation:
		return "PPTX"
	case AttachmentFormatLegacyOffice:
		return "旧版 Office 二进制（.doc/.xls/.ppt）"
	case AttachmentFormatArchive:
		return "ZIP/压缩包"
	case AttachmentFormatImage:
		return "图片"
	default:
		return "无法识别的二进制"
	}
}

func unsupportedFormatError(format AttachmentFormat, mimeType, name string) error {
	return fmt.Errorf("%s 不是可提取文本的格式：format=%s mimeType=%q name=%q: %w", attachmentFormatLabel(format), format, mimeType, name, ErrUnsupportedAttachmentFormat)
}
