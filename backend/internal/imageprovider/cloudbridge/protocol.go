package cloudbridge

import "github.com/u-ai/backend/internal/imageprovider"

const (
	ProviderName = "cloud_core"
	EndpointPath = "/api/device-mesh/v1/desktop-pet/image-generation"
)

type Operation string

const (
	OperationDescribe             Operation = "describe"
	OperationValidate             Operation = "validate"
	OperationCapabilities         Operation = "capabilities"
	OperationExtendedCapabilities Operation = "extended_capabilities"
	OperationSubmit               Operation = "submit"
	OperationQuery                Operation = "query"
	OperationCancel               Operation = "cancel"
)

type ConfigMetadata struct {
	ConfigID  int    `json:"configId"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Revision  string `json:"revision"`
	Enabled   bool   `json:"enabled"`
	HasAPIKey bool   `json:"hasApiKey"`
}

type Request struct {
	Operation         Operation                             `json:"operation"`
	ConfigID          int                                   `json:"configId"`
	GenerationRequest *imageprovider.ImageGenerationRequest `json:"generationRequest,omitempty"`
	OperationID       string                                `json:"operationId,omitempty"`
}

type Response struct {
	Config               *ConfigMetadata                            `json:"config,omitempty"`
	Capabilities         *imageprovider.ImageGenerationCapabilities `json:"capabilities,omitempty"`
	ExtendedCapabilities *imageprovider.ProviderCapabilities        `json:"extendedCapabilities,omitempty"`
	Submission           *imageprovider.ImageGenerationSubmission   `json:"submission,omitempty"`
	Result               *imageprovider.ImageGenerationResult       `json:"result,omitempty"`
}
