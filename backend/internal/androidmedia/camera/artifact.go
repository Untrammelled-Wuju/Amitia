package camera

import (
	"path/filepath"
	"strings"
	"time"
)

type ArtifactRecord struct {
	ResourceURI      string    `json:"resourceUri"`
	MIMEType         string    `json:"mimeType"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	SizeBytes        int64     `json:"sizeBytes"`
	ContentHash      string    `json:"contentHash,omitempty"`
	CameraID         string    `json:"cameraId"`
	LensFacing       string    `json:"lensFacing"`
	CaptureTimestamp time.Time `json:"captureTimestamp"`
	ExpiresAt        time.Time `json:"expiresAt"`
	EXIFStripped     bool      `json:"exifStripped"`
}

func (a ArtifactRecord) IsValid() bool {
	return a.ResourceURI != "" &&
		a.MIMEType != "" &&
		a.Width > 0 &&
		a.Height > 0 &&
		a.SizeBytes > 0
}

func (a ArtifactRecord) IsExpired(now time.Time) bool {
	return !a.ExpiresAt.IsZero() && now.After(a.ExpiresAt)
}

func ArtifactURI(requestID, ext string) string {
	name := SafeResourceName(requestID, ext)
	return "amitia://temp/android-media/camera/" + name
}

func SafeResourceName(requestID string, ext string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' {
			return r
		}
		return '_'
	}, requestID)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return safe + ext
}

func FormatToMIME(format string) string {
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	}
	return "application/octet-stream"
}

func FormatToExt(format string) string {
	switch format {
	case "jpeg":
		return ".jpg"
	case "png":
		return ".png"
	case "webp":
		return ".webp"
	}
	return ".bin"
}

func TempArtifactPath(dir, requestID, format string) string {
	ext := FormatToExt(format)
	return filepath.Join(dir, SafeResourceName("camera-"+requestID, ext))
}
