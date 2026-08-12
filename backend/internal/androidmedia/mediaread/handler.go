package mediaread

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/u-ai/backend/pkg/resourceuri"
)

type Handler struct {
	policy   Policy
	decoder  *ImageDecoder
	reader   ResourceReader
	mu       sync.Mutex
}

func NewHandler(policy Policy, resolver *resourceuri.PhysicalResolver) *Handler {
	r := NewPhysicalResourceReader(policy, resolver)
	d := NewImageDecoder(policy)
	return &Handler{
		policy:  policy,
		decoder: d,
		reader:  r,
	}
}

func NewHandlerWithReader(policy Policy, reader ResourceReader) *Handler {
	d := NewImageDecoder(policy)
	return &Handler{
		policy:  policy,
		decoder: d,
		reader:  reader,
	}
}

func (h *Handler) Info(ctx context.Context, uri string) (ImageInfo, error) {
	if uri == "" {
		return ImageInfo{}, &MediaReadError{Code: MediaReadInvalidURI, Message: "empty resource URI"}
	}

	ctx, cancel := context.WithTimeout(ctx, h.policy.MaxDecodeTime)
	defer cancel()

	rc, res, err := h.reader.Read(ctx, uri)
	if err != nil {
		return ImageInfo{}, err
	}
	defer rc.Close()

	info, err := h.decoder.Inspect(ctx, rc, res.MIMEType, res.SizeBytes)
	if err != nil {
		return ImageInfo{}, err
	}

	info.ResourceURI = uri
	info.Source = res.Source

	return info, nil
}

func (h *Handler) Image(ctx context.Context, uri string, opts DecodeOptions) (NormalizedImage, error) {
	if uri == "" {
		return NormalizedImage{}, &MediaReadError{Code: MediaReadInvalidURI, Message: "empty resource URI"}
	}

	ctx, cancel := context.WithTimeout(ctx, h.policy.MaxDecodeTime)
	defer cancel()

	rc, res, err := h.reader.Read(ctx, uri)
	if err != nil {
		return NormalizedImage{}, err
	}
	defer rc.Close()

	info, err := h.decoder.Inspect(ctx, rc, res.MIMEType, res.SizeBytes)
	if err != nil {
		return NormalizedImage{}, err
	}

	info.ResourceURI = uri
	info.Source = res.Source

	normalizeOrientation := h.policy.EffectiveNormalizeOrientation(&opts.NormalizeOrientation)
	stripMetadata := h.policy.EffectiveStripMetadata(&opts.StripMetadata)

	needsNormalize := false
	if normalizeOrientation && info.Orientation != 0 {
		needsNormalize = true
	}
	if stripMetadata {
		needsNormalize = true
	}

	if !needsNormalize {
		return NormalizedImage{
			ResourceURI: uri,
			MIMEType:    info.MIMEType,
			Width:       info.Width,
			Height:      info.Height,
			SizeBytes:   info.SizeBytes,
			Normalized:  false,
			SourceURI:   uri,
		}, nil
	}

	norm := NewImageNormalizer(h.policy, h.decoder)

	img, err := norm.DecodeFull(ctx, rc, info)
	if err != nil {
		return NormalizedImage{}, err
	}

	requestID := generateRequestID()
	destPath := h.tempPath(requestID, info.Format)

	sizeBytes, err := norm.EncodeToTemp(ctx, img, info.Format, destPath)
	if err != nil {
		return NormalizedImage{}, err
	}

	newURI := norm.ResourceURIFromTemp(requestID, info.Format)

	return NormalizedImage{
		ResourceURI: newURI,
		MIMEType:    info.MIMEType,
		Width:       info.Width,
		Height:      info.Height,
		SizeBytes:   sizeBytes,
		Normalized:  true,
		SourceURI:   uri,
	}, nil
}

func (h *Handler) ResolveImageInput(ctx context.Context, uri string) (ImageInput, error) {
	if uri == "" {
		return ImageInput{}, &MediaReadError{Code: MediaReadInvalidURI, Message: "empty resource URI"}
	}

	ctx, cancel := context.WithTimeout(ctx, h.policy.MaxDecodeTime)
	defer cancel()

	rc, res, err := h.reader.Read(ctx, uri)
	if err != nil {
		return ImageInput{}, err
	}
	defer rc.Close()

	data, err := readLimited(rc, h.policy.MaxInputBytes)
	if err != nil {
		return ImageInput{}, err
	}

	mimeType := DetectMIMEFromBytes(data)
	if mimeType == "application/octet-stream" {
		mimeType = res.MIMEType
	}

	format := MIMEToFormat(mimeType)
	if format == "" {
		return ImageInput{}, &MediaReadError{Code: MediaReadUnsupportedFormat, Message: "unsupported image format: " + mimeType}
	}

	return ImageInput{
		ResourceURI: uri,
		MIMEType:    mimeType,
		Format:      format,
		Bytes:       data,
		SizeBytes:   int64(len(data)),
	}, nil
}

func (h *Handler) tempPath(requestID, format string) string {
	ext := FormatToExt(format)
	return "resources/temp/android-media/mediaread/" + requestID + ext
}

func (h *Handler) Close() error {
	return nil
}

type ImageInput struct {
	ResourceURI string
	MIMEType    string
	Format      string
	Bytes       []byte
	SizeBytes   int64
}

func (i ImageInput) IsValid() bool {
	return i.ResourceURI != "" && i.MIMEType != "" && len(i.Bytes) > 0
}

func (i ImageInput) ToReader() io.Reader {
	return bytes.NewReader(i.Bytes)
}

func generateRequestID() string {
	return fmt.Sprintf("mr-%d", time.Now().UnixNano())
}
