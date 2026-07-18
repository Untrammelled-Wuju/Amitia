package emote

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/config"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	MaxFileSize        = int64(10 * 1024 * 1024)
	MaxDimension       = 4096
	MaxPixels          = 16 * 1024 * 1024
	MaxAnimationFrames = 200
	MaxAnimationMS     = 30000
)

type FileError struct {
	Code    string
	Message string
}

func (e *FileError) Error() string { return e.Message }

type ProcessedAsset struct {
	Hash          string
	Mime          string
	Ext           string
	Size          int64
	Width         int
	Height        int
	Animated      bool
	FrameCount    int
	DurationMS    int
	FilePath      string
	ThumbnailPath string
	FallbackPath  string
	CreatedFiles  []string
}

func ProcessUpload(header *multipart.FileHeader, id string) (*ProcessedAsset, error) {
	if header == nil {
		return nil, &FileError{Code: "invalid_image", Message: "缺少图片文件"}
	}
	if header.Size > MaxFileSize {
		return nil, &FileError{Code: "file_too_large", Message: "文件超过 10MB 限制"}
	}
	file, err := header.Open()
	if err != nil {
		return nil, &FileError{Code: "invalid_image", Message: "无法读取文件"}
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxFileSize+1))
	if err != nil || int64(len(data)) > MaxFileSize {
		return nil, &FileError{Code: "file_too_large", Message: "文件超过 10MB 限制"}
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	mime, canonicalExt, err := detectImage(data)
	if err != nil {
		return nil, err
	}
	if !extensionMatches(ext, canonicalExt) {
		return nil, &FileError{Code: "unsupported_format", Message: "文件扩展名与支持格式不符"}
	}
	declared := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	if declared != "" && !mimeMatches(declared, mime) {
		return nil, &FileError{Code: "mime_mismatch", Message: "文件 MIME 类型与实际内容不符"}
	}
	decoded, width, height, animated, frames, duration, err := decodeAsset(data, mime)
	if err != nil {
		return nil, &FileError{Code: "invalid_image", Message: "图片解码失败"}
	}
	if width < 1 || height < 1 || width > MaxDimension || height > MaxDimension || width*height > MaxPixels {
		return nil, &FileError{Code: "image_dimensions_exceeded", Message: "图片尺寸超出限制"}
	}
	if frames > MaxAnimationFrames {
		return nil, &FileError{Code: "too_many_animation_frames", Message: "动图帧数超过限制"}
	}
	if duration > MaxAnimationMS {
		return nil, &FileError{Code: "animation_duration_exceeded", Message: "动图时长超过限制"}
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	baseDir := filepath.Join(config.AppCfg.Storage.DataDir, "emotes", id)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	originalName := "original" + canonicalExt
	originalDisk := filepath.Join(baseDir, originalName)
	thumbDisk := filepath.Join(baseDir, "thumbnail.png")
	fallbackDisk := filepath.Join(baseDir, "fallback.png")
	created := []string{}
	cleanup := func() {
		for _, path := range created {
			_ = os.Remove(path)
		}
		_ = os.Remove(baseDir)
	}
	if err := os.WriteFile(originalDisk, data, 0644); err != nil {
		cleanup()
		return nil, err
	}
	created = append(created, originalDisk)
	if err := writePNG(thumbDisk, fitImage(decoded, 160, 160)); err != nil {
		cleanup()
		return nil, err
	}
	created = append(created, thumbDisk)
	if err := writePNG(fallbackDisk, fitImage(decoded, 1024, 1024)); err != nil {
		cleanup()
		return nil, err
	}
	created = append(created, fallbackDisk)
	prefix := "/emote-assets/" + id + "/"
	return &ProcessedAsset{Hash: hash, Mime: mime, Ext: canonicalExt, Size: int64(len(data)), Width: width, Height: height, Animated: animated, FrameCount: frames, DurationMS: duration, FilePath: prefix + originalName, ThumbnailPath: prefix + "thumbnail.png", FallbackPath: prefix + "fallback.png", CreatedFiles: created}, nil
}

func detectImage(data []byte) (string, string, error) {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return "image/png", ".png", nil
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg", ".jpg", nil
	}
	if len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))) {
		return "image/gif", ".gif", nil
	}
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp", ".webp", nil
	}
	return "", "", &FileError{Code: "unsupported_format", Message: "仅支持 PNG、JPEG、GIF 和 WebP"}
}

func extensionMatches(ext, canonical string) bool {
	if canonical == ".jpg" {
		return ext == ".jpg" || ext == ".jpeg"
	}
	return ext == canonical
}

func mimeMatches(declared, actual string) bool {
	declared = strings.Split(declared, ";")[0]
	if actual == "image/jpeg" {
		return declared == "image/jpeg" || declared == "image/jpg"
	}
	return declared == actual
}

func decodeAsset(data []byte, mime string) (image.Image, int, int, bool, int, int, error) {
	switch mime {
	case "image/png":
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, 0, 0, false, 0, 0, err
		}
		b := img.Bounds()
		return img, b.Dx(), b.Dy(), false, 1, 0, nil
	case "image/jpeg":
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, 0, 0, false, 0, 0, err
		}
		b := img.Bounds()
		return img, b.Dx(), b.Dy(), false, 1, 0, nil
	case "image/gif":
		all, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil || len(all.Image) == 0 {
			return nil, 0, 0, false, 0, 0, errors.New("invalid gif")
		}
		duration := 0
		for _, delay := range all.Delay {
			duration += delay * 10
		}
		b := all.Image[0].Bounds()
		return all.Image[0], b.Dx(), b.Dy(), len(all.Image) > 1, len(all.Image), duration, nil
	case "image/webp":
		width, height, animated, frames, duration, err := parseWebP(data)
		if err != nil {
			return nil, 0, 0, false, 0, 0, err
		}
		img, decodeErr := webp.Decode(bytes.NewReader(data))
		if decodeErr != nil && animated {
			img, decodeErr = decodeFirstWebPFrame(data)
		}
		if decodeErr != nil {
			return nil, 0, 0, false, 0, 0, decodeErr
		}
		return img, width, height, animated, frames, duration, nil
	}
	return nil, 0, 0, false, 0, 0, errors.New("unsupported")
}

func parseWebP(data []byte) (int, int, bool, int, int, error) {
	if len(data) < 20 {
		return 0, 0, false, 0, 0, errors.New("short webp")
	}
	width, height, frames, duration := 0, 0, 0, 0
	animated := false
	for pos := 12; pos+8 <= len(data); {
		kind := string(data[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		start := pos + 8
		end := start + size
		if end > len(data) || size < 0 {
			return 0, 0, false, 0, 0, errors.New("bad webp chunk")
		}
		switch kind {
		case "VP8X":
			if size >= 10 {
				animated = data[start]&2 != 0
				width = 1 + int(data[start+4]) + int(data[start+5])<<8 + int(data[start+6])<<16
				height = 1 + int(data[start+7]) + int(data[start+8])<<8 + int(data[start+9])<<16
			}
		case "ANMF":
			if size >= 16 {
				frames++
				duration += int(data[start+12]) + int(data[start+13])<<8 + int(data[start+14])<<16
			}
		}
		pos = end + size%2
	}
	if frames == 0 {
		frames = 1
	}
	if width == 0 || height == 0 {
		cfg, err := webp.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return 0, 0, false, 0, 0, err
		}
		width, height = cfg.Width, cfg.Height
	}
	return width, height, animated, frames, duration, nil
}

func decodeFirstWebPFrame(data []byte) (image.Image, error) {
	for pos := 12; pos+8 <= len(data); {
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		start := pos + 8
		end := start + size
		if end > len(data) {
			break
		}
		if string(data[pos:pos+4]) == "ANMF" && size > 16 {
			chunks := data[start+16 : end]
			riffSize := 4 + len(chunks)
			buf := bytes.NewBuffer(nil)
			buf.WriteString("RIFF")
			_ = binary.Write(buf, binary.LittleEndian, uint32(riffSize))
			buf.WriteString("WEBP")
			buf.Write(chunks)
			return webp.Decode(bytes.NewReader(buf.Bytes()))
		}
		pos = end + size%2
	}
	return nil, errors.New("webp first frame unavailable")
}

func fitImage(src image.Image, maxWidth, maxHeight int) image.Image {
	b := src.Bounds()
	width, height := b.Dx(), b.Dy()
	if width <= maxWidth && height <= maxHeight {
		return src
	}
	ratio := float64(maxWidth) / float64(width)
	if hRatio := float64(maxHeight) / float64(height); hRatio < ratio {
		ratio = hRatio
	}
	dst := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(width)*ratio)), max(1, int(float64(height)*ratio))))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(file, img)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func removeAssetDirectory(item *Emote) error {
	if item == nil || item.ID == "" {
		return nil
	}
	path := filepath.Join(config.AppCfg.Storage.DataDir, "emotes", item.ID)
	absBase, _ := filepath.Abs(filepath.Join(config.AppCfg.Storage.DataDir, "emotes"))
	absPath, _ := filepath.Abs(path)
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) {
		return fmt.Errorf("invalid asset path")
	}
	return os.RemoveAll(absPath)
}
