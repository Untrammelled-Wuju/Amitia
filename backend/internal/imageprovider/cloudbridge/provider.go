package cloudbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/imageprovider"
)

const (
	maxReferenceImageBytes = 32 << 20
	maxResponseBytes       = 96 << 20
)

type Provider struct {
	credentialStore *agent.CredentialStore
	client          *http.Client
}

func NewProvider(dataDir string) *Provider {
	return &Provider{
		credentialStore: agent.NewCredentialStore(dataDir),
		client:          &http.Client{Timeout: 3 * time.Minute},
	}
}

func (p *Provider) DescribeConfig(ctx context.Context, configID int) (*ConfigMetadata, error) {
	resp, err := p.call(ctx, Request{Operation: OperationDescribe, ConfigID: configID})
	if err != nil {
		return nil, err
	}
	if resp.Config == nil {
		return nil, fmt.Errorf("cloud image generation bridge returned no config metadata")
	}
	return resp.Config, nil
}

func (p *Provider) ValidateConfig(ctx context.Context, config imageprovider.ImageModelConfig) error {
	_, err := p.call(ctx, Request{Operation: OperationValidate, ConfigID: config.ConfigID})
	return err
}

func (p *Provider) Capabilities(ctx context.Context, config imageprovider.ImageModelConfig) (imageprovider.ImageGenerationCapabilities, error) {
	resp, err := p.call(ctx, Request{Operation: OperationCapabilities, ConfigID: config.ConfigID})
	if err != nil {
		return imageprovider.ImageGenerationCapabilities{}, err
	}
	if resp.Capabilities == nil {
		return imageprovider.ImageGenerationCapabilities{}, fmt.Errorf("cloud image generation bridge returned no capabilities")
	}
	return *resp.Capabilities, nil
}

func (p *Provider) ExtendedCapabilities(ctx context.Context, config imageprovider.ImageModelConfig) (imageprovider.ProviderCapabilities, error) {
	resp, err := p.call(ctx, Request{Operation: OperationExtendedCapabilities, ConfigID: config.ConfigID})
	if err != nil {
		return imageprovider.ProviderCapabilities{}, err
	}
	if resp.ExtendedCapabilities == nil {
		return imageprovider.ProviderCapabilities{}, fmt.Errorf("cloud image generation bridge returned no extended capabilities")
	}
	return *resp.ExtendedCapabilities, nil
}

func (p *Provider) Submit(ctx context.Context, config imageprovider.ImageModelConfig, request imageprovider.ImageGenerationRequest) (*imageprovider.ImageGenerationSubmission, error) {
	sanitized, err := materializeReferenceImages(request)
	if err != nil {
		return nil, err
	}
	resp, err := p.call(ctx, Request{Operation: OperationSubmit, ConfigID: config.ConfigID, GenerationRequest: &sanitized})
	if err != nil {
		return nil, err
	}
	if resp.Submission == nil {
		return nil, fmt.Errorf("cloud image generation bridge returned no submission")
	}
	return resp.Submission, nil
}

func (p *Provider) Query(ctx context.Context, config imageprovider.ImageModelConfig, operationID string) (*imageprovider.ImageGenerationResult, error) {
	resp, err := p.call(ctx, Request{Operation: OperationQuery, ConfigID: config.ConfigID, OperationID: operationID})
	if err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return nil, fmt.Errorf("cloud image generation bridge returned no query result")
	}
	return resp.Result, nil
}

func (p *Provider) Cancel(ctx context.Context, config imageprovider.ImageModelConfig, operationID string) error {
	_, err := p.call(ctx, Request{Operation: OperationCancel, ConfigID: config.ConfigID, OperationID: operationID})
	return err
}

func (p *Provider) call(ctx context.Context, payload Request) (*Response, error) {
	if payload.ConfigID <= 0 {
		return nil, fmt.Errorf("cloud image generation config id is required")
	}
	if p == nil || p.credentialStore == nil {
		return nil, fmt.Errorf("cloud image generation bridge is not configured")
	}
	cred, err := p.credentialStore.LoadCredential()
	if err != nil {
		return nil, fmt.Errorf("load device mesh credential: %w", err)
	}
	if cred == nil || strings.TrimSpace(cred.CloudBaseUrl) == "" || strings.TrimSpace(cred.Credential) == "" {
		return nil, fmt.Errorf("device is not connected to cloud core")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal cloud image generation request: %w", err)
	}
	endpoint := strings.TrimRight(cred.CloudBaseUrl, "/") + EndpointPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "AmitiaDevice "+cred.Credential)
	req.Header.Set("X-Amitia-Device-ID", cred.DeviceID.String())
	req.Header.Set("X-Amitia-Runtime-ID", cred.RuntimeID.String())

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloud image generation request failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read cloud image generation response: %w", err)
	}
	if len(responseBody) > maxResponseBytes {
		return nil, fmt.Errorf("cloud image generation response exceeds %d bytes", maxResponseBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Msg     string `json:"msg"`
		}
		_ = json.Unmarshal(responseBody, &apiErr)
		message := strings.TrimSpace(apiErr.Message)
		if message == "" {
			message = strings.TrimSpace(apiErr.Msg)
		}
		if message == "" {
			message = strings.TrimSpace(string(responseBody))
		}
		return nil, fmt.Errorf("cloud image generation rejected request (%d): %s", resp.StatusCode, message)
	}
	var out Response
	if err := json.Unmarshal(responseBody, &out); err != nil {
		return nil, fmt.Errorf("decode cloud image generation response: %w", err)
	}
	return &out, nil
}

func materializeReferenceImages(request imageprovider.ImageGenerationRequest) (imageprovider.ImageGenerationRequest, error) {
	out := request
	out.ReferenceImages = make([]imageprovider.ImageInput, 0, len(request.ReferenceImages))
	var total int64
	for _, input := range request.ReferenceImages {
		copyInput := imageprovider.ImageInput{MimeType: input.MimeType}
		data := input.Bytes
		if len(data) == 0 && strings.TrimSpace(input.Path) != "" {
			var err error
			data, err = os.ReadFile(input.Path)
			if err != nil {
				return request, fmt.Errorf("read local reference image: %w", err)
			}
		}
		total += int64(len(data))
		if total > maxReferenceImageBytes {
			return request, fmt.Errorf("reference images exceed %d bytes", maxReferenceImageBytes)
		}
		copyInput.Bytes = append([]byte(nil), data...)
		out.ReferenceImages = append(out.ReferenceImages, copyInput)
	}
	return out, nil
}

var _ imageprovider.ExtendedProvider = (*Provider)(nil)
