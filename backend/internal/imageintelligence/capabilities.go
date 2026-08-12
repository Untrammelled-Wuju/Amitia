package imageintelligence

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/vision"
)

type ImageDetailLevel string

const (
	DetailAuto ImageDetailLevel = "auto"
	DetailLow  ImageDetailLevel = "low"
	DetailHigh ImageDetailLevel = "high"
)

type ExecutionLocation string

const (
	LocationLocal ExecutionLocation = "local"
	LocationCloud ExecutionLocation = "cloud"
)

type ImageCapabilities struct {
	Understand         bool                `json:"understand"`
	OCR                bool                `json:"ocr"`
	Generate           bool                `json:"generate"`
	SupportedInputMIMEs []string           `json:"supportedInputMIMEs"`
	MaxInputBytes      int64               `json:"maxInputBytes"`
	MaxWidth           int                 `json:"maxWidth"`
	MaxHeight          int                 `json:"maxHeight"`
	MaxPixels          int64               `json:"maxPixels"`
	GenerationFormats  []string           `json:"generationFormats,omitempty"`
	SupportsLocalInput bool                `json:"supportsLocalInput"`
	SupportsRemoteModel bool               `json:"supportsRemoteModel"`
	Provider           string              `json:"provider,omitempty"`
	Model              string              `json:"model,omitempty"`
	ExecutionLocation  ExecutionLocation   `json:"executionLocation,omitempty"`
}

type CapabilityDetector struct {
	visionSvc  vision.Service
	providerRegistry *imageprovider.Registry
	mu         sync.RWMutex
	cached     *ImageCapabilities
	cachedAt   time.Time
	ttl        time.Duration
}

func NewCapabilityDetector(visionSvc vision.Service, providerRegistry *imageprovider.Registry) *CapabilityDetector {
	return &CapabilityDetector{
		visionSvc:        visionSvc,
		providerRegistry: providerRegistry,
		ttl:              30 * time.Second,
	}
}

func (d *CapabilityDetector) Detect(ctx context.Context) ImageCapabilities {
	d.mu.RLock()
	if d.cached != nil && time.Since(d.cachedAt) < d.ttl {
		cap := *d.cached
		d.mu.RUnlock()
		return cap
	}
	d.mu.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()

	cap := ImageCapabilities{
		SupportedInputMIMEs: []string{"image/jpeg", "image/png", "image/webp"},
		MaxInputBytes:       20 * 1024 * 1024,
		MaxWidth:            10000,
		MaxHeight:           10000,
		MaxPixels:           40000000,
		SupportsLocalInput:  true,
		SupportsRemoteModel: true,
		GenerationFormats:   []string{"image/png", "image/jpeg"},
	}

	if d.visionSvc != nil {
		active, err := d.visionSvc.GetActive()
		if err == nil && active != nil && active.ApiKey != "" {
			cap.Understand = true
			cap.OCR = true
			cap.Provider = active.ApiType
			cap.Model = active.ModelName
			cap.ExecutionLocation = LocationCloud
		}
	}

	if d.providerRegistry != nil {
		names := d.providerRegistry.Names()
		for _, name := range names {
			if name == "seedream" {
				cap.Generate = true
				if cap.Provider == "" {
					cap.Provider = "seedream"
				}
				break
			}
		}
	}

	d.cached = &cap
	d.cachedAt = time.Now()
	return cap
}

func (d *CapabilityDetector) Invalidate() {
	d.mu.Lock()
	d.cached = nil
	d.mu.Unlock()
}
