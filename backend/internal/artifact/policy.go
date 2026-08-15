package artifact

import (
	"path/filepath"
	"strings"
)

type UploadLimits struct {
	ImageMaxBytes int64
	AudioMaxBytes int64
	VideoMaxBytes int64
	FileMaxBytes  int64
}

var DefaultLimits = UploadLimits{
	ImageMaxBytes: 25 * 1024 * 1024,
	AudioMaxBytes: 100 * 1024 * 1024,
	VideoMaxBytes: 512 * 1024 * 1024,
	FileMaxBytes:  256 * 1024 * 1024,
}

func (l UploadLimits) MaxBytesForKind(kind Kind) int64 {
	switch kind {
	case KindImage:
		return l.ImageMaxBytes
	case KindAudio:
		return l.AudioMaxBytes
	case KindVideo:
		return l.VideoMaxBytes
	case KindFile, KindDocument, KindArchive, KindGenerated, KindToolOutput:
		return l.FileMaxBytes
	default:
		return l.FileMaxBytes
	}
}

func DeriveKindFromMIME(mime string) Kind {
	m := strings.ToLower(strings.TrimSpace(mime))
	switch {
	case strings.HasPrefix(m, "image/"):
		return KindImage
	case strings.HasPrefix(m, "audio/"):
		return KindAudio
	case strings.HasPrefix(m, "video/"):
		return KindVideo
	case m == "application/pdf", strings.Contains(m, "document"), m == "text/plain":
		return KindDocument
	case strings.Contains(m, "zip"), strings.Contains(m, "tar"), strings.Contains(m, "rar"), strings.Contains(m, "7z"):
		return KindArchive
	default:
		return KindFile
	}
}

func SanitizeFilename(name string) string {
	base := filepath.Base(name)
	var b []rune
	for _, r := range base {
		if r == 0 || r == '\n' || r == '\r' || r == '/' || r == '\\' {
			continue
		}
		b = append(b, r)
	}
	result := string(b)
	if len(result) > 255 {
		result = result[:255]
	}
	return strings.TrimSpace(result)
}

func ExtractExtension(name string) string {
	ext := filepath.Ext(name)
	if len(ext) > 10 {
		ext = ext[:10]
	}
	return strings.ToLower(ext)
}
