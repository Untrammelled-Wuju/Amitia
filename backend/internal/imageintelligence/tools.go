package imageintelligence

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	ToolID_Status    = "image.internal.status"
	ToolID_Understand = "media.image.understand"
	ToolID_OCR       = "media.image.ocr"
	ToolID_Generate  = "media.image.generate"
)

type ToolHandler struct {
	facade *Facade
}

func NewToolHandler(facade *Facade) *ToolHandler {
	return &ToolHandler{facade: facade}
}

func (h *ToolHandler) Dispatch(ctx context.Context, handlerName string, input json.RawMessage) (json.RawMessage, error) {
	switch handlerName {
	case "status":
		return h.handleStatus(ctx, input)
	case "understand":
		return h.handleUnderstand(ctx, input)
	case "ocr":
		return h.handleOCR(ctx, input)
	case "generate":
		return h.handleGenerate(ctx, input)
	default:
		return nil, fmt.Errorf("unknown image intelligence handler: %s", handlerName)
	}
}

func (h *ToolHandler) handleStatus(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	status := h.facade.Status(ctx)
	return marshalResult(status), nil
}

func (h *ToolHandler) handleUnderstand(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req ImageUnderstandRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return marshalErrorResponse(&Error{Code: ErrInvalid, Message: fmt.Sprintf("invalid input: %v", err), HTTPStatus: 400}), nil
	}

	result, err := h.facade.Understand(ctx, req)
	if err != nil {
		if ie, ok := err.(*Error); ok {
			return marshalErrorResponse(ie), nil
		}
		return marshalErrorResponse(&Error{Code: ErrUnderstandFailed, Message: err.Error(), HTTPStatus: 500}), nil
	}

	return marshalResult(result), nil
}

func (h *ToolHandler) handleOCR(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req ImageOCRRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return marshalErrorResponse(&Error{Code: ErrInvalid, Message: fmt.Sprintf("invalid input: %v", err), HTTPStatus: 400}), nil
	}

	result, err := h.facade.OCR(ctx, req)
	if err != nil {
		if ie, ok := err.(*Error); ok {
			return marshalErrorResponse(ie), nil
		}
		return marshalErrorResponse(&Error{Code: ErrOCRFailed, Message: err.Error(), HTTPStatus: 500}), nil
	}

	return marshalResult(result), nil
}

func (h *ToolHandler) handleGenerate(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req ImageGenerateRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return marshalErrorResponse(&Error{Code: ErrInvalid, Message: fmt.Sprintf("invalid input: %v", err), HTTPStatus: 400}), nil
	}

	result, err := h.facade.Generate(ctx, req)
	if err != nil {
		if ie, ok := err.(*Error); ok {
			return marshalErrorResponse(ie), nil
		}
		return marshalErrorResponse(&Error{Code: ErrGenFailed, Message: err.Error(), HTTPStatus: 500}), nil
	}

	return marshalResult(result), nil
}

func BuildToolDefinitions(handler *ToolHandler) []capability.ToolDefinition {
	understandSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"required": ["resourceUri"],
		"properties": {
			"resourceUri": {"type": "string"},
			"prompt": {"type": "string"},
			"detail": {"enum": ["auto", "low", "high"]}
		}
	}`)

	ocrSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"required": ["resourceUri"],
		"properties": {
			"resourceUri": {"type": "string"},
			"languageHints": {"type": "array", "items": {"type": "string"}},
			"includeBoxes": {"type": "boolean"}
		}
	}`)

	generateSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"required": ["prompt"],
		"properties": {
			"prompt": {"type": "string", "maxLength": 4096},
			"count": {"type": "integer", "minimum": 1, "maximum": 4},
			"width": {"type": "integer", "minimum": 256, "maximum": 4096},
			"height": {"type": "integer", "minimum": 256, "maximum": 4096},
			"quality": {"enum": ["standard", "hd"]}
		}
	}`)

	return []capability.ToolDefinition{
		{
			ID:           ToolID_Understand,
			ModelName:    "media_image_understand",
			Source:       capability.ToolSourceInternal,
			Name:         "Analyze Image",
			Description:  "Analyze the content of an image. Describe scenes, objects, people, text, and any visible information.",
			InputSchema:  understandSchema,
			Permissions:  []capability.PermissionRequirement{{Capability: "media.image.read", Risk: string(capability.RiskLow)}},
			RiskLevel:    capability.RiskLow,
			SideEffect:   capability.SideEffectExternal,
			Internal:     true,
			Enabled:      true,
			Idempotent:   true,
			Retryable:    true,
			TimeoutMS:    60000,
			Runtime:      capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeInternal, RuntimeID: "imageintelligence", HandlerName: "understand"},
		ModelExposure: capability.ModelExposureRule{ExposedByDefault: true},
	},
	{
		ID:           ToolID_OCR,
		ModelName:    "media_image_ocr",
		Source:       capability.ToolSourceInternal,
		Name:         "OCR Image",
		Description:  "Extract text from an image using optical character recognition. Returns the recognized text content.",
		InputSchema:  ocrSchema,
		Permissions:  []capability.PermissionRequirement{{Capability: "media.image.read", Risk: string(capability.RiskLow)}},
		RiskLevel:    capability.RiskLow,
		SideEffect:   capability.SideEffectExternal,
		Internal:     true,
		Enabled:      true,
		Idempotent:   true,
		Retryable:    true,
		TimeoutMS:    30000,
		Runtime:      capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeInternal, RuntimeID: "imageintelligence", HandlerName: "ocr"},
		ModelExposure: capability.ModelExposureRule{ExposedByDefault: true},
	},
	{
		ID:           ToolID_Generate,
		ModelName:    "media_image_generate",
		Source:       capability.ToolSourceInternal,
		Name:         "Generate Image",
		Description:  "Generate images from text descriptions.",
		InputSchema:  generateSchema,
		Permissions:  []capability.PermissionRequirement{{Capability: "media.image.generate", Risk: string(capability.RiskMedium)}},
		RiskLevel:    capability.RiskMedium,
		SideEffect:   capability.SideEffectExternal,
		Internal:     true,
		HasSideEffects: true,
		Enabled:      true,
		Idempotent:   true,
		Retryable:    false,
		TimeoutMS:    120000,
		Runtime:      capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeInternal, RuntimeID: "imageintelligence", HandlerName: "generate"},
		ModelExposure: capability.ModelExposureRule{ExposedByDefault: true},
	},
}
}

func BuildInternalStatusToolDefinition() capability.ToolDefinition {
	statusSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {}
	}`)

	return capability.ToolDefinition{
		ID:           ToolID_Status,
		ModelName:    "",
		Source:       capability.ToolSourceInternal,
		Name:         "Image Intelligence Status",
		Description:  "Check image intelligence capability status",
		InputSchema:  statusSchema,
		Permissions:  []capability.PermissionRequirement{{Capability: "media.image.read", Risk: string(capability.RiskLow)}},
		RiskLevel:    capability.RiskLow,
		SideEffect:   capability.SideEffectReadOnly,
		Internal:     true,
		Enabled:      true,
		Idempotent:   true,
		Retryable:    false,
		TimeoutMS:    5000,
		Runtime:      capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeInternal, RuntimeID: "imageintelligence", HandlerName: "status"},
		ModelExposure: capability.ModelExposureRule{ExposedByDefault: false},
	}
}
