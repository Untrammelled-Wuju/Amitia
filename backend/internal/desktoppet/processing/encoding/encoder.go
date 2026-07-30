package encoding

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

type FrameEncoder interface {
	Encode(img *image.NRGBA, path string) (FileResult, error)
	EncodeMask(mask *image.Gray, path string) (FileResult, error)
	SupportedFormat() string
}

type FileResult struct {
	RelativePath string
	AbsolutePath string
	Width        int
	Height       int
	ByteSize     int64
	FileHash     string
	PixelHash    string
	MimeType     string
}

type PNGEncoder struct {
	CompressionLevel int
}

func NewPNGEncoder(compressionLevel int) *PNGEncoder {
	return &PNGEncoder{CompressionLevel: compressionLevel}
}

func (e *PNGEncoder) Encode(img *image.NRGBA, path string) (FileResult, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return FileResult{}, err
	}
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return FileResult{}, err
	}

	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.CompressionLevel(e.CompressionLevel)}
	if err := encoder.Encode(&buf, img); err != nil {
		return FileResult{}, err
	}

	fileBytes := buf.Bytes()
	fileHash := sha256.Sum256(fileBytes)
	pixelHash := computeNRGBAPixelHash(img)

	if err := os.WriteFile(absPath, fileBytes, 0o644); err != nil {
		return FileResult{}, err
	}

	return FileResult{
		RelativePath: path,
		AbsolutePath: absPath,
		Width:        img.Rect.Dx(),
		Height:       img.Rect.Dy(),
		ByteSize:     int64(len(fileBytes)),
		FileHash:     hex.EncodeToString(fileHash[:]),
		PixelHash:    pixelHash,
		MimeType:     "image/png",
	}, nil
}

func (e *PNGEncoder) EncodeMask(mask *image.Gray, path string) (FileResult, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return FileResult{}, err
	}
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return FileResult{}, err
	}

	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.CompressionLevel(e.CompressionLevel)}
	if err := encoder.Encode(&buf, mask); err != nil {
		return FileResult{}, err
	}

	fileBytes := buf.Bytes()
	fileHash := sha256.Sum256(fileBytes)
	pixelHash := computeGrayPixelHash(mask)

	if err := os.WriteFile(absPath, fileBytes, 0o644); err != nil {
		return FileResult{}, err
	}

	return FileResult{
		RelativePath: path,
		AbsolutePath: absPath,
		Width:        mask.Rect.Dx(),
		Height:       mask.Rect.Dy(),
		ByteSize:     int64(len(fileBytes)),
		FileHash:     hex.EncodeToString(fileHash[:]),
		PixelHash:    pixelHash,
		MimeType:     "image/png",
	}, nil
}

func (e *PNGEncoder) SupportedFormat() string {
	return "png"
}

func computeNRGBAPixelHash(img *image.NRGBA) string {
	h := sha256.New()
	width := img.Rect.Dx()
	height := img.Rect.Dy()
	var dimBuf [8]byte
	binary.LittleEndian.PutUint32(dimBuf[:4], uint32(width))
	binary.LittleEndian.PutUint32(dimBuf[4:], uint32(height))
	h.Write(dimBuf[:])
	h.Write(img.Pix)
	return hex.EncodeToString(h.Sum(nil))
}

func computeGrayPixelHash(img *image.Gray) string {
	h := sha256.New()
	width := img.Rect.Dx()
	height := img.Rect.Dy()
	var dimBuf [8]byte
	binary.LittleEndian.PutUint32(dimBuf[:4], uint32(width))
	binary.LittleEndian.PutUint32(dimBuf[4:], uint32(height))
	h.Write(dimBuf[:])
	h.Write(img.Pix)
	return hex.EncodeToString(h.Sum(nil))
}
