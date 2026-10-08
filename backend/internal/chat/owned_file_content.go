package chat

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/devicemesh/business"
)

func ownedFileText(ctx context.Context, item business.Attachment) (string, error) {
	data, err := base64.StdEncoding.Strict().DecodeString(item.Data)
	if err != nil {
		return "", err
	}
	if item.MIME == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		return ownedWordText(ctx, data)
	}
	if item.MIME == "application/pdf" {
		return ownedPDFText(ctx, data)
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", errors.New("文件内容不是有效 UTF-8 文本")
	}
	return string(data), ctx.Err()
}

func ownedWordText(ctx context.Context, data []byte) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errors.New("Word 文件结构无效")
	}
	if len(archive.File) > 256 {
		return "", errors.New("Word 文件包含过多条目")
	}
	var document *zip.File
	for _, entry := range archive.File {
		if entry.Name == "word/document.xml" {
			if document != nil {
				return "", errors.New("Word 文件包含重复正文")
			}
			document = entry
		}
	}
	if document == nil || document.UncompressedSize64 > 2<<20 {
		return "", errors.New("Word 正文缺失或超过解析限制")
	}
	reader, err := document.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()
	decoder := xml.NewDecoder(io.LimitReader(reader, (2<<20)+1))
	var result strings.Builder
	depth, textDepth := 0, 0
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", errors.New("Word 正文 XML 无效")
		}
		switch value := token.(type) {
		case xml.StartElement:
			depth++
			if depth > 128 {
				return "", errors.New("Word 正文嵌套超过限制")
			}
			if value.Name.Local == "t" {
				textDepth = depth
			}
			if value.Name.Local == "tab" {
				result.WriteByte('\t')
			}
			if value.Name.Local == "br" {
				result.WriteByte('\n')
			}
		case xml.CharData:
			if textDepth > 0 {
				result.Write(value)
			}
		case xml.EndElement:
			if depth == textDepth {
				textDepth = 0
			}
			if value.Name.Local == "p" {
				result.WriteByte('\n')
			}
			depth--
		}
		if result.Len() > 1<<20 {
			return "", errors.New("Word 提取文本超过 1 MiB")
		}
	}
	if strings.TrimSpace(result.String()) == "" {
		return "", errors.New("Word 文件没有可读取的文字")
	}
	return result.String(), nil
}

func ownedPDFText(ctx context.Context, data []byte) (string, error) {
	commandName := "pdftotext"
	if config.AppCfg != nil && strings.TrimSpace(config.AppCfg.Providers.Search.Research.PDFTextCommand) != "" {
		commandName = strings.TrimSpace(config.AppCfg.Providers.Search.Research.PDFTextCommand)
	}
	tool, err := exec.LookPath(commandName)
	if err != nil {
		return "", errors.New("当前 Core 未安装 PDF 文字解析工具 pdftotext，请由 Core 管理员配置")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "amitia-owned-pdf-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "input.pdf")
	if err := os.WriteFile(input, data, 0600); err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, tool, "-enc", "UTF-8", "-f", "1", "-l", "64", input, "-")
	var output ownedTextBuffer
	command.Stdout = &output
	if err := command.Run(); err != nil {
		return "", errors.New("Core PDF 文字解析失败")
	}
	text := output.Bytes()
	if len(text) > 1<<20 || !utf8.Valid(text) {
		return "", errors.New("PDF 提取文字超过限制或编码无效")
	}
	if strings.TrimSpace(string(text)) == "" {
		return "", errors.New("PDF 没有可读取的文字，请使用图片识别扫描页")
	}
	return string(text), ctx.Err()
}

type ownedTextBuffer struct{ bytes.Buffer }

func (b *ownedTextBuffer) Write(data []byte) (int, error) {
	if b.Len()+len(data) > 1<<20 {
		return 0, errors.New("提取文本超过 1 MiB")
	}
	return b.Buffer.Write(data)
}
