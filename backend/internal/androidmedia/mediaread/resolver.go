package mediaread

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/pkg/resourceuri"
)

type ResourceReader interface {
	Read(ctx context.Context, uri string) (io.ReadCloser, ResolvedResource, error)
}

type PhysicalResourceReader struct {
	policy    Policy
	resolver  *resourceuri.PhysicalResolver
}

func NewPhysicalResourceReader(policy Policy, resolver *resourceuri.PhysicalResolver) *PhysicalResourceReader {
	return &PhysicalResourceReader{
		policy:   policy,
		resolver: resolver,
	}
}

func (r *PhysicalResourceReader) Read(ctx context.Context, uriStr string) (io.ReadCloser, ResolvedResource, error) {
	select {
	case <-ctx.Done():
		return nil, ResolvedResource{}, &MediaReadError{Code: MediaReadCancelled, Message: "context cancelled"}
	default:
	}

	uri, err := resourceuri.Parse(uriStr)
	if err != nil {
		return nil, ResolvedResource{}, &MediaReadError{Code: MediaReadInvalidURI, Message: fmt.Sprintf("invalid resource URI: %v", err)}
	}

	var localPath string
	if r.resolver != nil {
		resolved, err := r.resolver.Resolve(uri)
		if err != nil && !isNonFilesystem(err) {
			return nil, ResolvedResource{}, &MediaReadError{Code: MediaReadInvalidURI, Message: fmt.Sprintf("cannot resolve resource: %v", err)}
		}
		localPath = resolved.LocalPath
	}

	if localPath == "" {
		return nil, ResolvedResource{}, &MediaReadError{Code: MediaReadResourceNotFound, Message: "cannot resolve local path"}
	}

	info, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ResolvedResource{}, &MediaReadError{Code: MediaReadResourceNotFound, Message: "resource not found: " + uriStr}
		}
		return nil, ResolvedResource{}, &MediaReadError{Code: MediaReadPermissionDenied, Message: fmt.Sprintf("cannot access resource: %v", err)}
	}

	if info.Size() > r.policy.MaxInputBytes {
		return nil, ResolvedResource{}, &MediaReadError{
			Code:    MediaReadTooLarge,
			Message: fmt.Sprintf("resource size %d exceeds limit %d", info.Size(), r.policy.MaxInputBytes),
		}
	}

	file, err := os.Open(localPath)
	if err != nil {
		return nil, ResolvedResource{}, &MediaReadError{Code: MediaReadPermissionDenied, Message: fmt.Sprintf("cannot open resource: %v", err)}
	}

	mimeType := DetectMIMEFromPath(localPath)

	res := ResolvedResource{
		URI:       uriStr,
		LocalPath: localPath,
		MIMEType:  mimeType,
		SizeBytes: info.Size(),
		Source:    classifySource(string(uri.Root()), uri.RelativePath()),
	}

	return file, res, nil
}

func isNonFilesystem(err error) bool {
	return err == resourceuri.ErrNonFilesystemResource
}

func classifySource(root, relPath string) string {
	switch root {
	case "workspace":
		return SourceWorkspace
	case "attachments":
		return SourceAttachment
	case "temp":
		if strings.HasPrefix(relPath, "android-media/camera/") {
			return SourceCamera
		}
		return SourceTemp
	case "cache":
		return SourceCache
	case "data":
		return SourceUnknown
	}
	return SourceUnknown
}

func DetectMIMEFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if format := ExtToFormat(ext); format != "" {
		return FormatToMIME(format)
	}
	return "application/octet-stream"
}

func DetectMIMEFromBytes(data []byte) string {
	if len(data) < 4 {
		return "application/octet-stream"
	}
	mimeType := http.DetectContentType(data)
	if mimeType != "application/octet-stream" {
		return mimeType
	}
	if isPNGSignature(data) {
		return "image/png"
	}
	if isJPEGSignature(data) {
		return "image/jpeg"
	}
	if isWebPSignature(data) {
		return "image/webp"
	}
	if isGIFSignature(data) {
		return "image/gif"
	}
	if isBMPSignature(data) {
		return "image/bmp"
	}
	return "application/octet-stream"
}

func MIMEToFormat(mime string) string {
	switch mime {
	case "image/jpeg":
		return FormatJPEG
	case "image/png":
		return FormatPNG
	case "image/webp":
		return FormatWebP
	case "image/gif":
		return FormatGIF
	case "image/bmp":
		return FormatBMP
	case "image/heic":
		return FormatHEIC
	case "image/heif":
		return FormatHEIF
	}
	return ""
}

func isPNGSignature(data []byte) bool {
	return len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A
}

func isJPEGSignature(data []byte) bool {
	return len(data) > 2 && data[0] == 0xFF && data[1] == 0xD8
}

func isWebPSignature(data []byte) bool {
	return len(data) >= 12 &&
		data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
		data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P'
}

func isGIFSignature(data []byte) bool {
	return len(data) > 5 &&
		data[0] == 'G' && data[1] == 'I' && data[2] == 'F' && data[3] == '8' &&
		(data[4] == '7' || data[4] == '9') && data[5] == 'a'
}

func isBMPSignature(data []byte) bool {
	return len(data) > 2 && data[0] == 'B' && data[1] == 'M'
}
