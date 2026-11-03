package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/imageprovider/cloudbridge"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

const maxCloudImageGenerationBody = 96 << 20

func registerCloudImageGenerationGateway(group *gin.RouterGroup, services *AppServices) {
	if group == nil || services == nil || services.RuntimeProfile != runtimeprofile.ProfileCloudCore {
		return
	}
	group.POST(cloudbridge.EndpointPath, func(c *gin.Context) {
		if services.ImageGen == nil || services.ImageProviders == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "IMAGE_GENERATION_UNAVAILABLE", "message": "image generation services are unavailable"})
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxCloudImageGenerationBody+1))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "IMAGE_GENERATION_REQUEST_INVALID", "message": err.Error()})
			return
		}
		if len(body) > maxCloudImageGenerationBody {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": "IMAGE_GENERATION_REQUEST_TOO_LARGE", "message": "cloud image generation request exceeds size limit"})
			return
		}
		var request cloudbridge.Request
		if err := json.Unmarshal(body, &request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "IMAGE_GENERATION_REQUEST_INVALID", "message": err.Error()})
			return
		}
		config, err := cloudImageGenerationConfig(services, request.ConfigID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": "IMAGE_GENERATION_CONFIG_NOT_FOUND", "message": err.Error()})
			return
		}
		provider, ok := services.ImageProviders.Resolve(config.ApiType)
		if !ok {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "IMAGE_PROVIDER_NOT_REGISTERED", "message": "image provider is not registered: " + config.ApiType})
			return
		}
		providerConfig := imageprovider.ImageModelConfig{
			ConfigID:  config.ID,
			Name:      config.Name,
			ApiType:   config.ApiType,
			ApiKey:    config.ApiKey,
			ModelName: config.ModelName,
			BaseUrl:   config.BaseUrl,
		}
		response := cloudbridge.Response{}
		switch request.Operation {
		case cloudbridge.OperationDescribe:
			response.Config = &cloudbridge.ConfigMetadata{
				ConfigID:  config.ID,
				Name:      config.Name,
				Provider:  config.ApiType,
				Model:     config.ModelName,
				Revision:  config.UpdatedAt,
				Enabled:   config.Enabled != 0,
				HasAPIKey: config.ApiKey != "",
			}
		case cloudbridge.OperationValidate:
			if err := provider.ValidateConfig(c.Request.Context(), providerConfig); err != nil {
				writeCloudImageGenerationError(c, err)
				return
			}
		case cloudbridge.OperationCapabilities:
			capabilities, err := provider.Capabilities(c.Request.Context(), providerConfig)
			if err != nil {
				writeCloudImageGenerationError(c, err)
				return
			}
			response.Capabilities = &capabilities
		case cloudbridge.OperationExtendedCapabilities:
			extended, ok := provider.(imageprovider.ExtendedProvider)
			if !ok {
				c.JSON(http.StatusNotImplemented, gin.H{"code": "IMAGE_PROVIDER_CAPABILITIES_UNAVAILABLE", "message": "provider does not expose extended capabilities"})
				return
			}
			capabilities, err := extended.ExtendedCapabilities(c.Request.Context(), providerConfig)
			if err != nil {
				writeCloudImageGenerationError(c, err)
				return
			}
			response.ExtendedCapabilities = &capabilities
		case cloudbridge.OperationSubmit:
			if request.GenerationRequest == nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": "IMAGE_GENERATION_REQUEST_INVALID", "message": "generationRequest is required"})
				return
			}
			submission, err := provider.Submit(c.Request.Context(), providerConfig, *request.GenerationRequest)
			if err != nil {
				writeCloudImageGenerationError(c, err)
				return
			}
			response.Submission = submission
		case cloudbridge.OperationQuery:
			result, err := provider.Query(c.Request.Context(), providerConfig, request.OperationID)
			if err != nil {
				writeCloudImageGenerationError(c, err)
				return
			}
			response.Result = result
		case cloudbridge.OperationCancel:
			if err := provider.Cancel(c.Request.Context(), providerConfig, request.OperationID); err != nil {
				writeCloudImageGenerationError(c, err)
				return
			}
		default:
			c.JSON(http.StatusBadRequest, gin.H{"code": "IMAGE_GENERATION_OPERATION_INVALID", "message": "unsupported cloud image generation operation"})
			return
		}
		c.JSON(http.StatusOK, response)
	})
}

func cloudImageGenerationConfig(services *AppServices, configID int) (*imagegen.ImageGenConfig, error) {
	if configID <= 0 {
		return services.ImageGen.GetActive()
	}
	return services.ImageGen.GetRuntimeByID(configID)
}

func writeCloudImageGenerationError(c *gin.Context, err error) {
	status := http.StatusBadGateway
	var providerErr *imageprovider.ProviderError
	if errors.As(err, &providerErr) && providerErr.HTTPStatus >= 400 && providerErr.HTTPStatus <= 599 {
		status = providerErr.HTTPStatus
	}
	c.JSON(status, gin.H{"code": "IMAGE_GENERATION_FAILED", "message": err.Error(), "error": err})
}
