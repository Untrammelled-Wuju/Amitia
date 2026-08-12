package imageintelligence

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/pkg/resourceuri"
)

type ImageIntelligence interface {
	Understand(ctx context.Context, req ImageUnderstandRequest) (*ImageUnderstandResult, error)
	OCR(ctx context.Context, req ImageOCRRequest) (*ImageOCRResult, error)
	Generate(ctx context.Context, req ImageGenerateRequest) (*ImageGenerateResult, error)
	Status(ctx context.Context) ImageIntelligenceStatus
}

type ImageIntelligenceStatus struct {
	Capabilities ImageCapabilities `json:"capabilities"`
	Healthy      bool              `json:"healthy"`
}

type Facade struct {
	understand *UnderstandProvider
	ocr        *OCRProvider
	generate   *GenerateProvider
	resolver   *ImageResourceResolver
	detector   *CapabilityDetector
	mu         sync.RWMutex
}

func NewFacade(understand *UnderstandProvider, ocr *OCRProvider, generate *GenerateProvider, resolver *ImageResourceResolver, detector *CapabilityDetector) *Facade {
	return &Facade{
		understand: understand,
		ocr:        ocr,
		generate:   generate,
		resolver:   resolver,
		detector:   detector,
	}
}

func (f *Facade) Understand(ctx context.Context, req ImageUnderstandRequest) (*ImageUnderstandResult, error) {
	caps := f.detector.Detect(ctx)
	if !caps.Understand {
		return nil, &Error{Code: ErrUnAvailable, Message: "image understanding is not available", HTTPStatus: 503}
	}

	imageData, summary, resErr := f.resolver.ResolveAndValidate(ctx, req.Image, caps)
	if resErr != nil {
		return nil, resErr
	}

	result, err := f.understand.Understand(ctx, req, imageData, summary)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (f *Facade) OCR(ctx context.Context, req ImageOCRRequest) (*ImageOCRResult, error) {
	caps := f.detector.Detect(ctx)
	if !caps.OCR {
		return nil, &Error{Code: ErrOCRUnavailable, Message: "OCR is not available", HTTPStatus: 503}
	}

	imageData, summary, resErr := f.resolver.ResolveAndValidate(ctx, req.Image, caps)
	if resErr != nil {
		return nil, resErr
	}

	result, err := f.ocr.OCR(ctx, req, imageData, summary)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (f *Facade) Generate(ctx context.Context, req ImageGenerateRequest) (*ImageGenerateResult, error) {
	caps := f.detector.Detect(ctx)
	if !caps.Generate {
		return nil, &Error{Code: ErrGenUnavailable, Message: "image generation is not available", HTTPStatus: 503}
	}

	result, err := f.generate.Generate(ctx, req)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (f *Facade) Status(ctx context.Context) ImageIntelligenceStatus {
	caps := f.detector.Detect(ctx)
	return ImageIntelligenceStatus{
		Capabilities: caps,
		Healthy:      caps.Understand || caps.OCR || caps.Generate,
	}
}

type ImageIntelligenceFactory struct {
	visionSvc       vision.Service
	imagegenSvc     imagegen.Service
	providerRegistry *imageprovider.Registry
	resourceResolver *resourceuri.PhysicalResolver
	mu              sync.Mutex
	cached          *Facade
}

func NewImageIntelligenceFactory(visionSvc vision.Service, imagegenSvc imagegen.Service, providerRegistry *imageprovider.Registry, resourceResolver *resourceuri.PhysicalResolver) *ImageIntelligenceFactory {
	return &ImageIntelligenceFactory{
		visionSvc:       visionSvc,
		imagegenSvc:     imagegenSvc,
		providerRegistry: providerRegistry,
		resourceResolver: resourceResolver,
	}
}

func (factory *ImageIntelligenceFactory) Build() *Facade {
	factory.mu.Lock()
	defer factory.mu.Unlock()

	if factory.cached != nil {
		return factory.cached
	}

	understand := NewUnderstandProvider(factory.visionSvc)
	ocr := NewOCRProvider(factory.visionSvc)
	generate := NewGenerateProvider(factory.imagegenSvc, factory.providerRegistry)
	resolver := NewImageResourceResolver(factory.resourceResolver)
	detector := NewCapabilityDetector(factory.visionSvc, factory.providerRegistry)

	factory.cached = NewFacade(understand, ocr, generate, resolver, detector)
	return factory.cached
}

func marshalResult(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"error":"marshal failed"}`)
	}
	return data
}
