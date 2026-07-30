package decode

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"
)

var (
	ErrUnsafePath        = errors.New("decode: unsafe path")
	ErrNotRegularFile    = errors.New("decode: not a regular file")
	ErrFileTooLarge      = errors.New("decode: file too large")
	ErrUnsupportedFormat = errors.New("decode: unsupported format")
	ErrMIMEMismatch      = errors.New("decode: MIME mismatch")
	ErrMIMENotAllowed    = errors.New("decode: MIME not allowed")
	ErrDimensionExceeded = errors.New("decode: dimension exceeded")
	ErrPixelsExceeded    = errors.New("decode: pixels exceeded")
	ErrHashMismatch      = errors.New("decode: hash mismatch")
)

type DecodeRequest struct {
	AbsolutePath string
	ExpectedHash string
	ExpectedMIME string
	MaxBytes     int64
	MaxPixels    int64
	MaxDimension int
	AllowedMIMEs []string
}

type DecodedImage struct {
	Image              *image.NRGBA
	OriginalWidth      int
	OriginalHeight     int
	NormalizedWidth    int
	NormalizedHeight   int
	MIMEType           string
	OrientationApplied bool
	ColorProfile       string
	SourceHash         string
	PixelHash          string
}

func Decode(req DecodeRequest) (*DecodedImage, error) {
	if !isPathSafe(req.AbsolutePath) {
		return nil, fmt.Errorf("%w: %s", ErrUnsafePath, req.AbsolutePath)
	}

	info, err := os.Stat(req.AbsolutePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s", ErrNotRegularFile, req.AbsolutePath)
	}
	if req.MaxBytes > 0 && info.Size() > req.MaxBytes {
		return nil, fmt.Errorf("%w: size=%d max=%d", ErrFileTooLarge, info.Size(), req.MaxBytes)
	}

	data, err := os.ReadFile(req.AbsolutePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	mime := detectMIME(data)
	if mime == "" {
		return nil, fmt.Errorf("%w: path=%s", ErrUnsupportedFormat, req.AbsolutePath)
	}

	if req.ExpectedMIME != "" && mime != req.ExpectedMIME {
		return nil, fmt.Errorf("%w: expected=%s actual=%s", ErrMIMEMismatch, req.ExpectedMIME, mime)
	}
	if len(req.AllowedMIMEs) > 0 && !isAllowedMIME(mime, req.AllowedMIMEs) {
		return nil, fmt.Errorf("%w: mime=%s", ErrMIMENotAllowed, mime)
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if req.MaxDimension > 0 && (config.Width > req.MaxDimension || config.Height > req.MaxDimension) {
		return nil, fmt.Errorf("%w: %dx%d max=%d", ErrDimensionExceeded, config.Width, config.Height, req.MaxDimension)
	}
	if req.MaxPixels > 0 && int64(config.Width)*int64(config.Height) > req.MaxPixels {
		return nil, fmt.Errorf("%w: pixels=%d max=%d", ErrPixelsExceeded, int64(config.Width)*int64(config.Height), req.MaxPixels)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	nrgbaImg := toNRGBA(img)

	bounds := nrgbaImg.Bounds()
	if bounds.Min.X != 0 || bounds.Min.Y != 0 {
		shifted := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				shifted.SetNRGBA(x-bounds.Min.X, y-bounds.Min.Y, nrgbaImg.NRGBAAt(x, y))
			}
		}
		nrgbaImg = shifted
	}

	sourceHash := computeHashFromBytes(data)
	if req.ExpectedHash != "" && sourceHash != req.ExpectedHash {
		return nil, fmt.Errorf("%w: expected=%s actual=%s", ErrHashMismatch, req.ExpectedHash, sourceHash)
	}

	pixelHash := computePixelHash(nrgbaImg)

	finalBounds := nrgbaImg.Bounds()
	return &DecodedImage{
		Image:              nrgbaImg,
		OriginalWidth:      config.Width,
		OriginalHeight:     config.Height,
		NormalizedWidth:    finalBounds.Dx(),
		NormalizedHeight:   finalBounds.Dy(),
		MIMEType:           mime,
		OrientationApplied: false,
		ColorProfile:       "srgb",
		SourceHash:         sourceHash,
		PixelHash:          pixelHash,
	}, nil
}

func detectMIME(data []byte) string {
	if len(data) >= 8 && bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp"
	}
	return ""
}

func isPathSafe(path string) bool {
	if path == "" {
		return false
	}
	if !filepath.IsAbs(path) {
		return false
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if part == ".." {
			return false
		}
	}
	return true
}

func toNRGBA(img image.Image) *image.NRGBA {
	if nrgba, ok := img.(*image.NRGBA); ok {
		bounds := nrgba.Bounds()
		if bounds.Min.X == 0 && bounds.Min.Y == 0 {
			return nrgba
		}
		dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				dst.SetNRGBA(x-bounds.Min.X, y-bounds.Min.Y, nrgba.NRGBAAt(x, y))
			}
		}
		return dst
	}

	if rgba, ok := img.(*image.RGBA); ok {
		return UnpremultiplyRGBA(rgba)
	}

	bounds := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			dst.SetNRGBA(x-bounds.Min.X, y-bounds.Min.Y, c)
		}
	}
	return dst
}

func computePixelHash(img *image.NRGBA) string {
	h := sha256.New()
	bounds := img.Bounds()
	w := bounds.Dx()
	hCount := bounds.Dy()

	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(w))
	h.Write(buf[:])
	binary.BigEndian.PutUint32(buf[:], uint32(hCount))
	h.Write(buf[:])

	rowSize := w * 4
	for y := 0; y < hCount; y++ {
		start := (bounds.Min.Y+y)*img.Stride + bounds.Min.X*4
		h.Write(img.Pix[start : start+rowSize])
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

func computeFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return computeHashFromBytes(data), nil
}

func computeHashFromBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func isAllowedMIME(mime string, allowed []string) bool {
	for _, m := range allowed {
		if m == mime {
			return true
		}
	}
	return false
}
