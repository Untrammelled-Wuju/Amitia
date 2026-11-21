package chat

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfmodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/u-ai/backend/internal/vision"
)

var ErrPDFTextUnavailable = errors.New("PDF has no extractable text")

func ExtractAttachmentText(ctx context.Context, filePath, mimeType string) (string, error) {
	format, err := ResolveAttachmentFormat(filePath, mimeType)
	if err != nil {
		return "", err
	}
	switch format {
	case AttachmentFormatWord:
		return ownedWordText(ctx, filePath)
	case AttachmentFormatPDF:
		return ownedPDFText(ctx, filePath)
	case AttachmentFormatSpreadsheet:
		return ownedSpreadsheetText(ctx, filePath)
	case AttachmentFormatPresentation:
		return ownedPresentationText(ctx, filePath)
	case AttachmentFormatText, AttachmentFormatCSV:
		return ownedPlainText(ctx, filePath, mimeType)
	default:
		return "", unsupportedFormatError(format, mimeType, filepath.Base(filePath))
	}
}

func ownedPlainText(ctx context.Context, filePath, mimeType string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (8<<20)+1))
	if err != nil {
		return "", err
	}
	if len(data) > 8<<20 {
		return "", errors.New("text attachment exceeds the 8 MiB extraction limit")
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf("按文本解析的附件不是有效 UTF-8：mimeType=%q name=%q: %w", mimeType, filepath.Base(filePath), ErrUnsupportedAttachmentFormat)
	}
	return string(data), ctx.Err()
}

func ownedWordText(ctx context.Context, filePath string) (string, error) {
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("open Word archive: %w", err)
	}
	defer archive.Close()
	if len(archive.File) > 256 {
		return "", errors.New("Word archive contains too many entries")
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
		return "", errors.New("Word document body is missing or exceeds the parsing limit")
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
			return "", fmt.Errorf("parse Word document XML: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			depth++
			if depth > 128 {
				return "", errors.New("Word document XML nesting exceeds the limit")
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
			} else if value.Name.Local == "tc" {
				result.WriteByte('\t')
			} else if value.Name.Local == "tr" {
				result.WriteByte('\n')
			}
			depth--
		}
		if result.Len() > 1<<20 {
			return "", errors.New("Word extracted text exceeds 1 MiB")
		}
	}
	if strings.TrimSpace(result.String()) == "" {
		return "", errors.New("Word document contains no readable text")
	}
	return result.String(), nil
}

type ownedContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *ownedContextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func ownedSpreadsheetText(ctx context.Context, filePath string) (string, error) {
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("open spreadsheet archive: %w", err)
	}
	defer archive.Close()
	entries := make(map[string]*zip.File, len(archive.File))
	for _, entry := range archive.File {
		entries[entry.Name] = entry
	}
	shared, err := readSharedStrings(ctx, entries["xl/sharedStrings.xml"])
	if err != nil {
		return "", err
	}
	workbook, err := readWorkbookSheets(ctx, entries["xl/workbook.xml"], entries["xl/_rels/workbook.xml.rels"])
	if err != nil {
		return "", err
	}
	var result strings.Builder
	for _, sheet := range workbook {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		entry := entries[sheet.Path]
		if entry == nil {
			return "", fmt.Errorf("spreadsheet worksheet %q is missing at %s", sheet.Name, sheet.Path)
		}
		values, err := readWorksheetCells(ctx, entry, shared)
		if err != nil {
			return "", fmt.Errorf("read spreadsheet sheet %q: %w", sheet.Name, err)
		}
		if len(values) == 0 {
			continue
		}
		result.WriteString("【工作表】")
		result.WriteString(sheet.Name)
		result.WriteByte('\n')
		for _, value := range values {
			result.WriteString(value)
			result.WriteByte('\n')
			if result.Len() > 1<<20 {
				return "", errors.New("spreadsheet extracted text exceeds 1 MiB")
			}
		}
		result.WriteByte('\n')
	}
	if strings.TrimSpace(result.String()) == "" {
		return "", errors.New("spreadsheet contains no readable cells")
	}
	return result.String(), nil
}

func readSharedStrings(ctx context.Context, entry *zip.File) ([]string, error) {
	if entry == nil || entry.UncompressedSize64 > 16<<20 {
		return nil, errors.New("spreadsheet shared strings are missing or exceed the parsing limit")
	}
	reader, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("open spreadsheet shared strings: %w", err)
	}
	defer reader.Close()
	decoder := xml.NewDecoder(&ownedContextReader{ctx: ctx, reader: reader})
	values := make([]string, 0)
	var current strings.Builder
	inItem, inText := false, false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse spreadsheet shared strings: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "si":
				inItem = true
				current.Reset()
			case "t":
				if inItem {
					inText = true
				}
			}
		case xml.CharData:
			if inText {
				current.Write(value)
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "t":
				inText = false
			case "si":
				values = append(values, current.String())
				inItem = false
				if len(values) > 1_000_000 {
					return nil, errors.New("spreadsheet contains too many shared strings")
				}
			}
		}
	}
	return values, nil
}

type ownedWorkbookSheet struct {
	Name string
	Path string
}

func readWorkbookSheets(ctx context.Context, workbookEntry, relationshipsEntry *zip.File) ([]ownedWorkbookSheet, error) {
	if workbookEntry == nil || relationshipsEntry == nil || workbookEntry.UncompressedSize64 > 4<<20 || relationshipsEntry.UncompressedSize64 > 4<<20 {
		return nil, errors.New("spreadsheet workbook metadata is missing or exceeds the parsing limit")
	}
	relationships, err := readWorkbookRelationships(ctx, relationshipsEntry)
	if err != nil {
		return nil, err
	}
	reader, err := workbookEntry.Open()
	if err != nil {
		return nil, fmt.Errorf("open spreadsheet workbook: %w", err)
	}
	defer reader.Close()
	decoder := xml.NewDecoder(&ownedContextReader{ctx: ctx, reader: reader})
	sheets := make([]ownedWorkbookSheet, 0)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse spreadsheet workbook: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "sheet" {
			continue
		}
		var name, relationshipID string
		for _, attribute := range start.Attr {
			switch attribute.Name.Local {
			case "name":
				name = attribute.Value
			case "id":
				relationshipID = attribute.Value
			}
		}
		target := relationships[relationshipID]
		if name == "" || target == "" {
			return nil, fmt.Errorf("spreadsheet sheet metadata is incomplete name=%q relationship=%q", name, relationshipID)
		}
		sheets = append(sheets, ownedWorkbookSheet{Name: name, Path: normalizeWorkbookTarget(target)})
		if len(sheets) > 256 {
			return nil, errors.New("spreadsheet contains too many worksheets")
		}
	}
	if len(sheets) == 0 {
		return nil, errors.New("spreadsheet contains no worksheets")
	}
	return sheets, nil
}

func readWorkbookRelationships(ctx context.Context, entry *zip.File) (map[string]string, error) {
	reader, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("open spreadsheet relationships: %w", err)
	}
	defer reader.Close()
	decoder := xml.NewDecoder(&ownedContextReader{ctx: ctx, reader: reader})
	result := map[string]string{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse spreadsheet relationships: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "Relationship" {
			continue
		}
		var id, target string
		for _, attribute := range start.Attr {
			switch attribute.Name.Local {
			case "Id":
				id = attribute.Value
			case "Target":
				target = attribute.Value
			}
		}
		if id != "" && target != "" {
			result[id] = target
		}
	}
	return result, nil
}

func normalizeWorkbookTarget(target string) string {
	target = strings.TrimSpace(strings.ReplaceAll(target, "\\", "/"))
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(target, "/")
	}
	for strings.HasPrefix(target, "../") {
		target = strings.TrimPrefix(target, "../")
	}
	if strings.HasPrefix(target, "xl/") {
		return target
	}
	return "xl/" + strings.TrimPrefix(target, "./")
}

func readWorksheetCells(ctx context.Context, entry *zip.File, shared []string) ([]string, error) {
	if entry.UncompressedSize64 > 64<<20 {
		return nil, errors.New("spreadsheet worksheet exceeds the parsing limit")
	}
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	decoder := xml.NewDecoder(&ownedContextReader{ctx: ctx, reader: reader})
	values := make([]string, 0)
	var cellReference, cellType, cellValue, inlineValue strings.Builder
	inCell, inValue, inInline := false, false, false
	cellValue.Reset()
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse worksheet XML: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "c":
				inCell = true
				cellReference.Reset()
				cellType.Reset()
				cellValue.Reset()
				inlineValue.Reset()
				for _, attribute := range value.Attr {
					switch attribute.Name.Local {
					case "r":
						cellReference.WriteString(attribute.Value)
					case "t":
						cellType.WriteString(attribute.Value)
					}
				}
			case "v":
				if inCell {
					inValue = true
				}
			case "t":
				if inCell {
					inInline = true
				}
			}
		case xml.CharData:
			if inValue {
				cellValue.Write(value)
			} else if inInline {
				inlineValue.Write(value)
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "v":
				inValue = false
			case "t":
				inInline = false
			case "c":
				if !inCell {
					continue
				}
				content := cellValue.String()
				if cellType.String() == "s" {
					index, err := strconv.Atoi(strings.TrimSpace(content))
					if err != nil || index < 0 || index >= len(shared) {
						return nil, fmt.Errorf("invalid shared string reference %q", content)
					}
					content = shared[index]
				} else if cellType.String() == "inlineStr" {
					content = inlineValue.String()
				}
				if strings.TrimSpace(content) != "" {
					values = append(values, cellReference.String()+"="+content)
				}
				inCell = false
				if len(values) > 200_000 {
					return nil, errors.New("spreadsheet contains too many non-empty cells")
				}
			}
		}
	}
	return values, nil
}

func ownedPresentationText(ctx context.Context, filePath string) (string, error) {
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("open presentation archive: %w", err)
	}
	defer archive.Close()
	slides := make([]*zip.File, 0)
	for _, entry := range archive.File {
		if strings.HasPrefix(entry.Name, "ppt/slides/slide") && strings.HasSuffix(entry.Name, ".xml") {
			slides = append(slides, entry)
		}
	}
	sort.Slice(slides, func(left, right int) bool {
		return slides[left].Name < slides[right].Name
	})
	if len(slides) == 0 {
		return "", errors.New("presentation contains no slides")
	}
	var result strings.Builder
	for index, slide := range slides {
		if index >= 128 {
			break
		}
		text, err := readSlideText(ctx, slide)
		if err != nil {
			return "", fmt.Errorf("read presentation slide %s: %w", slide.Name, err)
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		result.WriteString(fmt.Sprintf("【第 %d 页】\n%s\n\n", index+1, text))
		if result.Len() > 1<<20 {
			return "", errors.New("presentation extracted text exceeds 1 MiB")
		}
	}
	if strings.TrimSpace(result.String()) == "" {
		return "", errors.New("presentation contains no readable text")
	}
	return result.String(), nil
}

func readSlideText(ctx context.Context, entry *zip.File) (string, error) {
	if entry.UncompressedSize64 > 8<<20 {
		return "", errors.New("presentation slide exceeds the parsing limit")
	}
	reader, err := entry.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()
	decoder := xml.NewDecoder(&ownedContextReader{ctx: ctx, reader: reader})
	var result strings.Builder
	inText := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parse presentation slide XML: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "t" {
				inText = true
			}
		case xml.CharData:
			if inText {
				result.Write(value)
			}
		case xml.EndElement:
			if value.Name.Local == "t" {
				inText = false
				result.WriteByte('\n')
			}
		}
		if result.Len() > 256<<10 {
			return "", errors.New("presentation slide text exceeds 256 KiB")
		}
	}
	return result.String(), nil
}

func ownedPDFText(ctx context.Context, filePath string) (string, error) {
	return ownedPDFTextWithCancel(ctx, filePath)
}

func ownedPDFTextWithCancel(ctx context.Context, filePath string) (text string, returnErr error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			returnErr = fmt.Errorf("parse PDF text: recovered panic: %v", recovered)
		}
	}()
	file, reader, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open PDF: %w", err)
	}
	defer file.Close()
	plain, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extract PDF text: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(&ownedContextReader{ctx: ctx, reader: plain}, (1<<20)+1))
	if err != nil {
		return "", fmt.Errorf("read PDF text: %w", err)
	}
	if len(data) > 1<<20 || !utf8.Valid(data) {
		return "", errors.New("PDF extracted text exceeds the limit or is not valid UTF-8")
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", ErrPDFTextUnavailable
	}
	return string(data), ctx.Err()
}

func AnalyzeAttachmentImage(ctx context.Context, filePath, mimeType string) (string, error) {
	cfg, err := getVisionModelConfig()
	if err != nil {
		return "", err
	}
	if cfg == nil {
		return "", errors.New("no active vision model is configured")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 20<<20+1))
	if err != nil {
		return "", err
	}
	if len(data) > 20<<20 {
		return "", errors.New("image exceeds the 20 MiB vision processing limit")
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return vision.GenerateImages(ctx, cfg, []string{"data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)}, "只描述图片中实际可见的内容，包括文字、表格、场景和物体。无法确认的内容必须说明无法确认。", 0)
}

func RenderPDFPages(ctx context.Context, filePath, outputDir string, maxPages int) ([]string, error) {
	tool, err := exec.LookPath("pdftoppm")
	if err != nil {
		return nil, fmt.Errorf("locate pdftoppm: %w", err)
	}
	if maxPages < 1 {
		maxPages = 1
	}
	if maxPages > 32 {
		maxPages = 32
	}
	prefix := filepath.Join(outputDir, "page")
	command := exec.CommandContext(ctx, tool, "-f", "1", "-l", strconv.Itoa(maxPages), "-r", "144", "-png", filePath, prefix)
	var stderr ownedTextBuffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("pdftoppm failed: %w; stderr=%s", err, string(stderr.Bytes()))
	}
	paths, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errors.New("pdftoppm produced no page images")
	}
	return paths, nil
}

func ExtractPDFImages(ctx context.Context, filePath, outputDir string, maxPages int) ([]string, error) {
	if maxPages < 1 {
		maxPages = 1
	}
	if maxPages > 32 {
		maxPages = 32
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open PDF for image extraction: %w", err)
	}
	defer file.Close()
	conf := pdfmodel.NewDefaultConfiguration()
	images, err := pdfapi.ExtractImagesRaw(ctx, file, []string{fmt.Sprintf("1-%d", maxPages)}, conf)
	if err != nil && len(images) == 0 {
		return nil, fmt.Errorf("extract PDF page images: %w", err)
	}
	paths := make([]string, 0, len(images))
	for pageIndex, pageImages := range images {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var selected *pdfmodel.Image
		for objectNumber := range pageImages {
			image := pageImages[objectNumber]
			if image.Thumb || image.Reader == nil || image.Size <= 0 {
				continue
			}
			if selected == nil || image.Size > selected.Size {
				candidate := image
				selected = &candidate
			}
		}
		if selected == nil {
			continue
		}
		extension := ""
		switch strings.ToLower(selected.FileType) {
		case "jpg", "jpeg":
			extension = ".jpg"
		case "png":
			extension = ".png"
		default:
			continue
		}
		if selected.Size > 20<<20 {
			return nil, fmt.Errorf("PDF page %d image exceeds the 20 MiB vision processing limit: %d bytes", pageIndex+1, selected.Size)
		}
		path := filepath.Join(outputDir, fmt.Sprintf("page-%03d%s", pageIndex+1, extension))
		output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, fmt.Errorf("create PDF page image %s: %w", path, err)
		}
		_, copyErr := io.Copy(output, io.LimitReader(&ownedContextReader{ctx: ctx, reader: selected.Reader}, (20<<20)+1))
		closeErr := output.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return nil, fmt.Errorf("write PDF page image %s: %w", path, err)
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("PDF contains no extractable page images for visual processing")
	}
	sort.Strings(paths)
	return paths, nil
}

type ownedTextBuffer struct{ bytes.Buffer }

func (b *ownedTextBuffer) Write(data []byte) (int, error) {
	if b.Len()+len(data) > 1<<20 {
		return 0, errors.New("提取文本超过 1 MiB")
	}
	return b.Buffer.Write(data)
}
