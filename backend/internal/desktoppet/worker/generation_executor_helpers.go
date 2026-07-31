package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/generation"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/log"
)

func (e *GenerationExecutor) resolveReferenceImages(plan *generation.GenerationPlanSnapshot, task *desktoppet.GenerationTask) ([]imageprovider.ImageInput, error) {
	if plan == nil || plan.ReferenceAssetID == "" {
		sourceAbsPath := filepath.Join(config.AppCfg.Storage.DataDir, task.SourceImagePath)
		return desktoppet.SelectReferenceImages(sourceAbsPath, "", false), nil
	}

	refAsset, err := e.refAssetRepo.GetReferenceAssetByID(plan.ReferenceAssetID)
	if err != nil {
		return nil, fmt.Errorf("get reference asset %s: %w", plan.ReferenceAssetID, err)
	}
	if refAsset == nil {
		sourceAbsPath := filepath.Join(config.AppCfg.Storage.DataDir, task.SourceImagePath)
		return desktoppet.SelectReferenceImages(sourceAbsPath, "", false), nil
	}

	imagePath := refAsset.NormalizedPath
	if imagePath == "" {
		imagePath = refAsset.SourcePath
	}
	if imagePath == "" {
		return nil, fmt.Errorf("reference asset %s has no valid image path", plan.ReferenceAssetID)
	}

	absPath := filepath.Join(config.AppCfg.Storage.DataDir, imagePath)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		log.Logger.Warnf("desktoppet executor reference asset file not found, falling back to task source: %s", absPath)
		sourceAbsPath := filepath.Join(config.AppCfg.Storage.DataDir, task.SourceImagePath)
		return desktoppet.SelectReferenceImages(sourceAbsPath, "", false), nil
	}

	imageData, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read reference asset file: %w", err)
	}

	mime := refAsset.NormalizedMIME
	if mime == "" {
		mime = refAsset.SourceMIME
	}

	return []imageprovider.ImageInput{
		{
			Path:     absPath,
			Bytes:    imageData,
			MimeType: mime,
		},
	}, nil
}

func (e *GenerationExecutor) persistProviderReceipt(attemptID, providerID, model string, submission *imageprovider.ImageGenerationSubmission, requestHash string) error {
	if e.receiptRepo == nil {
		return nil
	}
	if submission == nil {
		return fmt.Errorf("submission is nil")
	}

	receipt := &generation.ProviderReceipt{
		AttemptID:         attemptID,
		ProviderID:        providerID,
		Model:             model,
		ProviderRequestID: submission.RequestID,
		ProviderTaskID:    submission.OperationID,
		SubmittedAt:       nowRFC3339Worker(),
		RequestHash:       requestHash,
		ProviderStatus:    submission.Status,
	}

	if submission.Result != nil {
		responseHash := computeResponseHash(submission.Result)
		receipt.ResponseHash = responseHash
		if submission.Result.Status == "succeeded" || submission.Result.Status == "failed" {
			receipt.CompletedAt = nowRFC3339Worker()
			receipt.ProviderStatus = submission.Result.Status
		}
		if submission.Result.RawMetadata != nil {
			rawMeta, _ := json.Marshal(submission.Result.RawMetadata)
			receipt.RawMetadataJSON = string(rawMeta)
		}
	}

	if err := e.receiptRepo.Create(nil, receipt); err != nil {
		return fmt.Errorf("create provider receipt: %w", err)
	}
	return nil
}

func computeRequestHash(request imageprovider.ImageGenerationRequest) string {
	data, err := json.Marshal(request)
	if err != nil {
		return ""
	}
	return computeSHA256Hex(string(data))
}

func computeResponseHash(result *imageprovider.ImageGenerationResult) string {
	if result == nil {
		return ""
	}
	summary := map[string]interface{}{
		"status":      result.Status,
		"operationID": result.OperationID,
		"requestID":   result.RequestID,
		"provider":    result.Provider,
		"model":       result.Model,
		"imageCount":  len(result.Images),
	}
	if result.Usage != nil {
		summary["usage"] = result.Usage
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return ""
	}
	return computeSHA256Hex(string(data))
}

func nowRFC3339Worker() string {
	return time.Now().UTC().Format(time.RFC3339)
}
