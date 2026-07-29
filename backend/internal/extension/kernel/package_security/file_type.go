package package_security

import (
	"bytes"
	"net/http"
	"path"
	"strings"
)

type FileTypeDetector struct{}

func NewFileTypeDetector() *FileTypeDetector {
	return &FileTypeDetector{}
}

type FileTypeResult struct {
	Extension    string
	MIMEType     string
	MagicNumber  string
	IsArchive    bool
	IsExecutable bool
	IsText       bool
	Warnings     []string
}

var (
	archiveMagicNumbers = map[string]string{
		"PK\x03\x04":           "ZIP",
		"PK\x05\x06":           "ZIP(empty)",
		"\x1f\x8b":             "GZIP",
		"BZh":                  "BZIP2",
		"\xFD7zXZ\x00":         "XZ",
		"Rar!\x1a\x07":         "RAR",
		"\x75\x73\x74\x61\x72": "TAR",
	}

	executableMagicNumbers = map[string]string{
		"MZ":               "PE",
		"\x7fELF":          "ELF",
		"\xca\xfe\xba\xbe": "Mach-O Universal",
		"\xcf\xfa\xed\xfe": "Mach-O 64-bit",
		"\xce\xfa\xed\xfe": "Mach-O 32-bit",
	}
)

func (d *FileTypeDetector) Detect(raw []byte, declaredExtension string) FileTypeResult {
	result := FileTypeResult{
		Extension: strings.ToLower(path.Ext(declaredExtension)),
	}

	mime := http.DetectContentType(raw)
	result.MIMEType = mime

	result.MagicNumber = detectMagic(raw)

	if archiveMagic := findMagicPrefix(raw, archiveMagicNumbers); archiveMagic != "" {
		result.IsArchive = true
		if result.Extension != ".zip" && result.Extension != ".amitiax" {
			result.Warnings = append(result.Warnings, "archive magic without expected extension")
		}
	}

	if execMagic := findMagicPrefix(raw, executableMagicNumbers); execMagic != "" {
		result.IsExecutable = true
		result.Warnings = append(result.Warnings, "executable binary detected")
	}

	if strings.HasPrefix(mime, "text/") || strings.Contains(mime, "json") || strings.Contains(mime, "xml") {
		result.IsText = true
	}

	if result.Extension == ".amitiax" && !result.IsArchive {
		result.Warnings = append(result.Warnings, ".amitiax extension without valid archive magic")
	}

	return result
}

func detectMagic(raw []byte) string {
	if len(raw) < 4 {
		return ""
	}
	for magic, name := range archiveMagicNumbers {
		if bytes.HasPrefix(raw, []byte(magic)) {
			return name
		}
	}
	for magic, name := range executableMagicNumbers {
		if bytes.HasPrefix(raw, []byte(magic)) {
			return name
		}
	}
	return ""
}

func findMagicPrefix(raw []byte, magics map[string]string) string {
	for magic := range magics {
		if bytes.HasPrefix(raw, []byte(magic)) {
			return magic
		}
	}
	return ""
}
