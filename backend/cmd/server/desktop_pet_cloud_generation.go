package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/imageprovider/cloudbridge"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/pkg/app"
)

func registerCloudDesktopPetGenerationBridge(r *gin.Engine, ctx *app.AppContext, services *AppServices, webAccessSvc *security.WebAccessService) error {
	if services == nil || services.RuntimeProfile != runtimeprofile.ProfileCloudCore {
		return nil
	}
	if services.DeviceMesh == nil || services.DeviceMesh.CredentialSvc == nil {
		return fmt.Errorf("desktop pet cloud generation: device mesh credential service is required")
	}
	registry, err := desktoppet.NewProviderRegistry()
	if err != nil {
		return fmt.Errorf("desktop pet cloud generation: provider registry: %w", err)
	}
	repo := imagegen.NewRepository(ctx.DB)

	group := r.Group("/api/device-mesh/v1/desktop-pet")
	group.Use(credential.DeviceAuthMiddleware(services.DeviceMesh.CredentialSvc, services.DeviceMesh.DeviceReg))
	if webAccessSvc != nil {
		group.Use(security.RequireWebAccessForDeviceRuntimePrincipal(webAccessSvc, services.DeviceMesh.DeviceReg))
	}
	group.POST("/image-generation", func(c *gin.Context) {
		principal, ok := credential.GinPrincipal(c)
		if !ok || principal.SpaceID == "" || principal.DeviceID == "" || principal.RuntimeID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "mesh.credential_invalid", "message": "device credential required"})
			return
		}

		var req cloudbridge.Request
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "desktop_pet.imagegen.invalid_request", "message": err.Error()})
			return
		}
		if req.ConfigID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "desktop_pet.imagegen.config_required", "message": "configId is required"})
			return
		}

		cfg, err := repo.GetByID(req.ConfigID)
		if err != nil || cfg == nil {
			c.JSON(http.StatusNotFound, gin.H{"code": "desktop_pet.imagegen.config_not_found", "message": "image generation config not found"})
			return
		}
		if cfg.Enabled == 0 {
			c.JSON(http.StatusConflict, gin.H{"code": "desktop_pet.imagegen.config_disabled", "message": "image generation config is disabled"})
			return
		}
		if strings.TrimSpace(cfg.ApiKey) == "" {
			c.JSON(http.StatusConflict, gin.H{"code": "desktop_pet.imagegen.credential_missing", "message": "image generation provider credential is missing"})
			return
		}

		providerName := imageprovider.NormalizeProviderName(cfg.ApiType)
		if providerName == "" {
			providerName = "seedream"
		}
		provider, found := registry.Resolve(providerName)
		if !found || provider == nil {
			c.JSON(http.StatusConflict, gin.H{"code": "desktop_pet.imagegen.provider_unavailable", "message": "image generation provider is unavailable"})
			return
		}
		modelConfig := imageprovider.ImageModelConfig{
			ConfigID:  cfg.ID,
			Name:      cfg.Name,
			ApiType:   cfg.ApiType,
			ApiKey:    cfg.ApiKey,
			ModelName: cfg.ModelName,
			BaseUrl:   cfg.BaseUrl,
		}
		metadata := &cloudbridge.ConfigMetadata{
			ConfigID:  cfg.ID,
			Name:      cfg.Name,
			Provider:  providerName,
			Model:     cfg.ModelName,
			Revision:  cfg.UpdatedAt,
			Enabled:   cfg.Enabled != 0,
			HasAPIKey: strings.TrimSpace(cfg.ApiKey) != "",
		}

		out := cloudbridge.Response{Config: metadata}
		switch req.Operation {
		case cloudbridge.OperationDescribe:
			// Metadata only. Never return provider secrets to the device.
		case cloudbridge.OperationValidate:
			if err := provider.ValidateConfig(c.Request.Context(), modelConfig); err != nil {
				writeCloudImageGenerationError(c, http.StatusConflict, "desktop_pet.imagegen.config_invalid", err)
				return
			}
		case cloudbridge.OperationCapabilities:
			caps, err := provider.Capabilities(c.Request.Context(), modelConfig)
			if err != nil {
				writeCloudImageGenerationError(c, http.StatusBadGateway, "desktop_pet.imagegen.capabilities_failed", err)
				return
			}
			out.Capabilities = &caps
		case cloudbridge.OperationExtendedCapabilities:
			caps, err := resolveExtendedImageCapabilities(c.Request.Context(), provider, providerName, cfg.ModelName, modelConfig)
			if err != nil {
				writeCloudImageGenerationError(c, http.StatusBadGateway, "desktop_pet.imagegen.capabilities_failed", err)
				return
			}
			out.ExtendedCapabilities = &caps
		case cloudbridge.OperationSubmit:
			if req.GenerationRequest == nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": "desktop_pet.imagegen.generation_request_required", "message": "generationRequest is required"})
				return
			}
			submission, err := provider.Submit(c.Request.Context(), modelConfig, *req.GenerationRequest)
			if err != nil {
				writeCloudImageGenerationError(c, http.StatusBadGateway, "desktop_pet.imagegen.submit_failed", err)
				return
			}
			out.Submission = submission
		case cloudbridge.OperationQuery:
			if strings.TrimSpace(req.OperationID) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"code": "desktop_pet.imagegen.operation_required", "message": "operationId is required"})
				return
			}
			result, err := provider.Query(c.Request.Context(), modelConfig, req.OperationID)
			if err != nil {
				writeCloudImageGenerationError(c, http.StatusBadGateway, "desktop_pet.imagegen.query_failed", err)
				return
			}
			out.Result = result
		case cloudbridge.OperationCancel:
			if strings.TrimSpace(req.OperationID) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"code": "desktop_pet.imagegen.operation_required", "message": "operationId is required"})
				return
			}
			if err := provider.Cancel(c.Request.Context(), modelConfig, req.OperationID); err != nil {
				writeCloudImageGenerationError(c, http.StatusBadGateway, "desktop_pet.imagegen.cancel_failed", err)
				return
			}
		default:
			c.JSON(http.StatusBadRequest, gin.H{"code": "desktop_pet.imagegen.operation_invalid", "message": "unsupported image generation operation"})
			return
		}

		c.JSON(http.StatusOK, out)
	})
	return nil
}

func resolveExtendedImageCapabilities(
	ctx context.Context,
	provider imageprovider.ImageGenerationProvider,
	providerName string,
	modelName string,
	modelConfig imageprovider.ImageModelConfig,
) (imageprovider.ProviderCapabilities, error) {
	if extended, ok := provider.(imageprovider.ExtendedProvider); ok {
		return extended.ExtendedCapabilities(ctx, modelConfig)
	}
	basic, err := provider.Capabilities(ctx, modelConfig)
	if err != nil {
		return imageprovider.ProviderCapabilities{}, err
	}
	return imageprovider.ProviderCapabilities{
		SchemaVersion:          1,
		Provider:               providerName,
		Model:                  modelName,
		SupportedModes:         []imageprovider.GenerationMode{imageprovider.ModeSpriteSheet, imageprovider.ModeLegacyFrame},
		SupportsReferenceImage: basic.SupportsReferenceImage,
		SupportsMultipleImages: basic.SupportsMultipleImages,
		SupportsNegativePrompt: basic.SupportsNegativePrompt,
		SupportsSeed:           basic.SupportsSeed,
		SupportsAsyncOperation: basic.SupportsAsyncOperation,
		SupportsCancellation:   basic.SupportsCancellation,
		MaxReferenceImages:     basic.MaxReferenceImages,
		MaxOutputImages:        basic.MaxOutputImages,
	}, nil
}

func writeCloudImageGenerationError(c *gin.Context, status int, code string, err error) {
	message := "image generation request failed"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	c.JSON(status, gin.H{"code": code, "message": message})
}
